# TASK-205: CBOE Volatility Index Suite — SKEW, OVX, GVZ, RVX, COR3M

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service/vix/

## Deskripsi

Extend existing VIX fetcher untuk fetch 6+ additional CBOE volatility indices. Same CSV pattern, same parser, just more tickers. Provides cross-asset vol surface.

## Indices to Add

| Index | Signal | Priority |
|-------|--------|----------|
| SKEW | S&P 500 tail risk (>140 = crash warning) | HIGH |
| OVX | Oil volatility (commodity fear) | HIGH |
| GVZ | Gold volatility (safe haven demand) | HIGH |
| RVX | Russell 2000 vol (small cap risk) | MEDIUM |
| VIX9D | 9-day VIX (event pricing) | MEDIUM |
| COR3M | 3-month correlation (dispersion) | MEDIUM |

## File Changes

- `internal/service/vix/fetcher.go` — Add 6 new CSV fetch functions (reuse existing pattern)
- `internal/service/vix/models.go` — Add VolIndexSuite type with all indices
- `internal/service/vix/analysis.go` — Add cross-vol analysis (SKEW/VIX ratio, OVX/GVZ divergence)
- `internal/adapter/telegram/formatter.go` — Extend /vix output with full vol dashboard

## Acceptance Criteria

- [ ] Fetch 6 CBOE index CSVs daily
- [ ] SKEW analysis: level, percentile, >140 alert
- [ ] OVX/GVZ cross-reference with VIX (divergence detection)
- [ ] VIX9D vs VIX30 term structure slope
- [ ] COR3M for dispersion signal
- [ ] Display in expanded /vix command output
- [ ] Cache with 12h TTL (EOD data)
