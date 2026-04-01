// Package bis provides integration with the Bank for International Settlements (BIS)
// Effective Exchange Rate (EER) data.
//
// REER (Real Effective Exchange Rate) measures a currency's value against a basket
// of trading partners' currencies, adjusted for inflation differentials. It shows
// whether a currency is fundamentally overvalued or undervalued — a key input for
// medium-term FX directional analysis.
//
// API: https://stats.bis.org/api/v2/data/BIS,WS_EER,1.0/
// No authentication required. Data is updated monthly.
// Cache TTL: 24 hours (data updates once per month).
package bis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("bis") //nolint:gochecknoglobals

// BIS EER API parameters.
//
// URL pattern:
//
//	https://stats.bis.org/api/v2/data/BIS,WS_EER,1.0/M.B.{COUNTRY}.{TYPE}.?startPeriod=2019-01
//	M = Monthly frequency
//	B = Broad basket (60+ trading partners, preferred over Narrow)
//	TYPE: A = CPI-deflated (REER), N = Nominal (NEER)
const (
	bisBaseURL    = "https://stats.bis.org/api/v2/data/BIS,WS_EER,1.0"
	startPeriod   = "2019-01" // 5+ years for long-term average
	cacheTTL      = 24 * time.Hour
	httpTimeout   = 20 * time.Second
	ltAvgWindow   = 60 // months for long-term average (5 years)
	fairZonePct   = 5.0 // ±5% deviation = FAIR
)

// currencyConfig maps BIS country codes to display currency names.
// XM = Eurozone (BIS code), US = United States, etc.
var currencyConfig = []struct { //nolint:gochecknoglobals
	Code     string // BIS country code
	Currency string // Display currency symbol
}{
	{"US", "USD"},
	{"XM", "EUR"},
	{"GB", "GBP"},
	{"JP", "JPY"},
	{"CH", "CHF"},
	{"AU", "AUD"},
	{"CA", "CAD"},
	{"NZ", "NZD"},
}

// REERData holds REER and NEER data for a single currency.
type REERData struct {
	Currency  string  // "USD", "EUR", etc.
	REER      float64 // Latest REER value (index, base=2020)
	NEER      float64 // Latest NEER value (index, base=2020)
	LTAvg     float64 // Long-term average REER computed from ltAvgWindow months
	Deviation float64 // (REER / LTAvg - 1) * 100 — positive = overvalued
	Signal    string  // "OVERVALUED", "FAIR", "UNDERVALUED"
	AsOf      string  // "2025-11" (month string of latest data point)
	Available bool    // false if fetch or parse failed
}

// BISData is the top-level result returned by FetchBISData.
type BISData struct {
	Currencies []REERData
	FetchedAt  time.Time
}

// cache fields (package-level, protected by cacheMu).
var (
	globalCache *BISData //nolint:gochecknoglobals
	cacheMu     sync.RWMutex
	httpClient  = &http.Client{Timeout: httpTimeout} //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached BIS data if within TTL, otherwise fetches
// fresh data from the BIS API. Gracefully degrades: if the fetch fails, returns
// stale cache if available, or a BISData with all Available=false entries.
func GetCachedOrFetch(ctx context.Context) (*BISData, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.FetchedAt) < cacheTTL {
		data := globalCache
		cacheMu.RUnlock()
		return data, nil
	}
	cacheMu.RUnlock()

	data, err := FetchBISData(ctx)
	if err != nil {
		// Graceful degradation: return stale cache if available
		cacheMu.RLock()
		stale := globalCache
		cacheMu.RUnlock()
		if stale != nil {
			log.Warn().Err(err).Msg("BIS REER fetch failed; using stale cache")
			return stale, nil
		}
		// No stale cache — return empty result so callers can gracefully skip
		log.Warn().Err(err).Msg("BIS REER fetch failed; no stale cache available")
		return &BISData{
			Currencies: makeFallbackSlice(),
			FetchedAt:  time.Now(),
		}, nil
	}

	cacheMu.Lock()
	globalCache = data
	cacheMu.Unlock()

	return data, nil
}

// FetchBISData fetches REER and NEER data for all configured currencies in
// parallel. Currencies that fail to fetch are included with Available=false.
func FetchBISData(ctx context.Context) (*BISData, error) {
	type result struct {
		data REERData
		idx  int
	}

	results := make([]REERData, len(currencyConfig))
	ch := make(chan result, len(currencyConfig))

	var wg sync.WaitGroup
	for i, cc := range currencyConfig {
		wg.Add(1)
		go func(idx int, code, currency string) {
			defer wg.Done()
			data := fetchCurrency(ctx, code, currency)
			ch <- result{data: data, idx: idx}
		}(i, cc.Code, cc.Currency)
	}

	wg.Wait()
	close(ch)

	for r := range ch {
		results[r.idx] = r.data
	}

	return &BISData{
		Currencies: results,
		FetchedAt:  time.Now(),
	}, nil
}

// fetchCurrency fetches REER (type A) and NEER (type N) for a single currency.
func fetchCurrency(ctx context.Context, code, currency string) REERData {
	entry := REERData{Currency: currency}

	// Fetch REER (CPI-deflated)
	reerSeries, err := fetchSeries(ctx, code, "A")
	if err != nil {
		log.Warn().Str("currency", currency).Str("type", "REER").Err(err).Msg("BIS fetch failed")
		return entry
	}
	if len(reerSeries) == 0 {
		log.Warn().Str("currency", currency).Msg("BIS REER: empty series")
		return entry
	}

	// Fetch NEER (nominal)
	neerSeries, err := fetchSeries(ctx, code, "N")
	if err != nil {
		log.Warn().Str("currency", currency).Str("type", "NEER").Err(err).Msg("BIS fetch failed")
		return entry
	}

	// Sort series by period ascending (oldest first) for LT average calculation
	sort.Slice(reerSeries, func(i, j int) bool {
		return reerSeries[i].Period < reerSeries[j].Period
	})

	// Latest REER value (last in sorted slice)
	latest := reerSeries[len(reerSeries)-1]
	entry.REER = latest.Value
	entry.AsOf = latest.Period

	// Long-term average: use up to ltAvgWindow most recent observations
	window := reerSeries
	if len(window) > ltAvgWindow {
		window = window[len(window)-ltAvgWindow:]
	}
	sum := 0.0
	for _, obs := range window {
		sum += obs.Value
	}
	entry.LTAvg = sum / float64(len(window))

	// Deviation from long-term average
	if entry.LTAvg != 0 {
		entry.Deviation = (entry.REER/entry.LTAvg - 1) * 100
	}

	// Signal classification
	entry.Signal = classifySignal(entry.Deviation)

	// Latest NEER
	if len(neerSeries) > 0 {
		sort.Slice(neerSeries, func(i, j int) bool {
			return neerSeries[i].Period < neerSeries[j].Period
		})
		entry.NEER = neerSeries[len(neerSeries)-1].Value
	}

	entry.Available = true
	return entry
}

// classifySignal classifies deviation into OVERVALUED / FAIR / UNDERVALUED.
func classifySignal(deviation float64) string {
	switch {
	case deviation > fairZonePct:
		return "OVERVALUED"
	case deviation < -fairZonePct:
		return "UNDERVALUED"
	default:
		return "FAIR"
	}
}

// observation holds a single SDMX-JSON time-series data point.
type observation struct {
	Period string
	Value  float64
}

// sdmxResponse is the minimal SDMX-JSON structure we need to parse.
// BIS returns: {"data": {"dataSets": [{"series": {"0:0:0:0:0": {"observations": {"0": [value]}}}}]}}
type sdmxResponse struct {
	Data struct {
		Structure struct {
			Dimensions struct {
				Observation []struct {
					Values []struct {
						ID string `json:"id"`
					} `json:"values"`
				} `json:"observation"`
			} `json:"observation"`
		} `json:"dimensions"`
		DataSets []struct {
			Series map[string]struct {
				Observations map[string][]float64 `json:"observations"`
			} `json:"series"`
		} `json:"dataSets"`
	} `json:"data"`
}

// fetchSeries calls the BIS SDMX-JSON API for one currency+type combination
// and returns the time series of observations.
func fetchSeries(ctx context.Context, countryCode, seriesType string) ([]observation, error) {
	url := fmt.Sprintf(
		"%s/M.B.%s.%s.?startPeriod=%s&format=jsondata",
		bisBaseURL, countryCode, seriesType, startPeriod,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "ark-intelligent/1.0 (market-analysis-bot)")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get BIS/%s/%s: %w", countryCode, seriesType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BIS API status %d for %s/%s", resp.StatusCode, countryCode, seriesType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseSDMXJSON(body)
}

// parseSDMXJSON parses BIS SDMX-JSON response into a slice of observations.
// The SDMX-JSON format stores time periods in the dimension structure and
// observation values as a map of index → [value, ...].
func parseSDMXJSON(body []byte) ([]observation, error) {
	var raw sdmxResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal SDMX-JSON: %w", err)
	}

	if len(raw.Data.DataSets) == 0 {
		return nil, fmt.Errorf("no dataSets in SDMX response")
	}

	// Get time period labels from structure dimensions
	obsDims := raw.Data.Structure.Dimensions.Observation
	if len(obsDims) == 0 {
		return nil, fmt.Errorf("no observation dimensions in SDMX response")
	}
	// First observation dimension contains the time periods
	timeDim := obsDims[0]
	periods := make([]string, len(timeDim.Values))
	for i, v := range timeDim.Values {
		periods[i] = v.ID
	}

	dataSet := raw.Data.DataSets[0]

	// Each series key maps to observations map; typically one series per request
	var observations []observation
	for _, series := range dataSet.Series {
		for idxStr, vals := range series.Observations {
			if len(vals) == 0 {
				continue
			}
			idx := 0
			if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil {
				continue
			}
			if idx < 0 || idx >= len(periods) {
				continue
			}
			observations = append(observations, observation{
				Period: periods[idx],
				Value:  vals[0],
			})
		}
	}

	if len(observations) == 0 {
		return nil, fmt.Errorf("no observations parsed from SDMX response")
	}

	return observations, nil
}

// makeFallbackSlice returns a slice of REERData with Available=false for all
// configured currencies — used as graceful degradation when the API is unreachable.
func makeFallbackSlice() []REERData {
	result := make([]REERData, len(currencyConfig))
	for i, cc := range currencyConfig {
		result[i] = REERData{Currency: cc.Currency, Available: false}
	}
	return result
}
