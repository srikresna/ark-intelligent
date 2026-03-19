// Package config manages application configuration via environment variables.
// All settings have sensible defaults. Required vars fail fast on startup.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("config")

// Config holds all application configuration.
type Config struct {
	// Required
	BotToken string // Telegram bot token
	ChatID   string // Default Telegram chat ID

	// AI (optional — graceful degradation without)
	GeminiAPIKey string // Google Gemini API key
	GeminiModel  string // Model name (default: gemini-3.1-flash-lite-preview)

	// Storage
	DataDir string // BadgerDB data directory

	// COT
	COTFetchInterval time.Duration // How often to fetch COT data
	COTSocrataURL    string        // CFTC Socrata API URL
	COTHistoryWeeks  int           // How many weeks of COT history to maintain

	// Quantitative
	ConfluenceCalcInterval time.Duration // Confluence score recalculation interval

	// AI
	AICacheTTL time.Duration // How long to cache AI responses
	AIMaxRPM   int           // Max requests per minute to Gemini
	AIMaxDaily int           // Max AI calls per day

	// Logging
	LogLevel string // "debug", "info", "warn", "error"
}

// MustLoad loads configuration from environment variables.
// Panics if required variables are missing.
func MustLoad() *Config {
	cfg := &Config{
		// Required (will panic if empty)
		BotToken: mustGetEnv("BOT_TOKEN"),
		ChatID:   mustGetEnv("CHAT_ID"),

		// AI (optional)
		GeminiAPIKey: getEnv("GEMINI_API_KEY", ""),
		GeminiModel:  getEnv("GEMINI_MODEL", "gemini-3.1-flash-lite-preview"),

		// Storage
		DataDir: getEnv("DATA_DIR", "/app/data"),

		// COT
		COTFetchInterval: getDuration("COT_FETCH_INTERVAL", 6*time.Hour),
		COTSocrataURL:    getEnv("COT_SOCRATA_URL", "https://publicreporting.cftc.gov/resource/6dca-aqww.json"),
		COTHistoryWeeks:  getInt("COT_HISTORY_WEEKS", 52),

		// Quantitative
		ConfluenceCalcInterval: getDuration("CONFLUENCE_CALC_INTERVAL", 2*time.Hour),

		// AI
		AICacheTTL: getDuration("AI_CACHE_TTL", 1*time.Hour),
		AIMaxRPM:   getInt("AI_MAX_RPM", 15),
		AIMaxDaily: getInt("AI_MAX_DAILY", 200),

		// Logging
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	cfg.validate()
	return cfg
}

// HasGemini returns true if Gemini API key is configured.
func (c *Config) HasGemini() bool {
	return c.GeminiAPIKey != ""
}

// validate performs additional validation beyond required env vars.
func (c *Config) validate() {
	if c.COTHistoryWeeks < 4 {
		log.Fatal().Msg("COT_HISTORY_WEEKS must be >= 4")
	}
	if c.COTFetchInterval < 1*time.Minute {
		log.Fatal().Msg("COT_FETCH_INTERVAL must be >= 1m")
	}
	if c.ConfluenceCalcInterval < 1*time.Minute {
		log.Fatal().Msg("CONFLUENCE_CALC_INTERVAL must be >= 1m")
	}
}

// ---------------------------------------------------------------------------
// Env Helpers
// ---------------------------------------------------------------------------

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal().Str("key", key).Msg("Required env var is not set")
	}
	return v
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// GetEnvDefault is the exported version of getEnv for use outside the config package.
func GetEnvDefault(key, defaultVal string) string {
	return getEnv(key, defaultVal)
}

func getDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Warn().Str("key", key).Str("value", v).Dur("default", defaultVal).Msg("Invalid duration, using default")
			return defaultVal
		}
		return d
	}
	return defaultVal
}

func getInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Warn().Str("key", key).Str("value", v).Int("default", defaultVal).Msg("Invalid int, using default")
			return defaultVal
		}
		return n
	}
	return defaultVal
}

// String returns a redacted configuration summary for logging.
func (c *Config) String() string {
	geminiStatus := "NOT CONFIGURED"
	if c.HasGemini() {
		geminiStatus = "CONFIGURED"
	}
	return fmt.Sprintf(
		"Config{DataDir=%s, COTInterval=%v, Gemini=%s, LogLevel=%s}",
		c.DataDir, c.COTFetchInterval, geminiStatus, c.LogLevel,
	)
}
