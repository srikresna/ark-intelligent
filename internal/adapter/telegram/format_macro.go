package telegram

import (
	"fmt"
	"sort"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/internal/service/dvol"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
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

// FormatMacroSummary formats a plain-language executive summary of the macro regime.
// Designed for non-finance users: leads with "so what", follows with "why".
func (f *Formatter) FormatMacroSummary(regime fred.MacroRegime, data *fred.MacroData, implications []fred.TradingImplication) string {
	var b strings.Builder

	riskBar := buildRiskBar(regime.Score, 15)

	b.WriteString("🏦 <b>MACRO SNAPSHOT</b>\n")
	b.WriteString(fmt.Sprintf("<i>Data per %s</i>\n\n", fmtutil.FormatDateTimeWIB(data.FetchedAt)))

	// --- Section 1: Plain-language status ---
	b.WriteString(macroStatusLine("Ekonomi", macroEconomyLabel(regime, data)))
	b.WriteString(macroStatusLine("Inflasi", macroInflationLabel(data)))
	b.WriteString(macroStatusLine("Pasar Kerja", macroLaborLabel(regime, data)))
	b.WriteString(macroStatusLine("Stress", macroStressLabel(data)))
	b.WriteString(macroStatusLine("Resesi", macroRecessionLabel(regime)))
	b.WriteString(fmt.Sprintf("\n<code>Risk: [%s]</code>\n", riskBar))

	// --- Section 2: Trading implications ---
	b.WriteString("\n<b>━━━ APA ARTINYA UNTUK TRADING? ━━━</b>\n")
	for _, imp := range implications {
		b.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n", imp.Icon, imp.Asset, imp.Reason))
	}

	// --- Section 3: Alerts / warnings ---
	alerts := macroAlertLines(regime, data)
	if len(alerts) > 0 {
		b.WriteString("\n⚠️ <b>PERHATIAN:</b>\n")
		for _, alert := range alerts {
			b.WriteString(fmt.Sprintf("<i>%s</i>\n", alert))
		}
	}

	// Cache note
	age := fred.CacheAge()
	cacheNote := "live"
	if age >= 0 {
		cacheNote = fmt.Sprintf("cache %dm", int(age.Minutes()))
	}
	b.WriteString(fmt.Sprintf("\n<i>FRED %s</i>", cacheNote))

	return b.String()
}

// macroStatusLine formats a single status row: "Label : status_text"
func macroStatusLine(label, status string) string {
	// Pad label to 12 chars for alignment
	padded := label + strings.Repeat(" ", 12-len([]rune(label)))
	return fmt.Sprintf("<code>%s: %s</code>\n", padded, status)
}

// macroEconomyLabel produces a plain-language economy description.
func macroEconomyLabel(regime fred.MacroRegime, data *fred.MacroData) string {
	switch regime.Name {
	case "RECESSION":
		return "Resesi — ekonomi menyusut 🔴"
	case "STAGFLATION":
		return "Stagflasi — inflasi tinggi + pertumbuhan lemah 🔴"
	case "STRESS":
		return "Tekanan finansial tinggi 🔴"
	case "INFLATIONARY":
		return "Inflasi tinggi, pertumbuhan masih jalan ⚠️"
	case "GOLDILOCKS":
		if data.GDPGrowth > 2.0 {
			return "Ideal — pertumbuhan kuat, inflasi terjaga ✅"
		}
		return "Cukup baik — stabil ✅"
	case "NEUTRAL":
		if data.CorePCETrend.Direction == "DOWN" {
			return "Inflasi masih tinggi tapi mulai turun ⚠️"
		}
		return "Inflasi di atas target, pertumbuhan campuran ⚠️"
	case "DISINFLATIONARY":
		if data.GDPGrowth > 1.5 {
			return "Melambat tapi masih tumbuh ✅"
		}
		return "Melambat, perlu pantau ⚠️"
	default:
		return "Campuran — sinyal tidak jelas 🟡"
	}
}

// macroInflationLabel produces a plain-language inflation description.
func macroInflationLabel(data *fred.MacroData) string {
	if data.CorePCE <= 0 {
		return "Data tidak tersedia"
	}
	trend := ""
	switch data.CorePCETrend.Direction {
	case "DOWN":
		trend = " dan menurun ↓"
	case "UP":
		trend = " dan naik ↑"
	}

	switch {
	case data.CorePCE < 2.0:
		return fmt.Sprintf("Di bawah target (%.1f%%)%s ✅", data.CorePCE, trend)
	case data.CorePCE < 2.5:
		return fmt.Sprintf("Mendekati target (%.1f%%)%s ✅", data.CorePCE, trend)
	case data.CorePCE < 3.5:
		return fmt.Sprintf("Masih tinggi (%.1f%%)%s ⚠️", data.CorePCE, trend)
	default:
		return fmt.Sprintf("Sangat tinggi (%.1f%%)%s 🔴", data.CorePCE, trend)
	}
}

// macroLaborLabel produces a plain-language labor market description.
func macroLaborLabel(regime fred.MacroRegime, data *fred.MacroData) string {
	if regime.SahmAlert {
		return "Melemah tajam — sinyal resesi! 🔴"
	}
	if data.NFPChange < 0 {
		return "Kehilangan lapangan kerja 🔴"
	}
	if data.InitialClaims > 300000 {
		return fmt.Sprintf("Melemah (klaim: %.0fK) ⚠️", data.InitialClaims/1000)
	}
	if data.InitialClaims > 0 && data.InitialClaims < 250000 {
		trend := ""
		if data.ClaimsTrend.Direction == "UP" {
			trend = ", tapi mulai naik ↑"
		}
		return fmt.Sprintf("Kuat (klaim: %.0fK)%s ✅", data.InitialClaims/1000, trend)
	}
	return "Stabil 🟡"
}

// macroStressLabel produces a plain-language financial stress description.
func macroStressLabel(data *fred.MacroData) string {
	if data.VIX > 30 {
		return fmt.Sprintf("Tinggi — VIX %.0f, pasar takut 🔴", data.VIX)
	}
	if data.NFCI > 0.5 {
		return "Kondisi finansial ketat 🔴"
	}
	if data.NFCI > 0 {
		return "Sedikit tegang ⚠️"
	}
	if data.VIX > 20 {
		return fmt.Sprintf("Waspada — VIX %.0f ⚠️", data.VIX)
	}
	return "Tenang, tidak ada tekanan ✅"
}

// macroRecessionLabel produces a plain-language recession risk label.
func macroRecessionLabel(regime fred.MacroRegime) string {
	switch {
	case regime.SahmAlert:
		return "TINGGI — indikator resesi aktif! 🔴"
	case regime.Score >= 60:
		return "Meningkat ⚠️"
	case regime.Score >= 40:
		return "Sedang 🟡"
	default:
		return "Rendah ✅"
	}
}

// macroAlertLines returns contextual warnings in plain language.
func macroAlertLines(regime fred.MacroRegime, data *fred.MacroData) []string {
	var alerts []string

	if regime.SahmAlert {
		alerts = append(alerts, "Indikator Sahm Rule aktif — secara historis, ini sinyal resesi sudah dimulai. Akurasi 100% sejak 1970.")
	}
	if data.Spread3M10Y < 0 && data.Spread3M10Y != 0 {
		alerts = append(alerts, fmt.Sprintf("Yield curve 3M-10Y terbalik (%.2f%%) — secara historis ini sinyal resesi 6-18 bulan ke depan.", data.Spread3M10Y))
	} else if data.YieldSpread < 0 {
		alerts = append(alerts, fmt.Sprintf("Yield curve 2Y-10Y terbalik (%.2f%%) — sinyal perlambatan ekonomi.", data.YieldSpread))
	}
	if data.VIX > 30 {
		alerts = append(alerts, fmt.Sprintf("VIX di %.0f (>30) — pasar dalam mode ketakutan. Volatilitas tinggi.", data.VIX))
	}
	if data.NFPChange < 0 {
		alerts = append(alerts, "Nonfarm Payrolls negatif — ekonomi AS kehilangan lapangan kerja. Sangat jarang terjadi.")
	}
	if data.WageGrowth > 5 {
		alerts = append(alerts, fmt.Sprintf("Pertumbuhan upah %.1f%% (>5%%) — risiko spiral upah-harga, inflasi bisa bertahan lama.", data.WageGrowth))
	}

	return alerts
}

// FormatMacroExplain formats a plain-language glossary of macro indicators with current values.
func (f *Formatter) FormatMacroExplain(regime fred.MacroRegime, data *fred.MacroData) string {
	var b strings.Builder

	b.WriteString("📖 <b>PANDUAN INDIKATOR MACRO</b>\n")
	b.WriteString("<i>Penjelasan setiap indikator + nilai saat ini</i>\n")

	// Yield Curve
	b.WriteString("\n<b>Yield Curve (Kurva Imbal Hasil)</b>\n")
	b.WriteString("<i>Selisih bunga obligasi jangka pendek vs panjang.")
	b.WriteString(" Jika terbalik (negatif), pasar mengekspektasi resesi.</i>\n")
	b.WriteString(fmt.Sprintf("<code>  2Y-10Y : %s</code>\n", regime.YieldCurve))
	if regime.Yield3M10Y != "N/A" && regime.Yield3M10Y != "" {
		b.WriteString(fmt.Sprintf("<code>  3M-10Y : %s</code>\n", regime.Yield3M10Y))
	}

	// Core PCE
	b.WriteString("\n<b>Core PCE (Inflasi Inti)</b>\n")
	b.WriteString("<i>Ukuran inflasi favorit The Fed, tanpa makanan &amp; energi.")
	b.WriteString(" Target Fed: 2%. Lebih tinggi = Fed ketatkan suku bunga.</i>\n")
	b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.Inflation))

	// Fed Funds Rate
	if data.FedFundsRate > 0 {
		b.WriteString("\n<b>Suku Bunga Fed (Fed Funds Rate)</b>\n")
		b.WriteString("<i>Suku bunga acuan AS. Naik = USD menguat tapi menekan ekonomi.")
		b.WriteString(" Turun = USD melemah, ekonomi didorong.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.MonPolicy))
	}

	// NFCI
	b.WriteString("\n<b>NFCI (Kondisi Finansial)</b>\n")
	b.WriteString("<i>Indeks kondisi keuangan AS dari Chicago Fed.")
	b.WriteString(" Negatif = longgar (bagus). Positif = ketat (tekanan).")
	b.WriteString(" Di atas 0.5 = stress.</i>\n")
	b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.FinStress))

	// VIX
	if data.VIX > 0 {
		var vixStatus string
		switch {
		case data.VIX > 30:
			vixStatus = fmt.Sprintf("Tinggi (%.0f) 🔴 — pasar takut", data.VIX)
		case data.VIX > 20:
			vixStatus = fmt.Sprintf("Waspada (%.0f) ⚠️", data.VIX)
		default:
			vixStatus = fmt.Sprintf("Tenang (%.0f) ✅", data.VIX)
		}
		b.WriteString("\n<b>VIX (Indeks Ketakutan)</b>\n")
		b.WriteString("<i>Mengukur ekspektasi volatilitas pasar saham AS.")
		b.WriteString(" &lt;15 = tenang, 15-20 = normal, 20-30 = waspada, &gt;30 = panik.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", vixStatus))
	}

	// Sahm Rule
	b.WriteString("\n<b>Sahm Rule (Indikator Resesi)</b>\n")
	b.WriteString("<i>Jika naik di atas 0.5, resesi biasanya sudah dimulai.")
	b.WriteString(" Akurasi historis: 100% sejak 1970 (nol sinyal palsu).</i>\n")
	if regime.SahmAlert {
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code> 🚨 AKTIF!\n", regime.SahmLabel))
	} else if regime.SahmLabel != "N/A" && regime.SahmLabel != "" {
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.SahmLabel))
	}

	// Labor
	b.WriteString("\n<b>Pasar Kerja (Initial Claims)</b>\n")
	b.WriteString("<i>Klaim pengangguran mingguan. Makin rendah = makin sehat.")
	b.WriteString(" Di bawah 250K = kuat. Di atas 300K = melemah.</i>\n")
	b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.Labor))

	// GDP
	if regime.Growth != "N/A" && regime.Growth != "" {
		b.WriteString("\n<b>GDP (Pertumbuhan Ekonomi)</b>\n")
		b.WriteString("<i>Perubahan PDB AS per kuartal (tahunan).")
		b.WriteString(" Positif = tumbuh. Negatif 2 kuartal berturut = resesi teknis.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.Growth))
	}

	// Fed Balance Sheet
	if regime.FedBalance != "N/A" && regime.FedBalance != "" {
		b.WriteString("\n<b>Neraca Fed (QE/QT)</b>\n")
		b.WriteString("<i>Total aset Fed. Naik (QE) = cetak uang, likuiditas naik.")
		b.WriteString(" Turun (QT) = likuiditas dikurangi, pasar lebih ketat.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.FedBalance))
	}

	// DXY
	if regime.USDStrength != "N/A" && regime.USDStrength != "" {
		b.WriteString("\n<b>DXY (Kekuatan USD)</b>\n")
		b.WriteString("<i>Indeks dolar AS terhadap mata uang utama.")
		b.WriteString(" Naik = USD menguat. Turun = USD melemah.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.USDStrength))
	}

	return b.String()
}

// FormatRegimeLabel formats a COT-based regime result for display.
func (f *Formatter) FormatRegimeLabel(regime string, confidence float64, factors []string) string {
	icon := "⚪"
	switch regime {
	case "RISK-ON":
		icon = "🟢"
	case "RISK-OFF":
		icon = "🔴"
	case "UNCERTAINTY":
		icon = "🟡"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s <b>COT Regime: %s</b> (%.0f%% confidence)\n", icon, regime, confidence))
	if len(factors) > 0 {
		b.WriteString("<i>Signals: ")
		shown := factors
		if len(shown) > 3 {
			shown = factors[:3]
		}
		b.WriteString(strings.Join(shown, " | "))
		b.WriteString("</i>\n")
	}
	return b.String()
}

// FormatRegimePerformance formats the regime-asset performance matrix.
func (f *Formatter) FormatRegimePerformance(matrix *fred.RegimePerformanceMatrix) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x8A <b>REGIME-ASSET PERFORMANCE MATRIX</b>\n")
	b.WriteString("<i>Annualized returns (%) by FRED macro regime</i>\n\n")

	if matrix == nil || len(matrix.Regimes) == 0 {
		b.WriteString("No regime performance data available yet.\n")
		b.WriteString("<i>Data builds as signals accumulate with FRED regime labels.</i>")
		return b.String()
	}

	// For each regime, show a compact table
	for _, regime := range matrix.Regimes {
		returns := matrix.Data[regime]
		if len(returns) == 0 {
			continue
		}

		icon := "\xF0\x9F\x93\x88"
		if regime == "STRESS" || regime == "RECESSION" {
			icon = "\xF0\x9F\x94\xB4"
		} else if regime == "STAGFLATION" {
			icon = "\xF0\x9F\x9F\xA0"
		} else if regime == "GOLDILOCKS" {
			icon = "\xF0\x9F\x9F\xA2"
		}

		currentTag := ""
		if regime == matrix.Current {
			currentTag = " \xe2\x86\x90 CURRENT"
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b>%s\n<pre>", icon, regime, currentTag))
		b.WriteString(fmt.Sprintf("%-5s %7s %5s %4s\n", "CCY", "Ann.%", "WR%", "N"))
		b.WriteString(strings.Repeat("\xe2\x94\x80", 26) + "\n")

		for _, r := range returns {
			if r.TotalWeeks == 0 {
				continue
			}
			sign := "+"
			if r.AnnualizedReturn < 0 {
				sign = ""
			}
			b.WriteString(fmt.Sprintf("%-5s %s%6.1f %4.0f%% %4d\n",
				r.Currency, sign, r.AnnualizedReturn, r.WinRate, r.TotalWeeks))
		}
		b.WriteString("</pre>\n")
	}

	if matrix.Current != "" {
		b.WriteString(fmt.Sprintf("\nCurrent regime: <b>%s</b>\n", matrix.Current))
	}
	b.WriteString("<i>Ann.% = avg weekly return \xc3\x97 52 | WR% = weeks with positive return</i>")

	return b.String()
}

// FormatSentiment renders the sentiment survey dashboard as Telegram HTML.
// macroRegime is the current FRED regime name (e.g. "GOLDILOCKS"); pass "" to skip regime context.
func (f *Formatter) FormatSentiment(data *sentiment.SentimentData, macroRegime string) string {
	var b strings.Builder

	b.WriteString("🧠 <b>SENTIMENT SURVEY DASHBOARD</b>\n")
	b.WriteString(fmt.Sprintf("<i>Updated %s</i>\n", fmtutil.FormatDateTimeWIB(data.FetchedAt)))

	// --- CNN Fear & Greed Index ---
	b.WriteString("\n<b>CNN Fear &amp; Greed Index</b>\n")
	if data.CNNAvailable {
		gauge := sentimentGauge(data.CNNFearGreed, 15)
		emoji := fearGreedEmoji(data.CNNFearGreed)
		b.WriteString(fmt.Sprintf("<code>[%s]</code>\n", gauge))
		b.WriteString(fmt.Sprintf("<code>Score : %.0f / 100  %s %s</code>\n", data.CNNFearGreed, emoji, data.CNNFearGreedLabel))

		// Trend comparison — show how sentiment has changed
		b.WriteString("<code>Trend :</code>")
		if data.CNNPrev1Week > 0 {
			delta1w := data.CNNFearGreed - data.CNNPrev1Week
			b.WriteString(fmt.Sprintf(" <code>1W: %+.0f</code>", delta1w))
		}
		if data.CNNPrev1Month > 0 {
			delta1m := data.CNNFearGreed - data.CNNPrev1Month
			b.WriteString(fmt.Sprintf(" <code>| 1M: %+.0f</code>", delta1m))
		}
		if data.CNNPrev1Year > 0 {
			delta1y := data.CNNFearGreed - data.CNNPrev1Year
			b.WriteString(fmt.Sprintf(" <code>| 1Y: %+.0f</code>", delta1y))
		}
		b.WriteString("\n")

		// Velocity alert — rapid shift in sentiment
		if data.CNNPrev1Month > 0 {
			monthDelta := data.CNNFearGreed - data.CNNPrev1Month
			if monthDelta < -30 {
				b.WriteString("⚠️ <i>Penurunan tajam dari sebulan lalu — pasar bergeser ke fear cepat</i>\n")
			} else if monthDelta > 30 {
				b.WriteString("⚠️ <i>Lonjakan tajam dari sebulan lalu — euforia meningkat cepat</i>\n")
			}
		}

		// Contrarian signal
		if data.CNNFearGreed <= 25 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Extreme fear sering mendahului kenaikan\n")
		} else if data.CNNFearGreed >= 75 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Extreme greed sering mendahului koreksi\n")
		}
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}

	// --- Crypto Fear & Greed Index (alternative.me) ---
	b.WriteString("\n<b>Crypto Fear &amp; Greed Index</b>\n")
	if data.CryptoFearGreedAvailable {
		gauge := sentimentGauge(data.CryptoFearGreed, 15)
		emoji := fearGreedEmoji(data.CryptoFearGreed)
		b.WriteString(fmt.Sprintf("<code>[%s]</code>\n", gauge))
		b.WriteString(fmt.Sprintf("<code>Score : %.0f / 100  %s %s</code>\n", data.CryptoFearGreed, emoji, data.CryptoFearGreedLabel))
		if data.CryptoFearGreed <= 25 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Extreme fear di crypto bisa jadi zona akumulasi\n")
		} else if data.CryptoFearGreed >= 75 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Extreme greed di crypto sering mendahului koreksi\n")
		}
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}


	// --- Crypto Global Market Data (alternative.me v2) ---
	if data.CryptoGlobalAvailable {
		b.WriteString("\n<b>🌐 Crypto Global Market</b>\n")
		mcapT := data.CryptoTotalMarketCap / 1e12
		b.WriteString(fmt.Sprintf("<code>Total Mcap   : $%.2fT</code>\n", mcapT))
		b.WriteString(fmt.Sprintf("<code>BTC Dominance: %.1f%%</code>", data.CryptoBTCDominance))
		if data.CryptoBTCDominance >= 55 {
			b.WriteString(" — BTC season")
		} else if data.CryptoBTCDominance <= 40 {
			b.WriteString(" — 🔥 Alt season")
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("<code>Currencies   : %d</code>\n", data.CryptoActiveCurrencies))
		b.WriteString(fmt.Sprintf("<code>Markets      : %d</code>\n", data.CryptoActiveMarkets))
	}

	// --- Crypto Top Movers (alternative.me v2) ---
	if data.CryptoTickersAvailable && len(data.CryptoTopTickers) > 0 {
		b.WriteString("\n<b>📊 Crypto Top 10 (24h)</b>\n")
		topN := 10
		if len(data.CryptoTopTickers) < topN {
			topN = len(data.CryptoTopTickers)
		}
		for _, t := range data.CryptoTopTickers[:topN] {
			arrow := "⬆️"
			if t.PercentChange24h < 0 {
				arrow = "⬇️"
			}
			priceStr := ""
			if t.PriceUSD >= 1000 {
				priceStr = fmt.Sprintf("$%.0f", t.PriceUSD)
			} else if t.PriceUSD >= 1 {
				priceStr = fmt.Sprintf("$%.2f", t.PriceUSD)
			} else {
				priceStr = fmt.Sprintf("$%.4f", t.PriceUSD)
			}
			b.WriteString(fmt.Sprintf("<code>%s %-5s %8s %+.1f%%</code>\n", arrow, t.Symbol, priceStr, t.PercentChange24h))
		}
	}

	// --- AAII Investor Sentiment Survey ---
	b.WriteString("\n<b>AAII Investor Sentiment Survey</b>\n")
	if data.AAIIAvailable {
		if data.AAIIWeekDate != "" {
			b.WriteString(fmt.Sprintf("<i>Minggu berakhir %s</i>\n", data.AAIIWeekDate))
		}
		b.WriteString(fmt.Sprintf("<code>Bullish : %5.1f%%</code>  %s\n", data.AAIIBullish, sentimentBar(data.AAIIBullish, "🟢")))
		b.WriteString(fmt.Sprintf("<code>Neutral : %5.1f%%</code>  %s\n", data.AAIINeutral, sentimentBar(data.AAIINeutral, "⚪")))
		b.WriteString(fmt.Sprintf("<code>Bearish : %5.1f%%</code>  %s\n", data.AAIIBearish, sentimentBar(data.AAIIBearish, "🔴")))
		b.WriteString(fmt.Sprintf("<code>Bull/Bear: %.2f</code>", data.AAIIBullBear))
		if data.AAIIBullBear > 0 {
			if data.AAIIBullBear >= 2.0 {
				b.WriteString("  — ⚠️ Optimisme tinggi")
			} else if data.AAIIBullBear <= 0.5 {
				b.WriteString("  — 🟢 Pesimisme dalam (contrarian bullish)")
			}
		}
		b.WriteString("\n")

		// Historical context: AAII long-term averages are ~37.5% bull, 31% bear, 31.5% neutral
		if data.AAIIBullish >= 50 {
			b.WriteString("<code>Catatan: Bullish jauh di atas rata-rata historis (~37.5%%)</code>\n")
		} else if data.AAIIBearish >= 50 {
			b.WriteString("<code>Catatan: Bearish jauh di atas rata-rata historis (~31%%)</code>\n")
		}
	} else {
		b.WriteString("<code>Data tidak tersedia — set FIRECRAWL_API_KEY untuk mengaktifkan</code>\n")
	}

	// --- AAII contrarian signal ---
	if data.AAIIAvailable {
		if data.AAIIBearish >= 50 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Bearish >50%% secara historis mendahului rally\n")
		} else if data.AAIIBullish >= 50 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Bullish >50%% secara historis mendahului koreksi\n")
		}
	}

	// --- CBOE Put/Call Ratios ---
	b.WriteString("\n<b>CBOE Put/Call Ratios</b>\n")
	if data.PutCallAvailable {
		b.WriteString(fmt.Sprintf("<code>Total P/C : %.2f</code>\n", data.PutCallTotal))
		if data.PutCallEquity > 0 {
			b.WriteString(fmt.Sprintf("<code>Equity P/C: %.2f</code>\n", data.PutCallEquity))
		}
		if data.PutCallIndex > 0 {
			b.WriteString(fmt.Sprintf("<code>Index P/C : %.2f</code>\n", data.PutCallIndex))
		}
		if data.PutCallSignal != "" {
			signalEmoji := "🟡"
			switch data.PutCallSignal {
			case "EXTREME FEAR":
				signalEmoji = "🟢"
			case "FEAR":
				signalEmoji = "🟢"
			case "EXTREME COMPLACENCY":
				signalEmoji = "🔴"
			case "COMPLACENCY":
				signalEmoji = "🟠"
			}
			b.WriteString(fmt.Sprintf("<code>Signal    : %s %s</code>\n", signalEmoji, data.PutCallSignal))
		}
		// Context interpretation
		if data.PutCallIndex > 0 && data.PutCallEquity > 0 {
			if data.PutCallIndex > 1.0 && data.PutCallEquity < 0.8 {
				b.WriteString("<i>Index P/C tinggi → institusi melakukan hedging. Equity P/C normal → retail belum panik.</i>\n")
			} else if data.PutCallTotal >= 1.2 {
				b.WriteString("<i>Pembelian put ekstrem di semua instrumen — sinyal contrarian bullish kuat.</i>\n")
			} else if data.PutCallTotal < 0.7 {
				b.WriteString("<i>Pembelian proteksi sangat rendah — peringatan complacency. Contrarian bearish.</i>\n")
			}
		}
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}

	// --- Myfxbook Retail Positioning ---
	b.WriteString("\n<b>Retail Positioning (Myfxbook)</b>\n")
	if data.MyfxbookAvailable && len(data.MyfxbookPairs) > 0 {
		for _, mp := range data.MyfxbookPairs {
			var signalEmoji string
			switch mp.Signal {
			case "CONTRARIAN_BULLISH":
				signalEmoji = "🟢"
			case "LEAN_BULLISH":
				signalEmoji = "🟢"
			case "CONTRARIAN_BEARISH":
				signalEmoji = "🔴"
			case "LEAN_BEARISH":
				signalEmoji = "🔴"
			default:
				signalEmoji = "⚪"
			}
			signalLabel := mp.Signal
			if signalLabel == "CONTRARIAN_BULLISH" {
				signalLabel = "Contrarian Bullish"
			} else if signalLabel == "CONTRARIAN_BEARISH" {
				signalLabel = "Contrarian Bearish"
			} else if signalLabel == "LEAN_BULLISH" {
				signalLabel = "Lean Bullish"
			} else if signalLabel == "LEAN_BEARISH" {
				signalLabel = "Lean Bearish"
			} else {
				signalLabel = "Netral"
			}
			b.WriteString(fmt.Sprintf("<code>%-8s: %4.1f%% L / %4.1f%% S</code> %s %s\n", mp.Symbol, mp.LongPct, mp.ShortPct, signalEmoji, signalLabel))
		}
		b.WriteString("<i>Retail positioning adalah indikator contrarian — pembacaan ekstrem mengindikasikan potensi reversal.</i>\n")
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}

	// --- VIX Term Structure (CBOE) ---
	b.WriteString("\n<b>VIX Term Structure</b>\n")
	if data.VIXAvailable {
		b.WriteString(fmt.Sprintf("<code>Spot  : %.2f</code>\n", data.VIXSpot))
		if data.VIXM1 > 0 {
			b.WriteString(fmt.Sprintf("<code>M1    : %.2f</code>\n", data.VIXM1))
		}
		if data.VIXM2 > 0 {
			b.WriteString(fmt.Sprintf("<code>M2    : %.2f</code>\n", data.VIXM2))
		}
		if data.VVIX > 0 {
			b.WriteString(fmt.Sprintf("<code>VVIX  : %.1f</code>\n", data.VVIX))
		}
		var structLabel, structEmoji string
		if data.VIXContango {
			structLabel = "CONTANGO"
			structEmoji = "✅"
		} else {
			structLabel = "BACKWARDATION"
			structEmoji = "🔴"
		}
		if data.VIXSlopePct != 0 {
			b.WriteString(fmt.Sprintf("<code>Shape : %s (%+.1f%%) %s</code>\n", structLabel, data.VIXSlopePct, structEmoji))
		} else {
			b.WriteString(fmt.Sprintf("<code>Shape : %s %s</code>\n", structLabel, structEmoji))
		}
		if data.VIXRegime != "" {
			var regimeEmoji string
			switch data.VIXRegime {
			case "EXTREME_FEAR":
				regimeEmoji = "😱"
			case "FEAR":
				regimeEmoji = "😟"
			case "ELEVATED":
				regimeEmoji = "⚠️"
			case "RISK_ON_NORMAL":
				regimeEmoji = "🟢"
			case "RISK_ON_COMPLACENT":
				regimeEmoji = "😏"
			default:
				regimeEmoji = "🟡"
			}
			b.WriteString(fmt.Sprintf("<code>Regime: %s %s</code>\n", data.VIXRegime, regimeEmoji))
		}
		switch data.VIXRegime {
		case "EXTREME_FEAR":
			b.WriteString("<i>VIX backwardation ekstrem — pasar panik, hedging demand tinggi. Historically contrarian bullish.</i>\n")
		case "FEAR":
			b.WriteString("<i>VIX backwardation — ketakutan jangka pendek tinggi, pasar memperhitungkan risiko dekat.</i>\n")
		case "RISK_ON_COMPLACENT":
			b.WriteString("<i>Steep contango — pasar complacent, VIX ETPs merugi. Bullish ekuitas tapi waspada pembalikan mendadak.</i>\n")
		}
		// --- MOVE Index (bond volatility) ---
		if data.MOVEAvailable {
			b.WriteString("\n<b>MOVE Index (Bond Vol)</b>\n")
			b.WriteString(fmt.Sprintf("<code>MOVE  : %.1f (%+.1f%%)</code>\n", data.MOVELevel, data.MOVEChangePct))
			if data.VIXMOVERatio > 0 {
				var ratioEmoji string
				switch {
				case data.VIXMOVERatio > 0.35:
					ratioEmoji = "📈" // equity vol elevated
				case data.VIXMOVERatio < 0.12:
					ratioEmoji = "📉" // bond vol elevated
				default:
					ratioEmoji = "↔️"
				}
				b.WriteString(fmt.Sprintf("<code>VIX/MOVE: %.3f %s</code>\n", data.VIXMOVERatio, ratioEmoji))
			}
			switch data.MOVEDivergence {
			case "EQUITY_FEAR":
				b.WriteString("<i>VIX tinggi vs MOVE rendah — ketakutan spesifik ekuitas, bukan sistemik.</i>\n")
			case "BOND_STRESS":
				b.WriteString("<i>MOVE tinggi vs VIX rendah — stres obligasi / risiko carry FX.</i>\n")
			case "SYSTEMIC_STRESS":
				b.WriteString("<i>VIX dan MOVE keduanya tinggi — stres sistemik luas.</i>\n")
			}
		}
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}


	// --- Deribit DVOL - Crypto Volatility Index ---
	if data.DVOLAvailable {
		b.WriteString("\n<b>Crypto Volatility (Deribit DVOL)</b>\n")

		formatDVOLCurrency := func(label string, current, change24hPct, high24h, low24h, hv, ivhvSpread, ivhvRatio float64, spike, available bool) {
			if !available {
				return
			}
			changeArrow := "\u2192"
			changeEmoji := ""
			if change24hPct > 5 {
				changeArrow = "\u2191"
				changeEmoji = "\U0001f534" // red circle for vol up
			} else if change24hPct < -5 {
				changeArrow = "\u2193"
				changeEmoji = "\U0001f7e2" // green circle for vol down
			}
			b.WriteString(fmt.Sprintf("<code>%s DVOL : %.1f%%  %s %+.1f%% %s</code>\n", label, current, changeArrow, change24hPct, changeEmoji))
			b.WriteString(fmt.Sprintf("<code>  24h   : %.1f - %.1f</code>\n", low24h, high24h))
			if hv > 0 {
				spreadLabel := dvol.SpreadSignal(ivhvRatio)
				b.WriteString(fmt.Sprintf("<code>  IV/HV : %.1f%% / %.1f%% (spread: %+.1f)</code>\n", current, hv, ivhvSpread))
				b.WriteString(fmt.Sprintf("<code>  Signal: %s</code>\n", spreadLabel))
			}
			if spike {
				b.WriteString(fmt.Sprintf("\u26a0\ufe0f <i>%s DVOL spike >20%% dalam 24h \u2014 volatility surge!</i>\n", label))
			}
		}

		formatDVOLCurrency("BTC", data.DVOLBTCCurrent, data.DVOLBTCChange24hPct, data.DVOLBTCHigh24h, data.DVOLBTCLow24h, data.DVOLBTCHV, data.DVOLBTCIVHVSpread, data.DVOLBTCIVHVRatio, data.DVOLBTCSpike, data.DVOLBTCAvailable)
		formatDVOLCurrency("ETH", data.DVOLETHCurrent, data.DVOLETHChange24hPct, data.DVOLETHHigh24h, data.DVOLETHLow24h, data.DVOLETHHV, data.DVOLETHIVHVSpread, data.DVOLETHIVHVRatio, data.DVOLETHSpike, data.DVOLETHAvailable)

		// Cross-asset vol comparison: DVOL vs CBOE VIX
		if data.DVOLBTCAvailable && data.VIXAvailable && data.VIXSpot > 0 {
			dvolVixRatio := data.DVOLBTCCurrent / data.VIXSpot
			b.WriteString(fmt.Sprintf("\n<code>BTC DVOL/VIX: %.1fx</code>", dvolVixRatio))
			if dvolVixRatio > 5 {
				b.WriteString(" \u2014 <i>Crypto vol jauh melebihi ekuitas</i>")
			} else if dvolVixRatio < 2 {
				b.WriteString(" \u2014 <i>Crypto vol relatif rendah vs ekuitas</i>")
			}
			b.WriteString("\n")
		}
	}

	// --- Cross-Asset Volatility Suite (CBOE) ---
	if data.VolSuiteAvail {
		b.WriteString("\n<b>Vol Suite (CBOE)</b>\n")
		if data.VolSKEW > 0 {
			var skewEmoji string
			switch {
			case data.VolSKEW > 140:
				skewEmoji = "🔴"
			case data.VolSKEW > 130:
				skewEmoji = "⚠️"
			default:
				skewEmoji = "✅"
			}
			b.WriteString(fmt.Sprintf("<code>SKEW  : %.1f %s</code>\n", data.VolSKEW, skewEmoji))
		}
		if data.VolOVX > 0 {
			b.WriteString(fmt.Sprintf("<code>OVX   : %.1f</code>\n", data.VolOVX))
		}
		if data.VolGVZ > 0 {
			b.WriteString(fmt.Sprintf("<code>GVZ   : %.1f</code>\n", data.VolGVZ))
		}
		if data.VolRVX > 0 {
			var rvxEmoji string
			if data.RVXVIXRatio > 1.3 {
				rvxEmoji = " ⚠️"
			}
			b.WriteString(fmt.Sprintf("<code>RVX   : %.1f%s</code>\n", data.VolRVX, rvxEmoji))
		}
		if data.VolVIX9D > 0 {
			var v9dEmoji string
			if data.VIX9D30Ratio > 1.1 {
				v9dEmoji = " ⚠️"
			}
			b.WriteString(fmt.Sprintf("<code>VIX9D : %.2f%s</code>\n", data.VolVIX9D, v9dEmoji))
		}
		if data.SKEWVIXRatio > 0 {
			var ratioEmoji string
			if data.SKEWVIXRatio > 8.0 {
				ratioEmoji = " 🔴"
			}
			b.WriteString(fmt.Sprintf("<code>SKEW/VIX: %.1f%s</code>\n", data.SKEWVIXRatio, ratioEmoji))
		}
		if data.RVXVIXRatio > 0 {
			b.WriteString(fmt.Sprintf("<code>RVX/VIX : %.2f</code>\n", data.RVXVIXRatio))
		}
		switch data.VolTailRisk {
		case "EXTREME":
			b.WriteString("<i>🔴 TAIL RISK EXTREME — SKEW/VIX historically dangerous.</i>\n")
		case "ELEVATED":
			b.WriteString("<i>⚠️ Tail risk elevated — SKEW tinggi vs VIX rendah.</i>\n")
		}
		for _, d := range data.VolDivergences {
			b.WriteString(fmt.Sprintf("<i>📊 %s</i>\n", d))
		}
	}

	// --- Composite reading ---
	b.WriteString("\n<b>Pembacaan Gabungan</b>\n")
	compositeWritten := false

	// Cross-source agreement amplifies the signal
	if data.CNNAvailable && data.AAIIAvailable {
		cnnFear := data.CNNFearGreed <= 25
		cnnGreed := data.CNNFearGreed >= 75
		aaiiFear := data.AAIIBearish >= 50
		aaiiGreed := data.AAIIBullish >= 50

		if cnnFear && aaiiFear {
			b.WriteString("🟢 <b>STRONG CONTRARIAN BUY</b>\n")
			b.WriteString("<i>Kedua sumber menunjukkan ketakutan ekstrem — secara historis ini sinyal beli yang kuat. Pasar sering rebound dari level ini.</i>\n")
			compositeWritten = true
		} else if cnnGreed && aaiiGreed {
			b.WriteString("🔴 <b>STRONG CONTRARIAN SELL</b>\n")
			b.WriteString("<i>Kedua sumber menunjukkan keserakahan ekstrem — waspada koreksi. Euforia berlebihan jarang bertahan lama.</i>\n")
			compositeWritten = true
		} else if (cnnFear && !aaiiFear) || (!cnnFear && aaiiFear) {
			b.WriteString("🟡 <b>MIXED FEAR</b>\n")
			b.WriteString("<i>Hanya salah satu sumber menunjukkan fear ekstrem — sinyal belum sekuat jika keduanya sepakat.</i>\n")
			compositeWritten = true
		} else if (cnnGreed && !aaiiGreed) || (!cnnGreed && aaiiGreed) {
			b.WriteString("🟡 <b>MIXED GREED</b>\n")
			b.WriteString("<i>Hanya salah satu sumber menunjukkan greed ekstrem — belum cukup kuat untuk sinyal jual.</i>\n")
			compositeWritten = true
		}
	}

	if !compositeWritten {
		b.WriteString("<i>Sentiment survey adalah indikator contrarian.\n")
		b.WriteString("Pembacaan ekstrem sering menandai titik balik.</i>\n")
	}

	// --- Regime context ---
	if macroRegime != "" {
		b.WriteString(fmt.Sprintf("\n<b>Konteks Regime: %s</b>\n", macroRegime))
		sentimentFearish := (data.CNNAvailable && data.CNNFearGreed <= 35) || (data.AAIIAvailable && data.AAIIBearish >= 45)
		sentimentGreedish := (data.CNNAvailable && data.CNNFearGreed >= 65) || (data.AAIIAvailable && data.AAIIBullish >= 45)

		switch {
		case sentimentFearish && (macroRegime == "GOLDILOCKS" || macroRegime == "DISINFLATIONARY"):
			b.WriteString("<i>Fear di tengah ekonomi yang sehat — peluang beli lebih kredibel.</i>\n")
		case sentimentFearish && (macroRegime == "RECESSION" || macroRegime == "STRESS"):
			b.WriteString("<i>Fear di tengah tekanan makro nyata — ketakutan mungkin memang tepat. Hati-hati mengandalkan sinyal contrarian.</i>\n")
		case sentimentGreedish && (macroRegime == "RECESSION" || macroRegime == "STRESS"):
			b.WriteString("<i>Greed di tengah resesi/stress — disconnected dari fundamental. Risiko koreksi tinggi.</i>\n")
		case sentimentGreedish && macroRegime == "GOLDILOCKS":
			b.WriteString("<i>Greed di kondisi ideal — wajar tapi tetap waspada jika sudah terlalu jauh.</i>\n")
		default:
			b.WriteString("<i>Sentiment sejalan dengan kondisi makro saat ini — tidak ada divergensi signifikan.</i>\n")
		}
	}

	return b.String()
}

// FormatWorldBankFundamentals formats the World Bank global macro fundamentals section.
// Suitable for appending to /macro global view.
func (f *Formatter) FormatWorldBankFundamentals(wb *fred.WorldBankData) string {
	if wb == nil || !wb.Available || len(wb.Countries) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n🌍 <b>GLOBAL FUNDAMENTALS</b> <i>(World Bank, latest annual)</i>\n")

	// Display in consistent currency order
	order := []string{"USD", "EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF"}

	for _, currency := range order {
		cm, ok := wb.Countries[currency]
		if !ok {
			continue
		}

		var parts []string

		if cm.GDPGrowthYoY != 0 {
			parts = append(parts, fmt.Sprintf("GDP %+.1f%%", cm.GDPGrowthYoY))
		}
		if cm.CurrentAccount != 0 {
			parts = append(parts, fmt.Sprintf("CA %+.1f%% GDP", cm.CurrentAccount))
		}
		if cm.InflationCPI != 0 {
			parts = append(parts, fmt.Sprintf("CPI %+.1f%%", cm.InflationCPI))
		}

		if len(parts) == 0 {
			continue
		}

		yearStr := ""
		if cm.Year > 0 {
			yearStr = fmt.Sprintf(" (%d)", cm.Year)
		}

		b.WriteString(fmt.Sprintf("<b>%s</b>%s: %s\n", currency, yearStr, strings.Join(parts, " | ")))
	}

	b.WriteString(fmt.Sprintf("<i>Source: World Bank API • %s</i>\n",
		fmtutil.FormatDateTimeWIB(wb.FetchedAt)))

	return b.String()
}
