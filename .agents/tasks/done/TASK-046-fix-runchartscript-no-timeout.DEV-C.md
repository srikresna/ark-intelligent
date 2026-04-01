# TASK-046: Fix runChartScript di handler_cta.go tanpa timeout

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 17:30 WIB
**Siklus:** BugHunt-B

## Deskripsi
`runChartScript` di `handler_cta.go:728` adalah fungsi standalone yang mengeksekusi Python script dengan `context.Background()` tanpa timeout. Jika Python hang, goroutine handler akan hang selamanya.

Dipanggil dari `generateCTADetailChart` untuk mode ichimoku, fibonacci, zones. Berbeda dengan `generateCTAChart` yang sudah menerima dan meneruskan `ctx`, dan `handler_quant.go` yang punya timeout 60s.

```go
// Sekarang (WRONG)
cmd := exec.CommandContext(context.Background(), "python3", scriptPath, inputPath, outputPath)

// Seharusnya
cmdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
defer cancel()
cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
```

Ref: `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B2)

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `runChartScript` menerima `ctx context.Context` sebagai parameter pertama
- [ ] Python subprocess dijalankan dengan `context.WithTimeout(ctx, 60*time.Second)`
- [ ] Semua caller (`generateCTADetailChart`) di-update untuk pass `ctx`

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_cta.go`

## Referensi
- `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B2)
