// Package worldbank provides integration with the World Bank Open Data API.
// It fetches cross-country macro fundamentals (GDP growth, current account,
// CPI inflation) for major currency countries.
//
// API: https://api.worldbank.org/v2/country/{code}/indicator/{series}?format=json&mrv=3
// No API key required. Data updates annually; cache TTL is 24 hours.
package worldbank

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("worldbank") //nolint:gochecknoglobals

// World Bank indicator series IDs.
const (
	seriesGDPGrowth      = "NY.GDP.MKTP.KD.ZG" // GDP Growth Rate (%)
	seriesCurrentAccount = "BN.CAB.XOKA.CD"     // Current Account Balance (USD)
	seriesCPIInflation   = "FP.CPI.TOTL.ZG"     // CPI Inflation (%)
)

// countryConfig maps ISO-2 country codes to their display currency name.
// XC is the World Bank code for the Eurozone.
var countryConfig = []struct { //nolint:gochecknoglobals
	Code     string
	Currency string
}{
	{"US", "USD"},
	{"GB", "GBP"},
	{"JP", "JPY"},
	{"XC", "EUR"},
	{"AU", "AUD"},
	{"CA", "CAD"},
	{"NZ", "NZD"},
	{"CH", "CHF"},
}

// CountryMacro holds the latest annual macro data for a single country.
type CountryMacro struct {
	CountryCode    string
	Currency       string
	GDPGrowth      float64 // % YoY
	CurrentAccount float64 // USD billions
	CPIInflation   float64 // % YoY
	Year           int
	Available      bool // false if fetch failed or no data
}

// WorldBankData is the top-level result returned by FetchWorldBankData.
type WorldBankData struct {
	Countries []CountryMacro
	FetchedAt time.Time
}

// cache fields (package-level, protected by cacheMu).
var (
	globalCache  *WorldBankData //nolint:gochecknoglobals
	cacheMu      sync.RWMutex
	cacheTTL     = 24 * time.Hour //nolint:gochecknoglobals
	httpClient   = &http.Client{Timeout: 15 * time.Second} //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached data if within TTL, otherwise fetches fresh
// data from the World Bank API. Gracefully degrades: if fetch fails, returns
// the stale cache (if any) rather than an error.
func GetCachedOrFetch(ctx context.Context) (*WorldBankData, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.FetchedAt) < cacheTTL {
		data := globalCache
		cacheMu.RUnlock()
		return data, nil
	}
	cacheMu.RUnlock()

	data, err := FetchWorldBankData(ctx)
	if err != nil {
		// Graceful degradation: return stale cache if available
		cacheMu.RLock()
		stale := globalCache
		cacheMu.RUnlock()
		if stale != nil {
			log.Warn().Err(err).Msg("World Bank fetch failed; using stale cache")
			return stale, nil
		}
		return nil, err
	}

	cacheMu.Lock()
	globalCache = data
	cacheMu.Unlock()

	return data, nil
}

// FetchWorldBankData fetches macro data for all configured countries in
// parallel. Countries that fail to fetch are included with Available=false.
func FetchWorldBankData(ctx context.Context) (*WorldBankData, error) {
	type result struct {
		macro CountryMacro
		idx   int
	}

	results := make([]CountryMacro, len(countryConfig))
	ch := make(chan result, len(countryConfig))

	var wg sync.WaitGroup
	for i, cc := range countryConfig {
		wg.Add(1)
		go func(idx int, code, currency string) {
			defer wg.Done()
			macro := fetchCountry(ctx, code, currency)
			ch <- result{macro: macro, idx: idx}
		}(i, cc.Code, cc.Currency)
	}

	wg.Wait()
	close(ch)

	for r := range ch {
		results[r.idx] = r.macro
	}

	return &WorldBankData{
		Countries: results,
		FetchedAt: time.Now(),
	}, nil
}

// fetchCountry fetches all three indicators for a single country.
func fetchCountry(ctx context.Context, code, currency string) CountryMacro {
	macro := CountryMacro{
		CountryCode: code,
		Currency:    currency,
	}

	gdp, year, err := fetchIndicator(ctx, code, seriesGDPGrowth)
	if err != nil {
		log.Warn().Str("country", code).Str("series", seriesGDPGrowth).Err(err).Msg("World Bank fetch failed")
		return macro // Available stays false
	}

	ca, _, err := fetchIndicator(ctx, code, seriesCurrentAccount)
	if err != nil {
		log.Warn().Str("country", code).Str("series", seriesCurrentAccount).Err(err).Msg("World Bank fetch failed")
		return macro
	}

	cpi, _, err := fetchIndicator(ctx, code, seriesCPIInflation)
	if err != nil {
		log.Warn().Str("country", code).Str("series", seriesCPIInflation).Err(err).Msg("World Bank fetch failed")
		return macro
	}

	macro.GDPGrowth = gdp
	macro.CurrentAccount = ca / 1e9 // Convert USD to billions
	macro.CPIInflation = cpi
	macro.Year = year
	macro.Available = true
	return macro
}

// worldBankResponse is the raw JSON structure returned by the World Bank API.
// The response is a 2-element JSON array: [metadata, []dataPoints].
type worldBankResponse [2]json.RawMessage

type wbDataPoint struct {
	Date     string   `json:"date"`
	Value    *float64 `json:"value"` // pointer because value can be null
	Country  wbEntity `json:"country"`
}

type wbEntity struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// fetchIndicator calls the World Bank API for a single country + series.
// It returns the most recent non-null value and its year.
func fetchIndicator(ctx context.Context, countryCode, series string) (float64, int, error) {
	url := fmt.Sprintf(
		"https://api.worldbank.org/v2/country/%s/indicator/%s?format=json&mrv=5",
		countryCode, series,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "ark-intelligent/1.0 (market-analysis-bot)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("http get %s/%s: %w", countryCode, series, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("World Bank API status %d for %s/%s", resp.StatusCode, countryCode, series)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("read body: %w", err)
	}

	var raw worldBankResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, 0, fmt.Errorf("unmarshal response: %w", err)
	}

	var points []wbDataPoint
	if err := json.Unmarshal(raw[1], &points); err != nil {
		return 0, 0, fmt.Errorf("unmarshal data points: %w", err)
	}

	// Find most recent non-null value
	for _, p := range points {
		if p.Value != nil {
			year := 0
			fmt.Sscanf(p.Date, "%d", &year) //nolint:errcheck // parse best-effort
			return *p.Value, year, nil
		}
	}

	return 0, 0, fmt.Errorf("no non-null data for %s/%s", countryCode, series)
}
