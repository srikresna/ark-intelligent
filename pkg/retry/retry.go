// Package retry provides a generic retry-with-backoff utility for HTTP
// and other I/O operations. Designed for market-data API clients that
// currently fail immediately on transient network errors.
//
// Usage:
//
//	body, err := retry.Do(ctx, func() ([]byte, error) {
//	    return fetchFromAPI(ctx, url)
//	})
package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Option configures retry behaviour.
type Option func(*config)

type config struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	jitter      time.Duration
}

func defaults() config {
	return config{
		maxAttempts: 3,
		baseDelay:   1 * time.Second,
		maxDelay:    30 * time.Second,
		jitter:      500 * time.Millisecond,
	}
}

// WithMaxAttempts overrides the default 3 retry attempts.
func WithMaxAttempts(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxAttempts = n
		}
	}
}

// WithBaseDelay overrides the default 1s base delay.
func WithBaseDelay(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.baseDelay = d
		}
	}
}

// WithMaxDelay overrides the default 30s maximum delay.
func WithMaxDelay(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.maxDelay = d
		}
	}
}

// Do executes fn with exponential backoff and jitter.
// It retries on transient/retryable errors and returns immediately on
// non-retryable ones (4xx except 429).
// Context cancellation stops retries immediately.
func Do[T any](ctx context.Context, fn func() (T, error), opts ...Option) (T, error) {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}

	var lastErr error
	var zero T

	for attempt := 0; attempt < cfg.maxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		// Non-retryable errors: return immediately.
		if !isRetryable(err) {
			return zero, err
		}

		lastErr = err

		// Don't sleep after the last attempt.
		if attempt == cfg.maxAttempts-1 {
			break
		}

		delay := backoff(attempt, cfg)
		log.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max", cfg.maxAttempts).
			Dur("backoff", delay).
			Msg("retryable error, backing off")

		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			// continue to next attempt
		}
	}

	return zero, fmt.Errorf("all %d attempts failed: %w", cfg.maxAttempts, lastErr)
}

// backoff computes exponential delay with random jitter.
func backoff(attempt int, cfg config) time.Duration {
	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(cfg.baseDelay) * exp)
	if delay > cfg.maxDelay {
		delay = cfg.maxDelay
	}
	// Add random jitter [0, cfg.jitter).
	if cfg.jitter > 0 {
		delay += time.Duration(rand.Int63n(int64(cfg.jitter)))
	}
	return delay
}

// isRetryable returns true for transient errors that warrant a retry.
// Non-retryable: 400, 401, 403, 404 (client errors except 429).
// Retryable: network errors, 429 (rate limit), 500-503 (server errors).
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Context cancelled or deadline exceeded — don't retry.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Network-level errors are always retryable.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	msg := err.Error()

	// Rate limit (429) is retryable.
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") {
		return true
	}

	// Server errors (5xx) are retryable.
	for _, code := range []string{"500", "502", "503", "504"} {
		if strings.Contains(msg, "HTTP "+code) {
			return true
		}
	}

	// Client errors (400, 401, 403, 404) are NOT retryable.
	for _, code := range []string{"400", "401", "403", "404"} {
		if strings.Contains(msg, "HTTP "+code) {
			return false
		}
	}

	// Connection-related error strings.
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"EOF",
		"broken pipe",
		"timeout",
		"temporary failure",
		"no such host",
		"TLS handshake",
	}
	for _, pat := range retryablePatterns {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(pat)) {
			return true
		}
	}

	// Default: not retryable (fail fast for unknown errors).
	return false
}
