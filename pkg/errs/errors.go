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
	// ---------------------------------------------------------------------------
	// Data availability
	// ---------------------------------------------------------------------------

	// ErrNoData indicates the upstream source returned successfully but
	// contained no usable data (empty result set, no matching records, etc.).
	ErrNoData = errors.New("no data available")

	// ErrInsufficientData indicates not enough data points are available
	// to perform the requested analysis (e.g., too few bars for an indicator).
	ErrInsufficientData = errors.New("insufficient data for analysis")

	// ErrStaleData indicates the data exists but is outdated beyond an
	// acceptable threshold.
	ErrStaleData = errors.New("data is stale")

	// ErrCacheMiss indicates the requested entry was not found in the cache.
	ErrCacheMiss = errors.New("cache miss")

	// ---------------------------------------------------------------------------
	// API / Network
	// ---------------------------------------------------------------------------

	// ErrRateLimited indicates the request was rejected due to rate limiting
	// (HTTP 429 or provider-specific throttle response).
	ErrRateLimited = errors.New("rate limited")

	// ErrTimeout indicates the operation exceeded its deadline or context
	// was cancelled due to timeout.
	ErrTimeout = errors.New("timeout")

	// ErrAPIUnavailable indicates the API service is temporarily or permanently
	// unavailable (connection refused, DNS failure, HTTP 5xx).
	ErrAPIUnavailable = errors.New("API service unavailable")

	// ErrUpstream indicates a generic upstream failure (non-200 status,
	// connection error, etc.) that doesn't fit a more specific sentinel.
	ErrUpstream = errors.New("upstream error")

	// ErrNotFound indicates the requested resource does not exist
	// (HTTP 404 or missing record/contract/symbol).
	ErrNotFound = errors.New("not found")

	// ---------------------------------------------------------------------------
	// Auth / Configuration
	// ---------------------------------------------------------------------------

	// ErrNoAPIKey indicates the required API key or credential is not configured.
	ErrNoAPIKey = errors.New("API key not configured")

	// ErrFeatureDisabled indicates the requested feature is disabled in config.
	ErrFeatureDisabled = errors.New("feature disabled")

	// ErrUnauthorized indicates the request was rejected due to authentication
	// or authorization failure (HTTP 401/403).
	ErrUnauthorized = errors.New("unauthorized")

	// ---------------------------------------------------------------------------
	// Parsing / Validation
	// ---------------------------------------------------------------------------

	// ErrParseFailed indicates data parsing failed (malformed JSON, CSV, XML, etc.).
	ErrParseFailed = errors.New("data parsing failed")

	// ErrBadData indicates the upstream returned data that could not be
	// parsed or was structurally invalid (malformed JSON, unexpected schema, etc.).
	ErrBadData = errors.New("bad data")

	// ErrInvalidFormat indicates the input or output did not match the
	// expected schema or format (e.g. wrong column count in CSV).
	ErrInvalidFormat = errors.New("invalid data format")

	// ---------------------------------------------------------------------------
	// Computation
	// ---------------------------------------------------------------------------

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
