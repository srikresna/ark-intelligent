// Package finviz scrapes Finviz cross-asset performance data via Firecrawl.
//
// Endpoints scraped:
//   - /futures.ashx        → major futures (indices, energy, metals, currencies)
//   - /groups.ashx?g=sector&v=140 → S&P sector performance
//
// Data is cached in-memory with 1h TTL and optionally persisted to BadgerDB.
// Rate limit: max 2 scrapes per page per hour (naturally enforced by cache TTL).
package finviz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("finviz")

const (
	firecrawlURL = "https://api.firecrawl.dev/v1/scrape"
	cacheTTL     = 1 * time.Hour
	fetchTimeout = 45 * time.Second

	badgerKeyFutures = "finviz:v1:futures"
	badgerKeySectors = "finviz:v1:sectors"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// FuturesItem represents a single futures contract performance.
type FuturesItem struct {
	Name      string  `json:"name"`       // e.g. "S&P 500", "Gold", "Crude Oil"
	Ticker    string  `json:"ticker"`     // e.g. "ESU24"
	LastPrice float64 `json:"last_price"` // latest price
	Change    float64 `json:"change"`     // daily change %
	Category  string  `json:"category"`   // "indices", "energy", "metals", "currencies", "grains", "softs", "meats", "bonds"
}

// SectorItem represents S&P sector performance.
type SectorItem struct {
	Name      string  `json:"name"`       // e.g. "Technology", "Healthcare"
	Change1D  float64 `json:"change_1d"`  // 1-day change %
	Change1W  float64 `json:"change_1w"`  // 1-week change %
	Change1M  float64 `json:"change_1m"`  // 1-month change %
	ChangeYTD float64 `json:"change_ytd"` // YTD change %
}

// CrossAssetData holds all Finviz cross-asset data.
type CrossAssetData struct {
	Futures   []FuturesItem `json:"futures"`
	Sectors   []SectorItem  `json:"sectors"`
	RiskTone  string        `json:"risk_tone"` // "RISK-ON", "RISK-OFF", "MIXED"
	Available bool          `json:"available"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

var (
	cachedData  *CrossAssetData
	cacheMu     sync.RWMutex
	cacheExpiry time.Time
	db          *badger.DB
)

// InitCache injects a BadgerDB instance for persistence.
// If db is nil, the cache operates in pure in-memory mode.
func InitCache(badgerDB *badger.DB) {
	cacheMu.Lock()
	db = badgerDB
	cacheMu.Unlock()
	log.Debug().Bool("persistence", badgerDB != nil).Msg("finviz cache initialized")
}

// InvalidateCache forces the next GetCachedOrFetch to re-scrape.
func InvalidateCache() {
	cacheMu.Lock()
	cachedData = nil
	cacheExpiry = time.Time{}
	cacheMu.Unlock()
}

// CacheAge returns how long since the last fetch, or -1 if no cached data.
func CacheAge() time.Duration {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if cachedData == nil {
		return -1
	}
	return time.Since(cachedData.FetchedAt)
}

// GetCachedOrFetch returns cached data if within TTL, else fetches fresh.
func GetCachedOrFetch(ctx context.Context) (*CrossAssetData, error) {
	// Fast path: in-memory cache.
	cacheMu.RLock()
	if cachedData != nil && time.Now().Before(cacheExpiry) {
		d := cachedData
		cacheMu.RUnlock()
		return d, nil
	}
	cacheMu.RUnlock()

	// Try BadgerDB.
	if db != nil {
		if d := loadFromBadger(); d != nil && time.Since(d.FetchedAt) < cacheTTL {
			cacheMu.Lock()
			cachedData = d
			cacheExpiry = d.FetchedAt.Add(cacheTTL)
			cacheMu.Unlock()
			return d, nil
		}
	}

	// Fetch fresh data.
	data, err := FetchAll(ctx)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	cachedData = data
	cacheExpiry = time.Now().Add(cacheTTL)
	cacheMu.Unlock()

	if db != nil {
		saveToBadger(data)
	}

	return data, nil
}

func loadFromBadger() *CrossAssetData {
	var data CrossAssetData
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(badgerKeyFutures))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &data)
		})
	})
	if err != nil {
		return nil
	}
	return &data
}

func saveToBadger(data *CrossAssetData) {
	b, err := json.Marshal(data)
	if err != nil {
		log.Debug().Err(err).Msg("finviz: failed to marshal for BadgerDB")
		return
	}
	_ = db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(badgerKeyFutures), b).WithTTL(cacheTTL)
		return txn.SetEntry(e)
	})
}

// ---------------------------------------------------------------------------
// Firecrawl Fetchers
// ---------------------------------------------------------------------------

type fcJSONOpts struct {
	Prompt string          `json:"prompt"`
	Schema json.RawMessage `json:"schema"`
}
type fcReq struct {
	URL         string      `json:"url"`
	Formats     []string    `json:"formats"`
	WaitFor     int         `json:"waitFor"`
	JSONOptions *fcJSONOpts `json:"jsonOptions,omitempty"`
}

// FetchAll fetches futures + sectors, classifies risk tone.
func FetchAll(ctx context.Context) (*CrossAssetData, error) {
	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("finviz: skipping — FIRECRAWL_API_KEY not set")
		return &CrossAssetData{FetchedAt: time.Now()}, nil
	}

	data := &CrossAssetData{FetchedAt: time.Now()}

	// Fetch futures and sectors concurrently.
	var wg sync.WaitGroup
	var futErr, secErr error
	var futures []FuturesItem
	var sectors []SectorItem

	wg.Add(2)
	go func() {
		defer wg.Done()
		futures, futErr = fetchFutures(ctx, apiKey)
	}()
	go func() {
		defer wg.Done()
		sectors, secErr = fetchSectors(ctx, apiKey)
	}()
	wg.Wait()

	if futErr != nil {
		log.Debug().Err(futErr).Msg("finviz: futures fetch failed")
	}
	if secErr != nil {
		log.Debug().Err(secErr).Msg("finviz: sectors fetch failed")
	}

	data.Futures = futures
	data.Sectors = sectors
	data.Available = len(futures) > 0 || len(sectors) > 0
	data.RiskTone = classifyRiskTone(futures, sectors)

	return data, nil
}

func fetchFutures(ctx context.Context, apiKey string) ([]FuturesItem, error) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"futures": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"name":       {"type": "string"},
						"ticker":     {"type": "string"},
						"last_price": {"type": "number"},
						"change":     {"type": "number"},
						"category":   {"type": "string"}
					}
				}
			}
		}
	}`)

	reqBody := fcReq{
		URL:     "https://finviz.com/futures.ashx",
		Formats: []string{"json"},
		WaitFor: 3000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract ALL futures data from the Finviz futures page. For each futures contract, extract: name (e.g. 'S&P 500', 'Gold', 'Crude Oil'), ticker symbol, last price, daily change as percentage number (e.g. -0.5 for -0.5%), and category (one of: indices, energy, metals, currencies, grains, softs, meats, bonds). Return all contracts shown on the page.",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	client := httpclient.New(httpclient.WithTimeout(fetchTimeout))
	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("non-2xx: %d", resp.StatusCode)
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				Futures []FuturesItem `json:"futures"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if !fcResp.Success {
		return nil, fmt.Errorf("firecrawl returned unsuccessful")
	}

	return fcResp.Data.JSON.Futures, nil
}

func fetchSectors(ctx context.Context, apiKey string) ([]SectorItem, error) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"sectors": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"name":       {"type": "string"},
						"change_1d":  {"type": "number"},
						"change_1w":  {"type": "number"},
						"change_1m":  {"type": "number"},
						"change_ytd": {"type": "number"}
					}
				}
			}
		}
	}`)

	reqBody := fcReq{
		URL:     "https://finviz.com/groups.ashx?g=sector&v=140",
		Formats: []string{"json"},
		WaitFor: 3000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract all S&P sector performance data. For each sector, extract: name, 1-day change %, 1-week change %, 1-month change %, and YTD change %. Return change values as numbers (e.g. -1.5 for -1.5%).",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	client := httpclient.New(httpclient.WithTimeout(fetchTimeout))
	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("non-2xx: %d", resp.StatusCode)
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				Sectors []SectorItem `json:"sectors"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if !fcResp.Success {
		return nil, fmt.Errorf("firecrawl returned unsuccessful")
	}

	return fcResp.Data.JSON.Sectors, nil
}

// ---------------------------------------------------------------------------
// Risk-On / Risk-Off Classification
// ---------------------------------------------------------------------------

// classifyRiskTone determines the cross-asset risk regime.
// Logic: equities up + gold down + yields up = RISK-ON; inverse = RISK-OFF.
func classifyRiskTone(futures []FuturesItem, sectors []SectorItem) string {
	if len(futures) == 0 {
		return "MIXED"
	}

	var equityScore, safeHavenScore int

	for _, f := range futures {
		switch f.Category {
		case "indices":
			if f.Change > 0 {
				equityScore++
			} else if f.Change < 0 {
				equityScore--
			}
		case "metals":
			// Gold up = safe haven demand = risk-off signal.
			if containsAny(f.Name, "Gold") {
				if f.Change > 0.3 {
					safeHavenScore++
				} else if f.Change < -0.3 {
					safeHavenScore--
				}
			}
		case "bonds":
			// Treasury futures up = yields down = risk-off signal.
			if f.Change > 0.2 {
				safeHavenScore++
			} else if f.Change < -0.2 {
				safeHavenScore--
			}
		}
	}

	// Sector leaders/laggards contribute to the signal.
	var sectorUp, sectorDown int
	for _, s := range sectors {
		if s.Change1D > 0 {
			sectorUp++
		} else if s.Change1D < 0 {
			sectorDown++
		}
	}

	// Breadth signal.
	if sectorUp > 0 && sectorDown > 0 {
		breadthRatio := float64(sectorUp) / float64(sectorUp+sectorDown)
		if breadthRatio > 0.7 {
			equityScore++
		} else if breadthRatio < 0.3 {
			equityScore--
		}
	}

	netSignal := equityScore - safeHavenScore
	switch {
	case netSignal >= 2:
		return "RISK-ON"
	case netSignal <= -2:
		return "RISK-OFF"
	default:
		return "MIXED"
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// FuturesByCategory returns futures filtered by category, sorted by change desc.
func FuturesByCategory(futures []FuturesItem, category string) []FuturesItem {
	var out []FuturesItem
	for _, f := range futures {
		if f.Category == category {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Change > out[j].Change
	})
	return out
}

// TopSectors returns the top N sectors by 1-day change.
func TopSectors(sectors []SectorItem, n int) []SectorItem {
	sorted := make([]SectorItem, len(sectors))
	copy(sorted, sectors)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Change1D > sorted[j].Change1D
	})
	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// BottomSectors returns the bottom N sectors by 1-day change.
func BottomSectors(sectors []SectorItem, n int) []SectorItem {
	sorted := make([]SectorItem, len(sectors))
	copy(sorted, sectors)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Change1D < sorted[j].Change1D
	})
	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// GreenRedCount returns the number of positive and negative change futures.
func GreenRedCount(futures []FuturesItem) (green, red int) {
	for _, f := range futures {
		if f.Change > 0 {
			green++
		} else if f.Change < 0 {
			red++
		}
	}
	return
}
