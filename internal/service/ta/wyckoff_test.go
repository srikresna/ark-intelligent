package ta

import (
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func wyckoffBar(o, h, l, c, v float64) OHLCV {
	return OHLCV{Date: time.Now(), Open: o, High: h, Low: l, Close: c, Volume: v}
}

// makeFlatBars returns n bars with given price and volume (neutral consolidation).
func makeFlatBars(n int, price, vol float64) []OHLCV {
	bars := make([]OHLCV, n)
	for i := range bars {
		bars[i] = wyckoffBar(price, price*1.001, price*0.999, price, vol)
	}
	return bars
}

// ─────────────────────────────────────────────────────────────────────────────
// TestInsufficientData: < 20 bars → return nil, no panic
// ─────────────────────────────────────────────────────────────────────────────

func TestWyckoff_InsufficientData(t *testing.T) {
	bars := makeFlatBars(10, 1.0800, 1000)
	result := CalcWyckoff(bars, 0.0010)
	if result != nil {
		t.Errorf("expected nil for < 20 bars, got phase %s", result.Phase)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestVolumeDecline: declining volume in range → VolumeDecl=true
// ─────────────────────────────────────────────────────────────────────────────

func TestWyckoff_VolumeDecline(t *testing.T) {
	// Build 60 bars: 50 neutral bars with high volume, then 10 recent with low volume.
	// This creates a range with declining recent volume.
	var bars []OHLCV
	basePrice := 1.0800
	atr := 0.0010

	// 10 recent low-volume bars (index 0..9, newest-first)
	for i := 0; i < 10; i++ {
		bars = append(bars, wyckoffBar(basePrice, basePrice+atr*0.5, basePrice-atr*0.5, basePrice, 200))
	}
	// 50 older high-volume bars
	for i := 0; i < 50; i++ {
		price := basePrice + float64(i%5)*0.0002
		bars = append(bars, wyckoffBar(price, price+atr*0.5, price-atr*0.5, price, 1000))
	}

	result := CalcWyckoff(bars, atr)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.TradingRange != nil && !result.TradingRange.VolumeDecl {
		t.Errorf("expected VolumeDecl=true when recent volume < 70%% of avg, got false")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestSpringDetection: wick below range low + close inside → Spring event
// ─────────────────────────────────────────────────────────────────────────────

func TestWyckoff_SpringDetection(t *testing.T) {
	// Synthesize: range 1.0750–1.0850, ATR=0.0010.
	// Inject a SC event deep in history, AR, then a spring bar.
	const (
		basePrice = 1.0800
		rangeLow  = 1.0750
		rangeHigh = 1.0850
		atr       = 0.0010
		avgVol    = 1000.0
	)

	var bars []OHLCV

	// Index 0-1: most recent neutral bars (newest-first)
	bars = append(bars, wyckoffBar(basePrice, rangeHigh, rangeLow, basePrice, avgVol))
	bars = append(bars, wyckoffBar(basePrice, rangeHigh, rangeLow, basePrice, avgVol))

	// Index 2: Spring bar — wick below rangeLow, close back above
	bars = append(bars, wyckoffBar(rangeLow, rangeLow+atr, rangeLow-atr*0.5, rangeLow+atr*0.3, avgVol*0.6))

	// Index 3-9: consolidation in range
	for i := 0; i < 7; i++ {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh, rangeLow, basePrice, avgVol*0.8))
	}

	// Index 10-14: AR bounce (post-SC)
	for i := 0; i < 5; i++ {
		p := rangeLow + float64(i)*atr*0.4
		bars = append(bars, wyckoffBar(p, p+atr*0.3, p-atr*0.2, p+atr*0.2, avgVol*1.2))
	}

	// Index 15-24: SC area — large bearish bars with high volume
	for i := 0; i < 10; i++ {
		p := rangeLow - float64(i)*atr*0.3
		bars = append(bars, wyckoffBar(p+atr*2.1, p+atr*2.2, p-atr*0.1, p, avgVol*2.5))
	}

	// Index 25-59: older neutral bars to reach minimum count
	for len(bars) < 60 {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh, rangeLow, basePrice, avgVol))
	}

	tr := &TradingRange{High: rangeHigh, Low: rangeLow}
	events := wyckoffDetectEvents(bars, atr, avgVol, tr)

	hasSpring := wyckoffHasEvent(events, "SPRING")
	if !hasSpring {
		t.Errorf("expected SPRING event to be detected; events found: %v", eventNames(events))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAccumulationPhase: synthesize SC + AR + Spring → detect ACCUMULATION
// ─────────────────────────────────────────────────────────────────────────────

func TestWyckoff_AccumulationPhase(t *testing.T) {
	const (
		basePrice = 1.0800
		rangeLow  = 1.0700
		rangeHigh = 1.0900
		atr       = 0.0010
		avgVol    = 1000.0
	)

	var bars []OHLCV

	// Newest bars: still inside range, very low volume (ensures VolumeDecl=true)
	for i := 0; i < 5; i++ {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh-atr*0.3, rangeLow+atr*0.3, basePrice, avgVol*0.2))
	}

	// Spring bar
	bars = append(bars, wyckoffBar(rangeLow, rangeLow+atr*0.5, rangeLow-atr*0.3, rangeLow+atr*0.2, avgVol*0.6))

	// Consolidation in range
	for i := 0; i < 9; i++ {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh-atr, rangeLow+atr, basePrice, avgVol*0.6))
	}

	// AR (automatic rally) — bounce after SC
	for i := 0; i < 5; i++ {
		p := rangeLow + atr*float64(i+1)*1.5
		bars = append(bars, wyckoffBar(p-atr*0.5, p+atr*0.3, p-atr*0.5, p, avgVol*1.1))
	}

	// SC zone — large bearish, very high volume
	for i := 0; i < 10; i++ {
		p := rangeLow - atr*float64(i)*0.2
		bars = append(bars, wyckoffBar(p+atr*2.5, p+atr*2.6, p-atr*0.2, p, avgVol*2.8))
	}

	// Older range bars to fill minimum
	for len(bars) < 60 {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh, rangeLow, basePrice, avgVol*0.7))
	}

	result := CalcWyckoff(bars, atr)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Phase != PhaseAccumulation {
		t.Errorf("expected ACCUMULATION, got %s (bias=%s, conf=%.2f)", result.Phase, result.Bias, result.PhaseConf)
	}
	if result.Bias != "BULLISH" {
		t.Errorf("expected BULLISH bias, got %s", result.Bias)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMarkupPhase: SOS + price above range → detect MARKUP
// ─────────────────────────────────────────────────────────────────────────────

func TestWyckoff_MarkupPhase(t *testing.T) {
	const (
		basePrice = 1.0800
		rangeLow  = 1.0700
		rangeHigh = 1.0900
		atr       = 0.0010
		avgVol    = 1000.0
	)

	var bars []OHLCV

	// Newest bars: price above range high (markup), high volume
	for i := 0; i < 5; i++ {
		p := rangeHigh + atr*float64(i+1)*0.8
		bars = append(bars, wyckoffBar(p-atr*0.3, p+atr*0.5, p-atr*0.4, p, avgVol*1.8))
	}

	// SOS bar: strong bullish close above AR high with high volume
	sosClose := rangeHigh + atr*0.5
	bars = append(bars, wyckoffBar(rangeHigh-atr*0.2, sosClose+atr*0.3, rangeHigh-atr*0.3, sosClose, avgVol*2.0))

	// Range consolidation leading up to SOS
	for i := 0; i < 14; i++ {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh, rangeLow, basePrice, avgVol*0.7))
	}

	// AR zone
	for i := 0; i < 5; i++ {
		p := rangeLow + atr*float64(i+1)
		bars = append(bars, wyckoffBar(p-atr*0.5, p+atr*0.3, p-atr*0.5, p, avgVol*1.1))
	}

	// SC zone
	for i := 0; i < 10; i++ {
		p := rangeLow - atr*float64(i)*0.2
		bars = append(bars, wyckoffBar(p+atr*2.5, p+atr*2.6, p-atr*0.2, p, avgVol*2.8))
	}

	// Fill remaining
	for len(bars) < 60 {
		bars = append(bars, wyckoffBar(basePrice, basePrice+atr*0.5, basePrice-atr*0.5, basePrice, avgVol*0.7))
	}

	result := CalcWyckoff(bars, atr)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Phase != PhaseMarkup {
		t.Errorf("expected MARKUP, got %s (conf=%.2f)", result.Phase, result.PhaseConf)
	}
	if result.Bias != "BULLISH" {
		t.Errorf("expected BULLISH bias, got %s", result.Bias)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDistributionPhase: upthrust + declining vol → detect DISTRIBUTION
// ─────────────────────────────────────────────────────────────────────────────

func TestWyckoff_DistributionPhase(t *testing.T) {
	const (
		basePrice = 1.0800
		rangeLow  = 1.0700
		rangeHigh = 1.0900
		atr       = 0.0010
		avgVol    = 1000.0
	)

	var bars []OHLCV

	// Newest bars: inside range, very low volume (ensures VolumeDecl=true for distribution)
	for i := 0; i < 5; i++ {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh-atr*0.2, rangeLow+atr*0.2, basePrice, avgVol*0.2))
	}

	// Upthrust (UT): wick above rangeHigh, close back inside
	bars = append(bars, wyckoffBar(rangeHigh-atr*0.3, rangeHigh+atr*0.4, rangeHigh-atr*0.4, rangeHigh-atr*0.2, avgVol*1.1))

	// Consolidation in range with low volume
	for i := 0; i < 14; i++ {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh-atr, rangeLow+atr, basePrice, avgVol*0.5))
	}

	// Buying Climax / SC area (high volume selling at top)
	for i := 0; i < 10; i++ {
		p := rangeHigh - atr*float64(i)*0.1
		bars = append(bars, wyckoffBar(p-atr*2.5, p-atr*0.2, p-atr*2.6, p-atr*2.2, avgVol*2.8))
	}

	// Fill remaining
	for len(bars) < 60 {
		bars = append(bars, wyckoffBar(basePrice, rangeHigh, rangeLow, basePrice, avgVol*0.5))
	}

	result := CalcWyckoff(bars, atr)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Phase != PhaseDistribution {
		t.Errorf("expected DISTRIBUTION, got %s (conf=%.2f)", result.Phase, result.PhaseConf)
	}
	if result.Bias != "BEARISH" {
		t.Errorf("expected BEARISH bias, got %s", result.Bias)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func eventNames(events []WyckoffEvent) []string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.Name
	}
	return names
}
