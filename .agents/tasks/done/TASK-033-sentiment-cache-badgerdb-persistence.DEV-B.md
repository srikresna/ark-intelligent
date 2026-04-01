# TASK-033: Sentiment Service Cache Persistence ke BadgerDB

**Priority:** medium
**Type:** reliability
**Estimated:** S
**Area:** internal/service/sentiment
**Created by:** Research Agent
**Created at:** 2026-04-01 15:xx WIB
**Siklus:** Data (Siklus 2 Putaran 2)

## Deskripsi
Upgrade sentiment cache dari pure in-memory ke BadgerDB-backed persistence.
Saat ini setiap bot restart memaksa 3 Firecrawl API calls (AAII + CBOE P/C + CNN).
Firecrawl berbayar — minimasi unnecessary calls penting untuk efisiensi biaya.

## Konteks
File `internal/service/sentiment/cache.go` menggunakan global in-memory variables:
```go
var cachedSentiment *SentimentData  // hilang saat restart
var cacheMu         sync.RWMutex
var cacheExpiry     time.Time
```

BadgerDB sudah tersedia dan dipakai di project:
- `internal/adapter/storage/cache_repo.go` — AI response cache
- `internal/service/fred/persistence.go` — FRED data snapshots

Pattern yang diikuti: simpan JSON-serialized SentimentData ke BadgerDB dengan key
`sentiment:latest` dan TTL 6 jam. Saat startup: check BadgerDB dulu, baru Firecrawl.

**Kalkulasi penghematan:**
- Restart tanpa BadgerDB: 3 Firecrawl calls (expensive)
- Restart dengan BadgerDB: 0 Firecrawl calls jika cache masih < 6 jam
- Bot restart bisa terjadi saat deploy, crash, sistem update — setiap kali buang quota

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Modifikasi `internal/service/sentiment/cache.go`:
  - Tambah fungsi `InitSentimentCache(db *badger.DB)` untuk inject DB dependency
  - Modifikasi `GetCachedOrFetch()`: cek BadgerDB sebelum fetch (fallback ke in-memory jika DB nil)
  - Setelah fetch sukses: simpan ke BadgerDB dengan TTL 6 jam (`badger.NewEntry(key, val).WithTTL(cacheTTL)`)
  - Key: `sentiment:v1:latest` (versi dalam key agar mudah invalidasi saat struct berubah)
- [ ] Backward compatible: jika `InitSentimentCache` tidak dipanggil (DB nil), fallback ke in-memory behavior lama — tidak breaking
- [ ] `cmd/bot/main.go` memanggil `sentiment.InitSentimentCache(db)` setelah DB init
- [ ] Unit test: `cache_test.go` — test bahwa data di-load dari BadgerDB saat memory cache kosong
- [ ] Log saat load dari disk vs fetch fresh (debug level)

## File yang Kemungkinan Diubah
- `internal/service/sentiment/cache.go` (utama — refactor cache logic)
- `internal/service/sentiment/cache_test.go` (baru — unit test)
- `cmd/bot/main.go` (inisialisasi DB injection)

## Referensi
- `.agents/research/2026-04-01-15-data-integrasi-siklus2-putaran2.md` (GAP 3)
- `internal/service/fred/persistence.go` (BadgerDB write pattern)
- `internal/service/ai/cached_interpreter.go` (BadgerDB-backed cache wrapper pattern)
- `internal/adapter/storage/cache_repo.go` (TTL-based BadgerDB entry pattern)
- `internal/service/sentiment/cache.go` (file yang akan dimodifikasi)
