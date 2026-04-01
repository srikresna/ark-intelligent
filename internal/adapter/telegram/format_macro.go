package telegram

import (
	"fmt"
	"sort"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)


// regimeAdvisory returns a short advisory note based on the macro regime.
func regimeAdvisory(regimeName string) string {
	switch regimeName {
	case "STRESS":
		return "Safe-haven demand elevated — JPY/CHF/Gold favored"
	case "RECESSION":
		return "Recession risk — defensive FX (JPY/CHF) and Gold over commodity FX"
	case "INFLATIONARY":
		return "Inflationary regime — USD bias bullish; AUD/NZD/CAD under pressure"
	case "DISINFLATIONARY":
		return "Disinflation — risk-on tilt; commodity FX and EUR/GBP may benefit"
	case "STAGFLATION":
		return "Stagflation — Gold bullish; equities and commodity FX bearish"
	case "GOLDILOCKS":
		return "Goldilocks — risk appetite favors AUD/NZD/CAD; USD mild bearish"
	default:
		return ""
	}
}

// FormatMacroRegime formats the FRED macro regime dashboard message.
// P3.2 — /macro command output. Now includes trend arrows, Sahm Rule, 3M-10Y spread,
// SOFR/IORB, Fed balance sheet, and M2 YoY growth.
func (f *Formatter) FormatMacroRegime(regime fred.MacroRegime, data *fred.MacroData) string {
	var b strings.Builder

	riskBar := buildRiskBar(regime.Score, 15)

	b.WriteString("🏦 <b>MACRO REGIME DASHBOARD</b>\n")
	b.WriteString(fmt.Sprintf("<i>FRED Data — Updated %s</i>\n\n", fmtutil.FormatDateTimeWIB(data.FetchedAt)))
	b.WriteString(fmt.Sprintf("<b>REGIME: %s</b>  Risk: %d/100\n", regime.Name, regime.Score))
	b.WriteString(fmt.Sprintf("<code>[%s]</code>\n", riskBar))
	b.WriteString(fmt.Sprintf("<code>Recession Risk: %s</code>\n\n", regime.RecessionRisk))

	// --- Yield Curve ---
	b.WriteString("<code>━━━ Treasury Yield Curve ━━━</code>\n")
	b.WriteString("<code> 3M    2Y    5Y   10Y   30Y</code>\n")
	b.WriteString(fmt.Sprintf("<code>%4.2f  %4.2f  %4.2f  %4.2f  %4.2f</code>\n",
		data.Yield3M, data.Yield2Y, data.Yield5Y, data.Yield10Y, data.Yield30Y))
	b.WriteString(fmt.Sprintf("<code>2Y-10Y Spread: %s</code>\n", regime.YieldCurve))
	if regime.Yield3M10Y != "N/A" && regime.Yield3M10Y != "" {
		b.WriteString(fmt.Sprintf("<code>3M-10Y Spread: %s</code>\n", regime.Yield3M10Y))
	}
	if regime.Yield2Y30Y != "N/A" && regime.Yield2Y30Y != "" {
		b.WriteString(fmt.Sprintf("<code>2Y-30Y Spread: %s</code>\n", regime.Yield2Y30Y))
	}

	// --- Inflation ---
	b.WriteString(fmt.Sprintf("\n<code>Core PCE     : %s</code>\n", regime.Inflation))
	if data.Breakeven5Y > 0 {
		realRate := data.FedFundsRate - data.Breakeven5Y
		b.WriteString(fmt.Sprintf("<code>10Y Breakeven: %.2f%% | Real Rate: %+.2f%%</code>\n", data.Breakeven5Y, realRate))
	}
	if regime.M2Label != "N/A" && regime.M2Label != "" {
		b.WriteString(fmt.Sprintf("<code>M2 Supply    : %s</code>\n", regime.M2Label))
	}

	// --- Monetary Policy ---
	if data.FedFundsRate > 0 {
		b.WriteString(fmt.Sprintf("\n<code>Mon. Policy  : %s</code>\n", regime.MonPolicy))
	}
	if regime.SOFRLabel != "N/A" && regime.SOFRLabel != "" {
		b.WriteString(fmt.Sprintf("<code>SOFR/IORB    : %s</code>\n", regime.SOFRLabel))
	}
	if regime.FedBalance != "N/A" && regime.FedBalance != "" {
		b.WriteString(fmt.Sprintf("<code>Fed Balance  : %s</code>\n", regime.FedBalance))
	}
	if regime.TGALabel != "N/A" && regime.TGALabel != "" {
		b.WriteString(fmt.Sprintf("<code>TGA Balance  : %s</code>\n", regime.TGALabel))
	}
	if regime.LiquidityLabel != "" {
		b.WriteString(fmt.Sprintf("<code>Net Liquidity: %s</code>\n", regime.LiquidityLabel))
	}

	// --- Financial Stress ---
	b.WriteString(fmt.Sprintf("\n<code>Fin. Stress  : %s</code>\n", regime.FinStress))

	// --- Labor + Sahm ---
	b.WriteString(fmt.Sprintf("\n<code>Labor Market : %s</code>\n", regime.Labor))
	if regime.SahmAlert {
		b.WriteString(fmt.Sprintf("<code>Sahm Rule    : %s</code> ← 🚨 RECESSION SIGNAL\n", regime.SahmLabel))
	} else if regime.SahmLabel != "N/A" && regime.SahmLabel != "" {
		b.WriteString(fmt.Sprintf("<code>Sahm Rule    : %s</code>\n", regime.SahmLabel))
	}

	// --- Growth ---
	if regime.Growth != "N/A" && regime.Growth != "" {
		b.WriteString(fmt.Sprintf("\n<code>GDP Growth   : %s</code>\n", regime.Growth))
	}

	// --- USD ---
	if regime.USDStrength != "N/A" && regime.USDStrength != "" {
		b.WriteString(fmt.Sprintf("<code>USD Strength : %s</code>\n", regime.USDStrength))
	}

	b.WriteString(fmt.Sprintf("\n→ <b>%s</b>\n", regime.Bias))
	b.WriteString(fmt.Sprintf("<i>%s</i>\n", regime.Description))

	// Cache age hint
	age := fred.CacheAge()
	cacheNote := "live fetch"
	if age >= 0 {
		cacheNote = fmt.Sprintf("cached %dm ago", int(age.Minutes()))
	}
	b.WriteString(fmt.Sprintf("\n<i>St. Louis FRED (%s) | </i><code>/macro refresh</code><i> to force-update | </i><code>/outlook fred</code><i> for AI analysis</i>", cacheNote))
	return b.String()
}

// FormatRegimeAssetInsight formats the historical regime-asset performance
// section for the macro dashboard. Shows top 3 best and worst assets.
func (f *Formatter) FormatRegimeAssetInsight(insight fred.RegimeInsight) string {
	if len(insight.BestAssets) == 0 && len(insight.WorstAssets) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n📊 <b>Historical Performance in %s Regime</b>\n", insight.Regime))

	if len(insight.BestAssets) > 0 {
		b.WriteString("<code>Best performers:</code>\n")
		for _, a := range insight.BestAssets {
			arrow := "🟢"
			if a.AnnualizedReturn < 0 {
				arrow = "🔴"
			}
			b.WriteString(fmt.Sprintf("<code>  %s %s  %+.1f%% ann. (%dw)</code>\n",
				arrow, a.Currency, a.AnnualizedReturn, a.Occurrences))
		}
	}

	if len(insight.WorstAssets) > 0 {
		b.WriteString("<code>Worst performers:</code>\n")
		for _, a := range insight.WorstAssets {
			arrow := "🟢"
			if a.AnnualizedReturn < 0 {
				arrow = "🔴"
			}
			b.WriteString(fmt.Sprintf("<code>  %s %s  %+.1f%% ann. (%dw)</code>\n",
				arrow, a.Currency, a.AnnualizedReturn, a.Occurrences))
		}
	}

	return b.String()
}

// FormatFREDContext formats a compact FRED macro context block for COT detail view.
// Shows the most tradable macro filters relevant to currency positioning.
func (f *Formatter) FormatFREDContext(data *fred.MacroData, regime fred.MacroRegime) string {
	if data == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n🏦 <b>FRED Macro Context:</b>\n")

	// Line 1: Core macro trinity
	realRate := data.FedFundsRate - data.Breakeven5Y
	b.WriteString(fmt.Sprintf("<code>DXY: %.1f | Real Rate: %+.2f%% | NFCI: %.3f %s</code>\n",
		data.DXY, realRate, data.NFCI, data.NFCITrend.Arrow()))

	// Line 2: Regime + bias (truncated for space)
	b.WriteString(fmt.Sprintf("<code>Regime: %s | Score: %d/100</code>\n", regime.Name, regime.Score))
	b.WriteString(fmt.Sprintf("<i>→ %s</i>\n", regime.Bias))

	// Line 3: Alert flags
	if regime.SahmAlert {
		b.WriteString("🚨 <b>SAHM RULE TRIGGERED — Recession risk HIGH!</b>\n")
	}
	if data.Spread3M10Y < 0 && data.Spread3M10Y != 0 {
		b.WriteString(fmt.Sprintf("🔴 <code>3M-10Y INVERTED: %.2f%% (recession predictor)</code>\n", data.Spread3M10Y))
	}
	if data.YieldSpread < 0 {
		b.WriteString(fmt.Sprintf("⚠️ <code>2Y-10Y INVERTED: %.2f%%</code>\n", data.YieldSpread))
	}

	return b.String()
}

// buildRiskBar creates a visual risk score bar (higher = more risk-off).
func buildRiskBar(score, width int) string {
	filled := score * width / 100
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	var label string
	switch {
	case score >= 70:
		label = " HIGH RISK"
	case score >= 40:
		label = " MODERATE"
	default:
		label = " LOW RISK"
	}

	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + label
}

// compositeIcon returns an emoji based on score thresholds.
// For scores where higher = worse (like credit stress), swap good/bad thresholds.
func compositeIcon(score, badThreshold, warnThreshold float64) string {
	if score >= badThreshold {
		return "🔴"
	}
	if score >= warnThreshold {
		return "⚠️"
	}
	return "✅"
}

// FormatMacroComposites formats the composite scores section for /macro COMPOSITES view.
func (f *Formatter) FormatMacroComposites(composites *domain.MacroComposites, data *fred.MacroData) string {
	if composites == nil {
		return "<b>Composite Scores</b>\n\nNo composite data available. Run /macro to fetch first."
	}

	var b strings.Builder
	b.WriteString("<b>📊 MACRO COMPOSITE SCORES</b>\n")
	b.WriteString(fmt.Sprintf("<i>Computed: %s</i>\n\n", fmtutil.FormatDateTimeWIB(composites.ComputedAt)))

	// Labor Health
	laborIcon := compositeIcon(composites.LaborHealth, 60, 40)
	b.WriteString(fmt.Sprintf("👷 <b>Labor Health:</b> %.0f/100 %s %s\n", composites.LaborHealth, composites.LaborLabel, laborIcon))

	// Inflation Momentum
	inflIcon := "→"
	if composites.InflationMomentum > 0.2 {
		inflIcon = "↑"
	} else if composites.InflationMomentum < -0.2 {
		inflIcon = "↓"
	}
	inflEmoji := "✅"
	if composites.InflationMomentum > 0.5 {
		inflEmoji = "🔴"
	} else if composites.InflationMomentum > 0.2 {
		inflEmoji = "⚠️"
	}
	b.WriteString(fmt.Sprintf("🔥 <b>Inflation:</b> %+.2f %s %s %s\n", composites.InflationMomentum, composites.InflationLabel, inflIcon, inflEmoji))

	// Yield Curve
	curveEmoji := "✅"
	switch composites.YieldCurveSignal {
	case "DEEP_INVERSION":
		curveEmoji = "🔴"
	case "INVERTED":
		curveEmoji = "🔴"
	case "FLAT":
		curveEmoji = "⚠️"
	case "STEEP":
		curveEmoji = "🟢"
	}
	b.WriteString(fmt.Sprintf("📈 <b>Yield Curve:</b> %s %s\n", composites.YieldCurveSignal, curveEmoji))

	// Credit Stress
	creditIcon := compositeIcon(composites.CreditStress, 60, 40)
	b.WriteString(fmt.Sprintf("💳 <b>Credit Stress:</b> %.0f/100 %s %s\n", composites.CreditStress, composites.CreditLabel, creditIcon))

	// Housing Pulse
	housingEmoji := "→"
	switch composites.HousingPulse {
	case "EXPANDING":
		housingEmoji = "🟢"
	case "CONTRACTING":
		housingEmoji = "⚠️"
	case "COLLAPSING":
		housingEmoji = "🔴"
	case "N/A":
		housingEmoji = "—"
	}
	b.WriteString(fmt.Sprintf("🏠 <b>Housing:</b> %s %s\n", composites.HousingPulse, housingEmoji))

	// Financial Conditions
	finLabel := "NEUTRAL"
	finEmoji := "→"
	if composites.FinConditions > 0.3 {
		finLabel = "LOOSE"
		finEmoji = "🟢"
	} else if composites.FinConditions < -0.3 {
		finLabel = "TIGHT"
		finEmoji = "🔴"
	}
	b.WriteString(fmt.Sprintf("🏦 <b>Fin. Conditions:</b> %+.2f %s %s\n", composites.FinConditions, finLabel, finEmoji))

	// VIX Term Structure
	vixEmoji := "✅"
	if composites.VIXTermRegime == "BACKWARDATION" {
		vixEmoji = "🔴"
	} else if composites.VIXTermRegime == "FLAT" {
		vixEmoji = "⚠️"
	}
	if composites.VIXTermRatio > 0 {
		b.WriteString(fmt.Sprintf("📉 <b>VIX Term:</b> %s (%.2f) %s\n", composites.VIXTermRegime, composites.VIXTermRatio, vixEmoji))
	} else {
		b.WriteString(fmt.Sprintf("📉 <b>VIX Term:</b> %s %s\n", composites.VIXTermRegime, vixEmoji))
	}

	// Sentiment
	sentEmoji := "🟡"
	if composites.SentimentComposite > 30 {
		sentEmoji = "🟢" // fear = contrarian bullish
	} else if composites.SentimentComposite < -30 {
		sentEmoji = "🔴" // greed = contrarian bearish
	}
	b.WriteString(fmt.Sprintf("🧭 <b>Sentiment:</b> %+.0f %s %s\n", composites.SentimentComposite, composites.SentimentLabel, sentEmoji))

	// Global Macro Comparison
	b.WriteString("\n<b>🌍 RELATIVE MACRO STRENGTH</b>\n")

	type countryEntry struct {
		code  string
		score float64
	}
	countries := []countryEntry{
		{"USD", composites.USScore},
		{"EUR", composites.EZScore},
		{"GBP", composites.UKScore},
		{"JPY", composites.JPScore},
		{"AUD", composites.AUScore},
		{"CAD", composites.CAScore},
		{"NZD", composites.NZScore},
	}

	// Sort by score descending
	sort.Slice(countries, func(i, j int) bool {
		return countries[i].score > countries[j].score
	})

	for i, c := range countries {
		icon := "🟡"
		if c.score > 20 {
			icon = "🟢"
		} else if c.score < -20 {
			icon = "🔴"
		}
		rank := i + 1
		b.WriteString(fmt.Sprintf(" %d. %s %s %+.0f\n", rank, icon, c.code, c.score))
	}

	return b.String()
}

// FormatMacroGlobal formats the global macro comparison table.
func (f *Formatter) FormatMacroGlobal(composites *domain.MacroComposites, data *fred.MacroData) string {
	if composites == nil || data == nil {
		return "<b>Global Macro</b>\n\nNo data available."
	}

	var b strings.Builder
	b.WriteString("<b>🌍 GLOBAL MACRO COMPARISON</b>\n\n")

	type row struct {
		code  string
		score float64
		cpi   float64
		gdp   float64
		unemp float64
		rate  float64
	}

	rows := []row{
		{"USD", composites.USScore, data.CorePCE, data.GDPGrowth, data.UnemployRate, data.FedFundsRate},
		{"EUR", composites.EZScore, data.EZ_CPI, data.EZ_GDP, data.EZ_Unemployment, data.EZ_Rate},
		{"GBP", composites.UKScore, data.UK_CPI, 0, data.UK_Unemployment, 0},
		{"JPY", composites.JPScore, data.JP_CPI, 0, data.JP_Unemployment, data.JP_10Y},
		{"AUD", composites.AUScore, data.AU_CPI, 0, data.AU_Unemployment, 0},
		{"CAD", composites.CAScore, data.CA_CPI, 0, data.CA_Unemployment, 0},
		{"NZD", composites.NZScore, data.NZ_CPI, 0, 0, 0},
	}

	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-4s %5s  %5s %5s %5s %5s\n", "", "Score", "CPI", "GDP", "Jobs", "Rate"))
	b.WriteString("─────────────────────────────────\n")

	for _, r := range rows {
		icon := "🟡"
		if r.score > 20 {
			icon = "🟢"
		} else if r.score < -20 {
			icon = "🔴"
		}

		cpiStr := "  — "
		if r.cpi != 0 {
			cpiStr = fmt.Sprintf("%4.1f%%", r.cpi)
		}
		gdpStr := "  — "
		if r.gdp != 0 {
			gdpStr = fmt.Sprintf("%+4.1f%%", r.gdp)
		}
		unempStr := "  — "
		if r.unemp > 0 {
			unempStr = fmt.Sprintf("%4.1f%%", r.unemp)
		}
		rateStr := "  — "
		if r.rate != 0 {
			rateStr = fmt.Sprintf("%4.2f", r.rate)
		}

		b.WriteString(fmt.Sprintf("%s%-3s %+4.0f  %s %s %s %s\n", icon, r.code, r.score, cpiStr, gdpStr, unempStr, rateStr))
	}
	b.WriteString("</pre>")

	// Relative strength insight
	strongest := rows[0]
	weakest := rows[0]
	for _, r := range rows {
		if r.score > strongest.score {
			strongest = r
		}
		if r.score < weakest.score {
			weakest = r
		}
	}

	if strongest.code != weakest.code && strongest.score-weakest.score > 20 {
		b.WriteString("\n💡 <b>Relative Strength:</b>\n")
		b.WriteString(fmt.Sprintf("%s strongest (%+.0f), %s weakest (%+.0f)\n", strongest.code, strongest.score, weakest.code, weakest.score))
		if strongest.code == "USD" {
			b.WriteString(fmt.Sprintf("Supports: %s/%s bearish bias\n", weakest.code, strongest.code))
		} else if weakest.code == "USD" {
			b.WriteString(fmt.Sprintf("Supports: %s/%s bullish bias\n", strongest.code, weakest.code))
		} else {
			b.WriteString(fmt.Sprintf("Supports: %s strength vs %s\n", strongest.code, weakest.code))
		}
	}

	return b.String()
}

// FormatMacroLabor renders a detailed labor market deep-dive for Telegram.
func (f *Formatter) FormatMacroLabor(composites *domain.MacroComposites, data *fred.MacroData) string {
	var b strings.Builder

	b.WriteString("👷 <b>LABOR MARKET DEEP DIVE</b>\n")
	b.WriteString(fmt.Sprintf("<i>Updated %s</i>\n\n", fmtutil.FormatDateTimeWIB(data.FetchedAt)))

	// Health Index
	healthEmoji := "🟢"
	switch {
	case composites != nil && composites.LaborHealth < 20:
		healthEmoji = "🔴"
	case composites != nil && composites.LaborHealth < 40:
		healthEmoji = "🟠"
	case composites != nil && composites.LaborHealth < 60:
		healthEmoji = "🟡"
	}
	if composites != nil {
		b.WriteString(fmt.Sprintf("<b>Health Index: %.0f/100 %s %s</b>\n\n", composites.LaborHealth, healthEmoji, composites.LaborLabel))
	}

	b.WriteString("<code>┌─────────────────────────────────────┐</code>\n")

	// JOLTS Openings
	if data.JOLTSOpenings > 0 {
		b.WriteString(fmt.Sprintf("<code>│ JOLTS Openings  %6.0fK  %s %-9s│</code>\n",
			data.JOLTSOpenings, data.JOLTSOpeningsTrend.Arrow(), trendLabel(data.JOLTSOpeningsTrend.Direction)))
	}
	// JOLTS Hiring Rate
	if data.JOLTSHiringRate > 0 {
		b.WriteString(fmt.Sprintf("<code>│ JOLTS Hiring    %5.1f%%   %s %-9s│</code>\n",
			data.JOLTSHiringRate, data.JOLTSHiringRateTrend.Arrow(), trendLabel(data.JOLTSHiringRateTrend.Direction)))
	}
	// JOLTS Quit Rate
	if data.JOLTSQuitRate > 0 {
		b.WriteString(fmt.Sprintf("<code>│ JOLTS Quit Rate %5.1f%%   %s %-9s│</code>\n",
			data.JOLTSQuitRate, data.JOLTSQuitRateTrend.Arrow(), trendLabel(data.JOLTSQuitRateTrend.Direction)))
	}
	// Initial Claims
	if data.InitialClaims > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Initial Claims  %6.0fK  %s %-9s│</code>\n",
			data.InitialClaims/1_000, data.ClaimsTrend.Arrow(), trendLabel(data.ClaimsTrend.Direction)))
	}
	// Continuing Claims
	if data.ContinuingClaims > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Cont. Claims    %5.0fK  %s %-9s│</code>\n",
			data.ContinuingClaims/1_000, data.ContinuingClaimsTrend.Arrow(), trendLabel(data.ContinuingClaimsTrend.Direction)))
	}
	// Unemployment U3
	if data.UnemployRate > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Unemployment U3 %5.1f%%   → LEVEL    │</code>\n", data.UnemployRate))
	}
	// U6
	if data.U6Unemployment > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Unemployment U6 %5.1f%%   → LEVEL    │</code>\n", data.U6Unemployment))
	}
	// Emp-Pop Ratio
	if data.EmpPopRatio > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Emp-Pop Ratio   %5.1f%%   → LEVEL    │</code>\n", data.EmpPopRatio))
	}
	// NFP
	if data.NFP > 0 {
		nfpArrow := data.NFPTrend.Arrow()
		b.WriteString(fmt.Sprintf("<code>│ NFP (MoM)       %+6.0fK  %s          │</code>\n", data.NFPChange, nfpArrow))
	}
	// Wage Growth
	if data.WageGrowth > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Avg Hourly Earn %+5.1f%%  %s %-9s│</code>\n",
			data.WageGrowth, data.WageGrowthTrend.Arrow(), trendLabel(data.WageGrowthTrend.Direction)))
	}
	// Sahm Rule
	if data.SahmRule > 0 {
		sahmEmoji := "✅"
		if data.SahmRule >= 0.5 {
			sahmEmoji = "🚨"
		} else if data.SahmRule >= 0.3 {
			sahmEmoji = "⚠️"
		}
		b.WriteString(fmt.Sprintf("<code>│ Sahm Rule       %5.2f   %s          │</code>\n", data.SahmRule, sahmEmoji))
	}

	b.WriteString("<code>└─────────────────────────────────────┘</code>\n")

	// Key insight
	b.WriteString("\n<b>💡 Key Insight:</b>\n")
	if data.SahmRule >= 0.5 {
		b.WriteString("<i>Sahm Rule triggered — historically reliable recession signal. Labor market deteriorating rapidly.</i>\n")
	} else if data.InitialClaims > 280_000 {
		b.WriteString("<i>Initial claims elevated above 280K — labor demand weakening. Watch for sustained trend.</i>\n")
	} else if data.JOLTSQuitRate > 0 && data.JOLTSQuitRate > 2.5 {
		b.WriteString("<i>Quit rate high = workers confident about job prospects. Healthy labor demand.</i>\n")
	} else if data.JOLTSOpenings > 0 && data.JOLTSOpeningsTrend.Direction == "DOWN" {
		b.WriteString("<i>JOLTS openings declining — labor demand cooling. Leading indicator for future weakness.</i>\n")
	} else {
		b.WriteString("<i>Labor market stable. Monitor initial claims and JOLTS for early warning signals.</i>\n")
	}

	return b.String()
}

// FormatMacroInflation renders a detailed inflation deep-dive for Telegram.
func (f *Formatter) FormatMacroInflation(composites *domain.MacroComposites, data *fred.MacroData) string {
	var b strings.Builder

	b.WriteString("🔥 <b>INFLATION DEEP DIVE</b>\n")
	b.WriteString(fmt.Sprintf("<i>Updated %s</i>\n\n", fmtutil.FormatDateTimeWIB(data.FetchedAt)))

	// Momentum
	momEmoji := "🟢"
	switch {
	case composites != nil && composites.InflationMomentum > 0.5:
		momEmoji = "🔴"
	case composites != nil && composites.InflationMomentum > 0.2:
		momEmoji = "🟠"
	case composites != nil && composites.InflationMomentum < -0.2:
		momEmoji = "🔵"
	}
	if composites != nil {
		b.WriteString(fmt.Sprintf("<b>Momentum: %+.2f %s %s</b>\n\n", composites.InflationMomentum, momEmoji, composites.InflationLabel))
	}

	// Realized section
	b.WriteString("<b>REALIZED</b>\n")
	b.WriteString("<code>┌─────────────────────────────────────┐</code>\n")
	if data.CorePCE > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Core PCE        %5.1f%%  %s          │</code>\n", data.CorePCE, data.CorePCETrend.Arrow()))
	}
	if data.CPI > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Headline CPI    %5.1f%%  %s          │</code>\n", data.CPI, data.CPITrend.Arrow()))
	}
	if data.MedianCPI > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Median CPI      %5.1f%%  %s          │</code>\n", data.MedianCPI, data.MedianCPITrend.Arrow()))
	}
	if data.StickyCPI > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Sticky CPI      %5.1f%%  %s          │</code>\n", data.StickyCPI, data.StickyCPITrend.Arrow()))
	}
	if data.PPICommodities != 0 {
		b.WriteString(fmt.Sprintf("<code>│ PPI Commodities %+5.1f%%  %s          │</code>\n", data.PPICommodities, data.PPICommoditiesTrend.Arrow()))
	}
	if data.WageGrowth > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Wage Growth     %+5.1f%%  %s          │</code>\n", data.WageGrowth, data.WageGrowthTrend.Arrow()))
	}
	b.WriteString("<code>└─────────────────────────────────────┘</code>\n")

	// Expectations section
	b.WriteString("\n<b>EXPECTATIONS</b>\n")
	b.WriteString("<code>┌─────────────────────────────────────┐</code>\n")
	if data.Breakeven5Y > 0 {
		anchored := "STABLE"
		if data.Breakeven5Y > 2.8 {
			anchored = "ELEVATED"
		}
		b.WriteString(fmt.Sprintf("<code>│ 5Y Breakeven    %5.2f%%  → %-9s│</code>\n", data.Breakeven5Y, anchored))
	}
	if data.ForwardInflation > 0 {
		anchored := "ANCHORED"
		if data.ForwardInflation > 2.8 {
			anchored = "DE-ANCHORING"
		} else if data.ForwardInflation < 2.0 {
			anchored = "DEFL RISK"
		}
		b.WriteString(fmt.Sprintf("<code>│ 5Y5Y Forward    %5.2f%%  → %-9s│</code>\n", data.ForwardInflation, anchored))
	}
	if data.MichInflExp1Y > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Michigan 1Y     %5.1f%%  → SURVEY    │</code>\n", data.MichInflExp1Y))
	}
	if data.ClevelandInfExp1Y > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Cleveland 1Y    %5.2f%%  → MODEL     │</code>\n", data.ClevelandInfExp1Y))
	}
	if data.ClevelandInfExp10Y > 0 {
		b.WriteString(fmt.Sprintf("<code>│ Cleveland 10Y   %5.2f%%  → LONG-RUN  │</code>\n", data.ClevelandInfExp10Y))
	}
	b.WriteString("<code>└─────────────────────────────────────┘</code>\n")

	// Divergence detection
	b.WriteString("\n<b>💡 Key Insight:</b>\n")
	if data.Breakeven5Y > 0 && data.CorePCE > 0 {
		beHigh := data.Breakeven5Y > 2.5
		pceHigh := data.CorePCE > 3.0
		beFalling := data.Breakeven5Y < 2.2
		pceFalling := data.CorePCE < 2.5

		if beHigh && pceFalling {
			b.WriteString("<i>⚠️ Divergence: Market pricing inflation re-acceleration despite soft realized data. Hawkish repricing risk → USD bullish.</i>\n")
		} else if beFalling && pceHigh {
			b.WriteString("<i>⚠️ Divergence: Market expects disinflation but realized data hasn't confirmed. Risk of dovish over-pricing.</i>\n")
		} else if data.StickyCPI > 4.0 {
			b.WriteString("<i>Sticky CPI elevated — services inflation persistent. Fed unlikely to cut aggressively.</i>\n")
		} else if data.PPICommodities > 5.0 {
			b.WriteString("<i>PPI rising sharply — input cost pressure building. Watch for pass-through to consumer prices.</i>\n")
		} else if data.CorePCE < 2.5 && data.ForwardInflation > 0 && data.ForwardInflation < 2.5 {
			b.WriteString("<i>Inflation on target with anchored expectations — ideal for risk assets and potential rate cuts.</i>\n")
		} else {
			b.WriteString("<i>Inflation mixed. Monitor divergences between realized data and market expectations.</i>\n")
		}
	} else {
		b.WriteString("<i>Insufficient data for full inflation analysis.</i>\n")
	}

	return b.String()
}
