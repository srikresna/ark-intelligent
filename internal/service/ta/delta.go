package ta

import (
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// Estimated Delta — Tick Rule Buy/Sell Pressure Estimation
// ---------------------------------------------------------------------------

// DeltaResult holds the output of tick-rule delta estimation.
//
// Estimated Delta uses close-to-close direction to proxy buy/sell volume:
//   - Close[i] > Close[i-1] → buy pressure (+Volume)
//   - Close[i] < Close[i-1] → sell pressure (-Volume)
//   - Close[i] == Close[i-1] → neutral (0)
//
// Cumulative Delta is the running sum: sustained positive = bullish pressure,
// sustained negative = bearish pressure.
type DeltaResult struct {
	// Current bar delta (positive = net buying, negative = net selling)
	CurrentDelta float64

	// CumulativeDelta: running sum over the bars window (newest-first[0] is latest)
	CumulativeDelta float64

	// DeltaDivergence: price direction disagrees with delta direction
	// e.g. price up but cumulative delta down = bearish divergence
	DeltaDivergence string // "BEARISH_DIVERGENCE", "BULLISH_DIVERGENCE", "NONE"

	// Bias based on cumulative delta relative to recent max/min range
	Bias string // "BUYING_PRESSURE", "SELLING_PRESSURE", "NEUTRAL"

	// BiasStrength: 0.0–1.0 (how far cumulative delta is from zero relative to range)
	BiasStrength float64

	// BarsUsed: number of bars used in calculation
	BarsUsed int

	// Series: cumulative delta series, newest-first
	Series []float64
}

// String returns a compact human-readable summary.
func (d *DeltaResult) String() string {
	if d == nil {
		return ""
	}
	return fmt.Sprintf("CumDelta: %.0f (%s, strength=%.2f)%s",
		d.CumulativeDelta, d.Bias, d.BiasStrength,
		func() string {
			if d.DeltaDivergence != "NONE" {
				return " ⚠ " + d.DeltaDivergence
			}
			return ""
		}())
}

// ---------------------------------------------------------------------------
// CalcDelta — tick rule cumulative delta from OHLCV bars
// ---------------------------------------------------------------------------

// CalcDelta computes estimated buy/sell pressure using the tick rule.
// bars must be newest-first (index 0 = most recent bar).
// Returns nil if fewer than 2 bars.
func CalcDelta(bars []OHLCV) *DeltaResult {
	if len(bars) < 2 {
		return nil
	}

	n := len(bars)

	// Build oldest-first delta series.
	// bars[0] is most recent; bars[n-1] is oldest.
	// We compare bars[i] vs bars[i+1] (in newest-first order: [i] is newer, [i+1] is older).
	deltas := make([]float64, n-1)
	for i := 0; i < n-1; i++ {
		vol := bars[i].Volume
		if vol <= 0 {
			vol = 1.0 // equal-weight fallback
		}
		switch {
		case bars[i].Close > bars[i+1].Close:
			deltas[i] = +vol // buying pressure
		case bars[i].Close < bars[i+1].Close:
			deltas[i] = -vol // selling pressure
		default:
			deltas[i] = 0
		}
	}

	// Cumulative delta series (newest-first).
	// series[0] = sum of all bars' deltas (most recent cumulative)
	series := make([]float64, len(deltas))
	// Compute oldest-first cumsum, then reverse to newest-first.
	cum := make([]float64, len(deltas))
	running := 0.0
	for i := len(deltas) - 1; i >= 0; i-- {
		running += deltas[i]
		cum[i] = running
	}
	// cum[0] is cumulative sum starting from bar[n-1] up to bar[1].
	// Reverse so series[0] is the current cumulative delta.
	for i := range series {
		series[i] = cum[i]
	}

	cumulativeDelta := series[0]
	currentDelta := deltas[0]

	// Bias: compare cumulative delta to its range over the window.
	maxCum := series[0]
	minCum := series[0]
	for _, v := range series {
		if v > maxCum {
			maxCum = v
		}
		if v < minCum {
			minCum = v
		}
	}

	rangeAbs := maxCum - minCum
	biasStrength := 0.0
	if rangeAbs > 0 {
		biasStrength = math.Abs(cumulativeDelta-((maxCum+minCum)/2)) / (rangeAbs / 2)
		if biasStrength > 1 {
			biasStrength = 1
		}
	}

	bias := "NEUTRAL"
	if cumulativeDelta > 0 && biasStrength > 0.2 {
		bias = "BUYING_PRESSURE"
	} else if cumulativeDelta < 0 && biasStrength > 0.2 {
		bias = "SELLING_PRESSURE"
	}

	// Delta divergence: compare price direction vs cumulative delta direction
	// over the last N bars (use first 10 bars or all if fewer).
	lookback := 10
	if len(series) < lookback {
		lookback = len(series)
	}

	divergence := "NONE"
	if lookback >= 2 {
		// Price change: bars[0].Close vs bars[lookback].Close (newest vs older)
		priceStart := bars[lookback].Close
		priceEnd := bars[0].Close
		deltaStart := series[lookback-1]
		deltaEnd := series[0]

		priceUp := priceEnd > priceStart
		deltaUp := deltaEnd > deltaStart

		if priceUp && !deltaUp {
			divergence = "BEARISH_DIVERGENCE"
		} else if !priceUp && deltaUp {
			divergence = "BULLISH_DIVERGENCE"
		}
	}

	return &DeltaResult{
		CurrentDelta:    currentDelta,
		CumulativeDelta: cumulativeDelta,
		DeltaDivergence: divergence,
		Bias:            bias,
		BiasStrength:    biasStrength,
		BarsUsed:        n,
		Series:          series,
	}
}
