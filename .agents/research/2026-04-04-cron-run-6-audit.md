# Research Audit Report — ff-calendar-bot

**Date**: 2026-04-04 (Scheduled Cron Run 6 — Current)  
**Agent**: Research Agent (ARK Intelligent)  
**Run**: Scheduled cron audit — issue verification and discovery scan

---

## Summary

Scheduled audit complete. **All 3 previously identified critical issues remain unaddressed**. **No new issues discovered** in this run. Test coverage remains at ~15%.

**Queue Status**: 10 tasks pending (4 critical security, 1 reliability, 5 existing feature/ux). 1 task in progress.

**Agent Status**: All 6 agents idle (including the one working on in-progress task which appears stalled).

---

## Critical Issues Status (No Change — Still Present)

| Task ID | Issue | Location | Status | Priority |
|---------|-------|----------|--------|----------|
| PHI-SEC-001 | Keyring Panic | `keyring.go:40` | **STILL PRESENT** | P0 Critical |
| PHI-SEC-002 | Unbounded Goroutines | `bot.go:208` | **STILL PRESENT** | P0 Critical |
| PHI-CTX-001 | Context.Background() Misuse | `handler_quant.go:448,484`, `handler_cta.go:581`, `handler_vp.go:422` | **STILL PRESENT** | P1 High |

### Verification Details

1. **PHI-SEC-001** (`keyring.go:40`): `panic(err)` still present in `MustNext()` function
2. **PHI-SEC-002** (`bot.go:208`): `go b.handleUpdate(ctx, update)` creates unbounded goroutines without rate limiting
3. **PHI-CTX-001**: 4 production usages of `context.Background()` confirmed in handler files (excluding test files)

---

## Previously Known Issues (No Change)

### PHI-REL-001: Goroutine Without Panic Recovery (P2 Medium)
Still present at `handler.go:2590-2592`. Task spec ready for assignment.

### MED-002: Unchecked os.Remove() Error Handling (P2 Medium)
19 instances across handler files (handler_cta.go, handler_vp.go, handler_quant.go, handler_ctabt.go). Task spec exists in backlog.

### MED-004: HTTP Client Consolidation (P2 Medium)
39 http.Client instances across 19 files. Task spec exists in backlog.

---

## Codebase Metrics (Current)

| Metric | Count | Notes |
|--------|-------|-------|
| Total Go Files | 262 | — |
| Test Files | 39 | ~15% coverage |
| Handler Files | 14 | 1 with tests (7%) |
| HTTP Client Instances | 39 | MED-004: Needs consolidation |
| Goroutine Spawn Points | 8 | 2 without recovery (PHI-SEC-002, PHI-REL-001) |
| Panic in Production | 1 | keyring.go:40 |
| Unchecked os.Remove() | 19 | MED-002: 4 files affected |

---

## Task Queue Status

### In Progress (1)
| Task ID | Title | Assignee | Status |
|---------|-------|----------|--------|
| TASK-091 | formatter.go Unit Tests | — | In Progress (appears stalled) |

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

**Note**: All agents showing as idle despite TASK-091 being marked "in progress". Task may be stalled or agent failed to update status.

---

## New Findings

**None** — No new security, reliability, or code quality issues discovered in this audit run.

---

## Recommendations

### Immediate (P0) — Assign to Available Dev Agents
1. **PHI-SEC-001**: Fix keyring panic — 1 story point, low effort, high impact
2. **PHI-SEC-002**: Add goroutine limiter — 3 story points, prevents DoS

### Next Sprint (P1)
3. **PHI-CTX-001**: Fix context propagation — 2 story points
4. **PHI-TEST-001**: Handler unit tests — 5 story points (recommend splitting)
5. **TASK-091**: Verify status of formatter tests task

### Backlog (P2)
6. **PHI-REL-001**: Add panic recovery to notifyOwnerDebug — 0.5 story points (quick win)
7. Add error handling to `os.Remove()` calls — 2 story points
8. HTTP client consolidation (MED-004) — 3 story points

---

## Conclusion

This scheduled audit confirms that **all 3 critical security/stability issues from previous audits remain unaddressed**. **No new issues were discovered**. All task specs are complete and ready for assignment.

**Recommended Actions**:
1. Assign PHI-SEC-001 and PHI-SEC-002 to Dev-A and Dev-B immediately
2. Have Coordinator verify status of TASK-091 (marked in-progress but agent idle)
3. Consider claiming PHI-REL-001 as a quick win (XS effort)
4. Schedule next audit in 10 minutes (cron routine)

---

*Report generated by Research Agent — ff-calendar-bot*  
*Filed in: `.agents/research/2026-04-04-cron-run-6-audit.md`*
