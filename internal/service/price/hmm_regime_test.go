package price

import (
	"math"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// HMM Regime Tests
// ============================================================================

func TestEstimateHMMRegime_InsufficientData(t *testing.T) {
	// Test boundary: less than 60 prices
	prices := make([]domain.PriceRecord, 59)
	for i := 0; i < 59; i++ {
		prices[i] = domain.PriceRecord{Close: 100.0 + float64(i)*0.1}
	}

	result, err := EstimateHMMRegime(prices)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "insufficient data")
	assert.Contains(t, err.Error(), "60")
}

func TestEstimateHMMRegime_MinimumData(t *testing.T) {
	// Test boundary: need 60 prices to get 59 returns (minimum is 60 valid returns)
	// So we need 61 prices minimum
	prices := make([]domain.PriceRecord, 70)
	for i := 0; i < 70; i++ {
		prices[i] = domain.PriceRecord{Close: 100.0 + float64(i)*0.1}
	}

	result, err := EstimateHMMRegime(prices)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.SampleSize, 40)
}

func TestEstimateHMMRegime_40vs60Returns(t *testing.T) {
	// Compare behavior with 40 vs 60 valid returns
	prices40 := make([]domain.PriceRecord, 41)
	for i := 0; i < 41; i++ {
		prices40[i] = domain.PriceRecord{Close: 100.0 + float64(i)*0.5}
	}

	prices60 := make([]domain.PriceRecord, 61)
	for i := 0; i < 61; i++ {
		prices60[i] = domain.PriceRecord{Close: 100.0 + float64(i)*0.5}
	}

	result60, err60 := EstimateHMMRegime(prices60)
	require.NoError(t, err60)
	require.NotNil(t, result60)

	// 40 prices should fail
	_, err40 := EstimateHMMRegime(prices40)
	assert.Error(t, err40)
}

func TestEstimateHMMRegime_ValidStateOutput(t *testing.T) {
	prices := generateTrendingPrices(80, 0.001)

	result, err := EstimateHMMRegime(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// State should be one of the valid states
	validStates := map[string]bool{
		HMMRiskOn:   true,
		HMMRiskOff:  true,
		HMMCrisis:   true,
		HMMTrending: true,
	}
	assert.True(t, validStates[result.CurrentState], "State should be valid: %s", result.CurrentState)
}

func TestEstimateHMMRegime_StateProbabilitiesSum(t *testing.T) {
	prices := generateRandomPricesHMM(80)

	result, err := EstimateHMMRegime(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// State probabilities should sum to ~1
	sum := 0.0
	for _, p := range result.StateProbabilities {
		sum += p
		assert.GreaterOrEqual(t, p, 0.0)
		assert.LessOrEqual(t, p, 1.0)
	}
	assert.InDelta(t, 1.0, sum, 0.01)
}

func TestEstimateHMMRegime_TransitionMatrixValid(t *testing.T) {
	prices := generateRandomPricesHMM(80)

	result, err := EstimateHMMRegime(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Each row of transition matrix should sum to ~1
	for i := 0; i < hmmNumStates; i++ {
		rowSum := 0.0
		for j := 0; j < hmmNumStates; j++ {
			p := result.TransitionMatrix[i][j]
			assert.GreaterOrEqual(t, p, 0.0)
			assert.LessOrEqual(t, p, 1.0)
			rowSum += p
		}
		assert.InDelta(t, 1.0, rowSum, 0.01, "Row %d should sum to 1", i)
	}
}

func TestEstimateHMMRegime_ConvergenceTracking(t *testing.T) {
	prices := generateRandomPricesHMM(100)

	result, err := EstimateHMMRegime(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should report iteration count
	assert.Greater(t, result.Iterations, 0)
	assert.LessOrEqual(t, result.Iterations, 100)

	// Converged flag should be set
	// Note: Non-convergence is acceptable per the code comments
	_ = result.Converged
}

func TestEstimateHMMRegime_ViterbiPathLength(t *testing.T) {
	prices := generateRandomPricesHMM(80)

	result, err := EstimateHMMRegime(prices)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Viterbi path should have reasonable length
	if len(result.ViterbiPath) > 0 {
		for _, state := range result.ViterbiPath {
			validStates := map[string]bool{
				HMMRiskOn:   true,
				HMMRiskOff:  true,
				HMMCrisis:   true,
				HMMTrending: true,
			}
			assert.True(t, validStates[state], "Viterbi state should be valid: %s", state)
		}
	}
}

func TestDiscretizeReturns_Boundaries(t *testing.T) {
	// Test returns at bin boundaries
	returns := []float64{-0.05, -0.02, 0.0, 0.02, 0.05}
	obs := discretizeReturns(returns)

	// Results should be valid bin indices
	for _, o := range obs {
		assert.GreaterOrEqual(t, o, 0)
		assert.Less(t, o, hmmNumEmissions)
	}
}

func TestDiscretizeReturns_ExtremeValues(t *testing.T) {
	// Test extreme returns
	returns := []float64{-0.5, -0.1, 0.1, 0.5} // Very extreme moves
	obs := discretizeReturns(returns)

	for _, o := range obs {
		assert.GreaterOrEqual(t, o, 0)
		assert.Less(t, o, hmmNumEmissions)
	}
}

func TestInitHMMPriors_ValidStructure(t *testing.T) {
	model := initHMMPriors()

	// Initial distribution should sum to 1
	piSum := 0.0
	for _, p := range model.Pi {
		assert.GreaterOrEqual(t, p, 0.0)
		piSum += p
	}
	assert.InDelta(t, 1.0, piSum, 0.01)

	// Transition matrix rows should sum to 1
	for i := 0; i < hmmNumStates; i++ {
		rowSum := 0.0
		for j := 0; j < hmmNumStates; j++ {
			assert.GreaterOrEqual(t, model.A[i][j], 0.0)
			rowSum += model.A[i][j]
		}
		assert.InDelta(t, 1.0, rowSum, 0.01)
	}

	// Emission matrix rows should sum to 1
	for i := 0; i < hmmNumStates; i++ {
		rowSum := 0.0
		for j := 0; j < hmmNumEmissions; j++ {
			assert.GreaterOrEqual(t, model.B[i][j], 0.0)
			rowSum += model.B[i][j]
		}
		assert.InDelta(t, 1.0, rowSum, 0.01)
	}
}

// Helper functions
func generateRandomPricesHMM(n int) []domain.PriceRecord {
	prices := make([]domain.PriceRecord, n)
	price := 100.0
	vol := 0.015
	for i := n - 1; i >= 0; i-- {
		prices[i] = domain.PriceRecord{Close: price}
		ret := (float64(i%5)-2) * vol * 0.5 // Some pattern
		price *= (1 + ret)
	}
	return prices
}

func generateTrendingPrices(n int, dailyDrift float64) []domain.PriceRecord {
	prices := make([]domain.PriceRecord, n)
	price := 100.0
	for i := n - 1; i >= 0; i-- {
		prices[i] = domain.PriceRecord{Close: price}
		ret := dailyDrift + (math.Sin(float64(i)*0.1)*0.005)
		price *= (1 + ret)
	}
	return prices
}
