package ports

import (
	"context"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// ---------------------------------------------------------------------------
// COTFetcher — CFTC COT data acquisition
// ---------------------------------------------------------------------------

// COTFetcher defines the interface for fetching raw COT data from CFTC.
// Primary: Socrata Open Data API. Fallback: CSV download + parse.
type COTFetcher interface {
	// FetchLatest fetches the most recent COT data for all tracked contracts.
	// weeks specifies how many weeks of historical data to retrieve.
	FetchLatest(ctx context.Context, contracts []domain.COTContract, weeks int) ([]domain.COTRecord, error)

	// FetchByContract fetches COT data for a single contract.
	FetchByContract(ctx context.Context, contract domain.COTContract, weeks int) ([]domain.COTRecord, error)

	// HealthCheck verifies that the CFTC API is reachable.
	HealthCheck(ctx context.Context) error
}
