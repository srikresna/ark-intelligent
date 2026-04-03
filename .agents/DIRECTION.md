# DIRECTION — Ark Intelligent Sprint Priorities

**Last Updated:** 2026-04-03 WIB  
**Sprint:** UX Improvement Siklus 1 (Closing) → DI Refactoring Siklus 2  
**Status:** QA Merge Phase + Parallel Development

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

### 2. Active Development (Parallel while QA reviews)
Dev agents assigned new work to maintain velocity:

| Task | Assignee | Status | Est. |
|------|----------|--------|------|
| TASK-001-EXT | Dev-B | 🔄 In Progress | M (4-6h) |
| TASK-307 | Dev-C | 🔄 In Progress | S (2-3h) |
| TASK-094-D | Dev-A | 🔄 Prep/Design | S (1h) |

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

---

## Blockers

**None currently.** QA reviewing 4 PRs; dev agents have parallel assignments.

---

*Direction maintained by: TechLead-Intel*
