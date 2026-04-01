package backtest

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Walk-Forward Optimization — train/test split to detect overfitting
// ---------------------------------------------------------------------------

const (
	// Default window sizes in weeks.
	defaultTrainWeeks = 26
	defaultTestWeeks  = 13
	// Overfit threshold: if in-sample minus out-of-sample > 10pp, flag it.
	overfitThresholdPP = 10.0
)

// WindowResult holds performance for a single walk-forward window.
type WindowResult struct {
	TrainStart time.Time
	TrainEnd   time.Time
	TestStart  time.Time
	TestEnd    time.Time

	InSampleWinRate    float64 // Train set win rate (0-100%)
	OutOfSampleWinRate float64 // Test set win rate (0-100%)
	InSampleCount      int     // Number of evaluated signals in train set
	OutOfSampleCount   int     // Number of evaluated signals in test set
	Degradation        float64 // InSampleWinRate - OutOfSampleWinRate (pp)
}

// WalkForwardResult holds the full walk-forward analysis output.
type WalkForwardResult struct {
	Windows []WindowResult

	OverallInSampleWinRate    float64 // Aggregate in-sample win rate (0-100%)
	OverallOutOfSampleWinRate float64 // Aggregate out-of-sample win rate (0-100%)
	OverfitScore              float64 // In-sample minus out-of-sample (pp)
	IsOverfit                 bool    // OverfitScore > overfitThresholdPP
	Recommendation            string  // Human-readable verdict
}

// WalkForwardAnalyzer performs walk-forward analysis on persisted signals.
type WalkForwardAnalyzer struct {
	signalRepo ports.SignalRepository
}

// NewWalkForwardAnalyzer creates a new WalkForwardAnalyzer.
func NewWalkForwardAnalyzer(signalRepo ports.SignalRepository) *WalkForwardAnalyzer {
	return &WalkForwardAnalyzer{signalRepo: signalRepo}
}

// Analyze runs walk-forward optimization across all evaluated signals.
// It splits signals into rolling windows of trainWeeks training and testWeeks
// testing, advancing by testWeeks each step.
func (wfa *WalkForwardAnalyzer) Analyze(ctx context.Context) (*WalkForwardResult, error) {
	signals, err := wfa.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals for walk-forward: %w", err)
	}

	// Keep only signals with a definitive 1W outcome (WIN or LOSS).
	// Exclude PENDING and EXPIRED — expired signals have no real outcome data
	// and would dilute win rates if included.
	var evaluated []domain.PersistedSignal
	for i := range signals {
		if signals[i].Outcome1W == domain.OutcomeWin || signals[i].Outcome1W == domain.OutcomeLoss {
			evaluated = append(evaluated, signals[i])
		}
	}

	if len(evaluated) == 0 {
		return &WalkForwardResult{
			Recommendation: "No evaluated signals available for walk-forward analysis.",
		}, nil
	}

	// Sort by report date ascending.
	sort.Slice(evaluated, func(i, j int) bool {
		return evaluated[i].ReportDate.Before(evaluated[j].ReportDate)
	})

	// Defensive guard: re-check length at point-of-use in case future
	// refactoring adds filtering between the initial check and here.
	if len(evaluated) == 0 {
		return &WalkForwardResult{
			Recommendation: "No evaluated signals available for walk-forward analysis.",
		}, nil
	}

	earliest := evaluated[0].ReportDate
	latest := evaluated[len(evaluated)-1].ReportDate

	trainDur := time.Duration(defaultTrainWeeks) * 7 * 24 * time.Hour
	testDur := time.Duration(defaultTestWeeks) * 7 * 24 * time.Hour
	stepDur := testDur // roll forward by test window size

	var windows []WindowResult
	totalInWins, totalInCount := 0, 0
	totalOutWins, totalOutCount := 0, 0

	// Slide the window from earliest to latest.
	for winStart := earliest; ; winStart = winStart.Add(stepDur) {
		trainEnd := winStart.Add(trainDur)
		testStart := trainEnd
		testEnd := testStart.Add(testDur)

		// Stop if the test window extends beyond available data.
		if testStart.After(latest) {
			break
		}

		inWins, inCount := countWinRate(evaluated, winStart, trainEnd)
		outWins, outCount := countWinRate(evaluated, testStart, testEnd)

		// Skip windows with no data on either side.
		if inCount == 0 || outCount == 0 {
			continue
		}

		inWR := round2(float64(inWins) / float64(inCount) * 100)
		outWR := round2(float64(outWins) / float64(outCount) * 100)

		windows = append(windows, WindowResult{
			TrainStart:         winStart,
			TrainEnd:           trainEnd,
			TestStart:          testStart,
			TestEnd:            testEnd,
			InSampleWinRate:    inWR,
			OutOfSampleWinRate: outWR,
			InSampleCount:      inCount,
			OutOfSampleCount:   outCount,
			Degradation:        round2(inWR - outWR),
		})

		totalInWins += inWins
		totalInCount += inCount
		totalOutWins += outWins
		totalOutCount += outCount
	}

	result := &WalkForwardResult{
		Windows: windows,
	}

	if totalInCount > 0 {
		result.OverallInSampleWinRate = round2(float64(totalInWins) / float64(totalInCount) * 100)
	}
	if totalOutCount > 0 {
		result.OverallOutOfSampleWinRate = round2(float64(totalOutWins) / float64(totalOutCount) * 100)
	}

	result.OverfitScore = round2(result.OverallInSampleWinRate - result.OverallOutOfSampleWinRate)
	result.IsOverfit = result.OverfitScore > overfitThresholdPP
	result.Recommendation = buildWalkForwardRecommendation(result)

	return result, nil
}

// countWinRate counts wins and total evaluated (1W) signals within [start, end).
func countWinRate(signals []domain.PersistedSignal, start, end time.Time) (wins, total int) {
	for i := range signals {
		rd := signals[i].ReportDate
		if (rd.Equal(start) || rd.After(start)) && rd.Before(end) {
			total++
			if signals[i].Outcome1W == domain.OutcomeWin {
				wins++
			}
		}
	}
	return
}

// buildWalkForwardRecommendation generates a human-readable recommendation.
func buildWalkForwardRecommendation(r *WalkForwardResult) string {
	if len(r.Windows) == 0 {
		return "Insufficient data for walk-forward analysis. Need at least 39 weeks of evaluated signals."
	}

	if r.IsOverfit {
		return fmt.Sprintf(
			"Overfitting detected: in-sample outperforms out-of-sample by %.1fpp. "+
				"Strategy edge may not persist in live trading. "+
				"Consider simplifying signal logic or requiring higher conviction thresholds.",
			r.OverfitScore,
		)
	}

	if r.OverfitScore < 3 {
		return fmt.Sprintf(
			"Robust performance: only %.1fpp gap between train and test. "+
				"Signal edge appears genuine and likely to persist.",
			r.OverfitScore,
		)
	}

	return fmt.Sprintf(
		"Moderate gap of %.1fpp between train and test. "+
			"Edge likely real but some degradation expected in live conditions. "+
			"Monitor out-of-sample performance closely.",
		r.OverfitScore,
	)
}
