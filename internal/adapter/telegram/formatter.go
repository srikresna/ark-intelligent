package telegram

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
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

	modelDisplay := "Claude 🤖"
	if prefs.PreferredModel == "gemini" {
		modelDisplay = "Gemini ✨"
	}
	b.WriteString(fmt.Sprintf("<code>[AI] Chat Model    : %s</code>\n", modelDisplay))

	// Show active Claude model variant (only when using Claude)
	if prefs.PreferredModel != "gemini" {
		claudeVariant := "Server Default"
		if prefs.ClaudeModel != "" {
			claudeVariant = domain.ClaudeModelLabel(prefs.ClaudeModel)
		}
		b.WriteString(fmt.Sprintf("<code>[AI] Claude Variant : %s</code>\n", claudeVariant))
	}

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
// Direction is derived from the pair name: if the base currency (first 3 chars)
// matches the favored currency → LONG; if the base is the weak currency → SHORT.
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

			direction := pairDirection(pairName, long.Currency)
			pairs = append(pairs, fmt.Sprintf("→ %s <b>%s</b> (spread +%.0f)",
				direction, pairName, math.Abs(spread)))
		}
	}

	// If no strong spreads, show best available
	if len(pairs) == 0 && len(entries) >= 2 {
		long := entries[0]
		short := entries[len(entries)-1]
		spread := long.Score - short.Score
		pairName := formatPairName(long.Currency, short.Currency)
		direction := pairDirection(pairName, long.Currency)
		pairs = append(pairs, fmt.Sprintf("→ %s <b>%s</b> (spread +%.0f)", direction, pairName, spread))
	}

	return pairs
}

// pairDirection returns "LONG" if the favored currency is the base (first) in
// the pair, or "SHORT" if the favored currency ended up as the quote (second).
// Example: favored=USD, pair=AUDUSD → base is AUD (not favored) → SHORT AUDUSD.
//          favored=EUR, pair=EURUSD → base is EUR (favored)     → LONG EURUSD.
func pairDirection(pairName, favoredCurrency string) string {
	if strings.HasPrefix(pairName, favoredCurrency) {
		return "LONG"
	}
	return "SHORT"
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
	b.WriteString("<code>━━━ Treasury Yield Curve ━━━</code>\n")
	b.WriteString(fmt.Sprintf("<code> 3M    2Y    5Y   10Y   30Y</code>\n"))
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
			b.WriteString(fmt.Sprintf("<i>  +%d more signals available</i>\n", len(signals)-3))
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

// ---------------------------------------------------------------------------
// Backtest Formatting
// ---------------------------------------------------------------------------

// FormatBacktestStats formats a single BacktestStats into Telegram HTML.
func (f *Formatter) FormatBacktestStats(stats *domain.BacktestStats) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8A <b>Backtest: %s</b>\n\n", stats.GroupLabel))

	b.WriteString(fmt.Sprintf("<code>Signals  :</code> %d total, %d evaluated\n", stats.TotalSignals, stats.Evaluated))
	b.WriteString(fmt.Sprintf("<code>Win 1W   :</code> %.1f%% (n=%d)\n", stats.WinRate1W, stats.Evaluated1W))
	b.WriteString(fmt.Sprintf("<code>Win 2W   :</code> %.1f%% (n=%d)\n", stats.WinRate2W, stats.Evaluated2W))
	b.WriteString(fmt.Sprintf("<code>Win 4W   :</code> %.1f%% (n=%d)\n", stats.WinRate4W, stats.Evaluated4W))
	b.WriteString(fmt.Sprintf("<code>Best     :</code> %s at %.1f%%\n\n", stats.BestPeriod, stats.BestWinRate))

	b.WriteString(fmt.Sprintf("<code>Avg Ret 1W:</code> %.2f%%\n", stats.AvgReturn1W))
	b.WriteString(fmt.Sprintf("<code>Avg Ret 2W:</code> %.2f%%\n", stats.AvgReturn2W))
	b.WriteString(fmt.Sprintf("<code>Avg Ret 4W:</code> %.2f%%\n\n", stats.AvgReturn4W))

	if stats.AvgWinReturn1W != 0 || stats.AvgLossReturn1W != 0 {
		b.WriteString(fmt.Sprintf("<code>Avg Win  :</code> +%.2f%%\n", stats.AvgWinReturn1W))
		b.WriteString(fmt.Sprintf("<code>Avg Loss :</code> %.2f%%\n\n", stats.AvgLossReturn1W))
	}

	// Risk-adjusted performance metrics
	if stats.SharpeRatio != 0 || stats.MaxDrawdown != 0 || stats.ProfitFactor != 0 {
		b.WriteString("<b>Risk-Adjusted Metrics</b>\n")
		if stats.SharpeRatio != 0 {
			sharpeIcon := "\xE2\x9C\x85" // checkmark
			if stats.SharpeRatio < 0.5 {
				sharpeIcon = "\xE2\x9A\xA0\xEF\xB8\x8F" // warning
			}
			b.WriteString(fmt.Sprintf("<code>Sharpe   :</code> %.2f %s\n", stats.SharpeRatio, sharpeIcon))
		}
		if stats.MaxDrawdown != 0 {
			ddIcon := "\xE2\x9C\x85"
			if stats.MaxDrawdown > 10 {
				ddIcon = "\xE2\x9A\xA0\xEF\xB8\x8F"
			}
			b.WriteString(fmt.Sprintf("<code>Max DD   :</code> -%.2f%% %s\n", stats.MaxDrawdown, ddIcon))
		}
		if stats.CalmarRatio != 0 {
			b.WriteString(fmt.Sprintf("<code>Calmar   :</code> %.2f\n", stats.CalmarRatio))
		}
		if stats.ProfitFactor != 0 {
			pfIcon := "\xE2\x9C\x85"
			if stats.ProfitFactor < 1.0 {
				pfIcon = "\xF0\x9F\x94\xB4" // red circle
			}
			b.WriteString(fmt.Sprintf("<code>Profit F :</code> %.2f %s\n", stats.ProfitFactor, pfIcon))
		}
		if stats.ExpectedValue != 0 {
			b.WriteString(fmt.Sprintf("<code>Exp Value:</code> %.4f%%\n", stats.ExpectedValue))
		}
		if stats.KellyFraction != 0 {
			b.WriteString(fmt.Sprintf("<code>Kelly %%  :</code> %.1f%%\n", stats.KellyFraction*100))
		}
		b.WriteString("\n")
	}

	b.WriteString("<b>Strength Breakdown</b>\n")
	b.WriteString(fmt.Sprintf("<code>High (4-5):</code> %d signals, %.1f%% win\n", stats.HighStrengthCount, stats.HighStrengthWinRate))
	b.WriteString(fmt.Sprintf("<code>Low (1-3) :</code> %d signals, %.1f%% win\n\n", stats.LowStrengthCount, stats.LowStrengthWinRate))

	b.WriteString("<b>Confidence Calibration</b>\n")
	b.WriteString(fmt.Sprintf("<code>Stated   :</code> %.0f%%\n", stats.AvgConfidence))
	b.WriteString(fmt.Sprintf("<code>Actual   :</code> %.1f%%\n", stats.ActualAccuracy))

	calIcon := "\xE2\x9C\x85"
	if stats.CalibrationError > 15 {
		calIcon = "\xE2\x9A\xA0\xEF\xB8\x8F"
	}
	b.WriteString(fmt.Sprintf("<code>Error    :</code> %.1f%% %s\n", stats.CalibrationError, calIcon))

	// Brier score — lower is better
	if stats.BrierScore > 0 {
		brierIcon := "\xE2\x9C\x85" // checkmark — excellent (<0.15)
		if stats.BrierScore >= 0.25 {
			brierIcon = "\xF0\x9F\x94\xB4" // red circle — worse than random
		} else if stats.BrierScore >= 0.15 {
			brierIcon = "\xE2\x9A\xA0\xEF\xB8\x8F" // warning — decent but not great
		}
		b.WriteString(fmt.Sprintf("<code>Brier    :</code> %.4f %s\n", stats.BrierScore, brierIcon))
	}

	// Calibration method
	if stats.CalibrationMethod != "" {
		b.WriteString(fmt.Sprintf("<code>Method   :</code> %s\n", stats.CalibrationMethod))
	}

	// Statistical significance
	b.WriteString("\n<b>Statistical Significance</b>\n")
	if stats.Evaluated1W > 0 {
		if stats.IsStatisticallySignificant {
			b.WriteString("\xE2\x9C\x93 <b>Statistically Significant</b>\n")
		} else {
			b.WriteString("\xE2\x9A\xA0 <b>Insufficient Data</b>\n")
		}
		b.WriteString(fmt.Sprintf("<code>WR p-val :</code> %.4f\n", stats.WinRatePValue))
		b.WriteString(fmt.Sprintf("<code>WR 95%% CI:</code> [%.1f%%, %.1f%%]\n", stats.WinRateCI[0], stats.WinRateCI[1]))
		if stats.ReturnPValue < 1 {
			b.WriteString(fmt.Sprintf("<code>Ret t-stat:</code> %.2f (p=%.4f)\n", stats.ReturnTStat, stats.ReturnPValue))
		}
		if stats.Evaluated1W < stats.MinSamplesNeeded {
			b.WriteString(fmt.Sprintf("<code>Samples  :</code> %d / %d needed\n", stats.Evaluated1W, stats.MinSamplesNeeded))
		}
	} else {
		b.WriteString("\xE2\x9A\xA0 <b>Insufficient Data</b>\n")
		b.WriteString(fmt.Sprintf("<code>Need     :</code> %d+ evaluated signals\n", stats.MinSamplesNeeded))
	}

	return b.String()
}

// FormatBacktestSummary formats a map of BacktestStats into a comparison table.
func (f *Formatter) FormatBacktestSummary(statsMap map[string]*domain.BacktestStats, groupBy string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8A <b>Backtest by %s</b>\n\n", groupBy))

	// Sort keys for consistent output
	keys := make([]string, 0, len(statsMap))
	for k := range statsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-12s %4s %5s %5s %5s\n", "Group", "Eval", "1W", "2W", "4W"))
	b.WriteString(strings.Repeat("\xE2\x94\x80", 40) + "\n")

	for _, k := range keys {
		s := statsMap[k]
		label := s.GroupLabel
		if len(label) > 12 {
			label = label[:12]
		}
		b.WriteString(fmt.Sprintf("%-12s %4d %4.0f%% %4.0f%% %4.0f%%\n",
			label, s.Evaluated, s.WinRate1W, s.WinRate2W, s.WinRate4W))
	}
	b.WriteString("</pre>")

	return b.String()
}

// FormatSignalTiming formats per-signal-type timing analysis into Telegram HTML.
func (f *Formatter) FormatSignalTiming(analyses []backtestsvc.SignalTimingAnalysis) string {
	var b strings.Builder

	b.WriteString("\xE2\x8F\xB1 <b>Signal Timing Analysis</b>\n")
	b.WriteString("<i>Optimal horizon per signal type</i>\n\n")

	for _, a := range analyses {
		b.WriteString(fmt.Sprintf("<b>%s</b>\n", a.SignalType))
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-8s %5s %7s %6s %5s\n", "Horizon", "Win%", "AvgRet", "MaxDD", "R:R"))
		b.WriteString(strings.Repeat("\xe2\x94\x80", 36) + "\n")

		for _, h := range a.HorizonStats {
			marker := "  "
			if h.Horizon == a.OptimalHorizon {
				marker = "\xe2\x9e\xa4 "
			}
			rrStr := " -"
			if h.RiskRewardRatio > 0 {
				rrStr = fmt.Sprintf("%.1f", h.RiskRewardRatio)
			}
			ddStr := " -"
			if h.MaxDrawdown > 0 {
				ddStr = fmt.Sprintf("%.1f%%", h.MaxDrawdown)
			}
			if h.Evaluated == 0 {
				b.WriteString(fmt.Sprintf("%s%-5s %5s %7s %6s %5s\n",
					marker, h.Horizon, "-", "-", "-", "-"))
			} else {
				b.WriteString(fmt.Sprintf("%s%-5s %4.0f%% %+6.2f%% %6s %5s\n",
					marker, h.Horizon, h.WinRate, h.AvgReturn, ddStr, rrStr))
			}
		}
		b.WriteString("</pre>")

		// Recommendation line
		icon := "\xf0\x9f\x93\x8c" // pushpin
		if a.Degrading {
			icon = "\xe2\x9a\xa0\xef\xb8\x8f" // warning
		}
		b.WriteString(fmt.Sprintf("%s <i>%s</i>\n\n", icon, a.Recommendation))
	}

	return b.String()
}

// FormatWalkForward formats walk-forward analysis results into Telegram HTML.
func (f *Formatter) FormatWalkForward(result *backtestsvc.WalkForwardResult) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x94\xAC <b>Walk-Forward Analysis</b>\n")
	b.WriteString("<i>Train/test split to detect overfitting</i>\n\n")

	// Per-window table.
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-5s %6s %6s %6s %6s\n", "Win#", "Train", "Test", "Degr", "n(T/O)"))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 36) + "\n")

	for i, w := range result.Windows {
		degSign := ""
		if w.Degradation >= 0 {
			degSign = "+"
		}
		b.WriteString(fmt.Sprintf(" %2d   %5.1f%% %5.1f%% %s%.1f %3d/%-3d\n",
			i+1, w.InSampleWinRate, w.OutOfSampleWinRate,
			degSign, w.Degradation, w.InSampleCount, w.OutOfSampleCount))
	}
	b.WriteString("</pre>\n")

	// Window date ranges.
	b.WriteString("<i>Window periods:</i>\n")
	for i, w := range result.Windows {
		b.WriteString(fmt.Sprintf("<code>%d:</code> %s \xe2\x86\x92 %s | %s \xe2\x86\x92 %s\n",
			i+1,
			w.TrainStart.Format("02 Jan"),
			w.TrainEnd.Format("02 Jan"),
			w.TestStart.Format("02 Jan"),
			w.TestEnd.Format("02 Jan")))
	}

	b.WriteString("\n")

	// Overall summary.
	b.WriteString("<b>Overall</b>\n")
	b.WriteString(fmt.Sprintf("<code>In-Sample WR  :</code> %.1f%%\n", result.OverallInSampleWinRate))
	b.WriteString(fmt.Sprintf("<code>Out-of-Sample :</code> %.1f%%\n", result.OverallOutOfSampleWinRate))

	// Traffic light for overfit score.
	var light string
	switch {
	case result.OverfitScore < 3:
		light = "\xF0\x9F\x9F\xA2" // green
	case result.OverfitScore <= 10:
		light = "\xF0\x9F\x9F\xA1" // yellow
	default:
		light = "\xF0\x9F\x94\xB4" // red
	}
	b.WriteString(fmt.Sprintf("<code>Overfit Score :</code> %s %.1fpp\n", light, result.OverfitScore))

	if result.IsOverfit {
		b.WriteString("\n\xE2\x9A\xA0\xEF\xB8\x8F <b>OVERFITTING DETECTED</b>\n")
	}

	b.WriteString(fmt.Sprintf("\n\xF0\x9F\x93\x8C <i>%s</i>", result.Recommendation))

	return b.String()
}

// FormatWeightOptimization formats factor weight optimization results into Telegram HTML.
func (f *Formatter) FormatWeightOptimization(result *backtestsvc.WeightResult) string {
	var b strings.Builder

	b.WriteString("\xE2\x9A\x96\xEF\xB8\x8F <b>Factor Weight Optimization</b>\n")
	b.WriteString("<i>OLS regression: Return1W ~ COT + Stress + FRED + Price</i>\n\n")

	b.WriteString(fmt.Sprintf("<code>Sample Size   :</code> %d signals\n", result.SampleSize))
	b.WriteString(fmt.Sprintf("<code>R\xC2\xB2            :</code> %.4f\n", result.RSquared))
	b.WriteString(fmt.Sprintf("<code>Adj R\xC2\xB2        :</code> %.4f\n", result.AdjRSquared))

	// Weight comparison table.
	b.WriteString("\n<pre>")
	b.WriteString(fmt.Sprintf("%-10s %7s %7s %5s %6s\n", "Factor", "Current", "Optim.", "Sig?", "p-val"))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 42) + "\n")

	factorOrder := []string{"COT", "Stress", "FRED", "Price"}
	for _, name := range factorOrder {
		curr := 0.0
		if result.CurrentWeights != nil {
			curr = result.CurrentWeights[name]
		}
		opt := 0.0
		if result.OptimizedWeights != nil {
			opt = result.OptimizedWeights[name]
		}
		sig := " "
		if result.FactorSignificance != nil && result.FactorSignificance[name] {
			sig = "*"
		}
		pVal := 1.0
		if result.FactorPValues != nil {
			pVal = result.FactorPValues[name]
		}
		b.WriteString(fmt.Sprintf("%-10s %6.1f%% %6.1f%%   %s  %.3f\n",
			name, curr, opt, sig, pVal))
	}
	b.WriteString("</pre>\n")
	b.WriteString("<i>* = statistically significant (p &lt; 0.05)</i>\n")

	// Raw coefficients.
	if result.FactorCoefficients != nil {
		b.WriteString("\n<b>Raw Coefficients</b>\n<pre>")
		for _, name := range factorOrder {
			coeff := result.FactorCoefficients[name]
			b.WriteString(fmt.Sprintf("%-10s %+.4f\n", name, coeff))
		}
		b.WriteString("</pre>\n")
	}

	// Per-contract weights.
	if len(result.PerContractWeights) > 0 {
		b.WriteString("\n<b>Per-Currency Weights</b>\n<pre>")
		b.WriteString(fmt.Sprintf("%-5s %5s %5s %5s %5s\n", "Ccy", "COT", "Str", "FRED", "Prc"))
		b.WriteString(strings.Repeat("\xe2\x94\x80", 30) + "\n")

		// Sort currencies for deterministic output.
		var currencies []string
		for c := range result.PerContractWeights {
			currencies = append(currencies, c)
		}
		sort.Strings(currencies)

		for _, ccy := range currencies {
			w := result.PerContractWeights[ccy]
			b.WriteString(fmt.Sprintf("%-5s %4.0f%% %4.0f%% %4.0f%% %4.0f%%\n",
				ccy, w["COT"], w["Stress"], w["FRED"], w["Price"]))
		}
		b.WriteString("</pre>\n")
	}

	b.WriteString(fmt.Sprintf("\n\xF0\x9F\x93\x8C <i>%s</i>", result.Recommendation))

	return b.String()
}

// FormatPriceContext formats price context for a single contract.
func (f *Formatter) FormatPriceContext(pc *domain.PriceContext) string {
	if pc == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\xF0\x9F\x92\xB0 <b>Price Context</b>\n")
	b.WriteString(fmt.Sprintf("<code>Price    :</code> %.5f\n", pc.CurrentPrice))

	wIcon := "\xF0\x9F\x9F\xA2"
	if pc.WeeklyChgPct < 0 {
		wIcon = "\xF0\x9F\x94\xB4"
	}
	b.WriteString(fmt.Sprintf("<code>Weekly   :</code> %s %.2f%%\n", wIcon, pc.WeeklyChgPct))

	mIcon := "\xF0\x9F\x9F\xA2"
	if pc.MonthlyChgPct < 0 {
		mIcon = "\xF0\x9F\x94\xB4"
	}
	b.WriteString(fmt.Sprintf("<code>Monthly  :</code> %s %.2f%%\n", mIcon, pc.MonthlyChgPct))

	trendIcon := "\xE2\x9E\xA1\xEF\xB8\x8F"
	if pc.Trend4W == "UP" {
		trendIcon = "\xE2\xAC\x86\xEF\xB8\x8F"
	} else if pc.Trend4W == "DOWN" {
		trendIcon = "\xE2\xAC\x87\xEF\xB8\x8F"
	}
	b.WriteString(fmt.Sprintf("<code>Trend 4W :</code> %s %s\n", trendIcon, pc.Trend4W))

	ma4wStatus := "below"
	if pc.AboveMA4W {
		ma4wStatus = "above"
	}
	ma13wStatus := "below"
	if pc.AboveMA13W {
		ma13wStatus = "above"
	}
	b.WriteString(fmt.Sprintf("<code>MA4W     :</code> %.5f (%s)\n", pc.PriceMA4W, ma4wStatus))
	b.WriteString(fmt.Sprintf("<code>MA13W    :</code> %.5f (%s)\n", pc.PriceMA13W, ma13wStatus))

	// ATR-based volatility regime
	if pc.VolatilityRegime != "" {
		volIcon := "\xF0\x9F\x9F\xA1" // yellow circle for NORMAL
		switch pc.VolatilityRegime {
		case "EXPANDING":
			volIcon = "\xF0\x9F\x94\xB4" // red — high volatility
		case "CONTRACTING":
			volIcon = "\xF0\x9F\x9F\xA2" // green — low volatility
		}
		b.WriteString(fmt.Sprintf("<code>ATR Vol  :</code> %s %s (ATR: %.5f, %.2f%%)\n",
			volIcon, pc.VolatilityRegime, pc.ATR, pc.NormalizedATR))
	}

	return b.String()
}

// FormatPriceCOTDivergence formats a price-COT divergence alert.
func (f *Formatter) FormatPriceCOTDivergence(div pricesvc.PriceCOTDivergence) string {
	icon := "\xE2\x9A\xA0\xEF\xB8\x8F" // ⚠️
	if div.Severity == "HIGH" {
		icon = "\xF0\x9F\x94\xB4" // 🔴
	}
	return fmt.Sprintf("\n%s <b>DIVERGENCE: %s</b>\n<i>%s</i>\n", icon, div.Severity, div.Description)
}

// FormatStrengthRanking formats the dual price+COT currency strength ranking.
func (f *Formatter) FormatStrengthRanking(strengths []pricesvc.CurrencyStrength) string {
	if len(strengths) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\xF0\x9F\x92\xAA <b>Price + COT Strength</b>\n")
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-4s %6s %5s %6s\n", "CCY", "Price", "COT", "Score"))
	b.WriteString(strings.Repeat("\xE2\x94\x80", 24) + "\n")

	for _, s := range strengths {
		divFlag := " "
		if s.Divergence {
			divFlag = "!"
		}
		b.WriteString(fmt.Sprintf("%-4s %+5.1f %+4.0f %+5.1f %s\n",
			s.Currency, s.PriceScore, s.COTScore, s.CombinedScore, divFlag))
	}
	b.WriteString("</pre>")

	// Show divergence warnings
	for _, s := range strengths {
		if s.Divergence {
			b.WriteString(fmt.Sprintf("\xE2\x9A\xA0\xEF\xB8\x8F %s: %s\n", s.Currency, s.DivergenceMsg))
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Regime-Asset Performance Matrix Formatting
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Weekly Report Formatting
// ---------------------------------------------------------------------------

// FormatWeeklyReport formats a WeeklyReport into Telegram HTML.
func (f *Formatter) FormatWeeklyReport(r *domain.WeeklyReport) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8B <b>Weekly Performance Report</b>\n"))
	b.WriteString(fmt.Sprintf("<i>%s \xe2\x80\x94 %s</i>\n\n",
		r.WeekStart.Format("02 Jan"),
		r.WeekEnd.Format("02 Jan 2006"),
	))

	if len(r.Signals) == 0 {
		b.WriteString("No signals generated this week.\n")
	} else {
		// Cap displayed signals to avoid exceeding Telegram's 4096-char limit
		// when the <pre> block is too large (each row ~60 chars; 50 rows = 3000).
		maxDisplay := 50
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-5s %-14s %-8s %7s  %s\n", "CCY", "SIGNAL", "DIR", "MOVE", ""))
		b.WriteString(fmt.Sprintf("%-5s %-14s %-8s %7s  %s\n", "---", "-----------", "------", "------", "---"))
		for i, s := range r.Signals {
			if i >= maxDisplay {
				break
			}
			dir := shortDirection(s.Direction)
			result := resultBadge(s.Result)
			move := fmt.Sprintf("%+.2f%%", s.PipsChange)
			if s.Result == domain.OutcomePending {
				move = "   ---"
			}
			sigLabel := truncateStr(s.SignalType, 14)
			b.WriteString(fmt.Sprintf("%-5s %-14s %-8s %7s  %s\n",
				truncateStr(s.Contract, 5), sigLabel, dir, move, result))
		}
		b.WriteString("</pre>")
		if len(r.Signals) > maxDisplay {
			b.WriteString(fmt.Sprintf("\n<i>... +%d more signals (showing top %d)</i>\n", len(r.Signals)-maxDisplay, maxDisplay))
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString(fmt.Sprintf("<b>Weekly Score:</b> %s\n", r.WeeklyScore))

	if r.RunningAverage52W > 0 {
		b.WriteString(fmt.Sprintf("<b>52W Average:</b>  %.1f%%\n", r.RunningAverage52W))
	}

	b.WriteString(fmt.Sprintf("<b>Current Streak:</b> %d wins\n", r.CurrentStreak))
	b.WriteString(fmt.Sprintf("<b>Best Streak:</b>    %d wins\n", r.BestStreak))

	b.WriteString("\n<i>Use /backtest for full historical stats</i>")
	return b.String()
}

// shortDirection returns a compact direction label.
func shortDirection(d string) string {
	switch d {
	case "BULLISH":
		return "\xF0\x9F\x9F\xA2 BULL"
	case "BEARISH":
		return "\xF0\x9F\x94\xB4 BEAR"
	default:
		return d
	}
}

// resultBadge returns an emoji badge for a signal outcome.
func resultBadge(r string) string {
	switch r {
	case domain.OutcomeWin:
		return "\xE2\x9C\x85"
	case domain.OutcomeLoss:
		return "\xE2\x9D\x8C"
	default:
		return "\xE2\x8F\xB3"
	}
}

// truncateStr shortens a string to maxLen, adding ".." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

// ---------------------------------------------------------------------------
// Event Impact Formatting
// ---------------------------------------------------------------------------

// FormatEventImpact formats event impact summaries into a clean Telegram HTML message.
func (f *Formatter) FormatEventImpact(eventTitle string, summaries []domain.EventImpactSummary) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8A <b>EVENT IMPACT: %s</b>\n", strings.ToUpper(html.EscapeString(eventTitle))))
	b.WriteString("<i>Historical price reaction by surprise magnitude (1h horizon)</i>\n\n")

	if len(summaries) == 0 {
		b.WriteString("No impact data recorded yet for this event.\n")
		b.WriteString("<i>Data builds automatically after each release.</i>")
		return b.String()
	}

	// Group by currency
	byCurrency := make(map[string][]domain.EventImpactSummary)
	var currencies []string
	for _, s := range summaries {
		if _, exists := byCurrency[s.Currency]; !exists {
			currencies = append(currencies, s.Currency)
		}
		byCurrency[s.Currency] = append(byCurrency[s.Currency], s)
	}
	sort.Strings(currencies)

	for _, ccy := range currencies {
		items := byCurrency[ccy]
		b.WriteString(fmt.Sprintf("<b>%s</b>\n", ccy))
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-14s %7s %7s %4s\n", "Sigma", "AvgPip", "Median", "N"))
		b.WriteString(strings.Repeat("\xE2\x94\x80", 36) + "\n")

		for _, item := range items {
			direction := " "
			if item.AvgPriceImpactPips > 0 {
				direction = "+"
			}
			b.WriteString(fmt.Sprintf("%-14s %s%6.1f %+7.1f %4d\n",
				item.SigmaBucket, direction, math.Abs(item.AvgPriceImpactPips), item.MedianImpact, item.Occurrences))
		}
		b.WriteString("</pre>\n")
	}

	b.WriteString("<i>Positive = currency strengthened</i>")
	return b.String()
}

// FormatTrackedEvents formats a list of tracked event names for the /impact help message.
func (f *Formatter) FormatTrackedEvents(events []string) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x8B <b>EVENT IMPACT DATABASE</b>\n")
	b.WriteString("<i>Historical price reaction tracking</i>\n\n")

	if len(events) == 0 {
		b.WriteString("No events tracked yet. Impact data builds automatically\n")
		b.WriteString("after each economic release with price data available.\n\n")
		b.WriteString("Usage: <code>/impact NFP</code> or <code>/impact CPI</code>")
		return b.String()
	}

	b.WriteString("<b>Tracked Events:</b>\n")
	for i, ev := range events {
		if i >= 20 {
			b.WriteString(fmt.Sprintf("\n<i>... and %d more</i>", len(events)-20))
			break
		}
		b.WriteString(fmt.Sprintf("\xE2\x80\xA2 %s\n", ev))
	}

	b.WriteString("\nUsage: <code>/impact Event Name</code>\n")
	b.WriteString("Example: <code>/impact Non-Farm Employment Change</code>")
	return b.String()
}

// ---------------------------------------------------------------------------
// Smart Money Accuracy Formatting
// ---------------------------------------------------------------------------

// FormatSmartMoneyAccuracy formats smart money tracking accuracy per contract.
func (f *Formatter) FormatSmartMoneyAccuracy(results []backtestsvc.SmartMoneyAccuracy) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\xA7\xA0 <b>SMART MONEY TRACKING ACCURACY</b>\n")
	b.WriteString("<i>Does \"smart money\" actually predict price? (52-week analysis)</i>\n\n")

	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-5s %5s %5s %5s %5s %5s\n", "CCY", "1W", "2W", "4W", "Corr", "Edge"))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 38) + "\n")

	for _, r := range results {
		edgeIcon := "\xe2\x9c\x97"
		if r.Edge == "YES" {
			edgeIcon = "\xe2\x9c\x93"
		} else if r.Edge == "INSUFFICIENT" {
			edgeIcon = "?"
		}
		b.WriteString(fmt.Sprintf("%-5s %4.0f%% %4.0f%% %4.0f%% %+.2f  %s\n",
			r.Currency, r.Accuracy1W, r.Accuracy2W, r.Accuracy4W, r.Correlation, edgeIcon))
	}
	b.WriteString("</pre>\n")

	// Highlight best and worst
	if len(results) > 0 {
		best := results[0] // already sorted by BestAccuracy desc
		b.WriteString(fmt.Sprintf("\n\xF0\x9F\x8F\x86 <b>Most Reliable:</b> %s \xe2\x80\x94 %.0f%% at %s\n",
			best.Currency, best.BestAccuracy, best.BestHorizon))

		worst := results[len(results)-1]
		if worst.Edge == "NO" {
			b.WriteString(fmt.Sprintf("\xe2\x9a\xa0\xef\xb8\x8f <b>No Edge:</b> %s \xe2\x80\x94 %.0f%% (consider ignoring SM signals)\n",
				worst.Currency, worst.BestAccuracy))
		}
	}

	b.WriteString("\n<i>Edge = best horizon \xe2\x89\xa555%% with n\xe2\x89\xa510</i>\n")
	b.WriteString("<i>Corr = Pearson correlation (net change vs 1W price)</i>")

	return b.String()
}

// ---------------------------------------------------------------------------
// Sentiment Survey Dashboard
// ---------------------------------------------------------------------------

// FormatSentiment renders the sentiment survey dashboard as Telegram HTML.
func (f *Formatter) FormatSentiment(data *sentiment.SentimentData) string {
	var b strings.Builder

	b.WriteString("🧠 <b>SENTIMENT SURVEY DASHBOARD</b>\n")
	b.WriteString(fmt.Sprintf("<i>Updated %s WIB</i>\n", data.FetchedAt.Format("02 Jan 15:04")))

	// --- CNN Fear & Greed Index ---
	b.WriteString("\n<b>CNN Fear &amp; Greed Index</b>\n")
	if data.CNNAvailable {
		gauge := sentimentGauge(data.CNNFearGreed, 15)
		emoji := fearGreedEmoji(data.CNNFearGreed)
		b.WriteString(fmt.Sprintf("<code>[%s]</code>\n", gauge))
		b.WriteString(fmt.Sprintf("<code>Score : %.0f / 100  %s %s</code>\n", data.CNNFearGreed, emoji, data.CNNFearGreedLabel))

		// Contrarian signal
		if data.CNNFearGreed <= 25 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Extreme fear often precedes rallies\n")
		} else if data.CNNFearGreed >= 75 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Extreme greed often precedes pullbacks\n")
		}
	} else {
		b.WriteString("<code>Data unavailable</code>\n")
	}

	// --- AAII Investor Sentiment Survey ---
	b.WriteString("\n<b>AAII Investor Sentiment Survey</b>\n")
	if data.AAIIAvailable {
		b.WriteString(fmt.Sprintf("<code>Bullish : %5.1f%%</code>  %s\n", data.AAIIBullish, sentimentBar(data.AAIIBullish, "🟢")))
		b.WriteString(fmt.Sprintf("<code>Neutral : %5.1f%%</code>  %s\n", data.AAIINeutral, sentimentBar(data.AAIINeutral, "⚪")))
		b.WriteString(fmt.Sprintf("<code>Bearish : %5.1f%%</code>  %s\n", data.AAIIBearish, sentimentBar(data.AAIIBearish, "🔴")))
		b.WriteString(fmt.Sprintf("<code>Bull/Bear: %.2f</code>", data.AAIIBullBear))
		if data.AAIIBullBear > 0 {
			if data.AAIIBullBear >= 2.0 {
				b.WriteString("  — ⚠️ Elevated optimism")
			} else if data.AAIIBullBear <= 0.5 {
				b.WriteString("  — 🟢 Deep pessimism (contrarian bullish)")
			}
		}
		b.WriteString("\n")

		// Historical context: AAII long-term averages are ~37.5% bull, 31% bear, 31.5% neutral
		if data.AAIIBullish >= 50 {
			b.WriteString("<code>Note   : Bullish reading well above historical avg (~37.5%%)</code>\n")
		} else if data.AAIIBearish >= 50 {
			b.WriteString("<code>Note   : Bearish reading well above historical avg (~31%%)</code>\n")
		}
	} else {
		b.WriteString("<code>Data unavailable — AAII updates weekly (Thursday)</code>\n")
	}

	// --- Composite reading ---
	if data.CNNAvailable || data.AAIIAvailable {
		b.WriteString("\n<b>Interpretation</b>\n")
		b.WriteString("<i>Sentiment surveys are contrarian indicators.\n")
		b.WriteString("Extreme readings often mark turning points.</i>\n")
	}

	return b.String()
}

// sentimentGauge builds a visual gauge bar for Fear & Greed (0-100).
func sentimentGauge(score float64, width int) string {
	pos := int(score / 100 * float64(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}

	bar := make([]byte, width)
	for i := range bar {
		bar[i] = '-'
	}
	bar[pos] = '|'

	return "Fear " + string(bar) + " Greed"
}

// fearGreedEmoji returns an emoji indicator for the CNN F&G score.
func fearGreedEmoji(score float64) string {
	switch {
	case score <= 25:
		return "😱"
	case score <= 45:
		return "😟"
	case score <= 55:
		return "😐"
	case score <= 75:
		return "😏"
	default:
		return "🤑"
	}
}

// sentimentBar builds a compact visual bar for a percentage (0-100).
func sentimentBar(pct float64, emoji string) string {
	const barWidth = 10
	filled := int(pct / 100 * barWidth)
	if filled > barWidth {
		filled = barWidth
	}
	return strings.Repeat(emoji, filled)
}

// ---------------------------------------------------------------------------
// Seasonal Pattern Formatting
// ---------------------------------------------------------------------------

// FormatSeasonalPatterns formats seasonal analysis results as a compact HTML table.
func (f *Formatter) FormatSeasonalPatterns(patterns []pricesvc.SeasonalPattern) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x85 <b>SEASONAL PATTERN ANALYSIS</b>\n")
	b.WriteString("<i>Historical monthly bias (up to 5 years)</i>\n\n")

	// Compact table: currency + 12 months with emoji bias indicators
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-5s", "CCY"))
	shortMonths := [12]string{"J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"}
	for _, m := range shortMonths {
		b.WriteString(fmt.Sprintf(" %s", m))
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("\xe2\x94\x80", 30) + "\n")

	for _, p := range patterns {
		b.WriteString(fmt.Sprintf("%-5s", p.Currency))
		for i := 0; i < 12; i++ {
			icon := "\xc2\xb7" // middle dot for NEUTRAL
			switch p.Monthly[i].Bias {
			case "BULLISH":
				icon = "\xe2\x96\xb2" // triangle up
			case "BEARISH":
				icon = "\xe2\x96\xbc" // triangle down
			}
			if i+1 == p.CurrentMonth {
				b.WriteString(fmt.Sprintf("[%s", icon))
			} else {
				b.WriteString(fmt.Sprintf(" %s", icon))
			}
			if i+1 == p.CurrentMonth {
				b.WriteString("]")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("</pre>\n")

	// Legend
	b.WriteString("<i>\xe2\x96\xb2 = Bullish (avg&gt;0, WR&gt;55%%)  \xe2\x96\xbc = Bearish  \xc2\xb7 = Neutral</i>\n")
	b.WriteString("<i>[x] = current month</i>\n")

	// Strongest tendencies
	type tendency struct {
		currency string
		month    string
		avgRet   float64
		winRate  float64
		bias     string
	}
	var strong []tendency
	for _, p := range patterns {
		for i := 0; i < 12; i++ {
			ms := p.Monthly[i]
			if ms.Bias != "NEUTRAL" && ms.SampleSize >= 3 {
				strong = append(strong, tendency{
					currency: p.Currency,
					month:    ms.Month,
					avgRet:   ms.AvgReturn,
					winRate:  ms.WinRate,
					bias:     ms.Bias,
				})
			}
		}
	}

	// Sort by absolute avg return descending to find strongest
	sort.Slice(strong, func(i, j int) bool {
		ai := strong[i].avgRet
		if ai < 0 {
			ai = -ai
		}
		aj := strong[j].avgRet
		if aj < 0 {
			aj = -aj
		}
		return ai > aj
	})

	if len(strong) > 0 {
		b.WriteString("\n\xF0\x9F\x94\xA5 <b>Strongest Tendencies:</b>\n")
		limit := 5
		if len(strong) < limit {
			limit = len(strong)
		}
		for _, t := range strong[:limit] {
			icon := "\xF0\x9F\x9F\xA2"
			if t.bias == "BEARISH" {
				icon = "\xF0\x9F\x94\xB4"
			}
			b.WriteString(fmt.Sprintf("%s %s %s: %+.2f%% (%.0f%% WR)\n",
				icon, t.currency, t.month, t.avgRet, t.winRate))
		}
	}

	return b.String()
}

// FormatSeasonalSingle formats seasonal analysis for a single contract in detail.
func (f *Formatter) FormatSeasonalSingle(p pricesvc.SeasonalPattern) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x85 <b>SEASONAL PATTERNS: %s</b>\n", html.EscapeString(p.Currency)))
	b.WriteString("<i>Historical monthly return statistics (up to 5 years)</i>\n\n")

	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-5s %7s %5s %3s %s\n", "Month", "AvgRet", "WR", "N", "Bias"))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 32) + "\n")

	for i := 0; i < 12; i++ {
		ms := p.Monthly[i]
		marker := " "
		if i+1 == p.CurrentMonth {
			marker = "\xe2\x96\xb6" // current month indicator
		}

		biasIcon := "\xc2\xb7"
		switch ms.Bias {
		case "BULLISH":
			biasIcon = "\xe2\x96\xb2"
		case "BEARISH":
			biasIcon = "\xe2\x96\xbc"
		}

		b.WriteString(fmt.Sprintf("%s%-4s %+6.2f%% %4.0f%% %2d  %s\n",
			marker, ms.Month, ms.AvgReturn, ms.WinRate, ms.SampleSize, biasIcon))
	}
	b.WriteString("</pre>\n")

	// Current month summary
	curMs := p.Monthly[p.CurrentMonth-1]
	biasEmoji := "\xe2\x9a\xaa" // white circle
	switch p.CurrentBias {
	case "BULLISH":
		biasEmoji = "\xF0\x9F\x9F\xA2"
	case "BEARISH":
		biasEmoji = "\xF0\x9F\x94\xB4"
	}
	b.WriteString(fmt.Sprintf("\n%s <b>Current Month (%s):</b> %s \xe2\x80\x94 Avg %+.2f%%, WR %.0f%% (n=%d)\n",
		biasEmoji, curMs.Month, p.CurrentBias, curMs.AvgReturn, curMs.WinRate, curMs.SampleSize))

	return b.String()
}

// FormatDailyPrice formats a DailyPriceContext for Telegram display.
func (f *Formatter) FormatDailyPrice(dc *domain.DailyPriceContext) string {
	var b strings.Builder

	// Header with price and daily change
	arrow := "→"
	if dc.DailyChgPct > 0 {
		arrow = "▲"
	} else if dc.DailyChgPct < 0 {
		arrow = "▼"
	}

	b.WriteString(fmt.Sprintf("💹 <b>%s — %s %s</b>\n\n",
		dc.Currency, formatDailyPrice(dc.CurrentPrice, dc.Currency), arrow))

	// Change section
	b.WriteString("<b>📊 Price Changes</b>\n")
	b.WriteString(fmt.Sprintf("<code>Daily  : %+.2f%%</code>\n", dc.DailyChgPct))
	b.WriteString(fmt.Sprintf("<code>5-Day  : %+.2f%%</code>\n", dc.WeeklyChgPct))
	b.WriteString(fmt.Sprintf("<code>20-Day : %+.2f%%</code>\n", dc.MonthlyChgPct))

	// Consecutive days
	if dc.ConsecDays >= 2 {
		dirEmoji := "📈"
		if dc.ConsecDir == "DOWN" {
			dirEmoji = "📉"
		}
		b.WriteString(fmt.Sprintf("<code>Streak : %d days %s</code> %s\n", dc.ConsecDays, dc.ConsecDir, dirEmoji))
	}

	// Moving Averages
	b.WriteString("\n<b>📐 Moving Averages</b>\n")

	maStatus := func(price, ma float64, label string) string {
		if ma == 0 {
			return fmt.Sprintf("<code>%s: N/A</code>", label)
		}
		icon := "✅"
		pos := "above"
		if price < ma {
			icon = "❌"
			pos = "below"
		}
		return fmt.Sprintf("<code>%s: %s</code> %s (%s)", label, formatDailyPrice(ma, dc.Currency), icon, pos)
	}

	b.WriteString(maStatus(dc.CurrentPrice, dc.DMA20, "20 DMA ") + "\n")
	b.WriteString(maStatus(dc.CurrentPrice, dc.DMA50, "50 DMA ") + "\n")
	b.WriteString(maStatus(dc.CurrentPrice, dc.DMA200, "200 DMA") + "\n")

	// MA Trend alignment
	maTrend := dc.MATrendDaily()
	trendEmoji := "⚪"
	switch maTrend {
	case "BULLISH":
		trendEmoji = "🟢"
	case "BEARISH":
		trendEmoji = "🔴"
	}
	b.WriteString(fmt.Sprintf("<code>Alignment: %s</code> %s\n", maTrend, trendEmoji))

	// Volatility
	if dc.DailyATR > 0 {
		b.WriteString("\n<b>📏 Volatility</b>\n")
		b.WriteString(fmt.Sprintf("<code>Daily ATR : %s (%.2f%%)</code>\n",
			formatDailyPrice(dc.DailyATR, dc.Currency), dc.NormalizedATR))
	}

	// Momentum
	b.WriteString("\n<b>🚀 Momentum</b>\n")
	b.WriteString(fmt.Sprintf("<code>5D  ROC: %+.2f%%</code>\n", dc.Momentum5D))
	b.WriteString(fmt.Sprintf("<code>10D ROC: %+.2f%%</code>\n", dc.Momentum10D))
	b.WriteString(fmt.Sprintf("<code>20D ROC: %+.2f%%</code>\n", dc.Momentum20D))

	// Daily trend
	trendIcon := "➡️"
	switch dc.DailyTrend {
	case "UP":
		trendIcon = "📈"
	case "DOWN":
		trendIcon = "📉"
	}
	b.WriteString(fmt.Sprintf("\n<code>Trend: %s</code> %s\n", dc.DailyTrend, trendIcon))

	return b.String()
}

// formatDailyPrice is a local helper for FormatDailyPrice formatting.
func formatDailyPrice(price float64, currency string) string {
	switch {
	case currency == "JPY":
		return fmt.Sprintf("%.3f", price)
	case currency == "XAU" || currency == "XAG":
		return fmt.Sprintf("%.2f", price)
	case currency == "BTC" || currency == "ETH":
		return fmt.Sprintf("%.0f", price)
	case currency == "OIL" || currency == "COPPER":
		return fmt.Sprintf("%.2f", price)
	case strings.HasPrefix(currency, "BOND") || currency == "SPX500" || currency == "NDX" || currency == "DJI" || currency == "RUT":
		return fmt.Sprintf("%.2f", price)
	default:
		if price > 10 {
			return fmt.Sprintf("%.4f", price)
		}
		return fmt.Sprintf("%.5f", price)
	}
}

// FormatDailyMomentumSnapshot formats a compact daily momentum view for /rank.
func (f *Formatter) FormatDailyMomentumSnapshot(dailyCtxs map[string]*domain.DailyPriceContext) string {
	if len(dailyCtxs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n📈 <b>Daily Momentum</b>\n<pre>")
	b.WriteString("Pair   Day%   5D%    MA   Strk\n")
	b.WriteString("─────────────────────────────\n")

	// Sort by daily change descending
	type entry struct {
		currency string
		dc       *domain.DailyPriceContext
	}
	var entries []entry
	for _, dc := range dailyCtxs {
		entries = append(entries, entry{dc.Currency, dc})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].dc.DailyChgPct > entries[j].dc.DailyChgPct
	})

	for _, e := range entries {
		dc := e.dc
		// Skip non-core instruments for compact view
		if strings.HasPrefix(e.currency, "BOND") || e.currency == "ULSD" || e.currency == "RBOB" {
			continue
		}

		maTrend := dc.MATrendDaily()
		maIcon := "·"
		switch maTrend {
		case "BULLISH":
			maIcon = "▲"
		case "BEARISH":
			maIcon = "▼"
		}

		streak := "  "
		if dc.ConsecDays >= 2 {
			dir := "↑"
			if dc.ConsecDir == "DOWN" {
				dir = "↓"
			}
			streak = fmt.Sprintf("%d%s", dc.ConsecDays, dir)
		}

		b.WriteString(fmt.Sprintf("%-6s %+5.1f%% %+5.1f%%  %s   %s\n",
			dc.Currency, dc.DailyChgPct, dc.WeeklyChgPct, maIcon, streak))
	}
	b.WriteString("</pre>")

	return b.String()
}

// FormatExcursionSummary formats MFE/MAE analysis results.
func (f *Formatter) FormatExcursionSummary(s *backtestsvc.ExcursionSummary) string {
	var b strings.Builder

	b.WriteString("📊 <b>MFE/MAE EXCURSION ANALYSIS</b>\n")
	b.WriteString(fmt.Sprintf("<code>Signals Analyzed: %d</code>\n\n", s.TotalSignals))

	b.WriteString("<b>📏 Average Excursion</b>\n")
	b.WriteString(fmt.Sprintf("<code>Avg MFE : %+.2f%%</code> (max favorable move)\n", s.AvgMFEPct))
	b.WriteString(fmt.Sprintf("<code>Avg MAE : %+.2f%%</code> (max adverse move)\n", s.AvgMAEPct))
	b.WriteString(fmt.Sprintf("<code>Avg Optimal Return: %+.2f%%</code> (exit at best day)\n", s.AvgOptimalRet))
	b.WriteString(fmt.Sprintf("<code>Avg Optimal Day   : %.1f</code>\n", s.AvgOptimalDay))

	b.WriteString("\n<b>🎯 Signal Quality Diagnosis</b>\n")
	if s.MissedWins > 0 {
		b.WriteString(fmt.Sprintf("🔴 <b>%d missed wins</b> (%.0f%% of losses)\n", s.MissedWins, s.MissedWinPct))
		b.WriteString("<i>These signals were profitable intraweek but closed as losses.\n")
		b.WriteString("→ Signals are directionally correct, exit timing needs work.</i>\n")
	} else {
		b.WriteString("✅ No significant missed wins detected.\n")
	}

	b.WriteString("\n<b>📅 Best Exit Day Distribution</b>\n<pre>")
	for i := 0; i < len(s.OptimalDayDist); i++ {
		label := fmt.Sprintf("Day %d", i+1)
		bar := ""
		for j := 0; j < s.OptimalDayDist[i] && j < 20; j++ {
			bar += "█"
		}
		b.WriteString(fmt.Sprintf("%-5s %3d %s\n", label, s.OptimalDayDist[i], bar))
	}
	b.WriteString("</pre>")

	if len(s.BySignalType) > 0 {
		b.WriteString("\n<b>📋 By Signal Type</b>\n<pre>")
		b.WriteString("Type            MFE%  MAE%  MfWR% Day\n")
		b.WriteString("────────────────────────────────────\n")

		type typeEntry struct {
			name string
			ts   *backtestsvc.ExcursionTypeSummary
		}
		var entries []typeEntry
		for name, ts := range s.BySignalType {
			entries = append(entries, typeEntry{name, ts})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ts.MFEWinRate > entries[j].ts.MFEWinRate
		})

		for _, e := range entries {
			ts := e.ts
			shortName := e.name
			if len(shortName) > 15 {
				shortName = shortName[:15]
			}
			b.WriteString(fmt.Sprintf("%-15s %+5.1f %+5.1f %5.0f %3.0f\n",
				shortName, ts.AvgMFEPct, ts.AvgMAEPct, ts.MFEWinRate, ts.AvgOptimalDay))
		}
		b.WriteString("</pre>")
		b.WriteString("<i>MfWR = MFE Win Rate (% that moved >0.3% in signal direction)</i>\n")
	}

	return b.String()
}

// FormatTrendFilterStats formats daily trend filter analysis results.
func (f *Formatter) FormatTrendFilterStats(s *backtestsvc.TrendFilterStats) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x88 <b>DAILY TREND FILTER ANALYSIS</b>\n\n")

	if s.TotalSignals == 0 {
		b.WriteString("<i>No evaluated signals with daily trend data yet.</i>")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("<code>Total Signals :</code> %d\n", s.TotalSignals))
	b.WriteString(fmt.Sprintf("<code>With Filter   :</code> %d (%.0f%%)\n",
		s.FilteredSignals, float64(s.FilteredSignals)/float64(s.TotalSignals)*100))
	b.WriteString(fmt.Sprintf("<code>Avg Adjustment:</code> %+.1f%%\n\n", s.AvgAdjustment))

	// Alignment breakdown
	b.WriteString("<b>Trend Alignment vs Win Rate</b>\n")
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-10s %5s %7s\n", "Category", "Count", "Win 1W"))
	b.WriteString(fmt.Sprintf("%-10s %5d %6.1f%%\n", "Aligned", s.AlignedCount, s.AlignedWinRate1W))
	b.WriteString(fmt.Sprintf("%-10s %5d %6.1f%%\n", "Opposed", s.OpposedCount, s.OpposedWinRate1W))
	b.WriteString(fmt.Sprintf("%-10s %5d %6.1f%%\n", "Neutral", s.NeutralCount, s.NeutralWinRate1W))
	b.WriteString("</pre>\n")

	// Edge diagnosis
	edgeIcon := "\xE2\x9C\x85"
	if s.EdgeGain <= 0 {
		edgeIcon = "\xE2\x9A\xA0\xEF\xB8\x8F"
	}
	b.WriteString("<b>Edge Analysis</b>\n")
	b.WriteString(fmt.Sprintf("<code>Baseline 1W  :</code> %.1f%%\n", s.BaselineWinRate1W))
	b.WriteString(fmt.Sprintf("<code>Filtered Top :</code> %.1f%% (adj \xE2\x89\xA5 10)\n", s.FilteredWinRate1W))
	b.WriteString(fmt.Sprintf("<code>Edge Gain    :</code> %+.1f%% %s\n\n", s.EdgeGain, edgeIcon))

	// Confidence calibration
	b.WriteString("<b>Confidence Impact</b>\n")
	b.WriteString(fmt.Sprintf("<code>Avg Raw      :</code> %.1f%%\n", s.AvgRawConfidence))
	b.WriteString(fmt.Sprintf("<code>Avg Adjusted :</code> %.1f%%\n\n", s.AvgFinalConfidence))

	// By daily trend
	trends := s.SortedTrends()
	if len(trends) > 0 {
		b.WriteString("<b>By Daily Trend</b>\n")
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-6s %5s %7s %7s\n", "Trend", "Count", "Win 1W", "AvgAdj"))
		for _, t := range trends {
			b.WriteString(fmt.Sprintf("%-6s %5d %6.1f%% %+5.1f%%\n",
				t.Trend, t.Count, t.WinRate, t.AvgAdj))
		}
		b.WriteString("</pre>")
	}

	// Interpretation
	b.WriteString("\n<i>Aligned = daily trend confirms COT signal direction\n")
	b.WriteString("Opposed = daily trend contradicts COT signal\n")
	b.WriteString("Edge Gain = win rate improvement from filtering</i>")

	return b.String()
}

// FormatLevels formats support/resistance levels and pivot points.
func (f *Formatter) FormatLevels(lc *pricesvc.LevelsContext, currency string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8F <b>KEY LEVELS: %s</b>\n\n", currency))

	b.WriteString(fmt.Sprintf("<code>Price    :</code> %s\n", formatPrice(lc.CurrentPrice, currency)))
	if lc.DailyATR > 0 {
		b.WriteString(fmt.Sprintf("<code>Daily ATR:</code> %s (%.2f%%)\n\n",
			formatPrice(lc.DailyATR, currency),
			lc.DailyATR/lc.CurrentPrice*100))
	}

	// Pivot points
	b.WriteString("<b>Daily Pivots</b>\n")
	b.WriteString(fmt.Sprintf("<code>R2    :</code> %s\n", formatPrice(lc.PivotR2, currency)))
	b.WriteString(fmt.Sprintf("<code>R1    :</code> %s\n", formatPrice(lc.PivotR1, currency)))
	b.WriteString(fmt.Sprintf("<code>Pivot :</code> %s\n", formatPrice(lc.DailyPivot, currency)))
	b.WriteString(fmt.Sprintf("<code>S1    :</code> %s\n", formatPrice(lc.PivotS1, currency)))
	b.WriteString(fmt.Sprintf("<code>S2    :</code> %s\n\n", formatPrice(lc.PivotS2, currency)))

	// Key S/R levels (top 10 by proximity)
	maxLevels := 10
	if len(lc.Levels) < maxLevels {
		maxLevels = len(lc.Levels)
	}

	if maxLevels > 0 {
		b.WriteString("<b>Support / Resistance</b>\n")
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-12s %-5s %7s %s\n", "Level", "Type", "Dist", "Source"))
		for i := 0; i < maxLevels; i++ {
			l := lc.Levels[i]
			typeIcon := "S"
			if l.Type == "RESISTANCE" {
				typeIcon = "R"
			}
			stars := strings.Repeat("*", l.Strength)
			b.WriteString(fmt.Sprintf("%-12s %-5s %+6.2f%% %s\n",
				formatPrice(l.Price, currency), typeIcon+stars, l.Distance, l.Source))
		}
		b.WriteString("</pre>\n")
	}

	// Nearest S/R summary
	if lc.NearestSupport != nil {
		b.WriteString(fmt.Sprintf("\xF0\x9F\x9F\xA2 <b>Nearest Support:</b> %s (%+.2f%%) — %s\n",
			formatPrice(lc.NearestSupport.Price, currency),
			lc.NearestSupport.Distance,
			lc.NearestSupport.Source))
	}
	if lc.NearestResistance != nil {
		b.WriteString(fmt.Sprintf("\xF0\x9F\x94\xB4 <b>Nearest Resistance:</b> %s (%+.2f%%) — %s\n",
			formatPrice(lc.NearestResistance.Price, currency),
			lc.NearestResistance.Distance,
			lc.NearestResistance.Source))
	}

	return b.String()
}
