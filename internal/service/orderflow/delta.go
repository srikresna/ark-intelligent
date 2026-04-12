package orderflow

import (
	"math"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// estimateDeltaBars annotates each bar with estimated buy/sell volume using the
// tick-rule OHLCV approximation.
//
// bars must be newest-first (index 0 = most recent bar).
// Returns nil if bars is empty.
func estimateDeltaBars(bars []ta.OHLCV) []DeltaBar {
	if len(bars) == 0 {
		return nil
	}

	out := make([]DeltaBar, len(bars))
	for i, bar := range bars {
		bv, sv := tickRuleSplit(bar)
		out[i] = DeltaBar{
			OHLCV:   bar,
			BuyVol:  bv,
			SellVol: sv,
			Delta:   bv - sv,
		}
	}

	// Compute cumulative delta oldest-first, then assign newest-first.
	// bars[0] = newest, bars[n-1] = oldest.
	// We accumulate from oldest (bars[n-1]) toward newest (bars[0]).
	n := len(out)
	running := 0.0
	for i := n - 1; i >= 0; i-- {
		running += out[i].Delta
		out[i].CumDelta = running
	}
	// After the loop, out[0].CumDelta is the grand cumulative sum (current).

	return out
}

// tickRuleSplit splits bar volume into estimated buy and sell portions.
//
// Bullish bar (Close >= Open):
//
//	BuyVol  = Volume × (Close − Low)  / Range
//	SellVol = Volume × (High − Close) / Range
//
// Bearish bar (Close < Open):
//
//	SellVol = Volume × (High − Close) / Range
//	BuyVol  = Volume × (Close − Low)  / Range
//
// (The formula is actually symmetric — direction doesn't change the formula but
// affects how we interpret the result via Delta.)
// Zero-range bar: 50/50 split.
func tickRuleSplit(bar ta.OHLCV) (buyVol, sellVol float64) {
	vol := bar.Volume
	if vol <= 0 {
		return 0, 0
	}
	rng := bar.High - bar.Low
	if rng <= 0 || math.IsNaN(rng) || math.IsInf(rng, 0) {
		return vol * 0.5, vol * 0.5
	}

	buyFrac := (bar.Close - bar.Low) / rng
	// Clamp to [0,1] to handle floating-point edge cases.
	if buyFrac < 0 {
		buyFrac = 0
	} else if buyFrac > 1 {
		buyFrac = 1
	}
	buyVol = vol * buyFrac
	sellVol = vol * (1 - buyFrac)
	return buyVol, sellVol
}
