package vix

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("vix-cache")

const cacheTTL = 6 * time.Hour

// Cache provides a simple in-memory TTL cache for VIX term structure data.
// CBOE updates end-of-day, so 6-hour TTL is appropriate.
type Cache struct {
	mu        sync.RWMutex
	data      *VIXTermStructure
	fetchedAt time.Time
}

// NewCache creates a new VIX data cache.
func NewCache() *Cache {
	return &Cache{}
}

// Get returns the cached VIX term structure if it's still fresh,
// or fetches a new one from CBOE.
func (c *Cache) Get(ctx context.Context) (*VIXTermStructure, error) {
	c.mu.RLock()
	if c.data != nil && time.Since(c.fetchedAt) < cacheTTL {
		data := c.data
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	// Fetch fresh data
	ts, err := FetchTermStructure(ctx)
	if err != nil {
		// If we have stale cached data, return it with a warning instead of failing.
		c.mu.RLock()
		stale := c.data
		c.mu.RUnlock()
		if stale != nil {
			log.Warn().Err(err).
				Time("stale_since", c.fetchedAt).
				Msg("fetch failed, returning stale cache")
			return stale, nil
		}
		// No stale data — propagate the error to the caller.
		return nil, fmt.Errorf("vix cache: fetch term structure: %w", err)
	}

	if ts.Available {
		c.mu.Lock()
		c.data = ts
		c.fetchedAt = time.Now()
		c.mu.Unlock()
	}

	return ts, nil
}

// Invalidate clears the cache, forcing a fresh fetch on next Get.
func (c *Cache) Invalidate() {
	c.mu.Lock()
	c.data = nil
	c.mu.Unlock()
}
