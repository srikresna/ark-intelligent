package onchain

import (
	"testing"
)

func TestAnalyzeAsset_BasicFlow(t *testing.T) {
	// 5 days of data, oldest first (ascending as from API).
	points := []coinMetricsDataPoint{
		{Asset: "btc", Time: "2026-03-28T00:00:00.000000000Z", FlowInExNtv: "140", FlowOutExNtv: "240", AdrActCnt: "850000", TxCnt: "260000"},
		{Asset: "btc", Time: "2026-03-29T00:00:00.000000000Z", FlowInExNtv: "130", FlowOutExNtv: "230", AdrActCnt: "860000", TxCnt: "270000"},
		{Asset: "btc", Time: "2026-03-30T00:00:00.000000000Z", FlowInExNtv: "120", FlowOutExNtv: "220", AdrActCnt: "870000", TxCnt: "280000"},
		{Asset: "btc", Time: "2026-03-31T00:00:00.000000000Z", FlowInExNtv: "150", FlowOutExNtv: "250", AdrActCnt: "880000", TxCnt: "290000"},
		{Asset: "btc", Time: "2026-04-01T00:00:00.000000000Z", FlowInExNtv: "100", FlowOutExNtv: "200", AdrActCnt: "900000", TxCnt: "300000"},
	}

	s := analyzeAsset("btc", points)

	if !s.Available {
		t.Fatal("expected Available=true")
	}
	if s.Asset != "btc" {
		t.Errorf("expected asset=btc, got %s", s.Asset)
	}
	if len(s.Flows) != 5 {
		t.Fatalf("expected 5 flows, got %d", len(s.Flows))
	}

	// All days have net outflow (in < out), so consecutive outflow should be 5.
	if s.ConsecutiveOutflow != 5 {
		t.Errorf("expected 5 consecutive outflow days, got %d", s.ConsecutiveOutflow)
	}

	// Net flow 7D should be negative (accumulation).
	if s.NetFlow7D >= 0 {
		t.Errorf("expected negative 7D net flow, got %f", s.NetFlow7D)
	}

	if s.FlowTrend != "ACCUMULATION" {
		t.Errorf("expected ACCUMULATION trend, got %s", s.FlowTrend)
	}
}

func TestAnalyzeAsset_LargeInflowSpike(t *testing.T) {
	// Most recent day has huge inflow spike (ascending order).
	points := []coinMetricsDataPoint{
		{Asset: "eth", Time: "2026-03-30T00:00:00.000000000Z", FlowInExNtv: "180", FlowOutExNtv: "280", AdrActCnt: "480000", TxCnt: "98000"},
		{Asset: "eth", Time: "2026-03-31T00:00:00.000000000Z", FlowInExNtv: "200", FlowOutExNtv: "300", AdrActCnt: "490000", TxCnt: "99000"},
		{Asset: "eth", Time: "2026-04-01T00:00:00.000000000Z", FlowInExNtv: "5000", FlowOutExNtv: "100", AdrActCnt: "500000", TxCnt: "100000"},
	}

	s := analyzeAsset("eth", points)

	if !s.LargeInflowSpike {
		t.Error("expected LargeInflowSpike=true")
	}
	if s.FlowTrend != "DISTRIBUTION" {
		t.Errorf("expected DISTRIBUTION trend, got %s", s.FlowTrend)
	}
}

func TestAnalyzeAsset_EmptyData(t *testing.T) {
	s := analyzeAsset("btc", nil)
	if s.FlowTrend != "" {
		t.Errorf("expected empty flow trend for nil data, got %s", s.FlowTrend)
	}
}

func TestParseFloat_EdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"", 0},
		{"123.45", 123.45},
		{"abc", 0},
		{"NaN", 0},
		{"Inf", 0},
	}

	for _, tt := range tests {
		got := parseFloat(tt.input)
		if got != tt.want {
			t.Errorf("parseFloat(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestCountConsecutiveOutflow(t *testing.T) {
	flows := []ExchangeFlow{
		{NetFlow: 10},  // inflow
		{NetFlow: -5},  // outflow
		{NetFlow: -10}, // outflow
		{NetFlow: -3},  // outflow
	}
	got := countConsecutiveOutflow(flows)
	if got != 3 {
		t.Errorf("expected 3 consecutive outflow, got %d", got)
	}
}

func TestSumNetFlows(t *testing.T) {
	flows := []ExchangeFlow{
		{NetFlow: -10},
		{NetFlow: -20},
		{NetFlow: 5},
		{NetFlow: -15},
		{NetFlow: -30},
	}

	got7 := sumNetFlows(flows, 7) // all 5 days (less than 7 available)
	want := -10.0 + -20.0 + 5.0 + -15.0 + -30.0
	if got7 != want {
		t.Errorf("sumNetFlows(7) = %f, want %f", got7, want)
	}

	got3 := sumNetFlows(flows, 3) // last 3 days
	want3 := 5.0 + -15.0 + -30.0
	if got3 != want3 {
		t.Errorf("sumNetFlows(3) = %f, want %f", got3, want3)
	}
}
