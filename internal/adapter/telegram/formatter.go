package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/pkg/fmtutil"
)

// ---------------------------------------------------------------------------
// Formatter — Telegram HTML message builder
// ---------------------------------------------------------------------------

// Formatter builds HTML-formatted messages for Telegram.
// All output uses Telegram's supported HTML subset:
// <b>, <i>, <code>, <pre>, <a>, <s>, <u>, <tg-spoiler>
type Formatter struct{}

// NewFormatter creates a new Formatter.
func NewFormatter() *Formatter {
	return &Formatter{}
}

// Impact emoji mapping.
func impactIcon(impact domain.ImpactLevel) string {
	switch impact {
	case domain.ImpactHigh:
		return "🔴"
	case domain.ImpactMedium:
		return "🟠"
	case domain.ImpactLow:
		return "🟡"
	default:
		return "⚪️"
	}
}

// Direction arrow for numeric values.
func directionArrow(actual, forecast float64) string {
	if actual > forecast {
		return "🟢"
	} else if actual < forecast {
		return "🔴"
	}
	return "⚪️"
}



// ---------------------------------------------------------------------------
// COT Formatting
// ---------------------------------------------------------------------------

// FormatCOTOverview formats a summary of all COT analyses.
func (f *Formatter) FormatCOTOverview(analyses []domain.COTAnalysis) string {
	var b strings.Builder

	b.WriteString("<b>COT Positioning Overview</b>\n")
	if len(analyses) > 0 {
		b.WriteString(fmt.Sprintf("<i>Report: %s</i>\n\n",
			analyses[0].ReportDate.Format("Jan 2, 2006")))
	}

	for _, a := range analyses {
		bias := "NEUTRAL"
		if a.NetPosition > 0 {
			bias = "LONG"
		} else if a.NetPosition < 0 {
			bias = "SHORT"
		}

		// COT Index classification
		idxLabel := "Neutral"
		if a.COTIndex >= 80 {
			idxLabel = "Extreme Long"
		} else if a.COTIndex <= 20 {
			idxLabel = "Extreme Short"
		} else if a.COTIndex >= 60 {
			idxLabel = "Bullish"
		} else if a.COTIndex <= 40 {
			idxLabel = "Bearish"
		}

		b.WriteString(fmt.Sprintf("<b>%s</b> %s\n", a.Contract.Name, bias))
		b.WriteString(fmt.Sprintf("<code>  Net: %s | Idx: %.0f%% (%s)</code>\n",
			fmtutil.FmtNum(a.NetPosition, 0), a.COTIndex, idxLabel))
		b.WriteString(fmt.Sprintf("<code>  Chg: %s | Mom: %s</code>\n\n",
			fmtutil.FmtNumSigned(a.NetChange, 0),
			f.momentumLabel(a.MomentumDir)))
	}

	b.WriteString("<i>Tap a currency for detailed breakdown</i>")
	return b.String()
}

// FormatCOTDetail formats detailed COT analysis for one contract.
func (f *Formatter) FormatCOTDetail(a domain.COTAnalysis) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("<b>COT Analysis: %s</b>\n", a.Contract.Name))
	b.WriteString(fmt.Sprintf("<i>Report: %s</i>\n\n", a.ReportDate.Format("Jan 2, 2006")))

	// Positioning
	b.WriteString("<b>Positioning:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", fmtutil.FmtNum(a.NetPosition, 0)))
	b.WriteString(fmt.Sprintf("<code>  Net Change:     %s</code>\n", fmtutil.FmtNumSigned(a.NetChange, 0)))
	b.WriteString(fmt.Sprintf("<code>  L/S Ratio:      %.2f</code>\n", a.LongShortRatio))

	// COT Index
	b.WriteString(fmt.Sprintf("\n<b>COT Index:</b>\n"))
	b.WriteString(fmt.Sprintf("<code>  52-Week:        %.1f%%</code>\n", a.COTIndex))
	b.WriteString(f.formatProgressBar(a.COTIndex, 20))

	// Momentum
	b.WriteString(fmt.Sprintf("\n<b>Momentum:</b>\n"))
	b.WriteString(fmt.Sprintf("<code>  1-Week:         %s</code>\n", fmtutil.FmtNumSigned(a.NetChange, 0)))
	b.WriteString(fmt.Sprintf("<code>  Trend:          %s</code>\n", f.momentumLabel(a.MomentumDir)))

	// Sentiment
	b.WriteString(fmt.Sprintf("\n<b>Sentiment Score:</b>\n"))
	b.WriteString(fmt.Sprintf("<code>  Overall:        %.2f</code>\n", a.SentimentScore))
	b.WriteString(fmt.Sprintf("<code>  Crowding:       %.2f</code>\n", a.CrowdingIndex))

	return b.String()
}

// ---------------------------------------------------------------------------
// Confluence Formatting
// ---------------------------------------------------------------------------

// FormatConfluenceOverview formats all confluence scores.
func (f *Formatter) FormatConfluenceOverview(scores []domain.ConfluenceScore) string {
	var b strings.Builder

	b.WriteString("<b>Confluence Scores</b>\n")
	b.WriteString("<i>Multi-factor bias analysis</i>\n\n")

	for _, s := range scores {
		biasLabel := string(s.Bias)

		agreementLabel := "Low"
		if s.AgreementPct >= 0.7 {
			agreementLabel = "High"
		} else if s.AgreementPct >= 0.4 {
			agreementLabel = "Medium"
		}

		b.WriteString(fmt.Sprintf("<b>%s</b> %s (Agr: %s)\n",
			s.CurrencyPair, biasLabel, agreementLabel))
		b.WriteString(fmt.Sprintf("<code>  Score: %.1f/100 | Aligned: %d/6</code>\n\n",
			s.TotalScore, s.FactorsAligned))
	}

	b.WriteString("<i>Tap a pair for factor breakdown</i>")
	return b.String()
}

// FormatConfluenceDetail formats detailed confluence for one pair.
func (f *Formatter) FormatConfluenceDetail(s domain.ConfluenceScore) string {
	var b strings.Builder

	biasLabel := string(s.Bias)

	b.WriteString(fmt.Sprintf("<b>Confluence: %s</b> %s\n", s.CurrencyPair, biasLabel))
	b.WriteString(fmt.Sprintf("<i>Updated: %s WIB</i>\n\n",
		s.Timestamp.Format("Jan 2, 15:04")))

	b.WriteString(fmt.Sprintf("<code>Total Score:    %.1f/100</code>\n", s.TotalScore))
	b.WriteString(fmt.Sprintf("<code>Agreement:      %.0f%%</code>\n\n", s.AgreementPct*100))

	b.WriteString("<b>Factor Breakdown:</b>\n")
	for _, factor := range s.Factors {
		alignIcon := "+"
		if factor.RawScore < 45 {
			alignIcon = "-"
		} else if factor.RawScore >= 45 && factor.RawScore <= 55 {
			alignIcon = "="
		}
		b.WriteString(fmt.Sprintf("<code>  [%s] %-14s %.1f (w:%.2f)</code>\n",
			alignIcon, factor.Name, factor.RawScore, factor.Weight))
	}

	if s.AINarrative != "" {
		b.WriteString(fmt.Sprintf("\n<i>%s</i>", s.AINarrative))
	}

	return b.String()
}


// ---------------------------------------------------------------------------
// Currency Ranking Formatting
// ---------------------------------------------------------------------------

// FormatCurrencyRanking formats the multi-dimensional currency strength ranking.
func (f *Formatter) FormatCurrencyRanking(ranking domain.CurrencyRanking) string {
	var b strings.Builder

	b.WriteString("<b>Currency Strength Ranking</b>\n")
	b.WriteString(fmt.Sprintf("<i>Updated: %s WIB</i>\n\n",
		ranking.Timestamp.Format("Jan 2, 15:04")))

	// Table header
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-4s %-6s %-6s %-5s\n",
		"Rank", "CCY", "Score", "Bias"))
	b.WriteString(strings.Repeat("-", 25) + "\n")

	for i, entry := range ranking.Rankings {
		biasLabel := "⚪️"
		if entry.Score.CompositeScore > 0.3 {
			biasLabel = "🟢"
		} else if entry.Score.CompositeScore < -0.3 {
			biasLabel = "🔴"
		}

		b.WriteString(fmt.Sprintf("%-4d %-6s %+.2f  %-5s\n",
			i+1, string(entry.Score.Code), entry.Score.CompositeScore, biasLabel))
	}

	b.WriteString("</pre>")

	// Best pairs suggestion
	if len(ranking.Rankings) >= 2 {
		strongest := ranking.Rankings[0]
		weakest := ranking.Rankings[len(ranking.Rankings)-1]
		b.WriteString(fmt.Sprintf("\n<b>Top Pair:</b> Long <b>%s</b> / Short <b>%s</b>",
			string(strongest.Score.Code), string(weakest.Score.Code)))
	}

	return b.String()
}


// ---------------------------------------------------------------------------
// Weekly Outlook Formatting
// ---------------------------------------------------------------------------

// FormatWeeklyOutlook formats the AI-generated weekly market outlook.
func (f *Formatter) FormatWeeklyOutlook(outlook string, date time.Time) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("<b>Weekly Market Outlook</b>\n"))
	b.WriteString(fmt.Sprintf("<i>Week of %s | AI-Generated</i>\n\n",
		date.Format("Jan 2, 2006")))
	b.WriteString(outlook)
	b.WriteString("\n\n<i>This analysis is AI-generated. Always validate with your own research.</i>")

	return b.String()
}

// FormatAIInsight wraps an AI narrative with a labeled section.
func (f *Formatter) FormatAIInsight(label, narrative string) string {
	return fmt.Sprintf("<b>[AI] %s:</b>\n<i>%s</i>", label, narrative)
}

// ---------------------------------------------------------------------------
// Settings Formatting
// ---------------------------------------------------------------------------

// FormatSettings formats the user preferences display.
func (f *Formatter) FormatSettings(prefs domain.UserPrefs) string {
	var b strings.Builder

	alertsLabel := "OFF"
	if prefs.AlertsEnabled {
		alertsLabel = "ON"
	}

	aiLabel := "OFF"
	if prefs.AIReportsEnabled {
		aiLabel = "ON"
	}

	b.WriteString("<b>Settings</b>\n\n")
	b.WriteString(fmt.Sprintf("<code>Alerts:     %s</code>\n", alertsLabel))
	b.WriteString(fmt.Sprintf("<code>AI Reports: %s</code>\n", aiLabel))
	b.WriteString(fmt.Sprintf("<code>Impacts:    %s</code>\n", strings.Join(prefs.AlertImpacts, ", ")))
	b.WriteString(fmt.Sprintf("<code>Timing:     %s min before</code>\n",
		f.formatIntSlice(prefs.AlertMinutes)))

	if len(prefs.CurrencyFilter) > 0 {
		b.WriteString(fmt.Sprintf("<code>Currencies: %s</code>\n", strings.Join(prefs.CurrencyFilter, ", ")))
	} else {
		b.WriteString("<code>Currencies: All</code>\n")
	}

	b.WriteString("\n<i>Use the buttons below to adjust settings</i>")

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// formatProgressBar creates a text-based progress bar for COT Index.
func (f *Formatter) formatProgressBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("#", filled) + strings.Repeat(".", width-filled)

	// Mark extreme zones
	label := ""
	if pct >= 80 {
		label = " EXTREME LONG"
	} else if pct <= 20 {
		label = " EXTREME SHORT"
	}

	return fmt.Sprintf("<code>  [%s] %.0f%%%s</code>\n", bar, pct, label)
}

// momentumLabel converts MomentumDirection to readable label.
func (f *Formatter) momentumLabel(m domain.MomentumDirection) string {
	switch m {
	case "STRONG_UP":
		return "Strong Bullish"
	case "UP":
		return "Bullish"
	case "FLAT":
		return "Neutral"
	case "DOWN":
		return "Bearish"
	case "STRONG_DOWN":
		return "Strong Bearish"
	default:
		return string(m)
	}
}

// trendLabel converts trend float to readable label.
func (f *Formatter) trendLabel(t float64) string {
	switch {
	case t > 0.3:
		return "Improving"
	case t > 0.1:
		return "Slightly Better"
	case t > -0.1:
		return "Flat"
	case t > -0.3:
		return "Slightly Worse"
	default:
		return "Deteriorating"
	}
}

// formatIntSlice joins ints as comma-separated string.
func (f *Formatter) formatIntSlice(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, ", ")
}
