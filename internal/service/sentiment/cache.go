package sentiment

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// cacheTTL controls how long sentiment data stays valid before a re-fetch.
// AAII updates weekly; CNN F&G updates daily. 6 hours is a safe balance.
const cacheTTL = 6 * time.Hour

var (
	cachedSentiment *SentimentData
	cacheMu         sync.RWMutex
	cacheExpiry     time.Time

	// fetchGroup deduplicates concurrent cache-miss fetches so only one
	// goroutine calls FetchSentiment while others wait for the same result.
	fetchGroup singleflight.Group
)

// GetCachedOrFetch returns cached SentimentData if still within TTL,
// else fetches fresh data. Thread-safe; concurrent cache-miss callers are
// coalesced via singleflight so only one FetchSentiment call is in-flight.
func GetCachedOrFetch(ctx context.Context) (*SentimentData, error) {
	// Fast path: check cache under read lock.
	cacheMu.RLock()
	if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
		data := cachedSentiment
		cacheMu.RUnlock()
		return data, nil
	}
	cacheMu.RUnlock()

	// Slow path: singleflight ensures only one in-flight fetch.
	v, err, _ := fetchGroup.Do("sentiment", func() (any, error) {
		// Re-check under read lock — another caller in the group may have
		// already populated the cache before this function ran.
		cacheMu.RLock()
		if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
			data := cachedSentiment
			cacheMu.RUnlock()
			return data, nil
		}
		cacheMu.RUnlock()

		data, err := FetchSentiment(ctx)
		if err != nil {
			return nil, err
		}

		cacheMu.Lock()
		cachedSentiment = data
		cacheExpiry = time.Now().Add(cacheTTL)
		cacheMu.Unlock()

		return data, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*SentimentData), nil
}

// InvalidateCache forces the next call to GetCachedOrFetch to re-fetch.
func InvalidateCache() {
	cacheMu.Lock()
	cachedSentiment = nil
	cacheExpiry = time.Time{}
	cacheMu.Unlock()
}

// CacheAge returns how old the current cached data is, or -1 if no cache exists.
func CacheAge() time.Duration {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if cachedSentiment == nil {
		return -1
	}
	return time.Since(cachedSentiment.FetchedAt)
}
