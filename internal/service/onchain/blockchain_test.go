package onchain

import (
	"testing"
)

func TestAnalyzeHashRate_Capitulation(t *testing.T) {
	h := &BTCNetworkHealth{}

	// Simulate 14 days: first 7 days at 100e12 TH/s, last 7 at 85e12 (15% drop).
	points := make([]BlockchainChartPoint, 14)
	for i := 0; i < 7; i++ {
		points[i] = BlockchainChartPoint{Value: 100e12}
	}
	for i := 7; i < 14; i++ {
		points[i] = BlockchainChartPoint{Value: 85e12}
	}

	analyzeHashRate(h, points)

	if !h.MinerCapitulation {
		t.Error("expected MinerCapitulation=true for 15% hash rate drop")
	}
	if h.HashRateChange >= -10 {
		t.Errorf("expected HashRateChange < -10, got %.1f", h.HashRateChange)
	}
}

func TestAnalyzeHashRate_Stable(t *testing.T) {
	h := &BTCNetworkHealth{}

	// Stable: 14 days all at 100e12.
	points := make([]BlockchainChartPoint, 14)
	for i := range points {
		points[i] = BlockchainChartPoint{Value: 100e12}
	}

	analyzeHashRate(h, points)

	if h.MinerCapitulation {
		t.Error("expected MinerCapitulation=false for stable hash rate")
	}
}

func TestAnalyzeHashRate_Growing(t *testing.T) {
	h := &BTCNetworkHealth{}

	// Growing: 100e12 → 120e12 over 14 days.
	points := make([]BlockchainChartPoint, 14)
	for i := 0; i < 7; i++ {
		points[i] = BlockchainChartPoint{Value: 100e12}
	}
	for i := 7; i < 14; i++ {
		points[i] = BlockchainChartPoint{Value: 120e12}
	}

	analyzeHashRate(h, points)

	if h.MinerCapitulation {
		t.Error("expected MinerCapitulation=false for growing hash rate")
	}
	if h.HashRateChange <= 0 {
		t.Errorf("expected positive HashRateChange, got %.1f", h.HashRateChange)
	}
}

func TestAnalyzeHashRate_ShortData(t *testing.T) {
	h := &BTCNetworkHealth{}

	// Only 1 point — should not panic.
	points := []BlockchainChartPoint{{Value: 100e12}}
	analyzeHashRate(h, points)

	if h.MinerCapitulation {
		t.Error("expected MinerCapitulation=false with insufficient data")
	}
}

func TestAnalyzeFees_Spike(t *testing.T) {
	h := &BTCNetworkHealth{NTx24H: 300000}

	// 29 days at 10 BTC/day, last day at 25 BTC.
	points := make([]BlockchainChartPoint, 30)
	for i := 0; i < 29; i++ {
		points[i] = BlockchainChartPoint{Value: 10}
	}
	points[29] = BlockchainChartPoint{Value: 25}

	analyzeFees(h, points)

	if !h.FeeSpike {
		t.Error("expected FeeSpike=true when latest > 2x avg")
	}
	if h.TotalFeesBTC != 25 {
		t.Errorf("expected TotalFeesBTC=25, got %.1f", h.TotalFeesBTC)
	}
	if h.AvgFeeBTC <= 0 {
		t.Error("expected AvgFeeBTC > 0")
	}
}

func TestAnalyzeFees_Normal(t *testing.T) {
	h := &BTCNetworkHealth{NTx24H: 300000}

	// All days at 10 BTC/day — no spike.
	points := make([]BlockchainChartPoint, 30)
	for i := range points {
		points[i] = BlockchainChartPoint{Value: 10}
	}

	analyzeFees(h, points)

	if h.FeeSpike {
		t.Error("expected FeeSpike=false for normal fee levels")
	}
}

func TestAnalyzeFees_ShortData(t *testing.T) {
	h := &BTCNetworkHealth{}
	points := []BlockchainChartPoint{{Value: 10}}
	analyzeFees(h, points)
	// Should not panic with 1 data point.
	if h.FeeSpike {
		t.Error("expected FeeSpike=false with insufficient data")
	}
}
