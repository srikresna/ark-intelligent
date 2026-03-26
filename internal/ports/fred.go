package ports

import (
	"context"
	"time"
)

// FREDObservation represents a single persisted FRED data point.
type FREDObservation struct {
	SeriesID string
	Date     time.Time
	Value    float64
}

// FREDRepository defines persistence for FRED macro data snapshots.
type FREDRepository interface {
	// SaveSnapshot persists a single FRED observation.
	SaveSnapshot(ctx context.Context, seriesID string, date time.Time, value float64) error

	// SaveSnapshots persists multiple observations in a batch.
	SaveSnapshots(ctx context.Context, observations []FREDObservation) error

	// GetHistory returns the last N days of observations for a series.
	GetHistory(ctx context.Context, seriesID string, days int) ([]FREDObservation, error)

	// GetLatest returns the most recent observation for a series.
	GetLatest(ctx context.Context, seriesID string) (*FREDObservation, error)
}
