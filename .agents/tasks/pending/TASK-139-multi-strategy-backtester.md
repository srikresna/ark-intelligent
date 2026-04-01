# TASK-139: Multi-Strategy Backtester (Strategy Composer)

**Priority:** medium
**Type:** feature
**Estimated:** L
**Area:** internal/service/backtest
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB
**Siklus:** Fitur

## Deskripsi
Run independent backtests per strategy (COT, CTA, Wyckoff, GEX, Seasonal), compute per-strategy metrics (Sharpe, drawdown), analyze inter-strategy correlation, dan combine via optimal weighting.

## Konteks
- Backtest engine sudah mature: walkforward.go, montecarlo.go, bootstrap.go, portfolio.go, ruin.go, weights.go
- Walk-forward optimizer sudah ada (rolling OLS, regime-based weights)
- Current: single signal stream aggregated to portfolio
- Missing: per-strategy isolation, inter-strategy correlation, optimal combination
- Ref: `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Buat `internal/service/backtest/strategy_composer.go`
- [ ] Accept: N strategy signal streams (COT, CTA, Quant, Wyckoff, GEX, Seasonal)
- [ ] Run independent walk-forward backtest per strategy
- [ ] Compute per-strategy: Sharpe ratio, max drawdown, win rate, profit factor
- [ ] Compute: inter-strategy correlation matrix (return correlation)
- [ ] Optimal combination: equal-weight, inverse-vol-weight, atau correlation-aware weight
- [ ] Output: combined portfolio Sharpe, per-strategy contribution, diversification ratio
- [ ] Telegram: extend `/backtest` dengan "Multi-Strategy" mode
- [ ] Formatter: table per strategy + combined result

## File yang Kemungkinan Diubah
- `internal/service/backtest/strategy_composer.go` (baru)
- `internal/service/backtest/types.go` (multi-strategy types)
- `internal/adapter/telegram/handler.go` (extend /backtest)
- `internal/adapter/telegram/formatter.go` (multi-strategy formatter)

## Referensi
- `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`
- `internal/service/backtest/walkforward.go`
- `internal/service/backtest/portfolio.go`
