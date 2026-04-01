# TASK-036: SMC Market Structure — BOS / CHOCH / Premium-Discount Zones

**Priority:** HIGH
**Cycle:** 3 (Fitur Baru)
**Estimated effort:** M (3-5 hours)
**Branch target:** agents/main
**Depends on:** TASK-035 (reuses swing pivot logic from fibonacci.go)

---

## Context

Smart Money Concepts (SMC) adalah framework analisis berbasis institutional order flow.
BOS (Break of Structure) dan CHOCH (Change of Character) adalah building blocks dari
market structure analysis. Banyak trader ICT menggunakan ini sebagai filter utama
sebelum mencari entry di OB/FVG.

---

## Objective

Buat `internal/service/ta/smc.go` dengan:
1. **BOS** (Break of Structure) detection — trend confirmation
2. **CHOCH** (Change of Character) — reversal signal
3. **Market Structure** state machine (BULLISH / BEARISH / RANGING)
4. **Premium / Discount / Equilibrium** zones dari swing range
5. **Internal vs External** liquidity identification

---

## Acceptance Criteria

### Types

```go
// SMCResult holds Smart Money Concepts analysis.
type SMCResult struct {
    Structure      MarketStructure   // current overall structure
    RecentBOS      []StructureEvent  // last 5 BOS events, newest first
    RecentCHOCH    []StructureEvent  // last 3 CHOCH events, newest first
    PremiumZone    float64           // level above which is premium
    DiscountZone   float64           // level below which is discount
    Equilibrium    float64           // 50% of last significant swing
    CurrentZone    string            // "PREMIUM", "DISCOUNT", "EQUILIBRIUM"
    InternalLiq    []LiqRange        // internal liquidity pools
    Trend          string            // "BULLISH", "BEARISH", "RANGING"
    LastSwingHigh  float64
    LastSwingLow   float64
}

type MarketStructure string
const (
    StructureBullish MarketStructure = "BULLISH"  // HH + HL pattern
    StructureBearish MarketStructure = "BEARISH"  // LH + LL pattern
    StructureRanging MarketStructure = "RANGING"  // no clear HH/HL or LH/LL
)

type StructureEvent struct {
    Type     string  // "BOS" or "CHOCH"
    Dir      string  // "BULLISH" or "BEARISH"
    Price    float64 // level that was broken
    BarIndex int     // index in bars slice (newest-first)
    Impulse  float64 // size of move after break (ATR multiples)
}

type LiqRange struct {
    High    float64
    Low     float64
    Type    string  // "INTERNAL" or "EXTERNAL"
    Swept   bool
}
```

### Algorithm

**Swing Detection** (reuse logic from fibonacci.go CalcFibonacci):
- Swing High: bar[i].High > max(bar[i-n..i-1].High) AND > max(bar[i+1..i+n].High), n=3
- Swing Low: bar[i].Low < min of n bars each side, n=3

**BOS Detection**:
```
Maintain list of recent swing highs and lows (newest-first)
For each new bar:
  Bullish BOS: bar.Close > prev_swing_high.Price
    → confirms bullish structure
    → update last swing high reference
  Bearish BOS: bar.Close < prev_swing_low.Price
    → confirms bearish structure
    → update last swing low reference
```

**CHOCH Detection**:
```
Current structure = BULLISH (making HH+HL)
If price makes Bearish BOS (breaks last HL):
  → CHOCH detected (bearish reversal signal)
  → Structure shifts to BEARISH

Current structure = BEARISH (making LH+LL)  
If price makes Bullish BOS (breaks last LH):
  → CHOCH detected (bullish reversal signal)
  → Structure shifts to BULLISH
```

**Premium / Discount Zones**:
```
Use last significant swing (defined as swing with impulse > 1.5*ATR):
  Range = SwingHigh - SwingLow
  Equilibrium (EQ) = SwingLow + Range*0.5
  Premium Zone top boundary = SwingLow + Range*0.618   (golden pocket)
  Premium Zone = price > EQ (above 50%)
  Discount Zone = price < EQ (below 50%)
  Optimal Trade Entry (OTE): 61.8%-79% Fib = high-probability zone
```

**Market Structure State**:
```
Track last 4 swing points:
  HH+HL pattern → BULLISH
  LH+LL pattern → BEARISH
  Mixed or insufficient data → RANGING
```

### CalcSMC function
```go
// CalcSMC computes SMC market structure for the given bars.
// bars: newest-first, minimum 30 bars recommended.
// atr: ATR(14) for impulse sizing.
func CalcSMC(bars []OHLCV, atr float64) *SMCResult
```

### Integration
Add to `FullResult`:
```go
SMC *SMCResult // computed in ComputeSnapshot
```

Add to engine.go `ComputeSnapshot`:
```go
snap.SMC = CalcSMC(bars, atr14)
```

---

## Tests (`internal/service/ta/smc_test.go`)

1. `TestBOSBullish`: series of bars making HH → detect bullish BOS
2. `TestBOSBearish`: series of bars making LL → detect bearish BOS
3. `TestCHOCH`: bullish trend → bearish BOS → CHOCH detected
4. `TestPremiumZone`: price above 50% of range → CurrentZone="PREMIUM"
5. `TestDiscountZone`: price below 50% → CurrentZone="DISCOUNT"
6. `TestStructureRanging`: mixed swings → RANGING

---

## Definition of Done
- [ ] `internal/service/ta/smc.go` compiles clean
- [ ] `go vet ./internal/service/ta/...` passes
- [ ] All tests pass
- [ ] `FullResult.SMC` populated in ComputeSnapshot
- [ ] No new external dependencies
