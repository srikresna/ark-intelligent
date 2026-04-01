package cot

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// helper to build a minimal COTAnalysis for testing.
func makeAnalysis(code, currency string, netPos, oi float64) domain.COTAnalysis {
	return domain.COTAnalysis{
		Contract:     domain.COTContract{Code: code, Currency: currency},
		NetPosition:  netPos,
		OpenInterest: oi,
	}
}

func TestComputeUSDAggregate_Empty(t *testing.T) {
	agg := ComputeUSDAggregate(nil, nil)
	if agg.Direction != "NEUTRAL" {
		t.Fatalf("expected NEUTRAL for empty input, got %s", agg.Direction)
	}
	if len(agg.Contributions) != 0 {
		t.Fatalf("expected empty contributions, got %d", len(agg.Contributions))
	}
}

func TestComputeUSDAggregate_AllBearishUSD(t *testing.T) {
	// All FX pairs long (= short USD), OI-normalised.
	contracts := domain.DefaultCOTContracts
	analyses := []domain.COTAnalysis{
		makeAnalysis("099741", "EUR", 100000, 200000), // long EUR = short USD
		makeAnalysis("096742", "GBP", 80000, 200000),
		makeAnalysis("097741", "JPY", 60000, 200000),
		makeAnalysis("092741", "CHF", 50000, 200000),
		makeAnalysis("232741", "AUD", 40000, 200000),
		makeAnalysis("090741", "CAD", 30000, 200000),
		makeAnalysis("112741", "NZD", 20000, 200000),
	}

	agg := ComputeUSDAggregate(analyses, contracts)

	if agg.Direction != "BEARISH" {
		t.Errorf("expected BEARISH USD, got %s (score %.2f)", agg.Direction, agg.Score)
	}
	if agg.Score >= 0 {
		t.Errorf("expected negative score, got %.2f", agg.Score)
	}
	if agg.ConvictionPct < 70 {
		t.Errorf("expected high conviction (all pairs agree), got %.0f%%", agg.ConvictionPct)
	}
	if !agg.HighConviction {
		t.Error("expected HighConviction = true")
	}
}

func TestComputeUSDAggregate_BullishUSD(t *testing.T) {
	// All FX pairs short (= long USD).
	contracts := domain.DefaultCOTContracts
	analyses := []domain.COTAnalysis{
		makeAnalysis("099741", "EUR", -80000, 200000),
		makeAnalysis("096742", "GBP", -70000, 200000),
		makeAnalysis("097741", "JPY", -60000, 200000),
	}

	agg := ComputeUSDAggregate(analyses, contracts)

	if agg.Direction != "BULLISH" {
		t.Errorf("expected BULLISH USD, got %s (score %.2f)", agg.Direction, agg.Score)
	}
	if agg.Score <= 0 {
		t.Errorf("expected positive score, got %.2f", agg.Score)
	}
}

func TestComputeUSDAggregate_DXDivergence(t *testing.T) {
	// Cross-pairs say bearish USD, DX says bullish → divergence.
	contracts := domain.DefaultCOTContracts
	analyses := []domain.COTAnalysis{
		makeAnalysis("099741", "EUR", 100000, 200000),  // long EUR = short USD
		makeAnalysis("096742", "GBP", 80000, 200000),   // long GBP = short USD
		makeAnalysis("097741", "JPY", 60000, 200000),   // long JPY = short USD
		makeAnalysis("098662", "USD", 50000, 200000),    // long DX = long USD
	}

	agg := ComputeUSDAggregate(analyses, contracts)

	if !agg.Divergence {
		t.Error("expected divergence between DX and aggregate")
	}
	if agg.DivergenceDesc == "" {
		t.Error("expected non-empty divergence description")
	}
	if agg.DXDirectDir != "BULLISH" {
		t.Errorf("expected DX BULLISH, got %s", agg.DXDirectDir)
	}
	if agg.Direction != "BEARISH" {
		t.Errorf("expected aggregate BEARISH, got %s", agg.Direction)
	}
}

func TestComputeUSDAggregate_MixedSignals(t *testing.T) {
	contracts := domain.DefaultCOTContracts
	analyses := []domain.COTAnalysis{
		makeAnalysis("099741", "EUR", 80000, 200000),   // short USD
		makeAnalysis("096742", "GBP", -70000, 200000),  // long USD
		makeAnalysis("097741", "JPY", 5000, 200000),    // near neutral
		makeAnalysis("232741", "AUD", -40000, 200000),  // long USD
	}

	agg := ComputeUSDAggregate(analyses, contracts)

	if agg.HighConviction {
		t.Error("expected low conviction with mixed signals")
	}
}

func TestComputeUSDAggregate_SkipsNonFX(t *testing.T) {
	contracts := domain.DefaultCOTContracts
	analyses := []domain.COTAnalysis{
		makeAnalysis("099741", "EUR", 100000, 200000),
		makeAnalysis("088691", "XAU", 200000, 400000), // Gold — should be skipped
		makeAnalysis("067651", "OIL", 150000, 300000), // Oil — should be skipped
	}

	agg := ComputeUSDAggregate(analyses, contracts)

	if _, ok := agg.Contributions["XAU"]; ok {
		t.Error("XAU should not appear in contributions")
	}
	if _, ok := agg.Contributions["OIL"]; ok {
		t.Error("OIL should not appear in contributions")
	}
	if _, ok := agg.Contributions["EUR"]; !ok {
		t.Error("EUR should be in contributions")
	}
}

func TestComputeUSDAggregate_FallbackNoOI(t *testing.T) {
	contracts := domain.DefaultCOTContracts
	analyses := []domain.COTAnalysis{
		{
			Contract:     domain.COTContract{Code: "099741", Currency: "EUR"},
			NetPosition:  100000, // positive but no OI
			OpenInterest: 0,
			PctOfOI:      0,
		},
	}

	agg := ComputeUSDAggregate(analyses, contracts)

	// Should fallback to clamped -1 (short USD via long EUR).
	if v, ok := agg.Contributions["EUR"]; !ok || v >= 0 {
		t.Errorf("expected negative EUR contribution (short USD), got %v", v)
	}
}

func TestFormatUSDAggregate_NonEmpty(t *testing.T) {
	agg := USDAggregate{
		Score:         -35.2,
		Direction:     "BEARISH",
		Contributions: map[string]float64{"EUR": -0.40, "GBP": -0.20},
		DXDirectScore: 0.15,
		DXDirectDir:   "BULLISH",
		Divergence:    true,
		DivergenceDesc: "DX says BULLISH, cross-pairs say BEARISH",
		ConvictionPct: 85,
		HighConviction: true,
	}

	html := FormatUSDAggregate(agg)
	if html == "" {
		t.Fatal("expected non-empty output")
	}
	if !containsAll(html, "USD Aggregate", "BEARISH", "EUR", "DIVERGENCE") {
		t.Errorf("missing expected content in output:\n%s", html)
	}
}

func TestFormatUSDAggregate_Empty(t *testing.T) {
	agg := USDAggregate{Contributions: map[string]float64{}}
	html := FormatUSDAggregate(agg)
	if html != "" {
		t.Errorf("expected empty output for no contributions, got: %s", html)
	}
}

// containsAll checks that s contains all substrings.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
