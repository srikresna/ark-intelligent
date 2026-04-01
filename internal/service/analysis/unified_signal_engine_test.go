package analysis_test

import (
	"context"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/analysis"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// TestComputeUnifiedSignal_AllAvailable verifies the happy-path where all
// sub-systems provide data.
func TestComputeUnifiedSignal_AllAvailable(t *testing.T) {
	cotA := &domain.COTAnalysis{
		SentimentScore: 60, // bullish
	}
	cta := &ta.ConfluenceResult{
		Score: 55, // bullish
	}
	hmm := &price.HMMResult{
		CurrentState: "RISK_ON",
		Converged:    true,
	}
	garch := &price.GARCHResult{
		VolRatio: 0.9,
	}
	risk := &domain.RiskContext{
		VIXLevel: 14, // low VIX
	}
	seasonal := &price.SeasonalPattern{
		CurrentBias: "BULLISH",
	}

	in := analysis.UnifiedSignalInput{
		COTAnalysis:   cotA,
		CTAConfluence: cta,
		HMM:           hmm,
		GARCH:         garch,
		Risk:          risk,
		Seasonal:      seasonal,
	}

	sig := analysis.ComputeUnifiedSignal("EUR", in)

	if sig == nil {
		t.Fatal("expected non-nil result")
	}
	if sig.Currency != "EUR" {
		t.Errorf("expected EUR, got %s", sig.Currency)
	}
	if sig.UnifiedScore <= 0 {
		t.Errorf("expected positive unified score, got %.2f", sig.UnifiedScore)
	}
	if sig.Recommendation != analysis.RecommendationStrongLong &&
		sig.Recommendation != analysis.RecommendationLong {
		t.Errorf("expected LONG/STRONG_LONG, got %s", sig.Recommendation)
	}
	if sig.VIXMultiplier != 1.0 {
		t.Errorf("expected VIX multiplier 1.0 for low VIX, got %.3f", sig.VIXMultiplier)
	}
}

// TestComputeUnifiedSignal_Bearish verifies bearish signal when sub-systems align short.
func TestComputeUnifiedSignal_Bearish(t *testing.T) {
	cotA := &domain.COTAnalysis{
		SentimentScore: -70, // bearish
	}
	cta := &ta.ConfluenceResult{
		Score: -65,
	}
	hmm := &price.HMMResult{
		CurrentState: "CRISIS",
		Converged:    true,
	}
	risk := &domain.RiskContext{
		VIXLevel: 35, // panic
	}

	in := analysis.UnifiedSignalInput{
		COTAnalysis:   cotA,
		CTAConfluence: cta,
		HMM:           hmm,
		Risk:          risk,
	}

	sig := analysis.ComputeUnifiedSignal("JPY", in)

	if sig.UnifiedScore >= 0 {
		t.Errorf("expected negative score for bearish inputs, got %.2f", sig.UnifiedScore)
	}
	if sig.VIXMultiplier >= 1.0 {
		t.Errorf("expected VIX dampening when VIX=35, got %.3f", sig.VIXMultiplier)
	}
}

// TestComputeUnifiedSignal_Conflict verifies that conflict detection reduces confidence.
func TestComputeUnifiedSignal_Conflict(t *testing.T) {
	// COT says long, CTA says short → conflict
	cotA := &domain.COTAnalysis{
		SentimentScore: 75, // strongly bullish
	}
	cta := &ta.ConfluenceResult{
		Score: -75, // strongly bearish
	}

	in := analysis.UnifiedSignalInput{
		COTAnalysis:   cotA,
		CTAConfluence: cta,
	}

	sig := analysis.ComputeUnifiedSignal("EUR", in)

	if sig.ConflictCount == 0 {
		t.Error("expected at least 1 conflict when COT and CTA disagree")
	}
	if sig.Confidence >= 100 {
		t.Errorf("expected confidence < 100 when conflicts present, got %.1f", sig.Confidence)
	}
}

// TestComputeUnifiedSignal_MissingCOT verifies graceful degradation with nil COT.
func TestComputeUnifiedSignal_NilInputs(t *testing.T) {
	in := analysis.UnifiedSignalInput{}

	sig := analysis.ComputeUnifiedSignal("GBP", in)

	if sig == nil {
		t.Fatal("expected non-nil result even with all nil inputs")
	}
	if sig.UnifiedScore != 0 {
		t.Errorf("expected 0 score with no inputs, got %.2f", sig.UnifiedScore)
	}
	if sig.Recommendation != analysis.RecommendationNeutral {
		t.Errorf("expected NEUTRAL with no inputs, got %s", sig.Recommendation)
	}
}

// TestComputeUnifiedSignalForCurrency_ContextCancel verifies nil-safety on context cancel.
func TestComputeUnifiedSignalForCurrency_NilSafe(t *testing.T) {
	sig := analysis.ComputeUnifiedSignalForCurrency(
		context.Background(),
		"AUD",
		nil, // no COT
		fred.MacroRegime{},
		nil, // no macro
		0,
		nil, // no CTA
		nil, // no HMM
		nil, // no GARCH
		nil, // no risk
		nil, // no seasonal
	)
	if sig == nil {
		t.Fatal("expected non-nil result even with nil inputs")
	}
}

// TestScoreToRecommendation tests grade/recommendation classification boundaries.
func TestScoreToGrade(t *testing.T) {
	cases := []struct {
		score    float64
		wantGrade string
	}{
		{90, "A+"},
		{70, "A"},
		{55, "B"},
		{40, "C"},
		{20, "D"},
		{5, "F"},
		{-70, "A"},
		{-85, "A+"},
	}

	for _, tc := range cases {
		in := analysis.UnifiedSignalInput{
			COTAnalysis: &domain.COTAnalysis{SentimentScore: tc.score},
		}
		sig := analysis.ComputeUnifiedSignal("EUR", in)
		if sig.Grade != tc.wantGrade {
			t.Errorf("score %.0f: expected grade %s, got %s", tc.score, tc.wantGrade, sig.Grade)
		}
	}
}
