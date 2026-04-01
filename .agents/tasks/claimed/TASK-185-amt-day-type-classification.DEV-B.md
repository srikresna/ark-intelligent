# TASK-185: AMT Day Type Classification (Dalton's 6 Types)

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/ta/

## Deskripsi

Implementasi Module 1 dari docs/AMT_UPGRADE_PLAN.md — Day Type Classification. Classify setiap trading day ke salah satu dari 6 Dalton day types berdasarkan Initial Balance (IB) dan range extension.

## Day Types

1. **Normal:** IB = 85%+ of day range (balanced, mean-revert)
2. **Normal Variation:** IB = 70-85% of range (slight extension)
3. **Trend:** IB < 50% of range, one-directional extension
4. **Double Distribution:** Two separate value clusters (event-driven)
5. **P-shape:** High volume concentration in upper half (distribution)
6. **b-shape:** High volume concentration in lower half (accumulation)

## Detail Teknis

Initial Balance = High/Low of first 2 periods (first hour for 30m bars).
Extension = (Day Range - IB Range) / IB Range

## File Changes

- `internal/service/ta/amt_daytype.go` — NEW: Day type classifier (~200 LOC)
- `internal/service/ta/amt_models.go` — NEW: DayType, DayClassification, AMTResult types
- `internal/adapter/telegram/handler.go` — Add /auction command routing
- `internal/adapter/telegram/formatter_amt.go` — NEW: AMT formatting
- `internal/adapter/telegram/keyboard.go` — Add AMT keyboard (day type history toggle)

## Acceptance Criteria

- [ ] Classify today + last 5 trading days into 6 types
- [ ] Compute IB range from first hour of 30m bars
- [ ] Extension ratio calculation
- [ ] Volume distribution analysis for P-shape/b-shape detection
- [ ] /auction daytype shows classification + pattern (e.g., "3 trend days in a row")
- [ ] Unit tests for each day type with sample data
- [ ] Use existing 30m bars from price fetcher
