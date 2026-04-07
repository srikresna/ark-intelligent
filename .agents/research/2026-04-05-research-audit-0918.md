# Research Audit Report — ff-calendar-bot

**Audit Date:** 2026-04-05 09:18 UTC  
**Auditor:** Research Agent (Agent-2)  
**Scope:** Full codebase audit — verification of pending tasks and blocker check  
**Previous Audit:** 2026-04-05 09:06 UTC

---

## Executive Summary

✅ **All agents idle, 0 blockers**  
✅ **22 pending tasks verified valid**  
✅ **No new critical issues identified**  
✅ **TASK-TEST-001 still awaiting QA review (1,139 lines, 44 tests)**  
✅ **Codebase health: stable**

This scheduled audit confirms that all previously identified issues remain valid and have appropriate task coverage. **No new task specifications were created** — all actionable issues are already tracked in the 22 pending tasks.

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
| TASK-CODEQUALITY-002 | Medium | ⏳ Unfixed | context.Background() in production (10 occurrences) |
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

## Critical Issues Status (Still Unfixed — Verified)

### 1. TASK-BUG-001: Data Race ⚠️

**File:** `internal/adapter/telegram/handler_session.go:23`

```go
var sessionAnalysisCache = map[string]*sessionCache{}  // No synchronization!
```

**Risk:** Panic on concurrent map writes in production with concurrent Telegram requests.

**Verification:** Other handlers in the same package (handler_cta.go, handler_wyckoff.go, handler_vp.go, etc.) properly use `sync.Mutex` — only handler_session.go lacks protection.

**Fix:** Add `sync.RWMutex` wrapper (see task spec for implementation).

---

### 2. TASK-SECURITY-001: HTTP DefaultClient Timeout ⚠️

**File:** `internal/service/macro/tradingeconomics_client.go:246`

```go
resp, err := http.DefaultClient.Do(req)  // No timeout!
```

**Risk:** Requests can hang indefinitely, causing goroutine leaks and resource exhaustion.

**Fix:** Replace with `http.Client{Timeout: 30s, Transport: SharedTransport}`.

---

### 3. TASK-CODEQUALITY-002: context.Background() in Production ⚠️

**Count:** 10 occurrences across 6 production files (unchanged from last audit)

| File | Line | Context |
|------|------|---------|
| internal/service/news/impact_recorder.go | 108 | delayedRecord goroutine |
| internal/service/news/scheduler.go | 721 | record context with timeout (commented as intentional) |
| internal/service/ai/chat_service.go | 312 | ownerNotify goroutine |
| internal/scheduler/scheduler_skew_vix.go | 20,56,74 | VIX timeout, broadcast |
| internal/health/health.go | 66,134 | shutdown, Python check |
| cmd/bot/main.go | 76 | root context (acceptable) |

**Risk:** Poor cancellation propagation, makes testing harder.

**Note:** Some uses are intentional (fire-and-forget goroutines) and documented with comments.

---

## Test Coverage Analysis

| Metric | Value |
|--------|-------|
| Total Go Files | 509 |
| Test Files | 108 |
| Coverage | 21.2% (108/509) |
| Untested Files | 401 |
| Untested Percentage | 78.8% |

### Large Untested Files (with task coverage)

All large untested files (500+ lines) have existing task coverage:

| File | Lines | Covered By |
|------|-------|------------|
| format_cot.go | 1,394 | TASK-TEST-003 |
| scheduler.go | 1,335 | TASK-TEST-013 |
| handler_alpha.go | 1,276 | TASK-TEST-002 |
| news/scheduler.go | 1,134 | TASK-TEST-015 |
| ta/indicators.go | 1,025 | TASK-TEST-014 |
| format_cta.go | 963 | TASK-TEST-005 |
| api.go | 872 | TASK-TEST-004 |
| formatter_quant.go | 847 | TASK-TEST-006 |
| handler_backtest.go | 826 | TASK-TEST-007 |

---

## New Findings

**No new critical issues identified.**

All previously identified issues remain valid and tracked:
- Data race: covered by TASK-BUG-001
- HTTP timeout: covered by TASK-SECURITY-001  
- context.Background(): covered by TASK-CODEQUALITY-002
- Test gaps: covered by TASK-TEST-001 through TASK-TEST-015
- Code quality: covered by TASK-REFACTOR-001, TASK-REFACTOR-002
- Documentation: covered by TASK-DOCS-001

---

## Codebase Health Check

| Check | Status |
|-------|--------|
| Race conditions | 1 known (tracked) |
| Security issues | 1 known (tracked) |
| SQL injection | ✓ None found |
| Hardcoded credentials | ✓ None found |
| panic() usage | ✓ Test files only |
| os.Exit() | ✓ cmd/ package only |
| HTTP body.Close() | ✓ All proper |
| Test coverage | 21.2% (needs improvement) |

---

## Recommendations

1. **High Priority:** Assign TASK-BUG-001 (race condition) to Dev-A or Dev-B — this is a production stability risk that could cause panics under concurrent load
2. **High Priority:** Assign TASK-SECURITY-001 (HTTP timeout) — easy 1-hour fix with high security impact
3. **QA Action:** Review TASK-TEST-001 (keyboard tests) for merge approval
4. **Medium Priority:** Begin TASK-TEST-013 (scheduler.go tests) — core infrastructure with 1,335 lines

---

## Conclusion

**No new task specs created.** All 22 pending tasks remain valid and actionable. The codebase is stable with no new blockers. Three critical issues (race, timeout, context.Background()) are confirmed unfixed and awaiting assignment.

Current branch: `feat/TASK-TEST-001-keyboard-tests` (TASK-TEST-001 in review)

---

*Report generated by Research Agent (Agent-2) as scheduled cron job*
