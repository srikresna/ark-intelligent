package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
)

// knownSignalTypes is the canonical set of signal type names.
var knownSignalTypes = map[string]bool{
	"SMART_MONEY":        true,
	"EXTREME_POSITIONING": true,
	"DIVERGENCE":         true,
	"MOMENTUM_SHIFT":     true,
	"CONCENTRATION":      true,
	"CROWD_CONTRARIAN":   true,
	"THIN_MARKET":        true,
}

// cmdBacktest handles /backtest [contract|all|signals|SIGNAL_TYPE]
func (h *Handler) cmdBacktest(ctx context.Context, chatID string, userID int64, args string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Backtest data not available yet. Signal tracking is being initialized.")
		return err
	}

	calc := backtestsvc.NewStatsCalculator(h.signalRepo)
	args = strings.TrimSpace(strings.ToUpper(args))

	switch {
	case args == "" || args == "ALL":
		return h.backtestAll(ctx, chatID, calc)
	case args == "SIGNALS" || args == "TYPES":
		return h.backtestBySignalType(ctx, chatID, calc)
	case knownSignalTypes[args]:
		// e.g. /backtest SMART_MONEY
		return h.backtestOneSignalType(ctx, chatID, calc, args)
	default:
		// Try currency first; if not found, show help.
		// Exclude RiskOnly instruments (VIX, SPX) — they are not COT contracts.
		mapping := domain.FindPriceMappingByCurrency(args)
		if mapping != nil && !mapping.RiskOnly {
			return h.backtestByContract(ctx, chatID, calc, args)
		}
		helpMsg := "❓ <b>Usage:</b> <code>/backtest [option]</code>\n\n" +
			"<b>Options:</b>\n" +
			"<code>/backtest</code> — aggregate summary\n" +
			"<code>/backtest signals</code> — breakdown by signal type\n" +
			"<code>/backtest SMART_MONEY</code> — specific signal type\n" +
			"<code>/backtest EUR</code> — specific currency\n\n" +
			"<b>Signal types:</b> SMART_MONEY · EXTREME_POSITIONING · DIVERGENCE · MOMENTUM_SHIFT · CONCENTRATION · CROWD_CONTRARIAN · THIN_MARKET"
		_, err := h.bot.SendHTML(ctx, chatID, helpMsg)
		return err
	}
}

func (h *Handler) backtestAll(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
	stats, err := calc.ComputeAll(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if stats.TotalSignals == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No signal data available yet. Signals are generated on each COT release.")
		return err
	}

	html := h.fmt.FormatBacktestStats(stats)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

func (h *Handler) backtestBySignalType(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
	statsMap, err := calc.ComputeAllBySignalType(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if len(statsMap) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No signal data available yet.")
		return err
	}

	html := h.fmt.FormatBacktestSummary(statsMap, "Signal Type")
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// backtestOneSignalType shows detailed stats for a single signal type.
func (h *Handler) backtestOneSignalType(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator, sigType string) error {
	stats, err := calc.ComputeBySignalType(ctx, sigType)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if stats.TotalSignals == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("No signal data for type <code>%s</code> yet.", sigType))
		return err
	}

	stats.GroupLabel = sigType
	html := h.fmt.FormatBacktestStats(stats)

	// Append suppression recommendation
	suppressNote := ""
	if stats.Evaluated1W >= 10 {
		if stats.WinRate1W < 50 {
			suppressNote = fmt.Sprintf(
				"\n\n🔴 <b>Suppression Candidate</b>\n"+
					"<i>Win rate %.1f%% with n=%d is below 50%% threshold.\n"+
					"This signal type is consuming noise budget.\n"+
					"Consider: <code>suppress_%s=true</code> in config.</i>",
				stats.WinRate1W, stats.Evaluated1W, strings.ToLower(sigType),
			)
		} else if stats.WinRate1W >= 60 {
			suppressNote = fmt.Sprintf(
				"\n\n✅ <b>Edge Confirmed</b>\n"+
					"<i>Win rate %.1f%% with n=%d shows statistical edge.\n"+
					"Signal type is performing above expectation.</i>",
				stats.WinRate1W, stats.Evaluated1W,
			)
		}
	} else {
		suppressNote = fmt.Sprintf(
			"\n\n⏳ <i>Only %d evaluated signals — need ≥10 for suppression recommendation.</i>",
			stats.Evaluated1W,
		)
	}

	_, err = h.bot.SendHTML(ctx, chatID, html+suppressNote)
	return err
}

func (h *Handler) backtestByContract(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator, currency string) error {
	// Resolve currency to contract code
	mapping := domain.FindPriceMappingByCurrency(currency)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Unknown currency: %s\n\nUsage: /backtest [all|signals|EUR|GBP|...]", currency))
		return err
	}

	stats, err := calc.ComputeByContract(ctx, mapping.ContractCode)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if stats.TotalSignals == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("No signal data for %s yet.", currency))
		return err
	}

	stats.GroupLabel = currency
	html := h.fmt.FormatBacktestStats(stats)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// cmdAccuracy handles /accuracy — quick one-line accuracy summary
func (h *Handler) cmdAccuracy(ctx context.Context, chatID string, userID int64, args string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Backtest data not available yet.")
		return err
	}

	calc := backtestsvc.NewStatsCalculator(h.signalRepo)
	stats, err := calc.ComputeAll(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Error: %s", err))
		return sendErr
	}

	if stats.Evaluated == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No evaluated signals yet. Outcomes are calculated after price data becomes available.")
		return err
	}

	html := fmt.Sprintf(
		"\xF0\x9F\x8E\xAF <b>Signal Accuracy</b>\n\n"+
			"<code>Signals  :</code> %d total, %d evaluated\n"+
			"<code>Win Rate :</code> 1W %.1f%% | 2W %.1f%% | 4W %.1f%%\n"+
			"<code>Best     :</code> %s at %.1f%%\n"+
			"<code>Avg Conf :</code> %.0f%% (calibration error: %.1f%%)\n\n"+
			"<i>Use /backtest for detailed breakdown</i>",
		stats.TotalSignals, stats.Evaluated,
		stats.WinRate1W, stats.WinRate2W, stats.WinRate4W,
		stats.BestPeriod, stats.BestWinRate,
		stats.AvgConfidence, stats.CalibrationError,
	)

	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}
