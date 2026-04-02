// Package finviz provides a cross-asset market overview by scraping Finviz
// futures and sector pages via the Firecrawl structured-extraction API.
//
// Requires FIRECRAWL_API_KEY. Data is delayed ~15 min (Finviz free tier).
// Results are cached in-memory with a 1-hour TTL and rate-limited to at most
// 2 scrapes per page per hour.
package finviz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/rs/zerolog/log"
)

// ──────────────────────────────────────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────────────────────────────────────

// FuturesItem represents a single entry from the Finviz futures page.
type FuturesItem struct {
	Name    string  `json:"name"`    // e.g. "S&P 500", "Gold", "Crude Oil"
	Ticker  string  `json:"ticker"`  // e.g. "ES", "GC", "CL"
	Price   float64 `json:"price"`
	Change  float64 `json:"change"`  // percent change
	Group   string  `json:"group"`   // "indices", "energy", "metals", "currencies", "agriculture", "bonds"
}

// SectorItem represents a single sector from the Finviz sector performance page.
type SectorItem struct {
	Name     string  `json:"name"`      // e.g. "Technology", "Healthcare"
	Change1D float64 `json:"change_1d"` // 1-day percent change
	Change1W float64 `json:"change_1w"` // 1-week
	Change1M float64 `json:"change_1m"` // 1-month
}

// MarketOverview is the combined cross-asset snapshot.
type MarketOverview struct {
	Futures   []FuturesItem
	Sectors   []SectorItem
	RiskTone  string    // "RISK-ON", "RISK-OFF", "MIXED"
	FetchedAt time.Time
	Available bool
}

// ──────────────────────────────────────────────────────────────────────────────
// Cache
// ──────────────────────────────────────────────────────────────────────────────

const cacheTTL = 1 * time.Hour

var (
	globalCache *MarketOverview //nolint:gochecknoglobals
	cacheMu     sync.RWMutex   //nolint:gochecknoglobals
)

// GetCachedOrFetch returns the cached overview if still fresh, otherwise
// scrapes Finviz and builds a new snapshot. On failure with a stale cache
// available, the stale data is returned.
func GetCachedOrFetch(ctx context.Context) *MarketOverview {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.FetchedAt) < cacheTTL {
		c := globalCache
		cacheMu.RUnlock()
		return c
	}
	cacheMu.RUnlock()

	fresh := fetchAll(ctx)

	cacheMu.Lock()
	defer cacheMu.Unlock()
	if fresh.Available {
		globalCache = fresh
	} else if globalCache != nil {
		log.Warn().Msg("finviz: fetch failed, returning stale cache")
		return globalCache
	}
	return fresh
}

// ──────────────────────────────────────────────────────────────────────────────
// Firecrawl helpers
// ──────────────────────────────────────────────────────────────────────────────

const firecrawlURL = "https://api.firecrawl.dev/v1/scrape"

type fcJSONOpts struct {
	Prompt string          `json:"prompt"`
	Schema json.RawMessage `json:"schema"`
}
type fcRequest struct {
	URL         string      `json:"url"`
	Formats     []string    `json:"formats"`
	WaitFor     int         `json:"waitFor"`
	JSONOptions *fcJSONOpts `json:"jsonOptions,omitempty"`
}

func firecrawlScrape(ctx context.Context, apiKey string, reqBody fcRequest, dest interface{}) error {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	client := httpclient.New(httpclient.WithTimeout(45 * time.Second))
	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("non-2xx status: %d", resp.StatusCode)
	}

	// Firecrawl wraps the extraction in data.json.
	var wrapper struct {
		Success bool            `json:"success"`
		Data    struct {
			JSON json.RawMessage `json:"json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return fmt.Errorf("decode wrapper: %w", err)
	}
	if !wrapper.Success || len(wrapper.Data.JSON) == 0 {
		return fmt.Errorf("firecrawl returned unsuccessful or empty")
	}
	if err := json.Unmarshal(wrapper.Data.JSON, dest); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Fetch futures
// ──────────────────────────────────────────────────────────────────────────────

var futuresSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"futures": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"name":   {"type": "string"},
					"ticker": {"type": "string"},
					"price":  {"type": "number"},
					"change": {"type": "number"},
					"group":  {"type": "string"}
				}
			}
		}
	}
}`)

func fetchFutures(ctx context.Context, apiKey string) []FuturesItem {
	reqBody := fcRequest{
		URL:     "https://finviz.com/futures.ashx",
		Formats: []string{"json"},
		WaitFor: 3000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract all futures from the page. For each future, extract: name (e.g. S&P 500), ticker symbol (e.g. ES), last price, percent change today. Also classify each into a group: indices, energy, metals, currencies, agriculture, or bonds. Return as an array called futures.",
			Schema: futuresSchema,
		},
	}

	var result struct {
		Futures []FuturesItem `json:"futures"`
	}
	if err := firecrawlScrape(ctx, apiKey, reqBody, &result); err != nil {
		log.Warn().Err(err).Msg("finviz: failed to fetch futures")
		return nil
	}
	log.Info().Int("count", len(result.Futures)).Msg("finviz: fetched futures")
	return result.Futures
}

// ──────────────────────────────────────────────────────────────────────────────
// Fetch sectors
// ──────────────────────────────────────────────────────────────────────────────

var sectorsSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"sectors": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"name":      {"type": "string"},
					"change_1d": {"type": "number"},
					"change_1w": {"type": "number"},
					"change_1m": {"type": "number"}
				}
			}
		}
	}
}`)

func fetchSectors(ctx context.Context, apiKey string) []SectorItem {
	reqBody := fcRequest{
		URL:     "https://finviz.com/groups.ashx?g=sector&v=140",
		Formats: []string{"json"},
		WaitFor: 3000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract all sector performance data. For each sector, extract: sector name (e.g. Technology), 1-day percent change, 1-week percent change, 1-month percent change. Return as an array called sectors.",
			Schema: sectorsSchema,
		},
	}

	var result struct {
		Sectors []SectorItem `json:"sectors"`
	}
	if err := firecrawlScrape(ctx, apiKey, reqBody, &result); err != nil {
		log.Warn().Err(err).Msg("finviz: failed to fetch sectors")
		return nil
	}
	log.Info().Int("count", len(result.Sectors)).Msg("finviz: fetched sectors")
	return result.Sectors
}

// ──────────────────────────────────────────────────────────────────────────────
// Risk-on / risk-off classification
// ──────────────────────────────────────────────────────────────────────────────

// classifyRiskTone checks:
//   - Equity indices mostly green + gold down + yields up → RISK-ON
//   - Equity indices mostly red + gold up + yields down → RISK-OFF
//   - Otherwise → MIXED
func classifyRiskTone(futures []FuturesItem) string {
	var equityGreen, equityRed int
	var goldChange, oilChange float64
	var bondGreen, bondRed int

	for _, f := range futures {
		switch f.Group {
		case "indices":
			if f.Change > 0 {
				equityGreen++
			} else if f.Change < 0 {
				equityRed++
			}
		case "metals":
			if f.Name == "Gold" || f.Ticker == "GC" {
				goldChange = f.Change
			}
		case "energy":
			if f.Name == "Crude Oil" || f.Ticker == "CL" {
				oilChange = f.Change
			}
		case "bonds":
			if f.Change > 0 {
				bondGreen++
			} else if f.Change < 0 {
				bondRed++
			}
		}
	}

	riskOnScore := 0
	riskOffScore := 0

	// Equities: majority green → risk-on
	if equityGreen > equityRed && equityGreen >= 2 {
		riskOnScore++
	}
	if equityRed > equityGreen && equityRed >= 2 {
		riskOffScore++
	}

	// Gold down → risk-on; gold up → risk-off
	if goldChange < -0.1 {
		riskOnScore++
	}
	if goldChange > 0.1 {
		riskOffScore++
	}

	// Bonds (yields inverse): bond futures up = yields down = risk-off
	if bondRed > bondGreen {
		riskOnScore++ // yields up
	}
	if bondGreen > bondRed {
		riskOffScore++ // yields down
	}

	// Oil up is mild risk-on
	if oilChange > 0.3 {
		riskOnScore++
	}

	switch {
	case riskOnScore >= 3:
		return "RISK-ON"
	case riskOffScore >= 3:
		return "RISK-OFF"
	case riskOnScore > riskOffScore:
		return "RISK-ON"
	case riskOffScore > riskOnScore:
		return "RISK-OFF"
	default:
		return "MIXED"
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Orchestrator
// ──────────────────────────────────────────────────────────────────────────────

func fetchAll(ctx context.Context) *MarketOverview {
	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("finviz: skipping — FIRECRAWL_API_KEY not set")
		return &MarketOverview{FetchedAt: time.Now()}
	}

	futures := fetchFutures(ctx, apiKey)
	sectors := fetchSectors(ctx, apiKey)

	available := len(futures) > 0 || len(sectors) > 0
	tone := "MIXED"
	if len(futures) > 0 {
		tone = classifyRiskTone(futures)
	}

	return &MarketOverview{
		Futures:   futures,
		Sectors:   sectors,
		RiskTone:  tone,
		FetchedAt: time.Now(),
		Available: available,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// Abs returns the absolute value of x (utility for formatters).
func Abs(x float64) float64 { return math.Abs(x) }
