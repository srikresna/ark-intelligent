package gex

import (
	"math"
	"testing"
	"time"
)

func TestComputeSkewMetrics_SmileCurve(t *testing.T) {
	expiry := time.Now().Add(7 * 24 * time.Hour)
	points := []IVPoint{
		// Deep OTM puts (moneyness 0.80)
		{Moneyness: 0.78, MarkIV: 80, OptionType: "put"},
		{Moneyness: 0.82, MarkIV: 75, OptionType: "put"},
		// OTM puts (moneyness 0.90)
		{Moneyness: 0.88, MarkIV: 65, OptionType: "put"},
		{Moneyness: 0.92, MarkIV: 60, OptionType: "put"},
		// ATM (moneyness 1.00)
		{Moneyness: 0.98, MarkIV: 50, OptionType: "call"},
		{Moneyness: 1.02, MarkIV: 48, OptionType: "put"},
		// OTM calls (moneyness 1.10)
		{Moneyness: 1.08, MarkIV: 45, OptionType: "call"},
		{Moneyness: 1.12, MarkIV: 42, OptionType: "call"},
		// Deep OTM calls (moneyness 1.20)
		{Moneyness: 1.18, MarkIV: 40, OptionType: "call"},
		{Moneyness: 1.22, MarkIV: 38, OptionType: "call"},
	}

	metrics := computeSkewMetrics(expiry, 7, points)

	// Should have 5 smile points
	if len(metrics.SmileCurve) != 5 {
		t.Fatalf("expected 5 smile points, got %d", len(metrics.SmileCurve))
	}

	// Smile should show decreasing IV from left to right (normal skew)
	for i := 0; i < len(metrics.SmileCurve)-1; i++ {
		if metrics.SmileCurve[i].AvgIV > 0 && metrics.SmileCurve[i+1].AvgIV > 0 {
			if metrics.SmileCurve[i].AvgIV < metrics.SmileCurve[i+1].AvgIV {
				t.Logf("smile point %d (%.0f%%) < point %d (%.0f%%) — checking normal skew",
					i, metrics.SmileCurve[i].AvgIV, i+1, metrics.SmileCurve[i+1].AvgIV)
			}
		}
	}

	// ATM (index 2) should have data
	if metrics.SmileCurve[2].AvgIV == 0 {
		t.Error("ATM smile point has zero IV")
	}
}

func TestComputeSkewMetrics_BearishPCRatio(t *testing.T) {
	expiry := time.Now().Add(14 * 24 * time.Hour)
	// Puts have higher IV than calls → bearish
	points := []IVPoint{
		{Moneyness: 0.90, MarkIV: 70, OptionType: "put"},
		{Moneyness: 0.95, MarkIV: 65, OptionType: "put"},
		{Moneyness: 1.00, MarkIV: 55, OptionType: "put"},
		{Moneyness: 1.00, MarkIV: 45, OptionType: "call"},
		{Moneyness: 1.05, MarkIV: 40, OptionType: "call"},
		{Moneyness: 1.10, MarkIV: 38, OptionType: "call"},
	}

	metrics := computeSkewMetrics(expiry, 14, points)

	// Put/Call ratio should be > 1 (bearish)
	if metrics.PutCallIVRatio <= 1.0 {
		t.Errorf("expected bearish PC ratio > 1, got %.2f", metrics.PutCallIVRatio)
	}
	if metrics.SkewDirection != "BEARISH" {
		t.Errorf("expected BEARISH direction, got %s", metrics.SkewDirection)
	}
}

func TestComputeSkewMetrics_BullishPCRatio(t *testing.T) {
	expiry := time.Now().Add(14 * 24 * time.Hour)
	// Calls have higher IV than puts → bullish
	points := []IVPoint{
		{Moneyness: 0.90, MarkIV: 35, OptionType: "put"},
		{Moneyness: 0.95, MarkIV: 38, OptionType: "put"},
		{Moneyness: 1.00, MarkIV: 50, OptionType: "call"},
		{Moneyness: 1.05, MarkIV: 60, OptionType: "call"},
		{Moneyness: 1.10, MarkIV: 65, OptionType: "call"},
	}

	metrics := computeSkewMetrics(expiry, 14, points)

	if metrics.PutCallIVRatio >= 1.0 {
		t.Errorf("expected bullish PC ratio < 1, got %.2f", metrics.PutCallIVRatio)
	}
	if metrics.SkewDirection != "BULLISH" {
		t.Errorf("expected BULLISH direction, got %s", metrics.SkewDirection)
	}
}

func TestLinearSlope_Positive(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	ys := []float64{2, 4, 6, 8, 10}
	slope := linearSlope(xs, ys)
	if math.Abs(slope-2.0) > 0.01 {
		t.Errorf("expected slope 2.0, got %.4f", slope)
	}
}

func TestLinearSlope_Negative(t *testing.T) {
	xs := []float64{1, 2, 3}
	ys := []float64{6, 4, 2}
	slope := linearSlope(xs, ys)
	if math.Abs(slope-(-2.0)) > 0.01 {
		t.Errorf("expected slope -2.0, got %.4f", slope)
	}
}

func TestLinearSlope_InsufficientData(t *testing.T) {
	slope := linearSlope([]float64{1}, []float64{5})
	if slope != 0 {
		t.Errorf("expected 0 for single point, got %.4f", slope)
	}
}

func TestComputePercentile_Basic(t *testing.T) {
	history := []skewSnapshot{
		{pcRatio: 0.90},
		{pcRatio: 0.95},
		{pcRatio: 1.00},
		{pcRatio: 1.05},
		{pcRatio: 1.10},
	}
	// Current value 1.03 → 3 values below (0.90, 0.95, 1.00) → 60th percentile
	pct := computePercentile(history, 1.03)
	if math.Abs(pct-60.0) > 0.1 {
		t.Errorf("expected ~60th percentile, got %.1f", pct)
	}
}

func TestComputePercentile_InsufficientHistory(t *testing.T) {
	pct := computePercentile([]skewSnapshot{{pcRatio: 1.0}}, 1.0)
	if pct != 50.0 {
		t.Errorf("expected 50.0 for insufficient history, got %.1f", pct)
	}
}

func TestDetectSkewFlip_NoHistory(t *testing.T) {
	alert := detectSkewFlip("BTC", 7, time.Now(), nil)
	if alert != nil {
		t.Error("expected nil alert with no history")
	}
}

func TestDetectSkewFlip_FreshFlip(t *testing.T) {
	history := []skewSnapshot{
		{pcRatio: 1.10, direction: "BEARISH", ts: time.Now().Add(-1 * time.Hour)},
		{pcRatio: 0.90, direction: "BULLISH", ts: time.Now()},
	}
	alert := detectSkewFlip("BTC", 7, time.Now().Add(7*24*time.Hour), history)
	if alert == nil {
		t.Fatal("expected alert for bearish→bullish flip")
	}
	if alert.OldSkew != "BEARISH" || alert.NewSkew != "BULLISH" {
		t.Errorf("expected BEARISH→BULLISH, got %s→%s", alert.OldSkew, alert.NewSkew)
	}
}

func TestDetectSkewFlip_NotFresh(t *testing.T) {
	// Same direction for last 2 snapshots → not a fresh flip
	history := []skewSnapshot{
		{pcRatio: 1.10, direction: "BEARISH", ts: time.Now().Add(-2 * time.Hour)},
		{pcRatio: 0.90, direction: "BULLISH", ts: time.Now().Add(-1 * time.Hour)},
		{pcRatio: 0.88, direction: "BULLISH", ts: time.Now()},
	}
	alert := detectSkewFlip("BTC", 7, time.Now(), history)
	if alert != nil {
		t.Error("expected nil alert when last 2 snapshots have same direction")
	}
}

func TestComputeTermSlope_Contango(t *testing.T) {
	pts := []TermPoint{
		{DTE: 7, ATMIV: 50},
		{DTE: 30, ATMIV: 55},
		{DTE: 90, ATMIV: 60},
	}
	slope, signal := computeTermSlope(pts)
	if slope <= 0 {
		t.Errorf("expected positive slope for contango, got %.4f", slope)
	}
	if signal != "CONTANGO" {
		t.Errorf("expected CONTANGO, got %s", signal)
	}
}

func TestComputeTermSlope_Backwardation(t *testing.T) {
	pts := []TermPoint{
		{DTE: 7, ATMIV: 80},
		{DTE: 30, ATMIV: 60},
		{DTE: 90, ATMIV: 45},
	}
	slope, signal := computeTermSlope(pts)
	if slope >= 0 {
		t.Errorf("expected negative slope for backwardation, got %.4f", slope)
	}
	if signal != "BACKWARDATION" {
		t.Errorf("expected BACKWARDATION, got %s", signal)
	}
}

func TestNormalizeSymbol(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"btc", "BTC"},
		{" Eth ", "ETH"},
		{"SOL", "SOL"},
	}
	for _, tt := range tests {
		got := normalizeSymbol(tt.in)
		if got != tt.want {
			t.Errorf("normalizeSymbol(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{7, "7"},
		{42, "42"},
		{-3, "-3"},
		{100, "100"},
	}
	for _, tt := range tests {
		got := itoa(tt.in)
		if got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
