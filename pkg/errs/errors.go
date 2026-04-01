// Package errs provides sentinel errors and wrapping helpers for consistent
// error handling across the ark-intelligent codebase.
//
// Usage:
//
//	return errs.Wrap(errs.ErrRateLimited, "alphavantage")
//	// => "alphavantage: rate limited"
//
//	if errors.Is(err, errs.ErrRateLimited) { ... }
package errs

import (
	"errors"
	"fmt"
)

// Sentinel errors — use errors.Is() to distinguish error categories.
var (
	// ErrNoData indicates the upstream source returned successfully but
	// contained no usable data (empty result set, no matching records, etc.).
	ErrNoData = errors.New("no data available")

	// ErrRateLimited indicates the request was rejected due to rate limiting
	// (HTTP 429 or provider-specific throttle response).
	ErrRateLimited = errors.New("rate limited")

	// ErrNotFound indicates the requested resource does not exist
	// (HTTP 404 or missing record/contract/symbol).
	ErrNotFound = errors.New("not found")

	// ErrTimeout indicates the operation exceeded its deadline or context
	// was cancelled due to timeout.
	ErrTimeout = errors.New("timeout")

	// ErrBadData indicates the upstream returned data that could not be
	// parsed or was structurally invalid (malformed JSON, unexpected schema, etc.).
	ErrBadData = errors.New("bad data")

	// ErrUpstream indicates a generic upstream failure (non-200 status,
	// connection error, etc.) that doesn't fit a more specific sentinel.
	ErrUpstream = errors.New("upstream error")

	// ErrInsufficientData indicates not enough data points to run the analysis
	// (e.g. fewer bars than the lookback period requires).
	ErrInsufficientData = errors.New("insufficient data for analysis")

	// ErrStaleData indicates the data is too old to be useful (exceeds
	// freshness threshold for the operation).
	ErrStaleData = errors.New("data is stale")

	// ErrAPIUnavailable indicates the external API service is down or
	// returning unexpected errors beyond a simple rate limit or auth issue.
	ErrAPIUnavailable = errors.New("API service unavailable")

	// ErrNoAPIKey indicates the required API key is not configured in the
	// application config (empty string or missing env var).
	ErrNoAPIKey = errors.New("API key not configured")

	// ErrFeatureDisabled indicates the feature is intentionally disabled
	// via configuration (e.g. HasClaude() == false).
	ErrFeatureDisabled = errors.New("feature disabled")

	// ErrUnauthorized indicates the API key or credentials were rejected
	// by the upstream service (HTTP 401 or equivalent).
	ErrUnauthorized = errors.New("unauthorized")

	// ErrParseFailed indicates a parsing step failed — e.g. XML/JSON decode
	// succeeded but the resulting structure was semantically invalid.
	ErrParseFailed = errors.New("data parsing failed")

	// ErrInvalidFormat indicates the input or output did not match the
	// expected schema or format (e.g. wrong column count in CSV).
	ErrInvalidFormat = errors.New("invalid data format")

	// ErrCacheMiss indicates the requested key was not found in the cache.
	ErrCacheMiss = errors.New("cache miss")

	// ErrDivisionByZero indicates a computation attempted to divide by zero.
	ErrDivisionByZero = errors.New("division by zero")

	// ErrNaN indicates a computation produced a NaN result that cannot be
	// used downstream (e.g. in chart rendering or JSON serialization).
	ErrNaN = errors.New("NaN result")

	// ErrConvergenceFail indicates an iterative model (GARCH, HMM, etc.)
	// failed to converge within the allowed iterations.
	ErrConvergenceFail = errors.New("model did not converge")
)

// Wrap wraps a sentinel (or any) error with contextual information.
// The returned error satisfies errors.Is for the original sentinel.
//
//	return errs.Wrap(errs.ErrRateLimited, "alphavantage")
//	// error string: "alphavantage: rate limited"
func Wrap(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// Wrapf wraps an error with a formatted context string.
//
//	return errs.Wrapf(errs.ErrUpstream, "socrata status %d", resp.StatusCode)
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// IsRetryable returns true if the error is potentially transient and
// the operation may succeed on retry (rate limiting, timeouts, upstream errors).
func IsRetryable(err error) bool {
	return errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrUpstream) ||
		errors.Is(err, ErrAPIUnavailable)
}

// IsDataError returns true if the error relates to missing, insufficient, or
// stale data — as opposed to a network or auth failure.
func IsDataError(err error) bool {
	return errors.Is(err, ErrNoData) ||
		errors.Is(err, ErrInsufficientData) ||
		errors.Is(err, ErrStaleData) ||
		errors.Is(err, ErrCacheMiss)
}
