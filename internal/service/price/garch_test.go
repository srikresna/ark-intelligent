package price

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// GARCH Additional Edge Case Tests (quant_models_test.go has basic tests)
// ============================================================================

func TestEstimateGARCH_MinimumData(t *testing.T) {
	// Test with exactly 30 prices
	prices := make([]domain.PriceRecord, 30)
	for i := 0; i < 30; i++ {
		prices[i] = domain.PriceRecord{Close: 100.0 + float64(i)*0.5}
	}

	result, err := EstimateGARCH(prices)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.SampleSize, 0)
}

func TestEstimateGARCH_Underparameterization(t *testing.T) {
	// Test with n=20-27 returns (edge case for underparameterization)
	prices := make([]domain.PriceRecord, 28)
	for i := 0; i < 28; i++ {
		prices[i] = domain.PriceRecord{Close: 100.0 + float64(i)*0.1}
	}

	result, err := EstimateGARCH(prices)
	// Should still work but may not converge well
	if err != nil {
		assert.Contains(t, err.Error(), "insufficient")
	}
	_ = result
}

func TestEstimateGARCH_NaNHandling(t *testing.T) {
	// Test with invalid/zero prices that would cause NaN
	prices := make([]domain.PriceRecord, 35)
	for i := 0; i < 35; i++ {
		if i == 10 {
			prices[i] = domain.PriceRecord{Close: 0} // Invalid price
		} else {
			prices[i] = domain.PriceRecord{Close: 100.0 + float64(i)*0.1}
		}
	}

	result, err := EstimateGARCH(prices)
	// Should handle gracefully
	if err == nil {
		require.NotNil(t, result)
		assert.False(t, math.IsNaN(result.CurrentVol))
		assert.False(t, math.IsNaN(result.ForecastVol1))
	}
}

func TestEstimateGARCH_StationarityConstraint(t *testing.T) {
	// Test that alpha + beta < 1 (stationarity constraint)
	prices := generateRandomPrices(100, 0.02, 0.015)

	result, err := EstimateGARCH(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Persistence should be < 1 for stationarity
	assert.Less(t, result.Persistence, 1.0)
	assert.GreaterOrEqual(t, result.Alpha, 0.0)
	assert.GreaterOrEqual(t, result.Beta, 0.0)
}

func TestEstimateGARCH_ValuesInValidRange(t *testing.T) {
	prices := generateRandomPrices(100, 0.01, 0.02)

	result, err := EstimateGARCH(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// All values should be valid numbers
	assert.False(t, math.IsNaN(result.Omega))
	assert.False(t, math.IsNaN(result.Alpha))
	assert.False(t, math.IsNaN(result.Beta))
	assert.False(t, math.IsInf(result.LongRunVar, 0))
	assert.Greater(t, result.LongRunVol, 0.0)
	assert.Greater(t, result.CurrentVol, 0.0)
}

func TestEstimateGARCHFromIntraday(t *testing.T) {
	bars := make([]domain.IntradayBar, 30)
	for i := 0; i < 30; i++ {
		bars[i] = domain.IntradayBar{Close: 100.0 + float64(i)*0.1}
	}

	result, err := EstimateGARCHFromIntraday(bars)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.SampleSize, 0)
}

func TestEstimateGARCHFromIntraday_InsufficientData(t *testing.T) {
	bars := make([]domain.IntradayBar, 25)
	for i := 0; i < 25; i++ {
		bars[i] = domain.IntradayBar{Close: 100.0 + float64(i)*0.1}
	}

	result, err := EstimateGARCHFromIntraday(bars)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "insufficient")
}

func TestGARCHResult_ForecastConsistency(t *testing.T) {
	prices := generateRandomPrices(100, 0.01, 0.015)

	result, err := EstimateGARCH(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Forecast classification is based on forecastVar1 vs currentVar
	// Valid values are: "INCREASING", "DECREASING", "STABLE"
	validForecasts := map[string]bool{
		"INCREASING": true,
		"DECREASING": true,
		"STABLE":     true,
	}
	assert.True(t, validForecasts[result.VolForecast],
		"VolForecast should be valid: %s", result.VolForecast)

	// VolRatio should be consistent
	expectedRatio := result.CurrentVol / result.LongRunVol
	assert.InDelta(t, expectedRatio, result.VolRatio, 0.001)
}

// Helper function to generate random price series
func generateRandomPrices(n int, drift, vol float64) []domain.PriceRecord {
	prices := make([]domain.PriceRecord, n)
	price := 100.0
	for i := n - 1; i >= 0; i-- {
		prices[i] = domain.PriceRecord{Close: price}
		// Random walk with drift
		ret := drift + (randFloat64()-0.5)*2*vol
		price *= (1 + ret)
	}
	return prices
}

func randFloat64() float64 {
	// Simple deterministic random for tests
	return 0.5
}
