# PHI-SEC-001: Fix Keyring Panic in Production Code

## Problem Statement
`internal/service/marketdata/keyring/keyring.go:40` calls `panic(err)` in the `MustNext()` function. This causes the entire application to crash if API keys are not configured, rather than gracefully handling the error.

## Current Code
```go
// MustNext returns the next key. Panics if no keys configured.
func (k *Keyring) MustNext() string {
    key, err := k.Next()
    if err != nil {
        panic(err)  // ← CRITICAL: Crashes process
    }
    return key
}
```

## Expected Behavior
- Return error instead of panic
- Allow callers to implement retry/fallback logic
- Maintain backward compatibility with new naming

## Acceptance Criteria
- [ ] Remove `panic(err)` from `MustNext()` or rename to `MustNextOrError()`
- [ ] Update all callers to handle error gracefully
- [ ] Add retry logic for transient key exhaustion
- [ ] Log warning when keyring is empty
- [ ] Update tests to verify graceful handling
- [ ] Verify no process crash when keys are exhausted

## Files to Modify
- `internal/service/marketdata/keyring/keyring.go`
- All callers of `MustNext()` (search for usage)

## Risk Assessment
**Impact**: HIGH — Production crash  
**Effort**: LOW — 2-4 hours  
**Priority**: P0 (Critical)

## Related
- CRITICAL-002 in research audit
