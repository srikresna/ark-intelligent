# TASK-189: AMT Modules 3-5 — Rotation Factor, Close Location, MGI

**Priority:** medium
**Type:** feature
**Estimated:** L
**Area:** internal/service/ta/

## Deskripsi

Complete AMT implementation dengan Module 3 (Rotation Factor), Module 4 (Close Location), dan Module 5 (Multi-Day Migration + MGI). Depends on TASK-185 (Day Type) dan Volume Profile infrastructure.

## Module 3: Rotation Factor (~15 min code)

- Count half-rotations between VA extremes
- RF > 4 = balanced market (fade extremes)
- RF < 2 = directional market (follow momentum)
- Time ratio: in VA vs outside VA

## Module 4: Close Location (~15 min code)

- Track close relative to VP levels: in VA, above VAH, below VAL, at POC
- Historical follow-through rate (e.g., "close above VAH → 80% bullish continuation next day")
- Last 5 days close history

## Module 5: Multi-Day Migration + MGI (~40 min code)

- Value Migration Map: POC position across N days
- Developing Value Area: overlaid VA boundaries
- MGI: Acceptance/rejection at key levels (open inside VA = accepted, rejected = reversal)
- Composite VA context (weekly + monthly)

## File Changes

- `internal/service/ta/amt_rotation.go` — NEW: Rotation factor engine
- `internal/service/ta/amt_close.go` — NEW: Close location tracker
- `internal/service/ta/amt_migration.go` — NEW: Multi-day migration + MGI
- `internal/service/ta/amt_models.go` — Add rotation, close, migration types
- `internal/adapter/telegram/formatter_amt.go` — Add sections for modules 3-5

## Acceptance Criteria

- [ ] Rotation factor computed from intraday VA crossings
- [ ] Close location classified + follow-through % computed
- [ ] Multi-day POC migration visualized (text-based chart)
- [ ] MGI scoring: acceptance/rejection at key levels
- [ ] /auction full shows complete AMT report (all 5 modules)
- [ ] Depends on TASK-185 completion
