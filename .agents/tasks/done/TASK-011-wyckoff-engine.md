# TASK-011: Wyckoff Structure Detection Engine

**Priority:** MEDIUM-HIGH  
**Cycle:** Siklus 3 — Fitur Baru  
**Estimated Complexity:** HIGH  
**Research Ref:** `.agents/research/2026-04-01-09-ict-smc-wyckoff-elliott-features.md`

---

## Deskripsi

Implementasi engine Wyckoff Method untuk mendeteksi Accumulation dan Distribution schematics pada forex/crypto. Engine harus mengidentifikasi phase Wyckoff (A, B, C, D, E) dan event penting (SC, AR, ST, Spring, SOS, LPS, UTAD, SOW, dll) dari data OHLCV + volume.

## Konteks Teknis

### Data Source
- `[]ta.OHLCV` dengan Volume — sudah tersedia dari `service/price`
- Volume Profile (`scripts/vp_engine.py`) bisa menjadi complement

### File yang Perlu Dibuat
```
internal/service/wyckoff/
├── types.go      ← WyckoffEvent, WyckoffPhase, WyckoffResult structs
├── engine.go     ← Engine.Analyze(bars []ta.OHLCV) *WyckoffResult
├── events.go     ← Deteksi individual events (SC, AR, ST, Spring, dll)
├── phase.go      ← Phase identification (A→B→C→D→E)
└── classifier.go ← Accumulation vs Distribution classifier
```

### File yang Perlu Dimodifikasi
- `internal/adapter/telegram/handler_alpha.go` — tambah `/wyckoff` command
- `internal/adapter/telegram/formatter.go` — FormatWyckoffResult()
- `internal/adapter/telegram/bot.go` — wire Wyckoff engine

## Spesifikasi

### Types
```go
type WyckoffEvent struct {
    Name        string  // "SC", "AR", "ST", "SPRING", "SOS", "LPS", "UTAD", "SOW"
    BarIndex    int
    Price       float64
    Volume      float64
    Significance string  // "HIGH", "MEDIUM", "LOW"
    Description string
}

type WyckoffPhase struct {
    Phase  string  // "A", "B", "C", "D", "E"
    Start  int     // bar index start
    End    int     // bar index end (-1 = ongoing)
    Events []WyckoffEvent
}

type WyckoffResult struct {
    Symbol        string
    Timeframe     string
    Schematic     string   // "ACCUMULATION" | "DISTRIBUTION" | "UNKNOWN"
    CurrentPhase  string   // "A", "B", "C", "D", "E", "UNDEFINED"
    Events        []WyckoffEvent
    TradingRange  [2]float64  // [support, resistance] of current range
    CauseBuilt    float64     // estimate of "cause" built (higher = more energy)
    ProjectedMove float64     // estimated magnitude of breakout move
    Confidence    string      // "HIGH", "MEDIUM", "LOW"
    Summary       string
    AnalyzedAt    time.Time
}
```

### Algoritma Deteksi

**Phase A — Stopping the Prior Trend:**
```
Preliminary Support (PS): Volume spike + price deceleration in downtrend
Selling Climax (SC): Highest volume bar in downtrend + large range candle
Automatic Rally (AR): Rally dari SC, mostly covering shorts
Secondary Test (ST): Low volume test of SC lows (low volume = bullish)
```

**Phase B — Building the Cause:**
```
Trading range established between AR high dan SC low
Multiple tests of range boundaries
Volume generally decreases toward end of Phase B
```

**Phase C — The Test (Spring/UTAD):**
```
Spring (Accumulation): Brief break below SC low + rapid recovery
  - Volume RENDAH saat break = good spring (no supply)
  - Volume TINGGI saat break = bad spring (supply still present)
UTAD (Distribution): Brief break above range high + rapid drop back
```

**Phase D — Dominance:**
```
Sign of Strength (SOS): Break above AR high dengan volume tinggi
Last Point of Support (LPS): Low volume pullback ke prior resistance (now support)
```

**Volume Analysis:**
```
High Volume at Low Price = Absorption (accumulation signal)
Low Volume at Low Price = Lack of Demand (bearish) OR End of Supply (bullish)
High Volume Breakout = Confirmed move
```

## Telegram Command `/wyckoff`

Output format:
```
📊 WYCKOFF ANALYSIS — EURUSD H4

🏗️ SCHEMATIC: ACCUMULATION (65% confidence)
📍 CURRENT PHASE: C — Spring Test

⚡ EVENTS DETECTED:
  [A] 🔻 Selling Climax: 1.0750 (3/15, vol 2.4x avg)
  [A] 📈 Automatic Rally: 1.0890 (3/17)
  [A] 🔻 Secondary Test: 1.0765 (3/22, low vol ✅)
  [B] 📊 Range Trading: 1.0760 - 1.0885
  [C] 💎 Spring: 1.0745 (3/31, low vol ✅ BULLISH)

📏 TRADING RANGE: 1.0760 — 1.0885
🎯 PROJECTED MOVE: +135 pips (cause built)
⚡ NEXT TO WATCH: Sign of Strength (break above 1.0885)

💡 SUMMARY: Classic Wyckoff accumulation. Spring confirmed
   dengan volume rendah. Wait for SOS break above 1.0885.
```

## Acceptance Criteria

- [ ] Compile tanpa error
- [ ] Deteksi Phase A events (SC, AR, ST) pada historical data yang diketahui
- [ ] Volume analysis akurat (bukan hanya price)
- [ ] Output tidak lebih dari 4000 chars
- [ ] Min 3 unit tests
- [ ] Confidence score "LOW" jika data kurang dari 100 bars

