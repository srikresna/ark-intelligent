package intermarket

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

const (
	// cacheTTL is how long intermarket results are cached (correlations change slowly).
	cacheTTL = 4 * time.Hour

	// minDataPoints is the minimum number of returns needed for a valid correlation.
	minDataPoints = 15

	// calendarDayMultiplier accounts for weekends/holidays in fetching window.
	calendarDayMultiplier = 2
)

var engineLog = logger.Component("intermarket")

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

// Engine computes intermarket correlation signals using existing price data.
type Engine struct {
	priceRepo DailyPriceStore
}

// NewEngine creates a new Engine.
func NewEngine(priceRepo DailyPriceStore) *Engine {
	return &Engine{priceRepo: priceRepo}
}

// ---------------------------------------------------------------------------
// In-process cache (4h TTL)
// ---------------------------------------------------------------------------

type cachedResult struct {
	result    *IntermarketResult
	fetchedAt time.Time
}

var (
	globalCache *cachedResult
	cacheMu     sync.RWMutex
)

// GetCachedOrAnalyze returns cached IntermarketResult within TTL, else runs Analyze.
func (e *Engine) GetCachedOrAnalyze(ctx context.Context) (*IntermarketResult, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.fetchedAt) < cacheTTL {
		r := globalCache.result
		cacheMu.RUnlock()
		engineLog.Debug().Msg("intermarket: returning cached result")
		return r, nil
	}
	cacheMu.RUnlock()

	result, err := e.Analyze(ctx)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	globalCache = &cachedResult{result: result, fetchedAt: time.Now()}
	cacheMu.Unlock()

	return result, nil
}

// InvalidateCache clears the cached result, forcing the next call to re-compute.
func InvalidateCache() {
	cacheMu.Lock()
	globalCache = nil
	cacheMu.Unlock()
}

// ---------------------------------------------------------------------------
// Analyze
// ---------------------------------------------------------------------------

// Analyze runs all StandardRules and returns an IntermarketResult.
func (e *Engine) Analyze(ctx context.Context) (*IntermarketResult, error) {
	// Fetch all price series we need (deduplicated).
	needed := neededCurrencies(StandardRules)
	seriesMap, err := e.fetchSeries(ctx, needed)
	if err != nil {
		return nil, fmt.Errorf("intermarket: fetch error: %w", err)
	}

	var signals []IntermarketSignal
	var divergences []IntermarketSignal

	for _, rule := range StandardRules {
		sig := e.evaluateRule(rule, seriesMap)
		signals = append(signals, sig)
		if sig.Status == StatusDiverging || sig.Status == StatusBroken {
			divergences = append(divergences, sig)
		}
	}

	regime := synthesizeRiskRegime(signals)

	return &IntermarketResult{
		Signals:     signals,
		Divergences: divergences,
		RiskRegime:  regime,
		AsOf:        time.Now(),
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// neededCurrencies returns deduplicated currency codes referenced by rules.
func neededCurrencies(rules []IntermarketRule) []string {
	seen := make(map[string]bool)
	var out []string
	for _, r := range rules {
		if !seen[r.Base] {
			seen[r.Base] = true
			out = append(out, r.Base)
		}
		if !seen[r.Correlated] {
			seen[r.Correlated] = true
			out = append(out, r.Correlated)
		}
	}
	return out
}

// fetchSeries retrieves 30-day daily returns for each currency.
// Returns oldest-first log-returns. Inverse pairs (USDJPY, USDCAD, etc.) are
// automatically flipped so the series represents the base currency.
func (e *Engine) fetchSeries(ctx context.Context, currencies []string) (map[string][]float64, error) {
	out := make(map[string][]float64)
	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			engineLog.Warn().Str("currency", cur).Msg("skip: no price mapping")
			continue
		}

		window := 20 // maximum window needed
		calDays := window * calendarDayMultiplier
		records, err := e.priceRepo.GetDailyHistory(ctx, mapping.ContractCode, calDays)
		if err != nil {
			engineLog.Warn().Str("currency", cur).Err(err).Msg("skip: fetch error")
			continue
		}

		returns := computeReturns(records, window+5, mapping.Inverse)
		if len(returns) < minDataPoints {
			engineLog.Warn().Str("currency", cur).Int("points", len(returns)).Msg("skip: insufficient data")
			continue
		}
		out[cur] = returns
	}
	return out, nil
}

// computeReturns converts newest-first DailyPrice records into oldest-first log-returns.
// If inverse=true the price is 1/close to convert e.g. USDJPY → JPYUSD direction.
func computeReturns(records []domain.DailyPrice, window int, inverse bool) []float64 {
	n := len(records)
	if n < 2 {
		return nil
	}
	end := window
	if end >= n {
		end = n - 1
	}
	// records[0] = most recent, records[end] = oldest we need
	// Compute oldest-first returns
	var returns []float64
	for i := end; i >= 1; i-- {
		prev := records[i].Close
		curr := records[i-1].Close
		if prev <= 0 || curr <= 0 {
			continue
		}
		var ret float64
		if inverse {
			// Flip: price represents USD/X, we want X/USD direction
			ret = math.Log(prev / curr) // inverse: when USDJPY falls, JPY rises
		} else {
			ret = math.Log(curr / prev)
		}
		returns = append(returns, ret)
	}
	return returns
}

// evaluateRule computes correlation for one rule and classifies its status.
func (e *Engine) evaluateRule(rule IntermarketRule, seriesMap map[string][]float64) IntermarketSignal {
	baseSeries, baseOK := seriesMap[rule.Base]
	corrSeries, corrOK := seriesMap[rule.Correlated]

	if !baseOK || !corrOK {
		return IntermarketSignal{
			Rule:         rule,
			Status:       StatusAligned,
			Insufficient: true,
			Implication:  "Insufficient data",
		}
	}

	corr := pearsonCorrelation(baseSeries, corrSeries, rule.Window)
	if math.IsNaN(corr) {
		return IntermarketSignal{
			Rule:         rule,
			ActualCorr:   0,
			Status:       StatusAligned,
			Insufficient: true,
			Implication:  "Insufficient data",
		}
	}

	// Divergence detection:
	// agreement = actual_corr * expected_direction
	// > 0.2  → ALIGNED
	// < 0    → DIVERGING
	// < -0.3 → BROKEN
	agreement := corr * float64(rule.Direction)

	var status CorrelationStatus
	switch {
	case agreement < -0.3:
		status = StatusBroken
	case agreement < 0:
		status = StatusDiverging
	default:
		status = StatusAligned
	}

	// Strength = how far we are from neutral (0) → mapped to 0–1
	strength := math.Abs(corr)

	implication := buildImplication(rule, corr, status)

	return IntermarketSignal{
		Rule:        rule,
		ActualCorr:  corr,
		Status:      status,
		Implication: implication,
		Strength:    strength,
	}
}

// pearsonCorrelation computes the Pearson r between the last `window` points of x and y.
// Returns NaN if there are fewer than minDataPoints in the overlap.
func pearsonCorrelation(x, y []float64, window int) float64 {
	// Align to the same length using the last `window` points.
	n := window
	if len(x) < n {
		n = len(x)
	}
	if len(y) < n {
		n = len(y)
	}
	if n < minDataPoints {
		return math.NaN()
	}

	xs := x[len(x)-n:]
	ys := y[len(y)-n:]

	var sumX, sumY float64
	for i := 0; i < n; i++ {
		sumX += xs[i]
		sumY += ys[i]
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	var num, denX, denY float64
	for i := 0; i < n; i++ {
		dx := xs[i] - meanX
		dy := ys[i] - meanY
		num += dx * dy
		denX += dx * dx
		denY += dy * dy
	}

	denom := math.Sqrt(denX * denY)
	if denom == 0 {
		return math.NaN()
	}
	return num / denom
}

// buildImplication generates a trading implication string for a divergence.
func buildImplication(rule IntermarketRule, corr float64, status CorrelationStatus) string {
	if status == StatusAligned {
		return fmt.Sprintf("%s relationship holding (r=%.2f)", rule.Label, corr)
	}

	// Direction-specific implications
	switch rule.Label {
	case "AUD-Gold":
		if corr < 0 {
			return "AUD not following Gold → divergence; watch for AUD catch-up rally or Gold pullback"
		}
		return fmt.Sprintf("AUD-Gold diverging (r=%.2f) → monitor for re-alignment", corr)
	case "AUD-Equities (risk-on)":
		if corr < 0 {
			return "AUD weakening despite equity strength → risk-on signal breaking, fade AUD rallies"
		}
		return fmt.Sprintf("AUD-Equities diverging (r=%.2f) → watch for AUD catch-up or equity fade", corr)
	case "CAD-Oil (via USDCAD)":
		if corr < 0 {
			return "CAD not following Oil price → watch for USDCAD snap-back or oil reversal"
		}
		return fmt.Sprintf("CAD-Oil diverging (r=%.2f) → monitor supply/demand disconnect", corr)
	case "JPY-Yields (carry)":
		if corr < 0 {
			return "JPY strengthening despite rising yields → safe-haven demand overriding carry; risk-off alert"
		}
		return fmt.Sprintf("JPY-Yields diverging (r=%.2f) → carry trade under stress", corr)
	case "JPY-Equities (risk-off)":
		if corr > 0 {
			return "JPY weakening WITH equities rising → risk-on override, carry trade back in play"
		}
		return fmt.Sprintf("JPY-Equities diverging (r=%.2f) → monitor safe-haven demand", corr)
	case "CHF-Gold (safe haven)":
		if corr < 0 {
			return "CHF and Gold diverging → separate safe-haven drivers; watch for convergence"
		}
		return fmt.Sprintf("CHF-Gold diverging (r=%.2f)", corr)
	case "DXY-Gold (inverse)":
		if corr > 0 {
			return "DXY and Gold both moving same direction → dollar weakness driving Gold, or unique flow"
		}
		return fmt.Sprintf("DXY-Gold relationship weakening (r=%.2f)", corr)
	case "DXY-EUR (definitional)":
		if corr > 0 {
			return "EUR and DXY moving together → unusual flow; check for EUR-specific news"
		}
		return fmt.Sprintf("DXY-EUR correlation breaking (r=%.2f) → EUR-specific drivers in play", corr)
	case "Gold-Equities (crisis hedge)":
		if corr > 0 {
			return "Gold and equities both rising → dollar weakness driving both, not true risk-off"
		}
		return fmt.Sprintf("Gold-Equities diverging (r=%.2f) → reassess risk regime", corr)
	}

	// Generic fallback
	dir := "above"
	if corr < 0 {
		dir = "below"
	}
	return fmt.Sprintf("%s diverging: actual r=%.2f (%s expected) → watch for mean reversion", rule.Label, corr, dir)
}

// synthesizeRiskRegime determines the macro risk regime from all signals.
func synthesizeRiskRegime(signals []IntermarketSignal) string {
	var riskOnVotes, riskOffVotes int

	for _, s := range signals {
		if s.Insufficient {
			continue
		}
		// Signals that are ALIGNED give a regime vote
		if s.Status != StatusAligned {
			continue
		}
		switch s.Rule.Label {
		case "AUD-Equities (risk-on)":
			if s.ActualCorr > 0.2 {
				riskOnVotes++
			}
		case "JPY-Equities (risk-off)":
			// JPY/SPX normally negative = risk-off environment
			// Negative correlation = JPY rising as equities fall = risk-off
			if s.ActualCorr < -0.2 {
				riskOffVotes++
			} else if s.ActualCorr > 0.2 {
				riskOnVotes++
			}
		case "JPY-Yields (carry)":
			// JPY/BOND positive = rising yields hurting JPY = risk-on (carry)
			if s.ActualCorr > 0.2 {
				riskOnVotes++
			} else if s.ActualCorr < -0.2 {
				riskOffVotes++
			}
		case "Gold-Equities (crisis hedge)":
			// Negative = gold up, equities down = risk-off
			if s.ActualCorr < -0.2 {
				riskOffVotes++
			} else {
				riskOnVotes++
			}
		}
	}

	switch {
	case riskOnVotes > riskOffVotes+1:
		return "RISK_ON"
	case riskOffVotes > riskOnVotes+1:
		return "RISK_OFF"
	default:
		return "MIXED"
	}
}
