package ict

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DetectLiquiditySweeps finds candles that pierce through a prior swing
// high/low (liquidity grab) and then close back inside, suggesting a reversal.
//
// Criteria for a sweep:
//   SWEEP_HIGH: bar's High > previous swing high, but Close < that swing high.
//   SWEEP_LOW:  bar's Low  < previous swing low,  but Close > that swing low.
//   Reversed=true if the following candle confirms (close in the opposite direction).
func DetectLiquiditySweeps(bars []ta.OHLCV, swings []swingPoint) []LiquiditySweep {
	n := len(bars)
	if n < swingLookback*2+2 || len(swings) < 2 {
		return nil
	}

	// Work on a chronological copy.
	chron := make([]ta.OHLCV, n)
	for i, b := range bars {
		chron[n-1-i] = b
	}

	var sweeps []LiquiditySweep

	// Build a lookup of swing levels indexed by barIndex for quick access.
	for _, sp := range swings {
		// Look at bars that come AFTER this swing point.
		for i := sp.barIndex + 1; i < n; i++ {
			bar := chron[i]

			if sp.isHigh {
				// Check if bar's wick goes above the swing high but closes below.
				if bar.High > sp.level && bar.Close < sp.level {
					reversed := false
					if i+1 < n {
						next := chron[i+1]
						reversed = next.Close < next.Open // bearish follow-through after high sweep
					}
					sweeps = append(sweeps, LiquiditySweep{
						Kind:      "SWEEP_HIGH",
						Level:     sp.level,
						SweepHigh: bar.High,
						SweepLow:  bar.Low,
						BarIndex:  i,
						Reversed:  reversed,
					})
					break // only first sweep per swing level
				}
			} else {
				// Check if bar's wick goes below the swing low but closes above.
				if bar.Low < sp.level && bar.Close > sp.level {
					reversed := false
					if i+1 < n {
						next := chron[i+1]
						reversed = next.Close > next.Open // bullish follow-through after low sweep
					}
					sweeps = append(sweeps, LiquiditySweep{
						Kind:      "SWEEP_LOW",
						Level:     sp.level,
						SweepHigh: bar.High,
						SweepLow:  bar.Low,
						BarIndex:  i,
						Reversed:  reversed,
					})
					break // only first sweep per swing level
				}
			}
		}
	}

	// Return only the most recent 4 sweeps.
	if len(sweeps) > 4 {
		sweeps = sweeps[len(sweeps)-4:]
	}
	return sweeps
}
