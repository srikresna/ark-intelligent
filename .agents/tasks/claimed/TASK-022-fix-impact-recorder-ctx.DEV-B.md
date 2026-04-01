# TASK-022: Fix RecordImpact goroutine menggunakan ctx request

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/news
**Created by:** Research Agent
**Created at:** 2026-04-01 14:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `internal/service/news/scheduler.go:676`, goroutine `RecordImpact` dipanggil dengan `ctx` dari loop scheduler:

```go
go func() {
    ...
    s.impactRecorder.RecordImpact(ctx, ev, ...) // ctx dari loop scheduler
}()
```

Jika ctx scheduler di-cancel (restart loop, shutdown), impact records untuk horizons di masa lalu tidak akan tersimpan karena operasi DB akan langsung return error.

Untuk path async (future horizons), `delayedRecord` sudah benar menggunakan `context.Background()` — tapi path sinkron (past horizons) masih terpengaruh.

## Konteks
Impact records adalah data penting untuk calibrasi AI signal accuracy. Kehilangan records ini bisa menyebabkan degradasi model accuracy tracking.

Ref: `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A4)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Goroutine yang memanggil `RecordImpact` menggunakan `context.Background()` (bukan ctx loop)

## File yang Kemungkinan Diubah
- `internal/service/news/scheduler.go`

## Referensi
- `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A4)
