// Package telegram provides context utilities for Telegram bot handlers.
// These utilities help with timeout enforcement, request tracing, and
// cancellation propagation across the handler chain.
package telegram

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// contextKey is a private type to avoid key collisions in context values.
type contextKey int

const (
	// requestIDKey stores the unique request identifier in context.
	requestIDKey contextKey = iota
)

// ---------------------------------------------------------------------------
// Request ID Tracing
// ---------------------------------------------------------------------------

// withRequestID attaches a unique request ID to the context.
// This ID can be used for tracing requests through logs and metrics.
func withRequestID(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestIDKey, uuid.New().String())
}

// requestID retrieves the request ID from context, or returns empty string if not present.
func requestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// ---------------------------------------------------------------------------
// Timeout Helpers
// ---------------------------------------------------------------------------

// HandlerTimeout defines standard timeout durations for different handler types.
// Use these constants to ensure consistent timeout behavior across handlers.
const (
	// DefaultHandlerTimeout for most commands (database queries, simple API calls).
	DefaultHandlerTimeout = 30 * time.Second

	// SlowHandlerTimeout for handlers that may process larger datasets
	// or perform multiple sequential operations (e.g., COT analysis, backtesting).
	SlowHandlerTimeout = 60 * time.Second

	// ChartHandlerTimeout for chart generation handlers that run Python scripts.
	// These can take longer due to external process execution.
	ChartHandlerTimeout = 90 * time.Second

	// AIHandlerTimeout for AI-powered handlers (Gemini/Claude calls).
	// AI generation can be slow depending on model and prompt complexity.
	AIHandlerTimeout = 120 * time.Second

	// ExternalAPITimeout for calls to external financial data APIs.
	// These often have their own rate limiting and may be slow.
	ExternalAPITimeout = 45 * time.Second
)

// withTimeout wraps the parent context with a timeout, using the provided duration.
// It returns the derived context and a cancel function that should be deferred.
//
// Example:
//
//	ctx, cancel := withTimeout(ctx, DefaultHandlerTimeout)
//	defer cancel()
//	result, err := h.service.Call(ctx, params)
func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// withStandardTimeout applies DefaultHandlerTimeout to the context.
// Use this for most handlers that don't have special timeout requirements.
func withStandardTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return withTimeout(ctx, DefaultHandlerTimeout)
}

// ---------------------------------------------------------------------------
// Cancellation Helpers
// ---------------------------------------------------------------------------

// isCancelled checks if the context has been cancelled or timed out.
// Use this to early-exit from long-running operations.
func isCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// checkContext returns ctx.Err() if the context is done, nil otherwise.
// This is useful for checking cancellation at operation boundaries.
func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Complete Handler Context Setup
// ---------------------------------------------------------------------------

// setupHandlerContext prepares a complete handler context with:
// - Request ID for tracing
// - Timeout enforcement
// - Cancellation propagation
//
// Returns the configured context and a cancel function that must be deferred.
//
// Example:
//
//	func (h *Handler) cmdExample(ctx context.Context, chatID string, userID int64, args string) error {
//	    ctx, cancel := setupHandlerContext(ctx, DefaultHandlerTimeout)
//	    defer cancel()
//	    // ... handler logic
//	}
func setupHandlerContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx = withRequestID(ctx)
	return withTimeout(ctx, timeout)
}
