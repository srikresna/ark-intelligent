# DIRECTION — Ark Intelligent Sprint Priorities

**Last Updated:** 2026-04-03 WIB  
**Sprint:** UX Improvement Siklus 1 (Closing) → DI Refactoring Siklus 2  
**Status:** QA Merge Phase + Parallel Development + Escalation Active

---

## Current Priorities (P0)

### 1. QA Review & Merge (Next 4 hours)
The 4 open PRs from UX Siklus 1 need QA review and merge:

| PR | Author | Branch | Task | Status |
|----|--------|--------|------|--------|
| PHI-118 | Dev-A | `feat/TASK-002-button-standardization` | Button standardization | Ready |
| PHI-119 | Dev-C | `feat/PHI-119-compact-output` | Compact output mode | Ready |
| PHI-115-C3 | Dev-A | `feat/TASK-094-C3` | DI wire telegram | Ready |
| TASK-306 | Dev-A | `feat/TASK-306-httpclient-migration-extended` | httpclient migration | Ready |

**Goal:** All 4 PRs merged by end of day.

### 2. Active Development
| Task | Assignee | Status | Est. |
|------|----------|--------|------|
| TASK-001-EXT | Dev-B | ✅ COMPLETE | M (4-6h) |
| TASK-094-D | Dev-A | ✅ COMPLETE | S (1h) |
| TASK-307 | Dev-C | ⚠️ NOT STARTED | S (2-3h) |

---

## ⚠️ Escalation Update (Loop #32)

**Dev-C Inactivity on TASK-307 — RESOLVED by Reassignment**
- **Original Issue:** Dev-C assigned TASK-307 in loop #28, no progress after 5+ hours
- **Resolution:** ✅ **REASSIGNED to Dev-B** (available after completing TASK-001-EXT)
- **Status:** CTO notified for Dev-C follow-up
- **Files:** 
  - `.agents/escalations/2026-04-03-DEV-C-inactivity-TASK-307.md` (updated)
  - `.agents/tasks/claimed/TASK-307-audit-httpclient-usages.DEV-B.md` (reassigned)

**For CTO:** Review Dev-C availability and workload.

---

## Next Sprint (P1) — DI Framework Completion

Per ADR-012 (DI Framework Evaluation), finish the manual wiring restructure:

| Task | Assignee | Est. | Dependency |
|------|----------|------|------------|
| TASK-094-D: HandlerDeps struct | Dev-A | S | After C3 merged |
| TASK-094-Cleanup: main.go <200 LOC | Dev-A | S | After D |
| TASK-094-Docs: Update TECH_REFACTOR_PLAN.md | TechLead | XS | After cleanup |

---

## Backlog (P2)

### Technical Debt
- **TASK-308:** Connection pool metrics export ( observability )
- **TASK-309:** BadgerDB compaction schedule optimization

### Features (from research backlog)
- **TASK-006:** Help command search/filter functionality
- **TASK-011:** Multi-language support (ID/EN) for responses

---

## Decision Log

| Date | Decision | Context |
|------|----------|---------|
| 2026-04-02 | DI Framework: Manual restructuring (Option C) | ADR-012 evaluation complete |
| 2026-04-03 | No new DI framework dependencies | wire/fx overhead not justified |
| 2026-04-03 | Prioritize PR merge over new work | Close UX Siklus 1 before starting Siklus 2 |
| 2026-04-03 | Parallel development during QA bottleneck | Assigned TASK-001-EXT, TASK-307 to maintain velocity |
| 2026-04-03 | Escalation: Dev-C inactivity on TASK-307 | Task not started after 4 hours, Dev-A and Dev-B completed their work |

---

## Blockers

**⚠️ Dev-C Inactivity:** TASK-307 not started after 4 hours of assignment. Escalation active.

---

*Direction maintained by: TechLead-Intel*
