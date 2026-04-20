package telegram

import (
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// Help Keyboards — Smart /help with category navigation
// ---------------------------------------------------------------------------

// pinnedRow builds an optional row of pinned command buttons (TASK-078).
// Returns nil if pins is empty.
func (kb *KeyboardBuilder) pinnedRow(pins []string) [][]ports.InlineButton {
	if len(pins) == 0 {
		return nil
	}
	var row []ports.InlineButton
	for _, pin := range pins {
		label := "⭐ " + strings.ToUpper(pin)
		// Truncate long labels to keep buttons readable on mobile
		runes := []rune(label)
		if len(runes) > 18 {
			label = string(runes[:18])
		}
		cb := "cmd:" + strings.ReplaceAll(pin, " ", ":")
		row = append(row, ports.InlineButton{
			Text:         label,
			CallbackData: cb,
		})
	}
	return [][]ports.InlineButton{row}
}

// HelpCategoryMenu builds the top-level help category selector.
// If pins is non-empty, a pinned-commands quick-access row is prepended (TASK-078).
func (kb *KeyboardBuilder) HelpCategoryMenu(pins ...string) ports.InlineKeyboard {
	var rows [][]ports.InlineButton
	rows = append(rows, kb.pinnedRow(pins)...)
	rows = append(rows,
		[]ports.InlineButton{
			{Text: "📊 Market & COT", CallbackData: "help:market"},
			{Text: "🔬 Research & Alpha", CallbackData: "help:research"},
		},
		[]ports.InlineButton{
			{Text: "🧠 AI & Outlook", CallbackData: "help:ai"},
			{Text: "⚡ Signals & Alerts", CallbackData: "help:signals"},
		},
		[]ports.InlineButton{
			{Text: "⚙️ Settings", CallbackData: "help:settings"},
			{Text: "⚡ Shortcuts", CallbackData: "help:shortcuts"},
		},
		[]ports.InlineButton{
			{Text: "🆕 What's New", CallbackData: "help:changelog"},
		},
	)
	return ports.InlineKeyboard{Rows: rows}
}

// HelpCategoryMenuWithAdmin builds the top-level help category selector with admin option.
// If pins is non-empty, a pinned-commands quick-access row is prepended (TASK-078).
func (kb *KeyboardBuilder) HelpCategoryMenuWithAdmin(pins ...string) ports.InlineKeyboard {
	var rows [][]ports.InlineButton
	rows = append(rows, kb.pinnedRow(pins)...)
	rows = append(rows,
		[]ports.InlineButton{
			{Text: "📊 Market & COT", CallbackData: "help:market"},
			{Text: "🔬 Research & Alpha", CallbackData: "help:research"},
		},
		[]ports.InlineButton{
			{Text: "🧠 AI & Outlook", CallbackData: "help:ai"},
			{Text: "⚡ Signals & Alerts", CallbackData: "help:signals"},
		},
		[]ports.InlineButton{
			{Text: "⚙️ Settings", CallbackData: "help:settings"},
			{Text: "⚡ Shortcuts", CallbackData: "help:shortcuts"},
		},
		[]ports.InlineButton{
			{Text: "🔐 Admin", CallbackData: "help:admin"},
			{Text: "🆕 What's New", CallbackData: "help:changelog"},
		},
	)
	return ports.InlineKeyboard{Rows: rows}
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

// ---------------------------------------------------------------------------
// Related "Next Steps" Command Suggestions
// ---------------------------------------------------------------------------

// relatedCommands maps a command name to 2–3 related commands.
// Each entry is {label, callbackBase} where callbackBase uses "cmd:" prefix.
var relatedCommands = map[string][]struct {
	Label    string
	Callback string
}{
	"cot":         {{Label: "📈 Bias", Callback: "bias"}, {Label: "📊 Rank", Callback: "rank"}, {Label: "📡 Radar", Callback: "radar"}},
	"bias":        {{Label: "📉 COT", Callback: "cot"}, {Label: "🎯 CTA", Callback: "cta"}, {Label: "🌐 Macro", Callback: "macro"}},
	"cta":         {{Label: "📊 Quant", Callback: "quant"}, {Label: "🔑 Levels", Callback: "levels"}, {Label: "📈 Bias", Callback: "bias"}},
	"macro":       {{Label: "📅 Calendar", Callback: "calendar"}, {Label: "🔄 Transition", Callback: "transition"}, {Label: "📊 Sentiment", Callback: "sentiment"}},
	"quant":       {{Label: "📈 CTA", Callback: "cta"}, {Label: "📊 Backtest", Callback: "backtest"}, {Label: "📈 Price", Callback: "price"}},
	"quantbt":     {{Label: "📈 CTA", Callback: "cta"}, {Label: "📊 Quant", Callback: "quant"}, {Label: "📊 Backtest", Callback: "backtest"}},
	"calendar":    {{Label: "💥 Impact", Callback: "impact"}, {Label: "🌐 Macro", Callback: "macro"}, {Label: "📈 Price", Callback: "price"}},
	"gex":         {{Label: "📡 Radar", Callback: "radar"}, {Label: "📊 Sentiment", Callback: "sentiment"}, {Label: "📈 CryptoAlpha", Callback: "cryptoalpha"}},
	"skew":        {{Label: "📊 GEX", Callback: "gex"}, {Label: "📈 IV Surface", Callback: "ivol"}, {Label: "📡 Radar", Callback: "radar"}},
	"sentiment":   {{Label: "🌐 Macro", Callback: "macro"}, {Label: "📈 Bias", Callback: "bias"}, {Label: "📊 Rank", Callback: "rank"}},
	"price":       {{Label: "🔑 Levels", Callback: "levels"}, {Label: "📊 Quant", Callback: "quant"}, {Label: "🎯 CTA", Callback: "cta"}},
	"levels":      {{Label: "📈 Price", Callback: "price"}, {Label: "🎯 CTA", Callback: "cta"}, {Label: "📊 Quant", Callback: "quant"}},
	"radar":       {{Label: "📊 Rank", Callback: "rank"}, {Label: "📈 Bias", Callback: "bias"}, {Label: "🎯 CTA", Callback: "cta"}},
	"rank":        {{Label: "📈 Bias", Callback: "bias"}, {Label: "📉 COT", Callback: "cot"}, {Label: "📡 Radar", Callback: "radar"}},
	"outlook":     {{Label: "🌐 Macro", Callback: "macro"}, {Label: "📅 Calendar", Callback: "calendar"}, {Label: "📈 Bias", Callback: "bias"}},
	"impact":      {{Label: "📅 Calendar", Callback: "calendar"}, {Label: "🌐 Macro", Callback: "macro"}},
	"seasonal":    {{Label: "📉 COT", Callback: "cot"}, {Label: "📊 Backtest", Callback: "backtest"}, {Label: "📈 Bias", Callback: "bias"}},
	"backtest":    {{Label: "📊 Quant", Callback: "quant"}, {Label: "🎯 CTA", Callback: "cta"}, {Label: "📈 Seasonal", Callback: "seasonal"}},
	"intermarket": {{Label: "🌐 Macro", Callback: "macro"}, {Label: "📈 Price", Callback: "price"}, {Label: "📊 Sentiment", Callback: "sentiment"}},
	"briefing":    {{Label: "📅 Calendar", Callback: "calendar"}, {Label: "🎯 COT Bias", Callback: "bias"}, {Label: "🌐 Macro", Callback: "macro"}},
	"elliott":     {{Label: "📈 CTA", Callback: "cta"}, {Label: "📊 Quant", Callback: "quant"}, {Label: "🔑 Levels", Callback: "levels"}},
	"regime":      {{Label: "📊 Quant", Callback: "quant"}, {Label: "🌐 Macro", Callback: "macro"}, {Label: "📈 Price", Callback: "price"}},
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
	return ports.InlineKeyboard{Rows: [][]ports.InlineButton{row}}
}
