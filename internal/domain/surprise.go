package domain

import "time"

// ---------------------------------------------------------------------------
// Surprise Score — Individual Event Measurement
// ---------------------------------------------------------------------------

// SurpriseScore measures how much an economic release deviated from expectations.
// This is the atomic unit of the Surprise Index.
type SurpriseScore struct {
	// Identification
	EventID   string    `json:"event_id"`   // Links to FFEvent.ID
	EventName string    `json:"event_name"` // e.g., "Non-Farm Employment Change"
	Currency  string    `json:"currency"`   // e.g., "USD"
	Timestamp time.Time `json:"timestamp"`  // When the actual was released

	// Raw data
	Actual   float64 `json:"actual"`
	Forecast float64 `json:"forecast"`
	Previous float64 `json:"previous"`

	// Calculated metrics
	Surprise           float64     `json:"surprise"`             // Actual - Forecast (raw)
	NormalizedSurprise float64     `json:"normalized_surprise"`  // (Actual - Forecast) / Historical StdDev
	WeightedImpact     float64     `json:"weighted_impact"`      // NormalizedSurprise * ImpactWeight
	ImpactLevel        ImpactLevel `json:"impact_level"`         // High=3, Med=2, Low=1

	// Historical context
	HistoricalStdDev   float64 `json:"historical_stddev"`    // StdDev of (Actual-Forecast) over past releases
	HistoricalMean     float64 `json:"historical_mean"`      // Mean surprise for this event
	PercentileRank     float64 `json:"percentile_rank"`      // Where this surprise falls in distribution (0-100)
	DataPointsUsed     int     `json:"data_points_used"`     // How many historical releases used for StdDev
}

// IsPositiveSurprise returns true if the actual beat expectations.
func (s *SurpriseScore) IsPositiveSurprise() bool {
	return s.Surprise > 0
}

// IsSignificant returns true if the normalized surprise exceeds 1 standard deviation.
func (s *SurpriseScore) IsSignificant() bool {
	if s.NormalizedSurprise > 1.0 || s.NormalizedSurprise < -1.0 {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Surprise Index — Rolling Currency-Level Aggregate
// ---------------------------------------------------------------------------

// SurpriseIndex is the time-decayed rolling aggregate of surprise scores
// for a single currency. Similar to the Citi Economic Surprise Index.
type SurpriseIndex struct {
	Currency    string    `json:"currency"`     // e.g., "USD"
	Timestamp   time.Time `json:"timestamp"`    // When this index was last calculated

	// Composite score
	RollingScore float64 `json:"rolling_score"` // Time-decayed sum of weighted surprises
	Direction    string  `json:"direction"`     // "IMPROVING", "DETERIORATING", "STABLE"
	Streak       int     `json:"streak"`        // Consecutive positive or negative surprises

	// Configuration
	DecayHalfLife float64 `json:"decay_half_life"` // Half-life in days (default: 30)
	WindowDays    int     `json:"window_days"`     // Lookback window (default: 90)

	// Components breakdown
	Components []SurpriseComponent `json:"components"` // Recent surprise scores contributing to index

	// Statistics
	TotalEvents     int     `json:"total_events"`      // Number of events in window
	PositiveCount   int     `json:"positive_count"`    // Number of positive surprises
	NegativeCount   int     `json:"negative_count"`    // Number of negative surprises
	AvgSurprise     float64 `json:"avg_surprise"`      // Average normalized surprise in window
	ForecastAccuracy float64 `json:"forecast_accuracy"` // % of forecasts within 1 StdDev of actual
}

// SurpriseComponent is a single event contributing to the SurpriseIndex.
type SurpriseComponent struct {
	EventName      string    `json:"event_name"`
	Timestamp      time.Time `json:"timestamp"`
	WeightedImpact float64   `json:"weighted_impact"` // After time-decay applied
	DecayFactor    float64   `json:"decay_factor"`    // Current decay multiplier (0-1)
}

// IsPositive returns true if the rolling score indicates positive economic momentum.
func (si *SurpriseIndex) IsPositive() bool {
	return si.RollingScore > 0
}

// Strength returns the conviction level based on rolling score magnitude.
func (si *SurpriseIndex) Strength() string {
	abs := si.RollingScore
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs > 5.0:
		return "STRONG"
	case abs > 2.0:
		return "MODERATE"
	case abs > 0.5:
		return "WEAK"
	default:
		return "NEUTRAL"
	}
}

// ---------------------------------------------------------------------------
// Revision Momentum — Tracks persistent revision direction
// ---------------------------------------------------------------------------

// RevisionMomentum tracks the direction and persistence of economic data revisions
// for a currency. Persistent upward revisions = improving economy (leading indicator).
type RevisionMomentum struct {
	Currency  string    `json:"currency"`
	Timestamp time.Time `json:"timestamp"`

	// Momentum metrics
	Direction string  `json:"direction"` // "UP", "DOWN", "MIXED"
	Streak    int     `json:"streak"`    // Consecutive revisions in same direction
	Score     float64 `json:"score"`     // Weighted revision score (-10 to +10)

	// Breakdown
	TotalRevisions int     `json:"total_revisions"` // Total revisions in window
	UpRevisions    int     `json:"up_revisions"`    // Upward revisions count
	DownRevisions  int     `json:"down_revisions"`  // Downward revisions count
	AvgMagnitude   float64 `json:"avg_magnitude"`   // Average revision size

	// Window
	WindowDays int `json:"window_days"` // Lookback period (default: 90)
}

// IsUpward returns true if revision momentum is predominantly upward.
func (rm *RevisionMomentum) IsUpward() bool {
	return rm.Direction == "UP" && rm.Streak >= 2
}

// IsDownward returns true if revision momentum is predominantly downward.
func (rm *RevisionMomentum) IsDownward() bool {
	return rm.Direction == "DOWN" && rm.Streak >= 2
}

// IsSignificant returns true if there's a clear directional bias.
func (rm *RevisionMomentum) IsSignificant() bool {
	if rm.Score > 3.0 || rm.Score < -3.0 {
		return true
	}
	return false
}
