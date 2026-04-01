package price

import (
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// ATR-based Volatility Regime Detection
// ---------------------------------------------------------------------------

// VolatilityRegime classifies the current volatility environment.
const (
	VolatilityExpanding   = "EXPANDING"   // ATR > avgATR * 1.25
	VolatilityContracting = "CONTRACTING" // ATR < avgATR * 0.75
	VolatilityNormal      = "NORMAL"      // otherwise
)

// VolatilityContext holds ATR-based volatility metrics for a contract.
type VolatilityContext struct {
	ATR20W               float64 `json:"atr_20w"`               // 20-week Average True Range
	NormalizedATR        float64 `json:"normalized_atr"`         // ATR / Close * 100 (percentage)
	AvgATR4W             float64 `json:"avg_atr_4w"`             // 4-week average of weekly ATR
	Regime               string  `json:"regime"`                 // EXPANDING, CONTRACTING, NORMAL
	WeeklyRange          float64 `json:"weekly_range"`           // Latest week (High-Low)/Close %
	ConfidenceMultiplier float64 `json:"confidence_multiplier"`  // Applied to signal confidence
}

// ComputeATR calculates the Average True Range from weekly OHLC price records.
// Records must be sorted newest-first. Period specifies how many bars to average.
// True Range = max(High-Low, |High-PrevClose|, |Low-PrevClose|).
// Returns 0 if insufficient data (need at least period+1 records).
func ComputeATR(prices []domain.PriceRecord, period int) float64 {
	if len(prices) < period+1 || period <= 0 {
		return 0
	}

	var sum float64
	// prices[0] is newest; iterate from newest to oldest, using i+1 as previous bar.
	for i := 0; i < period; i++ {
		tr := trueRange(prices[i], prices[i+1].Close)
		sum += tr
	}
	return sum / float64(period)
}

// ComputeNormalizedATR returns ATR as a percentage of the current close price.
// NormalizedATR = ATR / Close * 100.
// Returns 0 if close price is zero or insufficient data.
func ComputeNormalizedATR(prices []domain.PriceRecord, period int) float64 {
	if len(prices) == 0 || prices[0].Close == 0 {
		return 0
	}
	atr := ComputeATR(prices, period)
	if atr == 0 {
		return 0
	}
	result := roundN(atr/prices[0].Close*100, 4)
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}
	return result
}

// ClassifyVolatilityRegime returns the volatility regime based on current vs average ATR.
//   - EXPANDING:   currentATR > avgATR * 1.25
//   - CONTRACTING: currentATR < avgATR * 0.75
//   - NORMAL:      otherwise
func ClassifyVolatilityRegime(currentATR, avgATR float64) string {
	if avgATR <= 0 || currentATR <= 0 || math.IsNaN(currentATR) || math.IsInf(currentATR, 0) {
		return VolatilityNormal
	}
	ratio := currentATR / avgATR
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return VolatilityNormal
	}
	switch {
	case ratio > 1.25:
		return VolatilityExpanding
	case ratio < 0.75:
		return VolatilityContracting
	default:
		return VolatilityNormal
	}
}

// ComputeVolatilityContext builds a full volatility context from price records.
// Records must be sorted newest-first. Needs at least 21 records for full 20-week ATR;
// degrades gracefully with fewer records.
func ComputeVolatilityContext(prices []domain.PriceRecord) *VolatilityContext {
	if len(prices) < 2 {
		return nil
	}

	vc := &VolatilityContext{}

	// Latest week range
	vc.WeeklyRange = roundN(prices[0].WeeklyRange(), 4)

	// 20-week ATR (or shorter if not enough data)
	atrPeriod := 20
	if len(prices) < atrPeriod+1 {
		atrPeriod = len(prices) - 1
	}
	if atrPeriod < 1 {
		return nil
	}

	vc.ATR20W = roundN(ComputeATR(prices, atrPeriod), 6)
	vc.NormalizedATR = ComputeNormalizedATR(prices, atrPeriod)

	// 4-week average of individual weekly TRs (short-term volatility)
	shortPeriod := 4
	if len(prices) < shortPeriod+1 {
		shortPeriod = len(prices) - 1
	}
	if shortPeriod >= 1 {
		vc.AvgATR4W = roundN(ComputeATR(prices, shortPeriod), 6)
	}

	// Classify regime: compare short-term ATR against longer-term ATR
	vc.Regime = ClassifyVolatilityRegime(vc.AvgATR4W, vc.ATR20W)

	// Confidence multiplier based on regime
	switch vc.Regime {
	case VolatilityExpanding:
		vc.ConfidenceMultiplier = 0.85 // reduce 15% — high vol = noisy signals
	case VolatilityContracting:
		vc.ConfidenceMultiplier = 1.10 // boost 10% — low vol = cleaner signals
	default:
		vc.ConfidenceMultiplier = 1.00
	}

	return vc
}

// trueRange computes the True Range for a single bar.
// TR = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
func trueRange(bar domain.PriceRecord, prevClose float64) float64 {
	hl := bar.High - bar.Low
	hpc := math.Abs(bar.High - prevClose)
	lpc := math.Abs(bar.Low - prevClose)
	return math.Max(hl, math.Max(hpc, lpc))
}

// CombineVolatilityMultiplier produces a single confidence multiplier from
// ATR-based volatility and VIX-based risk context. This avoids double-penalizing
// when both indicators agree.
//
// Strategy:
//   - If VIX risk context is unavailable, use ATR multiplier directly.
//   - If both are available, average the two multipliers. This ensures they
//     are complementary rather than stacking (e.g., both reducing by 15% would
//     compound to 0.72x, but averaging gives 0.85x).
func CombineVolatilityMultiplier(volCtx *VolatilityContext, riskCtx *domain.RiskContext) float64 {
	atrMult := 1.0
	if volCtx != nil {
		atrMult = volCtx.ConfidenceMultiplier
	}

	if riskCtx == nil {
		return atrMult
	}

	vixMult := riskCtx.ConfidenceAdjustment()
	// Average the two multipliers to avoid double-penalizing
	return (atrMult + vixMult) / 2
}
