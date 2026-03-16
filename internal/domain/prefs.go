package domain

// UserPrefs stores per-user notification preferences.
type UserPrefs struct {
	AlertMinutes    []int    `json:"alert_minutes"`    // Minutes before event to alert (e.g., [60, 15, 5])
	AlertImpacts    []string `json:"alert_impacts"`    // Impact levels to alert (e.g., ["High", "Medium"])
	AlertsEnabled   bool     `json:"alerts_enabled"`   // Master switch for economic news
	AIReportsEnabled bool    `json:"ai_reports_enabled"` // Whether to receive AI analysis reports
	COTAlertsEnabled bool    `json:"cot_alerts_enabled"` // Whether to receive alerts for new COT data
	CurrencyFilter  []string `json:"currency_filter"`    // If set, only alert for these currencies
	Language        string   `json:"language"`           // AI output language ("id" or "en")
}

// DefaultPrefs returns the default user preferences.
func DefaultPrefs() UserPrefs {
	return UserPrefs{
		AlertMinutes:     []int{60, 15, 5, 1},
		AlertImpacts:     []string{"High", "Medium"},
		AlertsEnabled:    true,
		AIReportsEnabled: true,
		COTAlertsEnabled: true,
		CurrencyFilter:   nil, // nil = all currencies
		Language:        "id", // Default to Indonesian
	}
}
