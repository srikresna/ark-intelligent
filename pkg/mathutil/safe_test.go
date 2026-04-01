package mathutil

import (
	"math"
	"testing"
)

func TestIsFinite(t *testing.T) {
	tests := []struct {
		name string
		val  float64
		want bool
	}{
		{"zero", 0, true},
		{"positive", 42.5, true},
		{"negative", -3.14, true},
		{"NaN", math.NaN(), false},
		{"positive Inf", math.Inf(1), false},
		{"negative Inf", math.Inf(-1), false},
		{"max float64", math.MaxFloat64, true},
		{"smallest positive", math.SmallestNonzeroFloat64, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFinite(tt.val); got != tt.want {
				t.Errorf("IsFinite(%v) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestSafeDiv(t *testing.T) {
	tests := []struct {
		name     string
		a, b, fb float64
		want     float64
	}{
		{"normal division", 10, 2, 0, 5},
		{"divide by zero", 10, 0, -1, -1},
		{"zero / zero", 0, 0, 99, 99},
		{"large / tiny", math.MaxFloat64, math.SmallestNonzeroFloat64, 0, 0}, // Inf result → fallback
		{"negative division", -6, 3, 0, -2},
		{"both negative", -6, -3, 0, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeDiv(tt.a, tt.b, tt.fb)
			if got != tt.want {
				t.Errorf("SafeDiv(%v, %v, %v) = %v, want %v", tt.a, tt.b, tt.fb, got, tt.want)
			}
		})
	}
}

func TestClampFloat(t *testing.T) {
	tests := []struct {
		name       string
		f, min, max float64
		want       float64
	}{
		{"within range", 5, 0, 10, 5},
		{"below min", -1, 0, 10, 0},
		{"above max", 15, 0, 10, 10},
		{"NaN fallback", math.NaN(), 0, 100, 50},
		{"Inf fallback", math.Inf(1), 0, 100, 50},
		{"neg Inf fallback", math.Inf(-1), 0, 100, 50},
		{"exact min", 0, 0, 10, 0},
		{"exact max", 10, 0, 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampFloat(tt.f, tt.min, tt.max)
			if got != tt.want {
				t.Errorf("ClampFloat(%v, %v, %v) = %v, want %v", tt.f, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestSanitizeFloat(t *testing.T) {
	tests := []struct {
		name     string
		f, fb    float64
		want     float64
	}{
		{"normal", 42, 0, 42},
		{"NaN", math.NaN(), -1, -1},
		{"Inf", math.Inf(1), 0, 0},
		{"neg Inf", math.Inf(-1), 0, 0},
		{"zero", 0, 99, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFloat(tt.f, tt.fb)
			if got != tt.want {
				t.Errorf("SanitizeFloat(%v, %v) = %v, want %v", tt.f, tt.fb, got, tt.want)
			}
		})
	}
}
