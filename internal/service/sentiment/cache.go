package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"golang.org/x/sync/singleflight"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

// cacheTTL controls how long sentiment data stays valid before a re-fetch.
// AAII updates weekly; CNN F&G updates daily. 6 hours is a safe balance.
const cacheTTL = 6 * time.Hour

// badgerCacheKey is the BadgerDB key for the persisted sentiment snapshot.
// The version suffix ("v1") makes it easy to invalidate on struct changes.
const badgerCacheKey = "sentiment:v1:latest"

var cacheLog = logger.Component("sentiment-cache")

var (
	cachedSentiment *SentimentData
	cacheMu         sync.RWMutex
	cacheExpiry     time.Time

	// badgerDB is the optional BadgerDB instance injected via InitSentimentCache.
	// If nil, the cache falls back to pure in-memory behavior.
	badgerDB *badger.DB

	// fetchGroup deduplicates concurrent cache-miss fetches so only one
	// goroutine calls FetchSentiment while others wait for the same result.
	fetchGroup singleflight.Group
)

// InitSentimentCache injects a BadgerDB instance for persistence.
// Call this once during application startup, after the DB is opened.
// If db is nil, the cache operates in pure in-memory mode (no change from
// prior behavior — fully backward compatible).
func InitSentimentCache(db *badger.DB) {
	cacheMu.Lock()
	badgerDB = db
	cacheMu.Unlock()
	cacheLog.Debug().Bool("persistence", db != nil).Msg("sentiment cache initialized")
}

// GetCachedOrFetch returns cached SentimentData if still within TTL,
// else fetches fresh data. Thread-safe; concurrent cache-miss callers are
// coalesced via singleflight so only one FetchSentiment call is in-flight.
//
// Load order:
//  1. In-memory cache (fastest)
//  2. BadgerDB on-disk cache (survives restarts)
//  3. Live fetch from all sources (expensive — Firecrawl calls)
func GetCachedOrFetch(ctx context.Context) (*SentimentData, error) {
	// Fast path: check in-memory cache under read lock.
	cacheMu.RLock()
	if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
		data := cachedSentiment
		cacheMu.RUnlock()
		return data, nil
	}
	cacheMu.RUnlock()

	// Slow path: singleflight ensures only one in-flight fetch.
	v, err, _ := fetchGroup.Do("sentiment", func() (any, error) {
		// Re-check in-memory under read lock — another caller may have
		// already populated it between the fast-path check and here.
		cacheMu.RLock()
		if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
			data := cachedSentiment
			cacheMu.RUnlock()
			return data, nil
		}
		db := badgerDB
		cacheMu.RUnlock()

		// Try BadgerDB before making expensive live fetches.
		if db != nil {
			if data := loadFromBadger(db); data != nil {
				cacheLog.Debug().Msg("sentiment: loaded from BadgerDB (disk hit)")
				cacheMu.Lock()
				cachedSentiment = data
				cacheExpiry = data.FetchedAt.Add(cacheTTL)
				cacheMu.Unlock()
				return data, nil
			}
		}

		// Cache miss — fetch fresh data from all sources.
		cacheLog.Debug().Msg("sentiment: cache miss, fetching fresh data")
		data, err := FetchSentiment(ctx)
		if err != nil {
			return nil, err
		}

		// Persist to in-memory cache.
		cacheMu.Lock()
		cachedSentiment = data
		cacheExpiry = time.Now().Add(cacheTTL)
		cacheMu.Unlock()

		// Persist to BadgerDB asynchronously (don't block callers on disk I/O).
		if db != nil {
			go saveToBadger(db, data)
		}

		return data, nil
	})
	if err != nil {
		return nil, err
	}
	data, ok := v.(*SentimentData)
	if !ok {
		return nil, fmt.Errorf("unexpected type from singleflight: %T", v)
	}
	return data, nil
}

// InvalidateCache forces the next call to GetCachedOrFetch to re-fetch.
// It clears in-memory state but does NOT purge the BadgerDB entry —
// BadgerDB entries expire naturally via TTL.
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

// ---------------------------------------------------------------------------
// BadgerDB helpers
// ---------------------------------------------------------------------------

// badgerEntry is the on-disk envelope wrapping SentimentData.
type badgerEntry struct {
	Data      *SentimentData `json:"data"`
	ExpiresAt time.Time      `json:"expires_at"`
}

// loadFromBadger reads a SentimentData snapshot from BadgerDB.
// Returns nil if the entry is missing, expired, or malformed.
func loadFromBadger(db *badger.DB) *SentimentData {
	var entry badgerEntry
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(badgerCacheKey))
		if err != nil {
			return err // badger.ErrKeyNotFound is expected on first run
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry)
		})
	})
	if err != nil {
		if err != badger.ErrKeyNotFound {
			cacheLog.Debug().Err(err).Msg("sentiment: BadgerDB read error (non-fatal)")
		}
		return nil
	}

	// Double-check application-level TTL (BadgerDB TTL already handles eviction,
	// but we guard against clock skew or entries written without TTL).
	if time.Now().After(entry.ExpiresAt) {
		cacheLog.Debug().Msg("sentiment: BadgerDB entry expired (application TTL)")
		return nil
	}

	if entry.Data == nil {
		return nil
	}
	return entry.Data
}

// saveToBadger persists SentimentData to BadgerDB with a TTL matching cacheTTL.
// Errors are logged but never returned — persistence is best-effort.
func saveToBadger(db *badger.DB, data *SentimentData) {
	entry := badgerEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(cacheTTL),
	}
	val, err := json.Marshal(&entry)
	if err != nil {
		cacheLog.Warn().Err(err).Msg("sentiment: failed to marshal for BadgerDB (skipped)")
		return
	}

	err = db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(badgerCacheKey), val).WithTTL(cacheTTL)
		return txn.SetEntry(e)
	})
	if err != nil {
		cacheLog.Warn().Err(err).Msg("sentiment: failed to save to BadgerDB (non-fatal)")
		return
	}
	cacheLog.Debug().Msg("sentiment: saved to BadgerDB")
}
