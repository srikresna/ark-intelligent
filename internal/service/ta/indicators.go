package ta

import (
	"math"
	"sort"
)

// ---------------------------------------------------------------------------
// Helper: reverse / SMA
// ---------------------------------------------------------------------------

// reverseOHLCV returns a copy of bars in reversed order.
// Used internally to convert newest-first → oldest-first for sequential calc.
func reverseOHLCV(bars []OHLCV) []OHLCV {
	n := len(bars)
	out := make([]OHLCV, n)
	for i, b := range bars {
		out[n-1-i] = b
	}
	return out
}

// reverseFloat64 returns a copy of the slice in reversed order.
func reverseFloat64(s []float64) []float64 {
	n := len(s)
	out := make([]float64, n)
	for i, v := range s {
		out[n-1-i] = v
	}
	return out
}

// CalcSMA computes a Simple Moving Average over the given values (same order as input).
// It returns a slice of SMA values; the first (period-1) entries are NaN.
// Input and output order is preserved (caller decides newest-first vs oldest-first).
//
// Formula: SMA[i] = sum(values[i-period+1 .. i]) / period
func CalcSMA(values []float64, period int) []float64 {
	n := len(values)
	if n < period || period <= 0 {
		return nil
	}
	out := make([]float64, n)
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += values[i]
		if i >= period {
			sum -= values[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// EMA
// ---------------------------------------------------------------------------

// CalcEMA computes an Exponential Moving Average series.
// Input: closes (newest-first). Output: EMA series (newest-first).
// Seed: EMA[0] (oldest) = SMA of first `period` bars.
// Subsequent: EMA[i] = (Close[i] - EMA[i-1]) * multiplier + EMA[i-1]
// Multiplier = 2 / (period + 1).
//
// Ref: Murphy, "Technical Analysis of the Financial Markets", EMA definition.
func CalcEMA(closes []float64, period int) []float64 {
	n := len(closes)
	if n < period || period <= 0 {
		return nil
	}

	// Work oldest-first
	asc := reverseFloat64(closes)

	mult := 2.0 / (float64(period) + 1.0)
	ema := make([]float64, n)

	// Seed: SMA of first `period` values
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += asc[i]
	}
	ema[period-1] = sum / float64(period)

	// Fill NaN for indices before seed
	for i := 0; i < period-1; i++ {
		ema[i] = math.NaN()
	}

	// EMA forward
	for i := period; i < n; i++ {
		ema[i] = (asc[i]-ema[i-1])*mult + ema[i-1]
	}

	return reverseFloat64(ema)
}

// ---------------------------------------------------------------------------
// RSI — Wilder's Smoothing
// ---------------------------------------------------------------------------

// CalcRSI computes the Relative Strength Index for the most recent bar.
// Uses Wilder's smoothing: first avg = simple average, subsequent use
// smoothing factor (period-1)/period.
//
// Ref: Wilder, "New Concepts in Technical Trading Systems" (1978).
// RSI = 100 - 100/(1 + RS), RS = avg_gain / avg_loss
func CalcRSI(bars []OHLCV, period int) *RSIResult {
	series := CalcRSISeries(bars, period)
	if len(series) == 0 {
		return nil
	}
	val := series[0] // newest

	zone := "NEUTRAL"
	if val > 70 {
		zone = "OVERBOUGHT"
	} else if val < 30 {
		zone = "OVERSOLD"
	}

	trend := "FLAT"
	if len(series) >= 3 {
		// Compare last 3 RSI values (newest-first: [0], [1], [2])
		if series[0] > series[1] && series[1] > series[2] {
			trend = "RISING"
		} else if series[0] < series[1] && series[1] < series[2] {
			trend = "FALLING"
		}
	}

	return &RSIResult{Value: val, Zone: zone, Trend: trend}
}

// CalcRSISeries computes the full RSI series (newest-first).
// Returns one RSI value per bar starting from bar index (period) in oldest-first order.
// The output length = len(bars) - period. Output is newest-first.
//
// Ref: Wilder's smoothing — first average uses simple average of first `period`
// changes; subsequent use: avg = (prev_avg * (period-1) + current) / period.
func CalcRSISeries(bars []OHLCV, period int) []float64 {
	if len(bars) < period+1 || period <= 0 {
		return nil
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	n := len(asc)

	gains := make([]float64, n)
	losses := make([]float64, n)
	for i := 1; i < n; i++ {
		change := asc[i].Close - asc[i-1].Close
		if change > 0 {
			gains[i] = change
		} else {
			losses[i] = -change
		}
	}

	// First averages (simple average of first `period` changes, indices 1..period)
	sumGain := 0.0
	sumLoss := 0.0
	for i := 1; i <= period; i++ {
		sumGain += gains[i]
		sumLoss += losses[i]
	}
	avgGain := sumGain / float64(period)
	avgLoss := sumLoss / float64(period)

	rsiValues := make([]float64, 0, n-period)
	rs := 0.0
	if avgLoss == 0 {
		if avgGain == 0 {
			rsiValues = append(rsiValues, 50) // no movement
		} else {
			rsiValues = append(rsiValues, 100)
		}
	} else {
		rs = avgGain / avgLoss
		rsiValues = append(rsiValues, 100-100/(1+rs))
	}

	// Subsequent values using Wilder's smoothing
	for i := period + 1; i < n; i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
		if avgLoss == 0 {
			if avgGain == 0 {
				rsiValues = append(rsiValues, 50)
			} else {
				rsiValues = append(rsiValues, 100)
			}
		} else {
			rs = avgGain / avgLoss
			rsiValues = append(rsiValues, 100-100/(1+rs))
		}
	}

	return reverseFloat64(rsiValues)
}

// ---------------------------------------------------------------------------
// MACD
// ---------------------------------------------------------------------------

// CalcMACD computes the MACD indicator for the most recent bar.
// MACD Line = EMA(fast) - EMA(slow), Signal = EMA(signal) of MACD line,
// Histogram = MACD - Signal. Cross detection compares the two most recent bars.
//
// Ref: Gerald Appel's MACD. EMA multiplier = 2/(period+1).
func CalcMACD(bars []OHLCV, fast, slow, signal int) *MACDResult {
	macdLine, signalLine, histogram := CalcMACDSeries(bars, fast, slow, signal)
	if len(macdLine) == 0 || len(signalLine) == 0 {
		return nil
	}

	// Find first two valid (non-NaN) entries (newest-first)
	m0, s0, h0 := macdLine[0], signalLine[0], histogram[0]
	if math.IsNaN(m0) || math.IsNaN(s0) {
		return nil
	}

	cross := "NONE"
	bullish := false
	bearish := false

	// Check crossover: need at least 2 valid points
	if len(macdLine) >= 2 && !math.IsNaN(macdLine[1]) && !math.IsNaN(signalLine[1]) {
		prevM, prevS := macdLine[1], signalLine[1]
		if prevM <= prevS && m0 > s0 {
			cross = "BULLISH_CROSS"
			bullish = true
		} else if prevM >= prevS && m0 < s0 {
			cross = "BEARISH_CROSS"
			bearish = true
		}
	}

	return &MACDResult{
		MACD:         m0,
		Signal:       s0,
		Histogram:    h0,
		Cross:        cross,
		BullishCross: bullish,
		BearishCross: bearish,
	}
}

// CalcMACDSeries computes the full MACD, Signal, and Histogram series (newest-first).
// The MACD line requires `slow` bars of close data. The signal line requires an
// additional `signal-1` MACD values after the slow EMA is seeded.
//
// Ref: MACD Line = EMA(fast) - EMA(slow); Signal = EMA(signal) of MACD Line.
func CalcMACDSeries(bars []OHLCV, fast, slow, signal int) (macdLine, signalLine, histogram []float64) {
	if len(bars) < slow || fast <= 0 || slow <= 0 || signal <= 0 || fast >= slow {
		return nil, nil, nil
	}

	closes := make([]float64, len(bars))
	for i, b := range bars {
		closes[i] = b.Close
	}

	emaFast := CalcEMA(closes, fast)
	emaSlow := CalcEMA(closes, slow)
	if emaFast == nil || emaSlow == nil {
		return nil, nil, nil
	}

	n := len(bars)
	// MACD line (newest-first): emaFast - emaSlow, NaN where either is NaN
	macdRaw := make([]float64, n)
	for i := 0; i < n; i++ {
		if math.IsNaN(emaFast[i]) || math.IsNaN(emaSlow[i]) {
			macdRaw[i] = math.NaN()
		} else {
			macdRaw[i] = emaFast[i] - emaSlow[i]
		}
	}

	// Build a clean (no NaN) MACD sub-slice for signal EMA (newest-first)
	// Count valid MACD values from the newest end
	validCount := 0
	for i := 0; i < n; i++ {
		if math.IsNaN(macdRaw[i]) {
			break
		}
		validCount++
	}

	if validCount < signal {
		// Not enough MACD values to compute signal line
		// Return MACD line only, signal/histogram as NaN
		signalLine = make([]float64, n)
		histogram = make([]float64, n)
		for i := 0; i < n; i++ {
			signalLine[i] = math.NaN()
			histogram[i] = math.NaN()
		}
		return macdRaw, signalLine, histogram
	}

	// Extract valid MACD values (newest-first)
	validMACD := macdRaw[:validCount]

	// Compute signal EMA on the valid MACD values
	sigEMA := CalcEMA(validMACD, signal)

	// Map signal EMA back to full-length arrays
	signalLine = make([]float64, n)
	histogram = make([]float64, n)
	for i := 0; i < n; i++ {
		signalLine[i] = math.NaN()
		histogram[i] = math.NaN()
	}
	for i := 0; i < len(sigEMA); i++ {
		signalLine[i] = sigEMA[i]
		if !math.IsNaN(sigEMA[i]) && !math.IsNaN(macdRaw[i]) {
			histogram[i] = macdRaw[i] - sigEMA[i]
		}
	}

	return macdRaw, signalLine, histogram
}

// ---------------------------------------------------------------------------
// Stochastic Oscillator
// ---------------------------------------------------------------------------

// CalcStochastic computes the Slow Stochastic for the most recent bar.
// %K_raw = (Close - LowestLow) / (HighestHigh - LowestLow) * 100
// %K = SMA(kSmooth) of %K_raw (this makes it "slow")
// %D = SMA(dSmooth) of %K
//
// Ref: George Lane's Stochastic Oscillator.
func CalcStochastic(bars []OHLCV, kPeriod, kSmooth, dSmooth int) *StochasticResult {
	kLine, dLine := CalcStochasticSeries(bars, kPeriod, kSmooth, dSmooth)
	if len(kLine) == 0 || len(dLine) == 0 {
		return nil
	}

	k0 := kLine[0]
	d0 := dLine[0]
	if math.IsNaN(k0) || math.IsNaN(d0) {
		return nil
	}

	zone := "NEUTRAL"
	if k0 > 80 {
		zone = "OVERBOUGHT"
	} else if k0 < 20 {
		zone = "OVERSOLD"
	}

	cross := "NONE"
	if len(kLine) >= 2 && len(dLine) >= 2 && !math.IsNaN(kLine[1]) && !math.IsNaN(dLine[1]) {
		prevK, prevD := kLine[1], dLine[1]
		if prevK <= prevD && k0 > d0 {
			cross = "BULLISH_CROSS"
		} else if prevK >= prevD && k0 < d0 {
			cross = "BEARISH_CROSS"
		}
	}

	return &StochasticResult{K: k0, D: d0, Zone: zone, Cross: cross}
}

// CalcStochasticSeries computes the full %K and %D series (newest-first).
// Requires at least kPeriod + kSmooth + dSmooth - 2 bars.
//
// Ref: %K_raw[i] = (Close[i] - LowestLow(kPeriod)) / (HighestHigh(kPeriod) - LowestLow(kPeriod)) * 100
func CalcStochasticSeries(bars []OHLCV, kPeriod, kSmooth, dSmooth int) (kLine, dLine []float64) {
	minBars := kPeriod + kSmooth + dSmooth - 2
	if len(bars) < minBars || kPeriod <= 0 || kSmooth <= 0 || dSmooth <= 0 {
		return nil, nil
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	n := len(asc)

	// Compute raw %K (oldest-first)
	rawK := make([]float64, n)
	for i := 0; i < n; i++ {
		if i < kPeriod-1 {
			rawK[i] = math.NaN()
			continue
		}
		hh := asc[i].High
		ll := asc[i].Low
		for j := i - kPeriod + 1; j < i; j++ {
			if asc[j].High > hh {
				hh = asc[j].High
			}
			if asc[j].Low < ll {
				ll = asc[j].Low
			}
		}
		denom := hh - ll
		if denom == 0 {
			rawK[i] = 50 // midpoint when no range
		} else {
			rawK[i] = (asc[i].Close - ll) / denom * 100
		}
	}

	// Slow %K = SMA(kSmooth) of rawK (oldest-first)
	slowK := CalcSMA(rawK[kPeriod-1:], kSmooth) // only pass valid rawK values
	if slowK == nil {
		return nil, nil
	}
	// Prepend NaNs for the kPeriod-1 initial positions
	fullSlowK := make([]float64, n)
	for i := 0; i < kPeriod-1; i++ {
		fullSlowK[i] = math.NaN()
	}
	for i, v := range slowK {
		fullSlowK[kPeriod-1+i] = v
	}

	// Extract valid slowK for %D SMA
	// Find first non-NaN in fullSlowK
	firstValid := -1
	for i := 0; i < n; i++ {
		if !math.IsNaN(fullSlowK[i]) {
			firstValid = i
			break
		}
	}
	if firstValid < 0 {
		return nil, nil
	}

	validSlowK := fullSlowK[firstValid:]
	dSMA := CalcSMA(validSlowK, dSmooth)
	if dSMA == nil {
		return nil, nil
	}
	fullD := make([]float64, n)
	for i := 0; i < firstValid; i++ {
		fullD[i] = math.NaN()
	}
	for i, v := range dSMA {
		fullD[firstValid+i] = v
	}

	return reverseFloat64(fullSlowK), reverseFloat64(fullD)
}

// ---------------------------------------------------------------------------
// Bollinger Bands
// ---------------------------------------------------------------------------

// CalcBollinger computes the Bollinger Bands for the most recent bar.
// Middle = SMA(period), Upper = Middle + stddev*σ, Lower = Middle - stddev*σ
// Bandwidth = (Upper - Lower) / Middle * 100
// %B = (Close - Lower) / (Upper - Lower)
// Squeeze: Bandwidth < 75% of its own `period`-bar average.
//
// Ref: John Bollinger, "Bollinger on Bollinger Bands".
func CalcBollinger(bars []OHLCV, period int, stddev float64) *BollingerResult {
	upper, middle, lower := CalcBollingerSeries(bars, period, stddev)
	if len(upper) == 0 {
		return nil
	}

	u0, m0, l0 := upper[0], middle[0], lower[0]
	if math.IsNaN(u0) || math.IsNaN(m0) || math.IsNaN(l0) {
		return nil
	}

	bw := 0.0
	if m0 != 0 {
		bw = (u0 - l0) / m0 * 100
	}

	pctB := 0.0
	denom := u0 - l0
	if denom != 0 {
		pctB = (bars[0].Close - l0) / denom
	}

	// Squeeze detection: current bandwidth < 75% of recent average bandwidth.
	// Collect bandwidth values from the most recent valid bars (newest-first slice).
	squeeze := false
	bwSeries := make([]float64, 0, period)
	for i := 0; i < len(upper) && i < period; i++ {
		if !math.IsNaN(upper[i]) && !math.IsNaN(middle[i]) && !math.IsNaN(lower[i]) && middle[i] != 0 {
			bwSeries = append(bwSeries, (upper[i]-lower[i])/middle[i]*100)
		}
	}
	// Require at least half the period for a meaningful average (fixes always-false bug).
	minSamples := period / 2
	if minSamples < 2 {
		minSamples = 2
	}
	if len(bwSeries) >= minSamples {
		avgBW := 0.0
		for _, v := range bwSeries {
			avgBW += v
		}
		avgBW /= float64(len(bwSeries))
		squeeze = bw < avgBW*0.75
	}

	return &BollingerResult{
		Upper:     u0,
		Middle:    m0,
		Lower:     l0,
		Bandwidth: bw,
		PercentB:  pctB,
		Squeeze:   squeeze,
	}
}

// CalcBollingerSeries computes the full Bollinger Bands series (newest-first).
// Returns upper, middle, lower slices. Values before SMA is available are NaN.
//
// Ref: Upper = SMA + k*σ, Lower = SMA - k*σ where σ = population std dev of period bars.
func CalcBollingerSeries(bars []OHLCV, period int, stddev float64) (upper, middle, lower []float64) {
	if len(bars) < period || period <= 0 {
		return nil, nil, nil
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	n := len(asc)

	closes := make([]float64, n)
	for i, b := range asc {
		closes[i] = b.Close
	}

	sma := CalcSMA(closes, period)
	if sma == nil {
		return nil, nil, nil
	}

	u := make([]float64, n)
	m := make([]float64, n)
	l := make([]float64, n)

	for i := 0; i < n; i++ {
		if math.IsNaN(sma[i]) {
			u[i] = math.NaN()
			m[i] = math.NaN()
			l[i] = math.NaN()
			continue
		}
		// Population standard deviation of last `period` closes
		mean := sma[i]
		sumSq := 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := closes[j] - mean
			sumSq += diff * diff
		}
		sd := math.Sqrt(sumSq / float64(period))
		m[i] = mean
		u[i] = mean + stddev*sd
		l[i] = mean - stddev*sd
	}

	return reverseFloat64(u), reverseFloat64(m), reverseFloat64(l)
}

// ---------------------------------------------------------------------------
// EMA Ribbon
// ---------------------------------------------------------------------------

// CalcEMARibbon computes EMA values for multiple periods and determines ribbon alignment.
// Periods should be provided in ascending order, e.g. {9, 21, 55, 100, 200}.
// RibbonAlignment: "BULLISH" if all EMAs are stacked short > long,
// "BEARISH" if stacked long > short, "MIXED" otherwise.
// AlignmentScore: fraction of adjacent pairs correctly ordered (bullish = +1, bearish = -1).
//
// Ref: EMA Ribbon concept — multiple EMAs of increasing period overlaid.
func CalcEMARibbon(bars []OHLCV, periods []int) *EMAResult {
	if len(bars) == 0 || len(periods) == 0 {
		return nil
	}

	closes := make([]float64, len(bars))
	for i, b := range bars {
		closes[i] = b.Close
	}

	sorted := make([]int, len(periods))
	copy(sorted, periods)
	sort.Ints(sorted)

	values := make(map[int]float64)
	emas := make(map[int][]float64)
	for _, p := range sorted {
		e := CalcEMA(closes, p)
		if e == nil || math.IsNaN(e[0]) {
			return nil // not enough data for this period
		}
		values[p] = e[0]
		emas[p] = e
	}

	// Alignment: check if shortest EMA > next > ... > longest (bullish)
	bullishPairs := 0
	bearishPairs := 0
	totalPairs := len(sorted) - 1
	for i := 0; i < totalPairs; i++ {
		shorter := sorted[i]
		longer := sorted[i+1]
		if values[shorter] > values[longer] {
			bullishPairs++
		} else if values[shorter] < values[longer] {
			bearishPairs++
		}
	}

	alignment := "MIXED"
	score := 0.0
	if totalPairs > 0 {
		score = float64(bullishPairs-bearishPairs) / float64(totalPairs)
		if bullishPairs == totalPairs {
			alignment = "BULLISH"
		} else if bearishPairs == totalPairs {
			alignment = "BEARISH"
		}
	}

	return &EMAResult{
		Values:          values,
		RibbonAlignment: alignment,
		AlignmentScore:  score,
	}
}

// ---------------------------------------------------------------------------
// ADX — Wilder's Exact Method
// ---------------------------------------------------------------------------

// CalcADX computes the Average Directional Index (ADX) with +DI and -DI.
// Uses Wilder's smoothing throughout:
//   - Smoothed TR/+DM/-DM: first = sum of `period` values, then prev - prev/period + current
//   - ADX: Wilder's smoothing of DX series (first ADX = SMA of first `period` DX values,
//     then prev_ADX*(period-1)/period + current_DX/period).
//
// Ref: Wilder, "New Concepts in Technical Trading Systems" (1978), Chapter on ADX.
func CalcADX(bars []OHLCV, period int) *ADXResult {
	// Need at least 2*period + 1 bars for a meaningful ADX
	// (period for smoothed DM/TR, period for DX, then Wilder smooth DX)
	minBars := 2*period + 1
	if len(bars) < minBars || period <= 0 {
		return nil
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	n := len(asc)

	// Step 1: Compute TR, +DM, -DM for each bar (index 1..n-1)
	tr := make([]float64, n)
	plusDM := make([]float64, n)
	minusDM := make([]float64, n)

	for i := 1; i < n; i++ {
		highDiff := asc[i].High - asc[i-1].High
		lowDiff := asc[i-1].Low - asc[i].Low

		// True Range
		hl := asc[i].High - asc[i].Low
		hpc := math.Abs(asc[i].High - asc[i-1].Close)
		lpc := math.Abs(asc[i].Low - asc[i-1].Close)
		tr[i] = math.Max(hl, math.Max(hpc, lpc))

		// +DM
		if highDiff > lowDiff && highDiff > 0 {
			plusDM[i] = highDiff
		}
		// -DM
		if lowDiff > highDiff && lowDiff > 0 {
			minusDM[i] = lowDiff
		}
	}

	// Step 2: First smoothed values = sum of first `period` values (indices 1..period)
	var smoothTR, smoothPlusDM, smoothMinusDM float64
	for i := 1; i <= period; i++ {
		smoothTR += tr[i]
		smoothPlusDM += plusDM[i]
		smoothMinusDM += minusDM[i]
	}

	// Compute first +DI, -DI, DX at index `period`
	diPlus := make([]float64, n)
	diMinus := make([]float64, n)
	dx := make([]float64, n)

	computeDIDX := func(idx int) {
		if smoothTR == 0 {
			diPlus[idx] = 0
			diMinus[idx] = 0
			dx[idx] = 0
			return
		}
		diPlus[idx] = smoothPlusDM / smoothTR * 100
		diMinus[idx] = smoothMinusDM / smoothTR * 100
		diSum := diPlus[idx] + diMinus[idx]
		if diSum == 0 {
			dx[idx] = 0
		} else {
			dx[idx] = math.Abs(diPlus[idx]-diMinus[idx]) / diSum * 100
		}
	}

	computeDIDX(period)

	// Step 3: Subsequent smoothed values using Wilder's method
	for i := period + 1; i < n; i++ {
		smoothTR = smoothTR - smoothTR/float64(period) + tr[i]
		smoothPlusDM = smoothPlusDM - smoothPlusDM/float64(period) + plusDM[i]
		smoothMinusDM = smoothMinusDM - smoothMinusDM/float64(period) + minusDM[i]
		computeDIDX(i)
	}

	// Step 4: ADX = Wilder's smoothing of DX
	// First ADX = average of first `period` DX values (from index period..2*period-1)
	if 2*period-1 >= n {
		return nil
	}
	sumDX := 0.0
	for i := period; i < 2*period; i++ {
		sumDX += dx[i]
	}
	adx := make([]float64, n)
	adx[2*period-1] = sumDX / float64(period)

	for i := 2 * period; i < n; i++ {
		adx[i] = (adx[i-1]*float64(period-1) + dx[i]) / float64(period)
	}

	// Latest values
	latestADX := adx[n-1]
	latestPlusDI := diPlus[n-1]
	latestMinusDI := diMinus[n-1]

	trending := latestADX > 25
	strength := "WEAK"
	if latestADX > 50 {
		strength = "STRONG"
	} else if latestADX > 25 {
		strength = "MODERATE"
	}

	return &ADXResult{
		ADX:           latestADX,
		PlusDI:        latestPlusDI,
		MinusDI:       latestMinusDI,
		Trending:      trending,
		TrendStrength: strength,
	}
}

// ---------------------------------------------------------------------------
// OBV — On-Balance Volume
// ---------------------------------------------------------------------------

// CalcOBV computes the On-Balance Volume for the most recent bar and its trend.
// OBV accumulates volume: +Volume if close > prev close, -Volume if close < prev close.
// Trend is determined by comparing current OBV to its 10-period SMA.
//
// Ref: Joseph Granville, "New Key to Stock Market Profits" (1963).
func CalcOBV(bars []OHLCV) *OBVResult {
	series := CalcOBVSeries(bars)
	if len(series) == 0 {
		return nil
	}

	trend := "FLAT"
	if len(series) >= 10 {
		// CalcSMA preserves input order. series is newest-first, so
		// sma[i] = avg(series[i-period+1..i]) → the first valid SMA is at
		// index period-1 (i.e. sma[9]). We need the SMA ending at the most
		// recent bar, so reverse to oldest-first, compute SMA, then check the last value.
		obvAsc := reverseFloat64(series)
		sma := CalcSMA(obvAsc, 10)
		if sma != nil {
			latestSMA := sma[len(sma)-1]
			latestOBV := obvAsc[len(obvAsc)-1]
			if !math.IsNaN(latestSMA) {
				if latestOBV > latestSMA*1.01 {
					trend = "RISING"
				} else if latestOBV < latestSMA*0.99 {
					trend = "FALLING"
				}
			}
		}
	} else if len(series) >= 5 {
		// With fewer bars, use a simple linear regression slope approximation
		// Compare the average of the newest 2 vs oldest 2 OBV values
		newestAvg := (series[0] + series[1]) / 2
		oldestAvg := (series[len(series)-1] + series[len(series)-2]) / 2
		if newestAvg > oldestAvg*1.01 {
			trend = "RISING"
		} else if newestAvg < oldestAvg*0.99 {
			trend = "FALLING"
		}
	}

	return &OBVResult{Value: series[0], Trend: trend, Series: series}
}

// CalcOBVSeries computes the full OBV series (newest-first).
// If close > prev close: OBV += volume, if close < prev close: OBV -= volume.
//
// Ref: Granville's On-Balance Volume.
func CalcOBVSeries(bars []OHLCV) []float64 {
	if len(bars) < 2 {
		return nil
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	n := len(asc)

	obv := make([]float64, n)
	obv[0] = 0 // seed
	for i := 1; i < n; i++ {
		if asc[i].Close > asc[i-1].Close {
			obv[i] = obv[i-1] + asc[i].Volume
		} else if asc[i].Close < asc[i-1].Close {
			obv[i] = obv[i-1] - asc[i].Volume
		} else {
			obv[i] = obv[i-1]
		}
	}

	return reverseFloat64(obv)
}

// ---------------------------------------------------------------------------
// Williams %R
// ---------------------------------------------------------------------------

// CalcWilliamsR computes Williams %R for the most recent bar.
// %R = (HighestHigh(period) - Close) / (HighestHigh(period) - LowestLow(period)) * -100
//
// Ref: Larry Williams' %R indicator. Range: -100 to 0.
func CalcWilliamsR(bars []OHLCV, period int) *WilliamsRResult {
	if len(bars) < period || period <= 0 {
		return nil
	}

	// Use newest `period` bars (newest-first, so indices 0..period-1)
	hh := bars[0].High
	ll := bars[0].Low
	for i := 1; i < period; i++ {
		if bars[i].High > hh {
			hh = bars[i].High
		}
		if bars[i].Low < ll {
			ll = bars[i].Low
		}
	}

	denom := hh - ll
	if denom == 0 {
		return &WilliamsRResult{Value: -50, Zone: "NEUTRAL"}
	}

	val := (hh - bars[0].Close) / denom * -100

	zone := "NEUTRAL"
	if val > -20 {
		zone = "OVERBOUGHT"
	} else if val < -80 {
		zone = "OVERSOLD"
	}

	return &WilliamsRResult{Value: val, Zone: zone}
}

// ---------------------------------------------------------------------------
// CCI — Commodity Channel Index
// ---------------------------------------------------------------------------

// CalcCCI computes the Commodity Channel Index for the most recent bar.
// TP = (High + Low + Close) / 3
// CCI = (TP - SMA(TP, period)) / (0.015 * MeanDeviation(TP, period))
//
// Ref: Donald Lambert's CCI. Constant 0.015 ensures ~75% of values fall within ±100.
func CalcCCI(bars []OHLCV, period int) *CCIResult {
	if len(bars) < period || period <= 0 {
		return nil
	}

	// Work oldest-first for sequential calc
	asc := reverseOHLCV(bars)
	n := len(asc)

	tp := make([]float64, n)
	for i, b := range asc {
		tp[i] = (b.High + b.Low + b.Close) / 3
	}

	// We need TP SMA at the latest bar
	if n < period {
		return nil
	}

	// SMA of last `period` TP values (at index n-1)
	sum := 0.0
	for i := n - period; i < n; i++ {
		sum += tp[i]
	}
	tpSMA := sum / float64(period)

	// Mean deviation: average of |TP - SMA|
	meanDev := 0.0
	for i := n - period; i < n; i++ {
		meanDev += math.Abs(tp[i] - tpSMA)
	}
	meanDev /= float64(period)

	if meanDev == 0 {
		return &CCIResult{Value: 0, Zone: "NEUTRAL"}
	}

	cci := (tp[n-1] - tpSMA) / (0.015 * meanDev)

	zone := "NEUTRAL"
	if cci > 100 {
		zone = "OVERBOUGHT"
	} else if cci < -100 {
		zone = "OVERSOLD"
	}

	return &CCIResult{Value: cci, Zone: zone}
}

// ---------------------------------------------------------------------------
// MFI — Money Flow Index
// ---------------------------------------------------------------------------

// CalcMFI computes the Money Flow Index for the most recent bar.
// TP = (High + Low + Close) / 3; RawMF = TP * Volume
// MFI = 100 - 100/(1 + PositiveMF/NegativeMF)
// Returns nil if all volumes are zero (common for FX spot).
//
// Ref: Gene Quong & Avrum Soudack's MFI (volume-weighted RSI variant).
func CalcMFI(bars []OHLCV, period int) *MFIResult {
	if len(bars) < period+1 || period <= 0 {
		return nil
	}

	// Check if all volumes are zero
	allZero := true
	for _, b := range bars {
		if b.Volume != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil // MFI requires volume data
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	n := len(asc)

	tp := make([]float64, n)
	for i, b := range asc {
		tp[i] = (b.High + b.Low + b.Close) / 3
	}

	// Compute positive and negative money flow over the most recent `period` bars
	// We compare TP[i] vs TP[i-1] for indices (n-period)..(n-1)
	positiveMF := 0.0
	negativeMF := 0.0
	startIdx := n - period
	if startIdx < 1 {
		startIdx = 1
	}
	for i := startIdx; i < n; i++ {
		rawMF := tp[i] * asc[i].Volume
		if tp[i] > tp[i-1] {
			positiveMF += rawMF
		} else if tp[i] < tp[i-1] {
			negativeMF += rawMF
		}
		// If TP unchanged, money flow is neither positive nor negative
	}

	if negativeMF == 0 {
		if positiveMF == 0 {
			return &MFIResult{Value: 50, Zone: "NEUTRAL"}
		}
		return &MFIResult{Value: 100, Zone: "OVERBOUGHT"}
	}

	mfi := 100 - 100/(1+positiveMF/negativeMF)

	zone := "NEUTRAL"
	if mfi > 80 {
		zone = "OVERBOUGHT"
	} else if mfi < 20 {
		zone = "OVERSOLD"
	}

	return &MFIResult{Value: mfi, Zone: zone}
}
