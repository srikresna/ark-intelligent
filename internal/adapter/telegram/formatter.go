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
	"github.com/arkcode369/ark-intelligent/pkg/format"
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

// directionArrow checks if Actual beats Forecast using numeric comparison,
// respecting ImpactDirection from MQL5 so inverted indicators (unemployment,
// CPI miss, trade deficit) show the correct color for the currency.
//
// impactDirection semantics (MQL5):
//   0 = neutral/unknown  → fall back to raw numeric comparison
//   1 = higher actual is bullish for the currency (e.g. NFP, GDP)
//   2 = higher actual is bearish for the currency (e.g. Unemployment Claims, CPI when above target)
func directionArrow(actual, forecast string, impactDirection ...int) string {
	if actual == "" || forecast == "" {
		return "⚪"
	}
	aVal := parseNumeric(actual)
	fVal := parseNumeric(forecast)
	if aVal == nil || fVal == nil {
		return "⚪"
	}

	diff := *aVal - *fVal

	// Determine effective direction using ImpactDirection when provided
	dir := 0
	if len(impactDirection) > 0 {
		dir = impactDirection[0]
	}

	var effectiveDiff float64
	switch dir {
	case 1:
		// Higher actual = bullish for currency (normal indicator)
		effectiveDiff = diff
	case 2:
		// Higher actual = bearish for currency (inverted indicator: unemployment, deficits, etc.)
		effectiveDiff = -diff
	default:
		// Unknown direction: use raw diff
		effectiveDiff = diff
	}

	if effectiveDiff > 0 {
		return "🟢"
	} else if effectiveDiff < 0 {
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
			timeDisplay = fmtutil.UpdatedAtShort(e.TimeWIB)
		}

		b.WriteString(fmt.Sprintf("%s <b>%s - %s</b>\n", e.FormatImpactColor(), timeDisplay, e.Currency))
		b.WriteString(fmt.Sprintf("↳ <i>%s</i>\n", e.Event))

		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast, e.ImpactDirection)
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
			timeDisplay = fmtutil.UpdatedAtShort(e.TimeWIB)
		}

		line := fmt.Sprintf("%s %s %s: <i>%s</i>", e.FormatImpactColor(), timeDisplay, e.Currency, e.Event)
		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast, e.ImpactDirection)
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
			timeDisplay = fmtutil.UpdatedAtShort(e.TimeWIB)
		}

		line := fmt.Sprintf("%s %s %s: <i>%s</i>", e.FormatImpactColor(), timeDisplay, e.Currency, e.Event)
		if e.Actual != "" {
			arrow := directionArrow(e.Actual, e.Forecast, e.ImpactDirection)
			line += fmt.Sprintf(" — ✅<b>%s</b>%s", e.Actual, arrow)
		}
		b.WriteString(line + "\n")
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// COT Formatting
// ---------------------------------------------------------------------------

// cotGroup defines the display groups for COT overview.
type cotGroup struct {
	Header    string
	Emoji     string
	Codes     []string // contract codes in preferred order
}

// cotGroups defines the canonical grouping and order for COT overview display.
var cotGroups = []cotGroup{
	{
		Header: "FOREX MAJORS",
		Emoji:  "🌍",
		Codes:  []string{"098662", "099741", "096742", "097741", "232741", "112741", "090741", "092741"},
	},
	{
		Header: "EQUITY INDICES",
		Emoji:  "📈",
		Codes:  []string{"13874A", "209742", "124601", "239742"},
	},
	{
		Header: "COMMODITIES",
		Emoji:  "🏅",
		Codes:  []string{"088691", "084691", "085692", "067651", "022651", "111659"},
	},
	{
		Header: "BONDS",
		Emoji:  "📊",
		Codes:  []string{"042601", "044601", "043602", "020601"},
	},
	{
		Header: "CRYPTO",
		Emoji:  "₿",
		Codes:  []string{"133741", "146021"},
	},
}

// cotIdxLabel returns a short label for a COT Index value.
func cotIdxLabel(idx float64) string {
	switch {
	case idx >= 80:
		return "X.Long"
	case idx >= 60:
		return "Bullish"
	case idx >= 40:
		return "Neutral"
	case idx >= 20:
		return "Bearish"
	default:
		return "X.Short"
	}
}

// convictionMiniBar renders a small conviction bar: e.g. "▓▓▓░░ 62"
func convictionMiniBar(score float64, dir string) string {
	filled := int(score / 20) // 0-5 blocks
	if filled > 5 {
		filled = 5
	}
	bar := strings.Repeat("▓", filled) + strings.Repeat("░", 5-filled)
	icon := "⚪"
	switch {
	case score >= 65 && dir == "LONG":
		icon = "🟢"
	case score >= 65 && dir == "SHORT":
		icon = "🔴"
	case score >= 55:
		icon = "🟡"
	}
	return fmt.Sprintf("%s[%s]%.0f", icon, bar, score)
}

// FormatCOTOverview formats a grouped, sorted summary of all COT analyses.
// convictions may be nil — conviction column is hidden gracefully.
func (f *Formatter) FormatCOTOverview(analyses []domain.COTAnalysis, convictions []cot.ConvictionScore) string {
	var b strings.Builder

	// Build lookup maps
	byCode := make(map[string]domain.COTAnalysis, len(analyses))
	for _, a := range analyses {
		byCode[a.Contract.Code] = a
	}
	convMap := make(map[string]cot.ConvictionScore, len(convictions))
	for _, cs := range convictions {
		convMap[cs.Currency] = cs
	}
	shown := make(map[string]bool)

	b.WriteString("📋 <b>COT POSITIONING OVERVIEW</b>\n")
	if len(analyses) > 0 {
		b.WriteString(fmt.Sprintf("<i>Report: %s</i>\n", analyses[0].ReportDate.Format("Jan 2, 2006")))
	}
	hasConv := len(convictions) > 0
	if hasConv {
		b.WriteString("<i>Conv = Conviction Score (COT+FRED+Price)</i>\n")
	}
	b.WriteString("\n")

	for _, grp := range cotGroups {
		// Collect analyses for this group
		var grpAnalyses []domain.COTAnalysis
		for _, code := range grp.Codes {
			if a, ok := byCode[code]; ok {
				grpAnalyses = append(grpAnalyses, a)
				shown[code] = true
			}
		}
		if len(grpAnalyses) == 0 {
			continue
		}

		// Sort within group by COT Index descending (strongest conviction first)
		sort.Slice(grpAnalyses, func(i, j int) bool {
			// Use conviction score if available, otherwise COT Index
			ci, ciOk := convMap[grpAnalyses[i].Contract.Currency]
			cj, cjOk := convMap[grpAnalyses[j].Contract.Currency]
			if ciOk && cjOk {
				return ci.Score > cj.Score
			}
			return grpAnalyses[i].COTIndex > grpAnalyses[j].COTIndex
		})

		b.WriteString(fmt.Sprintf("%s <b>%s</b>\n", grp.Emoji, grp.Header))

		for _, a := range grpAnalyses {
			bias := "NEUTRAL"
			biasIcon := "⚪"
			if a.NetPosition > 0 {
				bias = "LONG"
				biasIcon = "🟢"
			} else if a.NetPosition < 0 {
				bias = "SHORT"
				biasIcon = "🔴"
			}

			idxLbl := cotIdxLabel(a.COTIndex)

			// Line 1: name + bias
			b.WriteString(fmt.Sprintf("%s <b>%s</b> %s\n", biasIcon, a.Contract.Name, bias))

			// Line 2: Net | Idx | Conv (if available)
			if cs, ok := convMap[a.Contract.Currency]; ok {
				b.WriteString(fmt.Sprintf("<code>  Net:%-10s Idx:%.0f%% (%s)</code>\n",
					format.FormatInt(int64(a.NetPosition)), a.COTIndex, idxLbl))
				b.WriteString(fmt.Sprintf("<code>  Chg:%-10s Mom:%-10s Conv:%s</code>\n",
					fmtutil.FmtNumSigned(a.NetChange, 0),
					f.momentumLabel(a.MomentumDir),
					convictionMiniBar(cs.Score, cs.Direction)))
			} else {
				b.WriteString(fmt.Sprintf("<code>  Net:%-10s Idx:%.0f%% (%s)</code>\n",
					format.FormatInt(int64(a.NetPosition)), a.COTIndex, idxLbl))
				b.WriteString(fmt.Sprintf("<code>  Chg:%-10s Mom:%s</code>\n",
					fmtutil.FmtNumSigned(a.NetChange, 0),
					f.momentumLabel(a.MomentumDir)))
			}
			b.WriteString("\n")
		}
	}

	// Catch-all: any analyses not in a group (future contracts)
	var ungrouped []domain.COTAnalysis
	for _, a := range analyses {
		if !shown[a.Contract.Code] {
			ungrouped = append(ungrouped, a)
		}
	}
	if len(ungrouped) > 0 {
		b.WriteString("📌 <b>OTHER</b>\n")
		for _, a := range ungrouped {
			bias := "NEUTRAL"
			if a.NetPosition > 0 {
				bias = "LONG"
			} else if a.NetPosition < 0 {
				bias = "SHORT"
			}
			b.WriteString(fmt.Sprintf("<b>%s</b> %s\n", a.Contract.Name, bias))
			b.WriteString(fmt.Sprintf("<code>  Net: %s | Idx: %.0f%%</code>\n\n",
				format.FormatInt(int64(a.NetPosition)), a.COTIndex))
		}
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
		if rt == "DISAGGREGATED" {
			// Untuk komoditas, divergence antara Managed Money dan Prod/Swap adalah NORMAL
			// karena produsen selalu hedge (net short) sementara spekulan beli
			b.WriteString("🔀 <b>Divergence:</b> Spekulan vs produsen posisi berlawanan\n")
			b.WriteString("<i>  ℹ️ Untuk komoditas, ini NORMAL — produsen biasanya selalu hedge net short</i>\n")
		} else {
			b.WriteString("🔀 <b>Divergence:</b> Smart money vs commercials moving opposite\n")
		}
	}
	if a.CommExtremeBull {
		b.WriteString("🟢 <b>Commercial COT Extreme LONG</b> (contrarian bullish signal)\n")
	}
	if a.CommExtremeBear {
		b.WriteString("🔴 <b>Commercial COT Extreme SHORT</b> (contrarian bearish signal)\n")
	}
	if a.CategoryDivergence {
		b.WriteString(fmt.Sprintf("⚡ <b>Category Divergence:</b> %s\n", a.CategoryDivergenceDesc))
	}
	if a.AssetMgrAlert || a.ThinMarketAlert || a.SmartDumbDivergence || a.CommExtremeBull || a.CommExtremeBear || a.CategoryDivergence {
		b.WriteString("\n")
	}

	// Category Z-Score Breakdown (show if any alert or divergence exists)
	hasZAlert := a.DealerAlert || a.LevFundAlert || a.ManagedMoneyAlert || a.SwapDealerAlert || a.CategoryDivergence
	if hasZAlert {
		b.WriteString("📊 <b>Category Z-Scores (WoW Change vs 52W):</b>\n")
		zScoreEmoji := func(z float64, alert bool) string {
			if !alert {
				return "  "
			}
			if z > 0 {
				return "🟢"
			}
			return "🔴"
		}
		if rt == "TFF" {
			b.WriteString(fmt.Sprintf("<code>  Dealer:       %+.2fσ %s</code>\n", a.DealerZScore, zScoreEmoji(a.DealerZScore, a.DealerAlert)))
			b.WriteString(fmt.Sprintf("<code>  LevFund:      %+.2fσ %s</code>\n", a.LevFundZScore, zScoreEmoji(a.LevFundZScore, a.LevFundAlert)))
			b.WriteString(fmt.Sprintf("<code>  AssetMgr:     %+.2fσ %s</code>\n", a.AssetMgrZScore, zScoreEmoji(a.AssetMgrZScore, a.AssetMgrAlert)))
			b.WriteString(fmt.Sprintf("<code>  ManagedMoney: %+.2fσ %s</code>\n", a.ManagedMoneyZScore, zScoreEmoji(a.ManagedMoneyZScore, a.ManagedMoneyAlert)))
		} else {
			// DISAGGREGATED: SwapDealer and ManagedMoney are primary
			b.WriteString(fmt.Sprintf("<code>  ManagedMoney: %+.2fσ %s</code>\n", a.ManagedMoneyZScore, zScoreEmoji(a.ManagedMoneyZScore, a.ManagedMoneyAlert)))
			b.WriteString(fmt.Sprintf("<code>  SwapDealer:   %+.2fσ %s</code>\n", a.SwapDealerZScore, zScoreEmoji(a.SwapDealerZScore, a.SwapDealerAlert)))
			b.WriteString(fmt.Sprintf("<code>  LevFund:      %+.2fσ %s</code>\n", a.LevFundZScore, zScoreEmoji(a.LevFundZScore, a.LevFundAlert)))
		}
		b.WriteString(fmt.Sprintf("<i>  Alert threshold: |z| ≥ 2.0σ  |  max |z|: %.2f</i>\n",
			math.Max(math.Max(math.Abs(a.DealerZScore), math.Abs(a.LevFundZScore)),
				math.Max(math.Abs(a.ManagedMoneyZScore), math.Abs(a.SwapDealerZScore)))))
		b.WriteString("\n")
	}

	// Positioning
	b.WriteString(fmt.Sprintf("<b>%s (Smart Money):</b>\n", smartMoneyLabel))
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", format.FormatNetPosition(int64(a.NetPosition))))
	b.WriteString(fmt.Sprintf("<code>  Net Change:     %s</code>\n", fmtutil.FmtNumSigned(a.NetChange, 0)))
	if a.LongShortRatio >= 999 {
		b.WriteString("<code>  L/S Ratio:      ∞ (no shorts reported)</code>\n")
	} else if a.LongShortRatio == 0 {
		b.WriteString("<code>  L/S Ratio:      N/A (no positions)</code>\n")
	} else {
		b.WriteString(fmt.Sprintf("<code>  L/S Ratio:      %.2f</code>\n", a.LongShortRatio))
	}
	b.WriteString(fmt.Sprintf("<code>  Net as %% OI:    %.1f%%</code>\n", a.PctOfOI))

	b.WriteString(fmt.Sprintf("\n<b>%s:</b>\n", hedgerLabel))
	b.WriteString(fmt.Sprintf("<code>  Net Position:   %s</code>\n", fmtutil.FmtNumSigned(a.CommercialNet, 0)))
	b.WriteString(fmt.Sprintf("<code>  Comm %% OI:      %.1f%%</code>\n", a.CommPctOfOI))
	b.WriteString(fmt.Sprintf("<code>  COT Index:      %.1f%%</code>\n", a.COTIndexComm))
	b.WriteString(fmt.Sprintf("<code>  Signal:         %s</code>\n", commercialSignalLabel(a.CommercialSignal, rt)))

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

	// Smart Money vs Commercial Signal Confluence
	b.WriteString("\n<b>Signal Confluence:</b>\n")
	b.WriteString(fmt.Sprintf("<code>  Smart Money:    %s</code>\n", a.SpeculatorSignal))
	b.WriteString(fmt.Sprintf("<code>  %s:   %s</code>\n", hedgerLabel, commercialSignalLabel(a.CommercialSignal, rt)))
	b.WriteString(signalConfluenceInterpretation(a.SpeculatorSignal, a.CommercialSignal, rt))

	// Quick copy commands — prefer currency code (e.g. GOLD, EUR) over contract code
	if displayCode != "" {
		// Map known contract codes back to friendly currency shortcuts
		friendlyCode := contractCodeToFriendly(displayCode)
		b.WriteString(fmt.Sprintf("\n<i>Quick commands:</i>\n<code>/cot %s</code> | <code>/cot raw %s</code>", friendlyCode, friendlyCode))
	}

	return b.String()
}

// FormatCOTRaw formats raw CFTC data with plain-language explanations and calculated metrics.
func (f *Formatter) FormatCOTRaw(r domain.COTRecord) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("📊 <b>DATA MENTAH COT: %s</b>\n", r.ContractName))
	b.WriteString(fmt.Sprintf("<i>Laporan: %s | Sumber: CFTC (resmi)</i>\n", r.ReportDate.Format("Jan 2, 2006")))
	b.WriteString("<i>Data ini adalah angka posisi asli sebelum dikalkulasi</i>\n\n")

	// Open Interest
	b.WriteString("📌 <b>OPEN INTEREST (Total Kontrak Aktif)</b>\n")
	b.WriteString(fmt.Sprintf("<code>  Total: %s kontrak</code>\n", fmtutil.FmtNum(r.OpenInterest, 0)))
	b.WriteString("<i>  → Semakin besar = semakin banyak uang yang aktif di pasar ini</i>\n\n")

	// Determine report type from contract code (reliable) instead of contract name (fragile).
	// Lookup DefaultCOTContracts to find the ReportType for this contract.
	isDisagg := false
	for _, c := range domain.DefaultCOTContracts {
		if c.Code == r.ContractCode {
			isDisagg = c.ReportType == "DISAGGREGATED"
			break
		}
	}
	// Fallback: if contract code not found, infer from field presence
	// (DISAGGREGATED records populate ManagedMoneyLong; TFF records populate LevFundLong)
	if r.ContractCode == "" {
		isDisagg = r.ManagedMoneyLong > 0 || r.ManagedMoneyShort > 0
	}

	if isDisagg {
		// ── DISAGGREGATED (Komoditas fisik: Gold, Oil, dll) ──
		mmLong := r.ManagedMoneyLong
		mmShort := r.ManagedMoneyShort
		mmNet := mmLong - mmShort
		var mmRatio float64
		if mmShort > 0 {
			mmRatio = mmLong / mmShort
		}
		mmNetIcon := "🟢"
		mmNetDesc := "NET BELI"
		if mmNet < 0 {
			mmNetIcon = "🔴"
			mmNetDesc = "NET JUAL"
		}

		b.WriteString("🧠 <b>MANAGED MONEY — Hedge Fund / Spekulan Besar</b>\n")
		b.WriteString("<i>  Siapa ini? Dana investasi besar yang mencari profit dari pergerakan harga</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(mmLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(mmShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s (%s)</code>\n",
			func() string {
				if mmNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(mmNet, 0), mmNetDesc))
		if mmShort > 0 || mmLong > 0 {
			if mmRatio >= 1 {
				b.WriteString(fmt.Sprintf("<code>  Rasio L/S   : %.2fx lebih banyak BELI dari jual</code>\n", mmRatio))
			} else if mmShort > 0 && mmLong > 0 {
				b.WriteString(fmt.Sprintf("<code>  Rasio L/S   : %.2fx lebih banyak JUAL dari beli</code>\n", mmShort/mmLong))
			} else if mmLong == 0 && mmShort > 0 {
				b.WriteString("<code>  Rasio L/S   : seluruhnya posisi JUAL</code>\n")
			} else if mmShort == 0 && mmLong > 0 {
				b.WriteString("<code>  Rasio L/S   : seluruhnya posisi BELI</code>\n")
			}
		}
		b.WriteString(fmt.Sprintf("<i>  → Spekulan sedang %s %s — mereka %s harga naik</i>\n\n",
			mmNetIcon, mmNetDesc,
			func() string {
				if mmNet > 0 {
					return "EKSPEKTASI"
				}
				return "TIDAK ekspektasi"
			}()))

		// Commercials (Prod/Swap)
		commLong := r.ProdMercLong + r.SwapDealerLong
		commShort := r.ProdMercShort + r.SwapDealerShort
		commNet := commLong - commShort
		commNetDesc := "net beli"
		if commNet < 0 {
			commNetDesc = "net jual (hedge)"
		}

		b.WriteString("🏭 <b>PROD/SWAP — Produsen & Korporasi</b>\n")
		b.WriteString("<i>  Siapa ini? Perusahaan tambang, kilang minyak, bank komoditas</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(commLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(commShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s (%s)</code>\n",
			func() string {
				if commNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(commNet, 0), commNetDesc))
		b.WriteString("<i>  → Produsen biasanya net SHORT untuk lindungi produksi mereka.\n")
		b.WriteString("     Ini NORMAL dan bukan sinyal bearish murni.</i>\n\n")

		// Perbandingan Specs vs Commercials
		b.WriteString("⚡ <b>BACAAN CEPAT</b>\n")
		if mmNet > 0 && commNet < 0 {
			b.WriteString("  ✅ Kondisi normal: spekulan beli, produsen hedge\n")
			if mmNet > 50000 {
				b.WriteString("  ⚠️ Spekulan sudah beli banyak — risiko pembalikan jika mereka mulai keluar\n")
			}
		} else if mmNet < 0 && commNet > 0 {
			b.WriteString("  🔀 Kondisi terbalik: spekulan jual, tapi produsen justru beli\n")
			b.WriteString("  → Ini sinyal langka — bisa jadi titik balik harga\n")
		} else if mmNet < 0 && commNet < 0 {
			b.WriteString("  🔴 Semua pihak net jual — tekanan turun signifikan\n")
		}

	} else {
		// ── TFF (Financial: Mata uang, Bonds, Indices) ──
		lfLong := r.LevFundLong
		lfShort := r.LevFundShort
		lfNet := lfLong - lfShort
		lfNetIcon := "🟢"
		lfNetDesc := "NET BELI"
		if lfNet < 0 {
			lfNetIcon = "🔴"
			lfNetDesc = "NET JUAL"
		}
		var lfRatio float64
		if lfShort > 0 {
			lfRatio = lfLong / lfShort
		}

		b.WriteString("⚡ <b>LEVERAGED FUNDS — Hedge Fund / CTA</b>\n")
		b.WriteString("<i>  Siapa ini? Dana spekulatif yang trading dengan leverage tinggi</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(lfLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(lfShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s (%s)</code>\n",
			func() string {
				if lfNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(lfNet, 0), lfNetDesc))
		if lfRatio > 0 && lfShort > 0 {
			dominant := "beli"
			ratio := lfRatio
			if lfRatio < 1 {
				dominant = "jual"
				ratio = lfShort / lfLong
			}
			b.WriteString(fmt.Sprintf("<code>  Rasio L/S   : %.2fx lebih banyak %s</code>\n", ratio, dominant))
		}
		b.WriteString(fmt.Sprintf("<i>  → %s Hedge fund sedang %s — ini sinyal paling penting untuk arah harga</i>\n\n",
			lfNetIcon, lfNetDesc))

		// Asset Manager
		amLong := r.AssetMgrLong
		amShort := r.AssetMgrShort
		amNet := amLong - amShort
		amNetDesc := "net beli"
		if amNet < 0 {
			amNetDesc = "net jual"
		}

		b.WriteString("🏦 <b>ASSET MANAGER — Dana Pensiun & Reksa Dana</b>\n")
		b.WriteString("<i>  Siapa ini? Dana pensiun, reksa dana, asuransi — uang jangka panjang</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(amLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(amShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s (%s)</code>\n",
			func() string {
				if amNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(amNet, 0), amNetDesc))
		b.WriteString("<i>  → Pergerakan Asset Manager lebih lambat tapi lebih sustained</i>\n\n")

		// Dealers
		dlrLong := r.DealerLong
		dlrShort := r.DealerShort
		dlrNet := dlrLong - dlrShort

		b.WriteString("🏛 <b>DEALERS — Bank Besar / Market Maker</b>\n")
		b.WriteString("<i>  Siapa ini? Bank investasi yang jadi perantara pasar</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Long (beli) : %s kontrak</code>\n", fmtutil.FmtNum(dlrLong, 0)))
		b.WriteString(fmt.Sprintf("<code>  Short (jual): %s kontrak</code>\n", fmtutil.FmtNum(dlrShort, 0)))
		b.WriteString(fmt.Sprintf("<code>  Net         : %s%s</code>\n",
			func() string {
				if dlrNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(dlrNet, 0)))
		b.WriteString("<i>  → Dealer biasanya posisi berlawanan dengan Lev Funds (mereka sisi lain transaksi)</i>\n\n")

		// Bacaan cepat
		b.WriteString("⚡ <b>BACAAN CEPAT</b>\n")
		if lfNet > 0 && amNet > 0 {
			b.WriteString("  🟢 Hedge fund DAN asset manager sama-sama beli — sinyal naik kuat\n")
			if dlrNet < -50000 {
				b.WriteString("  ⚠️ Tapi bank/dealer net jual besar — mereka di sisi berlawanan, waspadai reversal\n")
			}
		} else if lfNet < 0 && amNet < 0 {
			b.WriteString("  🔴 Hedge fund DAN asset manager sama-sama jual — sinyal turun kuat\n")
			if dlrNet > 50000 {
				b.WriteString("  ⚠️ Tapi bank/dealer net beli besar — mereka di sisi berlawanan, waspadai reversal\n")
			}
		} else if lfNet > 0 && amNet < 0 {
			b.WriteString("  🟡 Hedge fund beli tapi asset manager jual — sinyal campur\n")
		} else if lfNet < 0 && amNet > 0 {
			b.WriteString("  🟡 Hedge fund jual tapi asset manager beli — sinyal campur\n")
		}
	}

	// Trader depth — selalu tampil dengan penjelasan
	b.WriteString("\n👥 <b>KEDALAMAN PASAR (Jumlah Trader Aktif)</b>\n")
	if isDisagg {
		totalT := r.TotalTradersDisag
		if totalT > 0 {
			b.WriteString(fmt.Sprintf("<code>  Spekulan Long : %d trader</code>\n", r.MMoneyLongTraders))
			b.WriteString(fmt.Sprintf("<code>  Spekulan Short: %d trader</code>\n", r.MMoneyShortTraders))
			b.WriteString(fmt.Sprintf("<code>  Total Aktif   : %d trader</code>\n", totalT))
			if r.MMoneyLongTraders > 0 && r.MMoneyShortTraders > 0 {
				ratio := float64(r.MMoneyLongTraders) / float64(r.MMoneyShortTraders)
				b.WriteString(fmt.Sprintf("<i>  → Rasio trader: %.1fx lebih banyak yang beli vs jual</i>\n", ratio))
			}
			depthLabel := "sedang"
			depthDesc := "likuiditas normal"
			if totalT > 300 {
				depthLabel = "DEEP (dalam)"
				depthDesc = "likuiditas bagus, mudah masuk/keluar posisi"
			} else if totalT < 100 {
				depthLabel = "TIPIS"
				depthDesc = "hati-hati — pasar tipis, slippage bisa besar"
			}
			b.WriteString(fmt.Sprintf("<i>  → Pasar %s — %s</i>\n", depthLabel, depthDesc))
		}
	} else {
		totalT := r.TotalTraders
		if totalT > 0 {
			if r.LevFundLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Lev Fund Long : %d trader</code>\n", r.LevFundLongTraders))
			}
			if r.LevFundShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  Lev Fund Short: %d trader</code>\n", r.LevFundShortTraders))
			}
			if r.AssetMgrLongTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  AssetMgr Long : %d trader</code>\n", r.AssetMgrLongTraders))
			}
			if r.AssetMgrShortTraders > 0 {
				b.WriteString(fmt.Sprintf("<code>  AssetMgr Short: %d trader</code>\n", r.AssetMgrShortTraders))
			}
			b.WriteString(fmt.Sprintf("<code>  Total Aktif   : %d trader</code>\n", totalT))
			depthLabel := "sedang"
			depthDesc := "likuiditas normal"
			if totalT > 300 {
				depthLabel = "DEEP (dalam)"
				depthDesc = "likuiditas bagus"
			} else if totalT < 80 {
				depthLabel = "TIPIS"
				depthDesc = "hati-hati — pasar tipis"
			}
			b.WriteString(fmt.Sprintf("<i>  → Pasar %s — %s</i>\n", depthLabel, depthDesc))
		}
	}

	// Small Specs jika ada
	if r.SmallLong > 0 || r.SmallShort > 0 {
		smallNet := r.SmallLong - r.SmallShort
		b.WriteString("\n🐟 <b>SMALL SPECULATORS — Trader Retail Kecil</b>\n")
		b.WriteString(fmt.Sprintf("<code>  Long : %s | Short: %s | Net: %s%s</code>\n",
			fmtutil.FmtNum(r.SmallLong, 0),
			fmtutil.FmtNum(r.SmallShort, 0),
			func() string {
				if smallNet >= 0 {
					return "+"
				}
				return ""
			}(),
			fmtutil.FmtNum(smallNet, 0)))
		b.WriteString("<i>  → Retail trader — sering dianggap 'wrong-side' oleh institusi</i>\n")
	}

	b.WriteString("\n<i>📌 Data resmi dari CFTC, dirilis setiap Jumat untuk data Selasa sebelumnya</i>")
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

// contractCodeToFriendly maps CFTC numeric contract codes to user-friendly currency shortcuts.
// Returns the input unchanged if no mapping exists.
func contractCodeToFriendly(code string) string {
	m := map[string]string{
		"099741": "EUR",
		"096742": "GBP",
		"097741": "JPY",
		"092741": "CHF",
		"232741": "AUD",
		"090741": "CAD",
		"112741": "NZD",
		"098662": "USD",
		"088691": "GOLD",
		"084691": "SILVER",
		"085692": "COPPER",
		"067651": "OIL",
		"022651": "ULSD",
		"111659": "RBOB",
		"043602": "BOND10",
		"020601": "BOND30",
		"044601": "BOND5",
		"042601": "BOND2",
		"13874A": "SPX",
		"209742": "NDX",
		"124601": "DJI",
		"239742": "RUT",
		"133741": "BTC",
		"146021": "ETH",
	}
	if friendly, ok := m[code]; ok {
		return friendly
	}
	return code
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
	b.WriteString(fmt.Sprintf("<i>Week of %s | Based on COT Positioning</i>\n\n", fmtutil.FormatDateWIB(date)))

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
	Currency        string
	Score           float64
	COTIndex        float64
	Conviction      cot.ConvictionScore
	ThinMarketAlert bool
	ThinMarketDesc  string
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
			Currency:        a.Contract.Currency,
			Score:           a.SentimentScore,
			COTIndex:        a.COTIndex,
			Conviction:      cs,
			ThinMarketAlert: a.ThinMarketAlert,
			ThinMarketDesc:  a.ThinMarketDesc,
		})
	}

	// Sort by conviction score descending (highest conviction first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Conviction.Score > entries[j].Conviction.Score
	})

	var b strings.Builder
	b.WriteString("🏆 <b>CURRENCY STRENGTH RANKING</b>\n")
	b.WriteString(fmt.Sprintf("<i>COT + FRED Conviction — %s</i>\n", fmtutil.FormatDateWIB(date)))

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

		// Thin market warning flag
		thinFlag := ""
		if e.ThinMarketAlert {
			thinFlag = " ⚠️THIN"
		}

		// Data quality: show sources count
		srcLabel := ""
		if e.Conviction.SourcesAvailable > 0 && e.Conviction.SourcesAvailable < 4 {
			srcLabel = fmt.Sprintf(" (%d/4)", e.Conviction.SourcesAvailable)
		}

		b.WriteString(fmt.Sprintf("%s %s <b>%s</b>%s | Sent: %s%.0f | Conv: <b>%d/100</b>%s %s\n",
			medal, colorDot, e.Currency, thinFlag, sentSign, e.Score, convScore, srcLabel, convLabel))

		// Component breakdown for top 3 currencies
		if i < 3 && e.Conviction.Version == 3 {
			b.WriteString(fmt.Sprintf("   <i>COT:%+.0f Macro:%+.0f Price:%+.0f Cal:%+.0f</i>\n",
				e.Conviction.COTComponent, e.Conviction.MacroComponent,
				e.Conviction.PriceComponent, e.Conviction.CalendarComponent))
		}
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

// FormatConvictionBlock renders a detailed conviction score block for the /cot detail view.
// Uses plain language so non-finance users can immediately understand the signal.
func (f *Formatter) FormatConvictionBlock(cs cot.ConvictionScore) string {
	var b strings.Builder

	// Determine icon and plain-language verdict
	var icon string
	var verdict, explanation string
	score := cs.Score

	switch {
	case score >= 75 && cs.Direction == "LONG":
		icon = "🟢"
		verdict = "STRONG BUY SIGNAL"
		explanation = "Hampir semua indikator sepakat: harga kemungkinan besar naik."
	case score >= 65 && cs.Direction == "LONG":
		icon = "🟢"
		verdict = "BUY SIGNAL"
		explanation = "Mayoritas indikator menunjukkan potensi kenaikan harga."
	case score >= 55 && cs.Direction == "LONG":
		icon = "🟡"
		verdict = "LEMAH BUY"
		explanation = "Ada sinyal naik tapi belum cukup kuat. Lebih baik tunggu konfirmasi."
	case score >= 75 && cs.Direction == "SHORT":
		icon = "🔴"
		verdict = "STRONG SELL SIGNAL"
		explanation = "Hampir semua indikator sepakat: harga kemungkinan besar turun."
	case score >= 65 && cs.Direction == "SHORT":
		icon = "🔴"
		verdict = "SELL SIGNAL"
		explanation = "Mayoritas indikator menunjukkan potensi penurunan harga."
	case score >= 55 && cs.Direction == "SHORT":
		icon = "🟡"
		verdict = "LEMAH SELL"
		explanation = "Ada sinyal turun tapi belum kuat. Perlu konfirmasi lebih lanjut."
	default:
		icon = "⚪"
		verdict = "NETRAL / TIDAK JELAS"
		explanation = "Indikator saling bertentangan. Tidak ada sinyal yang cukup jelas saat ini."
	}

	// Build conviction bar: 10 blocks
	filled := int(score / 10)
	if filled > 10 {
		filled = 10
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)

	b.WriteString("\n🎯 <b>KESIMPULAN SINYAL</b>\n")
	b.WriteString(fmt.Sprintf("<code>[%s] %.0f/100</code>\n", bar, score))
	b.WriteString(fmt.Sprintf("%s <b>%s</b>\n", icon, verdict))
	b.WriteString(fmt.Sprintf("<i>%s</i>\n", explanation))

	// Component breakdown — plain language
	b.WriteString("\n<b>Komponen Penilaian:</b>\n")

	cotIcon := "⚪"
	cotDesc := "Netral"
	switch cs.COTBias {
	case "BULLISH":
		cotIcon = "🟢"
		cotDesc = "Institusi besar sedang beli (bullish)"
	case "BEARISH":
		cotIcon = "🔴"
		cotDesc = "Institusi besar sedang jual (bearish)"
	}
	b.WriteString(fmt.Sprintf("<code>  COT Positioning : </code>%s %s\n", cotIcon, cotDesc))

	fredIcon := "⚪"
	fredDesc := "Kondisi makro netral"
	switch cs.FREDRegime {
	case "GOLDILOCKS":
		fredIcon = "🟢"
		fredDesc = "Ekonomi AS sehat, risk-on (GOLDILOCKS)"
	case "DISINFLATIONARY":
		fredIcon = "🟢"
		fredDesc = "Inflasi mereda, kondisi positif (DISINFLATIONARY)"
	case "INFLATIONARY":
		fredIcon = "🟡"
		fredDesc = "Inflasi masih tinggi, hati-hati (INFLATIONARY)"
	case "STRESS":
		fredIcon = "🔴"
		fredDesc = "Pasar dalam tekanan/stres (STRESS)"
	case "RECESSION":
		fredIcon = "🔴"
		fredDesc = "Risiko resesi tinggi (RECESSION)"
	case "STAGFLATION":
		fredIcon = "🔴"
		fredDesc = "Stagflasi: inflasi tinggi + ekonomi lemah"
	}
	b.WriteString(fmt.Sprintf("<code>  Kondisi Ekonomi  : </code>%s %s\n", fredIcon, fredDesc))

	b.WriteString("<i>  Data: Harga (30%) + COT (25%) + Ekonomi (20%) + Kalender (15%) + Stres (10%)</i>\n")

	return b.String()
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

// commercialSignalLabel returns a display string for the commercial signal,
// noting whether it's contrarian or structural depending on report type.
func commercialSignalLabel(signal, rt string) string {
	// For TFF (forex/indices): Dealers are the "dumb money" market maker side —
	// their signal is LESS reliable as a contrarian indicator (unlike classic commercials).
	// For DISAGGREGATED (commodities): Prod/Swap are true producers = contrarian smart.
	suffix := ""
	if rt == "TFF" {
		suffix = " (dealer)"
	} else if rt == "DISAGGREGATED" {
		suffix = " (contrarian)"
	}
	return signal + suffix
}

// signalConfluenceInterpretation returns a plain-language interpretation
// of the combined Smart Money + Commercial signal alignment.
func signalConfluenceInterpretation(specSignal, commSignal, rt string) string {
	isBullish := func(s string) bool {
		return s == "BULLISH" || s == "STRONG_BULLISH"
	}
	isBearish := func(s string) bool {
		return s == "BEARISH" || s == "STRONG_BEARISH"
	}
	isStrong := func(s string) bool {
		return s == "STRONG_BULLISH" || s == "STRONG_BEARISH"
	}

	specBull := isBullish(specSignal)
	specBear := isBearish(specSignal)
	commBull := isBullish(commSignal)
	commBear := isBearish(commSignal)
	commNeutral := !commBull && !commBear

	switch {
	// Strong agreement both sides
	case specBull && commBull && isStrong(specSignal) && isStrong(commSignal):
		return "<i>  ✅✅ KONFIRMASI KUAT: Smart money DAN hedger keduanya sangat bullish</i>\n"
	case specBear && commBear && isStrong(specSignal) && isStrong(commSignal):
		return "<i>  🔴🔴 KONFIRMASI KUAT: Smart money DAN hedger keduanya sangat bearish</i>\n"

	// Normal agreement
	case specBull && commBull:
		return "<i>  ✅ KONFIRMASI: Smart money dan hedger sama-sama bullish</i>\n"
	case specBear && commBear:
		return "<i>  🔴 KONFIRMASI: Smart money dan hedger sama-sama bearish</i>\n"

	// Classic divergence for commodities (normal)
	case specBull && commBear && rt == "DISAGGREGATED":
		return "<i>  ⚖️ Normal untuk komoditas: spekulan beli, produsen hedge jual</i>\n"
	case specBear && commBull && rt == "DISAGGREGATED":
		return "<i>  🔀 Tidak biasa: spekulan jual, tapi produsen justru akumulasi beli</i>\n"

	// Divergence for forex/indices
	case specBull && commBear:
		return "<i>  ⚠️ KONFLIK: Smart money bullish tapi dealer/hedger bearish — hati-hati</i>\n"
	case specBear && commBull:
		return "<i>  ⚠️ KONFLIK: Smart money bearish tapi dealer/hedger bullish — sinyal campur</i>\n"

	// Commercial neutral
	case specBull && commNeutral:
		return "<i>  🟡 Smart money bullish, hedger masih netral — belum full konfirmasi</i>\n"
	case specBear && commNeutral:
		return "<i>  🟡 Smart money bearish, hedger masih netral — belum full konfirmasi</i>\n"

	// Both neutral
	default:
		return "<i>  ⚪ Kedua sisi netral — tidak ada sinyal terarah saat ini</i>\n"
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

// trendLabel converts a direction string to a human-readable trend label.
func trendLabel(direction string) string {
	switch direction {
	case "UP":
		return "RISING"
	case "DOWN":
		return "FALLING"
	default:
		return "STABLE"
	}
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

// ---------------------------------------------------------------------------
// /macro Summary — plain-language narrative for non-finance users
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// /macro explain — Glossary of indicators for non-finance users
// ---------------------------------------------------------------------------

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
// Bias Detection Formatting
// ---------------------------------------------------------------------------

// FormatBiasHTML formats detected COT directional biases for Telegram display.
func (f *Formatter) FormatBiasHTML(signals []cot.Signal, filterCurrency string) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x8E\xAF <b>COT DIRECTIONAL BIAS</b>\n")
	if filterCurrency != "" {
		b.WriteString(fmt.Sprintf("<i>Filtered: %s</i>\n", filterCurrency))
	}
	b.WriteString("\n")

	if len(signals) == 0 {
		b.WriteString("No actionable biases detected.\n")
		b.WriteString("\n<i>Tip: Biases fire on extreme positioning, smart money moves,\ndivergences, momentum shifts, and thin markets.</i>")
		return b.String()
	}

	for i, s := range signals {
		if i >= 10 {
			b.WriteString(fmt.Sprintf("\n<i>... +%d more biases</i>", len(signals)-10))
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

	b.WriteString("<i>Tip: </i><code>/bias EUR</code> | <code>/cot EUR</code>")
	return b.String()
}

// FormatBiasSummary formats a compact bias summary for the /cot detail view.
func (f *Formatter) FormatBiasSummary(signals []cot.Signal) string {
	if len(signals) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\xF0\x9F\x8E\xAF <b>Active Biases:</b>\n")

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

	b.WriteString(fmt.Sprintf("<code>Signals  :</code> %d total, %d evaluated\n\n", stats.TotalSignals, stats.Evaluated))

	// Primary metrics: Expectancy and Profit Factor
	b.WriteString("<b>Performance</b>\n")
	if stats.ExpectedValue != 0 {
		evIcon := "\xE2\x9C\x85" // checkmark
		evLabel := "Positive Edge"
		if stats.ExpectedValue > 0.3 {
			evLabel = "Strong Edge"
		} else if stats.ExpectedValue <= 0 {
			evIcon = "\xF0\x9F\x94\xB4" // red circle
			evLabel = "Negative Edge"
		} else if stats.ExpectedValue < 0.1 {
			evIcon = "\xE2\x9A\xA0\xEF\xB8\x8F" // warning
			evLabel = "Marginal Edge"
		}
		b.WriteString(fmt.Sprintf("<code>EV/Trade :</code> %+.4f%% %s %s\n", stats.ExpectedValue, evIcon, evLabel))
	}
	if stats.ProfitFactor != 0 {
		pfIcon := "\xE2\x9C\x85"
		if stats.ProfitFactor < 1.0 {
			pfIcon = "\xF0\x9F\x94\xB4"
		}
		b.WriteString(fmt.Sprintf("<code>Profit F :</code> %.2f %s\n", stats.ProfitFactor, pfIcon))
	}
	if stats.AvgWinReturn1W != 0 || stats.AvgLossReturn1W != 0 {
		b.WriteString(fmt.Sprintf("<code>Avg Win  :</code> +%.2f%%\n", stats.AvgWinReturn1W))
		b.WriteString(fmt.Sprintf("<code>Avg Loss :</code> %.2f%%\n", stats.AvgLossReturn1W))
	}
	b.WriteString("\n")

	// Secondary metrics: Win rates
	b.WriteString("<b>Win Rates</b>\n")
	b.WriteString(fmt.Sprintf("<code>1W:</code> %.1f%% (n=%d) | <code>2W:</code> %.1f%% (n=%d) | <code>4W:</code> %.1f%% (n=%d)\n",
		stats.WinRate1W, stats.Evaluated1W,
		stats.WinRate2W, stats.Evaluated2W,
		stats.WinRate4W, stats.Evaluated4W))
	b.WriteString(fmt.Sprintf("<code>Best     :</code> %s at %.1f%%\n\n", stats.BestPeriod, stats.BestWinRate))

	b.WriteString("<b>Average Returns</b>\n")
	b.WriteString(fmt.Sprintf("<code>1W:</code> %.2f%% | <code>2W:</code> %.2f%% | <code>4W:</code> %.2f%%\n\n",
		stats.AvgReturn1W, stats.AvgReturn2W, stats.AvgReturn4W))

	// Risk-adjusted performance metrics
	if stats.SharpeRatio != 0 || stats.MaxDrawdown != 0 {
		b.WriteString("<b>Risk-Adjusted Metrics</b>\n")
		if stats.SharpeRatio != 0 {
			sharpeIcon := "\xE2\x9C\x85"
			if stats.SharpeRatio < 0.5 {
				sharpeIcon = "\xE2\x9A\xA0\xEF\xB8\x8F"
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
			fmtutil.FormatDateShortWIB(w.TrainStart),
			fmtutil.FormatDateShortWIB(w.TrainEnd),
			fmtutil.FormatDateShortWIB(w.TestStart),
			fmtutil.FormatDateShortWIB(w.TestEnd)))
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
// Uses plain language so non-finance users can understand each metric.
func (f *Formatter) FormatPriceContext(pc *domain.PriceContext) string {
	if pc == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n💰 <b>KONDISI HARGA SAAT INI</b>\n")
	b.WriteString(fmt.Sprintf("<code>Harga       : %.5f</code>\n", pc.CurrentPrice))

	// Weekly change with plain explanation
	wIcon := "🟢"
	wDesc := "naik minggu ini"
	if pc.WeeklyChgPct < 0 {
		wIcon = "🔴"
		wDesc = "turun minggu ini"
	} else if pc.WeeklyChgPct == 0 {
		wIcon = "⚪"
		wDesc = "flat minggu ini"
	}
	b.WriteString(fmt.Sprintf("<code>Perubahan 1W: </code>%s <b>%+.2f%%</b> <i>(%s)</i>\n", wIcon, pc.WeeklyChgPct, wDesc))

	// Monthly change
	mIcon := "🟢"
	mDesc := "naik sebulan terakhir"
	if pc.MonthlyChgPct < 0 {
		mIcon = "🔴"
		mDesc = "turun sebulan terakhir"
	} else if pc.MonthlyChgPct == 0 {
		mIcon = "⚪"
		mDesc = "flat sebulan terakhir"
	}
	b.WriteString(fmt.Sprintf("<code>Perubahan 1M: </code>%s <b>%+.2f%%</b> <i>(%s)</i>\n", mIcon, pc.MonthlyChgPct, mDesc))

	// 4-week trend with plain explanation
	trendIcon := "➡️"
	trendDesc := "bergerak sideways (tidak ada arah jelas)"
	if pc.Trend4W == "UP" {
		trendIcon = "⬆️"
		trendDesc = "tren 4 minggu ke ATAS"
	} else if pc.Trend4W == "DOWN" {
		trendIcon = "⬇️"
		trendDesc = "tren 4 minggu ke BAWAH"
	}
	b.WriteString(fmt.Sprintf("<code>Tren 4 Minggu:</code> %s <i>%s</i>\n", trendIcon, trendDesc))

	// MA explanation — simplified
	b.WriteString("\n<b>Posisi vs Rata-rata Harga:</b>\n")

	ma4wPos := "di BAWAH"
	ma4wIcon := "🔴"
	ma4wMeaning := "bearish jangka pendek"
	if pc.AboveMA4W {
		ma4wPos = "di ATAS"
		ma4wIcon = "🟢"
		ma4wMeaning = "bullish jangka pendek"
	}
	b.WriteString(fmt.Sprintf("<code>  Rata2 4-minggu : </code>%s %s (%.5f) — <i>%s</i>\n",
		ma4wIcon, ma4wPos, pc.PriceMA4W, ma4wMeaning))

	ma13wPos := "di BAWAH"
	ma13wIcon := "🔴"
	ma13wMeaning := "tren besar masih turun"
	if pc.AboveMA13W {
		ma13wPos = "di ATAS"
		ma13wIcon = "🟢"
		ma13wMeaning = "tren besar masih naik"
	}
	b.WriteString(fmt.Sprintf("<code>  Rata2 13-minggu: </code>%s %s (%.5f) — <i>%s</i>\n",
		ma13wIcon, ma13wPos, pc.PriceMA13W, ma13wMeaning))

	// MA alignment summary
	if pc.AboveMA4W && pc.AboveMA13W {
		b.WriteString("<i>  → Harga di atas kedua rata-rata = sinyal naik kuat</i>\n")
	} else if !pc.AboveMA4W && !pc.AboveMA13W {
		b.WriteString("<i>  → Harga di bawah kedua rata-rata = sinyal turun kuat</i>\n")
	} else if pc.AboveMA4W && !pc.AboveMA13W {
		b.WriteString("<i>  → Baru mulai rebound, tapi tren besar masih bearish</i>\n")
	} else {
		b.WriteString("<i>  → Mulai melemah dari tren naik, perlu waspada</i>\n")
	}

	// Volatility with plain explanation
	if pc.VolatilityRegime != "" {
		volIcon := "🟡"
		volDesc := "volatilitas normal — pergerakan harga wajar"
		switch pc.VolatilityRegime {
		case "EXPANDING":
			volIcon = "🔴"
			volDesc = "volatilitas TINGGI — harga sedang bergerak liar, risiko lebih besar"
		case "CONTRACTING":
			volIcon = "🟢"
			volDesc = "volatilitas RENDAH — harga sedang tenang, breakout mungkin segera terjadi"
		}
		b.WriteString(fmt.Sprintf("\n<code>Volatilitas: </code>%s <i>%s</i>\n", volIcon, volDesc))
		b.WriteString(fmt.Sprintf("<code>  ATR: %.5f (%.2f%% dari harga)</code>\n", pc.ATR, pc.NormalizedATR))
	}

	return b.String()
}

// FormatPriceCOTDivergence formats a price-COT divergence alert in plain language.
func (f *Formatter) FormatPriceCOTDivergence(div pricesvc.PriceCOTDivergence) string {
	var b strings.Builder

	icon := "⚠️"
	severityLabel := "Perlu Perhatian"
	if div.Severity == "HIGH" {
		icon = "🚨"
		severityLabel = "PERINGATAN KERAS"
	}

	b.WriteString(fmt.Sprintf("\n%s <b>SINYAL BERTENTANGAN (%s)</b>\n", icon, severityLabel))

	// Plain language explanation based on divergence type
	if div.PriceTrend == "UP" && div.COTDirection == "BEARISH" {
		b.WriteString("<b>Situasi:</b> Harga naik, tapi institusi besar justru JUAL\n")
		b.WriteString("<i>Artinya: Kenaikan harga ini mungkin tidak didukung oleh pemain besar.\n")
		b.WriteString("Bisa jadi ini \"rally palsu\" atau harga akan berbalik turun.\n")
		b.WriteString("Hati-hati beli di sini — tunggu konfirmasi lebih lanjut.</i>\n")
		if div.Severity == "HIGH" {
			b.WriteString("🚨 <b>COT Index di zona ekstrem SHORT — sinyal reversal kuat!</b>\n")
		}
	} else if div.PriceTrend == "DOWN" && div.COTDirection == "BULLISH" {
		b.WriteString("<b>Situasi:</b> Harga turun, tapi institusi besar justru BELI\n")
		b.WriteString("<i>Artinya: Penurunan harga ini mungkin sementara.\n")
		b.WriteString("Institusi besar melihat nilai di sini dan mulai akumulasi.\n")
		b.WriteString("Ini bisa menjadi kesempatan beli — tapi tunggu harga stabilisasi dulu.</i>\n")
		if div.Severity == "HIGH" {
			b.WriteString("🚨 <b>COT Index di zona ekstrem LONG — potensi reversal naik kuat!</b>\n")
		}
	} else {
		// Generic fallback
		b.WriteString(fmt.Sprintf("<i>%s</i>\n", div.Description))
	}

	b.WriteString(fmt.Sprintf("<code>  COT Index: %.0f%% | Tren Harga: %s</code>\n", div.COTIndex, div.PriceTrend))
	return b.String()
}

// FormatPriceCOTAlignment formats a confirmation when price and COT agree (no divergence).
// This replaces the silent "no divergence" gap — user always gets a price-COT verdict.
func (f *Formatter) FormatPriceCOTAlignment(pc *domain.PriceContext, a domain.COTAnalysis) string {
	if pc == nil {
		return ""
	}

	var b strings.Builder

	cotDir := "netral"
	if a.COTIndex > 60 {
		cotDir = "bullish (beli)"
	} else if a.COTIndex < 40 {
		cotDir = "bearish (jual)"
	}

	priceTrend := "sideways"
	if pc.Trend4W == "UP" {
		priceTrend = "naik"
	} else if pc.Trend4W == "DOWN" {
		priceTrend = "turun"
	}

	b.WriteString("\n🔗 <b>KONFIRMASI HARGA vs COT</b>\n")

	cotNeutral := a.COTIndex >= 40 && a.COTIndex <= 60
	priceFlat := pc.Trend4W == "FLAT"

	switch {
	// ✅ Harga dan COT sama-sama searah
	case (pc.Trend4W == "UP" && a.COTIndex > 60) || (pc.Trend4W == "DOWN" && a.COTIndex < 40):
		b.WriteString("✅ <b>Harga dan posisi institusi SELARAS</b>\n")
		b.WriteString(fmt.Sprintf("<i>Tren harga %s, dan institusi besar juga %s.\n", priceTrend, cotDir))
		b.WriteString("Ini sinyal lebih dapat dipercaya — momentum kemungkinan berlanjut.</i>\n")

	// ⚪ Harga naik/turun tapi COT netral — sinyal lemah
	case pc.Trend4W == "UP" && cotNeutral:
		b.WriteString("🟡 <b>Harga naik tapi institusi masih netral</b>\n")
		b.WriteString("<i>Harga sedang naik, tapi posisi institusi belum memihak ke atas.\n")
		b.WriteString("Bisa jadi pergerakan ini belum dikonfirmasi — tunggu COT bergerak ke atas dulu.</i>\n")

	case pc.Trend4W == "DOWN" && cotNeutral:
		b.WriteString("🟡 <b>Harga turun tapi institusi masih netral</b>\n")
		b.WriteString("<i>Harga sedang turun, tapi posisi institusi belum memihak ke bawah.\n")
		b.WriteString("Penurunan belum dikonfirmasi oleh data COT — hati-hati dengan false breakdown.</i>\n")

	// ⚪ COT punya arah tapi harga sideways — institusi menunggu
	case priceFlat && a.COTIndex > 60:
		b.WriteString("🟡 <b>Institusi bullish tapi harga masih sideways</b>\n")
		b.WriteString("<i>Dana besar sudah akumulasi posisi beli, tapi harga belum bergerak naik.\n")
		b.WriteString("Ini bisa jadi setup sebelum breakout — pantau level resistance.</i>\n")

	case priceFlat && a.COTIndex < 40:
		b.WriteString("🟡 <b>Institusi bearish tapi harga masih sideways</b>\n")
		b.WriteString("<i>Dana besar sudah akumulasi posisi jual, tapi harga belum turun.\n")
		b.WriteString("Bisa jadi distribusi diam-diam — waspadai breakdown ke bawah.</i>\n")

	// ⚪ Semua netral — tidak ada sinyal
	default:
		b.WriteString("⚪ <b>Tidak ada sinyal jelas saat ini</b>\n")
		b.WriteString(fmt.Sprintf("<i>Tren harga %s, posisi institusi %s.\n", priceTrend, cotDir))
		b.WriteString("Sebaiknya tunggu sampai salah satu pihak menunjukkan arah yang jelas.</i>\n")
	}

	return b.String()
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

	b.WriteString("📋 <b>Weekly Performance Report</b>\n")
	b.WriteString(fmt.Sprintf("<i>%s \xe2\x80\x94 %s</i>\n\n",
		fmtutil.FormatDateShortWIB(r.WeekStart),
		fmtutil.FormatDateWIB(r.WeekEnd),
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

// FormatEventImpact formats event impact summaries into a clean Telegram HTML message
// with confidence labels, directional summary, and asymmetry analysis.
func (f *Formatter) FormatEventImpact(eventTitle string, summaries []domain.EventImpactSummary) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8A <b>EVENT IMPACT: %s</b>\n", strings.ToUpper(html.EscapeString(eventTitle))))
	b.WriteString("<i>Historical price reaction by surprise magnitude</i>\n\n")

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

		// Compute total data points for confidence.
		var totalN int
		for _, item := range items {
			totalN += item.Occurrences
		}

		// Confidence label.
		confidence := "HIGH"
		confIcon := "\xE2\x9C\x85" // ✅
		if totalN < 5 {
			confidence = "LOW"
			confIcon = "\xE2\x9A\xA0\xEF\xB8\x8F" // ⚠️
		} else if totalN < 12 {
			confidence = "MEDIUM"
			confIcon = "\xF0\x9F\x9F\xA1" // 🟡
		}

		b.WriteString(fmt.Sprintf("<b>%s</b> %s <i>%s (N=%d)</i>\n", ccy, confIcon, confidence, totalN))
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-14s %7s %7s %4s\n", "Sigma", "AvgPip", "Median", "N"))
		b.WriteString(strings.Repeat("\xE2\x94\x80", 36) + "\n")

		for _, item := range items {
			b.WriteString(fmt.Sprintf("%-14s %+7.1f %+7.1f %4d\n",
				item.SigmaBucket, item.AvgPriceImpactPips, item.MedianImpact, item.Occurrences))
		}
		b.WriteString("</pre>")

		// Directional summary + asymmetry.
		posAvg, posN, negAvg, negN := impactAsymmetry(items)
		if posN > 0 || negN > 0 {
			b.WriteString("<i>")
			if posN > 0 {
				b.WriteString(fmt.Sprintf("Beat \xE2\x86\x92 avg %+.1f pips (N=%d)", posAvg, posN))
			}
			if posN > 0 && negN > 0 {
				b.WriteString(" | ")
			}
			if negN > 0 {
				b.WriteString(fmt.Sprintf("Miss \xE2\x86\x92 avg %+.1f pips (N=%d)", negAvg, negN))
			}
			b.WriteString("</i>\n")

			// Asymmetry ratio — only show when both sides have meaningful magnitude.
			if posN > 0 && negN > 0 && math.Abs(posAvg) >= 1.0 && math.Abs(negAvg) >= 1.0 {
				ratio := math.Abs(negAvg) / math.Abs(posAvg)
				if ratio > 1.3 {
					b.WriteString(fmt.Sprintf("\xE2\x9A\xA1 <i>Asymmetric: miss moves %.1fx stronger than beat</i>\n", ratio))
				} else if ratio < 0.7 {
					b.WriteString(fmt.Sprintf("\xE2\x9A\xA1 <i>Asymmetric: beat moves %.1fx stronger than miss</i>\n", 1/ratio))
				}
			}
		}

		b.WriteString("\n")
	}

	b.WriteString("<i>+ = currency strengthened | Surprise = Actual vs Forecast</i>")
	return b.String()
}

// impactAsymmetry computes average pips for positive-surprise vs negative-surprise buckets.
func impactAsymmetry(items []domain.EventImpactSummary) (posAvg float64, posN int, negAvg float64, negN int) {
	for _, item := range items {
		switch item.SigmaBucket {
		case ">+2\u03c3", "+1\u03c3 to +2\u03c3":
			posAvg += item.AvgPriceImpactPips * float64(item.Occurrences)
			posN += item.Occurrences
		case "<-2\u03c3", "-1\u03c3 to -2\u03c3":
			negAvg += item.AvgPriceImpactPips * float64(item.Occurrences)
			negN += item.Occurrences
		}
	}
	if posN > 0 {
		posAvg /= float64(posN)
	}
	if negN > 0 {
		negAvg /= float64(negN)
	}
	return
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
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Extreme fear often precedes rallies\n")
		} else if data.CNNFearGreed >= 75 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Extreme greed often precedes pullbacks\n")
		}
	} else {
		b.WriteString("<code>Data unavailable</code>\n")
	}

	// --- Crypto Fear & Greed Index (alternative.me) ---
	b.WriteString("\n<b>Crypto Fear &amp; Greed Index</b>\n")
	if data.CryptoFearGreedAvailable {
		gauge := sentimentGauge(data.CryptoFearGreed, 15)
		emoji := fearGreedEmoji(data.CryptoFearGreed)
		b.WriteString(fmt.Sprintf("<code>[%s]</code>\n", gauge))
		b.WriteString(fmt.Sprintf("<code>Score : %.0f / 100  %s %s</code>\n", data.CryptoFearGreed, emoji, data.CryptoFearGreedLabel))
		if data.CryptoFearGreed <= 25 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Extreme fear in crypto may signal accumulation zone\n")
		} else if data.CryptoFearGreed >= 75 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Extreme greed in crypto often precedes corrections\n")
		}
	} else {
		b.WriteString("<code>Data unavailable</code>\n")
	}

	// --- AAII Investor Sentiment Survey ---
	b.WriteString("\n<b>AAII Investor Sentiment Survey</b>\n")
	if data.AAIIAvailable {
		if data.AAIIWeekDate != "" {
			b.WriteString(fmt.Sprintf("<i>Week ending %s</i>\n", data.AAIIWeekDate))
		}
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
		b.WriteString("<code>Data unavailable — set FIRECRAWL_API_KEY to enable</code>\n")
	}

	// --- AAII contrarian signal ---
	if data.AAIIAvailable {
		if data.AAIIBearish >= 50 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Bearish >50%% historically precedes rallies\n")
		} else if data.AAIIBullish >= 50 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Bullish >50%% historically precedes pullbacks\n")
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
				b.WriteString("<i>Index P/C elevated → institutions hedging. Equity P/C normal → retail not panicking yet.</i>\n")
			} else if data.PutCallTotal >= 1.2 {
				b.WriteString("<i>Extreme put buying across the board — strong contrarian bullish signal.</i>\n")
			} else if data.PutCallTotal < 0.7 {
				b.WriteString("<i>Very low protection buying — complacency warning. Contrarian bearish.</i>\n")
			}
		}
	} else {
		b.WriteString("<code>Data unavailable</code>\n")
	}

	// --- Composite reading ---
	b.WriteString("\n<b>Combined Reading</b>\n")
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
		b.WriteString("<i>Sentiment surveys are contrarian indicators.\n")
		b.WriteString("Extreme readings often mark turning points.</i>\n")
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
// Enhanced: shows confidence bar for current month and regime context header.
func (f *Formatter) FormatSeasonalPatterns(patterns []pricesvc.SeasonalPattern) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x85 <b>ADVANCED SEASONAL ANALYSIS</b>\n")
	b.WriteString("<i>Statistical monthly bias (up to 5 years, min n\xe2\x89\xa53)</i>\n")

	// Show regime context if available
	if len(patterns) > 0 && patterns[0].RegimeStats != nil {
		b.WriteString(fmt.Sprintf("<i>Regime: %s</i>\n", patterns[0].RegimeStats.RegimeName))
	}
	b.WriteString("\n")

	// Compact grid: currency + 12 months + confluence
	b.WriteString("<pre>")
	shortMonths := [12]string{"J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"}
	// Determine current month for header alignment (bracket current month to match data rows)
	curMonth := 0
	if len(patterns) > 0 {
		curMonth = patterns[0].CurrentMonth
	}
	b.WriteString(fmt.Sprintf("%-6s", "CCY"))
	for i, m := range shortMonths {
		if i+1 == curMonth {
			b.WriteString(fmt.Sprintf("[%s]", m))
		} else {
			b.WriteString(fmt.Sprintf(" %s", m))
		}
	}
	b.WriteString(" Cf\n")
	b.WriteString(strings.Repeat("\xe2\x94\x80", 35) + "\n")

	for _, p := range patterns {
		b.WriteString(fmt.Sprintf("%-6s", p.Currency))
		for i := 0; i < 12; i++ {
			icon := "\xc2\xb7"
			switch p.Monthly[i].Bias {
			case "BULLISH":
				icon = "\xe2\x96\xb2"
			case "BEARISH":
				icon = "\xe2\x96\xbc"
			}
			if i+1 == p.CurrentMonth {
				b.WriteString(fmt.Sprintf("[%s]", icon))
			} else {
				b.WriteString(fmt.Sprintf(" %s", icon))
			}
		}
		// Confluence score for current month
		if p.Confluence != nil {
			b.WriteString(fmt.Sprintf(" %d/%d", p.Confluence.Score, p.Confluence.MaxScore))
		} else {
			b.WriteString("  -")
		}
		b.WriteString("\n")
	}
	b.WriteString("</pre>\n")

	b.WriteString("<i>\xe2\x96\xb2=Bullish \xe2\x96\xbc=Bearish \xc2\xb7=Neutral [x]=now Cf=confluence</i>\n")

	// Strongest tendencies with confidence
	type tendency struct {
		currency   string
		month      string
		avgRet     float64
		winRate    float64
		bias       string
		confidence pricesvc.ConfidenceTier
	}
	var strong []tendency
	for _, p := range patterns {
		for i := 0; i < 12; i++ {
			ms := p.Monthly[i]
			if ms.Bias != "NEUTRAL" && ms.SampleSize >= 3 {
				strong = append(strong, tendency{
					currency: p.Currency, month: ms.Month,
					avgRet: ms.AvgReturn, winRate: ms.WinRate,
					bias: ms.Bias, confidence: ms.Confidence,
				})
			}
		}
	}

	sort.Slice(strong, func(i, j int) bool {
		return math.Abs(strong[i].avgRet) > math.Abs(strong[j].avgRet)
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
			confTag := ""
			if t.confidence == pricesvc.ConfidenceStrong {
				confTag = " \xe2\x9c\xa8"
			}
			b.WriteString(fmt.Sprintf("%s %s %s: %+.2f%% (%.0f%% WR)%s\n",
				icon, t.currency, t.month, t.avgRet, t.winRate, confTag))
		}
		b.WriteString("<i>\xe2\x9c\xa8 = STRONG confidence</i>\n")
	}

	b.WriteString("\n<i>Use <code>/seasonal CCY</code> for deep dive</i>")

	return b.String()
}

// FormatSeasonalSingle formats the advanced seasonal deep-dive for a single contract.
func (f *Formatter) FormatSeasonalSingle(p pricesvc.SeasonalPattern) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x85 <b>%s \xe2\x80\x94 SEASONAL DEEP DIVE</b>\n", html.EscapeString(p.Currency)))
	b.WriteString("<i>Up to 5 years, regime-aware, multi-factor</i>\n\n")

	// --- STATISTICAL SUMMARY TABLE ---
	b.WriteString("<b>MONTHLY STATISTICS</b>\n<pre>")
	b.WriteString(fmt.Sprintf("%-4s %6s %6s %5s %4s %3s %s\n", "Mon", "Avg", "Med", "WR", "Wgt", "N", ""))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 38) + "\n")

	for i := 0; i < 12; i++ {
		ms := p.Monthly[i]
		marker := " "
		if i+1 == p.CurrentMonth {
			marker = "\xe2\x96\xb6"
		}

		biasIcon := "\xc2\xb7"
		switch ms.Bias {
		case "BULLISH":
			biasIcon = "\xe2\x96\xb2"
		case "BEARISH":
			biasIcon = "\xe2\x96\xbc"
		}

		wrStr := fmt.Sprintf("%.0f%%", ms.WinRate)
		wgtStr := fmt.Sprintf("%.0f%%", ms.WeightedWR)
		if ms.SampleSize == 0 {
			wrStr = "  -"
			wgtStr = "  -"
		}

		b.WriteString(fmt.Sprintf("%s%-3s %+5.1f%% %+5.1f%% %4s %4s %2d %s\n",
			marker, ms.Month, ms.AvgReturn, ms.MedianRet, wrStr, wgtStr, ms.SampleSize, biasIcon))
	}
	b.WriteString("</pre>\n")

	curMs := p.Monthly[p.CurrentMonth-1]

	// --- CURRENT MONTH SUMMARY ---
	biasEmoji := "\xe2\x9a\xaa"
	switch p.CurrentBias {
	case "BULLISH":
		biasEmoji = "\xF0\x9F\x9F\xA2"
	case "BEARISH":
		biasEmoji = "\xF0\x9F\x94\xB4"
	}
	b.WriteString(fmt.Sprintf("\n%s <b>%s (%s):</b> %+.2f%% avg, %.0f%% WR",
		biasEmoji, curMs.Month, p.CurrentBias, curMs.AvgReturn, curMs.WinRate))
	if curMs.StdDev > 0 {
		b.WriteString(fmt.Sprintf(", \xcf\x83=%.1f%%", curMs.StdDev))
	}
	b.WriteString(fmt.Sprintf(" (n=%d)\n", curMs.SampleSize))
	if curMs.WeightedAvg != 0 {
		b.WriteString(fmt.Sprintf("<i>Recency-weighted: %+.2f%% avg, %.0f%% WR</i>\n", curMs.WeightedAvg, curMs.WeightedWR))
	}

	// Year-by-year returns
	if len(curMs.YearReturns) > 0 {
		b.WriteString("<pre>")
		for _, yr := range curMs.YearReturns {
			icon := "\xe2\x96\xb2"
			if yr.Return < 0 {
				icon = "\xe2\x96\xbc"
			}
			b.WriteString(fmt.Sprintf("  %d: %+6.2f%% %s\n", yr.Year, yr.Return, icon))
		}
		b.WriteString("</pre>\n")
	}

	// --- REGIME CONTEXT (Phase 2) ---
	if p.RegimeStats != nil {
		b.WriteString("\n\xF0\x9F\x8F\x9B <b>REGIME CONTEXT</b>\n")
		b.WriteString(fmt.Sprintf("<code>Current : %s</code>\n", p.RegimeStats.RegimeName))
		if p.RegimeStats.SampleSize > 0 {
			b.WriteString(fmt.Sprintf("<code>In regime: %+.1f%% avg, %.0f%% WR (n=%d)</code>\n",
				p.RegimeStats.AvgReturn, p.RegimeStats.WinRate, p.RegimeStats.SampleSize))
		} else {
			b.WriteString("<code>In regime: no historical data in same regime</code>\n")
		}
		b.WriteString(fmt.Sprintf("<code>Driver  : %s</code>\n", p.RegimeStats.PrimaryFREDDriver))
		driverIcon := "\xe2\x9e\x96"
		switch p.RegimeStats.DriverAlignment {
		case "SUPPORTIVE":
			driverIcon = "\xe2\x9c\x85"
		case "HEADWIND":
			driverIcon = "\xe2\x9d\x8c"
		}
		b.WriteString(fmt.Sprintf("<code>Outlook : %s %s</code>\n", p.RegimeStats.DriverAlignment, driverIcon))
	}

	// --- COT ALIGNMENT (Phase 3a) ---
	if p.COTAlignment != nil {
		b.WriteString("\n\xF0\x9F\x93\x8A <b>COT ALIGNMENT</b>\n")
		alignIcon := "\xe2\x9d\x8c"
		if p.COTAlignment.CurrentAligned {
			alignIcon = "\xe2\x9c\x85"
		}
		b.WriteString(fmt.Sprintf("<code>Current : COT %s %s</code>\n", p.COTAlignment.CurrentCOTBias, alignIcon))
		b.WriteString(fmt.Sprintf("<code>Interp  : %s</code>\n", p.COTAlignment.Interpretation))
	}

	// --- EVENT DENSITY (Phase 3b) ---
	if p.EventDensity != nil {
		b.WriteString("\n\xF0\x9F\x93\x86 <b>EVENT DENSITY</b>\n")
		evIcon := "\xF0\x9F\x9F\xA2"
		if p.EventDensity.Rating == "HIGH" {
			evIcon = "\xF0\x9F\x94\xB4"
		} else if p.EventDensity.Rating == "MEDIUM" {
			evIcon = "\xF0\x9F\x9F\xA1"
		}
		b.WriteString(fmt.Sprintf("<code>Rating  : %s %s (%d high-impact)</code>\n",
			p.EventDensity.Rating, evIcon, p.EventDensity.HighImpactEvents))
		if p.EventDensity.KeyEvents != "" {
			b.WriteString(fmt.Sprintf("<code>Events  : %s</code>\n", html.EscapeString(p.EventDensity.KeyEvents)))
		}
	}

	// --- VOLATILITY CONTEXT (Phase 3c) ---
	if p.VolContext != nil && p.VolContext.AvgATR > 0 {
		b.WriteString("\n\xF0\x9F\x93\x89 <b>VOLATILITY</b>\n")
		b.WriteString(fmt.Sprintf("<code>Month vol: %.1f%% (%.1fx avg)</code>\n",
			p.VolContext.HistoricalATR, p.VolContext.VolRatio))
		if p.VolContext.CurrentVIXRegime != "N/A" {
			b.WriteString(fmt.Sprintf("<code>VIX     : %s (sensitivity: %s)</code>\n",
				p.VolContext.CurrentVIXRegime, p.VolContext.VIXSensitivity))
		}
		if p.VolContext.Assessment != "" {
			b.WriteString(fmt.Sprintf("<i>%s</i>\n", p.VolContext.Assessment))
		}
	}

	// --- CROSS-ASSET (Phase 3d) ---
	if p.CrossAsset != nil && len(p.CrossAsset.Correlations) > 0 {
		b.WriteString("\n\xF0\x9F\x94\x97 <b>CROSS-ASSET</b>\n")
		for _, cc := range p.CrossAsset.Correlations {
			checkIcon := "\xe2\x9c\x85"
			if !cc.IsAligned {
				checkIcon = "\xe2\x9a\xa0\xef\xb8\x8f"
			}
			b.WriteString(fmt.Sprintf("<code>%-6s %s seasonal %s</code> %s\n",
				cc.Asset, cc.Relation, cc.TheirBias, checkIcon))
		}
		b.WriteString(fmt.Sprintf("<code>Assessment: %s</code>\n", p.CrossAsset.Assessment))
	}

	// --- EIA ENERGY CONTEXT (Phase 4) ---
	if p.EIACtx != nil {
		b.WriteString("\n\xF0\x9F\x9B\xA2 <b>EIA ENERGY CONTEXT</b>\n")
		if p.EIACtx.InventoryTrend != "" {
			trendIcon := "\xe2\x9e\x96"
			switch p.EIACtx.InventoryTrend {
			case "BUILD":
				trendIcon = "\xF0\x9F\x93\x88"
			case "DRAW":
				trendIcon = "\xF0\x9F\x93\x89"
			}
			b.WriteString(fmt.Sprintf("<code>Inventory: %s %s (avg %+.1fM bbl/wk)</code>\n",
				p.EIACtx.InventoryTrend, trendIcon, p.EIACtx.AvgWeeklyChange))
		}
		if p.EIACtx.RefineryUtil > 0 {
			b.WriteString(fmt.Sprintf("<code>Refinery : %.1f%% utilization</code>\n", p.EIACtx.RefineryUtil))
		}
		if p.EIACtx.CurrentVs5YrAvg != "" {
			b.WriteString(fmt.Sprintf("<code>vs 5yr   : %s seasonal average</code>\n", p.EIACtx.CurrentVs5YrAvg))
		}
		if p.EIACtx.Assessment != "" {
			b.WriteString(fmt.Sprintf("<i>%s</i>\n", p.EIACtx.Assessment))
		}
	}

	// --- CONFLUENCE SCORE (Phase 5) ---
	if p.Confluence != nil {
		b.WriteString("\n\xF0\x9F\x8E\xAF <b>CONFLUENCE SCORE</b>\n")

		// Visual bar
		filled := p.Confluence.Score
		empty := p.Confluence.MaxScore - filled
		bar := strings.Repeat("\xe2\x96\x88", filled) + strings.Repeat("\xe2\x96\x91", empty)
		b.WriteString(fmt.Sprintf("<code>%s %d/%d</code>\n", bar, p.Confluence.Score, p.Confluence.MaxScore))

		// Factor details
		for _, factor := range p.Confluence.Factors {
			checkIcon := "\xe2\x9c\x97"
			if factor.Aligned {
				checkIcon = "\xe2\x9c\x93"
			}
			b.WriteString(fmt.Sprintf("<code>%s %-11s %s</code>\n", checkIcon, factor.Name, html.EscapeString(factor.Detail)))
		}

		b.WriteString(fmt.Sprintf("\n<b>Verdict: %s</b>\n", p.Confluence.Verdict))
	}

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
