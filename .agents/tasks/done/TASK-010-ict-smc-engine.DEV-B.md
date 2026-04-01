# TASK-010: ICT/SMC Analysis Engine

**Priority:** HIGH  
**Cycle:** Siklus 3 — Fitur Baru  
**Estimated Complexity:** MEDIUM-HIGH  
**Research Ref:** `.agents/research/2026-04-01-09-ict-smc-wyckoff-elliott-features.md`

---

## Deskripsi

Implementasi engine Smart Money Concepts (SMC) / Inner Circle Trader (ICT) untuk forex pairs. Engine ini harus bisa detect Fair Value Gap (FVG), Order Block (OB), Breaker Block (BB), Change of Character (CHOCH), Break of Structure (BOS), dan Liquidity Sweep dari data OHLCV yang sudah ada.

## Konteks Teknis

### Data Source
- Gunakan `[]ta.OHLCV` yang sudah ada dari `service/price`
- Tidak perlu data source baru

### File yang Perlu Dibuat
```
internal/service/ict/
├── types.go        ← FVGZone, OrderBlock, StructurePoint, LiquiditySweep structs
├── engine.go       ← Engine dengan method Analyze(bars []ta.OHLCV) *ICTResult
├── fvg.go          ← Fair Value Gap detection
├── orderblock.go   ← Order Block + Breaker Block detection
├── structure.go    ← CHOCH + BOS + swing point detection
└── liquidity.go    ← Liquidity Sweep detection
```

### File yang Perlu Dimodifikasi
- `internal/adapter/telegram/handler_alpha.go` — tambah `/ict` command
- `internal/adapter/telegram/handler_alpha.go` — AlphaServices struct tambah ICT engine
- `internal/adapter/telegram/formatter.go` — FormatICTResult()
- `internal/adapter/telegram/bot.go` — wire ICT engine di startup

## Spesifikasi Fitur

### Fair Value Gap (FVG)
```go
// Bullish FVG: candle[i+2].High < candle[i].Low (gap antara 3 candles)
// bearish FVG: candle[i+2].Low > candle[i].High
type FVGZone struct {
    Kind      string  // "BULLISH" | "BEARISH"
    Top       float64
    Bottom    float64
    CreatedAt time.Time
    BarIndex  int
    Filled    bool    // true jika price sudah masuk zone ini
    FillPct   float64 // seberapa jauh sudah terisi (0-100%)
}
```

### Order Block
```go
type OrderBlock struct {
    Kind      string  // "BULLISH" | "BEARISH"
    Top       float64
    Bottom    float64
    Volume    float64
    BarIndex  int
    Broken    bool    // true = sudah dilanggar (menjadi Breaker)
}
```

### Change of Character / Break of Structure
```go
type StructureEvent struct {
    Kind      string  // "CHOCH" | "BOS"
    Direction string  // "BULLISH" | "BEARISH" (direction of break)
    Level     float64 // swing high/low yang dibreak
    BarIndex  int
}
```

### Liquidity Sweep
```go
type LiquiditySweep struct {
    Kind      string  // "SWEEP_HIGH" | "SWEEP_LOW"
    Level     float64 // previous swing high/low yang disapu
    SweepHigh float64 // wajah high dari candle yang sweep
    SweepLow  float64
    BarIndex  int
    Reversed  bool    // true = confirmed reversal setelah sweep
}
```

### ICTResult (output utama)
```go
type ICTResult struct {
    Symbol      string
    Timeframe   string
    FVGZones    []FVGZone
    OrderBlocks []OrderBlock
    Structure   []StructureEvent
    Sweeps      []LiquiditySweep
    Bias        string  // current structural bias "BULLISH"|"BEARISH"|"NEUTRAL"
    Killzone    string  // current killzone jika applicable
    Summary     string  // human-readable narrative
    AnalyzedAt  time.Time
}
```

## Algoritma Kunci

### Swing Point Detection (prerequisite semua ICT concepts)
```
lookback = 5 bars (kiri dan kanan)
SwingHigh[i] = true jika bars[i].High > max(bars[i-5..i-1]) AND bars[i].High > max(bars[i+1..i+5])
SwingLow[i] = true jika bars[i].Low < min(bars[i-5..i-1]) AND bars[i].Low < min(bars[i+1..i+5])
```

### FVG Detection
```
For i = 2 to len(bars)-1:
  if bars[i-2].High < bars[i].Low:  // bullish FVG
    fvg = {kind: BULLISH, top: bars[i].Low, bottom: bars[i-2].High}
  if bars[i-2].Low > bars[i].High:  // bearish FVG
    fvg = {kind: BEARISH, top: bars[i-2].Low, bottom: bars[i].High}
```

### Order Block Detection
```
For each swing high:
  Find last bearish candle before swing low that starts impulsive move
  OB = that candle's range (High-Low)
For each swing low:
  Find last bullish candle before swing high
  OB = that candle's range
```

## Telegram Command `/ict`

Output format:
```
🔷 ICT/SMC ANALYSIS — EURUSD H4
📅 2026-04-01 | Killzone: 🇺🇸 New York Session

📐 MARKET STRUCTURE: BULLISH
  ✅ BOS at 1.0850 (last swing high broken)
  ⚠️  CHoCH at 1.0780 (bullish reversal confirmed)

📦 ACTIVE ORDER BLOCKS (3)
  🟢 Bullish OB: 1.0820-1.0835 (valid, not broken)
  🔴 Bearish OB: 1.0920-1.0935 (broken → Breaker)

⬜ FAIR VALUE GAPS (2)
  ⬆️  Bullish FVG: 1.0845-1.0860 (25% filled)
  ⬇️  Bearish FVG: 1.0910-1.0925 (100% filled ✓)

💧 LIQUIDITY SWEEPS (1)
  🔻 Sweep Low at 1.0770 → Reversed BULLISH

🎯 SUMMARY: Strong bullish SMC setup. Price swept lows,
   CHoCH confirmed, active bullish OB at 1.0820-1.0835.
   Watch for FVG fill entry at 1.0845.
```

Keyboard:
```
[H1] [H4] [D1]
[🔄 Refresh] [📊 CTA] [<< Back]
```

## Test Cases

1. Codebase harus compile (`go build ./...`)
2. Unit test: `ict_test.go` — test FVG detection dengan synthetic OHLCV data
3. Unit test: Order Block detection dengan known setups
4. Unit test: CHOCH detection
5. Handler test: command `/ict EURUSD` tidak panic

## Acceptance Criteria

- [ ] Engine detect FVG dengan akurasi tinggi (false positive < 30%)
- [ ] Order Block tidak overlap dengan FVG yang sama
- [ ] CHOCH hanya terdeteksi SEKALI per trend change (bukan setiap swing)
- [ ] Output Telegram terbaca di mobile (tidak lebih dari 3000 chars)
- [ ] Compile tanpa error
- [ ] Min 3 unit tests pass
