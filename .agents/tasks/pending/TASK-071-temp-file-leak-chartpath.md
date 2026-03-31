# TASK-071: Fix Temp File Leak chartPath di handler_vp dan handler_quant

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 03:00 WIB
**Siklus:** BugHunt

## Deskripsi
`chartPath` di `handler_vp.go` dan `handler_quant.go` tidak memiliki `defer os.Remove()`. Jika terjadi error setelah Python membuat file chart (misal `os.ReadFile` gagal atau error VP engine), file PNG tetinggal di `/tmp` selamanya.

## Konteks
- `handler_vp.go:407-410`:
  ```go
  chartPath := filepath.Join(os.TempDir(), ...)
  defer os.Remove(inputPath)   // ada
  defer os.Remove(outputPath)  // ada
  // chartPath TIDAK ada defer Remove!
  ```
- `handler_quant.go:436,441-442`: sama, chartPath tidak di-defer
- Setiap error path setelah Python menulis chartPath = file bocor di /tmp
- Akumulasi file bisa habiskan disk pada server dengan traffic tinggi

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `handler_vp.go`: tambah `defer os.Remove(chartPath)` setelah chartPath dibuat
- [ ] `handler_quant.go`: tambah `defer os.Remove(chartPath)` setelah chartPath dibuat
- [ ] Caller yang membaca `result.ChartPath` tetap bisa membaca file sebelum defer dieksekusi (karena defer berjalan saat fungsi return, bukan saat caller selesai)
  - Perlu verifikasi: apakah chartPath dibaca di dalam atau di luar fungsi runVPEngine? Jika di luar, defer di dalam fungsi sudah menghapus file sebelum caller membacanya!
  - Jika demikian: Remove chartPath di caller setelah membacanya, bukan defer di dalam fungsi

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_vp.go` (sekitar line 407-410 dan caller di line 370-376)
- `internal/adapter/telegram/handler_quant.go` (sekitar line 436-442)

## Referensi
- `.agents/research/2026-04-01-03-bug-hunting-subprocess-tempfile-race.md` — Bug #3 & #4
