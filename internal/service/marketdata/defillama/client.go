// Package defillama provides a client for the DeFiLlama TVL API.
// No API key required. Data is updated daily.
// Endpoint: https://api.llama.fi/v2/historicalChainTvl
package defillama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/rs/zerolog/log"
)

const (
	historicalTVLURL = "https://api.llama.fi/v2/historicalChainTvl"
	httpTimeout      = 15 * time.Second
	cacheTTL         = 6 * time.Hour
)

// tvlPoint represents a single data point from the API.
type tvlPoint struct {
	Date int64   `json:"date"`
	TVL  float64 `json:"tvl"`
}

// TVLSummary holds computed DeFi TVL metrics.
type TVLSummary struct {
	Current   float64   // Latest TVL in USD
	Change7D  float64   // Percentage change over last 7 days
	Change30D float64   // Percentage change over last 30 days
	Trend     string    // "EXPANDING" | "CONTRACTING" | "STABLE"
	FetchedAt time.Time // When data was fetched
	Available bool      // Whether data was successfully retrieved
}

// package-level cache (protected by cacheMu).
var (
	globalCache *TVLSummary    //nolint:gochecknoglobals
	cacheMu     sync.RWMutex   //nolint:gochecknoglobals
	httpClient  = httpclient.NewClient(httpTimeout) //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached TVL data if within TTL, otherwise fetches
// fresh data from DeFiLlama. Gracefully degrades: if fetch fails and a stale
// cache exists, the stale data is returned rather than nil.
func GetCachedOrFetch(ctx context.Context) *TVLSummary {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.FetchedAt) < cacheTTL {
		data := globalCache
		cacheMu.RUnlock()
		return data
	}
	cacheMu.RUnlock()

	// Fetch fresh data.
	fresh := fetchHistoricalTVL(ctx)

	cacheMu.Lock()
	if fresh.Available {
		globalCache = fresh
	} else if globalCache != nil {
		// Return stale cache on failure instead of empty summary.
		stale := globalCache
		cacheMu.Unlock()
		log.Warn().Msg("defillama: fetch failed, returning stale cache")
		return stale
	}
	cacheMu.Unlock()

	return fresh
}

// fetchHistoricalTVL fetches total DeFi TVL history and computes summary metrics.
func fetchHistoricalTVL(ctx context.Context) *TVLSummary {
	result := &TVLSummary{FetchedAt: time.Now()}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, historicalTVLURL, nil)
	if err != nil {
		log.Warn().Err(err).Msg("defillama: failed to create request")
		return result
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("defillama: failed to fetch TVL data")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Msg("defillama: non-200 response")
		return result
	}

	var points []tvlPoint
	if err := json.NewDecoder(resp.Body).Decode(&points); err != nil {
		log.Warn().Err(err).Msg("defillama: failed to decode response")
		return result
	}

	if len(points) < 2 {
		log.Warn().Int("points", len(points)).Msg("defillama: insufficient data points")
		return result
	}

	// Use the last 31 entries (30-day window).
	start := 0
	if len(points) > 31 {
		start = len(points) - 31
	}
	recent := points[start:]
	latest := recent[len(recent)-1]

	result.Current = latest.TVL
	result.Available = true

	// Compute 7-day change.
	if len(recent) > 7 {
		p7d := recent[len(recent)-8] // ~7 days ago
		if p7d.TVL > 0 {
			result.Change7D = (latest.TVL - p7d.TVL) / p7d.TVL * 100
		}
	}

	// Compute 30-day change.
	if len(recent) > 1 {
		p30d := recent[0] // ~30 days ago
		if p30d.TVL > 0 {
			result.Change30D = (latest.TVL - p30d.TVL) / p30d.TVL * 100
		}
	}

	// Classify trend based on 7-day change.
	switch {
	case result.Change7D > 5:
		result.Trend = "EXPANDING"
	case result.Change7D < -5:
		result.Trend = "CONTRACTING"
	default:
		result.Trend = "STABLE"
	}

	log.Info().
		Float64("tvl_billion", latest.TVL/1e9).
		Float64("change_7d", result.Change7D).
		Float64("change_30d", result.Change30D).
		Str("trend", result.Trend).
		Msg("defillama: TVL data fetched")

	return result
}

// FormatTVLBillions formats TVL as a human-readable string in billions.
func FormatTVLBillions(tvl float64) string {
	return fmt.Sprintf("$%.1fB", tvl/1e9)
}
