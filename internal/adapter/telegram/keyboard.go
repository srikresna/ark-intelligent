package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
)

// ---------------------------------------------------------------------------
// KeyboardBuilder — Telegram inline keyboard construction
// ---------------------------------------------------------------------------

// KeyboardBuilder creates inline keyboards for interactive bot messages.
// All callback_data values use a prefix-based routing scheme:
//   - "cot:XXX"   -> COT detail for currency/contract
//   - "set:XXX"   -> Settings toggle action
//   - "alert:XXX" -> Alert action (mute, dismiss)
type KeyboardBuilder struct{}

// NewKeyboardBuilder creates a new KeyboardBuilder.
func NewKeyboardBuilder() *KeyboardBuilder {
	return &KeyboardBuilder{}
}

// ---------------------------------------------------------------------------
// COT Keyboards
// ---------------------------------------------------------------------------

// COTCurrencySelector builds a keyboard for selecting a currency to view COT detail.
// Layout: 2 currencies per row for the 8 major contracts.
func (kb *KeyboardBuilder) COTCurrencySelector(analyses []domain.COTAnalysis) ports.InlineKeyboard {
	var rows [][]ports.InlineButton
	var currentRow []ports.InlineButton

	for i, a := range analyses {
		// Extract short label from contract name
		label := kb.contractLabel(a.Contract.Name, a.Contract.Code)

		// Add bias indicator
		indicator := "--"
		if a.NetPosition > 0 {
			indicator = "+"
		} else if a.NetPosition < 0 {
			indicator = "-"
		}

		btn := ports.InlineButton{
			Text:         fmt.Sprintf("%s [%s]", label, indicator),
			CallbackData: fmt.Sprintf("cot:analysis:%s", a.Contract.Code),
		}
		currentRow = append(currentRow, btn)

		// 2 buttons per row
		if len(currentRow) == 2 || i == len(analyses)-1 {
			rows = append(rows, currentRow)
			currentRow = nil
		}
	}

	// Add cross-market row if we have gold/oil
	crossRow := kb.crossMarketRow(analyses)
	if len(crossRow) > 0 {
		rows = append(rows, crossRow)
	}

	return ports.InlineKeyboard{Rows: rows}
}

// ---------------------------------------------------------------------------
// Calendar Keyboards
// ---------------------------------------------------------------------------

// CalendarFilter builds the filter keyboard for the /calendar command.
func (kb *KeyboardBuilder) CalendarFilter(activeFilter string, dateStr string, isWeek bool) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	btnText := func(label, filterKey string) string {
		if activeFilter == filterKey {
			return "✅ " + label
		}
		return label
	}

	viewType := "day"
	if isWeek {
		viewType = "week"
	}

	// Row 1: Navigation
	if isWeek {
		rows = append(rows, []ports.InlineButton{
			{Text: "◀ Prev Week", CallbackData: "cal:nav:prevwk:" + dateStr},
			{Text: "Harian", CallbackData: "cal:nav:day:" + dateStr},
			{Text: "Next Week ▶", CallbackData: "cal:nav:nextwk:" + dateStr},
		})
	} else {
		rows = append(rows, []ports.InlineButton{
			{Text: "◀ Kemarin", CallbackData: "cal:nav:prev:" + dateStr},
			{Text: "Seminggu", CallbackData: "cal:nav:week:" + dateStr},
			{Text: "Besok ▶", CallbackData: "cal:nav:next:" + dateStr},
		})
	}

	// Row 2: Impact filter — All / High Only / Med+
	rows = append(rows, []ports.InlineButton{
		{Text: btnText("All", "all"), CallbackData: "cal:filter:all:" + dateStr + ":" + viewType},
		{Text: btnText("High Only", "high"), CallbackData: "cal:filter:high:" + dateStr + ":" + viewType},
		{Text: btnText("Med+", "med"), CallbackData: "cal:filter:med:" + dateStr + ":" + viewType},
	})

	// Row 3: Currency filter — USD EUR GBP JPY
	rows = append(rows, []ports.InlineButton{
		{Text: btnText("🇺🇸 USD", "cur:USD"), CallbackData: "cal:filter:cur:USD:" + dateStr + ":" + viewType},
		{Text: btnText("🇪🇺 EUR", "cur:EUR"), CallbackData: "cal:filter:cur:EUR:" + dateStr + ":" + viewType},
		{Text: btnText("🇬🇧 GBP", "cur:GBP"), CallbackData: "cal:filter:cur:GBP:" + dateStr + ":" + viewType},
		{Text: btnText("🇯🇵 JPY", "cur:JPY"), CallbackData: "cal:filter:cur:JPY:" + dateStr + ":" + viewType},
	})

	// Row 4: Currency filter — AUD CAD CHF NZD
	rows = append(rows, []ports.InlineButton{
		{Text: btnText("🇦🇺 AUD", "cur:AUD"), CallbackData: "cal:filter:cur:AUD:" + dateStr + ":" + viewType},
		{Text: btnText("🇨🇦 CAD", "cur:CAD"), CallbackData: "cal:filter:cur:CAD:" + dateStr + ":" + viewType},
		{Text: btnText("🇨🇭 CHF", "cur:CHF"), CallbackData: "cal:filter:cur:CHF:" + dateStr + ":" + viewType},
		{Text: btnText("🇳🇿 NZD", "cur:NZD"), CallbackData: "cal:filter:cur:NZD:" + dateStr + ":" + viewType},
	})

	// Row 5: Month navigation
	rows = append(rows, []ports.InlineButton{
		{Text: "◀ Prev Month", CallbackData: "cal:nav:prevmonth:" + dateStr},
		{Text: "This Month", CallbackData: "cal:nav:thismonth:" + dateStr},
		{Text: "Next Month ▶", CallbackData: "cal:nav:nextmonth:" + dateStr},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// OutlookMenu builds a keyboard for choosing the AI Outlook type.
func (kb *KeyboardBuilder) OutlookMenu() ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	rows = append(rows, []ports.InlineButton{
		{Text: "📊 COT Positioning", CallbackData: "out:cot"},
		{Text: "📰 News Catalysts", CallbackData: "out:news"},
	})
	rows = append(rows, []ports.InlineButton{
		{Text: "🏦 FRED Macro", CallbackData: "out:fred"},
	})
	rows = append(rows, []ports.InlineButton{
		{Text: "🔗 Fused (COT + News + FRED)", CallbackData: "out:combine"},
	})
	rows = append(rows, []ports.InlineButton{
		{Text: "🌐 Cross-Market (Gold/Oil/Bond/USD)", CallbackData: "out:cross"},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// crossMarketRow adds quick-access buttons for Gold and Oil if available.
func (kb *KeyboardBuilder) crossMarketRow(analyses []domain.COTAnalysis) []ports.InlineButton {
	var row []ports.InlineButton
	for _, a := range analyses {
		code := strings.ToUpper(a.Contract.Code)
		if code == "088691" || code == "067651" { // Gold, Oil
			continue // Already in main grid
		}
	}
	return row
}

// contractLabel extracts a short display label from a contract name.
func (kb *KeyboardBuilder) contractLabel(name, code string) string {
	// Common CFTC contract code to currency mappings
	labels := map[string]string{
		"099741": "EUR",
		"096742": "GBP",
		"097741": "JPY",
		"232741": "AUD",
		"112741": "NZD",
		"090741": "CAD",
		"092741": "CHF",
		"098662": "DXY",
		"088691": "GOLD",
		"067651": "OIL",
	}

	if label, ok := labels[code]; ok {
		return label
	}

	// Fallback: first word of contract name
	if parts := strings.Fields(name); len(parts) > 0 {
		if len(parts[0]) <= 5 {
			return strings.ToUpper(parts[0])
		}
		return strings.ToUpper(parts[0][:4])
	}

	return code
}

// ---------------------------------------------------------------------------
// Settings Keyboards
// ---------------------------------------------------------------------------

// alertMinutesPreset returns the preset key matching the given slice, or "".
func alertMinutesPreset(minutes []int) string {
	if sliceEqual(minutes, []int{60, 15, 5}) {
		return "time_60_15_5"
	}
	if sliceEqual(minutes, []int{15, 5, 1}) {
		return "time_15_5_1"
	}
	if sliceEqual(minutes, []int{5, 1}) {
		return "time_5_1"
	}
	return ""
}

func sliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SettingsMenu builds the settings control keyboard.
// Shows current state and toggle buttons for all preference options.
func (kb *KeyboardBuilder) SettingsMenu(prefs domain.UserPrefs) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	// Row 1: COT Release Alerts toggle
	cotLabel := "COT Alerts: OFF -> Turn ON"
	if prefs.COTAlertsEnabled {
		cotLabel = "COT Alerts: ON -> Turn OFF"
	}
	rows = append(rows, []ports.InlineButton{{
		Text:         cotLabel,
		CallbackData: "set:cot_toggle",
	}})

	// Row 2: AI Reports toggle
	aiLabel := "AI Reports: OFF -> Turn ON"
	if prefs.AIReportsEnabled {
		_ = "ON"
		aiLabel = "AI Reports: ON -> Turn OFF"
	}
	rows = append(rows, []ports.InlineButton{{
		Text:         aiLabel,
		CallbackData: "set:ai_toggle",
	}})

	// Row 3: Language Toggle
	langLabel := "🌐 Language: Indo 🇮🇩 -> Eng 🇬🇧"
	if prefs.Language == "en" {
		langLabel = "🌐 Language: Eng 🇬🇧 -> Indo 🇮🇩"
	}
	rows = append(rows, []ports.InlineButton{{
		Text:         langLabel,
		CallbackData: "set:lang_toggle",
	}})

	// Row 4: Alert Minutes presets
	activePreset := alertMinutesPreset(prefs.AlertMinutes)
	presetLabel := func(key, label string) string {
		if activePreset == key {
			return "✅ " + label
		}
		return label
	}
	rows = append(rows, []ports.InlineButton{
		{Text: presetLabel("time_60_15_5", "⏰ 60/15/5"), CallbackData: "set:time_60_15_5"},
		{Text: presetLabel("time_15_5_1", "⏰ 15/5/1"), CallbackData: "set:time_15_5_1"},
		{Text: presetLabel("time_5_1", "⏰ 5/1"), CallbackData: "set:time_5_1"},
	})

	// Rows 5-6: Currency filter toggles
	curSet := make(map[string]bool)
	for _, c := range prefs.CurrencyFilter {
		curSet[strings.ToUpper(c)] = true
	}
	curBtn := func(flag, cur string) ports.InlineButton {
		label := flag + " " + cur
		if curSet[cur] {
			label = "✅ " + flag + " " + cur
		}
		return ports.InlineButton{
			Text:         label,
			CallbackData: "set:cur_toggle:" + cur,
		}
	}
	rows = append(rows, []ports.InlineButton{
		curBtn("🇺🇸", "USD"),
		curBtn("🇪🇺", "EUR"),
		curBtn("🇬🇧", "GBP"),
		curBtn("🇯🇵", "JPY"),
	})
	rows = append(rows, []ports.InlineButton{
		curBtn("🇦🇺", "AUD"),
		curBtn("🇨🇦", "CAD"),
		curBtn("🇨🇭", "CHF"),
		curBtn("🇳🇿", "NZD"),
	})

	// Row 7: All Currencies reset
	allCurLabel := "All Currencies"
	if len(prefs.CurrencyFilter) == 0 {
		allCurLabel = "✅ All Currencies"
	}
	rows = append(rows, []ports.InlineButton{{
		Text:         allCurLabel,
		CallbackData: "set:cur_reset",
	}})

	// Row 8: View Changelog
	rows = append(rows, []ports.InlineButton{{
		Text:         "📜 View Changelog",
		CallbackData: "set:changelog_view",
	}})

	return ports.InlineKeyboard{Rows: rows}
}

// ---------------------------------------------------------------------------
// Navigation Keyboards
// ---------------------------------------------------------------------------

// COTDetailMenu builds a keyboard for a specific COT detail view, allowing toggle between RAW and Analysis.
func (kb *KeyboardBuilder) COTDetailMenu(code string, isRaw bool) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	if isRaw {
		rows = append(rows, []ports.InlineButton{
			{Text: "📊 View Analysis", CallbackData: fmt.Sprintf("cot:analysis:%s", code)},
		})
	} else {
		rows = append(rows, []ports.InlineButton{
			{Text: "📄 View Raw Data", CallbackData: fmt.Sprintf("cot:raw:%s", code)},
		})
	}

	rows = append(rows, []ports.InlineButton{
		{Text: "<< Back to Overview", CallbackData: "cot:overview"},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// MainMenu builds a quick-access keyboard for the main bot features.
func (kb *KeyboardBuilder) MainMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "COT Analysis", CallbackData: "nav:cot"},
				{Text: "Weekly Outlook", CallbackData: "nav:outlook"},
			},
		},
	}
}
