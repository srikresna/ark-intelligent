package domain

import "time"

// ---------------------------------------------------------------------------
// Confidence Level
// ---------------------------------------------------------------------------

// ConfidenceLevel rates data quality for predictions.
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "HIGH"   // >12 historical data points
	ConfidenceMedium ConfidenceLevel = "MEDIUM" // 6-12 data points
	ConfidenceLow    ConfidenceLevel = "LOW"    // <6 data points
)

// ClassifyConfidence returns the confidence level based on data point count.
func ClassifyConfidence(dataPoints int) ConfidenceLevel {
	switch {
	case dataPoints >= 12:
		return ConfidenceHigh
	case dataPoints >= 6:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

// ---------------------------------------------------------------------------
// Volatility Prediction — Expected Market Impact of Upcoming Event
// ---------------------------------------------------------------------------

// VolatilityPrediction estimates the expected pip movement for an upcoming event.
type VolatilityPrediction struct {
	// Event reference
	EventName string      `json:"event_name"` // e.g., "Non-Farm Employment Change"
	Currency  string      `json:"currency"`   // e.g., "USD"
	EventDate time.Time   `json:"event_date"` // When the event occurs
	Impact    ImpactLevel `json:"impact"`     // Event impact level

	// Prediction
	ExpectedPipMove float64         `json:"expected_pip_move"` // Estimated pip movement
	Confidence      ConfidenceLevel `json:"confidence"`        // Data quality rating

	// Historical basis
	HistoricalAvgMove  float64 `json:"historical_avg_move"`  // Mean absolute pip move after this event
	HistoricalMaxMove  float64 `json:"historical_max_move"`  // Largest historical move
	HistoricalMinMove  float64 `json:"historical_min_move"`  // Smallest historical move
	HistoricalStdDev   float64 `json:"historical_stddev"`    // StdDev of historical moves
	DataPoints         int     `json:"data_points"`          // Number of historical releases used

	// Current context scaling
	RecentVolFactor float64 `json:"recent_vol_factor"` // Current volatility / historical (scaling factor)
	ImpactWeight    float64 `json:"impact_weight"`     // ImpactLevel.Weight()

	// Risk flags
	IsHighRisk     bool   `json:"is_high_risk"`     // Expected move > 50 pips
	RiskLevel      string `json:"risk_level"`       // "EXTREME", "HIGH", "MODERATE", "LOW"

	// Surprise context (if forecast available)
	Forecast              float64 `json:"forecast,omitempty"`                // Market consensus
	PreviousActual        float64 `json:"previous_actual,omitempty"`        // Last release actual
	HistoricalBeatRate    float64 `json:"historical_beat_rate,omitempty"`   // % of times actual > forecast
	HistoricalAvgSurprise float64 `json:"historical_avg_surprise,omitempty"` // Average surprise magnitude

	// AI prediction (filled by Gemini)
	AIPrediction string `json:"ai_prediction,omitempty"`
}

// ClassifyRisk returns the risk level based on expected pip move.
func (vp *VolatilityPrediction) ClassifyRisk() string {
	switch {
	case vp.ExpectedPipMove >= 100:
		return "EXTREME"
	case vp.ExpectedPipMove >= 50:
		return "HIGH"
	case vp.ExpectedPipMove >= 20:
		return "MODERATE"
	default:
		return "LOW"
	}
}

// IsSignificant returns true if expected move warrants an alert.
func (vp *VolatilityPrediction) IsSignificant() bool {
	return vp.ExpectedPipMove >= 30 && vp.Confidence != ConfidenceLow
}

// ---------------------------------------------------------------------------
// Volatility Forecast — Collection of Predictions
// ---------------------------------------------------------------------------

// VolatilityForecast holds predictions for all upcoming events in a time window.
type VolatilityForecast struct {
	Timestamp   time.Time               `json:"timestamp"`
	WindowHours int                     `json:"window_hours"` // Forecast window (e.g., 48 hours)
	Predictions []VolatilityPrediction  `json:"predictions"`  // Sorted by ExpectedPipMove desc

	// Summary
	HighRiskCount int     `json:"high_risk_count"` // Number of high-risk events
	TotalEvents   int     `json:"total_events"`    // Total upcoming events
	MaxExpected   float64 `json:"max_expected"`     // Highest expected pip move
	RiskWindow    string  `json:"risk_window"`      // "ELEVATED", "NORMAL", "QUIET"
}

// ClassifyWindow returns the overall risk assessment for the forecast window.
func (vf *VolatilityForecast) ClassifyWindow() string {
	switch {
	case vf.HighRiskCount >= 3:
		return "ELEVATED"
	case vf.HighRiskCount >= 1:
		return "NORMAL"
	default:
		return "QUIET"
	}
}

// TopRisks returns the top N highest-risk events.
func (vf *VolatilityForecast) TopRisks(n int) []VolatilityPrediction {
	if n > len(vf.Predictions) {
		n = len(vf.Predictions)
	}
	return vf.Predictions[:n]
}
