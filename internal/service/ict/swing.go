package ict

import (
	"math"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// swingLookback is the number of bars on each side used to confirm a swing point.
const swingLookback = 5

// detectSwings identifies swing highs and lows in the bar slice.
// The input slice is newest-first (index 0 = most recent bar) — we reverse
// internally so that index 0 = oldest bar for sequential logic.
// Only bars [lookback .. len-lookback-1] can be swing candidates (need bars on both sides).
// Returns chronological (oldest-first) slice of swingPoint.
func detectSwings(bars []ta.OHLCV) []swingPoint {
	n := len(bars)
	if n < 2*swingLookback+1 {
		return nil
	}

	// Work on a chronological copy (oldest first).
	chron := make([]ta.OHLCV, n)
	for i, b := range bars {
		chron[n-1-i] = b
	}

	var swings []swingPoint

	for i := swingLookback; i < n-swingLookback; i++ {
		// Swing High: bar[i].High is the highest in the window.
		isSwingHigh := true
		for j := i - swingLookback; j <= i+swingLookback; j++ {
			if j == i {
				continue
			}
			if chron[j].High >= chron[i].High {
				isSwingHigh = false
				break
			}
		}
		if isSwingHigh {
			swings = append(swings, swingPoint{isHigh: true, level: chron[i].High, barIndex: i})
		}

		// Swing Low: bar[i].Low is the lowest in the window.
		isSwingLow := true
		for j := i - swingLookback; j <= i+swingLowback(); j++ {
			if j == i {
				continue
			}
			if chron[j].Low <= chron[i].Low {
				isSwingLow = false
				break
			}
		}
		if isSwingLow {
			swings = append(swings, swingPoint{isHigh: false, level: chron[i].Low, barIndex: i})
		}
	}

	return swings
}

func swingLowback() int { return swingLookback }

// maxFloat returns the maximum value in a slice.
func maxFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return math.Inf(-1)
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// minFloat returns the minimum value in a slice.
func minFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return math.Inf(1)
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// abs64 returns the absolute value of a float64.
func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
