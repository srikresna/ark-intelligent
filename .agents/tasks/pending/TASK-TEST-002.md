# TASK-TEST-002: Unit Tests for handler_alpha.go Signal Generation

**Task ID:** TASK-TEST-002  
**Type:** Test Coverage  
**Priority:** High  
**Effort Estimate:** 4-6 hours  
**Created:** 2026-04-07  
**Author:** Dev-A (ARK Intelligent)

---

## Objective

Create comprehensive unit tests for `internal/adapter/telegram/handler_alpha.go` signal generation functions — testing the alpha ranking, factor computation, and signal classification logic.

---

## Background

The handler_alpha.go file (1,276 lines) handles new factor/strategy/microstructure commands:
- `/alpha` — unified dashboard with inline keyboard navigation
- `/xfactors` — cross-sectional factor ranking
- `/playbook` — strategy playbook (top long/short + macro context)
- `/heat` — portfolio exposure heat
- `/rankx` — compact rank leaderboard
- `/transition` — regime transition warning
- `/cryptoalpha` — Bybit microstructure confirmation for top crypto signals

**Current State:** 1,276 lines, **0 unit tests** for the signal generation logic.

---

## Acceptance Criteria

### Coverage Targets
- [ ] Minimum 60% code coverage for pure functions (aim for 70%+)
- [ ] All signal generation functions have test cases
- [ ] Alpha ranking computation logic fully tested
- [ ] Error handling paths tested

### Specific Test Cases Required

#### 1. Alpha State Cache
- [ ] Test `alphaStateCache.get()` returns nil for missing chatID
- [ ] Test `alphaStateCache.get()` returns nil for expired TTL
- [ ] Test `alphaStateCache.get()` returns valid state within TTL
- [ ] Test `alphaStateCache.set()` stores state correctly
- [ ] Test `alphaStateCache.set()` opportunistic cleanup after 50 entries

#### 2. Signal Classification
- [ ] Test `classifySignal()` for bullish spec (cotIndex=80, isCommercial=false)
- [ ] Test `classifySignal()` for bearish spec (cotIndex=20, isCommercial=false)
- [ ] Test `classifySignal()` for neutral spec (cotIndex=50)
- [ ] Test `classifySignal()` for commercial inverse logic

#### 3. Factor Ranking Integration
- [ ] Test alpha state computation with valid ranking result
- [ ] Test alpha state computation with nil ranking (graceful handling)
- [ ] Test factor score aggregation
- [ ] Test currency filtering in ranking

#### 4. Crypto Alpha Signal
- [ ] Test microstructure signal parsing
- [ ] Test crypto signal confidence scoring
- [ ] Test signal validation (sufficient data points)

#### 5. Playbook Generation
- [ ] Test playbook result generation with valid data
- [ ] Test playbook with empty/unavailable data
- [ ] Test top long/short selection logic

#### 6. Edge Cases
- [ ] Test with empty asset profile list
- [ ] Test with missing services (graceful degradation)
- [ ] Test concurrent access to alpha state cache
- [ ] Test division by zero guards in calculations

---

## Technical Notes

### Dependencies to Mock

```go
type AlphaServices struct {
    FactorEngine   *factors.Engine
    StrategyEngine *strategy.Engine
    MicroEngine    *microstructure.Engine
    ProfileBuilder AssetProfileBuilder
}
```

### Key Challenges
1. Testing time-based TTL expiration
2. Mocking external service dependencies (factors.Engine, strategy.Engine)
3. Testing concurrent access to shared state (alphaStateCache)
4. Testing private/unexported functions

### Critical Code Patterns to Test

```go
// From handler_alpha.go - these patterns need coverage:

// Alpha state TTL and cache access (lines 81-89)
func (c *alphaStateCache) get(chatID string) *alphaState {
    c.mu.Lock()
    defer c.mu.Unlock()
    s, ok := c.store[chatID]
    if !ok || time.Since(s.computedAt) > alphaStateTTL {
        return nil
    }
    return s
}

// Opportunistic cleanup (lines 95-101)
if len(c.store) > 50 {
    // cleanup logic
}

// Signal classification logic
func classifySignal(cotIndex, momentum float64, isCommercial bool) SignalType
```

### Suggested Test Structure

```go
func TestAlphaStateCache_GetMissing(t *testing.T) { }
func TestAlphaStateCache_GetExpired(t *testing.T) { }
func TestAlphaStateCache_SetAndGet(t *testing.T) { }
func TestAlphaStateCache_Cleanup(t *testing.T) { }
func TestClassifySignal_BullishSpec(t *testing.T) { }
func TestClassifySignal_BearishSpec(t *testing.T) { }
func TestClassifySignal_CommercialInverse(t *testing.T) { }
func TestAlphaState_ComputeRanking(t *testing.T) { }
func TestCryptoAlpha_SignalValidation(t *testing.T) { }
```

---

## Implementation Guidelines

1. **Create** `internal/adapter/telegram/handler_alpha_test.go`
2. **Use** `testify/mock` or manual mocks for dependency interfaces
3. **Use** `testify/assert` for assertions
4. **Follow** existing test patterns from `pkg/retry/retry_test.go`
5. **Ensure** tests run quickly (< 5s total)
6. **Avoid** actual network calls or external dependencies
7. **Use** `t.Parallel()` where safe
8. **Test** both success and error paths

---

## Definition of Done

- [ ] Test file created with comprehensive coverage
- [ ] All tests passing (`go test ./internal/adapter/telegram/... -run TestAlpha`)
- [ ] Coverage report shows 60%+ for tested functions
- [ ] No race conditions detected (`go test -race`)
- [ ] Code review approved
- [ ] Merged to main branch

---

## Related Files

- `internal/adapter/telegram/handler_alpha.go` (1,276 lines) — primary target
- `internal/service/factors/` — for factor engine mocks
- `internal/service/strategy/` — for strategy engine mocks
- `internal/service/microstructure/` — for microstructure mocks

---

## Context

This file is critical for the bot's alpha ranking and signal generation features. It:
- Computes cross-sectional factor rankings
- Generates strategy playbooks
- Provides microstructure confirmation for crypto signals
- Caches computation results with TTL for performance

**Related Issues:**
- Shares cache concurrency patterns with other handlers
- Uses AlphaServices dependency injection pattern

---

*Task created by Dev-A — ARK Intelligent*
