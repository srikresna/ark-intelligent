# TASK-035: ICT Fair Value Gap + Order Block Engine

**Priority:** HIGH
**Cycle:** 3 (Fitur Baru)
**Estimated effort:** M (4-6 hours)
**Branch target:** agents/main

---

## Context

Bot sudah punya TA engine komprehensif tapi belum ada konsep ICT (Inner Circle Trader).
ICT adalah metodologi Smart Money yang sangat populer di kalangan trader forex retail-to-institutional.
FVG dan Order Block adalah dua konsep paling fundamental dalam ICT.

---

## Objective

Buat file `internal/service/ta/ict.go` dengan implementasi:
1. **Fair Value Gap (FVG)** detection
2. **Order Block (OB)** detection
3. **Breaker Block** detection (mitigated OB that flips)
4. **Liquidity Sweep** detection (equal highs/lows sweep)
5. **ICT Killzone** timestamp detection (Asian, London, NY)

---

## Acceptance Criteria

### Types

```go
// ICTResult holds all ICT-derived analysis for a set of OHLCV bars.
type ICTResult struct {
    FairValueGaps  []FVG        // all detected FVGs, sorted newest first
    OrderBlocks    []OrderBlock // all detected OBs, sorted newest first
    BreakerBlocks  []OrderBlock // mitigated OBs that flipped polarity
    LiquidityLevels []LiquidityLevel // equal highs/lows (sweep targets)
    Killzone       string       // "ASIAN", "LONDON", "NY", "OFF" based on bar[0] time
    PremiumZone    bool         // price in premium (above 50% of last range)
    DiscountZone   bool         // price in discount (below 50% of last range)
    Equilibrium    float64      // 50% level of last significant swing
}

type FVG struct {
    High      float64   // top of gap
    Low       float64   // bottom of gap
    Type      string    // "BULLISH" (buy imbalance) or "BEARISH" (sell imbalance)
    BarIndex  int       // index of middle candle (newest-first)
    Filled    bool      // true if price has returned to close the gap
    FillPct   float64   // 0-100% how much of gap is filled
    Midpoint  float64   // (High+Low)/2
}

type OrderBlock struct {
    High      float64
    Low       float64
    Type      string  // "BULLISH" or "BEARISH"
    BarIndex  int     // index of OB candle
    Mitigated bool    // price returned to OB zone
    Broken    bool    // price closed through OB (now a breaker)
    Strength  int     // 1-3: based on impulsive move size after OB
}

type LiquidityLevel struct {
    Price    float64
    Type     string  // "BUY_SIDE" (equal highs) or "SELL_SIDE" (equal lows)
    Swept    bool    // price briefly broke and returned
    Count    int     // number of equal highs/lows clustered
}
```

### FVG Detection Algorithm
```
bars: newest-first []OHLCV
for i from 1 to len(bars)-2:
  prev = bars[i+1]  // older bar
  curr = bars[i]    // middle bar (creates the gap)
  next = bars[i-1]  // newer bar

  // Bullish FVG: gap between prev.High and next.Low
  if prev.High < next.Low AND (next.Low - prev.High) > ATR*0.1:
    → Bullish FVG: Low=prev.High, High=next.Low
    → Check fill: any bar[0..i-1] has Low <= High of FVG (partially/fully fills)

  // Bearish FVG: gap between next.High and prev.Low
  if next.High < prev.Low AND (prev.Low - next.High) > ATR*0.1:
    → Bearish FVG: Low=next.High, High=prev.Low
    → Check fill: any bar[0..i-1] has High >= Low of FVG

Limit: return only last 10 FVGs (5 bullish, 5 bearish)
```

### Order Block Algorithm
```
For bullish OB: find last bearish candle (Close < Open) before a bullish impulse
  - Bullish impulse: 3+ consecutive bullish bars OR single bar with size > 1.5*ATR
  - OB zone: Low to High of that bearish candle
  - Mitigated: price returns to touch OB zone (Low to High)
  - Broken: price closes below OB Low (bullish OB broken = now bearish)

For bearish OB: symmetric logic (last bullish candle before bearish impulse)

Strength scoring:
  1 = impulse move < 1*ATR
  2 = impulse move 1-2*ATR
  3 = impulse move > 2*ATR

Limit: last 5 OBs of each type
```

### Liquidity Sweep
```
Equal Highs: cluster of 3+ swing highs within ATR*0.15 tolerance
Equal Lows: cluster of 3+ swing lows within ATR*0.15 tolerance
Sweep: price bar[0..i] has wick beyond cluster level but CLOSES back inside
→ LiquidityLevel.Swept = true
```

### Killzone Detection
```go
// UTC times (convert bar time to UTC)
func detectKillzone(t time.Time) string {
    h := t.UTC().Hour()
    switch {
    case h >= 0 && h < 3:   return "ASIAN"
    case h >= 8 && h < 10:  return "LONDON"
    case h >= 13 && h < 15: return "NY"
    default:                 return "OFF"
    }
}
```

### CalcICT function signature
```go
// CalcICT computes all ICT structural elements for the given bars.
// bars: newest-first, minimum 20 bars required.
// atr: pre-computed ATR(14) for filtering noise.
// Returns nil if insufficient data.
func CalcICT(bars []OHLCV, atr float64) *ICTResult
```

### Integration into engine.go
Add to `FullResult`:
```go
ICT *ICTResult // computed in ComputeSnapshot
```

Add to `ComputeSnapshot`:
```go
snap.ICT = CalcICT(bars, atr14)
```

---

## Tests

File: `internal/service/ta/ict_test.go`

Test cases:
1. `TestFVGBullish`: 3 bars with clear gap → detect bullish FVG
2. `TestFVGFilled`: same setup + price returns → Filled=true, FillPct=100
3. `TestOrderBlockBullish`: bearish candle + bullish impulse → OB detected
4. `TestOrderBlockMitigated`: OB + price returns to zone → Mitigated=true
5. `TestLiquiditySweep`: 3 equal highs + sweep candle → Swept=true
6. `TestKillzoneDetection`: various UTC hours → correct session labels

---

## Definition of Done
- [ ] `internal/service/ta/ict.go` compiles clean
- [ ] `go vet ./internal/service/ta/...` passes
- [ ] All test cases pass
- [ ] `FullResult.ICT` populated in `ComputeSnapshot`
- [ ] No new external dependencies added
