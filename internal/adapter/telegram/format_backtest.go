package telegram

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatBacktestStats formats a single BacktestStats into Telegram HTML.
func (f *Formatter) FormatBacktestStats(stats *domain.BacktestStats) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8A <b>Backtest: %s</b>\n\n", stats.GroupLabel))

	b.WriteString(fmt.Sprintf("<code>Signals  :</code> %d total, %d evaluated\n\n", stats.TotalSignals, stats.Evaluated))

	// Primary metrics: Expectancy and Profit Factor
	b.WriteString("<b>Performance</b>\n")
	if stats.ExpectedValue != 0 {
		evIcon := "\xE2\x9C\x85" // checkmark
		evLabel := "Positive Edge"
		if stats.ExpectedValue > 0.3 {
			evLabel = "Strong Edge"
		} else if stats.ExpectedValue <= 0 {
			evIcon = "\xF0\x9F\x94\xB4 Weak" // red circle
			evLabel = "Negative Edge"
		} else if stats.ExpectedValue < 0.1 {
			evIcon = "\xE2\x9A\xA0\xEF\xB8\x8F" // warning
			evLabel = "Marginal Edge"
		}
		b.WriteString(fmt.Sprintf("<code>EV/Trade :</code> %+.4f%% %s %s\n", stats.ExpectedValue, evIcon, evLabel))
	}
	if stats.ProfitFactor != 0 {
		pfIcon := "\xE2\x9C\x85"
		if stats.ProfitFactor < 1.0 {
			pfIcon = "\xF0\x9F\x94\xB4 Low"
		}
		b.WriteString(fmt.Sprintf("<code>Profit F :</code> %.2f %s\n", stats.ProfitFactor, pfIcon))
	}
	if stats.AvgWinReturn1W != 0 || stats.AvgLossReturn1W != 0 {
		b.WriteString(fmt.Sprintf("<code>Avg Win  :</code> +%.2f%%\n", stats.AvgWinReturn1W))
		b.WriteString(fmt.Sprintf("<code>Avg Loss :</code> %.2f%%\n", stats.AvgLossReturn1W))
	}
	b.WriteString("\n")

	// Secondary metrics: Win rates
	b.WriteString("<b>Win Rates</b>\n")
	b.WriteString(fmt.Sprintf("<code>1W:</code> %.1f%% (n=%d) | <code>2W:</code> %.1f%% (n=%d) | <code>4W:</code> %.1f%% (n=%d)\n",
		stats.WinRate1W, stats.Evaluated1W,
		stats.WinRate2W, stats.Evaluated2W,
		stats.WinRate4W, stats.Evaluated4W))
	b.WriteString(fmt.Sprintf("<code>Best     :</code> %s at %.1f%%\n\n", stats.BestPeriod, stats.BestWinRate))

	b.WriteString("<b>Average Returns</b>\n")
	b.WriteString(fmt.Sprintf("<code>1W:</code> %.2f%% | <code>2W:</code> %.2f%% | <code>4W:</code> %.2f%%\n\n",
		stats.AvgReturn1W, stats.AvgReturn2W, stats.AvgReturn4W))

	// Risk-adjusted performance metrics
	if stats.SharpeRatio != 0 || stats.MaxDrawdown != 0 {
		b.WriteString("<b>Risk-Adjusted Metrics</b>\n")
		if stats.SharpeRatio != 0 {
			sharpeIcon := "\xE2\x9C\x85"
			if stats.SharpeRatio < 0.5 {
				sharpeIcon = "\xE2\x9A\xA0\xEF\xB8\x8F"
			}
			b.WriteString(fmt.Sprintf("<code>Sharpe   :</code> %.2f %s\n", stats.SharpeRatio, sharpeIcon))
		}
		if stats.MaxDrawdown != 0 {
			ddIcon := "\xE2\x9C\x85"
			if stats.MaxDrawdown > 10 {
				ddIcon = "\xE2\x9A\xA0\xEF\xB8\x8F"
			}
			b.WriteString(fmt.Sprintf("<code>Max DD   :</code> -%.2f%% %s\n", stats.MaxDrawdown, ddIcon))
		}
		if stats.CalmarRatio != 0 {
			b.WriteString(fmt.Sprintf("<code>Calmar   :</code> %.2f\n", stats.CalmarRatio))
		}
		if stats.KellyFraction != 0 {
			b.WriteString(fmt.Sprintf("<code>Kelly %%  :</code> %.1f%%\n", stats.KellyFraction*100))
		}
		b.WriteString("\n")
	}

	b.WriteString("<b>Strength Breakdown</b>\n")
	b.WriteString(fmt.Sprintf("<code>High (4-5):</code> %d signals, %.1f%% win\n", stats.HighStrengthCount, stats.HighStrengthWinRate))
	b.WriteString(fmt.Sprintf("<code>Low (1-3) :</code> %d signals, %.1f%% win\n\n", stats.LowStrengthCount, stats.LowStrengthWinRate))

	b.WriteString("<b>Confidence Calibration</b>\n")
	b.WriteString(fmt.Sprintf("<code>Stated   :</code> %.0f%%\n", stats.AvgConfidence))
	b.WriteString(fmt.Sprintf("<code>Actual   :</code> %.1f%%\n", stats.ActualAccuracy))

	calIcon := "\xE2\x9C\x85"
	if stats.CalibrationError > 15 {
		calIcon = "\xE2\x9A\xA0\xEF\xB8\x8F"
	}
	b.WriteString(fmt.Sprintf("<code>Error    :</code> %.1f%% %s\n", stats.CalibrationError, calIcon))

	// Brier score — lower is better
	if stats.BrierScore > 0 {
		brierIcon := "\xE2\x9C\x85" // checkmark — excellent (<0.15)
		if stats.BrierScore >= 0.25 {
			brierIcon = "\xF0\x9F\x94\xB4 Poor" // red circle — worse than random
		} else if stats.BrierScore >= 0.15 {
			brierIcon = "\xE2\x9A\xA0\xEF\xB8\x8F" // warning — decent but not great
		}
		b.WriteString(fmt.Sprintf("<code>Brier    :</code> %.4f %s\n", stats.BrierScore, brierIcon))
	}

	// Calibration method
	if stats.CalibrationMethod != "" {
		b.WriteString(fmt.Sprintf("<code>Method   :</code> %s\n", stats.CalibrationMethod))
	}

	// Statistical significance
	b.WriteString("\n<b>Statistical Significance</b>\n")
	if stats.Evaluated1W > 0 {
		if stats.IsStatisticallySignificant {
			b.WriteString("\xE2\x9C\x93 <b>Statistically Significant</b>\n")
		} else {
			b.WriteString("\xE2\x9A\xA0 <b>Insufficient Data</b>\n")
		}
		b.WriteString(fmt.Sprintf("<code>WR p-val :</code> %.4f\n", stats.WinRatePValue))
		b.WriteString(fmt.Sprintf("<code>WR 95%% CI:</code> [%.1f%%, %.1f%%]\n", stats.WinRateCI[0], stats.WinRateCI[1]))
		if stats.ReturnPValue < 1 {
			b.WriteString(fmt.Sprintf("<code>Ret t-stat:</code> %.2f (p=%.4f)\n", stats.ReturnTStat, stats.ReturnPValue))
		}
		if stats.Evaluated1W < stats.MinSamplesNeeded {
			b.WriteString(fmt.Sprintf("<code>Samples  :</code> %d / %d needed\n", stats.Evaluated1W, stats.MinSamplesNeeded))
		}
	} else {
		b.WriteString("\xE2\x9A\xA0 <b>Insufficient Data</b>\n")
		b.WriteString(fmt.Sprintf("<code>Need     :</code> %d+ evaluated signals\n", stats.MinSamplesNeeded))
	}

	return b.String()
}

// FormatBacktestSummary formats a map of BacktestStats into a comparison table.
func (f *Formatter) FormatBacktestSummary(statsMap map[string]*domain.BacktestStats, groupBy string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8A <b>Backtest by %s</b>\n\n", groupBy))

	// Sort keys for consistent output
	keys := make([]string, 0, len(statsMap))
	for k := range statsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-12s %4s %5s %5s %5s\n", "Group", "Eval", "1W", "2W", "4W"))
	b.WriteString(strings.Repeat("\xE2\x94\x80", 40) + "\n")

	for _, k := range keys {
		s := statsMap[k]
		label := s.GroupLabel
		if len(label) > 12 {
			label = label[:12]
		}
		b.WriteString(fmt.Sprintf("%-12s %4d %4.0f%% %4.0f%% %4.0f%%\n",
			label, s.Evaluated, s.WinRate1W, s.WinRate2W, s.WinRate4W))
	}
	b.WriteString("</pre>")

	return b.String()
}

// FormatSignalTiming formats per-signal-type timing analysis into Telegram HTML.
func (f *Formatter) FormatSignalTiming(analyses []backtestsvc.SignalTimingAnalysis) string {
	var b strings.Builder

	b.WriteString("\xE2\x8F\xB1 <b>Signal Timing Analysis</b>\n")
	b.WriteString("<i>Optimal horizon per signal type</i>\n\n")

	for _, a := range analyses {
		b.WriteString(fmt.Sprintf("<b>%s</b>\n", a.SignalType))
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-8s %5s %7s %6s %5s\n", "Horizon", "Win%", "AvgRet", "MaxDD", "R:R"))
		b.WriteString(strings.Repeat("\xe2\x94\x80", 36) + "\n")

		for _, h := range a.HorizonStats {
			marker := "  "
			if h.Horizon == a.OptimalHorizon {
				marker = "\xe2\x9e\xa4 "
			}
			rrStr := " -"
			if h.RiskRewardRatio > 0 {
				rrStr = fmt.Sprintf("%.1f", h.RiskRewardRatio)
			}
			ddStr := " -"
			if h.MaxDrawdown > 0 {
				ddStr = fmt.Sprintf("%.1f%%", h.MaxDrawdown)
			}
			if h.Evaluated == 0 {
				b.WriteString(fmt.Sprintf("%s%-5s %5s %7s %6s %5s\n",
					marker, h.Horizon, "-", "-", "-", "-"))
			} else {
				b.WriteString(fmt.Sprintf("%s%-5s %4.0f%% %+6.2f%% %6s %5s\n",
					marker, h.Horizon, h.WinRate, h.AvgReturn, ddStr, rrStr))
			}
		}
		b.WriteString("</pre>")

		// Recommendation line
		icon := "\xf0\x9f\x93\x8c" // pushpin
		if a.Degrading {
			icon = "\xe2\x9a\xa0\xef\xb8\x8f" // warning
		}
		b.WriteString(fmt.Sprintf("%s <i>%s</i>\n\n", icon, a.Recommendation))
	}

	return b.String()
}

// FormatWalkForward formats walk-forward analysis results into Telegram HTML.
func (f *Formatter) FormatWalkForward(result *backtestsvc.WalkForwardResult) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x94\xAC <b>Walk-Forward Analysis</b>\n")
	b.WriteString("<i>Train/test split to detect overfitting</i>\n\n")

	// Per-window table.
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-5s %6s %6s %6s %6s\n", "Win#", "Train", "Test", "Degr", "n(T/O)"))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 36) + "\n")

	for i, w := range result.Windows {
		degSign := ""
		if w.Degradation >= 0 {
			degSign = "+"
		}
		b.WriteString(fmt.Sprintf(" %2d   %5.1f%% %5.1f%% %s%.1f %3d/%-3d\n",
			i+1, w.InSampleWinRate, w.OutOfSampleWinRate,
			degSign, w.Degradation, w.InSampleCount, w.OutOfSampleCount))
	}
	b.WriteString("</pre>\n")

	// Window date ranges.
	b.WriteString("<i>Window periods:</i>\n")
	for i, w := range result.Windows {
		b.WriteString(fmt.Sprintf("<code>%d:</code> %s \xe2\x86\x92 %s | %s \xe2\x86\x92 %s\n",
			i+1,
			fmtutil.FormatDateShortWIB(w.TrainStart),
			fmtutil.FormatDateShortWIB(w.TrainEnd),
			fmtutil.FormatDateShortWIB(w.TestStart),
			fmtutil.FormatDateShortWIB(w.TestEnd)))
	}

	b.WriteString("\n")

	// Overall summary.
	b.WriteString("<b>Overall</b>\n")
	b.WriteString(fmt.Sprintf("<code>In-Sample WR  :</code> %.1f%%\n", result.OverallInSampleWinRate))
	b.WriteString(fmt.Sprintf("<code>Out-of-Sample :</code> %.1f%%\n", result.OverallOutOfSampleWinRate))

	// Traffic light for overfit score.
	var light string
	switch {
	case result.OverfitScore < 3:
		light = "\xF0\x9F\x9F\xA2 Go" // green
	case result.OverfitScore <= 10:
		light = "\xF0\x9F\x9F\xA1" // yellow
	default:
		light = "\xF0\x9F\x94\xB4 Stop" // red
	}
	b.WriteString(fmt.Sprintf("<code>Overfit Score :</code> %s %.1fpp\n", light, result.OverfitScore))

	if result.IsOverfit {
		b.WriteString("\n\xE2\x9A\xA0\xEF\xB8\x8F <b>OVERFITTING DETECTED</b>\n")
	}

	b.WriteString(fmt.Sprintf("\n\xF0\x9F\x93\x8C <i>%s</i>", result.Recommendation))

	return b.String()
}

// FormatWeightOptimization formats factor weight optimization results into Telegram HTML.
func (f *Formatter) FormatWeightOptimization(result *backtestsvc.WeightResult) string {
	var b strings.Builder

	b.WriteString("\xE2\x9A\x96\xEF\xB8\x8F <b>Factor Weight Optimization</b>\n")
	b.WriteString("<i>OLS regression: Return1W ~ COT + Stress + FRED + Price</i>\n\n")

	b.WriteString(fmt.Sprintf("<code>Sample Size   :</code> %d signals\n", result.SampleSize))
	b.WriteString(fmt.Sprintf("<code>R\xC2\xB2            :</code> %.4f\n", result.RSquared))
	b.WriteString(fmt.Sprintf("<code>Adj R\xC2\xB2        :</code> %.4f\n", result.AdjRSquared))

	// Weight comparison table.
	b.WriteString("\n<pre>")
	b.WriteString(fmt.Sprintf("%-10s %7s %7s %5s %6s\n", "Factor", "Current", "Optim.", "Sig?", "p-val"))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 42) + "\n")

	factorOrder := []string{"COT", "Stress", "FRED", "Price"}
	for _, name := range factorOrder {
		curr := 0.0
		if result.CurrentWeights != nil {
			curr = result.CurrentWeights[name]
		}
		opt := 0.0
		if result.OptimizedWeights != nil {
			opt = result.OptimizedWeights[name]
		}
		sig := " "
		if result.FactorSignificance != nil && result.FactorSignificance[name] {
			sig = "*"
		}
		pVal := 1.0
		if result.FactorPValues != nil {
			pVal = result.FactorPValues[name]
		}
		b.WriteString(fmt.Sprintf("%-10s %6.1f%% %6.1f%%   %s  %.3f\n",
			name, curr, opt, sig, pVal))
	}
	b.WriteString("</pre>\n")
	b.WriteString("<i>* = statistically significant (p &lt; 0.05)</i>\n")

	// Raw coefficients.
	if result.FactorCoefficients != nil {
		b.WriteString("\n<b>Raw Coefficients</b>\n<pre>")
		for _, name := range factorOrder {
			coeff := result.FactorCoefficients[name]
			b.WriteString(fmt.Sprintf("%-10s %+.4f\n", name, coeff))
		}
		b.WriteString("</pre>\n")
	}

	// Per-contract weights.
	if len(result.PerContractWeights) > 0 {
		b.WriteString("\n<b>Per-Currency Weights</b>\n<pre>")
		b.WriteString(fmt.Sprintf("%-5s %5s %5s %5s %5s\n", "Ccy", "COT", "Str", "FRED", "Prc"))
		b.WriteString(strings.Repeat("\xe2\x94\x80", 30) + "\n")

		// Sort currencies for deterministic output.
		var currencies []string
		for c := range result.PerContractWeights {
			currencies = append(currencies, c)
		}
		sort.Strings(currencies)

		for _, ccy := range currencies {
			w := result.PerContractWeights[ccy]
			b.WriteString(fmt.Sprintf("%-5s %4.0f%% %4.0f%% %4.0f%% %4.0f%%\n",
				ccy, w["COT"], w["Stress"], w["FRED"], w["Price"]))
		}
		b.WriteString("</pre>\n")
	}

	b.WriteString(fmt.Sprintf("\n\xF0\x9F\x93\x8C <i>%s</i>", result.Recommendation))

	return b.String()
}

// FormatSmartMoneyAccuracy formats smart money tracking accuracy per contract.
func (f *Formatter) FormatSmartMoneyAccuracy(results []backtestsvc.SmartMoneyAccuracy) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\xA7\xA0 <b>SMART MONEY TRACKING ACCURACY</b>\n")
	b.WriteString("<i>Does \"smart money\" actually predict price? (52-week analysis)</i>\n\n")

	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-5s %5s %5s %5s %5s %5s\n", "CCY", "1W", "2W", "4W", "Corr", "Edge"))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 38) + "\n")

	for _, r := range results {
		edgeIcon := "\xe2\x9c\x97"
		if r.Edge == "YES" {
			edgeIcon = "\xe2\x9c\x93"
		} else if r.Edge == "INSUFFICIENT" {
			edgeIcon = "?"
		}
		corrStr := fmt.Sprintf("%+.2f", r.Correlation)
		if math.IsNaN(r.Correlation) {
			corrStr = " N/A"
		}
		b.WriteString(fmt.Sprintf("%-5s %4.0f%% %4.0f%% %4.0f%% %5s  %s\n",
			r.Currency, r.Accuracy1W, r.Accuracy2W, r.Accuracy4W, corrStr, edgeIcon))
	}
	b.WriteString("</pre>\n")

	// Highlight best and worst
	if len(results) > 0 {
		best := results[0] // already sorted by BestAccuracy desc
		b.WriteString(fmt.Sprintf("\n\xF0\x9F\x8F\x86 <b>Most Reliable:</b> %s \xe2\x80\x94 %.0f%% at %s\n",
			best.Currency, best.BestAccuracy, best.BestHorizon))

		worst := results[len(results)-1]
		if worst.Edge == "NO" {
			b.WriteString(fmt.Sprintf("\xe2\x9a\xa0\xef\xb8\x8f <b>No Edge:</b> %s \xe2\x80\x94 %.0f%% (consider ignoring SM signals)\n",
				worst.Currency, worst.BestAccuracy))
		}
	}

	b.WriteString("\n<i>Edge = best horizon \xe2\x89\xa555%% with n\xe2\x89\xa510</i>\n")
	b.WriteString("<i>Corr = Pearson correlation (net change vs 1W price)</i>")

	return b.String()
}

// FormatExcursionSummary formats MFE/MAE analysis results.
func (f *Formatter) FormatExcursionSummary(s *backtestsvc.ExcursionSummary) string {
	var b strings.Builder

	b.WriteString("📊 <b>MFE/MAE EXCURSION ANALYSIS</b>\n")
	b.WriteString(fmt.Sprintf("<code>Signals Analyzed: %d</code>\n\n", s.TotalSignals))

	b.WriteString("<b>📏 Average Excursion</b>\n")
	b.WriteString(fmt.Sprintf("<code>Avg MFE : %+.2f%%</code> (max favorable move)\n", s.AvgMFEPct))
	b.WriteString(fmt.Sprintf("<code>Avg MAE : %+.2f%%</code> (max adverse move)\n", s.AvgMAEPct))
	b.WriteString(fmt.Sprintf("<code>Avg Optimal Return: %+.2f%%</code> (exit at best day)\n", s.AvgOptimalRet))
	b.WriteString(fmt.Sprintf("<code>Avg Optimal Day   : %.1f</code>\n", s.AvgOptimalDay))

	b.WriteString("\n<b>🎯 Signal Quality Diagnosis</b>\n")
	if s.MissedWins > 0 {
		b.WriteString(fmt.Sprintf("🔴 <b>%d missed wins</b> (%.0f%% of losses)\n", s.MissedWins, s.MissedWinPct))
		b.WriteString("<i>These signals were profitable intraweek but closed as losses.\n")
		b.WriteString("→ Signals are directionally correct, exit timing needs work.</i>\n")
	} else {
		b.WriteString("✅ No significant missed wins detected.\n")
	}

	b.WriteString("\n<b>📅 Best Exit Day Distribution</b>\n<pre>")
	for i := 0; i < len(s.OptimalDayDist); i++ {
		label := fmt.Sprintf("Day %d", i+1)
		bar := ""
		for j := 0; j < s.OptimalDayDist[i] && j < 20; j++ {
			bar += "█"
		}
		b.WriteString(fmt.Sprintf("%-5s %3d %s\n", label, s.OptimalDayDist[i], bar))
	}
	b.WriteString("</pre>")

	if len(s.BySignalType) > 0 {
		b.WriteString("\n<b>📋 By Signal Type</b>\n<pre>")
		b.WriteString("Type            MFE%  MAE%  MfWR% Day\n")
		b.WriteString("────────────────────────────────────\n")

		type typeEntry struct {
			name string
			ts   *backtestsvc.ExcursionTypeSummary
		}
		var entries []typeEntry
		for name, ts := range s.BySignalType {
			entries = append(entries, typeEntry{name, ts})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ts.MFEWinRate > entries[j].ts.MFEWinRate
		})

		for _, e := range entries {
			ts := e.ts
			shortName := e.name
			if len(shortName) > 15 {
				shortName = shortName[:15]
			}
			b.WriteString(fmt.Sprintf("%-15s %+5.1f %+5.1f %5.0f %3.0f\n",
				shortName, ts.AvgMFEPct, ts.AvgMAEPct, ts.MFEWinRate, ts.AvgOptimalDay))
		}
		b.WriteString("</pre>")
		b.WriteString("<i>MfWR = MFE Win Rate (% that moved >0.3% in signal direction)</i>\n")
	}

	return b.String()
}

// FormatTrendFilterStats formats daily trend filter analysis results.
func (f *Formatter) FormatTrendFilterStats(s *backtestsvc.TrendFilterStats) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x88 <b>DAILY TREND FILTER ANALYSIS</b>\n\n")

	if s.TotalSignals == 0 {
		b.WriteString("<i>No evaluated signals with daily trend data yet.</i>")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("<code>Total Signals :</code> %d\n", s.TotalSignals))
	b.WriteString(fmt.Sprintf("<code>With Filter   :</code> %d (%.0f%%)\n",
		s.FilteredSignals, float64(s.FilteredSignals)/float64(s.TotalSignals)*100))
	b.WriteString(fmt.Sprintf("<code>Avg Adjustment:</code> %+.1f%%\n\n", s.AvgAdjustment))

	// Alignment breakdown
	b.WriteString("<b>Trend Alignment vs Win Rate</b>\n")
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-10s %5s %7s\n", "Category", "Count", "Win 1W"))
	b.WriteString(fmt.Sprintf("%-10s %5d %6.1f%%\n", "Aligned", s.AlignedCount, s.AlignedWinRate1W))
	b.WriteString(fmt.Sprintf("%-10s %5d %6.1f%%\n", "Opposed", s.OpposedCount, s.OpposedWinRate1W))
	b.WriteString(fmt.Sprintf("%-10s %5d %6.1f%%\n", "Neutral", s.NeutralCount, s.NeutralWinRate1W))
	b.WriteString("</pre>\n")

	// Edge diagnosis
	edgeIcon := "\xE2\x9C\x85"
	if s.EdgeGain <= 0 {
		edgeIcon = "\xE2\x9A\xA0\xEF\xB8\x8F"
	}
	b.WriteString("<b>Edge Analysis</b>\n")
	b.WriteString(fmt.Sprintf("<code>Baseline 1W  :</code> %.1f%%\n", s.BaselineWinRate1W))
	b.WriteString(fmt.Sprintf("<code>Filtered Top :</code> %.1f%% (adj \xE2\x89\xA5 10)\n", s.FilteredWinRate1W))
	b.WriteString(fmt.Sprintf("<code>Edge Gain    :</code> %+.1f%% %s\n\n", s.EdgeGain, edgeIcon))

	// Confidence calibration
	b.WriteString("<b>Confidence Impact</b>\n")
	b.WriteString(fmt.Sprintf("<code>Avg Raw      :</code> %.1f%%\n", s.AvgRawConfidence))
	b.WriteString(fmt.Sprintf("<code>Avg Adjusted :</code> %.1f%%\n\n", s.AvgFinalConfidence))

	// By daily trend
	trends := s.SortedTrends()
	if len(trends) > 0 {
		b.WriteString("<b>By Daily Trend</b>\n")
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-6s %5s %7s %7s\n", "Trend", "Count", "Win 1W", "AvgAdj"))
		for _, t := range trends {
			b.WriteString(fmt.Sprintf("%-6s %5d %6.1f%% %+5.1f%%\n",
				t.Trend, t.Count, t.WinRate, t.AvgAdj))
		}
		b.WriteString("</pre>")
	}

	// Interpretation
	b.WriteString("\n<i>Aligned = daily trend confirms COT signal direction\n")
	b.WriteString("Opposed = daily trend contradicts COT signal\n")
	b.WriteString("Edge Gain = win rate improvement from filtering</i>")

	return b.String()
}
