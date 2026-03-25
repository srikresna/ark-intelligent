package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
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

// OutlookMenu builds a keyboard for the AI Outlook (unified — single action).
func (kb *KeyboardBuilder) OutlookMenu() ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	rows = append(rows, []ports.InlineButton{
		{Text: "🦅 Generate Unified Outlook", CallbackData: "out:unified"},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// MacroMenu builds the inline keyboard for the /macro command.
// Provides navigation between summary, detail, glossary, and performance views.
func (kb *KeyboardBuilder) MacroMenu(isAdmin bool) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	// Row 1: Primary navigation
	rows = append(rows, []ports.InlineButton{
		{Text: "📊 Data Lengkap", CallbackData: "macro:detail"},
		{Text: "📖 Panduan Indikator", CallbackData: "macro:explain"},
	})

	// Row 2: Additional views
	row2 := []ports.InlineButton{
		{Text: "📈 Performance", CallbackData: "macro:performance"},
	}
	if isAdmin {
		row2 = append(row2, ports.InlineButton{Text: "🔄 Refresh Data", CallbackData: "macro:refresh"})
	}
	rows = append(rows, row2)

	return ports.InlineKeyboard{Rows: rows}
}

// MacroDetailMenu builds the back-navigation keyboard for macro detail/explain views.
func (kb *KeyboardBuilder) MacroDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "<< Kembali ke Ringkasan", CallbackData: "macro:summary"},
			},
		},
	}
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

	// Row 8: AI Provider selector (Claude vs Gemini)
	providerLabel := func(key, label string) string {
		current := prefs.PreferredModel
		if current == "" {
			current = "claude" // default
		}
		if current == key {
			return "✅ " + label
		}
		return label
	}
	rows = append(rows, []ports.InlineButton{
		{Text: providerLabel("claude", "🤖 Claude"), CallbackData: "set:model_claude"},
		{Text: providerLabel("gemini", "✨ Gemini"), CallbackData: "set:model_gemini"},
	})

	// Rows 9-10: Claude model variant selector (shown for all users; only relevant when Claude is active)
	claudeModelBtn := func(m domain.ClaudeModelID) ports.InlineButton {
		label := "   " + domain.ClaudeModelLabel(m)
		if prefs.ClaudeModel == m {
			label = "✅ " + domain.ClaudeModelLabel(m)
		}
		return ports.InlineButton{
			Text:         label,
			CallbackData: "set:claude_model:" + string(m),
		}
	}
	rows = append(rows, []ports.InlineButton{
		claudeModelBtn(domain.ClaudeModelOpus4),
		claudeModelBtn(domain.ClaudeModelSonnet4),
	})
	rows = append(rows, []ports.InlineButton{
		claudeModelBtn(domain.ClaudeModelHaiku4),
	})

	// Row 11: View Changelog
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

	// Quick-access buttons for related features
	currency := kb.contractLabel(code, code) // derive currency label from contract code
	rows = append(rows, []ports.InlineButton{
		{Text: "📈 Seasonal", CallbackData: fmt.Sprintf("cmd:seasonal:%s", currency)},
		{Text: "💹 Sentiment", CallbackData: "cmd:sentiment"},
	})

	rows = append(rows, []ports.InlineButton{
		{Text: "<< Back to Overview", CallbackData: "cot:overview"},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// ---------------------------------------------------------------------------
// Impact Keyboards
// ---------------------------------------------------------------------------

// ImpactCategoryMenu builds a categorized event selection keyboard.
func (kb *KeyboardBuilder) ImpactCategoryMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🏦 Central Banks", CallbackData: "imp:cat:cb"},
				{Text: "📊 Inflation", CallbackData: "imp:cat:inf"},
			},
			{
				{Text: "👷 Employment", CallbackData: "imp:cat:emp"},
				{Text: "📈 Growth & PMI", CallbackData: "imp:cat:growth"},
			},
			{
				{Text: "🛒 Consumer", CallbackData: "imp:cat:consumer"},
				{Text: "🏠 Housing", CallbackData: "imp:cat:housing"},
			},
		},
	}
}

// ImpactEventMenu builds event selection buttons for a category.
func (kb *KeyboardBuilder) ImpactEventMenu(category string) ports.InlineKeyboard {
	type eventDef struct {
		Label string
		Alias string
	}

	categories := map[string][]eventDef{
		"cb": {
			{"🇺🇸 FOMC/Fed Rate", "FOMC"},
			{"🇬🇧 BOE Rate", "BOE"},
			{"🇪🇺 ECB Rate", "ECB"},
			{"🇯🇵 BOJ Rate", "BOJ"},
			{"🇦🇺 RBA Rate", "RBA"},
			{"🇨🇦 BOC Rate", "BOC"},
			{"🇳🇿 RBNZ Rate", "RBNZ"},
			{"🇨🇭 SNB Rate", "SNB"},
		},
		"inf": {
			{"CPI m/m", "CPI"},
			{"Core CPI m/m", "CORE_CPI"},
			{"PPI m/m", "PPI"},
			{"Core PCE", "PCE"},
		},
		"emp": {
			{"NFP", "NFP"},
			{"ADP Employment", "ADP"},
			{"Jobless Claims", "CLAIMS"},
			{"Avg Hourly Earnings", "WAGES"},
		},
		"growth": {
			{"GDP q/q", "GDP"},
			{"ISM Mfg PMI", "ISM"},
		},
		"consumer": {
			{"Core Retail Sales", "RETAIL"},
			{"CB Consumer Conf", "CB_CONSUMER"},
			{"Consumer Price Exp", "PRICE_EXP"},
		},
		"housing": {
			{"Existing Home Sales", "HOME_SALES"},
			{"Building Permits", "PERMITS"},
		},
	}

	events, ok := categories[category]
	if !ok {
		return ports.InlineKeyboard{}
	}

	var rows [][]ports.InlineButton
	var currentRow []ports.InlineButton
	for i, ev := range events {
		btn := ports.InlineButton{
			Text:         ev.Label,
			CallbackData: fmt.Sprintf("imp:ev:%s", ev.Alias),
		}
		currentRow = append(currentRow, btn)
		if len(currentRow) == 2 || i == len(events)-1 {
			rows = append(rows, currentRow)
			currentRow = nil
		}
	}
	// Add back button
	rows = append(rows, []ports.InlineButton{
		{Text: "<< Back to Categories", CallbackData: "imp:back"},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// ---------------------------------------------------------------------------
// Backtest Keyboards
// ---------------------------------------------------------------------------

// BacktestMenu builds the backtest sub-command selection keyboard.
func (kb *KeyboardBuilder) BacktestMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Overview", CallbackData: "cmd:backtest:all"},
				{Text: "📋 By Signal Type", CallbackData: "cmd:backtest:signals"},
			},
			{
				{Text: "⏱ Timing", CallbackData: "cmd:backtest:timing"},
				{Text: "🔄 Walk-Forward", CallbackData: "cmd:backtest:wf"},
			},
			{
				{Text: "⚖️ Weights", CallbackData: "cmd:backtest:weights"},
				{Text: "🧠 Smart Money", CallbackData: "cmd:backtest:sm"},
			},
			{
				{Text: "📊 MFE/MAE", CallbackData: "cmd:backtest:excursion"},
				{Text: "📈 Trend Filter", CallbackData: "cmd:backtest:trend"},
			},
			// Currency row
			{
				{Text: "EUR", CallbackData: "cmd:backtest:EUR"},
				{Text: "GBP", CallbackData: "cmd:backtest:GBP"},
				{Text: "JPY", CallbackData: "cmd:backtest:JPY"},
				{Text: "AUD", CallbackData: "cmd:backtest:AUD"},
			},
			{
				{Text: "NZD", CallbackData: "cmd:backtest:NZD"},
				{Text: "CAD", CallbackData: "cmd:backtest:CAD"},
				{Text: "CHF", CallbackData: "cmd:backtest:CHF"},
				{Text: "GOLD", CallbackData: "cmd:backtest:XAU"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Seasonal Keyboards
// ---------------------------------------------------------------------------

// SeasonalMenu builds a currency selector keyboard for the /seasonal grid view.
// Provides quick-access buttons for deep-dive into individual currencies.
func (kb *KeyboardBuilder) SeasonalMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// FX Majors
			{
				{Text: "EUR", CallbackData: "cmd:seasonal:EUR"},
				{Text: "GBP", CallbackData: "cmd:seasonal:GBP"},
				{Text: "JPY", CallbackData: "cmd:seasonal:JPY"},
				{Text: "CHF", CallbackData: "cmd:seasonal:CHF"},
			},
			{
				{Text: "AUD", CallbackData: "cmd:seasonal:AUD"},
				{Text: "NZD", CallbackData: "cmd:seasonal:NZD"},
				{Text: "CAD", CallbackData: "cmd:seasonal:CAD"},
				{Text: "DXY", CallbackData: "cmd:seasonal:USD"},
			},
			// Metals & Energy
			{
				{Text: "🥇 Gold", CallbackData: "cmd:seasonal:XAU"},
				{Text: "🥈 Silver", CallbackData: "cmd:seasonal:XAG"},
				{Text: "🛢 Oil", CallbackData: "cmd:seasonal:OIL"},
				{Text: "🔶 Copper", CallbackData: "cmd:seasonal:COPPER"},
			},
			// Indices
			{
				{Text: "S&P500", CallbackData: "cmd:seasonal:SPX500"},
				{Text: "Nasdaq", CallbackData: "cmd:seasonal:NDX"},
				{Text: "Dow", CallbackData: "cmd:seasonal:DJI"},
				{Text: "Russell", CallbackData: "cmd:seasonal:RUT"},
			},
			// Bonds & Crypto
			{
				{Text: "🏛 10Y", CallbackData: "cmd:seasonal:BOND"},
				{Text: "🏛 30Y", CallbackData: "cmd:seasonal:BOND30"},
				{Text: "₿ BTC", CallbackData: "cmd:seasonal:BTC"},
				{Text: "Ξ ETH", CallbackData: "cmd:seasonal:ETH"},
			},
		},
	}
}

// SeasonalDetailMenu builds a navigation keyboard for a single-currency seasonal deep dive.
func (kb *KeyboardBuilder) SeasonalDetailMenu(currency string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "<< Grid Overview", CallbackData: "cmd:seasonal"},
				{Text: "💹 Price", CallbackData: fmt.Sprintf("cmd:price:%s", currency)},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Signal Keyboards
// ---------------------------------------------------------------------------

// COTDetailMenuWithSignals builds a COT detail keyboard with optional signal view button.
func (kb *KeyboardBuilder) COTDetailMenuWithSignals(code string, isRaw bool, signalCount int, currency string) ports.InlineKeyboard {
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

	// Signal view button if there are signals
	if signalCount > 3 {
		rows = append(rows, []ports.InlineButton{
			{Text: fmt.Sprintf("🎯 View All %d Signals", signalCount), CallbackData: fmt.Sprintf("cmd:signals:%s", currency)},
		})
	}

	// Quick-access buttons
	currencyLabel := kb.contractLabel(code, code)
	rows = append(rows, []ports.InlineButton{
		{Text: "📈 Seasonal", CallbackData: fmt.Sprintf("cmd:seasonal:%s", currencyLabel)},
		{Text: "💹 Sentiment", CallbackData: "cmd:sentiment"},
	})

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
				{Text: "🦅 Unified Outlook", CallbackData: "out:unified"},
			},
			{
				{Text: "📊 Signals", CallbackData: "cmd:signals"},
				{Text: "🏦 Macro", CallbackData: "cmd:macro"},
				{Text: "📈 Rank", CallbackData: "cmd:rank"},
			},
			{
				{Text: "📅 Calendar", CallbackData: "cmd:calendar"},
				{Text: "💹 Price", CallbackData: "cmd:price"},
				{Text: "🎯 Accuracy", CallbackData: "cmd:accuracy"},
			},
		},
	}
}

// PriceMenu builds a categorized currency selection keyboard for the /price command.
func (kb *KeyboardBuilder) PriceMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// --- FX Majors ---
			{
				{Text: "EUR", CallbackData: "cmd:price:EUR"},
				{Text: "GBP", CallbackData: "cmd:price:GBP"},
				{Text: "JPY", CallbackData: "cmd:price:JPY"},
				{Text: "CHF", CallbackData: "cmd:price:CHF"},
			},
			{
				{Text: "AUD", CallbackData: "cmd:price:AUD"},
				{Text: "NZD", CallbackData: "cmd:price:NZD"},
				{Text: "CAD", CallbackData: "cmd:price:CAD"},
				{Text: "DXY", CallbackData: "cmd:price:USD"},
			},
			// --- Metals & Energy ---
			{
				{Text: "🥇 Gold", CallbackData: "cmd:price:XAU"},
				{Text: "🥈 Silver", CallbackData: "cmd:price:XAG"},
				{Text: "🛢 Oil", CallbackData: "cmd:price:OIL"},
				{Text: "🔶 Copper", CallbackData: "cmd:price:COPPER"},
			},
			// --- Indices ---
			{
				{Text: "📈 S&P500", CallbackData: "cmd:price:SPX500"},
				{Text: "📈 Nasdaq", CallbackData: "cmd:price:NDX"},
				{Text: "📈 Dow", CallbackData: "cmd:price:DJI"},
				{Text: "📈 Russell", CallbackData: "cmd:price:RUT"},
			},
			// --- Bonds ---
			{
				{Text: "🏛 2Y", CallbackData: "cmd:price:BOND2"},
				{Text: "🏛 5Y", CallbackData: "cmd:price:BOND5"},
				{Text: "🏛 10Y", CallbackData: "cmd:price:BOND"},
				{Text: "🏛 30Y", CallbackData: "cmd:price:BOND30"},
			},
			// --- Crypto & Energy ---
			{
				{Text: "₿ BTC", CallbackData: "cmd:price:BTC"},
				{Text: "Ξ ETH", CallbackData: "cmd:price:ETH"},
				{Text: "⛽ ULSD", CallbackData: "cmd:price:ULSD"},
				{Text: "⛽ RBOB", CallbackData: "cmd:price:RBOB"},
			},
		},
	}
}
