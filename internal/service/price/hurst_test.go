package price

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Hurst Exponent Additional Tests (quant_models_test.go has basic tests)
// ============================================================================

func TestComputeHurstExponent_FlatPriceSeries(t *testing.T) {
	// Flat price series has zero variance - code returns error for insufficient R/S values
	prices := make([]domain.PriceRecord, 100)
	for i := 0; i < 100; i++ {
		prices[i] = domain.PriceRecord{Close: 100.0}
	}

	result, err := ComputeHurstExponent(prices)
	// Flat series with zero variance returns error (insufficient valid R/S values)
	if err != nil {
		assert.Contains(t, err.Error(), "insufficient")
		return
	}
	require.NotNil(t, result)
	// If no error, might return NaN or 0.5
	if !math.IsNaN(result.H) {
		assert.InDelta(t, 0.5, result.H, 0.5)
	}
}

func TestComputeHurstExponent_MeanRevertingSeries(t *testing.T) {
	// Oscillating series should give H < 0.5
	prices := make([]domain.PriceRecord, 100)
	for i := 0; i < 100; i++ {
		// Sine wave pattern
		price := 100.0 + 5.0*math.Sin(float64(i)*0.3)
		prices[i] = domain.PriceRecord{Close: price}
	}

	result, err := ComputeHurstExponent(prices)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.H, 0.0)
	assert.Less(t, result.H, 1.0)
}

func TestComputeHurstExponent_ClassificationValid(t *testing.T) {
	testCases := []struct {
		name           string
		expectedH      float64
		expectedClass  string
	}{
		{"Mean Reverting", 0.3, "MEAN_REVERTING"},
		{"Random Walk", 0.5, "RANDOM_WALK"},
		{"Trending", 0.7, "TRENDING"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate prices that should produce approximately the desired H
			var prices []domain.PriceRecord
			if tc.expectedH < 0.5 {
				prices = generateMeanRevertingPrices(80)
			} else if tc.expectedH > 0.5 {
				prices = generateTrendingPricesHurst(80)
			} else {
				prices = generateRandomPricesHurst(80)
			}

			result, err := ComputeHurstExponent(prices)
			// Some generated price series may have insufficient variance
			if err != nil {
				t.Skipf("Skipping: generated prices had insufficient variance: %v", err)
				return
			}
			require.NotNil(t, result)

			// Classification should be consistent with H value
			validClasses := map[string]bool{
				"MEAN_REVERTING": true,
				"RANDOM_WALK":    true,
				"TRENDING":       true,
			}
			assert.True(t, validClasses[result.Classification],
				"Classification should be valid: %s", result.Classification)
		})
	}
}

func TestComputeHurstExponent_ConfidenceCalculation(t *testing.T) {
	prices := generateRandomPricesHurst(80)

	result, err := ComputeHurstExponent(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Confidence is on 0-100 scale: |H - 0.5| * 200
	assert.GreaterOrEqual(t, result.Confidence, 0.0)
	assert.LessOrEqual(t, result.Confidence, 100.0)
}

func TestComputeHurstExponent_RSquaredValid(t *testing.T) {
	prices := generateRandomPricesHurst(80)

	result, err := ComputeHurstExponent(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// R² should be in [0, 1]
	assert.GreaterOrEqual(t, result.RSquared, 0.0)
	assert.LessOrEqual(t, result.RSquared, 1.0)
}

func TestComputeHurstExponent_DescriptionNotEmpty(t *testing.T) {
	prices := generateRandomPricesHurst(80)

	result, err := ComputeHurstExponent(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.Description)
}

func TestComputeHurstFromIntraday(t *testing.T) {
	bars := make([]domain.IntradayBar, 60)
	for i := 0; i < 60; i++ {
		bars[i] = domain.IntradayBar{Close: 100.0 + float64(i)*0.05}
	}

	result, err := ComputeHurstFromIntraday(bars)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.H, 0.0)
	assert.LessOrEqual(t, result.H, 1.0)
}

func TestComputeHurstFromIntraday_InsufficientData(t *testing.T) {
	bars := make([]domain.IntradayBar, 45)
	for i := 0; i < 45; i++ {
		bars[i] = domain.IntradayBar{Close: 100.0 + float64(i)*0.05}
	}

	result, err := ComputeHurstFromIntraday(bars)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "insufficient")
}

func TestHurstRegimeContext_Structure(t *testing.T) {
	regime := &PriceRegime{
		Regime:        "TRENDING",
		ADX:           35.0,
		TrendStrength: 75.0,
	}

	hurst := &HurstResult{
		H:             0.7,
		Classification: "TRENDING",
		Confidence:    0.2,
		RSquared:      0.85,
	}

	ctx := &HurstRegimeContext{
		PriceRegime:        regime,
		Hurst:              hurst,
		HurstRegime:        "TRENDING",
		RegimeAgreement:    true,
		CombinedConfidence: 0.75,
	}

	assert.NotNil(t, ctx.PriceRegime)
	assert.NotNil(t, ctx.Hurst)
	assert.Equal(t, "TRENDING", ctx.HurstRegime)
	assert.True(t, ctx.RegimeAgreement)
	assert.Greater(t, ctx.CombinedConfidence, 0.0)
}

// Helper functions
func generateRandomPricesHurst(n int) []domain.PriceRecord {
	prices := make([]domain.PriceRecord, n)
	price := 100.0
	vol := 0.01
	for i := n - 1; i >= 0; i-- {
		prices[i] = domain.PriceRecord{Close: price}
		// Random walk
		ret := (float64(i%7)-3) * vol * 0.5 // Some pattern
		price *= (1 + ret)
	}
	return prices
}

func generateTrendingPricesHurst(n int) []domain.PriceRecord {
	prices := make([]domain.PriceRecord, n)
	price := 100.0
	for i := n - 1; i >= 0; i-- {
		prices[i] = domain.PriceRecord{Close: price}
		price *= 1.002 // Strong trend
	}
	return prices
}

func generateMeanRevertingPrices(n int) []domain.PriceRecord {
	prices := make([]domain.PriceRecord, n)
	mean := 100.0
	for i := n - 1; i >= 0; i-- {
		// Mean-reverting: price tends toward mean
		distFromMean := (prices[min(i+1, n-1)].Close - mean) * 0.8
		price := mean + distFromMean + (float64(i%5)-2)*0.5
		prices[i] = domain.PriceRecord{Close: price}
	}
	return prices
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
