package config

import "time"

// ---------------------------------------------------------------------------
// Signal Detection Thresholds
// ---------------------------------------------------------------------------

const (
	// SignalStrengthAlert is the minimum signal strength for scheduler alerts.
	SignalStrengthAlert = 4

	// ZScoreExtreme is the z-score threshold for extreme positioning alerts.
	ZScoreExtreme = 2.0
)

// ---------------------------------------------------------------------------
// Cache & State TTLs
// ---------------------------------------------------------------------------

const (
	// AlphaStateTTL is the time-to-live for per-chat alpha engine state.
	AlphaStateTTL = 60 * time.Second

	// CTAStateTTL is the time-to-live for per-chat CTA analysis state.
	CTAStateTTL = 120 * time.Second

	// QuantStateTTL is the time-to-live for per-chat quant analysis state.
	QuantStateTTL = 30 * time.Minute

	// VPStateTTL is the time-to-live for per-chat volume profile state.
	VPStateTTL = 30 * time.Minute
)

// ---------------------------------------------------------------------------
// Rate Limiting & Cooldowns
// ---------------------------------------------------------------------------

const (
	// AICooldownDefault is the default cooldown between AI requests.
	AICooldownDefault = 30 * time.Second

	// RateLimitWindow is the sliding window for command rate limiting.
	RateLimitWindow = 60 * time.Second

	// RateLimitMax is the maximum commands per rate limit window.
	RateLimitMax = 10

	// StaleEntryTTL is the cleanup threshold for idle rate-limit entries.
	StaleEntryTTL = 5 * time.Minute

	// CleanupInterval is how often the rate-limit cleanup goroutine runs.
	CleanupInterval = 2 * time.Minute
)

// ---------------------------------------------------------------------------
// Telegram API
// ---------------------------------------------------------------------------

const (
	// LongPollTimeout is the timeout for Telegram getUpdates long-polling.
	LongPollTimeout = 60 * time.Second

	// PollRetryDelay is the delay before retrying after a poll error.
	PollRetryDelay = 5 * time.Second
)

// ---------------------------------------------------------------------------
// Telegram Flood Control
// ---------------------------------------------------------------------------

const (
	// TelegramFloodDelay is the minimum pause between consecutive Telegram
	// API calls to avoid hitting flood-control limits.
	TelegramFloodDelay = 50 * time.Millisecond

	// TelegramRateLimitDelay is the minimum gap between consecutive sends
	// to the same chat (~28 msg/sec, under Telegram's 30 limit).
	TelegramRateLimitDelay = 35 * time.Millisecond

	// TelegramMaxMessageLen is Telegram's maximum message length in characters.
	TelegramMaxMessageLen = 4096
)

// ---------------------------------------------------------------------------
// External API Rate Limits
// ---------------------------------------------------------------------------

const (
	// PriceFetchDelay is the pause between consecutive price API calls
	// to avoid hitting provider rate limits.
	PriceFetchDelay = 300 * time.Millisecond

	// COTFetchDelay is the pause between consecutive COT data API calls.
	COTFetchDelay = 200 * time.Millisecond
)

// ---------------------------------------------------------------------------
// AI Model Defaults
// ---------------------------------------------------------------------------

const (
	// AIDefaultMaxTokens is the default maximum output tokens for AI models.
	AIDefaultMaxTokens = 4096
)

// ---------------------------------------------------------------------------
// Microstructure Thresholds
// ---------------------------------------------------------------------------

const (
	// MicroConfirmEntryThreshold is the minimum signal strength for
	// confirming an entry in microstructure analysis.
	MicroConfirmEntryThreshold = 0.50
)
