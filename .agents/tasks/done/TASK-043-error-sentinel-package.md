# TASK-043: Error Handling — Sentinel Errors & Consistent Wrapping

**Type:** refactor  
**Priority:** MEDIUM  
**Effort:** M (3-4h)  
**Phase:** Tech Refactor Phase 4 (TECH-007)  
**Assignee:** unassigned

---

## Problem

Error handling di seluruh codebase tidak konsisten:
```go
// Pattern 1 — zerolog only:
log.Error().Err(err).Msg("cot fetch failed")
return err

// Pattern 2 — fmt.Errorf wrap:
return nil, fmt.Errorf("cot: fetch: %w", err)

// Pattern 3 — bare return:
return nil, err

// Pattern 4 — silent drop:
if err != nil { return } // error discarded
```

Tidak ada sentinel errors → `errors.Is()` tidak bisa dipakai untuk distinguish error types.
Handler tidak tahu bedanya "no data" vs "rate limited" vs "parse failed".

## Solution

### Buat `pkg/errs/` package:
```go
// pkg/errs/errors.go

// Sentinel errors
var (
    ErrNoData      = errors.New("no data available")
    ErrRateLimited = errors.New("rate limited")
    ErrNotFound    = errors.New("not found")
    ErrTimeout     = errors.New("timeout")
    ErrBadData     = errors.New("bad data")
)

// Wrap dengan context
func Wrap(err error, context string) error {
    return fmt.Errorf("%s: %w", context, err)
}
```

### Refactor priority services (1 PR per service):
1. `internal/service/cot/fetcher.go` — ganti bare returns dengan `errs.Wrap()`
2. `internal/service/fred/fetcher.go` — tambahkan `ErrNoData` sentinel
3. `internal/service/price/fetcher.go` — `ErrRateLimited` untuk 429 responses

### Handler behavior:
```go
// Handler bisa distinguish:
if errors.Is(err, errs.ErrNoData) {
    bot.sendMessage("Data belum tersedia, coba lagi nanti")
    return nil  // bukan error serius
}
```

## Acceptance Criteria
- [ ] `pkg/errs/` package dibuat dengan sentinels + Wrap helper
- [ ] Minimal 3 service direfactor pakai sentinel errors
- [ ] `go build ./...` sukses
- [ ] `go test ./...` tidak ada test yang break
- [ ] Handler yang relevant pakai `errors.Is()` untuk distinguish error types

## Notes
- **JANGAN** refactor semua sekaligus — prioritas `cot/`, `fred/`, `price/` dahulu
- Silent error drops perlu diaudit lebih dulu sebelum diubah — bisa ada alasan intentional
- Ini enabling work untuk TECH-008 (context propagation) yang lebih besar
