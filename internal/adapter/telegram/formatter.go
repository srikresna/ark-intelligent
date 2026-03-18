package telegram

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/service/cot"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
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

// parseNumeric strips common suffixes and parses a numeric value from a string.
func parseNumeric(s string) *float64 {
	s = strings.TrimSpace(s)
	// Remove trailing %, K, M, B, and common suffixes
	s = strings.TrimRight(s, "%KMBkmb")
	s = strings.ReplaceAll(s, ",", "")
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return &f
	}
	return nil
}

// directionArrow checks if Actual beats Forecast using numeric comparison.
func directionArrow(actual, forecast string) string {
	if actual == "" || forecast == "" {
		return "⚪"
	}
	aVal := parseNumeric(actual)
	fVal := parseNumeric(forecast)
	if aVal == nil || fVal == nil {
		return "⚪"
	}
	if *aVal > *fVal {
		return "🟢"
	} else if *aVal < *fVal {
		return "🔴"
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
		b.WriteString("\n\n<i>Tip: </i><code>/calendar</code> | <code>/calendar week</code>")
		return b.String()
	}

	hasEvents := false
	for _, e := range events {
		// Apply filters before writing lines
		if !matchesFilter(e, filter) {
			continue
		}
		hasEvents = true

		timeDisplay := e.Time
		if !e.TimeWIB.IsZero() {
			timeDisplay = e.TimeWIB.Format("15:04 WIB")
		}

		b.WriteString(fmt.Sprintf("%s <b>%s - %s</b>\n", e.FormatImpactColor(), timeDisplay, e.Currency))
		b.WriteString(fmt.Sprintf("↳ <i>%s</i>\n", e.Event))

		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast)
			line := fmt.Sprintf("   ✅ Actual: <b>%s</b> %s (Fcast: %s | Prev: %s)", e.Actual, arrow, e.Forecast, e.Previous)
			if e.SurpriseLabel != "" {
				line += fmt.Sprintf(" — <i>%s</i>", e.SurpriseLabel)
			}
			b.WriteString(line + "\n")
			if e.OldPrevious != "" && e.OldPrevious != e.Previous {
				b.WriteString(fmt.Sprintf("   ↻ <i>Revised from %s to %s</i>\n", e.OldPrevious, e.Previous))
			}
		} else {
			line := fmt.Sprintf("   Fcast: %s | Prev: %s", e.Forecast, e.Previous)
			if e.OldPrevious != "" && e.OldPrevious != e.Previous {
				line += fmt.Sprintf(" (↻ rev from %s)", e.OldPrevious)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	if !hasEvents {
		b.WriteString("No events match the current filter.")
	}

	b.WriteString("\n<i>Tip: </i><code>/calendar</code> | <code>/calendar week</code>")
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
		b.WriteString("\n\n<i>Tip: </i><code>/calendar</code> | <code>/calendar week</code>")
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

		line := fmt.Sprintf("%s %s %s: <i>%s</i>", e.FormatImpactColor(), timeDisplay, e.Currency, e.Event)
		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast)
			line += fmt.Sprintf(" — ✅<b>%s</b>%s", e.Actual, arrow)
			if e.SurpriseLabel != "" {
				line += fmt.Sprintf(" <i>%s</i>", e.SurpriseLabel)
			}
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n<i>Tip: </i><code>/calendar</code> | <code>/calendar week</code>")
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

		line := fmt.Sprintf("%s %s %s: <i>%s</i>", e.FormatImpactColor(), timeDisplay, e.Currency, e.Event)
		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast)
			line += fmt.Sprintf(" — ✅<b>%s</b>%s", e.Actual, arrow)
		}
		b.WriteString(line + "\n")
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

	b.WriteString("<i>Tap a currency for detailed breakdown</i>\n")
	b.WriteString("<i>Tip: </i><code>/cot USD</code> | <code>/cot raw EUR</code> | <code>/cot GBP</code>")
	return b.String()
}

// FormatCOTDetail formats detailed COT analysis for one contract.
// Signature unchanged for backward compatibility.
func (f *Formatter) FormatCOTDetail(a domain.COTAnalysis) string {
	return f.FormatCOTDetailWithCode(a, "")
}

// FormatCOTDetailWithCode formats detailed COT analysis and appends quick-copy commands.
func (f *Formatter) FormatCOTDetailWithCode(a domain.COTAnalysis, displayCode string) string {
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

	// Alerts section — all warnings first
	if a.AssetMgrAlert {
		b.WriteString(fmt.Sprintf("⚠️ <b>Asset Manager Structural Shift!</b> (Z-Score: %.2f)\n", a.AssetMgrZScore))
	}
	if a.ThinMarketAlert {
		b.WriteString(fmt.Sprintf("🚨 <b>THIN MARKET:</b> %s\n", a.ThinMarketDesc))
	}
	if a.SmartDumbDivergence {
		b.WriteString("🔀 <b>Divergence:</b> Smart money vs commercials moving opposite\n")
	}
	if a.CommExtremeBull {
		b.WriteString("🟢 <b>Commercial COT Extreme LONG</b> (contrarian bullish signal)\n")
	}
	if a.CommExtremeBear {
		b.WriteString("🔴 <b>Commercial COT Extreme SHORT</b> (contrarian bearish signal)\n")
	}
	if a.AssetMgrAlert || a.ThinMarketAlert || a.SmartDumbDivergence || a.CommExtremeBull || a.CommExtremeBear {
		b.WriteString("\n")
	}

	// Positioning
	b.WriteString(fmt.Sprintf("<b>%s (Smart Money):</b>\n", smartMoneyLabel))
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", fmtutil.FmtNumSigned(a.NetPosition, 0)))
	b.WriteString(fmt.Sprintf("<code>  Net Change:     %s</code>\n", fmtutil.FmtNumSigned(a.NetChange, 0)))
	b.WriteString(fmt.Sprintf("<code>  L/S Ratio:      %.2f</code>\n", a.LongShortRatio))
	b.WriteString(fmt.Sprintf("<code>  Net as %% OI:    %.1f%%</code>\n", a.PctOfOI))

	b.WriteString(fmt.Sprintf("\n<b>%s:</b>\n", hedgerLabel))
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", fmtutil.FmtNumSigned(a.CommercialNet, 0)))
	b.WriteString(fmt.Sprintf("<code>  Comm %% OI:      %.1f%%</code>\n", a.CommPctOfOI))

	// COT Index
	b.WriteString(fmt.Sprintf("\n<b>COT Index (%s):</b>\n", smartMoneyLabel))
	b.WriteString(fmt.Sprintf("<code>  52-Week:        %.1f%%</code>\n", a.COTIndex))
	b.WriteString(f.formatProgressBar(a.COTIndex, 20))

	// Momentum (4W + 8W)
	b.WriteString("\n<b>Momentum:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  4W:             %s</code>\n", fmtutil.FmtNumSigned(a.SpecMomentum4W, 0)))
	if a.SpecMomentum8W != 0 {
		trendFilter := "✅ aligned"
		if (a.SpecMomentum4W > 0) != (a.SpecMomentum8W > 0) {
			trendFilter = "⚠️ opposing"
		}
		b.WriteString(fmt.Sprintf("<code>  8W:             %s (%s)</code>\n", fmtutil.FmtNumSigned(a.SpecMomentum8W, 0), trendFilter))
	}
	if a.ConsecutiveWeeks > 0 {
		b.WriteString(fmt.Sprintf("<code>  Streak:         %d weeks same dir</code>\n", a.ConsecutiveWeeks))
	}

	// Open Interest
	b.WriteString("\n<b>Open Interest:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  OI Change:      %s (%s)</code>\n", fmtutil.FmtNumSigned(a.OpenInterestChg, 0), a.OITrend))
	if a.SpreadPctOfOI > 0 {
		b.WriteString(fmt.Sprintf("<code>  Spread Pos:     %.1f%% of OI</code>\n", a.SpreadPctOfOI))
	}

	// Trader concentration
	if a.TotalTraders > 0 {
		b.WriteString(fmt.Sprintf("\n<b>Trader Depth (%s):</b>\n", a.TraderConcentration))
		if rt == "TFF" {
			if a.LevFundLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Lev Fund Long:  %d traders</code>\n", a.LevFundLongTraders))
			}
			if a.LevFundShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Lev Fund Short: %d traders</code>\n", a.LevFundShortTraders))
			}
			if a.DealerShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Dealer Short:   %d traders</code>\n", a.DealerShortTraders))
			}
			if a.AssetMgrLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  AssetMgr Long:  %d traders</code>\n", a.AssetMgrLongTraders))
			}
		} else {
			if a.MMoneyLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  MM Long:        %d traders</code>\n", a.MMoneyLongTraders))
			}
			if a.MMoneyShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  MM Short:       %d traders</code>\n", a.MMoneyShortTraders))
			}
		}
		b.WriteString(fmt.Sprintf("<code>  Total:          %d traders</code>\n", a.TotalTraders))
	}

	// Scalper Intel
	b.WriteString("\n<b>Scalper Intel:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  ST Bias:        %s</code>\n", a.ShortTermBias))
	b.WriteString(fmt.Sprintf("<code>  Crowding:       %.0f/100</code>\n", a.CrowdingIndex))
	b.WriteString(fmt.Sprintf("<code>  Divergence:     %v</code>\n", a.DivergenceFlag))

	// Quick copy commands
	if displayCode != "" {
		b.WriteString(fmt.Sprintf("\n<i>Quick commands:</i>\n<code>/cot %s</code> | <code>/cot raw %s</code>", displayCode, displayCode))
	}

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

	// Trader counts
	if r.TotalTraders > 0 || r.TotalTradersDisag > 0 {
		b.WriteString("\n<b>Trader Depth:</b>\n")
		if r.ContractName == "Gold" || r.ContractName == "Crude Oil WTI" {
			if r.MMoneyLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  MM Long:  %d traders</code>\n", r.MMoneyLongTraders))
			}
			if r.MMoneyShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  MM Short: %d traders</code>\n", r.MMoneyShortTraders))
			}
			b.WriteString(fmt.Sprintf("<code>  Total:    %d traders</code>\n", r.TotalTradersDisag))
		} else {
			if r.DealerLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Dlr Long: %d traders</code>\n", r.DealerLongTraders))
			}
			if r.DealerShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Dlr Short:%d traders</code>\n", r.DealerShortTraders))
			}
			if r.LevFundLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  LF Long:  %d traders</code>\n", r.LevFundLongTraders))
			}
			if r.LevFundShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  LF Short: %d traders</code>\n", r.LevFundShortTraders))
			}
			b.WriteString(fmt.Sprintf("<code>  Total:    %d traders</code>\n", r.TotalTraders))
		}
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
	b.WriteString("\n\n<i>Tip: </i><code>/outlook cot</code> | <code>/outlook news</code> | <code>/outlook combine</code>")

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

	// Alert minutes display
	if len(prefs.AlertMinutes) > 0 {
		parts := make([]string, len(prefs.AlertMinutes))
		for i, m := range prefs.AlertMinutes {
			parts[i] = fmt.Sprintf("%d", m)
		}
		b.WriteString(fmt.Sprintf("<code>Alert Minutes      : %s</code>\n", strings.Join(parts, "/")))
	} else {
		b.WriteString("<code>Alert Minutes      : -</code>\n")
	}

	// Currency filter display
	if len(prefs.CurrencyFilter) > 0 {
		b.WriteString(fmt.Sprintf("<code>Alert Currencies   : %s</code>\n", strings.Join(prefs.CurrencyFilter, ", ")))
	} else {
		b.WriteString("<code>Alert Currencies   : All Currencies</code>\n")
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

// ---------------------------------------------------------------------------
// P1.3 — Currency Strength Ranking
// ---------------------------------------------------------------------------

// rankEntry holds a currency ranking entry for FormatRanking.
type rankEntry struct {
	Currency string
	Score    float64
	COTIndex float64
}

// FormatRanking formats the weekly currency strength ranking based on COT sentiment scores.
// P1.3 — /rank command output.
func (f *Formatter) FormatRanking(analyses []domain.COTAnalysis, date time.Time) string {
	var b strings.Builder

	// Filter to 8 major currencies only (no commodities)
	majors := map[string]bool{"EUR": true, "GBP": true, "JPY": true, "AUD": true,
		"NZD": true, "CAD": true, "CHF": true, "USD": true}

	var entries []rankEntry
	for _, a := range analyses {
		if !majors[a.Contract.Currency] {
			continue
		}
		entries = append(entries, rankEntry{
			Currency: a.Contract.Currency,
			Score:    a.SentimentScore,
			COTIndex: a.COTIndex,
		})
	}

	// Sort by sentiment score descending (strongest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})

	b.WriteString("🏆 <b>Currency Strength Ranking</b>\n")
	b.WriteString(fmt.Sprintf("<i>Week of %s | Based on COT Positioning</i>\n\n", date.Format("02 Jan 2006")))

	medals := []string{"🥇", "🥈", "🥉", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣"}

	for i, e := range entries {
		medal := ""
		if i < len(medals) {
			medal = medals[i]
		}

		arrow := scoreArrow(e.Score)
		colorDot := scoreDot(e.Score)
		label := cotLabel(e.COTIndex)

		signStr := "+"
		if e.Score < 0 {
			signStr = ""
		}

		b.WriteString(fmt.Sprintf("%s %s %s: <b>%s%.0f %s</b>  <i>(%s)</i>\n",
			medal, colorDot, e.Currency, signStr, e.Score, arrow, label))
	}

	// Best pairs: top 3 spread combinations
	b.WriteString("\n📊 <b>Best Pairs:</b>\n")
	pairs := buildBestPairs(entries)
	for _, p := range pairs {
		b.WriteString(p + "\n")
	}

	b.WriteString("\n<i>Tip: </i><code>/cot GBP</code> untuk detail lengkap")
	return b.String()
}

// convictionRankEntry holds a ranking entry with conviction score for FormatRankingWithConviction.
type convictionRankEntry struct {
	Currency   string
	Score      float64
	COTIndex   float64
	Conviction cot.ConvictionScore
}

// FormatRankingWithConviction formats the weekly currency strength ranking with unified
// conviction scores from COT + FRED regime + calendar data.
// Gap D — exposes ConvictionScore per currency in /rank output.
// Falls back gracefully to plain ranking if convictions is empty.
func (f *Formatter) FormatRankingWithConviction(
	analyses []domain.COTAnalysis,
	convictions []cot.ConvictionScore,
	regime *fred.MacroRegime,
	date time.Time,
) string {
	// If no conviction data, fall back to the plain ranking
	if len(convictions) == 0 {
		return f.FormatRanking(analyses, date)
	}

	// Build a map from currency → conviction score
	convMap := make(map[string]cot.ConvictionScore, len(convictions))
	for _, cs := range convictions {
		convMap[cs.Currency] = cs
	}

	// Filter to 8 major currencies only
	majors := map[string]bool{"EUR": true, "GBP": true, "JPY": true, "AUD": true,
		"NZD": true, "CAD": true, "CHF": true, "USD": true}

	var entries []convictionRankEntry
	for _, a := range analyses {
		if !majors[a.Contract.Currency] {
			continue
		}
		cs := convMap[a.Contract.Currency]
		entries = append(entries, convictionRankEntry{
			Currency:   a.Contract.Currency,
			Score:      a.SentimentScore,
			COTIndex:   a.COTIndex,
			Conviction: cs,
		})
	}

	// Sort by conviction score descending (highest conviction first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Conviction.Score > entries[j].Conviction.Score
	})

	var b strings.Builder
	b.WriteString("🏆 <b>CURRENCY STRENGTH RANKING</b>\n")
	b.WriteString(fmt.Sprintf("<i>COT + FRED Conviction — %s</i>\n", date.Format("02 Jan 2006")))

	// Show regime context if available
	if regime != nil {
		b.WriteString(fmt.Sprintf("\n📊 Regime: <b>%s</b> | Risk-Off: %d/100\n", regime.Name, regime.Score))
	}
	b.WriteString("\n")

	medals := []string{"🥇", "🥈", "🥉", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣"}

	for i, e := range entries {
		medal := ""
		if i < len(medals) {
			medal = medals[i]
		}

		colorDot := scoreDot(e.Score)
		sentSign := "+"
		if e.Score < 0 {
			sentSign = ""
		}

		convScore := int(math.Round(e.Conviction.Score))
		convLabel := e.Conviction.Label
		if convLabel == "" {
			convLabel = e.Conviction.Direction
		}

		b.WriteString(fmt.Sprintf("%s %s <b>%s</b> | Sent: %s%.0f | Conv: <b>%d/100</b> %s\n",
			medal, colorDot, e.Currency, sentSign, e.Score, convScore, convLabel))
	}

	// Best pairs based on conviction spread
	b.WriteString("\n📊 <b>Best Pairs:</b>\n")
	var plainEntries []rankEntry
	for _, e := range entries {
		plainEntries = append(plainEntries, rankEntry{
			Currency: e.Currency,
			Score:    e.Score,
			COTIndex: e.COTIndex,
		})
	}
	// Re-sort by raw sentiment for pair building
	sort.Slice(plainEntries, func(i, j int) bool {
		return plainEntries[i].Score > plainEntries[j].Score
	})
	pairs := buildBestPairs(plainEntries)
	for _, p := range pairs {
		b.WriteString(p + "\n")
	}

	// Regime advisory
	if regime != nil {
		advisory := regimeAdvisory(regime.Name)
		if advisory != "" {
			b.WriteString(fmt.Sprintf("\n⚠️ %s\n", advisory))
		}
	}

	b.WriteString("\n<i>Tip: </i><code>/cot EUR</code> untuk detail lengkap | <code>/macro</code> untuk FRED regime")
	return b.String()
}

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

// FormatConvictionBlock renders a compact conviction score block for the /cot detail view.
// Gap D — shows the unified 0-100 conviction score (COT + FRED + Calendar).
func (f *Formatter) FormatConvictionBlock(cs cot.ConvictionScore) string {
	icon := "⚪"
	switch {
	case cs.Score >= 65 && cs.Direction == "LONG":
		icon = "🟢"
	case cs.Score >= 65 && cs.Direction == "SHORT":
		icon = "🔴"
	case cs.Score >= 55:
		icon = "🟡"
	}

	return fmt.Sprintf(
		"\n<b>🎯 Conviction Score</b>\n"+
			"<code>%s %s %.0f/100 — %s</code>\n"+
			"<i>COT+FRED+Calendar fused signal</i>\n",
		icon, cs.Direction, cs.Score, cs.Label,
	)
}

// scoreArrow returns directional arrows for a sentiment score.
func scoreArrow(score float64) string {
	switch {
	case score > 60:
		return "↑↑"
	case score > 30:
		return "↑"
	case score > -30:
		return "→"
	case score > -60:
		return "↓"
	default:
		return "↓↓↓"
	}
}

// scoreDot returns a colored dot based on score direction.
func scoreDot(score float64) string {
	if score > 15 {
		return "🟢"
	} else if score < -15 {
		return "🔴"
	}
	return "⚪"
}

// cotLabel returns a human-readable label for a COT Index value (0-100).
func cotLabel(idx float64) string {
	switch {
	case idx >= 80:
		return "Extreme Long"
	case idx >= 60:
		return "Bullish"
	case idx >= 40:
		return "Neutral"
	case idx >= 20:
		return "Bearish"
	default:
		return "Extreme Short"
	}
}

// buildBestPairs generates the top 3 long/short pair recommendations.
// Long the highest-ranked currency, short the lowest-ranked.
func buildBestPairs(entries []rankEntry) []string {
	if len(entries) < 2 {
		return nil
	}

	var pairs []string
	seen := make(map[string]bool)

	// Try top-bull vs bottom-bear combinations
	for i := 0; i < len(entries) && len(pairs) < 3; i++ {
		for j := len(entries) - 1; j > i && len(pairs) < 3; j-- {
			long := entries[i]
			short := entries[j]
			spread := long.Score - short.Score

			if spread < 30 {
				continue // not enough spread to be meaningful
			}

			pairName := formatPairName(long.Currency, short.Currency)
			if seen[pairName] {
				continue
			}
			seen[pairName] = true

			direction := "LONG"
			spreadSign := "+"
			pairs = append(pairs, fmt.Sprintf("→ %s <b>%s</b> (spread %s%.0f)",
				direction, pairName, spreadSign, math.Abs(spread)))
		}
	}

	// If no strong spreads, show best available
	if len(pairs) == 0 && len(entries) >= 2 {
		long := entries[0]
		short := entries[len(entries)-1]
		spread := long.Score - short.Score
		pairName := formatPairName(long.Currency, short.Currency)
		pairs = append(pairs, fmt.Sprintf("→ LONG <b>%s</b> (spread +%.0f)", pairName, spread))
	}

	return pairs
}

// formatPairName formats a forex pair name from two currency codes.
// Follows standard convention: USD is always the second in majors where applicable.
func formatPairName(longCur, shortCur string) string {
	// Standard major pairs where USD is quote
	usdQuote := map[string]bool{"EUR": true, "GBP": true, "AUD": true, "NZD": true}
	// Standard major pairs where USD is base
	usdBase := map[string]bool{"JPY": true, "CHF": true, "CAD": true}

	if longCur == "USD" {
		if usdBase[shortCur] {
			return "USD" + shortCur // e.g., USDJPY
		}
		return shortCur + "USD" // e.g., EURUSD (reversed — USD short)
	}
	if shortCur == "USD" {
		if usdQuote[longCur] {
			return longCur + "USD" // e.g., GBPUSD
		}
		return "USD" + longCur // e.g., USDCAD
	}
	// Cross pair: long first
	return longCur + shortCur
}

// ---------------------------------------------------------------------------
// P1.4 — Upcoming Catalysts (48h COT context)
// ---------------------------------------------------------------------------

// FormatUpcomingCatalysts formats upcoming high/medium impact events for a given currency.
// Used in /cot detail to show "Upcoming Catalysts (48h)".
func (f *Formatter) FormatUpcomingCatalysts(currency string, events []domain.NewsEvent) string {
	if len(events) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n📅 <b>Upcoming Catalysts (48h):</b>\n")

	shown := 0
	for _, e := range events {
		if shown >= 5 {
			break
		}
		if !strings.EqualFold(e.Currency, currency) {
			continue
		}
		if strings.ToLower(e.Impact) != "high" && strings.ToLower(e.Impact) != "medium" {
			continue
		}
		if e.Actual != "" {
			continue // already released
		}

		timeStr := "TBA"
		if !e.TimeWIB.IsZero() {
			timeStr = e.TimeWIB.Format("Mon 15:04 WIB")
		}

		forecastStr := ""
		if e.Forecast != "" {
			forecastStr = " (Fcast: " + e.Forecast
			if e.Previous != "" {
				forecastStr += " | Prev: " + e.Previous
			}
			forecastStr += ")"
		}

		b.WriteString(fmt.Sprintf("%s %s — <b>%s</b> %s%s\n",
			e.FormatImpactColor(), timeStr, e.Currency, e.Event, forecastStr))
		shown++
	}

	if shown == 0 {
		return ""
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// P3.2 — FRED Macro Regime Dashboard
// ---------------------------------------------------------------------------

// FormatMacroRegime formats the FRED macro regime dashboard message.
// P3.2 — /macro command output. Now includes trend arrows, Sahm Rule, 3M-10Y spread,
// SOFR/IORB, Fed balance sheet, and M2 YoY growth.
func (f *Formatter) FormatMacroRegime(regime fred.MacroRegime, data *fred.MacroData) string {
	var b strings.Builder

	riskBar := buildRiskBar(regime.Score, 15)

	b.WriteString("🏦 <b>MACRO REGIME DASHBOARD</b>\n")
	b.WriteString(fmt.Sprintf("<i>FRED Data — Updated %s WIB</i>\n\n", data.FetchedAt.Format("02 Jan 15:04")))
	b.WriteString(fmt.Sprintf("<b>REGIME: %s</b>  Risk: %d/100\n", regime.Name, regime.Score))
	b.WriteString(fmt.Sprintf("<code>[%s]</code>\n", riskBar))
	b.WriteString(fmt.Sprintf("<code>Recession Risk: %s</code>\n\n", regime.RecessionRisk))

	// --- Yield Curve ---
	b.WriteString(fmt.Sprintf("<code>2Y-10Y Curve : %s</code>\n", regime.YieldCurve))
	b.WriteString(fmt.Sprintf("<code>               10Y=%.2f%% | 2Y=%.2f%%</code>\n", data.Yield10Y, data.Yield2Y))
	if regime.Yield3M10Y != "N/A" && regime.Yield3M10Y != "" {
		b.WriteString(fmt.Sprintf("<code>3M-10Y Curve : %s</code>\n", regime.Yield3M10Y))
		if data.Yield3M > 0 {
			b.WriteString(fmt.Sprintf("<code>               3M=%.2f%%</code>\n", data.Yield3M))
		}
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

// ---------------------------------------------------------------------------
// P2.3 — COT Regime Summary (used in /outlook or /rank)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Signal Detection Formatting
// ---------------------------------------------------------------------------

// FormatSignalsHTML formats detected COT signals for Telegram display.
func (f *Formatter) FormatSignalsHTML(signals []cot.Signal, filterCurrency string) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x8E\xAF <b>COT SIGNAL DETECTION</b>\n")
	if filterCurrency != "" {
		b.WriteString(fmt.Sprintf("<i>Filtered: %s</i>\n", filterCurrency))
	}
	b.WriteString("\n")

	if len(signals) == 0 {
		b.WriteString("No actionable signals detected.\n")
		b.WriteString("\n<i>Tip: Signals fire on extreme positioning, smart money moves,\ndivergences, momentum shifts, and thin markets.</i>")
		return b.String()
	}

	for i, s := range signals {
		if i >= 10 {
			b.WriteString(fmt.Sprintf("\n<i>... +%d more signals</i>", len(signals)-10))
			break
		}

		dirIcon := "\xF0\x9F\x9F\xA2"
		if s.Direction == "BEARISH" {
			dirIcon = "\xF0\x9F\x94\xB4"
		}

		strengthBar := strings.Repeat("\xE2\x96\x88", s.Strength) + strings.Repeat("\xE2\x96\x91", 5-s.Strength)

		b.WriteString(fmt.Sprintf("%s <b>%s</b> \xE2\x80\x94 %s\n", dirIcon, s.Currency, s.Type))
		b.WriteString(fmt.Sprintf("<code>  Str: [%s] %d/5 | Conf: %.0f%%</code>\n", strengthBar, s.Strength, s.Confidence))
		b.WriteString(fmt.Sprintf("<i>  %s</i>\n", s.Description))

		for _, factor := range s.Factors {
			b.WriteString(fmt.Sprintf("<code>  \xE2\x80\xA2 %s</code>\n", factor))
		}
		b.WriteString("\n")
	}

	b.WriteString("<i>Tip: </i><code>/signals EUR</code> | <code>/cot EUR</code>")
	return b.String()
}

// FormatSignalsSummary formats a compact signal summary for the /cot detail view.
func (f *Formatter) FormatSignalsSummary(signals []cot.Signal) string {
	if len(signals) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\xF0\x9F\x8E\xAF <b>Active Signals:</b>\n")

	for i, s := range signals {
		if i >= 3 {
			b.WriteString(fmt.Sprintf("<i>  +%d more \xE2\x80\x94 use /signals %s</i>\n", len(signals)-3, s.Currency))
			break
		}

		dirIcon := "\xF0\x9F\x9F\xA2"
		if s.Direction == "BEARISH" {
			dirIcon = "\xF0\x9F\x94\xB4"
		}

		b.WriteString(fmt.Sprintf("%s %s (%d/5, %.0f%%) \xE2\x80\x94 <i>%s</i>\n",
			dirIcon, s.Type, s.Strength, s.Confidence, s.Description))
	}

	return b.String()
}
