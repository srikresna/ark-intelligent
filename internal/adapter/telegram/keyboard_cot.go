package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

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
