# TASK-013: Elliott Wave ABC Corrective Wave Count (Phase 2)

**Priority:** MEDIUM
**Type:** Feature Enhancement
**Estimated effort:** L (1.5-2 days)
**Ref:** research/2026-04-06-11-feature-deep-dive-siklus3.md

---

## Context

Package header `internal/service/elliott/engine.go` menyatakan secara eksplisit:
> "Only Phase 1 (MVP) is implemented: ZigZag swing detection, Basic 5-wave impulse
> identification, Rule validation (3 rules), Simple Fibonacci targets"

**Phase 2 = ABC Corrective wave count** — ini setengah dari Elliott Wave theory.

Tanpa corrective count, `/elliott` hanya bisa menghitung trend impulse (5 waves).
Trader tidak bisa tahu apakah harga sedang dalam Wave 2 atau Wave 4 correction,
atau apakah correction sudah selesai.

**Dua jenis corrective yang paling penting (implementation scope):**
1. **ZigZag (5-3-5)**: Sharp correction. Wave A = 5 waves down, B = 3 waves up, C = 5 waves down.
2. **Flat (3-3-5)**: Sideways correction. Wave A = 3, B = 3, C = 5. B nearly equals A start.

---

## Implementation

### Step 1: Add corrective wave types to `internal/service/elliott/types.go`

```go
// CorrectivePattern identifies the ABC corrective structure type.
type CorrectivePattern string

const (
    CorrectiveZigZag  CorrectivePattern = "ZIGZAG"   // 5-3-5 sharp correction
    CorrectiveFlat    CorrectivePattern = "FLAT"      // 3-3-5 sideways correction
    CorrectiveUnknown CorrectivePattern = "UNKNOWN"
)

// CorrectiveResult holds an identified ABC corrective count.
type CorrectiveResult struct {
    Symbol    string
    Timeframe string
    Pattern   CorrectivePattern
    Waves     []Wave    // A, B, C (Wave.Number = "A", "B", "C")
    CurrentWave string   // "A", "B", "C", or "POST_C" (complete)
    WaveProgress float64

    // Targets for Wave C completion
    Target1 float64  // 100% of wave A from B endpoint
    Target2 float64  // 161.8% of wave A from B endpoint (extended)

    InvalidationLevel float64 // B exceeds A start = not a ZigZag

    Confidence string  // "HIGH", "MEDIUM", "LOW"
    Summary    string
    AnalyzedAt time.Time
}
```

Add `Corrective *CorrectiveResult` to `WaveCountResult`.

### Step 2: Add `fitCorrective()` to `internal/service/elliott/engine.go`

```go
// fitCorrective attempts to identify an ABC corrective pattern in the most
// recent pivots after a completed or partial impulse.
// Returns nil if no clear corrective pattern is found.
func (e *Engine) fitCorrective(pivots []SwingPoint, bars []ta.OHLCV, symbol, timeframe string) *CorrectiveResult {
    if len(pivots) < 3 {
        return nil
    }
    // Take last 3 pivots as A-B-C candidates
    // Determine direction: if last impulse was bullish, correction is bearish
    // Classify ZigZag vs Flat based on:
    //   ZigZag: B retraces < 100% of A, C makes new extreme beyond A end
    //   Flat: B retraces 90-100%+ of A (nearly returns to A start)
    // Validate: B must not exceed A's start (invalidation)
    // Project C targets using Fibonacci
}
```

### Step 3: Update `Engine.Analyze()` to run corrective detection

After `fitImpulse()`, also run `fitCorrective()`:

```go
func (e *Engine) Analyze(bars []ta.OHLCV, symbol, timeframe string) *WaveCountResult {
    // ... existing impulse code ...
    
    // Phase 2: attempt corrective count on remaining pivots
    result.Corrective = e.fitCorrective(pivots, bars, symbol, timeframe)
    
    return result
}
```

### Step 4: Update handler/formatter

In `internal/adapter/telegram/handler_elliott.go` — display corrective result
if `result.Corrective != nil` alongside the impulse count.

Format example:
```
🌊 <b>Elliott Wave</b> — EURUSD (4H)

📈 <b>Impulse Count:</b> Wave 3 (confidence: MEDIUM)
Target: 1.0950 / 1.1020

📉 <b>Corrective Count:</b> ZigZag ABC (Wave B sedang berjalan)
A: 1.0880 → 1.0750 (−130 pips)
B: 1.0750 → 1.0820 (retracement 54%)
C Target: 1.0688 / 1.0618
Invalidation: jika B > 1.0880
```

---

## Acceptance Criteria

- [ ] `CorrectiveResult` type dan `CorrectivePattern` constants ditambahkan ke `types.go`
- [ ] `WaveCountResult.Corrective *CorrectiveResult` field ditambahkan
- [ ] `fitCorrective()` diimplementasikan untuk ZigZag dan Flat patterns
- [ ] `Engine.Analyze()` memanggil `fitCorrective()` dan menyimpan hasilnya
- [ ] Handler menampilkan corrective count di `/elliott` output jika tersedia
- [ ] Unit test: TestFitCorrective_ZigZag dengan synthetic pivot data
- [ ] Unit test: TestFitCorrective_Flat dengan synthetic pivot data
- [ ] Unit test: TestFitCorrective_Invalidation (B > A start = nil return)
- [ ] Existing impulse tests tetap pass (`go test ./...`)
- [ ] `go build ./...` bersih
