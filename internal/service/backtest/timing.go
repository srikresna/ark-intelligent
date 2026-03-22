package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Signal Timing Analysis — optimal horizon per signal type
// ---------------------------------------------------------------------------

// HorizonStat holds performance metrics for a single time horizon.
type HorizonStat struct {
	Horizon        string  // "+1W", "+2W", "+4W"
	Evaluated      int     // Number of evaluated signals at this horizon
	WinRate        float64 // Win rate as percentage (0-100)
	AvgReturn      float64 // Average return at this horizon (%)
	MaxDrawdown    float64 // Worst single-trade loss at this horizon (%)
	RiskRewardRatio float64 // Avg win / |Avg loss|
}

// SignalTimingAnalysis holds per-signal-type timing breakdown.
type SignalTimingAnalysis struct {
	SignalType     string
	HorizonStats   []HorizonStat // 1W, 2W, 4W
	OptimalHorizon string        // "+1W", "+2W", or "+4W"
	Recommendation string        // Human-readable recommendation
	Degrading      bool          // True if signal degrades rapidly beyond optimal
}

// TimingAnalyzer computes optimal time horizons for each signal type.
type TimingAnalyzer struct {
	signalRepo ports.SignalRepository
}

// NewTimingAnalyzer creates a new TimingAnalyzer.
func NewTimingAnalyzer(signalRepo ports.SignalRepository) *TimingAnalyzer {
	return &TimingAnalyzer{signalRepo: signalRepo}
}

// Analyze fetches all signals and returns timing analysis grouped by type.
func (ta *TimingAnalyzer) Analyze(ctx context.Context) ([]SignalTimingAnalysis, error) {
	signals, err := ta.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals for timing analysis: %w", err)
	}
	return AnalyzeSignalTiming(signals), nil
}

// AnalyzeSignalTiming groups signals by type and computes per-horizon stats.
func AnalyzeSignalTiming(signals []domain.PersistedSignal) []SignalTimingAnalysis {
	// Group by signal type.
	grouped := make(map[string][]domain.PersistedSignal)
	for _, s := range signals {
		grouped[s.SignalType] = append(grouped[s.SignalType], s)
	}

	// Sort type names for deterministic output.
	types := make([]string, 0, len(grouped))
	for t := range grouped {
		types = append(types, t)
	}
	sort.Strings(types)

	results := make([]SignalTimingAnalysis, 0, len(types))
	for _, sigType := range types {
		analysis := analyzeType(sigType, grouped[sigType])
		results = append(results, analysis)
	}
	return results
}

// analyzeType computes timing analysis for a single signal type.
func analyzeType(sigType string, signals []domain.PersistedSignal) SignalTimingAnalysis {
	hs1W := computeHorizonStat("+1W", signals, func(s *domain.PersistedSignal) (string, float64) {
		return s.Outcome1W, s.Return1W
	})
	hs2W := computeHorizonStat("+2W", signals, func(s *domain.PersistedSignal) (string, float64) {
		return s.Outcome2W, s.Return2W
	})
	hs4W := computeHorizonStat("+4W", signals, func(s *domain.PersistedSignal) (string, float64) {
		return s.Outcome4W, s.Return4W
	})

	horizons := []HorizonStat{hs1W, hs2W, hs4W}

	// Find optimal: highest win rate among horizons with positive expected value (avg return > 0).
	optimal := pickOptimalHorizon(horizons)
	degrading := isDegrading(horizons, optimal)
	rec := buildRecommendation(optimal, horizons, degrading)

	return SignalTimingAnalysis{
		SignalType:     sigType,
		HorizonStats:   horizons,
		OptimalHorizon: optimal,
		Recommendation: rec,
		Degrading:      degrading,
	}
}

// horizonExtractor returns the outcome and return for a given horizon.
type horizonExtractor func(s *domain.PersistedSignal) (outcome string, ret float64)

// computeHorizonStat computes stats for a single horizon across signals.
func computeHorizonStat(label string, signals []domain.PersistedSignal, extract horizonExtractor) HorizonStat {
	var (
		wins, evaluated int
		sumReturn       float64
		sumWinReturn    float64
		sumLossReturn   float64
		winCount        int
		lossCount       int
		worstLoss       float64
	)

	for i := range signals {
		outcome, ret := extract(&signals[i])
		if outcome == "" || outcome == domain.OutcomePending {
			continue
		}
		evaluated++
		sumReturn += ret
		if outcome == domain.OutcomeWin {
			wins++
			sumWinReturn += ret
			winCount++
		} else {
			sumLossReturn += ret
			lossCount++
			if ret < worstLoss {
				worstLoss = ret
			}
		}
	}

	hs := HorizonStat{
		Horizon:   label,
		Evaluated: evaluated,
	}

	if evaluated > 0 {
		hs.WinRate = round2(float64(wins) / float64(evaluated) * 100)
		hs.AvgReturn = round4(sumReturn / float64(evaluated))
		hs.MaxDrawdown = round2(math.Abs(worstLoss))
	}

	if winCount > 0 && lossCount > 0 {
		avgWin := sumWinReturn / float64(winCount)
		avgLoss := math.Abs(sumLossReturn / float64(lossCount))
		if avgLoss > 0 {
			hs.RiskRewardRatio = round2(avgWin / avgLoss)
		}
	}

	return hs
}

// pickOptimalHorizon selects the horizon with the highest win rate among those
// with a positive average return. Falls back to the highest win rate if none
// have positive returns.
func pickOptimalHorizon(horizons []HorizonStat) string {
	best := ""
	bestWR := -1.0

	// First pass: only consider horizons with positive expected value and data.
	for _, h := range horizons {
		if h.Evaluated == 0 {
			continue
		}
		if h.AvgReturn > 0 && h.WinRate > bestWR {
			bestWR = h.WinRate
			best = h.Horizon
		}
	}

	// Fallback: if none have positive returns, pick highest win rate.
	if best == "" {
		for _, h := range horizons {
			if h.Evaluated == 0 {
				continue
			}
			if h.WinRate > bestWR {
				bestWR = h.WinRate
				best = h.Horizon
			}
		}
	}

	// Final fallback.
	if best == "" {
		best = "+1W"
	}
	return best
}

// isDegrading checks whether performance drops significantly beyond the optimal horizon.
// A signal is "degrading" if the win rate drops by >10pp from optimal to the next horizon.
func isDegrading(horizons []HorizonStat, optimal string) bool {
	optIdx := horizonIndex(optimal)
	if optIdx < 0 || optIdx >= len(horizons)-1 {
		return false // optimal is the last horizon or not found
	}

	optWR := horizons[optIdx].WinRate
	for i := optIdx + 1; i < len(horizons); i++ {
		if horizons[i].Evaluated == 0 {
			continue
		}
		if optWR-horizons[i].WinRate > 10 {
			return true
		}
	}
	return false
}

// horizonIndex maps a horizon label to its position in the slice.
func horizonIndex(h string) int {
	switch h {
	case "+1W":
		return 0
	case "+2W":
		return 1
	case "+4W":
		return 2
	default:
		return -1
	}
}

// buildRecommendation generates a human-readable recommendation string.
func buildRecommendation(optimal string, horizons []HorizonStat, degrading bool) string {
	idx := horizonIndex(optimal)
	if idx < 0 || idx >= len(horizons) {
		return "Insufficient data"
	}

	h := horizons[idx]
	if h.Evaluated == 0 {
		return "No evaluated signals yet"
	}

	rec := fmt.Sprintf("Sweet spot: %s (%.0f%% win rate", optimal, h.WinRate)
	if h.RiskRewardRatio > 0 {
		rec += fmt.Sprintf(", R:R %.1f", h.RiskRewardRatio)
	}
	rec += ")"

	if degrading {
		rec += " — degrades beyond this horizon"
	}

	return rec
}
