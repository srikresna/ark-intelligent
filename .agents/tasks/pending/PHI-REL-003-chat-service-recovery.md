# PHI-REL-003: Add Panic Recovery to Chat Service notifyOwner

## Problem Statement

`internal/service/ai/chat_service.go:297-302` spawns a fire-and-forget goroutine without panic recovery:

```go
// Non-blocking — fires in a goroutine with a detached context so the
// notification survives even if the request context is cancelled.
func (cs *ChatService) notifyOwner(_ context.Context, html string) {
    if cs.ownerNotify == nil {
        return
    }
    go cs.ownerNotify(context.Background(), html)  // ← No panic recovery
}
```

If `cs.ownerNotify` callback panics (nil pointer in SendHTML, API client issue, formatting error), the unrecovered panic will crash the entire process.

## Expected Behavior

- Goroutine should have defer/recover protection
- Panics in owner notification should not crash the bot
- Log the panic for observability
- Maintain fire-and-forget behavior (don't block)

## Acceptance Criteria

- [ ] Add `defer func() { if r := recover(); r != nil { ... } }()` inside the goroutine
- [ ] Log recovered panic with structured logging: `log.Error().Interface("panic", r).Msg("notifyOwner panic recovered")`
- [ ] Verify no behavioral changes (still fire-and-forget)
- [ ] Optional: Add unit test to verify panic recovery

## Files to Modify

- `internal/service/ai/chat_service.go` — lines 297-302

## Implementation

```go
func (cs *ChatService) notifyOwner(_ context.Context, html string) {
    if cs.ownerNotify == nil {
        return
    }
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Error().Interface("panic", r).Msg("notifyOwner panic recovered")
            }
        }()
        cs.ownerNotify(context.Background(), html)
    }()
}
```

## Risk Assessment

**Impact**: MEDIUM — Non-critical path can crash process  
**Effort**: VERY LOW — 5 minute fix  
**Priority**: P2 (Medium)

## Related

- Follows same pattern as PHI-REL-001 (handler.go notifyOwnerDebug)
- New finding from Research Agent scheduled audit (Run 9)
