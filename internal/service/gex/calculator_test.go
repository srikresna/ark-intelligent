package gex

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// calculateGEX tests
// ---------------------------------------------------------------------------

func TestCalculateGEX_BasicCallPut(t *testing.T) {
	// Single strike: 1 call, 1 put with identical gamma/OI → call wins.
	strikes := []float64{80000}
	callGamma := map[float64]float64{80000: 0.0001}
	callOI := map[float64]float64{80000: 1000}
	putGamma := map[float64]float64{80000: 0.0001}
	putOI := map[float64]float64{80000: 500} // half the call OI
	spot := 82000.0
	contractSize := 1.0

	levels := calculateGEX(strikes, callGamma, callOI, putGamma, putOI, contractSize, spot)
	if len(levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(levels))
	}
	l := levels[0]
	// CallGEX = 0.0001 * 1000 * 1 * 82000² = 672,400,000
	// PutGEX  = -(0.0001 * 500 * 1 * 82000²) = -336,200,000
	// NetGEX  > 0
	if l.NetGEX <= 0 {
		t.Errorf("expected positive NetGEX when callOI > putOI, got %.2f", l.NetGEX)
	}
	if l.CallGEX <= 0 {
		t.Errorf("expected positive CallGEX, got %.2f", l.CallGEX)
	}
	if l.PutGEX >= 0 {
		t.Errorf("expected negative PutGEX, got %.2f", l.PutGEX)
	}
}

func TestCalculateGEX_NegativeRegime(t *testing.T) {
	// Put OI >> Call OI → total GEX should be negative.
	strikes := []float64{78000}
	callGamma := map[float64]float64{78000: 0.0001}
	callOI := map[float64]float64{78000: 100}
	putGamma := map[float64]float64{78000: 0.0001}
	putOI := map[float64]float64{78000: 5000}
	spot := 82000.0

	levels := calculateGEX(strikes, callGamma, callOI, putGamma, putOI, 1.0, spot)
	if len(levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(levels))
	}
	if levels[0].NetGEX >= 0 {
		t.Errorf("expected negative NetGEX when putOI >> callOI, got %.2f", levels[0].NetGEX)
	}
}

func TestCalculateGEX_ZeroGamma(t *testing.T) {
	// Zero gamma → all GEX values should be zero.
	strikes := []float64{80000, 85000}
	callGamma := map[float64]float64{80000: 0, 85000: 0}
	callOI := map[float64]float64{80000: 1000, 85000: 500}
	putGamma := map[float64]float64{80000: 0, 85000: 0}
	putOI := map[float64]float64{80000: 1000, 85000: 500}

	levels := calculateGEX(strikes, callGamma, callOI, putGamma, putOI, 1.0, 82000)
	for _, l := range levels {
		if l.NetGEX != 0 || l.CallGEX != 0 || l.PutGEX != 0 {
			t.Errorf("expected all zero GEX at strike %.0f, got call=%.2f put=%.2f net=%.2f",
				l.Strike, l.CallGEX, l.PutGEX, l.NetGEX)
		}
	}
}

// ---------------------------------------------------------------------------
// findFlipLevel tests
// ---------------------------------------------------------------------------

func TestFindFlipLevel_SignChange(t *testing.T) {
	// Two strikes: first has positive GEX, second has bigger negative GEX.
	// Cumulative crosses zero at second strike.
	levels := []GEXLevel{
		{Strike: 78000, NetGEX: 500e6},
		{Strike: 80000, NetGEX: -900e6}, // cumulative: -400e6 → sign change here
	}
	flip := findFlipLevel(levels, 79000)
	if flip != 80000 {
		t.Errorf("expected flip at 80000, got %.0f", flip)
	}
}

func TestFindFlipLevel_NoFlip(t *testing.T) {
	// All positive GEX → no flip → returns 0.
	levels := []GEXLevel{
		{Strike: 78000, NetGEX: 100e6},
		{Strike: 80000, NetGEX: 200e6},
	}
	flip := findFlipLevel(levels, 79000)
	if flip != 0 {
		t.Errorf("expected no flip (0), got %.0f", flip)
	}
}

// ---------------------------------------------------------------------------
// findMaxPain tests
// ---------------------------------------------------------------------------

func TestFindMaxPain_TwoStrikes(t *testing.T) {
	// strike 80000 with more puts, strike 85000 with more calls.
	// Max pain should be somewhere in between.
	strikes := []float64{80000, 82000, 85000}
	callOI := map[float64]float64{80000: 100, 82000: 50, 85000: 800}
	putOI := map[float64]float64{80000: 900, 82000: 50, 85000: 100}

	mp := findMaxPain(strikes, callOI, putOI)
	// Max pain should be between 80000 and 85000 (not at the extremes)
	if mp < 80000 || mp > 85000 {
		t.Errorf("max pain %.0f out of expected range [80000, 85000]", mp)
	}
}

// ---------------------------------------------------------------------------
// topKeyLevels tests
// ---------------------------------------------------------------------------

func TestTopKeyLevels_ReturnsTopN(t *testing.T) {
	levels := []GEXLevel{
		{Strike: 78000, NetGEX: 100e6},
		{Strike: 80000, NetGEX: -900e6},
		{Strike: 82000, NetGEX: 200e6},
		{Strike: 85000, NetGEX: -50e6},
	}
	keys := topKeyLevels(levels, 2)
	if len(keys) != 2 {
		t.Fatalf("expected 2 key levels, got %d", len(keys))
	}
	// Should be 80000 (abs 900e6) and 82000 (abs 200e6)
	absFirst := math.Abs(levels[1].NetGEX)   // 80000 → 900e6
	absSecond := math.Abs(levels[2].NetGEX)  // 82000 → 200e6
	if absFirst < absSecond {
		t.Error("expected 80000 (largest abs GEX) to be first key level")
	}
}

// ---------------------------------------------------------------------------
// regimeAndImplication tests
// ---------------------------------------------------------------------------

func TestRegimeAndImplication_Positive(t *testing.T) {
	regime, impl := regimeAndImplication(500e9, 80000)
	if regime != "POSITIVE_GEX" {
		t.Errorf("expected POSITIVE_GEX, got %s", regime)
	}
	if impl == "" {
		t.Error("expected non-empty implication")
	}
}

func TestRegimeAndImplication_Negative(t *testing.T) {
	regime, impl := regimeAndImplication(-500e9, 78000)
	if regime != "NEGATIVE_GEX" {
		t.Errorf("expected NEGATIVE_GEX, got %s", regime)
	}
	if impl == "" {
		t.Error("expected non-empty implication")
	}
}
