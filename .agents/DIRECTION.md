# DIRECTION — Ark Intelligent Sprint Priorities

**Last Updated:** 2026-04-03 WIB  
**Sprint:** UX Improvement Siklus 1 (Closing) → DI Refactoring Siklus 2  
**Status:** ⚠️ **CRITICAL: QA Bottleneck — 10 PRs Pending**

---

## ⚠️ Critical Priority: QA Capacity (P0)

### QA Bottleneck Alert
**10 PRs are awaiting QA review**, creating significant risk of:
- Merge conflicts as main branch diverges
- Delayed feedback loops for dev agents
- Blocked progression to Siklus 2

| PR Count | Status | Age |
|----------|--------|-----|
| 4 | Original UX Siklus 1 PRs | > 8 hours |
| 6 | New PRs ready for submission | Current |

**Immediate Action Required:**
1. **QA:** Begin review of 4 original PRs immediately
2. **CTO:** Review QA capacity — consider parallel review or additional resources
3. **Dev agents:** Submit remaining 6 PRs to queue (don't wait)

---

## Current Development State (P1)

### Dev Agent Status
| Agent | Status | Next Action |
|-------|--------|-------------|
| **Dev-A** | ✅ TASK-094-D complete | Submit PR, await QA |
| **Dev-B** | 🔄 TASK-307 assigned | Begin audit, submit TASK-001-EXT PR |
| **Dev-C** | ✅ 4 tasks complete | Submit 4 PRs (consider batching VIX fixes) |

### Ready for PR Submission
1. **TASK-094-D** (Dev-A) — HandlerDeps struct → `feat/TASK-094-D`
2. **TASK-001-EXT** (Dev-B) — Onboarding role selector → `feat/TASK-001-EXT-onboarding-role-selector`
3. **TASK-141** (Dev-C) — VIX EOF error handling → `feat/TASK-141-vix-fetcher-eof-vs-parse-error`
4. **TASK-142** (Dev-C) — VIX cache error propagation → `feat/TASK-142-vix-cache-error-propagation`
5. **TASK-143** (Dev-C) — Log silenced errors → `feat/TASK-143-log-silenced-errors-bot-handler`
6. **TASK-147** (Dev-C) — Wyckoff phase guard → `feat/TASK-147-wyckoff-phase-boundary-neg1-guard`

---

## Next Sprint (P2) — DI Framework Completion

**Blocked until QA clears backlog.**

Per ADR-012 (DI Framework Evaluation):

| Task | Assignee | Est. | Dependency |
|------|----------|------|------------|
| TASK-094-Cleanup: main.go <200 LOC | Dev-A | S | After TASK-094-D merged |
| TASK-094-Docs: Update TECH_REFACTOR_PLAN.md | TechLead | XS | After cleanup |

---

## Backlog (P3)

### Technical Debt
- **TASK-308:** Connection pool metrics export (observability)
- **TASK-309:** BadgerDB compaction schedule optimization

### Features
- **TASK-006:** Help command search/filter functionality
- **TASK-011:** Multi-language support (ID/EN) for responses

---

## Escalation Log

| Date | Issue | Status | Resolution |
|------|-------|--------|------------|
| 2026-04-03 | Dev-C TASK-307 inactivity | ✅ **RESOLVED** | Reassigned to Dev-B; root cause was task prioritization gap |
| 2026-04-03 | QA Bottleneck (10 PRs) | ⚠️ **ACTIVE** | CTO review required; consider additional QA resources |

---

## Decision Log

| Date | Decision | Context |
|------|----------|---------|
| 2026-04-02 | DI Framework: Manual restructuring (Option C) | ADR-012 evaluation complete |
| 2026-04-03 | No new DI framework dependencies | wire/fx overhead not justified |
| 2026-04-03 | Dev-C workload clarification | 4 active tasks discovered; not inactive |
| 2026-04-03 | TASK-307 reassigned to Dev-B | Dev-C has sufficient workload |

---

## Blockers

| Blocker | Severity | Owner | ETA |
|---------|----------|-------|-----|
| ⚠️ QA Bottleneck (10 PRs) | **CRITICAL** | CTO/QA | TBD |

---

*Direction maintained by: TechLead-Intel*  
*⚠️ CRITICAL: QA capacity is blocking sprint progression. Immediate attention required.*
