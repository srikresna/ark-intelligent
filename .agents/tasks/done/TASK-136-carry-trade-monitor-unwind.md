# TASK-136: Carry Trade Monitor & Unwind Detector

**Priority:** high
**Type:** feature
**Estimated:** M
**Status:** done
**Completed by:** Dev-A (loop #9) — orphan cleanup, fully implemented in codebase
**Verified:** domain/carry_monitor.go, service/fred carry_monitor, handler_carry.go, formatter_carry.go — all present on agents/main
**Area:** internal/service/fred
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB
**Siklus:** Fitur

## Deskripsi
Extend existing FRED rate differential engine ke real-time carry trade monitor. Rank pairs by carry (rate spread), track daily P&L accrual, detect carry unwind events (saat spread kolaps).

## Konteks
- `service/fred/rate_differential.go` sudah ada — FetchCarryRanking(), CarryAdjustment(), normalized -100 to +100
- 7 pairs: EUR, GBP, JPY, CHF, AUD, CAD, NZD vs USD
- Missing: roll yield, daily tracking, unwind detector
- Carry unwind = saat best-carry ke worst-carry spread compress <2% → danger signal
- Ref: `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Extend `rate_differential.go` atau buat `carry_monitor.go`
- [ ] Compute: carry spread per pair (annualized), rank by attractiveness
- [ ] Track: daily carry P&L accrual per pair (synthetic position)
- [ ] Detect: carry unwind event — saat spread range (max-min) collapse >30% dalam 1 minggu
- [ ] Telegram command: `/carry` showing:
  - Ranked pairs by carry attractiveness
  - Daily carry earned (bps)
  - Unwind risk indicator (🟢 Normal / 🟡 Narrowing / 🔴 Unwind Alert)
- [ ] Alert: push notification saat unwind detected

## File yang Kemungkinan Diubah
- `internal/service/fred/carry_monitor.go` (baru) atau extend `rate_differential.go`
- `internal/adapter/telegram/handler.go` (new /carry command)
- `internal/adapter/telegram/formatter.go` (carry monitor formatter)

## Referensi
- `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`
- `internal/service/fred/rate_differential.go`
