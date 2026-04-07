# TASK-237: ICT PD Array — Premium/Discount Zone + OTE Detection

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/ict/, internal/adapter/telegram/formatter_ict.go
**Created by:** Research Agent
**Created at:** 2026-04-02 21:00 WIB

## Deskripsi

Engine ICT (`service/ict/`) saat ini mendeteksi FVG, Order Block, CHoCH/BOS, dan Liquidity Sweep — tetapi **tidak ada PD Array ranking atau Premium/Discount zone classification**.

**PD Array** adalah konsep ICT kunci yang menjawab pertanyaan: "Price sekarang ada di mana — premium (sell zone) atau discount (buy zone)?" Tanpa ini, trader bisa masuk buy di premium zone atau sell di discount zone.

### Konsep yang perlu diimplementasi:

1. **Premium/Discount Zone** — relative to the defining swing range:
   - `0%–50%` dari range = **DISCOUNT** → favor buy-side entries
   - `50%–100%` dari range = **PREMIUM** → favor sell-side entries
   - Exactly `50%` = **EQUILIBRIUM** (EQ)

2. **OTE Zone (Optimal Trade Entry)** — Fibonacci 62%–79% retracement:
   - Bullish bias → OTE di 62%–79% pull-back dari low
   - Bearish bias → OTE di 62%–79% pull-back dari high

3. **PD Array Ranking** — setelah tahu zone, rank PD arrays berdasarkan proximity + strength:
   - Bullish: FVG Bullish > Bullish OB > FVG yang partial filled
   - Bearish: FVG Bearish > Bearish OB > FVG yang partial filled

## File yang Harus Dibuat/Diubah

- `internal/service/ict/pdarray.go` — NEW: `DetectPDArray()` function
- `internal/service/ict/types.go` — tambah `PDArrayResult` struct + field ke `ICTResult`
- `internal/service/ict/engine.go` — tambah Step untuk run `DetectPDArray()`
- `internal/adapter/telegram/formatter_ict.go` — tampilkan PD Array section

## Implementasi

### types.go — struct baru
```go
// PDArrayResult classifies the current market position within the ICT PD Array framework.
type PDArrayResult struct {
    // Defining range for Premium/Discount calculation
    RangeHigh   float64 // highest swing high used for PD reference
    RangeLow    float64 // lowest swing low used for PD reference
    RangeMid    float64 // 50% equilibrium level
    CurrentPct  float64 // 0–100, current price position within the range

    // Zone classification
    Zone        string  // "PREMIUM" | "DISCOUNT" | "EQUILIBRIUM" (±3% of 50%)

    // OTE (Optimal Trade Entry) — Fibonacci 62–79% retracement of current move
    OTEActive   bool    // true if current price is within OTE zone
    OTEHigh     float64 // upper bound of OTE zone (79% Fib)
    OTELow      float64 // lower bound of OTE zone (62% Fib)
    OTEBias     string  // "BULLISH" | "BEARISH" (direction of the move being retraced)

    // Nearest actionable PD array
    NearestBullishPDA string  // e.g. "FVG BULLISH @ 1.0821-1.0835" or ""
    NearestBearishPDA string  // e.g. "BEARISH OB @ 1.0890-1.0912" or ""
}
```

Tambah ke `ICTResult`:
```go
PDArray *PDArrayResult // may be nil if insufficient swing data
```

### pdarray.go — DetectPDArray()
```go
// DetectPDArray computes the Premium/Discount zone and OTE based on swing structure.
// Uses the most recent significant swing range as the PD reference.
func DetectPDArray(bars []ta.OHLCV, swings []swingPoint, result *ICTResult) *PDArrayResult {
    // 1. Find the defining range: last significant HH and LL
    // 2. Compute currentPct = (close - rangeLow) / (rangeHigh - rangeLow) * 100
    // 3. Classify zone: <47 = DISCOUNT, >53 = PREMIUM, else EQUILIBRIUM
    // 4. Find last swing leg for OTE: 62-79% Fibonacci retracement
    // 5. Rank nearest bullish/bearish PD arrays from FVGs and OBs
}
```

### Formatter display
```
📐 PD ARRAY ANALYSIS
Range  : 1.0750 – 1.0945  (195 pip range)
EQ     : 1.0848  (50%)
Price  : 1.0812  → 32% — 📉 DISCOUNT ZONE

🎯 Bullish PDA : FVG @ 1.0790–1.0808 (below price — filled 60%)
🎯 Bearish PDA : OB @ 1.0870–1.0895 (above price — intact)

🔑 OTE Zone   : 1.0813–1.0823 ← PRICE IN OTE ✅
Bias          : Bullish retracement → wait for entry 62-79% of up-leg
```

## Acceptance Criteria

- [ ] `DetectPDArray()` menghitung zone berdasarkan swing range terdeteksi
- [ ] `PDArrayResult.Zone` benar untuk price di atas/bawah 50% range
- [ ] OTE zone dihitung dari leg terbaru (62% dan 79% Fibonacci)
- [ ] `OTEActive = true` jika current close berada di dalam OTE band
- [ ] Nearest bullish/bearish PD array diambil dari `ICTResult.FVGZones` dan `ICTResult.OrderBlocks`
- [ ] Formatter menampilkan PD Array section sebagai bagian dari `/ict` output
- [ ] Jika swings < 4 → `PDArray = nil` (graceful skip)
- [ ] Unit test: `TestPDArrayDiscount`, `TestPDArrayPremium`, `TestOTEDetection`

## Referensi

- `.agents/research/2026-04-02-21-feature-gaps-skew-credit-ict-pdarray-cot-seasonal-putaran9.md` — Temuan 3
- `internal/service/ict/swing.go:detectSwings()` — swing detection yang sudah ada
- `internal/service/ict/fvg.go:DetectFVG()` — FVG zones untuk ranking
- `internal/service/ict/orderblock.go:DetectOrderBlocks()` — OB zones untuk ranking
- `internal/service/ict/engine.go` — tambah step setelah DetectLiquiditySweeps
