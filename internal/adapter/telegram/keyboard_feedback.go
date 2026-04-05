package telegram

import (
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

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
// Briefing Keyboard
// ---------------------------------------------------------------------------

// BriefingMenu returns the inline keyboard for the /briefing command.
// Provides quick refresh and deep-dive shortcuts.
func (kb *KeyboardBuilder) BriefingMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🔄 Refresh", CallbackData: "briefing:refresh"},
				{Text: "📅 Calendar", CallbackData: "cmd:calendar"},
			},
			{
				{Text: "📊 COT Detail", CallbackData: "cmd:cot"},
				{Text: "🏠 Home", CallbackData: "cmd:help"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Feedback Buttons — 👍/👎 Reactions on Analysis Messages (TASK-051)
// ---------------------------------------------------------------------------

// FeedbackRow returns a row with thumbs-up/down and alert buttons for user feedback.
// callbackBase format: "fb:<type>:<key>" e.g. "fb:cot:EUR", "fb:outlook:latest"
func (kb *KeyboardBuilder) FeedbackRow(callbackBase string) []ports.InlineButton {
	return []ports.InlineButton{
		{Text: "👍 Helpful", CallbackData: callbackBase + ":up"},
		{Text: "👎 Not Helpful", CallbackData: callbackBase + ":down"},
		{Text: "🔔 Alert on change", CallbackData: callbackBase + ":alert"},
	}
}

// AppendFeedbackRow appends a feedback row to an existing InlineKeyboard.
func AppendFeedbackRow(kb ports.InlineKeyboard, kbb *KeyboardBuilder, callbackBase string, feedbackEnabled bool) ports.InlineKeyboard {
	if !feedbackEnabled || kbb == nil {
		return kb
	}
	kb.Rows = append(kb.Rows, kbb.FeedbackRow(callbackBase))
	return kb
}
