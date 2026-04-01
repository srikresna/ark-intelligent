package telegram

// /pin, /unpin, /pins — Pinned Commands for Personalized Quick Access

import (
	"context"
	"fmt"
	"strings"
)

// maxPins is the maximum number of pinned commands per user.
const maxPins = 4

// validPinCommands lists commands that can be pinned.
// The key is the base command name (without "/"), values are display names.
var validPinCommands = map[string]bool{
	"cot":         true,
	"outlook":     true,
	"macro":       true,
	"calendar":    true,
	"price":       true,
	"rank":        true,
	"bias":        true,
	"accuracy":    true,
	"sentiment":   true,
	"seasonal":    true,
	"backtest":    true,
	"levels":      true,
	"quant":       true,
	"vp":          true,
	"alpha":       true,
	"intermarket": true,
	"history":     true,
	"impact":      true,
	"report":      true,
	"ict":         true,
	"gex":         true,
}

// cmdPin handles /pin [command [args]].
// Without args: show current pins (same as /pins).
// With args: add a pinned command.
func (h *Handler) cmdPin(ctx context.Context, chatID string, userID int64, args string) error {
	args = strings.TrimSpace(args)

	// No args → show pins
	if args == "" {
		return h.showPins(ctx, chatID, userID)
	}

	// Parse command + optional currency arg
	parts := strings.Fields(args)
	baseCmd := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	pinStr := baseCmd
	if len(parts) > 1 {
		pinStr = baseCmd + " " + strings.ToUpper(parts[1])
	}

	// Validate base command
	if !validPinCommands[baseCmd] {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("❌ <code>%s</code> bukan command yang bisa di-pin.\n\nContoh: <code>/pin cot EUR</code>, <code>/pin outlook</code>", baseCmd))
		return err
	}

	// Load prefs
	prefs, _ := h.prefsRepo.Get(ctx, userID)

	// Check if already pinned
	for _, p := range prefs.PinnedCommands {
		if strings.EqualFold(p, pinStr) {
			_, err := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("ℹ️ <code>%s</code> sudah di-pin.", pinStr))
			return err
		}
	}

	// Check max pins
	if len(prefs.PinnedCommands) >= maxPins {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("⚠️ Maksimal %d pins. Hapus salah satu dulu:\n<code>/unpin &lt;command&gt;</code>\n\nPins saat ini: %s",
				maxPins, formatPinList(prefs.PinnedCommands)))
		return err
	}

	// Add pin
	prefs.PinnedCommands = append(prefs.PinnedCommands, pinStr)
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	_, err := h.bot.SendHTML(ctx, chatID,
		fmt.Sprintf("⭐ <b>%s</b> di-pin!\n\nPins kamu: %s\n\n<i>Pin akan muncul di menu utama.</i>",
			strings.ToUpper(pinStr), formatPinList(prefs.PinnedCommands)))
	return err
}

// cmdUnpin handles /unpin <command>.
func (h *Handler) cmdUnpin(ctx context.Context, chatID string, userID int64, args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		_, err := h.bot.SendHTML(ctx, chatID,
			"Usage: <code>/unpin cot EUR</code> atau <code>/unpin outlook</code>")
		return err
	}

	parts := strings.Fields(args)
	baseCmd := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	target := baseCmd
	if len(parts) > 1 {
		target = baseCmd + " " + strings.ToUpper(parts[1])
	}

	prefs, _ := h.prefsRepo.Get(ctx, userID)

	found := false
	var updated []string
	for _, p := range prefs.PinnedCommands {
		if strings.EqualFold(p, target) {
			found = true
			continue
		}
		updated = append(updated, p)
	}

	if !found {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("ℹ️ <code>%s</code> tidak ada di pins kamu.", target))
		return err
	}

	prefs.PinnedCommands = updated
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	msg := fmt.Sprintf("🗑 <b>%s</b> di-unpin.", strings.ToUpper(target))
	if len(updated) > 0 {
		msg += "\n\nPins tersisa: " + formatPinList(updated)
	} else {
		msg += "\n\n<i>Tidak ada pin tersisa.</i>"
	}
	_, err := h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// showPins displays the user's current pinned commands.
func (h *Handler) showPins(ctx context.Context, chatID string, userID int64) error {
	prefs, _ := h.prefsRepo.Get(ctx, userID)

	if len(prefs.PinnedCommands) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID,
			`📌 <b>Pinned Commands</b>

<i>Belum ada pin.</i>

Tambahkan dengan:
<code>/pin cot EUR</code> — pin COT EUR
<code>/pin outlook</code> — pin Outlook
<code>/pin macro</code> — pin Macro

Max 4 pins. Pin akan muncul di menu utama.`)
		return err
	}

	var lines []string
	for i, p := range prefs.PinnedCommands {
		lines = append(lines, fmt.Sprintf("  %d. ⭐ <b>%s</b>", i+1, strings.ToUpper(p)))
	}

	_, err := h.bot.SendHTML(ctx, chatID,
		fmt.Sprintf("📌 <b>Pinned Commands</b> (%d/%d)\n\n%s\n\n<code>/pin &lt;cmd&gt;</code> — tambah\n<code>/unpin &lt;cmd&gt;</code> — hapus",
			len(prefs.PinnedCommands), maxPins, strings.Join(lines, "\n")))
	return err
}

// formatPinList formats pins as a comma-separated inline list.
func formatPinList(pins []string) string {
	var parts []string
	for _, p := range pins {
		parts = append(parts, "⭐"+strings.ToUpper(p))
	}
	return strings.Join(parts, ", ")
}
