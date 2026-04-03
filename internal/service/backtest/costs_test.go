package backtest

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// --- costs.go tests ---

// TestSpreadBps_KnownCurrencies verifies known currencies return documented values.
func TestSpreadBps_KnownCurrencies(t *testing.T) {
	tests := []struct {
		currency string
		want     float64
	}{
		{"EUR", 1.0},
		{"GBP", 1.5},
		{"JPY", 1.0},
		{"AUD", 2.0},
		{"NZD", 3.0},
		{"CAD", 2.0},
		{"CHF", 2.0},
		{"MXN", 5.0},
		{"XAU", 3.0},
		{"BTC", 10.0},
		{"ETH", 15.0},
		{"DXY", 2.0},
	}
	for _, tt := range tests {
		got := SpreadBps(tt.currency)
		if got != tt.want {
			t.Errorf("SpreadBps(%q) = %v, want %v", tt.currency, got, tt.want)
		}
	}
}

// TestSpreadBps_Unknown returns default 3.0 for unknown currencies.
func TestSpreadBps_Unknown(t *testing.T) {
	got := SpreadBps("XYZ")
	if got != 3.0 {
		t.Errorf("SpreadBps(unknown) = %v, want 3.0", got)
	}
	got = SpreadBps("")
	if got != 3.0 {
		t.Errorf("SpreadBps(\"\") = %v, want 3.0", got)
	}
}

// TestCostAdjustedReturn verifies round-trip cost subtraction.
func TestCostAdjustedReturn(t *testing.T) {
	// spread 1 bps → cost = 1/100 * 2 = 0.02%
	got := CostAdjustedReturn(0.5, 1.0)
	want := 0.5 - 0.02
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("CostAdjustedReturn(0.5, 1.0) = %v, want %v", got, want)
	}

	// negative raw return still subtracts cost
	got = CostAdjustedReturn(-0.1, 2.0)
	want = -0.1 - 0.04
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("CostAdjustedReturn(-0.1, 2.0) = %v, want %v", got, want)
	}

	// zero raw return → negative (cost drag)
	got = CostAdjustedReturn(0, 3.0)
	want = -(3.0 / 100 * 2)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("CostAdjustedReturn(0, 3.0) = %v, want %v", got, want)
	}
}

// TestComputeCostAnalysis_Empty returns zero result for empty input.
func TestComputeCostAnalysis_Empty(t *testing.T) {
	result := ComputeCostAnalysis(nil, "test")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Evaluated != 0 {
		t.Errorf("Evaluated = %d, want 0", result.Evaluated)
	}
	if result.GroupLabel != "test" {
		t.Errorf("GroupLabel = %q, want test", result.GroupLabel)
	}
}

// TestComputeCostAnalysis_SkipsPending skips PENDING and EXPIRED signals.
func TestComputeCostAnalysis_SkipsPending(t *testing.T) {
	signals := []domain.PersistedSignal{
		{Currency: "EUR", Outcome1W: domain.OutcomePending, Return1W: 1.0},
		{Currency: "EUR", Outcome1W: domain.OutcomeExpired, Return1W: 1.0},
		{Currency: "EUR", Outcome1W: "", Return1W: 1.0},
	}
	result := ComputeCostAnalysis(signals, "label")
	if result.Evaluated != 0 {
		t.Errorf("expected 0 evaluated signals, got %d", result.Evaluated)
	}
}

// TestComputeCostAnalysis_WinsAndLosses verifies basic aggregation.
func TestComputeCostAnalysis_WinsAndLosses(t *testing.T) {
	signals := []domain.PersistedSignal{
		{Currency: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 0.5},  // 0.5% win, cost = 0.02%
		{Currency: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 0.8},  // 0.8% win
		{Currency: "EUR", Outcome1W: domain.OutcomeLoss, Return1W: -0.3}, // 0.3% loss
	}
	result := ComputeCostAnalysis(signals, "test")
	if result.Evaluated != 3 {
		t.Fatalf("expected 3 evaluated, got %d", result.Evaluated)
	}
	// AvgCostPct should be EUR spread = 1 bps → 0.02% per trade
	expectedCost := 1.0 / 100 * 2
	if math.Abs(result.AvgCostPct-expectedCost) > 1e-9 {
		t.Errorf("AvgCostPct = %v, want %v", result.AvgCostPct, expectedCost)
	}
	// NetAvgReturn should be less than RawAvgReturn
	if result.NetAvgReturn1W >= result.RawAvgReturn1W {
		t.Errorf("expected NetAvgReturn1W < RawAvgReturn1W, got net=%v raw=%v",
			result.NetAvgReturn1W, result.RawAvgReturn1W)
	}
}

// TestComputeCostAnalysis_CostErasesEdge detects when cost turns positive EV negative.
func TestComputeCostAnalysis_CostErasesEdge(t *testing.T) {
	// Very small positive returns — cost should erase edge
	signals := []domain.PersistedSignal{
		{Currency: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 0.001},
		{Currency: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 0.001},
		{Currency: "EUR", Outcome1W: domain.OutcomeLoss, Return1W: -0.001},
	}
	result := ComputeCostAnalysis(signals, "tiny-wins")
	// RawEV might be positive but NetEV could be negative due to cost
	// Just verify CostErasesEdge logic: only set when RawEV>0 AND NetEV<=0
	if result.CostErasesEdge && !(result.RawEV > 0 && result.NetEV <= 0) {
		t.Error("CostErasesEdge set incorrectly: RawEV and NetEV don't match the condition")
	}
}

// --- daily_trend_filter.go tests ---

// TestClampConfidence verifies the [5, 98] range.
func TestClampConfidence(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{50.0, 50.0},   // in range — unchanged
		{0.0, 5.0},     // below min → clamped to 5
		{-10.0, 5.0},   // well below → 5
		{100.0, 98.0},  // above max → 98
		{150.0, 98.0},  // well above → 98
		{5.0, 5.0},     // exactly at min
		{98.0, 98.0},   // exactly at max
		{97.9, 97.9},   // just below max — unchanged
	}
	for _, tt := range tests {
		got := clampConfidence(tt.in)
		if got != tt.want {
			t.Errorf("clampConfidence(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestComputeTrendAdjustment_FullBullishAlignment expects large positive adjustment.
func TestComputeTrendAdjustment_FullBullishAlignment(t *testing.T) {
	dc := &domain.DailyPriceContext{
		DailyTrend:  "UP",
		DMA20:       1.1,
		DMA50:       1.05,
		DMA200:      1.00,
		CurrentPrice: 1.15,
		AboveDMA20:  true,
		Momentum5D:  1.0, // > 0.3 → +3
		ConsecDays:  4,
		ConsecDir:   "UP",
	}
	adj := computeTrendAdjustment(dc, true)
	// +5 (daily trend) +7 (MA aligned: bullish) +3 (AboveDMA20) +3 (momentum) +2 (streak) = 20
	if adj < 15 {
		t.Errorf("full bullish alignment: adj = %v, want >= 15", adj)
	}
}

// TestComputeTrendAdjustment_FullBearishOpposition gives penalty to bullish signal.
func TestComputeTrendAdjustment_FullBearishOpposition(t *testing.T) {
	dc := &domain.DailyPriceContext{
		DailyTrend:   "DOWN",
		DMA20:        0.95,
		DMA50:        1.00,
		DMA200:       1.05,
		CurrentPrice: 0.90,
		AboveDMA20:   false,
		Momentum5D:   -1.0,
		ConsecDays:   4,
		ConsecDir:    "DOWN",
	}
	// Bullish signal against bearish context → penalty
	adj := computeTrendAdjustment(dc, true)
	if adj >= 0 {
		t.Errorf("bullish signal vs bearish context: adj = %v, want negative", adj)
	}
}

// TestComputeTrendAdjustment_NeutralTrend produces minimal adjustment.
func TestComputeTrendAdjustment_NeutralTrend(t *testing.T) {
	dc := &domain.DailyPriceContext{
		DailyTrend: "FLAT",
		DMA20:      0, // not computed
		Momentum5D: 0,
		ConsecDays: 0,
	}
	adj := computeTrendAdjustment(dc, true)
	// FLAT trend → 0, no DMA20, no momentum, no streak → adj = 0
	if adj != 0 {
		t.Errorf("neutral context: adj = %v, want 0", adj)
	}
}

// TestExplainAdjustment covers all message branches.
func TestExplainAdjustment(t *testing.T) {
	dc := &domain.DailyPriceContext{}
	tests := []struct {
		adj     float64
		bullish bool
		substr  string
	}{
		{15, true, "strongly confirmed"},
		{8, true, "supported"},
		{3, true, "weakly confirmed"},
		{-12, false, "strongly opposed"},
		{-7, false, "opposed"},
		{-2, true, "weakly opposed"},
		{0, true, "neutral"},
	}
	for _, tt := range tests {
		msg := explainAdjustment(dc, tt.bullish, tt.adj)
		if msg == "" {
			t.Errorf("explainAdjustment(%v, %v, %v) returned empty string", tt.adj, tt.bullish, tt.adj)
		}
		// Spot-check expected keywords
		found := false
		for _, kw := range []string{tt.substr} {
			if len(msg) > 0 {
				// simple contains check via strings search
				for i := 0; i <= len(msg)-len(kw); i++ {
					if msg[i:i+len(kw)] == kw {
						found = true
						break
					}
				}
			}
		}
		if !found {
			t.Errorf("explainAdjustment(adj=%v) = %q, want substring %q", tt.adj, msg, tt.substr)
		}
	}
}

// TestNewDailyTrendFilter constructs without panic.
func TestNewDailyTrendFilter(t *testing.T) {
	f := NewDailyTrendFilter(nil)
	if f == nil {
		t.Fatal("NewDailyTrendFilter returned nil")
	}
}
