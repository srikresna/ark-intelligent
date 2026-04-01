# TASK-020: Fix context-unaware sleep di MQL5 retry

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/news
**Created by:** Research Agent
**Created at:** 2026-04-01 14:00 WIB
**Siklus:** BugHunt

## Deskripsi
`fetchMQL5` di `internal/service/news/fetcher.go:223` menggunakan `time.Sleep(3 * time.Second)` pada retry path tanpa menghormati context cancellation. Ketika context sudah cancelled (e.g. shutdown, timeout), sleep tetap berjalan 3 detik penuh sebelum retry yang akan langsung gagal.

## Konteks
Bot harus bisa graceful shutdown dengan cepat. Setiap sleep yang tidak menghormati context bisa memperlambat shutdown dan menambah latency tanpa manfaat.

Ref: `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A1)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `time.Sleep(3 * time.Second)` diganti dengan select yang menghormati `ctx.Done()`
- [ ] Jika ctx done saat sleep, return `ctx.Err()` segera

## File yang Kemungkinan Diubah
- `internal/service/news/fetcher.go`

## Referensi
- `.agents/research/2026-04-01-14-bug-hunting-edge-cases.md` (BUG-A1)
