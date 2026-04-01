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
// Multi-Window Walk-Forward Analysis
// ---------------------------------------------------------------------------

// WFConfig defines a single walk-forward window configuration.
type WFConfig struct {
	TrainWeeks int
	TestWeeks  int
}

// MultiWFResult holds results from multiple walk-forward window configurations.
type MultiWFResult struct {
	Windows map[string]*WalkForwardResult `json:"windows"` // key = "13W/6W", "26W/13W", "52W/26W"
}

// defaultMultiConfigs defines the standard institutional-grade window sizes.
var defaultMultiConfigs = []WFConfig{
	{TrainWeeks: 13, TestWeeks: 6},
	{TrainWeeks: 26, TestWeeks: 13},
	{TrainWeeks: 52, TestWeeks: 26},
}

// AnalyzeMultiWindow runs walk-forward analysis across multiple train/test
// window configurations. This detects whether strategy robustness holds
// across different time horizons — a key institutional due-diligence check.
func AnalyzeMultiWindow(ctx context.Context, signalRepo ports.SignalRepository) (*MultiWFResult, error) {
	signals, err := signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals for multi-window WF: %w", err)
	}

	// Keep only signals with a definitive 1W outcome (WIN or LOSS).
	// Exclude PENDING and EXPIRED to avoid diluting walk-forward win rates.
	var evaluated []domain.PersistedSignal
	for i := range signals {
		if signals[i].Outcome1W == domain.OutcomeWin || signals[i].Outcome1W == domain.OutcomeLoss {
			evaluated = append(evaluated, signals[i])
		}
	}

	result := &MultiWFResult{
		Windows: make(map[string]*WalkForwardResult, len(defaultMultiConfigs)),
	}

	for _, cfg := range defaultMultiConfigs {
		key := fmt.Sprintf("%dW/%dW", cfg.TrainWeeks, cfg.TestWeeks)
		wfr := runWalkForward(evaluated, cfg.TrainWeeks, cfg.TestWeeks)
		result.Windows[key] = wfr
	}

	return result, nil
}

// runWalkForward executes walk-forward analysis with the given window sizes.
func runWalkForward(evaluated []domain.PersistedSignal, trainWeeks, testWeeks int) *WalkForwardResult {
	if len(evaluated) == 0 {
		return &WalkForwardResult{
			Recommendation: "No evaluated signals available for walk-forward analysis.",
		}
	}

	// Sort by report date ascending.
	sort.Slice(evaluated, func(i, j int) bool {
		return evaluated[i].ReportDate.Before(evaluated[j].ReportDate)
	})

	// Defensive guard at point-of-use.
	if len(evaluated) == 0 {
		return &WalkForwardResult{
			Recommendation: "No evaluated signals available for walk-forward analysis.",
		}
	}

	earliest := evaluated[0].ReportDate
	latest := evaluated[len(evaluated)-1].ReportDate

	trainDur := time.Duration(trainWeeks) * 7 * 24 * time.Hour
	testDur := time.Duration(testWeeks) * 7 * 24 * time.Hour
	stepDur := testDur // roll forward by test window size

	var windows []WindowResult
	totalInWins, totalInCount := 0, 0
	totalOutWins, totalOutCount := 0, 0

	for winStart := earliest; ; winStart = winStart.Add(stepDur) {
		trainEnd := winStart.Add(trainDur)
		testStart := trainEnd
		testEnd := testStart.Add(testDur)

		if testStart.After(latest) {
			break
		}

		inWins, inCount := countWinRate(evaluated, winStart, trainEnd)
		outWins, outCount := countWinRate(evaluated, testStart, testEnd)

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

	return result
}
