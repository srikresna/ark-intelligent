package ai

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
	"github.com/arkcode369/ff-calendar-bot/pkg/fmtutil"
)

// SystemPrompt is the base system instruction for all financial analysis.
const SystemPrompt = `You are a senior institutional analyst specializing in COT (Commitments of Traders) data and macro positioning.

Rules:
- RESPOND IN THE LANGUAGE REQUESTED BY THE USER PROMPT (Indonesian/Bahasa Indonesia OR English).
- Be concise and actionable. Use bullet points.
- Always state the directional bias (BULLISH/BEARISH/NEUTRAL) clearly.
- Cite specific numbers from the data provided.
- Note any conflicting signals or risks.
- Format for Telegram: use plain text, no markdown headers.
- Keep responses under 800 words.
- Use WIB (UTC+7) for all times.`

// --- Prompt Builders ---

// BuildCOTAnalysisPrompt creates a prompt for COT data interpretation.
func BuildCOTAnalysisPrompt(analyses []domain.COTAnalysis) string {
	var b strings.Builder
	b.WriteString("Analyze the following COT positioning data for G8 currency futures.\n")
	b.WriteString("Identify the strongest directional setups and any divergences.\n\n")

	for _, a := range analyses {
		b.WriteString(fmt.Sprintf("--- %s (%s) | Report: %s ---\n",
			a.Contract.Code, a.Contract.Currency, a.ReportDate.Format("2006-01-02")))
		b.WriteString(fmt.Sprintf("Spec Net: %s (chg: %s) | L/S Ratio: %s\n",
			fmtutil.FmtNumSigned(a.NetPosition, 0),
			fmtutil.FmtNumSigned(a.NetChange, 0),
			fmtutil.FmtNum(a.LongShortRatio, 2)))
		b.WriteString(fmt.Sprintf("Comm Net: %s (momentum 4W: %s) | L/S Ratio: %s\n",
			fmtutil.FmtNumSigned(a.NetCommercial, 0),
			fmtutil.FmtNumSigned(a.CommMomentum4W, 0),
			fmtutil.FmtNum(a.CommLSRatio, 2)))
		b.WriteString(fmt.Sprintf("COT Index: Spec=%.1f Comm=%.1f\n", a.COTIndex, a.COTIndexComm))
		b.WriteString(fmt.Sprintf("Momentum 4W: Spec=%s Comm=%s\n",
			fmtutil.FmtNumSigned(a.SpecMomentum4W, 0),
			fmtutil.FmtNumSigned(a.CommMomentum4W, 0)))
		b.WriteString(fmt.Sprintf("Intraday Context: OITrend=%s STBias=%s\n", a.OITrend, a.ShortTermBias))
		b.WriteString(fmt.Sprintf("Sentiment: %.1f | Crowding: %.1f | Divergence: %v\n",
			a.SentimentScore, a.CrowdingIndex, a.DivergenceFlag))
		b.WriteString(fmt.Sprintf("Signals: Comm=%s Spec=%s SmallSpec=%s\n",
			a.CommercialSignal, a.SpeculatorSignal, a.SmallSpecSignal))
		b.WriteString(fmt.Sprintf("Concentration: Top4=%.1f%% Top8=%.1f%%\n\n",
			a.Top4Concentration, a.Top8Concentration))
	}

	b.WriteString("\nProvide:\n")
	b.WriteString("1. Overall positioning summary (which currencies are most/least favored)\n")
	b.WriteString("2. Smart money (commercial) vs speculator alignment for each\n")
	b.WriteString("3. Top 3 actionable setups with direction and conviction level\n")
	b.WriteString("4. Key risks or conflicting signals to watch\n")

	return b.String()
}

// BuildWeeklyOutlookPrompt creates a prompt for weekly market outlook.
func BuildWeeklyOutlookPrompt(data WeeklyOutlookData, lang string) string {
	var b strings.Builder
	b.WriteString("Generate a comprehensive weekly forex fundamental outlook.\n")

	if lang == "en" {
		b.WriteString("PLEASE RESPOND IN ENGLISH.\n\n")
	} else {
		b.WriteString("PLEASE RESPOND IN INDONESIAN (Bahasa Indonesia).\n\n")
	}

	// COT Summary
	if len(data.COTAnalyses) > 0 {
		b.WriteString("=== COT POSITIONING ===\n")
		for _, a := range data.COTAnalyses {
			b.WriteString(fmt.Sprintf("%s: SpecNet=%s COTIdx=%.0f CommSignal=%s 4WMom=%s OITrend=%s STBias=%s",
				a.Contract.Currency,
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

	b.WriteString("\nProvide a structured weekly outlook in INDONESIAN:\n")
	b.WriteString("1. MACRO THEME: Tema utama penggerak pasar minggu ini\n")
	b.WriteString("2. CURRENCY OUTLOOK: Bias Bullish/Bearish untuk mata uang G8 beserta alasannya\n")
	b.WriteString("3. TOP TRADES: 3 ide trading dengan keyakinan tertinggi beserta logikanya\n")
	b.WriteString("4. KEY RISKS: Skenario yang dapat membatalkan analisis ini\n")
	b.WriteString("5. SCALPER INTEL: Rekomendasi Intraday/Swing (Buy Dips/Sell Rallies) berdasarkan data 4W Momentum & OI Trend\n")

	return b.String()
}

// BuildCrossMarketPrompt creates a prompt for cross-market COT analysis.
func BuildCrossMarketPrompt(cotData map[string]*domain.COTAnalysis) string {
	var b strings.Builder
	b.WriteString("Analyze cross-market COT positioning for intermarket signals.\n")
	b.WriteString("Look for correlations, divergences, and risk-on/risk-off signals.\n\n")

	for code, a := range cotData {
		if a == nil {
			continue
		}
		b.WriteString(fmt.Sprintf("%s (%s): SpecNet=%s COTIdx=%.0f Sentiment=%.0f Crowd=%.0f\n",
			code, a.Contract.Currency,
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
func BuildNewsOutlookPrompt(events []domain.NewsEvent, lang string) string {
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
			b.WriteString(fmt.Sprintf("%s | %s - %s | Impact: %s | Fcast: %s | Prev: %s | Act: %s\n",
				e.Date, e.Currency, e.Event, e.Impact,
				e.Forecast, e.Previous, e.Actual))
		}
	}

	b.WriteString("\nProvide a structured outlook:\n")
	b.WriteString("1. Currency Strength Context: Discuss which pairs will be most volatile based on event density.\n")
	b.WriteString("2. Storm Days Detection: Identify days with multiple clustered high-impact events across countries.\n")
	b.WriteString("3. Fundamental Tracking: Analyze the trajectory of repeating data (e.g. CPI/NFP trends) based on Forecast vs Previous.\n")
	b.WriteString("4. Central Bank Watch: Highlight any rate decisions, speeches, or minutes acting as macro catalysts.\n")

	return b.String()
}

// BuildCombinedOutlookPrompt creates a prompt for fusing COT positioning and calendar news.
func BuildCombinedOutlookPrompt(data ports.WeeklyData) string {
	var b strings.Builder
	b.WriteString("Generate a fused analysis combining COT Speculator Positioning and Upcoming Economic Catalysts.\n")

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
			b.WriteString(fmt.Sprintf("%s | %s - %s | Fcast: %s | Act: %s\n",
				e.Date, e.Currency, e.Event, e.Forecast, e.Actual))
		}
	}

	b.WriteString("\nProvide a structured fused outlook:\n")
	b.WriteString("1. Positioning Extreme + Catalyst Alignment: Identify 'Crowded exit risks'. e.g., if EUR is heavily net long and ECB is upcoming, what is the fragility risk?\n")
	b.WriteString("2. The Volatility Window: Highlight which pairs will experience liquidity compression before their respective events.\n")
	b.WriteString("3. Surprise Factor Scenarios: For the top 2 events, model what happens if Actual significantly beats or misses Forecast against the current COT positioning.\n")

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

	// Yield curve
	b.WriteString(fmt.Sprintf("2Y Treasury:       %.2f%%\n", data.Yield2Y))
	b.WriteString(fmt.Sprintf("10Y Treasury:      %.2f%%\n", data.Yield10Y))
	b.WriteString(fmt.Sprintf("Yield Spread:      %.2f%% (%s)\n", data.YieldSpread, regime.YieldCurve))

	// Inflation
	if data.CorePCE > 0 {
		b.WriteString(fmt.Sprintf("Core PCE:          %.2f%% (%s)\n", data.CorePCE, regime.Inflation))
	}
	if data.CPI > 0 {
		b.WriteString(fmt.Sprintf("CPI (headline):    %.2f%%\n", data.CPI))
	}
	if data.Breakeven5Y > 0 {
		b.WriteString(fmt.Sprintf("10Y Breakeven:     %.2f%%\n", data.Breakeven5Y))
	}

	// Monetary policy
	if data.FedFundsRate > 0 {
		b.WriteString(fmt.Sprintf("Fed Funds Rate:    %.2f%% (%s)\n", data.FedFundsRate, regime.MonPolicy))
	}

	// Financial stress
	b.WriteString(fmt.Sprintf("NFCI:              %.3f (%s)\n", data.NFCI, regime.FinStress))
	if data.TedSpread > 0 {
		b.WriteString(fmt.Sprintf("TED Spread:        %.0f bps\n", data.TedSpread))
	}

	// Labor
	if data.InitialClaims > 0 {
		b.WriteString(fmt.Sprintf("Initial Claims:    %.0fK/week\n", data.InitialClaims/1_000))
	}
	if data.UnemployRate > 0 {
		b.WriteString(fmt.Sprintf("Unemployment:      %.1f%%\n", data.UnemployRate))
	}

	// Growth
	if data.GDPGrowth != 0 {
		b.WriteString(fmt.Sprintf("Real GDP Growth:   %.1f%% QoQ ann. (%s)\n", data.GDPGrowth, regime.Growth))
	}

	// USD
	if data.DXY > 0 {
		b.WriteString(fmt.Sprintf("USD Index (DXY):   %.1f (%s)\n", data.DXY, regime.USDStrength))
	}

	b.WriteString("\n=== DERIVED REGIME ===\n")
	b.WriteString(fmt.Sprintf("Macro Regime:      %s\n", regime.Name))
	b.WriteString(fmt.Sprintf("Risk-Off Score:    %d/100\n", regime.Score))
	b.WriteString(fmt.Sprintf("Implied Bias:      %s\n", regime.Bias))

	b.WriteString("\n=== ANALYSIS REQUESTED ===\n")
	b.WriteString("Provide a structured FRED Macro Outlook covering:\n")
	b.WriteString("1. FED POLICY OUTLOOK: Given current FFR, inflation, and yield curve shape — what is the likely Fed trajectory? Rate cuts, holds, or hikes?\n")
	b.WriteString("2. USD STRUCTURAL BIAS: Based on real rates (FFR - breakeven), DXY level, and financial conditions — what is the medium-term dollar outlook?\n")
	b.WriteString("3. RISK APPETITE: Using NFCI, yield curve, and labor data together — assess current risk-on vs risk-off positioning pressure.\n")
	b.WriteString("4. GOLD & SAFE HAVENS: Given real yields and financial stress indicators — is gold/JPY/CHF structurally attractive?\n")
	b.WriteString("5. GROWTH TRAJECTORY: Based on GDP + labor + yield curve — is the economy heading toward expansion, slowdown, or recession?\n")
	b.WriteString("6. KEY INFLECTION POINTS: What specific data releases (e.g. next CPI, NFP, FOMC) could change this regime?\n")

	return b.String()
}

// BuildCombinedWithFREDPrompt creates a fused prompt that includes FRED macro context
// alongside COT positioning and economic calendar catalysts.
func BuildCombinedWithFREDPrompt(data ports.WeeklyData, regime fred.MacroRegime) string {
	var b strings.Builder
	b.WriteString("Generate a comprehensive market outlook fusing COT Speculator Positioning, Economic Calendar Catalysts, and FRED Macro Fundamentals.\n")

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
			b.WriteString(fmt.Sprintf("%s | %s - %s | Fcast: %s | Act: %s\n",
				e.Date, e.Currency, e.Event, e.Forecast, e.Actual))
		}
	}

	if data.MacroData != nil {
		b.WriteString("\n=== 3. FRED MACRO BACKDROP ===\n")
		m := data.MacroData
		b.WriteString(fmt.Sprintf("Macro Regime: %s (Risk-Off Score: %d/100)\n", regime.Name, regime.Score))
		b.WriteString(fmt.Sprintf("Yield Curve: %.2f%% spread (%s)\n", m.YieldSpread, regime.YieldCurve))
		if m.CorePCE > 0 {
			b.WriteString(fmt.Sprintf("Core PCE: %.2f%% | ", m.CorePCE))
		}
		if m.FedFundsRate > 0 {
			b.WriteString(fmt.Sprintf("FFR: %.2f%%\n", m.FedFundsRate))
		} else {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("NFCI: %.3f (%s)\n", m.NFCI, regime.FinStress))
		if m.InitialClaims > 0 {
			b.WriteString(fmt.Sprintf("Claims: %.0fK | ", m.InitialClaims/1_000))
		}
		if m.UnemployRate > 0 {
			b.WriteString(fmt.Sprintf("U-Rate: %.1f%%\n", m.UnemployRate))
		} else {
			b.WriteString("\n")
		}
		if m.DXY > 0 {
			b.WriteString(fmt.Sprintf("DXY: %.1f (%s)\n", m.DXY, regime.USDStrength))
		}
		if m.GDPGrowth != 0 {
			b.WriteString(fmt.Sprintf("GDP Growth: %.1f%% QoQ ann.\n", m.GDPGrowth))
		}
		b.WriteString(fmt.Sprintf("Implied Bias: %s\n", regime.Bias))
	}

	b.WriteString("\n=== ANALYSIS REQUESTED ===\n")
	b.WriteString("Provide a fused trading outlook:\n")
	b.WriteString("1. MACRO-COT ALIGNMENT: Where do FRED macro signals confirm or conflict with COT positioning? Identify high-conviction setups.\n")
	b.WriteString("2. CATALYST + POSITIONING RISK: For top upcoming events, overlay current COT crowding to identify fragile setups (crowded longs/shorts facing catalyst risk).\n")
	b.WriteString("3. REGIME-ADJUSTED TRADES: Given the macro regime, which COT-driven trade ideas have the strongest macro tailwind?\n")
	b.WriteString("4. RISK SCENARIOS: What would change the outlook? (e.g., FOMC surprise, inflation shock, weak NFP)\n")

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
	b.WriteString(fmt.Sprintf("Previous: %s\nForecast: %s\nActual: %s\n\n", event.Previous, event.Forecast, event.Actual))

	b.WriteString("Provide a 3-sentence maximum flash analysis covering:\n")
	b.WriteString("1. The deviation (did it beat or miss expectations?).\n")
	b.WriteString(fmt.Sprintf("2. Immediate directional bias for %s pairs (Bullish/Bearish).\n", event.Currency))
	b.WriteString("3. The likely macro narrative traders will adopt based on this number.\n")

	return b.String()
}
