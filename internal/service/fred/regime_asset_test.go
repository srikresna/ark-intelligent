package fred

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestComputeRegimeAssetMatrix_EmptyInputs(t *testing.T) {
	m := ComputeRegimeAssetMatrix(nil, nil)
	if m == nil {
		t.Fatal("Expected non-nil matrix")
	}
	if len(m.Data) != 0 {
		t.Errorf("Expected empty data, got %d entries", len(m.Data))
	}
}

func TestComputeRegimeAssetMatrix_Normal(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	regimes := []RegimeSnapshot{
		{Date: base, Regime: "GOLDILOCKS"},
		{Date: base.AddDate(0, 0, 28), Regime: "STRESS"},
	}

	prices := map[string][]domain.PriceRecord{
		"EURUSD": {
			{Date: base, Close: 1.10},
			{Date: base.AddDate(0, 0, 7), Close: 1.12},
			{Date: base.AddDate(0, 0, 14), Close: 1.11},
			{Date: base.AddDate(0, 0, 21), Close: 1.09},
			{Date: base.AddDate(0, 0, 28), Close: 1.08},
			{Date: base.AddDate(0, 0, 35), Close: 1.07},
		},
	}

	// Need to check if the price mapping exists for EURUSD
	mapping := domain.FindPriceMapping("EURUSD")
	if mapping == nil {
		t.Skip("No price mapping for EURUSD — test environment may lack fixture data")
	}

	m := ComputeRegimeAssetMatrix(regimes, prices)
	if m == nil {
		t.Fatal("Expected non-nil matrix")
	}
}

func TestRegimeAt(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	snapshots := []RegimeSnapshot{
		{Date: base, Regime: "A"},
		{Date: base.AddDate(0, 0, 7), Regime: "B"},
		{Date: base.AddDate(0, 0, 14), Regime: "C"},
	}

	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{"before all", base.AddDate(0, 0, -1), ""},
		{"exact first", base, "A"},
		{"mid first", base.AddDate(0, 0, 3), "A"},
		{"exact second", base.AddDate(0, 0, 7), "B"},
		{"after all", base.AddDate(0, 0, 30), "C"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := regimeAt(snapshots, tt.date)
			if got != tt.want {
				t.Errorf("regimeAt(%v) = %q, want %q", tt.date, got, tt.want)
			}
		})
	}
}

func TestAnnualizeWeekly(t *testing.T) {
	tests := []struct {
		weekly  float64
		wantMin float64
		wantMax float64
	}{
		{0, -0.01, 0.01},
		{1.0, 50, 80}, // ~67.7%
		{-1.0, -60, -40},
	}
	for _, tt := range tests {
		got := annualizeWeekly(tt.weekly)
		if got < tt.wantMin || got > tt.wantMax {
			t.Errorf("annualizeWeekly(%v) = %v, want [%v, %v]", tt.weekly, got, tt.wantMin, tt.wantMax)
		}
	}
}

func TestGetCurrentRegimeInsight_NilMatrix(t *testing.T) {
	insight := GetCurrentRegimeInsight("GOLDILOCKS", nil)
	if insight.Regime != "GOLDILOCKS" {
		t.Errorf("Regime = %q, want GOLDILOCKS", insight.Regime)
	}
	if len(insight.BestAssets) != 0 {
		t.Errorf("Expected empty BestAssets")
	}
}

func TestGetCurrentRegimeInsight_WithData(t *testing.T) {
	matrix := &RegimeAssetMatrix{
		Data: map[string]map[string]PerformanceStats{
			"GOLDILOCKS": {
				"EUR": {AvgAnnualizedReturn: 15.0, Occurrences: 20},
				"GBP": {AvgAnnualizedReturn: 10.0, Occurrences: 18},
				"AUD": {AvgAnnualizedReturn: 8.0, Occurrences: 15},
				"JPY": {AvgAnnualizedReturn: -5.0, Occurrences: 20},
				"CHF": {AvgAnnualizedReturn: -3.0, Occurrences: 16},
				"CAD": {AvgAnnualizedReturn: 5.0, Occurrences: 12},
			},
		},
	}
	insight := GetCurrentRegimeInsight("GOLDILOCKS", matrix)
	if len(insight.BestAssets) != 3 {
		t.Errorf("BestAssets count = %d, want 3", len(insight.BestAssets))
	}
	if len(insight.BestAssets) > 0 && insight.BestAssets[0].Currency != "EUR" {
		t.Errorf("Best asset = %q, want EUR", insight.BestAssets[0].Currency)
	}
	if len(insight.WorstAssets) == 0 {
		t.Error("Expected non-empty WorstAssets")
	}
	// Worst should be JPY (most negative)
	if len(insight.WorstAssets) > 0 && insight.WorstAssets[0].Currency != "JPY" {
		t.Errorf("Worst asset = %q, want JPY", insight.WorstAssets[0].Currency)
	}
}

func TestGetCurrentRegimeInsight_InsufficientOccurrences(t *testing.T) {
	matrix := &RegimeAssetMatrix{
		Data: map[string]map[string]PerformanceStats{
			"STRESS": {
				"EUR": {AvgAnnualizedReturn: 15.0, Occurrences: 2}, // < 4 min
			},
		},
	}
	insight := GetCurrentRegimeInsight("STRESS", matrix)
	if len(insight.BestAssets) != 0 {
		t.Error("Expected no assets with insufficient occurrences")
	}
}

func TestGetCurrentRegimeInsight_UnknownRegime(t *testing.T) {
	matrix := &RegimeAssetMatrix{
		Data: map[string]map[string]PerformanceStats{
			"GOLDILOCKS": {"EUR": {Occurrences: 10}},
		},
	}
	insight := GetCurrentRegimeInsight("UNKNOWN", matrix)
	if len(insight.BestAssets) != 0 {
		t.Error("Expected empty for unknown regime")
	}
}

func TestBuildRegimeHistoryFromCurrent_NilData(t *testing.T) {
	if got := BuildRegimeHistoryFromCurrent(nil, 10); got != nil {
		t.Errorf("Expected nil for nil data, got %d snapshots", len(got))
	}
}

func TestBuildRegimeHistoryFromCurrent_ZeroWeeks(t *testing.T) {
	data := &MacroData{YieldSpread: 1.0}
	if got := BuildRegimeHistoryFromCurrent(data, 0); got != nil {
		t.Errorf("Expected nil for 0 weeks, got %d snapshots", len(got))
	}
}

func TestBuildRegimeHistoryFromCurrent_Normal(t *testing.T) {
	data := &MacroData{
		YieldSpread: 1.0,
		CorePCE:     2.5,
		VIX:         15,
	}
	got := BuildRegimeHistoryFromCurrent(data, 26)
	if got == nil {
		t.Fatal("Expected non-nil snapshots")
	}
	if len(got) != 26 {
		t.Errorf("Got %d snapshots, want 26", len(got))
	}
	// All should have the same regime since we're using current data
	for i := 1; i < len(got); i++ {
		if got[i].Regime != got[0].Regime {
			t.Errorf("Snapshot %d regime = %q, want same as first (%q)", i, got[i].Regime, got[0].Regime)
			break
		}
	}
	// Should be sorted oldest-first
	if got[0].Date.After(got[len(got)-1].Date) {
		t.Error("Expected oldest-first ordering")
	}
}
