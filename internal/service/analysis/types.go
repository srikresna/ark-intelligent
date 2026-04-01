// Package analysis provides unified multi-source directional signal fusion for currencies.
// It aggregates COT positioning, CTA technical signals, Quant regime models,
// Sentiment (VIX/Risk), and Seasonal data into a single scored recommendation.
package analysis

// Recommendation is the unified directional recommendation output.
type Recommendation string

const (
	RecommendationStrongLong  Recommendation = "STRONG_LONG"
	RecommendationLong        Recommendation = "LONG"
	RecommendationNeutral     Recommendation = "NEUTRAL"
	RecommendationShort       Recommendation = "SHORT"
	RecommendationStrongShort Recommendation = "STRONG_SHORT"
)

// VoteDirection captures the directional vote from a sub-system.
type VoteDirection int

const (
	VoteLong    VoteDirection = 1
	VoteNeutral VoteDirection = 0
	VoteShort   VoteDirection = -1
)

// ComponentScore holds the contribution of a single sub-system to the unified signal.
type ComponentScore struct {
	Name          string        // e.g. "COT", "CTA", "Quant", "Sentiment", "Seasonal"
	RawScore      float64       // input score from the sub-system (-100 to +100)
	NormalizedScore float64     // normalized to -1 to +1 before weighting
	WeightedScore float64       // NormalizedScore × Weight
	Weight        float64       // configured weight (0-1)
	Vote          VoteDirection // directional vote
	Available     bool          // whether the sub-system contributed data
}

// VotingMatrix summarizes agreement/disagreement among sub-systems.
type VotingMatrix struct {
	LongVotes    int      // number of sub-systems voting long
	ShortVotes   int      // number of sub-systems voting short
	NeutralVotes int      // number of sub-systems voting neutral
	ConflictCount int     // number of long-short conflicts
	Agreeing     []string // names of sub-systems in majority
	Dissenting   []string // names of sub-systems disagreeing with majority
}

// UnifiedSignalV2 is the output of the unified signal engine per currency.
type UnifiedSignalV2 struct {
	Currency       string          // e.g. "EUR"
	UnifiedScore   float64         // -100 to +100 (positive = bullish)
	Grade          string          // "A+", "A", "B", "C", "D", "F"
	Confidence     float64         // 0-100 (reduced when conflicts present)
	ConflictCount  int             // number of long-short sub-system conflicts
	Recommendation Recommendation  // STRONG_LONG / LONG / NEUTRAL / SHORT / STRONG_SHORT
	Components     []ComponentScore // per-subsystem breakdown
	VotingMatrix   VotingMatrix    // agreement/disagreement summary
	VIXMultiplier  float64         // applied VIX dampening factor (1.0 = no dampening)
	AsOf           string          // timestamp of computation
}
