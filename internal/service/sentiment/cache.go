package sentiment

import (
	"context"
	"sync"
	"time"
)

// cacheTTL controls how long sentiment data stays valid before a re-fetch.
// AAII updates weekly; CNN F&G updates daily. 6 hours is a safe balance.
const cacheTTL = 6 * time.Hour

var (
	cachedSentiment *SentimentData
	cacheMu         sync.Mutex // full Mutex prevents TOCTOU race on cache miss
	cacheExpiry     time.Time
)

// GetCachedOrFetch returns cached SentimentData if still within TTL,
// else fetches fresh data. Thread-safe; only one in-flight fetch at a time
// (prevents duplicate API calls when multiple goroutines hit an expired cache).
func GetCachedOrFetch(ctx context.Context) (*SentimentData, error) {
	cacheMu.Lock()

	// Fast path: cache hit
	if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
		data := cachedSentiment
		cacheMu.Unlock()
		return data, nil
	}

	// Slow path: fetch while holding lock (serializes concurrent misses)
	cacheMu.Unlock()

	data, err := FetchSentiment(ctx)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	// Double-check: another goroutine may have fetched while we were fetching
	if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
		data = cachedSentiment
	} else {
		cachedSentiment = data
		cacheExpiry = time.Now().Add(cacheTTL)
	}
	cacheMu.Unlock()

	return data, nil
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
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedSentiment == nil {
		return -1
	}
	return time.Since(cachedSentiment.FetchedAt)
}
