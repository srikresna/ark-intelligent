// Package config manages application configuration via environment variables.
// All settings have sensible defaults. Required vars fail fast on startup.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// Price APIs (optional — graceful degradation to Yahoo fallback)
	TwelveDataAPIKeys   []string      // Twelve Data API keys (comma-separated for round-robin)
	AlphaVantageAPIKeys []string      // Alpha Vantage API keys (comma-separated, for oil + treasury)
	CoinGeckoAPIKey     string        // CoinGecko API key (for TOTAL3 altcoin market cap)
	PriceFetchInterval  time.Duration // How often to fetch price data
	PriceHistoryWeeks   int           // How many weeks of price history to bootstrap

	// Intraday multi-timeframe
	IntradayFetchInterval time.Duration // How often to fetch intraday data (default: 15m)
	IntradayRetentionDays int           // How many days of intraday data to retain (default: 60)

	// Claude Chatbot (optional — graceful degradation without)
	ClaudeEndpoint       string        // Claude API proxy URL
	ClaudeModel          string        // Model name (default: claude-opus-4-6)
	ClaudeMaxTokens      int           // Max output tokens (default: 8192)
	ClaudeTimeout        time.Duration // HTTP timeout (default: 120s)
	ClaudeThinkingBudget int           // Extended thinking budget_tokens (default: 2048, 0=disabled)

	// Chat history
	ChatHistoryLimit int           // Max messages per user conversation (default: 50)
	ChatHistoryTTL   time.Duration // Conversation expiry (default: 7 days)

	// Logging
	LogLevel string // "debug", "info", "warn", "error"

	// Impact Bootstrap
	ImpactBootstrapMonths int // How many months of historical events to backfill (default: 12)

	// Massive API (formerly Polygon.io) — multiple keys for rotation (free tier)
	MassiveAPIKeys     []string // rotating REST API keys (comma-separated: MASSIVE_API_KEYS)
	MassiveWSBase      string   // WebSocket base URL (default: wss://socket.massive.com)
	MassiveRestBase    string   // REST base URL (default: https://api.massive.com)
	MassiveS3Endpoint  string   // Flat Files S3 endpoint (default: https://files.massive.com)
	MassiveS3AccessKey string   // Flat Files S3 access key
	MassiveS3SecretKey string   // Flat Files S3 secret key

	// Bybit API (crypto microstructure)
	BybitAPIKey    string // Bybit API key (optional - some endpoints are public)
	BybitAPISecret string // Bybit API secret
	BybitTestnet   bool   // Use testnet (default: false)
	BybitRestBase  string // REST base URL (derived from BybitTestnet; default: https://api.bybit.com)
	BybitWSBase    string // WebSocket base URL (default: wss://stream.bybit.com/v5/public/linear)

	// Feature flags
	EnableBybitMicrostructure bool // Enable Bybit crypto microstructure module (default: true if BybitAPIKey set)
	EnableMassiveResearch     bool // Enable Massive historical research layer (default: true if MassiveAPIKeys set)
	EnableFactorEngine        bool // Enable cross-sectional factor ranking engine (default: true)
	EnableStrategyPlaybook    bool // Enable regime playbook + conviction engine (default: true)
	EnablePortfolioHeat       bool // Enable portfolio exposure heat engine (default: true)
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

		// Claude Chatbot
		ClaudeEndpoint:       getEnv("CLAUDE_ENDPOINT", ""),
		ClaudeModel:          getEnv("CLAUDE_MODEL", "claude-opus-4-6"),
		ClaudeMaxTokens:      getInt("CLAUDE_MAX_TOKENS", 8192),
		ClaudeTimeout:        getDuration("CLAUDE_TIMEOUT", 120*time.Second),
		ClaudeThinkingBudget: getInt("CLAUDE_THINKING_BUDGET", 2048),

		// Chat history
		ChatHistoryLimit: getInt("CHAT_HISTORY_LIMIT", 50),
		ChatHistoryTTL:   getDuration("CHAT_HISTORY_TTL", 7*24*time.Hour),

		// Logging
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// Price APIs
		TwelveDataAPIKeys:   getStringSlice("TWELVE_DATA_API_KEYS"),
		AlphaVantageAPIKeys: getStringSlice("ALPHA_VANTAGE_API_KEYS"),
		CoinGeckoAPIKey:     getEnv("COINGECKO_API_KEY", ""),
		PriceFetchInterval:  getDuration("PRICE_FETCH_INTERVAL", 6*time.Hour),
		PriceHistoryWeeks:   getInt("PRICE_HISTORY_WEEKS", 52),

		// Intraday multi-timeframe
		IntradayFetchInterval: getDuration("INTRADAY_FETCH_INTERVAL", 15*time.Minute),
		IntradayRetentionDays: getInt("INTRADAY_RETENTION_DAYS", 60),

		// Impact Bootstrap
		ImpactBootstrapMonths: getInt("IMPACT_BOOTSTRAP_MONTHS", 12),

		// Massive API
		MassiveAPIKeys:     getStringSlice("MASSIVE_API_KEYS"),
		MassiveWSBase:      getEnv("MASSIVE_WS_BASE", "wss://socket.massive.com"),
		MassiveRestBase:    getEnv("MASSIVE_REST_BASE", "https://api.massive.com"),
		MassiveS3Endpoint:  getEnv("MASSIVE_S3_ENDPOINT", "https://files.massive.com"),
		MassiveS3AccessKey: getEnv("MASSIVE_S3_ACCESS_KEY", ""),
		MassiveS3SecretKey: getEnv("MASSIVE_S3_SECRET_KEY", ""),

		// Bybit API
		BybitAPIKey:    getEnv("BYBIT_API_KEY", ""),
		BybitAPISecret: getEnv("BYBIT_API_SECRET", ""),
		BybitTestnet:   getBool("BYBIT_TESTNET", false),
	}

	// Backward compat: TWELVE_DATA_API_KEY (singular) works for single key
	if len(cfg.TwelveDataAPIKeys) == 0 {
		if single := getEnv("TWELVE_DATA_API_KEY", ""); single != "" {
			cfg.TwelveDataAPIKeys = []string{single}
		}
	}

	// Backward compat: MASSIVE_API_KEY (singular) works too
	if len(cfg.MassiveAPIKeys) == 0 {
		if single := getEnv("MASSIVE_API_KEY", ""); single != "" {
			cfg.MassiveAPIKeys = []string{single}
		}
	}

	// Compute Bybit base URLs from testnet flag
	if cfg.BybitTestnet {
		cfg.BybitRestBase = "https://api-testnet.bybit.com"
		cfg.BybitWSBase = "wss://stream-testnet.bybit.com/v5/public/linear"
	} else {
		cfg.BybitRestBase = "https://api.bybit.com"
		cfg.BybitWSBase = "wss://stream.bybit.com/v5/public/linear"
	}

	// Feature flags (auto-detect from API key presence, overridable via env)
	cfg.EnableBybitMicrostructure = cfg.BybitAPIKey != "" || getEnv("ENABLE_BYBIT_MICROSTRUCTURE", "") == "true"
	cfg.EnableMassiveResearch = len(cfg.MassiveAPIKeys) > 0 || getEnv("ENABLE_MASSIVE_RESEARCH", "") == "true"
	cfg.EnableFactorEngine = getBool("ENABLE_FACTOR_ENGINE", true)
	cfg.EnableStrategyPlaybook = getBool("ENABLE_STRATEGY_PLAYBOOK", true)
	cfg.EnablePortfolioHeat = getBool("ENABLE_PORTFOLIO_HEAT", true)

	cfg.validate()
	return cfg
}

// HasGemini returns true if Gemini API key is configured.
func (c *Config) HasGemini() bool {
	return c.GeminiAPIKey != ""
}

// HasClaude returns true if Claude endpoint is configured.
func (c *Config) HasClaude() bool {
	return c.ClaudeEndpoint != ""
}

// HasTwelveData returns true if at least one Twelve Data API key is configured.
func (c *Config) HasTwelveData() bool {
	return len(c.TwelveDataAPIKeys) > 0
}

// HasAlphaVantage returns true if at least one Alpha Vantage API key is configured.
func (c *Config) HasAlphaVantage() bool {
	return len(c.AlphaVantageAPIKeys) > 0
}

// HasCoinGecko returns true if CoinGecko API key is configured.
func (c *Config) HasCoinGecko() bool {
	return c.CoinGeckoAPIKey != ""
}

// HasMassive returns true if at least one Massive API key is configured.
func (c *Config) HasMassive() bool { return len(c.MassiveAPIKeys) > 0 }

// HasBybit returns true if Bybit API key is configured.
func (c *Config) HasBybit() bool { return c.BybitAPIKey != "" }

// HasMassiveS3 returns true if Massive S3 credentials are configured.
func (c *Config) HasMassiveS3() bool {
	return c.MassiveS3AccessKey != "" && c.MassiveS3SecretKey != ""
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

	// Cross-field: Claude endpoint requires model to be set.
	if c.ClaudeEndpoint != "" && c.ClaudeModel == "" {
		log.Fatal().Msg("CLAUDE_MODEL must be set when CLAUDE_ENDPOINT is configured")
	}

	// Cross-field: Massive S3 credentials must be paired (both or neither).
	hasS3Key := c.MassiveS3AccessKey != ""
	hasS3Secret := c.MassiveS3SecretKey != ""
	if hasS3Key != hasS3Secret {
		log.Fatal().Msg("MASSIVE_S3_ACCESS_KEY and MASSIVE_S3_SECRET_KEY must both be set or both empty")
	}

	// DATA_DIR writable check — fail fast at startup rather than on first write.
	testFile := filepath.Join(c.DataDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("ok"), 0600); err != nil {
		log.Fatal().Str("dir", c.DataDir).Err(err).Msg("DATA_DIR is not writable")
	}
	_ = os.Remove(testFile)

	// Advisory: Gemini API key set but model left at default — warn only.
	if c.GeminiAPIKey != "" && c.GeminiModel == "" {
		log.Warn().Msg("GEMINI_API_KEY is set but GEMINI_MODEL is empty; using hardcoded default")
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

func getStringSlice(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func getBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultVal
	}
	return b
}

// String returns a redacted configuration summary for logging.
func (c *Config) String() string {
	geminiStatus := "NOT CONFIGURED"
	if c.HasGemini() {
		geminiStatus = "CONFIGURED"
	}
	claudeStatus := "NOT CONFIGURED"
	if c.HasClaude() {
		claudeStatus = "CONFIGURED"
	}
	priceStatus := "YAHOO_ONLY"
	if c.HasTwelveData() && c.HasAlphaVantage() {
		priceStatus = "FULL"
	} else if c.HasTwelveData() {
		priceStatus = "TWELVEDATA+YAHOO"
	} else if c.HasAlphaVantage() {
		priceStatus = "ALPHAVANTAGE+YAHOO"
	}
	massiveStatus := "NOT CONFIGURED"
	if c.HasMassive() {
		massiveStatus = fmt.Sprintf("CONFIGURED (%d keys)", len(c.MassiveAPIKeys))
	}
	bybitStatus := "NOT CONFIGURED"
	if c.HasBybit() {
		bybitStatus = "CONFIGURED"
		if c.BybitTestnet {
			bybitStatus = "CONFIGURED (TESTNET)"
		}
	}
	return fmt.Sprintf(
		"Config{DataDir=%s, COTInterval=%v, Gemini=%s, Claude=%s, Price=%s, Massive=%s, Bybit=%s, LogLevel=%s}",
		c.DataDir, c.COTFetchInterval, geminiStatus, claudeStatus, priceStatus, massiveStatus, bybitStatus, c.LogLevel,
	)
}
