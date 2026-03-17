package domain

import "time"

// NewsEvent represents a single scheduled economic calendar event.
type NewsEvent struct {
	ID         string    `json:"id"`
	Date       string    `json:"date"`     // e.g., "Mon Mar 17"
	Time       string    `json:"time"`     // e.g., "7:30am" or "Tentative"
	TimeWIB    time.Time `json:"time_wib"` // Parsed time for sorting and cron scheduling
	Currency   string    `json:"currency"`
	Event      string    `json:"event"`
	Impact     string    `json:"impact"` // "high", "medium", "low", "non"
	Forecast   string    `json:"forecast"`
	Previous   string    `json:"previous"`
	Actual     string    `json:"actual"`
	Status     string    `json:"status"` // "upcoming", "released", "pending_retry", "missed"
	RetryCount int       `json:"retry_count"`

	// Optional meta for detailed views
	Description string `json:"description,omitempty"`
}

// FormatImpactColor returns the appropriate UI color dot for the impact.
func (e NewsEvent) FormatImpactColor() string {
	switch e.Impact {
	case "high":
		return "🔴"
	case "medium":
		return "🟠"
	case "low":
		return "🟡"
	default:
		return "⚪"
	}
}
