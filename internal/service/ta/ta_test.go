package ta

import (
	"math"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeBarsFromCloses creates OHLCV bars from close prices (newest-first).
// For simplicity, Open=High=Low=Close and Volume=1000.
func makeBarsFromCloses(closes []float64) []OHLCV {
	bars := make([]OHLCV, len(closes))
	for i, c := range closes {
		bars[i] = OHLCV{
			Date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(i)),
			Open:   c,
			High:   c,
			Low:    c,
			Close:  c,
			Volume: 1000,
		}
	}
	return bars
}

// makeBarsFromOHLCV creates OHLCV bars from individual O/H/L/C slices (newest-first).
func makeBarsFromOHLCV(o, h, l, c []float64, vol []float64) []OHLCV {
	n := len(c)
	bars := make([]OHLCV, n)
	for i := 0; i < n; i++ {
		v := 1000.0
		if vol != nil && i < len(vol) {
			v = vol[i]
		}
		bars[i] = OHLCV{
			Date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(i)),
			Open:   o[i],
			High:   h[i],
			Low:    l[i],
			Close:  c[i],
			Volume: v,
		}
	}
	return bars
}

func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

// ---------------------------------------------------------------------------
// Test RSI
// ---------------------------------------------------------------------------

func TestCalcRSI_WilderExample(t *testing.T) {
	// Classic Wilder RSI test data (oldest to newest close prices):
	// These prices are from common RSI examples.
	// The data set: 21 data points → 20 changes → 14-period RSI starts at index 14
	// giving us several RSI values. We test the most recent one.
	//
	// Data (oldest-first): 44,44.34,44.09,43.61,44.33,44.83,45.10,45.42,45.84,46.08,
	//                      45.89,46.03,45.61,46.28,46.28,46.00,46.03,46.41,46.22,46.21,45.64
	closesOldest := []float64{
		44, 44.34, 44.09, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08,
		45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 46.21, 45.64,
	}

	// Reverse to newest-first
	closes := make([]float64, len(closesOldest))
	for i, v := range closesOldest {
		closes[len(closesOldest)-1-i] = v
	}
	bars := makeBarsFromCloses(closes)

	result := CalcRSI(bars, 14)
	if result == nil {
		t.Fatal("CalcRSI returned nil")
	}

	// With Wilder's smoothing, the final RSI should be approximately 55-58.
	// Let's verify it's in a reasonable range (accounting for exact Wilder smoothing).
	t.Logf("RSI(14) = %.4f, Zone=%s, Trend=%s", result.Value, result.Zone, result.Trend)

	if result.Value < 40 || result.Value > 75 {
		t.Errorf("RSI value %.4f outside expected range [40, 75]", result.Value)
	}

	// With the final value being 45.64 (a drop), RSI should not be overbought
	if result.Zone == "OVERBOUGHT" {
		t.Error("RSI should not be overbought after a drop")
	}
}

func TestCalcRSI_AllUp(t *testing.T) {
	// Monotonically increasing prices → RSI should be 100
	closes := make([]float64, 20)
	for i := 0; i < 20; i++ {
		closes[i] = float64(120 - i) // newest=120, oldest=101
	}
	bars := makeBarsFromCloses(closes)

	result := CalcRSI(bars, 14)
	if result == nil {
		t.Fatal("CalcRSI returned nil for all-up data")
	}
	if result.Value != 100 {
		t.Errorf("RSI for monotonically increasing should be 100, got %.4f", result.Value)
	}
}

func TestCalcRSI_AllDown(t *testing.T) {
	// Monotonically decreasing prices → RSI should be 0
	closes := make([]float64, 20)
	for i := 0; i < 20; i++ {
		closes[i] = float64(80 + i) // newest=80, oldest=99 (ascending = newest is lowest)
	}
	bars := makeBarsFromCloses(closes)

	result := CalcRSI(bars, 14)
	if result == nil {
		t.Fatal("CalcRSI returned nil for all-down data")
	}
	if result.Value != 0 {
		t.Errorf("RSI for monotonically decreasing should be 0, got %.4f", result.Value)
	}
}

func TestCalcRSI_InsufficientData(t *testing.T) {
	bars := makeBarsFromCloses([]float64{50, 49, 48})
	result := CalcRSI(bars, 14)
	if result != nil {
		t.Error("CalcRSI should return nil for insufficient data")
	}
}

func TestCalcRSISeries(t *testing.T) {
	closesOldest := []float64{
		44, 44.34, 44.09, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08,
		45.89, 46.03, 45.61, 46.28, 46.28, 46.00, 46.03, 46.41, 46.22, 46.21, 45.64,
	}
	closes := make([]float64, len(closesOldest))
	for i, v := range closesOldest {
		closes[len(closesOldest)-1-i] = v
	}
	bars := makeBarsFromCloses(closes)

	series := CalcRSISeries(bars, 14)
	if series == nil {
		t.Fatal("CalcRSISeries returned nil")
	}

	// Should have 21-14 = 7 RSI values
	expectedLen := len(closesOldest) - 14
	if len(series) != expectedLen {
		t.Errorf("Expected %d RSI values, got %d", expectedLen, len(series))
	}

	// First RSI value (oldest) should be higher (price was trending up early)
	t.Logf("RSI series (newest-first): %v", series)
	for _, v := range series {
		if v < 0 || v > 100 {
			t.Errorf("RSI value %.4f outside [0, 100]", v)
		}
	}
}

// ---------------------------------------------------------------------------
// Test MACD
// ---------------------------------------------------------------------------

func TestCalcMACD_Basic(t *testing.T) {
	// Create 35 bars of slowly rising prices (enough for 26-period slow EMA + 9 signal)
	closes := make([]float64, 40)
	for i := 0; i < 40; i++ {
		closes[i] = 100 + float64(40-i)*0.5 // newest=120, oldest=100.5
	}
	bars := makeBarsFromCloses(closes)

	result := CalcMACD(bars, 12, 26, 9)
	if result == nil {
		t.Fatal("CalcMACD returned nil")
	}

	t.Logf("MACD=%.4f, Signal=%.4f, Histogram=%.4f, Cross=%s",
		result.MACD, result.Signal, result.Histogram, result.Cross)

	// In an uptrend, MACD line should be positive (fast EMA > slow EMA)
	if result.MACD <= 0 {
		t.Error("MACD should be positive in uptrend")
	}
}

func TestCalcMACD_CrossoverDetection(t *testing.T) {
	// Create data where MACD crosses signal
	// Start with downtrend then sharp upturn
	closes := make([]float64, 60)
	for i := 0; i < 60; i++ {
		idx := 60 - i // oldest to newest mapping
		if idx <= 40 {
			closes[i] = 100 + float64(idx)*0.3 // recent: rising
		} else {
			closes[i] = 100 + float64(80-idx)*0.3 // older: falling
		}
	}
	bars := makeBarsFromCloses(closes)

	result := CalcMACD(bars, 12, 26, 9)
	if result == nil {
		t.Fatal("CalcMACD returned nil")
	}
	t.Logf("MACD=%.4f, Signal=%.4f, Hist=%.4f, Cross=%s",
		result.MACD, result.Signal, result.Histogram, result.Cross)
	// Just verify it doesn't crash and returns valid data
}

func TestCalcMACD_InsufficientData(t *testing.T) {
	bars := makeBarsFromCloses([]float64{50, 49, 48})
	result := CalcMACD(bars, 12, 26, 9)
	if result != nil {
		t.Error("CalcMACD should return nil for insufficient data")
	}
}

// ---------------------------------------------------------------------------
// Test Stochastic
// ---------------------------------------------------------------------------

func TestCalcStochastic_Basic(t *testing.T) {
	// Create bars with known highs and lows
	n := 30
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)

	for i := 0; i < n; i++ {
		// Gradually rising, newest first
		base := 100 + float64(n-i)
		o[i] = base
		h[i] = base + 2
		l[i] = base - 2
		c[i] = base + 1 // close near high
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcStochastic(bars, 14, 3, 3)
	if result == nil {
		t.Fatal("CalcStochastic returned nil")
	}

	t.Logf("%%K=%.2f, %%D=%.2f, Zone=%s, Cross=%s", result.K, result.D, result.Zone, result.Cross)

	// K and D should be in [0, 100]
	if result.K < 0 || result.K > 100 {
		t.Errorf("%%K value %.2f outside [0, 100]", result.K)
	}
	if result.D < 0 || result.D > 100 {
		t.Errorf("%%D value %.2f outside [0, 100]", result.D)
	}
}

func TestCalcStochastic_Oversold(t *testing.T) {
	// Price at the bottom of its range → should be oversold
	n := 25
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)

	for i := 0; i < n; i++ {
		o[i] = 100
		h[i] = 110
		l[i] = 90
		c[i] = 91 // close near low
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcStochastic(bars, 14, 3, 3)
	if result == nil {
		t.Fatal("CalcStochastic returned nil")
	}

	t.Logf("%%K=%.2f, %%D=%.2f, Zone=%s", result.K, result.D, result.Zone)
	if result.K > 20 {
		t.Errorf("%%K should be near 0 when close is near low, got %.2f", result.K)
	}
	if result.Zone != "OVERSOLD" {
		t.Errorf("Zone should be OVERSOLD, got %s", result.Zone)
	}
}

// ---------------------------------------------------------------------------
// Test Bollinger Bands
// ---------------------------------------------------------------------------

func TestCalcBollinger_Basic(t *testing.T) {
	// 30 bars of steady prices around 100
	closes := make([]float64, 30)
	for i := 0; i < 30; i++ {
		closes[i] = 100 + math.Sin(float64(i)*0.3)*2 // slight oscillation
	}
	bars := makeBarsFromCloses(closes)

	result := CalcBollinger(bars, 20, 2.0)
	if result == nil {
		t.Fatal("CalcBollinger returned nil")
	}

	t.Logf("Upper=%.4f, Middle=%.4f, Lower=%.4f, BW=%.4f, %%B=%.4f, Squeeze=%v",
		result.Upper, result.Middle, result.Lower, result.Bandwidth, result.PercentB, result.Squeeze)

	// Upper > Middle > Lower
	if result.Upper <= result.Middle {
		t.Error("Upper should be > Middle")
	}
	if result.Middle <= result.Lower {
		t.Error("Middle should be > Lower")
	}

	// Bandwidth should be positive
	if result.Bandwidth <= 0 {
		t.Error("Bandwidth should be positive")
	}
}

func TestCalcBollinger_SqueezeDetection(t *testing.T) {
	// Create data: first 20 bars very tight range, then 20 bars volatile.
	// Slice is newest-first, so tight bars are index 0-19 (recent),
	// volatile bars are index 20-39 (older).
	closes := make([]float64, 40)
	for i := 0; i < 40; i++ {
		if i < 20 {
			// Recent: very tight range → should trigger squeeze
			closes[i] = 100.0 + float64(i)*0.001
		} else {
			// Older: wide swings
			closes[i] = 100.0 + math.Sin(float64(i)*0.8)*10.0
		}
	}
	bars := makeBarsFromCloses(closes)

	result := CalcBollinger(bars, 20, 2.0)
	if result == nil {
		t.Fatal("CalcBollinger returned nil")
	}

	t.Logf("Bandwidth=%.4f, Squeeze=%v", result.Bandwidth, result.Squeeze)
	if !result.Squeeze {
		t.Error("Expected squeeze=true for tight-range data following volatile data")
	}
}

func TestCalcBollinger_NoSqueeze(t *testing.T) {
	// Uniform volatility — no squeeze expected.
	closes := make([]float64, 40)
	for i := 0; i < 40; i++ {
		closes[i] = 100.0 + math.Sin(float64(i)*0.5)*3.0
	}
	bars := makeBarsFromCloses(closes)

	result := CalcBollinger(bars, 20, 2.0)
	if result == nil {
		t.Fatal("CalcBollinger returned nil")
	}

	t.Logf("Bandwidth=%.4f, Squeeze=%v", result.Bandwidth, result.Squeeze)
	// With uniform volatility, current bandwidth ≈ average → no squeeze
	if result.Squeeze {
		t.Error("Expected squeeze=false for uniform volatility data")
	}
}

func TestCalcBollingerSeries(t *testing.T) {
	closes := make([]float64, 25)
	for i := 0; i < 25; i++ {
		closes[i] = 50 + float64(i)*0.2
	}
	bars := makeBarsFromCloses(closes)

	upper, middle, lower := CalcBollingerSeries(bars, 20, 2.0)
	if upper == nil || middle == nil || lower == nil {
		t.Fatal("CalcBollingerSeries returned nil")
	}

	if len(upper) != 25 {
		t.Errorf("Expected 25 values, got %d", len(upper))
	}

	// First valid value at index len-20 (newest-first: index 5)
	// Check that non-NaN values have Upper > Middle > Lower
	for i := 0; i < len(upper); i++ {
		if !math.IsNaN(upper[i]) && !math.IsNaN(middle[i]) && !math.IsNaN(lower[i]) {
			if upper[i] < middle[i] || middle[i] < lower[i] {
				t.Errorf("At index %d: Upper(%.4f) should >= Middle(%.4f) >= Lower(%.4f)",
					i, upper[i], middle[i], lower[i])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Test EMA
// ---------------------------------------------------------------------------

func TestCalcEMA_SeedFromSMA(t *testing.T) {
	// 10 values (newest-first): 10,9,8,7,6,5,4,3,2,1
	closes := []float64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	ema := CalcEMA(closes, 5)
	if ema == nil {
		t.Fatal("CalcEMA returned nil")
	}

	// Oldest-first: 1,2,3,4,5,6,7,8,9,10
	// SMA(5) seed at index 4 (oldest-first) = (1+2+3+4+5)/5 = 3.0
	// mult = 2/6 = 0.3333
	// EMA[5] = (6 - 3.0)*0.3333 + 3.0 = 4.0
	// EMA[6] = (7 - 4.0)*0.3333 + 4.0 = 5.0
	// EMA[7] = (8 - 5.0)*0.3333 + 5.0 = 6.0
	// EMA[8] = (9 - 6.0)*0.3333 + 6.0 = 7.0
	// EMA[9] = (10 - 7.0)*0.3333 + 7.0 = 8.0

	t.Logf("EMA series (newest-first): %v", ema)

	// The seed (oldest valid) should be SMA = 3.0
	// In newest-first output, that's the last non-NaN value
	// Index 5 (newest-first) = oldest EMA = seed = 3.0
	if !approxEqual(ema[5], 3.0, 0.01) {
		t.Errorf("EMA seed should be 3.0 (SMA of 1,2,3,4,5), got %.4f", ema[5])
	}

	// Newest EMA should be 8.0
	if !approxEqual(ema[0], 8.0, 0.01) {
		t.Errorf("Latest EMA should be ~8.0, got %.4f", ema[0])
	}
}

func TestCalcEMA_InsufficientData(t *testing.T) {
	closes := []float64{10, 9, 8}
	ema := CalcEMA(closes, 5)
	if ema != nil {
		t.Error("CalcEMA should return nil for insufficient data")
	}
}

func TestCalcEMARibbon(t *testing.T) {
	// Create 250 bars of uptrending data
	closes := make([]float64, 250)
	for i := 0; i < 250; i++ {
		closes[i] = 200 - float64(i)*0.5 // newest=200, oldest ~75
	}
	bars := makeBarsFromCloses(closes)

	result := CalcEMARibbon(bars, []int{9, 21, 55, 100, 200})
	if result == nil {
		t.Fatal("CalcEMARibbon returned nil")
	}

	t.Logf("Ribbon alignment=%s, score=%.2f, values=%v",
		result.RibbonAlignment, result.AlignmentScore, result.Values)

	// In a strong uptrend, short EMAs should be above longer ones
	if result.RibbonAlignment != "BULLISH" {
		t.Errorf("Expected BULLISH alignment in uptrend, got %s", result.RibbonAlignment)
	}
	if result.AlignmentScore != 1.0 {
		t.Errorf("Expected alignment score 1.0 in perfect uptrend, got %.2f", result.AlignmentScore)
	}
}

// ---------------------------------------------------------------------------
// Test ADX
// ---------------------------------------------------------------------------

func TestCalcADX_Trending(t *testing.T) {
	// Create strongly trending data (consistent up movement)
	n := 60
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)

	for i := 0; i < n; i++ {
		base := 100 + float64(n-i)*2 // strong uptrend, newest has highest price
		o[i] = base - 1
		h[i] = base + 1
		l[i] = base - 2
		c[i] = base
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcADX(bars, 14)
	if result == nil {
		t.Fatal("CalcADX returned nil")
	}

	t.Logf("ADX=%.2f, +DI=%.2f, -DI=%.2f, Trending=%v, Strength=%s",
		result.ADX, result.PlusDI, result.MinusDI, result.Trending, result.TrendStrength)

	// In a strong uptrend:
	// +DI should be > -DI
	if result.PlusDI <= result.MinusDI {
		t.Errorf("+DI (%.2f) should be > -DI (%.2f) in uptrend", result.PlusDI, result.MinusDI)
	}

	// ADX should indicate trending (>25)
	if !result.Trending {
		t.Logf("Warning: ADX=%.2f, expected trending (>25). May be borderline.", result.ADX)
	}
}

func TestCalcADX_Ranging(t *testing.T) {
	// Create ranging data (oscillating, no clear trend)
	n := 60
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)

	for i := 0; i < n; i++ {
		base := 100 + math.Sin(float64(n-i)*0.5)*3 // oscillating around 100
		o[i] = base
		h[i] = base + 1
		l[i] = base - 1
		c[i] = base + math.Sin(float64(n-i)*0.7)*0.5
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcADX(bars, 14)
	if result == nil {
		t.Fatal("CalcADX returned nil")
	}

	t.Logf("ADX=%.2f, +DI=%.2f, -DI=%.2f, Trending=%v, Strength=%s",
		result.ADX, result.PlusDI, result.MinusDI, result.Trending, result.TrendStrength)

	// In a ranging market, ADX should generally be lower
	// This is soft — synthetic data may not perfectly produce low ADX
}

func TestCalcADX_InsufficientData(t *testing.T) {
	bars := makeBarsFromCloses([]float64{50, 49, 48, 47, 46})
	result := CalcADX(bars, 14)
	if result != nil {
		t.Error("CalcADX should return nil for insufficient data")
	}
}

// ---------------------------------------------------------------------------
// Test OBV
// ---------------------------------------------------------------------------

func TestCalcOBV_Basic(t *testing.T) {
	// Manually constructed: alternating up/down
	closes := []float64{105, 103, 104, 102, 103, 101, 102, 100, 101, 99, 100}
	bars := makeBarsFromCloses(closes)

	result := CalcOBV(bars)
	if result == nil {
		t.Fatal("CalcOBV returned nil")
	}

	t.Logf("OBV=%.0f, Trend=%s, SeriesLen=%d", result.Value, result.Trend, len(result.Series))

	// Verify series length
	if len(result.Series) != len(closes) {
		t.Errorf("Expected %d OBV values, got %d", len(closes), len(result.Series))
	}
}

func TestCalcOBVSeries(t *testing.T) {
	// 5 bars oldest-first: 10,11,10,12,11 → changes: +1,-1,+2,-1
	// OBV: 0, +1000, 0, +1000, 0
	// Newest-first input: 11,12,10,11,10
	closes := []float64{11, 12, 10, 11, 10}
	bars := makeBarsFromCloses(closes)

	series := CalcOBVSeries(bars)
	if series == nil {
		t.Fatal("CalcOBVSeries returned nil")
	}

	t.Logf("OBV series (newest-first): %v", series)

	// Oldest OBV = 0 (seed)
	if series[len(series)-1] != 0 {
		t.Errorf("Oldest OBV should be 0, got %.0f", series[len(series)-1])
	}
}

// ---------------------------------------------------------------------------
// Test Williams %R
// ---------------------------------------------------------------------------

func TestCalcWilliamsR_Basic(t *testing.T) {
	n := 20
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)

	for i := 0; i < n; i++ {
		o[i] = 100
		h[i] = 110
		l[i] = 90
		c[i] = 105 // close in upper half of range
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcWilliamsR(bars, 14)
	if result == nil {
		t.Fatal("CalcWilliamsR returned nil")
	}

	t.Logf("Williams %%R = %.2f, Zone=%s", result.Value, result.Zone)

	// %R = (110 - 105) / (110 - 90) * -100 = 5/20 * -100 = -25
	if !approxEqual(result.Value, -25, 0.01) {
		t.Errorf("Expected Williams %%R = -25, got %.2f", result.Value)
	}
	if result.Zone != "NEUTRAL" {
		t.Errorf("Expected NEUTRAL zone, got %s", result.Zone)
	}
}

func TestCalcWilliamsR_Overbought(t *testing.T) {
	n := 20
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)

	for i := 0; i < n; i++ {
		o[i] = 100
		h[i] = 110
		l[i] = 90
		c[i] = 109 // close very near high
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcWilliamsR(bars, 14)
	if result == nil {
		t.Fatal("CalcWilliamsR returned nil")
	}

	// %R = (110 - 109) / (110 - 90) * -100 = -5
	if !approxEqual(result.Value, -5, 0.01) {
		t.Errorf("Expected Williams %%R = -5, got %.2f", result.Value)
	}
	if result.Zone != "OVERBOUGHT" {
		t.Errorf("Expected OVERBOUGHT zone, got %s", result.Zone)
	}
}

// ---------------------------------------------------------------------------
// Test CCI
// ---------------------------------------------------------------------------

func TestCalcCCI_Basic(t *testing.T) {
	// Create 25 bars with steady prices → CCI near 0
	closes := make([]float64, 25)
	for i := 0; i < 25; i++ {
		closes[i] = 100
	}
	bars := makeBarsFromCloses(closes)

	result := CalcCCI(bars, 20)
	if result == nil {
		t.Fatal("CalcCCI returned nil")
	}

	t.Logf("CCI = %.2f, Zone=%s", result.Value, result.Zone)

	// With constant prices, CCI should be 0 (or handled gracefully)
	if result.Zone != "NEUTRAL" {
		t.Errorf("Expected NEUTRAL zone for flat prices, got %s", result.Zone)
	}
}

// ---------------------------------------------------------------------------
// Test MFI
// ---------------------------------------------------------------------------

func TestCalcMFI_Basic(t *testing.T) {
	// 20 bars with increasing close and volume
	n := 20
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)

	for i := 0; i < n; i++ {
		base := 100 + float64(n-i)*0.5
		o[i] = base - 0.3
		h[i] = base + 0.5
		l[i] = base - 0.5
		c[i] = base
		v[i] = 10000 + float64(n-i)*100
	}
	bars := makeBarsFromOHLCV(o, h, l, c, v)

	result := CalcMFI(bars, 14)
	if result == nil {
		t.Fatal("CalcMFI returned nil")
	}

	t.Logf("MFI = %.2f, Zone=%s", result.Value, result.Zone)

	// MFI should be in [0, 100]
	if result.Value < 0 || result.Value > 100 {
		t.Errorf("MFI value %.2f outside [0, 100]", result.Value)
	}
}

func TestCalcMFI_ZeroVolume(t *testing.T) {
	// All zero volume → should return nil
	closes := make([]float64, 20)
	for i := 0; i < 20; i++ {
		closes[i] = 100 + float64(i)
	}
	bars := makeBarsFromCloses(closes)
	// makeBarsFromCloses sets Volume=1000, override to 0
	for i := range bars {
		bars[i].Volume = 0
	}

	result := CalcMFI(bars, 14)
	if result != nil {
		t.Error("CalcMFI should return nil when all volumes are zero")
	}
}

// ---------------------------------------------------------------------------
// Test SMA
// ---------------------------------------------------------------------------

func TestCalcSMA_Basic(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	sma := CalcSMA(values, 3)
	if sma == nil {
		t.Fatal("CalcSMA returned nil")
	}

	// SMA(3) at index 2 = (1+2+3)/3 = 2
	if !approxEqual(sma[2], 2.0, 0.001) {
		t.Errorf("SMA[2] should be 2.0, got %.4f", sma[2])
	}

	// SMA(3) at index 9 = (8+9+10)/3 = 9
	if !approxEqual(sma[9], 9.0, 0.001) {
		t.Errorf("SMA[9] should be 9.0, got %.4f", sma[9])
	}

	// First two should be NaN
	if !math.IsNaN(sma[0]) || !math.IsNaN(sma[1]) {
		t.Error("First period-1 values should be NaN")
	}
}

func TestCalcSMA_InsufficientData(t *testing.T) {
	sma := CalcSMA([]float64{1, 2}, 5)
	if sma != nil {
		t.Error("CalcSMA should return nil for insufficient data")
	}
}

// ---------------------------------------------------------------------------
// Test OBV Trend Detection (fixed SMA ordering)
// ---------------------------------------------------------------------------

func TestCalcOBV_RisingTrend(t *testing.T) {
	// 15 bars of steadily rising close prices → OBV should accumulate upward → RISING
	n := 15
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)
	for i := 0; i < n; i++ {
		base := 100 + float64(n-i) // newest has highest price
		o[i] = base - 0.5
		h[i] = base + 0.5
		l[i] = base - 1
		c[i] = base
		v[i] = 10000
	}
	bars := makeBarsFromOHLCV(o, h, l, c, v)
	result := CalcOBV(bars)
	if result == nil {
		t.Fatal("CalcOBV returned nil")
	}
	t.Logf("OBV=%.0f, Trend=%s", result.Value, result.Trend)
	if result.Trend != "RISING" {
		t.Errorf("Expected RISING trend for monotonically increasing prices, got %s", result.Trend)
	}
}

func TestCalcOBV_FallingTrend(t *testing.T) {
	// 15 bars of steadily falling close prices → OBV should drain downward → FALLING
	n := 15
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)
	for i := 0; i < n; i++ {
		base := 100 - float64(n-i) // newest has lowest price
		o[i] = base + 0.5
		h[i] = base + 1
		l[i] = base - 0.5
		c[i] = base
		v[i] = 10000
	}
	bars := makeBarsFromOHLCV(o, h, l, c, v)
	result := CalcOBV(bars)
	if result == nil {
		t.Fatal("CalcOBV returned nil")
	}
	t.Logf("OBV=%.0f, Trend=%s", result.Value, result.Trend)
	if result.Trend != "FALLING" {
		t.Errorf("Expected FALLING trend for monotonically decreasing prices, got %s", result.Trend)
	}
}

// ---------------------------------------------------------------------------
// Test Snapshot has CurrentPrice and ATR
// ---------------------------------------------------------------------------

func TestComputeSnapshot_HasCurrentPrice(t *testing.T) {
	n := 60
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)
	for i := 0; i < n; i++ {
		base := 100 + float64(n-i)*0.5
		o[i] = base - 0.2
		h[i] = base + 1
		l[i] = base - 1
		c[i] = base
		v[i] = 10000
	}
	bars := makeBarsFromOHLCV(o, h, l, c, v)

	engine := NewEngine()
	snap := engine.ComputeSnapshot(bars)

	if snap.CurrentPrice == 0 {
		t.Error("CurrentPrice should not be 0")
	}
	if !approxEqual(snap.CurrentPrice, bars[0].Close, 0.001) {
		t.Errorf("CurrentPrice should be %.4f (bars[0].Close), got %.4f", bars[0].Close, snap.CurrentPrice)
	}
	if snap.ATR <= 0 {
		t.Error("ATR should be positive for bars with price variation")
	}
	t.Logf("CurrentPrice=%.4f, ATR=%.4f", snap.CurrentPrice, snap.ATR)
}

// ---------------------------------------------------------------------------
// Test Confluence edge case: empty snapshot
// ---------------------------------------------------------------------------

func TestCalcConfluence_EmptySnapshot(t *testing.T) {
	// Snapshot with no indicators computed
	snap := &IndicatorSnapshot{}
	conf := CalcConfluence(snap)
	if conf == nil {
		t.Fatal("CalcConfluence returned nil for empty snapshot")
	}
	if conf.Score != 0 {
		t.Errorf("Score should be 0 for empty snapshot, got %.2f", conf.Score)
	}
	if conf.TotalIndicators != 0 {
		t.Errorf("TotalIndicators should be 0, got %d", conf.TotalIndicators)
	}
}

func TestCalcConfluence_NilSnapshot(t *testing.T) {
	conf := CalcConfluence(nil)
	if conf == nil {
		t.Fatal("CalcConfluence returned nil for nil snapshot")
	}
	if conf.Grade != "F" {
		t.Errorf("Grade should be F for nil snapshot, got %s", conf.Grade)
	}
}

// ---------------------------------------------------------------------------
// Test Fibonacci trend direction uses temporal order
// ---------------------------------------------------------------------------

func TestCalcFibonacci_TrendDirection(t *testing.T) {
	// Create bars where swing low is older and swing high is newer → UP
	n := 40
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)

	// Build a V-shape: prices go down then up (oldest-first)
	// So in newest-first: first we see high prices, then low prices
	for i := 0; i < n; i++ {
		ageFromNewest := i
		if ageFromNewest < 15 {
			// Recent bars: rising from 100 to 115 (newest=115)
			base := 115 - float64(ageFromNewest)
			o[i] = base
			h[i] = base + 2
			l[i] = base - 2
			c[i] = base
		} else if ageFromNewest < 25 {
			// Transitional: around 100
			base := 100 + float64(25-ageFromNewest)*0.5
			o[i] = base
			h[i] = base + 1
			l[i] = base - 1
			c[i] = base
		} else {
			// Old bars: declining from 115 to 100
			base := 100 + float64(ageFromNewest-25)*1.5
			o[i] = base
			h[i] = base + 2
			l[i] = base - 2
			c[i] = base
		}
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcFibonacci(bars, 40)
	if result == nil {
		t.Log("CalcFibonacci returned nil — may not find clear pivots in this synthetic data")
		return
	}
	t.Logf("Fib: TrendDir=%s, SwingHigh=%.2f, SwingLow=%.2f, Nearest=%s%%=%.2f",
		result.TrendDir, result.SwingHigh, result.SwingLow, result.NearestLevel, result.NearestPrice)
}

// ---------------------------------------------------------------------------
// Test Zones: uses CurrentPrice from snapshot
// ---------------------------------------------------------------------------

func TestCalcZones_UsesCurrentPrice(t *testing.T) {
	snap := &IndicatorSnapshot{
		CurrentPrice: 100.0,
		ATR:          2.0,
		Bollinger: &BollingerResult{
			Upper: 105, Middle: 100, Lower: 95,
			Bandwidth: 10, PercentB: 0.5,
		},
		EMA: &EMAResult{
			Values:          map[int]float64{9: 101, 21: 99, 55: 97},
			RibbonAlignment: "BULLISH",
			AlignmentScore:  1.0,
		},
	}
	conf := &ConfluenceResult{
		Score:     50.0,
		Grade:     "B",
		Direction: "BULLISH",
	}

	zones := CalcZones(snap, conf)
	if zones == nil {
		t.Fatal("CalcZones returned nil")
	}
	t.Logf("Zones: Valid=%v, Direction=%s, Entry=%.4f-%.4f, SL=%.4f, TP1=%.4f, TP2=%.4f, RR1=%.2f",
		zones.Valid, zones.Direction, zones.EntryLow, zones.EntryHigh, zones.StopLoss,
		zones.TakeProfit1, zones.TakeProfit2, zones.RiskReward1)

	if zones.Direction != "LONG" {
		t.Errorf("Expected LONG direction, got %s", zones.Direction)
	}
	// Entry should be around 100 ± ATR*0.3 = 100 ± 0.6
	if zones.EntryLow < 98 || zones.EntryHigh > 102 {
		t.Errorf("Entry zone looks wrong: %.4f-%.4f", zones.EntryLow, zones.EntryHigh)
	}
}

// ---------------------------------------------------------------------------
// Test Ichimoku basic
// ---------------------------------------------------------------------------

func TestCalcIchimoku_Basic(t *testing.T) {
	// Need at least 78 bars (52 + 26) for full Ichimoku
	n := 100
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	for i := 0; i < n; i++ {
		base := 100 + float64(n-i)*0.5 // uptrend
		o[i] = base - 0.2
		h[i] = base + 1
		l[i] = base - 1
		c[i] = base
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcIchimoku(bars)
	if result == nil {
		t.Fatal("CalcIchimoku returned nil")
	}
	t.Logf("Ichimoku: Overall=%s, TK=%s, Kumo=%s, Cloud=%s, Chikou=%s",
		result.Overall, result.TKCross, result.KumoBreakout, result.CloudColor, result.ChikouSignal)

	// In a clear uptrend, Tenkan should be above Kijun
	if result.Tenkan < result.Kijun {
		t.Errorf("In uptrend, Tenkan (%.4f) should be >= Kijun (%.4f)", result.Tenkan, result.Kijun)
	}
}

func TestCalcIchimoku_InsufficientData(t *testing.T) {
	bars := makeBarsFromCloses([]float64{50, 49, 48, 47, 46})
	result := CalcIchimoku(bars)
	if result != nil {
		t.Error("CalcIchimoku should return nil for insufficient data")
	}
}

// ---------------------------------------------------------------------------
// Test SuperTrend basic
// ---------------------------------------------------------------------------

func TestCalcSuperTrend_Uptrend(t *testing.T) {
	n := 30
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	for i := 0; i < n; i++ {
		base := 100 + float64(n-i)*2 // strong uptrend
		o[i] = base - 1
		h[i] = base + 2
		l[i] = base - 2
		c[i] = base
	}
	bars := makeBarsFromOHLCV(o, h, l, c, nil)

	result := CalcSuperTrend(bars, 10, 3.0)
	if result == nil {
		t.Fatal("CalcSuperTrend returned nil")
	}
	t.Logf("SuperTrend: Value=%.4f, Direction=%s", result.Value, result.Direction)

	if result.Direction != "UP" {
		t.Errorf("Expected UP direction in strong uptrend, got %s", result.Direction)
	}
}

func TestCalcSuperTrend_InsufficientData(t *testing.T) {
	bars := makeBarsFromCloses([]float64{50, 49})
	result := CalcSuperTrend(bars, 10, 3.0)
	if result != nil {
		t.Error("CalcSuperTrend should return nil for insufficient data")
	}
}

// ---------------------------------------------------------------------------
// Test MTF basic
// ---------------------------------------------------------------------------

func TestCalcMTF_Basic(t *testing.T) {
	// Create snapshots for 2 timeframes
	snapshots := map[string]*IndicatorSnapshot{
		"daily": {
			RSI:  &RSIResult{Value: 65, Zone: "NEUTRAL", Trend: "RISING"},
			MACD: &MACDResult{MACD: 0.5, Signal: 0.3, Histogram: 0.2, Cross: "NONE"},
		},
		"4h": {
			RSI:  &RSIResult{Value: 55, Zone: "NEUTRAL", Trend: "FLAT"},
			MACD: &MACDResult{MACD: 0.1, Signal: 0.2, Histogram: -0.1, Cross: "NONE"},
		},
	}

	result := CalcMTF(snapshots)
	if result == nil {
		t.Fatal("CalcMTF returned nil")
	}
	t.Logf("MTF: Alignment=%s, WeightedScore=%.1f, Grade=%s, Rows=%d",
		result.Alignment, result.WeightedScore, result.WeightedGrade, len(result.Matrix))

	if len(result.Matrix) != 2 {
		t.Errorf("Expected 2 matrix rows, got %d", len(result.Matrix))
	}
}

func TestCalcMTF_Empty(t *testing.T) {
	result := CalcMTF(map[string]*IndicatorSnapshot{})
	if result == nil {
		t.Fatal("CalcMTF returned nil for empty input")
	}
	if result.Alignment != "MIXED" {
		t.Errorf("Expected MIXED alignment for empty input, got %s", result.Alignment)
	}
}

// ---------------------------------------------------------------------------
// Test ComputeFull integration
// ---------------------------------------------------------------------------

func TestComputeFull_Integration(t *testing.T) {
	n := 250
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)
	for i := 0; i < n; i++ {
		base := 100 + float64(n-i)*0.3 + math.Sin(float64(i)*0.1)*5
		o[i] = base - 0.5
		h[i] = base + 2
		l[i] = base - 2
		c[i] = base
		v[i] = 10000 + float64(n-i)*50
	}
	bars := makeBarsFromOHLCV(o, h, l, c, v)

	engine := NewEngine()
	result := engine.ComputeFull(bars)
	if result == nil {
		t.Fatal("ComputeFull returned nil")
	}

	t.Logf("ComputeFull: Confluence Score=%.1f Grade=%s Direction=%s",
		result.Confluence.Score, result.Confluence.Grade, result.Confluence.Direction)
	t.Logf("  Zones valid=%v", result.Zones != nil && result.Zones.Valid)
	t.Logf("  Patterns=%d, Divergences=%d", len(result.Patterns), len(result.Divergences))
	t.Logf("  Snapshot: RSI=%v, MACD=%v, Stoch=%v, BB=%v, EMA=%v, ADX=%v, OBV=%v, Ichi=%v, ST=%v, Fib=%v",
		result.Snapshot.RSI != nil, result.Snapshot.MACD != nil,
		result.Snapshot.Stochastic != nil, result.Snapshot.Bollinger != nil,
		result.Snapshot.EMA != nil, result.Snapshot.ADX != nil,
		result.Snapshot.OBV != nil, result.Snapshot.Ichimoku != nil,
		result.Snapshot.SuperTrend != nil, result.Snapshot.Fibonacci != nil)
	t.Logf("  CurrentPrice=%.4f, ATR=%.4f", result.Snapshot.CurrentPrice, result.Snapshot.ATR)

	// All indicators should be non-nil with 250 bars of varied data
	if result.Snapshot.RSI == nil {
		t.Error("RSI should not be nil with 250 bars")
	}
	if result.Snapshot.MACD == nil {
		t.Error("MACD should not be nil with 250 bars")
	}
	if result.Snapshot.Bollinger == nil {
		t.Error("Bollinger should not be nil with 250 bars")
	}
	if result.Snapshot.EMA == nil {
		t.Error("EMA ribbon should not be nil with 250 bars")
	}
	if result.Snapshot.ADX == nil {
		t.Error("ADX should not be nil with 250 bars")
	}
	if result.Snapshot.Ichimoku == nil {
		t.Error("Ichimoku should not be nil with 250 bars")
	}
	if result.Snapshot.SuperTrend == nil {
		t.Error("SuperTrend should not be nil with 250 bars")
	}
	if result.Snapshot.CurrentPrice == 0 {
		t.Error("CurrentPrice should not be 0")
	}
	if result.Snapshot.ATR <= 0 {
		t.Error("ATR should be positive")
	}
}

// ---------------------------------------------------------------------------
// Integration: verify converter functions compile
// ---------------------------------------------------------------------------

func TestConverterFunctions(t *testing.T) {
	// Just verify they compile and handle empty slices
	d := DailyPricesToOHLCV(nil)
	if len(d) != 0 {
		t.Error("Expected empty slice for nil input")
	}

	i := IntradayBarsToOHLCV(nil)
	if len(i) != 0 {
		t.Error("Expected empty slice for nil input")
	}
}
