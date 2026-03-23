package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// DailyPriceProvider defines the interface for accessing daily price data.
type DailyPriceProvider interface {
	GetDailyHistory(ctx context.Context, contractCode string, days int) ([]domain.DailyPrice, error)
}

// ExcursionResult holds MFE/MAE analysis for a single signal.
type ExcursionResult struct {
	ContractCode string  `json:"contract_code"`
	Currency     string  `json:"currency"`
	SignalType   string  `json:"signal_type"`
	Direction    string  `json:"direction"`
	Strength     int     `json:"strength"`
	EntryPrice   float64 `json:"entry_price"`

	// Maximum Favorable Excursion — max move in signal direction (positive = favorable)
	MFE       float64 `json:"mfe"`        // In pips/points
	MFEPct    float64 `json:"mfe_pct"`    // As percentage
	MFEDay    int     `json:"mfe_day"`    // Day # when MFE occurred (1-based)

	// Maximum Adverse Excursion — max move against signal direction (positive = adverse)
	MAE       float64 `json:"mae"`        // In pips/points
	MAEPct    float64 `json:"mae_pct"`    // As percentage
	MAEDay    int     `json:"mae_day"`    // Day # when MAE occurred

	// Optimal exit
	OptimalExitDay int     `json:"optimal_exit_day"` // Day with best close in signal direction
	OptimalReturn  float64 `json:"optimal_return"`   // % return if exited at optimal day

	// Actual outcome
	FinalReturn float64 `json:"final_return"` // % return at end of evaluation window
	Outcome     string  `json:"outcome"`      // WIN or LOSS at weekly close
}

// ExcursionSummary holds aggregated MFE/MAE stats across multiple signals.
type ExcursionSummary struct {
	TotalSignals   int     `json:"total_signals"`
	AvgMFEPct      float64 `json:"avg_mfe_pct"`
	AvgMAEPct      float64 `json:"avg_mae_pct"`
	AvgOptimalDay  float64 `json:"avg_optimal_day"`
	AvgOptimalRet  float64 `json:"avg_optimal_ret"`

	// MFE says "signal was right" but weekly close says "LOSS"
	MissedWins     int     `json:"missed_wins"`      // MFE > threshold but outcome = LOSS
	MissedWinPct   float64 `json:"missed_win_pct"`   // % of losses that were actually profitable intraweek

	// Optimal holding period distribution
	OptimalDayDist [5]int `json:"optimal_day_dist"` // Count per day (Mon=0..Fri=4)

	// By signal type
	BySignalType map[string]*ExcursionTypeSummary `json:"by_signal_type"`
}

// ExcursionTypeSummary holds per-signal-type excursion stats.
type ExcursionTypeSummary struct {
	SignalType    string  `json:"signal_type"`
	Count         int     `json:"count"`
	AvgMFEPct     float64 `json:"avg_mfe_pct"`
	AvgMAEPct     float64 `json:"avg_mae_pct"`
	AvgOptimalDay float64 `json:"avg_optimal_day"`
	MissedWins    int     `json:"missed_wins"`
	WinRate       float64 `json:"win_rate"` // Traditional weekly close win rate
	MFEWinRate    float64 `json:"mfe_win_rate"` // Win rate if exited at MFE > 0.3%
}

// ExcursionAnalyzer computes MFE/MAE using daily price data.
type ExcursionAnalyzer struct {
	signalRepo ports_SignalRepo
	dailyRepo  DailyPriceProvider
}

// ports_SignalRepo is a minimal interface for signal retrieval.
type ports_SignalRepo interface {
	GetAllSignals(ctx context.Context) ([]domain.PersistedSignal, error)
}

// NewExcursionAnalyzer creates a new excursion analyzer.
func NewExcursionAnalyzer(signalRepo ports_SignalRepo, dailyRepo DailyPriceProvider) *ExcursionAnalyzer {
	return &ExcursionAnalyzer{signalRepo: signalRepo, dailyRepo: dailyRepo}
}

// Analyze computes MFE/MAE for all evaluated signals using daily price data.
// evaluationDays is the window to analyze (e.g., 10 for ~2 weeks of trading days).
func (a *ExcursionAnalyzer) Analyze(ctx context.Context, evaluationDays int) (*ExcursionSummary, error) {
	if evaluationDays == 0 {
		evaluationDays = 10 // Default: 2 weeks of trading days
	}

	signals, err := a.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get signals: %w", err)
	}

	summary := &ExcursionSummary{
		BySignalType: make(map[string]*ExcursionTypeSummary),
	}

	var results []ExcursionResult

	for _, sig := range signals {
		// Only analyze signals that have at least 1W outcome
		if sig.Outcome1W == "" || sig.Outcome1W == domain.OutcomePending {
			continue
		}
		if sig.EntryPrice == 0 {
			continue
		}

		result, err := a.analyzeSignal(ctx, sig, evaluationDays)
		if err != nil {
			continue // Skip signals without daily data
		}

		results = append(results, *result)
	}

	if len(results) == 0 {
		return summary, nil
	}

	// Aggregate
	summary.TotalSignals = len(results)
	var sumMFE, sumMAE, sumOptDay, sumOptRet float64
	missedThreshold := 0.3 // 0.3% — consider MFE above this as "was profitable"

	for _, r := range results {
		sumMFE += r.MFEPct
		sumMAE += r.MAEPct
		sumOptDay += float64(r.OptimalExitDay)
		sumOptRet += r.OptimalReturn

		// Track missed wins: MFE was good but weekly close = LOSS
		if r.Outcome == domain.OutcomeLoss && r.MFEPct > missedThreshold {
			summary.MissedWins++
		}

		// Optimal day distribution (map to weekday: 0=Mon..4=Fri)
		if r.OptimalExitDay >= 1 && r.OptimalExitDay <= 5 {
			summary.OptimalDayDist[r.OptimalExitDay-1]++
		}

		// Per signal type
		ts, ok := summary.BySignalType[r.SignalType]
		if !ok {
			ts = &ExcursionTypeSummary{SignalType: r.SignalType}
			summary.BySignalType[r.SignalType] = ts
		}
		ts.Count++
		ts.AvgMFEPct += r.MFEPct
		ts.AvgMAEPct += r.MAEPct
		ts.AvgOptimalDay += float64(r.OptimalExitDay)
		if r.Outcome == domain.OutcomeWin {
			ts.WinRate++
		}
		if r.MFEPct > missedThreshold {
			ts.MFEWinRate++
		}
		if r.Outcome == domain.OutcomeLoss && r.MFEPct > missedThreshold {
			ts.MissedWins++
		}
	}

	summary.AvgMFEPct = sumMFE / float64(len(results))
	summary.AvgMAEPct = sumMAE / float64(len(results))
	summary.AvgOptimalDay = sumOptDay / float64(len(results))
	summary.AvgOptimalRet = sumOptRet / float64(len(results))

	totalLosses := summary.TotalSignals - countWins(results)
	if totalLosses > 0 {
		summary.MissedWinPct = float64(summary.MissedWins) / float64(totalLosses) * 100
	}

	// Finalize per-type averages
	for _, ts := range summary.BySignalType {
		if ts.Count > 0 {
			ts.AvgMFEPct /= float64(ts.Count)
			ts.AvgMAEPct /= float64(ts.Count)
			ts.AvgOptimalDay /= float64(ts.Count)
			ts.WinRate = ts.WinRate / float64(ts.Count) * 100
			ts.MFEWinRate = ts.MFEWinRate / float64(ts.Count) * 100
		}
	}

	return summary, nil
}

// analyzeSignal computes MFE/MAE for a single signal using daily data.
func (a *ExcursionAnalyzer) analyzeSignal(ctx context.Context, sig domain.PersistedSignal, evalDays int) (*ExcursionResult, error) {
	// Get daily data starting from signal detection date
	dailyRecords, err := a.dailyRepo.GetDailyHistory(ctx, sig.ContractCode, 365)
	if err != nil || len(dailyRecords) == 0 {
		return nil, fmt.Errorf("no daily data for %s", sig.ContractCode)
	}

	// Records are newest-first; we need to find records AFTER the signal date
	// Reverse to oldest-first for easier iteration
	reversed := make([]domain.DailyPrice, len(dailyRecords))
	copy(reversed, dailyRecords)
	sort.Slice(reversed, func(i, j int) bool {
		return reversed[i].Date.Before(reversed[j].Date)
	})

	// Find the starting index (first trading day after signal detection)
	signalDate := sig.ReportDate
	startIdx := -1
	for i, dp := range reversed {
		if dp.Date.After(signalDate) || dp.Date.Equal(signalDate) {
			startIdx = i
			break
		}
	}

	if startIdx < 0 || startIdx+evalDays > len(reversed) {
		return nil, fmt.Errorf("insufficient daily data after signal date")
	}

	entry := sig.EntryPrice
	bullish := sig.Direction == "BULLISH"
	inverse := sig.Inverse

	result := &ExcursionResult{
		ContractCode: sig.ContractCode,
		Currency:     sig.Currency,
		SignalType:   sig.SignalType,
		Direction:    sig.Direction,
		Strength:     sig.Strength,
		EntryPrice:   entry,
		Outcome:      sig.Outcome1W,
	}

	var maxFavorable, maxAdverse float64
	var bestClose float64
	bestCloseDay := 1

	for d := 0; d < evalDays && startIdx+d < len(reversed); d++ {
		dp := reversed[startIdx+d]
		dayNum := d + 1

		// Compute high/low/close moves relative to entry
		highMove := (dp.High - entry) / entry * 100
		lowMove := (dp.Low - entry) / entry * 100
		closeMove := (dp.Close - entry) / entry * 100

		// For inverse pairs, flip signs
		if inverse {
			highMove, lowMove = -lowMove, -highMove
			closeMove = -closeMove
		}

		// For bearish signals, favorable = price going down
		if !bullish {
			highMove, lowMove = -lowMove, -highMove
			closeMove = -closeMove
		}

		// MFE: maximum favorable move (using high of the day for bullish, low for bearish)
		favorableExtreme := highMove // Best case for bullish (after inverse adjustment)
		if favorableExtreme > maxFavorable {
			maxFavorable = favorableExtreme
			result.MFEDay = dayNum
		}

		// MAE: maximum adverse move (worst drawdown)
		adverseExtreme := -lowMove // Worst case
		if adverseExtreme > maxAdverse {
			maxAdverse = adverseExtreme
			result.MAEDay = dayNum
		}

		// Track best close for optimal exit
		if closeMove > bestClose || d == 0 {
			bestClose = closeMove
			bestCloseDay = dayNum
		}

		// Final day return
		if d == evalDays-1 || startIdx+d == len(reversed)-1 {
			result.FinalReturn = closeMove
		}
	}

	result.MFEPct = math.Round(maxFavorable*100) / 100
	result.MAEPct = math.Round(maxAdverse*100) / 100
	result.OptimalExitDay = bestCloseDay
	result.OptimalReturn = math.Round(bestClose*100) / 100

	return result, nil
}

func countWins(results []ExcursionResult) int {
	n := 0
	for _, r := range results {
		if r.Outcome == domain.OutcomeWin {
			n++
		}
	}
	return n
}
