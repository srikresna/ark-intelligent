// Package orderflow provides estimated delta and order flow analysis from OHLCV bars.
package orderflow

import (
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// MaxBars is the maximum number of bars retained in OrderFlowResult.DeltaBars.
const MaxBars = 20

// Analyze runs the full order flow analysis on the provided bars.
//
// bars must be newest-first (index 0 = most recent).
// symbol and timeframe are informational labels only.
// Returns nil if bars has fewer than 3 elements.
func Analyze(bars []ta.OHLCV, symbol, timeframe string) *OrderFlowResult {
	if len(bars) < 3 {
		return nil
	}

	// Trim to MaxBars for analysis and display.
	if len(bars) > MaxBars {
		bars = bars[:MaxBars]
	}

	// 1. Annotate bars with estimated delta.
	deltaBars := estimateDeltaBars(bars)

	// 2. Point of Control.
	poc := pointOfControl(bars)

	// 3. Absorption patterns.
	bullAbs, bearAbs := detectAbsorption(deltaBars)

	// 4. Delta divergence.
	divergence := detectDivergence(deltaBars)

	// 5. Delta trend (compare oldest CumDelta vs newest CumDelta).
	deltatrend := deltaTrend(deltaBars)

	// 6. Overall cumulative delta (most recent bar, index 0).
	cumDelta := deltaBars[0].CumDelta

	// 7. Bias synthesis.
	bias := synthBias(divergence, deltatrend, deltaBars)

	// 8. Summary.
	summary := buildSummary(bias, divergence, bullAbs, bearAbs, poc, deltaBars)

	return &OrderFlowResult{
		Symbol:               symbol,
		Timeframe:            timeframe,
		DeltaBars:            deltaBars,
		PriceDeltaDivergence: divergence,
		PointOfControl:       poc,
		BullishAbsorption:    bullAbs,
		BearishAbsorption:    bearAbs,
		DeltaTrend:           deltatrend,
		CumDelta:             cumDelta,
		Bias:                 bias,
		Summary:              summary,
		AnalyzedAt:           time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// detectDivergence compares price extreme vs cumulative delta extreme
// over the full bar window to detect hidden divergences.
//
// Bearish divergence: price makes a higher close than any previous bar,
// but cumulative delta at that bar is lower than at the previous high.
//
// Bullish divergence: price makes a lower close, but cumulative delta is higher.
func detectDivergence(bars []DeltaBar) string {
	n := len(bars)
	if n < 4 {
		return "NONE"
	}

	// bars[0] = newest, bars[n-1] = oldest.
	// Compare the newest bar to the second half of the window.
	newestClose := bars[0].OHLCV.Close
	newestCum := bars[0].CumDelta

	// Find the price/delta values of the "previous" swing (use bars[n/2 .. n-1]).
	mid := n / 2
	if mid < 2 {
		mid = 2
	}

	var priorHighClose, priorLowClose float64
	var priorHighCum, priorLowCum float64
	initialized := false

	for i := mid; i < n; i++ {
		c := bars[i].OHLCV.Close
		cd := bars[i].CumDelta
		if !initialized {
			priorHighClose = c
			priorLowClose = c
			priorHighCum = cd
			priorLowCum = cd
			initialized = true
			continue
		}
		if c > priorHighClose {
			priorHighClose = c
			priorHighCum = cd
		}
		if c < priorLowClose {
			priorLowClose = c
			priorLowCum = cd
		}
	}

	if !initialized {
		return "NONE"
	}

	// Bearish divergence: price higher high but delta lower high.
	if newestClose > priorHighClose && newestCum < priorHighCum {
		return "BEARISH_DIV"
	}
	// Bullish divergence: price lower low but delta higher low.
	if newestClose < priorLowClose && newestCum > priorLowCum {
		return "BULLISH_DIV"
	}
	return "NONE"
}

// deltaTrend compares the cumulative delta of the newest bar (bars[0]) against
// the oldest bar (bars[n-1]) to determine whether buying pressure is rising or
// falling over the window.
func deltaTrend(bars []DeltaBar) string {
	if len(bars) < 2 {
		return "FLAT"
	}
	newest := bars[0].CumDelta
	oldest := bars[len(bars)-1].CumDelta
	// Use 5% of the oldest absolute value as noise floor.
	threshold := oldest * 0.05
	if threshold < 0 {
		threshold = -threshold
	}
	if threshold == 0 {
		threshold = 1
	}
	diff := newest - oldest
	if diff > threshold {
		return "RISING"
	} else if diff < -threshold {
		return "FALLING"
	}
	return "FLAT"
}

// synthBias combines divergence, delta trend, and the current delta value
// into a single directional bias.
func synthBias(divergence, trend string, bars []DeltaBar) string {
	if len(bars) == 0 {
		return "NEUTRAL"
	}
	switch divergence {
	case "BULLISH_DIV":
		return "BULLISH"
	case "BEARISH_DIV":
		return "BEARISH"
	}
	switch trend {
	case "RISING":
		return "BULLISH"
	case "FALLING":
		return "BEARISH"
	}
	// Fallback: sign of latest cumulative delta.
	if bars[0].CumDelta > 0 {
		return "BULLISH"
	} else if bars[0].CumDelta < 0 {
		return "BEARISH"
	}
	return "NEUTRAL"
}

// buildSummary produces a compact human-readable summary.
func buildSummary(bias, divergence string, bullAbs, bearAbs []int, poc float64, bars []DeltaBar) string {
	switch {
	case divergence == "BULLISH_DIV":
		return fmt.Sprintf("Delta divergence bullish: price lower low tapi delta higher low. Potential reversal up. POC %.5g", poc)
	case divergence == "BEARISH_DIV":
		return fmt.Sprintf("Delta divergence bearish: price higher high tapi delta lower high. Potential reversal down. POC %.5g", poc)
	case len(bullAbs) > 0:
		return fmt.Sprintf("Bullish absorption terdeteksi (%d bar). Sellers tidak mampu push harga lebih rendah. Bias: %s", len(bullAbs), bias)
	case len(bearAbs) > 0:
		return fmt.Sprintf("Bearish absorption terdeteksi (%d bar). Buyers tidak mampu push harga lebih tinggi. Bias: %s", len(bearAbs), bias)
	}
	if len(bars) > 0 {
		return fmt.Sprintf("Cum. delta %+.0f, trend: %s, bias: %s. POC %.5g", bars[0].CumDelta, synthTrendLabel(bars), bias, poc)
	}
	return "Tidak cukup data untuk analisis order flow."
}

func synthTrendLabel(bars []DeltaBar) string {
	return deltaTrend(bars)
}
