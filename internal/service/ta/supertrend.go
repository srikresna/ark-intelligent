package ta

import "math"

// ---------------------------------------------------------------------------
// SuperTrend (period, multiplier)
// ---------------------------------------------------------------------------

// SuperTrendResult holds the SuperTrend indicator values.
//
// Ref: Olivier Seban's SuperTrend indicator.
// - ATR = ATR(period)
// - Basic Upper Band = (High + Low) / 2 + multiplier * ATR
// - Basic Lower Band = (High + Low) / 2 - multiplier * ATR
// - Final Upper = min(Basic Upper, prev Final Upper) if prev Close <= prev Final Upper, else Basic Upper
// - Final Lower = max(Basic Lower, prev Final Lower) if prev Close >= prev Final Lower, else Basic Lower
// - If prev SuperTrend == prev Final Upper and Close > current Final Upper → SuperTrend = Final Lower (flip to UP)
// - If prev SuperTrend == prev Final Lower and Close < current Final Lower → SuperTrend = Final Upper (flip to DOWN)
// - Otherwise maintain previous direction.
type SuperTrendResult struct {
	Value           float64   // Current SuperTrend value
	Direction       string    // "UP" (bullish) or "DOWN" (bearish)
	Series          []float64 // Full SuperTrend series (newest-first)
	DirectionSeries []string  // Full direction series (newest-first)
}

// calcATRSeries computes the True Range series and then ATR using Wilder's smoothing.
// Input: bars oldest-first. Output: ATR series oldest-first, same length as input.
// First (period-1) values are NaN. ATR[period-1] = simple average of first `period` TRs.
func calcATRSeries(asc []OHLCV, period int) []float64 {
	n := len(asc)
	if n < period+1 {
		return nil
	}

	tr := make([]float64, n)
	tr[0] = asc[0].High - asc[0].Low // first bar: just HL range

	for i := 1; i < n; i++ {
		hl := asc[i].High - asc[i].Low
		hpc := math.Abs(asc[i].High - asc[i-1].Close)
		lpc := math.Abs(asc[i].Low - asc[i-1].Close)
		tr[i] = math.Max(hl, math.Max(hpc, lpc))
	}

	atr := make([]float64, n)
	for i := 0; i < period; i++ {
		atr[i] = math.NaN()
	}

	// First ATR = simple average of TR[1..period] (we skip index 0 since it has no prev close)
	// Actually, standard ATR uses TR starting from index 1. But the first bar's TR = HL.
	// For SuperTrend, we use TR from index 0, seed ATR at index period-1.
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	atr[period-1] = sum / float64(period)

	// Wilder's smoothing: ATR[i] = (ATR[i-1] * (period-1) + TR[i]) / period
	for i := period; i < n; i++ {
		atr[i] = (atr[i-1]*float64(period-1) + tr[i]) / float64(period)
	}

	return atr
}

// CalcSuperTrend computes the SuperTrend indicator.
// Bars are newest-first. Typical parameters: period=10, multiplier=3.0.
//
// Returns nil if insufficient data.
func CalcSuperTrend(bars []OHLCV, period int, multiplier float64) *SuperTrendResult {
	if len(bars) < period+1 || period <= 0 {
		return nil
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	n := len(asc)

	atr := calcATRSeries(asc, period)
	if atr == nil {
		return nil
	}

	st := make([]float64, n)
	dir := make([]string, n) // "UP" or "DOWN"
	finalUpper := make([]float64, n)
	finalLower := make([]float64, n)

	// Initialize everything as NaN until we have enough ATR data
	for i := 0; i < period; i++ {
		st[i] = math.NaN()
		dir[i] = ""
		finalUpper[i] = math.NaN()
		finalLower[i] = math.NaN()
	}

	// First valid bar at index period-1 (where ATR first becomes available)
	// But we need at least one prior bar for comparison, so start calculations at period-1
	startIdx := period - 1

	// Initialize at startIdx
	hl2 := (asc[startIdx].High + asc[startIdx].Low) / 2
	basicUpper := hl2 + multiplier*atr[startIdx]
	basicLower := hl2 - multiplier*atr[startIdx]
	finalUpper[startIdx] = basicUpper
	finalLower[startIdx] = basicLower

	// Default direction: if close > basic upper → UP, else DOWN
	if asc[startIdx].Close > basicUpper {
		st[startIdx] = finalLower[startIdx]
		dir[startIdx] = "UP"
	} else {
		st[startIdx] = finalUpper[startIdx]
		dir[startIdx] = "DOWN"
	}

	for i := startIdx + 1; i < n; i++ {
		if math.IsNaN(atr[i]) {
			st[i] = math.NaN()
			dir[i] = ""
			continue
		}

		hl2 = (asc[i].High + asc[i].Low) / 2
		basicUpper = hl2 + multiplier*atr[i]
		basicLower = hl2 - multiplier*atr[i]

		// Final Upper Band
		if basicUpper < finalUpper[i-1] || asc[i-1].Close > finalUpper[i-1] {
			finalUpper[i] = basicUpper
		} else {
			finalUpper[i] = finalUpper[i-1]
		}

		// Final Lower Band
		if basicLower > finalLower[i-1] || asc[i-1].Close < finalLower[i-1] {
			finalLower[i] = basicLower
		} else {
			finalLower[i] = finalLower[i-1]
		}

		// Determine SuperTrend value and direction
		prevST := st[i-1]
		if math.IsNaN(prevST) {
			// Shouldn't happen after startIdx, but be safe
			if asc[i].Close > finalUpper[i] {
				st[i] = finalLower[i]
				dir[i] = "UP"
			} else {
				st[i] = finalUpper[i]
				dir[i] = "DOWN"
			}
			continue
		}

		if prevST == finalUpper[i-1] {
			// Previous was in DOWN mode (ST was tracking upper band)
			if asc[i].Close > finalUpper[i] {
				// Price broke above upper band → flip to UP
				st[i] = finalLower[i]
				dir[i] = "UP"
			} else {
				// Stay DOWN
				st[i] = finalUpper[i]
				dir[i] = "DOWN"
			}
		} else {
			// Previous was in UP mode (ST was tracking lower band)
			if asc[i].Close < finalLower[i] {
				// Price broke below lower band → flip to DOWN
				st[i] = finalUpper[i]
				dir[i] = "DOWN"
			} else {
				// Stay UP
				st[i] = finalLower[i]
				dir[i] = "UP"
			}
		}
	}

	// Reverse to newest-first
	stRev := reverseFloat64(st)
	dirRev := make([]string, n)
	for i, d := range dir {
		dirRev[n-1-i] = d
	}

	// Find latest valid values
	latestValue := math.NaN()
	latestDir := ""
	for i := 0; i < n; i++ {
		if !math.IsNaN(stRev[i]) && dirRev[i] != "" {
			latestValue = stRev[i]
			latestDir = dirRev[i]
			break
		}
	}

	if math.IsNaN(latestValue) {
		return nil
	}

	return &SuperTrendResult{
		Value:           latestValue,
		Direction:       latestDir,
		Series:          stRev,
		DirectionSeries: dirRev,
	}
}
