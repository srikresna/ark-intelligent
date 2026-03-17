package telegram

import (
	"fmt"
	"sort"
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

// directionArrow checks if Actual beats Forecast.
func directionArrow(actual, forecast string) string {
	// Need numeric logic if you want to be precise, but since FF returns strings like "3.5%",
	// we'll just return a neutral dot if it's not easily parsed.
	if actual != "" && forecast != "" {
		if actual > forecast {
			return "🟢"
		} else if actual < forecast {
			return "🔴"
		}
	}
	return "⚪"
}

// ---------------------------------------------------------------------------
// Calendar Formatting
// ---------------------------------------------------------------------------

// FormatCalendarDay builds a message for a single day of events.
func (f *Formatter) FormatCalendarDay(dateStr string, events []domain.NewsEvent, filter string) string {
	var b strings.Builder

	sort.Slice(events, func(i, j int) bool {
		return events[i].TimeWIB.Before(events[j].TimeWIB)
	})

	// Format title
	b.WriteString(fmt.Sprintf("📅 <b>Economic Calendar</b>\n<i>Date: %s</i>\n\n", dateStr))

	if len(events) == 0 {
		b.WriteString("No events found for this filter.")
		return b.String()
	}

	for _, e := range events {
		// Apply filters before writing lines
		if !matchesFilter(e, filter) {
			continue
		}

		timeDisplay := e.Time
		if !e.TimeWIB.IsZero() {
			timeDisplay = e.TimeWIB.Format("15:04 WIB")
		}

		b.WriteString(fmt.Sprintf("%s <b>%s - %s</b>\n", e.FormatImpactColor(), timeDisplay, e.Currency))
		b.WriteString(fmt.Sprintf("↳ <i>%s</i>\n", e.Event))

		if e.Actual != "" {
			b.WriteString(fmt.Sprintf("   Actual: <b>%s</b> %s (Fcast: %s | Prev: %s)\n", e.Actual, directionArrow(e.Actual, e.Forecast), e.Forecast, e.Previous))
		} else {
			b.WriteString(fmt.Sprintf("   Fcast: %s | Prev: %s\n", e.Forecast, e.Previous))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// FormatCalendarWeek summarizes all events in a week based on the filter.
func (f *Formatter) FormatCalendarWeek(weekStart string, events []domain.NewsEvent, filter string) string {
	var b strings.Builder

	sort.Slice(events, func(i, j int) bool {
		return events[i].TimeWIB.Before(events[j].TimeWIB)
	})

	b.WriteString(fmt.Sprintf("📅 <b>Weekly Economic Calendar</b>\n<i>Week starting: %s</i>\n\n", weekStart))

	if len(events) == 0 {
		b.WriteString("No events found.")
		return b.String()
	}

	lastDate := ""
	for _, e := range events {
		// Apply filters
		if !matchesFilter(e, filter) {
			continue
		}

		// Print date header if it changed
		if e.Date != lastDate {
			b.WriteString(fmt.Sprintf("<b>--- %s ---</b>\n", e.Date))
			lastDate = e.Date
		}

		timeDisplay := e.Time
		if !e.TimeWIB.IsZero() {
			timeDisplay = e.TimeWIB.Format("15:04 WIB")
		}

		b.WriteString(fmt.Sprintf("%s %s %s: <i>%s</i>\n", e.FormatImpactColor(), timeDisplay, e.Currency, e.Event))
	}

	return b.String()
}

// FormatCalendarMonth formats all events for a whole month, grouped by day.
func (f *Formatter) FormatCalendarMonth(monthLabel string, events []domain.NewsEvent, filter string) string {
	var b strings.Builder

	sort.Slice(events, func(i, j int) bool {
		return events[i].TimeWIB.Before(events[j].TimeWIB)
	})

	b.WriteString(fmt.Sprintf("📅 <b>Monthly Economic Calendar</b>\n<i>%s</i>\n\n", monthLabel))

	if len(events) == 0 {
		b.WriteString("No events found.")
		return b.String()
	}

	lastDate := ""
	for _, e := range events {
		if !matchesFilter(e, filter) {
			continue
		}

		if e.Date != lastDate {
			b.WriteString(fmt.Sprintf("<b>--- %s ---</b>\n", e.Date))
			lastDate = e.Date
		}

		timeDisplay := e.Time
		if !e.TimeWIB.IsZero() {
			timeDisplay = e.TimeWIB.Format("15:04 WIB")
		}

		b.WriteString(fmt.Sprintf("%s %s %s: <i>%s</i>\n", e.FormatImpactColor(), timeDisplay, e.Currency, e.Event))
	}

	return b.String()
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

	if a.AssetMgrAlert {
		b.WriteString(fmt.Sprintf("⚠️ <b>WARNING: Asset Manager Structural Shift!</b> (Z-Score: %.2f)\n\n", a.AssetMgrZScore))
	}

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
	b.WriteString("\n<b>Scalper Intel:</b>\n")
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

		b.WriteString("<b>Asset Manager (Real Money):</b>\n")
		b.WriteString(fmt.Sprintf("<code>  Long:     %s</code>\n", fmtutil.FmtNum(r.AssetMgrLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short:    %s</code>\n\n", fmtutil.FmtNum(r.AssetMgrShort, 0)))

		b.WriteString("<b>Dealers (Commercials):</b>\n")
		b.WriteString(fmt.Sprintf("<code>  Long:     %s</code>\n", fmtutil.FmtNum(r.DealerLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short:    %s</code>\n", fmtutil.FmtNum(r.DealerShort, 0)))
	}

	b.WriteString("\n<i>Data sourced directly from CFTC</i>")
	return b.String()
}

// ---------------------------------------------------------------------------
// Weekly Outlook Formatting
// ---------------------------------------------------------------------------

// FormatWeeklyOutlook formats the AI-generated weekly market outlook.
func (f *Formatter) FormatWeeklyOutlook(outlook string, date time.Time) string {
	var b strings.Builder

	b.WriteString("<b>Weekly Market Outlook</b>\n")
	b.WriteString(fmt.Sprintf("<i>Week of %s</i>\n\n", date.Format("Jan 2, 2006")))
	b.WriteString(outlook)

	return b.String()
}

// FormatAIInsight wraps an AI narrative with a labeled section.
func (f *Formatter) FormatAIInsight(label, narrative string) string {
	return fmt.Sprintf("<b>%s Analysis:</b>\n<i>%s</i>", label, narrative)
}

// ---------------------------------------------------------------------------
// Settings Formatting
// ---------------------------------------------------------------------------

// FormatSettings formats the user preferences display.
func (f *Formatter) FormatSettings(prefs domain.UserPrefs) string {
	var b strings.Builder

	aiReports := "OFF"
	if prefs.AIReportsEnabled {
		aiReports = "ON"
	}

	cotAlerts := "OFF"
	if prefs.COTAlertsEnabled {
		cotAlerts = "ON"
	}

	b.WriteString("🦅 <b>ARK Intelligence Settings</b>\n\n")
	b.WriteString(fmt.Sprintf("<code>[COT] Release Alerts: %s</code>\n", cotAlerts))
	b.WriteString(fmt.Sprintf("<code>[AI] Weekly Reports : %s</code>\n", aiReports))

	langDisplay := "Indonesian 🇮🇩"
	if prefs.Language == "en" {
		langDisplay = "English 🇬🇧"
	}
	b.WriteString(fmt.Sprintf("<code>[AI] Output Language: %s</code>\n", langDisplay))

	if len(prefs.CurrencyFilter) > 0 {
		b.WriteString(fmt.Sprintf("<code>Currencies filter  : %s</code>\n", strings.Join(prefs.CurrencyFilter, ", ")))
	} else {
		b.WriteString("<code>Currencies filter  : All</code>\n")
	}

	b.WriteString("\n<i>Use the buttons below to adjust preferences</i>")

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

// matchesFilter checks if a NewsEvent passes the given filter string.
// filter values:
//   - "all"     → no filtering, show everything
//   - "high"    → only high impact
//   - "med"     → high + medium impact
//   - "cur:USD" → only events for the specified currency (e.g. "cur:USD", "cur:GBP")
func matchesFilter(e domain.NewsEvent, filter string) bool {
	switch {
	case filter == "" || filter == "all":
		return true
	case filter == "high":
		return e.Impact == "high"
	case filter == "med":
		return e.Impact == "high" || e.Impact == "medium"
	case strings.HasPrefix(filter, "cur:"):
		currency := strings.ToUpper(strings.TrimPrefix(filter, "cur:"))
		return strings.ToUpper(e.Currency) == currency
	default:
		return true
	}
}
