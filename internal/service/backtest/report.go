package backtest

import (
	"context"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ReportGenerator generates weekly performance reports from persisted signals.
type ReportGenerator struct {
	signalRepo ports.SignalRepository
}

// NewReportGenerator creates a new ReportGenerator.
func NewReportGenerator(signalRepo ports.SignalRepository) *ReportGenerator {
	return &ReportGenerator{signalRepo: signalRepo}
}

// GenerateWeeklyReport builds a WeeklyReport from signals detected in the last 7 days,
// using all-time signals for streak and running-average calculations.
func (rg *ReportGenerator) GenerateWeeklyReport(ctx context.Context) (*domain.WeeklyReport, error) {
	recentSignals, err := rg.signalRepo.GetRecentSignals(ctx, 7)
	if err != nil {
		return nil, fmt.Errorf("get recent signals: %w", err)
	}

	allSignals, err := rg.signalRepo.GetAllSignals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all signals: %w", err)
	}

	return buildWeeklyReport(recentSignals, allSignals, time.Now()), nil
}

// buildWeeklyReport computes the report from recent (this-week) and all-time signals.
func buildWeeklyReport(recent, all []domain.PersistedSignal, now time.Time) *domain.WeeklyReport {
	weekEnd := now
	weekStart := now.AddDate(0, 0, -7)

	report := &domain.WeeklyReport{
		WeekStart: weekStart,
		WeekEnd:   weekEnd,
	}

	// Build per-signal results from the recent window.
	var wins, losses, pending int
	for _, s := range recent {
		sr := domain.SignalResult{
			Contract:      resolveCurrency(s),
			SignalType:    s.SignalType,
			Direction:     s.Direction,
			PriceAtSignal: s.EntryPrice,
			DetectedAt:    s.DetectedAt,
		}

		switch {
		case s.Outcome1W == domain.OutcomeWin:
			sr.Result = domain.OutcomeWin
			sr.CurrentPrice = s.Price1W
			sr.PipsChange = s.Return1W
			wins++
		case s.Outcome1W == domain.OutcomeLoss:
			sr.Result = domain.OutcomeLoss
			sr.CurrentPrice = s.Price1W
			sr.PipsChange = s.Return1W
			losses++
		default:
			sr.Result = domain.OutcomePending
			sr.CurrentPrice = s.EntryPrice // no follow-up price yet
			sr.PipsChange = 0
			pending++
		}

		report.Signals = append(report.Signals, sr)
	}

	report.Wins = wins
	report.Losses = losses
	report.Pending = pending

	evaluated := wins + losses
	if evaluated > 0 {
		pct := float64(wins) / float64(evaluated) * 100
		report.WeeklyScore = fmt.Sprintf("%d/%d (%.0f%%)", wins, evaluated, pct)
	} else if pending > 0 {
		report.WeeklyScore = fmt.Sprintf("0/0 — %d pending", pending)
	} else {
		report.WeeklyScore = "No signals"
	}

	// Running 52-week average: win rate across all evaluated signals from the past 52 weeks.
	cutoff52W := now.AddDate(0, 0, -364)
	var totalEval52, totalWins52 int
	for _, s := range all {
		if s.DetectedAt.Before(cutoff52W) {
			continue
		}
		if s.Outcome1W == domain.OutcomeWin {
			totalWins52++
			totalEval52++
		} else if s.Outcome1W == domain.OutcomeLoss {
			totalEval52++
		}
	}
	if totalEval52 > 0 {
		report.RunningAverage52W = round2(float64(totalWins52) / float64(totalEval52) * 100)
	}

	// Streak calculation: iterate all signals newest-first, counting consecutive wins.
	report.BestStreak, report.CurrentStreak = computeStreaks(all)

	return report
}

// computeStreaks calculates the current and best win streaks from all-time signals (newest-first).
func computeStreaks(signals []domain.PersistedSignal) (best, current int) {
	// signals are already newest-first
	streak := 0
	currentSet := false
	for _, s := range signals {
		if s.Outcome1W == "" || s.Outcome1W == domain.OutcomePending {
			continue
		}
		if s.Outcome1W == domain.OutcomeWin {
			streak++
			if streak > best {
				best = streak
			}
		} else {
			if !currentSet {
				current = streak
				currentSet = true
			}
			streak = 0
		}
	}
	// If we never hit a loss, the current streak is the full winning streak.
	if !currentSet {
		current = streak
	}
	return best, current
}

// resolveCurrency returns the currency label for a signal, preferring Currency over ContractCode.
func resolveCurrency(s domain.PersistedSignal) string {
	if s.Currency != "" {
		return s.Currency
	}
	return s.ContractCode
}
