package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

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

	// Row 13: Token info toggle (show token usage stats in AI chat responses)
	tokenInfoLabel := "📊 Token Info: OFF → Turn ON"
	if prefs.ShowTokenInfo {
		tokenInfoLabel = "📊 Token Info: ON → Turn OFF"
	}
	rows = append(rows, []ports.InlineButton{{
		Text:         tokenInfoLabel,
		CallbackData: "set:token_info_toggle",
	}})

	// Row 14: Alert Management sub-menu (TASK-202)
	rows = append(rows, []ports.InlineButton{{
		Text:         "🔔 Manage Alert Types & Quiet Hours",
		CallbackData: "set:alert_mgr",
	}})

	// Row 15: Reset / Change experience level (TASK-254)
	rows = append(rows, []ports.InlineButton{{
		Text:         "🔄 Ubah Level Pengalaman",
		CallbackData: "set:reset_onboard",
	}})

	// Row 16: View Changelog
	rows = append(rows, []ports.InlineButton{{
		Text:         "📜 View Changelog",
		CallbackData: "set:changelog_view",
	}})

	// Row 17: Home button
	rows = append(rows, []ports.InlineButton{{
		Text:         btnHome,
		CallbackData: "nav:home",
	}})

	return ports.InlineKeyboard{Rows: rows}
}

// AlertManagementMenu builds the alert management sub-menu keyboard (TASK-202).
// Shows quiet hours, per-alert-type toggles, and daily cap controls.
func (kb *KeyboardBuilder) AlertManagementMenu(prefs domain.UserPrefs) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	// Row 1: Quiet Hours toggle
	qhLabel := "🌙 Quiet Hours: OFF → Turn ON"
	if prefs.QuietHoursEnabled {
		qhLabel = fmt.Sprintf("🌙 Quiet Hours: ON (%02d:00–%02d:00 WIB) → Turn OFF",
			prefs.QuietHoursStart, prefs.QuietHoursEnd)
	}
	rows = append(rows, []ports.InlineButton{{
		Text:         qhLabel,
		CallbackData: "alertmgr:qh_toggle",
	}})

	// Row 2: Quiet hours presets (only shown when enabled)
	if prefs.QuietHoursEnabled {
		qhPreset := func(start, end int, label string) ports.InlineButton {
			prefix := "  "
			if prefs.QuietHoursStart == start && prefs.QuietHoursEnd == end {
				prefix = "✅ "
			}
			return ports.InlineButton{
				Text:         prefix + label,
				CallbackData: fmt.Sprintf("alertmgr:qh_set:%d:%d", start, end),
			}
		}
		rows = append(rows, []ports.InlineButton{
			qhPreset(23, 7, "23-07"),
			qhPreset(22, 8, "22-08"),
			qhPreset(0, 9, "00-09"),
		})
	}

	// Rows 3+: Per-alert-type toggles
	for _, key := range domain.ValidAlertTypes() {
		enabled := prefs.IsAlertTypeEnabled(key)
		label := domain.AlertTypeLabel(key)
		if enabled {
			label += ": ON → OFF"
		} else {
			label = "❌ " + label + ": OFF → ON"
		}
		rows = append(rows, []ports.InlineButton{{
			Text:         label,
			CallbackData: "alertmgr:type_toggle:" + key,
		}})
	}

	// Row: Daily cap presets
	capLabel := func(n int, display string) ports.InlineButton {
		prefix := "  "
		if prefs.MaxAlertsPerDay == n {
			prefix = "✅ "
		}
		return ports.InlineButton{
			Text:         prefix + display,
			CallbackData: fmt.Sprintf("alertmgr:cap:%d", n),
		}
	}
	rows = append(rows, []ports.InlineButton{
		capLabel(0, "No Limit"),
		capLabel(10, "10/day"),
		capLabel(20, "20/day"),
		capLabel(50, "50/day"),
	})

	// Back to settings
	rows = append(rows, []ports.InlineButton{{
		Text:         "⬅️ Back to Settings",
		CallbackData: "alertmgr:back",
	}})

	return ports.InlineKeyboard{Rows: rows}
}
