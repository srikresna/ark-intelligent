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

	// Row 3: View Changelog
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
