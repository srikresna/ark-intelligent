// Package imf provides integration with the IMF DataMapper API for
// World Economic Outlook (WEO) forecasts. Data is free, requires no
// API key, and provides forward-looking GDP growth, inflation, and
// current-account-balance forecasts — more relevant to FX trading
// than historical World Bank data.
//
// API: https://www.imf.org/external/datamapper/api/v1/{INDICATOR}/{COUNTRY_LIST}
// Updated approximately 2× per year (April and October WEO releases).
package imf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("imf") //nolint:gochecknoglobals

// IMF DataMapper indicator codes relevant to FX analysis.
const (
	indicatorGDPGrowth      = "NGDP_RPCH"    // Real GDP growth (%)
	indicatorCPIInflation   = "PCPIPCH"       // CPI Inflation (%)
	indicatorCurrentAccount = "BCA_NGDPDP"    // Current Account Balance (% of GDP)
)

const baseURL = "https://www.imf.org/external/datamapper/api/v1"

// countryConfig maps IMF DataMapper country codes to display currency names.
var countryConfig = []struct { //nolint:gochecknoglobals
	Code     string // IMF DataMapper country code
	Currency string
}{
	{"USA", "USD"},
	{"GBR", "GBP"},
	{"JPN", "JPY"},
	{"DEU", "EUR"}, // Germany as EUR proxy
	{"AUS", "AUD"},
	{"CAN", "CAD"},
	{"NZL", "NZD"},
	{"CHE", "CHF"},
}

// IMFCountryData holds WEO forecast data for a single country.
type IMFCountryData struct {
	Country        string  // IMF code ("USA", "GBR", …)
	Currency       string  // Display currency ("USD", "GBP", …)
	GDPGrowth      float64 // IMF forecast GDP growth % for forecast year
	GDPGrowthNext  float64 // GDP growth % for forecast year + 1
	Inflation      float64 // IMF forecast CPI % for forecast year
	CurrentAccount float64 // Current Account Balance (% of GDP)
	ForecastYear   int     // Primary forecast year
	Available      bool
}

// IMFWEOData is the top-level result returned by FetchIMFWEO.
type IMFWEOData struct {
	Countries []IMFCountryData
	FetchedAt time.Time
	Available bool
}

// package-level cache, protected by cacheMu.
var (
	globalCache *IMFWEOData    //nolint:gochecknoglobals
	cacheMu     sync.RWMutex   //nolint:gochecknoglobals
	cacheTTL    = 24 * time.Hour //nolint:gochecknoglobals
	httpClient  = httpclient.New(httpclient.WithTimeout(20 * time.Second)) //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached data if within TTL, otherwise fetches fresh
// data from the IMF API. Gracefully degrades: if fetch fails, returns the
// stale cache (if any) rather than an error.
func GetCachedOrFetch(ctx context.Context) (*IMFWEOData, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.FetchedAt) < cacheTTL {
		data := globalCache
		cacheMu.RUnlock()
		return data, nil
	}
	cacheMu.RUnlock()

	data, err := FetchIMFWEO(ctx)
	if err != nil {
		cacheMu.RLock()
		stale := globalCache
		cacheMu.RUnlock()
		if stale != nil {
			log.Warn().Err(err).Msg("IMF WEO fetch failed; using stale cache")
			return stale, nil
		}
		return nil, err
	}

	cacheMu.Lock()
	globalCache = data
	cacheMu.Unlock()

	return data, nil
}

// FetchIMFWEO fetches all three WEO indicators in parallel and merges
// them into a single IMFWEOData result.
func FetchIMFWEO(ctx context.Context) (*IMFWEOData, error) {
	countryCodes := make([]string, len(countryConfig))
	for i, cc := range countryConfig {
		countryCodes[i] = cc.Code
	}
	countryList := strings.Join(countryCodes, "+")

	type indicatorResult struct {
		indicator string
		data      map[string]map[string]float64 // country → year → value
		err       error
	}

	indicators := []string{indicatorGDPGrowth, indicatorCPIInflation, indicatorCurrentAccount}
	ch := make(chan indicatorResult, len(indicators))

	var wg sync.WaitGroup
	for _, ind := range indicators {
		wg.Add(1)
		go func(indicator string) {
			defer wg.Done()
			data, err := fetchIndicator(ctx, indicator, countryList)
			ch <- indicatorResult{indicator: indicator, data: data, err: err}
		}(ind)
	}

	wg.Wait()
	close(ch)

	// Collect results per indicator.
	indicatorData := make(map[string]map[string]map[string]float64) // indicator → country → year → value
	for r := range ch {
		if r.err != nil {
			log.Warn().Str("indicator", r.indicator).Err(r.err).Msg("IMF indicator fetch failed")
			continue
		}
		indicatorData[r.indicator] = r.data
	}

	// Determine forecast year: current year or next year if we are past the
	// primary WEO release cycle. Use the most common year with GDP data.
	forecastYear := determineForecastYear(indicatorData[indicatorGDPGrowth])

	// Build per-country results.
	countries := make([]IMFCountryData, len(countryConfig))
	for i, cc := range countryConfig {
		cd := IMFCountryData{
			Country:      cc.Code,
			Currency:     cc.Currency,
			ForecastYear: forecastYear,
		}
		yearStr := strconv.Itoa(forecastYear)
		nextYearStr := strconv.Itoa(forecastYear + 1)

		if gdpMap, ok := indicatorData[indicatorGDPGrowth]; ok {
			if countryMap, ok := gdpMap[cc.Code]; ok {
				if v, ok := countryMap[yearStr]; ok {
					cd.GDPGrowth = v
					cd.Available = true
				}
				if v, ok := countryMap[nextYearStr]; ok {
					cd.GDPGrowthNext = v
				}
			}
		}

		if cpiMap, ok := indicatorData[indicatorCPIInflation]; ok {
			if countryMap, ok := cpiMap[cc.Code]; ok {
				if v, ok := countryMap[yearStr]; ok {
					cd.Inflation = v
				}
			}
		}

		if caMap, ok := indicatorData[indicatorCurrentAccount]; ok {
			if countryMap, ok := caMap[cc.Code]; ok {
				if v, ok := countryMap[yearStr]; ok {
					cd.CurrentAccount = v
				}
			}
		}

		countries[i] = cd
	}

	anyAvailable := false
	for _, c := range countries {
		if c.Available {
			anyAvailable = true
			break
		}
	}

	return &IMFWEOData{
		Countries: countries,
		FetchedAt: time.Now(),
		Available: anyAvailable,
	}, nil
}

// determineForecastYear picks the best forecast year from available data.
// It prefers the current calendar year; if no data exists for the current
// year, it falls back to the next year with data.
func determineForecastYear(gdpData map[string]map[string]float64) int {
	currentYear := time.Now().Year()

	// Check if current year has data for at least one country.
	yearStr := strconv.Itoa(currentYear)
	for _, countryMap := range gdpData {
		if _, ok := countryMap[yearStr]; ok {
			return currentYear
		}
	}

	// Collect all available years across countries.
	yearSet := make(map[int]bool)
	for _, countryMap := range gdpData {
		for yStr := range countryMap {
			if y, err := strconv.Atoi(yStr); err == nil && y >= currentYear {
				yearSet[y] = true
			}
		}
	}

	years := make([]int, 0, len(yearSet))
	for y := range yearSet {
		years = append(years, y)
	}
	sort.Ints(years)

	if len(years) > 0 {
		return years[0]
	}

	return currentYear
}

// fetchIndicator fetches a single IMF DataMapper indicator for the given
// country list. Returns: country → year → value.
func fetchIndicator(ctx context.Context, indicator, countryList string) (map[string]map[string]float64, error) {
	url := fmt.Sprintf("%s/%s/%s", baseURL, indicator, countryList)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "ark-intelligent/1.0 (market-analysis-bot)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", indicator, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IMF API status %d for %s", resp.StatusCode, indicator)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// IMF DataMapper response format:
	// { "values": { "INDICATOR": { "COUNTRY": { "YEAR": value, ... }, ... } } }
	var raw struct {
		Values map[string]map[string]map[string]float64 `json:"values"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal response for %s: %w", indicator, err)
	}

	// Extract the indicator data (the key is the indicator name itself).
	if data, ok := raw.Values[indicator]; ok {
		return data, nil
	}

	// Sometimes the key is lower-cased or absent; iterate.
	for _, data := range raw.Values {
		return data, nil // take the first (and usually only) key
	}

	return nil, fmt.Errorf("no values key found for indicator %s", indicator)
}

// BuildPromptSection returns a compact IMF WEO section string for AI prompts.
func BuildPromptSection(data *IMFWEOData) string {
	if data == nil || !data.Available {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("IMF WEO Forecasts:\n")

	parts := make([]string, 0, len(data.Countries))
	for _, c := range data.Countries {
		if !c.Available {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: GDP=%.1f%% CPI=%.1f%% CA=%+.1f%%GDP",
			c.Currency, c.GDPGrowth, c.Inflation, c.CurrentAccount))
	}
	sb.WriteString(strings.Join(parts, " | "))
	sb.WriteString("\n")

	// Insight: highest vs lowest GDP growth divergence
	var high, low IMFCountryData
	first := true
	for _, c := range data.Countries {
		if !c.Available {
			continue
		}
		if first {
			high, low = c, c
			first = false
			continue
		}
		if c.GDPGrowth > high.GDPGrowth {
			high = c
		}
		if c.GDPGrowth < low.GDPGrowth {
			low = c
		}
	}
	if !first && high.Currency != low.Currency {
		sb.WriteString(fmt.Sprintf("→ %s strongest growth (%.1f%%), %s weakest (%.1f%%)\n",
			high.Currency, high.GDPGrowth, low.Currency, low.GDPGrowth))
	}

	return sb.String()
}
