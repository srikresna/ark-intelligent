# Research Agent Scheduled Audit Report

**Date:** 2026-04-04 (Current Run)  
**Run Type:** Scheduled cron audit  
**Agent:** Research Agent (ARK Intelligent / ff-calendar-bot)

---

## Summary

**Status:** ✅ Verification complete. All 10 previously identified issues confirmed present. **No new issues discovered.**

**Current Queue:** 15 tasks pending (unchanged)
- 4 Critical security/reliability issues (PHI-SEC-001, PHI-SEC-002, PHI-CTX-001, PHI-TEST-001)
- 5 Reliability issues (PHI-REL-001 through PHI-REL-005)
- 6 Feature/Tech Debt tasks (PHI-SETUP-001, PHI-DATA-001, PHI-DATA-002, PHI-UX-001, PHI-UX-002, PHI-TEST-002)

**Agent Status:** All 6 agents idle and available for assignment.

---

## Verification of Known Issues

### Critical Issues (Verified Present)

| Issue | Status | File | Details |
|-------|--------|------|---------|
| PHI-SEC-001 | **Present** | `internal/service/marketdata/keyring/keyring.go:40` | `panic(err)` in MustNext() - process crash risk |
| PHI-SEC-002 | **Present** | `internal/adapter/telegram/bot.go:208` | Unbounded goroutine spawning per update - DoS risk |
| PHI-CTX-001 | **Present** | `internal/adapter/telegram/handler_*.go` | 4 context.Background() calls (down from 36 - partial improvement detected) |
| PHI-TEST-001 | **Pending** | `internal/adapter/telegram/` | 28+ files, 1 test file (~3.5% coverage) |

### Reliability Issues (Verified Present)

| Issue | Status | File | Details |
|-------|--------|------|---------|
| PHI-REL-001 | **Present** | `internal/adapter/telegram/handler.go:2590` | notifyOwnerDebug goroutine without panic recovery |
| PHI-REL-002 | **Present** | `internal/scheduler/scheduler.go:186` | Impact bootstrap goroutine without recovery |
| PHI-REL-003 | **Present** | `internal/service/ai/chat_service.go:301` | notifyOwner goroutine without recovery |
| PHI-REL-004 | **Present** | `internal/service/fred/bis/worldbank/` | 3 worker pool goroutines without recovery |
| PHI-REL-005 | **Present** | `internal/config/config.go:237-254` | 4 log.Fatal() calls in validation |

### Test Coverage Issues (Verified Present)

| Issue | Status | File | Details |
|-------|--------|------|---------|
| PHI-TEST-002 | **Pending** | `internal/config/` | 2 files, 0 tests (0% coverage) |

---

## Test Coverage Audit

| Metric | Value | Status |
|--------|-------|--------|
| Total Go files | 262 | — |
| Source Go files | 223 | — |
| Test files | 39 | — |
| Overall coverage | **17.5%** | ⚠️ Low |

**Critical Packages with 0% Test Coverage:**
- `internal/config` (0/2) — PHI-TEST-002
- `internal/service/ai` (0/13)
- `internal/service/marketdata` (0/6)
- `internal/adapter/storage` (0/16)

**Low Coverage Packages:**
- `internal/adapter/telegram` (1/28 = 3.5%) — PHI-TEST-001

---

## Security Scan Results

| Check | Result | Notes |
|-------|--------|-------|
| panic() calls | 2 found | 1 in production (keyring.go:40), 1 in test file |
| log.Fatal() | 4 found | All in config validation (PHI-REL-005) |
| Hardcoded credentials | None found | Clean scan |
| SQL injection risk | None found | Uses BadgerDB (key-value), not SQL |
| http.Client timeout | ✅ 39 clients | All have timeouts configured |
| goroutine spawning | 8 locations | 5 without explicit panic recovery |

---

## Code Quality Observations

### Positive Patterns
- ✅ Proper use of `defer` for resource cleanup (67 occurrences)
- ✅ Mutex usage for shared state (33 occurrences)
- ✅ Context cancellation handling (26 select statements checking Done())
- ✅ Channel safety patterns (all channels bounded)
- ✅ Circuit breaker pattern usage (3 implementations)

### Areas for Improvement
- ⚠️ 4 context.Background() calls in production code (improved from 36)
- ⚠️ 5 goroutine spawns without explicit panic recovery (documented in PHI-REL tasks)
- ⚠️ No new TODO/FIXME technical debt (existing 9 comments are currency code references)

---

## Detailed Verification Notes

### Context.Background() Improvements
Previous audits reported 36 context.Background() calls. Current count shows significant improvement:
- Only 4 calls in production handler code (handler_cta.go, handler_quant.go, handler_vp.go)
- These are the specific locations documented in PHI-CTX-001

### Panic Recovery Status
The scheduler's `runJob()` function **does** have panic recovery (lines 286-290), which protects:
- Worker pool goroutines started by `startJobWithDelay()`
- All scheduled job executions

However, these goroutines still lack protection:
1. Impact bootstrap (PHI-REL-002) — scheduler.go:186
2. notifyOwnerDebug (PHI-REL-001) — handler.go:2590
3. notifyOwner (PHI-REL-003) — chat_service.go:301
4. FRED/BIS/World Bank workers (PHI-REL-004)

---

## Recommendations

### Immediate (Next Sprint)
1. **Claim PHI-REL-001, PHI-REL-002, PHI-REL-003** — Panic recovery (XS-Small each, 5-10 min fixes)
2. **Claim PHI-SEC-001** — Keyring panic handling (Small, 2-4 hrs)

### Short-term
1. **Claim PHI-SEC-002** — Goroutine limiter (Medium, 4-8 hrs)
2. **Claim PHI-CTX-001** — Context propagation (Medium, 4-6 hrs)
3. **Claim PHI-REL-004** — Worker pool recovery (Small, 30 min)
4. **Claim PHI-REL-005** — Config validation error handling (Small, 1-2 hrs)

### Medium-term
1. **Claim PHI-TEST-002** — Config package tests (Small, 2-4 hrs)
2. **Split PHI-TEST-001** — Handler tests (Large, split into 3 subtasks as documented)

---

## Conclusion

The codebase remains stable with **no new critical issues** discovered during this verification audit. All 10 previously identified security and reliability issues are still present and documented with task specs in `.agents/tasks/pending/`.

**Ready for Development:**
- 4 Critical issues ready for Dev-A, Dev-B, or Dev-C to claim
- 5 Reliability issues (quick wins, XS-Small size)
- 6 Feature/Tech Debt tasks (some already assigned)

**All agents are idle and ready for task assignment.**

---

*Report auto-generated by Research Agent — ARK Intelligent*
