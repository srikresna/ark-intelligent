// Package domain provides domain-level sentinel errors for consistent error
// handling across the ARK Intelligent application.
//
// These errors should be used at domain boundaries and can be checked with
// errors.Is() to determine specific error conditions.
package domain

import "errors"

// ---------------------------------------------------------------------------
// Common domain errors - use these for consistent error checking
// ---------------------------------------------------------------------------

var (
	// ErrNotFound indicates a requested resource or data was not found.
	// Use this instead of fmt.Errorf("... not found") for consistency.
	ErrNotFound = errors.New("data not found")

	// ErrInvalidInput indicates the provided input parameters are invalid.
	// Wrap this with context using fmt.Errorf("...: %w", ErrInvalidInput).
	ErrInvalidInput = errors.New("invalid input")

	// ErrInsufficientData indicates there isn't enough data to perform an operation.
	// Common in analysis operations that require minimum data points.
	ErrInsufficientData = errors.New("insufficient data")

	// ErrTimeout indicates an operation timed out.
	// Usually wraps context.DeadlineExceeded.
	ErrTimeout = errors.New("operation timed out")

	// ErrRateLimited indicates the operation was rate limited by an external service.
	ErrRateLimited = errors.New("rate limited")

	// ErrUnavailable indicates a service or resource is temporarily unavailable.
	ErrUnavailable = errors.New("service unavailable")
)

// ---------------------------------------------------------------------------
// Storage/Repository errors
// ---------------------------------------------------------------------------

var (
	// ErrStorage indicates a general storage/database error.
	ErrStorage = errors.New("storage error")

	// ErrKeyNotFound indicates a specific key was not found in storage.
	// This is more specific than ErrNotFound for storage operations.
	ErrKeyNotFound = errors.New("key not found")
)

// ---------------------------------------------------------------------------
// External API errors
// ---------------------------------------------------------------------------

var (
	// ErrAPIRequest indicates an external API request failed.
	ErrAPIRequest = errors.New("api request failed")

	// ErrAPITimeout indicates an external API request timed out.
	ErrAPITimeout = errors.New("api request timed out")

	// ErrAPIRateLimit indicates an external API rate limit was hit.
	ErrAPIRateLimit = errors.New("api rate limit exceeded")
)

// ---------------------------------------------------------------------------
// AI/Generation errors
// ---------------------------------------------------------------------------

var (
	// ErrAIGeneration indicates AI content generation failed.
	ErrAIGeneration = errors.New("ai generation failed")

	// ErrAIServiceUnavailable indicates the AI service is not available.
	ErrAIServiceUnavailable = errors.New("ai service unavailable")
)

// ---------------------------------------------------------------------------
// Data parsing errors
// ---------------------------------------------------------------------------

var (
	// ErrParse indicates data parsing failed.
	ErrParse = errors.New("parse error")

	// ErrInvalidFormat indicates the data format is not as expected.
	ErrInvalidFormat = errors.New("invalid data format")
)
