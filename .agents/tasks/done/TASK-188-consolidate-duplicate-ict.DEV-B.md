# TASK-188: Consolidate Duplicate ICT Implementations

**Priority:** medium
**Type:** refactor
**Estimated:** M
**Area:** internal/service/ict/, internal/service/ta/

## Deskripsi

Dua implementasi ICT yang overlap dan confusing:
1. `internal/service/ict/` — 6 files, own engine, types (FVGZone)
2. `internal/service/ta/ict.go` — 585 LOC, different types (FVG)

Consolidate ke single authoritative source. Recommend keeping `ta/ict.go` karena sudah integrated ke ta/engine.go.

## Strategy

Option A (recommended): Keep `ta/ict.go` as primary. Make `service/ict/engine.go` wrapper that calls ta/ict CalcICT() and converts types.

Option B: Merge all logic into `service/ict/`, update ta/engine.go to import.

## File Changes

- `internal/service/ict/engine.go` — Refactor to wrapper calling ta/ict.CalcICT()
- `internal/service/ict/types.go` — Align type names with ta/ict (FVG not FVGZone)
- `internal/adapter/telegram/handler_ict.go` — Update to use unified types
- `internal/adapter/telegram/formatter_ict.go` — Update formatting for unified types

## Acceptance Criteria

- [ ] Single ICT calculation path (no duplicate algorithm)
- [ ] Unified type names across codebase
- [ ] handler_ict.go still works correctly
- [ ] ta/engine.go still computes ICT via CalcICT()
- [ ] All existing ICT tests pass
- [ ] go build ./... clean
- [ ] No behavior change in /ict command output
