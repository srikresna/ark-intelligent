package price

import (
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// syntheticPrices generates n synthetic daily prices (newest-first) starting
// from startPrice with given daily drift and volatility.
func syntheticPrices(n int, startPrice, dailyDrift, dailyVol float64) []domain.PriceRecord {
	// Build oldest-first, then reverse
	prices := make([]domain.PriceRecord, n)
	price := startPrice
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < n; i++ {
		open := price
		ret := dailyDrift + dailyVol*0.1*float64(i%7-3) // deterministic pseudo-noise
		price = price * math.Exp(ret)
		prices[i] = domain.PriceRecord{
			ContractCode: "099741",
			Symbol:       "EUR/USD",
			Date:         base.AddDate(0, 0, i),
			Open:         open,
			High:         math.Max(open, price) * 1.001,
			Low:          math.Min(open, price) * 0.999,
			Close:        price,
			Source:       "synthetic",
		}
	}

	// Reverse to newest-first
	for i, j := 0, len(prices)-1; i < j; i, j = i+1, j-1 {
		prices[i], prices[j] = prices[j], prices[i]
	}
	return prices
}

func TestGenerateScenario_Basic(t *testing.T) {
	prices := syntheticPrices(120, 1.0800, 0.0001, 0.005)

	result, err := GenerateScenario(prices, "EUR/USD", nil)
	if err != nil {
		t.Fatalf("GenerateScenario failed: %v", err)
	}

	// Basic validations
	if result.Symbol != "EUR/USD" {
		t.Errorf("expected symbol EUR/USD, got %s", result.Symbol)
	}
	if result.HorizonDays != defaultHorizonDay {
		t.Errorf("expected horizon %d, got %d", defaultHorizonDay, result.HorizonDays)
	}
	if result.NumPaths != defaultNumPaths {
		t.Errorf("expected %d paths, got %d", defaultNumPaths, result.NumPaths)
	}
	if result.CurrentPrice <= 0 {
		t.Errorf("current price should be positive, got %f", result.CurrentPrice)
	}

	// Percentiles should be monotonically increasing
	for i := 1; i < len(result.Percentiles); i++ {
		if result.Percentiles[i].Price < result.Percentiles[i-1].Price {
			t.Errorf("percentiles not monotonic: P%.0f=%f > P%.0f=%f",
				result.Percentiles[i-1].Percentile*100, result.Percentiles[i-1].Price,
				result.Percentiles[i].Percentile*100, result.Percentiles[i].Price)
		}
	}

	// VaR95 should be negative (a loss)
	if result.VaR95 > 0 {
		t.Logf("VaR95 is positive (%.2f%%) — unusual but possible in strong uptrend", result.VaR95)
	}

	// VaR99 should be <= VaR95 (worse loss)
	if result.VaR99 > result.VaR95 {
		t.Errorf("VaR99 (%.2f%%) should be <= VaR95 (%.2f%%)", result.VaR99, result.VaR95)
	}

	// CVaR95 should be <= VaR95
	if result.CVaR95 > result.VaR95 {
		t.Errorf("CVaR95 (%.2f%%) should be <= VaR95 (%.2f%%)", result.CVaR95, result.VaR95)
	}

	// Regime should be one of the valid HMM states
	validRegimes := map[string]bool{HMMRiskOn: true, HMMRiskOff: true, HMMCrisis: true}
	if !validRegimes[result.Regime] {
		t.Errorf("invalid regime: %s", result.Regime)
	}

	// Vol estimate should be reasonable (0% < vol < 200% annualised)
	if result.VolEstimate <= 0 || result.VolEstimate > 2.0 {
		t.Errorf("vol estimate out of range: %.4f", result.VolEstimate)
	}
}

func TestGenerateScenario_CustomConfig(t *testing.T) {
	prices := syntheticPrices(100, 1.0800, 0.0002, 0.004)

	cfg := &ScenarioConfig{
		NumPaths:    500,
		HorizonDays: 10,
	}
	result, err := GenerateScenario(prices, "EUR/USD", cfg)
	if err != nil {
		t.Fatalf("GenerateScenario with config failed: %v", err)
	}

	if result.NumPaths != 500 {
		t.Errorf("expected 500 paths, got %d", result.NumPaths)
	}
	if result.HorizonDays != 10 {
		t.Errorf("expected 10-day horizon, got %d", result.HorizonDays)
	}
}

func TestGenerateScenario_InsufficientData(t *testing.T) {
	prices := syntheticPrices(30, 1.0800, 0.0, 0.005) // too few

	_, err := GenerateScenario(prices, "EUR/USD", nil)
	if err == nil {
		t.Error("expected error for insufficient data, got nil")
	}
}

func TestLogReturnsFromPrices(t *testing.T) {
	prices := []domain.PriceRecord{
		{Close: 110}, // newest
		{Close: 100},
		{Close: 105}, // oldest
	}

	returns := logReturnsFromPrices(prices)
	if len(returns) != 2 {
		t.Fatalf("expected 2 returns, got %d", len(returns))
	}

	// returns[0] = ln(110/100) ≈ 0.0953
	expected0 := math.Log(110.0 / 100.0)
	if math.Abs(returns[0]-expected0) > 1e-10 {
		t.Errorf("returns[0] = %f, expected %f", returns[0], expected0)
	}

	// returns[1] = ln(100/105) ≈ -0.0488
	expected1 := math.Log(100.0 / 105.0)
	if math.Abs(returns[1]-expected1) > 1e-10 {
		t.Errorf("returns[1] = %f, expected %f", returns[1], expected1)
	}
}

func TestSampleVariance(t *testing.T) {
	// Known: variance of [1, 2, 3, 4, 5] = 2.0 (population)
	data := []float64{1, 2, 3, 4, 5}
	v := sampleVariance(data)
	if math.Abs(v-2.0) > 0.01 {
		t.Errorf("sampleVariance = %f, expected ~2.0", v)
	}

	// Edge case: single element
	v = sampleVariance([]float64{42})
	if v != 0 {
		t.Errorf("sampleVariance of single element = %f, expected 0", v)
	}

	// Edge case: empty
	v = sampleVariance(nil)
	if v != 0 {
		t.Errorf("sampleVariance of nil = %f, expected 0", v)
	}
}

func TestGenerateScenario_MedianNearCurrentPrice(t *testing.T) {
	// With zero drift and low vol, median should be close to current price
	prices := syntheticPrices(120, 1.0800, 0.0, 0.002)

	cfg := &ScenarioConfig{
		NumPaths:    2000, // more paths for stability
		HorizonDays: 5,   // short horizon
	}
	result, err := GenerateScenario(prices, "EUR/USD", cfg)
	if err != nil {
		t.Fatalf("GenerateScenario failed: %v", err)
	}

	// Find median (P50)
	var medianReturn float64
	for _, p := range result.Percentiles {
		if p.Percentile == 0.50 {
			medianReturn = p.Return
			break
		}
	}

	// With low drift and short horizon, median return should be < ±5%
	if math.Abs(medianReturn) > 5.0 {
		t.Errorf("median return %.2f%% too far from zero for low-drift scenario", medianReturn)
	}
}
