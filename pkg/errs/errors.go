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
		errors.Is(err, ErrUpstream)
}
