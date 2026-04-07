# Research Agent Audit Report

**Date:** 2026-04-05 14:16 UTC  
**Agent:** Research Agent (Agent-2)  
**Scope:** Comprehensive codebase audit for ARK Intelligent (ff-calendar-bot)

---

## Executive Summary

**Status:** All 22 pending tasks verified valid. **No new task specs created** — all actionable issues already have task coverage.

**Last Go Code Change:** 2026-04-03 14:49 UTC (no source code changes since 13:29 UTC audit)  
**Last Commit:** 2026-04-04 21:48 UTC (documentation only — .agents/ research files)

---

## Audit Scope

- **Production Go Files:** 401 files
- **Test Files:** 108 files
- **Test Coverage:** ~21% (318 files without tests, 79% untested)
- **All Agents:** Idle
- **Blockers:** None

---

## Confirmed Issues (Already Tracked)

### 1. TASK-BUG-001: Data Race in handler_session.go ⚠️ HIGH PRIORITY
**Location:** `internal/adapter/telegram/handler_session.go:23`

```go
var sessionAnalysisCache = map[string]*sessionCache{}  // Global map, no synchronization
```

**Issue:** Global map accessed concurrently without mutex protection. Cache reads at line 57 and writes elsewhere lack synchronization.

**Status:** Confirmed present — awaiting Dev assignment

---

### 2. TASK-SECURITY-001: HTTP Client Without Timeout ⚠️ HIGH PRIORITY
**Location:** `internal/service/macro/tradingeconomics_client.go:246`

```go
resp, err := http.DefaultClient.Do(req)  // No timeout configured
```

**Issue:** Using http.DefaultClient without custom timeout can lead to hanging requests.

**Status:** Confirmed present — awaiting Dev assignment

---

### 3. TASK-CODEQUALITY-002: context.Background() in Production Code
**Count:** 10 occurrences in 6 production files

| File | Line | Context |
|------|------|---------|
| impact_recorder.go | 108 | goroutine background recording |
| scheduler.go | 715-721 | timeout context creation |
| chat_service.go | 312 | owner notification goroutine |
| scheduler_skew_vix.go | 20, 56, 74 | VIX operations |
| health.go | 66, 134 | shutdown and command execution |
| main.go | 76 | root context creation |

**Status:** Confirmed present — awaiting Dev assignment

---

## Code Health Checks

| Check | Result | Notes |
|-------|--------|-------|
| HTTP body.Close() | ✅ PASS | Properly deferred in all HTTP clients |
| SQL injection | ✅ PASS | No raw SQL concatenation found |
| New race conditions | ✅ PASS | Only known issue at handler_session.go |
| Resource leaks | ✅ PASS | No obvious leaks detected |
| TODO/FIXME in production | ✅ CLEAN | 0 actionable TODOs (9 XXX refs are currency notation) |
| panic() usage | ⚠️ 1 | keyring.go:40 — justified initialization failure |
| time.Now() usage | ℹ️ 233 | Testing concern — low priority |

---

## Large Untested Files (300+ lines)

Files already covered by existing tasks:
- format_cot.go (1,394) → TASK-TEST-003
- scheduler.go (1,335) → TASK-TEST-013
- handler_alpha.go (1,276) → TASK-TEST-002
- news/scheduler.go (1,134) → TASK-TEST-015
- indicators.go (1,025) → TASK-TEST-014
- format_cta.go (963) → TASK-TEST-005
- api.go (872) → TASK-TEST-004
- formatter_quant.go (847) → TASK-TEST-006
- handler_backtest.go (826) → TASK-TEST-007

**Notable untested files without task coverage (lower priority):**
- unified_outlook.go (909 lines) — AI outlook service
- fred/fetcher.go (906 lines) — FRED data fetcher
- bybit/client.go (762 lines) — Market data client
- seasonal_context.go (716 lines) — Price analysis

**Assessment:** These are lower priority than current queue. Current 22 pending tasks provide good coverage of critical infrastructure.

---

## Task Queue Status

### Pending (22 tasks)
All verified valid with accurate acceptance criteria:

| ID | Priority | Focus |
|----|----------|-------|
| TASK-BUG-001 | HIGH | Race condition fix |
| TASK-SECURITY-001 | HIGH | HTTP timeout |
| TASK-TEST-002 | HIGH | handler_alpha.go tests |
| TASK-TEST-003 | HIGH | format_cot.go tests |
| TASK-TEST-013 | HIGH | scheduler.go tests |
| TASK-TEST-015 | HIGH | news/scheduler.go tests |
| TASK-TEST-004..12, 14 | MEDIUM | Various test coverage |
| TASK-CODEQUALITY-002 | MEDIUM | context.Background() fix |
| TASK-REFACTOR-001/002 | MEDIUM | Refactoring |
| TASK-DOCS-001 | LOW | Documentation |
| TASK-CODEQUALITY-001 | LOW | Test context cleanup |

### In Review
- **TASK-TEST-001:** keyboard.go tests — 1,139 lines, 44 tests — awaiting QA review

### In Progress
- None

### Blocked
- None

---

## Recommendations

1. **Immediate Action:** Assign TASK-BUG-001 (race condition) and TASK-SECURITY-001 (HTTP timeout) to Dev-A/Dev-B
2. **QA Priority:** Complete review of TASK-TEST-001 for merge
3. **Next Sprint:** Begin TASK-TEST-013 (scheduler.go) and TASK-TEST-015 (news/scheduler.go) — core infrastructure
4. **Backlog:** Consider unified_outlook.go tests when current queue clears

---

## Conclusion

**Codebase Health:** Stable

- No new critical issues discovered
- All known issues have task coverage
- No blockers preventing development
- All agents idle and ready for assignment

**Next Audit:** Scheduled in next cron cycle

---

*Report generated by Research Agent (Agent-2)*  
*File: .agents/research/2026-04-05-1416-research-audit.md*
