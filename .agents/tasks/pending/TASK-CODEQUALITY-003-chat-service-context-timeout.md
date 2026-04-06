# TASK-CODEQUALITY-003: Fix context.Background() without timeout in chat_service.go

**Priority:** Medium  
**Estimated Effort:** 1 hour  
**Status:** Pending

## Issue Description

The `notifyOwner` function in `chat_service.go` uses `context.Background()` without timeout when firing a goroutine for owner notifications. This could lead to goroutine leaks if the notification callback hangs.

## Location

- File: `internal/service/ai/chat_service.go`
- Line: 312
- Current code:
```go
func (cs *ChatService) notifyOwner(_ context.Context, html string) {
    if cs.ownerNotify == nil {
        return
    }
    go cs.ownerNotify(context.Background(), html)
}
```

## Expected Behavior

Use a context with timeout to prevent goroutine leaks:
```go
func (cs *ChatService) notifyOwner(_ context.Context, html string) {
    if cs.ownerNotify == nil {
        return
    }
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        cs.ownerNotify(ctx, html)
    }()
}
```

## Acceptance Criteria (Dev MUST validate before PR)

- [ ] Add timeout to the background context in `notifyOwner`
- [ ] Use reasonable timeout (suggested: 30 seconds for Telegram API calls)
- [ ] Ensure proper cancel() defer
- [ ] Check for similar patterns in other goroutine-spawning notification code
- [ ] **VALIDATION: `go build ./...` passes**
- [ ] **VALIDATION: `go vet ./...` passes**
- [ ] **VALIDATION: No new test failures**

## Context

This is a fire-and-forget goroutine for owner notifications. While the risk is lower than API calls (owner notifications are infrequent), having a timeout prevents resource leaks if the Telegram API becomes unresponsive.

## Related

- Similar proper pattern in `internal/service/news/scheduler.go:721` which uses `context.WithTimeout(context.Background(), 5*time.Minute)`
