package telegram

// /settings — User Preferences

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// /settings — User preferences
// ---------------------------------------------------------------------------

func (h *Handler) cmdSettings(ctx context.Context, chatID string, userID int64, args string) error {
	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get preferences: %w", err)
	}

	html := h.fmt.FormatSettings(prefs)
	kb := h.kb.SettingsMenu(prefs)
	_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

// cbSettings handles settings toggle callbacks.
func (h *Handler) cbSettings(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "set:")

	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return err
	}

	switch action {
	case "lang_toggle":
		if prefs.Language == "en" {
			prefs.Language = "id"
		} else {
			prefs.Language = "en"
		}
	case "changelog_view":
		if h.changelog == "" {
			return h.bot.EditMessage(ctx, chatID, msgID, "Changelog unavailable.")
		}
		html := fmt.Sprintf("🦅 <b>ARK Intelligence Changelog</b>\n\n%s", h.changelog)
		kb := h.kb.SettingsMenu(prefs)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)

	case "alerts_toggle":
		prefs.AlertsEnabled = !prefs.AlertsEnabled
	case "cot_toggle":
		prefs.COTAlertsEnabled = !prefs.COTAlertsEnabled
	case "ai_toggle":
		prefs.AIReportsEnabled = !prefs.AIReportsEnabled
	case "alert_mgr":
		// Open the alert management sub-menu
		html := h.fmt.FormatAlertManagement(prefs)
		kb := h.kb.AlertManagementMenu(prefs)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	case "mobile_toggle":
		prefs.MobileMode = !prefs.MobileMode
	case "model_claude":
		prefs.PreferredModel = "claude"
	case "model_gemini":
		prefs.PreferredModel = "gemini"
	case "impact_high_only":
		prefs.AlertImpacts = []string{"High"}
	case "impact_high_med":
		prefs.AlertImpacts = []string{"High", "Medium"}
	case "impact_all":
		prefs.AlertImpacts = []string{"High", "Medium", "Low"}
	case "time_60_15_5":
		prefs.AlertMinutes = []int{60, 15, 5}
	case "time_15_5_1":
		prefs.AlertMinutes = []int{15, 5, 1}
	case "time_5_1":
		prefs.AlertMinutes = []int{5, 1}
	case "cur_reset":
		prefs.CurrencyFilter = nil
	default:
		// Handle cur_toggle:XXX dynamically
		if strings.HasPrefix(action, "cur_toggle:") {
			cur := strings.ToUpper(strings.TrimPrefix(action, "cur_toggle:"))
			if cur != "" {
				found := false
				newFilter := make([]string, 0, len(prefs.CurrencyFilter))
				for _, c := range prefs.CurrencyFilter {
					if strings.ToUpper(c) == cur {
						found = true
						// Skip it (remove)
					} else {
						newFilter = append(newFilter, c)
					}
				}
				if !found {
					newFilter = append(newFilter, cur)
				}
				prefs.CurrencyFilter = newFilter
			}
		} else if strings.HasPrefix(action, "claude_model:") {
			// Handle set:claude_model:claude-opus-4-5 etc (specific Claude variant)
			modelID := domain.ClaudeModelID(strings.TrimPrefix(action, "claude_model:"))
			if domain.IsValidClaudeModel(modelID) {
				prefs.ClaudeModel = modelID
				// Automatically switch provider to Claude when a Claude model is selected
				prefs.PreferredModel = "claude"
				log.Info().Str("model", string(modelID)).Int64("user_id", userID).Msg("user selected Claude model variant")
			} else {
				log.Warn().Str("model", string(modelID)).Msg("unknown Claude model ID in settings callback")
				return nil
			}
		} else {
			log.Warn().Str("action", action).Msg("unknown settings action")
			return nil
		}
	}

	if err := h.prefsRepo.Set(ctx, userID, prefs); err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}

	// Update the message with new settings state
	html := h.fmt.FormatSettings(prefs)
	kb := h.kb.SettingsMenu(prefs)
	if err := h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb); err != nil {
		return err
	}

	// Show toast feedback so user gets visual/audio confirmation of the change.
	if toast := settingsActionToast(action, prefs); toast != "" {
		return callbackToast(toast)
	}
	return nil
}

// settingsActionToast returns a Telegram AnswerCallback toast for a settings action.
// Returns empty string for navigation actions that don't change persistent state.
func settingsActionToast(action string, prefs domain.UserPrefs) string {
	switch action {
	case "lang_toggle":
		if prefs.Language == "id" {
			return "✅ Bahasa: Indonesia"
		}
		return "✅ Language: English"
	case "alerts_toggle":
		if prefs.AlertsEnabled {
			return "✅ Alert diaktifkan"
		}
		return "🔕 Alert dinonaktifkan"
	case "cot_toggle":
		if prefs.COTAlertsEnabled {
			return "✅ Alert COT aktif"
		}
		return "🔕 Alert COT nonaktif"
	case "ai_toggle":
		if prefs.AIReportsEnabled {
			return "✅ AI Reports aktif"
		}
		return "🔕 AI Reports nonaktif"
	case "mobile_toggle":
		if prefs.MobileMode {
			return "✅ Mode kompak aktif"
		}
		return "✅ Mode normal aktif"
	case "model_claude":
		return "✅ Model: Claude"
	case "model_gemini":
		return "✅ Model: Gemini"
	case "impact_high_only":
		return "✅ Filter impact: High only"
	case "impact_high_med":
		return "✅ Filter impact: High + Medium"
	case "impact_all":
		return "✅ Filter impact: Semua"
	case "time_60_15_5":
		return "✅ Notif: 60/15/5 menit sebelum"
	case "time_15_5_1":
		return "✅ Notif: 15/5/1 menit sebelum"
	case "time_5_1":
		return "✅ Notif: 5/1 menit sebelum"
	case "cur_reset":
		return "✅ Filter mata uang direset"
	}
	if strings.HasPrefix(action, "claude_model:") {
		modelID := strings.TrimPrefix(action, "claude_model:")
		return fmt.Sprintf("✅ Model: %s", modelID)
	}
	if strings.HasPrefix(action, "cur_toggle:") {
		cur := strings.ToUpper(strings.TrimPrefix(action, "cur_toggle:"))
		for _, c := range prefs.CurrencyFilter {
			if strings.ToUpper(c) == cur {
				return fmt.Sprintf("✅ %s ditambahkan ke filter", cur)
			}
		}
		return fmt.Sprintf("✅ %s dihapus dari filter", cur)
	}
	return ""
}

// cbAlertToggle handles quick alert toggle from notification messages.
// Supports granular per-type disabling via "alert:off:<type>" callbacks.
func (h *Handler) cbAlertToggle(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "alert:")

	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return err
	}

	switch action {
	case "mute_1h", "disable":
		// Disable ALL alerts until manually re-enabled via /settings.
		// Note: "mute_1h" is a legacy callback key retained for backward compatibility.
		prefs.AlertsEnabled = false
		prefs.COTAlertsEnabled = false
		_ = h.prefsRepo.Set(ctx, userID, prefs)
		return h.bot.EditMessage(ctx, chatID, msgID,
			"\xf0\x9f\x94\x95 Semua alert dimatikan. Gunakan /settings untuk mengaktifkan kembali.")

	case "off:cot":
		// Disable COT release alerts only.
		prefs.COTAlertsEnabled = false
		_ = h.prefsRepo.Set(ctx, userID, prefs)
		return h.bot.EditMessage(ctx, chatID, msgID,
			"\xf0\x9f\x94\x95 Alert COT dimatikan.\nGunakan /settings untuk mengaktifkan kembali.")

	case "off:fred":
		// Disable FRED macro alerts (reuses COTAlertsEnabled flag).
		prefs.COTAlertsEnabled = false
		_ = h.prefsRepo.Set(ctx, userID, prefs)
		return h.bot.EditMessage(ctx, chatID, msgID,
			"\xf0\x9f\x94\x95 Alert Macro dimatikan.\nGunakan /settings untuk mengaktifkan kembali.")

	case "off:signal":
		// Disable strong signal alerts (reuses COTAlertsEnabled flag).
		prefs.COTAlertsEnabled = false
		_ = h.prefsRepo.Set(ctx, userID, prefs)
		return h.bot.EditMessage(ctx, chatID, msgID,
			"\xf0\x9f\x94\x95 Alert Signal dimatikan.\nGunakan /settings untuk mengaktifkan kembali.")

	case "dismiss":
		return h.bot.DeleteMessage(ctx, chatID, msgID)
	}

	return nil
}

// cbAlertMgr handles alert management sub-menu callbacks (TASK-202).
func (h *Handler) cbAlertMgr(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "alertmgr:")

	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return err
	}

	switch {
	case action == "qh_toggle":
		prefs.QuietHoursEnabled = !prefs.QuietHoursEnabled
		if prefs.QuietHoursEnabled && prefs.QuietHoursStart == 0 && prefs.QuietHoursEnd == 0 {
			// Default quiet window: 23:00-07:00 WIB
			prefs.QuietHoursStart = 23
			prefs.QuietHoursEnd = 7
		}

	case strings.HasPrefix(action, "qh_set:"):
		// Format: qh_set:START:END
		parts := strings.Split(strings.TrimPrefix(action, "qh_set:"), ":")
		if len(parts) == 2 {
			start, e1 := strconv.Atoi(parts[0])
			end, e2 := strconv.Atoi(parts[1])
			if e1 == nil && e2 == nil && start >= 0 && start <= 23 && end >= 0 && end <= 23 {
				prefs.QuietHoursStart = start
				prefs.QuietHoursEnd = end
				prefs.QuietHoursEnabled = true
			}
		}

	case strings.HasPrefix(action, "type_toggle:"):
		key := strings.TrimPrefix(action, "type_toggle:")
		if prefs.AlertTypes == nil {
			prefs.AlertTypes = make(map[string]bool)
		}
		current := prefs.IsAlertTypeEnabled(key)
		prefs.AlertTypes[key] = !current

	case strings.HasPrefix(action, "cap:"):
		n, err := strconv.Atoi(strings.TrimPrefix(action, "cap:"))
		if err == nil && n >= 0 {
			prefs.MaxAlertsPerDay = n
		}

	case action == "back":
		// Return to main settings
		if err := h.prefsRepo.Set(ctx, userID, prefs); err != nil {
			return err
		}
		html := h.fmt.FormatSettings(prefs)
		kb := h.kb.SettingsMenu(prefs)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)

	default:
		log.Warn().Str("action", action).Msg("unknown alertmgr action")
		return nil
	}

	if err := h.prefsRepo.Set(ctx, userID, prefs); err != nil {
		return err
	}

	// Re-render alert management menu
	html := h.fmt.FormatAlertManagement(prefs)
	kb := h.kb.AlertManagementMenu(prefs)
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
}
