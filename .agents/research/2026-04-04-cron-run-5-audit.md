# Research Audit Report — ff-calendar-bot

**Date**: 2026-04-04 (Scheduled Cron Run 5)  
**Agent**: Research Agent (ARK Intelligent)  
**Run**: Scheduled cron audit — issue verification and new discovery scan

---

## Summary

Scheduled audit complete. **All 3 previously identified critical issues remain unaddressed**. **1 new minor reliability issue discovered** (PHI-REL-001). Test coverage is 17.5% (up from previously reported 15% due to recount methodology).

**Queue Status**: 9 tasks pending (4 critical security, 5 existing feature/ux tasks). All agent instances are idle and available for assignment.

---

## Critical Issues Status (No Change)

| Task ID | Issue | Location | Status | Priority |
|---------|-------|----------|--------|----------|
| PHI-SEC-001 | Keyring Panic | `keyring.go:40` | **STILL PRESENT** | P0 Critical |
| PHI-SEC-002 | Unbounded Goroutines | `bot.go:208` | **STILL PRESENT** | P0 Critical |
| PHI-CTX-001 | Context.Background() Misuse | `handler_quant.go:448,484`, `handler_cta.go:581`, `handler_vp.go:422` | **STILL PRESENT** | P1 High |

---

## New Finding

### PHI-REL-001: Goroutine Without Panic Recovery (MEDIUM)

**Location**: `internal/adapter/telegram/handler.go:2590-2592`

**Code**:
```go
func (h *Handler) notifyOwnerDebug(ctx context.Context, html string) {
    ownerID := h.bot.OwnerID()
    if ownerID <= 0 {
        return
    }
    go func() {
        _, _ = h.bot.SendHTML(ctx, fmt.Sprintf("%d", ownerID), html)
    }()  // ← No panic recovery
}
```

**Issue**: Fire-and-forget goroutine lacks `defer recover()` protection. If `SendHTML` panics (e.g., nil pointer in formatting, API client issue), the entire process will crash.

**Risk**: Process crash from non-critical debug notification path  
**Effort**: Very Low — add defer/recover block  
**Priority**: P2 (Medium)

**Recommended Fix**:
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).Msg("notifyOwnerDebug panic recovered")
        }
    }()
    _, _ = h.bot.SendHTML(ctx, fmt.Sprintf("%d", ownerID), html)
}()
```

---

## Codebase Metrics (Updated)

| Metric | Count | Notes |
|--------|-------|-------|
| Total Go Files | 223 | — |
| Test Files | 39 | 17.5% coverage |
| Handler Files | 14 | 0 with tests (0%) |
| Service Files | ~120 | ~30 with tests (~25%) |
| HTTP Client Instances | 39 | MED-004: Needs consolidation |
| Goroutine Spawn Points | 25 | 2 without recovery (PHI-SEC-002, PHI-REL-001) |
| Panic in Production | 1 | keyring.go:40 |
| Unchecked os.Remove() | 11 | MED-002: 4 files affected |

---

## Previously Known Issues (No Change)

### PHI-SEC-001: Keyring Panic (P0 Critical)
Still present at `internal/service/marketdata/keyring/keyring.go:40`.

### PHI-SEC-002: Unbounded Goroutine Creation (P0 Critical)
Still present at `internal/adapter/telegram/bot.go:208`.

### PHI-CTX-001: Context Propagation Issues (P1 High)
Still present across 4 locations in handler files.

### MED-002: Unchecked os.Remove() Error Handling (P2 Medium)
11 instances across 4 files. Task spec exists in backlog.

### MED-004: HTTP Client Consolidation (P2 Medium)
39 http.Client instances. Task spec exists in backlog.

---

## Task Specs Status

All task specs ready for assignment:

| Task ID | Title | Priority | Est | Status |
|---------|-------|----------|-----|--------|
| PHI-SEC-001 | Fix Keyring Panic | P0 | S | Ready |
| PHI-SEC-002 | Add Goroutine Limiter | P0 | M | Ready |
| PHI-CTX-001 | Fix Context Propagation | P1 | M | Ready |
| PHI-TEST-001 | Add Handler Unit Tests | P1 | L | Ready |
| PHI-REL-001 | Fix notifyOwnerDebug Panic Recovery | P2 | XS | **NEW — needs task spec** |
| PHI-SETUP-001 | Setup Task Ledger System | P1 | S | Assigned Dev-A |
| PHI-DATA-001 | Implement AAII Sentiment | P2 | M | Assigned Dev-B |
| PHI-DATA-002 | Implement Fear & Greed Index | P2 | M | Assigned Dev-C |
| PHI-UX-001 | Standardize Navigation Buttons | P2 | S | Assigned Dev-A |
| PHI-UX-002 | Add Command Aliases | P3 | S | Assigned Dev-B |

---

## Recommendations

### Immediate (P0) — Assign to Available Dev Agents
1. **PHI-SEC-001**: Fix keyring panic — 1 story point, low effort
2. **PHI-SEC-002**: Add goroutine limiter — 3 story points, medium effort

### Next Sprint (P1)
3. **PHI-CTX-001**: Fix context propagation — 2 story points
4. **PHI-TEST-001**: Handler unit tests (recommend splitting)

### Backlog (P2)
5. **PHI-REL-001**: Add panic recovery to notifyOwnerDebug — 0.5 story points
6. Add error handling to `os.Remove()` calls — 1 story point
7. HTTP client consolidation (MED-004)

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

All 3 Dev agents are available to claim the P0 critical security tasks.

---

## Conclusion

This scheduled audit confirms that **all 3 critical security/stability issues from previous audits remain unaddressed**. **1 new minor reliability issue was discovered** (PHI-REL-001: goroutine without panic recovery). The task specs are complete and ready for assignment.

**Recommended Action**: 
1. Create task spec for PHI-REL-001
2. Prioritize assignment of PHI-SEC-001 and PHI-SEC-002 to Dev agents immediately
3. Consider claiming PHI-REL-001 as a quick win (XS effort)

---

*Report generated by Research Agent — ff-calendar-bot*  
*Filed in: `.agents/research/2026-04-04-cron-run-5-audit.md`*
