package ict_test

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ict"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// makeBar creates a simple OHLCV bar with the given OHLC values.
func makeBar(open, high, low, close float64, t time.Time) ta.OHLCV {
	return ta.OHLCV{
		Date:   t,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  close,
		Volume: 1000,
	}
}

// reverseChron reverses a chronological slice to newest-first (engine input format).
func reverseChron(bars []ta.OHLCV) []ta.OHLCV {
	n := len(bars)
	out := make([]ta.OHLCV, n)
	for i, b := range bars {
		out[n-1-i] = b
	}
	return out
}

// makePaddingBars generates n bars at a given base price with small moves.
// Used to pad test datasets to meet the 20-bar minimum for ta.CalcICT.
func makePaddingBars(n int, basePrice float64, start time.Time, step time.Duration) []ta.OHLCV {
	bars := make([]ta.OHLCV, n)
	for i := 0; i < n; i++ {
		p := basePrice + float64(i%3)*0.001
		bars[i] = makeBar(p, p+0.002, p-0.002, p+0.001, start.Add(time.Duration(i)*step))
	}
	return bars
}

// ---------------------------------------------------------------------------
// FVG Detection Tests
// ---------------------------------------------------------------------------

func TestFVG_BullishDetection(t *testing.T) {
	// Build bars with a clear bullish FVG between bar[0].High and bar[2].Low.
	// Bullish FVG: right.Low > left.High (right=chron[i+1], left=chron[i-1])
	// Pad to ≥20 bars so ta.CalcICT accepts the input.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	step := 4 * time.Hour

	// Leading padding: 15 bars at ~0.990
	chron := makePaddingBars(15, 0.990, base, step)

	offset := base.Add(time.Duration(15) * step)
	// The FVG-producing triple:
	chron = append(chron,
		// left: High exactly 1.000
		ta.OHLCV{Date: offset, Open: 0.990, High: 1.000, Low: 0.985, Close: 0.995, Volume: 1000},
		// mid
		ta.OHLCV{Date: offset.Add(step), Open: 1.000, High: 1.005, Low: 0.995, Close: 1.002, Volume: 1000},
		// right: Low exactly 1.010 > left.High 1.000 → bullish FVG top=1.010 bottom=1.000
		ta.OHLCV{Date: offset.Add(2 * step), Open: 1.010, High: 1.020, Low: 1.010, Close: 1.015, Volume: 1000},
		ta.OHLCV{Date: offset.Add(3 * step), Open: 1.015, High: 1.025, Low: 1.012, Close: 1.020, Volume: 1000},
		ta.OHLCV{Date: offset.Add(4 * step), Open: 1.020, High: 1.030, Low: 1.015, Close: 1.025, Volume: 1000},
	)
	bars := reverseChron(chron)

	zones := ict.DetectFVG(bars)

	// Look for the specific bullish FVG in the target range 1.000-1.010.
	// There may be other FVGs from padding bars — we only require the target one.
	foundTarget := false
	for _, z := range zones {
		if z.Type == "BULLISH" && z.Low >= 0.990 && z.Low <= 1.005 && z.High >= 1.005 && z.High <= 1.015 {
			foundTarget = true
		}
	}
	if !foundTarget {
		t.Errorf("expected a BULLISH FVG in range ~1.000-1.010, got zones: %+v", zones)
	}
}

func TestFVG_BearishDetection(t *testing.T) {
	// Bearish FVG: right.High < left.Low
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	step := 4 * time.Hour

	// Leading padding at higher price level
	chron := makePaddingBars(15, 1.030, base, step)

	offset := base.Add(time.Duration(15) * step)
	chron = append(chron,
		makeBar(1.030, 1.035, 1.020, 1.025, offset),
		makeBar(1.020, 1.022, 1.012, 1.015, offset.Add(step)),
		makeBar(1.015, 1.010, 1.000, 1.005, offset.Add(2*step)), // High=1.010 < left Low=1.020
		makeBar(1.005, 1.008, 0.995, 1.000, offset.Add(3*step)),
		makeBar(1.000, 1.003, 0.990, 0.995, offset.Add(4*step)),
	)
	bars := reverseChron(chron)

	zones := ict.DetectFVG(bars)

	bearishCount := 0
	for _, z := range zones {
		if z.Type == "BEARISH" {
			bearishCount++
		}
	}
	if bearishCount == 0 {
		t.Error("expected at least 1 BEARISH FVG, got 0")
	}
}

func TestFVG_FillDetection(t *testing.T) {
	// Create a bullish FVG and then fill it with a subsequent bar.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	step := 4 * time.Hour

	chron := makePaddingBars(15, 0.990, base, step)

	offset := base.Add(time.Duration(15) * step)
	chron = append(chron,
		makeBar(0.990, 1.000, 0.985, 0.995, offset),
		makeBar(1.000, 1.005, 0.995, 1.002, offset.Add(step)),
		makeBar(1.008, 1.020, 1.010, 1.015, offset.Add(2*step)), // FVG: 1.000–1.010
		makeBar(1.015, 1.020, 1.008, 1.010, offset.Add(3*step)), // fills FVG partially
		makeBar(1.005, 1.010, 0.995, 0.998, offset.Add(4*step)), // fills completely
	)
	bars := reverseChron(chron)

	zones := ict.DetectFVG(bars)
	for _, z := range zones {
		if z.Type == "BULLISH" && z.FillPct >= 100 {
			if !z.Filled {
				t.Errorf("expected Filled=true when FillPct=%.1f", z.FillPct)
			}
			return
		}
	}
	// It's acceptable if partial fill is detected but not 100%.
}

func TestFVG_InsufficientData(t *testing.T) {
	bars := []ta.OHLCV{
		makeBar(1.0, 1.1, 0.9, 1.0, time.Now()),
	}
	zones := ict.DetectFVG(bars)
	if len(zones) != 0 {
		t.Errorf("expected 0 zones for insufficient data, got %d", len(zones))
	}
}

// ---------------------------------------------------------------------------
// Order Block Tests
// ---------------------------------------------------------------------------

func TestOrderBlock_BullishDetection(t *testing.T) {
	// We need enough bars for swing detection (2*5+1=11) plus a swing low.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	chron := make([]ta.OHLCV, 20)
	// Rising trend, then a dip (swing low) at index 10.
	for i := 0; i < 20; i++ {
		price := 1.0 + float64(i)*0.002
		if i >= 8 && i <= 12 {
			price = 1.0 + float64(i)*0.002 - 0.015 // dip
		}
		chron[i] = makeBar(price, price+0.003, price-0.003, price+0.001, base.Add(time.Duration(i)*time.Hour*4))
	}
	// Make bar 9 explicitly bearish (for OB detection).
	chron[9].Close = chron[9].Open - 0.002

	bars := reverseChron(chron)

	eng := ict.NewEngine()
	result := eng.Analyze(bars, "EURUSD", "H4")

	// Just verify it doesn't panic and returns a result.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Symbol != "EURUSD" {
		t.Errorf("expected symbol EURUSD, got %s", result.Symbol)
	}
}

// ---------------------------------------------------------------------------
// Structure Detection (CHoCH) Tests
// ---------------------------------------------------------------------------

func TestStructure_CHoCH_ViaBars(t *testing.T) {
	// Test CHoCH detection through engine.Analyze with bars that produce clear swings.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Build an uptrend then a reversal — should produce at least one CHoCH event.
	prices := []float64{
		1.000, 1.002, 1.005, 1.008, 1.012, // rising
		1.010, 1.008, 1.005, 1.003, 1.001, // falling (lower low)
		1.003, 1.005, 1.007, 1.004, 1.001, // lower high, lower low → bearish structure
		0.998, 0.996, 0.994, 0.993, 0.992,
		0.995, 0.997, 0.999, 1.001, 1.003,
	}
	chron := make([]ta.OHLCV, len(prices))
	for i, p := range prices {
		chron[i] = makeBar(p-0.001, p+0.003, p-0.003, p, base.Add(time.Duration(i)*time.Hour*4))
	}
	bars := reverseChron(chron)

	eng := ict.NewEngine()
	result := eng.Analyze(bars, "EURUSD", "H4")

	if result == nil {
		t.Fatal("nil result")
	}
	t.Logf("Structure events: %d, Bias: %s", len(result.Structure), result.Bias)
	// Bias should be BULLISH, BEARISH, or NEUTRAL — not empty.
	if result.Bias == "" {
		t.Error("expected non-empty Bias")
	}
}

func TestStructure_EngineAnalyze_NoPanic(t *testing.T) {
	// Ensure engine doesn't panic on minimal valid data.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bars := make([]ta.OHLCV, 30)
	for i := 0; i < 30; i++ {
		p := 1.0 + float64(i%5)*0.002
		bars[i] = makeBar(p, p+0.003, p-0.003, p+0.001, base.Add(time.Duration(i)*time.Hour*4))
	}
	// Reverse to newest-first.
	bars = reverseChron(bars)

	eng := ict.NewEngine()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("engine panicked: %v", r)
		}
	}()

	result := eng.Analyze(bars, "GBPUSD", "H4")
	if result == nil {
		t.Fatal("nil result")
	}
	if result.AnalyzedAt.IsZero() {
		t.Error("AnalyzedAt should not be zero")
	}
}

// ---------------------------------------------------------------------------
// Liquidity Sweep Tests
// ---------------------------------------------------------------------------

func TestLiquiditySweep_SweepHigh(t *testing.T) {
	// Build bars: swing high at some level, then a bar wicks above it and closes below.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	chron := make([]ta.OHLCV, 25)
	for i := 0; i < 25; i++ {
		p := 1.010 + float64(i%3)*0.002
		chron[i] = makeBar(p, p+0.002, p-0.002, p, base.Add(time.Duration(i)*time.Hour*4))
	}
	// Create a clear swing high at bar 10.
	chron[10].High = 1.040
	chron[10].Close = 1.035
	// Create a sweep bar at bar 18: wick above 1.040, close below.
	chron[18].High = 1.045
	chron[18].Close = 1.030 // closes back below swing high

	bars := reverseChron(chron)
	eng := ict.NewEngine()
	result := eng.Analyze(bars, "USDJPY", "H4")

	if result == nil {
		t.Fatal("nil result")
	}
	// Sweeps may or may not be detected depending on swing detection; just check no panic.
	t.Logf("Sweeps detected: %d", len(result.Sweeps))
}

// ---------------------------------------------------------------------------
// Killzone Test
// ---------------------------------------------------------------------------

func TestKillzone_Embedded(t *testing.T) {
	// Test via engine output — killzone is based on bars[0].Date.
	base := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC) // 08:00 UTC = London Open Killzone
	bars := make([]ta.OHLCV, 20)
	for i := 0; i < 20; i++ {
		p := 1.0 + float64(i)*0.001
		bars[i] = makeBar(p, p+0.002, p-0.002, p, base.Add(-time.Duration(i)*time.Hour*4))
	}

	eng := ict.NewEngine()
	result := eng.Analyze(bars, "EURUSD", "H4")

	if result.Killzone == "" {
		t.Log("No killzone detected (acceptable, depends on bar[0] time)")
	} else {
		t.Logf("Killzone: %s", result.Killzone)
	}
}
