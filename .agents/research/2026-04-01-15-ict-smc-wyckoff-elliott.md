# Research Report: ICT/SMC/Wyckoff/Elliott Wave — Cycle 3

**Date:** 2026-04-01 15:45 WIB
**Fokus:** Siklus 3 — Fitur Baru (ICT, SMC, Wyckoff, Elliott Wave, VWAP)
**Agent:** Research

---

## Ringkasan Eksekutif

Codebase sudah memiliki TA engine yang solid (RSI, MACD, Bollinger, Ichimoku, SuperTrend, Fibonacci, GARCH, HMM, Hurst, Candlestick Patterns, Multi-Timeframe). Namun konsep-konsep **ICT, SMC, Wyckoff, dan VWAP** belum ada sama sekali — ini gap terbesar untuk trader institusional forex yang familiar dengan Smart Money metodologi.

---

## Analisis Gap Berdasarkan Codebase

### Yang Sudah Ada (internal/service/ta/)
- `indicators.go`: RSI, MACD, Stochastic, Bollinger, EMA, ADX, OBV, Williams %R, CCI, MFI, Ichimoku, SuperTrend
- `fibonacci.go`: Auto swing detection + retracement (5-bar pivot strength)
- `patterns.go`: Candlestick patterns
- `divergence.go`: RSI/MACD divergence regular + hidden
- `confluence.go`: Multi-indicator scoring -100 to +100
- `mtf.go`: Multi-timeframe alignment matrix
- `zones.go`: Entry/exit zones + R:R calculator
- `price/hmm_regime.go`: HMM 3-state (RISK_ON/OFF/CRISIS)
- `price/hurst.go`: Hurst exponent
- `price/garch.go`: GARCH(1,1) vol forecast
- `microstructure/engine.go`: Bybit orderbook/taker/OI/funding/LS ratio

### Yang Belum Ada — Gap Cycle 3
1. ICT: Fair Value Gap, Order Block, Breaker Block, Liquidity Sweep, Killzone, PD Array
2. SMC: BOS, CHOCH, market structure, premium/discount/equilibrium zones
3. Wyckoff: Phase detection, Spring, Upthrust, Volume drying analysis
4. VWAP: Anchored VWAP + deviation bands (intraday)
5. Elliott Wave: Simplified 5-3 wave counting + Fib validation

---

## Prioritas Implementasi

### HIGH — Immediate value, pure Go, no new deps

**ICT Fair Value Gap + Order Block (ict.go)**
- FVG algorithm: 3-candle gap structure
  - Bullish FVG: bars[i-1].High < bars[i+1].Low (buy imbalance)
  - Bearish FVG: bars[i+1].High < bars[i-1].Low (sell imbalance)
  - Track fill status: price returned to close gap
- Order Block: last opposite-color candle before impulsive move
  - Bullish OB: last bearish candle before bullish impulse
  - Bearish OB: last bullish candle before bearish impulse
  - Mitigation: price returns to OB zone
- Breaker Block: mitigated OB that flips polarity
- Liquidity Sweep: price briefly takes equal highs/lows then reverses

**SMC Structure Analysis (smc.go)**
- Reuse existing 5-bar pivot from fibonacci.go
- BOS: close above prev swing high (bullish BOS) or below swing low (bearish BOS)
- CHOCH: BOS opposite to current trend = potential reversal
- Premium zone: > 61.8% Fib of last swing range
- Discount zone: < 38.2% Fib of last swing range
- Equilibrium: 50% midpoint

### MEDIUM

**Anchored VWAP (vwap.go)**
- VWAP = cumsum(TypicalPrice × Volume) / cumsum(Volume)
- TypicalPrice = (H+L+C)/3
- Anchor: daily open, weekly open, or user-specified swing
- VWAP Bands: ±1σ, ±2σ using rolling std dev of TP
- Uses existing IntradayStore data

**Wyckoff Phase Detection (wyckoff.go)**
- Phase A: Selling climax (SC) + automatic rally (AR) — defines range
- Phase B: Building cause — volume declining in range
- Phase C: Spring (test of support below SC low) or UTAD
- Phase D: BOS above range top (sign of strength)
- Phase E: Markup out of range
- Uses: volume patterns + ATR normalization

### LOW (Future)
- Elliott Wave: too subjective for automation at this stage
- GEX/Options Flow: requires paid data
- WD-GAN: ML overhead not justified yet

---

## File Structure Recommendation

```
internal/service/ta/
├── ict.go      # ICT: FVG, Order Block, Breaker Block, Liquidity Sweep, Killzone
├── smc.go      # SMC: BOS, CHOCH, Market Structure, Premium/Discount zones
├── wyckoff.go  # Wyckoff: Phase detection, Spring/Upthrust
└── vwap.go     # VWAP: Anchored VWAP + deviation bands
```

### engine.go additions:
```go
type FullResult struct {
    // existing fields...
    ICT     *ICTResult     // FVG, Order Blocks, Breaker Blocks
    SMC     *SMCResult     // BOS, CHOCH, Market Structure
    Wyckoff *WyckoffResult // Phase, Spring/Upthrust
    // VWAP computed separately (needs volume from intraday)
}
```

---

## Telegram UX — /smc command (new)

```
📐 SMC/ICT Analysis [EURUSD 4H]

🏗 Market Structure:
  • Trend: BULLISH (BOS ↑ at 1.0820 — 3 bars ago)
  • Last CHOCH: BEARISH at 1.0780 (12 bars ago, overridden)

⚡ Fair Value Gaps:
  • Bullish FVG: 1.0835-1.0852 (unfilled, magnet zone)
  • Bearish FVG: 1.0878-1.0885 (80% filled)

🔲 Order Blocks:
  • Bullish OB: 1.0818-1.0825 (unmitigated ✅)
  • Bearish OB: 1.0892-1.0900 (mitigated, broken 🔴)
  • Breaker: 1.0875-1.0882 (former bullish OB, now bearish)

💧 Liquidity:
  • Buy-side: 1.0905 (equal highs, sweep target)
  • Sell-side: 1.0798 (equal lows, sweep risk)

📊 Zone: PREMIUM (price at 68% of range)
   → Smart money likely distributing / look for shorts
```

### /cta enhancement
Tambah section ICT/SMC setelah existing output.

---

## Implementasi Notes

### ICT Killzone (FX-specific)
- Asian Session: 00:00-03:00 UTC
- London Open: 08:00-10:00 UTC
- NY Open: 13:00-15:00 UTC
- Killzone = high-probability ICT entry windows
- Detect using bar timestamps (WIB timezone already in pkg/timeutil)

### Liquidity Sweep Detection
- Equal highs: 3+ swing highs within ATR*0.15 of each other
- Equal lows: 3+ swing lows within ATR*0.15 of each other
- Sweep: price briefly breaks equal highs/lows then closes back inside

---

## Kesimpulan & Next Steps

5 task specs dibuat:
- TASK-035: ICT FVG + Order Block engine (HIGH)
- TASK-036: SMC BOS/CHOCH market structure (HIGH)
- TASK-037: /smc Telegram command (wires 035+036) (HIGH)
- TASK-038: Anchored VWAP + deviation bands (MEDIUM)
- TASK-039: Wyckoff phase detection (MEDIUM)
