# TASK-038: Anchored VWAP + Deviation Bands

**Priority:** MEDIUM
**Cycle:** 3 (Fitur Baru)
**Estimated effort:** S (2-3 hours)
**Branch target:** agents/main

---

## Context

VWAP (Volume Weighted Average Price) adalah benchmark harga institusional.
"Anchored" VWAP memungkinkan trader memilih titik awal (anchor) seperti awal session,
swing low/high, atau tanggal spesifik. Bands ±1σ/±2σ berfungsi mirip Bollinger Bands
tapi berbasis volume-weighted distribution.

Intraday data + volume sudah tersedia di `IntradayStore`.

---

## Objective

Buat `internal/service/ta/vwap.go` dengan:
1. Anchored VWAP computation
2. VWAP deviation bands (±1σ, ±2σ, ±3σ)
3. VWAP position analysis (price above/below VWAP)
4. Multiple anchor presets: Daily, Weekly, Monthly, SwingLow, SwingHigh

---

## Acceptance Criteria

### Types

```go
// VWAPResult holds VWAP computation for a given anchor point.
type VWAPResult struct {
    VWAP        float64 // Volume Weighted Average Price at current bar
    Band1Upper  float64 // VWAP + 1σ
    Band1Lower  float64 // VWAP - 1σ
    Band2Upper  float64 // VWAP + 2σ
    Band2Lower  float64 // VWAP - 2σ
    Band3Upper  float64 // VWAP + 3σ
    Band3Lower  float64 // VWAP - 3σ
    Position    string  // "ABOVE", "BELOW", "AT" (within 0.1% of VWAP)
    Deviation   float64 // current deviation in σ units (signed)
    AnchorType  string  // "DAILY", "WEEKLY", "SWING_LOW", "SWING_HIGH"
    AnchorPrice float64 // price at anchor bar
    BarsUsed    int     // number of bars from anchor to current
}
```

### VWAP Algorithm

```
Input: bars (newest-first) from anchor point to current
Step 1: Reverse to oldest-first for sequential computation
Step 2: For each bar i:
  TypicalPrice[i] = (High[i] + Low[i] + Close[i]) / 3
  cumVolume[i] = cumVolume[i-1] + Volume[i]
  cumTPV[i] = cumTPV[i-1] + TypicalPrice[i] * Volume[i]
  VWAP[i] = cumTPV[i] / cumVolume[i]

Step 3: Compute deviation bands using rolling variance:
  For each bar i:
    variance[i] = Σ(Volume[j] * (TP[j] - VWAP[i])²) / cumVolume[i]
    σ[i] = sqrt(variance[i])
  Band1Upper = VWAP + 1*σ
  Band2Upper = VWAP + 2*σ
  etc.

Step 4: Return VWAPResult for the most recent bar (bars[0])
```

### Anchor Presets

```go
type VWAPAnchor int
const (
    AnchorDaily     VWAPAnchor = iota // midnight UTC of current day
    AnchorWeekly                      // Monday 00:00 UTC of current week
    AnchorMonthly                     // first bar of current calendar month
    AnchorSwingLow                    // auto-detected recent swing low (5-bar pivot)
    AnchorSwingHigh                   // auto-detected recent swing high
)
```

### Function signatures

```go
// CalcVWAP computes anchored VWAP from the given bars.
// bars: newest-first (all bars from anchor point onward).
// Returns nil if fewer than 2 bars or zero cumulative volume.
func CalcVWAP(bars []OHLCV, anchorType string) *VWAPResult

// CalcVWAPAnchored computes VWAP starting from a specific bar index.
// anchorIdx: index in bars slice (newest-first) where anchor begins.
func CalcVWAPAnchored(bars []OHLCV, anchorIdx int, anchorType string) *VWAPResult
```

### ComputeVWAPSet (multiple anchors)

```go
// VWAPSet holds VWAP calculations for multiple anchor points simultaneously.
type VWAPSet struct {
    Daily   *VWAPResult
    Weekly  *VWAPResult
    SwingLow  *VWAPResult // nil if no swing low found in data
    SwingHigh *VWAPResult // nil if no swing high found in data
}

// CalcVWAPSet computes VWAP for all standard anchor presets.
func CalcVWAPSet(bars []OHLCV) *VWAPSet
```

### Integration

VWAP is computed separately from main ComputeSnapshot because it requires:
- Full intraday bar history (not just last N bars)
- Volume data (bars must have non-zero Volume)

Add to handler_cta.go / handler_smc.go:
```go
// After fetching bars, compute VWAP set
if hasVolume(bars) {
    state.vwap = ta.CalcVWAPSet(bars)
}
```

Display in `/cta` output as additional section:
```
📏 VWAP (Daily Anchor)
  VWAP: 1.08320
  Position: ABOVE (+0.8σ) — mild bullish bias
  +1σ: 1.08450 | -1σ: 1.08190
  +2σ: 1.08580 | -2σ: 1.08060
```

---

## Tests (`internal/service/ta/vwap_test.go`)

1. `TestVWAPBasic`: uniform price + volume → VWAP = price
2. `TestVWAPBands`: known data → verify band calculation
3. `TestVWAPPosition`: price above VWAP → Position="ABOVE"
4. `TestVWAPZeroVolume`: volume=0 bars → handle gracefully (return nil or use equal weights)
5. `TestVWAPSet`: multi-anchor set from synthetic data

---

## Definition of Done
- [ ] `internal/service/ta/vwap.go` compiles
- [ ] `go vet ./internal/service/ta/...` passes
- [ ] All tests pass
- [ ] VWAPSet displayed in `/cta` output (when intraday data available)
- [ ] Graceful degradation when volume data is zero/missing
