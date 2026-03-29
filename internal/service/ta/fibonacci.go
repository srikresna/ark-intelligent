package ta

import "math"

// ---------------------------------------------------------------------------
// Auto Fibonacci Retracement
// ---------------------------------------------------------------------------

// FibResult holds the auto-detected Fibonacci retracement levels.
//
// Swing detection uses a 5-bar pivot: a bar is a swing high if its High is greater
// than the High of the 5 bars before AND 5 bars after it. Similarly for swing low.
//
// Levels are keyed by percentage string: "0", "23.6", "38.2", "50", "61.8", "78.6", "100".
// In an uptrend: 0% = swing low, 100% = swing high (retracement from high).
// In a downtrend: 0% = swing high, 100% = swing low (retracement from low).
type FibResult struct {
	SwingHigh    float64            // Detected swing high price
	SwingLow     float64            // Detected swing low price
	SwingHighIdx int                // Index of swing high bar (in original newest-first input)
	SwingLowIdx  int                // Index of swing low bar (in original newest-first input)
	Levels       map[string]float64 // Fibonacci levels {"0": ..., "23.6": ..., etc.}
	TrendDir     string             // "UP" or "DOWN"
	NearestLevel string             // Key of the Fibonacci level nearest to current price
	NearestPrice float64            // Value of the nearest Fibonacci level
}

// CalcFibonacci computes auto Fibonacci retracement levels.
// Bars are newest-first. lookback is the number of bars to scan (default 50 if <= 0).
//
// Returns nil if no valid swing pair is found within the lookback window.
func CalcFibonacci(bars []OHLCV, lookback int) *FibResult {
	if lookback <= 0 {
		lookback = 50
	}
	if len(bars) < 11 { // Need at least 5+1+5 = 11 bars for any pivot
		return nil
	}
	if lookback > len(bars) {
		lookback = len(bars)
	}

	const pivotStrength = 5 // number of bars on each side

	// Work oldest-first for the lookback window
	window := bars[:lookback]
	asc := reverseOHLCV(window)
	n := len(asc)

	// Detect swing highs and lows
	type pivot struct {
		idx   int     // index in asc (oldest-first)
		price float64 // High for swing high, Low for swing low
	}

	var swingHighs []pivot
	var swingLows []pivot

	for i := pivotStrength; i < n-pivotStrength; i++ {
		// Swing High: High[i] > High of 5 bars before AND 5 bars after
		isSwingHigh := true
		for j := i - pivotStrength; j < i; j++ {
			if asc[j].High >= asc[i].High {
				isSwingHigh = false
				break
			}
		}
		if isSwingHigh {
			for j := i + 1; j <= i+pivotStrength; j++ {
				if asc[j].High >= asc[i].High {
					isSwingHigh = false
					break
				}
			}
		}
		if isSwingHigh {
			swingHighs = append(swingHighs, pivot{idx: i, price: asc[i].High})
		}

		// Swing Low: Low[i] < Low of 5 bars before AND 5 bars after
		isSwingLow := true
		for j := i - pivotStrength; j < i; j++ {
			if asc[j].Low <= asc[i].Low {
				isSwingLow = false
				break
			}
		}
		if isSwingLow {
			for j := i + 1; j <= i+pivotStrength; j++ {
				if asc[j].Low <= asc[i].Low {
					isSwingLow = false
					break
				}
			}
		}
		if isSwingLow {
			swingLows = append(swingLows, pivot{idx: i, price: asc[i].Low})
		}
	}

	if len(swingHighs) == 0 || len(swingLows) == 0 {
		return nil
	}

	// Use most recent swing high and swing low
	sh := swingHighs[len(swingHighs)-1]
	sl := swingLows[len(swingLows)-1]

	// Ensure they are a meaningful pair (not the same bar)
	if sh.idx == sl.idx {
		// Try to find alternate: use second most recent of whichever came later
		if sh.idx > sl.idx && len(swingHighs) > 1 {
			sh = swingHighs[len(swingHighs)-2]
		} else if sl.idx > sh.idx && len(swingLows) > 1 {
			sl = swingLows[len(swingLows)-2]
		} else {
			return nil
		}
	}

	swingHigh := sh.price
	swingLow := sl.price

	// Convert asc indices back to newest-first indices
	swingHighIdxNewest := lookback - 1 - sh.idx
	swingLowIdxNewest := lookback - 1 - sl.idx

	currentPrice := bars[0].Close

	// Determine trend direction based on temporal order of swings:
	// If the swing high occurred more recently than the swing low → UP trend
	// (the most recent major move was upward).
	// If the swing low occurred more recently → DOWN trend.
	trendDir := "UP"
	if sl.idx > sh.idx {
		// Swing low is more recent (higher asc index = more recent) → DOWN
		trendDir = "DOWN"
	}

	// Calculate Fibonacci levels
	diff := swingHigh - swingLow
	levels := make(map[string]float64)

	fibRatios := map[string]float64{
		"0":    0.0,
		"23.6": 0.236,
		"38.2": 0.382,
		"50":   0.500,
		"61.8": 0.618,
		"78.6": 0.786,
		"100":  1.000,
	}

	if trendDir == "UP" {
		// Uptrend: retracement from high to low
		// 0% = swing high (top), 100% = swing low (bottom)
		for key, ratio := range fibRatios {
			levels[key] = swingHigh - ratio*diff
		}
	} else {
		// Downtrend: retracement from low to high
		// 0% = swing low (bottom), 100% = swing high (top)
		for key, ratio := range fibRatios {
			levels[key] = swingLow + ratio*diff
		}
	}

	// Find nearest Fibonacci level to current price
	nearestLevel := ""
	nearestPrice := 0.0
	minDist := math.MaxFloat64

	for key, price := range levels {
		dist := math.Abs(currentPrice - price)
		if dist < minDist {
			minDist = dist
			nearestLevel = key
			nearestPrice = price
		}
	}

	return &FibResult{
		SwingHigh:    swingHigh,
		SwingLow:     swingLow,
		SwingHighIdx: swingHighIdxNewest,
		SwingLowIdx:  swingLowIdxNewest,
		Levels:       levels,
		TrendDir:     trendDir,
		NearestLevel: nearestLevel,
		NearestPrice: nearestPrice,
	}
}
