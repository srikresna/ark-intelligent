# Research Audit Report — ff-calendar-bot

**Date**: 2026-04-04  
**Agent**: Research Agent (ARK Intelligent)  
**Run**: Scheduled cron audit

---

## Summary

Codebase audit complete. Found **3 critical**, **5 medium**, and **4 low-priority** issues requiring attention. Overall code quality is good but test coverage and security practices need improvement.

---

## Critical Issues (Immediate Action Required)

### 1. CRITICAL-001: Unbounded Goroutine Creation Risk
- **Location**: `internal/adapter/telegram/bot.go:208`
- **Issue**: `go b.handleUpdate(ctx, update)` spawns goroutines without limit
- **Risk**: Resource exhaustion under high load, potential DoS vulnerability
- **Fix**: Implement worker pool with max concurrency

### 2. CRITICAL-002: Panic in Production Code
- **Location**: `internal/service/marketdata/keyring/keyring.go:40`
- **Issue**: `MustNext()` calls `panic(err)` instead of graceful error handling
- **Risk**: Single misconfiguration crashes entire process
- **Fix**: Return error instead of panic; add retry logic at caller

### 3. CRITICAL-003: Missing Context Propagation
- **Locations**: 
  - `internal/adapter/telegram/handler_cta.go:581`
  - `internal/adapter/telegram/handler_quant.go:448, 484`
  - `internal/adapter/telegram/handler_vp.go:422`
- **Issue**: Using `context.Background()` instead of request context
- **Risk**: Operations can't be cancelled on client disconnect; resource leaks
- **Fix**: Propagate request context through all call chains

---

## Medium Priority Issues

### 4. MED-001: Test Coverage Gap (Critical Path)
- **Stats**: 
  - Overall: 262 Go files, 39 test files (~15% coverage)
  - `internal/adapter/telegram/`: 29 files, 1 test (3% coverage)
  - `internal/service/*`: 135 files, 30 tests (~22% coverage)
- **Risk**: Critical business logic untested, regression risk
- **Priority Files Missing Tests**:
  - `handler.go` (2,909 lines) - no test
  - `formatter.go` (4,539 lines) - no test
  - `api.go` (762 lines) - no test
  - `bot.go` (433 lines) - no test
  - `keyboard.go` - no test

### 5. MED-002: Error Handling Gaps
- **Pattern**: `os.Remove()` called without error handling
- **Locations**: 
  - `handler_quant.go:442-443, 454, 461, 467`
  - `handler_cta.go:707-708, 745-746`
  - `handler_vp.go:377, 410-411, 430, 436, 442, 447`
  - `handler_ctabt.go:467-468`
- **Risk**: Silent failures during temp file cleanup
- **Fix**: Log errors or use structured cleanup with `errors.Join()`

### 6. MED-003: Scheduler Context Detachment
- **Location**: `internal/service/news/scheduler.go:684`, `impact_recorder.go:108`
- **Issue**: Background goroutines use `context.Background()` from scheduler
- **Risk**: Impact recording can't be cancelled during shutdown
- **Fix**: Pass parent context through job execution chain

### 7. MED-004: HTTP Client Configuration
- **Observation**: Multiple HTTP clients without shared configuration
- **Risk**: Inconsistent timeout/retry behavior
- **Suggestion**: Use shared HTTP client factory with standard config

### 8. MED-005: Missing Input Validation
- **Observation**: Several handlers lack input sanitization
- **Risk**: Potential injection or malformed data processing
- **Fix**: Add validation layer at handler entry points

---

## Low Priority Issues

### 9. LOW-001: Documentation Gaps
- **Missing**:
  - Architecture decision records (ADRs)
  - API documentation for internal services
  - Handler/formatter usage documentation
- **README** doesn't document newer commands (/alpha, /quant, /cta, etc.)

### 10. LOW-002: Logging Inconsistencies
- **Stats**: 315 logger references across 41 files
- **Issue**: Mix of structured (zerolog) and unstructured patterns
- **Note**: Most code uses zerolog correctly; minor cleanup needed

### 11. LOW-003: math/rand vs crypto/rand
- **Location**: `internal/adapter/telegram/api.go:9`
- **Issue**: Using `math/rand` for jitter/backoff
- **Note**: Acceptable for non-security use; `crypto/rand` recommended for defense-in-depth

### 12. LOW-004: Code Duplication
- **Pattern**: Contract code constants repeated across multiple files
- **Suggestion**: Centralize in `internal/domain/contracts.go`

---

## Recommendations

### Immediate (This Sprint)
1. Fix CRITICAL-002 (keyring panic) - 1 story point
2. Add goroutine limiter for CRITICAL-001 - 3 story points
3. Fix context.Background() in handler_quant.go - 2 story points

### Next Sprint
4. Add tests for telegram handlers (highest business impact)
5. Implement shared HTTP client factory
6. Add error handling to all os.Remove() calls

### Backlog
7. Create ADR documentation
8. Standardize logging patterns
9. Add request validation layer

---

## Task Specs Created

- `PHI-SEC-001`: Fix keyring panic (CRITICAL-002)
- `PHI-SEC-002`: Add goroutine limiter (CRITICAL-001)
- `PHI-CTX-001`: Fix context propagation (CRITICAL-003)
- `PHI-TEST-001`: Add handler tests (MED-001)

---

## Metrics

| Metric | Count |
|--------|-------|
| Total Go Files | 262 |
| Test Files | 39 (15%) |
| Handler Files | 29 (1 with test) |
| Service Files | 135 (30 with tests) |
| Goroutine Spawn Points | 28 |
| HTTP Client Files | 70 |
| Critical Issues | 3 |
| Medium Issues | 5 |
| Low Issues | 4 |

---

*Report generated by Research Agent — ff-calendar-bot*
