package cot

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
)

// ---------------------------------------------------------------------------
// ConfluenceScoreV2
// ---------------------------------------------------------------------------

func TestConfluenceScoreV2_NilMacroData(t *testing.T) {
	// Without macro data: weights are 60/40 (COT/surprise).
	// stress = 0 when macro is nil, and is not included in the total.
	a := domain.COTAnalysis{
		Contract:       baseContract("TFF"),
		SentimentScore: 80, // strong bullish
	}
	score := ConfluenceScoreV2(a, nil, 2.0) // surprise = 2 sigma → 40

	// total = 80*0.60 + 40*0.40 = 48 + 16 = 64
	if math.Abs(score-64) > 1.0 {
		t.Errorf("expected ~64, got %.2f", score)
	}
}

func TestConfluenceScoreV2_WithMacroData(t *testing.T) {
	// With macro data: weights are 35/20/45 (COT/surprise/macro).
	macro := &fred.MacroData{
		YieldSpread:   0.5,      // > 0 → +10
		CorePCE:       2.0,      // < 2.5 → +25
		NFCI:          -0.5,     // nfciScore = Clamp(20, -25, 25) = 20
		InitialClaims: 200_000,  // < 220k → +15
		// macroScore = 10+25+20+15 = 70
	}
	a := domain.COTAnalysis{
		Contract:       baseContract("TFF"),
		SentimentScore: 60,
	}
	score := ConfluenceScoreV2(a, macro, 1.0) // surprise = 1 sigma → 20

	// cotScore = 60, surpriseScore = 20, macroScore = 70
	// total = 60*0.35 + 20*0.20 + 70*0.45 = 21 + 4 + 31.5 = 56.5
	if math.Abs(score-56.5) > 1.0 {
		t.Errorf("expected ~56.50, got %.2f", score)
	}
}

func TestConfluenceScoreV2_ExtremeBullish(t *testing.T) {
	macro := &fred.MacroData{
		YieldSpread:   1.0,
		CorePCE:       2.0,
		NFCI:          -1.0,
		InitialClaims: 180_000,
		GDPGrowth:     4.0, // > 3.0 → gdpFactor = 10
	}
	a := domain.COTAnalysis{
		Contract:       baseContract("TFF"),
		SentimentScore: 100,
	}
	score := ConfluenceScoreV2(a, macro, 5.0) // max surprise

	if score <= 0 {
		t.Errorf("expected positive bullish score, got %.2f", score)
	}
}

func TestConfluenceScoreV2_ExtremeBearish(t *testing.T) {
	macro := &fred.MacroData{
		YieldSpread:   -1.0,   // no +30
		CorePCE:       4.0,    // > 2.5, no +30
		NFCI:          2.0,    // positive → no +20, stressScore = -100
		InitialClaims: 400_000, // > 250k → no +20
		GDPGrowth:     -2.0,   // negative → gdpFactor = -15
	}
	a := domain.COTAnalysis{
		Contract:       baseContract("TFF"),
		SentimentScore: -100,
	}
	score := ConfluenceScoreV2(a, macro, -5.0) // max bearish surprise

	if score >= 0 {
		t.Errorf("expected negative bearish score, got %.2f", score)
	}
}

func TestConfluenceScoreV2_NeutralInputs(t *testing.T) {
	a := domain.COTAnalysis{
		Contract:       baseContract("TFF"),
		SentimentScore: 0,
	}
	score := ConfluenceScoreV2(a, nil, 0)

	// All zeros → total = 0
	if math.Abs(score) > 0.01 {
		t.Errorf("expected ~0 for neutral inputs, got %.2f", score)
	}
}

func TestConfluenceScoreV2_Clamped(t *testing.T) {
	// Even with extreme inputs, score should be clamped to [-100, 100].
	a := domain.COTAnalysis{
		Contract:       baseContract("TFF"),
		SentimentScore: 100,
	}
	score := ConfluenceScoreV2(a, nil, 10.0)
	if score > 100 || score < -100 {
		t.Errorf("score out of [-100,100] range: %.2f", score)
	}
}

// ---------------------------------------------------------------------------
// FREDRegimeMultiplier
// ---------------------------------------------------------------------------

func TestFREDRegimeMultiplier(t *testing.T) {
	tests := []struct {
		currency string
		regime   string
		want     float64
	}{
		{"USD", "INFLATIONARY", 15},
		{"EUR", "INFLATIONARY", -10},
		{"GBP", "INFLATIONARY", -10},
		{"CHF", "INFLATIONARY", -10},
		{"AUD", "STRESS", -20},
		{"NZD", "STRESS", -20},
		{"CAD", "STRESS", -20},
		{"JPY", "RECESSION", 20},
		{"XAU", "GOLDILOCKS", -5},
		{"USD", "DISINFLATIONARY", -5},
		{"EUR", "DISINFLATIONARY", 10},
		{"AUD", "DISINFLATIONARY", 10},
		{"JPY", "DISINFLATIONARY", 0},
		{"USD", "RECESSION", -15},
		{"EUR", "RECESSION", 0},
		{"AUD", "RECESSION", -20},
		{"USD", "STAGFLATION", 0},
		{"EUR", "STAGFLATION", -5},
		{"AUD", "STAGFLATION", -15},
		{"JPY", "STAGFLATION", 15},
		{"USD", "GOLDILOCKS", -5},
		{"EUR", "GOLDILOCKS", 5},
		{"AUD", "GOLDILOCKS", 15},
		{"XAU", "STRESS", 20},
		// Unknown currency
		{"MXN", "INFLATIONARY", 0},
		// Unknown regime
		{"USD", "UNKNOWN_REGIME", 0},
		// Both unknown
		{"MXN", "UNKNOWN_REGIME", 0},
	}

	for _, tt := range tests {
		t.Run(tt.currency+"_"+tt.regime, func(t *testing.T) {
			regime := fred.MacroRegime{Name: tt.regime}
			got := FREDRegimeMultiplier(tt.currency, regime)
			if got != tt.want {
				t.Errorf("FREDRegimeMultiplier(%s, %s) = %.0f, want %.0f", tt.currency, tt.regime, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ComputeRegimeAdjustedScore
// ---------------------------------------------------------------------------

func TestComputeRegimeAdjustedScore(t *testing.T) {
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{Currency: "USD"},
		SentimentScore: 50,
	}
	regime := fred.MacroRegime{Name: "INFLATIONARY"} // USD → +15

	got := ComputeRegimeAdjustedScore(a, regime)
	want := 65.0
	if math.Abs(got-want) > 0.01 {
		t.Errorf("got %.2f, want %.2f", got, want)
	}
}

func TestComputeRegimeAdjustedScore_Clamped(t *testing.T) {
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{Currency: "USD"},
		SentimentScore: 95,
	}
	regime := fred.MacroRegime{Name: "INFLATIONARY"} // +15

	got := ComputeRegimeAdjustedScore(a, regime)
	if got > 100 {
		t.Errorf("expected clamped to 100, got %.2f", got)
	}
}

// ---------------------------------------------------------------------------
// ComputeConvictionScore
// ---------------------------------------------------------------------------

func TestComputeConvictionScore_HighConvictionLong(t *testing.T) {
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{Currency: "EUR", ReportType: "TFF"},
		SentimentScore: 80,
	}
	regime := fred.MacroRegime{Name: "DISINFLATIONARY"} // EUR → +10
	// nil macro → no FRED component, weights 50/30/20
	cs := ComputeConvictionScore(a, regime, 3.0, "ECB hawkish", nil)

	if cs.Direction != "LONG" {
		t.Errorf("direction = %s, want LONG", cs.Direction)
	}
	if cs.Score <= 75 {
		t.Errorf("expected score > 75 for high conviction, got %.2f", cs.Score)
	}
	if cs.COTBias != "BULLISH" {
		t.Errorf("COTBias = %s, want BULLISH", cs.COTBias)
	}
	if cs.Currency != "EUR" {
		t.Errorf("currency = %s, want EUR", cs.Currency)
	}
	if cs.FREDRegime != "DISINFLATIONARY" {
		t.Errorf("regime = %s, want DISINFLATIONARY", cs.FREDRegime)
	}
	if cs.CalendarBias != "ECB hawkish" {
		t.Errorf("calendarBias = %s, want 'ECB hawkish'", cs.CalendarBias)
	}
	// Label should contain HIGH CONVICTION
	if cs.Label != "HIGH CONVICTION LONG" {
		t.Errorf("label = %s, want 'HIGH CONVICTION LONG'", cs.Label)
	}
}

func TestComputeConvictionScore_HighConvictionShort(t *testing.T) {
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{Currency: "AUD", ReportType: "TFF"},
		SentimentScore: -80,
	}
	regime := fred.MacroRegime{Name: "STRESS"} // AUD → -20

	cs := ComputeConvictionScore(a, regime, -3.0, "", nil)

	if cs.Direction != "SHORT" {
		t.Errorf("direction = %s, want SHORT", cs.Direction)
	}
	if cs.Score >= 45 {
		t.Errorf("expected score < 45 for short, got %.2f", cs.Score)
	}
	if cs.COTBias != "BEARISH" {
		t.Errorf("COTBias = %s, want BEARISH", cs.COTBias)
	}
}

func TestComputeConvictionScore_Neutral(t *testing.T) {
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{Currency: "CHF", ReportType: "TFF"},
		SentimentScore: 0,
	}
	regime := fred.MacroRegime{Name: "DISINFLATIONARY"} // CHF → +10

	cs := ComputeConvictionScore(a, regime, 0, "", nil)

	// base = 0*0.5 + 0 + 0 = 0; adjusted = 0 + 10 = 10; conviction = (10+100)/2 = 55
	// 55 is borderline; direction threshold is >55 LONG, <45 SHORT, else NEUTRAL
	if cs.Direction != "NEUTRAL" {
		t.Errorf("direction = %s, want NEUTRAL (score=%.2f)", cs.Direction, cs.Score)
	}
	if cs.COTBias != "NEUTRAL" {
		t.Errorf("COTBias = %s, want NEUTRAL", cs.COTBias)
	}
}

func TestComputeConvictionScore_LabelMatches(t *testing.T) {
	tests := []struct {
		name      string
		sentiment float64
		surprise  float64
		regime    string
		currency  string
		wantDir   string
	}{
		{
			name:      "strong_long",
			sentiment: 90,
			surprise:  4.0,
			regime:    "GOLDILOCKS",
			currency:  "AUD", // +15
			wantDir:   "LONG",
		},
		{
			name:      "strong_short",
			sentiment: -90,
			surprise:  -4.0,
			regime:    "STRESS",
			currency:  "AUD", // -20
			wantDir:   "SHORT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := domain.COTAnalysis{
				Contract:       domain.COTContract{Currency: tt.currency, ReportType: "TFF"},
				SentimentScore: tt.sentiment,
			}
			cs := ComputeConvictionScore(a, fred.MacroRegime{Name: tt.regime}, tt.surprise, "", nil)
			if cs.Direction != tt.wantDir {
				t.Errorf("direction = %s, want %s (score=%.2f)", cs.Direction, tt.wantDir, cs.Score)
			}
			// Label should contain the direction
			if tt.wantDir == "LONG" || tt.wantDir == "SHORT" {
				if len(cs.Label) == 0 {
					t.Error("label is empty")
				}
			}
		})
	}
}

func TestComputeConvictionScore_ScoreRange(t *testing.T) {
	// Verify score is always in [0, 100] range regardless of inputs.
	extremes := []float64{-100, -50, 0, 50, 100}
	regimes := []string{"INFLATIONARY", "RECESSION", "STRESS", "GOLDILOCKS"}
	currencies := []string{"USD", "EUR", "AUD", "JPY"}

	for _, sent := range extremes {
		for _, reg := range regimes {
			for _, cur := range currencies {
				a := domain.COTAnalysis{
					Contract:       domain.COTContract{Currency: cur, ReportType: "TFF"},
					SentimentScore: sent,
				}
				cs := ComputeConvictionScore(a, fred.MacroRegime{Name: reg}, 5.0, "", nil)
				if cs.Score < 0 || cs.Score > 100 {
					t.Errorf("score %.2f out of [0,100] for sent=%.0f regime=%s cur=%s",
						cs.Score, sent, reg, cur)
				}
			}
		}
	}
}

func TestComputeConvictionScore_WithMacroData(t *testing.T) {
	macro := &fred.MacroData{
		YieldSpread:   0.5,
		CorePCE:       2.0,
		NFCI:          -0.5,
		InitialClaims: 200_000,
	}
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{Currency: "EUR", ReportType: "TFF"},
		SentimentScore: 60,
	}
	regime := fred.MacroRegime{Name: "DISINFLATIONARY"} // EUR → +10

	cs := ComputeConvictionScore(a, regime, 1.0, "positive data", macro)

	if cs.Score < 0 || cs.Score > 100 {
		t.Errorf("score out of range: %.2f", cs.Score)
	}
	if cs.Direction != "LONG" {
		t.Errorf("expected LONG direction for bullish inputs, got %s (score=%.2f)", cs.Direction, cs.Score)
	}
}

// ---------------------------------------------------------------------------
// TASK-173 — nil-pointer guard tests for FRED composites paths
// ---------------------------------------------------------------------------

// TestConfluenceScoreV2_EmptyMacroData verifies that ConfluenceScoreV2 does
// not panic when macroData is non-nil but contains all zero-value fields.
// ComputeComposites should still return a valid struct and the code should
// pick up the USScore / country differential path safely.
func TestConfluenceScoreV2_EmptyMacroData(t *testing.T) {
	macro := &fred.MacroData{} // all zero values — no raw data available
	a := domain.COTAnalysis{
		Contract:       baseContract("TFF"),
		SentimentScore: 50,
	}
	// Should not panic; score should be within [-100, 100].
	score := ConfluenceScoreV2(a, macro, 0.0)
	if score < -100 || score > 100 {
		t.Errorf("expected score in [-100,100], got %.2f", score)
	}
}

// TestConfluenceScoreV2_EmptyMacroData_NonUSDPair verifies the composites
// differential path for a non-USD pair with empty macro data.
func TestConfluenceScoreV2_EmptyMacroData_NonUSDPair(t *testing.T) {
	macro := &fred.MacroData{} // zero-value
	a := domain.COTAnalysis{
		Contract: domain.COTContract{
			Currency:   "EUR",
			ReportType: "TFF",
		},
		SentimentScore: 60,
	}
	// Should not panic even for non-USD pair with empty composites.
	score := ConfluenceScoreV2(a, macro, 1.0)
	if score < -100 || score > 100 {
		t.Errorf("expected score in [-100,100], got %.2f", score)
	}
}

// TestComputeConvictionScore_EmptyMacroData checks that conviction score
// handles empty MacroData (composites path) without panicking.
func TestComputeConvictionScore_EmptyMacroData(t *testing.T) {
	macro := &fred.MacroData{}
	a := domain.COTAnalysis{
		Contract:       domain.COTContract{Currency: "GBP", ReportType: "TFF"},
		SentimentScore: 70,
	}
	regime := fred.MacroRegime{Name: "DISINFLATIONARY"}

	cs := ComputeConvictionScore(a, regime, 1.0, "partial data", macro)
	if cs.Score < 0 || cs.Score > 100 {
		t.Errorf("score out of range: %.2f", cs.Score)
	}
}

// TestComputeComposites_NilDataPath verifies ComputeComposites returns nil for
// nil input and a valid non-nil struct for empty-but-non-nil input.
func TestComputeComposites_NilDataPath(t *testing.T) {
	got := fred.ComputeComposites(nil)
	if got != nil {
		t.Error("ComputeComposites(nil) should return nil")
	}

	empty := fred.ComputeComposites(&fred.MacroData{})
	if empty == nil {
		t.Error("ComputeComposites(&MacroData{}) should return non-nil struct")
	}
	// All scores should be in valid ranges even with zero-value input.
	if empty.LaborHealth < 0 || empty.LaborHealth > 100 {
		t.Errorf("LaborHealth out of range for empty data: %.2f", empty.LaborHealth)
	}
	if empty.CreditStress < 0 || empty.CreditStress > 100 {
		t.Errorf("CreditStress out of range for empty data: %.2f", empty.CreditStress)
	}
	if empty.VIXTermRegime == "" {
		t.Error("VIXTermRegime should have default value for empty data")
	}
}
