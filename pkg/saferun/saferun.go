// Package saferun provides panic-safe goroutine helpers.
//
// Instead of repeating the defer/recover pattern in every goroutine:
//
//	go func() {
//	    defer func() {
//	        if r := recover(); r != nil {
//	            log.Error().Interface("panic", r).Msg("PANIC in X")
//	        }
//	    }()
//	    doWork()
//	}()
//
// Use:
//
//	saferun.Go(ctx, "X", logger, func() { doWork() })
package saferun

import (
	"context"
	"runtime/debug"

	"github.com/rs/zerolog"
)

// Go launches fn in a new goroutine with panic recovery.
// If fn panics, the panic value and a stack trace are logged via logger.
// The context is available for future extensions (e.g. tracing) but is
// not currently checked for cancellation — the callee should handle that.
func Go(_ context.Context, name string, logger zerolog.Logger, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error().
					Interface("panic", r).
					Str("goroutine", name).
					Str("stack", string(debug.Stack())).
					Msg("PANIC recovered")
			}
		}()
		fn()
	}()
}
