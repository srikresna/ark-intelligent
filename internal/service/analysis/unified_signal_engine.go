package analysis

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// defaultWeights defines the sub-system contribution weights for UnifiedSignalV2.
// All weights must sum to 1.0.
var defaultWeights = struct {
	COT       float64
	CTA       float64
	Quant     float64
	Sentiment float64
	Seasonal  float64
}{
	COT:       0.30,
	CTA:       0.30,
	Quant:     0.20,
	Sentiment: 0.15,
	Seasonal:  0.05,
}

// UnifiedSignalInput carries the raw inputs needed to compute UnifiedSignalV2.
type UnifiedSignalInput struct {
	// COT sub-system
	COTAnalysis   *domain.COTAnalysis
	MacroRegime   fred.MacroRegime
	MacroData     *fred.MacroData
	SurpriseSigma float64

	// CTA sub-system: technical confluence from TA engine
	CTAConfluence *ta.ConfluenceResult

	// Quant sub-system: HMM regime + GARCH vol context
	HMM      *price.HMMResult
	GARCH    *price.GARCHResult
	GJRGARCH *price.GJRGARCHResult

	// Sentiment sub-system: VIX / risk context
	Risk *domain.RiskContext

	// Seasonal sub-system: current month bias for this currency
	Seasonal *price.SeasonalPattern
}

// ComputeUnifiedSignal computes a UnifiedSignalV2 for the given currency using
// available sub-system inputs. Missing inputs degrade gracefully — the engine
// skips unavailable sub-systems and redistributes weights proportionally.
//
// Algorithm:
//  1. Normalize each sub-system score to [-1, +1]
//  2. Apply proportional weights (only available sub-systems contribute)
//  3. Scale to [-100, +100] → UnifiedScore
//  4. Detect conflicts: long-short disagreements reduce Confidence by 20% each
//  5. Apply VIX dampening when VIX > 20 (up to -15% score magnitude)
//  6. Classify Grade, Recommendation
func ComputeUnifiedSignal(currency string, in UnifiedSignalInput) *UnifiedSignalV2 {
	components := buildComponents(in)

	// Compute weighted sum over available components
	totalWeight := 0.0
	weightedSum := 0.0
	for _, c := range components {
		if c.Available {
			totalWeight += c.Weight
			weightedSum += c.WeightedScore
		}
	}

	var rawScore float64
	if totalWeight > 0 {
		// Normalize weighted sum back to -1..+1 range then scale to -100..+100
		rawScore = (weightedSum / totalWeight) * 100
	}
	rawScore = mathutil.Clamp(rawScore, -100, 100)

	// Build voting matrix
	vm := buildVotingMatrix(components)

	// Confidence starts at 100%, reduced per conflict
	confidence := 100.0
	for i := 0; i < vm.ConflictCount; i++ {
		confidence *= 0.80 // -20% per conflict
	}

	// VIX dampening: when VIX > 20, dampen score magnitude by up to 15%
	vixMultiplier := computeVIXMultiplier(in.Risk)
	finalScore := rawScore * vixMultiplier
	finalScore = mathutil.Clamp(finalScore, -100, 100)

	sig := &UnifiedSignalV2{
		Currency:       currency,
		UnifiedScore:   roundN(finalScore, 2),
		Grade:          scoreToGrade(finalScore),
		Confidence:     roundN(confidence, 1),
		ConflictCount:  vm.ConflictCount,
		Recommendation: scoreToRecommendation(finalScore, confidence),
		Components:     components,
		VotingMatrix:   vm,
		VIXMultiplier:  roundN(vixMultiplier, 3),
		AsOf:           time.Now().Format("2006-01-02 15:04"),
	}

	return sig
}

// ComputeUnifiedSignalForCurrency is a convenience wrapper that fetches COT
// data and calls ComputeUnifiedSignal. Non-critical inputs (CTA, HMM, GARCH,
// Risk, Seasonal) are accepted as optional parameters.
func ComputeUnifiedSignalForCurrency(
	ctx context.Context,
	currency string,
	cotAnalysis *domain.COTAnalysis,
	regime fred.MacroRegime,
	macroData *fred.MacroData,
	surpriseSigma float64,
	ctaConfl *ta.ConfluenceResult,
	hmm *price.HMMResult,
	garch *price.GARCHResult,
	gjrGarch *price.GJRGARCHResult,
	risk *domain.RiskContext,
	seasonal *price.SeasonalPattern,
) *UnifiedSignalV2 {
	in := UnifiedSignalInput{
		COTAnalysis:   cotAnalysis,
		MacroRegime:   regime,
		MacroData:     macroData,
		SurpriseSigma: surpriseSigma,
		CTAConfluence: ctaConfl,
		HMM:           hmm,
		GARCH:         garch,
		GJRGARCH:      gjrGarch,
		Risk:          risk,
		Seasonal:      seasonal,
	}
	return ComputeUnifiedSignal(currency, in)
}

// ---------------------------------------------------------------------------
// Component builders
// ---------------------------------------------------------------------------

func buildComponents(in UnifiedSignalInput) []ComponentScore {
	return []ComponentScore{
		buildCOTComponent(in),
		buildCTAComponent(in),
		buildQuantComponent(in),
		buildSentimentComponent(in),
		buildSeasonalComponent(in),
	}
}

// buildCOTComponent derives the COT signal from SentimentScore + ConvictionScoreV3.
func buildCOTComponent(in UnifiedSignalInput) ComponentScore {
	c := ComponentScore{
		Name:   "COT",
		Weight: defaultWeights.COT,
	}
	if in.COTAnalysis == nil {
		return c
	}

	// Use ConvictionScoreV3 if we have macro regime data; fall back to SentimentScore
	var rawScore float64
	if in.MacroData != nil {
		cs := cot.ComputeConvictionScoreV3(*in.COTAnalysis, in.MacroRegime, in.SurpriseSigma, "", in.MacroData, nil)
		// ConvictionScore is 0-100; convert to -100..+100
		rawScore = (cs.Score - 50) * 2
	} else {
		rawScore = in.COTAnalysis.SentimentScore
	}

	rawScore = mathutil.Clamp(rawScore, -100, 100)
	norm := rawScore / 100.0
	c.RawScore = rawScore
	c.NormalizedScore = norm
	c.WeightedScore = norm * c.Weight
	c.Vote = toVote(norm, 0.15)
	c.Available = true
	return c
}

// buildCTAComponent uses the TA confluence score.
func buildCTAComponent(in UnifiedSignalInput) ComponentScore {
	c := ComponentScore{
		Name:   "CTA",
		Weight: defaultWeights.CTA,
	}
	if in.CTAConfluence == nil {
		return c
	}

	rawScore := mathutil.Clamp(in.CTAConfluence.Score, -100, 100)
	norm := rawScore / 100.0
	c.RawScore = rawScore
	c.NormalizedScore = norm
	c.WeightedScore = norm * c.Weight
	c.Vote = toVote(norm, 0.15)
	c.Available = true
	return c
}

// buildQuantComponent fuses HMM regime + GARCH vol into a directional bias.
// RISK_ON → bullish bias; CRISIS → bearish; RISK_OFF → neutral.
// High GARCH vol reduces directional conviction.
func buildQuantComponent(in UnifiedSignalInput) ComponentScore {
	c := ComponentScore{
		Name:   "Quant",
		Weight: defaultWeights.Quant,
	}
	if in.HMM == nil {
		return c
	}

	// Base score from HMM regime
	var rawScore float64
	switch in.HMM.CurrentState {
	case "RISK_ON":
		rawScore = 60
	case "RISK_OFF":
		rawScore = 0
	case "CRISIS":
		rawScore = -60
	default:
		rawScore = 0
	}

	// Adjust by regime transition warning — if transitioning to crisis, dampen bull bias
	if in.HMM.TransitionWarning != "" && rawScore > 0 {
		rawScore *= 0.6
	}

	// GARCH vol ratio adjustment: high vol → reduce magnitude
	if in.GARCH != nil && in.GARCH.VolRatio > 1.5 {
		rawScore *= 0.75
	}

	// GJR-GARCH leverage effect: high asymmetry → dampen long conviction
	// Downside vol elevated → reduce bullish bias
	if in.GJRGARCH != nil && in.GJRGARCH.LeverageEffect && rawScore > 0 {
		if in.GJRGARCH.AsymmetryLabel == "HIGH" {
			rawScore *= 0.80 // HIGH asymmetry → 20% bullish dampening
		} else {
			rawScore *= 0.90 // MODERATE asymmetry → 10% bullish dampening
		}
	}

	rawScore = mathutil.Clamp(rawScore, -100, 100)
	norm := rawScore / 100.0
	c.RawScore = rawScore
	c.NormalizedScore = norm
	c.WeightedScore = norm * c.Weight
	c.Vote = toVote(norm, 0.1)
	c.Available = true
	return c
}

// buildSentimentComponent derives a signal from VIX / risk context.
// Low VIX (risk-on) → bullish; high VIX (risk-off) → bearish.
func buildSentimentComponent(in UnifiedSignalInput) ComponentScore {
	c := ComponentScore{
		Name:   "Sentiment",
		Weight: defaultWeights.Sentiment,
	}
	if in.Risk == nil {
		return c
	}

	var rawScore float64
	switch domain.ClassifyRiskRegime(in.Risk.VIXLevel) {
	case domain.RiskRegimeLow:
		rawScore = 60 // VIX < 15: risk appetite high
	case domain.RiskRegimeNormal:
		rawScore = 20 // VIX 15-20: mild risk-on
	case domain.RiskRegimeElevated:
		rawScore = -30 // VIX 20-30: cautious
	case domain.RiskRegimePanic:
		rawScore = -75 // VIX > 30: risk-off / panic
	}

	// Additional adjustment: backwardation (VIX > VIX3M) = stress
	if in.Risk.IsBackwardation {
		rawScore -= 15
	}

	rawScore = mathutil.Clamp(rawScore, -100, 100)
	norm := rawScore / 100.0
	c.RawScore = rawScore
	c.NormalizedScore = norm
	c.WeightedScore = norm * c.Weight
	c.Vote = toVote(norm, 0.1)
	c.Available = true
	return c
}

// buildSeasonalComponent extracts the current month's seasonal bias.
func buildSeasonalComponent(in UnifiedSignalInput) ComponentScore {
	c := ComponentScore{
		Name:   "Seasonal",
		Weight: defaultWeights.Seasonal,
	}
	if in.Seasonal == nil {
		return c
	}

	var rawScore float64
	switch in.Seasonal.CurrentBias {
	case "BULLISH":
		rawScore = 50
	case "BEARISH":
		rawScore = -50
	default:
		rawScore = 0
	}

	// Adjust by current month's win rate and average return (from Monthly[month-1])
	month := time.Now().Month()
	if int(month) >= 1 && int(month) <= 12 {
		ms := in.Seasonal.Monthly[int(month)-1]
		if ms.SampleSize >= 5 {
			// Scale avg return to -50..+50 contribution
			rawScore = mathutil.Clamp(ms.AvgReturn*500, -50, 50)
			if ms.WinRate >= 0.65 {
				rawScore *= 1.2 // high conviction
			} else if ms.WinRate <= 0.35 {
				rawScore *= 1.2
				if rawScore > 0 {
					rawScore = -rawScore
				}
			}
		}
	}

	rawScore = mathutil.Clamp(rawScore, -100, 100)
	norm := rawScore / 100.0
	c.RawScore = rawScore
	c.NormalizedScore = norm
	c.WeightedScore = norm * c.Weight
	c.Vote = toVote(norm, 0.1)
	c.Available = true
	return c
}

// ---------------------------------------------------------------------------
// Voting matrix
// ---------------------------------------------------------------------------

func buildVotingMatrix(components []ComponentScore) VotingMatrix {
	var vm VotingMatrix
	var agreeing, dissenting []string

	// Count votes
	longVoters := []string{}
	shortVoters := []string{}
	for _, c := range components {
		if !c.Available {
			continue
		}
		switch c.Vote {
		case VoteLong:
			vm.LongVotes++
			longVoters = append(longVoters, c.Name)
		case VoteShort:
			vm.ShortVotes++
			shortVoters = append(shortVoters, c.Name)
		default:
			vm.NeutralVotes++
		}
	}

	// Conflict = both long and short votes present
	vm.ConflictCount = int(math.Min(float64(vm.LongVotes), float64(vm.ShortVotes)))

	// Majority direction
	if vm.LongVotes > vm.ShortVotes {
		agreeing = longVoters
		dissenting = shortVoters
	} else if vm.ShortVotes > vm.LongVotes {
		agreeing = shortVoters
		dissenting = longVoters
	} else {
		// Tie — all in neutral
		for _, c := range components {
			if c.Available {
				agreeing = append(agreeing, c.Name)
			}
		}
	}

	vm.Agreeing = agreeing
	vm.Dissenting = dissenting
	return vm
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// computeVIXMultiplier returns a dampening factor for the unified score
// when VIX is elevated. Returns 1.0 when VIX < 20 (no dampening).
func computeVIXMultiplier(risk *domain.RiskContext) float64 {
	if risk == nil {
		return 1.0
	}
	vix := risk.VIXLevel
	switch {
	case vix >= 30:
		return 0.85 // -15% magnitude
	case vix >= 25:
		return 0.90
	case vix >= 20:
		return 0.95
	default:
		return 1.0
	}
}

// toVote converts a normalized score (-1 to +1) to a directional vote.
// The threshold defines the minimum magnitude to cast a directional vote.
func toVote(norm, threshold float64) VoteDirection {
	if norm >= threshold {
		return VoteLong
	}
	if norm <= -threshold {
		return VoteShort
	}
	return VoteNeutral
}

// scoreToGrade converts a -100..+100 score to a letter grade based on magnitude.
func scoreToGrade(score float64) string {
	abs := math.Abs(score)
	switch {
	case abs >= 85:
		return "A+"
	case abs >= 70:
		return "A"
	case abs >= 55:
		return "B"
	case abs >= 40:
		return "C"
	case abs >= 20:
		return "D"
	default:
		return "F"
	}
}

// scoreToRecommendation converts unified score + confidence to a Recommendation.
func scoreToRecommendation(score, confidence float64) Recommendation {
	// Low confidence → downgrade strong signals
	if confidence < 40 {
		if score > 0 {
			return RecommendationLong
		}
		if score < 0 {
			return RecommendationShort
		}
		return RecommendationNeutral
	}
	switch {
	case score >= 60:
		return RecommendationStrongLong
	case score >= 25:
		return RecommendationLong
	case score <= -60:
		return RecommendationStrongShort
	case score <= -25:
		return RecommendationShort
	default:
		return RecommendationNeutral
	}
}

func roundN(v float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(v*factor) / factor
}

// RecommendationEmoji returns an emoji for the recommendation.
func RecommendationEmoji(r Recommendation) string {
	switch r {
	case RecommendationStrongLong:
		return "🟢🟢"
	case RecommendationLong:
		return "🟢"
	case RecommendationNeutral:
		return "⚪"
	case RecommendationShort:
		return "🔴"
	case RecommendationStrongShort:
		return "🔴🔴"
	default:
		return "❓"
	}
}

// ConfidenceLabel returns a human-readable confidence label.
func ConfidenceLabel(confidence float64) string {
	switch {
	case confidence >= 80:
		return "HIGH"
	case confidence >= 60:
		return "MODERATE"
	case confidence >= 40:
		return "LOW"
	default:
		return "VERY LOW"
	}
}

// RecommendationLabel returns a display string for a Recommendation.
func RecommendationLabel(r Recommendation) string {
	switch r {
	case RecommendationStrongLong:
		return "STRONG LONG"
	case RecommendationLong:
		return "LONG"
	case RecommendationNeutral:
		return "NEUTRAL"
	case RecommendationShort:
		return "SHORT"
	case RecommendationStrongShort:
		return "STRONG SHORT"
	default:
		return string(r)
	}
}

// FormatComponentBar returns a simple ASCII bar visualization of a normalized score.
func FormatComponentBar(norm float64) string {
	filled := int(math.Round(math.Abs(norm) * 5))
	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < 5; i++ {
		bar += "░"
	}
	if norm >= 0 {
		return fmt.Sprintf("🟩%s", bar)
	}
	return fmt.Sprintf("🟥%s", bar)
}
