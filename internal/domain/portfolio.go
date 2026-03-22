package domain

import "time"

// ---------------------------------------------------------------------------
// Portfolio Position — User-managed trading positions
// ---------------------------------------------------------------------------

// Position represents a single position in a user's portfolio.
type Position struct {
	Currency   string    `json:"currency"`    // e.g. "EUR", "GBP", "XAU"
	Direction  string    `json:"direction"`   // "LONG" or "SHORT"
	Size       float64   `json:"size"`        // Lots or units
	EntryPrice float64   `json:"entry_price"` // Price at entry
	AddedAt    time.Time `json:"added_at"`    // When the position was added
}

// ---------------------------------------------------------------------------
// Portfolio Risk — Cross-correlation and concentration analysis
// ---------------------------------------------------------------------------

// PortfolioRisk holds the computed risk analysis for a user's portfolio.
type PortfolioRisk struct {
	Positions               []Position                       `json:"positions"`
	CorrelationMatrix       map[string]map[string]float64    `json:"correlation_matrix"`
	HighCorrelationWarnings []string                         `json:"high_correlation_warnings"` // Pairs with |corr| > 0.7
	ConcentrationScore      float64                          `json:"concentration_score"`       // 0-100, higher = more concentrated
	RegimeRiskScore         float64                          `json:"regime_risk_score"`
	OverallRiskLevel        string                           `json:"overall_risk_level"` // LOW, MEDIUM, HIGH, CRITICAL
}
