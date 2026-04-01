package domain

import "time"

// Feedback represents a user reaction (thumbs up/down) on an analysis message.
type Feedback struct {
	UserID       int64     `json:"user_id"`
	AnalysisType string    `json:"analysis_type"` // "cot", "outlook", "alpha", "bias"
	AnalysisKey  string    `json:"analysis_key"`  // e.g. "EUR", "latest"
	Rating       string    `json:"rating"`        // "up" or "down"
	CreatedAt    time.Time `json:"created_at"`
}
