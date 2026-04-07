package backtest

import (
	"context"
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// StatsCalculator Additional Tests
// ============================================================================

func TestStatsCalculator_ComputeAll_ZeroEvaluatedSignals(t *testing.T) {
	// Test win rate with 0 evaluated signals (all pending)
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			// All signals are pending, none evaluated
			{ContractCode: "EUR", Outcome1W: domain.OutcomePending, Confidence: 50},
			{ContractCode: "GBP", Outcome1W: domain.OutcomePending, Confidence: 50},
		},
	}

	calc := NewStatsCalculator(mockRepo)
	stats, err := calc.ComputeAll(context.Background())

	require.NoError(t, err)
	require.NotNil(t, stats)
	
	// With 0 evaluated signals, win rates should be 0
	assert.Equal(t, 0.0, stats.WinRate1W)
	assert.Equal(t, 0.0, stats.WinRate2W)
	assert.Equal(t, 0.0, stats.WinRate4W)
	// TotalSignals counts all signals, not just evaluated ones
	assert.Equal(t, 2, stats.TotalSignals)
	assert.Equal(t, "ALL", stats.GroupLabel)
}

func TestStatsCalculator_ComputeByContract_ZeroSignals(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{},
	}

	calc := NewStatsCalculator(mockRepo)
	stats, err := calc.ComputeByContract(context.Background(), "EUR")

	require.NoError(t, err)
	require.NotNil(t, stats)
	
	// With 0 signals, should return empty stats
	assert.Equal(t, 0, stats.TotalSignals)
	assert.Equal(t, 0.0, stats.WinRate1W)
}

func TestStatsCalculator_ComputeBySignalType_ZeroSignals(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{},
	}

	calc := NewStatsCalculator(mockRepo)
	stats, err := calc.ComputeBySignalType(context.Background(), "SMART_MONEY")

	require.NoError(t, err)
	require.NotNil(t, stats)
	
	// With 0 signals, should return empty stats
	assert.Equal(t, 0, stats.TotalSignals)
	assert.Equal(t, 0.0, stats.WinRate1W)
}

func TestStatsCalculator_ComputeAllByContract_Empty(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{},
	}

	calc := NewStatsCalculator(mockRepo)
	result, err := calc.ComputeAllByContract(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestStatsCalculator_ComputeAllBySignalType_Empty(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{},
	}

	calc := NewStatsCalculator(mockRepo)
	result, err := calc.ComputeAllBySignalType(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestStatsCalculator_ComputeAll_WithSignals(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{
				ContractCode: "EUR",
				Outcome1W:    domain.OutcomeWin,
				Return1W:     1.5,
				Direction:    "BULLISH",
				Strength:     4,
				Confidence:   75.0,
			},
			{
				ContractCode: "EUR",
				Outcome1W:    domain.OutcomeLoss,
				Return1W:     -0.5,
				Direction:    "BULLISH",
				Strength:     3,
				Confidence:   60.0,
			},
			{
				ContractCode: "GBP",
				Outcome1W:    domain.OutcomeWin,
				Return1W:     2.0,
				Direction:    "BEARISH",
				Strength:     5,
				Confidence:   80.0,
			},
		},
	}

	calc := NewStatsCalculator(mockRepo)
	stats, err := calc.ComputeAll(context.Background())

	require.NoError(t, err)
	require.NotNil(t, stats)
	
	assert.Equal(t, 3, stats.TotalSignals)
	assert.Equal(t, "ALL", stats.GroupLabel)
	// 2 wins out of 3
	assert.InDelta(t, 66.67, stats.WinRate1W, 0.01)
}

func TestStatsCalculator_ComputeByContract_WithSignals(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{
				ContractCode: "EUR",
				Outcome1W:    domain.OutcomeWin,
				Return1W:     1.5,
				Direction:    "BULLISH",
			},
			{
				ContractCode: "EUR",
				Outcome1W:    domain.OutcomeWin,
				Return1W:     2.0,
				Direction:    "BULLISH",
			},
			{
				ContractCode: "GBP",
				Outcome1W:    domain.OutcomeLoss,
				Return1W:     -1.0,
				Direction:    "BEARISH",
			},
		},
	}

	calc := NewStatsCalculator(mockRepo)
	stats, err := calc.ComputeByContract(context.Background(), "EUR")

	require.NoError(t, err)
	require.NotNil(t, stats)
	
	assert.Equal(t, 2, stats.TotalSignals)
	assert.Equal(t, 100.0, stats.WinRate1W) // 2 wins out of 2
}

func TestStatsCalculator_ComputeBySignalType_WithSignals(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{
				ContractCode: "EUR",
				SignalType:   "SMART_MONEY",
				Outcome1W:    domain.OutcomeWin,
				Return1W:     1.5,
			},
			{
				ContractCode: "GBP",
				SignalType:   "SMART_MONEY",
				Outcome1W:    domain.OutcomeLoss,
				Return1W:     -0.5,
			},
			{
				ContractCode: "EUR",
				SignalType:   "COT",
				Outcome1W:    domain.OutcomeWin,
				Return1W:     2.0,
			},
		},
	}

	calc := NewStatsCalculator(mockRepo)
	stats, err := calc.ComputeBySignalType(context.Background(), "SMART_MONEY")

	require.NoError(t, err)
	require.NotNil(t, stats)
	
	assert.Equal(t, 2, stats.TotalSignals)
	assert.Equal(t, 50.0, stats.WinRate1W) // 1 win out of 2
}

func TestStatsCalculator_ComputeAllByContract_Grouping(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{ContractCode: "EUR", Currency: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 1.0},
			{ContractCode: "EUR", Currency: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 1.5},
			{ContractCode: "GBP", Currency: "GBP", Outcome1W: domain.OutcomeLoss, Return1W: -0.5},
		},
	}

	calc := NewStatsCalculator(mockRepo)
	result, err := calc.ComputeAllByContract(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	
	assert.Len(t, result, 2)
	
	eurStats, ok := result["EUR"]
	require.True(t, ok)
	assert.Equal(t, 2, eurStats.TotalSignals)
	assert.Equal(t, 100.0, eurStats.WinRate1W)
	
	gbpStats, ok := result["GBP"]
	require.True(t, ok)
	assert.Equal(t, 1, gbpStats.TotalSignals)
	assert.Equal(t, 0.0, gbpStats.WinRate1W)
}

func TestStatsCalculator_ComputeAllBySignalType_Grouping(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{ContractCode: "EUR", SignalType: "SMART_MONEY", Outcome1W: domain.OutcomeWin, Return1W: 1.0},
			{ContractCode: "EUR", SignalType: "SMART_MONEY", Outcome1W: domain.OutcomeLoss, Return1W: -0.5},
			{ContractCode: "GBP", SignalType: "COT", Outcome1W: domain.OutcomeWin, Return1W: 2.0},
		},
	}

	calc := NewStatsCalculator(mockRepo)
	result, err := calc.ComputeAllBySignalType(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	
	assert.Len(t, result, 2)
	
	smStats, ok := result["SMART_MONEY"]
	require.True(t, ok)
	assert.Equal(t, 2, smStats.TotalSignals)
	assert.Equal(t, 50.0, smStats.WinRate1W)
	
	cotStats, ok := result["COT"]
	require.True(t, ok)
	assert.Equal(t, 1, cotStats.TotalSignals)
	assert.Equal(t, 100.0, cotStats.WinRate1W)
}

func TestStatsCalculator_ComputeByContract_NonExistent(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{ContractCode: "EUR", Outcome1W: domain.OutcomeWin, Return1W: 1.0},
		},
	}

	calc := NewStatsCalculator(mockRepo)
	stats, err := calc.ComputeByContract(context.Background(), "JPY")

	require.NoError(t, err)
	require.NotNil(t, stats)
	
	assert.Equal(t, 0, stats.TotalSignals)
	assert.Equal(t, 0.0, stats.WinRate1W)
}

func TestStatsCalculator_ComputeBySignalType_NonExistent(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{ContractCode: "EUR", SignalType: "SMART_MONEY", Outcome1W: domain.OutcomeWin, Return1W: 1.0},
		},
	}

	calc := NewStatsCalculator(mockRepo)
	stats, err := calc.ComputeBySignalType(context.Background(), "NON_EXISTENT")

	require.NoError(t, err)
	require.NotNil(t, stats)
	
	assert.Equal(t, 0, stats.TotalSignals)
	assert.Equal(t, 0.0, stats.WinRate1W)
}
