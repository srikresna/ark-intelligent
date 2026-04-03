# TASK-039: Wyckoff Accumulation/Distribution Phase Detection

**Priority:** MEDIUM
**Cycle:** 3 (Fitur Baru)
**Estimated effort:** L (5-7 hours)
**Branch target:** agents/main

---

## Context

Wyckoff Method adalah framework analisis pasar yang dikembangkan Richard Wyckoff (1930s).
Fokus pada identifikasi fase akumulasi dan distribusi "smart money" (composite operator)
sebelum pergerakan besar. Sangat relevan untuk forex institutional trading.

Implementasi ini menggunakan pendekatan simplified (bukan full Wyckoff schematic)
tapi cukup untuk memberikan nilai praktis kepada trader.

---

## Objective

Buat `internal/service/ta/wyckoff.go` dengan:
1. **Phase Detection**: Accumulation, Markup, Distribution, Markdown, Transition
2. **Key Event Detection**: Selling Climax (SC), Automatic Rally (AR), Spring, Upthrust After Distribution (UTAD), Sign of Strength (SOS), Sign of Weakness (SOW)
3. **Trading Range** identification
4. **Volume Analysis** dalam range (drying = accumulation, expanding = markup)
5. **Effort vs Result** divergence detection

---

## Acceptance Criteria

### Types

```go
// WyckoffResult holds Wyckoff analysis output.
type WyckoffResult struct {
    Phase        WyckoffPhase    // current detected phase
    PhaseConf    float64         // confidence 0-1
    TradingRange *TradingRange   // nil if not in a range
    KeyEvents    []WyckoffEvent  // recent key events (newest first)
    Bias         string          // "BULLISH", "BEARISH", "NEUTRAL"
    Description  string          // human-readable phase explanation
}

type WyckoffPhase string
const (
    PhaseAccumulation  WyckoffPhase = "ACCUMULATION"   // Phase A-E (classic acc)
    PhaseMarkup        WyckoffPhase = "MARKUP"          // trending up after acc
    PhaseDistribution  WyckoffPhase = "DISTRIBUTION"   // Phase A-E (classic dist)
    PhaseMarkdown      WyckoffPhase = "MARKDOWN"        // trending down after dist
    PhaseTransition    WyckoffPhase = "TRANSITION"      // unclear / changing
)

type TradingRange struct {
    High        float64  // resistance (top of range)
    Low         float64  // support (bottom of range)
    MidPoint    float64  // (High+Low)/2
    Width       float64  // ATR-normalized width
    BarsInRange int      // how many bars price has stayed in range
    VolumeDecl  bool     // is volume declining in range (accumulation signal)
}

type WyckoffEvent struct {
    Name     string  // "SC", "AR", "ST", "SPRING", "SOS", "SOW", "UT", "UTAD"
    BarIndex int     // newest-first
    Price    float64
    Volume   float64 // relative to 20-bar avg
    Signal   string  // "BULLISH", "BEARISH"
}
```

### Phase Detection Algorithm (Simplified)

**Step 1: Identify Trading Range**
```
- Find recent swing high and swing low (20-bar window)
- Range = SwingHigh - SwingLow
- Price is "in range" if it has not broken out by > ATR*0.5 for > 10 bars
- TradingRange.Width = Range / ATR (normalized)
- Range valid if Width >= 2.0 (at least 2 ATR wide)
```

**Step 2: Volume Analysis in Range**
```
- Compute 20-bar average volume
- Compute 5-bar rolling average for recent bars
- VolumeDecl = recent 5-bar avg < 0.7 * 20-bar avg (drying volume)
- VolumeExpand = recent 5-bar avg > 1.3 * 20-bar avg
```

**Step 3: Key Event Detection**
```
Selling Climax (SC):
  - Large bearish bar (body > 2*ATR)
  - Volume > 2x avg
  - At or below prior swing low

Automatic Rally (AR):
  - After SC: sharp bounce (>1 ATR up)
  - Defines top of accumulation range

Spring:
  - Price briefly pierces below range low (SC level)
  - Closes back above range low within 1-2 bars
  - Volume should be LOWER than during SC (test failed, smart money absorbing)

Sign of Strength (SOS):
  - Strong bullish bar breaking above AR high
  - Volume > 1.5x avg
  - → Confirms end of accumulation, markup starting

Upthrust (UT):
  - Price briefly pierces above range high
  - Closes back inside range within 1-2 bars
  - → Distribution version of Spring

Sign of Weakness (SOW):
  - Strong bearish bar breaking below spring low
  - → Confirms distribution ending, markdown starting
```

**Step 4: Phase Classification**
```
ACCUMULATION:
  - Price in trading range
  - VolumeDecl in range
  - SC detected + AR defined
  - Spring detected (optional, confirms phase C)
  - Bias: BULLISH (looking for SOS to confirm markup)

MARKUP:
  - Price broke above range high (SOS confirmed)
  - Volume expanding on upside
  - Bias: BULLISH

DISTRIBUTION:
  - Price in trading range AFTER markup phase
  - UT detected (failed test of range high)
  - Volume declining in range
  - Bias: BEARISH (looking for SOW to confirm markdown)

MARKDOWN:
  - Price broke below range low (SOW confirmed)
  - Volume expanding on downside
  - Bias: BEARISH

TRANSITION:
  - Insufficient evidence or conflicting signals
```

### Effort vs Result

```go
// EffortResult measures if volume effort matches price result.
// High volume + small price move = absorption (bearish for distribution, bullish for acc)
// Low volume + large price move = ease of movement (trend continuation)
type EffortResult struct {
    BarIndex     int
    Effort       float64 // volume relative to 20-bar avg
    Result       float64 // price move relative to ATR
    Divergence   bool    // true if high effort + low result (absorption)
    Type         string  // "ABSORPTION", "EASE_OF_MOVEMENT", "NORMAL"
}
```

### Function Signature

```go
// CalcWyckoff analyzes Wyckoff phase and key events.
// bars: newest-first, minimum 50 bars recommended for reliable phase detection.
// Returns nil if fewer than 20 bars.
func CalcWyckoff(bars []OHLCV, atr float64) *WyckoffResult
```

### Integration

Add to `FullResult`:
```go
Wyckoff *WyckoffResult
```

Add to `ComputeSnapshot` (requires minimum 50 bars):
```go
if len(bars) >= 50 {
    snap.Wyckoff = CalcWyckoff(bars, atr14)
}
```

Display in `/cta` output (new section):
```
🎯 Wyckoff Phase: ACCUMULATION (conf: 72%)
  Range: 1.0790 – 1.0905 (4.2 ATR wide, 18 bars)
  Events: SC ✓ | AR ✓ | Spring ✓ | SOS pending
  Volume: DECLINING in range (✅ accumulation signal)
  Bias: BULLISH — watch for SOS above 1.0905
```

---

## Tests (`internal/service/ta/wyckoff_test.go`)

1. `TestAccumulationPhase`: synthesize SC + AR + Spring → detect ACCUMULATION
2. `TestMarkupPhase`: SOS + price above range → detect MARKUP
3. `TestDistributionPhase`: upthrust + declining vol → detect DISTRIBUTION
4. `TestSpringDetection`: wick below range low + close inside → Spring event
5. `TestVolumeDecline`: declining volume in range → VolumeDecl=true
6. `TestInsufficientData`: < 20 bars → return nil, no panic

---

## Definition of Done
- [ ] `internal/service/ta/wyckoff.go` compiles
- [ ] `go vet ./internal/service/ta/...` passes
- [ ] All 6 tests pass
- [ ] Phase displayed in `/cta` output
- [ ] Handles edge cases: no range, insufficient bars, zero volume
- [ ] `go build ./...` passes
