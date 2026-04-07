# TASK-194: Unit Test Coverage — Price + Backtest Services

**Priority:** medium
**Type:** test
**Estimated:** L
**Area:** internal/service/price/, internal/service/backtest/

## Deskripsi

Add unit tests untuk 40+ untested files di price and backtest services. Focus on edge cases yang cause silent failures.

## Priority Files

### Price Service (target 40%):
- `garch.go` — Test convergence, underparameterization (n=20-27), NaN handling
- `hmm_regime.go` — Test boundary conditions (40 vs 60 returns)
- `correlation.go` — Test pearsonCorrelation with n<3, n=2, zero variance
- `hurst.go` — Test with flat price series (Hurst = 0.5)
- `levels.go` — Test S/R detection with no clear levels
- `aggregator.go` — Test provider fallback cascade

### Backtest Service (target 30%):
- `walkforward.go` — Test empty slice, single-element
- `montecarlo.go` — Test with zero volatility
- `bootstrap.go` — Test confidence intervals with small samples
- `stats.go` — Test win rate with 0 evaluated signals

## Acceptance Criteria

- [ ] GARCH: 8+ test cases (convergence, underfit, NaN, extreme vol)
- [ ] HMM: 5+ test cases (boundary, insufficient data, 3-state validation)
- [ ] Correlation: 5+ test cases (n=2, n=5, zero variance, perfect correlation)
- [ ] Walkforward: 5+ test cases (empty, single, normal)
- [ ] All tests pass: `go test ./internal/service/price/... ./internal/service/backtest/...`
