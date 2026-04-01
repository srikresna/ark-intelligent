# TASK-167: Fix Temp File Lifecycle in Python Subprocess Handlers

**Priority:** high
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram/

## Deskripsi

Fix race condition antara `defer os.Remove()` dan Python subprocess output. Temp files dihapus via defer saat function exit, tapi chartPath dikembalikan ke caller yang belum baca file.

## Current Problem

```go
defer os.Remove(inputPath)   // cleanup scheduled
defer os.Remove(outputPath)  // cleanup scheduled
cmd.Run()                    // Python writes files
result.ChartPath = chartPath // returned — but defer deletes it!
```

## Solution

Replace defer cleanup dengan explicit cleanup after use:
```go
defer os.Remove(inputPath)  // input can be deferred (not returned)
// outputPath: read immediately, then clean
resultJSON, err := os.ReadFile(outputPath)
os.Remove(outputPath) // explicit cleanup after read
// chartPath: DON'T cleanup here — caller responsible
```

## File Changes

- `internal/adapter/telegram/handler_vp.go` — Fix temp file lifecycle di runVPEngine()
- `internal/adapter/telegram/handler_cta.go` — Fix temp file lifecycle di chart generation
- `internal/adapter/telegram/handler_quant.go` — Fix temp file lifecycle di quant Python calls

## Acceptance Criteria

- [ ] inputPath: defer cleanup (safe — not returned)
- [ ] outputPath: explicit cleanup after ReadFile (not deferred)
- [ ] chartPath: cleanup by caller after sending chart to Telegram
- [ ] No file leak — all temp files eventually cleaned
- [ ] No race condition — files not deleted while in use
- [ ] Add chartPath cleanup in caller (handleCTA, cmdVP, etc.) after SendPhoto
