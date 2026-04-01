# TASK-163: Monte Carlo Scenario Generator

**Priority:** medium
**Type:** feature
**Estimated:** L
**Area:** internal/service/price/

## Deskripsi

Monte Carlo simulation menggunakan GARCH volatility + HMM regime state untuk generate probabilistic price scenarios. Institutional-grade risk quantification.

## Detail Teknis

Algorithm:
1. Fit GARCH(1,1) → current σ estimate
2. Get HMM regime → drift parameter per regime
3. Generate N paths (1000 default):
   ```
   For each path:
     r_t = drift_regime + σ_garch * z_t  (z_t ~ N(0,1))
     price_t = price_{t-1} * exp(r_t)
   ```
4. Compute percentile distribution (5th, 25th, 50th, 75th, 95th)

## File Changes

- `internal/service/price/montecarlo.go` — NEW: MC engine, path generation, percentile computation
- `internal/service/price/models.go` — Add MonteCarloResult, PriceDistribution types
- `internal/adapter/telegram/handler.go` — Add /scenario command routing
- `internal/adapter/telegram/formatter.go` — Add scenario distribution formatting

## Acceptance Criteria

- [ ] Generate 1000 synthetic price paths (30-day horizon)
- [ ] Percentile output: P5, P25, P50, P75, P95 price levels
- [ ] Regime-conditional scenarios (bullish regime vs bearish regime)
- [ ] /scenario EURUSD 30 → show price distribution
- [ ] VaR (Value at Risk) at 95% confidence
- [ ] Runtime <5 seconds untuk 1000 paths
- [ ] Reuse existing GARCH + HMM outputs
- [ ] Unit tests untuk path generation dan percentile math
