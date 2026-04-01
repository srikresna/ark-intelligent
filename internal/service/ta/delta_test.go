package ta

import (
	"math"
	"testing"
	"time"
)

// buildBars creates a slice of OHLCV bars (newest-first) with specified closes.
// closes[0] = most recent close, closes[n-1] = oldest close.
func buildBars(closes []float64) []OHLCV {
	bars := make([]OHLCV, len(closes))
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	for i, c := range closes {
		bars[i] = OHLCV{
			Date:   base.Add(-time.Duration(i) * time.Hour),
			Open:   c,
			High:   c * 1.001,
			Low:    c * 0.999,
			Close:  c,
			Volume: 100,
		}
	}
	return bars
}

func TestCalcDelta_TooFewBars(t *testing.T) {
	if CalcDelta(nil) != nil {
		t.Error("expected nil for nil bars")
	}
	if CalcDelta([]OHLCV{{Close: 1}}) != nil {
		t.Error("expected nil for single bar")
	}
}

func TestCalcDelta_AllRising(t *testing.T) {
	// Newest bar has highest close → all bars are rising → all deltas positive.
	closes := []float64{1.10, 1.09, 1.08, 1.07, 1.06} // newest-first
	bars := buildBars(closes)
	d := CalcDelta(bars)
	if d == nil {
		t.Fatal("expected non-nil DeltaResult")
	}
	if d.CumulativeDelta <= 0 {
		t.Errorf("expected positive cumulative delta for all-rising prices, got %.0f", d.CumulativeDelta)
	}
	if d.Bias != "BUYING_PRESSURE" {
		t.Errorf("expected BUYING_PRESSURE bias, got %s", d.Bias)
	}
}

func TestCalcDelta_AllFalling(t *testing.T) {
	// Newest bar has lowest close → all bars falling → all deltas negative.
	closes := []float64{1.00, 1.01, 1.02, 1.03, 1.04} // newest-first
	bars := buildBars(closes)
	d := CalcDelta(bars)
	if d == nil {
		t.Fatal("expected non-nil DeltaResult")
	}
	if d.CumulativeDelta >= 0 {
		t.Errorf("expected negative cumulative delta for all-falling prices, got %.0f", d.CumulativeDelta)
	}
	if d.Bias != "SELLING_PRESSURE" {
		t.Errorf("expected SELLING_PRESSURE bias, got %s", d.Bias)
	}
}

func TestCalcDelta_FlatPrices(t *testing.T) {
	// All same price → zero delta everywhere.
	closes := []float64{1.05, 1.05, 1.05, 1.05, 1.05}
	bars := buildBars(closes)
	d := CalcDelta(bars)
	if d == nil {
		t.Fatal("expected non-nil DeltaResult")
	}
	if math.Abs(d.CumulativeDelta) > 1e-9 {
		t.Errorf("expected zero cumulative delta for flat prices, got %.6f", d.CumulativeDelta)
	}
	if d.Bias != "NEUTRAL" {
		t.Errorf("expected NEUTRAL bias, got %s", d.Bias)
	}
	if d.DeltaDivergence != "NONE" {
		t.Errorf("expected NONE divergence, got %s", d.DeltaDivergence)
	}
}

func TestCalcDelta_BarsUsed(t *testing.T) {
	closes := []float64{1.1, 1.09, 1.08, 1.07, 1.06, 1.05, 1.04}
	bars := buildBars(closes)
	d := CalcDelta(bars)
	if d == nil {
		t.Fatal("expected non-nil DeltaResult")
	}
	if d.BarsUsed != len(closes) {
		t.Errorf("BarsUsed = %d, want %d", d.BarsUsed, len(closes))
	}
}

func TestCalcDelta_SeriesLength(t *testing.T) {
	closes := []float64{1.1, 1.09, 1.08, 1.07, 1.06, 1.05, 1.04}
	bars := buildBars(closes)
	d := CalcDelta(bars)
	if d == nil {
		t.Fatal("expected non-nil DeltaResult")
	}
	// Series should have len(bars)-1 entries.
	if len(d.Series) != len(closes)-1 {
		t.Errorf("Series length = %d, want %d", len(d.Series), len(closes)-1)
	}
}
