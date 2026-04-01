# TASK-048: Wire SurpriseProvider (newsScheduler) ke scheduler.Deps

**Priority:** medium
**Type:** fix
**Estimated:** M
**Area:** internal/scheduler, cmd/bot
**Created by:** Research Agent
**Created at:** 2026-04-01 17:30 WIB
**Siklus:** BugHunt-B

## Deskripsi
Di `internal/scheduler/scheduler.go:1042`, ada komentar BUG-5 yang menyatakan `newsScheduler` tidak di-wire ke `scheduler.Deps`, sehingga `ComputeConvictionScoreV3` dipanggil dengan `surpriseSigma=0` pada setiap broadcast COT release.

`newsSched` sudah implement interface `SurpriseProvider` (via `GetSurpriseSigma`) dan sudah di-pass ke `Handler`. Tapi di `cmd/bot/main.go`, `sched` dibuat sebelum `newsSched`, sehingga tidak bisa langsung di-inject ke `Deps`.

```go
// scheduler.go:1050
cs := cotsvc.ComputeConvictionScoreV3(*analysis, macroRegime, 0, "", macroData, priceCtx) // 0 = placeholder
```

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] `scheduler.Deps` memiliki field `SurpriseProvider` (interface dengan `GetSurpriseSigma(currency string) float64`)
- [ ] `broadcastCOTRelease` menggunakan `SurpriseProvider` untuk mendapatkan sigma, tidak hardcode 0
- [ ] `cmd/bot/main.go` meng-inject `newsSched` ke scheduler (via setter method atau reorder inisialisasi)
- [ ] Komentar BUG-5 dihapus

## File yang Kemungkinan Diubah
- `internal/scheduler/scheduler.go`
- `cmd/bot/main.go`

## Referensi
- `.agents/research/2026-04-01-17-bug-hunting-siklus5-lanjutan.md` (BUG-B4)
- `internal/adapter/telegram/handler.go:29` (SurpriseProvider interface sudah ada di sini)
