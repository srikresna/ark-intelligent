# Research Agent Audit Report — 2026-04-05 10:54 UTC

**Agent:** Research Agent (Agent-2)  
**Audit Scope:** Comprehensive codebase analysis — ff-calendar-bot  
**Triggered By:** Scheduled cron audit  

---

## Executive Summary

All agents idle, 0 blockers. **No new task specs created** — comprehensive audit of 509 Go files verified all 22 pending tasks remain valid. No code changes since 10:40 UTC.

| Metric | Value |
|--------|-------|
| Total Go files | 509 |
| Test files | 108 |
| Production files | 401 |
| Untested production files | 317 (79% untested) |
| Pending tasks | 22 |
| In progress | 0 |
| In review | 1 (TASK-TEST-001) |
| Blockers | 0 |

---

## Verified Issues (Still Unfixed)

### 1. TASK-BUG-001: Data Race in handler_session.go ✅ Confirmed
- **Location:** `internal/adapter/telegram/handler_session.go:23`
- **Issue:** Global map `sessionAnalysisCache` accessed without synchronization
- **Risk:** Concurrent map write panic, data corruption
- **Fix:** Add sync.RWMutex (see task spec for implementation)

### 2. TASK-SECURITY-001: http.DefaultClient Timeout ✅ Confirmed
- **Location:** `internal/service/macro/tradingeconomics_client.go:246`
- **Issue:** `http.DefaultClient.Do(req)` without timeout
- **Risk:** Requests hang indefinitely, goroutine leaks, resource exhaustion
- **Fix:** Use http.Client with 30s timeout

### 3. TASK-CODEQUALITY-002: context.Background() in Production ✅ Confirmed
- **Count:** 8 occurrences in 6 production files
- **Files affected:**
  - `internal/service/news/impact_recorder.go:108`
  - `internal/service/news/scheduler.go:721`
  - `internal/service/ai/chat_service.go:312`
  - `internal/scheduler/scheduler_skew_vix.go:20,56,74` (3 occurrences)
  - `internal/health/health.go:66,134`
  - `cmd/bot/main.go:76` (acceptable — root context)

---

## Security Scan Results

| Check | Status | Details |
|-------|--------|---------|
| SQL injection | ✅ Clean | No database queries found |
| HTTP body close | ✅ Clean | 56 proper body.Close() patterns |
| Unsafe/syscall/cgo | ✅ Clean | No unsafe operations |
| panic() usage | ⚠️ 1 justified | `keyring.go:40` — critical init failure |
| Hardcoded secrets | ✅ Clean | No credentials in code |
| http.DefaultClient | ⚠️ Found | TASK-SECURITY-001 still pending |

---

## Code Quality Scan

| Check | Status | Count |
|-------|--------|-------|
| context.Background() | ⚠️ Needs fix | 8 occurrences (TASK-CODEQUALITY-002) |
| time.Now() usage | ℹ️ Info | 233 usages (testing concern, low priority) |
| TODO/FIXME comments | ✅ Clean | 0 in production |
| Magic numbers | ℹ️ Tracked | TASK-REFACTOR-001 covers this |

---

## Test Coverage Analysis

### Files with Test Coverage (by existing tasks)
| Task | File | Lines | Status |
|------|------|-------|--------|
| TASK-TEST-001 | keyboard.go | 1,139 lines tests | ✅ In review |
| TASK-TEST-002 | handler_alpha.go | 1,276 lines code | 📝 Pending |
| TASK-TEST-003 | format_cot.go | 1,394 lines code | 📝 Pending |
| TASK-TEST-004 | api.go | 872 lines code | 📝 Pending |
| TASK-TEST-005 | format_cta.go | 963 lines code | 📝 Pending |
| TASK-TEST-006 | formatter_quant.go | 847 lines code | 📝 Pending |
| TASK-TEST-007 | handler_backtest.go | 826 lines code | 📝 Pending |
| TASK-TEST-008 | storage layer | 17 files | 📝 Pending |
| TASK-TEST-009 | format_price.go | 697 lines code | 📝 Pending |
| TASK-TEST-010 | format_macro.go | 693 lines code | 📝 Pending |
| TASK-TEST-011 | format_sentiment.go | 552 lines code | 📝 Pending |
| TASK-TEST-012 | bot.go | TBD | 📝 Pending |
| TASK-TEST-013 | scheduler.go | 1,335 lines code | 📝 Pending |
| TASK-TEST-014 | indicators.go | 1,025 lines code | 📝 Pending |
| TASK-TEST-015 | news/scheduler.go | 1,134 lines code | 📝 Pending |

### Large Untested Files (No Task Coverage — Lower Priority)

These files are 500+ lines but lower priority than current queue:

| File | Lines | Notes |
|------|-------|-------|
| `unified_outlook.go` | 909 | AI service |
| `fred/fetcher.go` | 906 | Data fetching |
| `bybit/client.go` | 762 | Exchange client |
| `seasonal_context.go` | 716 | Price analysis |
| `ai/prompts.go` | 702 | AI prompts |
| `ai/claude.go` | 688 | AI service |
| `handler_ctabt.go` | 686 | Handler |
| `handler_cot_cmd.go` | 667 | Handler |
| `keyboard_trading.go` | 651 | UI |
| `ta/confluence.go` | 646 | Technical analysis |
| `handler_onboarding.go` | 632 | Handler |
| `handler_quant.go` | 627 | Handler |
| `cot/fetcher.go` | 599 | Data fetching |
| `factor_decomposition.go` | 597 | Backtest |
| `ta/backtest.go` | 567 | Technical analysis |
| `hmm_regime.go` | 546 | Price analysis |
| `handler_cta.go` | 531 | Handler |
| `format_backtest.go` | 528 | Formatter |

---

## Task Queue Verification

All 22 pending tasks verified valid and accurately describe current technical debt:

### High Priority (5 tasks)
1. TASK-BUG-001 — Data race fix
2. TASK-SECURITY-001 — HTTP timeout fix
3. TASK-TEST-002 — handler_alpha.go tests
4. TASK-TEST-003 — format_cot.go tests
5. TASK-TEST-013 — scheduler.go tests

### Medium Priority (12 tasks)
- TASK-TEST-004 through TASK-TEST-012
- TASK-TEST-014, TASK-TEST-015
- TASK-REFACTOR-001, TASK-REFACTOR-002
- TASK-CODEQUALITY-002

### Low Priority (3 tasks)
- TASK-CODEQUALITY-001 (test files context)
- TASK-DOCS-001 (emoji documentation)
- TASK-TEST-008 (storage layer)

---

## Recommendations

### Immediate (Next 24 Hours)
1. **QA Review TASK-TEST-001** — keyboard.go tests ready for review (1,139 lines, 44 tests)
2. **Assign TASK-BUG-001** — Race condition is highest priority bug
3. **Assign TASK-SECURITY-001** — Simple 1-hour security fix

### Short Term (This Week)
4. Pick up test tasks for critical infrastructure:
   - TASK-TEST-013: scheduler.go (1,335 lines — core orchestration)
   - TASK-TEST-015: news/scheduler.go (1,134 lines — alert infrastructure)

### Medium Term (Next Sprint)
5. Complete remaining test coverage tasks
6. Address TASK-CODEQUALITY-002 (context.Background())

---

## Audit Methodology

1. **File Inventory:** Counted 509 Go files (401 production + 108 test)
2. **Git Check:** Verified no code changes since 10:40 UTC
3. **Issue Verification:** Confirmed 3 critical issues still present
4. **Security Scan:** Checked for SQL injection, unsafe operations, panics
5. **Coverage Analysis:** Identified 317 untested files
6. **Task Validation:** Verified all 22 pending tasks still accurate

---

## Conclusion

Codebase health: **Stable**. All known issues are tracked. No new blockers or critical findings. The 22 pending tasks accurately represent current technical debt.

**Next Actions:**
- QA should review TASK-TEST-001
- Dev agents should pick up high-priority bugs (TASK-BUG-001, TASK-SECURITY-001)

---

*Report generated by Research Agent — 2026-04-05 10:54 UTC*
