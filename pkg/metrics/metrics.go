// Package metrics provides structured, machine-parseable logging for command
// execution metrics. All output is JSON via zerolog, designed as a stepping
// stone toward TECH-015 (Prometheus). Log fields use a consistent "metric"
// key so they can be easily filtered and aggregated by log analysis tools.
//
// Log format examples:
//
//	{"metric":"command_exec","command":"/cot","user_id":123,"latency_ms":1200,"success":true,...}
//	{"metric":"slow_command","command":"/outlook","latency_ms":7500,...}
//	{"metric":"api_call","service":"fred","endpoint":"/series/observations",...}
package metrics

import (
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("metrics")

// SlowCommandThreshold is the duration above which a command is flagged as slow.
const SlowCommandThreshold = 5 * time.Second

// RecordCommand logs a structured command execution metric.
// Fields emitted: metric="command_exec", command, user_id, latency_ms, success, (err).
// Commands exceeding SlowCommandThreshold also emit a separate Warn-level
// "slow_command" metric entry for easy alerting.
func RecordCommand(command string, userID int64, duration time.Duration, err error) {
	event := log.Info().
		Str("metric", "command_exec").
		Str("command", command).
		Int64("user_id", userID).
		Dur("latency_ms", duration).
		Bool("success", err == nil)

	if err != nil {
		event = event.Err(err)
	}

	event.Msg("command_metrics")

	if duration > SlowCommandThreshold {
		log.Warn().
			Str("metric", "slow_command").
			Str("command", command).
			Int64("user_id", userID).
			Dur("latency_ms", duration).
			Msg("command exceeded 5s threshold")
	}
}

// RecordCallback logs a structured callback execution metric.
// Fields emitted: metric="callback_exec", callback, user_id, latency_ms, success, (err).
func RecordCallback(callback string, userID int64, duration time.Duration, err error) {
	event := log.Info().
		Str("metric", "callback_exec").
		Str("callback", callback).
		Int64("user_id", userID).
		Dur("latency_ms", duration).
		Bool("success", err == nil)

	if err != nil {
		event = event.Err(err)
	}

	event.Msg("callback_metrics")

	if duration > SlowCommandThreshold {
		log.Warn().
			Str("metric", "slow_callback").
			Str("callback", callback).
			Int64("user_id", userID).
			Dur("latency_ms", duration).
			Msg("callback exceeded 5s threshold")
	}
}

// RecordAPICall logs an external API call for rate-limit visibility.
// Fields emitted: metric="api_call", service, endpoint, latency_ms, success, (err).
func RecordAPICall(service, endpoint string, duration time.Duration, err error) {
	event := log.Info().
		Str("metric", "api_call").
		Str("service", service).
		Str("endpoint", endpoint).
		Dur("latency_ms", duration).
		Bool("success", err == nil)

	if err != nil {
		event = event.Err(err)
	}

	event.Msg("external_api_call")
}
