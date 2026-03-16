package ai

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
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
