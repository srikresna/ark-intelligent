package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// SystemPromptTemplate is the base system instruction for all financial analysis.
// It includes a placeholder for the current date to prevent AI date hallucinations.
const SystemPromptTemplate = `You are a senior institutional analyst specializing in COT (Commitments of Traders) data and macro positioning.

Today's date: %s (UTC+7 WIB).

Rules:
- RESPOND IN THE LANGUAGE REQUESTED BY THE USER PROMPT (Indonesian/Bahasa Indonesia OR English).
- Be concise and actionable. Use bullet points.
- Always state the directional bias (BULLISH/BEARISH/NEUTRAL) clearly.
- Cite specific numbers from the data provided.
- Note any conflicting signals or risks.
- Format for Telegram HTML: you may use ONLY these tags: <b>bold</b>, <i>italic</i>, <code>code</code>. NO other HTML tags.
- NEVER use angle brackets < > for anything other than the allowed HTML tags above. Write currency pairs, labels, and terms as plain text (e.g. write USD not <USD>, write rate cuts not <rate cuts>).
- Keep responses under 800 words.
- Use WIB (UTC+7) for all times.
- IMPORTANT: Use ONLY the dates provided in the data. Do NOT invent or guess report dates. The COT report dates in the data are authoritative.`

// SystemPrompt returns the system prompt with the current date injected.
func SystemPrompt() string {
	now := time.Now().UTC().Add(7 * time.Hour) // WIB = UTC+7
	return fmt.Sprintf(SystemPromptTemplate, now.Format("Monday, 02 January 2006"))
}

// --- Prompt Builders ---

// BuildCOTAnalysisPrompt creates a prompt for COT data interpretation.
// priceContexts is optional — if provided, price data is included for each contract.
func BuildCOTAnalysisPrompt(analyses []domain.COTAnalysis, priceContexts ...map[string]*domain.PriceContext) string {
	var b strings.Builder
	b.WriteString("Analyze the following COT positioning data for G8 currency futures.\n")
	b.WriteString("Identify the strongest directional setups and any divergences.\n\n")

	// Extract price context map if provided
	var priceMap map[string]*domain.PriceContext
	if len(priceContexts) > 0 {
		priceMap = priceContexts[0]
	}

	for _, a := range analyses {
		b.WriteString(fmt.Sprintf("--- %s (%s) | Report: %s ---\n",
			a.Contract.Code, a.Contract.Currency, a.ReportDate.Format("2006-01-02")))
		b.WriteString(fmt.Sprintf("Spec Net: %s (chg: %s) | L/S Ratio: %s\n",
			fmtutil.FmtNumSigned(a.NetPosition, 0),
			fmtutil.FmtNumSigned(a.NetChange, 0),
			fmtutil.FmtNum(a.LongShortRatio, 2)))
		b.WriteString(fmt.Sprintf("Comm Net: %s (chg: %s, mom4W: %s) | L/S: %s\n",
			fmtutil.FmtNumSigned(a.NetCommercial, 0),
			fmtutil.FmtNumSigned(a.CommNetChange, 0),
			fmtutil.FmtNumSigned(a.CommMomentum4W, 0),
			fmtutil.FmtNum(a.CommLSRatio, 2)))
		b.WriteString(fmt.Sprintf("COT Index: Spec=%.1f Comm=%.1f\n", a.COTIndex, a.COTIndexComm))
		b.WriteString(fmt.Sprintf("Momentum 4W: Spec=%s Comm=%s | 8W: Spec=%s\n",
			fmtutil.FmtNumSigned(a.SpecMomentum4W, 0),
			fmtutil.FmtNumSigned(a.CommMomentum4W, 0),
			fmtutil.FmtNumSigned(a.SpecMomentum8W, 0)))
		if a.ConsecutiveWeeks > 0 {
			b.WriteString(fmt.Sprintf("Trend Streak: %d consecutive weeks same direction\n", a.ConsecutiveWeeks))
		}
		b.WriteString(fmt.Sprintf("OI Context: Trend=%s Chg=%s (%.1f%%) | STBias=%s\n",
			a.OITrend, fmtutil.FmtNumSigned(a.OpenInterestChg, 0), a.OIPctChange, a.ShortTermBias))
		if a.SpreadPctOfOI > 0 {
			b.WriteString(fmt.Sprintf("Spread Positions: %.1f%% of OI\n", a.SpreadPctOfOI))
		}
		b.WriteString(fmt.Sprintf("Sentiment: %.1f | Crowding: %.1f | Divergence: %v\n",
			a.SentimentScore, a.CrowdingIndex, a.DivergenceFlag))
		b.WriteString(fmt.Sprintf("Signals: Comm=%s Spec=%s SmallSpec=%s\n",
			a.CommercialSignal, a.SpeculatorSignal, a.SmallSpecSignal))
		b.WriteString(fmt.Sprintf("Concentration: Top4=%.1f%% Top8=%.1f%%\n",
			a.Top4Concentration, a.Top8Concentration))
		// Trader depth — thin market is a critical risk factor
		if a.TotalTraders > 0 {
			b.WriteString(fmt.Sprintf("Trader Depth: %d total (%s)", a.TotalTraders, a.TraderConcentration))
			if a.ThinMarketAlert {
				b.WriteString(fmt.Sprintf(" ⚠️ THIN: %s", a.ThinMarketDesc))
			}
			b.WriteString("\n")
		}
		// Price context (if available)
		if priceMap != nil {
			if pc, ok := priceMap[a.Contract.Code]; ok {
				b.WriteString(fmt.Sprintf("Price: %.5f | Wk: %+.2f%% | Mo: %+.2f%% | Trend4W: %s | MA4W: %s MA13W: %s\n",
					pc.CurrentPrice, pc.WeeklyChgPct, pc.MonthlyChgPct, pc.Trend4W,
					maLabel(pc.AboveMA4W), maLabel(pc.AboveMA13W)))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\nProvide:\n")
	b.WriteString("1. Overall positioning summary (which currencies are most/least favored)\n")
	b.WriteString("2. Smart money (commercial) vs speculator alignment for each\n")
	b.WriteString("3. Top 3 actionable setups with direction and conviction level\n")
	b.WriteString("4. Key risks or conflicting signals to watch\n")
	if priceMap != nil {
		b.WriteString("5. Price-positioning alignment: Flag any cases where price trend contradicts COT positioning (potential divergence/reversal signals)\n")
	}

	return b.String()
}

// maLabel returns "above" or "below" for MA status.
func maLabel(above bool) string {
	if above {
		return "above"
	}
	return "below"
}

// BuildWeeklyOutlookPrompt creates a prompt for weekly market outlook.
//
// Gap E: accepts optional macroRegime — if provided, injects FRED macro regime context
// so the COT-focused outlook is always regime-aware, without requiring /outlook combine.
// backtestStats is optional — if provided, includes signal accuracy context.
// priceContexts is optional — if provided, injects per-currency price lines (close, weekly %, MA position).
func BuildWeeklyOutlookPrompt(data WeeklyOutlookData, lang string, macroRegime *fred.MacroRegime, backtestStats ...*domain.BacktestStats) string {
	var b strings.Builder
	now := time.Now().UTC().Add(7 * time.Hour) // WIB
	b.WriteString("Generate a comprehensive weekly forex fundamental outlook.\n")
	b.WriteString(fmt.Sprintf("Analysis date: %s (WIB).\n", now.Format("02 January 2006")))

	if lang == "en" {
		b.WriteString("PLEASE RESPOND IN ENGLISH.\n\n")
	} else {
		b.WriteString("PLEASE RESPOND IN INDONESIAN (Bahasa Indonesia).\n\n")
	}

	// COT Summary
	if len(data.COTAnalyses) > 0 {
		b.WriteString("=== COT POSITIONING ===\n")
		for _, a := range data.COTAnalyses {
			b.WriteString(fmt.Sprintf("%s (Report: %s): SpecNet=%s COTIdx=%.0f CommSignal=%s 4WMom=%s OITrend=%s STBias=%s",
				a.Contract.Currency,
				a.ReportDate.Format("2006-01-02"),
				fmtutil.FmtNumSigned(a.NetPosition, 0),
				a.COTIndex, a.CommercialSignal,
				fmtutil.FmtNumSigned(a.SpecMomentum4W, 0),
				a.OITrend, a.ShortTermBias))
			if a.AssetMgrAlert {
				b.WriteString(fmt.Sprintf(" [⚠️ AssetMgrAlert Z=%.1f]", a.AssetMgrZScore))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Price context — per-currency close, weekly/monthly change, MA position
	if len(data.PriceContexts) > 0 {
		b.WriteString("=== PRICE CONTEXT (Weekly Closes) ===\n")
		for _, a := range data.COTAnalyses {
			if pc, ok := data.PriceContexts[a.Contract.Code]; ok {
				b.WriteString(fmt.Sprintf("%s: %.5f | Wk %+.2f%% | Mo %+.2f%% | Trend4W: %s | MA4W: %s MA13W: %s\n",
					a.Contract.Currency,
					pc.CurrentPrice, pc.WeeklyChgPct, pc.MonthlyChgPct,
					pc.Trend4W, maLabel(pc.AboveMA4W), maLabel(pc.AboveMA13W)))
			}
		}
		b.WriteString("\n")
	}

	// Gap E: inject FRED regime context when available
	if macroRegime != nil {
		b.WriteString("=== FRED MACRO REGIME (CONTEXT) ===\n")
		b.WriteString(fmt.Sprintf("Regime: %s | Risk-Off Score: %d/100\n", macroRegime.Name, macroRegime.Score))
		b.WriteString(fmt.Sprintf("Yield Curve: %s\n", macroRegime.YieldCurve))
		b.WriteString(fmt.Sprintf("Financial Stress: %s\n", macroRegime.FinStress))
		b.WriteString(fmt.Sprintf("Implied Bias: %s\n", macroRegime.Bias))
		b.WriteString("NOTE: Adjust currency biases above considering this macro regime context.\n\n")
	}

	// Backtest accuracy context (if available)
	if len(backtestStats) > 0 && backtestStats[0] != nil {
		bs := backtestStats[0]
		if bs.Evaluated > 0 {
			b.WriteString("=== SIGNAL ACCURACY CONTEXT ===\n")
			b.WriteString(fmt.Sprintf("Historical signals: %d evaluated\n", bs.Evaluated))
			b.WriteString(fmt.Sprintf("Win rates: 1W=%.0f%% 2W=%.0f%% 4W=%.0f%%\n", bs.WinRate1W, bs.WinRate2W, bs.WinRate4W))
			b.WriteString(fmt.Sprintf("Best holding period: %s (%.0f%% win rate)\n", bs.BestPeriod, bs.BestWinRate))
			if bs.HighStrengthCount > 0 {
				b.WriteString(fmt.Sprintf("High-strength signals (4-5): %.0f%% win rate\n", bs.HighStrengthWinRate))
			}
			b.WriteString("NOTE: Weight your conviction based on historical accuracy. High-strength signals have proven more reliable.\n\n")
		}
	}

	hasPriceCtx := len(data.PriceContexts) > 0
	if lang == "en" {
		b.WriteString("\nProvide a structured weekly outlook in ENGLISH:\n")
		b.WriteString("1. MACRO THEME: Key market drivers for this week\n")
		b.WriteString("2. CURRENCY OUTLOOK: Bullish/Bearish bias for G8 currencies with reasoning\n")
		b.WriteString("3. TOP TRADES: 3 highest conviction trade ideas with rationale\n")
		b.WriteString("4. KEY RISKS: Scenarios that could invalidate the analysis\n")
		b.WriteString("5. SCALPER INTEL: Intraday/Swing recommendations (Buy Dips/Sell Rallies) based on 4W Momentum & OI Trend data\n")
		if hasPriceCtx {
			b.WriteString("6. PRICE-COT DIVERGENCE: Flag any currency where price trend contradicts COT positioning (e.g. price rising but smart money reducing longs) — these are potential reversal setups.\n")
		}
	} else {
		b.WriteString("\nProvide a structured weekly outlook in INDONESIAN:\n")
		b.WriteString("1. MACRO THEME: Tema utama penggerak pasar minggu ini\n")
		b.WriteString("2. CURRENCY OUTLOOK: Bias Bullish/Bearish untuk mata uang G8 beserta alasannya\n")
		b.WriteString("3. TOP TRADES: 3 ide trading dengan keyakinan tertinggi beserta logikanya\n")
		b.WriteString("4. KEY RISKS: Skenario yang dapat membatalkan analisis ini\n")
		b.WriteString("5. SCALPER INTEL: Rekomendasi Intraday/Swing (Buy Dips/Sell Rallies) berdasarkan data 4W Momentum & OI Trend\n")
		if hasPriceCtx {
			b.WriteString("6. PRICE-COT DIVERGENCE: Tandai mata uang di mana tren harga bertentangan dengan positioning COT (mis. harga naik tapi smart money mengurangi long) — ini adalah setup reversal potensial.\n")
		}
	}

	return b.String()
}

// BuildCrossMarketPrompt creates a prompt for cross-market COT analysis.
func BuildCrossMarketPrompt(cotData map[string]*domain.COTAnalysis) string {
	var b strings.Builder
	now := time.Now().UTC().Add(7 * time.Hour) // WIB
	b.WriteString(fmt.Sprintf("Analyze cross-market COT positioning for intermarket signals.\n"))
	b.WriteString(fmt.Sprintf("Analysis date: %s (WIB).\n", now.Format("02 January 2006")))
	b.WriteString("Look for correlations, divergences, and risk-on/risk-off signals.\n\n")

	for code, a := range cotData {
		if a == nil {
			continue
		}
		b.WriteString(fmt.Sprintf("%s (%s) | Report: %s: SpecNet=%s COTIdx=%.0f Sentiment=%.0f Crowd=%.0f\n",
			code, a.Contract.Currency,
			a.ReportDate.Format("2006-01-02"),
			fmtutil.FmtNumSigned(a.NetPosition, 0),
			a.COTIndex, a.SentimentScore, a.CrowdingIndex))
	}

	b.WriteString("\nProvide:\n")
	b.WriteString("1. Risk-on vs risk-off positioning assessment\n")
	b.WriteString("2. Safe-haven demand signals (JPY, CHF, USD)\n")
	b.WriteString("3. Commodity currency alignment (AUD, NZD, CAD)\n")
	b.WriteString("4. Any unusual cross-market divergences\n")

	return b.String()
}

// BuildNewsOutlookPrompt creates a prompt for analyzing the weekly economic calendar.
//
// Gap E: accepts optional macroRegime — if provided, injects FRED regime context so
// news analysis is always macro-aware without requiring /outlook combine.
func BuildNewsOutlookPrompt(events []domain.NewsEvent, lang string, macroRegime *fred.MacroRegime) string {
	var b strings.Builder
	b.WriteString("Analyze the following economic calendar events for the week.\n")

	if lang == "en" {
		b.WriteString("PLEASE RESPOND IN ENGLISH.\n\n")
	} else {
		b.WriteString("PLEASE RESPOND IN INDONESIAN (Bahasa Indonesia).\n\n")
	}

	b.WriteString("=== ECONOMIC CALENDAR ===\n")
	for _, e := range events {
		if e.Impact == "high" || e.Impact == "medium" {
			line := fmt.Sprintf("%s | %s - %s | Impact: %s | Fcast: %s | Prev: %s | Act: %s",
				e.Date, e.Currency, e.Event, e.Impact,
				e.Forecast, e.Previous, e.Actual)
			if e.SurpriseScore != 0 {
				line += fmt.Sprintf(" | Surprise: %.1fσ %s", e.SurpriseScore, e.SurpriseLabel)
			}
			if e.OldPrevious != "" && e.OldPrevious != e.Previous {
				line += fmt.Sprintf(" | Rev: %s→%s", e.OldPrevious, e.Previous)
			}
			b.WriteString(line + "\n")
		}
	}

	// Gap E: inject FRED regime context when available
	if macroRegime != nil {
		b.WriteString("\n=== FRED MACRO REGIME (CONTEXT) ===\n")
		b.WriteString(fmt.Sprintf("Regime: %s | Risk-Off Score: %d/100\n", macroRegime.Name, macroRegime.Score))
		b.WriteString(fmt.Sprintf("Yield Curve: %s\n", macroRegime.YieldCurve))
		b.WriteString(fmt.Sprintf("Financial Stress: %s\n", macroRegime.FinStress))
		b.WriteString(fmt.Sprintf("Implied Bias: %s\n", macroRegime.Bias))
		b.WriteString("NOTE: Adjust currency biases above considering this macro regime context.\n")
	}

	if lang == "en" {
		b.WriteString("\nProvide a structured outlook:\n")
		b.WriteString("1. Currency Strength Context: Discuss which pairs will be most volatile based on event density.\n")
		b.WriteString("2. Storm Days Detection: Identify days with multiple clustered high-impact events across countries.\n")
		b.WriteString("3. Fundamental Tracking: Analyze the trajectory of repeating data (e.g. CPI/NFP trends) based on Forecast vs Previous.\n")
		b.WriteString("4. Central Bank Watch: Highlight any rate decisions, speeches, or minutes acting as macro catalysts.\n")
	} else {
		b.WriteString("\nBerikan outlook terstruktur:\n")
		b.WriteString("1. Currency Strength Context: Diskusikan pasangan mana yang paling volatil berdasarkan kepadatan event.\n")
		b.WriteString("2. Storm Days Detection: Identifikasi hari dengan beberapa event high-impact yang berkluster di berbagai negara.\n")
		b.WriteString("3. Fundamental Tracking: Analisis tren data berulang (Contoh: tren CPI/NFP) berdasarkan Forecast vs Previous.\n")
		b.WriteString("4. Central Bank Watch: Sorot keputusan suku bunga, pidato, atau minutes yang menjadi katalis makro.\n")
	}

	return b.String()
}

// BuildCombinedOutlookPrompt creates a prompt for fusing COT positioning and calendar news.
func BuildCombinedOutlookPrompt(data ports.WeeklyData) string {
	var b strings.Builder
	now := time.Now().UTC().Add(7 * time.Hour) // WIB
	b.WriteString("Generate a fused analysis combining COT Speculator Positioning and Upcoming Economic Catalysts.\n")
	b.WriteString(fmt.Sprintf("Analysis date: %s (WIB).\n", now.Format("02 January 2006")))

	if data.Language == "en" {
		b.WriteString("PLEASE RESPOND IN ENGLISH.\n\n")
	} else {
		b.WriteString("PLEASE RESPOND IN INDONESIAN (Bahasa Indonesia).\n\n")
	}

	b.WriteString("=== 1. COT POSITIONING ===\n")
	for _, a := range data.COTAnalyses {
		b.WriteString(fmt.Sprintf("%s: SpecNet=%s COTIdx=%.0f CommSignal=%s Crowding=%.1f\n",
			a.Contract.Currency,
			fmtutil.FmtNumSigned(a.NetPosition, 0),
			a.COTIndex, a.CommercialSignal, a.CrowdingIndex))
	}

	b.WriteString("\n=== 2. UPCOMING CATALYSTS (HIGH IMPACT) ===\n")
	for _, e := range data.NewsEvents {
		if e.Impact == "high" {
			line := fmt.Sprintf("%s | %s - %s | Fcast: %s | Act: %s",
				e.Date, e.Currency, e.Event, e.Forecast, e.Actual)
			if e.SurpriseScore != 0 {
				line += fmt.Sprintf(" | %.1fσ %s", e.SurpriseScore, e.SurpriseLabel)
			}
			b.WriteString(line + "\n")
		}
	}

	if data.Language == "en" {
		b.WriteString("\nProvide a structured fused outlook:\n")
		b.WriteString("1. Positioning Extreme + Catalyst Alignment: Identify 'Crowded exit risks'. e.g., if EUR is heavily net long and ECB is upcoming, what is the fragility risk?\n")
		b.WriteString("2. The Volatility Window: Highlight which pairs will experience liquidity compression before their respective events.\n")
		b.WriteString("3. Surprise Factor Scenarios: For the top 2 events, model what happens if Actual significantly beats or misses Forecast against the current COT positioning.\n")
	} else {
		b.WriteString("\nBerikan analisis fusi terstruktur:\n")
		b.WriteString("1. Positioning Extreme + Catalyst Alignment: Identifikasi 'Crowded exit risks'. Contoh: jika EUR net long besar dan ECB akan datang, apa risiko kerapuhannya?\n")
		b.WriteString("2. The Volatility Window: Sorot pasangan mana yang akan mengalami kompresi likuiditas sebelum event masing-masing.\n")
		b.WriteString("3. Surprise Factor Scenarios: Untuk 2 event teratas, modelkan apa yang terjadi jika Actual jauh melampaui atau meleset dari Forecast terhadap positioning COT saat ini.\n")
	}

	return b.String()
}

// BuildFREDOutlookPrompt creates a comprehensive prompt for AI macro analysis
// using FRED quantitative data and the derived macro regime classification.
func BuildFREDOutlookPrompt(data *fred.MacroData, regime fred.MacroRegime, lang string) string {
	var b strings.Builder
	b.WriteString("Analyze the following FRED (Federal Reserve Economic Data) macro indicators and provide a trading-oriented macro outlook.\n")

	if lang == "en" {
		b.WriteString("PLEASE RESPOND IN ENGLISH.\n\n")
	} else {
		b.WriteString("PLEASE RESPOND IN INDONESIAN (Bahasa Indonesia).\n\n")
	}

	b.WriteString("=== FRED QUANTITATIVE DATA ===\n")

	// Yield curve (2Y-10Y)
	b.WriteString(fmt.Sprintf("2Y Treasury:       %.2f%%\n", data.Yield2Y))
	b.WriteString(fmt.Sprintf("10Y Treasury:      %.2f%%\n", data.Yield10Y))
	b.WriteString(fmt.Sprintf("2Y-10Y Spread:     %+.2f%% %s (%s)\n",
		data.YieldSpread, data.YieldSpreadTrend.Arrow(), regime.YieldCurve))

	// 3M-10Y spread (NY Fed recession predictor)
	if data.Yield3M > 0 {
		b.WriteString(fmt.Sprintf("3M Treasury:       %.2f%%\n", data.Yield3M))
		b.WriteString(fmt.Sprintf("3M-10Y Spread:     %+.2f%% (%s)\n", data.Spread3M10Y, regime.Yield3M10Y))
	}

	// Inflation
	if data.CorePCE > 0 {
		b.WriteString(fmt.Sprintf("Core PCE:          %.2f%% %s (%s)\n",
			data.CorePCE, data.CorePCETrend.Arrow(), regime.Inflation))
	}
	if data.CPI > 0 {
		b.WriteString(fmt.Sprintf("CPI (headline):    %.2f%% %s\n", data.CPI, data.CPITrend.Arrow()))
	}
	if data.Breakeven5Y > 0 {
		b.WriteString(fmt.Sprintf("10Y Breakeven:     %.2f%%\n", data.Breakeven5Y))
	}

	// Monetary policy
	if data.FedFundsRate > 0 {
		realRate := data.FedFundsRate - data.Breakeven5Y
		b.WriteString(fmt.Sprintf("Fed Funds Rate:    %.2f%% (Real Rate: %+.2f%%) (%s)\n",
			data.FedFundsRate, realRate, regime.MonPolicy))
	}
	if data.SOFR > 0 {
		b.WriteString(fmt.Sprintf("SOFR:              %.2f%%\n", data.SOFR))
	}
	if data.IORB > 0 {
		b.WriteString(fmt.Sprintf("IORB:              %.2f%%\n", data.IORB))
	}
	if regime.SOFRLabel != "N/A" && regime.SOFRLabel != "" {
		b.WriteString(fmt.Sprintf("SOFR/IORB Status:  %s\n", regime.SOFRLabel))
	}

	// Financial stress
	b.WriteString(fmt.Sprintf("NFCI:              %.3f %s (%s)\n",
		data.NFCI, data.NFCITrend.Arrow(), regime.FinStress))
	if data.TedSpread > 0 {
		b.WriteString(fmt.Sprintf("HY Credit Spread:  %.2f%% (ICE BofA OAS; >4%%=elevated, >6%%=stress)\n", data.TedSpread))
	}

	// Labor
	if data.InitialClaims > 0 {
		b.WriteString(fmt.Sprintf("Initial Claims:    %.0fK/week %s\n",
			data.InitialClaims/1_000, data.ClaimsTrend.Arrow()))
	}
	if data.UnemployRate > 0 {
		b.WriteString(fmt.Sprintf("Unemployment:      %.1f%%\n", data.UnemployRate))
	}

	// Sahm Rule
	if data.SahmRule > 0 {
		triggered := ""
		if data.SahmRule >= 0.5 {
			triggered = " ⚠️ RECESSION SIGNAL"
		}
		b.WriteString(fmt.Sprintf("Sahm Rule:         %.2f%s (%s)\n",
			data.SahmRule, triggered, regime.SahmLabel))
	}

	// Growth
	if data.GDPGrowth != 0 {
		b.WriteString(fmt.Sprintf("Real GDP Growth:   %.1f%% QoQ ann. (%s)\n", data.GDPGrowth, regime.Growth))
	}

	// M2 money supply
	if data.M2Growth != 0 {
		b.WriteString(fmt.Sprintf("M2 YoY Growth:     %+.1f%% %s (%s)\n",
			data.M2Growth, data.M2GrowthTrend.Arrow(), regime.M2Label))
	}

	// Fed balance sheet
	if data.FedBalSheet > 0 {
		b.WriteString(fmt.Sprintf("Fed Balance Sheet: $%.2fT %s (%s)\n",
			data.FedBalSheet/1_000, data.FedBalSheetTrend.Arrow(), regime.FedBalance))
	}

	// USD
	if data.DXY > 0 {
		b.WriteString(fmt.Sprintf("USD Index (DXY):   %.1f (%s)\n", data.DXY, regime.USDStrength))
	}

	b.WriteString("\n=== DERIVED REGIME ===\n")
	b.WriteString(fmt.Sprintf("Macro Regime:      %s\n", regime.Name))
	b.WriteString(fmt.Sprintf("Risk-Off Score:    %d/100\n", regime.Score))
	b.WriteString(fmt.Sprintf("Recession Risk:    %s\n", regime.RecessionRisk))
	b.WriteString(fmt.Sprintf("Implied Bias:      %s\n", regime.Bias))

	b.WriteString("\n=== ANALYSIS REQUESTED ===\n")
	b.WriteString("Provide a structured FRED Macro Outlook covering:\n")
	b.WriteString("1. FED POLICY OUTLOOK: Given current FFR, real rate, inflation trends (Core PCE + CPI arrows), and yield curve shape — what is the likely Fed trajectory? Rate cuts, holds, or hikes?\n")
	b.WriteString("2. USD STRUCTURAL BIAS: Based on real rates (FFR - breakeven), SOFR/IORB spread, DXY level, M2 growth trend, and financial conditions — what is the medium-term dollar outlook?\n")
	b.WriteString("3. RISK APPETITE: Using NFCI trend, both yield curves (2Y-10Y AND 3M-10Y), Sahm Rule, and labor data together — assess current risk-on vs risk-off pressure.\n")
	b.WriteString("4. GOLD & SAFE HAVENS: Given real yields, financial stress, Fed balance sheet direction (QE/QT), and Sahm Rule reading — is gold/JPY/CHF structurally attractive?\n")
	b.WriteString("5. GROWTH TRAJECTORY: Based on GDP + labor + yield curve + Sahm Rule — is the economy heading toward expansion, slowdown, or recession?\n")
	b.WriteString("6. KEY INFLECTION POINTS: What specific data releases (e.g. next CPI, NFP, FOMC) could change this regime? What Sahm/curve levels would trigger regime shift?\n")

	return b.String()
}

// BuildCombinedWithFREDPrompt creates a fused prompt that includes FRED macro context
// alongside COT positioning and economic calendar catalysts.
func BuildCombinedWithFREDPrompt(data ports.WeeklyData, regime fred.MacroRegime) string {
	var b strings.Builder
	now := time.Now().UTC().Add(7 * time.Hour) // WIB
	b.WriteString("Generate a comprehensive market outlook fusing COT Speculator Positioning, Economic Calendar Catalysts, and FRED Macro Fundamentals.\n")
	b.WriteString(fmt.Sprintf("Analysis date: %s (WIB).\n", now.Format("02 January 2006")))

	if data.Language == "en" {
		b.WriteString("PLEASE RESPOND IN ENGLISH.\n\n")
	} else {
		b.WriteString("PLEASE RESPOND IN INDONESIAN (Bahasa Indonesia).\n\n")
	}

	b.WriteString("=== 1. COT POSITIONING ===\n")
	for _, a := range data.COTAnalyses {
		b.WriteString(fmt.Sprintf("%s: SpecNet=%s COTIdx=%.0f CommSignal=%s Crowding=%.1f\n",
			a.Contract.Currency,
			fmtutil.FmtNumSigned(a.NetPosition, 0),
			a.COTIndex, a.CommercialSignal, a.CrowdingIndex))
	}

	b.WriteString("\n=== 2. UPCOMING CATALYSTS (HIGH IMPACT) ===\n")
	for _, e := range data.NewsEvents {
		if e.Impact == "high" {
			line := fmt.Sprintf("%s | %s - %s | Fcast: %s | Act: %s",
				e.Date, e.Currency, e.Event, e.Forecast, e.Actual)
			if e.SurpriseScore != 0 {
				line += fmt.Sprintf(" | %.1fσ %s", e.SurpriseScore, e.SurpriseLabel)
			}
			b.WriteString(line + "\n")
		}
	}

	// Price context — inject per-currency close, weekly/monthly change, MA position
	if len(data.PriceContexts) > 0 {
		b.WriteString("\n=== 3. PRICE CONTEXT (Weekly Closes) ===\n")
		for _, a := range data.COTAnalyses {
			if pc, ok := data.PriceContexts[a.Contract.Code]; ok {
				b.WriteString(fmt.Sprintf("%s: %.5f | Wk %+.2f%% | Mo %+.2f%% | Trend4W: %s | MA4W: %s MA13W: %s\n",
					a.Contract.Currency,
					pc.CurrentPrice, pc.WeeklyChgPct, pc.MonthlyChgPct,
					pc.Trend4W, maLabel(pc.AboveMA4W), maLabel(pc.AboveMA13W)))
			}
		}
	}

	if data.MacroData != nil {
		b.WriteString("\n=== 4. FRED MACRO BACKDROP ===\n")
		m := data.MacroData
		b.WriteString(fmt.Sprintf("Macro Regime: %s (Risk-Off Score: %d/100 | Recession Risk: %s)\n",
			regime.Name, regime.Score, regime.RecessionRisk))

		// Yield curves
		b.WriteString(fmt.Sprintf("2Y-10Y Spread: %+.2f%% %s (%s)\n",
			m.YieldSpread, m.YieldSpreadTrend.Arrow(), regime.YieldCurve))
		if m.Yield3M > 0 {
			b.WriteString(fmt.Sprintf("3M-10Y Spread: %+.2f%% (%s)\n", m.Spread3M10Y, regime.Yield3M10Y))
		}

		// Inflation
		if m.CorePCE > 0 {
			b.WriteString(fmt.Sprintf("Core PCE: %.2f%% %s | ", m.CorePCE, m.CorePCETrend.Arrow()))
		}
		if m.CPI > 0 {
			b.WriteString(fmt.Sprintf("CPI: %.2f%% %s\n", m.CPI, m.CPITrend.Arrow()))
		} else {
			b.WriteString("\n")
		}

		// Monetary policy
		if m.FedFundsRate > 0 {
			realRate := m.FedFundsRate - m.Breakeven5Y
			b.WriteString(fmt.Sprintf("FFR: %.2f%% (Real: %+.2f%%)", m.FedFundsRate, realRate))
		}
		if m.SOFR > 0 && m.IORB > 0 {
			b.WriteString(fmt.Sprintf(" | SOFR: %.2f%% IORB: %.2f%%", m.SOFR, m.IORB))
		}
		b.WriteString("\n")

		// Financial stress
		b.WriteString(fmt.Sprintf("NFCI: %.3f %s (%s)\n", m.NFCI, m.NFCITrend.Arrow(), regime.FinStress))

		// Labor
		if m.InitialClaims > 0 {
			b.WriteString(fmt.Sprintf("Claims: %.0fK %s | ", m.InitialClaims/1_000, m.ClaimsTrend.Arrow()))
		}
		if m.UnemployRate > 0 {
			b.WriteString(fmt.Sprintf("U-Rate: %.1f%%\n", m.UnemployRate))
		} else {
			b.WriteString("\n")
		}

		// Sahm Rule
		if m.SahmRule > 0 {
			b.WriteString(fmt.Sprintf("Sahm Rule: %.2f (%s)\n", m.SahmRule, regime.SahmLabel))
		}

		// Growth & money supply
		if m.GDPGrowth != 0 {
			b.WriteString(fmt.Sprintf("GDP Growth: %.1f%% QoQ ann.\n", m.GDPGrowth))
		}
		if m.M2Growth != 0 {
			b.WriteString(fmt.Sprintf("M2 YoY: %+.1f%% %s (%s)\n",
				m.M2Growth, m.M2GrowthTrend.Arrow(), regime.M2Label))
		}

		// Fed balance sheet
		if m.FedBalSheet > 0 {
			b.WriteString(fmt.Sprintf("Fed Balance: $%.2fT %s (%s)\n",
				m.FedBalSheet/1_000, m.FedBalSheetTrend.Arrow(), regime.FedBalance))
		}

		// USD
		if m.DXY > 0 {
			b.WriteString(fmt.Sprintf("DXY: %.1f (%s)\n", m.DXY, regime.USDStrength))
		}
		b.WriteString(fmt.Sprintf("Implied Bias: %s\n", regime.Bias))
	}

	hasPriceCtxCombined := len(data.PriceContexts) > 0
	b.WriteString("\n=== ANALYSIS REQUESTED ===\n")
	if data.Language == "en" {
		b.WriteString("Provide a fused trading outlook:\n")
		b.WriteString("1. MACRO-COT ALIGNMENT: Where do FRED macro signals confirm or conflict with COT positioning? Identify high-conviction setups.\n")
		b.WriteString("2. CATALYST + POSITIONING RISK: For top upcoming events, overlay current COT crowding to identify fragile setups (crowded longs/shorts facing catalyst risk).\n")
		b.WriteString("3. REGIME-ADJUSTED TRADES: Given the macro regime, which COT-driven trade ideas have the strongest macro tailwind?\n")
		b.WriteString("4. RISK SCENARIOS: What would change the outlook? (e.g., FOMC surprise, inflation shock, weak NFP)\n")
		if hasPriceCtxCombined {
			b.WriteString("5. PRICE-COT DIVERGENCE: Flag any currency where price trend contradicts COT positioning — these are potential reversal or trap setups.\n")
		}
	} else {
		b.WriteString("Berikan outlook trading fusi:\n")
		b.WriteString("1. MACRO-COT ALIGNMENT: Di mana sinyal makro FRED mengkonfirmasi atau bertentangan dengan positioning COT? Identifikasi setup high-conviction.\n")
		b.WriteString("2. CATALYST + POSITIONING RISK: Untuk event besar mendatang, overlay crowding COT saat ini untuk identifikasi setup rapuh (crowded longs/shorts menghadapi risiko katalis).\n")
		b.WriteString("3. REGIME-ADJUSTED TRADES: Berdasarkan regime makro, ide trading COT mana yang memiliki tailwind makro paling kuat?\n")
		b.WriteString("4. RISK SCENARIOS: Apa yang bisa mengubah outlook? (Contoh: kejutan FOMC, kejutan inflasi, NFP lemah)\n")
		if hasPriceCtxCombined {
			b.WriteString("5. PRICE-COT DIVERGENCE: Tandai mata uang di mana tren harga bertentangan dengan positioning COT — ini adalah setup reversal atau jebakan potensial.\n")
		}
	}

	return b.String()
}

// BuildActualReleasePrompt evaluates a single economic release.
func BuildActualReleasePrompt(event domain.NewsEvent, lang string) string {
	var b strings.Builder
	b.WriteString("Analyze this specific economic data release and its immediate currency impact.\n")
	if lang == "en" {
		b.WriteString("PLEASE RESPOND IN ENGLISH.\n\n")
	} else {
		b.WriteString("PLEASE RESPOND IN INDONESIAN (Bahasa Indonesia).\n\n")
	}

	b.WriteString(fmt.Sprintf("Event: %s\nCurrency: %s\nImpact: %s\n", event.Event, event.Currency, event.Impact))
	b.WriteString(fmt.Sprintf("Previous: %s\nForecast: %s\nActual: %s\n", event.Previous, event.Forecast, event.Actual))
	if event.SurpriseScore != 0 {
		b.WriteString(fmt.Sprintf("Surprise: %.2f sigma (%s)\n", event.SurpriseScore, event.SurpriseLabel))
	}
	if event.ImpactDirection == 1 {
		b.WriteString("MQL5 Impact: BULLISH for currency (higher actual = positive)\n")
	} else if event.ImpactDirection == 2 {
		b.WriteString("MQL5 Impact: BEARISH for currency (higher actual = negative)\n")
	}
	if event.OldPrevious != "" && event.OldPrevious != event.Previous {
		b.WriteString(fmt.Sprintf("Revision: Previous revised from %s to %s", event.OldPrevious, event.Previous))
		if event.RevisionLabel != "" {
			b.WriteString(fmt.Sprintf(" (%s)", event.RevisionLabel))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("Provide a 3-sentence maximum flash analysis covering:\n")
	b.WriteString("1. The deviation (did it beat or miss expectations?).\n")
	b.WriteString(fmt.Sprintf("2. Immediate directional bias for %s pairs (Bullish/Bearish).\n", event.Currency))
	b.WriteString("3. The likely macro narrative traders will adopt based on this number.\n")

	return b.String()
}
