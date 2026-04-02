# TASK-160: Elliott Wave Automated Counter

**Priority:** high
**Type:** feature
**Estimated:** L
**Area:** internal/service/ta/

## Deskripsi

Implementasi automated Elliott Wave counting — 5-wave impulse + 3-wave corrective pattern detection dengan rule enforcement dan target projection. Reuse existing swing detection dari fibonacci.go.

## Detail Teknis

Elliott Wave Rules:
1. Wave 2 tidak retrace >100% Wave 1
2. Wave 3 tidak boleh terpendek dari 1, 3, 5
3. Wave 4 tidak overlap price territory Wave 1
4. Wave 3 target: 1.618x Wave 1 extension
5. Wave 5 target: 0.618x atau 1.0x Wave 1

## File Changes

- `internal/service/ta/elliott_wave.go` — NEW: Wave counter, rule validator, target projector
- `internal/service/ta/models.go` — Add ElliottWaveResult, WaveCount, WaveTarget types
- `internal/adapter/telegram/handler.go` — Add /elliott command routing
- `internal/adapter/telegram/formatter.go` — Add Elliott Wave formatting section
- `internal/adapter/telegram/keyboard.go` — Add Elliott keyboard (timeframe toggle)

## Acceptance Criteria

- [ ] Detect 5-wave impulse patterns dari OHLCV swing points
- [ ] Validate rules (Wave 2, 3, 4 rules)
- [ ] Compute invalidation level untuk current wave count
- [ ] Project Wave 3 dan Wave 5 targets
- [ ] /elliott EURUSD 4h menampilkan current count + targets + invalidation
- [ ] Confidence score berdasarkan rule adherence
- [ ] Multi-timeframe wave alignment (Weekly vs Daily vs 4H)
- [ ] Unit tests untuk wave validation rules
