# TASK-115: Bounded LRU User Mutex in Middleware

**Priority:** high
**Type:** refactor
**Estimated:** M
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 23:00 WIB
**Siklus:** Refactor

## Deskripsi
Per-user mutexes di `sync.Map` pada middleware.go TIDAK PERNAH di-evict. Setiap user baru menambah entry yang persist selamanya. Untuk deployment jangka panjang, ini memory leak. Replace dengan bounded LRU map dengan TTL eviction.

## Konteks
- `middleware.go:27` — `userMu sync.Map` declaration
- `middleware.go:54-57` — `getUserMutex()` uses `LoadOrStore` tanpa eviction
- `middleware.go:435-437` — Comment explicitly mentions "intentionally no eviction" tapi ini masalah
- Ref: `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Replace `sync.Map` untuk userMu dengan bounded structure (options: LRU cache, atau sync.Map dengan periodic eviction goroutine)
- [ ] Max entries: configurable, default 10,000 (cukup untuk active users)
- [ ] TTL: 30 menit — user yang tidak aktif selama 30 menit, mutex-nya di-evict
- [ ] Eviction goroutine berjalan setiap 5 menit, membersihkan entries yang expired
- [ ] Goroutine harus bisa di-cancel saat shutdown (accept context)
- [ ] Tidak ada behavior change — per-user locking tetap bekerja identik
- [ ] Tambah log saat eviction berjalan (debug level)

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/middleware.go`

## Referensi
- `.agents/research/2026-04-01-23-tech-refactor-race-memory-resilience.md`
- `.agents/TECH_REFACTOR_PLAN.md`
