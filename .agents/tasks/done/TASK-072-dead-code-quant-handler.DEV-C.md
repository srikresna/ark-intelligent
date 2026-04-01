# TASK-072: Remove Dead Code di handler_quant.go (Redundant exec.CommandContext)

**Priority:** low
**Type:** refactor
**Estimated:** S
**Area:** internal/adapter/telegram
**Created by:** Research Agent
**Created at:** 2026-04-01 03:00 WIB
**Siklus:** BugHunt

## Deskripsi
`handler_quant.go:445` membuat `cmd` yang langsung di-overwrite di line 451. Baris 445-447 adalah dead code yang membingungkan pembaca.

## Konteks
```go
// Line 445-447: DEAD CODE — cmd ini tidak pernah dipakai
cmd := exec.CommandContext(context.Background(), "python3", scriptPath, inputPath, outputPath, chartPath)
cmd.Stderr = os.Stderr

// Timeout: 60s for complex models
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
// Line 451: overwrite — ini yang benar-benar dieksekusi
cmd = exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath, chartPath)
cmd.Stderr = os.Stderr
```

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Baris dead code (line 445-447) dihapus
- [ ] Variabel `cmdCtx` dan `cancel` dipindahkan sebelum pembuatan `cmd` yang valid
- [ ] Behavior tidak berubah — masih menggunakan timeout 60s

## File yang Kemungkinan Diubah
- `internal/adapter/telegram/handler_quant.go` (sekitar line 440-455)

## Referensi
- `.agents/research/2026-04-01-03-bug-hunting-subprocess-tempfile-race.md` — Bug #3
