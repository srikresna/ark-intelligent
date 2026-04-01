# TASK-191: Create pkg/errs/ Sentinel Error Package

**Priority:** high
**Type:** refactor
**Estimated:** M
**Area:** pkg/errs/ (new)

## Deskripsi

Create standardized sentinel error package. Currently only 4 sentinel errors exist. Need 15+ covering all common failure modes.

## Sentinel Errors to Define

```go
package errs

import "errors"

// Data availability
var ErrNoData          = errors.New("no data available")
var ErrInsufficientData = errors.New("insufficient data for analysis")
var ErrStaleData       = errors.New("data is stale")

// API/Network
var ErrRateLimited     = errors.New("API rate limited")
var ErrTimeout         = errors.New("request timed out")
var ErrAPIUnavailable  = errors.New("API service unavailable")

// Auth/Config
var ErrNoAPIKey        = errors.New("API key not configured")
var ErrFeatureDisabled = errors.New("feature disabled")
var ErrUnauthorized    = errors.New("unauthorized")

// Parsing
var ErrParseFailed     = errors.New("data parsing failed")
var ErrInvalidFormat   = errors.New("invalid data format")

// Cache
var ErrCacheMiss       = errors.New("cache miss")

// Computation
var ErrDivisionByZero  = errors.New("division by zero")
var ErrNaN             = errors.New("NaN result")
var ErrConvergenceFail = errors.New("model did not converge")

// Wrap helper
func Wrap(err error, context string) error {
    return fmt.Errorf("%s: %w", context, err)
}
```

## File Changes

- `pkg/errs/errors.go` — NEW: Sentinel errors + Wrap helper
- `internal/service/fred/fetcher.go` — Replace 10 silent `_, _` with proper error wrapping
- `internal/service/cot/analyzer.go` — Replace silent error suppression
- `internal/service/marketdata/keyring/keyring.go` — Replace panic with ErrNoAPIKey return

## Acceptance Criteria

- [ ] 15+ sentinel errors defined in pkg/errs/
- [ ] Wrap() helper for context-enriched errors
- [ ] MustNext() panic replaced with graceful error return
- [ ] 13 silenced error locations updated (minimum 5 in this PR)
- [ ] errors.Is() usable for all sentinels
- [ ] No breaking changes in function signatures
