package bis

// cbpol.go — Central bank policy rate data from BIS WS_CBPOL dataset.
// API: https://stats.bis.org/api/v2/data/BIS,WS_CBPOL,1.0/
// No authentication required. Data updated monthly/quarterly.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	cbpolBaseURL  = "https://stats.bis.org/api/v2/data/BIS,WS_CBPOL,1.0"
	cbpolCacheTTL = 24 * time.Hour
)

// cbConfig maps central bank codes to display names for WS_CBPOL dataset.
var cbConfig = []struct { //nolint:gochecknoglobals
	Code     string // BIS country/area code
	Currency string // Currency label
	CB       string // Central bank short name
}{
	{"US", "USD", "Fed"},
	{"XM", "EUR", "ECB"},
	{"GB", "GBP", "BoE"},
	{"JP", "JPY", "BoJ"},
	{"CH", "CHF", "SNB"},
	{"AU", "AUD", "RBA"},
	{"CA", "CAD", "BoC"},
	{"NZ", "NZD", "RBNZ"},
}

// PolicyRate holds the current central bank policy rate for one currency.
type PolicyRate struct {
	Currency  string  // "USD", "EUR", etc.
	CB        string  // "Fed", "ECB", etc.
	Rate      float64 // Latest rate in percent
	PrevRate  float64 // Rate ~3 observations ago (for trend)
	Change    float64 // Rate - PrevRate
	Trend     string  // "HIKING", "CUTTING", "HOLD"
	AsOf      string  // "2025-11" or "2025-Q4"
	Available bool
}

// PolicyRateSuite holds all central bank rates.
type PolicyRateSuite struct {
	Rates     []PolicyRate
	FetchedAt time.Time
}

var (
	globalPolicyCache *PolicyRateSuite //nolint:gochecknoglobals
	policyCacheMu     sync.RWMutex
)

// GetPolicyRates returns cached or fresh central bank policy rates.
func GetPolicyRates(ctx context.Context) *PolicyRateSuite {
	policyCacheMu.RLock()
	if globalPolicyCache != nil && time.Since(globalPolicyCache.FetchedAt) < cbpolCacheTTL {
		d := globalPolicyCache
		policyCacheMu.RUnlock()
		return d
	}
	policyCacheMu.RUnlock()

	fresh := fetchPolicyRates(ctx)

	policyCacheMu.Lock()
	if anyPolicyAvailable(fresh.Rates) {
		globalPolicyCache = fresh
	} else if globalPolicyCache != nil {
		stale := globalPolicyCache
		policyCacheMu.Unlock()
		log.Warn().Msg("BIS CBPOL fetch failed; using stale cache")
		return stale
	}
	policyCacheMu.Unlock()

	return fresh
}

func fetchPolicyRates(ctx context.Context) *PolicyRateSuite {
	type result struct {
		rate PolicyRate
		idx  int
	}

	results := make([]PolicyRate, len(cbConfig))
	ch := make(chan result, len(cbConfig))

	var wg sync.WaitGroup
	for i, cc := range cbConfig {
		wg.Add(1)
		go func(idx int, code, currency, cbName string) {
			defer wg.Done()
			r := fetchOnePolicy(ctx, code, currency, cbName)
			ch <- result{rate: r, idx: idx}
		}(i, cc.Code, cc.Currency, cc.CB)
	}

	wg.Wait()
	close(ch)

	for r := range ch {
		results[r.idx] = r.rate
	}

	return &PolicyRateSuite{
		Rates:     results,
		FetchedAt: time.Now(),
	}
}

func fetchOnePolicy(ctx context.Context, code, currency, cbName string) PolicyRate {
	entry := PolicyRate{Currency: currency, CB: cbName}

	// Try quarterly first, then monthly.
	for _, freq := range []string{"Q", "M"} {
		url := fmt.Sprintf("%s/%s.%s.P?lastNObservations=6&format=jsondata", cbpolBaseURL, freq, code)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "ark-intelligent/1.0 (market-analysis-bot)")

		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			continue
		}

		series, parseErr := parseSDMXJSON(body)
		if parseErr != nil || len(series) == 0 {
			continue
		}

		return buildPolicyRate(series, currency, cbName)
	}

	log.Warn().Str("currency", currency).Msg("BIS CBPOL: no data from any frequency")
	return entry
}

func buildPolicyRate(series []observation, currency, cbName string) PolicyRate {
	entry := PolicyRate{Currency: currency, CB: cbName}

	sort.Slice(series, func(i, j int) bool {
		return series[i].Period < series[j].Period
	})

	n := len(series)
	latest := series[n-1]
	entry.Rate = latest.Value
	entry.AsOf = latest.Period

	// Compare to ~3 observations ago for trend.
	prevIdx := n - 4
	if prevIdx < 0 {
		prevIdx = 0
	}
	entry.PrevRate = series[prevIdx].Value
	entry.Change = entry.Rate - entry.PrevRate
	entry.Trend = classifyPolicyTrend(entry.Change)
	entry.Available = true

	return entry
}

func classifyPolicyTrend(change float64) string {
	const threshold = 0.001 // ~0.1 bps noise floor
	switch {
	case change > threshold:
		return "HIKING"
	case change < -threshold:
		return "CUTTING"
	default:
		return "HOLD"
	}
}

func anyPolicyAvailable(rates []PolicyRate) bool {
	for _, r := range rates {
		if r.Available {
			return true
		}
	}
	return false
}
