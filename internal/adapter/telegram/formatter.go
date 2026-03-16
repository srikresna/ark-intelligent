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

	rt := a.Contract.ReportType
	smartMoneyLabel := "Speculator"
	hedgerLabel := "Hedger"
	if rt == "TFF" {
		smartMoneyLabel = "Lev Funds"
		hedgerLabel = "Dealers"
	} else if rt == "DISAGGREGATED" {
		smartMoneyLabel = "Managed Money"
		hedgerLabel = "Prod/Swap"
	}

	b.WriteString(fmt.Sprintf("<b>COT Analysis: %s</b>\n", a.Contract.Name))
	b.WriteString(fmt.Sprintf("<i>Report: %s (%s)</i>\n\n", a.ReportDate.Format("Jan 2, 2006"), rt))

	// Positioning
	b.WriteString(fmt.Sprintf("<b>%s (Smart Money):</b>\n", smartMoneyLabel))
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", fmtutil.FmtNumSigned(a.NetPosition, 0)))
	b.WriteString(fmt.Sprintf("<code>  Net Change:     %s</code>\n", fmtutil.FmtNumSigned(a.NetChange, 0)))
	b.WriteString(fmt.Sprintf("<code>  L/S Ratio:      %.2f</code>\n", a.LongShortRatio))

	b.WriteString(fmt.Sprintf("\n<b>%s:</b>\n", hedgerLabel))
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", fmtutil.FmtNumSigned(a.CommercialNet, 0)))

	// COT Index
	b.WriteString(fmt.Sprintf("\n<b>COT Index (%s):</b>\n", smartMoneyLabel))
	b.WriteString(fmt.Sprintf("<code>  52-Week:        %.1f%%</code>\n", a.COTIndex))
	b.WriteString(f.formatProgressBar(a.COTIndex, 20))

	// Scalper / Intraday Intel
	b.WriteString(fmt.Sprintf("\n<b>Scalper Intel:</b>\n"))
	b.WriteString(fmt.Sprintf("<code>  4W Momentum:    %s</code>\n", fmtutil.FmtNumSigned(a.SpecMomentum4W, 0)))
	b.WriteString(fmt.Sprintf("<code>  OI Change WoW:  %s (%s)</code>\n", fmtutil.FmtNumSigned(a.OpenInterestChg, 0), a.OITrend))
	b.WriteString(fmt.Sprintf("<code>  ST Bias:        %s</code>\n", a.ShortTermBias))

	return b.String()
}

// FormatCOTRaw formats raw uncalculated CFTC data for a contract.
func (f *Formatter) FormatCOTRaw(r domain.COTRecord) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("<b>Raw COT Data: %s</b>\n", r.ContractName))
	b.WriteString(fmt.Sprintf("<i>Report: %s</i>\n\n", r.ReportDate.Format("Jan 2, 2006")))

	b.WriteString("<b>Open Interest:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  Total:    %s</code>\n\n", fmtutil.FmtNum(r.OpenInterest, 0)))

	if r.ContractName == "Gold" || r.ContractName == "Crude Oil WTI" {
		// Disaggregated Format
		b.WriteString("<b>Managed Money (Specs):</b>\n")
		b.WriteString(fmt.Sprintf("<code>  Long:     %s</code>\n", fmtutil.FmtNum(r.ManagedMoneyLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short:    %s</code>\n\n", fmtutil.FmtNum(r.ManagedMoneyShort, 0)))

		b.WriteString("<b>Prod/Swap (Commercials):</b>\n")
		b.WriteString(fmt.Sprintf("<code>  Long:     %s</code>\n", fmtutil.FmtNum(r.ProdMercLong+r.SwapDealerLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short:    %s</code>\n", fmtutil.FmtNum(r.ProdMercShort+r.SwapDealerShort, 0)))
	} else {
		// TFF Format
		b.WriteString("<b>Lev Funds (Specs):</b>\n")
		b.WriteString(fmt.Sprintf("<code>  Long:     %s</code>\n", fmtutil.FmtNum(r.LevFundLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short:    %s</code>\n\n", fmtutil.FmtNum(r.LevFundShort, 0)))

		b.WriteString("<b>Dealers (Commercials):</b>\n")
		b.WriteString(fmt.Sprintf("<code>  Long:     %s</code>\n", fmtutil.FmtNum(r.DealerLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short:    %s</code>\n", fmtutil.FmtNum(r.DealerShort, 0)))
	}

	b.WriteString(fmt.Sprintf("\n<i>Data sourced directly from CFTC</i>"))
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

	newsAlerts := "OFF"
	if prefs.AlertsEnabled {
		newsAlerts = "ON"
	}

	aiReports := "OFF"
	if prefs.AIReportsEnabled {
		aiReports = "ON"
	}

	cotAlerts := "OFF"
	if prefs.COTAlertsEnabled {
		cotAlerts = "ON"
	}

	b.WriteString("<b>Settings</b>\n\n")
	b.WriteString(fmt.Sprintf("<code>[News] Notifications: %s</code>\n", newsAlerts))
	b.WriteString(fmt.Sprintf("<code>[COT] Release Alerts: %s</code>\n", cotAlerts))
	b.WriteString(fmt.Sprintf("<code>[AI] Weekly Reports: %s</code>\n", aiReports))
	b.WriteString(fmt.Sprintf("<code>News Impacts:        %s</code>\n", strings.Join(prefs.AlertImpacts, ", ")))
	b.WriteString(fmt.Sprintf("<code>News Timing:         %s min</code>\n",
		f.formatIntSlice(prefs.AlertMinutes)))

	if len(prefs.CurrencyFilter) > 0 {
		b.WriteString(fmt.Sprintf("<code>Currencies filter:    %s</code>\n", strings.Join(prefs.CurrencyFilter, ", ")))
	} else {
		b.WriteString("<code>Currencies filter:    All</code>\n")
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
