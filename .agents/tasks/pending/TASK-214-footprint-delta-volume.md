# TASK-214: Footprint Chart / Delta Volume Analysis

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/price/, internal/adapter/telegram/handler_vp.go

## Deskripsi

Tambahkan Delta Volume / Footprint analysis sebagai extension dari `/vp` (Volume Profile) command. Footprint chart menunjukkan bid/ask delta per bar — mengungkap order flow imbalance yang tidak terlihat di OHLCV biasa.

## Pendekatan

### OHLCV Approximation (Primary — no extra data needed)
```go
// Per bar:
BullVolume = (Close - Low) / (High - Low) * Volume
BearVolume = (High - Close) / (High - Low) * Volume
Delta = BullVolume - BearVolume

// Signals:
PositiveDelta + Up Close = Confirmed buying pressure
NegativeDelta + Down Close = Confirmed selling pressure
NegativeDelta + Up Close = "Hidden selling" (absorption) — bearish divergence
PositiveDelta + Down Close = "Hidden buying" (absorption) — bullish divergence
```

### Cumulative Delta (CVD)
- Kumulatif dari Delta per bar
- Rising price + falling CVD = bearish divergence (selling into rally)
- Falling price + rising CVD = bullish divergence (buying into dip)
- CVD breakout often precedes price breakout

### Delta Exhaustion
- Extreme positive delta at resistance = exhaustion → likely reversal
- Extreme negative delta at support = exhaustion → likely reversal

## Output Integration

Tambahkan Delta section ke `/vp` output (opsional toggle):
```
📊 Delta Volume — EURUSD H4 (last 20 bars)
━━━━━━━━━━━━━━━━━━━━
Bar        Close   Delta    Sentiment
2026-04-01 1.0895  +2,341  🟢 Confirmed Buy
2026-03-31 1.0850  -1,876  🔴 Hidden Sell ⚠️
2026-03-30 1.0820  +3,012  🟢 Confirmed Buy
...

📈 Cumulative Delta: +12,456 (Rising)
⚡ Signal: Hidden selling detected at recent high — caution
```

## File Changes

- `internal/service/price/delta.go` — NEW: DeltaEngine, ComputeDelta(), CVD()
- `internal/service/price/delta_test.go` — Unit tests
- `internal/domain/price.go` — Add DeltaBar struct, DeltaResult
- `internal/adapter/telegram/handler_vp.go` — Add delta toggle to /vp flow
- `internal/adapter/telegram/formatter.go` or `formatter_vp.go` — FormatDelta()
- `internal/adapter/telegram/keyboard.go` — Add "📊 Delta" button to VP keyboard

## Acceptance Criteria

- [ ] BullVolume/BearVolume/Delta computed per bar from OHLCV
- [ ] CVD (cumulative delta) across N bars
- [ ] Detect hidden buying/selling divergences
- [ ] Delta exhaustion detection at key levels
- [ ] Integration in /vp output via toggle button
- [ ] Works with all symbols that have volume data (forex + crypto)
- [ ] Unit tests: 5+ cases (confirmed buy, hidden sell, exhaustion)
- [ ] go build ./... clean
