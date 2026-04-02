# TASK-162: Cross-Asset Flow Divergence Detection

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/factors/

## Deskripsi

Build flow divergence detector di atas existing correlation infrastructure. Detect when normally-correlated assets decouple — strong regime change signal.

## Detail Teknis

Algorithms:
1. Rolling correlation (20-bar) antara asset pairs
2. Lead-lag analysis: cross-correlation dengan offset ±1 to ±5 bars
3. Divergence score: |current_corr - normal_corr| / σ(corr)
4. Regime stability: correlation breakdown detection

Key pairs to monitor:
- DXY ↔ EUR/USD (inverse, should be <-0.8)
- Gold ↔ USD (inverse)
- BTC ↔ SPX (risk-on correlation)
- VIX ↔ SPX (inverse)
- Oil ↔ CAD (positive)
- Yields ↔ USD (positive)

## File Changes

- `internal/service/factors/flow_divergence.go` — NEW: Divergence engine
- `internal/service/factors/models.go` — Add FlowDivergence, LeadLag types
- `internal/adapter/telegram/handler.go` — Add /flows command routing
- `internal/adapter/telegram/formatter.go` — Add flow divergence formatting
- `internal/adapter/telegram/keyboard.go` — Add flows keyboard

## Acceptance Criteria

- [ ] Compute rolling correlation untuk 10+ asset pairs
- [ ] Detect divergence events (>2σ from normal correlation)
- [ ] Lead-lag analysis: which asset leads movement
- [ ] /flows command menampilkan top divergences + regime stability
- [ ] Alert-worthy divergences highlighted
- [ ] Cache correlation state, recompute on new data
- [ ] Unit tests untuk divergence detection logic

## Done

**Completed by:** Dev-B  
**Date:** 2026-04-02  
**PR:** #243 (feat/TASK-162-cross-asset-flow-divergence → agents/main)  
**Status:** MERGED via PR
