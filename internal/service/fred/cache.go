package fred

import (
	"context"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

// defaultTTL is how long fetched FRED data stays valid before a re-fetch.
// FRED daily series update intraday; monthly series (PCE, CPI) update less often.
// 1 hour is a safe balance between freshness and rate-limit protection.
const defaultTTL = 1 * time.Hour

var cacheLog = logger.Component("fred-cache")

type cachedMacroData struct {
	data      *MacroData
	fetchedAt time.Time
}

var (
	globalCache    *cachedMacroData
	cacheMu        sync.RWMutex
	cacheTTL       = defaultTTL //nolint:gochecknoglobals
	postFetchHook  func(context.Context, *MacroData) //nolint:gochecknoglobals
	postFetchHookMu sync.RWMutex                      //nolint:gochecknoglobals
)

// SetPostFetchHook registers a callback that is invoked after every fresh
// FRED data fetch (not on cache hits). Use this to persist snapshots.
func SetPostFetchHook(fn func(context.Context, *MacroData)) {
	postFetchHookMu.Lock()
	postFetchHook = fn
	postFetchHookMu.Unlock()
}

func invokePostFetchHook(ctx context.Context, data *MacroData) {
	postFetchHookMu.RLock()
	fn := postFetchHook
	postFetchHookMu.RUnlock()
	if fn != nil {
		fn(ctx, data)
	}
}

// CacheResult wraps MacroData with metadata about whether it came from cache.
type CacheResult struct {
	Data      *MacroData
	FromCache bool
	CacheAge  time.Duration // How old the cached data is; 0 if freshly fetched
}

// GetCachedOrFetch returns cached MacroData if still within TTL, else fetches fresh data.
// Thread-safe. Use this in all handlers instead of FetchMacroData directly.
func GetCachedOrFetch(ctx context.Context) (*MacroData, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.fetchedAt) < cacheTTL {
		data := globalCache.data
		age := time.Since(globalCache.fetchedAt)
		cacheMu.RUnlock()
		cacheLog.Debug().Dur("age", age).Msg("returning cached data")
		return data, nil
	}
	cacheMu.RUnlock()

	// Fetch fresh data
	data, err := FetchMacroData(ctx)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	globalCache = &cachedMacroData{data: data, fetchedAt: time.Now()}
	cacheMu.Unlock()

	cacheLog.Debug().Msg("fetched fresh data from FRED API")
	invokePostFetchHook(ctx, data)

	return data, nil
}

// GetCachedOrFetchWithMeta is like GetCachedOrFetch but also returns cache metadata.
func GetCachedOrFetchWithMeta(ctx context.Context) (*CacheResult, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.fetchedAt) < cacheTTL {
		result := &CacheResult{
			Data:      globalCache.data,
			FromCache: true,
			CacheAge:  time.Since(globalCache.fetchedAt),
		}
		cacheMu.RUnlock()
		cacheLog.Debug().Dur("age", result.CacheAge).Msg("returning cached data")
		return result, nil
	}
	cacheMu.RUnlock()

	data, err := FetchMacroData(ctx)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	globalCache = &cachedMacroData{data: data, fetchedAt: time.Now()}
	cacheMu.Unlock()

	cacheLog.Debug().Msg("fetched fresh data from FRED API")
	invokePostFetchHook(ctx, data)

	return &CacheResult{Data: data, FromCache: false, CacheAge: 0}, nil
}

// InvalidateCache forces the next call to GetCachedOrFetch to re-fetch from FRED.
// Call this when the user explicitly requests a refresh.
func InvalidateCache() {
	cacheMu.Lock()
	globalCache = nil
	cacheMu.Unlock()
}

// CacheAge returns how old the current cached data is, or -1 if no cache exists.
func CacheAge() time.Duration {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if globalCache == nil {
		return -1
	}
	return time.Since(globalCache.fetchedAt)
}
