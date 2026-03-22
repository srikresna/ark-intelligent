package ports

import (
	"context"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Price Data Interfaces
// ---------------------------------------------------------------------------

// PriceFetcher defines the interface for fetching price data from external APIs.
type PriceFetcher interface {
	// FetchWeekly fetches weekly OHLC data for a single contract.
	// weeks specifies how many weeks of history to retrieve.
	FetchWeekly(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error)

	// FetchAll fetches weekly prices for all 11 default contracts,
	// routing each to the optimal API source.
	FetchAll(ctx context.Context, weeks int) ([]domain.PriceRecord, error)

	// HealthCheck verifies that at least one price API is reachable.
	HealthCheck(ctx context.Context) error
}

// PriceRepository defines storage operations for weekly price data.
type PriceRepository interface {
	// SavePrices persists a batch of weekly price records.
	SavePrices(ctx context.Context, records []domain.PriceRecord) error

	// GetLatest retrieves the most recent price record for a contract.
	GetLatest(ctx context.Context, contractCode string) (*domain.PriceRecord, error)

	// GetHistory retrieves price records for a contract over N weeks.
	// Ordered newest-first.
	GetHistory(ctx context.Context, contractCode string, weeks int) ([]domain.PriceRecord, error)

	// GetPriceAt retrieves the price record closest to the given date,
	// searching within 7 days in either direction. Returns nil if no
	// record exists within that window.
	GetPriceAt(ctx context.Context, contractCode string, date time.Time) (*domain.PriceRecord, error)
}

// ---------------------------------------------------------------------------
// Signal Backtesting Interfaces
// ---------------------------------------------------------------------------

// SignalRepository defines storage operations for persisted signal snapshots.
type SignalRepository interface {
	// SaveSignals persists a batch of signal snapshots.
	SaveSignals(ctx context.Context, signals []domain.PersistedSignal) error

	// GetSignalsByContract retrieves all persisted signals for a contract.
	// Ordered newest-first by report date.
	GetSignalsByContract(ctx context.Context, contractCode string) ([]domain.PersistedSignal, error)

	// GetSignalsByType retrieves all persisted signals of a given type across all contracts.
	GetSignalsByType(ctx context.Context, signalType string) ([]domain.PersistedSignal, error)

	// GetAllSignals retrieves all persisted signals.
	GetAllSignals(ctx context.Context) ([]domain.PersistedSignal, error)

	// GetPendingSignals retrieves signals that need outcome evaluation
	// (any horizon still pending and enough time has passed).
	GetPendingSignals(ctx context.Context) ([]domain.PersistedSignal, error)

	// UpdateSignal overwrites a single persisted signal (for outcome evaluation).
	UpdateSignal(ctx context.Context, signal domain.PersistedSignal) error

	// GetRecentSignals retrieves all signals from the last N days, newest-first.
	GetRecentSignals(ctx context.Context, days int) ([]domain.PersistedSignal, error)
}
