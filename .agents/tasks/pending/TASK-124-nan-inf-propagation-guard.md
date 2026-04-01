# TASK-124: Financial Calculation NaN/Inf Propagation Guard

**Priority:** medium
**Type:** fix
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-02 00:00 WIB
**Siklus:** BugHunt

## Deskripsi
Beberapa financial calculations bisa produce NaN atau Inf (division by zero, log of zero, sqrt of negative). Nilai ini propagate downstream dan muncul sebagai "NaN" atau "Inf" di output Telegram — confusing untuk user. Buat guard utility dan apply di critical calculation paths.

## Konteks
- `volatility.go:70` — avgATR division (Inf possible)
- `garch.go` — volatility ratio calculations
- `hurst.go` — log calculations (log(0) = -Inf)
- `correlation.go` — stddev could be 0
- `indicators.go` — various normalizations
- User pernah lihat "NaN%" di output (anecdotal dari UX audit)
- Ref: `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `pkg/mathutil/safe.go` dengan functions:
  - `SafeDiv(a, b, fallback float64) float64` — return fallback jika b==0 atau result is NaN/Inf
  - `IsFinite(f float64) bool`
  - `ClampFloat(f, min, max float64) float64`
- [ ] Apply `SafeDiv` di minimal: volatility.go, garch.go, hurst.go, correlation.go
- [ ] Audit formatters: pastikan NaN/Inf tidak pernah sampai ke user output
- [ ] Add `math.IsNaN()` / `math.IsInf()` check di fmtutil number formatting

## File yang Kemungkinan Diubah
- `pkg/mathutil/safe.go` (baru)
- `internal/service/price/volatility.go`
- `internal/service/price/garch.go`
- `internal/service/price/hurst.go`
- `internal/service/price/correlation.go`
- `pkg/fmtutil/` (NaN/Inf guard di format output)

## Referensi
- `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`
