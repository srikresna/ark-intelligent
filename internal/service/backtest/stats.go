package backtest

import (
	"context"
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// StatsCalculator computes aggregate backtest statistics from persisted signals.
type StatsCalculator struct {
	signalRepo ports.SignalRepository
}

// NewStatsCalculator creates a new stats calculator.
func NewStatsCalculator(signalRepo ports.SignalRepository) *StatsCalculator {
	return &StatsCalculator{signalRepo: signalRepo}
}

// ComputeAll computes aggregate stats across all signals.
func (sc *StatsCalculator) ComputeAll(ctx context.Context) (*domain.BacktestStats, error) {
	signals, err := sc.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}
	return computeStats(signals, "ALL"), nil
}

// ComputeByContract computes stats for a specific contract.
func (sc *StatsCalculator) ComputeByContract(ctx context.Context, contractCode string) (*domain.BacktestStats, error) {
	signals, err := sc.signalRepo.GetSignalsByContract(ctx, contractCode)
	if err != nil {
		return nil, fmt.Errorf("get signals for %s: %w", contractCode, err)
	}
	return computeStats(signals, contractCode), nil
}

// ComputeBySignalType computes stats for a specific signal type.
func (sc *StatsCalculator) ComputeBySignalType(ctx context.Context, signalType string) (*domain.BacktestStats, error) {
	signals, err := sc.signalRepo.GetSignalsByType(ctx, signalType)
	if err != nil {
		return nil, fmt.Errorf("get signals for type %s: %w", signalType, err)
	}
	return computeStats(signals, signalType), nil
}

// ComputeAllByContract returns stats grouped by contract code.
func (sc *StatsCalculator) ComputeAllByContract(ctx context.Context) (map[string]*domain.BacktestStats, error) {
	signals, err := sc.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	grouped := make(map[string][]domain.PersistedSignal)
	for _, s := range signals {
		grouped[s.ContractCode] = append(grouped[s.ContractCode], s)
	}

	result := make(map[string]*domain.BacktestStats, len(grouped))
	for code, sigs := range grouped {
		label := code
		if len(sigs) > 0 && sigs[0].Currency != "" {
			label = sigs[0].Currency
		}
		result[code] = computeStats(sigs, label)
	}
	return result, nil
}

// ComputeAllBySignalType returns stats grouped by signal type.
func (sc *StatsCalculator) ComputeAllBySignalType(ctx context.Context) (map[string]*domain.BacktestStats, error) {
	signals, err := sc.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	grouped := make(map[string][]domain.PersistedSignal)
	for _, s := range signals {
		grouped[s.SignalType] = append(grouped[s.SignalType], s)
	}

	result := make(map[string]*domain.BacktestStats, len(grouped))
	for sigType, sigs := range grouped {
		result[sigType] = computeStats(sigs, sigType)
	}
	return result, nil
}

// computeStats calculates BacktestStats from a slice of signals.
func computeStats(signals []domain.PersistedSignal, label string) *domain.BacktestStats {
	stats := &domain.BacktestStats{
		GroupLabel:   label,
		TotalSignals: len(signals),
	}

	if len(signals) == 0 {
		return stats
	}

	// Accumulators
	var (
		wins1W, wins2W, wins4W       int
		eval1W, eval2W, eval4W       int
		sumReturn1W, sumReturn2W     float64
		sumReturn4W                  float64
		sumWinReturn1W               float64
		sumLossReturn1W              float64
		winCount1W, lossCount1W      int
		sumConfidenceEval            float64 // BUG-H2 fix: only count evaluated signals
		sumConfidenceAll             float64 // for reference (all signals including pending)
		highStrengthWins, highTotal  int
		lowStrengthWins, lowTotal    int
	)

	for _, s := range signals {
		sumConfidenceAll += s.Confidence

		// Strength breakdown (only count evaluated signals)
		if s.Outcome1W != "" && s.Outcome1W != domain.OutcomePending {
			if s.Strength >= 4 {
				highTotal++
				if s.Outcome1W == domain.OutcomeWin {
					highStrengthWins++
				}
			} else {
				lowTotal++
				if s.Outcome1W == domain.OutcomeWin {
					lowStrengthWins++
				}
			}
		}

		// 1W outcomes
		if s.Outcome1W != "" && s.Outcome1W != domain.OutcomePending {
			eval1W++
			sumConfidenceEval += s.Confidence // BUG-H2: accumulate confidence for evaluated-only population
			sumReturn1W += s.Return1W
			if s.Outcome1W == domain.OutcomeWin {
				wins1W++
				sumWinReturn1W += s.Return1W
				winCount1W++
			} else {
				sumLossReturn1W += s.Return1W
				lossCount1W++
			}
		}

		// 2W outcomes
		if s.Outcome2W != "" && s.Outcome2W != domain.OutcomePending {
			eval2W++
			sumReturn2W += s.Return2W
			if s.Outcome2W == domain.OutcomeWin {
				wins2W++
			}
		}

		// 4W outcomes
		if s.Outcome4W != "" && s.Outcome4W != domain.OutcomePending {
			eval4W++
			sumReturn4W += s.Return4W
			if s.Outcome4W == domain.OutcomeWin {
				wins4W++
			}
		}
	}

	// Per-horizon evaluation counts
	stats.Evaluated1W = eval1W
	stats.Evaluated2W = eval2W
	stats.Evaluated4W = eval4W
	stats.Evaluated = eval1W // Primary evaluation count (shortest horizon)

	// Win rates
	if eval1W > 0 {
		stats.WinRate1W = round2(float64(wins1W) / float64(eval1W) * 100)
		stats.AvgReturn1W = round4(sumReturn1W / float64(eval1W))
	}
	if eval2W > 0 {
		stats.WinRate2W = round2(float64(wins2W) / float64(eval2W) * 100)
		stats.AvgReturn2W = round4(sumReturn2W / float64(eval2W))
	}
	if eval4W > 0 {
		stats.WinRate4W = round2(float64(wins4W) / float64(eval4W) * 100)
		stats.AvgReturn4W = round4(sumReturn4W / float64(eval4W))
	}

	// Avg win/loss return at 1W
	if winCount1W > 0 {
		stats.AvgWinReturn1W = round4(sumWinReturn1W / float64(winCount1W))
	}
	if lossCount1W > 0 {
		stats.AvgLossReturn1W = round4(sumLossReturn1W / float64(lossCount1W))
	}

	// Best period
	stats.BestPeriod = "1W"
	stats.BestWinRate = stats.WinRate1W
	if stats.WinRate2W > stats.BestWinRate {
		stats.BestPeriod = "2W"
		stats.BestWinRate = stats.WinRate2W
	}
	if stats.WinRate4W > stats.BestWinRate {
		stats.BestPeriod = "4W"
		stats.BestWinRate = stats.WinRate4W
	}

	// Confidence calibration — BUG-H2 fix: use consistent evaluated-only population.
	// AvgConfidence is computed over evaluated signals only (same set as WinRate1W).
	// This prevents pending signals from diluting/inflating calibration metrics.
	// ActualAccuracy uses WinRate1W (shortest horizon) for consistency.
	stats.AvgConfidence = round2(sumConfidenceAll / float64(len(signals))) // kept for display
	if eval1W > 0 {
		evalAvgConf := round2(sumConfidenceEval / float64(eval1W))
		stats.ActualAccuracy = stats.WinRate1W
		stats.CalibrationError = round2(math.Abs(evalAvgConf - stats.ActualAccuracy))
	} else {
		stats.ActualAccuracy = 0
		stats.CalibrationError = 0
	}

	// Strength breakdown
	stats.HighStrengthCount = highTotal
	stats.LowStrengthCount = lowTotal
	if highTotal > 0 {
		stats.HighStrengthWinRate = round2(float64(highStrengthWins) / float64(highTotal) * 100)
	}
	if lowTotal > 0 {
		stats.LowStrengthWinRate = round2(float64(lowStrengthWins) / float64(lowTotal) * 100)
	}

	return stats
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
