# TASK-163: Monte Carlo Scenario Generator

**Status:** done
**Completed by:** Dev-B
**Date:** 2026-04-02
**PR:** #272
**Branch:** feat/TASK-163-montecarlo-scenario-generator

## Implementation

### New Files
- `internal/service/price/montecarlo_scenario.go` — MC engine:
  - GARCH(1,1) volatility estimation for time-varying variance
  - HMM regime detection for conditional drift (RISK_ON/RISK_OFF/CRISIS)
  - 1000 GBM path generation with GARCH variance evolution
  - Percentile distribution (P1, P5, P10, P25, P50, P75, P90, P95, P99)
  - VaR95, VaR99, CVaR95, mean return computation
  - Historical regime drift estimation from Viterbi path
- `internal/service/price/montecarlo_scenario_test.go` — 6 unit tests
- `internal/adapter/telegram/handler_scenario.go` — /scenario command
- `internal/adapter/telegram/formatter_scenario.go` — HTML formatting

### Modified Files
- `internal/adapter/telegram/handler.go` — registered /scenario command

### Acceptance Criteria
- [x] Generate 1000 synthetic price paths (configurable horizon up to 90 days)
- [x] Percentile output: P1, P5, P10, P25, P50, P75, P90, P95, P99
- [x] Regime-conditional drift from HMM
- [x] /scenario EUR 30 → show price distribution
- [x] VaR (Value at Risk) at 95% and 99% confidence
- [x] CVaR (Expected Shortfall) at 95%
- [x] Runtime <1 second for 1000 paths (measured: ~18ms)
- [x] Reuses existing GARCH + HMM outputs
- [x] 6 unit tests for path generation, percentile math, edge cases
- [x] go build clean, go vet clean
