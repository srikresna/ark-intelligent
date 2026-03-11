package quant

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/pkg/fmtutil"
	"github.com/arkcode369/ff-calendar-bot/pkg/mathutil"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

// ConfluenceScorer combines 6 independent factors into a single directional
// score (0-100) for each currency pair. Higher = more bullish for the pair.
//
// Factors and weights:
//   1. COT Positioning      (25%) - Net speculator positioning vs history
//   2. Economic Surprise    (20%) - How data is beating/missing expectations
//   3. Rate Trajectory      (20%) - Interest rate expectations direction
//   4. Revision Momentum    (15%) - Are data releases being revised up or down
//   5. Crowd Sentiment      (10%) - Contrarian signal from retail positioning
//   6. Event Risk Premium   (10%) - Upcoming high-impact event density
type ConfluenceScorer struct {
	eventRepo    ports.EventRepository
	cotRepo      ports.COTRepository
	surpriseRepo ports.SurpriseRepository
}

// NewConfluenceScorer creates a confluence scorer with all dependencies.
func NewConfluenceScorer(
	eventRepo ports.EventRepository,
	cotRepo ports.COTRepository,
	surpriseRepo ports.SurpriseRepository,
) *ConfluenceScorer {
	return &ConfluenceScorer{
		eventRepo:    eventRepo,
		cotRepo:      cotRepo,
		surpriseRepo: surpriseRepo,
	}
}

// ComputeForPair calculates the confluence score for a currency pair.
func (cs *ConfluenceScorer) ComputeForPair(ctx context.Context, base, quote string) (*domain.ConfluenceScore, error) {
	score := &domain.ConfluenceScore{
		CurrencyPair: base + quote,
		UpdatedAt:    timeutil.NowWIB(),
	}

	// Compute each factor
	factors := []domain.ConfluenceFactor{
		cs.computeCOTFactor(ctx, base, quote),
		cs.computeSurpriseFactor(ctx, base, quote),
		cs.computeRateFactor(ctx, base, quote),
		cs.computeRevisionFactor(ctx, base, quote),
		cs.computeCrowdFactor(ctx, base, quote),
		cs.computeEventRiskFactor(ctx, base, quote),
	}

	score.Factors = factors

	// Calculate total weighted score
	totalScore := 0.0
	for _, f := range factors {
		totalScore += f.WeightedScore
	}
	score.TotalScore = mathutil.Clamp(totalScore, 0, 100)

	// Determine direction
	switch {
	case score.TotalScore >= 65:
		score.Direction = "BULLISH"
	case score.TotalScore <= 35:
		score.Direction = "BEARISH"
	default:
		score.Direction = "NEUTRAL"
	}

	// Confidence based on factor agreement
	score.Confidence = computeConfidence(factors)

	// Save
	if err := cs.surpriseRepo.SaveConfluence(ctx, *score); err != nil {
		log.Printf("[confluence] warn: save: %v", err)
	}

	return score, nil
}

// ComputeAllMajorPairs calculates confluence for all major pairs.
func (cs *ConfluenceScorer) ComputeAllMajorPairs(ctx context.Context) ([]domain.ConfluenceScore, error) {
	pairs := []struct{ Base, Quote string }{
		{"EUR", "USD"}, {"GBP", "USD"}, {"USD", "JPY"},
		{"AUD", "USD"}, {"NZD", "USD"}, {"USD", "CAD"},
		{"USD", "CHF"}, {"EUR", "GBP"}, {"EUR", "JPY"},
		{"GBP", "JPY"}, {"AUD", "JPY"}, {"EUR", "AUD"},
	}

	var results []domain.ConfluenceScore
	for _, p := range pairs {
		score, err := cs.ComputeForPair(ctx, p.Base, p.Quote)
		if err != nil {
			log.Printf("[confluence] warn: %s%s: %v", p.Base, p.Quote, err)
			continue
		}
		results = append(results, *score)
	}

	log.Printf("[confluence] computed %d pair scores", len(results))
	return results, nil
}

// --- Factor computations ---

// Factor 1: COT Positioning (25%)
func (cs *ConfluenceScorer) computeCOTFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   "COT Positioning",
		Weight: 0.25,
	}

	// Get COT analysis for base and quote currencies
	baseContract := domain.CurrencyToContract(base)
	quoteContract := domain.CurrencyToContract(quote)

	baseScore := 50.0
	quoteScore := 50.0

	if baseContract != "" {
		if analysis, err := cs.cotRepo.GetLatestAnalysis(ctx, baseContract); err == nil && analysis != nil {
			baseScore = analysis.COTIndex
		}
	}
	if quoteContract != "" {
		if analysis, err := cs.cotRepo.GetLatestAnalysis(ctx, quoteContract); err == nil && analysis != nil {
			quoteScore = analysis.COTIndex
		}
	}

	// Differential: higher base COT vs quote = bullish for pair
	f.RawScore = mathutil.Clamp(50+(baseScore-quoteScore)/2, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 2: Economic Surprise (20%)
func (cs *ConfluenceScorer) computeSurpriseFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   "Economic Surprise",
		Weight: 0.20,
	}

	baseIdx, _ := cs.surpriseRepo.GetSurpriseIndex(ctx, base, 30)
	quoteIdx, _ := cs.surpriseRepo.GetSurpriseIndex(ctx, quote, 30)

	baseSurprise := 0.0
	quoteSurprise := 0.0
	if baseIdx != nil {
		baseSurprise = baseIdx.RollingScore
	}
	if quoteIdx != nil {
		quoteSurprise = quoteIdx.RollingScore
	}

	// Differential: base beating expectations more than quote = bullish
	diff := baseSurprise - quoteSurprise
	f.RawScore = mathutil.Clamp(50+diff*2, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 3: Interest Rate Trajectory (20%)
// Uses recent rate-sensitive event data as proxy for rate expectations.
func (cs *ConfluenceScorer) computeRateFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   "Rate Trajectory",
		Weight: 0.20,
	}

	// Look at rate-related events in last 30 days
	now := timeutil.NowWIB()
	start := now.AddDate(0, 0, -30)

	events, err := cs.eventRepo.GetEventsByDateRange(ctx, start, now)
	if err != nil {
		f.RawScore = 50
		f.WeightedScore = f.RawScore * f.Weight
		f.Signal = "NEUTRAL"
		return f
	}

	rateKeywords := []string{"rate", "interest", "monetary", "policy", "fed fund", "bank rate", "cash rate"}

	baseRateScore := computeRateScore(events, base, rateKeywords)
	quoteRateScore := computeRateScore(events, quote, rateKeywords)

	f.RawScore = mathutil.Clamp(50+(baseRateScore-quoteRateScore)*10, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 4: Revision Momentum (15%)
func (cs *ConfluenceScorer) computeRevisionFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   "Revision Momentum",
		Weight: 0.15,
	}

	baseRevs, _ := cs.eventRepo.GetRevisions(ctx, base, 30)
	quoteRevs, _ := cs.eventRepo.GetRevisions(ctx, quote, 30)

	baseRevScore := revisionDirectionScore(baseRevs)
	quoteRevScore := revisionDirectionScore(quoteRevs)

	f.RawScore = mathutil.Clamp(50+(baseRevScore-quoteRevScore)*25, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 5: Crowd Sentiment (10%) - Contrarian
func (cs *ConfluenceScorer) computeCrowdFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   "Crowd Sentiment",
		Weight: 0.10,
	}

	baseContract := domain.CurrencyToContract(base)
	quoteContract := domain.CurrencyToContract(quote)

	baseCrowd := 50.0
	quoteCrowd := 50.0

	if baseContract != "" {
		if analysis, err := cs.cotRepo.GetLatestAnalysis(ctx, baseContract); err == nil && analysis != nil {
			// Invert crowding: high crowd long = contrarian bearish
			baseCrowd = 100 - analysis.CrowdingIndex
		}
	}
	if quoteContract != "" {
		if analysis, err := cs.cotRepo.GetLatestAnalysis(ctx, quoteContract); err == nil && analysis != nil {
			quoteCrowd = 100 - analysis.CrowdingIndex
		}
	}

	f.RawScore = mathutil.Clamp(50+(baseCrowd-quoteCrowd)/2, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// Factor 6: Event Risk Premium (10%)
func (cs *ConfluenceScorer) computeEventRiskFactor(ctx context.Context, base, quote string) domain.ConfluenceFactor {
	f := domain.ConfluenceFactor{
		Name:   "Event Risk Premium",
		Weight: 0.10,
	}

	// Count upcoming high-impact events in next 7 days
	now := timeutil.NowWIB()
	end := now.AddDate(0, 0, 7)

	events, err := cs.eventRepo.GetEventsByDateRange(ctx, now, end)
	if err != nil {
		f.RawScore = 50
		f.WeightedScore = f.RawScore * f.Weight
		f.Signal = "NEUTRAL"
		return f
	}

	baseHighCount := 0
	quoteHighCount := 0
	for _, ev := range events {
		if ev.Impact != domain.ImpactHigh {
			continue
		}
		if ev.Currency == base {
			baseHighCount++
		}
		if ev.Currency == quote {
			quoteHighCount++
		}
	}

	// More events = more uncertainty = slight negative
	// But also opportunity for surprise
	riskDiff := float64(quoteHighCount - baseHighCount) // positive if quote has more risk
	f.RawScore = mathutil.Clamp(50+riskDiff*5, 0, 100)
	f.WeightedScore = f.RawScore * f.Weight
	f.Signal = classifyFactorSignal(f.RawScore)

	return f
}

// --- helpers ---

func classifyFactorSignal(rawScore float64) string {
	switch {
	case rawScore >= 70:
		return "STRONG_BULLISH"
	case rawScore >= 55:
		return "BULLISH"
	case rawScore <= 30:
		return "STRONG_BEARISH"
	case rawScore <= 45:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

func computeConfidence(factors []domain.ConfluenceFactor) float64 {
	// Confidence = how much factors agree
	bullish := 0
	bearish := 0
	for _, f := range factors {
		if strings.Contains(f.Signal, "BULLISH") {
			bullish++
		} else if strings.Contains(f.Signal, "BEARISH") {
			bearish++
		}
	}

	total := len(factors)
	if total == 0 {
		return 50
	}

	// Max agreement = max confidence
	maxAgree := bullish
	if bearish > maxAgree {
		maxAgree = bearish
	}

	return float64(maxAgree) / float64(total) * 100
}

func computeRateScore(events []domain.FFEvent, currency string, keywords []string) float64 {
	score := 0.0
	count := 0

	for _, ev := range events {
		if ev.Currency != currency || ev.Actual == "" {
			continue
		}

		titleLower := strings.ToLower(ev.Title)
		isRate := false
		for _, kw := range keywords {
			if strings.Contains(titleLower, kw) {
				isRate = true
				break
			}
		}
		if !isRate {
			continue
		}

		actual := parseNumericValue(ev.Actual)
		previous := parseNumericValue(ev.Previous)
		if actual > previous {
			score += 1
		} else if actual < previous {
			score -= 1
		}
		count++
	}

	if count == 0 {
		return 0
	}
	return score / float64(count)
}

func revisionDirectionScore(revisions []domain.EventRevision) float64 {
	if len(revisions) == 0 {
		return 0
	}

	upward := 0
	downward := 0
	for _, rev := range revisions {
		switch rev.Direction {
		case "upward":
			upward++
		case "downward":
			downward++
		}
	}

	total := upward + downward
	if total == 0 {
		return 0
	}
	return float64(upward-downward) / float64(total)
}

// FormatConfluenceScore creates a Telegram-formatted confluence display.
func FormatConfluenceScore(score *domain.ConfluenceScore) string {
	if score == nil {
		return "No confluence data."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== %s CONFLUENCE ===\n", score.CurrencyPair))
	b.WriteString(fmt.Sprintf("Score: %s/100 | %s | Confidence: %.0f%%\n\n",
		fmtutil.FmtNum(score.TotalScore, 1), score.Direction, score.Confidence))

	for _, f := range score.Factors {
		bar := fmtutil.COTIndexBar(f.RawScore)
		b.WriteString(fmt.Sprintf("  %s (%.0f%%): %s %s [%s]\n",
			f.Name, f.Weight*100,
			fmtutil.FmtNum(f.RawScore, 1), bar, f.Signal))
	}

	return b.String()
}
