# TASK-074: Fix TOCTOU Race di Sentiment Cache dengan singleflight

**Priority:** low
**Type:** fix
**Estimated:** S
**Area:** internal/service/sentiment
**Created by:** Research Agent
**Created at:** 2026-04-01 03:00 WIB
**Siklus:** BugHunt

## Deskripsi
`GetCachedOrFetch()` di sentiment/cache.go memiliki TOCTOU (Time-of-Check-Time-of-Use) race antara `RUnlock()` dan `Lock()`. Beberapa goroutine concurrent bisa bersamaan mendeteksi cache expired dan memanggil `FetchSentiment()` N kali — membuang API calls dan berpotensi kena rate limit.

## Konteks
Pattern saat ini (dengan race):
```go
cacheMu.RLock()
if cachedSentiment != nil && time.Now().Before(cacheExpiry) {
    data := cachedSentiment
    cacheMu.RUnlock()
    return data, nil
}
cacheMu.RUnlock()
// <-- GAP: N goroutine bisa masuk sini!
data, err := FetchSentiment(ctx) // dipanggil N kali
```

Solusi direkomendasikan: gunakan `golang.org/x/sync/singleflight` (sudah ada di go.sum atau bisa pakai implementasi manual). Package ini memastikan hanya 1 in-flight fetch per key, goroutine lain menunggu hasilnya.

Alternatif lebih sederhana: ganti RLock check → full Lock check (turunkan performa sedikit tapi jauh lebih simpel).

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Concurrent calls ke `GetCachedOrFetch()` saat cache expired hanya menghasilkan 1 call ke `FetchSentiment()`
- [ ] Tidak ada data race (bisa verifikasi dengan `-race` flag jika ada test)

## File yang Kemungkinan Diubah
- `internal/service/sentiment/cache.go`
- `go.mod` (jika menambahkan singleflight dependency yang belum ada)

## Referensi
- `.agents/research/2026-04-01-03-bug-hunting-subprocess-tempfile-race.md` — Bug #5
