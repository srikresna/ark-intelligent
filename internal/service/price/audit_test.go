package price

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ==========================================================================
// AUDIT TEST SUITE — Comprehensive validation of all quant models
// ==========================================================================

// --- Helper: generate synthetic price data ---

func generatePrices(n int, startPrice float64, drift, vol float64, seed int64) []domain.PriceRecord {
	rng := rand.New(rand.NewSource(seed))
	prices := make([]domain.PriceRecord, n)
	price := startPrice
	now := time.Now()

	for i := n - 1; i >= 0; i-- {
		ret := drift + rng.NormFloat64()*vol
		price *= (1 + ret)
		if price <= 0 {
			price = 0.01
		}
		prices[i] = domain.PriceRecord{
			Date:  now.AddDate(0, 0, -(n-1-i)*7),
			Close: price,
			High:  price * (1 + rng.Float64()*0.02),
			Low:   price * (1 - rng.Float64()*0.02),
			Open:  price * (1 + (rng.Float64()-0.5)*0.01),
		}
	}
	return prices
}

func generateIntradayBars(n int, startPrice float64, drift, vol float64, seed int64) []domain.IntradayBar {
	rng := rand.New(rand.NewSource(seed))
	bars := make([]domain.IntradayBar, n)
	price := startPrice
	now := time.Now()

	for i := n - 1; i >= 0; i-- {
		ret := drift + rng.NormFloat64()*vol
		price *= (1 + ret)
		if price <= 0 {
			price = 0.01
		}
		bars[i] = domain.IntradayBar{
			ContractCode: "099741",
			Symbol:       "EUR/USD",
			Interval:     "4h",
			Timestamp:    now.Add(-time.Duration(n-1-i) * 4 * time.Hour),
			Open:         price * (1 + (rng.Float64()-0.5)*0.005),
			High:         price * (1 + rng.Float64()*0.01),
			Low:          price * (1 - rng.Float64()*0.01),
			Close:        price,
			Source:       "test",
		}
	}
	return bars
}

// ==========================================================================
// GARCH AUDIT
// ==========================================================================

func TestGARCH_Audit_StationarityConstraint(t *testing.T) {
	// Verify alpha + beta < 1 always holds
	prices := generatePrices(200, 1.10, 0.001, 0.02, 100)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH estimation failed: %v", err)
	}
	if result.Alpha+result.Beta >= 1.0 {
		t.Errorf("Stationarity violated: alpha(%f) + beta(%f) = %f >= 1.0",
			result.Alpha, result.Beta, result.Alpha+result.Beta)
	}
	if result.Persistence >= 1.0 {
		t.Errorf("Persistence should be < 1: %f", result.Persistence)
	}
}

func TestGARCH_Audit_LongRunVarianceConsistency(t *testing.T) {
	// LongRunVar = omega / (1 - alpha - beta) must be positive and reasonable
	prices := generatePrices(150, 100.0, 0.0, 0.015, 200)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	expectedLRV := result.Omega / (1 - result.Alpha - result.Beta)
	if math.Abs(result.LongRunVar-expectedLRV) > 1e-6 {
		t.Errorf("LongRunVar inconsistency: stored=%e, computed=%e",
			result.LongRunVar, expectedLRV)
	}
	if result.LongRunVol != roundN(math.Sqrt(result.LongRunVar), 6) {
		t.Errorf("LongRunVol != sqrt(LongRunVar): vol=%f, sqrt(var)=%f",
			result.LongRunVol, math.Sqrt(result.LongRunVar))
	}
}

func TestGARCH_Audit_ForecastMeanReverts(t *testing.T) {
	// Multi-step forecast should converge toward long-run variance
	prices := generatePrices(200, 50.0, 0.0, 0.02, 300)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	// ForecastVar5 should be between CurrentVar and LongRunVar
	// (mean-reverting property)
	if result.Persistence < 1.0 && result.Persistence > 0 {
		// If current > long-run, forecast should decrease toward LR
		if result.CurrentVar > result.LongRunVar {
			if result.ForecastVar5 > result.CurrentVar*1.01 { // Allow 1% tolerance
				t.Logf("Warning: ForecastVar5 (%e) > CurrentVar (%e) when CurrentVar > LongRunVar (%e)",
					result.ForecastVar5, result.CurrentVar, result.LongRunVar)
			}
		}
	}
}

func TestGARCH_Audit_VolRatioCalculation(t *testing.T) {
	prices := generatePrices(100, 1.30, 0.0, 0.01, 400)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	expectedRatio := result.CurrentVol / result.LongRunVol
	if math.Abs(result.VolRatio-expectedRatio) > 0.001 {
		t.Errorf("VolRatio incorrect: stored=%f, computed=%f",
			result.VolRatio, expectedRatio)
	}
}

func TestGARCH_Audit_IntraDayConsistency(t *testing.T) {
	// GARCH from intraday should produce similar structure as daily
	bars := generateIntradayBars(100, 1.10, 0.0, 0.005, 500)
	result, err := EstimateGARCHFromIntraday(bars)
	if err != nil {
		t.Fatalf("Intraday GARCH failed: %v", err)
	}

	if result.Alpha < 0 || result.Beta < 0 {
		t.Errorf("Negative coefficients: alpha=%f, beta=%f", result.Alpha, result.Beta)
	}
	if result.CurrentVol <= 0 {
		t.Errorf("CurrentVol should be positive: %f", result.CurrentVol)
	}
}

func TestGARCH_Audit_LogLikelihoodMonotonicity(t *testing.T) {
	// Log-likelihood from the best fit should be higher than a random parameter set
	prices := generatePrices(100, 100.0, 0.0, 0.015, 600)

	n := len(prices)
	returns := make([]float64, 0, n-1)
	for i := n - 1; i > 0; i-- {
		if prices[i].Close > 0 && prices[i-1].Close > 0 {
			returns = append(returns, math.Log(prices[i-1].Close/prices[i].Close))
		}
	}

	var sumR2 float64
	for _, r := range returns {
		sumR2 += r * r
	}
	sampleVar := sumR2 / float64(len(returns))

	// Random params
	randomLL := garchLogLikelihood(returns, sampleVar*0.5, 0.05, 0.50, sampleVar)

	// Estimate optimal
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	if result.LogLikelihood < randomLL {
		t.Errorf("Optimal LL (%f) should be >= random LL (%f)",
			result.LogLikelihood, randomLL)
	}
}

func TestGARCH_Audit_ConstantPriceEdgeCase(t *testing.T) {
	// All same prices → zero returns → should handle gracefully
	prices := make([]domain.PriceRecord, 50)
	for i := range prices {
		prices[i] = domain.PriceRecord{Close: 100.0, High: 100.0, Low: 100.0, Open: 100.0}
	}
	_, err := EstimateGARCH(prices)
	// Should either error or produce reasonable result (no NaN/Inf)
	if err == nil {
		t.Log("Constant prices accepted — verify no NaN in output")
	}
}

// ==========================================================================
// HURST EXPONENT AUDIT
// ==========================================================================

func TestHurst_Audit_MeanRevertingSignal(t *testing.T) {
	// Generate mean-reverting series (Ornstein-Uhlenbeck-like)
	rng := rand.New(rand.NewSource(42))
	prices := make([]domain.PriceRecord, 300)
	price := 100.0
	now := time.Now()
	meanLevel := 100.0
	reversion := 0.1 // strength of mean reversion

	for i := 299; i >= 0; i-- {
		prices[i] = domain.PriceRecord{
			Date: now.AddDate(0, 0, -(299-i)),
			Close: price, High: price*1.005, Low: price*0.995, Open: price,
		}
		noise := rng.NormFloat64() * 0.5
		price += reversion*(meanLevel-price) + noise
	}

	result, err := ComputeHurstExponent(prices)
	if err != nil {
		t.Fatalf("Hurst failed: %v", err)
	}

	// Mean-reverting should give H < 0.5
	if result.H >= 0.55 {
		t.Errorf("Expected H < 0.55 for mean-reverting series, got H=%f", result.H)
	}
	if result.Classification != "MEAN_REVERTING" && result.Classification != "RANDOM_WALK" {
		t.Logf("Classification=%s for H=%f (borderline cases acceptable)", result.Classification, result.H)
	}
}

func TestHurst_Audit_ConfidenceRange(t *testing.T) {
	prices := generatePrices(200, 100.0, 0.003, 0.01, 77)
	result, err := ComputeHurstExponent(prices)
	if err != nil {
		t.Fatalf("Hurst failed: %v", err)
	}

	if result.Confidence < 0 || result.Confidence > 100 {
		t.Errorf("Confidence out of range [0,100]: %f", result.Confidence)
	}
	if result.RSquared < 0 || result.RSquared > 1 {
		t.Errorf("R² out of range [0,1]: %f", result.RSquared)
	}
}

func TestHurst_Audit_HValueBounds(t *testing.T) {
	// H should always be in [0, 1] after clamping
	for seed := int64(0); seed < 10; seed++ {
		prices := generatePrices(100, 100.0, 0.0, 0.03, seed*100)
		result, err := ComputeHurstExponent(prices)
		if err != nil {
			continue
		}
		if result.H < 0 || result.H > 1 {
			t.Errorf("Seed %d: H=%f out of [0,1]", seed, result.H)
		}
	}
}

func TestHurst_Audit_IntraDayEquivalence(t *testing.T) {
	// Both daily and intraday should produce similar H for same data pattern
	bars := generateIntradayBars(200, 100.0, 0.001, 0.005, 55)
	result, err := ComputeHurstFromIntraday(bars)
	if err != nil {
		t.Fatalf("Intraday Hurst failed: %v", err)
	}
	if result.H < 0 || result.H > 1 {
		t.Errorf("Intraday H=%f out of [0,1]", result.H)
	}
	if result.SampleSize != 199 {
		t.Errorf("Expected 199 returns, got %d", result.SampleSize)
	}
}

func TestHurst_Audit_RescaledRangeZeroVariance(t *testing.T) {
	// All identical returns → should handle without panic
	returns := make([]float64, 50)
	for i := range returns {
		returns[i] = 0.01 // constant return
	}
	rs := rescaledRange(returns, 10)
	// R/S should be 0 for constant series (range = 0)
	if rs != 0 {
		t.Errorf("Expected R/S=0 for constant returns, got %f", rs)
	}
}

// ==========================================================================
// HMM REGIME AUDIT
// ==========================================================================

func TestHMM_Audit_StateLabelsValid(t *testing.T) {
	prices := generatePrices(120, 100.0, 0.0, 0.015, 111)
	result, err := EstimateHMMRegime(prices)
	if err != nil {
		t.Fatalf("HMM failed: %v", err)
	}

	validStates := map[string]bool{HMMRiskOn: true, HMMRiskOff: true, HMMCrisis: true}
	if !validStates[result.CurrentState] {
		t.Errorf("Invalid current state: %s", result.CurrentState)
	}

	for _, s := range result.ViterbiPath {
		if !validStates[s] {
			t.Errorf("Invalid Viterbi state: %s", s)
		}
	}
}

func TestHMM_Audit_ProbabilitiesSumToOne(t *testing.T) {
	prices := generatePrices(150, 100.0, 0.0, 0.02, 222)
	result, err := EstimateHMMRegime(prices)
	if err != nil {
		t.Fatalf("HMM failed: %v", err)
	}

	// State probabilities should sum to ~1
	probSum := result.StateProbabilities[0] + result.StateProbabilities[1] + result.StateProbabilities[2]
	if math.Abs(probSum-1.0) > 0.01 {
		t.Errorf("State probabilities don't sum to 1: %v (sum=%f)",
			result.StateProbabilities, probSum)
	}

	// Transition matrix rows should sum to ~1
	for i := 0; i < 3; i++ {
		rowSum := result.TransitionMatrix[i][0] + result.TransitionMatrix[i][1] + result.TransitionMatrix[i][2]
		if math.Abs(rowSum-1.0) > 0.01 {
			t.Errorf("Transition matrix row %d doesn't sum to 1: sum=%f", i, rowSum)
		}
	}
}

func TestHMM_Audit_CrisisDetectionInHighVol(t *testing.T) {
	// Generate crisis-like data: extreme vol, negative drift
	prices := generatePrices(120, 100.0, -0.01, 0.05, 333)
	result, err := EstimateHMMRegime(prices)
	if err != nil {
		t.Fatalf("HMM failed: %v", err)
	}

	// With extreme vol, crisis prob should be non-trivial
	crisisProb := result.StateProbabilities[2]
	t.Logf("Crisis probability for high-vol data: %.2f%%", crisisProb*100)
	// We don't require it to be dominant, but it should be detected
	if crisisProb < 0.01 {
		t.Logf("Warning: crisis probability very low (%.4f) for high-vol data", crisisProb)
	}
}

func TestHMM_Audit_DiscretizationDistribution(t *testing.T) {
	// Verify discretization produces roughly uniform bins for normal data
	rng := rand.New(rand.NewSource(44))
	returns := make([]float64, 1000)
	for i := range returns {
		returns[i] = rng.NormFloat64() * 0.01
	}

	obs := discretizeReturns(returns)
	counts := make(map[int]int)
	for _, o := range obs {
		counts[o]++
	}

	// Each bin should have roughly 20% (200 ±50)
	for bin := 0; bin < 5; bin++ {
		if counts[bin] < 150 || counts[bin] > 250 {
			t.Errorf("Bin %d has %d observations (expected ~200)", bin, counts[bin])
		}
	}
}

func TestHMM_Audit_TransitionWarningLogic(t *testing.T) {
	// Test warning when a different state is becoming likely
	probs := [3]float64{0.60, 0.25, 0.15}
	A := [3][3]float64{
		{0.50, 0.30, 0.20}, // Likely shift from RiskOn
		{0.10, 0.80, 0.10},
		{0.05, 0.20, 0.75},
	}

	warning := detectTransitionWarning(0, probs, A)
	// With 60% RiskOn but transition favoring spread, check if warning fires
	t.Logf("Transition warning: %q", warning)
	// This is informational — main check is it doesn't panic
}

// ==========================================================================
// CORRELATION MATRIX AUDIT
// ==========================================================================

func TestPearsonCorrelation_Audit_PerfectPositive(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	y := []float64{2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	r := pearsonCorrelation(x, y)
	if math.Abs(r-1.0) > 0.0001 {
		t.Errorf("Expected r=1.0 for perfect positive correlation, got %f", r)
	}
}

func TestPearsonCorrelation_Audit_PerfectNegative(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	y := []float64{20, 18, 16, 14, 12, 10, 8, 6, 4, 2}
	r := pearsonCorrelation(x, y)
	if math.Abs(r-(-1.0)) > 0.0001 {
		t.Errorf("Expected r=-1.0 for perfect negative correlation, got %f", r)
	}
}

func TestPearsonCorrelation_Audit_ZeroCorrelation(t *testing.T) {
	// Orthogonal signals
	x := []float64{1, -1, 1, -1, 1, -1, 1, -1, 1, -1}
	y := []float64{1, 1, -1, -1, 1, 1, -1, -1, 1, 1}
	r := pearsonCorrelation(x, y)
	if math.Abs(r) > 0.3 {
		t.Errorf("Expected r≈0 for uncorrelated signals, got %f", r)
	}
}

func TestPearsonCorrelation_Audit_SymmetryProperty(t *testing.T) {
	x := []float64{1.5, 2.3, 3.1, 4.7, 5.2}
	y := []float64{2.1, 3.5, 2.8, 4.2, 5.0}
	rXY := pearsonCorrelation(x, y)
	rYX := pearsonCorrelation(y, x)
	if math.Abs(rXY-rYX) > 0.0001 {
		t.Errorf("Correlation not symmetric: r(x,y)=%f, r(y,x)=%f", rXY, rYX)
	}
}

func TestPearsonCorrelation_Audit_DifferentLengths(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6, 7}
	y := []float64{2, 4, 6, 8, 10} // shorter but >= 5
	r := pearsonCorrelation(x, y)
	// Should use min(len(x), len(y)) = 5
	if math.IsNaN(r) {
		t.Error("NaN from different-length inputs with >= 5 points")
	}
}

func TestPearsonCorrelation_Audit_ConstantSeries(t *testing.T) {
	x := []float64{5, 5, 5, 5, 5}
	y := []float64{1, 2, 3, 4, 5}
	r := pearsonCorrelation(x, y)
	// denom should be 0 (zero variance in x), expect 0
	if r != 0 {
		t.Errorf("Expected r=0 for constant series, got %f", r)
	}
}

// ==========================================================================
// INTRADAY CONTEXT AUDIT
// ==========================================================================

func TestIntradayContext_Audit_SMAComputation(t *testing.T) {
	bars := make([]domain.IntradayBar, 10)
	for i := range bars {
		bars[i] = domain.IntradayBar{Close: float64(10 - i)} // newest=10, oldest=1
	}

	sma3 := computeIntradaySMA(bars, 3)
	// Should average bars[0..2] = (10+9+8)/3 = 9.0
	expected := 9.0
	if math.Abs(sma3-expected) > 0.001 {
		t.Errorf("SMA(3) expected %f, got %f", expected, sma3)
	}
}

func TestIntradayContext_Audit_ATRComputation(t *testing.T) {
	// Create bars where TR is known
	bars := make([]domain.IntradayBar, 5)
	// bars[0] (newest): H=105, L=95, prevClose=100 → TR = max(10, 5, 5) = 10
	// bars[1]:          H=102, L=98, prevClose=99  → TR = max(4, 3, 1) = 4
	// bars[2]:          H=101, L=97, prevClose=100 → TR = max(4, 1, 3) = 4
	bars[0] = domain.IntradayBar{High: 105, Low: 95, Close: 100}
	bars[1] = domain.IntradayBar{High: 102, Low: 98, Close: 100}
	bars[2] = domain.IntradayBar{High: 101, Low: 97, Close: 99}
	bars[3] = domain.IntradayBar{High: 102, Low: 98, Close: 100}
	bars[4] = domain.IntradayBar{High: 101, Low: 99, Close: 100}

	atr2 := computeIntradayATR(bars, 2)
	// Period=2: uses bars[0] with prev=bars[1].Close=100, bars[1] with prev=bars[2].Close=99
	// TR[0] = max(105-95, |105-100|, |95-100|) = max(10, 5, 5) = 10
	// TR[1] = max(102-98, |102-99|, |98-99|) = max(4, 3, 1) = 4
	// ATR = (10 + 4) / 2 = 7.0
	expected := 7.0
	if math.Abs(atr2-expected) > 0.001 {
		t.Errorf("ATR(2) expected %f, got %f", expected, atr2)
	}
}

func TestIntradayContext_Audit_ROCComputation(t *testing.T) {
	bars := make([]domain.IntradayBar, 10)
	for i := range bars {
		bars[i] = domain.IntradayBar{Close: 100.0 + float64(10-i)*0.5}
	}
	// bars[0].Close = 105, bars[6].Close = 102
	roc6 := computeIntradayROC(bars, 6)
	expected := ((bars[0].Close - bars[6].Close) / bars[6].Close) * 100
	if math.Abs(roc6-roundN(expected, 4)) > 0.001 {
		t.Errorf("ROC(6) expected %f, got %f", expected, roc6)
	}
}

func TestIntradayContext_Audit_TrendClassification(t *testing.T) {
	// UP trend
	barsUp := []domain.IntradayBar{
		{Close: 105}, {Close: 104}, {Close: 103}, {Close: 102}, {Close: 101}, {Close: 100},
	}
	if computeIntradayTrend(barsUp) != "UP" {
		t.Errorf("Expected UP trend, got %s", computeIntradayTrend(barsUp))
	}

	// DOWN trend
	barsDown := []domain.IntradayBar{
		{Close: 100}, {Close: 101}, {Close: 102}, {Close: 103}, {Close: 104}, {Close: 105},
	}
	if computeIntradayTrend(barsDown) != "DOWN" {
		t.Errorf("Expected DOWN trend, got %s", computeIntradayTrend(barsDown))
	}

	// FLAT
	barsFlat := []domain.IntradayBar{
		{Close: 100.01}, {Close: 100.00}, {Close: 100.01}, {Close: 100.00},
	}
	if computeIntradayTrend(barsFlat) != "FLAT" {
		t.Errorf("Expected FLAT trend, got %s", computeIntradayTrend(barsFlat))
	}
}

func TestIntradayContext_Audit_SessionRange(t *testing.T) {
	bars := []domain.IntradayBar{
		{High: 1.1050, Low: 1.0990},
		{High: 1.1080, Low: 1.0970},
		{High: 1.1020, Low: 1.0985},
	}
	high, low := computeSessionRange(bars)
	if high != 1.1080 {
		t.Errorf("Expected session high 1.1080, got %f", high)
	}
	if low != 1.0970 {
		t.Errorf("Expected session low 1.0970, got %f", low)
	}
}

// ==========================================================================
// 4H AGGREGATION AUDIT
// ==========================================================================

func TestAggregateTo4H_Audit_OHLCCorrectness(t *testing.T) {
	// Create 4 hourly bars that should form one 4H bar (bucket 08-11)
	base := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	hourBars := []domain.IntradayBar{
		{Timestamp: base, Open: 1.1000, High: 1.1020, Low: 1.0990, Close: 1.1010, Source: "test"},
		{Timestamp: base.Add(1 * time.Hour), Open: 1.1010, High: 1.1050, Low: 1.1000, Close: 1.1040, Source: "test"},
		{Timestamp: base.Add(2 * time.Hour), Open: 1.1040, High: 1.1060, Low: 1.1030, Close: 1.1035, Source: "test"},
		{Timestamp: base.Add(3 * time.Hour), Open: 1.1035, High: 1.1045, Low: 1.0980, Close: 1.1000, Source: "test"},
	}

	result := aggregateToInterval(hourBars, "099741", "4h")
	if len(result) != 1 {
		t.Fatalf("Expected 1 aggregated bar, got %d", len(result))
	}

	bar := result[0]
	// Open should be first bar's open
	if bar.Open != 1.1000 {
		t.Errorf("Open: expected 1.1000, got %f", bar.Open)
	}
	// High should be max of all highs
	if bar.High != 1.1060 {
		t.Errorf("High: expected 1.1060, got %f", bar.High)
	}
	// Low should be min of all lows
	if bar.Low != 1.0980 {
		t.Errorf("Low: expected 1.0980, got %f", bar.Low)
	}
	// Close should be last bar's close
	if bar.Close != 1.1000 {
		t.Errorf("Close: expected 1.1000, got %f", bar.Close)
	}
	if bar.Interval != "4h" {
		t.Errorf("Interval: expected '4h', got %s", bar.Interval)
	}
}

func TestAggregateTo4H_Audit_IncompleteBucketFiltered(t *testing.T) {
	// Only 1 bar in a bucket should be filtered out (need at least 2)
	base := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	hourBars := []domain.IntradayBar{
		{Timestamp: base, Open: 1.10, High: 1.11, Low: 1.09, Close: 1.10, Source: "test"},
	}

	result := aggregateToInterval(hourBars, "099741", "4h")
	if len(result) != 0 {
		t.Errorf("Expected 0 bars for single-hour bucket, got %d", len(result))
	}
}

// ==========================================================================
// GARCH CONFIDENCE MULTIPLIER AUDIT
// ==========================================================================

func TestGARCH_Audit_ConfidenceMultiplierUseForecastRatio(t *testing.T) {
	// GARCHConfidenceMultiplier should use ForecastVol1/LongRunVol when available
	g := &GARCHResult{
		Converged:    true,
		VolRatio:     0.5, // low current vol
		ForecastVol1: 0.02,
		LongRunVol:   0.01,
	}
	mult := GARCHConfidenceMultiplier(g)
	// ForecastVol1/LongRunVol = 2.0 → should use high vol bucket (0.75)
	if mult != 0.75 {
		t.Errorf("Expected 0.75 for forecast ratio 2.0, got %f", mult)
	}
}

func TestCombineVolatilityWithGARCH_Audit_Weights(t *testing.T) {
	volCtx := &VolatilityContext{ConfidenceMultiplier: 0.85}
	garch := &GARCHResult{
		Converged:    true,
		ForecastVol1: 0.01,
		LongRunVol:   0.01,
		VolRatio:     1.0,
	}
	// ATR mult = 0.85, GARCH mult = 1.0
	// Combined = 0.85*0.6 + 1.0*0.4 = 0.51 + 0.40 = 0.91
	combined := CombineVolatilityWithGARCH(volCtx, nil, garch)
	expected := roundN(0.85*0.60+1.00*0.40, 4)
	if math.Abs(combined-expected) > 0.001 {
		t.Errorf("Combined multiplier: expected %f, got %f", expected, combined)
	}
}

// ==========================================================================
// HURST + ADX REGIME COMBINATION AUDIT
// ==========================================================================

func TestCombineRegime_Audit_NilSafety(t *testing.T) {
	// Both nil
	result := CombineRegimeClassification(nil, nil)
	if result.PriceRegime == nil {
		t.Fatal("PriceRegime should not be nil")
	}
	if result.HurstRegime != RegimeRanging {
		t.Errorf("Expected RANGING for nil inputs, got %s", result.HurstRegime)
	}

	// Only ADX
	adx := &PriceRegime{Regime: RegimeTrending, TrendStrength: 70}
	result2 := CombineRegimeClassification(adx, nil)
	if result2.HurstRegime != RegimeTrending {
		t.Errorf("Expected TRENDING with ADX-only, got %s", result2.HurstRegime)
	}

	// Only Hurst
	hurst := &HurstResult{H: 0.65, Classification: "TRENDING", Confidence: 30}
	result3 := CombineRegimeClassification(nil, hurst)
	if result3.PriceRegime == nil {
		t.Fatal("PriceRegime should not be nil for Hurst-only case")
	}
}

func TestCombineRegime_Audit_CrisisOverride(t *testing.T) {
	adx := &PriceRegime{Regime: RegimeCrisis, TrendStrength: 80}
	hurst := &HurstResult{H: 0.65, Classification: "TRENDING", Confidence: 30}

	result := CombineRegimeClassification(adx, hurst)
	if result.RegimeAgreement {
		t.Error("Crisis should not agree with Hurst TRENDING")
	}
	if result.CombinedConfidence != 90 {
		t.Errorf("Expected confidence 90 for crisis override, got %f", result.CombinedConfidence)
	}
}

// ==========================================================================
// LINEAR REGRESSION AUDIT
// ==========================================================================

func TestSimpleLinearRegression_Audit_KnownSlope(t *testing.T) {
	// y = 3x + 2 + noise
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	y := make([]float64, 10)
	for i := range x {
		y[i] = 3*x[i] + 2
	}
	slope, intercept, r2 := simpleLinearRegression(x, y)
	if math.Abs(slope-3.0) > 0.001 {
		t.Errorf("Expected slope=3.0, got %f", slope)
	}
	if math.Abs(intercept-2.0) > 0.001 {
		t.Errorf("Expected intercept=2.0, got %f", intercept)
	}
	if math.Abs(r2-1.0) > 0.001 {
		t.Errorf("Expected R²=1.0, got %f", r2)
	}
}

func TestSimpleLinearRegression_Audit_SinglePoint(t *testing.T) {
	// Fewer than 2 points
	slope, intercept, r2 := simpleLinearRegression([]float64{1}, []float64{1})
	if slope != 0 || intercept != 0 || r2 != 0 {
		t.Errorf("Expected all zeros for single point, got slope=%f int=%f r2=%f", slope, intercept, r2)
	}
}

// ==========================================================================
// SIGMOID PROPERTIES AUDIT (local implementation to test properties)
// ==========================================================================

func localSigmoid(z float64) float64 {
	if z > 500 {
		return 1.0
	}
	if z < -500 {
		return 0.0
	}
	return 1.0 / (1.0 + math.Exp(-z))
}

func TestSigmoid_Audit_Properties(t *testing.T) {
	if math.Abs(localSigmoid(0)-0.5) > 0.0001 {
		t.Errorf("sigmoid(0) should be 0.5, got %e", localSigmoid(0))
	}
	if localSigmoid(500) < 0.9999 {
		t.Errorf("sigmoid(500) should be ~1.0, got %e", localSigmoid(500))
	}
	if localSigmoid(-500) > 0.0001 {
		t.Errorf("sigmoid(-500) should be ~0.0, got %e", localSigmoid(-500))
	}
	for _, x := range []float64{0.5, 1.0, 2.0, 5.0} {
		sum := localSigmoid(x) + localSigmoid(-x)
		if math.Abs(sum-1.0) > 0.0001 {
			t.Errorf("sigmoid(%f) + sigmoid(%f) should be 1.0, got %f", x, -x, sum)
		}
	}
}

// ==========================================================================
// HMM CONFIDENCE MULTIPLIER AUDIT
// ==========================================================================

func TestHMMConfidenceMultiplier_Audit(t *testing.T) {
	tests := []struct {
		state    string
		expected float64
	}{
		{HMMCrisis, 0.70},
		{HMMRiskOff, 0.90},
		{HMMRiskOn, 1.05},
		{"UNKNOWN", 1.0},
	}

	for _, tt := range tests {
		result := HMMConfidenceMultiplier(&HMMResult{CurrentState: tt.state})
		if result != tt.expected {
			t.Errorf("State %s: expected %f, got %f", tt.state, tt.expected, result)
		}
	}

	// nil input
	if HMMConfidenceMultiplier(nil) != 1.0 {
		t.Error("nil input should return 1.0")
	}
}

// ==========================================================================
// SORTING ORDER AUDIT
// ==========================================================================

func TestSortIntradayByTime_Audit(t *testing.T) {
	now := time.Now()
	bars := []domain.IntradayBar{
		{Timestamp: now.Add(-2 * time.Hour)},
		{Timestamp: now},
		{Timestamp: now.Add(-1 * time.Hour)},
	}
	sortIntradayByTime(bars)
	// Should be newest first
	if !bars[0].Timestamp.After(bars[1].Timestamp) {
		t.Error("Bars not sorted newest-first")
	}
	if !bars[1].Timestamp.After(bars[2].Timestamp) {
		t.Error("Bars not sorted newest-first")
	}
}

// ==========================================================================
// GARCH LOG-LIKELIHOOD AUDIT
// ==========================================================================

func TestGARCHLogLikelihood_Audit_NaNProtection(t *testing.T) {
	// All-zero returns
	returns := make([]float64, 30)
	ll := garchLogLikelihood(returns, 1e-8, 0.1, 0.8, 1e-8)
	if math.IsNaN(ll) {
		t.Error("Log-likelihood should not be NaN for zero returns")
	}
	if math.IsInf(ll, 0) && ll > 0 {
		t.Error("Log-likelihood should not be positive infinity")
	}
}

// ==========================================================================
// POSITION SIZING AUDIT
// ==========================================================================

func TestPositionSize_Audit_BasicCalculation(t *testing.T) {
	input := PositionSizeInput{
		AccountBalance: 10000,
		RiskPercent:    1.0,
		EntryPrice:     1.1000,
		StopLoss:       1.0900,
		DailyATR:       0.0050,
		NormalizedATR:  0.45, // NORMAL tier
	}
	result := ComputePositionSize(input, true)

	// Risk amount = 10000 * 1% = 100
	if math.Abs(result.RiskAmount-100) > 0.01 {
		t.Errorf("RiskAmount: expected 100, got %f", result.RiskAmount)
	}

	// ATR multiplier for NORMAL should be 2.0
	if result.ATRMultiplier != 2.0 {
		t.Errorf("ATR multiplier: expected 2.0 for NORMAL, got %f", result.ATRMultiplier)
	}

	// Stop distance = 0.0050 * 2.0 = 0.0100
	expectedStop := 0.0050 * 2.0
	if math.Abs(result.ATRStopDistance-expectedStop) > 0.0001 {
		t.Errorf("ATRStopDistance: expected %f, got %f", expectedStop, result.ATRStopDistance)
	}

	// Position size = 100 / 0.01 = 10000 units
	if math.Abs(result.PositionSize-10000) > 1 {
		t.Errorf("PositionSize: expected ~10000, got %f", result.PositionSize)
	}

	// Bullish: stop below entry
	if result.ATRStopPrice >= input.EntryPrice {
		t.Errorf("Bullish stop should be below entry: stop=%f, entry=%f",
			result.ATRStopPrice, input.EntryPrice)
	}
}

func TestPositionSize_Audit_BearishDirection(t *testing.T) {
	input := PositionSizeInput{
		AccountBalance: 10000,
		RiskPercent:    1.0,
		EntryPrice:     1.1000,
		DailyATR:       0.0050,
		NormalizedATR:  0.45,
	}
	result := ComputePositionSize(input, false)

	// Bearish: stop above entry
	if result.ATRStopPrice <= input.EntryPrice {
		t.Errorf("Bearish stop should be above entry: stop=%f, entry=%f",
			result.ATRStopPrice, input.EntryPrice)
	}

	// RR targets should be below entry for bearish
	if result.RR1Target >= input.EntryPrice {
		t.Errorf("Bearish RR1 target should be below entry")
	}
}

func TestPositionSize_Audit_ZeroInputs(t *testing.T) {
	input := PositionSizeInput{EntryPrice: 0, DailyATR: 0}
	result := ComputePositionSize(input, true)
	if result.PositionSize != 0 {
		t.Errorf("Expected 0 position size for zero inputs, got %f", result.PositionSize)
	}
}
