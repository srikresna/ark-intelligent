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
	cacheMu         sync.RWMutex
	cacheExpiry     time.Time
)

// GetCachedOrFetch returns cached SentimentData if still within TTL,
// else fetches fresh data. Thread-safe.
func GetCachedOrFetch(ctx context.Context) (*SentimentData, error) {
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
