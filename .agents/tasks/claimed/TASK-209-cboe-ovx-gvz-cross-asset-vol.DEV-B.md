# TASK-209: Cross-Asset Volatility Dashboard (OVX, GVZ, RVX vs VIX)

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/vix/

## Deskripsi

Build cross-asset volatility comparison dashboard. OVX (oil), GVZ (gold), RVX (small cap) vs VIX (S&P 500). Divergences between asset-class volatilities signal regime transitions.

## Signal Logic

```
OVX rising + VIX flat   → Energy-specific risk (geopolitical, supply shock)
GVZ rising + VIX rising → Broad risk-off (safe haven + equity fear)
RVX/VIX ratio > 1.3     → Small cap underperforming (risk appetite declining)
All rising               → Systemic stress (2020, 2022 pattern)
```

## File Changes

- `internal/service/vix/cross_vol.go` — NEW: Cross-asset vol comparison engine
- `internal/service/vix/models.go` — Add CrossVolResult type
- `internal/adapter/telegram/formatter.go` — Add cross-vol section to /vix output
- `internal/adapter/telegram/keyboard.go` — Add vol dashboard toggle

## Acceptance Criteria

- [ ] Compute OVX/VIX, GVZ/VIX, RVX/VIX ratios
- [ ] Detect divergences (one rising while others flat)
- [ ] Regime classification: energy-specific / broad risk-off / systemic
- [ ] Display in /vix command with visual bar chart
- [ ] Historical percentile for each ratio
- [ ] Depends on TASK-205 completion
