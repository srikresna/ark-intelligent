# TASK-161: VWAP + Estimated Delta Profile

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/ta/

## Deskripsi

Tambah VWAP (Volume-Weighted Average Price) dan estimated delta profile ke technical analysis toolkit. Pure computation dari existing OHLCV — no new data needed.

## Detail Teknis

VWAP Formula:
```
VWAP = Σ(Typical_Price × Volume) / Σ(Volume)
Typical_Price = (High + Low + Close) / 3
```

Estimated Delta (Tick Rule):
```
if Close[i] > Close[i-1]: delta = +Volume
if Close[i] < Close[i-1]: delta = -Volume
if Close[i] == Close[i-1]: delta = 0
Cumulative Delta = running sum
```

## File Changes

- `internal/service/ta/vwap.go` — NEW: VWAP computation, deviation bands (±1σ, ±2σ)
- `internal/service/ta/delta.go` — NEW: Tick rule delta estimation, cumulative delta
- `internal/service/ta/models.go` — Add VWAPResult, DeltaResult types
- `internal/service/ta/confluence.go` — Integrate VWAP + delta into confluence scoring
- `internal/adapter/telegram/formatter.go` — Add VWAP/delta to /cta output

## Acceptance Criteria

- [ ] Compute session VWAP + ±1σ, ±2σ deviation bands
- [ ] Price position relative to VWAP (above/below + deviation)
- [ ] Estimated cumulative delta dari OHLCV
- [ ] Delta divergence: price up + cumulative delta down = bearish divergence
- [ ] Integrate into /cta output (new section)
- [ ] VWAP anchor types: session, weekly, monthly
- [ ] Unit tests untuk VWAP computation dan delta estimation
