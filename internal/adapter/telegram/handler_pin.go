package telegram

// /pin, /unpin, /pins — Personalized pinned command shortcuts (TASK-078)

import (
	"context"
	"fmt"
	"strings"
)

// maxPins is the maximum number of pinned commands a user can have.
const maxPins = 4

// validPinCommands lists commands that may be pinned.
// Each entry is a base command name (without slash) that cbQuickCommand can route.
var validPinCommands = map[string]bool{
	"cot":          true,
	"outlook":      true,
	"macro":        true,
	"calendar":     true,
	"price":        true,
	"bias":         true,
	"rank":         true,
	"quant":        true,
	"cta":          true,
	"vp":           true,
	"alpha":        true,
	"gex":          true,
	"sentiment":    true,
	"seasonal":     true,
	"backtest":     true,
	"accuracy":     true,
	"levels":       true,
	"intermarket":  true,
	"impact":       true,
	"session":      true,
	"carry":        true,
	"regime":       true,
	"defi":         true,
	"onchain":      true,
	"wyckoff":      true,
	"smc":          true,
	"elliott":      true,
	"signal":       true,
}

// cmdPin handles /pin [command [args]] — add a pinned shortcut.
// Without arguments it behaves like /pins (list current pins).
func (h *Handler) cmdPin(ctx context.Context, chatID string, userID int64, args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		return h.cmdPins(ctx, chatID, userID, "")
	}

	prefs, _ := h.prefsRepo.Get(ctx, userID)

	// Validate: extract base command from args (e.g. "cot EUR" → "cot")
	parts := strings.Fields(args)
	baseCmd := strings.ToLower(parts[0])
	// Strip leading slash if user typed "/pin /cot EUR"
	baseCmd = strings.TrimPrefix(baseCmd, "/")

	if !validPinCommands[baseCmd] {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("❌ <code>%s</code> bukan command yang valid untuk di-pin.\n\nContoh: <code>/pin cot EUR</code>, <code>/pin outlook</code>, <code>/pin macro</code>", baseCmd))
		return err
	}

	// Normalize pin label: lowercase base + original-case args
	pin := baseCmd
	if len(parts) > 1 {
		pin = baseCmd + " " + strings.Join(parts[1:], " ")
	}

	// Check duplicates
	for _, existing := range prefs.PinnedCommands {
		if strings.EqualFold(existing, pin) {
			_, err := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("ℹ️ <code>%s</code> sudah ada di pins kamu.", pin))
			return err
		}
	}

	// Check max
	if len(prefs.PinnedCommands) >= maxPins {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("⚠️ Maksimal %d pins. Hapus dulu dengan <code>/unpin &lt;command&gt;</code> sebelum menambah yang baru.\n\nPins saat ini: %s",
				maxPins, formatPinList(prefs.PinnedCommands)))
		return err
	}

	prefs.PinnedCommands = append(prefs.PinnedCommands, pin)
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	_, err := h.bot.SendHTML(ctx, chatID,
		fmt.Sprintf("⭐ <b>Pinned!</b> <code>%s</code>\n\nPins (%d/%d): %s",
			pin, len(prefs.PinnedCommands), maxPins, formatPinList(prefs.PinnedCommands)))
	return err
}

// cmdUnpin handles /unpin <command> — remove a pinned shortcut.
func (h *Handler) cmdUnpin(ctx context.Context, chatID string, userID int64, args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		_, err := h.bot.SendHTML(ctx, chatID, "❌ Gunakan: <code>/unpin &lt;command&gt;</code>\n\nContoh: <code>/unpin outlook</code>")
		return err
	}

	target := strings.ToLower(strings.TrimPrefix(args, "/"))
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
			fmt.Sprintf("❌ <code>%s</code> tidak ada di pins kamu.\n\nPins saat ini: %s",
				target, formatPinList(prefs.PinnedCommands)))
		return err
	}

	prefs.PinnedCommands = updated
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	msg := fmt.Sprintf("🗑 <b>Unpinned:</b> <code>%s</code>", target)
	if len(updated) > 0 {
		msg += fmt.Sprintf("\n\nPins (%d/%d): %s", len(updated), maxPins, formatPinList(updated))
	} else {
		msg += "\n\nTidak ada pins aktif."
	}
	_, err := h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// cmdPins handles /pins — list current pinned commands.
func (h *Handler) cmdPins(ctx context.Context, chatID string, userID int64, _ string) error {
	prefs, _ := h.prefsRepo.Get(ctx, userID)

	if len(prefs.PinnedCommands) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID,
			"📌 <b>Pinned Commands</b>\n\nBelum ada pins. Tambahkan dengan:\n<code>/pin cot EUR</code>\n<code>/pin outlook</code>\n<code>/pin macro</code>")
		return err
	}

	var lines []string
	for i, p := range prefs.PinnedCommands {
		lines = append(lines, fmt.Sprintf("%d. ⭐ <code>%s</code>", i+1, p))
	}

	_, err := h.bot.SendHTML(ctx, chatID,
		fmt.Sprintf("📌 <b>Pinned Commands</b> (%d/%d)\n\n%s\n\n<i>Tambah:</i> <code>/pin &lt;command&gt;</code>\n<i>Hapus:</i> <code>/unpin &lt;command&gt;</code>",
			len(prefs.PinnedCommands), maxPins, strings.Join(lines, "\n")))
	return err
}

// formatPinList returns a comma-separated display of pins.
func formatPinList(pins []string) string {
	if len(pins) == 0 {
		return "<i>kosong</i>"
	}
	var parts []string
	for _, p := range pins {
		parts = append(parts, fmt.Sprintf("<code>%s</code>", p))
	}
	return strings.Join(parts, ", ")
}
