package backtest

import (
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ==========================================================================
// DEEP AUDIT — Second-pass backtest stress tests
// ==========================================================================

// --- Logistic: Feature extraction SentimentScore range ---
func TestLogistic_Deep_SentimentScoreRange(t *testing.T) {
	// SentimentScore range is unclear -- comment says "-1 to 1" but
	// if actual values are in a wider range, normalization (score+1)/2 breaks.
	// Here we document the behavior for a "normal" score of 0.8 (within [-1,1]).

	signal := &domain.PersistedSignal{
		SentimentScore: 0.8, // In the [-1, 1] range per the comment
		Strength:       3,
		COTIndex:       70,
	}
	features := extractFeatures(signal)

	// x3 = (0.8 + 1) / 2 = 0.9 -- should be in [0, 1]
	if features[3] > 1.0 || features[3] < 0.0 {
		t.Logf("BUG: x3 (sentiment) = %f, outside [0,1]. SentimentScore normalization may be incorrect.", features[3])
	} else {
		t.Logf("x3 (sentiment) = %f, within [0,1] for SentimentScore=0.8", features[3])
	}
}

// --- Logistic: Feature extraction bounds ---
func TestLogistic_Deep_FeatureBounds(t *testing.T) {
	// All features except x3 (sentiment) and x5 (trend, can be -1) should be in [0,1]
	signal := &domain.PersistedSignal{
		Strength:        5,
		Confidence:      100.0,
		COTIndex:        100.0,
		SentimentScore:  0.0, // Use 0 to avoid edge cases
		ConvictionScore: 100.0,
		Direction:       "BULLISH",
		DailyTrend:      "UP",
		FREDRegime:      "GOLDILOCKS",
	}
	features := extractFeatures(signal)

	// x0: (5-1)/4 = 1.0
	if features[0] < 0 || features[0] > 1 {
		t.Errorf("x0 out of range: %f", features[0])
	}
	// x1: 100/100 = 1.0
	if features[1] < 0 || features[1] > 1 {
		t.Errorf("x1 out of range: %f", features[1])
	}
	// x2: 100/100 = 1.0
	if features[2] < 0 || features[2] > 1 {
		t.Errorf("x2 out of range: %f", features[2])
	}
	// x4: 100/100 = 1.0
	if features[4] < 0 || features[4] > 1 {
		t.Errorf("x4 out of range: %f", features[4])
	}
	// x5: should be +1 (aligned: BULLISH + UP)
	if features[5] != 1.0 {
		t.Errorf("x5 should be 1.0 for aligned bullish+UP, got %f", features[5])
	}
	// x6: GOLDILOCKS = 1.0
	if features[6] != 1.0 {
		t.Errorf("x6 should be 1.0 for GOLDILOCKS, got %f", features[6])
	}
}

// --- Factor decomposition: z-score properties ---
func TestFactorDecomp_Deep_ZScoreProperties(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50}
	zScored := zScoreNormalize(data)

	// Mean of z-scored data should be ~0
	sum := 0.0
	for _, v := range zScored {
		sum += v
	}
	meanZ := sum / float64(len(zScored))
	if math.Abs(meanZ) > 0.001 {
		t.Errorf("Z-scored mean should be ~0, got %f", meanZ)
	}

	// Std dev of z-scored data should be ~1
	// stdDev now uses sample stddev (divides by N-1), so variance = ss/N = (N-1)/N
	// For N=5: variance = 4/5 = 0.8 when computed as population variance of z-scored data.
	// The sample variance (ss/(N-1)) should be ~1.0.
	ss := 0.0
	for _, v := range zScored {
		ss += v * v
	}
	sampleVariance := ss / float64(len(zScored)-1)
	if math.Abs(sampleVariance-1.0) > 0.01 {
		t.Errorf("Z-scored sample variance should be ~1.0, got %f", sampleVariance)
	}
}

// --- Factor decomposition: OLS with known solution ---
func TestFactorDecomp_Deep_OLSKnownSolution(t *testing.T) {
	// y = 2*x1 + 3*x2 + 5 (intercept)
	// Use independent x1 and x2 to avoid collinearity with intercept column.
	n := 20
	X := make([][]float64, n)
	y := make([]float64, n)

	for i := 0; i < n; i++ {
		x1 := float64(i) / float64(n)
		x2 := float64(i*i) / float64(n*n) // quadratic, independent of x1 and intercept
		X[i] = []float64{1.0, x1, x2}     // intercept + 2 features
		y[i] = 5.0 + 2.0*x1 + 3.0*x2
	}

	betas, r2, _ := simpleOLS(X, y)

	if len(betas) != 3 {
		t.Fatalf("Expected 3 betas, got %d", len(betas))
	}

	// R2 should be 1.0 for perfect linear relationship
	if math.Abs(r2-1.0) > 0.01 {
		t.Errorf("R2 should be ~1.0, got %f", r2)
	}

	// Coefficients: intercept~5, b1~2, b2~3
	if math.Abs(betas[0]-5.0) > 0.1 {
		t.Errorf("Intercept: expected ~5.0, got %f", betas[0])
	}
	if math.Abs(betas[1]-2.0) > 0.1 {
		t.Errorf("b1: expected ~2.0, got %f", betas[1])
	}
	if math.Abs(betas[2]-3.0) > 0.1 {
		t.Errorf("b2: expected ~3.0, got %f", betas[2])
	}
}

// --- AUC: Known rankings ---
func TestAUC_Deep_PerfectClassifier(t *testing.T) {
	// Perfect classifier: all positives score higher than all negatives
	predictions := []float64{0.9, 0.8, 0.2, 0.1}
	targets := []float64{1.0, 1.0, 0.0, 0.0}
	auc := approximateAUC(predictions, targets)
	if math.Abs(auc-1.0) > 0.001 {
		t.Errorf("Perfect classifier AUC should be 1.0, got %f", auc)
	}
}

func TestAUC_Deep_RandomClassifier(t *testing.T) {
	// Same predictions for all -> tied -> AUC = 0.5
	predictions := []float64{0.5, 0.5, 0.5, 0.5}
	targets := []float64{1.0, 1.0, 0.0, 0.0}
	auc := approximateAUC(predictions, targets)
	if math.Abs(auc-0.5) > 0.001 {
		t.Errorf("Tied classifier AUC should be 0.5, got %f", auc)
	}
}

func TestAUC_Deep_InverseClassifier(t *testing.T) {
	// Worst classifier: positives score lower
	predictions := []float64{0.1, 0.2, 0.8, 0.9}
	targets := []float64{1.0, 1.0, 0.0, 0.0}
	auc := approximateAUC(predictions, targets)
	if auc > 0.01 {
		t.Errorf("Inverse classifier AUC should be ~0.0, got %f", auc)
	}
}

// --- Matrix inversion: known inverse ---
func TestMatrixInversion_Deep_Known(t *testing.T) {
	// 2x2 matrix: [[4, 7], [2, 6]]
	// Inverse: 1/10 * [[6, -7], [-2, 4]]
	A := [][]float64{{4, 7}, {2, 6}}
	inv, err := invertMatrix(A)
	if err != nil {
		t.Fatalf("Matrix inversion failed: %v", err)
	}

	expected := [][]float64{{0.6, -0.7}, {-0.2, 0.4}}
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			if math.Abs(inv[i][j]-expected[i][j]) > 0.001 {
				t.Errorf("inv[%d][%d] = %f, expected %f", i, j, inv[i][j], expected[i][j])
			}
		}
	}
}

// --- Matrix inversion: singular matrix ---
func TestMatrixInversion_Deep_Singular(t *testing.T) {
	A := [][]float64{{1, 2}, {2, 4}} // Row 2 = 2 * Row 1
	_, err := invertMatrix(A)
	if err == nil {
		t.Error("Expected error for singular matrix")
	}
}

// --- Walk-forward: date filtering correctness ---
func TestWFO_Deep_DateFiltering(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	signals := []domain.PersistedSignal{
		{ReportDate: base},
		{ReportDate: base.Add(24 * time.Hour)},
		{ReportDate: base.Add(48 * time.Hour)},
		{ReportDate: base.Add(72 * time.Hour)},
	}

	// Filter [day0, day2) should include day0 and day1
	filtered := filterByDateRange(signals, base, base.Add(48*time.Hour))
	if len(filtered) != 2 {
		t.Errorf("Expected 2 signals in [day0, day2), got %d", len(filtered))
	}

	// Filter [day2, day4) should include day2 and day3
	filtered2 := filterByDateRange(signals, base.Add(48*time.Hour), base.Add(96*time.Hour))
	if len(filtered2) != 2 {
		t.Errorf("Expected 2 signals in [day2, day4), got %d", len(filtered2))
	}
}

// --- Weight normalization: coefficients sum to 100 ---
func TestWeights_Deep_NormalizationSum(t *testing.T) {
	// normalizeCoefficients returns map[string]float64 keyed by factor names.
	// It uses absolute values and normalizes to sum to 100.
	coefficients := []float64{0.5, -0.3, 0.8, 0.1}
	weights := normalizeCoefficients(coefficients)

	total := 0.0
	for _, w := range weights {
		total += w
	}

	// Should sum to ~100 (rounding may cause small deviation)
	if math.Abs(total-100.0) > 1.0 {
		t.Errorf("Weights should sum to ~100, got %f", total)
	}
}

// --- FRED regime encoding consistency ---
func TestFREDRegime_Deep_EncodingConsistency(t *testing.T) {
	// All regime labels should have a defined mapping (no silent zeros)
	knownRegimes := []string{"EXPANSION", "GOLDILOCKS", "STRESS", "RECESSION", "STAGFLATION", "TIGHTENING"}
	for _, regime := range knownRegimes {
		score := encodeFREDRegime(regime)
		// EXPANSION and GOLDILOCKS map to 1.0
		// STRESS, RECESSION, STAGFLATION map to -1.0
		// TIGHTENING maps to -0.5
		if score == 0 {
			t.Errorf("Regime %s maps to 0 -- might be unintentional", regime)
		}
	}

	// Unknown regime should return 0
	if encodeFREDRegime("UNKNOWN") != 0.0 {
		t.Error("Unknown regime should return 0")
	}
}

// --- normalCDF: verify Abramowitz & Stegun implementation ---
func TestNormalCDF_Deep_KnownValues(t *testing.T) {
	// Known CDF values: phi(0) = 0.5, phi(1) ~ 0.8413, phi(-1) ~ 0.1587
	tests := []struct {
		x    float64
		want float64
		tol  float64
	}{
		{0.0, 0.5, 0.001},
		{1.0, 0.8413, 0.001},
		{-1.0, 0.1587, 0.001},
		{2.0, 0.9772, 0.001},
		{-2.0, 0.0228, 0.001},
		{3.0, 0.9987, 0.001},
	}

	for _, tt := range tests {
		got := normalCDF(tt.x)
		if math.Abs(got-tt.want) > tt.tol {
			t.Errorf("normalCDF(%f) = %f, want %f (+/-%f)", tt.x, got, tt.want, tt.tol)
		}
	}
}

// --- Sigmoid: matches logistic function definition ---
func TestSigmoid_Deep_Definition(t *testing.T) {
	for _, z := range []float64{-10, -5, -1, 0, 1, 5, 10} {
		got := sigmoid(z)
		want := 1.0 / (1.0 + math.Exp(-z))
		if math.Abs(got-want) > 1e-10 {
			t.Errorf("sigmoid(%f) = %f, want %f", z, got, want)
		}
	}
}

// --- Adjusted R2: formula verification ---
func TestAdjR2_Deep_Formula(t *testing.T) {
	// For R2=0.8, n=100, p=4: adjR2 = 1 - (1-0.8)*99/95 = 1 - 0.2*1.0421 = 0.7916
	adj := adjustedRSquared(0.8, 100, 4)
	expected := 1 - (1-0.8)*99.0/95.0
	if math.Abs(adj-expected) > 0.001 {
		t.Errorf("adjustedR2: expected %f, got %f", expected, adj)
	}

	// Edge case: n <= p+1 should return 0
	adj2 := adjustedRSquared(0.8, 5, 4)
	if adj2 != 0 {
		t.Errorf("adjustedR2 with n<=p+1 should be 0, got %f", adj2)
	}
}
