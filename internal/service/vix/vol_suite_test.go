package vix

import (
	"math"
	"testing"
)

func TestSanitizeFloat(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"normal", 3.14, 3.14},
		{"zero", 0, 0},
		{"negative", -1.5, -1.5},
		{"NaN", math.NaN(), 0},
		{"Inf", math.Inf(1), 0},
		{"NegInf", math.Inf(-1), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFloat(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeFloat(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestClassifyTailRisk(t *testing.T) {
	tests := []struct {
		name     string
		skew     float64
		vixSpot  float64
		ratio    float64
		expected string
	}{
		{"extreme_high_skew_low_vix", 145, 12, 145.0 / 12.0, "EXTREME"},
		{"elevated_moderate", 135, 16, 135.0 / 16.0, "ELEVATED"},
		{"extreme_ratio", 120, 14, 8.5, "EXTREME"},
		{"elevated_skew_only", 142, 22, 142.0 / 22.0, "ELEVATED"},
		{"normal", 115, 20, 115.0 / 20.0, "NORMAL"},
		{"normal_low", 100, 15, 100.0 / 15.0, "NORMAL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vs := &VolSuite{
				SKEW:         tt.skew,
				SKEWVIXRatio: tt.ratio,
			}
			vs.classifyTailRisk(tt.vixSpot)
			if vs.TailRisk != tt.expected {
				t.Errorf("classifyTailRisk(SKEW=%.0f, VIX=%.0f) = %q, want %q",
					tt.skew, tt.vixSpot, vs.TailRisk, tt.expected)
			}
		})
	}
}

func TestDetectDivergences(t *testing.T) {
	tests := []struct {
		name     string
		vs       VolSuite
		vixSpot  float64
		minCount int
		contains string
	}{
		{
			name:     "high_ovx_vix_ratio",
			vs:       VolSuite{OVX: 50, OVXVIXRatio: 3.5},
			vixSpot:  14,
			minCount: 1,
			contains: "OVX/VIX",
		},
		{
			name:     "rvx_vix_elevated",
			vs:       VolSuite{RVX: 35, RVXVIXRatio: 1.4},
			vixSpot:  25,
			minCount: 1,
			contains: "RVX/VIX",
		},
		{
			name:     "vix9d_event",
			vs:       VolSuite{VIX9D: 30, VIX9D30Ratio: 1.2},
			vixSpot:  25,
			minCount: 1,
			contains: "VIX9D/VIX",
		},
		{
			name:     "broad_risk_off",
			vs:       VolSuite{GVZ: 25},
			vixSpot:  28,
			minCount: 1,
			contains: "broad risk-off",
		},
		{
			name:     "normal_no_divergences",
			vs:       VolSuite{OVX: 20, GVZ: 12, RVX: 18, RVXVIXRatio: 1.1, OVXVIXRatio: 1.5},
			vixSpot:  15,
			minCount: 0,
			contains: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.vs.detectDivergences(tt.vixSpot)
			if len(tt.vs.Divergences) < tt.minCount {
				t.Errorf("expected at least %d divergences, got %d", tt.minCount, len(tt.vs.Divergences))
			}
			if tt.contains != "" {
				found := false
				for _, d := range tt.vs.Divergences {
					if containsStr(d, tt.contains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected divergence containing %q, got %v", tt.contains, tt.vs.Divergences)
				}
			}
		})
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
