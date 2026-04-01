# TASK-047: Fix handler_ctabt.go chart subprocess tanpa timeout

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 17:30 WIB
**Siklus:** BugHunt-B

## Deskripsi
Di `internal/adapter/telegram/handler_ctabt.go:472`, Python chart subprocess dijalankan dengan `context.Background()` tanpa timeout:

```go
cmd := exec.CommandContext(context.Background(), "python3", scriptPath, inputPath, outputPath)
```

Jika backtest chart Python script hang, handler goroutine akan hang selamanya tanpa bisa di-cancel oleh user request context atau bot shutdown.

Ref: `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B3)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `context.Background()` diganti dengan `context.WithTimeout(ctx, 60*time.Second)`
- [ ] `cancel()` di-defer dengan benar
- [ ] `ctx` yang digunakan adalah ctx dari request/caller

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_ctabt.go`

## Referensi
- `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B3)
