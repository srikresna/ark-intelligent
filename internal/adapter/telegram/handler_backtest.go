package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
)

// cmdReport handles /report — weekly signal performance summary.
func (h *Handler) cmdReport(ctx context.Context, chatID string, userID int64, args string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Report data not available yet. Signal tracking is being initialized.")
		return err
	}

	loadingID, _ := h.bot.SendLoading(ctx, chatID, "📊 Generating weekly report... ⏳")

	gen := backtestsvc.NewReportGenerator(h.signalRepo)
	report, err := gen.GenerateWeeklyReport(ctx)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	htmlOut := h.fmt.FormatWeeklyReport(report)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

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
	case args == "ALL":
		return h.backtestAll(ctx, chatID, calc)
	case args == "":
		return h.backtestMenu(ctx, chatID, calc)
	case args == "SIGNALS" || args == "TYPES":
		return h.backtestBySignalType(ctx, chatID, calc)
	case args == "TIMING":
		return h.backtestTiming(ctx, chatID)
	case args == "WALKFORWARD" || args == "WF":
		return h.backtestWalkForward(ctx, chatID)
	case args == "WEIGHTS" || args == "WEIGHT":
		return h.backtestWeights(ctx, chatID)
	case args == "SMARTMONEY" || args == "SM":
		return h.backtestSmartMoney(ctx, chatID)
	case args == "EXCURSION" || args == "MFE" || args == "MAE":
		return h.backtestExcursion(ctx, chatID)
	case args == "TREND" || args == "TRENDFILTER" || args == "DAILY":
		return h.backtestTrendFilter(ctx, chatID)
	case args == "BASELINE" || args == "BASE":
		return h.backtestBaseline(ctx, chatID)
	case args == "REGIME" || args == "REGIMES":
		return h.backtestByRegime(ctx, chatID, calc)
	case args == "DEDUP" || args == "OVERLAP":
		return h.backtestDedup(ctx, chatID)
	case args == "MATRIX":
		return h.backtestMatrix(ctx, chatID)
	case args == "MONTECARLO" || args == "MC":
		return h.backtestMonteCarlo(ctx, chatID)
	case args == "PORTFOLIO" || args == "PORT":
		return h.backtestPortfolio(ctx, chatID)
	case args == "COST" || args == "COSTS":
		return h.backtestCost(ctx, chatID)
	case args == "RUIN" || args == "ROR":
		return h.backtestRuin(ctx, chatID)
	case args == "AUDIT":
		return h.backtestAudit(ctx, chatID)
	case args == "MULTI" || args == "MULTISTRATEGY" || args == "COMPOSE":
		return h.backtestMultiStrategy(ctx, chatID)
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
		helpMsg := "\xE2\x9D\x93 <b>Usage:</b> <code>/backtest [option]</code>\n\n" +
			"<b>Core</b>\n" +
			"<code>/backtest</code> \xe2\x80\x94 aggregate summary\n" +
			"<code>/backtest all</code> \xe2\x80\x94 full statistics\n" +
			"<code>/backtest signals</code> \xe2\x80\x94 by signal type\n" +
			"<code>/backtest EUR</code> \xe2\x80\x94 specific currency\n\n" +
			"<b>Analysis</b>\n" +
			"<code>/backtest timing</code> \xe2\x80\x94 optimal horizon\n" +
			"<code>/backtest walkforward</code> \xe2\x80\x94 overfit detection\n" +
			"<code>/backtest weights</code> \xe2\x80\x94 factor weights\n" +
			"<code>/backtest sm</code> \xe2\x80\x94 smart money tracking\n" +
			"<code>/backtest excursion</code> \xe2\x80\x94 MFE/MAE analysis\n" +
			"<code>/backtest trend</code> \xe2\x80\x94 daily trend filter\n\n" +
			"<b>Advanced</b>\n" +
			"<code>/backtest baseline</code> \xe2\x80\x94 vs random baseline\n" +
			"<code>/backtest dedup</code> \xe2\x80\x94 signal overlap\n" +
			"<code>/backtest regime</code> \xe2\x80\x94 by macro regime\n" +
			"<code>/backtest cost</code> \xe2\x80\x94 transaction cost impact\n" +
			"<code>/backtest matrix</code> \xe2\x80\x94 type \xc3\x97 regime matrix\n" +
			"<code>/backtest mc</code> \xe2\x80\x94 Monte Carlo simulation\n" +
			"<code>/backtest portfolio</code> \xe2\x80\x94 portfolio-level\n" +
			"<code>/backtest ruin</code> \xe2\x80\x94 risk of ruin\n" +
			"<code>/backtest audit</code> \xe2\x80\x94 bias audit\n" +
			"<code>/backtest multi</code> \xe2\x80\x94 multi-strategy composer"
		_, err := h.bot.SendHTML(ctx, chatID, helpMsg)
		return err
	}
}

func (h *Handler) backtestMenu(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
	stats, err := calc.ComputeAll(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	summary := "📊 <b>BACKTEST DASHBOARD</b>\n"
	if stats.TotalSignals > 0 {
		summary += fmt.Sprintf("<code>Signals: %d | Win 1W: %.1f%% | Win 4W: %.1f%%</code>\n",
			stats.TotalSignals, stats.WinRate1W, stats.WinRate4W)
	} else {
		summary += "<i>No signal data available yet.</i>\n"
	}
	summary += "\n<i>Select a view from Core, Analysis, or Advanced:</i>"

	kb := h.kb.BacktestMenu()
	_, err = h.bot.SendWithKeyboard(ctx, chatID, summary, kb)
	return err
}

func (h *Handler) backtestAll(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
	loadingID, _ := h.bot.SendLoading(ctx, chatID, "📊 Computing full backtest statistics... ⏳")

	stats, err := calc.ComputeAll(ctx)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if stats.TotalSignals == 0 {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err := h.bot.SendHTML(ctx, chatID, "No signal data available yet. Signals are generated on each COT release.")
		return err
	}

	htmlOut := h.fmt.FormatBacktestStats(stats)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

func (h *Handler) backtestBySignalType(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
	statsMap, err := calc.ComputeAllBySignalType(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
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
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if stats.TotalSignals == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("No signal data for type <code>%s</code> yet.", sigType))
		return err
	}

	stats.GroupLabel = sigType
	html := h.fmt.FormatBacktestStats(stats)

	// Append suppression recommendation based on expected value (not just win rate)
	suppressNote := ""
	if stats.Evaluated1W >= 10 {
		if stats.ExpectedValue < 0 && stats.WinRate1W < 45 {
			suppressNote = fmt.Sprintf(
				"\n\n\xF0\x9F\x94\xB4 <b>Suppression Candidate</b>\n"+
					"<i>EV %.4f%% with win rate %.1f%% (n=%d).\n"+
					"Negative expected value \xe2\x80\x94 consuming risk budget.\n"+
					"Consider: <code>suppress_%s=true</code> in config.</i>",
				stats.ExpectedValue, stats.WinRate1W, stats.Evaluated1W, strings.ToLower(sigType),
			)
		} else if stats.ExpectedValue > 0.2 {
			suppressNote = fmt.Sprintf(
				"\n\n\xE2\x9C\x85 <b>Edge Confirmed</b>\n"+
					"<i>EV +%.4f%% with win rate %.1f%% (n=%d).\n"+
					"Positive expected value \xe2\x80\x94 signal type is contributing.</i>",
				stats.ExpectedValue, stats.WinRate1W, stats.Evaluated1W,
			)
		} else if stats.ExpectedValue > 0 {
			suppressNote = fmt.Sprintf(
				"\n\n\xE2\x9A\xA0\xEF\xB8\x8F <b>Marginal Edge</b>\n"+
					"<i>EV +%.4f%% with win rate %.1f%% (n=%d).\n"+
					"Positive but thin \xe2\x80\x94 monitor closely.</i>",
				stats.ExpectedValue, stats.WinRate1W, stats.Evaluated1W,
			)
		}
	} else {
		suppressNote = fmt.Sprintf(
			"\n\n\xE2\x8F\xB3 <i>Only %d evaluated signals \xe2\x80\x94 need \xE2\x89\xA510 for suppression recommendation.</i>",
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
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
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
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
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
			"<code>Avg Conf :</code> %.0f%% (calibration error: %.1f%%)\n",
		stats.TotalSignals, stats.Evaluated,
		stats.WinRate1W, stats.WinRate2W, stats.WinRate4W,
		stats.BestPeriod, stats.BestWinRate,
		stats.AvgConfidence, stats.CalibrationError,
	)

	if stats.Evaluated < 30 {
		html += fmt.Sprintf("\n⚠️ <b>Small sample (%d signals) — win rate has high uncertainty</b>\n", stats.Evaluated)
	}

	html += "\n<i>Use /backtest for detailed breakdown</i>"

	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// backtestBaseline compares system performance against random direction baseline.
func (h *Handler) backtestBaseline(ctx context.Context, chatID string) error {
	loadingID, _ := h.bot.SendLoading(ctx, chatID, "📊 Running baseline simulation (1000 iterations)... ⏳")

	gen := backtestsvc.NewBaselineGenerator(h.signalRepo)
	result, err := gen.ComputeBaseline(ctx, 1000)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	var b strings.Builder
	b.WriteString("\xF0\x9F\x93\x8A <b>Baseline Comparison</b> (1000 simulations)\n\n")

	b.WriteString("<b>Win Rate (1W)</b>\n")
	b.WriteString(fmt.Sprintf("<code>System   :</code> %.1f%%\n", result.SystemWinRate1W))
	b.WriteString(fmt.Sprintf("<code>Random   :</code> %.1f%%\n", result.RandomWinRate1W))
	edgeIcon1W := "\xE2\x9C\x85"
	if result.SystemEdge1W < 0 {
		edgeIcon1W = "\xF0\x9F\x94\xB4"
	}
	b.WriteString(fmt.Sprintf("<code>Edge     :</code> %+.1fpp %s\n\n", result.SystemEdge1W, edgeIcon1W))

	b.WriteString("<b>Expected Value</b>\n")
	b.WriteString(fmt.Sprintf("<code>System EV:</code> %+.4f%%\n", result.SystemEV))
	b.WriteString(fmt.Sprintf("<code>Random EV:</code> %+.4f%%\n", result.RandomEV))
	evEdgeIcon := "\xE2\x9C\x85"
	if result.EVEdge < 0 {
		evEdgeIcon = "\xF0\x9F\x94\xB4"
	}
	b.WriteString(fmt.Sprintf("<code>EV Edge  :</code> %+.4fpp %s\n\n", result.EVEdge, evEdgeIcon))

	if result.SystemEdge1W > 0 && result.EVEdge > 0 {
		b.WriteString("\xE2\x9C\x85 <i>System beats random baseline on both win rate and expected value.</i>")
	} else if result.EVEdge > 0 {
		b.WriteString("\xE2\x9A\xA0\xEF\xB8\x8F <i>System captures asymmetric payoff (positive EV) despite lower win rate.</i>")
	} else {
		b.WriteString("\xF0\x9F\x94\xB4 <i>System underperforms random baseline. Review signal quality.</i>")
	}

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// backtestByRegime shows performance grouped by FRED macro regime.
func (h *Handler) backtestByRegime(ctx context.Context, chatID string, calc *backtestsvc.StatsCalculator) error {
	statsMap, err := calc.ComputeByRegime(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if len(statsMap) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No regime data available. FRED regime labels may not be populated yet.")
		return err
	}

	htmlOut := h.fmt.FormatBacktestSummary(statsMap, "Regime")
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// backtestDedup shows raw vs deduplicated signal statistics.
func (h *Handler) backtestDedup(ctx context.Context, chatID string) error {
	signals, err := h.signalRepo.GetAllSignals(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	result := backtestsvc.ComputeDedupStats(signals)

	var b strings.Builder
	b.WriteString("\xF0\x9F\x93\x8A <b>Signal Deduplication Analysis</b>\n\n")
	b.WriteString(fmt.Sprintf("<code>Raw signals   :</code> %d\n", result.RawSignalCount))
	b.WriteString(fmt.Sprintf("<code>Dedup signals :</code> %d\n", result.DedupSignalCount))
	b.WriteString(fmt.Sprintf("<code>Overlap rate  :</code> %.1f%%\n\n", result.OverlapRate))
	b.WriteString(fmt.Sprintf("<code>Dedup WR 1W   :</code> %.1f%%\n", result.DedupWinRate1W))
	b.WriteString(fmt.Sprintf("<code>Dedup Avg Ret :</code> %.4f%%\n", result.DedupAvgReturn1W))

	if result.OverlapRate > 30 {
		b.WriteString("\n\xE2\x9A\xA0\xEF\xB8\x8F <i>High overlap: multiple signal types firing on same contract/week.\nDeduplicated stats may be more representative of true edge.</i>")
	}

	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// backtestTiming shows per-signal-type timing analysis with optimal horizons.
func (h *Handler) backtestTiming(ctx context.Context, chatID string) error {
	analyzer := backtestsvc.NewTimingAnalyzer(h.signalRepo)
	analyses, err := analyzer.Analyze(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if len(analyses) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No signal data available yet for timing analysis.")
		return err
	}

	htmlOut := h.fmt.FormatSignalTiming(analyses)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// backtestWalkForward shows walk-forward overfit analysis.
func (h *Handler) backtestWalkForward(ctx context.Context, chatID string) error {
	analyzer := backtestsvc.NewWalkForwardAnalyzer(h.signalRepo)
	result, err := analyzer.Analyze(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if len(result.Windows) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "Not enough data for walk-forward analysis. Need at least 39 weeks of evaluated signals (26w train + 13w test).")
		return err
	}

	htmlOut := h.fmt.FormatWalkForward(result)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// backtestWeights shows factor weight optimization analysis.
func (h *Handler) backtestWeights(ctx context.Context, chatID string) error {
	optimizer := backtestsvc.NewWeightOptimizer(h.signalRepo)
	result, err := optimizer.OptimizeWeights(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	htmlOut := h.fmt.FormatWeightOptimization(result)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// backtestSmartMoney shows smart money tracking accuracy per contract.
func (h *Handler) backtestSmartMoney(ctx context.Context, chatID string) error {
	if h.cotRepo == nil || h.priceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Smart money analysis requires both COT and price data.")
		return err
	}

	loadingID, _ := h.bot.SendLoading(ctx, chatID, "🏛 Analyzing smart money tracking accuracy... ⏳")

	analyzer := backtestsvc.NewSmartMoneyAnalyzer(h.cotRepo, h.priceRepo)
	results, err := analyzer.Analyze(ctx)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if len(results) == 0 {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err := h.bot.SendHTML(ctx, chatID, "Not enough data for smart money analysis. Need COT + price history.")
		return err
	}

	htmlOut := h.fmt.FormatSmartMoneyAccuracy(results)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// backtestExcursion shows MFE/MAE analysis using daily price data.
func (h *Handler) backtestExcursion(ctx context.Context, chatID string) error {
	if h.signalRepo == nil || h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Excursion analysis requires both signal and daily price data.")
		return err
	}

	loadingID, _ := h.bot.SendLoading(ctx, chatID, "📊 Computing MFE/MAE excursion analysis... ⏳")

	analyzer := backtestsvc.NewExcursionAnalyzer(h.signalRepo, h.dailyPriceRepo)
	summary, err := analyzer.Analyze(ctx, 10)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if summary.TotalSignals == 0 {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err := h.bot.SendHTML(ctx, chatID, "Not enough data for excursion analysis. Need evaluated signals + daily price history.")
		return err
	}

	htmlOut := h.fmt.FormatExcursionSummary(summary)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// backtestTrendFilter shows daily trend filter effectiveness analysis.
func (h *Handler) backtestTrendFilter(ctx context.Context, chatID string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Trend filter analysis requires signal data.")
		return err
	}

	analyzer := backtestsvc.NewTrendFilterAnalyzer(h.signalRepo)
	stats, err := analyzer.Analyze(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if stats.TotalSignals == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "Not enough evaluated signals with daily trend data yet. The trend filter applies to newly detected signals.")
		return err
	}

	htmlOut := h.fmt.FormatTrendFilterStats(stats)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// backtestMatrix shows conditional performance matrix (signal type × regime).
func (h *Handler) backtestMatrix(ctx context.Context, chatID string) error {
	analyzer := backtestsvc.NewMatrixAnalyzer(h.signalRepo)
	matrix, err := analyzer.Analyze(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	if len(matrix.Cells) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No evaluated signals available for matrix analysis.")
		return err
	}

	var b strings.Builder
	b.WriteString("\xF0\x9F\x93\x8A <b>Performance Matrix (1W Win Rate)</b>\n\n")

	// Collect all regimes
	regimes := make(map[string]bool)
	for _, regimeMap := range matrix.Cells {
		for r := range regimeMap {
			regimes[r] = true
		}
	}
	regimeList := make([]string, 0, len(regimes))
	for r := range regimes {
		regimeList = append(regimeList, r)
	}

	// Header
	b.WriteString("<code>            ")
	for _, r := range regimeList {
		b.WriteString(fmt.Sprintf(" %8s", truncate(r, 8)))
	}
	b.WriteString("</code>\n")

	// Rows
	for sigType, regimeMap := range matrix.Cells {
		b.WriteString(fmt.Sprintf("<code>%-12s", truncate(sigType, 12)))
		for _, r := range regimeList {
			cell := regimeMap[r]
			if cell != nil && cell.SampleSize >= 5 {
				b.WriteString(fmt.Sprintf("  %5.1f%%", cell.WinRate))
			} else {
				b.WriteString("      - ")
			}
		}
		b.WriteString("</code>\n")
	}

	if matrix.BestCombo != "" {
		b.WriteString(fmt.Sprintf("\n\xF0\x9F\x8E\xAF Best: %s (%.1f%%)\n", matrix.BestCombo, matrix.BestWinRate))
	}
	if matrix.WorstCombo != "" {
		b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8C Worst: %s (%.1f%%)\n", matrix.WorstCombo, matrix.WorstWinRate))
	}

	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// backtestMonteCarlo runs Monte Carlo bootstrap simulation.
func (h *Handler) backtestMonteCarlo(ctx context.Context, chatID string) error {
	loadingID, _ := h.bot.SendLoading(ctx, chatID, "🎲 Running Monte Carlo simulation (1000 runs)... ⏳")

	sim := backtestsvc.NewMonteCarloSimulator(h.signalRepo)
	result, err := sim.Simulate(ctx, 1000)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	var b strings.Builder
	b.WriteString("\xF0\x9F\x8E\xB2 <b>Monte Carlo Simulation</b> (1000 runs)\n")
	b.WriteString(fmt.Sprintf("<i>Resampling %d weekly portfolio returns into simulated 52W years</i>\n\n", result.WeeksResampled))

	b.WriteString("<b>Cumulative Return (52W)</b>\n")
	b.WriteString(fmt.Sprintf("<code>Median   :</code> %+.2f%%\n", result.MedianReturn))
	b.WriteString(fmt.Sprintf("<code>Best 5%%  :</code> %+.2f%%\n", result.P95Return))
	b.WriteString(fmt.Sprintf("<code>Worst 5%% :</code> %+.2f%%\n\n", result.P5Return))

	b.WriteString("<b>Max Drawdown</b>\n")
	b.WriteString(fmt.Sprintf("<code>Median   :</code> -%.2f%%\n", result.MedianMaxDD))
	b.WriteString(fmt.Sprintf("<code>Worst 5%% :</code> -%.2f%%\n\n", result.WorstCaseMaxDD))

	b.WriteString(fmt.Sprintf("<code>P(Loss)  :</code> %.1f%%\n", result.ProbabilityOfLoss))
	b.WriteString(fmt.Sprintf("<code>Med Sharpe:</code> %.2f\n", result.MedianSharpe))

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// backtestPortfolio shows portfolio-level performance (equal-weight weekly).
func (h *Handler) backtestPortfolio(ctx context.Context, chatID string) error {
	analyzer := backtestsvc.NewPortfolioAnalyzer(h.signalRepo)
	result, err := analyzer.Analyze(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	var b strings.Builder
	b.WriteString("\xF0\x9F\x93\x88 <b>Portfolio-Level Analysis</b>\n\n")
	b.WriteString(fmt.Sprintf("<code>Total weeks    :</code> %d\n", result.TotalWeeks))
	b.WriteString(fmt.Sprintf("<code>Active weeks   :</code> %d\n", result.ActiveWeeks))
	b.WriteString(fmt.Sprintf("<code>Avg signals/wk :</code> %.1f\n\n", result.AvgSignalsPerWeek))

	b.WriteString("<b>Portfolio Returns</b>\n")
	b.WriteString(fmt.Sprintf("<code>Cumulative     :</code> %+.2f%%\n", result.CumulativeReturn))
	b.WriteString(fmt.Sprintf("<code>Sharpe (ann.)  :</code> %.2f\n", result.PortfolioSharpe))
	b.WriteString(fmt.Sprintf("<code>Max Drawdown   :</code> -%.2f%%\n", result.PortfolioMaxDD))
	if result.CalmarRatio != 0 {
		b.WriteString(fmt.Sprintf("<code>Calmar         :</code> %.2f\n", result.CalmarRatio))
	}

	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// backtestCost shows transaction cost impact analysis.
func (h *Handler) backtestCost(ctx context.Context, chatID string) error {
	signals, err := h.signalRepo.GetAllSignals(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	result := backtestsvc.ComputeCostAnalysis(signals, "ALL")

	var b strings.Builder
	b.WriteString("\xF0\x9F\x92\xB0 <b>Transaction Cost Impact</b>\n\n")
	b.WriteString(fmt.Sprintf("<code>Evaluated      :</code> %d signals\n", result.Evaluated))
	b.WriteString(fmt.Sprintf("<code>Avg Cost/Trade :</code> %.4f%%\n\n", result.AvgCostPct))

	b.WriteString("<b>Expected Value</b>\n")
	b.WriteString(fmt.Sprintf("<code>Before cost    :</code> %+.4f%%\n", result.RawEV))
	b.WriteString(fmt.Sprintf("<code>After cost     :</code> %+.4f%%\n", result.NetEV))

	if result.CostErasesEdge {
		b.WriteString("\n\xF0\x9F\x94\xB4 <b>WARNING:</b> Transaction costs erase the positive edge!\n")
		b.WriteString("<i>Consider focusing on higher-EV signal types or wider take-profit targets.</i>")
	} else if result.NetEV > 0 {
		b.WriteString("\n\xE2\x9C\x85 <i>Edge survives after transaction costs.</i>")
	}

	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// truncate shortens a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// backtestRuin shows risk of ruin analysis.
func (h *Handler) backtestRuin(ctx context.Context, chatID string) error {
	signals, err := h.signalRepo.GetAllSignals(ctx)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	result := backtestsvc.ComputeRiskOfRuinFromSignals(signals)

	var b strings.Builder
	b.WriteString("\xF0\x9F\x8E\xB0 <b>Risk of Ruin Analysis</b>\n\n")
	b.WriteString(fmt.Sprintf("<code>Win Rate     :</code> %.1f%%\n", result.WinRate))
	b.WriteString(fmt.Sprintf("<code>Avg Win      :</code> +%.4f%%\n", result.AvgWin))
	b.WriteString(fmt.Sprintf("<code>Avg Loss     :</code> %.4f%%\n", result.AvgLoss))
	b.WriteString(fmt.Sprintf("<code>Kelly %%      :</code> %.1f%%\n\n", result.KellyFraction*100))

	b.WriteString("<b>Drawdown Probabilities</b>\n")
	b.WriteString(fmt.Sprintf("<code>P(10%% DD)    :</code> %.1f%%\n", result.RuinProb10Pct*100))
	b.WriteString(fmt.Sprintf("<code>P(25%% DD)    :</code> %.1f%%\n", result.RuinProb25Pct*100))
	b.WriteString(fmt.Sprintf("<code>P(50%% DD)    :</code> %.1f%%\n\n", result.RuinProb50Pct*100))

	b.WriteString(fmt.Sprintf("<code>Safe size    :</code> %.1f%% of capital\n", result.SafePositionSize*100))
	b.WriteString("<i>(Max position for <5%% probability of 25%% drawdown)</i>")

	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// backtestAudit shows bias audit for each signal type.
func (h *Handler) backtestAudit(ctx context.Context, chatID string) error {
	audits := backtestsvc.GetAuditSummary()

	var b strings.Builder
	b.WriteString("\xF0\x9F\x94\x8D <b>Signal Bias Audit</b>\n\n")

	for _, a := range audits {
		b.WriteString(fmt.Sprintf("<b>%s</b> (added %s)\n", a.SignalType, a.DateAdded))
		b.WriteString(fmt.Sprintf("<i>%s</i>\n", a.Hypothesis))
		b.WriteString(fmt.Sprintf("Ref: %s\n", a.TheoreticalBasis))
		for _, bias := range a.PotentialBiases {
			b.WriteString(fmt.Sprintf("\xE2\x9A\xA0\xEF\xB8\x8F %s\n", bias))
		}
		b.WriteString("\n")
	}

	_, err := h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// backtestMultiStrategy runs per-strategy analysis and shows combined portfolio metrics.
func (h *Handler) backtestMultiStrategy(ctx context.Context, chatID string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Signal data not available yet.")
		return err
	}

	loadingID, _ := h.bot.SendLoading(ctx, chatID, "📊 Running multi-strategy analysis... ⏳")

	composer := backtestsvc.NewStrategyComposer(h.signalRepo)
	result, err := composer.Compose(ctx)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "backtest")
		return nil
	}

	htmlOut := h.fmt.FormatMultiStrategy(result)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}
