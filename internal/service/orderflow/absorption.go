package orderflow

import "math"

// detectAbsorption scans annotated bars and returns indices where absorption
// patterns are present.
//
// Bullish Absorption (index returned in bullish slice):
//   A bar has a large negative delta (sellers dominated) BUT its range is
//   compressed relative to the average range AND the bar did not close near
//   its low — indicating that buyers were absorbing the supply.
//
// Bearish Absorption (index returned in bearish slice):
//   A bar has a large positive delta (buyers dominated) BUT its range is
//   compressed AND the bar did not close near its high — sellers absorbing
//   demand.
//
// Thresholds use the median absolute delta and median range of the window.
func detectAbsorption(bars []DeltaBar) (bullish, bearish []int) {
	n := len(bars)
	if n < 3 {
		return nil, nil
	}

	// Compute median |delta| and median range as adaptive thresholds.
	medDelta := medianAbsDelta(bars)
	medRange := medianRange(bars)

	if medDelta == 0 || medRange == 0 {
		return nil, nil
	}

	// Volume threshold: bar must have above-average absolute delta to qualify.
	deltaThreshold := medDelta * 1.5

	for i, db := range bars {
		rng := db.OHLCV.High - db.OHLCV.Low
		if rng <= 0 {
			continue
		}
		// Range must be compressed (< 80% of median range).
		isCompressed := rng < medRange*0.8

		absDelta := math.Abs(db.Delta)
		if absDelta < deltaThreshold {
			continue
		}

		if db.Delta < 0 && isCompressed {
			// Heavy selling but narrow range → bullish absorption.
			// Additional check: close is not near the low.
			closeFrac := (db.OHLCV.Close - db.OHLCV.Low) / rng
			if closeFrac > 0.35 { // closed in upper 65% of range
				bullish = append(bullish, i)
			}
		} else if db.Delta > 0 && isCompressed {
			// Heavy buying but narrow range → bearish absorption.
			// Additional check: close is not near the high.
			closeFrac := (db.OHLCV.Close - db.OHLCV.Low) / rng
			if closeFrac < 0.65 { // closed in lower 65% of range
				bearish = append(bearish, i)
			}
		}
	}
	return bullish, bearish
}

// medianAbsDelta returns the median |delta| from the delta bar slice.
func medianAbsDelta(bars []DeltaBar) float64 {
	if len(bars) == 0 {
		return 0
	}
	vals := make([]float64, len(bars))
	for i, db := range bars {
		v := db.Delta
		if v < 0 {
			v = -v
		}
		vals[i] = v
	}
	return medianFloat(vals)
}

// medianRange returns the median High-Low range from the delta bar slice.
func medianRange(bars []DeltaBar) float64 {
	if len(bars) == 0 {
		return 0
	}
	vals := make([]float64, len(bars))
	for i, db := range bars {
		vals[i] = db.OHLCV.High - db.OHLCV.Low
	}
	return medianFloat(vals)
}

// medianFloat returns the median of a float64 slice without sorting in-place.
func medianFloat(vals []float64) float64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	// Copy and sort.
	cp := make([]float64, n)
	copy(cp, vals)
	// Simple insertion sort (small slices).
	for i := 1; i < n; i++ {
		key := cp[i]
		j := i - 1
		for j >= 0 && cp[j] > key {
			cp[j+1] = cp[j]
			j--
		}
		cp[j+1] = key
	}
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}
