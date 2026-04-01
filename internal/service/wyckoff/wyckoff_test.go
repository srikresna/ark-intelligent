package wyckoff

import (
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// bar is a shorthand for building a ta.OHLCV.
func bar(o, h, l, c, v float64) ta.OHLCV {
	return ta.OHLCV{Date: time.Now(), Open: o, High: h, Low: l, Close: c, Volume: v}
}

// makeBars returns n neutral bars with the given base price and average volume.
func makeBars(n int, price, vol float64) []ta.OHLCV {
	bars := make([]ta.OHLCV, n)
	for i := range bars {
		bars[i] = bar(price, price*1.002, price*0.998, price, vol)
	}
	return bars
}

// newestFirst reverses a slice so newest bar is at index 0.
func newestFirst(bars []ta.OHLCV) []ta.OHLCV {
	n := len(bars)
	out := make([]ta.OHLCV, n)
	for i, b := range bars {
		out[n-1-i] = b
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: insufficient data returns LOW / UNKNOWN
// ─────────────────────────────────────────────────────────────────────────────

func TestAnalyze_InsufficientData(t *testing.T) {
	eng := NewEngine()
	bars := makeBars(30, 1.0800, 1000) // only 30 bars, need 50+
	result := eng.Analyze("EURUSD", "H4", newestFirst(bars))

	if result.Schematic != "UNKNOWN" {
		t.Errorf("expected UNKNOWN schematic, got %s", result.Schematic)
	}
	if result.Confidence != "LOW" {
		t.Errorf("expected LOW confidence, got %s", result.Confidence)
	}
	if result.CurrentPhase != "UNDEFINED" {
		t.Errorf("expected UNDEFINED phase, got %s", result.CurrentPhase)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Phase A events (SC, AR, ST) detected on synthetic accumulation data
// ─────────────────────────────────────────────────────────────────────────────

// buildAccumulationBars builds a 120-bar synthetic accumulation pattern
// (oldest-first). The result is returned oldest-first for internal use and
// reversed for input to Analyze.
func buildAccumulationBars() []ta.OHLCV {
	avgVol := 1000.0
	bars := make([]ta.OHLCV, 0, 120)

	// Bars 0-29: gradual downtrend leading to SC
	price := 1.1000
	for i := 0; i < 30; i++ {
		price -= 0.0008
		v := avgVol * 0.9
		if i == 15 { // preliminary support attempt
			v = avgVol * 1.5
		}
		bars = append(bars, bar(price+0.001, price+0.002, price-0.001, price, v))
	}

	// Bar 30: Selling Climax — high volume, wide range, bearish close
	scPrice := price - 0.015
	bars = append(bars, bar(price, price+0.001, scPrice-0.002, scPrice, avgVol*3.0))
	scIdx := 30

	// Bars 31-35: Automatic Rally
	arPrice := scPrice + 0.012
	for i := 31; i <= 35; i++ {
		p := scPrice + float64(i-30)*0.002
		bars = append(bars, bar(p-0.001, p+0.001, p-0.002, p, avgVol*0.8))
	}
	_ = scIdx

	// Bars 36-50: Range bound (Phase B)
	for i := 36; i <= 50; i++ {
		p := arPrice - 0.001
		bars = append(bars, bar(p, p+0.001, p-0.001, p, avgVol*0.7))
	}

	// Bar 51: Secondary Test — low volume, near SC low
	stPrice := scPrice + 0.001
	bars = append(bars, bar(stPrice+0.001, stPrice+0.003, stPrice-0.001, stPrice, avgVol*0.5))

	// Bars 52-65: More range trading (Phase B continued)
	for i := 52; i <= 65; i++ {
		p := arPrice - 0.002
		bars = append(bars, bar(p, p+0.001, p-0.001, p, avgVol*0.6))
	}

	// Bar 66: Spring — brief break below SC low, low volume
	springPrice := scPrice - 0.003
	bars = append(bars, bar(springPrice+0.002, springPrice+0.003, springPrice-0.001, springPrice, avgVol*0.7))

	// Bars 67-68: Recovery above SC (spring recovery)
	for i := 67; i <= 68; i++ {
		p := scPrice + 0.005
		bars = append(bars, bar(p, p+0.002, p-0.001, p, avgVol*0.9))
	}

	// Bars 69-80: Range trading after spring
	for i := 69; i <= 80; i++ {
		p := arPrice - 0.002
		bars = append(bars, bar(p, p+0.001, p-0.001, p, avgVol*0.7))
	}

	// Bar 81: Sign of Strength — high volume break above AR high
	sosPrice := arPrice + 0.005
	bars = append(bars, bar(arPrice, sosPrice+0.002, arPrice-0.001, sosPrice, avgVol*2.2))

	// Bars 82-90: Markup / LPS
	for i := 82; i <= 90; i++ {
		p := sosPrice + float64(i-81)*0.001
		v := avgVol
		if i == 85 { // LPS: pullback to prior AR high with low volume
			p = arPrice + 0.001
			v = avgVol * 0.5
		}
		bars = append(bars, bar(p, p+0.001, p-0.001, p, v))
	}

	// Bars 91-119: Additional markup bars to reach 120
	p := sosPrice + 0.015
	for i := 90; i < 120; i++ {
		p += 0.0005
		bars = append(bars, bar(p, p+0.001, p-0.001, p, avgVol))
	}

	// Trim or pad to exactly 120.
	if len(bars) > 120 {
		bars = bars[:120]
	}
	for len(bars) < 120 {
		bars = append(bars, bar(p, p+0.001, p-0.001, p, avgVol))
	}
	return bars
}

func TestAnalyze_AccumulationPhaseAEvents(t *testing.T) {
	eng := NewEngine()
	oldestFirst := buildAccumulationBars()
	input := newestFirst(oldestFirst)
	result := eng.Analyze("EURUSD", "H4", input)

	// We expect at least one of SC, AR, or ST to be detected.
	eventNames := make(map[EventName]bool)
	for _, e := range result.Events {
		eventNames[e.Name] = true
	}

	wantAtLeastOne := []EventName{EventSC, EventAR}
	found := false
	for _, name := range wantAtLeastOne {
		if eventNames[name] {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least SC or AR event, got events: %v", eventNames)
	}

	// Schematic should be ACCUMULATION or at worst UNKNOWN (not DISTRIBUTION).
	if result.Schematic == "DISTRIBUTION" {
		t.Errorf("expected ACCUMULATION or UNKNOWN schematic, got DISTRIBUTION")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: volume analysis — high volume at lows biases toward accumulation
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifySchematic_VolumeAbsorption(t *testing.T) {
	// Build bars: 60 bars with high volume when price is low.
	bars := make([]ta.OHLCV, 60)
	for i := 0; i < 60; i++ {
		price := 1.0 + float64(i)*0.001
		vol := 500.0
		if price < 1.030 { // lower-half prices get higher volume
			vol = 2000.0
		}
		bars[i] = bar(price, price+0.001, price-0.001, price, vol)
	}

	av := avgVolume(bars)
	events := []WyckoffEvent{
		{Name: EventSC, BarIndex: 5, Price: 1.005, Volume: av * 2},
		{Name: EventAR, BarIndex: 15, Price: 1.020, Volume: av * 0.8},
	}
	schematic, _, _ := classifySchematic(bars, events, av)
	if schematic != "ACCUMULATION" {
		t.Errorf("expected ACCUMULATION (volume absorption at lows), got %s", schematic)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: projected move is proportional to trading range width
// ─────────────────────────────────────────────────────────────────────────────

func TestProjectedMove(t *testing.T) {
	tr := [2]float64{1.0800, 1.1000} // 200-pip range
	cause := 80.0                     // 80% cause score → multiplier 1.3
	move := projectedMove(tr, cause)

	rangeW := tr[1] - tr[0]
	expected := rangeW * (0.5 + cause/100)

	if math.Abs(move-expected) > 1e-9 {
		t.Errorf("projectedMove = %.6f, want %.6f", move, expected)
	}
	if move <= 0 {
		t.Error("projected move should be positive")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: TradingRange returns sensible [lo, hi] for detected events
// ─────────────────────────────────────────────────────────────────────────────

func TestTradingRange_Accumulation(t *testing.T) {
	bars := makeBars(60, 1.0900, 1000)
	// Fake SC at bar 10 (low = 1.0750) and AR at bar 20 (high = 1.0950)
	bars[10].Low = 1.0750
	bars[20].High = 1.0950

	events := []WyckoffEvent{
		{Name: EventSC, BarIndex: 10, Price: 1.0780, Volume: 2000},
		{Name: EventAR, BarIndex: 20, Price: 1.0940, Volume: 800},
	}

	tr := tradingRangeFor(bars, events)
	// Support should come from SC bar's low
	if tr[0] != 1.0750 {
		t.Errorf("expected support=1.0750, got %.4f", tr[0])
	}
	// Resistance should come from AR bar's high
	if tr[1] != 1.0950 {
		t.Errorf("expected resistance=1.0950, got %.4f", tr[1])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test: Spring detection (low breaks SC low then recovers)
// ─────────────────────────────────────────────────────────────────────────────

func TestDetectSpring(t *testing.T) {
	avgVol := 1000.0
	bars := makeBars(80, 1.0900, avgVol)

	// SC at bar 10
	bars[10].Low = 1.0700
	bars[10].Close = 1.0710
	bars[10].Volume = avgVol * 2.5

	// AR at bar 20
	bars[20].High = 1.0870
	bars[20].Close = 1.0860

	// Spring at bar 35: low breaks SC low (1.0700) then recovers above it
	bars[35].Low = 1.0695
	bars[35].Close = 1.0695
	bars[35].Volume = avgVol * 0.8 // low vol = good spring

	// Recovery bar 36
	bars[36].Close = 1.0720
	bars[36].High = 1.0730

	spring := detectSpring(bars, 10, 20, avgVol)
	if spring == nil {
		t.Fatal("expected Spring to be detected, got nil")
	}
	if spring.Name != EventSpring {
		t.Errorf("expected EventSpring, got %s", spring.Name)
	}
	if spring.BarIndex != 35 {
		t.Errorf("expected spring at bar 35, got bar %d", spring.BarIndex)
	}
}
