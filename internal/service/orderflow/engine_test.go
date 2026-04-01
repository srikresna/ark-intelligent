package orderflow

import (
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// helpers

func bar(o, h, l, c, v float64) ta.OHLCV {
	return ta.OHLCV{
		Date:   time.Now(),
		Open:   o,
		High:   h,
		Low:    l,
		Close:  c,
		Volume: v,
	}
}

// ---------------------------------------------------------------------------
// tickRuleSplit tests
// ---------------------------------------------------------------------------

func TestTickRuleSplit_BullishBar(t *testing.T) {
	// Close at the top → almost all buy volume.
	b := bar(1.0, 1.1, 1.0, 1.1, 1000)
	buy, sell := tickRuleSplit(b)

	if buy+sell > 1001 || buy+sell < 999 {
		t.Fatalf("buy+sell=%.2f, expected ~1000", buy+sell)
	}
	if buy <= sell {
		t.Errorf("expected buy (%.2f) > sell (%.2f) for bullish bar at high", buy, sell)
	}
}

func TestTickRuleSplit_BearishBar(t *testing.T) {
	// Close at the bottom → almost all sell volume.
	b := bar(1.1, 1.1, 1.0, 1.0, 1000)
	buy, sell := tickRuleSplit(b)

	if buy+sell > 1001 || buy+sell < 999 {
		t.Fatalf("buy+sell=%.2f, expected ~1000", buy+sell)
	}
	if sell <= buy {
		t.Errorf("expected sell (%.2f) > buy (%.2f) for bearish bar at low", sell, buy)
	}
}

func TestTickRuleSplit_ZeroRange(t *testing.T) {
	// Doji: High == Low → 50/50 split.
	b := bar(1.05, 1.05, 1.05, 1.05, 1000)
	buy, sell := tickRuleSplit(b)

	if math.Abs(buy-sell) > 0.01 {
		t.Errorf("expected 50/50 split on doji, got buy=%.2f sell=%.2f", buy, sell)
	}
}

func TestTickRuleSplit_ZeroVolume(t *testing.T) {
	b := bar(1.0, 1.1, 0.9, 1.05, 0)
	buy, sell := tickRuleSplit(b)
	if buy != 0 || sell != 0 {
		t.Errorf("expected 0/0 for zero-volume bar, got buy=%.2f sell=%.2f", buy, sell)
	}
}

// ---------------------------------------------------------------------------
// estimateDeltaBars tests
// ---------------------------------------------------------------------------

func TestEstimateDeltaBars_CumDeltaConsistency(t *testing.T) {
	// 5 bars (newest-first) all bullish.
	bars := []ta.OHLCV{
		bar(1.00, 1.05, 0.99, 1.04, 100), // newest
		bar(0.98, 1.02, 0.97, 1.01, 120),
		bar(0.95, 0.99, 0.94, 0.98, 90),
		bar(0.93, 0.97, 0.92, 0.96, 110),
		bar(0.90, 0.94, 0.89, 0.93, 80), // oldest
	}

	db := estimateDeltaBars(bars)
	if len(db) != 5 {
		t.Fatalf("expected 5 delta bars, got %d", len(db))
	}

	// CumDelta[0] must be the grand total of all bar deltas.
	total := 0.0
	for _, d := range db {
		total += d.Delta
	}
	if math.Abs(db[0].CumDelta-total) > 0.01 {
		t.Errorf("CumDelta[0]=%.4f but sum of deltas=%.4f", db[0].CumDelta, total)
	}
}

// ---------------------------------------------------------------------------
// Analyze integration test
// ---------------------------------------------------------------------------

func TestAnalyze_ReturnsBias(t *testing.T) {
	// Build 10 consistently bullish bars (close near high each bar).
	bars := make([]ta.OHLCV, 10)
	price := 1.1000
	for i := range bars {
		o := price - 0.002
		h := price + 0.003
		l := price - 0.003
		c := price + 0.002 // close near high = buyer dominated
		bars[i] = bar(o, h, l, c, 500)
		price -= 0.005 // simulate older bars having lower price
	}

	result := Analyze(bars, "EURUSD", "4h")
	if result == nil {
		t.Fatal("expected non-nil OrderFlowResult")
	}
	if result.Bias == "" {
		t.Error("expected non-empty Bias")
	}
	if result.PointOfControl == 0 {
		t.Error("expected non-zero PointOfControl")
	}
	if result.Summary == "" {
		t.Error("expected non-empty Summary")
	}
}

func TestAnalyze_TooFewBars(t *testing.T) {
	bars := []ta.OHLCV{
		bar(1.0, 1.1, 0.9, 1.05, 100),
		bar(0.9, 1.0, 0.8, 0.95, 90),
	}
	result := Analyze(bars, "EURUSD", "4h")
	if result != nil {
		t.Error("expected nil result for fewer than 3 bars")
	}
}

// ---------------------------------------------------------------------------
// detectDivergence test
// ---------------------------------------------------------------------------

func TestDetectDivergence_BullishDiv(t *testing.T) {
	// Price: lower low in the second half vs the first, but cumulative delta higher.
	// To trigger bullish divergence: newest close < prior low close,
	// but newest cumDelta > prior low cumDelta.
	//
	// We construct bars manually with explicit CumDelta values.
	bars := []DeltaBar{
		{OHLCV: ta.OHLCV{Close: 1.00}, CumDelta: 500},  // newest — lower close, higher cum
		{OHLCV: ta.OHLCV{Close: 1.01}, CumDelta: 400},
		{OHLCV: ta.OHLCV{Close: 1.02}, CumDelta: 300},
		{OHLCV: ta.OHLCV{Close: 1.03}, CumDelta: 200},
		{OHLCV: ta.OHLCV{Close: 1.05}, CumDelta: 100},  // prior section: high close, low cum
		{OHLCV: ta.OHLCV{Close: 1.04}, CumDelta: 50},
	}
	got := detectDivergence(bars)
	if got != "BULLISH_DIV" {
		t.Errorf("expected BULLISH_DIV, got %s", got)
	}
}

// ---------------------------------------------------------------------------
// medianFloat test
// ---------------------------------------------------------------------------

func TestMedianFloat_OddLength(t *testing.T) {
	vals := []float64{3, 1, 2}
	if m := medianFloat(vals); m != 2 {
		t.Errorf("expected median 2, got %f", m)
	}
}

func TestMedianFloat_EvenLength(t *testing.T) {
	vals := []float64{4, 1, 3, 2}
	if m := medianFloat(vals); m != 2.5 {
		t.Errorf("expected median 2.5, got %f", m)
	}
}
