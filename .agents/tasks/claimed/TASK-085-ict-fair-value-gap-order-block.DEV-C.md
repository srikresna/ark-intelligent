# TASK-085: ICT Fair Value Gap (FVG) & Order Block Detector

**Priority:** high
**Type:** feature
**Estimated:** M (2-4h)
**Area:** internal/service/ta
**Created by:** Research Agent
**Created at:** 2026-04-01 15:00 WIB
**Siklus:** Fitur (Siklus 3)

## Deskripsi
Implementasi detector ICT (Inner Circle Trader) Fair Value Gap dan Order Block di `internal/service/ta/ict.go`. Tambahkan hasil ke `IndicatorSnapshot` dan tampilkan di output `/cta`.

## Konteks
Fair Value Gap (FVG) adalah area gap pada chart 3-candle yang tidak terisi. Order Block adalah candle terakhir berlawanan sebelum impulse move besar. Ini adalah dua konsep ICT yang paling populer di komunitas forex institusional dan banyak diminta trader.

Ref: `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md#2a`

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat `internal/service/ta/ict.go` dengan:
  - Struct `FVGResult` berisi: Direction (BULLISH/BEARISH), HighEdge, LowEdge, Filled bool, BarIndex int
  - Struct `OrderBlockResult` berisi: Direction, High, Low, LastTestedAt, Strength (1-5), Mitigated bool
  - Struct `ICTResult` berisi: FVGs []FVGResult (max 5 terbaru unfilled), OrderBlocks []OrderBlockResult (max 3 terbaru)
  - `DetectFVG(bars []OHLCV) []FVGResult` — scan bars untuk 3-candle FVG pattern
    - Bullish FVG: `bars[i+2].High < bars[i].Low` (gap antara candle 1 high dan candle 3 low)
    - Bearish FVG: `bars[i+2].Low > bars[i].High` (gap antara candle 1 low dan candle 3 high)
    - Minimum gap size = 0.2x ATR
    - Mark as filled jika subsequent price trades through the gap
  - `DetectOrderBlocks(bars []OHLCV, atr float64) []OrderBlockResult` — detect OB
    - Bullish OB: candle bearish terakhir sebelum impulse bullish (≥1.5x ATR range dalam 1 candle)
    - Bearish OB: candle bullish terakhir sebelum impulse bearish (≥1.5x ATR range)
    - Mitigated = true jika price sudah kembali ke OB zone
- [ ] Tambahkan field `ICT *ICTResult` ke `IndicatorSnapshot` di `types.go`
- [ ] Panggil ICT detection dari `engine.go` dalam `ComputeSnapshot()`
- [ ] Tambahkan ICT summary ke formatter/output `/cta` (tampilkan FVG nearest + OB nearest)
- [ ] Unit test minimal: 1 test FVG detection pada synthetic bars

## File yang Kemungkinan Diubah
- `internal/service/ta/ict.go` (baru)
- `internal/service/ta/types.go` (tambah ICTResult field)
- `internal/service/ta/engine.go` (call DetectFVG + DetectOrderBlocks)
- `internal/adapter/telegram/` atau formatter yang render /cta output

## Referensi
- `.agents/research/2026-04-01-15-fitur-baru-ict-smc-wyckoff.md`
