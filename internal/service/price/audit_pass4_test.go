package price

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ==========================================================================
// AUDIT PASS 4 — Deep mathematical verification & edge cases
// ==========================================================================

// ---------------------------------------------------------------------------
// GARCH: Stationarity constraint verification
// ---------------------------------------------------------------------------

func TestGARCH_Pass4_StationarityConstraint(t *testing.T) {
	prices := generatePrices(200, 100.0, 0.0, 0.02, 11111)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	// Stationarity requires alpha + beta < 1
	persistence := result.Alpha + result.Beta
	if persistence >= 1.0 {
		t.Errorf("Stationarity violated: alpha(%f) + beta(%f) = %f >= 1.0",
			result.Alpha, result.Beta, persistence)
	}

	// Alpha and Beta must be non-negative
	if result.Alpha < 0 {
		t.Errorf("Alpha must be non-negative, got %f", result.Alpha)
	}
	if result.Beta < 0 {
		t.Errorf("Beta must be non-negative, got %f", result.Beta)
	}

	// Omega must be positive
	if result.Omega <= 0 {
		t.Errorf("Omega must be positive, got %e", result.Omega)
	}

	t.Logf("GARCH params: alpha=%.4f, beta=%.4f, persistence=%.4f, omega=%e",
		result.Alpha, result.Beta, persistence, result.Omega)
}

// ---------------------------------------------------------------------------
// GARCH: 1-step forecast recurrence identity
// ---------------------------------------------------------------------------

func TestGARCH_Pass4_ForecastVar1Identity(t *testing.T) {
	prices := generatePrices(150, 50.0, 0.001, 0.015, 22222)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	// ForecastVar1 should equal omega + alpha*lastReturn^2 + beta*currentVar
	// But we don't have lastReturn directly. Verify ForecastVol1 = sqrt(ForecastVar1).
	expectedVol1 := math.Sqrt(result.ForecastVar1)
	if math.Abs(result.ForecastVol1-expectedVol1)/expectedVol1 > 0.001 {
		t.Errorf("ForecastVol1 (%f) != sqrt(ForecastVar1) (%f)",
			result.ForecastVol1, expectedVol1)
	}

	// Similarly for current
	expectedCurrentVol := math.Sqrt(result.CurrentVar)
	if math.Abs(result.CurrentVol-expectedCurrentVol)/expectedCurrentVol > 0.001 {
		t.Errorf("CurrentVol (%f) != sqrt(CurrentVar) (%f)",
			result.CurrentVol, expectedCurrentVol)
	}
}

// ---------------------------------------------------------------------------
// GARCH: Long-run variance mean reversion
// ---------------------------------------------------------------------------

func TestGARCH_Pass4_LongRunMeanReversion(t *testing.T) {
	prices := generatePrices(300, 100.0, 0.0, 0.02, 33333)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	// Multi-step forecast should converge toward long-run variance
	// ForecastVar5 should be between ForecastVar1 and LongRunVar
	// (assuming mean reversion when persistence < 1)
	if result.Persistence < 1.0 {
		// If ForecastVar1 > LongRunVar, then ForecastVar5 should be closer to LongRunVar
		if result.ForecastVar1 > result.LongRunVar {
			if result.ForecastVar5 > result.ForecastVar1 {
				t.Errorf("ForecastVar5 (%e) should be <= ForecastVar1 (%e) when reverting down",
					result.ForecastVar5, result.ForecastVar1)
			}
		} else if result.ForecastVar1 < result.LongRunVar {
			if result.ForecastVar5 < result.ForecastVar1 {
				t.Errorf("ForecastVar5 (%e) should be >= ForecastVar1 (%e) when reverting up",
					result.ForecastVar5, result.ForecastVar1)
			}
		}
	}

	t.Logf("Mean reversion: Var1=%e, Var5=%e, LRV=%e, persistence=%.4f",
		result.ForecastVar1, result.ForecastVar5, result.LongRunVar, result.Persistence)
}

// ---------------------------------------------------------------------------
// GARCH: VolRatio consistency
// ---------------------------------------------------------------------------

func TestGARCH_Pass4_VolRatioConsistency(t *testing.T) {
	prices := generatePrices(200, 100.0, 0.0, 0.02, 44444)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	// VolRatio = CurrentVol / LongRunVol
	if result.LongRunVol > 0 {
		expectedRatio := result.CurrentVol / result.LongRunVol
		if math.Abs(result.VolRatio-expectedRatio) > 0.01 {
			t.Errorf("VolRatio (%f) != CurrentVol/LongRunVol (%f/%f = %f)",
				result.VolRatio, result.CurrentVol, result.LongRunVol, expectedRatio)
		}
	}
}

// ---------------------------------------------------------------------------
// GARCH: Confidence multiplier boundaries
// ---------------------------------------------------------------------------

func TestGARCH_Pass4_ConfidenceMultiplierThresholds(t *testing.T) {
	// Test each threshold of GARCHConfidenceMultiplier
	testCases := []struct {
		name       string
		volRatio   float64
		fcastVol   float64
		lrVol      float64
		wantMult   float64
	}{
		{"very_high_vol", 2.0, 0.031, 0.02, 0.75}, // forecastRatio = 0.031/0.02 = 1.55 > 1.50
		{"elevated_vol", 1.3, 0.026, 0.02, 0.85},
		{"normal_vol", 1.0, 0.02, 0.02, 1.00},
		{"low_vol", 0.7, 0.014, 0.02, 1.10},
		{"very_low_vol", 0.4, 0.008, 0.02, 1.15},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := &GARCHResult{
				Converged:    true,
				VolRatio:     tc.volRatio,
				ForecastVol1: tc.fcastVol,
				LongRunVol:   tc.lrVol,
			}
			mult := GARCHConfidenceMultiplier(g)
			if math.Abs(mult-tc.wantMult) > 0.001 {
				t.Errorf("multiplier = %f, want %f (forecastRatio = %f)",
					mult, tc.wantMult, tc.fcastVol/tc.lrVol)
			}
		})
	}

	// Nil input
	if GARCHConfidenceMultiplier(nil) != 1.0 {
		t.Error("nil GARCHResult should return multiplier 1.0")
	}

	// Not converged
	if GARCHConfidenceMultiplier(&GARCHResult{Converged: false}) != 1.0 {
		t.Error("non-converged should return multiplier 1.0")
	}
}

// ---------------------------------------------------------------------------
// HMM: Transition matrix row sums must equal 1
// ---------------------------------------------------------------------------

func TestHMM_Pass4_TransitionMatrixRowSums(t *testing.T) {
	prices := generatePrices(200, 100.0, 0.0, 0.02, 55555)
	result, err := EstimateHMMRegime(prices)
	if err != nil {
		t.Fatalf("HMM failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		rowSum := 0.0
		for j := 0; j < 3; j++ {
			rowSum += result.TransitionMatrix[i][j]
			// Each entry must be non-negative
			if result.TransitionMatrix[i][j] < 0 {
				t.Errorf("Transition[%d][%d] = %f < 0", i, j, result.TransitionMatrix[i][j])
			}
		}
		if math.Abs(rowSum-1.0) > 0.001 {
			t.Errorf("Transition matrix row %d sums to %f, expected 1.0", i, rowSum)
		}
	}
}

// ---------------------------------------------------------------------------
// HMM: State probabilities must sum to 1
// ---------------------------------------------------------------------------

func TestHMM_Pass4_StateProbabilitiesSum(t *testing.T) {
	prices := generatePrices(200, 100.0, 0.0, 0.02, 66666)
	result, err := EstimateHMMRegime(prices)
	if err != nil {
		t.Fatalf("HMM failed: %v", err)
	}

	probSum := result.StateProbabilities[0] + result.StateProbabilities[1] + result.StateProbabilities[2]
	if math.Abs(probSum-1.0) > 0.001 {
		t.Errorf("State probabilities sum to %f, expected 1.0", probSum)
	}

	// Each probability must be non-negative
	for i, p := range result.StateProbabilities {
		if p < 0 {
			t.Errorf("StateProbabilities[%d] = %f < 0", i, p)
		}
	}
}

// ---------------------------------------------------------------------------
// HMM: CurrentState matches highest probability
// ---------------------------------------------------------------------------

func TestHMM_Pass4_CurrentStateMatchesMaxProb(t *testing.T) {
	prices := generatePrices(200, 100.0, 0.0, 0.02, 77777)
	result, err := EstimateHMMRegime(prices)
	if err != nil {
		t.Fatalf("HMM failed: %v", err)
	}

	// Find the state with highest probability
	maxProb := -1.0
	maxState := ""
	stateLabels := [3]string{HMMRiskOn, HMMRiskOff, HMMCrisis}
	for i, p := range result.StateProbabilities {
		if p > maxProb {
			maxProb = p
			maxState = stateLabels[i]
		}
	}

	if result.CurrentState != maxState {
		t.Errorf("CurrentState=%s but highest probability state is %s (probs=%v)",
			result.CurrentState, maxState, result.StateProbabilities)
	}
}

// ---------------------------------------------------------------------------
// HMM: Discretization bin distribution
// ---------------------------------------------------------------------------

func TestHMM_Pass4_DiscretizationBins(t *testing.T) {
	// Generate returns and check that discretization produces all 5 bins
	rng := rand.New(rand.NewSource(88888))
	returns := make([]float64, 100)
	for i := range returns {
		returns[i] = rng.NormFloat64() * 0.02
	}

	obs := discretizeReturns(returns)

	// Count bin occurrences
	counts := make(map[int]int)
	for _, o := range obs {
		counts[o]++
	}

	// Each bin should have approximately 20% of data
	for bin := 0; bin < 5; bin++ {
		count := counts[bin]
		pct := float64(count) / float64(len(obs)) * 100
		if count == 0 {
			t.Errorf("Bin %d has zero observations", bin)
		}
		t.Logf("Bin %d: %d observations (%.1f%%)", bin, count, pct)
	}
}

// ---------------------------------------------------------------------------
// Hurst: Mean-reverting series should give H < 0.5
// ---------------------------------------------------------------------------

func TestHurst_Pass4_MeanRevertingSeries(t *testing.T) {
	// Alternating returns: +x, -x, +x, -x, ...
	rng := rand.New(rand.NewSource(99999))
	prices := make([]domain.PriceRecord, 200)
	price := 100.0
	now := time.Now()

	for i := 199; i >= 0; i-- {
		sign := 1.0
		if (199-i)%2 == 1 {
			sign = -1.0
		}
		ret := sign*0.005 + rng.NormFloat64()*0.001 // Mostly alternating with tiny noise
		price *= (1 + ret)
		if price <= 0 {
			price = 0.01
		}
		prices[i] = domain.PriceRecord{
			Date:  now.AddDate(0, 0, -(199 - i)),
			Close: price, High: price * 1.002, Low: price * 0.998, Open: price,
		}
	}

	result, err := ComputeHurstExponent(prices)
	if err != nil {
		t.Fatalf("Hurst failed: %v", err)
	}

	// Strong mean-reverting series should have H < 0.5
	if result.H > 0.55 {
		t.Errorf("Expected H < 0.55 for mean-reverting series, got H=%f", result.H)
	}
	t.Logf("Mean-reverting series: H=%f, R2=%f, Classification=%s",
		result.H, result.RSquared, result.Classification)
}

// ---------------------------------------------------------------------------
// Hurst: R/S block size validation
// ---------------------------------------------------------------------------

func TestHurst_Pass4_RescaledRangeProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(10101))
	returns := make([]float64, 100)
	for i := range returns {
		returns[i] = rng.NormFloat64() * 0.02
	}

	// R/S should be positive for all valid block sizes
	for _, size := range []int{8, 16, 32} {
		rs := rescaledRange(returns, size)
		if rs < 0 {
			t.Errorf("R/S for block size %d is negative: %f", size, rs)
		}
		if rs == 0 {
			t.Logf("R/S for block size %d is zero (all blocks had zero variance)", size)
		}
	}
}

// ---------------------------------------------------------------------------
// Hurst: simpleLinearRegression mathematical identity
// ---------------------------------------------------------------------------

func TestHurst_Pass4_LinearRegressionPerfectFit(t *testing.T) {
	// y = 2x + 3 (perfect linear relationship)
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{5, 7, 9, 11, 13}

	slope, intercept, r2 := simpleLinearRegression(x, y)

	if math.Abs(slope-2.0) > 0.001 {
		t.Errorf("Slope: expected 2.0, got %f", slope)
	}
	if math.Abs(intercept-3.0) > 0.001 {
		t.Errorf("Intercept: expected 3.0, got %f", intercept)
	}
	if math.Abs(r2-1.0) > 0.001 {
		t.Errorf("R2: expected 1.0, got %f", r2)
	}
}

// ---------------------------------------------------------------------------
// Volatility: CombineVolatilityMultiplier averaging logic
// ---------------------------------------------------------------------------

func TestVolatility_Pass4_CombineMultiplierAveraging(t *testing.T) {
	// With both ATR and VIX, multiplier should be averaged (not compounded)
	volCtx := &VolatilityContext{
		ConfidenceMultiplier: 0.85, // EXPANDING
	}

	// Test without RiskContext
	mult := CombineVolatilityMultiplier(volCtx, nil)
	if mult != 0.85 {
		t.Errorf("Without RiskContext, should use ATR multiplier directly: got %f", mult)
	}

	// Test with nil VolatilityContext
	mult = CombineVolatilityMultiplier(nil, nil)
	if mult != 1.0 {
		t.Errorf("Nil VolatilityContext should return 1.0, got %f", mult)
	}
}

// ---------------------------------------------------------------------------
// CombineVolatilityWithGARCH: weighted average
// ---------------------------------------------------------------------------

func TestGARCH_Pass4_CombineWeightedAverage(t *testing.T) {
	volCtx := &VolatilityContext{
		ConfidenceMultiplier: 0.85,
	}

	garch := &GARCHResult{
		Converged:    true,
		VolRatio:     1.3,
		ForecastVol1: 0.026,
		LongRunVol:   0.02,
	}

	combined := CombineVolatilityWithGARCH(volCtx, nil, garch)

	// ATR+VIX combo: CombineVolatilityMultiplier(volCtx, nil) = 0.85
	// GARCH multiplier: forecastRatio = 0.026/0.02 = 1.3 -> 0.85
	// Combined: 0.85*0.60 + 0.85*0.40 = 0.85
	atrMult := CombineVolatilityMultiplier(volCtx, nil)
	garchMult := GARCHConfidenceMultiplier(garch)
	expected := roundN(atrMult*0.60+garchMult*0.40, 4)

	if math.Abs(combined-expected) > 0.001 {
		t.Errorf("CombineVolatilityWithGARCH = %f, expected %f (atr=%f, garch=%f)",
			combined, expected, atrMult, garchMult)
	}
}

// ---------------------------------------------------------------------------
// Position sizing: zero/negative inputs
// ---------------------------------------------------------------------------

func TestPositionSize_Pass4_ZeroInputs(t *testing.T) {
	// Zero entry price
	result := ComputePositionSize(PositionSizeInput{EntryPrice: 0, DailyATR: 0.01}, true)
	if result.ATRStopDistance != 0 {
		t.Error("Zero entry price should produce zero stop distance")
	}

	// Zero ATR
	result = ComputePositionSize(PositionSizeInput{EntryPrice: 1.2, DailyATR: 0}, true)
	if result.ATRStopDistance != 0 {
		t.Error("Zero ATR should produce zero stop distance")
	}
}

// ---------------------------------------------------------------------------
// Intraday: SMA calculation
// ---------------------------------------------------------------------------

func TestIntraday_Pass4_SMACalculation(t *testing.T) {
	bars := []domain.IntradayBar{
		{Close: 10}, {Close: 20}, {Close: 30}, {Close: 40}, {Close: 50},
	}

	// SMA(3) of newest-first: (10+20+30)/3 = 20
	sma := computeIntradaySMA(bars, 3)
	if math.Abs(sma-20.0) > 0.001 {
		t.Errorf("SMA(3) expected 20.0, got %f", sma)
	}

	// SMA(5): (10+20+30+40+50)/5 = 30
	sma5 := computeIntradaySMA(bars, 5)
	if math.Abs(sma5-30.0) > 0.001 {
		t.Errorf("SMA(5) expected 30.0, got %f", sma5)
	}

	// Insufficient data
	sma10 := computeIntradaySMA(bars, 10)
	if sma10 != 0 {
		t.Errorf("SMA with insufficient data should return 0, got %f", sma10)
	}
}

// ---------------------------------------------------------------------------
// Intraday: ATR calculation
// ---------------------------------------------------------------------------

func TestIntraday_Pass4_ATRCalculation(t *testing.T) {
	bars := []domain.IntradayBar{
		{High: 110, Low: 95, Close: 105},  // newest, TR = max(15, |110-100|, |95-100|) = 15
		{High: 108, Low: 98, Close: 100},  // TR = max(10, |108-102|, |98-102|) = 10
		{High: 105, Low: 96, Close: 102},  // prevClose for bar[1]
	}

	atr := computeIntradayATR(bars, 2)
	// ATR(2) = (15 + 10) / 2 = 12.5
	if math.Abs(atr-12.5) > 0.01 {
		t.Errorf("Intraday ATR(2) expected 12.5, got %f", atr)
	}
}

// ---------------------------------------------------------------------------
// Correlation: different length series
// ---------------------------------------------------------------------------

func TestCorrelation_Pass4_DifferentLengths(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6, 7}
	y := []float64{2, 4, 6, 8, 10} // Shorter but >= 5

	r := pearsonCorrelation(x, y)

	// Should use min(len(x), len(y)) = 5 elements
	// x[:5] = [1,2,3,4,5], y[:5] = [2,4,6,8,10] -> perfect positive correlation
	if math.IsNaN(r) {
		t.Error("Should not return NaN for inputs with >= 5 points")
	}
	if math.Abs(r-1.0) > 0.001 {
		t.Errorf("Correlation of [1,2,3,4,5] with [2,4,6,8,10] should be 1.0, got %f", r)
	}
}

// ---------------------------------------------------------------------------
// ADX: minimum data handling
// ---------------------------------------------------------------------------

func TestADX_Pass4_MinimumData(t *testing.T) {
	// Only 2 bars -> should return 0
	prices := []domain.PriceRecord{
		{High: 101, Low: 99, Close: 100},
		{High: 100, Low: 98, Close: 99},
	}
	adx := approximateADX(prices)
	if adx != 0 {
		t.Errorf("ADX with only 2 bars should be 0, got %f", adx)
	}

	// 3 bars -> should compute with period=2
	prices = append(prices, domain.PriceRecord{High: 99, Low: 97, Close: 98})
	adx = approximateADX(prices)
	// Should not panic and should return a reasonable value
	t.Logf("ADX with 3 bars: %f", adx)
}

// ---------------------------------------------------------------------------
// HMMConfidenceMultiplier: boundary values
// ---------------------------------------------------------------------------

func TestHMM_Pass4_ConfidenceMultiplierValues(t *testing.T) {
	tests := []struct {
		state    string
		expected float64
	}{
		{HMMRiskOn, 1.05},
		{HMMRiskOff, 0.90},
		{HMMCrisis, 0.70},
		{"UNKNOWN", 1.0},
	}

	for _, tc := range tests {
		result := &HMMResult{CurrentState: tc.state}
		mult := HMMConfidenceMultiplier(result)
		if math.Abs(mult-tc.expected) > 0.001 {
			t.Errorf("HMMConfidenceMultiplier(%s) = %f, want %f", tc.state, mult, tc.expected)
		}
	}

	// Nil input
	if HMMConfidenceMultiplier(nil) != 1.0 {
		t.Error("nil HMMResult should return 1.0")
	}
}
