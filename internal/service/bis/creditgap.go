package bis

// creditgap.go — Credit-to-GDP gap data from BIS WS_CREDIT_GAP dataset.
// A positive credit gap (>2%) is an early warning indicator of financial crises
// (per BIS research). This is the primary BIS macro-prudential signal.
//
// API: https://stats.bis.org/api/v2/data/BIS,WS_CREDIT_GAP,1.0/
// No authentication required. Data updated quarterly.

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
	creditGapBaseURL  = "https://stats.bis.org/api/v2/data/BIS,WS_CREDIT_GAP,1.0"
	creditGapCacheTTL = 24 * time.Hour

	// BIS early warning thresholds for credit gap.
	creditGapWarnHigh  = 2.0  // >2% = early warning signal
	creditGapAlertHigh = 10.0 // >10% = high stress zone
)

// creditGapConfig lists countries we track for credit gap data.
// BIS publishes: US, UK, JP, XM (Eurozone), AU, CA, CH, SE, KR, CN
var creditGapConfig = []struct { //nolint:gochecknoglobals
	Code    string // BIS country code
	Country string // Display name
}{
	{"US", "US"},
	{"GB", "UK"},
	{"JP", "JP"},
	{"XM", "EU"},
	{"AU", "AU"},
	{"CA", "CA"},
	{"CH", "CH"},
}

// CreditGap holds credit-to-GDP gap metrics for one country.
type CreditGap struct {
	Country   string  // "US", "UK", etc.
	Gap       float64 // Credit gap in percentage points
	Signal    string  // "WARNING", "ALERT", "NORMAL"
	AsOf      string  // "2025-Q3"
	Available bool
}

// CreditGapReport holds credit gap data for all tracked countries.
type CreditGapReport struct {
	Gaps      []CreditGap
	FetchedAt time.Time
}

var (
	globalCreditCache *CreditGapReport //nolint:gochecknoglobals
	creditCacheMu     sync.RWMutex
)

// GetCreditGaps returns cached or fresh credit-to-GDP gap data.
func GetCreditGaps(ctx context.Context) *CreditGapReport {
	creditCacheMu.RLock()
	if globalCreditCache != nil && time.Since(globalCreditCache.FetchedAt) < creditGapCacheTTL {
		d := globalCreditCache
		creditCacheMu.RUnlock()
		return d
	}
	creditCacheMu.RUnlock()

	fresh := fetchCreditGaps(ctx)

	creditCacheMu.Lock()
	if anyCreditAvailable(fresh.Gaps) {
		globalCreditCache = fresh
	} else if globalCreditCache != nil {
		stale := globalCreditCache
		creditCacheMu.Unlock()
		log.Warn().Msg("BIS credit gap fetch failed; using stale cache")
		return stale
	}
	creditCacheMu.Unlock()

	return fresh
}

func fetchCreditGaps(ctx context.Context) *CreditGapReport {
	type result struct {
		gap CreditGap
		idx int
	}

	results := make([]CreditGap, len(creditGapConfig))
	ch := make(chan result, len(creditGapConfig))

	var wg sync.WaitGroup
	for i, cc := range creditGapConfig {
		wg.Add(1)
		go func(idx int, code, country string) {
			defer wg.Done()
			g := fetchOneGap(ctx, code, country)
			ch <- result{gap: g, idx: idx}
		}(i, cc.Code, cc.Country)
	}

	wg.Wait()
	close(ch)

	for r := range ch {
		results[r.idx] = r.gap
	}

	return &CreditGapReport{
		Gaps:      results,
		FetchedAt: time.Now(),
	}
}

func fetchOneGap(ctx context.Context, code, country string) CreditGap {
	entry := CreditGap{Country: country}

	// WS_CREDIT_GAP key: Q.{COUNTRY}.H.A.M — Total credit gap, All sectors,
	// Adjusted for breaks, Market value. Try common key structures.
	url := fmt.Sprintf("%s/Q.%s.H.A.M?lastNObservations=4&format=jsondata", creditGapBaseURL, code)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return entry
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ark-intelligent/1.0 (market-analysis-bot)")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Warn().Str("country", country).Err(err).Msg("BIS credit gap: http failed")
		return entry
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Str("country", country).Int("status", resp.StatusCode).Msg("BIS credit gap: non-200")
		return entry
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return entry
	}

	series, err := parseSDMXJSON(body)
	if err != nil || len(series) == 0 {
		log.Warn().Str("country", country).Err(err).Msg("BIS credit gap: parse failed")
		return entry
	}

	sort.Slice(series, func(i, j int) bool {
		return series[i].Period < series[j].Period
	})

	latest := series[len(series)-1]
	entry.Gap = latest.Value
	entry.AsOf = latest.Period
	entry.Signal = classifyCreditGap(latest.Value)
	entry.Available = true

	return entry
}

func classifyCreditGap(gap float64) string {
	switch {
	case gap > creditGapAlertHigh:
		return "ALERT"
	case gap > creditGapWarnHigh:
		return "WARNING"
	default:
		return "NORMAL"
	}
}

func anyCreditAvailable(gaps []CreditGap) bool {
	for _, g := range gaps {
		if g.Available {
			return true
		}
	}
	return false
}
