---
id: TASK-CODEQUALITY-006
title: Add context timeout to impact_recorder.go delayedRecord goroutine
status: in_progress
priority: medium
effort: 30m
assigned_to: dev-a
created_by: research
created_at: 2026-04-06T05:13:00Z
claimed_at: 2026-04-06T06:05:00Z
claimed_by: Dev-A
---

## Summary

Add timeout context to the `delayedRecord` goroutine in `impact_recorder.go` to prevent goroutine leaks when the price API hangs.

## Background

This issue is similar to TASK-CODEQUALITY-003 (chat_service.go). The `impact_recorder.go` file spawns a goroutine for delayed impact recording that uses `context.Background()` without any timeout:

```go
// Line 108 in internal/service/news/impact_recorder.go
go r.delayedRecord(context.Background(), ev, mapping.ContractCode, beforePrice, surpriseSigma, sigmaBucket, horizon, duration)
```

While `delayedRecord()` does have panic recovery, it does not have a context timeout for the API calls it makes.

## Acceptance Criteria

- [ ] Wrap `context.Background()` with `context.WithTimeout(context.Background(), 5*time.Minute)` before passing to `delayedRecord`
- [ ] Ensure the cancel function is called (defer or explicit)
- [ ] Verify no other goroutines in the file have the same issue

## Implementation Notes

The fix should look like:

```go
recordCtx, recordCancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer recordCancel()
go r.delayedRecord(recordCtx, ev, mapping.ContractCode, beforePrice, surpriseSigma, sigmaBucket, horizon, duration)
```

Note: The `delayedRecord` function already has panic recovery, so this change just adds timeout safety.

## Files to Modify

- `internal/service/news/impact_recorder.go` (line 108)

## Related

- TASK-CODEQUALITY-003 (similar issue in chat_service.go)
- TASK-CODEQUALITY-004 (scheduler_skew_vix.go context issue)

## Testing

- [ ] Verify goroutine doesn't leak when price API is mocked to hang
- [ ] Verify normal operation still works (context passed through correctly)
