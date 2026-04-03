package telegram

import (
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

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

// ---------------------------------------------------------------------------
// Outlook & Macro Keyboards
// ---------------------------------------------------------------------------

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
