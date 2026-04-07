---
id: TASK-TEST-001
title: Add unit tests for internal/scheduler/scheduler.go
status: in_review
priority: high
effort: 4h
assigned_to: dev-a (claimed from dev-b - was idle)
created_by: research
created_at: 2026-04-06T05:13:00Z
pr_url: https://github.com/arkcode369/ark-intelligent/pull/361
---

## Summary

Add comprehensive unit tests for `internal/scheduler/scheduler.go`, the core job scheduling and orchestration module.

## Background

The scheduler is **critical infrastructure** (1335 lines) that:
- Runs background jobs for COT fetching, price updates, FRED alerts
- Manages panic recovery for all scheduled jobs
- Handles graceful shutdown coordination
- Implements job timeout and retry logic

**Current state:** Zero test coverage. This is a high-risk blind spot.

## Acceptance Criteria

- [x] Test job registration and scheduling
- [x] Test panic recovery in `runJob()`
- [x] Test graceful shutdown with active jobs
- [x] Test job timeout handling
- [x] Test FRED alert filtering and broadcasting
- [x] Test scheduler metrics/logging
- [x] All validation passes (build, vet, test)

## Test Scenarios

### Job Execution
- Job runs at scheduled interval
- Job function is called with correct parameters
- Multiple jobs can run concurrently

### Error Handling
- Panic in job is recovered and logged
- Job timeout is enforced
- Retry logic works correctly

### Lifecycle
- Scheduler starts and stops cleanly
- In-flight jobs complete during shutdown (with timeout)
- Context cancellation propagates to jobs

### Alert Broadcasting
- Alerts are filtered by user preferences
- Ban check prevents banned users from receiving alerts
- Rate limiting (sleep) between sends works

## Implementation Notes

The scheduler has these key dependencies that need mocking:
- `Deps` struct (COTAnalyzer, Bot, PrefsRepo, etc.)
- `time.Ticker` (may need to abstract for testing)
- `context.Context` for cancellation

Consider creating a test helper to mock the ticker for deterministic scheduling tests.

## Files to Create

- `internal/scheduler/scheduler_test.go`

## Related

- `internal/scheduler/scheduler.go` (the file to test)
- TASK-TEST-002 (news scheduler tests)

## Risk Assessment

**High** - Without tests, scheduler bugs could:
- Cause goroutine leaks
- Fail to recover from panics
- Not shut down cleanly (data loss)
- Broadcast alerts to wrong users
