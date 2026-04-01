package price

import (
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Hurst Exponent — Rescaled Range (R/S) Analysis
// ---------------------------------------------------------------------------
//
// The Hurst exponent H classifies time series behaviour:
//   H < 0.5  → Mean-reverting (anti-persistent)
//   H = 0.5  → Random walk (no memory)
//   H > 0.5  → Trending (persistent)
//
// Method: Classical R/S analysis with multiple sub-period sizes.
// For each sub-period size n:
//   1. Divide the series into blocks of size n
//   2. For each block, compute cumulative deviations from the block mean
//   3. R(n) = max(cumdev) - min(cumdev)
//   4. S(n) = std dev of the block
//   5. (R/S)(n) = mean of R(n)/S(n) across blocks
//
// Then H is estimated as the slope of log(R/S) vs log(n).

// HurstResult holds the output of a Hurst exponent estimation.
type HurstResult struct {
	H             float64 `json:"h"`              // Hurst exponent (0-1)
	Classification string `json:"classification"` // "MEAN_REVERTING", "RANDOM_WALK", "TRENDING"
	Confidence    float64 `json:"confidence"`     // How far from 0.5 (strength of signal)
	RSquared      float64 `json:"r_squared"`      // Goodness of fit of log-log regression
	SampleSize    int     `json:"sample_size"`     // Number of observations used
	Description   string  `json:"description"`     // Human-readable interpretation
}

// HurstRegimeContext extends PriceRegime with Hurst-based classification.
type HurstRegimeContext struct {
	*PriceRegime
	Hurst              *HurstResult `json:"hurst,omitempty"`
	HurstRegime        string       `json:"hurst_regime"`         // "TRENDING", "RANGING", "RANDOM"
	RegimeAgreement    bool         `json:"regime_agreement"`     // ADX and Hurst agree
	CombinedConfidence float64      `json:"combined_confidence"`  // Confidence in regime classification
}

// ComputeHurstExponent estimates the Hurst exponent from price records
// using Rescaled Range (R/S) analysis.
//
// Prices must be newest-first. Requires at least 50 observations.
// Uses log-returns internally.
func ComputeHurstExponent(prices []domain.PriceRecord) (*HurstResult, error) {
	if len(prices) < 50 {
		return nil, fmt.Errorf("insufficient data for Hurst: need 50, got %d", len(prices))
	}

	// Compute log-returns in chronological order (oldest first)
	n := len(prices)
	returns := make([]float64, 0, n-1)
	for i := n - 1; i > 0; i-- {
		if prices[i].Close <= 0 || prices[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, math.Log(prices[i-1].Close/prices[i].Close))
	}

	return computeHurstFromReturns(returns)
}

// ComputeHurstFromIntraday estimates Hurst from intraday bars.
func ComputeHurstFromIntraday(bars []domain.IntradayBar) (*HurstResult, error) {
	if len(bars) < 50 {
		return nil, fmt.Errorf("insufficient intraday data for Hurst: need 50, got %d", len(bars))
	}

	n := len(bars)
	returns := make([]float64, 0, n-1)
	for i := n - 1; i > 0; i-- {
		if bars[i].Close <= 0 || bars[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, math.Log(bars[i-1].Close/bars[i].Close))
	}

	return computeHurstFromReturns(returns)
}

// computeHurstFromReturns performs R/S analysis on a return series.
// Returns must be in chronological order (oldest first).
func computeHurstFromReturns(returns []float64) (*HurstResult, error) {
	n := len(returns)
	if n < 20 {
		return nil, fmt.Errorf("insufficient returns for Hurst: need 20, got %d", n)
	}

	// Choose sub-period sizes: powers of 2 from 8 up to n/2
	var sizes []int
	for s := 8; s <= n/2; s *= 2 {
		sizes = append(sizes, s)
	}
	// Also add some intermediate sizes for better regression
	for s := 12; s <= n/2; s = int(float64(s) * 1.5) {
		sizes = appendUnique(sizes, s)
	}

	if len(sizes) < 3 {
		return nil, fmt.Errorf("insufficient range of sub-periods for Hurst estimation")
	}

	// Compute R/S for each sub-period size
	var logN, logRS []float64
	for _, size := range sizes {
		rs := rescaledRange(returns, size)
		if rs > 0 {
			logN = append(logN, math.Log(float64(size)))
			logRS = append(logRS, math.Log(rs))
		}
	}

	if len(logN) < 3 {
		return nil, fmt.Errorf("insufficient valid R/S values for regression")
	}

	// OLS regression: log(R/S) = H * log(n) + c
	h, _, r2 := simpleLinearRegression(logN, logRS)

	// Guard NaN/Inf from OLS (e.g. degenerate input)
	if math.IsNaN(h) || math.IsInf(h, 0) {
		h = 0.5
	}
	if math.IsNaN(r2) || math.IsInf(r2, 0) {
		r2 = 0
	}

	// Clamp H to reasonable range
	rawH := h
	if h < 0 {
		h = 0
	}
	if h > 1 {
		h = 1
	}

	result := &HurstResult{
		H:          roundN(h, 4),
		RSquared:   roundN(r2, 4),
		SampleSize: n,
	}

	// Classify
	// If raw H was outside [0,1], the fit is unreliable — zero confidence
	confidence := math.Abs(h-0.5) * 200 // 0-100 scale
	if rawH < 0 || rawH > 1 {
		confidence = 0
	}
	result.Confidence = roundN(confidence, 2)

	switch {
	case h < 0.40:
		result.Classification = "MEAN_REVERTING"
		result.Description = fmt.Sprintf("H=%.3f — strong mean reversion (anti-persistent)", h)
	case h < 0.45:
		result.Classification = "MEAN_REVERTING"
		result.Description = fmt.Sprintf("H=%.3f — mild mean reversion", h)
	case h <= 0.55:
		result.Classification = "RANDOM_WALK"
		result.Description = fmt.Sprintf("H=%.3f — approximately random walk (no exploitable pattern)", h)
	case h <= 0.60:
		result.Classification = "TRENDING"
		result.Description = fmt.Sprintf("H=%.3f — mild trending tendency", h)
	default:
		result.Classification = "TRENDING"
		result.Description = fmt.Sprintf("H=%.3f — strong trending behaviour (persistent)", h)
	}

	return result, nil
}

// rescaledRange computes the average R/S statistic for a given sub-period size.
func rescaledRange(returns []float64, size int) float64 {
	n := len(returns)
	numBlocks := n / size
	if numBlocks == 0 {
		return 0
	}

	var totalRS float64
	validBlocks := 0

	for b := 0; b < numBlocks; b++ {
		start := b * size
		block := returns[start : start+size]

		// Block mean
		var sum float64
		for _, v := range block {
			sum += v
		}
		mean := sum / float64(size)

		// Standard deviation
		var ss float64
		for _, v := range block {
			d := v - mean
			ss += d * d
		}
		s := math.Sqrt(ss / float64(size-1))
		if s < 1e-15 {
			continue // Skip blocks with zero variance
		}

		// Cumulative deviations from mean
		cumDev := 0.0
		maxDev := math.Inf(-1)
		minDev := math.Inf(1)
		for _, v := range block {
			cumDev += v - mean
			if cumDev > maxDev {
				maxDev = cumDev
			}
			if cumDev < minDev {
				minDev = cumDev
			}
		}

		r := maxDev - minDev
		totalRS += r / s
		validBlocks++
	}

	if validBlocks == 0 {
		return 0
	}
	return totalRS / float64(validBlocks)
}

// simpleLinearRegression performs y = a*x + b, returns (slope, intercept, r²).
func simpleLinearRegression(x, y []float64) (float64, float64, float64) {
	n := float64(len(x))
	if n < 2 {
		return 0, 0, 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-15 {
		return 0, 0, 0
	}

	slope := (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n

	// R²
	ssRes := 0.0
	ssTot := 0.0
	meanY := sumY / n
	for i := range x {
		predicted := slope*x[i] + intercept
		ssRes += (y[i] - predicted) * (y[i] - predicted)
		ssTot += (y[i] - meanY) * (y[i] - meanY)
	}
	r2 := 0.0
	if ssTot > 1e-15 {
		r2 = 1 - ssRes/ssTot
	}
	// Clamp R² to [0, 1] — negative means model is worse than mean
	if r2 < 0 {
		r2 = 0
	}

	return slope, intercept, r2
}

// appendUnique appends v to slice if not already present.
func appendUnique(slice []int, v int) []int {
	for _, s := range slice {
		if s == v {
			return slice
		}
	}
	return append(slice, v)
}

// HurstToRegime maps a Hurst classification to a price regime string.
// This complements the ADX-based regime in regime.go.
func HurstToRegime(h *HurstResult) string {
	if h == nil {
		return ""
	}
	switch h.Classification {
	case "TRENDING":
		return RegimeTrending
	case "MEAN_REVERTING":
		return RegimeRanging
	default:
		return RegimeRanging // Random walk → treat as ranging
	}
}

// CombineRegimeClassification merges ADX-based and Hurst-based regime detection.
//
// When both agree, confidence is high (both confirming the same regime).
// When they disagree, we prefer Hurst for statistical robustness but
// flag lower confidence.
func CombineRegimeClassification(adxRegime *PriceRegime, hurst *HurstResult) *HurstRegimeContext {
	ctx := &HurstRegimeContext{
		PriceRegime: adxRegime,
		Hurst:       hurst,
	}

	// Handle nil inputs — avoid nil pointer dereference on embedded *PriceRegime
	if adxRegime == nil && hurst == nil {
		ctx.PriceRegime = &PriceRegime{Regime: RegimeRanging}
		ctx.HurstRegime = RegimeRanging
		return ctx
	}
	if adxRegime == nil {
		// No ADX data — use Hurst only, provide a stub PriceRegime to avoid nil deref
		ctx.PriceRegime = &PriceRegime{Regime: HurstToRegime(hurst)}
		ctx.HurstRegime = HurstToRegime(hurst)
		ctx.CombinedConfidence = hurst.Confidence
		return ctx
	}
	if hurst == nil {
		ctx.HurstRegime = adxRegime.Regime
		ctx.CombinedConfidence = adxRegime.TrendStrength
		return ctx
	}

	hurstRegime := HurstToRegime(hurst)
	ctx.HurstRegime = hurstRegime

	// Crisis always takes priority
	if adxRegime.Regime == RegimeCrisis {
		ctx.RegimeAgreement = false
		ctx.CombinedConfidence = 90
		return ctx
	}

	// Check agreement
	ctx.RegimeAgreement = adxRegime.Regime == hurstRegime

	if ctx.RegimeAgreement {
		// Both agree — high confidence
		ctx.CombinedConfidence = clamp(
			(adxRegime.TrendStrength+hurst.Confidence)/2*1.2, // 20% bonus for agreement
			0, 100,
		)
	} else {
		// Disagreement — use weighted average, prefer Hurst if R² is good
		hurstWeight := 0.5
		if hurst.RSquared > 0.90 {
			hurstWeight = 0.65
		} else if hurst.RSquared < 0.70 {
			hurstWeight = 0.35
		}
		adxWeight := 1 - hurstWeight

		ctx.CombinedConfidence = clamp(
			adxRegime.TrendStrength*adxWeight+hurst.Confidence*hurstWeight*0.8, // 20% penalty for disagreement
			0, 100,
		)
	}

	return ctx
}
