package ports

import (
	"context"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// NewsRepository defines the storage interface for Economic Calendar events.
type NewsRepository interface {
	SaveEvents(ctx context.Context, events []domain.NewsEvent) error
	GetByDate(ctx context.Context, date string) ([]domain.NewsEvent, error)
	GetByWeek(ctx context.Context, weekStart string) ([]domain.NewsEvent, error)
	// GetByMonth returns all events for a given month. yearMonth format: "202603"
	GetByMonth(ctx context.Context, yearMonth string) ([]domain.NewsEvent, error)
	GetPending(ctx context.Context, date string) ([]domain.NewsEvent, error)
	UpdateActual(ctx context.Context, id string, actual string) error
	UpdateStatus(ctx context.Context, id string, status string, retryCount int) error
}

// NewsFetcher defines the interface for fetching the economic calendar.
type NewsFetcher interface {
	ScrapeCalendar(ctx context.Context, week string) ([]domain.NewsEvent, error)
	ScrapeActuals(ctx context.Context, date string) ([]domain.NewsEvent, error)
	// ScrapeMonth fetches all events for a given month. monthType: "current" | "prev" | "next"
	ScrapeMonth(ctx context.Context, monthType string) ([]domain.NewsEvent, error)
}
