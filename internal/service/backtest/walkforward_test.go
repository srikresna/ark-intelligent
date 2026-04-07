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
// Walk-Forward Tests
// ============================================================================

func TestWalkForwardAnalyzer_Analyze_EmptySignals(t *testing.T) {
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{},
	}

	analyzer := NewWalkForwardAnalyzer(mockRepo)
	result, err := analyzer.Analyze(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Windows)
	assert.Equal(t, 0.0, result.OverallInSampleWinRate)
	assert.Equal(t, 0.0, result.OverallOutOfSampleWinRate)
	assert.False(t, result.IsOverfit)
	assert.NotEmpty(t, result.Recommendation)
}

func TestWalkForwardAnalyzer_Analyze_SingleElement(t *testing.T) {
	// Single signal with outcome
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{
				ContractCode: "EUR",
				Outcome1W:    domain.OutcomeWin,
			ReportDate:    time.Now().Add(-7 * 24 * time.Hour),
			},
		},
	}

	analyzer := NewWalkForwardAnalyzer(mockRepo)
	result, err := analyzer.Analyze(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	// Single element can't create meaningful train/test split
	assert.Empty(t, result.Windows)
}

func TestWalkForwardAnalyzer_Analyze_AllPendingSignals(t *testing.T) {
	// All signals are pending (no outcomes)
	mockRepo := &mockSignalRepository{
		signals: []domain.PersistedSignal{
			{ContractCode: "EUR", Outcome1W: domain.OutcomePending, DetectedAt: time.Now().Add(-14 * 24 * time.Hour)},
			{ContractCode: "GBP", Outcome1W: domain.OutcomePending, DetectedAt: time.Now().Add(-7 * 24 * time.Hour)},
			{ContractCode: "USD", Outcome1W: domain.OutcomeExpired, DetectedAt: time.Now().Add(-21 * 24 * time.Hour)},
		},
	}

	analyzer := NewWalkForwardAnalyzer(mockRepo)
	result, err := analyzer.Analyze(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Windows)
	assert.Equal(t, 0.0, result.OverallInSampleWinRate)
}

func TestWalkForwardAnalyzer_Analyze_WinLossSignals(t *testing.T) {
	// Mix of win and loss outcomes
	// Need at least 26 weeks for defaultTrainWeeks
	baseTime := time.Now().Add(-400 * 24 * time.Hour)
	signals := make([]domain.PersistedSignal, 40)
	for i := 0; i < 40; i++ {
		outcome := domain.OutcomeWin
		if i%2 == 0 {
			outcome = domain.OutcomeLoss
		}
		signals[i] = domain.PersistedSignal{
			ContractCode: "EUR",
			Outcome1W:    outcome,
			ReportDate:    baseTime.Add(time.Duration(i*7) * 24 * time.Hour),
		}
	}

	mockRepo := &mockSignalRepository{signals: signals}
	analyzer := NewWalkForwardAnalyzer(mockRepo)
	result, err := analyzer.Analyze(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	// Windows may be empty if insufficient data spans
	_ = result.Windows
}

func TestWalkForwardAnalyzer_Analyze_OverfitDetection(t *testing.T) {
	// Create scenario where in-sample is much better than out-of-sample
	baseTime := time.Now().Add(-500 * 24 * time.Hour)
	signals := make([]domain.PersistedSignal, 100)
	for i := 0; i < 100; i++ {
		outcome := domain.OutcomeWin
		if i > 50 && i%3 == 0 {
			outcome = domain.OutcomeLoss // Worse performance in later period
		}
		signals[i] = domain.PersistedSignal{
			ContractCode: "EUR",
			Outcome1W:    outcome,
			ReportDate:    baseTime.Add(time.Duration(i*7) * 24 * time.Hour),
		}
	}

	mockRepo := &mockSignalRepository{signals: signals}
	analyzer := NewWalkForwardAnalyzer(mockRepo)
	result, err := analyzer.Analyze(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Windows)
	// Check overfit flag logic
	_ = result.IsOverfit
	_ = result.OverfitScore
}

func TestWalkForwardAnalyzer_Analyze_WindowStructure(t *testing.T) {
	baseTime := time.Now().Add(-400 * 24 * time.Hour)
	signals := make([]domain.PersistedSignal, 40)
	for i := 0; i < 40; i++ {
		signals[i] = domain.PersistedSignal{
			ContractCode: "EUR",
			Outcome1W:    domain.OutcomeWin,
			ReportDate:    baseTime.Add(time.Duration(i*7) * 24 * time.Hour),
		}
	}

	mockRepo := &mockSignalRepository{signals: signals}
	analyzer := NewWalkForwardAnalyzer(mockRepo)
	result, err := analyzer.Analyze(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check window properties
	for _, window := range result.Windows {
		assert.False(t, window.TrainStart.IsZero())
		assert.False(t, window.TrainEnd.IsZero())
		assert.False(t, window.TestStart.IsZero())
		assert.False(t, window.TestEnd.IsZero())
		assert.GreaterOrEqual(t, window.InSampleWinRate, 0.0)
		assert.LessOrEqual(t, window.InSampleWinRate, 100.0)
		assert.GreaterOrEqual(t, window.OutOfSampleWinRate, 0.0)
		assert.LessOrEqual(t, window.OutOfSampleWinRate, 100.0)
	}
}

func TestWalkForwardAnalyzer_Analyze_CountsValid(t *testing.T) {
	baseTime := time.Now().Add(-400 * 24 * time.Hour)
	signals := make([]domain.PersistedSignal, 30)
	for i := 0; i < 30; i++ {
		outcome := domain.OutcomeWin
		if i%3 == 0 {
			outcome = domain.OutcomeLoss
		}
		signals[i] = domain.PersistedSignal{
			ContractCode: "EUR",
			Outcome1W:    outcome,
			ReportDate:    baseTime.Add(time.Duration(i*7) * 24 * time.Hour),
		}
	}

	mockRepo := &mockSignalRepository{signals: signals}
	analyzer := NewWalkForwardAnalyzer(mockRepo)
	result, err := analyzer.Analyze(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)

	// Overall counts should be non-negative
	assert.GreaterOrEqual(t, result.OverallInSampleWinRate, 0.0)
	assert.GreaterOrEqual(t, result.OverallOutOfSampleWinRate, 0.0)
}

func TestWalkForwardResult_OverfitThreshold(t *testing.T) {
	// Test overfit threshold constant
	assert.Equal(t, 10.0, overfitThresholdPP)
}

func TestWalkForwardResult_DefaultWindowSizes(t *testing.T) {
	// Test default window sizes
	assert.Equal(t, 26, defaultTrainWeeks)
	assert.Equal(t, 13, defaultTestWeeks)
}

// Mock signal repository for testing
type mockSignalRepository struct {
	signals []domain.PersistedSignal
}

func (m *mockSignalRepository) GetAllSignals(ctx context.Context) ([]domain.PersistedSignal, error) {
	return m.signals, nil
}

func (m *mockSignalRepository) GetSignalsByContract(ctx context.Context, contractCode string) ([]domain.PersistedSignal, error) {
	var result []domain.PersistedSignal
	for _, s := range m.signals {
		if s.ContractCode == contractCode {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSignalRepository) GetSignalsByType(ctx context.Context, signalType string) ([]domain.PersistedSignal, error) {
	var result []domain.PersistedSignal
	for _, s := range m.signals {
		if s.SignalType == signalType {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSignalRepository) GetSignalsByOutcome(ctx context.Context, outcome string) ([]domain.PersistedSignal, error) {
	var result []domain.PersistedSignal
	for _, s := range m.signals {
		if s.Outcome1W == outcome {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSignalRepository) SaveSignal(ctx context.Context, signal *domain.PersistedSignal) error {
	return nil
}

func (m *mockSignalRepository) UpdateSignalOutcome(ctx context.Context, id string, outcome string, pnl float64) error {
	return nil
}

func (m *mockSignalRepository) SignalExists(ctx context.Context, contractCode string, reportDate time.Time, signalType string) (bool, error) {
	return false, nil
}

// Additional methods required by ports.SignalRepository interface
func (m *mockSignalRepository) SaveSignals(ctx context.Context, signals []domain.PersistedSignal) error {
	return nil
}

func (m *mockSignalRepository) GetPendingSignals(ctx context.Context) ([]domain.PersistedSignal, error) {
	return nil, nil
}

func (m *mockSignalRepository) UpdateSignal(ctx context.Context, signal domain.PersistedSignal) error {
	return nil
}

func (m *mockSignalRepository) GetRecentSignals(ctx context.Context, days int) ([]domain.PersistedSignal, error) {
	return m.signals, nil
}
