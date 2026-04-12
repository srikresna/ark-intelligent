package telegram

import (
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

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
// Error Retry Keyboard
// ---------------------------------------------------------------------------

// ErrorRetryKeyboard returns a keyboard with a retry button that re-executes
// the given command (with args) and a home button. Reuses the existing
// "cmd:" callback prefix so cbQuickCommand handles the re-execution.
// Example: ErrorRetryKeyboard("wyckoff", "EUR 4h") -> callback "cmd:wyckoff:EUR 4h"
func (kb *KeyboardBuilder) ErrorRetryKeyboard(command, args string) ports.InlineKeyboard {
	cb := "cmd:" + command
	if args != "" {
		cb += ":" + args
	}
	retryRow := []ports.InlineButton{
		{Text: "🔄 Coba Lagi", CallbackData: cb},
		{Text: btnHome, CallbackData: "nav:home"},
	}
	return ports.InlineKeyboard{Rows: [][]ports.InlineButton{retryRow}}
}
