# PHI-REL-001: Add Panic Recovery to notifyOwnerDebug Goroutine

## Problem Statement

`internal/adapter/telegram/handler.go:2590-2592` spawns a fire-and-forget goroutine without panic recovery:

```go
func (h *Handler) notifyOwnerDebug(ctx context.Context, html string) {
    ownerID := h.bot.OwnerID()
    if ownerID <= 0 {
        return
    }
    go func() {
        _, _ = h.bot.SendHTML(ctx, fmt.Sprintf("%d", ownerID), html)
    }()  // ← No panic recovery
}
```

If `SendHTML` panics (nil pointer, API client issue, formatting error), the unrecovered panic will crash the entire process.

## Expected Behavior
- Goroutine should have defer/recover protection
- Panics in debug notification should not crash the bot
- Log the panic for observability

## Acceptance Criteria
- [ ] Add `defer func() { if r := recover(); r != nil { ... } }()` inside the goroutine
- [ ] Log recovered panic with `log.Error().Interface("panic", r).Msg("notifyOwnerDebug panic recovered")`
- [ ] Verify no behavioral changes (still fire-and-forget)
- [ ] Optional: Add unit test to verify panic recovery

## Files to Modify
- `internal/adapter/telegram/handler.go` — lines 2590-2592

## Implementation

```go
func (h *Handler) notifyOwnerDebug(ctx context.Context, html string) {
    ownerID := h.bot.OwnerID()
    if ownerID <= 0 {
        return
    }
    go func() {
        defer func() {
            if r := recover(); r != nil {
                log.Error().Interface("panic", r).Msg("notifyOwnerDebug panic recovered")
            }
        }()
        _, _ = h.bot.SendHTML(ctx, fmt.Sprintf("%d", ownerID), html)
    }()
}
```

## Risk Assessment
**Impact**: MEDIUM — Non-critical path can crash process  
**Effort**: VERY LOW — 5 minute fix  
**Priority**: P2 (Medium)

## Related
- Follows same pattern as `bot.go:231-235` (handleUpdate has panic recovery)
