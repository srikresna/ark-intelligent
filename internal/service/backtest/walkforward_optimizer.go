package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
)

// ---------------------------------------------------------------------------
// Walk-Forward Optimization — Rolling Train/Test Weight Auto-Tuning
// ---------------------------------------------------------------------------
//
// Extends the existing walk-forward analyzer with automatic weight optimization:
//   1. Rolling 26-week train → 4-week test windows
//   2. On each train window, run OLS to find optimal factor weights
//   3. Evaluate with those weights on the test window
//   4. Track per-regime optimal weights (Trending vs Ranging vs Crisis)
//   5. Output: recommended weights per regime + stability metrics
//
// This adapts to changing market conditions without manual tuning.

const (
	wfoTrainWeeks = 26
	wfoTestWeeks  = 4
	wfoMinTrain   = 15 // Minimum signals in a train window
	wfoMinTest    = 5  // Minimum signals in a test window
)

// WFOResult holds the full walk-forward optimization output.
type WFOResult struct {
	Windows []WFOWindowResult `json:"windows"`

	// Aggregate optimal weights across all windows
	AggregateWeights map[string]float64 `json:"aggregate_weights"`

	// Per-regime optimal weights
	RegimeWeights map[string]map[string]float64 `json:"regime_weights,omitempty"`

	// Stability metrics
	WeightStability float64 `json:"weight_stability"` // 0-100, higher = more stable
	AvgOOSWinRate   float64 `json:"avg_oos_win_rate"` // Average out-of-sample win rate
	AvgOOSReturn    float64 `json:"avg_oos_return"`   // Average out-of-sample return
	TotalWindows    int     `json:"total_windows"`
	ValidWindows    int     `json:"valid_windows"`

	// Improvement over static weights
	StaticOOSWinRate   float64 `json:"static_oos_win_rate"`   // OOS win rate with static V3 weights
	AdaptiveOOSWinRate float64 `json:"adaptive_oos_win_rate"` // OOS win rate with adaptive weights
	Improvement        float64 `json:"improvement"`            // Adaptive - Static (pp)

	Recommendation string `json:"recommendation"`
}

// WFOWindowResult holds results for a single walk-forward window.
type WFOWindowResult struct {
	TrainStart time.Time          `json:"train_start"`
	TrainEnd   time.Time          `json:"train_end"`
	TestStart  time.Time          `json:"test_start"`
	TestEnd    time.Time          `json:"test_end"`
	TrainCount int                `json:"train_count"`
	TestCount  int                `json:"test_count"`
	Weights    map[string]float64 `json:"weights"`     // Optimized weights from train
	OOSWinRate float64            `json:"oos_win_rate"` // Out-of-sample win rate (%)
	OOSReturn  float64            `json:"oos_return"`   // Average out-of-sample return
	Regime     string             `json:"regime"`        // Dominant regime in train window
}

// WalkForwardOptimizer performs rolling weight optimization.
type WalkForwardOptimizer struct {
	signalRepo ports.SignalRepository
}

// NewWalkForwardOptimizer creates a new optimizer.
func NewWalkForwardOptimizer(signalRepo ports.SignalRepository) *WalkForwardOptimizer {
	return &WalkForwardOptimizer{signalRepo: signalRepo}
}

// Optimize runs the walk-forward optimization pipeline.
func (wfo *WalkForwardOptimizer) Optimize(ctx context.Context) (*WFOResult, error) {
	signals, err := wfo.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get signals: %w", err)
	}

	// Filter to evaluated signals with returns
	var evaluated []domain.PersistedSignal
	for _, s := range signals {
		if (s.Outcome1W == domain.OutcomeWin || s.Outcome1W == domain.OutcomeLoss) && s.EntryPrice > 0 {
			evaluated = append(evaluated, s)
		}
	}

	if len(evaluated) < wfoMinTrain+wfoMinTest {
		return nil, fmt.Errorf("insufficient evaluated signals: need %d, got %d",
			wfoMinTrain+wfoMinTest, len(evaluated))
	}

	// Sort chronologically
	sort.Slice(evaluated, func(i, j int) bool {
		return evaluated[i].ReportDate.Before(evaluated[j].ReportDate)
	})

	// Defensive guard at point-of-use (filtering above may reduce slice).
	if len(evaluated) < wfoMinTrain+wfoMinTest {
		return nil, fmt.Errorf("insufficient evaluated signals after filtering: need %d, got %d",
			wfoMinTrain+wfoMinTest, len(evaluated))
	}

	earliest := evaluated[0].ReportDate
	latest := evaluated[len(evaluated)-1].ReportDate

	trainDur := time.Duration(wfoTrainWeeks) * 7 * 24 * time.Hour
	testDur := time.Duration(wfoTestWeeks) * 7 * 24 * time.Hour
	stepDur := testDur // Roll forward by test window

	var windows []WFOWindowResult
	// Track per-regime weight accumulations
	regimeWeightSums := make(map[string]map[string]float64)
	regimeWeightCounts := make(map[string]int)

	// Track adaptive vs static performance
	var adaptiveWins, adaptiveTotal int
	var staticWins, staticTotal int

	for winStart := earliest; ; winStart = winStart.Add(stepDur) {
		trainEnd := winStart.Add(trainDur)
		testStart := trainEnd
		testEnd := testStart.Add(testDur)

		if testStart.After(latest) {
			break
		}

		trainSigs := filterByDateRange(evaluated, winStart, trainEnd)
		testSigs := filterByDateRange(evaluated, testStart, testEnd)

		if len(trainSigs) < wfoMinTrain || len(testSigs) < wfoMinTest {
			continue
		}

		// Optimize weights on train set
		weights, err := optimizeWindowWeights(trainSigs)
		if err != nil {
			continue
		}

		// Detect dominant regime in train window
		regime := detectDominantRegime(trainSigs)

		// Evaluate on test set with optimized weights
		oosWinRate, oosReturn, adaptiveCorrect, adaptiveCount := evaluateWithWeights(testSigs, weights)

		// Evaluate with static weights for comparison
		_, _, staticCorrect, staticCount := evaluateWithWeights(testSigs, currentV3Weights)

		// Track adaptive vs static using actual counts (no rounding)
		adaptiveWins += adaptiveCorrect
		adaptiveTotal += adaptiveCount
		staticWins += staticCorrect
		staticTotal += staticCount

		win := WFOWindowResult{
			TrainStart: winStart,
			TrainEnd:   trainEnd,
			TestStart:  testStart,
			TestEnd:    testEnd,
			TrainCount: len(trainSigs),
			TestCount:  len(testSigs),
			Weights:    weights,
			OOSWinRate: roundN(oosWinRate, 2),
			OOSReturn:  roundN(oosReturn, 4),
			Regime:     regime,
		}
		windows = append(windows, win)

		// Accumulate per-regime weights
		if _, ok := regimeWeightSums[regime]; !ok {
			regimeWeightSums[regime] = make(map[string]float64)
		}
		for factor, w := range weights {
			regimeWeightSums[regime][factor] += w
		}
		regimeWeightCounts[regime]++
	}

	if len(windows) == 0 {
		return nil, fmt.Errorf("no valid walk-forward windows (need %d+ weeks of data)", wfoTrainWeeks+wfoTestWeeks)
	}

	result := &WFOResult{
		Windows:      windows,
		TotalWindows: len(windows),
		ValidWindows: len(windows),
	}

	// Compute aggregate weights (average across all windows)
	result.AggregateWeights = averageWeights(windows)

	// Compute per-regime weights
	result.RegimeWeights = make(map[string]map[string]float64)
	for regime, sums := range regimeWeightSums {
		count := float64(regimeWeightCounts[regime])
		regWeights := make(map[string]float64)
		for factor, sum := range sums {
			regWeights[factor] = roundN(sum/count, 2)
		}
		result.RegimeWeights[regime] = regWeights
	}

	// Compute stability (how consistent are weights across windows)
	result.WeightStability = computeWeightStability(windows)

	// Average OOS metrics
	var sumWR, sumRet float64
	for _, w := range windows {
		sumWR += w.OOSWinRate
		sumRet += w.OOSReturn
	}
	result.AvgOOSWinRate = roundN(sumWR/float64(len(windows)), 2)
	result.AvgOOSReturn = roundN(sumRet/float64(len(windows)), 4)

	// Adaptive vs static comparison
	if adaptiveTotal > 0 {
		result.AdaptiveOOSWinRate = roundN(float64(adaptiveWins)/float64(adaptiveTotal)*100, 2)
	}
	if staticTotal > 0 {
		result.StaticOOSWinRate = roundN(float64(staticWins)/float64(staticTotal)*100, 2)
	}
	result.Improvement = roundN(result.AdaptiveOOSWinRate-result.StaticOOSWinRate, 2)

	result.Recommendation = buildWFORecommendation(result)

	return result, nil
}

// --- Internal helpers ---

// filterByDateRange returns signals within [start, end).
func filterByDateRange(signals []domain.PersistedSignal, start, end time.Time) []domain.PersistedSignal {
	var result []domain.PersistedSignal
	for _, s := range signals {
		if (s.ReportDate.Equal(start) || s.ReportDate.After(start)) && s.ReportDate.Before(end) {
			result = append(result, s)
		}
	}
	return result
}

// optimizeWindowWeights runs OLS on a window of signals to find optimal weights.
func optimizeWindowWeights(signals []domain.PersistedSignal) (map[string]float64, error) {
	if len(signals) < len(factorNames)+1 {
		return nil, fmt.Errorf("too few signals: %d", len(signals))
	}

	X, y := buildDesignMatrix(signals)
	reg, err := mathutil.OLSRegression(X, y)
	if err != nil {
		return nil, err
	}

	return normalizeCoefficients(reg.Coefficients), nil
}

// evaluateWithWeights scores signals using given weights, selects those
// above a conviction threshold, and computes OOS win rate and return on the
// selected subset. This allows adaptive weights to produce different results
// from static weights by changing which signals are considered "high conviction".
func evaluateWithWeights(signals []domain.PersistedSignal, weights map[string]float64) (winRate, avgReturn float64, winCount, totalCount int) {
	if len(signals) == 0 {
		return 0, 0, 0, 0
	}

	// Score every signal with the given weights
	type scored struct {
		signal domain.PersistedSignal
		score  float64
	}
	scoredSigs := make([]scored, 0, len(signals))
	for _, s := range signals {
		factorScores := extractFactorScores(s)
		wScore := 0.0
		for i, name := range factorNames {
			wScore += factorScores[i] * weights[name] / 100.0
		}
		scoredSigs = append(scoredSigs, scored{signal: s, score: wScore})
	}

	// Sort by absolute score descending (highest conviction first)
	sort.Slice(scoredSigs, func(i, j int) bool {
		return math.Abs(scoredSigs[i].score) > math.Abs(scoredSigs[j].score)
	})

	// Select top 70% by conviction (filter low-conviction signals)
	selectN := len(scoredSigs) * 70 / 100
	if selectN < 3 {
		selectN = len(scoredSigs) // If too few, use all
	}
	selected := scoredSigs[:selectN]

	wins := 0
	totalReturn := 0.0
	for _, ss := range selected {
		// Check if weight-based direction agrees with signal direction
		// Positive score = model agrees with bullish, negative = agrees with bearish
		agrees := (ss.score >= 0 && ss.signal.Direction == "BULLISH") ||
			(ss.score < 0 && ss.signal.Direction == "BEARISH")

		if agrees && ss.signal.Outcome1W == domain.OutcomeWin {
			wins++
		}
		// Note: disagree + loss is an "avoided loss" — tracked separately, not counted as a win.
		totalReturn += ss.signal.Return1W
	}

	winRate = float64(wins) / float64(len(selected)) * 100
	avgReturn = totalReturn / float64(len(selected))
	return winRate, avgReturn, wins, len(selected)
}

// detectDominantRegime determines the most common FRED regime in a signal set.
func detectDominantRegime(signals []domain.PersistedSignal) string {
	counts := make(map[string]int)
	for _, s := range signals {
		regime := s.FREDRegime
		if regime == "" {
			regime = "NORMAL"
		}
		counts[regime]++
	}

	best := "NORMAL"
	bestCount := 0
	for regime, count := range counts {
		if count > bestCount {
			bestCount = count
			best = regime
		}
	}

	// Map to broader categories
	switch best {
	case "EXPANSION", "GOLDILOCKS":
		return "RISK_ON"
	case "STRESS", "RECESSION", "STAGFLATION":
		return "CRISIS"
	default:
		return "NORMAL"
	}
}

// averageWeights computes mean weights across all windows.
func averageWeights(windows []WFOWindowResult) map[string]float64 {
	sums := make(map[string]float64)
	for _, w := range windows {
		for factor, weight := range w.Weights {
			sums[factor] += weight
		}
	}
	result := make(map[string]float64)
	n := float64(len(windows))
	for factor, sum := range sums {
		result[factor] = roundN(sum/n, 2)
	}
	return result
}

// computeWeightStability measures how consistent weights are across windows.
// Returns 0-100 where 100 = perfectly stable.
func computeWeightStability(windows []WFOWindowResult) float64 {
	if len(windows) < 2 {
		return 100
	}

	// Compute coefficient of variation for each factor's weight across windows
	avgWeights := averageWeights(windows)
	totalCV := 0.0
	factorCount := 0

	for _, factor := range factorNames {
		avg := avgWeights[factor]
		if avg < 1 {
			continue // Skip near-zero factors
		}

		var sumSqDiff float64
		for _, w := range windows {
			diff := w.Weights[factor] - avg
			sumSqDiff += diff * diff
		}
		sd := math.Sqrt(sumSqDiff / float64(len(windows)))
		cv := sd / avg * 100 // Coefficient of variation as %
		totalCV += cv
		factorCount++
	}

	if factorCount == 0 {
		return 50
	}

	avgCV := totalCV / float64(factorCount)
	// Convert: low CV = high stability
	// CV of 0 → 100 stability, CV of 50+ → 0 stability
	stability := math.Max(0, 100-avgCV*2)
	return roundN(stability, 1)
}

// buildWFORecommendation generates a human-readable recommendation.
func buildWFORecommendation(r *WFOResult) string {
	if r.ValidWindows < 3 {
		return fmt.Sprintf(
			"Only %d valid windows. Need more data (≥%d weeks) for reliable walk-forward optimization.",
			r.ValidWindows, (wfoTrainWeeks+wfoTestWeeks)*3,
		)
	}

	if r.Improvement > 3 {
		return fmt.Sprintf(
			"Adaptive weights outperform static by %.1fpp OOS. Weight stability: %.0f%%. "+
				"Consider deploying adaptive weights. Retrain monthly.",
			r.Improvement, r.WeightStability,
		)
	}

	if r.Improvement < -3 {
		return fmt.Sprintf(
			"Static weights outperform adaptive by %.1fpp OOS. "+
				"Adaptive optimization may be overfitting to recent data. "+
				"Stick with static V3 weights.",
			-r.Improvement,
		)
	}

	return fmt.Sprintf(
		"Minimal difference between adaptive and static weights (%.1fpp). "+
			"Weight stability: %.0f%%. Current V3 weights are adequate. "+
			"Re-evaluate when sample size grows.",
		r.Improvement, r.WeightStability,
	)
}
