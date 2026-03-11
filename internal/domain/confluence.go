package domain

import "time"

// ---------------------------------------------------------------------------
// Confluence Factor Definitions
// ---------------------------------------------------------------------------

// FactorName identifies a confluence factor.
type FactorName string

const (
	FactorCOTPositioning    FactorName = "COT_POSITIONING"     // 25% weight
	FactorEconomicSurprise  FactorName = "ECONOMIC_SURPRISE"   // 20% weight
	FactorInterestRate      FactorName = "INTEREST_RATE"       // 20% weight
	FactorRevisionMomentum  FactorName = "REVISION_MOMENTUM"   // 15% weight
	FactorCrowdSentiment    FactorName = "CROWD_SENTIMENT"     // 10% weight
	FactorEventRisk         FactorName = "EVENT_RISK"          // 10% weight
)

// DefaultFactorWeights defines the standard weighting for each confluence factor.
var DefaultFactorWeights = map[FactorName]float64{
	FactorCOTPositioning:   0.25,
	FactorEconomicSurprise: 0.20,
	FactorInterestRate:     0.20,
	FactorRevisionMomentum: 0.15,
	FactorCrowdSentiment:   0.10,
	FactorEventRisk:        0.10,
}

// ---------------------------------------------------------------------------
// Confluence Factor — Individual Component
// ---------------------------------------------------------------------------

// ConfluenceFactor represents a single factor within the multi-factor model.
type ConfluenceFactor struct {
	Name          FactorName `json:"name"`           // Factor identifier
	Weight        float64    `json:"weight"`         // Weight in composite (0.0 - 1.0)
	RawScore      float64    `json:"raw_score"`      // Unweighted score (0-100)
	WeightedScore float64    `json:"weighted_score"` // RawScore * Weight
	Signal        string     `json:"signal"`         // "BULLISH", "BEARISH", "NEUTRAL"
	Confidence    string     `json:"confidence"`     // "HIGH", "MEDIUM", "LOW"
	Description   string     `json:"description"`    // Human-readable explanation
}

// IsBullish returns true if this factor leans bullish (raw score > 55).
func (f *ConfluenceFactor) IsBullish() bool {
	return f.RawScore > 55
}

// IsBearish returns true if this factor leans bearish (raw score < 45).
func (f *ConfluenceFactor) IsBearish() bool {
	return f.RawScore < 45
}

// ---------------------------------------------------------------------------
// Confluence Score — Multi-Factor Composite
// ---------------------------------------------------------------------------

// ConfluenceBias classifies the overall directional bias.
type ConfluenceBias string

const (
	BiasStrongBull ConfluenceBias = "STRONG_BULL" // Score > 75
	BiasBullish    ConfluenceBias = "BULLISH"     // Score 60-75
	BiasLeanBull   ConfluenceBias = "LEAN_BULL"   // Score 55-60
	BiasNeutral    ConfluenceBias = "NEUTRAL"     // Score 45-55
	BiasLeanBear   ConfluenceBias = "LEAN_BEAR"   // Score 40-45
	BiasBearish    ConfluenceBias = "BEARISH"     // Score 25-40
	BiasStrongBear ConfluenceBias = "STRONG_BEAR" // Score < 25
)

// ConfluenceScore is the composite multi-factor score for a currency pair.
// Score ranges from 0 (extreme bearish) to 100 (extreme bullish).
type ConfluenceScore struct {
	// Identification
	CurrencyPair string    `json:"currency_pair"` // e.g., "EURUSD"
	BaseCurrency string    `json:"base_currency"` // e.g., "EUR"
	QuoteCurrency string   `json:"quote_currency"` // e.g., "USD"
	Timestamp    time.Time `json:"timestamp"`

	// Composite score
	TotalScore float64        `json:"total_score"` // 0-100
	Bias       ConfluenceBias `json:"bias"`        // Overall direction classification

	// Individual factors
	Factors []ConfluenceFactor `json:"factors"` // All 6 factor breakdowns

	// Agreement metrics
	FactorsAligned  int     `json:"factors_aligned"`  // How many factors agree on direction
	AgreementPct    float64 `json:"agreement_pct"`    // % of factors in agreement
	StrongestFactor string  `json:"strongest_factor"` // Which factor has highest conviction
	WeakestFactor   string  `json:"weakest_factor"`   // Which factor is most uncertain

	// Change tracking
	PreviousScore float64 `json:"previous_score"` // Last calculation's score
	ScoreChange   float64 `json:"score_change"`   // Change from previous

	// AI interpretation (filled by Gemini)
	AINarrative string `json:"ai_narrative,omitempty"`
}

// ClassifyBias returns the ConfluenceBias based on the total score.
func ClassifyBias(score float64) ConfluenceBias {
	switch {
	case score >= 75:
		return BiasStrongBull
	case score >= 60:
		return BiasBullish
	case score >= 55:
		return BiasLeanBull
	case score >= 45:
		return BiasNeutral
	case score >= 40:
		return BiasLeanBear
	case score >= 25:
		return BiasBearish
	default:
		return BiasStrongBear
	}
}

// IsActionable returns true if the score is outside the neutral zone.
func (cs *ConfluenceScore) IsActionable() bool {
	return cs.TotalScore >= 65 || cs.TotalScore <= 35
}

// IsHighConviction returns true if most factors agree and the score is extreme.
func (cs *ConfluenceScore) IsHighConviction() bool {
	return cs.AgreementPct >= 0.7 && (cs.TotalScore >= 70 || cs.TotalScore <= 30)
}

// CrossedThreshold returns true if the score crossed a key level since last calc.
func (cs *ConfluenceScore) CrossedThreshold() bool {
	thresholds := []float64{30, 40, 50, 60, 70}
	for _, t := range thresholds {
		if (cs.PreviousScore < t && cs.TotalScore >= t) ||
			(cs.PreviousScore >= t && cs.TotalScore < t) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Major Pairs — Standard currency pairs for confluence analysis
// ---------------------------------------------------------------------------

// MajorPairs lists the 7 major currency pairs for confluence scoring.
var MajorPairs = []struct {
	Pair  string
	Base  string
	Quote string
}{
	{"EURUSD", "EUR", "USD"},
	{"GBPUSD", "GBP", "USD"},
	{"USDJPY", "USD", "JPY"},
	{"USDCHF", "USD", "CHF"},
	{"AUDUSD", "AUD", "USD"},
	{"USDCAD", "USD", "CAD"},
	{"NZDUSD", "NZD", "USD"},
}
