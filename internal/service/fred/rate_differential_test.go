package fred

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestCarryAdjustment_BullishPositiveCarry(t *testing.T) {
	diff := domain.RateDifferential{Differential: 2.0}
	got := CarryAdjustment(diff, "BULLISH")
	if got < 0 || got > 5 {
		t.Errorf("CarryAdjustment(+2, BULLISH) = %v, want 0-5", got)
	}
	if got != 4.0 { // 2.0 * 2 = 4.0
		t.Errorf("CarryAdjustment(+2, BULLISH) = %v, want 4.0", got)
	}
}

func TestCarryAdjustment_BullishLargeCarry_Capped(t *testing.T) {
	diff := domain.RateDifferential{Differential: 5.0}
	got := CarryAdjustment(diff, "BULLISH")
	if got != 5.0 {
		t.Errorf("CarryAdjustment(+5, BULLISH) = %v, want 5.0 (capped)", got)
	}
}

func TestCarryAdjustment_BearishNegativeCarry(t *testing.T) {
	diff := domain.RateDifferential{Differential: -2.0}
	got := CarryAdjustment(diff, "BEARISH")
	if got != 4.0 { // abs(-2) * 2 = 4
		t.Errorf("CarryAdjustment(-2, BEARISH) = %v, want 4.0", got)
	}
}

func TestCarryAdjustment_BullishNegativeCarry_Headwind(t *testing.T) {
	diff := domain.RateDifferential{Differential: -2.0}
	got := CarryAdjustment(diff, "BULLISH")
	if got >= 0 {
		t.Errorf("CarryAdjustment(-2, BULLISH) = %v, want negative (headwind)", got)
	}
	// -2.0 * 1.5 = -3.0, max with -5 = -3.0
	if got != -3.0 {
		t.Errorf("CarryAdjustment(-2, BULLISH) = %v, want -3.0", got)
	}
}

func TestCarryAdjustment_BearishPositiveCarry_Headwind(t *testing.T) {
	diff := domain.RateDifferential{Differential: 2.0}
	got := CarryAdjustment(diff, "BEARISH")
	if got >= 0 {
		t.Errorf("CarryAdjustment(+2, BEARISH) = %v, want negative (headwind)", got)
	}
}

func TestCarryAdjustment_Neutral(t *testing.T) {
	diff := domain.RateDifferential{Differential: 0.5}
	got := CarryAdjustment(diff, "BEARISH")
	if got != 0 {
		t.Errorf("CarryAdjustment(+0.5, BEARISH) = %v, want 0 (neutral carry)", got)
	}
}

func TestCarryAdjustment_ZeroDiff(t *testing.T) {
	diff := domain.RateDifferential{Differential: 0}
	got := CarryAdjustment(diff, "BULLISH")
	if got != 0 {
		t.Errorf("CarryAdjustment(0, BULLISH) = %v, want 0", got)
	}
}

func TestCarryAdjustment_NaN(t *testing.T) {
	diff := domain.RateDifferential{Differential: math.NaN()}
	got := CarryAdjustment(diff, "BULLISH")
	// NaN comparisons return false, so should fall through to neutral
	if got != 0 {
		t.Errorf("CarryAdjustment(NaN, BULLISH) = %v, want 0", got)
	}
}

func TestClampFloat(t *testing.T) {
	tests := []struct {
		v, lo, hi, want float64
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 0, 0},
	}
	for _, tt := range tests {
		got := clampFloat(tt.v, tt.lo, tt.hi)
		if got != tt.want {
			t.Errorf("clampFloat(%v, %v, %v) = %v, want %v", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}

func TestRoundN(t *testing.T) {
	tests := []struct {
		v    float64
		n    int
		want float64
	}{
		{3.14159, 2, 3.14},
		{3.14159, 0, 3},
		{3.14159, 4, 3.1416},
		{0, 2, 0},
	}
	for _, tt := range tests {
		got := roundN(tt.v, tt.n)
		if math.Abs(got-tt.want) > 0.00001 {
			t.Errorf("roundN(%v, %d) = %v, want %v", tt.v, tt.n, got, tt.want)
		}
	}
}
