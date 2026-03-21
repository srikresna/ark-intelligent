// Package logger provides structured logging via zerolog.
// All components use Component("name") to get a sub-logger with
// the component field pre-set. Output is JSON to stderr by default.
//
// Usage:
//
//	logger.Init("info")
//	log := logger.Component("scheduler")
//	log.Info().Str("job", "cot-fetch").Msg("started")
package logger

import (
	"os"

	"github.com/rs/zerolog"
)

// Log is the root logger instance. Use Component() for sub-loggers.
// Initialized with stderr output immediately so that Component() loggers
// created at package init time (before Init() is called) still produce output.
var Log = zerolog.New(os.Stderr).With().
	Timestamp().
	Caller().
	Logger()

// Init initializes the global log level from a level string.
// Valid levels: "trace", "debug", "info", "warn", "error", "fatal", "panic".
// Defaults to "info" if the level string is invalid.
//
// Note: zerolog.SetGlobalLevel affects ALL loggers (including those created
// before Init), so Component() loggers from package init time will respect
// the level set here.
func Init(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
}

// Component returns a sub-logger tagged with a component name.
// Example: logger.Component("scheduler") -> {"component":"scheduler",...}
func Component(name string) zerolog.Logger {
	return Log.With().Str("component", name).Logger()
}
