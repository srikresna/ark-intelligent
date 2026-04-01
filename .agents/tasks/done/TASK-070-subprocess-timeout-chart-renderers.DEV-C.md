# TASK-070: Fix Subprocess Timeout di Chart Renderers (CTA & Backtest)

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 03:00 WIB
**Siklus:** BugHunt

## Deskripsi
Tambahkan timeout pada `exec.CommandContext` di `runChartScript()` dan `runBacktestChartScript()`. Saat ini keduanya menggunakan `context.Background()` tanpa timeout — jika Python script hang, goroutine handler akan hang selamanya.

## Konteks
- `handler_cta.go:745` — fungsi `runChartScript()` tidak punya timeout
- `handler_ctabt.go:472` — fungsi `runBacktestChartScript()` tidak punya timeout
- Referensi implementasi yang benar ada di `handler_vp.go:421`:
  ```go
  cmdCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
  defer cancel()
  cmd := exec.CommandContext(cmdCtx, "python3", ...)
  ```

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `runChartScript()` di handler_cta.go menggunakan `context.WithTimeout` (60s atau 90s)
- [ ] `runBacktestChartScript()` di handler_ctabt.go menggunakan `context.WithTimeout` (90s)
- [ ] Error message saat timeout jelas (misal: "chart renderer timed out after 90s")

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_cta.go` (sekitar line 745)
- `internal/adapter/telegram/handler_ctabt.go` (sekitar line 472)

## Referensi
- `.agents/research/2026-04-01-03-bug-hunting-subprocess-tempfile-race.md` — Bug #1 & #2
