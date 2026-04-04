# Research Agent Audit Report — 2026-04-04

**Agent**: Research Agent (ARK Intelligent)  
**Run Type**: Scheduled Cron Audit  
**Timestamp**: 2026-04-04 07:25 UTC

---

## Summary

This is a verification audit following up on the comprehensive audit conducted earlier today. All critical issues from the previous audit have been confirmed as **still present** in the codebase. No new critical issues were discovered during this run.

### Previous Audit Reference
- Previous audit report: `.agents/research/2026-04-04-audit-findings.md`
- 4 task specs were created from previous audit
- All agents currently **idle** — no tasks in progress

---

## Critical Issues Verification

| Issue ID | Description | Status | Location |
|----------|-------------|--------|----------|
| **PHI-SEC-001** | Keyring panic on empty keys | 🔴 **STILL PRESENT** | `internal/service/marketdata/keyring/keyring.go:40` |
| **PHI-SEC-002** | Unbounded goroutines (BIS) | 🔴 **STILL PRESENT** | `internal/service/bis/reer.go:138` |
| **PHI-SEC-002** | Unbounded goroutines (WorldBank) | 🔴 **STILL PRESENT** | `internal/service/worldbank/client.go:117` |
| **PHI-CTX-001** | context.Background() misuse | 🔴 **STILL PRESENT** | `handler_cta.go`, `handler_quant.go`, `handler_vp.go` |
| **PHI-TEST-001** | Missing handler unit tests | 🔴 **STILL PRESENT** | `internal/adapter/telegram/` |

---

## Detailed Findings

### PHI-SEC-001: Keyring Panic (CRITICAL)

**Verification**: Confirmed still present at line 40

```go
func (k *Keyring) MustNext() string {
    key, err := k.Next()
    if err != nil {
        panic(err)  // ← STILL PRESENT: crashes process
    }
    return key
}
```

**Risk**: Single misconfiguration crashes entire bot process

---

### PHI-SEC-002: Unbounded Goroutines (CRITICAL)

**Verification**: Confirmed present in two locations

1. **BIS REER Fetcher** (`internal/service/bis/reer.go:136-142`)
   - Spawns goroutine per currency without semaphore
   - No concurrency limiting

2. **WorldBank Client** (`internal/service/worldbank/client.go:115-121`)
   - Spawns goroutine per country without semaphore  
   - No concurrency limiting

**Comparison**: FRED fetcher (`internal/service/fred/fetcher.go:339`) correctly uses semaphore pattern:
```go
sem := make(chan struct{}, 10) // max 10 concurrent
```

**Risk**: Resource exhaustion under high load → OOM kills

---

### PHI-CTX-001: Context Propagation Issues (HIGH)

**Verification**: Confirmed in 3 files:

| File | Line | Issue |
|------|------|-------|
| `handler_cta.go` | 581 | `ctx := context.Background()` |
| `handler_quant.go` | 448, 484 | `context.WithTimeout(context.Background(), ...)` |
| `handler_vp.go` | 422 | `cmdCtx, cancel := context.WithTimeout(context.Background(), ...)` |

**Risk**: Operations can't be cancelled on client disconnect; resource waste

---

### PHI-TEST-001: Test Coverage Gap (HIGH)

**Verification**: Still no test files for telegram handlers

- `handler.go` (2,909 lines): No tests
- `handler_cta.go` (1,624 lines): No tests  
- `handler_quant.go`: No tests
- `formatter.go` (4,539 lines): No tests

**Stats**: 223 source files, 39 test files (~17.5% coverage ratio)

---

## Minor Findings

### Ignored Error Handling
- `internal/adapter/telegram/handler.go`: ~39 occurrences of `_, _ =` pattern
- Suggests error handling could be improved throughout handler layer

### Hardcoded Timeouts
Several handlers have hardcoded timeout values instead of configurable ones:
- `handler_cta.go`: 90 second timeout
- `handler_vp.go`: 90 second timeout  
- `handler_quant.go`: 60 second timeout

**Recommendation**: Consider making timeouts configurable via config or env vars

---

## Task Specs Status

All critical issues have comprehensive task specifications in `.agents/tasks/pending/`:

| Task File | Priority | Est | Status |
|-----------|----------|-----|--------|
| `PHI-SEC-001-fix-keyring-panic.md` | CRITICAL | S | Ready for dev |
| `PHI-SEC-002-goroutine-limiter.md` | CRITICAL | M | Ready for dev |
| `PHI-CTX-001-fix-context-propagation.md` | HIGH | M | Ready for dev |
| `PHI-TEST-001-handler-unit-tests.md` | HIGH | L | Ready for dev (recommend splitting) |

---

## Recommendations

### Immediate (This Sprint)
1. **Assign PHI-SEC-001** to Dev-A — smallest fix, highest impact
2. **Assign PHI-SEC-002** to Dev-B — requires semaphore pattern implementation
3. **Assign PHI-CTX-001** to Dev-C — context propagation fixes

### Next Sprint
4. **Split PHI-TEST-001** into smaller chunks (suggested in task spec)
5. **Address ignored errors** in handler layer
6. **Make timeouts configurable** across handlers

---

## Conclusion

✅ **Audit Complete**: All 3 critical issues from previous audit confirmed present  
✅ **Task Specs Ready**: All issues have detailed acceptance criteria  
✅ **No New Blockers**: No additional critical issues discovered  
⚠️ **Action Needed**: Critical security issues need immediate assignment

**Next Steps**: Coordinator Agent should assign PHI-SEC-001, PHI-SEC-002, and PHI-CTX-001 to available dev agents.

---

*Report generated by Research Agent — ARK Intelligent ff-calendar-bot*
