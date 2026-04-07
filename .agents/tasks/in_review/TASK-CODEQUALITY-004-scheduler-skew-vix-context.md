# TASK-CODEQUALITY-004: Use parent context in scheduler_skew_vix broadcast calls

**Priority:** Low  
**Estimated Effort:** 30 minutes  
**Status:** Pending

## Issue Description

The `checkSKEWVIXAlert` function in `scheduler_skew_vix.go` accepts a `parentCtx` parameter but uses `context.Background()` when broadcasting alerts (lines 56 and 74). This loses cancellation propagation from the parent.

## Location

- File: `internal/scheduler/scheduler_skew_vix.go`
- Lines: 56, 74
- Current code:
```go
// Line 56
s.broadcastToActiveUsers(context.Background(), msg)
// Line 74  
s.broadcastToActiveUsers(context.Background(), msg)
```

## Expected Behavior

Use the parent context (or a child context with timeout) to allow proper cancellation propagation:
```go
// Option 1: Use parent context directly
s.broadcastToActiveUsers(parentCtx, msg)

// Option 2: Use child context with timeout for broadcasts
broadcastCtx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
defer cancel()
s.broadcastToActiveUsers(broadcastCtx, msg)
```

## Acceptance Criteria (Dev MUST validate before PR)

- [ ] Replace `context.Background()` with `parentCtx` (or child with timeout)
- [ ] Verify the function receives `parentCtx` which is already available
- [ ] Test that scheduler shutdown properly cancels in-flight broadcasts
- [ ] **VALIDATION: `go build ./...` passes**
- [ ] **VALIDATION: `go vet ./...` passes**
- [ ] **VALIDATION: No new test failures**

## Context

The function already receives `parentCtx` as a parameter but doesn't use it. Using `context.Background()` means broadcast operations won't be cancelled during graceful shutdown, potentially delaying process termination.

## Related

- Pattern in same file at line 20 correctly uses `context.WithTimeout(context.Background(), 30*time.Second)` for the VIX fetch
