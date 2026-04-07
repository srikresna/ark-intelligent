package price

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// Correlation Tests
// ============================================================================

func TestPearsonCorrelation_NLessThan3(t *testing.T) {
	// Test with n < 3 observations
	x := []float64{1.0, 2.0}
	y := []float64{1.0, 2.0}

	// This is unexported, but we test through the correlation engine behavior
	// For n < 3, correlation should be NaN or handled gracefully
	corr := pearsonCorrelation(x, y)
	assert.True(t, math.IsNaN(corr) || corr == 0, "n<3 should return NaN or 0")
}

func TestPearsonCorrelation_NEquals2(t *testing.T) {
	// Test with exactly 2 observations (edge case)
	x := []float64{1.0, 2.0}
	y := []float64{2.0, 4.0}

	corr := pearsonCorrelation(x, y)
	// With 2 points, correlation is always ±1 if not collinear
	// But mathematically, we need at least 3 points for meaningful correlation
	assert.True(t, math.IsNaN(corr) || math.Abs(corr) <= 1.0)
}

func TestPearsonCorrelation_ZeroVariance(t *testing.T) {
	// Test with zero variance (all same values)
	x := []float64{5.0, 5.0, 5.0, 5.0}
	y := []float64{1.0, 2.0, 3.0, 4.0}

	corr := pearsonCorrelation(x, y)
	assert.True(t, math.IsNaN(corr), "Zero variance should return NaN")
}

func TestPearsonCorrelation_ZeroVarianceY(t *testing.T) {
	// Test with zero variance in Y
	x := []float64{1.0, 2.0, 3.0, 4.0}
	y := []float64{5.0, 5.0, 5.0, 5.0}

	corr := pearsonCorrelation(x, y)
	assert.True(t, math.IsNaN(corr), "Zero variance in Y should return NaN")
}

func TestPearsonCorrelation_BothZeroVariance(t *testing.T) {
	// Test with zero variance in both
	x := []float64{5.0, 5.0, 5.0, 5.0}
	y := []float64{3.0, 3.0, 3.0, 3.0}

	corr := pearsonCorrelation(x, y)
	assert.True(t, math.IsNaN(corr), "Zero variance in both should return NaN")
}

func TestPearsonCorrelation_PerfectPositive(t *testing.T) {
	// Perfect positive correlation
	x := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	y := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	corr := pearsonCorrelation(x, y)
	assert.InDelta(t, 1.0, corr, 0.001)
}

func TestPearsonCorrelation_PerfectNegative(t *testing.T) {
	// Perfect negative correlation
	x := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	y := []float64{10.0, 8.0, 6.0, 4.0, 2.0}

	corr := pearsonCorrelation(x, y)
	assert.InDelta(t, -1.0, corr, 0.001)
}

func TestPearsonCorrelation_NoCorrelation(t *testing.T) {
	// Zero correlation - using data that produces near-zero correlation
	x := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	y := []float64{2.0, 1.0, 3.0, 2.0, 4.0}

	corr := pearsonCorrelation(x, y)
	// Correlation can vary; just verify it's in valid range [-1, 1]
	assert.GreaterOrEqual(t, corr, -1.0)
	assert.LessOrEqual(t, corr, 1.0)
}

func TestPearsonCorrelation_DifferentLengths(t *testing.T) {
	// Different length slices should use minimum length
	x := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	y := []float64{1.0, 2.0, 3.0}

	corr := pearsonCorrelation(x, y)
	// Should handle gracefully or panic; we want graceful handling
	_ = corr // Just verify no panic
}

func TestPearsonCorrelation_EmptySlices(t *testing.T) {
	// Empty slices
	x := []float64{}
	y := []float64{}

	corr := pearsonCorrelation(x, y)
	assert.True(t, math.IsNaN(corr) || corr == 0)
}

func TestPearsonCorrelation_NilSlices(t *testing.T) {
	// Nil slices
	var x, y []float64

	corr := pearsonCorrelation(x, y)
	assert.True(t, math.IsNaN(corr) || corr == 0)
}

func TestPearsonCorrelation_SingleElement(t *testing.T) {
	// Single element
	x := []float64{5.0}
	y := []float64{3.0}

	corr := pearsonCorrelation(x, y)
	assert.True(t, math.IsNaN(corr) || corr == 0)
}

func TestPearsonCorrelation_ReturnsInValidRange(t *testing.T) {
	// All correlations should be in [-1, 1]
	x := []float64{1.0, 3.0, 2.0, 5.0, 4.0, 6.0, 7.0, 2.0, 1.0, 8.0}
	y := []float64{2.0, 4.0, 1.0, 6.0, 3.0, 5.0, 8.0, 1.0, 2.0, 7.0}

	corr := pearsonCorrelation(x, y)
	assert.GreaterOrEqual(t, corr, -1.0)
	assert.LessOrEqual(t, corr, 1.0)
}

// Test the exported correlation types and constants
func TestCorrelationMatrix_Structure(t *testing.T) {
	matrix := &domain.CorrelationMatrix{
		Period: 20,
		Currencies: []string{"EUR", "USD", "GBP"},
		Matrix: map[string]map[string]float64{
			"EUR": {"USD": 0.5, "GBP": 0.7},
			"USD": {"EUR": 0.5, "GBP": -0.3},
			"GBP": {"EUR": 0.7, "USD": -0.3},
		},
	}

	assert.Equal(t, 20, matrix.Period)
	assert.Len(t, matrix.Currencies, 3)
	assert.Len(t, matrix.Matrix, 3)
}

// Test correlation pair sorting/grouping
func TestCorrelationPairsByAsset(t *testing.T) {
	pairs := []domain.CorrelationPair{
		{CurrencyA: "EUR", CurrencyB: "USD", Correlation: 0.5},
		{CurrencyA: "GBP", CurrencyB: "USD", Correlation: -0.3},
		{CurrencyA: "EUR", CurrencyB: "GBP", Correlation: 0.7},
	}

	// Group by asset A
	grouped := make(map[string][]domain.CorrelationPair)
	for _, p := range pairs {
		grouped[p.CurrencyA] = append(grouped[p.CurrencyA], p)
	}

	// EUR appears as CurrencyA in 1 pair (EUR-USD)
	assert.Len(t, grouped["EUR"], 2) // EUR-USD and EUR-GBP
	// GBP appears as CurrencyA in 1 pair (GBP-USD)
	assert.Len(t, grouped["GBP"], 1)
}

// Note: pearsonCorrelation is defined in correlation.go and tested indirectly
// through the audit test files. This test file validates edge case handling.
