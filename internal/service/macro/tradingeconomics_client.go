// Package macro — TradingEconomics scraper via Firecrawl.
//
// Scrapes key macro indicators for G10 countries:
//   GDP growth rate, CPI, unemployment rate, manufacturing PMI,
//   consumer confidence.
//
// Data is cached in BadgerDB with a 6h TTL (macro data changes slowly).
// Rate limit: max 1 scrape per country per 6h (enforced by cache TTL).
// If FIRECRAWL_API_KEY is not set, all methods return Available=false.
package macro

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var teLog = logger.Component("tradingeconomics")

const (
	teCacheTTL     = 6 * time.Hour
	teFetchTimeout = 45 * time.Second
	teFirecrawlURL = "https://api.firecrawl.dev/v1/scrape"
	teBadgerKeyPfx = "tradingeconomics:v1:"
)

// teCountries maps TradingEconomics URL slug to display metadata.
var teCountries = []struct {
	Slug     string
	Name     string
	Flag     string
	Currency string
}{
	{"united-states", "US", "🇺🇸", "USD"},
	{"euro-area", "Euro Area", "🇪🇺", "EUR"},
	{"united-kingdom", "UK", "🇬🇧", "GBP"},
	{"japan", "Japan", "🇯🇵", "JPY"},
	{"canada", "Canada", "🇨🇦", "CAD"},
	{"australia", "Australia", "🇦🇺", "AUD"},
	{"switzerland", "Switzerland", "🇨🇭", "CHF"},
	{"new-zealand", "New Zealand", "🇳🇿", "NZD"},
}

// teIndicators lists the indicators to scrape per country.
var teIndicators = []struct {
	Slug string
	Name string
}{
	{"gdp-growth-rate", "GDP Growth"},
	{"inflation-cpi", "CPI"},
	{"unemployment-rate", "Unemployment"},
	{"manufacturing-pmi", "Mfg PMI"},
	{"consumer-confidence", "Consumer Conf"},
}

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// TEIndicatorValue holds a single macro indicator reading.
type TEIndicatorValue struct {
	Current  float64 `json:"current"`
	Previous float64 `json:"previous"`
	YoY      float64 `json:"yoy"`
	Unit     string  `json:"unit"` // e.g. "%", "points"
	Date     string  `json:"date"` // most recent data date
}

// IsZero returns true if no data was extracted.
func (v TEIndicatorValue) IsZero() bool {
	return v.Current == 0 && v.Previous == 0
}

// TECountryData holds all indicators for a single country.
type TECountryData struct {
	Country    string                      `json:"country"`
	Currency   string                      `json:"currency"`
	Flag       string                      `json:"flag"`
	Indicators map[string]TEIndicatorValue `json:"indicators"` // key = indicator slug
	FetchedAt  time.Time                   `json:"fetched_at"`
	Available  bool                        `json:"available"`
}

// TEGlobalMacroData holds all G10 country data.
type TEGlobalMacroData struct {
	Countries map[string]*TECountryData `json:"countries"` // key = country slug
	FetchedAt time.Time                 `json:"fetched_at"`
	Available bool                      `json:"available"`
}

// IsZero returns true if no data has been fetched.
func (d *TEGlobalMacroData) IsZero() bool {
	return d == nil || d.FetchedAt.IsZero() || len(d.Countries) == 0
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

var (
	teCacheMu     sync.RWMutex
	teCachedData  *TEGlobalMacroData
	teCacheExpiry time.Time
	teBadgerDB    *badger.DB
)

// InitTradingEconomicsDB injects a BadgerDB instance for cache persistence.
// If db is nil, the cache operates in in-memory mode only.
func InitTradingEconomicsDB(db *badger.DB) {
	teCacheMu.Lock()
	teBadgerDB = db
	teCacheMu.Unlock()
	teLog.Debug().Bool("persistence", db != nil).Msg("tradingeconomics cache initialized")
}

// GetTECachedOrFetch returns cached global macro data, fetching if stale.
func GetTECachedOrFetch(ctx context.Context) (*TEGlobalMacroData, error) {
	// Fast path: in-memory.
	teCacheMu.RLock()
	if teCachedData != nil && time.Now().Before(teCacheExpiry) {
		d := teCachedData
		teCacheMu.RUnlock()
		return d, nil
	}
	db := teBadgerDB
	teCacheMu.RUnlock()

	// Try BadgerDB.
	if db != nil {
		if d := teLoadFromBadger(db); d != nil && time.Since(d.FetchedAt) < teCacheTTL {
			teCacheMu.Lock()
			teCachedData = d
			teCacheExpiry = d.FetchedAt.Add(teCacheTTL)
			teCacheMu.Unlock()
			return d, nil
		}
	}

	// Live fetch.
	data, err := FetchTEGlobalMacro(ctx)
	if err != nil {
		return nil, err
	}

	teCacheMu.Lock()
	teCachedData = data
	teCacheExpiry = time.Now().Add(teCacheTTL)
	teCacheMu.Unlock()

	if db != nil {
		teSaveToBadger(db, data)
	}

	return data, nil
}

// InvalidateTECache clears the in-memory TradingEconomics cache.
func InvalidateTECache() {
	teCacheMu.Lock()
	teCachedData = nil
	teCacheExpiry = time.Time{}
	teCacheMu.Unlock()
}

// ---------------------------------------------------------------------------
// Firecrawl scraping
// ---------------------------------------------------------------------------

type teFirecrawlReq struct {
	URL         string      `json:"url"`
	Formats     []string    `json:"formats"`
	WaitFor     int         `json:"waitFor"`
	JSONOptions *teJSONOpts `json:"jsonOptions,omitempty"`
}

type teJSONOpts struct {
	Prompt string          `json:"prompt"`
	Schema json.RawMessage `json:"schema"`
}

// teIndicatorSchema is the Firecrawl JSON extraction schema.
var teIndicatorSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"current_value":  {"type": "number", "description": "The latest/current value of the indicator"},
		"previous_value": {"type": "number", "description": "The previous period value"},
		"yoy_change":     {"type": "number", "description": "Year-over-year change in percentage points or percent"},
		"unit":           {"type": "string",  "description": "Unit of measurement, e.g. % or points"},
		"date":           {"type": "string",  "description": "Date of the latest data point"}
	}
}`)

// teScrapedIndicator is the JSON result from Firecrawl.
type teScrapedIndicator struct {
	CurrentValue  float64 `json:"current_value"`
	PreviousValue float64 `json:"previous_value"`
	YoYChange     float64 `json:"yoy_change"`
	Unit          string  `json:"unit"`
	Date          string  `json:"date"`
}

// scrapeIndicator scrapes a single country+indicator page via Firecrawl.
func scrapeIndicator(ctx context.Context, apiKey, country, indicator string) (*teScrapedIndicator, error) {
	url := fmt.Sprintf("https://tradingeconomics.com/%s/%s", country, indicator)
	prompt := fmt.Sprintf(
		"Extract the latest %s data: current value, previous period value, "+
			"year-over-year change, unit (percent or points), and the date of the latest data.",
		strings.ReplaceAll(indicator, "-", " "),
	)

	reqBody := teFirecrawlReq{
		URL:     url,
		Formats: []string{"json"},
		WaitFor: 3000,
		JSONOptions: &teJSONOpts{
			Prompt: prompt,
			Schema: teIndicatorSchema,
		},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("tradingeconomics: marshal request: %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, teFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, teFirecrawlURL, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("tradingeconomics: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tradingeconomics: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tradingeconomics: firecrawl status %d", resp.StatusCode)
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON json.RawMessage `json:"json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		return nil, fmt.Errorf("tradingeconomics: decode response: %w", err)
	}
	if !fcResp.Success {
		return nil, fmt.Errorf("tradingeconomics: firecrawl returned success=false")
	}

	var scraped teScrapedIndicator
	if err := json.Unmarshal(fcResp.Data.JSON, &scraped); err != nil {
		return nil, fmt.Errorf("tradingeconomics: decode json: %w", err)
	}

	return &scraped, nil
}

// fetchCountryData scrapes all indicators for a single country concurrently.
func fetchCountryData(ctx context.Context, apiKey, countrySlug, countryName, flag, currency string) *TECountryData {
	cd := &TECountryData{
		Country:    countryName,
		Currency:   currency,
		Flag:       flag,
		Indicators: make(map[string]TEIndicatorValue, len(teIndicators)),
		FetchedAt:  time.Now(),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ind := range teIndicators {
		ind := ind
		wg.Add(1)
		go func() {
			defer wg.Done()
			scraped, err := scrapeIndicator(ctx, apiKey, countrySlug, ind.Slug)
			if err != nil {
				teLog.Debug().Err(err).
					Str("country", countrySlug).
					Str("indicator", ind.Slug).
					Msg("tradingeconomics: scrape failed")
				return
			}
			val := TEIndicatorValue{
				Current:  scraped.CurrentValue,
				Previous: scraped.PreviousValue,
				YoY:      scraped.YoYChange,
				Unit:     scraped.Unit,
				Date:     scraped.Date,
			}
			mu.Lock()
			cd.Indicators[ind.Slug] = val
			mu.Unlock()
		}()
	}
	wg.Wait()

	cd.Available = len(cd.Indicators) > 0
	return cd
}

// FetchTEGlobalMacro fetches all G10 macro indicators from TradingEconomics.
// Countries are fetched concurrently. If FIRECRAWL_API_KEY is not set,
// returns an empty Available=false result with no error.
func FetchTEGlobalMacro(ctx context.Context) (*TEGlobalMacroData, error) {
	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		teLog.Debug().Msg("tradingeconomics: skipping — FIRECRAWL_API_KEY not set")
		return &TEGlobalMacroData{FetchedAt: time.Now()}, nil
	}

	result := &TEGlobalMacroData{
		Countries: make(map[string]*TECountryData, len(teCountries)),
		FetchedAt: time.Now(),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, c := range teCountries {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			cd := fetchCountryData(ctx, apiKey, c.Slug, c.Name, c.Flag, c.Currency)
			mu.Lock()
			result.Countries[c.Slug] = cd
			mu.Unlock()
		}()
	}
	wg.Wait()

	result.Available = len(result.Countries) > 0
	return result, nil
}

// ---------------------------------------------------------------------------
// BadgerDB helpers
// ---------------------------------------------------------------------------

func teLoadFromBadger(db *badger.DB) *TEGlobalMacroData {
	var data TEGlobalMacroData
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(teBadgerKeyPfx + "global"))
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

func teSaveToBadger(db *badger.DB, data *TEGlobalMacroData) {
	b, err := json.Marshal(data)
	if err != nil {
		teLog.Debug().Err(err).Msg("tradingeconomics: failed to marshal for BadgerDB")
		return
	}
	_ = db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(teBadgerKeyPfx+"global"), b).WithTTL(teCacheTTL)
		return txn.SetEntry(e)
	})
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

// FormatTEGlobalMacro formats the global macro dashboard for Telegram HTML.
func FormatTEGlobalMacro(data *TEGlobalMacroData) string {
	if data == nil || !data.Available {
		return "<i>Global macro data unavailable (Firecrawl not configured)</i>\n"
	}

	var b strings.Builder
	b.WriteString("🌍 <b>GLOBAL MACRO DASHBOARD</b> <i>(TradingEconomics)</i>\n")
	b.WriteString(fmt.Sprintf("<i>Updated %s</i>\n\n", data.FetchedAt.Format("2006-01-02 15:04")))

	b.WriteString("<code>Ccy   GDP%  CPI%  Unemp  PMI   CSI</code>\n")
	b.WriteString("<code>──────────────────────────────────────</code>\n")

	for _, c := range teCountries {
		cd, ok := data.Countries[c.Slug]
		if !ok || !cd.Available {
			continue
		}
		gdp := teIndStr(cd, "gdp-growth-rate")
		cpi := teIndStr(cd, "inflation-cpi")
		unemp := teIndStr(cd, "unemployment-rate")
		pmi := teIndStr(cd, "manufacturing-pmi")
		csi := teIndStr(cd, "consumer-confidence")
		b.WriteString(fmt.Sprintf("<code>%s %-4s %5s %5s %6s %5s %5s</code>\n",
			c.Flag, c.Currency, gdp, cpi, unemp, pmi, csi))
	}

	b.WriteString("\n<i>GDP: QoQ% | CPI: YoY% | Unemp/PMI/CSI: latest value</i>\n")
	return b.String()
}

// teIndStr returns a formatted indicator string or " N/A".
func teIndStr(cd *TECountryData, slug string) string {
	v, ok := cd.Indicators[slug]
	if !ok || v.IsZero() {
		return "  N/A"
	}
	return fmt.Sprintf("%5.1f", v.Current)
}
