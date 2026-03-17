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

	// P2.1 — Surprise Scoring Engine
	SurpriseScore float64 `json:"surprise_score,omitempty"` // sigma units: (Actual - Forecast) / StdDev
	SurpriseLabel string  `json:"surprise_label,omitempty"` // e.g., "HAWKISH SURPRISE", "IN LINE"
}

// ---------------------------------------------------------------------------
// SurpriseRecord — Intra-week surprise accumulator for P2.2
// ---------------------------------------------------------------------------

// SurpriseRecord stores a processed surprise result for a currency event.
// Used by the Surprise-Adjusted COT Sentiment engine.
type SurpriseRecord struct {
	Currency   string    `json:"currency"`
	EventName  string    `json:"event_name"`
	Date       time.Time `json:"date"`
	SigmaValue float64   `json:"sigma_value"` // normalized surprise in sigma units
	Label      string    `json:"label"`       // e.g., "HAWKISH SURPRISE"
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
	case "holiday", "none":
		// MQL5 returns "none" for holidays/special days (not "holiday")
		return "🔵"
	default:
		return "⚪"
	}
}
