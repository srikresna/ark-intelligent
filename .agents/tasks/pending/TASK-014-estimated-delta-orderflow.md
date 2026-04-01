# TASK-014: Estimated Delta / Order Flow Analysis dari OHLCV

**Priority:** MEDIUM  
**Cycle:** Siklus 3 — Fitur Baru  
**Estimated Complexity:** MEDIUM  
**Research Ref:** `.agents/research/2026-04-01-09-ict-smc-wyckoff-elliott-features.md`

---

## Deskripsi

Implementasi estimated delta dan order flow analysis menggunakan data OHLCV. Karena tick data tidak tersedia untuk forex, gunakan "tick rule" dan OHLCV approximation untuk menghasilkan Delta signal yang berguna. Untuk crypto, gunakan Bybit trades data yang sudah ada (GetRecentTrades).

## Konteks Teknis

### Existing Resources
- `internal/service/microstructure/engine.go` — sudah ada TakerBuyRatio dari Bybit trades
- `internal/service/marketdata/bybit/client.go` — GetRecentTrades() sudah ada
- `[]ta.OHLCV` — price data sudah ada dengan Volume field

### File yang Perlu Dibuat
```
internal/service/orderflow/
├── types.go       ← DeltaBar, OrderFlowResult structs  
├── engine.go      ← Engine.Analyze(bars []ta.OHLCV) *OrderFlowResult
├── delta.go       ← Estimated delta dari OHLCV (tick rule estimation)
├── poc.go         ← Point of Control dari volume distribution
└── absorption.go  ← Absorption pattern detection
```

### File yang Perlu Dimodifikasi
- `internal/adapter/telegram/handler_alpha.go` — tambah `/orderflow` command
- `internal/adapter/telegram/formatter.go` — FormatOrderFlowResult()

## Spesifikasi

### Delta Estimation (OHLCV-based)
```go
// Tick Rule Estimation:
// Jika bar bullish (close > open):
//   EstBuyVol = Volume × (Close - Low) / (High - Low)
//   EstSellVol = Volume - EstBuyVol
// Jika bar bearish:
//   EstSellVol = Volume × (High - Close) / (High - Low)
//   EstBuyVol = Volume - EstSellVol

type DeltaBar struct {
    OHLCV      ta.OHLCV
    BuyVol     float64  // estimated buy volume
    SellVol    float64  // estimated sell volume
    Delta      float64  // BuyVol - SellVol (positive = buyer dominated)
    CumDelta   float64  // cumulative delta from start of lookback
}
```

### Key Order Flow Signals
```go
type OrderFlowResult struct {
    Symbol        string
    Timeframe     string
    Bars          []DeltaBar
    
    // Divergence signals
    PriceDeltaDivergence string  // "BULLISH_DIV" | "BEARISH_DIV" | "NONE"
    // Price new high but delta lower high = bearish divergence
    // Price new low but delta higher low = bullish divergence
    
    // POC (Point of Control) — price level dengan volume terbesar
    PointOfControl float64
    
    // Absorption patterns  
    BullishAbsorption []int  // bar indices di mana selling terserap buyers
    BearishAbsorption []int  // bar indices di mana buying terserap sellers
    
    // Overall delta trend
    DeltaTrend string  // "RISING" | "FALLING" | "FLAT"
    CumDelta   float64 // total cumulative delta
    
    Bias       string  // "BULLISH" | "BEARISH" | "NEUTRAL"
    Summary    string
    AnalyzedAt time.Time
}
```

### Absorption Detection
```go
// Bullish Absorption:
// Bar dengan significant selling (down bar, besar range) TAPI volume sangat tinggi
// Dan candle TIDAK turun jauh = buyers menyerap supply
// Signal: potential reversal bullish

// Bearish Absorption:
// Bar dengan significant buying (up bar) TAPI volume sangat tinggi
// Dan candle TIDAK naik jauh = sellers menyerap demand
```

### Delta Divergence
```go
// Bearish Divergence:
// Price makes higher high, tapi Cumulative Delta makes lower high
// = selling tersamarkan oleh passive orders
// Signal: potential reversal bearish

// Bullish Divergence:
// Price makes lower low, tapi Cumulative Delta makes higher low  
// = buying underlying meskipun harga turun
// Signal: potential reversal bullish
```

## Telegram Command `/orderflow`

```
📊 ORDER FLOW — EURUSD H4 (14 bars)

⚡ DELTA: BULLISH DIVERGENCE ⬆️
  Price: Lower Low (1.0840 < 1.0860 sebelumnya)
  Cum. Delta: Higher Low (+12,430 vs +8,200)
  → Buyers menyerap tekanan jual

📊 DELTA BARS (last 5):
  1.0890 ▲ +5,240 🟢 (buyers dominate)
  1.0870 ▼ -3,120 🔴
  1.0855 ▼ -1,890 🔴 (low delta = absorption?)
  1.0840 ▼ -4,560 🔴
  1.0850 ▲ +6,780 🟢 ← current

🎯 POINT OF CONTROL: 1.0875 (highest volume zone)

🔰 ABSORPTION DETECTED:
  Bar at 1.0840: High volume (-4,560 delta) but limited range
  → Potential bullish absorption — sellers running out

💡 SUMMARY: Delta divergence bullish terkonfirmasi. Buyers
   mulai mengontrol pada level 1.0840-1.0850. Watch untuk
   momentum shift ke atas.
```

## Acceptance Criteria

- [ ] Compile tanpa error
- [ ] Delta estimation logically consistent dengan OHLCV direction
- [ ] Divergence detection tidak produce false positives pada ranging market
- [ ] POC calculation akurat (highest volume level)
- [ ] Output < 3000 chars
- [ ] Min 3 unit tests

