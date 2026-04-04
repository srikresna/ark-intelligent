# PHI-CTX-001: Fix Context Propagation in Handlers

## Problem Statement
Multiple handler files use `context.Background()` instead of propagating the request context. This prevents:
- Request cancellation on client disconnect
- Timeout propagation through call chains
- Distributed tracing correlation
- Resource cleanup on shutdown

## Affected Locations
1. `internal/adapter/telegram/handler_cta.go:581`
   ```go
   func (h *Handler) generateCTAChart(...) ([]byte, error) {
       ctx := context.Background()  // ← Should use request ctx
   ```

2. `internal/adapter/telegram/handler_quant.go:448, 484`
   ```go
   cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)  // ← Background
   
   func (h *Handler) fetchMultiAssetCloses(...) {
       ctx := context.Background()  // ← Should use request ctx
   ```

3. `internal/adapter/telegram/handler_vp.go:422`
   ```go
   cmdCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)  // ← Background
   ```

## Expected Behavior
- All handlers accept `ctx context.Context` as first parameter
- Pass request context through to all downstream calls
- Use `context.WithTimeout()` derived from request context, not Background
- Ensure repository and service calls respect cancellation

## Acceptance Criteria
- [ ] Update `generateCTAChart` to accept context parameter
- [ ] Update `runQuantEngine` to accept context parameter
- [ ] Update `fetchMultiAssetCloses` to accept context parameter
- [ ] Update `generateVPChart` to accept context parameter
- [ ] Update all call sites to pass request context
- [ ] Verify repository methods check `ctx.Err()` before expensive ops
- [ ] Add unit tests for context cancellation
- [ ] Verify graceful shutdown cancels in-flight requests

## Files to Modify
- `internal/adapter/telegram/handler_cta.go`
- `internal/adapter/telegram/handler_quant.go`
- `internal/adapter/telegram/handler_vp.go`
- `internal/adapter/telegram/handler_ctabt.go` (if affected)
- All handler entry points that call these functions

## Implementation Pattern
```go
// Before
func (h *Handler) generateCTAChart(state *ctaState, tf string) ([]byte, error) {
    ctx := context.Background()
}

// After
func (h *Handler) generateCTAChart(ctx context.Context, state *ctaState, tf string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()
}
```

## Risk Assessment
**Impact**: MEDIUM — Resource leaks, poor UX  
**Effort**: MEDIUM — 4-6 hours  
**Priority**: P1 (High)

## Related
- CRITICAL-003 in research audit
