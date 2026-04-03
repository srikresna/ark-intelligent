package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Main Menu
// ---------------------------------------------------------------------------

// MainMenu builds a quick-access keyboard for the main bot features.
// If pins is non-empty, a pinned-commands row is prepended (max 4 buttons).
func (kb *KeyboardBuilder) MainMenu(pins []string) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	// Pinned commands row (TASK-078)
	if len(pins) > 0 {
		var pinnedRow []ports.InlineButton
		for _, pin := range pins {
			label := "⭐ " + strings.ToUpper(pin)
			// Truncate long labels to keep buttons readable
			if len(label) > 20 {
				label = label[:20]
			}
			cb := "cmd:" + strings.ReplaceAll(pin, " ", ":")
			pinnedRow = append(pinnedRow, ports.InlineButton{
				Text:         label,
				CallbackData: cb,
			})
		}
		rows = append(rows, pinnedRow)
	}

	rows = append(rows,
		[]ports.InlineButton{
			{Text: "📊 COT Analysis", CallbackData: "nav:cot"},
			{Text: "🦅 Unified Outlook", CallbackData: "out:unified"},
		},
		[]ports.InlineButton{
			{Text: "🏦 Macro", CallbackData: "cmd:macro"},
			{Text: "📅 Calendar", CallbackData: "cmd:calendar"},
			{Text: "💹 Price", CallbackData: "cmd:price"},
		},
		[]ports.InlineButton{
			{Text: "📈 Rank", CallbackData: "cmd:rank"},
			{Text: "📊 Bias", CallbackData: "cmd:bias"},
			{Text: "🎯 Accuracy", CallbackData: "cmd:accuracy"},
		},
		[]ports.InlineButton{
			{Text: "⚡ Alpha Engine", CallbackData: "alpha:back"},
		},
		[]ports.InlineButton{
			{Text: "🔬 Quant", CallbackData: "cmd:quant"},
			{Text: "📊 Volume Profile", CallbackData: "cmd:vp"},
		},
	)

	return ports.InlineKeyboard{Rows: rows}
}

// ---------------------------------------------------------------------------
// Session Analysis Keyboards
// ---------------------------------------------------------------------------

// SessionMenu builds a currency selector keyboard for the /session command.
func (kb *KeyboardBuilder) SessionMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "EUR", CallbackData: "cmd:session:EUR"},
				{Text: "GBP", CallbackData: "cmd:session:GBP"},
				{Text: "JPY", CallbackData: "cmd:session:JPY"},
				{Text: "CHF", CallbackData: "cmd:session:CHF"},
			},
			{
				{Text: "AUD", CallbackData: "cmd:session:AUD"},
				{Text: "NZD", CallbackData: "cmd:session:NZD"},
				{Text: "CAD", CallbackData: "cmd:session:CAD"},
				{Text: "DXY", CallbackData: "cmd:session:USD"},
			},
			{
				{Text: "🥇 Gold", CallbackData: "cmd:session:XAU"},
				{Text: "₿ BTC", CallbackData: "cmd:session:BTC"},
				{Text: "Ξ ETH", CallbackData: "cmd:session:ETH"},
			},
			{
				{Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// SessionDetailMenu builds a navigation keyboard for a single-currency session view.
func (kb *KeyboardBuilder) SessionDetailMenu(currency string) ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "◀ Grid", CallbackData: "cmd:session"},
				{Text: "💹 Price", CallbackData: fmt.Sprintf("cmd:price:%s", currency)},
				{Text: "📈 Seasonal", CallbackData: fmt.Sprintf("cmd:seasonal:%s", currency)},
				{Text: btnHome, CallbackData: "nav:home"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Elliott Wave Keyboard
// ---------------------------------------------------------------------------

// ElliottKeyboard returns the inline keyboard for the /elliott command,
// showing timeframe toggle buttons for the given symbol.
func (kb *KeyboardBuilder) ElliottKeyboard(symbol, currentTF string) ports.InlineKeyboard {
	timeframes := []struct {
		Label string
		TF    string
	}{
		{"Daily", "daily"},
		{"4H", "4h"},
		{"1H", "1h"},
	}

	var tfRow []ports.InlineButton
	for _, tf := range timeframes {
		label := tf.Label
		if tf.TF == currentTF {
			label = "✅ " + label
		}
		tfRow = append(tfRow, ports.InlineButton{
			Text:         label,
			CallbackData: "cmd:elliott:" + symbol + " " + tf.TF,
		})
	}

	relatedRow := kb.RelatedCommandsRow("elliott", symbol)

	rows := [][]ports.InlineButton{tfRow}
	if len(relatedRow) > 0 {
		rows = append(rows, relatedRow)
	}
	rows = append(rows, []ports.InlineButton{{Text: btnHome, CallbackData: "nav:home"}})

	return ports.InlineKeyboard{Rows: rows}
}

// ---------------------------------------------------------------------------
// Helper Methods (used across multiple keyboard files)
// ---------------------------------------------------------------------------

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
