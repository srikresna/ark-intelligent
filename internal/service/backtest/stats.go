package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
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

// ComputeByRegime returns stats grouped by FRED regime label.
func (sc *StatsCalculator) ComputeByRegime(ctx context.Context) (map[string]*domain.BacktestStats, error) {
	signals, err := sc.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	grouped := make(map[string][]domain.PersistedSignal)
	for _, s := range signals {
		regime := s.FREDRegime
		if regime == "" {
			regime = "UNKNOWN"
		}
		grouped[regime] = append(grouped[regime], s)
	}

	result := make(map[string]*domain.BacktestStats, len(grouped))
	for regime, sigs := range grouped {
		result[regime] = computeStats(sigs, "Regime:"+regime)
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
		weeklyReturns                []float64
		winReturns1W                 []float64
		lossReturns1W                []float64
		brierConfidences             []float64 // raw confidences for Brier score
		brierOutcomes                []bool    // outcomes for Brier score
	)

	for _, s := range signals {
		sumConfidenceAll += s.Confidence

		// Strength breakdown (only count evaluated signals, exclude EXPIRED)
		if s.Outcome1W != "" && s.Outcome1W != domain.OutcomePending && s.Outcome1W != domain.OutcomeExpired {
			if s.Strength >= config.SignalStrengthAlert {
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

		// 1W outcomes — exclude EXPIRED signals (no real outcome data).
		if s.Outcome1W != "" && s.Outcome1W != domain.OutcomePending && s.Outcome1W != domain.OutcomeExpired {
			eval1W++
			sumConfidenceEval += s.Confidence // BUG-H2: accumulate confidence for evaluated-only population
			sumReturn1W += s.Return1W
			weeklyReturns = append(weeklyReturns, s.Return1W)
			brierConfidences = append(brierConfidences, s.Confidence)
			brierOutcomes = append(brierOutcomes, s.Outcome1W == domain.OutcomeWin)
			if s.Outcome1W == domain.OutcomeWin {
				wins1W++
				sumWinReturn1W += math.Abs(s.Return1W) // abs: BEARISH wins have negative Return1W
				winCount1W++
				winReturns1W = append(winReturns1W, math.Abs(s.Return1W))
			} else {
				sumLossReturn1W += math.Abs(s.Return1W) // abs: store magnitude, sign handled separately
				lossCount1W++
				lossReturns1W = append(lossReturns1W, math.Abs(s.Return1W))
			}
		}

		// 2W outcomes — exclude EXPIRED.
		if s.Outcome2W != "" && s.Outcome2W != domain.OutcomePending && s.Outcome2W != domain.OutcomeExpired {
			eval2W++
			sumReturn2W += s.Return2W
			if s.Outcome2W == domain.OutcomeWin {
				wins2W++
			}
		}

		// 4W outcomes — exclude EXPIRED.
		if s.Outcome4W != "" && s.Outcome4W != domain.OutcomePending && s.Outcome4W != domain.OutcomeExpired {
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

	// Avg win/loss return at 1W (absolute magnitude for wins, negative for losses)
	if winCount1W > 0 {
		stats.AvgWinReturn1W = round4(sumWinReturn1W / float64(winCount1W))
	}
	if lossCount1W > 0 {
		stats.AvgLossReturn1W = round4(-(sumLossReturn1W / float64(lossCount1W)))
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

	// Brier score — measures calibration quality of raw confidence predictions.
	// Uses raw confidence / 100 as probability prediction vs actual outcome.
	if len(brierConfidences) > 0 {
		preds := make([]float64, len(brierConfidences))
		for i, c := range brierConfidences {
			preds[i] = c / 100.0 // convert 0-100 to 0-1 probability
		}
		stats.BrierScore = round4(mathutil.BrierScore(preds, brierOutcomes))
	}

	// Calibration method — indicates which recalibration approach is used
	// for this group based on sample size thresholds.
	if eval1W >= 20 {
		// Platt scaling requires sufficient data for logistic regression
		stats.CalibrationMethod = "Platt"
	} else if eval1W >= 5 {
		// Simple win-rate replacement with smaller samples
		stats.CalibrationMethod = "WinRate"
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

	// Statistical significance testing
	if eval1W > 0 {
		// Binomial test: is win rate significantly > 50%?
		stats.WinRatePValue = round4(mathutil.WinRatePValue(wins1W, eval1W))
		stats.IsStatisticallySignificant = stats.WinRatePValue < 0.05

		// 95% confidence interval for win rate (using normal approx for proportion)
		winRateFrac := float64(wins1W) / float64(eval1W)
		// Pass the population stddev (not SE) — ConfidenceInterval divides by sqrt(n) internally.
		winRateSD := math.Sqrt(winRateFrac * (1 - winRateFrac))
		ciLower, ciUpper := mathutil.ConfidenceInterval(winRateFrac*100, winRateSD*100, eval1W, 0.95)
		stats.WinRateCI = [2]float64{round2(ciLower), round2(ciUpper)}
	}

	if len(weeklyReturns) >= 2 {
		// One-sample t-test: are returns significantly different from 0?
		tStat, pVal := mathutil.TTestOneSample(weeklyReturns, 0)
		stats.ReturnTStat = round4(tStat)
		stats.ReturnPValue = round4(pVal)
	} else {
		stats.ReturnPValue = 1
	}

	// Minimum sample size for ±5 percentage points precision at 95% CI
	stats.MinSamplesNeeded = mathutil.MinSampleSize(0.05, 0.95)

	// Risk-adjusted performance metrics (require evaluated 1W data)
	// Use per-signal returns for profit factor/Kelly, but aggregate by calendar
	// week for Sharpe/MaxDD to avoid inflating drawdown from parallel signals.
	stats.WeeklyReturns = weeklyReturns
	if len(weeklyReturns) >= 2 {
		// Aggregate returns by calendar week for risk metrics.
		// Multiple signals in the same week are averaged (equal-weighted portfolio).
		aggWeekly := aggregateByWeek(signals)

		if len(aggWeekly) >= 2 {
			stats.SharpeRatio = round2(mathutil.SharpeRatio(aggWeekly, 0))
			maxDD, _, _ := mathutil.MaxDrawdown(aggWeekly)
			stats.MaxDrawdown = round2(maxDD)
			// Calmar: use mean of aggregated weekly returns (same population as MaxDD)
			// to avoid mixing per-signal returns with portfolio-level drawdown.
			aggSum := 0.0
			for _, r := range aggWeekly {
				aggSum += r
			}
			aggMean := aggSum / float64(len(aggWeekly))
			avgAnnualReturn := aggMean * 52
			stats.CalmarRatio = round2(mathutil.CalmarRatio(avgAnnualReturn, stats.MaxDrawdown))
		}
	}

	// Profit factor, expected value, Kelly criterion
	if len(winReturns1W) > 0 && len(lossReturns1W) > 0 {
		stats.ProfitFactor = round2(mathutil.ProfitFactor(winReturns1W, lossReturns1W))

		winRate := float64(winCount1W) / float64(eval1W)
		stats.ExpectedValue = round4(mathutil.ExpectedValue(winRate, stats.AvgWinReturn1W, stats.AvgLossReturn1W))

		if math.Abs(stats.AvgLossReturn1W) > 0 {
			winLossRatio := stats.AvgWinReturn1W / math.Abs(stats.AvgLossReturn1W)
			stats.KellyFraction = round4(mathutil.KellyCriterion(winRate, winLossRatio))
		}
	}

	return stats
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// aggregateByWeek groups evaluated 1W signal returns by ISO calendar week
// and computes the average return per week. This produces a proper time series
// for Sharpe ratio and drawdown calculations, avoiding the distortion of
// treating parallel signals as sequential trades.
func aggregateByWeek(signals []domain.PersistedSignal) []float64 {
	type weekAcc struct {
		sumReturn float64
		count     int
		isoWeek   string
	}
	weekMap := make(map[string]*weekAcc)

	for _, s := range signals {
		if s.Outcome1W == "" || s.Outcome1W == domain.OutcomePending || s.Outcome1W == domain.OutcomeExpired {
			continue
		}
		y, w := s.ReportDate.ISOWeek()
		key := fmt.Sprintf("%04d-W%02d", y, w)
		acc, ok := weekMap[key]
		if !ok {
			acc = &weekAcc{isoWeek: key}
			weekMap[key] = acc
		}
		acc.sumReturn += s.Return1W
		acc.count++
	}

	// Sort weeks chronologically
	weeks := make([]string, 0, len(weekMap))
	for k := range weekMap {
		weeks = append(weeks, k)
	}
	sort.Strings(weeks)

	// Average return per week (equal-weighted portfolio of that week's signals)
	result := make([]float64, 0, len(weeks))
	for _, k := range weeks {
		acc := weekMap[k]
		result = append(result, acc.sumReturn/float64(acc.count))
	}
	return result
}
