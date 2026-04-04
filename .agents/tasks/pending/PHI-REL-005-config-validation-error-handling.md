# PHI-REL-005: Replace log.Fatal in Config Validation with Error Returns

## Problem

The `internal/config/config.go` file contains 4 `log.Fatal()` calls in the `validate()` function and `mustGetEnv()` helper. These calls terminate the process immediately without giving the main package a chance to handle errors gracefully or provide user-friendly error messages.

### Current Code Locations

1. **Line 237:** `log.Fatal().Msg("COT_HISTORY_WEEKS must be >= 4")`
2. **Line 240:** `log.Fatal().Msg("COT_FETCH_INTERVAL must be >= 1m")`
3. **Line 243:** `log.Fatal().Msg("CONFLUENCE_CALC_INTERVAL must be >= 1m")`
4. **Line 254:** `log.Fatal().Str("key", key).Msg("Required env var is not set")`

## Impact

- **Poor UX:** Users see cryptic log output instead of helpful error messages
- **No graceful degradation:** Cannot implement fallback configurations
- **Testing difficulty:** Cannot test invalid config scenarios (process exits)
- **Container orchestration:** Pod restarts without clear failure reason

## Proposed Solution

1. Change `validate()` to return `error` instead of calling `log.Fatal()`
2. Change `mustGetEnv()` to return `(string, error)` or use sentinel errors
3. Propagate errors up to `Load()` which already returns `(*Config, error)`
4. Handle fatal errors in `cmd/bot/main.go` with user-friendly messages

### Acceptance Criteria

- [ ] `validate()` returns `error` instead of calling `log.Fatal()`
- [ ] `mustGetEnv()` returns `(string, error)` instead of calling `log.Fatal()`
- [ ] `Load()` propagates validation errors properly
- [ ] `cmd/bot/main.go` handles config errors with user-friendly output
- [ ] Unit tests can verify invalid config scenarios without panicking
- [ ] Process exit codes remain non-zero for invalid configs

## Files to Modify

- `internal/config/config.go` - Refactor validation to return errors
- `cmd/bot/main.go` - Add error handling for config.Load()

## Estimation

**Size:** Small (S)  
**Estimated Time:** 1-2 hours

## Dependencies

None. Can be worked on independently.

## Related Issues

- PHI-SETUP-001 (config validation improvements)
