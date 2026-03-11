// Package ports defines interfaces for dependency inversion.
// All adapters (storage, scraper, telegram, gemini) implement these interfaces.
// Services depend only on these interfaces, never on concrete implementations.
package ports

import (
	"context"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// ---------------------------------------------------------------------------
// EventRepository — Persistence for FF calendar events
// ---------------------------------------------------------------------------

// EventRepository defines storage operations for economic calendar events.
type EventRepository interface {
	// SaveEvents persists a batch of scraped events.
	// Existing events (same ID) are upserted.
	SaveEvents(ctx context.Context, events []domain.FFEvent) error

	// GetEventsByDateRange retrieves events within [start, end] inclusive.
	GetEventsByDateRange(ctx context.Context, start, end time.Time) ([]domain.FFEvent, error)

	// GetEventsByDate retrieves all events for a specific date.
	GetEventsByDate(ctx context.Context, date time.Time) ([]domain.FFEvent, error)

	// GetHighImpactEvents retrieves only high-impact events in date range.
	GetHighImpactEvents(ctx context.Context, start, end time.Time) ([]domain.FFEvent, error)

	// GetEventsByCurrency retrieves events filtered by currency code.
	GetEventsByCurrency(ctx context.Context, currency string, start, end time.Time) ([]domain.FFEvent, error)

	// SaveEventDetails persists historical data points for a recurring event.
	SaveEventDetails(ctx context.Context, details []domain.FFEventDetail) error

	// GetEventHistory retrieves historical data for a specific event.
	// Returns the last N months of data, ordered oldest-first.
	GetEventHistory(ctx context.Context, eventName, currency string, months int) ([]domain.FFEventDetail, error)

	// SaveRevision records a data revision.
	SaveRevision(ctx context.Context, rev domain.EventRevision) error

	// GetRevisions retrieves recent revisions for a currency.
	GetRevisions(ctx context.Context, currency string, days int) ([]domain.EventRevision, error)

	// GetAllRevisions retrieves all revisions within a time window.
	GetAllRevisions(ctx context.Context, days int) ([]domain.EventRevision, error)
}

// ---------------------------------------------------------------------------
// COTRepository — Persistence for COT data and analyses
// ---------------------------------------------------------------------------

// COTRepository defines storage operations for CFTC COT data.
type COTRepository interface {
	// SaveRecords persists raw COT records from CFTC.
	SaveRecords(ctx context.Context, records []domain.COTRecord) error

	// GetLatest retrieves the most recent COT record for a contract.
	GetLatest(ctx context.Context, contractCode string) (*domain.COTRecord, error)

	// GetHistory retrieves the last N weeks of COT records for a contract.
	// Ordered oldest-first for time-series calculations.
	GetHistory(ctx context.Context, contractCode string, weeks int) ([]domain.COTRecord, error)

	// SaveAnalyses persists computed COT analyses.
	SaveAnalyses(ctx context.Context, analyses []domain.COTAnalysis) error

	// GetLatestAnalysis retrieves the most recent analysis for a contract.
	GetLatestAnalysis(ctx context.Context, contractCode string) (*domain.COTAnalysis, error)

	// GetAllLatestAnalyses retrieves the most recent analysis for ALL contracts.
	GetAllLatestAnalyses(ctx context.Context) ([]domain.COTAnalysis, error)
}

// ---------------------------------------------------------------------------
// SurpriseRepository — Persistence for surprise scores and confluence
// ---------------------------------------------------------------------------

// SurpriseRepository defines storage for quantitative calculations.
type SurpriseRepository interface {
	// SaveSurprise persists a single surprise score.
	SaveSurprise(ctx context.Context, score domain.SurpriseScore) error

	// GetSurpriseScores retrieves surprise scores for a currency in the last N days.
	GetSurpriseScores(ctx context.Context, currency string, days int) ([]domain.SurpriseScore, error)

	// SaveSurpriseIndex persists a computed surprise index.
	SaveSurpriseIndex(ctx context.Context, index domain.SurpriseIndex) error

	// GetSurpriseIndex retrieves the latest surprise index for a currency.
	GetSurpriseIndex(ctx context.Context, currency string) (*domain.SurpriseIndex, error)

	// GetAllSurpriseIndices retrieves latest indices for all currencies.
	GetAllSurpriseIndices(ctx context.Context) ([]domain.SurpriseIndex, error)

	// SaveConfluence persists a confluence score.
	SaveConfluence(ctx context.Context, score domain.ConfluenceScore) error

	// GetLatestConfluence retrieves the latest confluence score for a pair.
	GetLatestConfluence(ctx context.Context, pair string) (*domain.ConfluenceScore, error)

	// GetAllConfluences retrieves latest confluence scores for all pairs.
	GetAllConfluences(ctx context.Context) ([]domain.ConfluenceScore, error)

	// SaveCurrencyRanking persists a currency ranking snapshot.
	SaveCurrencyRanking(ctx context.Context, ranking domain.CurrencyRanking) error

	// GetLatestRanking retrieves the most recent currency ranking.
	GetLatestRanking(ctx context.Context) (*domain.CurrencyRanking, error)

	// SaveVolatilityForecast persists a volatility forecast.
	SaveVolatilityForecast(ctx context.Context, forecast domain.VolatilityForecast) error

	// GetLatestVolatilityForecast retrieves the most recent forecast.
	GetLatestVolatilityForecast(ctx context.Context) (*domain.VolatilityForecast, error)
}

// ---------------------------------------------------------------------------
// PrefsRepository — User preferences persistence
// ---------------------------------------------------------------------------

// PrefsRepository defines storage for user notification preferences.
type PrefsRepository interface {
	// Get retrieves preferences for a user. Returns defaults if not found.
	Get(ctx context.Context, userID int64) (domain.UserPrefs, error)

	// Set persists preferences for a user.
	Set(ctx context.Context, userID int64, prefs domain.UserPrefs) error

	// GetAllActive retrieves all users with alerts enabled.
	GetAllActive(ctx context.Context) (map[int64]domain.UserPrefs, error)
}
