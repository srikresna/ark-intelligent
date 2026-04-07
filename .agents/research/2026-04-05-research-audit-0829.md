# Research Audit Report — 2026-04-05 08:29 UTC

**Auditor:** Research Agent (ff-calendar-bot)  
**Scope:** Full codebase audit — 509 Go files (401 production, 108 test)  
**Duration:** ~5 minutes  

---

## Executive Summary

**Status:** All 22 pending tasks remain valid. **0 new task specs created.**

| Metric | Value |
|--------|-------|
| Production files | 401 |
| Files with tests | 83 (20.7%) |
| Files without tests | 318 (79.3%) |
| Confirmed unfixed issues | 3 |
| New critical issues | 0 |
| Blockers | 0 |

---

## Confirmed Unfixed Issues (from existing tasks)

### 1. TASK-BUG-001: Data Race in handler_session.go ⚠️ HIGH
**Status:** Still unfixed  
**Location:** `internal/adapter/telegram/handler_session.go:23,57,94`  
**Issue:** Global map `sessionAnalysisCache` accessed concurrently without synchronization
```go
var sessionAnalysisCache = map[string]*sessionCache{}  // line 23 - no mutex!
```
**Verification:** No sync.Mutex or sync.RWMutex found in handler_session.go, while other handlers (handler_cta.go, handler_wyckoff.go, etc.) all have proper mutex protection.

### 2. TASK-SECURITY-001: http.DefaultClient Without Timeout ⚠️ HIGH
**Status:** Still unfixed  
**Location:** `internal/service/macro/tradingeconomics_client.go:246`  
**Issue:** Uses `http.DefaultClient.Do(req)` without timeout, risking goroutine leaks
```go
resp, err := http.DefaultClient.Do(req)  // line 246 - no timeout!
```

### 3. TASK-CODEQUALITY-002: context.Background() in Production ⚠️ MEDIUM
**Status:** Still unfixed — 9 occurrences confirmed  
**Locations:**
1. `cmd/bot/main.go:76` — root context creation (proper usage, excluded from count)
2. `internal/scheduler/scheduler_skew_vix.go:20,56,74` — 3 occurrences
3. `internal/service/news/scheduler.go:721` — 1 occurrence
4. `internal/service/news/impact_recorder.go:108` — 1 occurrence
5. `internal/service/ai/chat_service.go:312` — 1 occurrence
6. `internal/health/health.go:66,134` — 2 occurrences

---

## Test Coverage Analysis

### Files with Tests (83 files, 20.7%)
Test coverage concentrated in:
- `internal/service/fred/*` — well tested
- `internal/service/price/*` — moderately tested
- `internal/service/strategy/*` — moderately tested

### Large Untested Files (Top 10)
| File | Lines | Task Coverage |
|------|-------|---------------|
| format_cot.go | 1,394 | TASK-TEST-003 (pending) |
| scheduler.go | 1,335 | TASK-TEST-013 (pending) |
| handler_alpha.go | 1,276 | TASK-TEST-002 (pending) |
| news/scheduler.go | 1,134 | TASK-TEST-015 (pending) |
| ta/indicators.go | 1,025 | TASK-TEST-014 (pending) |
| format_cta.go | 963 | TASK-TEST-005 (pending) |
| unified_outlook.go | 909 | Lower priority |
| fred/fetcher.go | 906 | Lower priority |
| api.go | 872 | TASK-TEST-004 (pending) |
| formatter_quant.go | 847 | TASK-TEST-006 (pending) |

All critical infrastructure files are already covered by pending tasks.

---

## Security Scan

| Check | Status |
|-------|--------|
| SQL injection | ✅ Clean — no raw SQL concatenation |
| HTTP body.Close() | ✅ All proper defer statements present |
| Hardcoded secrets | ✅ No API keys in source |
| panic() in prod | ✅ Clean — only in test files |
| log.Fatal() in prod | ✅ Clean — only in test files |

---

## Code Quality Checks

| Check | Status | Notes |
|-------|--------|-------|
| context.Background() in prod | ⚠️ 9 occurrences | Tracked in TASK-CODEQUALITY-002 |
| time.Now() usage | ℹ️ 233 usages | Makes testing harder; low priority |
| Magic numbers | ⚠️ Present | Tracked in TASK-REFACTOR-001 |
| Error handling | ✅ Consistent | No naked returns or ignored errors |

---

## New Findings

**None.** All actionable issues are already covered by existing 22 pending tasks.

### Lower Priority Gaps (No Task Created)
1. `internal/service/ai/unified_outlook.go` — 909 lines, 0 tests
2. `internal/service/fred/fetcher.go` — 906 lines, 0 tests

These are lower priority than the existing queue of 22 pending tasks.

---

## Recommendations

### Immediate (High Priority)
1. **Assign TASK-BUG-001** — Data race is a ticking time bomb in production
2. **Assign TASK-SECURITY-001** — Simple 1-hour fix with high security impact

### Short Term (Medium Priority)
3. Continue with existing test coverage tasks (TASK-TEST-002 through TASK-TEST-015)
4. Address TASK-CODEQUALITY-002 for proper context propagation

### Long Term (Low Priority)
5. Add tests for unified_outlook.go and fred/fetcher.go after current queue clears

---

## Conclusion

Codebase health: **Stable**. No new critical issues identified. All 22 pending tasks remain valid and accurately describe current technical debt.

**Action Required:** Coordinate task assignment for the 3 high-priority unfixed issues (TASK-BUG-001, TASK-SECURITY-001, and complete QA review for TASK-TEST-001).

---

*Report generated: 2026-04-05 08:29 UTC*  
*Next audit recommended: 2026-04-05 09:00 UTC or upon significant code changes*
