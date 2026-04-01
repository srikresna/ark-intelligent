package strategy

import (
	"math"
	"testing"
)

func TestComputeKelly(t *testing.T) {
	tests := []struct {
		name       string
		winRate    float64
		avgWinLoss float64
		wantMin    float64
		wantMax    float64
	}{
		{"55% win, 1:1 R:R", 55.0, 1.0, 0.09, 0.11},
		{"60% win, 1.5:1 R:R (capped)", 60.0, 1.5, 0.24, 0.26}, // Kelly=0.333 but capped at 0.25 // Capped at 0.25
		{"50% win, 1:1 R:R → zero edge", 50.0, 1.0, 0, 0.001},
		{"40% win, 1:1 R:R → negative edge", 40.0, 1.0, 0, 0.001},
		{"zero win rate", 0, 1.0, 0, 0.001},
		{"zero avg win/loss", 55.0, 0, 0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeKelly(tt.winRate, tt.avgWinLoss)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeKelly(%v, %v) = %v, want [%v, %v]",
					tt.winRate, tt.avgWinLoss, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestVolRegimeMultiplier(t *testing.T) {
	if m := volRegimeMultiplier("EXPANDING"); m != 0.80 {
		t.Errorf("EXPANDING = %v, want 0.80", m)
	}
	if m := volRegimeMultiplier("CONTRACTING"); m != 1.10 {
		t.Errorf("CONTRACTING = %v, want 1.10", m)
	}
	if m := volRegimeMultiplier("NORMAL"); m != 1.0 {
		t.Errorf("NORMAL = %v, want 1.0", m)
	}
	if m := volRegimeMultiplier(""); m != 1.0 {
		t.Errorf("empty = %v, want 1.0", m)
	}
}

func TestComputeRiskParity_Basic(t *testing.T) {
	input := RiskParityInput{
		Positions: []PositionRisk{
			{Symbol: "EURUSD", Direction: DirectionLong, Entry: 1.1000, StopLoss: 1.0950, Size: 10000},
			{Symbol: "GBPUSD", Direction: DirectionShort, Entry: 1.2700, StopLoss: 1.2780, Size: 10000},
		},
		AccountBalance: 10000,
		MaxHeatPct:     6.0,
		WinRate:        58.0,
		AvgWinLoss:     1.2,
	}

	result := ComputeRiskParity(input)

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.TotalHeatPct <= 0 {
		t.Error("TotalHeatPct should be > 0")
	}

	if result.KellyFraction <= 0 {
		t.Error("KellyFraction should be > 0 with positive edge")
	}

	if result.HalfKelly != result.KellyFraction/2 {
		t.Errorf("HalfKelly = %v, want %v", result.HalfKelly, result.KellyFraction/2)
	}

	if len(result.HeatBreakdown) != 2 {
		t.Errorf("HeatBreakdown len = %d, want 2", len(result.HeatBreakdown))
	}

	if len(result.AdjustedPositions) != 2 {
		t.Errorf("AdjustedPositions len = %d, want 2", len(result.AdjustedPositions))
	}

	for _, adj := range result.AdjustedPositions {
		if adj.RecommendedSize <= 0 {
			t.Errorf("RecommendedSize for %s should be > 0", adj.Symbol)
		}
		if adj.ScaleFactor <= 0 {
			t.Errorf("ScaleFactor for %s should be > 0", adj.Symbol)
		}
	}
}

func TestComputeRiskParity_OverheatScaleDown(t *testing.T) {
	input := RiskParityInput{
		Positions: []PositionRisk{
			{Symbol: "EURUSD", Direction: DirectionLong, Entry: 1.1000, StopLoss: 1.0800, Size: 50000},
			{Symbol: "GBPUSD", Direction: DirectionLong, Entry: 1.2700, StopLoss: 1.2500, Size: 50000},
			{Symbol: "USDJPY", Direction: DirectionShort, Entry: 150.00, StopLoss: 152.00, Size: 500},
		},
		AccountBalance: 10000,
		MaxHeatPct:     6.0,
	}

	result := ComputeRiskParity(input)

	if result.Recommendation != SizingScaleDown {
		t.Errorf("Recommendation = %v, want SCALE_DOWN (positions are overleveraged)", result.Recommendation)
	}

	// All adjusted positions should be scaled down
	for _, adj := range result.AdjustedPositions {
		if adj.ScaleFactor >= 1.0 {
			t.Errorf("ScaleFactor for %s = %v, want < 1.0 for overheat", adj.Symbol, adj.ScaleFactor)
		}
	}
}

func TestComputeRiskParity_ZeroBalance(t *testing.T) {
	result := ComputeRiskParity(RiskParityInput{AccountBalance: 0})
	if result.Recommendation != SizingBalanced {
		t.Errorf("zero balance recommendation = %v, want BALANCED", result.Recommendation)
	}
}

func TestComputeRiskParity_NoPositions(t *testing.T) {
	result := ComputeRiskParity(RiskParityInput{
		AccountBalance: 10000,
		MaxHeatPct:     6.0,
	})
	if result.TotalHeatPct != 0 {
		t.Errorf("no positions: TotalHeatPct = %v, want 0", result.TotalHeatPct)
	}
	if result.Recommendation != SizingScaleUp {
		t.Errorf("no positions: Recommendation = %v, want SCALE_UP", result.Recommendation)
	}
}

func TestComputeRiskParity_VolRegimeAdjustment(t *testing.T) {
	input := RiskParityInput{
		Positions: []PositionRisk{
			{Symbol: "EURUSD", Direction: DirectionLong, Entry: 1.1000, StopLoss: 1.0950, Size: 10000},
		},
		AccountBalance: 100000,
		MaxHeatPct:     6.0,
		WinRate:        55.0,
		AvgWinLoss:     1.0,
		VolRegimes:     map[string]string{"EURUSD": "EXPANDING"},
	}

	result := ComputeRiskParity(input)

	if len(result.AdjustedPositions) == 0 {
		t.Fatal("no adjusted positions")
	}

	adj := result.AdjustedPositions[0]
	if adj.VolAdjustment != 0.80 {
		t.Errorf("EXPANDING vol adjustment = %v, want 0.80", adj.VolAdjustment)
	}
}

func TestRoundN_EdgeCases(t *testing.T) {
	if r := roundN(math.NaN(), 2); r != 0 {
		t.Errorf("NaN → %v, want 0", r)
	}
	if r := roundN(math.Inf(1), 2); r != 0 {
		t.Errorf("+Inf → %v, want 0", r)
	}
	if r := roundN(1.23456, 2); r != 1.23 {
		t.Errorf("1.23456 round 2 = %v, want 1.23", r)
	}
}
