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
// Standardized button label constants.
const (
	btnExpand  = "📖 Detail Lengkap"
	btnCompact = "📊 Compact"
)

type KeyboardBuilder struct{}

// NewKeyboardBuilder creates a new KeyboardBuilder.
func NewKeyboardBuilder() *KeyboardBuilder {
	return &KeyboardBuilder{}
}


// ---------------------------------------------------------------------------
// Standardized Button Labels
// ---------------------------------------------------------------------------

const (
	// Navigation — generic
	btnBack       = "◀ Kembali"
	btnHome       = "🏠 Menu Utama"
	btnPrevDay    = "◀ Kemarin"
	btnNextDay    = "Besok ▶"
	btnPrevWeek   = "◀ Minggu Lalu"
	btnNextWeek   = "Minggu Depan ▶"
	btnPrevMonth  = "◀ Bulan Lalu"
	btnNextMonth  = "Bulan Depan ▶"

	// Navigation — context-specific back buttons (Indonesian, per UX standard)
	btnBackRingkasan = "◀ Ringkasan"   // back to summary/overview
	btnBackDashboard = "◀ Dashboard"   // back to main section dashboard
	btnBackKategori  = "◀ Kategori"    // back to category list
	btnBackGrid      = "◀ Grid"        // back to seasonal grid overview

	// Calendar
	btnThisMonth = "Bulan Ini"

	// Actions
	btnRefresh = "🔄 Refresh"
	btnClose   = "✖ Tutup"
)


// HomeRow returns a single-row keyboard with the home button.
func (kb *KeyboardBuilder) HomeRow() []ports.InlineButton {
	return []ports.InlineButton{
		{Text: btnHome, CallbackData: "nav:home"},
	}
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

// COTCurrencySelectorWithLast is like COTCurrencySelector but prepends a shortcut
// row for the user's last viewed currency if it is non-empty.
func (kb *KeyboardBuilder) COTCurrencySelectorWithLast(analyses []domain.COTAnalysis, lastCurrency string) ports.InlineKeyboard {
	base := kb.COTCurrencySelector(analyses)
	if lastCurrency == "" {
		return base
	}
	// Find contract code for lastCurrency
	contractCode := ""
	for _, a := range analyses {
		label := kb.contractLabel(a.Contract.Name, a.Contract.Code)
		if strings.EqualFold(label, lastCurrency) || strings.EqualFold(a.Contract.Code, lastCurrency) {
			contractCode = a.Contract.Code
			break
		}
	}
	if contractCode == "" {
		return base
	}
	lastRow := []ports.InlineButton{
		{Text: "🔄 Same as last: " + strings.ToUpper(lastCurrency), CallbackData: fmt.Sprintf("cot:analysis:%s", contractCode)},
	}
	rows := append([][]ports.InlineButton{lastRow}, base.Rows...)
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
			{Text: btnPrevWeek, CallbackData: "cal:nav:prevwk:" + dateStr},
			{Text: "Harian", CallbackData: "cal:nav:day:" + dateStr},
			{Text: btnNextWeek, CallbackData: "cal:nav:nextwk:" + dateStr},
		})
	} else {
		rows = append(rows, []ports.InlineButton{
			{Text: btnPrevDay, CallbackData: "cal:nav:prev:" + dateStr},
			{Text: "Seminggu", CallbackData: "cal:nav:week:" + dateStr},
			{Text: btnNextDay, CallbackData: "cal:nav:next:" + dateStr},
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
		{Text: btnPrevMonth, CallbackData: "cal:nav:prevmonth:" + dateStr},
		{Text: btnThisMonth, CallbackData: "cal:nav:thismonth:" + dateStr},
		{Text: btnNextMonth, CallbackData: "cal:nav:nextmonth:" + dateStr},
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

	// Row 2: Composite scores & global view
	rows = append(rows, []ports.InlineButton{
		{Text: "🧮 Composites", CallbackData: "macro:composites"},
		{Text: "🌍 Global", CallbackData: "macro:global"},
	})

	// Row 3: Drill-down views
	rows = append(rows, []ports.InlineButton{
		{Text: "👷 Labor", CallbackData: "macro:labor"},
		{Text: "🔥 Inflation", CallbackData: "macro:inflation"},
	})

	// Row 4: Additional views
	row3 := []ports.InlineButton{
		{Text: "📈 Performance", CallbackData: "macro:performance"},
	}
	if isAdmin {
		row3 = append(row3, ports.InlineButton{Text: "🔄 Refresh Data", CallbackData: "macro:refresh"})
	}
	rows = append(rows, row3)

	return ports.InlineKeyboard{Rows: rows}
}

// MacroDetailMenu builds the back-navigation keyboard for macro detail/explain views.
func (kb *KeyboardBuilder) MacroDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackRingkasan, CallbackData: "macro:summary"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// MacroDrillDownMenu builds navigation keyboard for macro drill-down views (labor, inflation).
func (kb *KeyboardBuilder) MacroDrillDownMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "👷 Labor", CallbackData: "macro:labor"},
				{Text: "🔥 Inflation", CallbackData: "macro:inflation"},
			},
			{
				{Text: "🧮 Composites", CallbackData: "macro:composites"},
				{Text: btnBackRingkasan, CallbackData: "macro:summary"},
				{Text: btnHome, CallbackData: "nav:home"},
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

	// Row 11: Output Mode toggle (compact / full / minimal)
	nextMode := domain.NextOutputMode(prefs.OutputMode)
	outputLabel := fmt.Sprintf("%s → %s", domain.OutputModeLabel(prefs.OutputMode), domain.OutputModeLabel(nextMode))
	rows = append(rows, []ports.InlineButton{{
		Text:         outputLabel,
		CallbackData: "set:output_mode_toggle",
	}})

	// Row 12: Mobile sparkline mode toggle
	mobileLabel := "📱 Mobile Mode: OFF → Turn ON"
	if prefs.MobileMode {
		mobileLabel = "📱 Mobile Mode: ON → Turn OFF"
	}
	rows = append(rows, []ports.InlineButton{{
		Text:         mobileLabel,
		CallbackData: "set:mobile_toggle",
	}})

	// Row 13: View Changelog
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

	// Share button for copy-paste friendly output
	rows = append(rows, kb.ShareRow(fmt.Sprintf("share:cot:%s", code)))

	rows = append(rows, []ports.InlineButton{
		{Text: btnBackRingkasan, CallbackData: "cot:overview"}, {Text: btnHome, CallbackData: "nav:home"},
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
		{Text: btnBackKategori, CallbackData: "imp:back"}, {Text: btnHome, CallbackData: "nav:home"},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// ImpactBackMenu builds a back-to-categories button for impact detail views.
func (kb *KeyboardBuilder) ImpactBackMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackKategori, CallbackData: "imp:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Backtest Keyboards
// ---------------------------------------------------------------------------

// BacktestMenu builds the backtest sub-command selection keyboard.
// Organized into three sections: Core, Analysis, Advanced, plus currency drill-down.
func (kb *KeyboardBuilder) BacktestMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// --- Core ---
			{
				{Text: "📊 Overview", CallbackData: "cmd:backtest:all"},
				{Text: "📋 By Signal Type", CallbackData: "cmd:backtest:signals"},
			},
			// --- Analysis ---
			{
				{Text: "⏱ Timing", CallbackData: "cmd:backtest:timing"},
				{Text: "🔄 Walk-Forward", CallbackData: "cmd:backtest:wf"},
				{Text: "⚖️ Weights", CallbackData: "cmd:backtest:weights"},
			},
			{
				{Text: "🧠 Smart Money", CallbackData: "cmd:backtest:sm"},
				{Text: "📊 MFE/MAE", CallbackData: "cmd:backtest:excursion"},
				{Text: "📈 Trend", CallbackData: "cmd:backtest:trend"},
			},
			// --- Advanced ---
			{
				{Text: "🎯 Baseline", CallbackData: "cmd:backtest:baseline"},
				{Text: "🌐 Regime", CallbackData: "cmd:backtest:regime"},
				{Text: "📐 Matrix", CallbackData: "cmd:backtest:matrix"},
			},
			{
				{Text: "🎲 Monte Carlo", CallbackData: "cmd:backtest:mc"},
				{Text: "📈 Portfolio", CallbackData: "cmd:backtest:portfolio"},
				{Text: "💰 Cost", CallbackData: "cmd:backtest:cost"},
			},
			{
				{Text: "🔗 Dedup", CallbackData: "cmd:backtest:dedup"},
				{Text: "🎰 Ruin", CallbackData: "cmd:backtest:ruin"},
				{Text: "🔍 Audit", CallbackData: "cmd:backtest:audit"},
			},
			// --- Currency drill-down ---
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
			{
				{Text: "⛽ ULSD", CallbackData: "cmd:seasonal:ULSD"},
				{Text: "⛽ RBOB", CallbackData: "cmd:seasonal:RBOB"},
			},
			// Indices
			{
				{Text: "S&P500", CallbackData: "cmd:seasonal:SPX500"},
				{Text: "Nasdaq", CallbackData: "cmd:seasonal:NDX"},
				{Text: "Dow", CallbackData: "cmd:seasonal:DJI"},
				{Text: "Russell", CallbackData: "cmd:seasonal:RUT"},
			},
			// Bonds
			{
				{Text: "🏛 2Y", CallbackData: "cmd:seasonal:BOND2"},
				{Text: "🏛 5Y", CallbackData: "cmd:seasonal:BOND5"},
				{Text: "🏛 10Y", CallbackData: "cmd:seasonal:BOND"},
				{Text: "🏛 30Y", CallbackData: "cmd:seasonal:BOND30"},
			},
			// Crypto & Crosses
			{
				{Text: "₿ BTC", CallbackData: "cmd:seasonal:BTC"},
				{Text: "Ξ ETH", CallbackData: "cmd:seasonal:ETH"},
			},
			{
				{Text: "XAU/EUR", CallbackData: "cmd:seasonal:XAUEUR"},
				{Text: "XAU/GBP", CallbackData: "cmd:seasonal:XAUGBP"},
				{Text: "XAG/EUR", CallbackData: "cmd:seasonal:XAGEUR"},
				{Text: "XAG/GBP", CallbackData: "cmd:seasonal:XAGGBP"},
			},
		},
	}
}

// SeasonalDetailMenu builds a navigation keyboard for a single-currency seasonal deep dive.
func (kb *KeyboardBuilder) SeasonalDetailMenu(currency string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackGrid, CallbackData: "cmd:seasonal"},
				{Text: "💹 Price", CallbackData: fmt.Sprintf("cmd:price:%s", currency)},
				{Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Bias Keyboards
// ---------------------------------------------------------------------------

// COTDetailMenuWithBias builds a COT detail keyboard with optional bias view button.
func (kb *KeyboardBuilder) COTDetailMenuWithBias(code string, isRaw bool, signalCount int, currency string) ports.InlineKeyboard {
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

	// Bias view button if there are biases
	if signalCount > 3 {
		rows = append(rows, []ports.InlineButton{
			{Text: fmt.Sprintf("🎯 View All %d Biases", signalCount), CallbackData: fmt.Sprintf("cmd:bias:%s", currency)},
		})
	}

	// Quick-access buttons
	currencyLabel := kb.contractLabel(code, code)
	rows = append(rows, []ports.InlineButton{
		{Text: "📈 Seasonal", CallbackData: fmt.Sprintf("cmd:seasonal:%s", currencyLabel)},
		{Text: "💹 Sentiment", CallbackData: "cmd:sentiment"},
	})

	// Share button for copy-paste friendly output
	rows = append(rows, kb.ShareRow(fmt.Sprintf("share:cot:%s", code)))

	rows = append(rows, []ports.InlineButton{
		{Text: btnBackRingkasan, CallbackData: "cot:overview"}, {Text: btnHome, CallbackData: "nav:home"},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// MainMenu builds a quick-access keyboard for the main bot features.
func (kb *KeyboardBuilder) MainMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 COT Analysis", CallbackData: "nav:cot"},
				{Text: "🦅 Unified Outlook", CallbackData: "out:unified"},
			},
			{
				{Text: "🏦 Macro", CallbackData: "cmd:macro"},
				{Text: "📅 Calendar", CallbackData: "cmd:calendar"},
				{Text: "💹 Price", CallbackData: "cmd:price"},
			},
			{
				{Text: "📈 Rank", CallbackData: "cmd:rank"},
				{Text: "📊 Bias", CallbackData: "cmd:bias"},
				{Text: "🎯 Accuracy", CallbackData: "cmd:accuracy"},
			},
			{
				{Text: "⚡ Alpha Engine", CallbackData: "alpha:back"},
			},
			{
				{Text: "🔬 Quant", CallbackData: "cmd:quant"},
				{Text: "📊 Volume Profile", CallbackData: "cmd:vp"},
			},
		},
	}
}

// AlphaMenu builds the inline keyboard for the unified /alpha dashboard.
func (kb *KeyboardBuilder) AlphaMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Factor Ranking", CallbackData: "alpha:factors"},
				{Text: "🎯 Playbook", CallbackData: "alpha:playbook"},
			},
			{
				{Text: "🌡 Portfolio Heat", CallbackData: "alpha:heat"},
				{Text: "📈 RankX", CallbackData: "alpha:rankx"},
			},
			{
				{Text: "🔄 Regime & Transisi", CallbackData: "alpha:transition"},
				{Text: "⚡ Crypto Alpha", CallbackData: "alpha:crypto"},
			},
			{
				{Text: "🔄 Refresh Data", CallbackData: "alpha:refresh"},
			},
		},
	}
}

// AlphaDetailMenu builds the back-navigation keyboard for alpha detail views.
func (kb *KeyboardBuilder) AlphaDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackRingkasan, CallbackData: "alpha:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// AlphaCryptoDetailMenu builds the back-navigation keyboard for alpha crypto detail views
// with individual crypto symbol buttons.
func (kb *KeyboardBuilder) AlphaCryptoDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "₿ BTC", CallbackData: "alpha:crypto:BTC"},
				{Text: "Ξ ETH", CallbackData: "alpha:crypto:ETH"},
				{Text: "◎ SOL", CallbackData: "alpha:crypto:SOL"},
				{Text: "🔶 BNB", CallbackData: "alpha:crypto:BNB"},
			},
			{
				{Text: btnBackRingkasan, CallbackData: "alpha:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// CTA Keyboards
// ---------------------------------------------------------------------------

// CTAMenu builds the inline keyboard for the /cta dashboard.
func (kb *KeyboardBuilder) CTAMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 15m", CallbackData: "cta:tf:15m"},
				{Text: "📊 30m", CallbackData: "cta:tf:30m"},
				{Text: "📊 1H", CallbackData: "cta:tf:1h"},
				{Text: "📊 4H", CallbackData: "cta:tf:4h"},
			},
			{
				{Text: "📊 6H", CallbackData: "cta:tf:6h"},
				{Text: "📊 12H", CallbackData: "cta:tf:12h"},
				{Text: "📊 Daily", CallbackData: "cta:tf:daily"},
			},
			{
				{Text: "🏯 Ichimoku", CallbackData: "cta:ichi"},
				{Text: "📐 Fibonacci", CallbackData: "cta:fib"},
				{Text: "🕯 Patterns", CallbackData: "cta:patterns"},
			},
			{
				{Text: "⚡ Confluence", CallbackData: "cta:confluence"},
				{Text: "📱 Multi-TF", CallbackData: "cta:mtf"},
				{Text: "🎯 Zones", CallbackData: "cta:zones"},
			},
			{
				{Text: "🔄 Refresh", CallbackData: "cta:refresh"},
			},
		},
	}
}

// CTADetailMenu builds the back-navigation keyboard for CTA detail views.
func (kb *KeyboardBuilder) CTADetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackRingkasan, CallbackData: "cta:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// CTATimeframeMenu builds the timeframe selection keyboard for CTA.
func (kb *KeyboardBuilder) CTATimeframeMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 15m", CallbackData: "cta:tf:15m"},
				{Text: "📊 30m", CallbackData: "cta:tf:30m"},
				{Text: "📊 1H", CallbackData: "cta:tf:1h"},
				{Text: "📊 4H", CallbackData: "cta:tf:4h"},
			},
			{
				{Text: "📊 6H", CallbackData: "cta:tf:6h"},
				{Text: "📊 12H", CallbackData: "cta:tf:12h"},
				{Text: "📊 Daily", CallbackData: "cta:tf:daily"},
			},
			{
				{Text: btnBackRingkasan, CallbackData: "cta:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// CTABTMenu builds the inline keyboard for the /ctabt backtest dashboard.
func (kb *KeyboardBuilder) CTABTMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Daily", CallbackData: "ctabt:daily"},
				{Text: "📊 12H", CallbackData: "ctabt:12h"},
				{Text: "📊 6H", CallbackData: "ctabt:6h"},
			},
			{
				{Text: "📊 4H", CallbackData: "ctabt:4h"},
				{Text: "📊 1H", CallbackData: "ctabt:1h"},
				{Text: "📊 30M", CallbackData: "ctabt:30m"},
				{Text: "📊 15M", CallbackData: "ctabt:15m"},
			},
			{
				{Text: "Grade: A", CallbackData: "ctabt:gradeA"},
				{Text: "Grade: B", CallbackData: "ctabt:gradeB"},
				{Text: "Grade: C", CallbackData: "ctabt:gradeC"},
			},
			{
				{Text: "📋 Detail Trades", CallbackData: "ctabt:trades"},
				{Text: "🔄 Refresh", CallbackData: "ctabt:refresh"},
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
			// --- Cross Pairs ---
			{
				{Text: "XAU/EUR", CallbackData: "cmd:price:XAUEUR"},
				{Text: "XAU/GBP", CallbackData: "cmd:price:XAUGBP"},
				{Text: "XAG/EUR", CallbackData: "cmd:price:XAGEUR"},
				{Text: "XAG/GBP", CallbackData: "cmd:price:XAGGBP"},
			},
		},
	}
}

// QuantMenu builds the main /quant dashboard inline keyboard.
func (kb *KeyboardBuilder) QuantMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Stats", CallbackData: "quant:stats"},
				{Text: "📈 Volatility", CallbackData: "quant:garch"},
				{Text: "🔗 Correlation", CallbackData: "quant:corr"},
			},
			{
				{Text: "📅 Seasonal", CallbackData: "quant:seasonal"},
				{Text: "🔄 Mean Revert", CallbackData: "quant:meanrevert"},
				{Text: "⚡ Granger", CallbackData: "quant:granger"},
			},
			{
				{Text: "🎭 Regime (HMM)", CallbackData: "quant:regime"},
				{Text: "🔗 Cointegration", CallbackData: "quant:coint"},
			},
			{
				{Text: "🧬 PCA", CallbackData: "quant:pca"},
				{Text: "🌐 VAR", CallbackData: "quant:var"},
				{Text: "⚠️ Risk", CallbackData: "quant:risk"},
			},
			{
				{Text: "📋 Full Report", CallbackData: "quant:full"},
			},
			{
				{Text: "15m", CallbackData: "quant:tf:15m"},
				{Text: "30m", CallbackData: "quant:tf:30m"},
				{Text: "1H", CallbackData: "quant:tf:1h"},
				{Text: "4H", CallbackData: "quant:tf:4h"},
			},
			{
				{Text: "6H", CallbackData: "quant:tf:6h"},
				{Text: "12H", CallbackData: "quant:tf:12h"},
				{Text: "📊 Daily", CallbackData: "quant:tf:daily"},
			},
		},
	}
}

// QuantDetailMenu builds the back-navigation keyboard for quant detail views.
func (kb *KeyboardBuilder) QuantDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackDashboard, CallbackData: "quant:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// VPMenu builds the main /vp Volume Profile dashboard keyboard.
func (kb *KeyboardBuilder) VPMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// Analysis modes
			{
				{Text: "📊 Profile", CallbackData: "vp:profile"},
				{Text: "🕐 Session", CallbackData: "vp:session"},
				{Text: "📐 Shape", CallbackData: "vp:shape"},
			},
			{
				{Text: "🔀 Composite", CallbackData: "vp:composite"},
				{Text: "📏 VWAP", CallbackData: "vp:vwap"},
				{Text: "⏱ TPO", CallbackData: "vp:tpo"},
			},
			{
				{Text: "📈 Delta", CallbackData: "vp:delta"},
				{Text: "🏛 Auction", CallbackData: "vp:auction"},
				{Text: "🎯 Confluence", CallbackData: "vp:confluence"},
			},
			{
				{Text: "📋 Full Report", CallbackData: "vp:full"},
			},
			// TF selector
			{
				{Text: "15m", CallbackData: "vp:tf:15m"},
				{Text: "30m", CallbackData: "vp:tf:30m"},
				{Text: "1H", CallbackData: "vp:tf:1h"},
				{Text: "4H", CallbackData: "vp:tf:4h"},
			},
			{
				{Text: "6H", CallbackData: "vp:tf:6h"},
				{Text: "12H", CallbackData: "vp:tf:12h"},
				{Text: "📅 Daily", CallbackData: "vp:tf:daily"},
				{Text: "🔄 Refresh", CallbackData: "vp:refresh"},
			},
		},
	}
}

// VPDetailMenu builds the back-navigation keyboard for VP detail views.
func (kb *KeyboardBuilder) VPDetailMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackDashboard, CallbackData: "vp:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Help Keyboards — Smart /help with category navigation
// ---------------------------------------------------------------------------

// HelpCategoryMenu builds the top-level help category selector.
func (kb *KeyboardBuilder) HelpCategoryMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Market & COT", CallbackData: "help:market"},
				{Text: "🔬 Research & Alpha", CallbackData: "help:research"},
			},
			{
				{Text: "🧠 AI & Outlook", CallbackData: "help:ai"},
				{Text: "⚡ Signals & Alerts", CallbackData: "help:signals"},
			},
			{
				{Text: "⚙️ Settings", CallbackData: "help:settings"},
				{Text: "⚡ Shortcuts", CallbackData: "help:shortcuts"},
			},
			{
				{Text: "🆕 What's New", CallbackData: "help:changelog"},
			},
		},
	}
}

// HelpCategoryMenuWithAdmin builds the top-level help category selector with admin option.
func (kb *KeyboardBuilder) HelpCategoryMenuWithAdmin() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "📊 Market & COT", CallbackData: "help:market"},
				{Text: "🔬 Research & Alpha", CallbackData: "help:research"},
			},
			{
				{Text: "🧠 AI & Outlook", CallbackData: "help:ai"},
				{Text: "⚡ Signals & Alerts", CallbackData: "help:signals"},
			},
			{
				{Text: "⚙️ Settings", CallbackData: "help:settings"},
				{Text: "⚡ Shortcuts", CallbackData: "help:shortcuts"},
			},
			{
				{Text: "🔐 Admin", CallbackData: "help:admin"},
				{Text: "🆕 What's New", CallbackData: "help:changelog"},
			},
		},
	}
}

// HelpSubMenu builds the back button for help sub-category views.
func (kb *KeyboardBuilder) HelpSubMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: btnBackRingkasan, CallbackData: "help:back"}, {Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// VPSymbolMenu builds a symbol selector for /vp (no argument).
func (kb *KeyboardBuilder) VPSymbolMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			// FX Majors
			{
				{Text: "EUR", CallbackData: "vp:sym:EUR"},
				{Text: "GBP", CallbackData: "vp:sym:GBP"},
				{Text: "JPY", CallbackData: "vp:sym:JPY"},
				{Text: "CHF", CallbackData: "vp:sym:CHF"},
			},
			{
				{Text: "AUD", CallbackData: "vp:sym:AUD"},
				{Text: "NZD", CallbackData: "vp:sym:NZD"},
				{Text: "CAD", CallbackData: "vp:sym:CAD"},
				{Text: "DXY", CallbackData: "vp:sym:USD"},
			},
			// Metals & Energy
			{
				{Text: "🥇 Gold", CallbackData: "vp:sym:XAU"},
				{Text: "🥈 Silver", CallbackData: "vp:sym:XAG"},
				{Text: "🛢 Oil", CallbackData: "vp:sym:OIL"},
				{Text: "🔶 Copper", CallbackData: "vp:sym:COPPER"},
			},
			// Indices
			{
				{Text: "S&P500", CallbackData: "vp:sym:SPX500"},
				{Text: "Nasdaq", CallbackData: "vp:sym:NDX"},
				{Text: "Dow", CallbackData: "vp:sym:DJI"},
				{Text: "Russell", CallbackData: "vp:sym:RUT"},
			},
			// Bonds & Crypto
			{
				{Text: "🏛 10Y", CallbackData: "vp:sym:BOND"},
				{Text: "🏛 30Y", CallbackData: "vp:sym:BOND30"},
				{Text: "₿ BTC", CallbackData: "vp:sym:BTC"},
				{Text: "Ξ ETH", CallbackData: "vp:sym:ETH"},
			},
			// Energy & Crosses
			{
				{Text: "⛽ ULSD", CallbackData: "vp:sym:ULSD"},
				{Text: "⛽ RBOB", CallbackData: "vp:sym:RBOB"},
				{Text: "XAU/EUR", CallbackData: "vp:sym:XAUEUR"},
				{Text: "XAU/GBP", CallbackData: "vp:sym:XAUGBP"},
			},
		},
	}
}

// CTASymbolMenu builds a symbol selector for /cta.
func (kb *KeyboardBuilder) CTASymbolMenu() ports.InlineKeyboard {
	return kb.buildSymbolMenu("cta")
}

// CTABTSymbolMenu builds a symbol selector for /ctabt.
func (kb *KeyboardBuilder) CTABTSymbolMenu() ports.InlineKeyboard {
	return kb.buildSymbolMenu("ctabt")
}

// QuantSymbolMenu builds a symbol selector for /quant.
func (kb *KeyboardBuilder) QuantSymbolMenu() ports.InlineKeyboard {
	return kb.buildSymbolMenu("quant")
}

// buildSymbolMenu creates a reusable symbol selector grid.
func (kb *KeyboardBuilder) buildSymbolMenu(prefix string) ports.InlineKeyboard {
	p := func(sym string) string { return prefix + ":sym:" + sym }
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "EUR", CallbackData: p("EUR")},
				{Text: "GBP", CallbackData: p("GBP")},
				{Text: "JPY", CallbackData: p("JPY")},
				{Text: "CHF", CallbackData: p("CHF")},
			},
			{
				{Text: "AUD", CallbackData: p("AUD")},
				{Text: "NZD", CallbackData: p("NZD")},
				{Text: "CAD", CallbackData: p("CAD")},
				{Text: "DXY", CallbackData: p("USD")},
			},
			{
				{Text: "🥇 Gold", CallbackData: p("XAU")},
				{Text: "🥈 Silver", CallbackData: p("XAG")},
				{Text: "🛢 Oil", CallbackData: p("OIL")},
				{Text: "🔶 Copper", CallbackData: p("COPPER")},
			},
			{
				{Text: "S&P500", CallbackData: p("SPX500")},
				{Text: "Nasdaq", CallbackData: p("NDX")},
				{Text: "Dow", CallbackData: p("DJI")},
				{Text: "Russell", CallbackData: p("RUT")},
			},
			{
				{Text: "🏛 10Y", CallbackData: p("BOND")},
				{Text: "🏛 30Y", CallbackData: p("BOND30")},
				{Text: "₿ BTC", CallbackData: p("BTC")},
				{Text: "Ξ ETH", CallbackData: p("ETH")},
			},
			{
				{Text: "⛽ ULSD", CallbackData: p("ULSD")},
				{Text: "⛽ RBOB", CallbackData: p("RBOB")},
				{Text: "XAU/EUR", CallbackData: p("XAUEUR")},
				{Text: "XAU/GBP", CallbackData: p("XAUGBP")},
			},
		},
	}
}

// buildSymbolMenuWithLast is like buildSymbolMenu but prepends a "Same as last" row
// if lastCurrency is non-empty and recognized.
func (kb *KeyboardBuilder) buildSymbolMenuWithLast(prefix, lastCurrency string) ports.InlineKeyboard {
	base := kb.buildSymbolMenu(prefix)
	if lastCurrency == "" {
		return base
	}
	// Build label for last currency button
	label := "🔄 Same as last: " + lastCurrency
	lastRow := []ports.InlineButton{
		{Text: label, CallbackData: prefix + ":sym:" + lastCurrency},
	}
	// Prepend the shortcut row
	rows := append([][]ports.InlineButton{lastRow}, base.Rows...)
	return ports.InlineKeyboard{Rows: rows}
}

// CTASymbolMenuWithLast builds a CTA symbol menu with a "same as last" shortcut.
func (kb *KeyboardBuilder) CTASymbolMenuWithLast(lastCurrency string) ports.InlineKeyboard {
	return kb.buildSymbolMenuWithLast("cta", lastCurrency)
}

// QuantSymbolMenuWithLast builds a Quant symbol menu with a "same as last" shortcut.
func (kb *KeyboardBuilder) QuantSymbolMenuWithLast(lastCurrency string) ports.InlineKeyboard {
	return kb.buildSymbolMenuWithLast("quant", lastCurrency)
}

// ---------------------------------------------------------------------------
// Onboarding — Role Selector + Starter Kits
// ---------------------------------------------------------------------------

// OnboardingRoleMenu builds the experience-level selector for new users.
func (kb *KeyboardBuilder) OnboardingRoleMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🌱 Pemula", CallbackData: "onboard:beginner"},
			},
			{
				{Text: "📈 Intermediate", CallbackData: "onboard:intermediate"},
			},
			{
				{Text: "🏛 Pro / Institutional", CallbackData: "onboard:pro"},
			},
		},
	}
}

// StarterKitMenu returns a role-appropriate starter keyboard.
func (kb *KeyboardBuilder) StarterKitMenu(level string) ports.InlineKeyboard {
	switch level {
	case "beginner":
		return ports.InlineKeyboard{
			Rows: [][]ports.InlineButton{
				{
					{Text: "📊 COT (Posisi Big Player)", CallbackData: "nav:cot"},
					{Text: "📅 Kalender Ekonomi", CallbackData: "cmd:calendar"},
				},
				{
					{Text: "💹 Cek Harga", CallbackData: "cmd:price"},
					{Text: "📈 Ranking Mata Uang", CallbackData: "cmd:rank"},
				},
				{
					{Text: "📖 Lihat Semua Command", CallbackData: "onboard:showhelp"},
				},
			},
		}
	case "intermediate":
		return ports.InlineKeyboard{
			Rows: [][]ports.InlineButton{
				{
					{Text: "📊 COT Analysis", CallbackData: "nav:cot"},
					{Text: "🦅 AI Outlook", CallbackData: "out:unified"},
				},
				{
					{Text: "📉 CTA Dashboard", CallbackData: "cmd:cta"},
					{Text: "🔬 Quant Analysis", CallbackData: "cmd:quant"},
				},
				{
					{Text: "🏦 Macro Regime", CallbackData: "cmd:macro"},
					{Text: "📊 Bias", CallbackData: "cmd:bias"},
				},
				{
					{Text: "📖 Lihat Semua Command", CallbackData: "onboard:showhelp"},
				},
			},
		}
	default: // pro
		return ports.InlineKeyboard{
			Rows: [][]ports.InlineButton{
				{
					{Text: "⚡ Alpha Engine", CallbackData: "alpha:back"},
					{Text: "🦅 AI Outlook", CallbackData: "out:unified"},
				},
				{
					{Text: "📊 Volume Profile", CallbackData: "cmd:vp"},
					{Text: "🔬 Quant", CallbackData: "cmd:quant"},
				},
				{
					{Text: "📉 CTA + Backtest", CallbackData: "cmd:cta"},
					{Text: "🏦 Macro", CallbackData: "cmd:macro"},
				},
				{
					{Text: "📖 Lihat Semua Command", CallbackData: "onboard:showhelp"},
				},
			},
		}
	}
}

// ---------------------------------------------------------------------------
// Share Buttons
// ---------------------------------------------------------------------------

// ShareRow returns a single-button row with a share/forward button.
// callbackBase should be e.g. "share:cot:EUR", "share:outlook:latest".
func (kb *KeyboardBuilder) ShareRow(callbackBase string) []ports.InlineButton {
	return []ports.InlineButton{
		{Text: "📤 Share", CallbackData: callbackBase},
	}
}

// ---------------------------------------------------------------------------
// Alert Action Keyboards
// ---------------------------------------------------------------------------

// AlertActionKeyboard builds an inline keyboard for alert messages,
// providing quick actions: view details, mute this alert type, or open settings.
//
// alertType identifies the alert category (e.g. "cot", "fred", "signal").
// detailCmd is the suggested command to view details (e.g. "/cot", "/macro").
func (kb *KeyboardBuilder) AlertActionKeyboard(alertType, detailCmd string) ports.InlineKeyboard {
	rows := [][]ports.InlineButton{
		{
			{Text: "📊 Lihat Detail", CallbackData: "cmd:" + detailCmd},
			{Text: "🔕 Matikan Alert Ini", CallbackData: "alert:off:" + alertType},
		},
		{
			{Text: "⚙️ Pengaturan Alert", CallbackData: "set:alerts"},
		},
	}
	return ports.InlineKeyboard{Rows: rows}
}

// SignalAlertKeyboard builds an inline keyboard for strong signal alert messages.
// currency is the COT currency code, e.g. "EUR".
func (kb *KeyboardBuilder) SignalAlertKeyboard(currency string) ports.InlineKeyboard {
	rows := [][]ports.InlineButton{
		{
			{Text: "📊 Detail COT", CallbackData: "cot:" + currency},
			{Text: "🎯 Bias " + currency, CallbackData: "cmd:bias " + currency},
		},
		{
			{Text: "🔕 Matikan Signal Alert", CallbackData: "alert:off:signal"},
			{Text: "⚙️ Pengaturan", CallbackData: "set:alerts"},
		},
	}
	return ports.InlineKeyboard{Rows: rows}
}

// ---------------------------------------------------------------------------
// Related "Next Steps" Command Suggestions
// ---------------------------------------------------------------------------

// relatedCommands maps a command name to 2–3 related commands.
// Each entry is {label, callbackBase} where callbackBase uses "cmd:" prefix.
var relatedCommands = map[string][]struct {
	Label    string
	Callback string
}{
	"cot":         {{Label: "📈 Bias", Callback: "bias"}, {Label: "📊 Rank", Callback: "rank"}, {Label: "🔬 Alpha", Callback: "alpha"}},
	"bias":        {{Label: "📉 COT", Callback: "cot"}, {Label: "🎯 CTA", Callback: "cta"}, {Label: "🌐 Macro", Callback: "macro"}},
	"cta":         {{Label: "📊 Quant", Callback: "quant"}, {Label: "🔑 Levels", Callback: "levels"}, {Label: "🎯 Playbook", Callback: "playbook"}},
	"macro":       {{Label: "📅 Calendar", Callback: "calendar"}, {Label: "🔄 Transition", Callback: "transition"}, {Label: "📊 Sentiment", Callback: "sentiment"}},
	"quant":       {{Label: "📈 CTA", Callback: "cta"}, {Label: "📊 Backtest", Callback: "backtest"}, {Label: "📈 Price", Callback: "price"}},
	"calendar":    {{Label: "💥 Impact", Callback: "impact"}, {Label: "🌐 Macro", Callback: "macro"}, {Label: "📈 Price", Callback: "price"}},
	"gex":         {{Label: "🔬 Alpha", Callback: "alpha"}, {Label: "📊 Sentiment", Callback: "sentiment"}, {Label: "📈 CryptoAlpha", Callback: "cryptoalpha"}},
	"sentiment":   {{Label: "🌐 Macro", Callback: "macro"}, {Label: "📈 Bias", Callback: "bias"}, {Label: "📊 Rank", Callback: "rank"}},
	"price":       {{Label: "🔑 Levels", Callback: "levels"}, {Label: "📊 Quant", Callback: "quant"}, {Label: "🎯 CTA", Callback: "cta"}},
	"levels":      {{Label: "📈 Price", Callback: "price"}, {Label: "🎯 CTA", Callback: "cta"}, {Label: "📊 Quant", Callback: "quant"}},
	"alpha":       {{Label: "📊 Rank", Callback: "rank"}, {Label: "📈 Bias", Callback: "bias"}, {Label: "🎯 CTA", Callback: "cta"}},
	"rank":        {{Label: "📈 Bias", Callback: "bias"}, {Label: "📉 COT", Callback: "cot"}, {Label: "🔬 Alpha", Callback: "alpha"}},
	"outlook":     {{Label: "🌐 Macro", Callback: "macro"}, {Label: "📅 Calendar", Callback: "calendar"}, {Label: "📈 Bias", Callback: "bias"}},
	"impact":      {{Label: "📅 Calendar", Callback: "calendar"}, {Label: "🌐 Macro", Callback: "macro"}},
	"seasonal":    {{Label: "📉 COT", Callback: "cot"}, {Label: "📊 Backtest", Callback: "backtest"}, {Label: "📈 Bias", Callback: "bias"}},
	"backtest":    {{Label: "📊 Quant", Callback: "quant"}, {Label: "🎯 CTA", Callback: "cta"}, {Label: "📈 Seasonal", Callback: "seasonal"}},
	"intermarket": {{Label: "🌐 Macro", Callback: "macro"}, {Label: "📈 Price", Callback: "price"}, {Label: "📊 Sentiment", Callback: "sentiment"}},
}

// RelatedCommandsRow returns a keyboard row with 2–3 related command buttons.
// command is the base command name (e.g. "cot", "bias").
// currency, if non-empty, is appended to each callback (e.g. "cmd:bias:EUR").
func (kb *KeyboardBuilder) RelatedCommandsRow(command, currency string) []ports.InlineButton {
	related, ok := relatedCommands[command]
	if !ok {
		return nil
	}
	row := make([]ports.InlineButton, 0, len(related))
	for _, r := range related {
		cb := "cmd:" + r.Callback
		label := r.Label
		if currency != "" {
			cb += ":" + currency
			label += " " + currency
		}
		row = append(row, ports.InlineButton{Text: label, CallbackData: cb})
	}
	return row
}

// RelatedCommandsKeyboard returns a full keyboard with just the related row.
// Useful for commands that don't have their own keyboard.
func (kb *KeyboardBuilder) RelatedCommandsKeyboard(command, currency string) ports.InlineKeyboard {
	row := kb.RelatedCommandsRow(command, currency)
	if len(row) == 0 {
		return ports.InlineKeyboard{}
	}
	return ports.InlineKeyboard{Rows: [][]ports.InlineButton{row}}}
