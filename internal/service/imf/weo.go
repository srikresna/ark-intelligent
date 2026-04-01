// Package imf provides integration with the IMF World Economic Outlook (WEO)
// DataMapper API. Data is forward-looking GDP growth, inflation, and current
// account forecasts — updated twice a year (April and October WEO releases).
//
// API: https://www.imf.org/external/datamapper/api/v1/{INDICATOR}/{COUNTRY_LIST}
// No API key required. In-memory cache TTL: 24 hours.
package imf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("imf") //nolint:gochecknoglobals

const (
	datamapperBase = "https://www.imf.org/external/datamapper/api/v1"
	httpTimeout    = 20 * time.Second
	cacheTTL       = 24 * time.Hour

	// IMF indicator codes for forex-relevant macro data.
	indicatorGDPGrowth     = "NGDP_RPCH"    // Real GDP Growth Rate (%)
	indicatorInflation     = "PCPIPCH"       // CPI Inflation (%)
	indicatorCurrentAcct   = "BCA_NGDPDP"   // Current Account Balance (% of GDP)
)

// countryConfig maps IMF country codes to their major currency.
var countryConfig = []struct { //nolint:gochecknoglobals
	Code     string
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
	Country        string  // IMF country code, e.g. "USA"
	Currency       string  // Major currency, e.g. "USD"
	GDPGrowth2026  float64 // IMF forecast GDP growth % for 2026
	GDPGrowth2027  float64 // IMF forecast GDP growth % for 2027
	Inflation2026  float64 // IMF forecast CPI inflation % for 2026
	CurrentAccount float64 // Current account balance % of GDP (latest forecast)
	Available      bool    // false if API returned no usable data for this country
}

// IMFWEOData is the top-level result returned by FetchIMFWEO.
type IMFWEOData struct {
	Countries []IMFCountryData
	FetchedAt time.Time
	Available bool // false if all countries failed
}

// indicatorData is the nested map parsed from the IMF DataMapper response.
// Structure: map[countryCode]map[year]float64
type indicatorData map[string]map[string]float64

// datamapperResponse is the raw JSON envelope from the DataMapper API.
type datamapperResponse struct {
	Values map[string]indicatorData `json:"values"`
}

// package-level cache.
var (
	globalCache *IMFWEOData //nolint:gochecknoglobals
	cacheMu     sync.RWMutex
	client      = &http.Client{Timeout: httpTimeout} //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached IMF WEO data if within TTL; otherwise
// fetches fresh data. Gracefully degrades on failure using stale cache.
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

// FetchIMFWEO fetches GDP growth, inflation, and current account forecasts
// for all configured countries in parallel using the IMF DataMapper API.
func FetchIMFWEO(ctx context.Context) (*IMFWEOData, error) {
	indicators := []string{indicatorGDPGrowth, indicatorInflation, indicatorCurrentAcct}

	type indicatorResult struct {
		code string
		data indicatorData
		err  error
	}

	ch := make(chan indicatorResult, len(indicators))
	var wg sync.WaitGroup

	for _, ind := range indicators {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			data, err := fetchIndicator(ctx, code)
			ch <- indicatorResult{code: code, data: data, err: err}
		}(ind)
	}

	wg.Wait()
	close(ch)

	fetched := make(map[string]indicatorData, 3)
	for r := range ch {
		if r.err != nil {
			log.Warn().Err(r.err).Str("indicator", r.code).Msg("IMF DataMapper fetch failed")
			continue
		}
		fetched[r.code] = r.data
	}

	// Build per-country results.
	countries := make([]IMFCountryData, len(countryConfig))
	currentYear := time.Now().Year()
	y26 := fmt.Sprintf("%d", currentYear+1)
	y27 := fmt.Sprintf("%d", currentYear+2)
	// Handle when it's already 2026+ (adjust relative to current year)
	if currentYear >= 2026 {
		y26 = fmt.Sprintf("%d", currentYear)
		y27 = fmt.Sprintf("%d", currentYear+1)
	}

	for i, cc := range countryConfig {
		cd := IMFCountryData{
			Country:  cc.Code,
			Currency: cc.Currency,
		}

		if gdpData, ok := fetched[indicatorGDPGrowth]; ok {
			if countryVals, ok := gdpData[cc.Code]; ok {
				if v, ok := countryVals[y26]; ok {
					cd.GDPGrowth2026 = v
					cd.Available = true
				}
				if v, ok := countryVals[y27]; ok {
					cd.GDPGrowth2027 = v
				}
			}
		}

		if cpiData, ok := fetched[indicatorInflation]; ok {
			if countryVals, ok := cpiData[cc.Code]; ok {
				if v, ok := countryVals[y26]; ok {
					cd.Inflation2026 = v
				}
			}
		}

		if caData, ok := fetched[indicatorCurrentAcct]; ok {
			if countryVals, ok := caData[cc.Code]; ok {
				// Use nearest forecast year available
				for _, yr := range []string{y26, y27} {
					if v, ok := countryVals[yr]; ok {
						cd.CurrentAccount = v
						break
					}
				}
			}
		}

		countries[i] = cd
	}

	available := false
	for _, c := range countries {
		if c.Available {
			available = true
			break
		}
	}

	result := &IMFWEOData{
		Countries: countries,
		FetchedAt: time.Now(),
		Available: available,
	}

	if available {
		log.Info().
			Int("countries", len(countries)).
			Msg("IMF WEO data fetched successfully")
	} else {
		log.Warn().Msg("IMF WEO fetch returned no usable country data")
	}

	return result, nil
}

// fetchIndicator fetches data for one IMF indicator across all configured countries.
func fetchIndicator(ctx context.Context, indicatorCode string) (indicatorData, error) {
	codes := make([]string, len(countryConfig))
	for i, cc := range countryConfig {
		codes[i] = cc.Code
	}
	countryList := strings.Join(codes, "/")

	url := fmt.Sprintf("%s/%s/%s", datamapperBase, indicatorCode, countryList)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", indicatorCode, err)
	}
	req.Header.Set("User-Agent", "ark-intelligent/1.0 (market-analysis-bot)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", indicatorCode, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IMF DataMapper %s status %d", indicatorCode, resp.StatusCode)
	}

	var raw datamapperResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", indicatorCode, err)
	}

	// The values field is: map[indicatorCode]map[countryCode]map[year]float64
	if data, ok := raw.Values[indicatorCode]; ok {
		return data, nil
	}

	return nil, fmt.Errorf("indicator %s not found in response", indicatorCode)
}

// BuildPromptSection returns an IMF WEO section string for inclusion in AI prompts.
// Returns empty string if data is unavailable.
func BuildPromptSection(data *IMFWEOData) string {
	if data == nil || !data.Available {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("IMF WEO Forecasts (2026):\n")

	// Build compact line: "USD: GDP=2.1% CPI=2.4% CA=-3.2%GDP"
	parts := make([]string, 0, len(data.Countries))
	for _, c := range data.Countries {
		if !c.Available {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: GDP=%.1f%% CPI=%.1f%% CA=%+.1f%%GDP",
			c.Currency, c.GDPGrowth2026, c.Inflation2026, c.CurrentAccount))
	}
	sb.WriteString(strings.Join(parts, " | "))
	sb.WriteString("\n")

	// Add insight: highest/lowest growth and best CA surplus
	if insight := buildInsight(data.Countries); insight != "" {
		sb.WriteString(insight)
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildInsight generates a summary insight from country data.
func buildInsight(countries []IMFCountryData) string {
	available := make([]IMFCountryData, 0, len(countries))
	for _, c := range countries {
		if c.Available {
			available = append(available, c)
		}
	}
	if len(available) == 0 {
		return ""
	}

	sort.Slice(available, func(i, j int) bool {
		return available[i].GDPGrowth2026 > available[j].GDPGrowth2026
	})

	highest := available[0]
	lowest := available[len(available)-1]

	// Best current account surplus
	bestCA := available[0]
	for _, c := range available[1:] {
		if c.CurrentAccount > bestCA.CurrentAccount {
			bestCA = c
		}
	}

	return fmt.Sprintf("→ Highest growth: %s (%.1f%%) | Lowest growth: %s (%.1f%%) | Best CA surplus: %s (%+.1f%%GDP)",
		highest.Currency, highest.GDPGrowth2026,
		lowest.Currency, lowest.GDPGrowth2026,
		bestCA.Currency, bestCA.CurrentAccount)
}
