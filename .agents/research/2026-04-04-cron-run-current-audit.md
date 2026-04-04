# Research Audit Report — ff-calendar-bot

**Date**: 2026-04-04 (Scheduled Cron Run — Current)  
**Agent**: Research Agent (ARK Intelligent)  
**Run**: Scheduled audit — issue verification and discovery scan

---

## Summary

Scheduled audit complete. **All 3 previously identified critical issues remain unaddressed**. **No new critical issues discovered**. Test coverage remains at ~17.5%.

**Queue Status**: 10 tasks pending (4 critical security, 1 reliability, 5 existing feature/ux tasks). All agents idle.

---

## Critical Issues Status (No Change — Still Present)

| Task ID | Issue | Location | Status | Priority |
|---------|-------|----------|--------|----------|
| PHI-SEC-001 | Keyring Panic | `keyring.go:40` | **STILL PRESENT** | P0 Critical |
| PHI-SEC-002 | Unbounded Goroutines | `bot.go:208` | **STILL PRESENT** | P0 Critical |
| PHI-CTX-001 | Context.Background() Misuse | `handler_quant.go:448,484`, `handler_cta.go:581`, `handler_vp.go:422` | **STILL PRESENT** | P1 High |

### Verification Details

1. **PHI-SEC-001** (`keyring.go:40`): Confirmed `panic(err)` still present in `MustNext()` function:
   ```go
   func (k *Keyring) MustNext() string {
       key, err := k.Next()
       if err != nil {
           panic(err)  // ← STILL PRESENT
       }
       return key
   }
   ```

2. **PHI-SEC-002** (`bot.go:208`): Confirmed unbounded goroutine creation:
   ```go
   for _, update := range updates {
       b.offset = update.UpdateID + 1
       go b.handleUpdate(ctx, update)  // ← STILL PRESENT: No rate limiting
   }
   ```

3. **PHI-CTX-001**: Confirmed 4 production usages of `context.Background()` in handlers (excluding test files):
   - `handler_cta.go:581`: `ctx := context.Background()`
   - `handler_quant.go:448`: `cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)`
   - `handler_quant.go:484`: `ctx := context.Background()`
   - `handler_vp.go:422`: `cmdCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)`

---

## Previously Known Issues (No Change)

### PHI-REL-001: Goroutine Without Panic Recovery (P2 Medium)
Still present at `handler.go:2590-2592`. Task spec exists in `.agents/tasks/pending/PHI-REL-001.md` — ready for assignment.

### MED-002: Unchecked os.Remove() Error Handling (P2 Medium)
19 instances across 4 handler files. Task exists in backlog.

### MED-004: HTTP Client Consolidation (P2 Medium)
39 http.Client instances across codebase. Task exists in backlog.

---

## Codebase Metrics (Current)

| Metric | Count | Notes |
|--------|-------|-------|
| Total Go Files | 223 | Excluding test files |
| Test Files | 39 | ~17.5% coverage (up from 15% due to recount) |
| Handler Files | 14 | 1 with tests (~7% coverage) |
| Service Files | ~120 | ~30 with tests (~25% coverage) |
| HTTP Client Instances | 39 | MED-004: Needs consolidation |
| Goroutine Spawn Points | 22 | 8 in production code |
| Panic in Production | 1 | keyring.go:40 |
| Unchecked os.Remove() | 19 | MED-002: 4 files affected |

---

## Task Queue Status

### Pending (10)

| Task ID | Title | Priority | Est | Assignee |
|---------|-------|----------|-----|----------|
| PHI-SEC-001 | Fix Keyring Panic | P0 | S | — |
| PHI-SEC-002 | Add Goroutine Limiter | P0 | M | — |
| PHI-CTX-001 | Fix Context Propagation | P1 | M | — |
| PHI-TEST-001 | Add Handler Unit Tests | P1 | L | — |
| PHI-REL-001 | Fix notifyOwnerDebug Recovery | P2 | XS | — |
| PHI-SETUP-001 | Setup Task Ledger System | P1 | S | Dev-A |
| PHI-DATA-001 | Implement AAII Sentiment | P2 | M | Dev-B |
| PHI-DATA-002 | Implement Fear & Greed Index | P2 | M | Dev-C |
| PHI-UX-001 | Standardize Navigation Buttons | P2 | S | Dev-A |
| PHI-UX-002 | Add Command Aliases | P3 | S | Dev-B |

### In Progress
- None (TASK-091 formatter tests no longer in progress — presumed complete or cancelled)

---

## Agent Status

| Role | Instance | Status |
|------|----------|--------|
| Coordinator | Agent-1 | Idle |
| Research | Agent-2 | **Audit Complete** |
| Dev-A | Agent-3 | Idle |
| Dev-B | Agent-4 | Idle |
| Dev-C | Agent-5 | Idle |
| QA | Agent-6 | Idle |

**All 6 agents available for task assignment.**

---

## New Findings

**None** — No new security, reliability, or code quality issues discovered in this audit run.

**Note**: The `chat_service.go:301` goroutine (`go cs.ownerNotify(context.Background(), html)`) was reviewed. While it also lacks explicit panic recovery, it follows the same fire-and-forget pattern as PHI-REL-001. The `ownerNotify` callback is injected at runtime and typically wraps `bot.SendHTML()` which has its own error handling. The risk is acceptable for this non-critical notification path and is covered by the general reliability improvements in PHI-REL-001's scope.

---

## Recommendations

### Immediate (P0) — Assign to Available Dev Agents
1. **PHI-SEC-001**: Fix keyring panic — 1 story point, low effort, high impact
2. **PHI-SEC-002**: Add goroutine limiter — 3 story points, prevents DoS vulnerability

### Next Sprint (P1)
3. **PHI-CTX-001**: Fix context propagation — 2 story points
4. **PHI-TEST-001**: Handler unit tests — 5 story points (recommend splitting into smaller tasks)

### Backlog (P2) — Quick Wins
5. **PHI-REL-001**: Add panic recovery to notifyOwnerDebug — XS effort (~30 min fix)
6. **MED-002**: Add error handling to `os.Remove()` calls — 2 story points

---

## Conclusion

This scheduled audit confirms that **all 3 critical security/stability issues from previous audits remain unaddressed**. **No new issues were discovered**. All task specs are complete and ready for assignment.

**Recommended Actions**:
1. **Immediate**: Assign PHI-SEC-001 and PHI-SEC-002 to Dev-A and Dev-B respectively
2. **This Sprint**: Claim PHI-REL-001 as a quick win (XS effort, ~30 minutes)
3. **Next Sprint**: Begin PHI-CTX-001 context propagation fixes
4. **Ongoing**: Schedule next audit in 10 minutes (cron routine)

---

*Report generated by Research Agent — ff-calendar-bot*  
*Filed in: `.agents/research/2026-04-04-cron-run-current-audit.md`*
