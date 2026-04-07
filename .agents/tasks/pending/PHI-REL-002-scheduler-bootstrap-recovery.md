# PHI-REL-002: Add Panic Recovery to Scheduler Impact Bootstrap

## Problem Statement

`internal/scheduler/scheduler.go:186-204` spawns a goroutine for impact bootstrapping without panic recovery:

```go
// One-time impact bootstrap (backfills historical event impacts on startup)
if s.deps.ImpactBootstrapper != nil {
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        // Delay to let price data load first.
        select {
        case <-time.After(2 * time.Minute):
        case <-ctx.Done():
            return
        case <-s.stopCh:
            return
        }
        bootstrapCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
        defer cancel()
        created, err := s.deps.ImpactBootstrapper.Bootstrap(bootstrapCtx)
        if err != nil {
            log.Error().Err(err).Msg("impact bootstrap failed")
        } else if created > 0 {
            log.Info().Int("created", created).Msg("impact bootstrap completed")
        }
    }()
}
```

If `ImpactBootstrapper.Bootstrap()` panics (nil pointer, DB error, unexpected condition), the unrecovered panic will crash the entire process.

## Expected Behavior

- Goroutine should have defer/recover protection
- Panics in bootstrap should not crash the bot
- Log the panic for observability
- Ensure `wg.Done()` is still called even if panic occurs

## Acceptance Criteria

- [ ] Add `defer func() { if r := recover(); r != nil { ... } }()` at the start of the goroutine
- [ ] Log recovered panic with `log.Error().Interface("panic", r).Msg("impact bootstrap panic recovered")`
- [ ] Ensure `defer s.wg.Done()` still executes (place before or wrap both in a function)
- [ ] Verify bootstrap failure does not prevent other scheduler jobs from running
- [ ] Optional: Add metric counter `scheduler_bootstrap_panics_recovered`

## Files to Modify

- `internal/scheduler/scheduler.go` — lines 186-204

## Implementation Pattern

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).Msg("impact bootstrap panic recovered")
        }
    }()
    defer s.wg.Done()
    // ... rest of bootstrap logic
}()
```

## Risk Assessment

**Impact**: MEDIUM — Startup goroutine can crash process  
**Effort**: VERY LOW — 10 minute fix  
**Priority**: P2 (Medium)

## Related

- Follows same pattern as `runJob()` which already has panic recovery (lines 286-290)
- New finding from Research Agent scheduled audit (Run 9)
