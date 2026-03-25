package telegram

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// ---------------------------------------------------------------------------
// Intraday Context Formatter
// ---------------------------------------------------------------------------

// FormatIntradayContext formats an IntradayContext for Telegram display.
func (f *Formatter) FormatIntradayContext(ic *domain.IntradayContext) string {
	var b strings.Builder

	arrow := "→"
	if ic.Chg4H > 0 {
		arrow = "▲"
	} else if ic.Chg4H < 0 {
		arrow = "▼"
	}

	b.WriteString(fmt.Sprintf("⏰ <b>%s — 4H Context</b> %s\n", ic.Currency, arrow))
	b.WriteString(fmt.Sprintf("<code>Price: %s | As of %s UTC</code>\n\n",
		formatDailyPrice(ic.CurrentPrice, ic.Currency),
		ic.AsOf.Format("Jan 02 15:04")))

	// Short-term changes
	b.WriteString("<b>📊 Short-Term Changes</b>\n")
	b.WriteString(fmt.Sprintf("<code>4H   : %+.2f%%</code>\n", ic.Chg4H))
	b.WriteString(fmt.Sprintf("<code>12H  : %+.2f%%</code>\n", ic.Chg12H))
	b.WriteString(fmt.Sprintf("<code>24H  : %+.2f%%</code>\n", ic.Chg24H))

	// Intraday MAs
	b.WriteString("\n<b>📐 Intraday Moving Averages (4H)</b>\n")

	maLine := func(label string, ma float64, above bool) string {
		if ma == 0 {
			return fmt.Sprintf("<code>%s: N/A</code>", label)
		}
		icon := "✅"
		pos := "above"
		if !above {
			icon = "❌"
			pos = "below"
		}
		return fmt.Sprintf("<code>%s: %s</code> %s (%s)", label, formatDailyPrice(ma, ic.Currency), icon, pos)
	}

	b.WriteString(maLine("IMA8  (~32h)", ic.IMA8, ic.AboveIMA8) + "\n")
	b.WriteString(maLine("IMA21 (~3.5d)", ic.IMA21, ic.AboveIMA21) + "\n")
	b.WriteString(maLine("IMA55 (~9d)  ", ic.IMA55, ic.AboveIMA55) + "\n")

	// MA Trend
	maTrend := ic.IntradayMATrend()
	trendEmoji := "⚪"
	switch maTrend {
	case "BULLISH":
		trendEmoji = "🟢"
	case "BEARISH":
		trendEmoji = "🔴"
	}
	b.WriteString(fmt.Sprintf("<code>Alignment: %s</code> %s\n", maTrend, trendEmoji))

	// Volatility
	if ic.IntradayATR > 0 {
		b.WriteString("\n<b>📏 4H Volatility</b>\n")
		b.WriteString(fmt.Sprintf("<code>4H ATR : %s (%.3f%%)</code>\n",
			formatDailyPrice(ic.IntradayATR, ic.Currency), ic.NormalizedIATR))
	}

	// Session range
	if ic.SessionHigh > 0 {
		b.WriteString(fmt.Sprintf("<code>24H Hi : %s</code>\n", formatDailyPrice(ic.SessionHigh, ic.Currency)))
		b.WriteString(fmt.Sprintf("<code>24H Lo : %s</code>\n", formatDailyPrice(ic.SessionLow, ic.Currency)))
	}

	// Momentum
	b.WriteString("\n<b>🚀 Momentum</b>\n")
	b.WriteString(fmt.Sprintf("<code>6-bar  ROC: %+.2f%%</code>\n", ic.Momentum6))
	b.WriteString(fmt.Sprintf("<code>12-bar ROC: %+.2f%%</code>\n", ic.Momentum12))

	// Trend
	trendIcon := "➡️"
	switch ic.IntradayTrend {
	case "UP":
		trendIcon = "📈"
	case "DOWN":
		trendIcon = "📉"
	}
	b.WriteString(fmt.Sprintf("\n<code>4H Trend: %s</code> %s\n", ic.IntradayTrend, trendIcon))

	return b.String()
}

// ---------------------------------------------------------------------------
// Correlation Matrix Formatter
// ---------------------------------------------------------------------------

// FormatCorrelationMatrix formats a correlation matrix for Telegram display.
func (f *Formatter) FormatCorrelationMatrix(m *domain.CorrelationMatrix) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("🔗 <b>CORRELATION MATRIX (%d-day)</b>\n\n", m.Period))

	// Show compact matrix for first 8 currencies (FX)
	maxShow := 8
	if len(m.Currencies) < maxShow {
		maxShow = len(m.Currencies)
	}
	shown := m.Currencies[:maxShow]

	// Header row
	b.WriteString("<code>       ")
	for _, cur := range shown {
		b.WriteString(fmt.Sprintf("%-5s", truncLabel(cur, 3)))
	}
	b.WriteString("</code>\n")

	// Matrix rows
	for _, a := range shown {
		b.WriteString(fmt.Sprintf("<code>%-6s ", truncLabel(a, 3)))
		for _, c := range shown {
			corr := m.Matrix[a][c]
			b.WriteString(fmt.Sprintf("%+.1f ", corr))
		}
		b.WriteString("</code>\n")
	}

	// Cross-asset correlations (if more than 8 currencies)
	if len(m.Currencies) > 8 {
		b.WriteString("\n<b>📊 Cross-Asset vs FX</b>\n")
		for _, asset := range m.Currencies[8:] {
			var pairs []string
			for _, fx := range shown[:4] { // Show against top 4 FX
				if corr, ok := m.Matrix[asset][fx]; ok {
					icon := corrIcon(corr)
					pairs = append(pairs, fmt.Sprintf("%s:%+.2f%s", fx[:3], corr, icon))
				}
			}
			if len(pairs) > 0 {
				b.WriteString(fmt.Sprintf("<code>%-7s %s</code>\n", asset, strings.Join(pairs, " ")))
			}
		}
	}

	// Breakdowns
	if len(m.Breakdowns) > 0 {
		b.WriteString("\n<b>⚠️ Correlation Breakdowns</b>\n")
		limit := 5
		if len(m.Breakdowns) < limit {
			limit = len(m.Breakdowns)
		}
		for _, bd := range m.Breakdowns[:limit] {
			sevIcon := "🟡"
			if bd.Severity == "HIGH" {
				sevIcon = "🔴"
			}
			b.WriteString(fmt.Sprintf("%s <code>%s/%s: %.2f → %.2f (Δ%+.2f)</code>\n",
				sevIcon, bd.CurrencyA, bd.CurrencyB, bd.HistoricalCorr, bd.CurrentCorr, bd.Delta))
		}
	}

	return b.String()
}

// FormatCorrelationClusters formats correlation clusters for display.
func (f *Formatter) FormatCorrelationClusters(clusters []domain.CorrelationCluster) string {
	var b strings.Builder

	b.WriteString("<b>🔲 Correlation Clusters (r ≥ 0.70)</b>\n")
	for _, c := range clusters {
		b.WriteString(fmt.Sprintf("<code>%s: %s (avg r=%.2f)</code>\n",
			c.Name, strings.Join(c.Currencies, ", "), c.AvgCorr))
	}
	b.WriteString("\n<i>Avoid simultaneous exposure in same cluster</i>")

	return b.String()
}

func corrIcon(r float64) string {
	abs := math.Abs(r)
	switch {
	case abs >= 0.80:
		return "🔴" // Strong
	case abs >= 0.50:
		return "🟠" // Moderate
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Carry Trade / Rate Differential Formatter
// ---------------------------------------------------------------------------

// FormatCarryRanking formats the interest rate differential ranking.
func (f *Formatter) FormatCarryRanking(r *domain.CarryRanking) string {
	var b strings.Builder

	b.WriteString("🏦 <b>CARRY TRADE RANKING</b>\n")
	b.WriteString(fmt.Sprintf("<code>US Policy Rate: %.2f%% | %s</code>\n\n", r.USRate, r.AsOf))

	for i, p := range r.Pairs {
		// Position indicator
		medal := fmt.Sprintf("%d.", i+1)
		if i == 0 {
			medal = "🥇"
		} else if i == 1 {
			medal = "🥈"
		} else if i == 2 {
			medal = "🥉"
		}

		// Direction icon
		dirIcon := "🟢" // Positive carry (long)
		if p.Differential < 0 {
			dirIcon = "🔴" // Negative carry (short)
		}

		// Carry bar visualization
		bar := carryBar(p.CarryScore)

		b.WriteString(fmt.Sprintf("%s <code>%s/USD  Rate:%.2f%%  Diff:%+.2f%%</code> %s\n",
			medal, p.Currency, p.QuoteRate, p.Differential, dirIcon))
		b.WriteString(fmt.Sprintf("   <code>Carry: %s %+.0f</code>\n",
			bar, p.CarryScore))
	}

	b.WriteString("\n<b>📋 Summary</b>\n")
	b.WriteString(fmt.Sprintf("<code>Best Carry : %s (long = earn interest)</code>\n", r.BestCarry))
	b.WriteString(fmt.Sprintf("<code>Worst Carry: %s (long = pay interest)</code>\n", r.WorstCarry))

	b.WriteString("\n<i>Positive diff = earn carry going long XXX/USD</i>\n")
	b.WriteString("<i>Negative diff = pay carry going long XXX/USD</i>")

	return b.String()
}

// carryBar creates a visual bar for carry score (-100 to +100).
func carryBar(score float64) string {
	const width = 10
	filled := int(math.Abs(score) / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if score >= 0 {
		return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
	}
	return "[" + strings.Repeat("░", width-filled) + strings.Repeat("█", filled) + "]"
}

// truncLabel safely truncates a string to maxLen characters.
func truncLabel(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ---------------------------------------------------------------------------
// GARCH(1,1) Volatility Forecast Formatter
// ---------------------------------------------------------------------------

// FormatGARCH formats a GARCH(1,1) result for Telegram display.
func (f *Formatter) FormatGARCH(currency string, g *pricesvc.GARCHResult) string {
	var b strings.Builder

	fcastIcon := "⚪"
	switch g.VolForecast {
	case "INCREASING":
		fcastIcon = "🔴"
	case "DECREASING":
		fcastIcon = "🟢"
	}

	b.WriteString(fmt.Sprintf("📊 <b>%s — GARCH(1,1) Volatility</b> %s\n\n", currency, fcastIcon))

	// Convergence status
	if g.Converged {
		b.WriteString("<code>✓ Converged</code>\n")
	} else {
		b.WriteString("<code>⚠ NOT CONVERGED — estimates may be unreliable</code>\n")
	}
	if g.SampleSize > 0 {
		b.WriteString(fmt.Sprintf("<code>Samples       : %d</code>\n", g.SampleSize))
	}
	if g.LogLikelihood != 0 {
		b.WriteString(fmt.Sprintf("<code>Log-Likelihood: %.4f</code>\n", g.LogLikelihood))
	}

	// Model parameters
	b.WriteString("\n<b>🔧 Model Parameters</b>\n")
	b.WriteString(fmt.Sprintf("<code>α (shock)     : %.4f</code>\n", g.Alpha))
	b.WriteString(fmt.Sprintf("<code>β (persistence): %.4f</code>\n", g.Beta))
	b.WriteString(fmt.Sprintf("<code>α + β         : %.4f</code>\n", g.Persistence))

	// Volatility estimates
	b.WriteString("\n<b>📈 Volatility Estimates</b>\n")
	b.WriteString(fmt.Sprintf("<code>Current Vol   : %.4f%% (daily)</code>\n", g.CurrentVol*100))
	b.WriteString(fmt.Sprintf("<code>Long-Run Vol  : %.4f%% (daily)</code>\n", g.LongRunVol*100))
	b.WriteString(fmt.Sprintf("<code>Vol Ratio     : %.2fx</code>\n", g.VolRatio))

	// Interpretation
	ratioText := "at long-run average"
	if g.VolRatio > 1.25 {
		ratioText = "ABOVE average — elevated risk"
	} else if g.VolRatio < 0.75 {
		ratioText = "BELOW average — calm market"
	}
	b.WriteString(fmt.Sprintf("<code>              → %s</code>\n", ratioText))

	// Forecast
	b.WriteString("\n<b>🔮 Forward Forecast</b>\n")
	b.WriteString(fmt.Sprintf("<code>1-step Vol    : %.4f%%</code>\n", g.ForecastVol1*100))
	b.WriteString(fmt.Sprintf("<code>5-step Vol    : %.4f%%</code>\n", g.ForecastVol5*100))
	b.WriteString(fmt.Sprintf("<code>Direction     : %s</code> %s\n", g.VolForecast, fcastIcon))

	// Confidence impact
	mult := pricesvc.GARCHConfidenceMultiplier(g)
	multText := "neutral"
	if mult < 1.0 {
		multText = fmt.Sprintf("reduce confidence by %d%%", int((1-mult)*100))
	} else if mult > 1.0 {
		multText = fmt.Sprintf("boost confidence by %d%%", int((mult-1)*100))
	}
	b.WriteString(fmt.Sprintf("\n<code>Signal Impact : %.2fx (%s)</code>\n", mult, multText))

	b.WriteString("\n<i>GARCH provides forward-looking vol, complementing backward ATR</i>")

	return b.String()
}

// ---------------------------------------------------------------------------
// Hurst Exponent Formatter
// ---------------------------------------------------------------------------

// FormatHurst formats a Hurst exponent result for Telegram display.
func (f *Formatter) FormatHurst(currency string, h *pricesvc.HurstResult, regime *pricesvc.HurstRegimeContext) string {
	var b strings.Builder

	icon := "⚪"
	switch h.Classification {
	case "TRENDING":
		icon = "📈"
	case "MEAN_REVERTING":
		icon = "🔄"
	}

	b.WriteString(fmt.Sprintf("📐 <b>%s — Hurst Exponent</b> %s\n\n", currency, icon))

	// Core result
	b.WriteString("<b>📊 R/S Analysis</b>\n")
	b.WriteString(fmt.Sprintf("<code>H Exponent    : %.4f</code>\n", h.H))
	b.WriteString(fmt.Sprintf("<code>Classification: %s</code> %s\n", h.Classification, icon))
	b.WriteString(fmt.Sprintf("<code>Confidence    : %.1f%%</code>\n", h.Confidence))
	b.WriteString(fmt.Sprintf("<code>R²            : %.4f</code>\n", h.RSquared))
	b.WriteString(fmt.Sprintf("<code>Samples       : %d</code>\n", h.SampleSize))

	// Interpretation
	b.WriteString(fmt.Sprintf("\n<code>%s</code>\n", h.Description))

	// Trading implications
	b.WriteString("\n<b>💡 Trading Implications</b>\n")
	switch h.Classification {
	case "TRENDING":
		b.WriteString("<code>→ Momentum/trend-following strategies favored</code>\n")
		b.WriteString("<code>→ Breakouts more likely to sustain</code>\n")
		b.WriteString("<code>→ Mean-reversion entries risky</code>\n")
	case "MEAN_REVERTING":
		b.WriteString("<code>→ Range-trading & reversion strategies favored</code>\n")
		b.WriteString("<code>→ Extremes tend to snap back</code>\n")
		b.WriteString("<code>→ Breakout trades have lower edge</code>\n")
	default:
		b.WriteString("<code>→ No clear statistical edge from regime</code>\n")
		b.WriteString("<code>→ Rely on other signal sources</code>\n")
	}

	// Combined regime (if available)
	if regime != nil {
		b.WriteString("\n<b>🔄 Combined Regime (ADX + Hurst)</b>\n")
		b.WriteString(fmt.Sprintf("<code>ADX Regime    : %s (ADX %.1f)</code>\n", regime.PriceRegime.Regime, regime.PriceRegime.ADX))
		b.WriteString(fmt.Sprintf("<code>Hurst Regime  : %s</code>\n", regime.HurstRegime))

		agreeIcon := "✅"
		agreeText := "AGREE"
		if !regime.RegimeAgreement {
			agreeIcon = "⚠️"
			agreeText = "DISAGREE"
		}
		b.WriteString(fmt.Sprintf("<code>Agreement     : %s</code> %s\n", agreeText, agreeIcon))
		b.WriteString(fmt.Sprintf("<code>Combined Conf : %.1f%%</code>\n", regime.CombinedConfidence))
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// HMM Regime-Switching Formatter
// ---------------------------------------------------------------------------

// FormatHMMRegime formats an HMM regime result for Telegram display.
func (f *Formatter) FormatHMMRegime(currency string, h *pricesvc.HMMResult) string {
	var b strings.Builder

	if h == nil {
		b.WriteString(fmt.Sprintf("🔀 <b>%s — HMM Regime Model</b>\n\n", currency))
		b.WriteString("<code>Transition matrix unavailable</code>\n")
		return b.String()
	}

	icon := "⚪"
	switch h.CurrentState {
	case pricesvc.HMMRiskOn:
		icon = "🟢"
	case pricesvc.HMMRiskOff:
		icon = "🟡"
	case pricesvc.HMMCrisis:
		icon = "🔴"
	}

	b.WriteString(fmt.Sprintf("🔀 <b>%s — HMM Regime Model</b> %s\n\n", currency, icon))

	// Current state
	b.WriteString("<b>📊 Current Regime</b>\n")
	b.WriteString(fmt.Sprintf("<code>State     : %s</code> %s\n", h.CurrentState, icon))
	b.WriteString(fmt.Sprintf("<code>P(Risk-On): %.1f%%</code>\n", h.StateProbabilities[0]*100))
	b.WriteString(fmt.Sprintf("<code>P(Risk-Of): %.1f%%</code>\n", h.StateProbabilities[1]*100))
	b.WriteString(fmt.Sprintf("<code>P(Crisis) : %.1f%%</code>\n", h.StateProbabilities[2]*100))
	b.WriteString(fmt.Sprintf("<code>Samples   : %d</code>\n", h.SampleSize))
	b.WriteString(fmt.Sprintf("<code>Converged : %v (%d iter)</code>\n", h.Converged, h.Iterations))

	// Transition warning
	if h.TransitionWarning != "" {
		b.WriteString(fmt.Sprintf("\n⚠️ <b>%s</b>\n", h.TransitionWarning))
	}

	// Transition matrix
	b.WriteString("\n<b>🔄 Transition Probabilities</b>\n")
	labels := []string{"R_ON", "R_OF", "CRIS"}
	b.WriteString("<code>        R_ON  R_OF  CRIS</code>\n")
	for i, label := range labels {
		b.WriteString(fmt.Sprintf("<code>%-5s  %.2f  %.2f  %.2f</code>\n",
			label,
			h.TransitionMatrix[i][0],
			h.TransitionMatrix[i][1],
			h.TransitionMatrix[i][2],
		))
	}

	// Recent Viterbi path
	if len(h.ViterbiPath) > 0 {
		b.WriteString("\n<b>📈 Recent State Path</b>\n<code>")
		for i, state := range h.ViterbiPath {
			if i > 0 && i%5 == 0 {
				b.WriteString(" ")
			}
			switch state {
			case pricesvc.HMMRiskOn:
				b.WriteString("●")
			case pricesvc.HMMRiskOff:
				b.WriteString("○")
			case pricesvc.HMMCrisis:
				b.WriteString("✕")
			default:
				b.WriteString("?")
			}
		}
		b.WriteString("</code>\n")
		b.WriteString("<code>● Risk-On  ○ Risk-Off  ✕ Crisis</code>\n")
	}

	// Trading implications
	b.WriteString("\n<b>💡 Trading Implications</b>\n")
	mult := pricesvc.HMMConfidenceMultiplier(h)
	switch h.CurrentState {
	case pricesvc.HMMRiskOn:
		b.WriteString("<code>→ Trend-following signals more reliable</code>\n")
		b.WriteString("<code>→ Wider position sizing acceptable</code>\n")
	case pricesvc.HMMRiskOff:
		b.WriteString("<code>→ Reduce position sizes by 10%</code>\n")
		b.WriteString("<code>→ Prefer defensive/hedge signals</code>\n")
	case pricesvc.HMMCrisis:
		b.WriteString("<code>→ Reduce position sizes by 30%</code>\n")
		b.WriteString("<code>→ Avoid trend trades, prefer safe havens</code>\n")
	}
	b.WriteString(fmt.Sprintf("<code>Signal multiplier: %.2fx</code>\n", mult))

	return b.String()
}

// ---------------------------------------------------------------------------
// Factor Decomposition Formatter
// ---------------------------------------------------------------------------

// FormatFactorDecomposition formats a factor decomposition result for Telegram display.
func (f *Formatter) FormatFactorDecomposition(r *backtestsvc.DecompositionResult) string {
	var b strings.Builder

	b.WriteString("🧬 <b>FACTOR DECOMPOSITION</b>\n")
	b.WriteString(fmt.Sprintf("<code>R²: %.4f  Adj.R²: %.4f  n=%d</code>\n\n", r.RSquared, r.AdjRSquared, r.SampleSize))

	b.WriteString("<b>📊 Factor Contributions</b>\n")
	for _, fc := range r.Factors {
		sigIcon := " "
		if fc.IsSignificant {
			sigIcon = "★"
		}

		dirIcon := "→"
		switch fc.Direction {
		case "POSITIVE":
			dirIcon = "▲"
		case "NEGATIVE":
			dirIcon = "▼"
		}

		bar := factorBar(fc.PctExplained, r.RSquared*100)

		b.WriteString(fmt.Sprintf("<code>%s %-17s β:%+.4f %s</code>\n",
			sigIcon, fc.Name, fc.Coefficient, dirIcon))
		b.WriteString(fmt.Sprintf("<code>  Explained: %.1f%%  %s</code>\n",
			fc.PctExplained, bar))
		if fc.IsSignificant {
			b.WriteString(fmt.Sprintf("<code>  p=%.4f ★ significant</code>\n", fc.PValue))
		} else {
			b.WriteString(fmt.Sprintf("<code>  p=%.4f</code>\n", fc.PValue))
		}
	}

	// Residual
	b.WriteString(fmt.Sprintf("\n<code>Unexplained: %.1f%%</code>\n", r.ResidualPct))

	// Edge source
	b.WriteString(fmt.Sprintf("\n<b>🎯 Top Factor:</b> %s\n", r.TopFactor))
	b.WriteString(fmt.Sprintf("<b>💡 Edge Source:</b> %s\n", r.EdgeSource))

	// Per-currency breakdown
	if len(r.PerCurrency) > 0 {
		b.WriteString("\n<b>📋 Per-Currency Top Factor</b>\n")
		for cur, decomp := range r.PerCurrency {
			b.WriteString(fmt.Sprintf("<code>%-6s → %s (R²=%.3f, n=%d)</code>\n",
				cur, decomp.TopFactor, decomp.RSquared, decomp.SampleSize))
		}
	}

	return b.String()
}

// factorBar creates a visual bar for factor contribution.
func factorBar(pct, maxPct float64) string {
	const width = 10
	if maxPct <= 0 {
		return "[" + strings.Repeat("░", width) + "]"
	}
	filled := int(pct / maxPct * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// ---------------------------------------------------------------------------
// Walk-Forward Optimization Formatter
// ---------------------------------------------------------------------------

// FormatWFOptimization formats a walk-forward optimization result for Telegram display.
func (f *Formatter) FormatWFOptimization(r *backtestsvc.WFOResult) string {
	var b strings.Builder

	b.WriteString("🔄 <b>WALK-FORWARD OPTIMIZATION</b>\n")
	b.WriteString(fmt.Sprintf("<code>Windows: %d  Train: %dW  Test: %dW</code>\n\n", r.ValidWindows, 26, 4))

	// Aggregate optimal weights — iterate dynamically from the map
	b.WriteString("<b>📊 Optimized Weights (avg across windows)</b>\n")
	aggFactors := sortedMapKeys(r.AggregateWeights)
	for _, factor := range aggFactors {
		w := r.AggregateWeights[factor]
		bar := weightBar(w)
		b.WriteString(fmt.Sprintf("<code>%-8s %5.1f%% %s</code>\n", factor, w, bar))
	}

	// Per-regime weights
	if len(r.RegimeWeights) > 0 {
		b.WriteString("\n<b>🔀 Per-Regime Weights</b>\n")
		for regime, weights := range r.RegimeWeights {
			b.WriteString(fmt.Sprintf("<code>%s:</code>\n", regime))
			regFactors := sortedMapKeys(weights)
			for _, factor := range regFactors {
				w := weights[factor]
				b.WriteString(fmt.Sprintf("<code>  %-8s %5.1f%%</code>\n", factor, w))
			}
		}
	}

	// Performance comparison
	b.WriteString("\n<b>📈 Performance (OOS)</b>\n")
	b.WriteString(fmt.Sprintf("<code>Static V3 WR  : %.1f%%</code>\n", r.StaticOOSWinRate))
	b.WriteString(fmt.Sprintf("<code>Adaptive WR   : %.1f%%</code>\n", r.AdaptiveOOSWinRate))

	impIcon := "→"
	if r.Improvement > 1 {
		impIcon = "▲"
	} else if r.Improvement < -1 {
		impIcon = "▼"
	}
	b.WriteString(fmt.Sprintf("<code>Improvement   : %+.1fpp</code> %s\n", r.Improvement, impIcon))
	b.WriteString(fmt.Sprintf("<code>Avg OOS Return: %+.2f%%</code>\n", r.AvgOOSReturn))

	// Stability
	b.WriteString("\n<b>🔒 Stability</b>\n")
	stabIcon := "🟢"
	if r.WeightStability < 50 {
		stabIcon = "🔴"
	} else if r.WeightStability < 70 {
		stabIcon = "🟡"
	}
	b.WriteString(fmt.Sprintf("<code>Weight Stability: %.0f%%</code> %s\n", r.WeightStability, stabIcon))

	// Recommendation
	b.WriteString(fmt.Sprintf("\n<b>💡 Recommendation:</b>\n<code>%s</code>\n", r.Recommendation))

	return b.String()
}

// weightBar creates a visual bar for weight percentage.
func weightBar(pct float64) string {
	const width = 10
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// sortedMapKeys returns the keys of a map[string]float64 in sorted order.
func sortedMapKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
