package vix

import (
	"context"
	"sync"
	"time"
)

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
		return &VIXTermStructure{
			Available: false,
			Error:     err.Error(),
			AsOf:      time.Now().UTC(),
		}, nil // graceful fallback
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
