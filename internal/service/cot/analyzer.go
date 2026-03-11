package cot

import (
	"context"
	"fmt"
	"log"
	"math"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/pkg/mathutil"
)

// Analyzer computes all 20+ COT metrics from raw positioning data.
// It processes historical records to derive net positions, ratios,
// momentum, sentiment, concentration, and generates trading signals.
type Analyzer struct {
	cotRepo ports.COTRepository
	fetcher *Fetcher
}

// NewAnalyzer creates a COT analyzer.
func NewAnalyzer(repo ports.COTRepository, fetcher *Fetcher) *Analyzer {
	return &Analyzer{
		cotRepo: repo,
		fetcher: fetcher,
	}
}

// AnalyzeAll fetches latest data, computes metrics for all contracts,
// and stores the results. Returns computed analyses.
func (a *Analyzer) AnalyzeAll(ctx context.Context) ([]domain.COTAnalysis, error) {
	contracts := domain.DefaultContracts()

	// Fetch latest records
	records, err := a.fetcher.FetchLatest(ctx, contracts)
	if err != nil {
		return nil, fmt.Errorf("fetch latest: %w", err)
	}

	// Save raw records
	if err := a.cotRepo.SaveRecords(ctx, records); err != nil {
		log.Printf("[cot] warn: save records: %v", err)
	}

	var analyses []domain.COTAnalysis

	for _, record := range records {
		// Get historical data for index calculation (52 weeks)
		history, err := a.cotRepo.GetHistory(ctx, record.ContractCode, 52)
		if err != nil {
			log.Printf("[cot] warn: get history for %s: %v", record.ContractCode, err)
			history = []domain.COTRecord{record} // use just current if no history
		}

		analysis := a.computeMetrics(record, history)
		analyses = append(analyses, analysis)
	}

	// Save analyses
	if err := a.cotRepo.SaveAnalyses(ctx, analyses); err != nil {
		return analyses, fmt.Errorf("save analyses: %w", err)
	}

	log.Printf("[cot] analyzed %d contracts", len(analyses))
	return analyses, nil
}

// AnalyzeContract computes metrics for a single contract.
func (a *Analyzer) AnalyzeContract(ctx context.Context, contractCode string) (*domain.COTAnalysis, error) {
	latest, err := a.cotRepo.GetLatest(ctx, contractCode)
	if err != nil {
		return nil, fmt.Errorf("get latest: %w", err)
	}

	history, err := a.cotRepo.GetHistory(ctx, contractCode, 52)
	if err != nil {
		history = []domain.COTRecord{*latest}
	}

	analysis := a.computeMetrics(*latest, history)
	return &analysis, nil
}

// computeMetrics calculates all 20+ COT metrics from a record + history.
func (a *Analyzer) computeMetrics(current domain.COTRecord, history []domain.COTRecord) domain.COTAnalysis {
	analysis := domain.COTAnalysis{
		ContractCode: current.ContractCode,
		Currency:     current.Currency,
		ReportDate:   current.ReportDate,
	}

	// === Core Position Metrics ===

	// 1. Net positions
	analysis.SpecNetPosition = current.SpecLong - current.SpecShort
	analysis.CommNetPosition = current.CommLong - current.CommShort
	analysis.SmallNetPosition = current.SmallLong - current.SmallShort

	// 2. Net change (week-over-week)
	analysis.SpecNetChange = current.SpecLongChg - current.SpecShortChg
	analysis.CommNetChange = current.CommLongChg - current.CommShortChg
	analysis.SmallNetChange = current.SmallLongChg - current.SmallShortChg

	// 3. Long/Short ratios
	analysis.SpecLongShortRatio = safeRatio(float64(current.SpecLong), float64(current.SpecShort))
	analysis.CommLongShortRatio = safeRatio(float64(current.CommLong), float64(current.CommShort))

	// 4. Percentage of Open Interest
	if current.OpenInterest > 0 {
		oi := float64(current.OpenInterest)
		analysis.SpecPctOfOI = float64(analysis.SpecNetPosition) / oi * 100
		analysis.CommPctOfOI = float64(analysis.CommNetPosition) / oi * 100
	}

	// 5. Open Interest change %
	if len(history) > 1 {
		prevOI := history[1].OpenInterest // history[0] = current
		if prevOI > 0 {
			analysis.OIPctChange = float64(current.OpenInterest-prevOI) / float64(prevOI) * 100
		}
	}

	// === Index Metrics (Larry Williams COT Index) ===

	if len(history) >= 3 {
		// 6. COT Index for speculators (0-100)
		specNets := extractNets(history, func(r domain.COTRecord) int64 { return r.SpecLong - r.SpecShort })
		analysis.COTIndex = computeCOTIndex(specNets)

		// 7. COT Index for commercials
		commNets := extractNets(history, func(r domain.COTRecord) int64 { return r.CommLong - r.CommShort })
		analysis.COTIndexComm = computeCOTIndex(commNets)
	}

	// === Momentum Metrics ===

	if len(history) >= 4 {
		// 8. Speculator momentum (4-week)
		specNets4 := extractNetsFloat(history[:min(5, len(history))], func(r domain.COTRecord) float64 {
			return float64(r.SpecLong - r.SpecShort)
		})
		analysis.SpecMomentum4W = mathutil.Momentum(specNets4, 4)

		// 9. Commercial momentum (4-week)
		commNets4 := extractNetsFloat(history[:min(5, len(history))], func(r domain.COTRecord) float64 {
			return float64(r.CommLong - r.CommShort)
		})
		analysis.CommMomentum4W = mathutil.Momentum(commNets4, 4)

		// 10. Momentum direction
		analysis.MomentumDir = classifyMomentum(analysis.SpecMomentum4W, analysis.CommMomentum4W)
	}

	// === Sentiment & Signal Metrics ===

	// 11. Overall sentiment score (-100 to +100)
	analysis.SentimentScore = computeSentiment(analysis)

	// 12-14. Individual trader signals
	analysis.CommercialSignal = classifySignal(analysis.COTIndexComm, analysis.CommMomentum4W, true)
	analysis.SpeculatorSignal = classifySignal(analysis.COTIndex, analysis.SpecMomentum4W, false)
	analysis.SmallSpecSignal = classifySmallSpec(analysis)

	// === Advanced Metrics ===

	// 15. Divergence flag (commercials vs speculators moving opposite)
	analysis.DivergenceFlag = detectDivergence(analysis)

	// 16. Crowding index (how one-sided is positioning)
	analysis.CrowdingIndex = computeCrowding(current)

	// 17-18. Concentration (top 4/8 traders)
	analysis.Top4Concentration = (current.Top4Long + current.Top4Short) / 2
	analysis.Top8Concentration = (current.Top8Long + current.Top8Short) / 2

	// === Extreme Detection ===

	// 19. Z-Score for extreme positioning
	if len(history) >= 10 {
		specNetsAll := extractNetsFloat(history, func(r domain.COTRecord) float64 {
			return float64(r.SpecLong - r.SpecShort)
		})
		analysis.SpecZScore = mathutil.ZScore(specNetsAll)
	}

	// 20. Percentile ranking
	if len(history) >= 10 {
		specNetsAll := extractNetsFloat(history, func(r domain.COTRecord) float64 {
			return float64(r.SpecLong - r.SpecShort)
		})
		analysis.SpecPercentile = mathutil.Percentile(specNetsAll, float64(analysis.SpecNetPosition))
	}

	return analysis
}

// --- computation helpers ---

// computeCOTIndex implements the Larry Williams COT Index formula:
// Index = (Current Net - Min Net) / (Max Net - Min Net) * 100
func computeCOTIndex(nets []int64) float64 {
	if len(nets) < 3 {
		return 50.0 // neutral if insufficient data
	}

	current := float64(nets[0])
	minVal, maxVal := float64(nets[0]), float64(nets[0])

	for _, n := range nets {
		v := float64(n)
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	span := maxVal - minVal
	if span == 0 {
		return 50.0
	}

	return mathutil.Clamp((current-minVal)/span*100, 0, 100)
}

// computeSentiment creates a composite score from all factors.
func computeSentiment(a domain.COTAnalysis) float64 {
	score := 0.0

	// COT Index contribution (40% weight)
	// Index > 80 = bullish, < 20 = bearish
	indexScore := (a.COTIndex - 50) * 2 // scale to -100..+100
	score += indexScore * 0.40

	// Commercial positioning (30% weight)
	// Commercials are contrarian: high index = bearish for price (they're hedging)
	commScore := (50 - a.COTIndexComm) * 2
	score += commScore * 0.30

	// Momentum contribution (20% weight)
	if a.SpecMomentum4W > 0 {
		score += 20
	} else if a.SpecMomentum4W < 0 {
		score -= 20
	}

	// Crowding penalty (10% weight)
	// High crowding = contrarian signal
	if a.CrowdingIndex > 70 {
		score -= 10
	} else if a.CrowdingIndex < 30 {
		score += 10
	}

	return mathutil.Clamp(score, -100, 100)
}

// classifySignal generates a directional signal from COT index + momentum.
func classifySignal(cotIndex, momentum float64, isCommercial bool) string {
	// Commercial signals are inverted (hedgers are contrarian)
	threshHigh := 75.0
	threshLow := 25.0
	if isCommercial {
		threshHigh, threshLow = threshLow, threshHigh
	}

	switch {
	case cotIndex >= threshHigh && momentum > 0:
		if isCommercial {
			return "STRONG_BEARISH"
		}
		return "STRONG_BULLISH"
	case cotIndex >= threshHigh:
		if isCommercial {
			return "BEARISH"
		}
		return "BULLISH"
	case cotIndex <= threshLow && momentum < 0:
		if isCommercial {
			return "STRONG_BULLISH"
		}
		return "STRONG_BEARISH"
	case cotIndex <= threshLow:
		if isCommercial {
			return "BULLISH"
		}
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}

// classifySmallSpec generates small speculator signal (contrarian indicator).
func classifySmallSpec(a domain.COTAnalysis) string {
	if a.SmallNetPosition > 0 && a.CrowdingIndex > 65 {
		return "CROWD_LONG" // potential contrarian sell
	}
	if a.SmallNetPosition < 0 && a.CrowdingIndex > 65 {
		return "CROWD_SHORT" // potential contrarian buy
	}
	return "NEUTRAL"
}

// classifyMomentum determines the overall positioning momentum direction.
func classifyMomentum(specMom, commMom float64) string {
	if specMom > 0 && commMom < 0 {
		return "SPEC_BULLISH_COMM_BEARISH" // classic bullish setup
	}
	if specMom < 0 && commMom > 0 {
		return "SPEC_BEARISH_COMM_BULLISH" // classic bearish setup
	}
	if specMom > 0 && commMom > 0 {
		return "BOTH_ADDING" // increasing open interest
	}
	if specMom < 0 && commMom < 0 {
		return "BOTH_REDUCING" // decreasing open interest
	}
	return "MIXED"
}

// detectDivergence checks if commercials and speculators are moving in opposite directions.
func detectDivergence(a domain.COTAnalysis) bool {
	// Divergence: spec adding longs while comm adding shorts (or vice versa)
	specDir := sign(a.SpecNetChange)
	commDir := sign(a.CommNetChange)

	// Only flag if both are significant moves
	if math.Abs(float64(a.SpecNetChange)) < 1000 || math.Abs(float64(a.CommNetChange)) < 1000 {
		return false
	}

	return specDir != 0 && commDir != 0 && specDir != commDir
}

// computeCrowding measures how one-sided positioning is (0-100).
func computeCrowding(r domain.COTRecord) float64 {
	totalLong := float64(r.SpecLong + r.CommLong + r.SmallLong)
	totalShort := float64(r.SpecShort + r.CommShort + r.SmallShort)
	total := totalLong + totalShort

	if total == 0 {
		return 50.0
	}

	// Crowding = how far from 50/50 balance
	longPct := totalLong / total * 100
	deviation := math.Abs(longPct - 50)

	// Scale: 0% deviation = 0 crowding, 50% deviation = 100 crowding
	return mathutil.Clamp(deviation*2, 0, 100)
}

// --- utility helpers ---

func extractNets(history []domain.COTRecord, fn func(domain.COTRecord) int64) []int64 {
	nets := make([]int64, len(history))
	for i, r := range history {
		nets[i] = fn(r)
	}
	return nets
}

func extractNetsFloat(history []domain.COTRecord, fn func(domain.COTRecord) float64) []float64 {
	nets := make([]float64, len(history))
	for i, r := range history {
		nets[i] = fn(r)
	}
	return nets
}

func safeRatio(a, b float64) float64 {
	if b == 0 {
		if a > 0 {
			return 999.99
		}
		return 0
	}
	return math.Round(a/b*100) / 100
}

func sign(v int64) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
