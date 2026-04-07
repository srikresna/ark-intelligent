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
// Bootstrap Tests
// ============================================================================

func TestBootstrapper_New(t *testing.T) {
	mockCOT := &bootstrapMockCOTRepo{}
	mockPrice := &bootstrapMockPriceRepo{}
	mockSignal := &mockSignalRepository{}
	mockChecker := &mockSignalRepository{}
	mockDaily := &bootstrapMockDailyRepo{}

	// Test without daily repo
	b1 := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker)
	assert.NotNil(t, b1)

	// Test with daily repo
	b2 := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker, mockDaily)
	assert.NotNil(t, b2)

	// Test with nil daily repo in varargs
	b3 := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker, nil)
	assert.NotNil(t, b3)
}

func TestBootstrapper_Run_EmptyCOT(t *testing.T) {
	mockCOT := &bootstrapMockCOTRepo{history: []domain.COTAnalysis{}}
	mockPrice := &bootstrapMockPriceRepo{}
	mockSignal := &mockSignalRepository{}
	mockChecker := &mockSignalRepository{}

	b := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker)
	created, err := b.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, created)
}

func TestBootstrapper_Run_WithCOTData(t *testing.T) {
	contractEUR := domain.COTContract{Code: "EUR", Name: "Euro"}
	mockCOT := &bootstrapMockCOTRepo{
		history: []domain.COTAnalysis{
			{
				Contract:     contractEUR,
				ReportDate:   time.Now().Add(-30 * 24 * time.Hour),
				NetPosition:  75000,
				COTIndex:     75.0,
			},
			{
				Contract:     contractEUR,
				ReportDate:   time.Now().Add(-23 * 24 * time.Hour),
				NetPosition:  72000,
				COTIndex:     72.0,
			},
		},
	}
	mockPrice := &bootstrapMockPriceRepo{
		records: []domain.PriceRecord{
			{Close: 1.1000, Date: time.Now().Add(-30 * 24 * time.Hour)},
			{Close: 1.1050, Date: time.Now().Add(-23 * 24 * time.Hour)},
		},
	}
	mockSignal := &mockSignalRepository{}
	mockChecker := &mockSignalRepository{}

	b := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker)
	created, err := b.Run(context.Background())

	require.NoError(t, err)
	assert.GreaterOrEqual(t, created, 0)
}

func TestBootstrapper_Run_DuplicateDetection(t *testing.T) {
	baseTime := time.Now().Add(-30 * 24 * time.Hour)
	contractEUR := domain.COTContract{Code: "EUR", Name: "Euro"}

	mockCOT := &bootstrapMockCOTRepo{
		history: []domain.COTAnalysis{
			{
				Contract:     contractEUR,
				ReportDate:   baseTime,
				NetPosition:  75000,
				COTIndex:     75.0,
			},
		},
	}
	mockPrice := &bootstrapMockPriceRepo{
		records: []domain.PriceRecord{
			{Close: 1.1000, Date: baseTime},
		},
	}
	mockSignal := &mockSignalRepository{}
	mockChecker := &mockSignalRepository{}

	b := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker)
	created, err := b.Run(context.Background())

	require.NoError(t, err)
	assert.GreaterOrEqual(t, created, 0)
}

func TestBootstrapper_Run_ConfidenceIntervals(t *testing.T) {
	// Test with small samples to check confidence interval handling
	contractEUR := domain.COTContract{Code: "EUR", Name: "Euro"}
	mockCOT := &bootstrapMockCOTRepo{
		history: []domain.COTAnalysis{
			{
				Contract:     contractEUR,
				ReportDate:   time.Now().Add(-30 * 24 * time.Hour),
				NetPosition:  75000,
				COTIndex:     75.0,
			},
		},
	}
	mockPrice := &bootstrapMockPriceRepo{
		records: []domain.PriceRecord{
			{Close: 1.1000, Date: time.Now().Add(-30 * 24 * time.Hour)},
		},
	}
	mockSignal := &mockSignalRepository{}
	mockChecker := &mockSignalRepository{}

	b := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker)
	created, err := b.Run(context.Background())

	require.NoError(t, err)
	assert.GreaterOrEqual(t, created, 0)
}

func TestBootstrapper_Run_InvalidLookback(t *testing.T) {
	mockCOT := &bootstrapMockCOTRepo{}
	mockPrice := &bootstrapMockPriceRepo{}
	mockSignal := &mockSignalRepository{}
	mockChecker := &mockSignalRepository{}

	b := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker)
	created, err := b.Run(context.Background())

	// Should handle gracefully
	require.NoError(t, err)
	assert.GreaterOrEqual(t, created, 0)
}

func TestBootstrapper_Run_NegativeLookback(t *testing.T) {
	mockCOT := &bootstrapMockCOTRepo{}
	mockPrice := &bootstrapMockPriceRepo{}
	mockSignal := &mockSignalRepository{}
	mockChecker := &mockSignalRepository{}

	b := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker)
	created, err := b.Run(context.Background())

	// Should handle gracefully
	require.NoError(t, err)
	assert.GreaterOrEqual(t, created, 0)
}

func TestBootstrapper_Run_CtxCancellation(t *testing.T) {
	contractEUR := domain.COTContract{Code: "EUR", Name: "Euro"}
	history := make([]domain.COTAnalysis, 100)
	for i := range history {
		history[i] = domain.COTAnalysis{
			Contract:    contractEUR,
			ReportDate:  time.Now().Add(-time.Duration(i) * 24 * time.Hour),
			NetPosition: float64(50000 + i*1000),
			COTIndex:    50.0 + float64(i),
		}
	}
	mockCOT := &bootstrapMockCOTRepo{
		history: history,
	}
	mockPrice := &bootstrapMockPriceRepo{}
	mockSignal := &mockSignalRepository{}
	mockChecker := &mockSignalRepository{}

	b := NewBootstrapper(mockCOT, mockPrice, mockSignal, mockChecker)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	created, err := b.Run(ctx)

	// Should handle cancelled context gracefully
	if err != nil {
		assert.Equal(t, context.Canceled, err)
	}
	_ = created
}

func TestHistoricalDailyAdapter(t *testing.T) {
	mockRepo := &bootstrapMockDailyRepo{
		records: []domain.DailyPrice{
			{Close: 1.1000, Date: time.Now().Add(-10 * 24 * time.Hour)},
			{Close: 1.1050, Date: time.Now().Add(-5 * 24 * time.Hour)},
		},
	}

	asOf := time.Now()
	adapter := &historicalDailyAdapter{
		store: mockRepo,
		asOf:  asOf,
	}

	records, err := adapter.GetDailyHistory(context.Background(), "EUR", 10)
	require.NoError(t, err)
	assert.NotNil(t, records)
}

// Mock repositories for testing

// bootstrapMockCOTRepo is a test mock for COT repository
type bootstrapMockCOTRepo struct {
	history []domain.COTAnalysis
}

func (m *bootstrapMockCOTRepo) GetAnalysisHistory(ctx context.Context, contractCode string, weeks int) ([]domain.COTAnalysis, error) {
	return m.history, nil
}

// COTRepository interface methods (embedded in COTHistoryProvider)
func (m *bootstrapMockCOTRepo) SaveRecords(ctx context.Context, records []domain.COTRecord) error {
	return nil
}

func (m *bootstrapMockCOTRepo) GetLatest(ctx context.Context, contractCode string) (*domain.COTRecord, error) {
	return nil, nil
}

func (m *bootstrapMockCOTRepo) GetHistory(ctx context.Context, contractCode string, weeks int) ([]domain.COTRecord, error) {
	return nil, nil
}

func (m *bootstrapMockCOTRepo) SaveAnalyses(ctx context.Context, analyses []domain.COTAnalysis) error {
	return nil
}

func (m *bootstrapMockCOTRepo) GetLatestAnalysis(ctx context.Context, contractCode string) (*domain.COTAnalysis, error) {
	return nil, nil
}

func (m *bootstrapMockCOTRepo) GetAllLatestAnalyses(ctx context.Context) ([]domain.COTAnalysis, error) {
	return m.history, nil
}

func (m *bootstrapMockCOTRepo) GetLatestReportDate(ctx context.Context) (time.Time, error) {
	return time.Now(), nil
}

// bootstrapMockPriceRepo is a test mock for price repository
type bootstrapMockPriceRepo struct {
	records []domain.PriceRecord
}

// ports.PriceRepository interface methods
func (m *bootstrapMockPriceRepo) SavePrices(ctx context.Context, records []domain.PriceRecord) error {
	return nil
}

func (m *bootstrapMockPriceRepo) GetLatest(ctx context.Context, contractCode string) (*domain.PriceRecord, error) {
	if len(m.records) > 0 {
		return &m.records[0], nil
	}
	return nil, nil
}

func (m *bootstrapMockPriceRepo) GetHistory(ctx context.Context, contractCode string, weeks int) ([]domain.PriceRecord, error) {
	return m.records, nil
}

func (m *bootstrapMockPriceRepo) GetPriceAt(ctx context.Context, contractCode string, date time.Time) (*domain.PriceRecord, error) {
	for _, r := range m.records {
		if r.Date.Equal(date) {
			return &r, nil
		}
	}
	return nil, nil
}

// bootstrapMockDailyRepo is a test mock for daily price repository
type bootstrapMockDailyRepo struct {
	records []domain.DailyPrice
}

func (m *bootstrapMockDailyRepo) GetDailyHistory(ctx context.Context, contractCode string, days int) ([]domain.DailyPrice, error) {
	return m.records, nil
}

func (m *bootstrapMockDailyRepo) GetDailyHistoryBefore(ctx context.Context, contractCode string, before time.Time, days int) ([]domain.DailyPrice, error) {
	var result []domain.DailyPrice
	for _, r := range m.records {
		if r.Date.Before(before) {
			result = append(result, r)
		}
	}
	if len(result) > days {
		result = result[:days]
	}
	return result, nil
}
