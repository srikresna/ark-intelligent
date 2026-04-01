# TASK-122: Walk-Forward Backtest Empty Slice Safety

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/backtest
**Created by:** Research Agent
**Created at:** 2026-04-02 00:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `walkforward.go:90-91`, code mengakses `evaluated[0]` dan `evaluated[len(evaluated)-1]` setelah early return di line 74. Antara line 74 dan 90, filter operations bisa theoretically reduce slice ke empty. Pattern serupa di `walkforward_multi.go` dan `walkforward_optimizer.go`.

## Konteks
- `walkforward.go:90-91` — `earliest := evaluated[0].ReportDate`
- Early return di line 74 guard untuk `len < 1`, tapi subsequent filtering bisa reduce
- Defensive programming: guard should be at point-of-use, bukan hanya point-of-entry
- Ref: `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Tambah bounds check sebelum `evaluated[0]` access di line 90
- [ ] Audit `walkforward_multi.go` dan `walkforward_optimizer.go` untuk pattern serupa
- [ ] Return meaningful error (bukan panic) jika slice empty setelah filtering
- [ ] Tidak ada behavior change untuk normal data

## File yang Kemungkinan Diubah
- `internal/service/backtest/walkforward.go`
- `internal/service/backtest/walkforward_multi.go`
- `internal/service/backtest/walkforward_optimizer.go`

## Referensi
- `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`
