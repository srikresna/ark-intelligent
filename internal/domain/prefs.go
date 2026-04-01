package domain

// ClaudeModelID identifies a specific Claude model variant.
type ClaudeModelID string

const (
	ClaudeModelOpus4   ClaudeModelID = "claude-opus-4-6"
	ClaudeModelSonnet4 ClaudeModelID = "claude-sonnet-4-6"
	ClaudeModelHaiku4  ClaudeModelID = "claude-haiku-4-5-20251001"
)

// ValidClaudeModels returns all supported Claude model IDs.
func ValidClaudeModels() []ClaudeModelID {
	return []ClaudeModelID{ClaudeModelOpus4, ClaudeModelSonnet4, ClaudeModelHaiku4}
}

// ClaudeModelLabel returns a short display label.
func ClaudeModelLabel(m ClaudeModelID) string {
	switch m {
	case ClaudeModelOpus4:
		return "Opus 4.6 (Terbaik)"
	case ClaudeModelSonnet4:
		return "Sonnet 4.6 (Seimbang)"
	case ClaudeModelHaiku4:
		return "Haiku 4.6 (Cepat)"
	default:
		return string(m)
	}
}

// IsValidClaudeModel returns true if the model ID is supported.
func IsValidClaudeModel(m ClaudeModelID) bool {
	for _, v := range ValidClaudeModels() {
		if v == m {
			return true
		}
	}
	return false
}


// OutputMode controls the verbosity of bot output.
type OutputMode string

const (
	OutputCompact  OutputMode = "compact"
	OutputFull     OutputMode = "full"
	OutputMinimal  OutputMode = "minimal"
)

// NextOutputMode cycles: compact -> full -> minimal -> compact.
func NextOutputMode(m OutputMode) OutputMode {
	switch m {
	case OutputCompact:
		return OutputFull
	case OutputFull:
		return OutputMinimal
	default:
		return OutputCompact
	}
}

// OutputModeLabel returns a display label for the mode.
func OutputModeLabel(m OutputMode) string {
	switch m {
	case OutputFull:
		return "Full Detail \U0001f4d6"
	case OutputMinimal:
		return "Minimal \u26a1"
	default:
		return "Compact \U0001f4ca"
	}
}

// UserPrefs stores per-user notification preferences.
type UserPrefs struct {
	AlertMinutes     []int    `json:"alert_minutes"`      // Minutes before event to alert (e.g., [60, 15, 5])
	AlertImpacts     []string `json:"alert_impacts"`      // Impact levels to alert (e.g., ["High", "Medium"])
	AlertsEnabled    bool     `json:"alerts_enabled"`     // Master switch for economic news
	AIReportsEnabled bool     `json:"ai_reports_enabled"` // Whether to receive AI analysis reports
	COTAlertsEnabled bool     `json:"cot_alerts_enabled"` // Whether to receive alerts for new COT data
	CurrencyFilter   []string `json:"currency_filter"`    // If set, only alert for these currencies
	Language         string   `json:"language"`           // AI output language ("id" or "en")

	// PreferredModel: "claude" (default), "gemini", or "" (= claude)
	PreferredModel string `json:"preferred_model,omitempty"`

	// ClaudeModel: specific Claude model variant (empty = server default from CLAUDE_MODEL env)
	// Only applies when PreferredModel is "claude" or empty.
	ClaudeModel ClaudeModelID `json:"claude_model,omitempty"`

	// Broadcast & UI state
	ChatID         string `json:"chat_id"`         // Telegram chat ID (set on /start, used for push alerts)
	CalendarFilter string `json:"calendar_filter"` // Last used calendar filter: "all", "high", "med", "cur:USD", etc.
	CalendarView   string     `json:"calendar_view"`   // Last used view: "day", "week", "month"
	OutputMode     OutputMode `json:"output_mode,omitempty"` // "compact" (default), "full", or "minimal"
	LastCurrency   string     `json:"last_currency,omitempty"` // Last viewed currency (e.g. "EUR", "USD")

	// DefaultTimeframe stores the user-preferred timeframe (e.g. "daily", "4h").
	DefaultTimeframe string `json:"default_timeframe,omitempty"`

	// PinnedCommands stores user-customized pinned command shortcuts.
	PinnedCommands []string `json:"pinned_commands,omitempty"`

	// ExperienceLevel: user's self-reported trading experience.
	// "beginner", "intermediate", "pro", or "" (not set yet → trigger onboarding).
	ExperienceLevel string `json:"experience_level,omitempty"`

	// MobileMode collapses wide tables into sparkline + one-liner summaries.
	// Optimised for narrow screens (320px mobile).
	MobileMode bool `json:"mobile_mode,omitempty"`
}

// DefaultPrefs returns the default user preferences.
func DefaultPrefs() UserPrefs {
	return UserPrefs{
		AlertMinutes:     []int{60, 15, 5, 1},
		AlertImpacts:     []string{"High", "Medium"},
		AlertsEnabled:    true,
		AIReportsEnabled: true,
		COTAlertsEnabled: true,
		CurrencyFilter:   nil,  // nil = all currencies
		Language:         "id", // Default to Indonesian
		ChatID:           "",
		CalendarFilter:   "all",
		CalendarView:     "day",
	}
}

// ValidTimeframes returns the list of valid analysis timeframes.
func ValidTimeframes() []string {
	return []string{"daily", "4h", "1h", "30m", "15m", "weekly"}
}

// IsValidTimeframe returns true if the timeframe string is recognized.
func IsValidTimeframe(tf string) bool {
	for _, v := range ValidTimeframes() {
		if v == tf {
			return true
		}
	}
	return false
}

// TimeframeLabel returns a human-friendly label for the timeframe.
func TimeframeLabel(tf string) string {
	switch tf {
	case "weekly":
		return "Weekly"
	case "daily":
		return "Daily"
	case "4h":
		return "4H"
	case "1h":
		return "1H"
	case "30m":
		return "30m"
	case "15m":
		return "15m"
	default:
		return "Daily"
	}
}

// ResolveDefaultTimeframe returns the user preference or "daily" if empty/invalid.
func ResolveDefaultTimeframe(pref string) string {
	if pref != "" && IsValidTimeframe(pref) {
		return pref
	}
	return "daily"
}
