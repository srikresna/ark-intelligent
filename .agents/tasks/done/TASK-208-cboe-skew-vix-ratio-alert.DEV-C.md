# TASK-208: SKEW/VIX Ratio — Tail Risk Alert System - DONE

**Completed by:** Agent Dev-C
**Completed at:** 2026-04-02 04:25 WIB
**Branch:** feat/TASK-208-skew-vix-ratio-alert
**PR:** pending
**Depends on:** TASK-205 (CBOE Volatility Index Suite)

## Changes Made

### New Files
- `internal/service/vix/skew_vix_alert.go` — Historical percentile computation (SKEW/VIX ratio + standalone SKEW), tail risk context generator, alert helpers (ShouldAlert, AlertSummary, FormatAlertDetail, TailRiskContext)
- `internal/scheduler/scheduler_skew_vix.go` — checkSKEWVIXAlert scheduler job + broadcastToActiveUsers helper

### Modified Files
- `internal/service/vix/vol_suite.go` — Added SKEWVIXPercentile + SKEWPercentile fields to VolSuite; added computeHistoricalPercentile call
- `internal/service/fred/alerts.go` — Added AlertSKEWVIXExtreme, AlertSKEWVIXElevated, AlertSKEWVIXNormal alert types
- `internal/service/sentiment/sentiment.go` — Added SKEWVIXPctile, SKEWPctile, TailRiskCtx fields to SentimentData; mapped from VolSuite
- `internal/adapter/telegram/format_macro.go` — Enhanced Vol Suite section with percentile display (P##), SKEW percentile, historical context for tail risk
- `internal/scheduler/scheduler.go` — Added lastTailRisk field; hooked checkSKEWVIXAlert into jobFREDAlerts

## Features
- 3-tier tail risk classification: NORMAL / ELEVATED / EXTREME (from TASK-205)
- Historical percentile for SKEW/VIX ratio computed from full CBOE SKEW + VIX CSV history
- Historical percentile for standalone SKEW level
- Alert broadcast on state transitions (NORMAL→ELEVATED, ELEVATED→EXTREME, EXTREME→NORMAL)
- All-clear notification when tail risk normalizes
- Dedup: only alerts on transitions, not every hourly check
- Formatter shows percentile rank and historical context in /sentiment output

## Acceptance Criteria
- [x] SKEW/VIX ratio computed from TASK-205 data
- [x] 3-tier alert: normal / elevated / extreme
- [x] Historical percentile for current ratio
- [x] Alert broadcast when ratio crosses extreme threshold
- [x] Display in /vix output with historical context
- [x] Depends on TASK-205 completion (branched from TASK-205 branch)
- [x] go build ./... passes
- [x] go vet ./... passes
