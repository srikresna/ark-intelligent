package elliott

import (
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// syntheticBars creates a slice of OHLCV bars (newest-first) following a
// price path defined by `levels`.  Each price step becomes one bar.
func syntheticBars(levels []float64) []ta.OHLCV {
	n := len(levels)
	bars := make([]ta.OHLCV, n)
	for i, p := range levels {
		bars[n-1-i] = ta.OHLCV{
			Date:  time.Now().Add(-time.Duration(n-1-i) * time.Hour),
			Open:  p,
			High:  p * 1.001,
			Low:   p * 0.999,
			Close: p,
		}
	}
	return bars
}

// syntheticWave5Bars builds a synthetic 5-wave impulse (bullish) from scratch
// using explicit price levels for each pivot.
func syntheticWave5Bars(p0, p1, p2, p3, p4, p5 float64) []ta.OHLCV {
	// Build point-by-point: p0→p1 (W1), p1→p2 (W2), ... p4→p5 (W5)
	pts := []float64{p0, p1, p2, p3, p4, p5}
	// Expand each leg into 10 bars for the ZigZag to detect pivots.
	var levels []float64
	for i := 0; i < len(pts)-1; i++ {
		from := pts[i]
		to := pts[i+1]
		for j := 0; j <= 9; j++ {
			levels = append(levels, from+(to-from)*float64(j)/9.0)
		}
	}
	return syntheticBars(levels)
}

// ---------------------------------------------------------------------------
// Rule 1 — Wave 2 must not retrace > 100% of Wave 1
// ---------------------------------------------------------------------------

func TestRule1_Wave2CannotRetraceOver100Pct(t *testing.T) {
	waves := []Wave{
		{Number: "1", Direction: "UP", Start: 100, End: 200}, // W1 = +100 pts
		{Number: "2", Direction: "DOWN", Start: 200, End: 50}, // W2 = -150 pts (>100%)
		{Number: "3", Direction: "UP", Start: 50, End: 300},
		{Number: "4", Direction: "DOWN", Start: 300, End: 220},
		{Number: "5", Direction: "UP", Start: 220, End: 350},
	}
	validateImpulse(waves)

	if waves[1].Valid {
		t.Error("Rule 1: expected Wave 2 to be invalid (retraced >100% of Wave 1)")
	}
	if waves[1].Retracement <= 1.0 {
		t.Errorf("Rule 1: expected retracement > 1.0, got %.3f", waves[1].Retracement)
	}
}

func TestRule1_Wave2ValidRetrace(t *testing.T) {
	waves := []Wave{
		{Number: "1", Direction: "UP", Start: 100, End: 200}, // W1 = +100
		{Number: "2", Direction: "DOWN", Start: 200, End: 138.2}, // ~61.8% retrace
		{Number: "3", Direction: "UP", Start: 138.2, End: 320},
		{Number: "4", Direction: "DOWN", Start: 320, End: 280},
		{Number: "5", Direction: "UP", Start: 280, End: 360},
	}
	validateImpulse(waves)

	if !waves[1].Valid {
		t.Errorf("Rule 1: Wave 2 should be valid; violation: %s", waves[1].Violation)
	}
}

// ---------------------------------------------------------------------------
// Rule 2 — Wave 3 must NOT be the shortest impulse wave
// ---------------------------------------------------------------------------

func TestRule2_Wave3CannotBeShortest(t *testing.T) {
	waves := []Wave{
		{Number: "1", Direction: "UP", Start: 100, End: 200},  // W1 = 100 pts
		{Number: "2", Direction: "DOWN", Start: 200, End: 150},
		{Number: "3", Direction: "UP", Start: 150, End: 200},  // W3 = 50 pts (shortest!)
		{Number: "4", Direction: "DOWN", Start: 200, End: 170},
		{Number: "5", Direction: "UP", Start: 170, End: 340},  // W5 = 170 pts
	}
	validateImpulse(waves)

	if waves[2].Valid {
		t.Error("Rule 2: expected Wave 3 to be invalid (it is the shortest)")
	}
}

func TestRule2_Wave3NotShortest(t *testing.T) {
	waves := []Wave{
		{Number: "1", Direction: "UP", Start: 100, End: 200}, // 100 pts
		{Number: "2", Direction: "DOWN", Start: 200, End: 138},
		{Number: "3", Direction: "UP", Start: 138, End: 300}, // 162 pts (longest)
		{Number: "4", Direction: "DOWN", Start: 300, End: 260},
		{Number: "5", Direction: "UP", Start: 260, End: 340}, // 80 pts
	}
	validateImpulse(waves)

	if !waves[2].Valid {
		t.Errorf("Rule 2: Wave 3 should be valid; violation: %s", waves[2].Violation)
	}
}

// ---------------------------------------------------------------------------
// Rule 3 — Wave 4 must not overlap Wave 1 territory
// ---------------------------------------------------------------------------

func TestRule3_Wave4OverlapsWave1(t *testing.T) {
	// Bullish count: W1 ends at 200. W4 must not drop below 200.
	waves := []Wave{
		{Number: "1", Direction: "UP", Start: 100, End: 200},
		{Number: "2", Direction: "DOWN", Start: 200, End: 150},
		{Number: "3", Direction: "UP", Start: 150, End: 350},
		{Number: "4", Direction: "DOWN", Start: 350, End: 180}, // drops below W1 end (200)
		{Number: "5", Direction: "UP", Start: 180, End: 400},
	}
	validateImpulse(waves)

	if waves[3].Valid {
		t.Error("Rule 3: expected Wave 4 to be invalid (overlaps Wave 1 territory)")
	}
}

func TestRule3_Wave4DoesNotOverlap(t *testing.T) {
	waves := []Wave{
		{Number: "1", Direction: "UP", Start: 100, End: 200},
		{Number: "2", Direction: "DOWN", Start: 200, End: 150},
		{Number: "3", Direction: "UP", Start: 150, End: 350},
		{Number: "4", Direction: "DOWN", Start: 350, End: 210}, // stays above W1 end (200) ✓
		{Number: "5", Direction: "UP", Start: 210, End: 400},
	}
	validateImpulse(waves)

	if !waves[3].Valid {
		t.Errorf("Rule 3: Wave 4 should be valid; violation: %s", waves[3].Violation)
	}
}

// ---------------------------------------------------------------------------
// ZigZag detection
// ---------------------------------------------------------------------------

func TestZigZag_DetectsAlternatePivots(t *testing.T) {
	// Explicit 5-wave bullish impulse.
	// p0=100 (low, W1 start), p1=200 (W1 end / W2 start), p2=140 (W2 end),
	// p3=310 (W3 end), p4=245 (W4 end), p5=370 (W5 end).
	bars := syntheticWave5Bars(100, 200, 140, 310, 245, 370)

	pivots := detectZigZag(bars, 0.05)
	if len(pivots) < 6 {
		t.Fatalf("expected ≥6 pivots for 5-wave impulse, got %d", len(pivots))
	}

	// Verify alternating high/low.
	for i := 1; i < len(pivots); i++ {
		if pivots[i].IsHigh == pivots[i-1].IsHigh {
			t.Errorf("ZigZag: consecutive pivots at index %d and %d are both IsHigh=%v",
				i-1, i, pivots[i].IsHigh)
		}
	}
}

// ---------------------------------------------------------------------------
// Engine integration — LOW confidence for tiny bar count
// ---------------------------------------------------------------------------

func TestEngine_LowConfidenceForFewBars(t *testing.T) {
	// Analyze returns nil for bars < 20. Generate a small but >= 20 bar set.
	// With < 50 bars, confidence must be LOW.
	var levels []float64
	for i := 0; i < 30; i++ {
		levels = append(levels, 100+float64(i)*0.5)
	}
	bars := syntheticBars(levels)
	eng := NewEngine()
	result := eng.Analyze(bars, "TEST", "H1")
	if result == nil {
		t.Fatal("expected non-nil result for 30-bar input")
	}
	if result.Confidence != "LOW" {
		t.Errorf("expected LOW confidence for <50 bars, got %s", result.Confidence)
	}
}

// ---------------------------------------------------------------------------
// Projector — target calculations
// ---------------------------------------------------------------------------

func TestProjectTargets_Bullish(t *testing.T) {
	w1 := Wave{Number: "1", Direction: "UP", Start: 100, End: 200} // 100 pts
	waves := []Wave{w1}

	t1, t2 := projectTargets(waves)

	// Conservative: 100 + 100*1.0 = 200
	if !approxEqual(t1, 200, 0.01) {
		t.Errorf("target1: expected 200, got %.4f", t1)
	}
	// Aggressive: 100 + 100*1.618 = 261.8
	if !approxEqual(t2, 261.8, 0.01) {
		t.Errorf("target2: expected 261.8, got %.4f", t2)
	}
}

func TestProjectTargets_Bearish(t *testing.T) {
	w1 := Wave{Number: "1", Direction: "DOWN", Start: 200, End: 100} // 100 pts down
	waves := []Wave{w1}

	t1, t2 := projectTargets(waves)

	// Conservative: 200 - 100*1.0 = 100
	if !approxEqual(t1, 100, 0.01) {
		t.Errorf("target1: expected 100, got %.4f", t1)
	}
	// Aggressive: 200 - 100*1.618 = 38.2
	if !approxEqual(t2, 38.2, 0.01) {
		t.Errorf("target2: expected 38.2, got %.4f", t2)
	}
}

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}
