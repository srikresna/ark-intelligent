# TASK-295: Fix Sentiment Cache Stampede — Singleflight Pattern

**Priority:** medium
**Type:** bug-fix
**Estimated:** S
**Area:** internal/service/sentiment/cache.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

`GetCachedOrFetch()` di `internal/service/sentiment/cache.go` memiliki **cache stampede** (thundering herd) bug:

Ketika cache expired, multiple goroutines yang concurrently hit miss semua memanggil `FetchSentiment()` secara paralel sebelum lock di-acquire kembali. Double-check setelah fetch memang mencegah data corruption, tapi **N goroutines tetap semua memanggil API** (CNN F&G, AAII Firecrawl, CBOE, Crypto F&G) secara bersamaan.

Ini menyebabkan:
1. N × 4 concurrent HTTP calls saat cache expired
2. Potensi rate-limiting di CNN/CBOE endpoints
3. Firecrawl credits terbuang (AAII scraping)

```go
// BUG: Unlock terjadi SEBELUM fetch, bukan sesudahnya
cacheMu.Unlock()                      // ← race window terbuka
data, err := FetchSentiment(ctx)      // ← N goroutines semua masuk sini
...
cacheMu.Lock()
// double-check: hanya satu yang "menang" update cache
```

## Perubahan yang Diperlukan

### Solusi: `golang.org/x/sync/singleflight`

```go
import "golang.org/x/sync/singleflight"

var (
    cachedSentiment *SentimentData
    cacheMu         sync.RWMutex  // upgrade ke RWMutex untuk fast path read
    cacheExpiry     time.Time
    sfGroup         singleflight.Group  // deduplicate concurrent misses
)

func GetCachedOrFetch(ctx context.Context) (*SentimentData, error) {
    // Fast path: RLock untuk concurrent reads
    cacheMu.RLock()
    if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
        data := cachedSentiment
        cacheMu.RUnlock()
        return data, nil
    }
    cacheMu.RUnlock()

    // Slow path: singleflight deduplicate concurrent misses
    v, err, _ := sfGroup.Do("sentiment", func() (interface{}, error) {
        // Double-check under write lock
        cacheMu.Lock()
        if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
            data := cachedSentiment
            cacheMu.Unlock()
            return data, nil
        }
        cacheMu.Unlock()

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
```

Jika `golang.org/x/sync` tidak tersedia, alternatif sederhana: gunakan `sync.Mutex` + flag `fetching bool` untuk serialisasi.

## File yang Harus Diubah

1. `internal/service/sentiment/cache.go` — refactor `GetCachedOrFetch`, upgrade ke `sync.RWMutex`
2. `go.mod` / `go.sum` — tambahkan `golang.org/x/sync` jika belum ada (cek dulu: `grep "golang.org/x/sync" go.mod`)

## Verifikasi

```bash
# Cek apakah x/sync sudah ada
grep "golang.org/x/sync" go.mod

# Build clean
go build ./...

# Run existing tests
go test ./internal/service/sentiment/...
```

## Acceptance Criteria

- [ ] `GetCachedOrFetch()` hanya memanggil `FetchSentiment()` sekali untuk concurrent cache misses
- [ ] Fast path (cache hit) menggunakan RLock sehingga concurrent reads tidak block satu sama lain
- [ ] `InvalidateCache()` dan `CacheAge()` tetap thread-safe
- [ ] `go build ./...` clean
- [ ] Tidak ada perubahan behavior untuk caller (return type sama, error handling sama)

## Referensi

- `.agents/research/2026-04-02-10-codebase-bug-analysis-putaran21.md` — BUG-1
- `internal/service/sentiment/cache.go:29-58` — fungsi yang diubah
- `internal/service/sentiment/sentiment.go` — FetchSentiment() yang dipanggil
