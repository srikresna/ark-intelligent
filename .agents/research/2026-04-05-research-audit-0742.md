# Research Agent Audit Report — 2026-04-05 07:42 UTC

**Auditor:** Research Agent (ARK Intelligent)  
**Scope:** Full codebase audit (401 Go files, 108 test files)  
**Status:** All agents idle, 0 blockers, 22 pending tasks verified valid

---

## Executive Summary

Comprehensive audit of the ff-calendar-bot codebase completed. **No new task specs created** — all actionable issues already covered by existing 22 pending tasks. Verified three high-priority issues remain unfixed. Codebase health stable.

| Metric | Value |
|--------|-------|
| Total Go Files | 401 |
| Test Files | 108 |
| Test Coverage | ~27% (293-318 files untested) |
| Pending Tasks | 22 |
| In Progress | 0 |
| Blockers | 0 |
| New Issues Found | 0 |

---

## Verified Unfixed Issues (Known Technical Debt)

### 1. TASK-BUG-001: Data Race in handler_session.go ⚠️ HIGH PRIORITY
**Status:** CONFIRMED — Still unfixed  
**Location:** `internal/adapter/telegram/handler_session.go:23`

```go
var sessionAnalysisCache = map[string]*sessionCache{}  // line 23 — no synchronization
```

**Concurrent Access Points:**
- Line 57: Read access (`sessionAnalysisCache[mapping.Currency]`)
- Line 94: Write access (`sessionAnalysisCache[mapping.Currency] = ...`)

**Risk:** Concurrent map write panic in production under concurrent Telegram requests.

**Fix Required:** Add `sync.RWMutex` wrapper (see task spec for implementation).

---

### 2. TASK-SECURITY-001: HTTP DefaultClient Timeout ⚠️ HIGH PRIORITY
**Status:** CONFIRMED — Still unfixed  
**Location:** `internal/service/macro/tradingeconomics_client.go:246`

```go
resp, err := http.DefaultClient.Do(req)  // line 246 — no timeout
```

**Risk:** 
- Requests can hang indefinitely
- Goroutine leaks under load
- Circuit breaker ineffective
- Potential DoS vector

**Fix Required:** Use `http.Client{Timeout: 30s}` with `httpclient.SharedTransport`.

---

### 3. TASK-CODEQUALITY-002: context.Background() in Production ⚠️ MEDIUM PRIORITY
**Status:** CONFIRMED — 9-10 occurrences still present

**Affected Files:**
| File | Lines | Context |
|------|-------|---------|
| `internal/scheduler/scheduler_skew_vix.go` | 20, 56, 74 | VIX alerts, broadcast messages |
| `internal/service/news/scheduler.go` | 721 | Impact recording (5m timeout) |
| `internal/service/news/impact_recorder.go` | 108 | Delayed recording goroutine |
| `internal/service/ai/chat_service.go` | 312 | Owner notification goroutine |
| `internal/health/health.go` | 66, 134 | Shutdown timeout, Python check |

**Note:** Some usages are intentional (fire-and-forget goroutines), but should be documented. Others should accept passed contexts for proper cancellation propagation.

---

## Code Health Assessment

### Resource Management ✓
- **76 defer Close() patterns found** — proper resource cleanup
- HTTP response bodies consistently closed
- Database connections properly managed
- File handles properly closed

### Concurrency Safety
- `sync.Mutex` used for `sentReminders` in news/scheduler.go ✓
- `sync.RWMutex` used for TE cache in tradingeconomics_client.go ✓
- **Race condition exists** in handler_session.go (tracked as TASK-BUG-001)

### Error Handling ✓
- **3 panic() calls identified** — all justified:
  - `keyring.go:40` — MustNext() pattern (by design)
  - `fetcher_test.go:322` — test helper
  - `saferun_test.go:41` — test panic (intentional)

### Security Scan ✓
- **Only 1 http.DefaultClient usage** — tracked as TASK-SECURITY-001
- **2 SQL.Exec patterns** — both in `cot/fetcher.go`, appear parameterized
- No SQL injection vectors detected
- No hardcoded credentials found

### Code Quality
- **9 TODO/FIXME comments** — normal residue, acceptable level
- **84 time.Now() usages** — testing concern (low priority)
- Magic numbers present in handler_cta.go — tracked as TASK-REFACTOR-001

---

## Test Coverage Status

### Files with Tests ✓
| Package | Test File | Tests |
|---------|-----------|-------|
| `internal/adapter/telegram` | keyboard_test.go | 44 tests, 1,139 lines |
| `internal/service/fred` | Multiple files | Good coverage |
| `internal/service/price` | Multiple files | Good coverage |
| `pkg/*` | Various | Good coverage |

### Critical Untested Files (Tasks Exist)
| File | Lines | Task |
|------|-------|------|
| `internal/scheduler/scheduler.go` | 1,335 | TASK-TEST-013 |
| `internal/service/news/scheduler.go` | 1,134 | TASK-TEST-015 |
| `internal/service/ta/indicators.go` | 1,025 | TASK-TEST-014 |
| `internal/adapter/telegram/handler_session.go` | 198 | Covered by bug fix |

### In Review
- **TASK-TEST-001**: keyboard_test.go — 44 tests, 1,139 lines, awaiting QA

---

## Audit Findings: No New Issues

### Checked For | Result
--------------|--------
New race conditions | None found
New http.DefaultClient | None found
SQL injection risks | None found
Resource leaks | None found
New panic() calls | None found
Breaking API changes | None found

---

## Recommendations

### Immediate (High Priority)
1. **Fix TASK-BUG-001** — Race condition can cause production panics
2. **Fix TASK-SECURITY-001** — HTTP timeout is a reliability/security risk
3. **QA Review TASK-TEST-001** — 1,139 lines of keyboard tests ready

### Short Term (Medium Priority)
4. **Address TASK-CODEQUALITY-002** — Context propagation for better cancellation
5. **Start TASK-TEST-013** — Scheduler tests (critical infrastructure)
6. **Start TASK-TEST-015** — News scheduler tests (alert system)

### Long Term (Low Priority)
7. Address 84 `time.Now()` usages for testability
8. Add doc.go to 62 directories without package documentation
9. Continue test coverage expansion (target: 50%+)

---

## Agent Coordination Status

| Role | Instance | Status |
|------|----------|--------|
| Coordinator | Agent-1 | Idle |
| Research | Agent-2 | Idle (this audit) |
| Dev-A | Agent-3 | Idle |
| Dev-B | Agent-4 | Idle |
| Dev-C | Agent-5 | Idle |
| QA | Agent-6 | Idle |

**Queue:** 22 tasks pending, 0 in progress, 0 blocked  
**Next Actions:** Assign high-priority bugs (TASK-BUG-001, TASK-SECURITY-001) to Dev agents

---

## Conclusion

Codebase health is **stable**. All critical issues are tracked. The three high-priority issues (race condition, HTTP timeout, context.Background()) remain unfixed and should be addressed before next deployment. No new task specs required — existing queue covers all actionable technical debt.

**Next Audit:** Scheduled for next cron interval or on-demand.

---

*Report generated by Research Agent*  
*Location: `/home/ubuntu/ff-calendar-bot/.agents/research/2026-04-05-research-audit-0742.md`*
