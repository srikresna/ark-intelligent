# TASK-186: AMT Opening Type Analysis (OD, OTD, ORR, OA)

**Priority:** high
**Type:** feature
**Estimated:** M
**Area:** internal/service/ta/

## Deskripsi

Implementasi Module 2 dari docs/AMT_UPGRADE_PLAN.md — Opening Type Analysis. Classify today's opening berdasarkan posisi relative to yesterday's Value Area.

## Opening Types

1. **Open Drive (OD):** Opens outside VA, aggressively moves AWAY. Follow momentum.
2. **Open Test Drive (OTD):** Opens outside VA, tests back into VA, then drives away. Follow after test confirms.
3. **Open Rejection Reverse (ORR):** Opens outside VA, reverses back INTO VA. Fade the move.
4. **Open Auction (OA):** Opens inside VA, auctions within. Wait for breakout direction.

## Detail Teknis

Requires:
- Yesterday's Value Area (VAH, VAL, POC) — from Volume Profile engine
- Today's first 30-60 minutes of price action
- Classification algorithm: compare open location vs VA, then track first 30m direction

## File Changes

- `internal/service/ta/amt_opening.go` — NEW: Opening type classifier (~180 LOC)
- `internal/service/ta/amt_models.go` — Add OpeningType, OpeningClassification types
- `internal/adapter/telegram/formatter_amt.go` — Add opening type section

## Acceptance Criteria

- [ ] Classify today's opening into 4 types
- [ ] Show yesterday's VA levels (VAH, VAL, POC)
- [ ] Show open position relative to VA
- [ ] Trading implication per opening type
- [ ] Historical win rate per opening type (last 20 days)
- [ ] Available 30 minutes after market open
- [ ] Unit tests for each opening type
