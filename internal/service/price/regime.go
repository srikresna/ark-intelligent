package price

import (
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Price Regime Detection — TRENDING / RANGING / CRISIS
// ---------------------------------------------------------------------------

// Price regime constants.
const (
	RegimeTrending = "TRENDING"
	RegimeRanging  = "RANGING"
	RegimeCrisis   = "CRISIS"
)

// PriceRegime holds the classified market regime for a contract.
type PriceRegime struct {
	ContractCode  string  `json:"contract_code"`
	Currency      string  `json:"currency"`
	Regime        string  `json:"regime"`         // "TRENDING", "RANGING", "CRISIS"
	ADX           float64 `json:"adx"`            // Approximated ADX value
	TrendStrength float64 `json:"trend_strength"` // 0-100
	Description   string  `json:"description"`    // Human-readable
}

// ClassifyPriceRegime determines whether a contract is TRENDING, RANGING, or in CRISIS
// based on an ADX approximation, ATR ratio, MA distance, and weekly range analysis.
//
// prices must be sorted newest-first and should contain at least 14 records for a
// meaningful ADX approximation. pc may be nil, in which case only price-derived
// metrics are used.
func ClassifyPriceRegime(prices []domain.PriceRecord, pc *domain.PriceContext) *PriceRegime {
	if len(prices) < 2 {
		return nil
	}

	pr := &PriceRegime{}
	if pc != nil {
		pr.ContractCode = pc.ContractCode
		pr.Currency = pc.Currency
	}

	// --- 1. Approximate ADX from weekly OHLC ---
	pr.ADX = approximateADX(prices)

	// --- 2. Crisis detection (checked first — overrides everything) ---
	if isCrisis(prices, pc) {
		pr.Regime = RegimeCrisis
		pr.TrendStrength = clamp(pr.ADX, 0, 100)
		pr.Description = describeCrisis(pc)
		return pr
	}

	// --- 3. Trending vs Ranging ---
	trending := false

	// ADX > 25 implies trending
	if pr.ADX > 25 {
		trending = true
	}

	// Price consistently above/below both MAs with aligned trends
	if pc != nil && pc.Trend4W == pc.Trend13W && pc.Trend4W != "FLAT" {
		if (pc.AboveMA4W && pc.AboveMA13W) || (!pc.AboveMA4W && !pc.AboveMA13W) {
			trending = true
		}
	}

	if trending {
		pr.Regime = RegimeTrending
		pr.TrendStrength = clamp(pr.ADX, 0, 100)
		pr.Description = describeTrending(pc, pr.ADX)
		return pr
	}

	// ADX < 20 AND price oscillating around MAs
	ranging := false
	if pr.ADX < 20 {
		ranging = true
	}

	// Oscillation: above one MA but not the other, or both trends FLAT
	if pc != nil {
		if pc.AboveMA4W != pc.AboveMA13W {
			ranging = true
		}
		if pc.Trend4W == "FLAT" && pc.Trend13W == "FLAT" {
			ranging = true
		}
	}

	if ranging {
		pr.Regime = RegimeRanging
		pr.TrendStrength = clamp(pr.ADX, 0, 100)
		pr.Description = describeRanging(pc, pr.ADX)
		return pr
	}

	// Default: if ADX is between 20–25 and no clear signal, treat as ranging
	pr.Regime = RegimeRanging
	pr.TrendStrength = clamp(pr.ADX, 0, 100)
	pr.Description = fmt.Sprintf("ADX %.1f — no strong directional signal", pr.ADX)
	return pr
}

// ---------------------------------------------------------------------------
// ADX Approximation
// ---------------------------------------------------------------------------

// approximateADX computes a simplified Average Directional Index from weekly OHLC.
// Uses up to 14 periods (standard). Prices must be newest-first.
//
// Steps:
//  1. Compute +DM and -DM from successive bars.
//  2. Smooth +DM, -DM, and TR over the look-back period (Wilder smoothing).
//  3. Compute +DI and -DI.
//  4. Compute DX, then smooth DX to get ADX.
func approximateADX(prices []domain.PriceRecord) float64 {
	period := 14
	// Need at least period+1 bars to compute 'period' DM values
	if len(prices) < period+1 {
		// Use whatever we have if at least 3 bars, but cap period to avoid degenerate results
		if len(prices) < 3 {
			return 0
		}
		period = len(prices) - 1
		// With very few bars (period < 5), the ADX approximation is unreliable;
		// return 0 to avoid degenerate 100.0 outputs.
		if period < 5 {
			return 0
		}
	}

	// Prices are newest-first; we iterate from oldest to newest for the calculation.
	// Build oldest-first slice of indices: prices[n-1] is oldest, prices[0] is newest.
	n := period + 1 // number of bars we use
	// reverse index helper: bar i (oldest-first) = prices[n-1-i]
	bar := func(i int) domain.PriceRecord { return prices[n-1-i] }

	// First pass: compute raw +DM, -DM, TR for bars 1..period
	plusDM := make([]float64, period)
	minusDM := make([]float64, period)
	trSlice := make([]float64, period)

	for i := 1; i <= period; i++ {
		curr := bar(i)
		prev := bar(i - 1)

		upMove := curr.High - prev.High
		downMove := prev.Low - curr.Low

		if upMove > downMove && upMove > 0 {
			plusDM[i-1] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i-1] = downMove
		}

		trSlice[i-1] = trueRange(curr, prev.Close)
	}

	// Wilder smoothing: first value is sum of first 'period' values
	smoothPlusDM := sum(plusDM)
	smoothMinusDM := sum(minusDM)
	smoothTR := sum(trSlice)

	if smoothTR == 0 {
		return 0
	}

	plusDI := (smoothPlusDM / smoothTR) * 100
	minusDI := (smoothMinusDM / smoothTR) * 100

	diSum := plusDI + minusDI
	if diSum == 0 {
		return 0
	}

	dx := math.Abs(plusDI-minusDI) / diSum * 100

	// With only one smoothing period we return DX as the ADX approximation.
	// For a true ADX we would need more bars to smooth DX over another 'period' bars,
	// but this is sufficient for regime classification.
	return roundN(dx, 2)
}

// ---------------------------------------------------------------------------
// Crisis Detection
// ---------------------------------------------------------------------------

// isCrisis returns true if the market shows crisis-level volatility.
// Criteria:
//   - NormalizedATR > 2x the average weekly range over the look-back, OR
//   - Latest weekly range > 3x the average weekly range.
func isCrisis(prices []domain.PriceRecord, pc *domain.PriceContext) bool {
	if pc != nil && pc.NormalizedATR > 0 {
		avgRange := averageWeeklyRange(prices)
		if avgRange > 0 && pc.NormalizedATR > avgRange*2 {
			return true
		}
	}

	if len(prices) < 5 {
		return false
	}

	latestRange := prices[0].WeeklyRange()
	avgRange := averageWeeklyRange(prices[1:])
	if avgRange > 0 && latestRange > avgRange*3 {
		return true
	}
	return false
}

// averageWeeklyRange computes the mean weekly range across all provided records.
func averageWeeklyRange(prices []domain.PriceRecord) float64 {
	if len(prices) == 0 {
		return 0
	}
	var total float64
	for i := range prices {
		total += prices[i].WeeklyRange()
	}
	return total / float64(len(prices))
}

// ---------------------------------------------------------------------------
// Description Helpers
// ---------------------------------------------------------------------------

func describeCrisis(pc *domain.PriceContext) string {
	if pc != nil {
		return fmt.Sprintf("CRISIS — NormalizedATR %.2f%%, volatility regime %s; extreme range expansion",
			pc.NormalizedATR, pc.VolatilityRegime)
	}
	return "CRISIS — weekly range far exceeds historical average"
}

func describeTrending(pc *domain.PriceContext, adx float64) string {
	dir := "directional"
	if pc != nil {
		switch pc.Trend4W {
		case "UP":
			dir = "bullish"
		case "DOWN":
			dir = "bearish"
		}
	}
	return fmt.Sprintf("TRENDING %s — ADX %.1f", dir, adx)
}

func describeRanging(pc *domain.PriceContext, adx float64) string {
	return fmt.Sprintf("RANGING — ADX %.1f, price oscillating around moving averages", adx)
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func sum(vals []float64) float64 {
	var s float64
	for _, v := range vals {
		s += v
	}
	return s
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
