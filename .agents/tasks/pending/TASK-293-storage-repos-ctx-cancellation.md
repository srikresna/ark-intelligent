# TASK-293: Storage Repos — Honor context.Context untuk Cancellation (TECH-008)

**Priority:** medium
**Type:** tech-refactor
**Estimated:** S
**Area:** internal/adapter/storage/ (intraday_repo.go, event_repo.go, price_repo.go, signal_repo.go)
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB

## Deskripsi

Semua storage repos di `internal/adapter/storage/` menerima `context.Context` dalam signature tapi menggunakan **blank identifier `_`** — context sepenuhnya diabaikan:

```go
// Contoh pattern yang bermasalah (ada di semua 4 repo):
func (r *IntradayRepo) GetHistory(_ context.Context, ...) ([]domain.IntradayBar, error) {
    err := r.db.View(func(txn *badger.Txn) error { ... })
    // Jika caller cancel context → operasi BadgerDB TETAP berjalan sampai selesai
}
```

**Total: 20+ fungsi** di 4 repos yang ignore context. Jika goroutine caller sudah timeout atau cancelled, operasi DB tetap berjalan karena context tidak di-check.

**Fix sederhana:** Tambah `ctx.Err()` check di awal setiap fungsi untuk early return jika context sudah cancelled/timed out. Untuk operasi batch (loop), tambah `select ctx.Done()` check periodik.

**Ini berbeda dari TASK-098/TASK-217** — TASK-098 fix impact-recorder, TASK-217 fix handlers. Task ini fix **storage/persistence layer** yang menggunakan BadgerDB.

## Perubahan yang Diperlukan

### Pattern yang harus diterapkan ke semua fungsi

```go
// SEBELUM:
func (r *IntradayRepo) GetHistory(_ context.Context, ...) {
    err := r.db.View(func(txn *badger.Txn) error { ... })
    ...
}

// SESUDAH:
func (r *IntradayRepo) GetHistory(ctx context.Context, ...) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    err := r.db.View(func(txn *badger.Txn) error { ... })
    ...
}
```

### 1. `internal/adapter/storage/intraday_repo.go` — 4 fungsi

Fungsi yang perlu diubah:
- `SaveBars(_ context.Context, ...)` — tambah ctx check sebelum `wb := r.db.NewWriteBatch()`
- `GetHistory(_ context.Context, ...)` — tambah ctx check sebelum `r.db.View(...)`
- `GetLatest(_ context.Context, ...)` — tambah ctx check
- `PurgeOlderThan(_ context.Context, ...)` — tambah ctx check sebelum loop + tambah `select ctx.Done()` di dalam loop jika ada

### 2. `internal/adapter/storage/event_repo.go` — ~8 fungsi

Fungsi yang perlu diubah:
- `SaveEvents`, `GetEventsByDateRange`, `GetEventHistory`, `SaveEventDetails`
- `GetAllRevisions`, `SaveRevision`, `GetRevisions`, `GetEvent`
- `DeleteEventsByDate`, `CountEvents`

### 3. `internal/adapter/storage/price_repo.go` — ~5 fungsi

Fungsi yang perlu diubah:
- `SavePrices`, `GetLatest`, `GetHistory`, `GetPriceAt`

### 4. `internal/adapter/storage/signal_repo.go` — ~2 fungsi

Fungsi yang perlu diubah:
- `SaveSignals`, `GetSignalsByContract`

### Catatan BadgerDB

BadgerDB `View()` dan `Update()` tidak accept context langsung. Pattern yang tepat:

```go
func (r *EventRepo) GetEventsByDateRange(ctx context.Context, start, end time.Time) ([]domain.FFEvent, error) {
    // Early exit jika context sudah cancelled sebelum kita mulai
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    var events []domain.FFEvent
    err := r.db.View(func(txn *badger.Txn) error {
        // ... existing logic ...
        return nil
    })
    return events, err
}
```

Untuk batch writes yang punya loop panjang, tambah check di dalam loop:
```go
for _, item := range items {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    // ... process item ...
}
```

## File yang Harus Diubah

1. `internal/adapter/storage/intraday_repo.go` — ganti `_` dengan `ctx` di 4 fungsi, tambah ctx check
2. `internal/adapter/storage/event_repo.go` — ganti `_` dengan `ctx` di ~10 fungsi, tambah ctx check
3. `internal/adapter/storage/price_repo.go` — ganti `_` dengan `ctx` di ~5 fungsi, tambah ctx check
4. `internal/adapter/storage/signal_repo.go` — ganti `_` dengan `ctx` di ~2 fungsi, tambah ctx check

**Tidak perlu mengubah interface/ports** — signature sudah `context.Context`, hanya implementasinya yang menggunakan `_`.

## Verifikasi

```bash
go build ./...  # Harus clean
go vet ./...    # Tidak ada warning baru
# Cek manual: grep "_ context.Context" di storage/ harus 0 hasil
grep -r "_ context.Context" internal/adapter/storage/
# Expected: no output
```

## Acceptance Criteria

- [ ] `grep "_ context.Context" internal/adapter/storage/` menghasilkan 0 baris
- [ ] Setiap fungsi yang sebelumnya pakai `_` sekarang punya `if err := ctx.Err(); err != nil { return ... }` di awal
- [ ] Batch loops dengan WriteBatch punya `select ctx.Done()` check di dalam loop
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] Tidak ada behavior change (REFACTOR — bukan feature)

## Referensi

- `.agents/TECH_REFACTOR_PLAN.md` — TECH-008 context propagation
- `.agents/research/2026-04-02-03-tech-refactor-gaps-test-coverage-ctx-storage-putaran20.md` — Temuan 3
- `internal/adapter/storage/intraday_repo.go:38,65,108,120` — fungsi yang pakai `_`
- `internal/adapter/storage/event_repo.go:58,81,121,166,225,265,282,325,349,371` — fungsi yang pakai `_`
