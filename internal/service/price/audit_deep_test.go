package price

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ==========================================================================
// DEEP AUDIT — Second-pass stress tests
// ==========================================================================

// --- GARCH: Verify variance targeting identity ---
func TestGARCH_Deep_VarianceTargeting(t *testing.T) {
	// omega = sampleVar * (1 - alpha - beta) means that LongRunVar approx sampleVar
	prices := generatePrices(200, 1.10, 0.0, 0.015, 7777)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	// Verify the variance targeting identity: omega / (1-alpha-beta) = LongRunVar
	// Both Omega and LongRunVar are rounded, so use generous tolerance.
	reconstructedOmega := result.LongRunVar * (1 - result.Alpha - result.Beta)
	if math.Abs(reconstructedOmega-result.Omega)/math.Max(math.Abs(result.Omega), 1e-15) > 0.01 {
		t.Errorf("Variance targeting broken: omega=%e, LRV*(1-alpha-beta)=%e",
			result.Omega, reconstructedOmega)
	}
}

// --- GARCH: Multi-step forecast formula verification ---
func TestGARCH_Deep_MultiStepForecast(t *testing.T) {
	prices := generatePrices(150, 50.0, 0.0, 0.02, 8888)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	// Manually compute 5-step forecast using the recurrence:
	// sigma2(t+h) = omega + (alpha+beta) * sigma2(t+h-1)
	persistence := result.Alpha + result.Beta
	manualVar := result.ForecastVar1
	for h := 2; h <= 5; h++ {
		manualVar = result.Omega + persistence*manualVar
	}

	if math.Abs(result.ForecastVar5-manualVar)/math.Max(result.ForecastVar5, 1e-15) > 0.01 {
		t.Errorf("5-step forecast mismatch: stored=%e, manual=%e",
			result.ForecastVar5, manualVar)
	}
}

// --- GARCH: Vol forecast direction consistency ---
func TestGARCH_Deep_VolForecastDirection(t *testing.T) {
	prices := generatePrices(150, 100.0, 0.0, 0.015, 9999)
	result, err := EstimateGARCH(prices)
	if err != nil {
		t.Fatalf("GARCH failed: %v", err)
	}

	// VolForecast should match the actual direction
	// Code: INCREASING if ForecastVar1 > CurrentVar*1.10, DECREASING if < CurrentVar*0.90
	switch result.VolForecast {
	case "INCREASING":
		if result.ForecastVar1 <= result.CurrentVar*1.10 {
			t.Errorf("INCREASING but ForecastVar1 (%e) <= CurrentVar*1.1 (%e)",
				result.ForecastVar1, result.CurrentVar*1.10)
		}
	case "DECREASING":
		if result.ForecastVar1 >= result.CurrentVar*0.90 {
			t.Errorf("DECREASING but ForecastVar1 (%e) >= CurrentVar*0.9 (%e)",
				result.ForecastVar1, result.CurrentVar*0.90)
		}
	case "STABLE":
		// Acceptable
	default:
		t.Errorf("Unknown VolForecast: %s", result.VolForecast)
	}
}

// --- Hurst: Trending series should give H > 0.5 ---
func TestHurst_Deep_TrendingSeries(t *testing.T) {
	// Generate a strongly trending series (geometric Brownian motion with high drift)
	rng := rand.New(rand.NewSource(1234))
	prices := make([]domain.PriceRecord, 300)
	price := 100.0
	now := time.Now()

	for i := 299; i >= 0; i-- {
		prices[i] = domain.PriceRecord{
			Date:  now.AddDate(0, 0, -(299 - i)),
			Close: price, High: price * 1.003, Low: price * 0.997, Open: price,
		}
		price *= 1 + 0.005 + rng.NormFloat64()*0.003 // Strong drift
	}

	result, err := ComputeHurstExponent(prices)
	if err != nil {
		t.Fatalf("Hurst failed: %v", err)
	}

	if result.H < 0.50 {
		t.Errorf("Expected H > 0.50 for trending series, got H=%f", result.H)
	}
	t.Logf("Trending series: H=%f, Classification=%s, R2=%f", result.H, result.Classification, result.RSquared)
}

// --- Hurst: R/S analysis block sizes must be valid ---
func TestHurst_Deep_BlockSizeSanity(t *testing.T) {
	// With exactly 50 prices (49 returns), check that sub-period sizes are reasonable
	prices := generatePrices(50, 100.0, 0.0, 0.02, 5555)
	result, err := ComputeHurstExponent(prices)
	if err != nil {
		t.Fatalf("Hurst failed: %v", err)
	}
	if result.SampleSize != 49 {
		t.Errorf("Expected 49 returns from 50 prices, got %d", result.SampleSize)
	}
}

// --- ADX: Known trending data should give high ADX ---
func TestADX_Deep_TrendingData(t *testing.T) {
	// Prices monotonically increasing (newest-first)
	prices := make([]domain.PriceRecord, 20)
	for i := 0; i < 20; i++ {
		base := 100.0 + float64(19-i)*2.0 // newest=138, oldest=100
		prices[i] = domain.PriceRecord{
			Close: base,
			High:  base + 1.0,
			Low:   base - 0.5,
			Open:  base - 0.5,
		}
	}

	adx := approximateADX(prices)
	if adx < 20 {
		t.Errorf("Expected ADX > 20 for trending data, got %.2f", adx)
	}
}

// --- ADX: Flat data should give low ADX ---
func TestADX_Deep_FlatData(t *testing.T) {
	prices := make([]domain.PriceRecord, 20)
	for i := 0; i < 20; i++ {
		prices[i] = domain.PriceRecord{
			Close: 100.0,
			High:  100.5,
			Low:   99.5,
			Open:  100.0,
		}
	}

	adx := approximateADX(prices)
	// DX = |+DI - -DI| / (+DI + -DI) -- with no directional movement, should be 0
	if adx > 20 {
		t.Errorf("Expected low ADX for flat data, got %.2f", adx)
	}
}

// --- Volatility: ATR calculation verification ---
func TestVolatility_Deep_ATRManualCheck(t *testing.T) {
	// Manual TR calculation
	prices := []domain.PriceRecord{
		{High: 110, Low: 95, Close: 105}, // newest
		{High: 108, Low: 98, Close: 100}, // prevClose for bar[0]
		{High: 105, Low: 96, Close: 102}, // prevClose for bar[1]
	}

	// TR[0] = max(110-95, |110-100|, |95-100|) = max(15, 10, 5) = 15
	// TR[1] = max(108-98, |108-102|, |98-102|) = max(10, 6, 4) = 10
	// ATR(2) = (15 + 10) / 2 = 12.5
	atr := ComputeATR(prices, 2)
	if math.Abs(atr-12.5) > 0.001 {
		t.Errorf("ATR(2) expected 12.5, got %f", atr)
	}
}

// --- Volatility regime classification ---
func TestVolatility_Deep_RegimeClassification(t *testing.T) {
	tests := []struct {
		current, avg float64
		expected     string
	}{
		{10, 8, VolatilityNormal},       // 10/8 = 1.25 exactly -> normal (not >1.25)
		{6, 8, VolatilityNormal},        // 6/8 = 0.75 exactly -> normal (not <0.75)
		{8, 8, VolatilityNormal},        // 8/8 = 1.0 -> normal
		{10.1, 8, VolatilityExpanding},  // 10.1/8 = 1.2625 > 1.25
		{5.9, 8, VolatilityContracting}, // 5.9/8 = 0.7375 < 0.75
	}

	for _, tt := range tests {
		got := ClassifyVolatilityRegime(tt.current, tt.avg)
		if got != tt.expected {
			t.Errorf("ClassifyVolatilityRegime(%f, %f) = %s, want %s",
				tt.current, tt.avg, got, tt.expected)
		}
	}
}

// --- Position sizing: R:R targets symmetry ---
func TestPositionSize_Deep_RRTargets(t *testing.T) {
	input := PositionSizeInput{
		AccountBalance: 50000,
		RiskPercent:    2.0,
		EntryPrice:     1.2000,
		DailyATR:       0.0100,
		NormalizedATR:  0.83,
	}

	bull := ComputePositionSize(input, true)
	bear := ComputePositionSize(input, false)

	// For same ATR, stop distance should be equal
	if math.Abs(bull.ATRStopDistance-bear.ATRStopDistance) > 0.0001 {
		t.Errorf("Stop distance should be equal for bull/bear: %f vs %f",
			bull.ATRStopDistance, bear.ATRStopDistance)
	}

	// Bullish targets above entry, bearish below
	if bull.RR2Target <= input.EntryPrice {
		t.Error("Bullish RR2 should be above entry")
	}
	if bear.RR2Target >= input.EntryPrice {
		t.Error("Bearish RR2 should be below entry")
	}
}

// --- HMM: Convergence flag should reflect actual convergence ---
func TestHMM_Deep_ConvergenceCheck(t *testing.T) {
	prices := generatePrices(200, 100.0, 0.0, 0.02, 4321)
	result, err := EstimateHMMRegime(prices)
	if err != nil {
		t.Fatalf("HMM failed: %v", err)
	}

	// With 200 data points, Baum-Welch should typically converge.
	// maxIter is 100, so the iteration variable can be at most 99
	// when convergence happens via break, or 100 when the loop completes without converging.
	if result.Converged {
		if result.Iterations >= 100 {
			t.Errorf("Converged=true but used max iterations (%d)", result.Iterations)
		}
	}
	t.Logf("HMM: converged=%v, iterations=%d", result.Converged, result.Iterations)
}

// --- Correlation: Self-correlation must be 1.0 ---
func TestCorrelation_Deep_SelfCorrelation(t *testing.T) {
	x := []float64{1.5, -2.3, 0.7, 3.1, -1.4, 2.8, 0.0, -0.5, 1.2, 3.4}
	r := pearsonCorrelation(x, x)
	if math.Abs(r-1.0) > 0.0001 {
		t.Errorf("Self-correlation should be 1.0, got %f", r)
	}
}

// --- Crisis detection: extreme weekly range ---
func TestCrisis_Deep_ExtremeRange(t *testing.T) {
	// Normal prices followed by one extreme bar
	prices := make([]domain.PriceRecord, 10)
	for i := 0; i < 10; i++ {
		prices[i] = domain.PriceRecord{
			High: 100.5, Low: 99.5, Close: 100.0,
		}
	}
	// Make the newest bar have 10x the normal range
	prices[0] = domain.PriceRecord{
		High: 110.0, Low: 90.0, Close: 100.0,
	}

	// WeeklyRange() = (High-Low)/Close * 100
	// Latest range = (110-90)/100*100 = 20%, avg of rest = (100.5-99.5)/100*100 = 1%
	// 20 > 1*3 = true -> crisis
	crisis := isCrisis(prices, nil)
	if !crisis {
		t.Error("Expected crisis detection for 10x range expansion")
	}
}
