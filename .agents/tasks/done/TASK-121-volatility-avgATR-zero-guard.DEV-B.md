# TASK-121: Volatility avgATR Zero-Division Guard

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/price
**Created by:** Research Agent
**Created at:** 2026-04-02 00:00 WIB
**Siklus:** BugHunt

## Deskripsi
Di `volatility.go:70`, `ratio := currentATR / avgATR` bisa divide by zero jika `avgATR == 0`. Ini terjadi saat semua bars punya identical Close price (low-liquidity pairs, stale data). Result: `Inf` atau `NaN` yang propagate ke downstream calculations.

## Konteks
- `volatility.go:70` — `ratio := currentATR / avgATR`
- Guard di line 66 check `avgATR <= 0` tapi hanya untuk early return — jika flow melewati guard (edge case), zero division terjadi
- Low-liquidity exotic pairs bisa punya zero ATR saat price flat
- Ref: `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Pastikan `avgATR == 0` guard explicit sebelum division di line 70
- [ ] Return `VolatilityNormal` (default) jika avgATR atau currentATR adalah 0
- [ ] Audit semua division operations di `volatility.go` untuk zero-checks
- [ ] Juga audit `garch.go` dan `hurst.go` untuk pattern serupa

## File yang Kemungkinan Diubah
- `internal/service/price/volatility.go`
- `internal/service/price/garch.go` (audit)
- `internal/service/price/hurst.go` (audit)

## Referensi
- `.agents/research/2026-04-01-24-bug-hunting-bounds-divzero-goroutine.md`
