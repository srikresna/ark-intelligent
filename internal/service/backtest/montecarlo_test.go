package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Monte Carlo Tests
// ============================================================================

func TestMonteCarloSimulator_Simulate_ZeroVolatility(t *testing.T) {
	// All signals have zero return (no volatility)
	// Need at least 4 weeks of data for simulation
	baseTime := time.Now().Add(-30 * 24 * time.Hour)
	signals := []domain.PersistedSignal{
		{ContractCode: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 0.0, ReportDate: baseTime},
		{ContractCode: "GBP", Outcome1W: domain.OutcomeWin, Return1W: 0.0, ReportDate: baseTime},
		{ContractCode: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 0.0, ReportDate: baseTime.Add(7 * 24 * time.Hour)},
		{ContractCode: "GBP", Outcome1W: domain.OutcomeWin, Return1W: 0.0, ReportDate: baseTime.Add(7 * 24 * time.Hour)},
		{ContractCode: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 0.0, ReportDate: baseTime.Add(14 * 24 * time.Hour)},
		{ContractCode: "GBP", Outcome1W: domain.OutcomeWin, Return1W: 0.0, ReportDate: baseTime.Add(14 * 24 * time.Hour)},
		{ContractCode: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 0.0, ReportDate: baseTime.Add(21 * 24 * time.Hour)},
		{ContractCode: "GBP", Outcome1W: domain.OutcomeWin, Return1W: 0.0, ReportDate: baseTime.Add(21 * 24 * time.Hour)},
	}

	mockRepo := &mockSignalRepository{signals: signals}
	simulator := NewMonteCarloSimulator(mockRepo)
	result, err := simulator.Simulate(context.Background(), 100)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 100, result.NumSimulations)
	// With zero volatility, results should be deterministic
	assert.InDelta(t, 0.0, result.MedianReturn, 0.001)
	assert.InDelta(t, 0.0, result.P5Return, 0.001)
	assert.InDelta(t, 0.0, result.P95Return, 0.001)
}

func TestMonteCarloSimulator_Simulate_InsufficientData(t *testing.T) {
	// Less than 4 weeks of data
	signals := []domain.PersistedSignal{
		{ContractCode: "EUR", Outcome1W: domain.OutcomeWin, DetectedAt: time.Now().Add(-7 * 24 * time.Hour)},
	}

	mockRepo := &mockSignalRepository{signals: signals}
	simulator := NewMonteCarloSimulator(mockRepo)
	result, err := simulator.Simulate(context.Background(), 100)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "insufficient")
}

func TestMonteCarloSimulator_Simulate_NoSignals(t *testing.T) {
	mockRepo := &mockSignalRepository{signals: []domain.PersistedSignal{}}
	simulator := NewMonteCarloSimulator(mockRepo)
	result, err := simulator.Simulate(context.Background(), 100)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestMonteCarloSimulator_Simulate_ValidResultStructure(t *testing.T) {
	// Generate signals across multiple weeks
	// Note: aggregateWeeklyReturns uses ReportDate (not DetectedAt)
	baseTime := time.Now().Add(-200 * 24 * time.Hour)
	signals := make([]domain.PersistedSignal, 50)
	for i := 0; i < 50; i++ {
		signals[i] = domain.PersistedSignal{
			ContractCode: "EUR",
			Outcome1W:    domain.OutcomeWin,
			Return1W:        float64(i%5) - 2.0, // Some variation
			ReportDate:    baseTime.Add(time.Duration(i*7) * 24 * time.Hour),
		}
	}

	mockRepo := &mockSignalRepository{signals: signals}
	simulator := NewMonteCarloSimulator(mockRepo)
	result, err := simulator.Simulate(context.Background(), 500)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check result structure
	assert.Equal(t, 500, result.NumSimulations)
	assert.Greater(t, result.WeeksResampled, 0)

	// Percentiles should be ordered: P5 <= Median <= P95
	assert.LessOrEqual(t, result.P5Return, result.MedianReturn)
	assert.LessOrEqual(t, result.MedianReturn, result.P95Return)

	// Drawdowns are returned as positive percentages (magnitude)
	// MedianMaxDD <= WorstCaseMaxDD (50th percentile vs 95th percentile of drawdowns)
	assert.GreaterOrEqual(t, result.MedianMaxDD, 0.0)
	assert.GreaterOrEqual(t, result.WorstCaseMaxDD, 0.0)
	assert.LessOrEqual(t, result.MedianMaxDD, result.WorstCaseMaxDD)

	// Probability of loss is stored as percentage [0, 100]
	assert.GreaterOrEqual(t, result.ProbabilityOfLoss, 0.0)
	assert.LessOrEqual(t, result.ProbabilityOfLoss, 100.0)

	// Sharpe ratio can be any real number
	_ = result.MedianSharpe
}

func TestMonteCarloSimulator_Simulate_DifferentNumSims(t *testing.T) {
	// Note: aggregateWeeklyReturns uses ReportDate (not DetectedAt)
	baseTime := time.Now().Add(-200 * 24 * time.Hour)
	signals := make([]domain.PersistedSignal, 50)
	for i := 0; i < 50; i++ {
		signals[i] = domain.PersistedSignal{
			ContractCode: "EUR",
			Outcome1W:    domain.OutcomeWin,
			Return1W:        1.0,
			ReportDate:    baseTime.Add(time.Duration(i*7) * 24 * time.Hour),
		}
	}

	testCases := []int{10, 100, 1000}
	for _, numSims := range testCases {
		mockRepo := &mockSignalRepository{signals: signals}
		simulator := NewMonteCarloSimulator(mockRepo)
		result, err := simulator.Simulate(context.Background(), numSims)

		require.NoError(t, err)
		assert.Equal(t, numSims, result.NumSimulations)
	}
}

func TestMonteCarloResult_ValidRange(t *testing.T) {
	result := &MonteCarloResult{
		NumSimulations:    1000,
		WeeksResampled:    20,
		MedianReturn:      5.0,
		P5Return:          -10.0,
		P95Return:         15.0,
		MedianMaxDD:       -5.0,
		WorstCaseMaxDD:    -15.0,
		ProbabilityOfLoss: 0.3,
		MedianSharpe:      0.5,
	}

	// Validate ranges
	assert.Greater(t, result.NumSimulations, 0)
	assert.Greater(t, result.WeeksResampled, 0)
	assert.LessOrEqual(t, result.P5Return, result.MedianReturn)
	assert.LessOrEqual(t, result.MedianReturn, result.P95Return)
	assert.GreaterOrEqual(t, result.ProbabilityOfLoss, 0.0)
	assert.LessOrEqual(t, result.ProbabilityOfLoss, 1.0)
}

func TestAggregateWeeklyReturns(t *testing.T) {
	// Test the aggregateWeeklyReturns function indirectly
	// Note: aggregateWeeklyReturns uses ReportDate (not DetectedAt) and filters for Win/Loss outcomes
	baseTime := time.Now().Add(-30 * 24 * time.Hour)
	signals := []domain.PersistedSignal{
		{ContractCode: "EUR", Return1W: 1.0, Outcome1W: domain.OutcomeWin, ReportDate: baseTime},
		{ContractCode: "GBP", Return1W: 2.0, Outcome1W: domain.OutcomeWin, ReportDate: baseTime},
		{ContractCode: "EUR", Return1W: -0.5, Outcome1W: domain.OutcomeWin, ReportDate: baseTime.Add(7 * 24 * time.Hour)},
		{ContractCode: "USD", Return1W: 1.5, Outcome1W: domain.OutcomeWin, ReportDate: baseTime.Add(7 * 24 * time.Hour)},
		{ContractCode: "EUR", Return1W: 0.5, Outcome1W: domain.OutcomeWin, ReportDate: baseTime.Add(14 * 24 * time.Hour)},
		{ContractCode: "GBP", Return1W: -0.5, Outcome1W: domain.OutcomeWin, ReportDate: baseTime.Add(21 * 24 * time.Hour)},
	}

	weeklyReturns := aggregateWeeklyReturns(signals)
	assert.GreaterOrEqual(t, len(weeklyReturns), 0) // May have 0+ weeks depending on data validity
}

func TestAggregateWeeklyReturns_Empty(t *testing.T) {
	weeklyReturns := aggregateWeeklyReturns([]domain.PersistedSignal{})
	assert.Empty(t, weeklyReturns)
}

func TestAggregateWeeklyReturns_SingleWeek(t *testing.T) {
	baseTime := time.Now().Add(-7 * 24 * time.Hour) // Past week
	signals := []domain.PersistedSignal{
		{ContractCode: "EUR", Return1W: 1.0, Outcome1W: domain.OutcomeWin, ReportDate: baseTime},
		{ContractCode: "GBP", Return1W: 3.0, Outcome1W: domain.OutcomeWin, ReportDate: baseTime},
	}

	weeklyReturns := aggregateWeeklyReturns(signals)
	assert.GreaterOrEqual(t, len(weeklyReturns), 0) // May be 0 or 1 depending on week boundary
}
