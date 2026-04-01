package fred

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// wbLog uses the existing fred package logger.

// WorldBankData holds global macro fundamentals fetched from the World Bank API.
// Data is annual — updated once per year by the World Bank.
type WorldBankData struct {
	Countries map[string]*CountryMacro // key: ISO 3-letter country code
	Available bool
	FetchedAt time.Time
}

// CountryMacro holds annual macro fundamentals for one country/economy.
type CountryMacro struct {
	Country        string  // "Australia"
	Currency       string  // "AUD"
	GDPGrowthYoY   float64 // Real GDP growth (%)
	CurrentAccount float64 // CA balance (% of GDP)
	InflationCPI   float64 // CPI YoY (%)
	FXReserves     float64 // Total reserves excluding gold (USD billions)
	Year           int     // Data vintage year
}

// wbIndicators maps World Bank indicator codes to internal field names.
var wbIndicators = map[string]string{
	"NY.GDP.MKTP.KD.ZG": "gdp_growth",   // Real GDP growth (%)
	"BN.CAB.XOKA.GD.ZS": "current_acct", // Current account balance (% of GDP)
	"FP.CPI.TOTL.ZG":    "inflation",    // CPI inflation (%)
	"FI.RES.XFGD.CD":    "fx_reserves",  // Total reserves excl. gold (current USD)
}

// currencyCountry maps currency codes to World Bank country codes.
var currencyCountry = map[string]string{
	"EUR": "EMU", // Euro area
	"GBP": "GBR", // United Kingdom
	"JPY": "JPN", // Japan
	"AUD": "AUS", // Australia
	"NZD": "NZL", // New Zealand
	"CAD": "CAN", // Canada
	"CHF": "CHE", // Switzerland
	"USD": "USA", // United States
}

// currencyName maps currency codes to country/economy names for display.
var currencyName = map[string]string{
	"EUR": "Euro Area",
	"GBP": "United Kingdom",
	"JPY": "Japan",
	"AUD": "Australia",
	"NZD": "New Zealand",
	"CAD": "Canada",
	"CHF": "Switzerland",
	"USD": "United States",
}

// wbBaseURL is the World Bank API v2 base URL.
const wbBaseURL = "https://api.worldbank.org/v2/country/%s/indicator/%s?format=json&mrv=5&per_page=5"

// ---- in-memory cache -------------------------------------------------------

const wbCacheTTL = 7 * 24 * time.Hour // World Bank data is annual; cache 7 days

type wbCachedData struct {
	data      *WorldBankData
	fetchedAt time.Time
}

var (
	wbCache   *wbCachedData //nolint:gochecknoglobals
	wbCacheMu sync.RWMutex  //nolint:gochecknoglobals
)

// GetWorldBankCachedOrFetch returns cached WorldBankData if fresh; otherwise fetches.
func GetWorldBankCachedOrFetch(ctx context.Context) (*WorldBankData, error) {
	wbCacheMu.RLock()
	if wbCache != nil && time.Since(wbCache.fetchedAt) < wbCacheTTL {
		d := wbCache.data
		wbCacheMu.RUnlock()
		return d, nil
	}
	wbCacheMu.RUnlock()

	// Cache miss — fetch
	data, err := FetchWorldBankMacro(ctx)
	if err != nil {
		return nil, err
	}

	wbCacheMu.Lock()
	wbCache = &wbCachedData{data: data, fetchedAt: time.Now()}
	wbCacheMu.Unlock()

	return data, nil
}

// WorldBankCacheAge returns the age of the World Bank cache in hours, or -1 if empty.
func WorldBankCacheAge() float64 {
	wbCacheMu.RLock()
	defer wbCacheMu.RUnlock()
	if wbCache == nil {
		return -1
	}
	return time.Since(wbCache.fetchedAt).Hours()
}

// ---- World Bank API response structs ---------------------------------------

type wbResponseEntry struct {
	Date  string   `json:"date"`
	Value *float64 `json:"value"` // pointer to handle JSON null
}

type wbPage struct {
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// wbFetch fetches the most-recent non-null value for one country/indicator pair.
// The World Bank API returns a 2-element array: [pagination_info, []data].
func wbFetch(ctx context.Context, client *http.Client, country, indicator string) (float64, int, error) {
	url := fmt.Sprintf(wbBaseURL, country, indicator)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("worldbank: build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("worldbank: http: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("worldbank: status %d for %s/%s", resp.StatusCode, country, indicator)
	}

	// Decode as [pagination_object, array_of_observations]
	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return 0, 0, fmt.Errorf("worldbank: decode outer: %w", err)
	}
	if len(raw) < 2 {
		return 0, 0, fmt.Errorf("worldbank: unexpected response shape for %s/%s", country, indicator)
	}

	var entries []wbResponseEntry
	if err := json.Unmarshal(raw[1], &entries); err != nil {
		return 0, 0, fmt.Errorf("worldbank: decode entries: %w", err)
	}

	// Find most recent non-null value
	for _, e := range entries {
		if e.Value != nil {
			year := 0
			fmt.Sscanf(e.Date, "%d", &year) //nolint:errcheck
			return *e.Value, year, nil
		}
	}
	return 0, 0, nil // no data available
}

// FetchWorldBankMacro fetches annual macro fundamentals for all 8 currency blocs.
// Requests are sequential with a 300ms polite delay to avoid hammering the API.
func FetchWorldBankMacro(ctx context.Context) (*WorldBankData, error) {
	wbLog := log.With().Str("source", "worldbank").Logger()
	wbLog.Info().Msg("Fetching World Bank macro fundamentals")

	client := &http.Client{Timeout: 15 * time.Second}
	result := &WorldBankData{
		Countries: make(map[string]*CountryMacro, len(currencyCountry)),
		FetchedAt: time.Now(),
	}

	for currency, country := range currencyCountry {
		cm := &CountryMacro{
			Country:  currencyName[currency],
			Currency: currency,
		}

		for indicator, field := range wbIndicators {
			val, year, err := wbFetch(ctx, client, country, indicator)
			if err != nil {
				wbLog.Warn().Err(err).Str("country", country).Str("indicator", indicator).Msg("WorldBank fetch skipped")
				continue
			}

			switch field {
			case "gdp_growth":
				cm.GDPGrowthYoY = val
				if year > cm.Year {
					cm.Year = year
				}
			case "current_acct":
				cm.CurrentAccount = val
			case "inflation":
				cm.InflationCPI = val
			case "fx_reserves":
				// Convert from USD to USD billions
				cm.FXReserves = val / 1e9
			}

			// Polite delay between requests
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(300 * time.Millisecond):
			}
		}

		result.Countries[currency] = cm
		wbLog.Debug().Str("currency", currency).
			Float64("gdp", cm.GDPGrowthYoY).
			Float64("ca", cm.CurrentAccount).
			Float64("cpi", cm.InflationCPI).
			Msg("WorldBank country data fetched")
	}

	result.Available = len(result.Countries) > 0
	wbLog.Info().Int("countries", len(result.Countries)).Msg("World Bank fetch complete")
	return result, nil
}
