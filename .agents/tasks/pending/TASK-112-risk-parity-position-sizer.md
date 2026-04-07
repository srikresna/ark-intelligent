# TASK-112: Risk Parity Position Sizer

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/strategy
**Created by:** Research Agent
**Created at:** 2026-04-01 22:00 WIB
**Siklus:** Fitur

## Deskripsi
Extend position sizing dari per-pair menjadi cross-portfolio. Account total portfolio heat (sum semua pair risk), apply Kelly fraction dari backtest stats, dan adjust berdasarkan volatility regime.

## Konteks
- ATR-based position sizing sudah ada di `service/price/position_size.go` (6KB) — per-pair only
- Backtest engine sudah ada — win rate, Sharpe, max drawdown available
- GARCH volatility regime sudah ada
- `/heat` command sudah ada tapi hanya display, tidak recommend sizing
- Gap: no cross-portfolio risk balancing
- Ref: `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `internal/service/strategy/risk_parity_sizer.go`
- [ ] Input: list of positions (pair, direction, entry, stop, current size), account balance, max portfolio heat %
- [ ] Output: adjusted sizes per pair, total heat, Kelly fraction, heat breakdown, recommendation (SCALE_DOWN/BALANCED/SCALE_UP)
- [ ] Kelly formula: f* = (2p - 1) / b, capped at half-Kelly for safety
- [ ] Volatility regime adjustment: high vol → reduce sizes 10-20%, low vol → allow slight increase
- [ ] Expose via extended `/heat` command → show optimal sizing recommendations
- [ ] Handle edge case: no backtest data → fallback to fixed fractional sizing

## File yang Kemungkinan Diubah
- `internal/service/strategy/risk_parity_sizer.go` (baru)
- `internal/service/price/position_size.go` (possible integration)
- `internal/adapter/telegram/handler.go` (extend /heat command)
- `internal/adapter/telegram/formatter.go` (format risk parity output)

## Referensi
- `.agents/research/2026-04-01-22-fitur-regime-overlay-unified-signal.md`
- `internal/service/price/position_size.go`
- `internal/service/backtest/`
