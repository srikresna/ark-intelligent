package backtest

import (
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ==========================================================================
// AUDIT PASS 4 — Deep cross-component verification & bug reproduction
// ==========================================================================

// ---------------------------------------------------------------------------
// BUG REPRODUCTION: SentimentScore range mismatch
// ---------------------------------------------------------------------------

// TestBug_SentimentScore_LogisticFeature_ProductionRange reproduces the bug where
// logistic_calibration.go:176 assumes SentimentScore in [-1, +1] but actual
// production values are in [-100, +100].
func TestBug_SentimentScore_LogisticFeature_ProductionRange(t *testing.T) {
	// Simulate production values from computeSentiment() -> [-100, +100]
	testCases := []struct {
		name           string
		sentimentScore float64
		expectInBounds bool // Should x3 be in [0, 1]?
	}{
		{"max_positive", 100.0, false},  // (100+1)/2 = 50.5, WAY outside [0,1]
		{"max_negative", -100.0, false}, // (-100+1)/2 = -49.5, WAY outside [0,1]
		{"moderate_positive", 60.0, false},
		{"moderate_negative", -40.0, false},
		{"zero", 0.0, true}, // (0+1)/2 = 0.5, OK
		{"tiny_positive", 0.5, true},
		{"tiny_negative", -0.5, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signal := &domain.PersistedSignal{
				SentimentScore: tc.sentimentScore,
				Strength:       3,
				COTIndex:       50,
			}
			features := extractFeatures(signal)
			x3 := features[3]
			inBounds := x3 >= 0.0 && x3 <= 1.0

			if inBounds != tc.expectInBounds {
				if tc.expectInBounds {
					t.Errorf("SentimentScore=%f: x3=%f should be in [0,1]", tc.sentimentScore, x3)
				} else {
					t.Logf("BUG CONFIRMED: SentimentScore=%f -> x3=%f (outside [0,1]). "+
						"Formula (score+1)/2 assumes [-1,+1] but production range is [-100,+100]",
						tc.sentimentScore, x3)
				}
			}
		})
	}
}

// TestBug_SentimentScore_FactorDecomp_100xMultiplier reproduces the bug where
// factor_decomposition.go:239 multiplies SentimentScore by 100, suggesting
// developer expected [-1, +1] but actual range is [-100, +100].
func TestBug_SentimentScore_FactorDecomp_100xMultiplier(t *testing.T) {
	// With production range [-100, +100], the formula:
	//   score = cotIndexScore*0.6 + SentimentScore*100*0.4
	// becomes: score = cotIndexScore*0.6 + (100)*100*0.4 = cotIndexScore*0.6 + 4000
	// Then clamped to [-100, +100], making COT index contribution irrelevant.

	signal := &domain.PersistedSignal{
		COTIndex:       75,
		SentimentScore: 50.0, // Moderate positive (production range)
		Direction:      "BULLISH",
	}
	score := extractCOTScore(signal)

	// With correct range [-100,+100]:
	// cotIndexPart = (75-50)*2 = 50
	// sentimentPart = 50 * 100 * 0.4 = 2000
	// combined = 50*0.6 + 2000 = 2030
	// clamped to 100 (for BULLISH, no negation)
	// Since direction is BULLISH and score is positive, no negation
	// The sentiment part completely dominates due to *100 multiplier

	if score == 100.0 || score == -100.0 {
		t.Logf("BUG CONFIRMED: extractCOTScore with SentimentScore=50 (production range) "+
			"saturates to %f. The *100 multiplier on line 239 assumes [-1,+1] range "+
			"but receives [-100,+100], making the score always clamp to ±100", score)
	}

	// Compare with a very different COTIndex to show it has no effect
	signal2 := &domain.PersistedSignal{
		COTIndex:       25, // Very different from 75
		SentimentScore: 50.0,
		Direction:      "BULLISH",
	}
	score2 := extractCOTScore(signal2)

	if score == score2 {
		t.Logf("BUG IMPACT: Changing COTIndex from 75 to 25 produces same score (%f) "+
			"because SentimentScore*100 dominates and clamps to ±100", score)
	}
}

// ---------------------------------------------------------------------------
// GARCH: Hardcoded Converged=true verification
// ---------------------------------------------------------------------------

func TestBug_GARCH_ConvergedAlwaysTrue(t *testing.T) {
	// Generate minimal data that might produce a poor fit
	// With only 30 data points (minimum), convergence is questionable
	rng := newRNG(9876)
	prices := make([]domain.PriceRecord, 30)
	for i := 29; i >= 0; i-- {
		p := 100.0 + rng.NormFloat64()*0.001 // Extremely low volatility
		if p <= 0 {
			p = 0.01
		}
		prices[i] = domain.PriceRecord{
			Close: p,
			High:  p + 0.001,
			Low:   p - 0.001,
		}
	}

	// Note: EstimateGARCH is in price package, can't call from here.
	// Instead, we verify the GARCHResult struct always has Converged=true
	// by checking the code comment. This is a documentation test.
	t.Log("GARCH Converged field is hardcoded to true on garch.go:222. " +
		"This means even poor-quality fits are treated as converged, " +
		"affecting GARCHConfidenceMultiplier which checks g.Converged.")
}

// ---------------------------------------------------------------------------
// Walk-Forward: evaluateWithWeights win counting analysis
// ---------------------------------------------------------------------------

func TestBug_WFO_AmbiguousWinCounting(t *testing.T) {
	// The evaluateWithWeights function counts as "win" when:
	// 1. Model agrees with signal direction AND signal won
	// 2. Model DISAGREES with signal direction AND signal lost
	// Case 2 inflates win rates by counting losses as wins.

	signals := []domain.PersistedSignal{
		{Direction: "BULLISH", Outcome1W: domain.OutcomeWin, Return1W: 0.5,
			SentimentScore: 50, COTIndex: 70, ConvictionScore: 70, FREDRegime: "GOLDILOCKS"},
		{Direction: "BEARISH", Outcome1W: domain.OutcomeLoss, Return1W: -0.3,
			SentimentScore: 50, COTIndex: 70, ConvictionScore: 70, FREDRegime: "GOLDILOCKS"},
	}

	// Use all-positive weights to make scores positive
	weights := map[string]float64{
		"COT": 30, "Stress": 15, "FRED": 25, "Price": 30,
	}

	winRate, _, wins, total := evaluateWithWeights(signals, weights)

	t.Logf("evaluateWithWeights: winRate=%.1f%%, wins=%d, total=%d", winRate, wins, total)

	// If both signals score positively (because weights are positive and features are positive):
	// Signal 1: positive score + BULLISH direction = agrees, Outcome=WIN -> counted as win
	// Signal 2: positive score + BEARISH direction = disagrees, Outcome=LOSS -> also counted as win!
	// This means a LOSS is counted as a "win" when the model disagrees with the original direction.

	if wins == total && total > 0 {
		t.Logf("BUG CONFIRMED: All %d signals counted as wins (%.0f%% win rate) "+
			"even though signal 2 was a LOSS. The ambiguous win-counting "+
			"at walkforward_optimizer.go:305-313 inflates OOS win rates.", total, winRate)
	}
}

// ---------------------------------------------------------------------------
// Population vs Sample StdDev in Factor Decomposition
// ---------------------------------------------------------------------------

func TestFactorDecomp_PopulationVsSampleStdDev(t *testing.T) {
	// zScoreNormalize now uses sample stddev (N-1) for unbiased estimation.
	data := []float64{10, 20, 30}

	zScored := zScoreNormalize(data)

	// Sample stddev = sqrt(((10-20)^2+(20-20)^2+(30-20)^2)/2) = sqrt(200/2) = 10.0
	m := mean(data)
	sd := stdDev(data, m)
	expectedSD := math.Sqrt(200.0 / 2.0)

	t.Logf("stdDev=%.4f, expected sample SD=%.4f", sd, expectedSD)

	if math.Abs(sd-expectedSD) > 0.01 {
		t.Errorf("stdDev should use sample stddev (N-1): got %.4f, expected %.4f", sd, expectedSD)
	}

	// With sample stddev, z-scores should have sample variance = 1.0
	variance := 0.0
	for _, v := range zScored {
		variance += v * v
	}
	variance /= float64(len(zScored) - 1) // sample variance

	if math.Abs(variance-1.0) > 0.01 {
		t.Errorf("z-score variance with population SD should be ~1.0, got %f", variance)
	}
}

// ---------------------------------------------------------------------------
// Cross-component: Weight extraction consistency
// ---------------------------------------------------------------------------

func TestCrossComponent_FactorScoreExtraction_Consistency(t *testing.T) {
	// extractFactorScores (weights.go) and extractCOTScore (factor_decomposition.go)
	// should produce consistent COT scores for the same signal.
	// They use different normalization approaches.

	signal := domain.PersistedSignal{
		COTIndex:        75.0,
		SentimentScore:  0.0, // Use 0 to avoid the *100 bug
		ConvictionScore: 70.0,
		Direction:       "BULLISH",
		FREDRegime:      "GOLDILOCKS",
	}

	// weights.go: cotScore = SentimentScore (0), fallback to (COTIndex-50)*2 = 50
	// Then normalized to [-1,+1]: 50/100 = 0.5
	weightsScores := extractFactorScores(signal)
	weightsCOT := weightsScores[0] // Should be 0.5

	// factor_decomposition.go: score = (COTIndex-50)*2 = 50, SentimentScore=0 so no blend
	// Direction BULLISH: no negation since score > 0
	// clamped to [-100, 100]: 50
	decompCOT := extractCOTScore(&signal) // Should be 50

	// Both should represent the same COT positioning, just at different scales
	// weights: 0.5 (in [-1,1]), decomp: 50 (in [-100,100])
	if math.Abs(weightsCOT*100-decompCOT) > 1.0 {
		t.Errorf("COT score inconsistency: weights=%.4f (x100=%.1f), decomp=%.1f",
			weightsCOT, weightsCOT*100, decompCOT)
	}
}

// ---------------------------------------------------------------------------
// FRED Regime Encoding: weights.go vs factor_decomposition.go vs logistic
// ---------------------------------------------------------------------------

func TestCrossComponent_FREDRegimeEncoding_Comparison(t *testing.T) {
	regimes := []string{"GOLDILOCKS", "EXPANSION", "STRESS", "RECESSION", "STAGFLATION", "TIGHTENING", "NORMAL", "INFLATIONARY", "DISINFLATIONARY"}

	for _, regime := range regimes {
		// logistic_calibration.go: encodeFREDRegime
		logisticVal := encodeFREDRegime(regime)

		// weights.go: fredRegimeToScore
		weightsVal := fredRegimeToScore(regime)

		// weights.go: fredRegimeToStressScore
		stressVal := fredRegimeToStressScore(regime)

		// Check they have consistent directionality
		// Bullish regimes should be positive in all encodings
		// Bearish regimes should be negative
		logisticDir := sign(logisticVal)
		weightsDir := sign(weightsVal)

		if logisticVal != 0 && weightsVal != 0 && logisticDir != weightsDir {
			t.Logf("WARNING: Regime %s has different sign in logistic (%+.1f) vs weights (%+.1f)",
				regime, logisticVal, weightsVal)
		}

		t.Logf("Regime %-15s: logistic=%+.1f, weights_fred=%+.1f, weights_stress=%+.1f",
			regime, logisticVal, weightsVal, stressVal)
	}
}

// ---------------------------------------------------------------------------
// Logistic: IRLS convergence with all-zero features
// ---------------------------------------------------------------------------

func TestLogistic_AllZeroFeatures(t *testing.T) {
	// If all features are zero (except bias), IRLS should still converge
	n := 40
	signals := make([]domain.PersistedSignal, n)
	for i := 0; i < n; i++ {
		signals[i] = domain.PersistedSignal{
			Strength:        3,    // maps to 0.5
			Confidence:      50.0, // maps to 0.5
			COTIndex:        50.0, // maps to 0.5
			SentimentScore:  0.0,  // maps to 0.5
			ConvictionScore: 50.0, // maps to 0.5
			DailyTrend:      "FLAT",
			Direction:       "BULLISH",
			FREDRegime:      "NORMAL",
			Outcome1W:       domain.OutcomeWin,
		}
		if i%2 == 0 {
			signals[i].Outcome1W = domain.OutcomeLoss
		}
	}

	model, err := trainLogistic(signals, "1W")
	if err != nil {
		t.Fatalf("trainLogistic failed with neutral features: %v", err)
	}

	// With 50/50 outcomes and near-constant features, bias should be ~0
	if math.Abs(model.Weights[0]) > 2.0 {
		t.Errorf("Bias weight too large for 50/50 outcomes: %f", model.Weights[0])
	}

	// AUC should be around 0.5 (no discriminative power)
	if math.Abs(model.TrainAUC-0.5) > 0.15 {
		t.Errorf("Expected AUC ~0.5 for non-discriminative features, got %f", model.TrainAUC)
	}

	t.Logf("Neutral features: AUC=%.4f, Brier=%.4f, converged=%v, iterations=%d",
		model.TrainAUC, model.BrierScore, model.Converged, model.Iterations)
}

// ---------------------------------------------------------------------------
// Logistic: Predict with mismatched weight dimensions
// ---------------------------------------------------------------------------

func TestLogistic_PredictDimensionMismatch(t *testing.T) {
	features := []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.0, 0.0}

	// Correct: 8 weights (1 bias + 7 features)
	correctWeights := make([]float64, 8)
	result := logisticPredict(features, correctWeights)
	if math.Abs(result-0.5) > 0.001 {
		t.Errorf("Zero weights should predict 0.5, got %f", result)
	}

	// Mismatched: wrong number of weights
	wrongWeights := make([]float64, 5)
	result = logisticPredict(features, wrongWeights)
	if result != 0.5 {
		t.Errorf("Mismatched dimensions should return fallback 0.5, got %f", result)
	}
}

// ---------------------------------------------------------------------------
// OLS regression: negative R² clamping
// ---------------------------------------------------------------------------

func TestFactorDecomp_NegativeR2Clamping(t *testing.T) {
	// Construct data where intercept-only model is better than full model
	// This can produce negative R²
	n := 20
	X := make([][]float64, n)
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		x1 := float64(i) / float64(n)
		X[i] = []float64{1.0, x1}
		// y has no relationship with x1, plus noise
		y[i] = 5.0
		if i%3 == 0 {
			y[i] = 100.0 // Outlier
		}
	}

	_, r2, _ := simpleOLS(X, y)

	// R² should be >= 0 (clamped)
	if r2 < 0 {
		t.Errorf("simpleOLS should clamp negative R² to 0, got %f", r2)
	}
}

// ---------------------------------------------------------------------------
// Walk-forward: window overlap verification
// ---------------------------------------------------------------------------

func TestWFO_WindowOverlap(t *testing.T) {
	// Verify that train and test windows don't overlap
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	trainDur := time.Duration(wfoTrainWeeks) * 7 * 24 * time.Hour
	testDur := time.Duration(wfoTestWeeks) * 7 * 24 * time.Hour

	trainEnd := base.Add(trainDur)
	testStart := trainEnd
	testEnd := testStart.Add(testDur)

	// Train: [base, trainEnd), Test: [testStart, testEnd)
	// testStart == trainEnd, so no overlap (half-open intervals)
	if testStart.Before(trainEnd) {
		t.Error("Test window starts before train window ends (overlap)")
	}

	// Check step size = test duration
	nextWindowStart := base.Add(testDur)
	if nextWindowStart.After(trainEnd) {
		t.Logf("Rolling windows: train=%dW, test=%dW, step=%dW",
			wfoTrainWeeks, wfoTestWeeks, wfoTestWeeks)
	}

	t.Logf("Window: train=[%s, %s), test=[%s, %s)",
		base.Format("2006-01-02"), trainEnd.Format("2006-01-02"),
		testStart.Format("2006-01-02"), testEnd.Format("2006-01-02"))
}

// ---------------------------------------------------------------------------
// normalizeCoefficients: edge cases
// ---------------------------------------------------------------------------

func TestWeights_NormalizeCoefficients_AllZero(t *testing.T) {
	coefficients := []float64{0, 0, 0, 0}
	weights := normalizeCoefficients(coefficients)

	// All zero coefficients -> equal weights (25% each)
	for _, name := range factorNames {
		w := weights[name]
		if math.Abs(w-25.0) > 0.5 {
			t.Errorf("Factor %s: expected ~25.0 for all-zero coefficients, got %.2f", name, w)
		}
	}
}

func TestWeights_NormalizeCoefficients_SingleDominant(t *testing.T) {
	coefficients := []float64{100, 0.01, 0.01, 0.01}
	weights := normalizeCoefficients(coefficients)

	// Factor 0 (COT) should dominate
	cotWeight := weights[factorNames[0]]
	if cotWeight < 90 {
		t.Errorf("Dominant factor should have >90%% weight, got %.2f%%", cotWeight)
	}
}

// ---------------------------------------------------------------------------
// Pearson correlation mathematical properties
// ---------------------------------------------------------------------------

func TestPearsonCorrelation_Antisymmetry(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := make([]float64, len(x))
	for i := range x {
		y[i] = -x[i] // Perfect negative correlation
	}

	r := pearsonCorrelation(x, y)
	if math.Abs(r+1.0) > 0.001 {
		t.Errorf("Perfect negative correlation should be -1.0, got %f", r)
	}
}

func TestPearsonCorrelation_ZeroVariance(t *testing.T) {
	x := []float64{5, 5, 5, 5, 5}
	y := []float64{1, 2, 3, 4, 5}

	r := pearsonCorrelation(x, y)
	if r != 0 {
		t.Errorf("Zero-variance series should produce correlation 0, got %f", r)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sign(v float64) float64 {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

func newRNG(seed int64) *localRNG {
	return &localRNG{seed: seed}
}

type localRNG struct {
	seed int64
	idx  int
}

func (r *localRNG) NormFloat64() float64 {
	// Simple LCG + Box-Muller approximation
	r.idx++
	r.seed = r.seed*6364136223846793005 + 1442695040888963407
	u1 := float64(uint64(r.seed)>>1) / float64(1<<63)
	if u1 < 1e-15 {
		u1 = 1e-15
	}
	r.seed = r.seed*6364136223846793005 + 1442695040888963407
	u2 := float64(uint64(r.seed)>>1) / float64(1<<63)
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}
