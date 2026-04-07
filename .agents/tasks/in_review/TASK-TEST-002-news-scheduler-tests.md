---
id: TASK-TEST-002
title: Add unit tests for internal/service/news/scheduler.go
status: in_review
priority: high
effort: 4h
assigned_to: dev-a
created_by: research
created_at: 2026-04-06T05:13:00Z
---

## Summary

Add comprehensive unit tests for `internal/service/news/scheduler.go`, the news alert scheduling and broadcasting module.

## Background

The news scheduler (1134 lines) is **critical infrastructure** that:
- Polls MQL5 Economic Calendar for events
- Calculates surprise scores (actual vs forecast)
- Broadcasts alerts to active users
- Implements confluence scoring with COT data
- Records event impact for backtesting

**Current state:** Zero test coverage. This is a high-risk blind spot for the alert system.

## Acceptance Criteria

- [ ] Test event polling and parsing
- [ ] Test surprise score calculation
- [ ] Test alert filtering (currency, impact, user tier)
- [ ] Test confluence scoring with COT data
- [ ] Test impact recording for backtesting
- [ ] Test Fed speech RSS integration
- [ ] Test quiet hours and daily alert caps (alert gate)
- [ ] Achieve at least 60% coverage on news/scheduler.go

## Test Scenarios

### Event Processing
- Events are parsed correctly from MQL5
- Surprise score calculated correctly (actual vs forecast)
- Revisions detected and scored properly
- Events are de-duplicated

### Alert Broadcasting
- Alerts sent to correct users based on preferences
- Free tier users only get USD + High impact (filter test)
- Banned users don't receive alerts
- Alert gate (quiet hours) prevents sends
- Daily alert cap enforced per user

### Confluence Scoring
- COT data is available → confluence score added
- COT data unavailable → graceful degradation
- Score calculation is correct

### Impact Recording
- Impact recorded for each time horizon (15m, 30m, 1h, 4h)
- Delayed recording goroutine spawned correctly
- Price lookups work for before/after comparison

## Implementation Notes

Key dependencies to mock:
- `NewsRepository` (event storage)
- `COTRepository` (for confluence)
- `ports.Messenger` (Bot) for sends
- `PriceRepository` (for impact recording)
- `ImpactRepository` (for storing results)

The scheduler has many goroutines - ensure tests handle concurrent operations safely.

## Files to Create

- `internal/service/news/scheduler_test.go`

## Related

- `internal/service/news/scheduler.go` (the file to test)
- TASK-TEST-001 (main scheduler tests)
- TASK-CODEQUALITY-006 (impact_recorder context fix)

## Risk Assessment

**High** - Without tests, news scheduler bugs could:
- Send alerts to wrong users
- Miss critical economic events
- Calculate wrong surprise scores
- Leak goroutines in impact recording
- Fail to respect user preferences/tiers
