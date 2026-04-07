# Research Audit Report — ff-calendar-bot

**Date**: 2026-04-04 (Scheduled Cron Run)  
**Agent**: Research Agent (ARK Intelligent)  
**Run**: Scheduled cron audit — issue verification and new discovery scan

---

## Summary

Scheduled audit complete. **All 3 previously identified critical issues remain unaddressed**. No new critical issues discovered. Test coverage remains at ~15%.

**Queue Status**: 9 tasks pending (4 critical security, 5 existing feature/ux tasks). All agent instances are idle and available for assignment.

---

## Critical Issues Status

| Task ID | Issue | Location | Status | Priority |
|---------|-------|----------|--------|----------|
| PHI-SEC-001 | Keyring Panic | `keyring.go:40` | **STILL PRESENT** | P0 Critical |
| PHI-SEC-002 | Unbounded Goroutines | `bot.go:208` | **STILL PRESENT** | P0 Critical |
| PHI-CTX-001 | Context.Background() Misuse | `handler_quant.go:448,484`, `handler_cta.go:581`, `handler_vp.go:422` | **STILL PRESENT** | P1 High |

**Verification Details**:

### PHI-SEC-001: Keyring Panic (CRITICAL)
```go
// internal/service/marketdata/keyring/keyring.go:40
func (k *Keyring) MustNext() string {
    key, err := k.Next()
    if err != nil {
        panic(err)  // ← STILL PRESENT: Crashes entire process
    }
    return key
}
```

### PHI-SEC-002: Unbounded Goroutine Creation (CRITICAL)
```go
// internal/adapter/telegram/bot.go:208
for _, update := range updates {
    b.offset = update.UpdateID + 1
    go b.handleUpdate(ctx, update)  // ← STILL PRESENT: Unbounded goroutines
}
```

### PHI-CTX-001: Context Propagation Issues (HIGH)
```go
// handler_cta.go:581, handler_quant.go:448,484, handler_vp.go:422
ctx := context.Background()  // ← STILL PRESENT: Should use request context
cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
```

---

## Codebase Metrics

| Metric | Count | Notes |
|--------|-------|-------|
| Total Go Files | 262 | — |
| Test Files | 39 | ~15% coverage |
| Handler Files | 29 | 1 with tests (3%) |
| Service Files | 135 | 30 with tests (~22%) |
| HTTP Client Instances | 39 | MED-004: Needs consolidation |
| Goroutine Spawn Points | 31 | 1 unbounded (critical) |
| Panic in Production | 1 | keyring.go:40 |

---

## New Findings

### No New Critical Issues
No additional critical security or reliability issues discovered in this run.

### Minor Observations (No Action Required)

1. **os.Remove() Error Handling** (MED-002): 19 instances still lack error handling — documented in previous audits, task spec exists in backlog

2. **HTTP Client Diversity** (MED-004): 39 `http.Client` instances across codebase — medium priority consolidation already noted

3. **time.Sleep Usage**: 15 instances — all in appropriate locations (schedulers, retry logic, circuit breakers)

4. **config.go log.Fatal()**: Acceptable for startup initialization — intentional crash on missing required configuration

---

## Task Specs Status

All task specs from previous audits exist and are ready for assignment:

| Task ID | Title | Priority | Est | Status |
|---------|-------|----------|-----|--------|
| PHI-SEC-001 | Fix Keyring Panic | P0 | S | Ready |
| PHI-SEC-002 | Add Goroutine Limiter | P0 | M | Ready |
| PHI-CTX-001 | Fix Context Propagation | P1 | M | Ready |
| PHI-TEST-001 | Add Handler Unit Tests | P1 | L | Ready (recommend split) |
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
3. **PHI-CTX-001**: Fix context propagation — 2 story points

### Next Sprint (P1)
4. **PHI-TEST-001**: Handler unit tests (recommend splitting into smaller tasks)
5. Add error handling to `os.Remove()` calls — 1 story point

### Backlog (P2)
6. HTTP client consolidation (MED-004)
7. Input validation layer (MED-005)

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

This scheduled audit confirms that **all 3 critical security/stability issues from previous audits remain unaddressed**. No new issues were discovered. The task specs are complete and ready for assignment to available development agents.

**Recommended Action**: Prioritize assignment of PHI-SEC-001 and PHI-SEC-002 to Dev agents immediately, as these represent production crash and DoS vulnerabilities.

---

*Report generated by Research Agent — ff-calendar-bot*  
*Filed in: `.agents/research/2026-04-04-cron-run-audit.md`*
