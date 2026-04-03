# DIRECTION — Ark Intelligent Sprint Priorities

**Last Updated:** 2026-04-03 WIB  
**Sprint:** UX Improvement Siklus 1 (Closing) → DI Refactoring Siklus 2  
**Status:** ✅ **Corrected — 5 PRs Ready, 5 Already Merged**

---

## ✅ Priority: Clear PR Queue (P1)

### PR Queue Status
**5 PRs have been submitted and are in review**, all need rebase to pick up new CI.

| PR | Task | Assignee | Status | Action |
|----|------|----------|--------|--------|
| #346 | TASK-002 | Dev-A | 🔄 Awaiting rebase | Rebase on main |
| #347 | PHI-119 | Dev-C | 🔄 Awaiting rebase | Rebase on main |
| #348 | TASK-001-EXT | Dev-B | 🔄 Awaiting rebase | Rebase on main |
| #349 | TASK-094-C3 | Dev-A | 🔄 Awaiting rebase | Rebase on main |
| #350 | TASK-094-D | Dev-A | 🔄 Awaiting rebase | Rebase on main |

**Immediate Action Required:**
1. **Dev-A:** Rebase #346, #349, #350 on main
2. **Dev-B:** Rebase #348 on main
3. **Dev-C:** Rebase #347 on main
4. **QA:** Begin review once CI passes after rebase

**Root Cause:** Main branch now has comprehensive CI/CD with linting (commit 008a86b). PRs created before this update need rebase.

---

## Current Development State (P1)

### Dev Agent Status
| Agent | Status | Next Action |
|-------|--------|-------------|
| **Dev-A** | 🔄 3 PRs ready | Submit PRs for all 3 ready branches |
| **Dev-B** | 🔄 TASK-307 assigned | Submit TASK-001-EXT PR, continue audit |
| **Dev-C** | 🔄 1 PR ready | Submit PHI-119 PR, then IDLE |

### Ready for PR Submission (5 total)
1. **TASK-002** (Dev-A) — Button standardization → `feat/TASK-002-button-standardization`
2. **PHI-119** (Dev-C) — Compact output → `feat/PHI-119-compact-output`
3. **TASK-094-C3** (Dev-A) — DI wiring → `feat/TASK-094-C3`
4. **TASK-094-D** (Dev-A) — HandlerDeps struct → `feat/TASK-094-D`
5. **TASK-001-EXT** (Dev-B) — Onboarding role selector → `feat/TASK-001-EXT-onboarding-role-selector`

### Already Merged (Previously Misfiled)
| Task | Assignee | Commit | Status |
|------|----------|--------|--------|
| TASK-141 | Dev-C | de4901e | ✅ In main |
| TASK-142 | Dev-C | fbc3846 | ✅ In main |
| TASK-143 | Dev-C | 98290a0 | ✅ In main |
| TASK-147 | Dev-C | 4d7d54b | ✅ In main |
| TASK-306 | Dev-A | 1144f17 | ✅ In main |

---

## Next Sprint (P2) — DI Framework Completion

**Progressing once QA clears 5-PR backlog.**

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
| 2026-04-03 | QA Bottleneck (10 PRs) | ✅ **RESOLVED** | Corrected to 5 PRs ready + 5 already merged |
| 2026-04-03 | Dev-C 4 tasks incomplete | ✅ **RESOLVED** | Tasks 141-147 already merged to main |
| 2026-04-03 | TASK-306 empty | ✅ **RESOLVED** | Already merged (1144f17), filing error |

---

## Decision Log

| Date | Decision | Context |
|------|----------|---------|
| 2026-04-02 | DI Framework: Manual restructuring (Option C) | ADR-012 evaluation complete |
| 2026-04-03 | No new DI framework dependencies | wire/fx overhead not justified |
| 2026-04-03 | Dev-C workload clarification | 4 tasks already merged; filing error discovered |
| 2026-04-03 | TASK-307 reassigned to Dev-B | Dev-C work complete |

---

## Blockers

**No critical blockers.** QA backlog of 5 PRs is manageable.

---

*Direction maintained by: TechLead-Intel*  
*✅ All escalations resolved. Sprint progressing normally.*
