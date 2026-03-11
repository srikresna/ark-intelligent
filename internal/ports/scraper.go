package ports

import (
	"context"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// ---------------------------------------------------------------------------
// FFScraper — ForexFactory calendar data acquisition
// ---------------------------------------------------------------------------

// FFScraper defines the interface for scraping ForexFactory calendar data.
// Implementations may use cloudscraper, headless browser, or API fallback.
type FFScraper interface {
	// ScrapeWeeklyCalendar fetches all events for the current week.
	// Returns enriched events with revision detection, speech details, etc.
	ScrapeWeeklyCalendar(ctx context.Context) ([]domain.FFEvent, error)

	// ScrapeEventHistory fetches historical data points for a specific event.
	// Returns 12-24 months of Actual/Forecast/Previous data.
	ScrapeEventHistory(ctx context.Context, eventURL string) ([]domain.FFEventDetail, error)

	// ScrapeRevisions checks current events against stored data for revisions.
	// Returns newly detected revisions.
	ScrapeRevisions(ctx context.Context, events []domain.FFEvent) ([]domain.EventRevision, error)

	// HealthCheck verifies that the scraping endpoint is reachable.
	HealthCheck(ctx context.Context) error
}
