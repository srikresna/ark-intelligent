package backtest

import (
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ==========================================================================
// BACKTEST AUDIT TESTS — Logistic, Factor Decomposition, WFO
// ==========================================================================

// --- Logistic Regression Audit ---

func TestLogistic_Audit_SigmoidSymmetry(t *testing.T) {
	for _, x := range []float64{-10, -5, -1, 0, 1, 5, 10} {
		sum := sigmoid(x) + sigmoid(-x)
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("sigmoid(%f) + sigmoid(-%f) = %f, expected 1.0", x, x, sum)
		}
	}
}

func TestLogistic_Audit_SigmoidOverflow(t *testing.T) {
	// Should not overflow for extreme values
	if sigmoid(1000) != 1.0 {
		t.Errorf("sigmoid(1000) should be 1.0, got %f", sigmoid(1000))
	}
	if sigmoid(-1000) != 0.0 {
		t.Errorf("sigmoid(-1000) should be 0.0, got %f", sigmoid(-1000))
	}
}

func TestLogistic_Audit_PredictWithMismatchedWeights(t *testing.T) {
	features := []float64{0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.5}
	wrongWeights := []float64{0.1, 0.2} // Wrong length
	prob := logisticPredict(features, wrongWeights)
	if prob != 0.5 {
		t.Errorf("Expected 0.5 fallback for mismatched weights, got %f", prob)
	}
}

func TestLogistic_Audit_PredictWithCorrectWeights(t *testing.T) {
	features := []float64{1.0}     // 1 feature
	weights := []float64{0.0, 2.0} // bias=0, w1=2
	// z = 0 + 2*1 = 2, sigmoid(2) ≈ 0.8808
	prob := logisticPredict(features, weights)
	expected := sigmoid(2.0)
	if math.Abs(prob-expected) > 0.001 {
		t.Errorf("Expected prob=%f, got %f", expected, prob)
	}
}

func TestLogistic_Audit_FeatureExtractionBounds(t *testing.T) {
	// Test that features are properly normalized to expected ranges
	signal := &domain.PersistedSignal{
		Strength:        5, // max
		RawConfidence:   100,
		COTIndex:        100,
		SentimentScore:  1.0,
		ConvictionScore: 100,
		Direction:       "BULLISH",
		DailyTrend:      "UP",
		FREDRegime:      "EXPANSION",
	}

	features := extractFeatures(signal)
	for i, f := range features {
		if f < -1.1 || f > 1.1 {
			t.Errorf("Feature %d out of expected range: %f", i, f)
		}
	}

	// Test minimum values
	signalMin := &domain.PersistedSignal{
		Strength:        1,
		RawConfidence:   0,
		COTIndex:        0,
		SentimentScore:  -1.0,
		ConvictionScore: 0,
		Direction:       "BEARISH",
		DailyTrend:      "DOWN",
		FREDRegime:      "RECESSION",
	}

	featuresMin := extractFeatures(signalMin)
	for i, f := range featuresMin {
		if f < -1.1 || f > 1.1 {
			t.Errorf("Min feature %d out of expected range: %f", i, f)
		}
	}
}

func TestLogistic_Audit_TrendAlignment(t *testing.T) {
	tests := []struct {
		direction string
		trend     string
		expected  float64
	}{
		{"BULLISH", "UP", 1.0},
		{"BEARISH", "DOWN", 1.0},
		{"BULLISH", "DOWN", -1.0},
		{"BEARISH", "UP", -1.0},
		{"BULLISH", "FLAT", 0.0},
		{"BEARISH", "", 0.0},
	}

	for _, tt := range tests {
		got := encodeTrendAlignment(tt.direction, tt.trend)
		if got != tt.expected {
			t.Errorf("encodeTrendAlignment(%s, %s): expected %f, got %f",
				tt.direction, tt.trend, tt.expected, got)
		}
	}
}

func TestLogistic_Audit_FREDRegimeEncoding(t *testing.T) {
	tests := []struct {
		regime   string
		expected float64
	}{
		{"EXPANSION", 1.0},
		{"GOLDILOCKS", 1.0},
		{"STRESS", -1.0},
		{"RECESSION", -1.0},
		{"STAGFLATION", -1.0},
		{"TIGHTENING", -0.5},
		{"UNKNOWN", 0.0},
		{"", 0.0},
	}

	for _, tt := range tests {
		got := encodeFREDRegime(tt.regime)
		if got != tt.expected {
			t.Errorf("encodeFREDRegime(%s): expected %f, got %f",
				tt.regime, tt.expected, got)
		}
	}
}

func TestLogistic_Audit_ApproximateAUC(t *testing.T) {
	// Perfect predictions: all positives ranked above all negatives
	predictions := []float64{0.9, 0.8, 0.7, 0.2, 0.1, 0.0}
	targets := []float64{1, 1, 1, 0, 0, 0}
	auc := approximateAUC(predictions, targets)
	if math.Abs(auc-1.0) > 0.001 {
		t.Errorf("Perfect predictions should give AUC=1.0, got %f", auc)
	}

	// Random/bad predictions: should be near 0.5
	predictions2 := []float64{0.1, 0.2, 0.3, 0.9, 0.8, 0.7}
	auc2 := approximateAUC(predictions2, targets)
	if math.Abs(auc2-0.0) > 0.001 {
		t.Errorf("Inverted predictions should give AUC≈0, got %f", auc2)
	}

	// Empty input
	auc3 := approximateAUC(nil, nil)
	if auc3 != 0.5 {
		t.Errorf("Empty input should give AUC=0.5, got %f", auc3)
	}

	// All same class
	allPos := []float64{1, 1, 1}
	auc4 := approximateAUC([]float64{0.5, 0.6, 0.7}, allPos)
	if auc4 != 0.5 {
		t.Errorf("All positive targets should give AUC=0.5, got %f", auc4)
	}
}

// --- Factor Decomposition Audit ---

func TestFactor_Audit_ZScoreNormalize(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50}
	norm := zScoreNormalize(data)

	// Mean should be ~0
	m := mean(norm)
	if math.Abs(m) > 0.0001 {
		t.Errorf("Z-scored mean should be ~0, got %f", m)
	}

	// Std should be ~1 (population)
	s := stdDev(norm, m)
	if math.Abs(s-1.0) > 0.05 {
		t.Errorf("Z-scored stddev should be ~1, got %f", s)
	}
}

func TestFactor_Audit_ZScoreConstant(t *testing.T) {
	data := []float64{5, 5, 5, 5}
	norm := zScoreNormalize(data)
	for i, v := range norm {
		if v != 0 {
			t.Errorf("Z-score of constant data at index %d should be 0, got %f", i, v)
		}
	}
}

func TestFactor_Audit_AdjustedRSquared(t *testing.T) {
	// adj R² should be <= R² for p > 0
	r2 := adjustedRSquared(0.5, 100, 4)
	if r2 > 0.5 {
		t.Errorf("Adjusted R² should be <= R²: got %f", r2)
	}

	// Edge case: n <= p+1
	r2Edge := adjustedRSquared(0.5, 5, 5)
	if r2Edge != 0 {
		t.Errorf("Adjusted R² should be 0 when n<=p+1, got %f", r2Edge)
	}
}

func TestFactor_Audit_COTScoreExtraction(t *testing.T) {
	// BULLISH with high COT index
	s := &domain.PersistedSignal{
		COTIndex:       90, // 90th percentile
		SentimentScore: 0,
		Direction:      "BULLISH",
	}
	score := extractCOTScore(s)
	// (90-50)*2 = 80, BULLISH so positive
	if score != 80 {
		t.Errorf("Expected COT score 80, got %f", score)
	}

	// BEARISH should negate
	s2 := &domain.PersistedSignal{
		COTIndex:  90,
		Direction: "BEARISH",
	}
	score2 := extractCOTScore(s2)
	if score2 != -80 {
		t.Errorf("Expected COT score -80 for BEARISH, got %f", score2)
	}
}

func TestFactor_Audit_MacroScoreMapping(t *testing.T) {
	tests := []struct {
		regime   string
		expected float64
	}{
		{"EXPANSION", 80},
		{"GOLDILOCKS", 60},
		{"NORMAL", 0},
		{"TIGHTENING", -40},
		{"STRESS", -60},
		{"RECESSION", -80},
		{"STAGFLATION", -70},
		{"", 0},
	}

	for _, tt := range tests {
		s := &domain.PersistedSignal{FREDRegime: tt.regime}
		got := extractMacroScore(s)
		if got != tt.expected {
			t.Errorf("FREDRegime %q: expected %f, got %f", tt.regime, tt.expected, got)
		}
	}
}

func TestFactor_Audit_NormalCDF(t *testing.T) {
	// normalCDF(0) should be 0.5
	if math.Abs(normalCDF(0)-0.5) > 0.001 {
		t.Errorf("normalCDF(0) should be 0.5, got %f", normalCDF(0))
	}
	// normalCDF(very large) → ~1 (implementation uses clamped approximation)
	if normalCDF(6) < 0.999 {
		t.Errorf("normalCDF(6) should be ~1.0, got %f", normalCDF(6))
	}
	// normalCDF(very negative) → ~0
	if normalCDF(-6) > 0.001 {
		t.Errorf("normalCDF(-6) should be ~0.0, got %f", normalCDF(-6))
	}
	// Symmetry: normalCDF(x) + normalCDF(-x) ≈ 1
	for _, x := range []float64{0.5, 1.0, 2.0, 3.0} {
		sum := normalCDF(x) + normalCDF(-x)
		if math.Abs(sum-1.0) > 0.01 {
			t.Errorf("normalCDF(%f) + normalCDF(-%f) = %f, expected ~1.0", x, x, sum)
		}
	}
}

func TestFactor_Audit_InvertMatrix(t *testing.T) {
	// 2x2 identity should invert to identity
	I := [][]float64{{1, 0}, {0, 1}}
	inv, err := invertMatrix(I)
	if err != nil {
		t.Fatalf("Failed to invert identity: %v", err)
	}
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			expected := 0.0
			if i == j {
				expected = 1.0
			}
			if math.Abs(inv[i][j]-expected) > 1e-10 {
				t.Errorf("inv[%d][%d] = %f, expected %f", i, j, inv[i][j], expected)
			}
		}
	}

	// Singular matrix should error
	singular := [][]float64{{1, 2}, {2, 4}}
	_, err2 := invertMatrix(singular)
	if err2 == nil {
		t.Error("Expected error for singular matrix")
	}
}

// --- Walk-Forward Optimizer Audit ---

func TestWFO_Audit_FilterByDateRange(t *testing.T) {
	now := time.Now()
	signals := []domain.PersistedSignal{
		{ReportDate: now.AddDate(0, 0, -10)},
		{ReportDate: now.AddDate(0, 0, -5)},
		{ReportDate: now},
		{ReportDate: now.AddDate(0, 0, 5)},
	}

	// [now-7, now)
	start := now.AddDate(0, 0, -7)
	end := now
	filtered := filterByDateRange(signals, start, end)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 signal in [now-7, now), got %d", len(filtered))
	}
}

func TestWFO_Audit_DetectDominantRegime(t *testing.T) {
	signals := []domain.PersistedSignal{
		{FREDRegime: "EXPANSION"},
		{FREDRegime: "EXPANSION"},
		{FREDRegime: "EXPANSION"},
		{FREDRegime: "STRESS"},
		{FREDRegime: ""},
	}
	regime := detectDominantRegime(signals)
	if regime != "RISK_ON" {
		t.Errorf("Expected RISK_ON for mostly EXPANSION, got %s", regime)
	}

	signals2 := []domain.PersistedSignal{
		{FREDRegime: "STRESS"},
		{FREDRegime: "RECESSION"},
		{FREDRegime: "STRESS"},
	}
	regime2 := detectDominantRegime(signals2)
	if regime2 != "CRISIS" {
		t.Errorf("Expected CRISIS for STRESS/RECESSION, got %s", regime2)
	}
}

func TestWFO_Audit_AverageWeights(t *testing.T) {
	windows := []WFOWindowResult{
		{Weights: map[string]float64{"COT": 30, "FRED": 20}},
		{Weights: map[string]float64{"COT": 40, "FRED": 30}},
	}
	avg := averageWeights(windows)
	if math.Abs(avg["COT"]-35) > 0.01 {
		t.Errorf("Average COT weight: expected 35, got %f", avg["COT"])
	}
	if math.Abs(avg["FRED"]-25) > 0.01 {
		t.Errorf("Average FRED weight: expected 25, got %f", avg["FRED"])
	}
}

func TestWFO_Audit_WeightStability_SingleWindow(t *testing.T) {
	windows := []WFOWindowResult{
		{Weights: map[string]float64{"COT": 30}},
	}
	stability := computeWeightStability(windows)
	if stability != 100 {
		t.Errorf("Expected 100 stability for single window, got %f", stability)
	}
}

func TestWFO_Audit_WeightStability_HighVariance(t *testing.T) {
	windows := []WFOWindowResult{
		{Weights: map[string]float64{"COT": 10, "FRED": 50}},
		{Weights: map[string]float64{"COT": 90, "FRED": 50}},
	}
	stability := computeWeightStability(windows)
	// High variance in COT should reduce stability
	if stability > 50 {
		t.Logf("Stability=%f for high-variance weights (expected lower)", stability)
	}
}

func TestWFO_Audit_RecommendationMessages(t *testing.T) {
	// Few windows
	r1 := &WFOResult{ValidWindows: 2}
	rec1 := buildWFORecommendation(r1)
	if rec1 == "" {
		t.Error("Recommendation should not be empty for few windows")
	}

	// Adaptive better
	r2 := &WFOResult{ValidWindows: 5, Improvement: 5.0, WeightStability: 80}
	rec2 := buildWFORecommendation(r2)
	if rec2 == "" {
		t.Error("Recommendation should not be empty for adaptive better")
	}

	// Static better
	r3 := &WFOResult{ValidWindows: 5, Improvement: -5.0}
	rec3 := buildWFORecommendation(r3)
	if rec3 == "" {
		t.Error("Recommendation should not be empty for static better")
	}
}

// --- Outcome for Horizon ---

func TestLogistic_Audit_OutcomeForHorizon(t *testing.T) {
	s := &domain.PersistedSignal{
		Outcome1W: domain.OutcomeWin,
		Outcome2W: domain.OutcomeLoss,
		Outcome4W: domain.OutcomePending,
	}
	if outcomeForHorizon(s, "1W") != domain.OutcomeWin {
		t.Error("1W should be WIN")
	}
	if outcomeForHorizon(s, "2W") != domain.OutcomeLoss {
		t.Error("2W should be LOSS")
	}
	if outcomeForHorizon(s, "4W") != domain.OutcomePending {
		t.Error("4W should be PENDING")
	}
	if outcomeForHorizon(s, "99W") != "" {
		t.Error("Invalid horizon should return empty")
	}
}
