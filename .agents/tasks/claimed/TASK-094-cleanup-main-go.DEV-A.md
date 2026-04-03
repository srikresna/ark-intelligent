# TASK-094-Cleanup — Reduce main.go to <200 LOC

## Assignment
- **Assignee:** Dev-A
- **Priority:** P2 (Next Sprint)
- **Estimated:** S (1-2 days)
- **Dependency:** TASK-094-D merged ✅
- **Source:** DIRECTION.md P2 — DI Framework Completion

---

## Objective
Further reduce `main.go` from its current size to under 200 lines of code by completing the DI restructuring work started in TASK-094-C3 and TASK-094-D.

---

## Current State
- **main.go before TASK-094:** ~683 LOC
- **main.go after TASK-094-C3:** ~337 LOC
- **Target:** <200 LOC

---

## Requirements

### Cleanup Tasks
1. **Extract remaining initialization code:**
   - Move any remaining service initialization to wire_services.go
   - Ensure all bot handler registration is in wire_telegram.go
   - Move scheduler setup fully to wire_schedulers.go

2. **Refactor main() function:**
   - Should only contain: config loading, dependency wiring, server start
   - Target: <50 lines for main() function

3. **Verify no regression:**
   - All existing tests must pass
   - Bot must start and respond correctly
   - All commands must function

### Success Criteria
- [ ] main.go <200 LOC (verified by `wc -l cmd/bot/main.go`)
- [ ] All tests pass (`go test ./...`)
- [ ] Bot starts without errors
- [ ] All commands respond correctly
- [ ] Code review approved

---

## Technical Notes

### Current Structure (post-TASK-094-D)
- `wire_telegram.go` — Telegram bot and handler wiring (208 LOC)
- `wire_schedulers.go` — Scheduler initialization (151 LOC)
- `wire_services.go` — Service layer DI (added in PR #346 merge)
- `handler.go` — HandlerDeps struct for handler dependencies

### Remaining Work
- Review main.go for any initialization code that should be extracted
- Ensure clean separation of concerns
- Add any missing wiring functions

---

## References
- ADR-012: DI Framework Evaluation
- TECH_REFACTOR_PLAN.md
- TASK-094-C3: DI wiring extraction
- TASK-094-D: HandlerDeps struct

---

*Created by: TechLead-Intel (Loop #119)*
*Assignment: Dev-A — Begin after PR #350 merge confirmation*
