# TASK-213: Elliott Wave Engine — Automated Wave Counting

**Priority:** medium
**Type:** feature
**Estimated:** L
**Area:** internal/service/elliott/ (NEW)

## Deskripsi

Implementasi Elliott Wave analysis engine. Automated wave counting dari price data menggunakan rule-based approach. Bisa reuse swing detection dari `internal/service/ict/swing.go`.

## Elliott Wave Rules

### Impulse Wave (5-wave)
```
W1: Initial impulse (from trend start)
W2: Retracement of W1 — MUST NOT exceed W1 start (key rule)
    Typical: 38.2% – 61.8% of W1
W3: Strongest wave — MUST be longer than W2 (often 161.8% of W1)
    MUST NOT be shortest of W1, W3, W5
W4: Retracement of W3 — MUST NOT overlap W1 territory
    Typical: 23.6% – 38.2% of W3
W5: Final push — often equals W1 length or 61.8% of W1-W3
```

### Corrective Wave (3-wave ABC)
```
A: First corrective leg
B: Partial retracement of A (typically 50-78.6%)
C: Continuation of A direction (typically 100-161.8% of A)
```

### Wave Degree (scale)
```
Grand Supercycle → Supercycle → Cycle → Primary → Intermediate → Minor → Minute → Minuette
```

## Implementation Strategy

```go
// 1. Reuse swing detection (from ict/swing.go or copy + adapt)
// 2. Find significant pivots (min prominence filter)
// 3. Apply Elliott rules to label waves
// 4. Use Fibonacci ratios to validate relationships
// 5. If impulse found → project W3 target (161.8%) and W5 target
// 6. If corrective found → project C target
// 7. Confidence: HIGH (all rules met) / MEDIUM (most rules) / LOW (ambiguous)
```

## File Changes

- `internal/service/elliott/types.go` — NEW: Wave, WaveCount, ElliottResult
- `internal/service/elliott/swing.go` — NEW: pivot detection (adapted from ict/swing.go)
- `internal/service/elliott/rules.go` — NEW: Elliott rule validator
- `internal/service/elliott/engine.go` — NEW: Engine.Analyze()
- `internal/service/elliott/projections.go` — NEW: Fib-based price projections
- `internal/service/elliott/elliott_test.go` — Unit tests
- `internal/adapter/telegram/handler_ew.go` — NEW: /ew command handler
- `internal/adapter/telegram/formatter_ew.go` — NEW: formatter
- `internal/adapter/telegram/bot.go` — Wire handler

## Output Format

```
📈 Elliott Wave — EURUSD H4
━━━━━━━━━━━━━━━
Pattern: Impulse (5-wave) — Wave 3 in progress
Confidence: HIGH ✅

Wave count:
  W1: 1.0650 → 1.0820 (+170 pips)
  W2: 1.0820 → 1.0740 (retrace 47%) ✅
  W3: 1.0740 → [CURRENT 1.0895] (+155 pips, 91% of W1)
  W4: expected 1.0830-1.0860 (38.2-50% retrace)
  W5: target 1.0920-1.0960

🎯 W3 extension target: 1.0960 (161.8%)
⚠️  Invalidation: Below 1.0740 (W2 low)
```

## Acceptance Criteria

- [ ] Detect 5-wave impulse with all three core rules validated
- [ ] Detect 3-wave ABC correction
- [ ] Fibonacci ratios used to validate each wave relationship
- [ ] Price projections for next wave (W3 extension, W5, C)
- [ ] Confidence score (HIGH/MEDIUM/LOW)
- [ ] /ew SYMBOL TIMEFRAME command working
- [ ] At least 5 unit tests (valid impulse, invalid impulse, ABC correction)
- [ ] go build ./... clean
