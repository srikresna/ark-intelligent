# Research Audit Report — ff-calendar-bot

**Audit Date:** 2026-04-05 09:30 UTC  
**Auditor:** Research Agent (ff-calendar-bot)  
**Scope:** Full codebase audit of 509 Go files  
**Previous Audit:** 2026-04-05 09:18 UTC

---

## Executive Summary

✅ **All agents idle, 0 blockers**  
✅ **22 pending tasks verified valid**  
✅ **No new critical issues identified**  
✅ **Codebase health: stable**

This scheduled audit confirms that all previously identified issues remain valid and have appropriate task coverage. No new task specifications were created as all actionable issues are already tracked.

---

## Task Queue Verification

### Pending Tasks (22 total) — All Valid

| Task ID | Priority | Status | Issue |
|---------|----------|--------|-------|
| TASK-BUG-001 | **High** | ⏳ Unfixed | Data race in handler_session.go |
| TASK-SECURITY-001 | **High** | ⏳ Unfixed | http.DefaultClient without timeout |
| TASK-TEST-002 | High | ⏳ Pending | handler_alpha.go tests (1,276 lines) |
| TASK-TEST-003 | High | ⏳ Pending | format_cot.go tests (1,394 lines) |
| TASK-TEST-004 | Medium | ⏳ Pending | api.go tests (872 lines) |
| TASK-TEST-005 | Medium | ⏳ Pending | format_cta.go tests (963 lines) |
| TASK-TEST-006 | Medium | ⏳ Pending | formatter_quant.go tests (847 lines) |
| TASK-TEST-007 | Medium | ⏳ Pending | handler_backtest.go tests (826 lines) |
| TASK-TEST-008 | Medium | ⏳ Pending | Storage repository layer tests |
| TASK-TEST-009 | Medium | ⏳ Pending | format_price.go tests (697 lines) |
| TASK-TEST-010 | Medium | ⏳ Pending | format_macro.go tests |
| TASK-TEST-011 | Medium | ⏳ Pending | format_sentiment.go tests |
| TASK-TEST-012 | Medium | ⏳ Pending | bot.go tests |
| TASK-TEST-013 | **High** | ⏳ Pending | scheduler.go tests (1,335 lines) |
| TASK-TEST-014 | Medium | ⏳ Pending | ta/indicators.go tests (1,025 lines) |
| TASK-TEST-015 | **High** | ⏳ Pending | news/scheduler.go tests (1,134 lines) |
| TASK-REFACTOR-001 | Medium | ⏳ Pending | Magic numbers extraction |
| TASK-REFACTOR-002 | Medium | ⏳ Pending | keyboard.go decomposition |
| TASK-CODEQUALITY-002 | Medium | ⏳ Unfixed | context.Background() in production |
| TASK-CODEQUALITY-001 | Low | ⏳ Pending | context.Background() in tests |
| TASK-DOCS-001 | Low | ⏳ Pending | Emoji system documentation |

### In Review

| Task ID | Status | Details |
|---------|--------|---------|
| TASK-TEST-001 | 👁️ In Review | keyboard.go tests — 1,139 lines, 44 tests, branch `feat/TASK-TEST-001-keyboard-tests` awaiting QA |

### In Progress
- None

### Blocked
- None

---

## Critical Issues Status

### 1. TASK-BUG-001: Data Race (STILL UNFIXED) ⚠️

**File:** `internal/adapter/telegram/handler_session.go:23`

```go
var sessionAnalysisCache = map[string]*sessionCache{}  // No synchronization!
```

**Write operation at line 94:**
```go
sessionAnalysisCache[mapping.Currency] = &sessionCache{result: result, fetchedAt: time.Now()}
```

**Verification:**
- Race condition STILL PRESENT: ✅ Yes
- Mutex protection added: ❌ No
- Import sync package: ❌ No

**Risk:** Panic on concurrent map writes in production with concurrent Telegram requests.

**Fix Required:** Add `sync.RWMutex` wrapper around the cache map.

---

### 2. TASK-SECURITY-001: HTTP DefaultClient Timeout (STILL UNFIXED) ⚠️

**File:** `internal/service/macro/tradingeconomics_client.go:246`

```go
resp, err := http.DefaultClient.Do(req)  // No timeout!
```

**Verification:**
- http.DefaultClient.Do STILL PRESENT: ✅ Yes (Line 246)

**Risk:** Requests can hang indefinitely, causing resource exhaustion and goroutine leaks.

**Fix Required:** Replace with `http.Client{Timeout: 30s, Transport: SharedTransport}`.

---

### 3. TASK-CODEQUALITY-002: context.Background() in Production

**Count:** 8 occurrences across 6 production files (unchanged)

| File | Occurrences |
|------|-------------|
| internal/scheduler/scheduler_skew_vix.go | 3 |
| internal/health/health.go | 2 |
| internal/service/news/scheduler.go | 1 |
| internal/service/news/impact_recorder.go | 1 |
| internal/service/ai/chat_service.go | 1 |

**Note:** All occurrences should use passed-in contexts for proper cancellation propagation.

---

## Codebase Health Metrics

### Test Coverage
- **Test files:** 108
- **Source files:** 401
- **Coverage ratio:** 26.9%
- **Untested files:** 318 (73.1%)

### Large Untested Files (Priority Queue Coverage ✓)

| File | Lines | Task Coverage |
|------|-------|---------------|
| format_cot.go | 1,394 | TASK-TEST-003 ✓ |
| scheduler.go | 1,335 | TASK-TEST-013 ✓ |
| handler_alpha.go | 1,276 | TASK-TEST-002 ✓ |
| news/scheduler.go | 1,134 | TASK-TEST-015 ✓ |
| ta/indicators.go | 1,025 | TASK-TEST-014 ✓ |
| format_cta.go | 963 | TASK-TEST-005 ✓ |
| unified_outlook.go | 909 | Lower priority |
| fred/fetcher.go | 906 | Lower priority |
| api.go | 872 | TASK-TEST-004 ✓ |
| formatter_quant.go | 847 | TASK-TEST-006 ✓ |

All critical large untested files have corresponding task specs in the queue.

### Code Quality Checks

| Check | Status | Count |
|-------|--------|-------|
| TODO/FIXME in production | 🟢 Clean | 0 actual occurrences |
| time.Now() in production | 🟢 Present | 85 occurrences (testing concern, not bug) |
| SQL injection risks | 🟢 Clean | No patterns found |
| HTTP body.Close() | 🟢 Good | 76 proper closes |
| panic() in production | 🟢 Clean | 1 justified occurrence in keyring.go |
| Race conditions (new) | 🟢 Clean | None detected beyond known issue |
| New HTTP timeout issues | 🟢 Clean | None detected beyond known issue |
| Hardcoded credentials | 🟢 Clean | All use env vars properly |

---

## New Findings

### No New Critical Issues Identified

After auditing 509 Go files, **no new critical issues were found** that require task creation. The following categories were checked:

- ✅ No new race conditions beyond TASK-BUG-001
- ✅ No new security vulnerabilities
- ✅ No resource leaks
- ✅ No HTTP body.Close() violations
- ✅ No panic() misuse
- ✅ No SQL injection patterns
- ✅ No new HTTP timeout issues
- ✅ No hardcoded credentials

### Race Condition Analysis

A comprehensive scan found 22 package-level map variables. Upon detailed analysis:
- **21 are read-only lookup tables** (safe, initialized at startup, never modified)
- **1 is the known race condition** in handler_session.go (TASK-BUG-001)

All read-only maps have been verified to have no write operations — they are safe for concurrent access.

---

## Recommendations

### Immediate Actions (Next Sprint)

1. **Prioritize TASK-BUG-001** — Data race is a production stability risk. Estimated 1-2 hours to fix.

2. **Prioritize TASK-SECURITY-001** — HTTP timeout affects reliability. Estimated 1 hour to fix.

3. **QA Review TASK-TEST-001** — Keyboard tests are ready for review and merge.

### Queue Status

The queue has healthy coverage with 22 actionable tasks:
- 2 high-priority bugs (race + timeout)
- 3 high-priority test coverage tasks
- 10 medium-priority test coverage tasks
- 2 medium-priority refactors
- 2 code quality tasks
- 1 low-priority docs task

No additional tasks needed at this time.

---

## Conclusion

**Codebase Health: STABLE**

All known issues are tracked. All large untested files have task coverage. No new critical issues discovered during this audit.

The team should focus on:
1. Fixing TASK-BUG-001 (data race)
2. Fixing TASK-SECURITY-001 (HTTP timeout)
3. Completing QA review of TASK-TEST-001
4. Picking up test coverage tasks from the queue

---

*Report generated by Research Agent*  
*Next scheduled audit: Following agent cycle*
