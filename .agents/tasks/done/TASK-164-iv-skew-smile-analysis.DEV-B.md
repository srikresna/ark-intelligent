# TASK-164: IV Skew / Smile Analysis + Skew Flip Detection

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/gex/

## Deskripsi

Build IV skew analysis layer di atas existing Deribit options data. Skew direction change (bearish→bullish flip) adalah salah satu sinyal reversal terkuat di options market. Different from TASK-130 (raw IV surface) — this is the analysis/alert layer.

## Detail Teknis

Metrics:
1. IV Smile: IV per strike grouped by moneyness (0.8, 0.9, 1.0, 1.1, 1.2 delta)
2. Put-Call IV Ratio: avg_put_IV / avg_call_IV (>1 = bearish skew)
3. Skew Slope: regression of IV vs moneyness (negative = normal, positive = inverse)
4. Term Structure: ATM IV near vs far expiry (contango = calm, backwardation = event)

## File Changes

- `internal/service/gex/skew.go` — NEW: Skew analyzer, smile fitting, flip detection
- `internal/service/gex/models.go` — Add SkewResult, SmilePoint, SkewAlert types
- `internal/adapter/telegram/formatter_gex.go` — Add skew section to GEX output
- `internal/adapter/telegram/keyboard.go` — Add skew toggle to GEX keyboard

## Acceptance Criteria

- [ ] Compute IV smile curve per expiry (5-point moneyness)
- [ ] Put/call IV ratio dengan historical percentile
- [ ] Detect skew flip events (bearish→bullish or vice versa)
- [ ] ATM IV term structure slope
- [ ] Display skew analysis di /gex output (new section)
- [ ] Alert on skew flip (>2σ change in 24h)
- [ ] Reuse existing Deribit ticker data (mark_iv per strike)
- [ ] Unit tests untuk skew computation
