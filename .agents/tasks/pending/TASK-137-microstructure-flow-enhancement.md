# TASK-137: Microstructure Flow Enhancement — Absorption + Delta Divergence

**Priority:** medium
**Type:** feature
**Estimated:** L
**Area:** internal/service/microstructure
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB
**Siklus:** Fitur

## Deskripsi
Upgrade Bybit microstructure engine: tambah trade size bucketing (flag large/institutional trades), absorption detection (bids/asks disappearing after large print), dan delta divergence (OI vs price direction mismatch).

## Konteks
- `service/microstructure/engine.go` — basic: OrderbookImbalance, TakerBuyRatio, LongShortRatio, FundingRate
- Bybit trade history endpoints sudah ada di `marketdata/bybit/`
- Current: 500-trade window, simple imbalance
- Missing: large trade flagging, absorption pattern, delta divergence
- Ref: `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`

## Acceptance Criteria
- [ ] go build ./... sukses
- [ ] go vet ./... sukses
- [ ] Trade size bucketing: flag trades >2 std dev volume as "large/institutional"
- [ ] `LargeTradePresence`: percentage of volume from large trades (0-100%)
- [ ] Absorption detection: if top-5 bid levels disappear within 10s after large buy → absorption signal
- [ ] Delta divergence: compare OI change direction vs price direction; flag mismatch as warning
- [ ] Extend `MicrostructureResult` dengan: AbsorptionScore, LargeTradePresence, DeltaDivergence bool
- [ ] Telegram: extend existing microstructure output dengan new metrics

## File yang Kemungkinan Diubah
- `internal/service/microstructure/engine.go` (extend)
- `internal/service/microstructure/types.go` (extend result types)
- `internal/adapter/telegram/formatter.go` (microstructure section)

## Referensi
- `.agents/research/2026-04-02-03-fitur-volcom-carry-microstructure-regime-alert.md`
- `internal/service/microstructure/engine.go`
