package telegram

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
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
	b.WriteString(fmt.Sprintf("<code>Price: %s | As of %s</code>\n\n",
		formatDailyPrice(ic.CurrentPrice, ic.Currency),
		fmtutil.FormatDateTimeUTC(ic.AsOf)))

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
	trendEmoji := "⚪ Mixed"
	switch maTrend {
	case "BULLISH":
		trendEmoji = "🟢 Bullish"
	case "BEARISH":
		trendEmoji = "🔴 Bearish"
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

	return truncateMsg(b.String())
}

// ---------------------------------------------------------------------------
// Correlation Matrix Formatter
// ---------------------------------------------------------------------------

// FormatCorrelationMatrix formats a correlation matrix for Telegram display.
// Splits output into FX grid, cross-asset vs FX, and inter-asset correlations.
func (f *Formatter) FormatCorrelationMatrix(m *domain.CorrelationMatrix) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("🔗 <b>CORRELATION MATRIX (%d-day)</b>\n\n", m.Period))

	// Categorise currencies into FX vs non-FX.
	fxSet := map[string]bool{
		"EUR": true, "GBP": true, "JPY": true, "AUD": true,
		"NZD": true, "CAD": true, "CHF": true, "USD": true,
	}
	var fxCurrencies, crossAssets []string
	for _, cur := range m.Currencies {
		if fxSet[cur] {
			fxCurrencies = append(fxCurrencies, cur)
		} else {
			crossAssets = append(crossAssets, cur)
		}
	}

	// --- FX NxN Grid ---
	if len(fxCurrencies) > 0 {
		b.WriteString("<code>       ")
		for _, cur := range fxCurrencies {
			b.WriteString(fmt.Sprintf("%-5s", truncLabel(cur, 3)))
		}
		b.WriteString("</code>\n")
		for _, a := range fxCurrencies {
			b.WriteString(fmt.Sprintf("<code>%-6s ", truncLabel(a, 3)))
			for _, c := range fxCurrencies {
				corr := m.Matrix[a][c]
				if math.IsNaN(corr) {
					b.WriteString(" N/A ")
				} else {
					b.WriteString(fmt.Sprintf("%+.1f ", corr))
				}
			}
			b.WriteString("</code>\n")
		}
	}

	// --- Cross-Asset vs FX (top 4 FX pairs) ---
	if len(crossAssets) > 0 && len(fxCurrencies) > 0 {
		topFX := fxCurrencies[:min(4, len(fxCurrencies))]

		// Group cross-assets by category for readability.
		type assetGroup struct {
			label  string
			assets []string
		}
		groups := []assetGroup{
			{"Metals", filterPresent(crossAssets, []string{"XAU", "XAG", "COPPER"})},
			{"Energy", filterPresent(crossAssets, []string{"OIL", "ULSD", "RBOB"})},
			{"Bonds", filterPresent(crossAssets, []string{"BOND", "BOND30", "BOND5", "BOND2"})},
			{"Indices", filterPresent(crossAssets, []string{"SPX500", "NDX", "DJI", "RUT"})},
			{"Crypto", filterPresent(crossAssets, []string{"BTC", "ETH"})},
		}

		b.WriteString("\n<b>📊 Cross-Asset vs FX</b>\n")
		for _, g := range groups {
			if len(g.assets) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("<i>%s</i>\n", g.label))
			for _, asset := range g.assets {
				var pairs []string
				for _, fx := range topFX {
					if corr, ok := m.Matrix[asset][fx]; ok {
						if math.IsNaN(corr) {
							pairs = append(pairs, fmt.Sprintf("%s: N/A", truncLabel(fx, 3)))
						} else {
							icon := corrIcon(corr)
							pairs = append(pairs, fmt.Sprintf("%s:%+.2f%s", truncLabel(fx, 3), corr, icon))
						}
					}
				}
				if len(pairs) > 0 {
					b.WriteString(fmt.Sprintf("<code>%-7s %s</code>\n", asset, strings.Join(pairs, " ")))
				}
			}
		}
	}

	// --- Inter-Asset Correlations (notable non-FX pairs) ---
	if len(crossAssets) >= 2 {
		type corrPair struct {
			a, b string
			r    float64
		}
		var notable []corrPair
		for i := 0; i < len(crossAssets); i++ {
			for j := i + 1; j < len(crossAssets); j++ {
				a, bb := crossAssets[i], crossAssets[j]
				if corr, ok := m.Matrix[a][bb]; ok && !math.IsNaN(corr) && math.Abs(corr) >= 0.40 {
					notable = append(notable, corrPair{a, bb, corr})
				}
			}
		}
		// Sort by absolute correlation descending.
		sort.Slice(notable, func(i, j int) bool {
			return math.Abs(notable[i].r) > math.Abs(notable[j].r)
		})
		if len(notable) > 0 {
			b.WriteString("\n<b>🔗 Inter-Asset Correlations (|r| ≥ 0.40)</b>\n")
			limit := 10
			if len(notable) < limit {
				limit = len(notable)
			}
			for _, p := range notable[:limit] {
				icon := corrIcon(p.r)
				b.WriteString(fmt.Sprintf("<code>%s/%s: %+.2f</code>%s\n", p.a, p.b, p.r, icon))
			}
		}
	}

	// --- Breakdowns ---
	if len(m.Breakdowns) > 0 {
		b.WriteString("\n<b>⚠️ Correlation Breakdowns</b>\n")
		limit := 5
		if len(m.Breakdowns) < limit {
			limit = len(m.Breakdowns)
		}
		for _, bd := range m.Breakdowns[:limit] {
			sevIcon := "🟡 Medium"
			if bd.Severity == "HIGH" {
				sevIcon = "🔴 High"
			}
			b.WriteString(fmt.Sprintf("%s <code>%s/%s: %.2f → %.2f (Δ%+.2f)</code>\n",
				sevIcon, bd.CurrencyA, bd.CurrencyB, bd.HistoricalCorr, bd.CurrentCorr, bd.Delta))
		}
	}

	return truncateMsg(b.String())
}

// filterPresent returns the subset of candidates that exist in available.
func filterPresent(available, candidates []string) []string {
	set := make(map[string]bool, len(available))
	for _, a := range available {
		set[a] = true
	}
	var out []string
	for _, c := range candidates {
		if set[c] {
			out = append(out, c)
		}
	}
	return out
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

	return truncateMsg(b.String())
}

func corrIcon(r float64) string {
	abs := math.Abs(r)
	switch {
	case abs >= 0.80:
		return "🔴 Strong" // Strong correlation
	case abs >= 0.50:
		return "🟠 Moderate" // Moderate correlation
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
		dirIcon := "🟢 Long" // Positive carry (long)
		if p.Differential < 0 {
			dirIcon = "🔴 Short" // Negative carry (short)
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

	return truncateMsg(b.String())
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

// FormatGARCH formats a GARCH(1,1) result for Telegram display (human-friendly).
func (f *Formatter) FormatGARCH(currency string, g *pricesvc.GARCHResult) string {
	var b strings.Builder

	fcastIcon := "⚪ Stable"
	fcastLabel := "Stable"
	switch g.VolForecast {
	case "INCREASING":
		fcastIcon = "🔴 Rising"
		fcastLabel = "Getting wilder"
	case "DECREASING":
		fcastIcon = "🟢 Falling"
		fcastLabel = "Calming down"
	}

	b.WriteString(fmt.Sprintf("📊 <b>%s — Volatility Forecast</b> %s\n\n", currency, fcastIcon))

	// Convergence
	if !g.Converged {
		b.WriteString("<code>⚠ Model did not fully converge — take with a grain of salt</code>\n\n")
	}

	// Main summary (human-readable)
	b.WriteString("<b>📈 What's happening</b>\n")
	b.WriteString(fmt.Sprintf("<code>Daily swing    : ±%.2f%%</code>\n", g.CurrentVol*100))
	b.WriteString(fmt.Sprintf("<code>Normal swing   : ±%.2f%%</code>\n", g.LongRunVol*100))

	ratioText := "around normal"
	if g.VolRatio > 1.25 {
		ratioText = "ABOVE normal — extra caution"
	} else if g.VolRatio < 0.75 {
		ratioText = "BELOW normal — calm market"
	}
	b.WriteString(fmt.Sprintf("<code>Current vs norm: %.1fx (%s)</code>\n", g.VolRatio, ratioText))

	// Forecast
	b.WriteString(fmt.Sprintf("\n<b>🔮 Prediction: %s</b> %s\n", fcastLabel, fcastIcon))
	b.WriteString(fmt.Sprintf("<code>Tomorrow       : ±%.2f%%</code>\n", g.ForecastVol1*100))
	b.WriteString(fmt.Sprintf("<code>Next week      : ±%.2f%%</code>\n", g.ForecastVol5*100))

	// Signal impact
	mult := pricesvc.GARCHConfidenceMultiplier(g)
	if mult < 1.0 {
		b.WriteString(fmt.Sprintf("\n<code>⚠️ High volatility → reduce confidence by %d%%</code>\n", int((1-mult)*100)))
	} else if mult > 1.0 {
		b.WriteString(fmt.Sprintf("\n<code>✅ Low volatility → boost confidence by %d%%</code>\n", int((mult-1)*100)))
	} else {
		b.WriteString("\n<code>→ No impact on signal confidence</code>\n")
	}

	// Technical details (collapsed feel)
	b.WriteString(fmt.Sprintf("\n<i>Model: α=%.3f β=%.3f persistence=%.3f | %d samples</i>",
		g.Alpha, g.Beta, g.Persistence, g.SampleSize))

	return truncateMsg(b.String())
}

// ---------------------------------------------------------------------------
// Hurst Exponent Formatter
// ---------------------------------------------------------------------------

// FormatHurst formats a Hurst exponent result for Telegram display (human-friendly).
func (f *Formatter) FormatHurst(currency string, h *pricesvc.HurstResult, regime *pricesvc.HurstRegimeContext) string {
	var b strings.Builder

	icon := "⚪ Random"
	label := "Random (no clear pattern)"
	switch h.Classification {
	case "TRENDING":
		icon = "📈 Trending"
		label = "Trending — momentum carries forward"
	case "MEAN_REVERTING":
		icon = "🔄 Reverting"
		label = "Mean-reverting — extremes snap back"
	}

	b.WriteString(fmt.Sprintf("📐 <b>%s — Market Behaviour</b> %s\n\n", currency, icon))

	// Main result
	b.WriteString(fmt.Sprintf("<b>Result: %s</b>\n", label))
	b.WriteString(fmt.Sprintf("<code>Hurst H       : %.3f</code>\n", h.H))
	b.WriteString(fmt.Sprintf("<code>Confidence    : %.0f%%</code>\n", h.Confidence))
	b.WriteString(fmt.Sprintf("<code>Fit quality   : %.0f%%</code>\n", h.RSquared*100))

	// What it means
	b.WriteString("\n<b>💡 What this means</b>\n")
	switch h.Classification {
	case "TRENDING":
		b.WriteString("<code>→ Price moves tend to continue in same direction</code>\n")
		b.WriteString("<code>→ Good for: momentum & trend-following</code>\n")
		b.WriteString("<code>→ Bad for: buying dips / fading moves</code>\n")
	case "MEAN_REVERTING":
		b.WriteString("<code>→ Price moves tend to reverse / snap back</code>\n")
		b.WriteString("<code>→ Good for: range-trading & buying dips</code>\n")
		b.WriteString("<code>→ Bad for: chasing breakouts</code>\n")
	default:
		b.WriteString("<code>→ No reliable pattern detected</code>\n")
		b.WriteString("<code>→ Rely on other signal sources instead</code>\n")
	}

	// Combined regime (if available)
	if regime != nil {
		b.WriteString("\n<b>🔄 Cross-check with Price Trend (ADX)</b>\n")
		b.WriteString(fmt.Sprintf("<code>ADX says      : %s (ADX %.1f)</code>\n", regime.PriceRegime.Regime, regime.PriceRegime.ADX))
		b.WriteString(fmt.Sprintf("<code>Hurst says    : %s</code>\n", regime.HurstRegime))

		if regime.RegimeAgreement {
			b.WriteString("<code>Agreement     : ✅ Both agree — high confidence</code>\n")
		} else {
			b.WriteString("<code>Agreement     : ⚠️ Disagree — mixed signals</code>\n")
		}
		b.WriteString(fmt.Sprintf("<code>Combined conf : %.0f%%</code>\n", regime.CombinedConfidence))
	}

	b.WriteString(fmt.Sprintf("\n<i>Based on %d daily samples</i>", h.SampleSize))

	return truncateMsg(b.String())
}

// ---------------------------------------------------------------------------
// HMM Regime-Switching Formatter
// ---------------------------------------------------------------------------

// FormatHMMRegime formats an HMM regime result for Telegram display (human-friendly).
func (f *Formatter) FormatHMMRegime(currency string, h *pricesvc.HMMResult) string {
	var b strings.Builder

	if h == nil {
		b.WriteString(fmt.Sprintf("🔀 <b>%s — Market Regime</b>\n\n", currency))
		b.WriteString("<code>Not enough data to detect regime</code>\n")
		return truncateMsg(b.String())
	}

	icon := "⚪ Unknown"
	label := "Unknown"
	desc := ""
	switch h.CurrentState {
	case pricesvc.HMMRiskOn:
		icon = "🟢 Risk-On"
		label = "Risk-On (Calm)"
		desc = "Market is in a calm, confident phase. Trends tend to be reliable."
	case pricesvc.HMMRiskOff:
		icon = "🟡 Risk-Off"
		label = "Risk-Off (Cautious)"
		desc = "Market is getting defensive. Expect choppy action, reduce exposure."
	case pricesvc.HMMCrisis:
		icon = "🔴 Crisis"
		label = "Crisis (Panic)"
		desc = "Market is in stress mode. Correlations spike, safe havens outperform."
	case pricesvc.HMMTrending:
		icon = "🔵 Trending"
		label = "Trending (Directional)"
		desc = "Market is in a strong directional move with low vol. Ride the trend."
	}

	b.WriteString(fmt.Sprintf("🔀 <b>%s — Market Regime</b> %s\n\n", currency, icon))

	// Main result
	b.WriteString(fmt.Sprintf("<b>Current: %s</b>\n", label))
	b.WriteString(fmt.Sprintf("<code>%s</code>\n", desc))

	// Probabilities in plain language
	b.WriteString("\n<b>📊 How certain?</b>\n")
	b.WriteString(fmt.Sprintf("<code>Calm   : %.0f%%</code>\n", h.StateProbabilities[0]*100))
	b.WriteString(fmt.Sprintf("<code>Cautious: %.0f%%</code>\n", h.StateProbabilities[1]*100))
	b.WriteString(fmt.Sprintf("<code>Panic  : %.0f%%</code>\n", h.StateProbabilities[2]*100))
	b.WriteString(fmt.Sprintf("<code>Trending: %.0f%%</code>\n", h.StateProbabilities[3]*100))

	// Transition warning
	if h.TransitionWarning != "" {
		b.WriteString(fmt.Sprintf("\n⚠️ <b>%s</b>\n", h.TransitionWarning))
	}

	// Trading impact
	b.WriteString("\n<b>💡 What to do</b>\n")
	mult := pricesvc.HMMConfidenceMultiplier(h)
	switch h.CurrentState {
	case pricesvc.HMMRiskOn:
		b.WriteString("<code>→ Trade normally, trends are reliable</code>\n")
	case pricesvc.HMMRiskOff:
		b.WriteString("<code>→ Reduce position sizes by 10%</code>\n")
		b.WriteString("<code>→ Prefer defensive signals</code>\n")
	case pricesvc.HMMCrisis:
		b.WriteString("<code>→ Reduce position sizes by 30%</code>\n")
		b.WriteString("<code>→ Focus on safe havens (JPY, CHF, XAU)</code>\n")
	}
	b.WriteString(fmt.Sprintf("<code>Signal multiplier: %.2fx</code>\n", mult))

	// Recent state path (visual)
	if len(h.ViterbiPath) > 0 {
		b.WriteString("\n<b>📈 Recent Path</b>\n<code>")
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
		b.WriteString("<code>● Calm  ○ Cautious  ✕ Panic</code>\n")
	}

	// Technical footnote
	if !h.Converged {
		b.WriteString("\n<i>⚠ Model not fully converged — take with caution</i>")
	}
	b.WriteString(fmt.Sprintf("\n<i>%d samples, %d iterations</i>", h.SampleSize, h.Iterations))

	return truncateMsg(b.String())
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

	return truncateMsg(b.String())
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
	stabIcon := "🟢 Stable"
	if r.WeightStability < 50 {
		stabIcon = "🔴 Unstable"
	} else if r.WeightStability < 70 {
		stabIcon = "🟡 Moderate"
	}
	b.WriteString(fmt.Sprintf("<code>Weight Stability: %.0f%%</code> %s\n", r.WeightStability, stabIcon))

	// Recommendation
	b.WriteString(fmt.Sprintf("\n<b>💡 Recommendation:</b>\n<code>%s</code>\n", r.Recommendation))

	return truncateMsg(b.String())
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

// ---------------------------------------------------------------------------
// FormatVolCone — Volatility Cone Analysis formatter
// ---------------------------------------------------------------------------

// FormatVolCone formats a VolCone result for Telegram HTML output.
func (f *Formatter) FormatVolCone(cone *pricesvc.VolCone) string {
	if cone == nil {
		return "⚠️ <b>Volatility Cone</b>\n<code>Insufficient history (need ≥150 trading days)</code>\n"
	}

	var b strings.Builder
	anomalyIcon := "🟢 Normal"
	if cone.IsAnomaly {
		anomalyIcon = "🔴 Anomaly"
	}
	b.WriteString(fmt.Sprintf("📐 <b>VOLATILITY CONE — %s</b> %s\n", cone.Symbol, anomalyIcon))
	b.WriteString(fmt.Sprintf("<code>%s</code>\n\n", cone.AsOf.Format("02 Jan 2006")))

	for _, w := range cone.Windows {
		b.WriteString(volConeWindowBlock(w))
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("<b>💡 %s</b>\n", cone.Summary))
	return truncateMsg(b.String())
}

// volConeWindowBlock renders a single window as an ASCII band visualization.
func volConeWindowBlock(w *pricesvc.VolConeWindow) string {
	var b strings.Builder

	icon := "📊"
	if w.IsAnomaly && w.AnomalyDir == "HIGH" {
		icon = "⚠️"
	} else if w.IsAnomaly && w.AnomalyDir == "LOW" {
		icon = "🔵"
	}

	label := fmt.Sprintf("%dd", w.Window)
	b.WriteString(fmt.Sprintf("%s <b>%s Window</b>  <code>%.1f%%</code>  P%.0f\n",
		icon, label, w.CurrentVol, w.Percentile))

	// ASCII cone bar: position of current vol in P5..P95 range
	bar := volConeBar(w.CurrentVol, w.P5, w.P95, 16)
	b.WriteString(fmt.Sprintf("<code>P5 %s P95</code>\n", bar))
	b.WriteString(fmt.Sprintf("<code>   P25=%.1f%%  P50=%.1f%%  P75=%.1f%%</code>\n",
		w.P25, w.P50, w.P75))

	return b.String()
}

// volConeBar renders an ASCII bar showing where current vol sits in [lo, hi].
func volConeBar(current, lo, hi float64, width int) string {
	if hi <= lo || width < 4 {
		return strings.Repeat("-", width)
	}
	pos := int(math.Round((current - lo) / (hi - lo) * float64(width-1)))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}
	bar := make([]rune, width)
	for i := range bar {
		bar[i] = '-'
	}
	bar[pos] = '^'
	return string(bar)
}

// ---------------------------------------------------------------------------
// GJR-GARCH(1,1) Asymmetric Volatility Formatter
// ---------------------------------------------------------------------------

// FormatGJRGARCH formats a GJR-GARCH(1,1) result for Telegram display.
func (f *Formatter) FormatGJRGARCH(currency string, g *pricesvc.GJRGARCHResult) string {
	var b strings.Builder

	asymIcon := "⚪"
	switch g.AsymmetryLabel {
	case "HIGH":
		asymIcon = "🔴"
	case "MODERATE":
		asymIcon = "🟡"
	case "LOW":
		asymIcon = "🟢"
	}

	b.WriteString(fmt.Sprintf("📊 <b>%s — GJR-GARCH(1,1) Asymmetric Vol</b> %s\n\n", currency, asymIcon))

	if !g.Converged {
		b.WriteString("<code>⚠ Model tidak konvergen — gunakan dengan hati-hati</code>\n\n")
	}

	b.WriteString("<b>📈 Volatilitas saat ini</b>\n")
	b.WriteString(fmt.Sprintf("<code>Current Vol    : ±%.2f%%/hari</code>\n", g.CurrentVol*100))
	b.WriteString(fmt.Sprintf("<code>Forecast 1-step: ±%.2f%%/hari</code>\n", g.ForecastVol1*100))

	b.WriteString(fmt.Sprintf("\n<b>⚡ Leverage Effect: %s</b> %s\n", g.AsymmetryLabel, asymIcon))
	b.WriteString(fmt.Sprintf("<code>Asym. Ratio    : %.0f%% shock dari downside</code>\n", g.AsymmetryRatio))

	if g.LeverageEffect {
		b.WriteString("<code>→ Downside risk elevated — pertimbangkan kurangi long size</code>\n")
	} else {
		b.WriteString("<code>→ Volatilitas relatif simetris</code>\n")
	}

	b.WriteString(fmt.Sprintf("\n<i>Model: α=%.3f γ=%.3f β=%.3f persist=%.3f | %d samples</i>",
		g.Alpha, g.Gamma, g.Beta, g.Persistence, g.SampleSize))

	return truncateMsg(b.String())
}
