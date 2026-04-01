package sentiment

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

// openTestBadger opens a temporary BadgerDB instance for unit testing.
// The caller is responsible for closing it.
func openTestBadger(t *testing.T) *badger.DB {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "badger-sentiment-test")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	opts := badger.DefaultOptions(dir).
		WithLogger(nil). // suppress badger internal logs in tests
		WithSyncWrites(false)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("failed to open test BadgerDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// resetCacheState clears both in-memory and global badgerDB for test isolation.
func resetCacheState() {
	cacheMu.Lock()
	cachedSentiment = nil
	cacheExpiry = time.Time{}
	badgerDB = nil
	cacheMu.Unlock()
}

// ---------------------------------------------------------------------------
// InitSentimentCache
// ---------------------------------------------------------------------------

func TestInitSentimentCache_NilDB(t *testing.T) {
	resetCacheState()
	// Should not panic when nil is passed.
	InitSentimentCache(nil)
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if badgerDB != nil {
		t.Error("expected badgerDB to be nil after InitSentimentCache(nil)")
	}
}

func TestInitSentimentCache_WithDB(t *testing.T) {
	resetCacheState()
	db := openTestBadger(t)
	InitSentimentCache(db)
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if badgerDB == nil {
		t.Error("expected badgerDB to be set after InitSentimentCache(db)")
	}
	resetCacheState()
}

// ---------------------------------------------------------------------------
// loadFromBadger / saveToBadger
// ---------------------------------------------------------------------------

func TestLoadFromBadger_EmptyDB(t *testing.T) {
	db := openTestBadger(t)
	result := loadFromBadger(db)
	if result != nil {
		t.Error("expected nil from empty BadgerDB")
	}
}

func TestSaveThenLoadFromBadger_RoundTrip(t *testing.T) {
	db := openTestBadger(t)

	original := &SentimentData{
		CNNFearGreed:    65.5,
		CNNAvailable:    true,
		AAIIBullish:     42.0,
		AAIIBearish:     28.5,
		AAIINeutral:     29.5,
		AAIIAvailable:   true,
		PutCallTotal:    0.95,
		PutCallAvailable: true,
		FetchedAt:       time.Now().Round(time.Second), // round to avoid nanosecond JSON precision issues
	}

	// Save synchronously (saveToBadger is normally async).
	saveToBadger(db, original)

	loaded := loadFromBadger(db)
	if loaded == nil {
		t.Fatal("expected non-nil data after save+load")
	}

	if loaded.CNNFearGreed != original.CNNFearGreed {
		t.Errorf("CNNFearGreed: got %.2f, want %.2f", loaded.CNNFearGreed, original.CNNFearGreed)
	}
	if !loaded.CNNAvailable {
		t.Error("CNNAvailable should be true after round-trip")
	}
	if loaded.AAIIBullish != original.AAIIBullish {
		t.Errorf("AAIIBullish: got %.2f, want %.2f", loaded.AAIIBullish, original.AAIIBullish)
	}
	if loaded.PutCallTotal != original.PutCallTotal {
		t.Errorf("PutCallTotal: got %.2f, want %.2f", loaded.PutCallTotal, original.PutCallTotal)
	}
}

func TestLoadFromBadger_ExpiredEntry(t *testing.T) {
	db := openTestBadger(t)

	// Write an entry manually with an already-expired application TTL.
	entry := badgerEntry{
		Data: &SentimentData{
			CNNFearGreed: 50.0,
			CNNAvailable: true,
			FetchedAt:    time.Now().Add(-7 * time.Hour), // older than cacheTTL
		},
		ExpiresAt: time.Now().Add(-1 * time.Second), // expired
	}
	writeTestEntry(t, db, entry)

	loaded := loadFromBadger(db)
	if loaded != nil {
		t.Error("expected nil for expired BadgerDB entry")
	}
}

// writeTestEntry writes a badgerEntry directly to BadgerDB for testing.
func writeTestEntry(t *testing.T, db *badger.DB, entry badgerEntry) {
	t.Helper()
	import_json, err := json.Marshal(&entry)
	if err != nil {
		t.Fatalf("writeTestEntry marshal: %v", err)
	}
	err = db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(badgerCacheKey), import_json)
	})
	if err != nil {
		t.Fatalf("writeTestEntry: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetCachedOrFetch — BadgerDB load path (no live fetch)
// ---------------------------------------------------------------------------

func TestGetCachedOrFetch_LoadsFromBadger_WhenMemoryEmpty(t *testing.T) {
	resetCacheState()
	db := openTestBadger(t)

	// Pre-populate BadgerDB with fresh data.
	preloaded := &SentimentData{
		CNNFearGreed: 33.0,
		CNNAvailable: true,
		FetchedAt:    time.Now(),
	}
	saveToBadger(db, preloaded)

	// Inject DB and ensure memory cache is empty.
	InitSentimentCache(db)
	InvalidateCache()

	ctx := context.Background()
	got, err := GetCachedOrFetch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.CNNFearGreed != 33.0 {
		t.Errorf("CNNFearGreed: got %.2f, want 33.0", got.CNNFearGreed)
	}

	resetCacheState()
}

// ---------------------------------------------------------------------------
// GetCachedOrFetch — in-memory fast path still works with DB injected
// ---------------------------------------------------------------------------

func TestGetCachedOrFetch_ReturnsCachedMemory_WithDBInjected(t *testing.T) {
	resetCacheState()
	db := openTestBadger(t)
	InitSentimentCache(db)

	want := &SentimentData{
		CNNFearGreed: 77.0,
		CNNAvailable: true,
		FetchedAt:    time.Now(),
	}
	cacheMu.Lock()
	cachedSentiment = want
	cacheExpiry = time.Now().Add(1 * time.Hour)
	cacheMu.Unlock()

	ctx := context.Background()
	got, err := GetCachedOrFetch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Error("expected in-memory cached pointer to be returned")
	}

	resetCacheState()
}

// ---------------------------------------------------------------------------
// InvalidateCache — only clears memory, does not purge BadgerDB
// ---------------------------------------------------------------------------

func TestInvalidateCache_PreservesBadgerEntry(t *testing.T) {
	resetCacheState()
	db := openTestBadger(t)
	InitSentimentCache(db)

	original := &SentimentData{
		CNNFearGreed: 55.0,
		CNNAvailable: true,
		FetchedAt:    time.Now(),
	}
	saveToBadger(db, original)

	// Populate in-memory then invalidate.
	cacheMu.Lock()
	cachedSentiment = original
	cacheExpiry = time.Now().Add(1 * time.Hour)
	cacheMu.Unlock()

	InvalidateCache()

	// Memory should be cleared.
	if CacheAge() != -1 {
		t.Error("expected CacheAge=-1 after invalidate")
	}

	// BadgerDB entry should still be readable.
	loaded := loadFromBadger(db)
	if loaded == nil {
		t.Error("expected BadgerDB entry to survive InvalidateCache")
	}

	resetCacheState()
}

// ---------------------------------------------------------------------------
// Backward compatibility — nil DB falls back to in-memory only
// ---------------------------------------------------------------------------

func TestGetCachedOrFetch_NilDB_FallbackToMemory(t *testing.T) {
	resetCacheState()
	// No InitSentimentCache call — badgerDB remains nil.

	want := &SentimentData{
		CNNFearGreed: 42.0,
		CNNAvailable: true,
		FetchedAt:    time.Now(),
	}
	cacheMu.Lock()
	cachedSentiment = want
	cacheExpiry = time.Now().Add(1 * time.Hour)
	cacheMu.Unlock()

	ctx := context.Background()
	got, err := GetCachedOrFetch(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Error("expected in-memory pointer with nil DB")
	}

	resetCacheState()
}
