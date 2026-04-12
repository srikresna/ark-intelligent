package telegram

// handler_setalert.go — /setalert command: per-pair granular COT alert configuration
//
// Usage:
//   /setalert EUR          → alert if EUR bias changes (default: any change)
//   /setalert EUR 2.0      → alert if EUR conviction delta > 2.0
//   /setalert EUR flip     → alert only on bias flip bullish↔bearish
//   /setalert list         → show all active pair alerts
//   /setalert clear EUR    → remove EUR alert

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// cmdSetAlert handles the /setalert command for per-pair COT alert management.
func (h *Handler) cmdSetAlert(ctx context.Context, chatID string, userID int64, args string) error {
	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return err
	}

	args = strings.TrimSpace(args)

	// No args or "list" → show current alerts
	if args == "" || strings.EqualFold(args, "list") {
		return h.sendPairAlertList(ctx, chatID, prefs)
	}

	parts := strings.Fields(args)
	first := strings.ToUpper(parts[0])

	// "clear <currency>" → remove alert
	if strings.EqualFold(first, "CLEAR") || strings.EqualFold(first, "REMOVE") {
		if len(parts) < 2 {
			_, err := h.bot.SendHTML(ctx, chatID, "⚠️ Usage: <code>/setalert clear EUR</code>")
			return err
		}
		currency := strings.ToUpper(parts[1])
		return h.removePairAlert(ctx, chatID, userID, prefs, currency)
	}

	// Validate currency
	if !domain.IsValidAlertCurrency(first) {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ Currency <b>%s</b> tidak valid.\n\nCurrency yang didukung: %s",
			first, strings.Join(domain.ValidAlertCurrencies(), ", ")))
		return err
	}

	currency := first

	// Parse mode: default (any change), delta threshold, or flip-only
	var convictionDelta float64
	var biasFlip bool

	if len(parts) >= 2 {
		mode := strings.ToLower(parts[1])
		switch mode {
		case "flip":
			biasFlip = true
		default:
			delta, parseErr := strconv.ParseFloat(mode, 64)
			if parseErr != nil {
				_, err := h.bot.SendHTML(ctx, chatID,
					"⚠️ Format salah.\n\nUsage:\n"+
						"<code>/setalert EUR</code> — alert semua perubahan\n"+
						"<code>/setalert EUR 2.0</code> — alert jika delta &gt; 2.0\n"+
						"<code>/setalert EUR flip</code> — alert hanya saat bias flip")
				return err
			}
			convictionDelta = delta
		}
	}

	// Add or update pair alert
	return h.upsertPairAlert(ctx, chatID, userID, prefs, domain.PairAlert{
		Currency:        currency,
		ConvictionDelta: convictionDelta,
		BiasFlip:        biasFlip,
		Enabled:         true,
	})
}

// sendPairAlertList displays the user's current per-pair alerts.
func (h *Handler) sendPairAlertList(ctx context.Context, chatID string, prefs domain.UserPrefs) error {
	if len(prefs.PairAlerts) == 0 {
		kb := ports.InlineKeyboard{Rows: [][]ports.InlineButton{
			{
				{Text: "➕ Tambah Alert", CallbackData: "setalert:add"},
			},
		}}
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			"📋 <b>Per-Pair Alerts</b>\n\nBelum ada alert aktif.\n\n"+
				"Gunakan <code>/setalert EUR</code> untuk menambah alert, "+
				"atau tap tombol di bawah.",
			kb)
		return err
	}

	var b strings.Builder
	b.WriteString("📋 <b>Per-Pair Alerts</b>\n\n")

	for _, pa := range prefs.PairAlerts {
		status := "✅"
		if !pa.Enabled {
			status = "⏸"
		}

		desc := "Semua perubahan"
		if pa.BiasFlip {
			desc = "Hanya bias flip"
		} else if pa.ConvictionDelta > 0 {
			desc = fmt.Sprintf("Delta &gt; %.1f", pa.ConvictionDelta)
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n", status, pa.Currency, desc))
	}

	b.WriteString(fmt.Sprintf("\n<i>Total: %d alert aktif</i>\n", len(prefs.PairAlerts)))
	b.WriteString("\nGunakan <code>/setalert clear EUR</code> untuk hapus.")

	kb := ports.InlineKeyboard{Rows: [][]ports.InlineButton{
		{
			{Text: "➕ Tambah Alert", CallbackData: "setalert:add"},
			{Text: "🗑 Hapus Semua", CallbackData: "setalert:clearall"},
		},
	}}

	_, err := h.bot.SendWithKeyboard(ctx, chatID, b.String(), kb)
	return err
}

// upsertPairAlert adds or updates a pair alert in user prefs.
func (h *Handler) upsertPairAlert(ctx context.Context, chatID string, userID int64, prefs domain.UserPrefs, alert domain.PairAlert) error {
	// Max 8 pair alerts
	const maxPairAlerts = 8

	// Check if already exists → update
	found := false
	for i, pa := range prefs.PairAlerts {
		if pa.Currency == alert.Currency {
			prefs.PairAlerts[i] = alert
			found = true
			break
		}
	}

	if !found {
		if len(prefs.PairAlerts) >= maxPairAlerts {
			_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
				"⚠️ Maksimum %d pair alerts. Hapus salah satu dulu:\n<code>/setalert clear EUR</code>",
				maxPairAlerts))
			return err
		}
		prefs.PairAlerts = append(prefs.PairAlerts, alert)
	}

	if err := h.prefsRepo.Set(ctx, userID, prefs); err != nil {
		return err
	}

	desc := "semua perubahan"
	if alert.BiasFlip {
		desc = "hanya bias flip"
	} else if alert.ConvictionDelta > 0 {
		desc = fmt.Sprintf("delta &gt; %.1f", alert.ConvictionDelta)
	}

	action := "ditambahkan"
	if found {
		action = "diperbarui"
	}

	_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
		"✅ Alert %s <b>%s</b> %s — %s\n\nGunakan <code>/setalert list</code> untuk lihat semua.",
		alert.Currency, alert.Currency, action, desc))
	return err
}

// removePairAlert removes a pair alert from user prefs.
func (h *Handler) removePairAlert(ctx context.Context, chatID string, userID int64, prefs domain.UserPrefs, currency string) error {
	found := false
	newAlerts := make([]domain.PairAlert, 0, len(prefs.PairAlerts))
	for _, pa := range prefs.PairAlerts {
		if pa.Currency == currency {
			found = true
			continue
		}
		newAlerts = append(newAlerts, pa)
	}

	if !found {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ Tidak ada alert aktif untuk <b>%s</b>.\n\n"+
				"Gunakan <code>/setalert list</code> untuk lihat semua.",
			currency))
		return err
	}

	prefs.PairAlerts = newAlerts
	if err := h.prefsRepo.Set(ctx, userID, prefs); err != nil {
		return err
	}

	_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
		"🗑 Alert <b>%s</b> dihapus.\n\nGunakan <code>/setalert list</code> untuk lihat semua.", currency))
	return err
}

// cbSetAlert handles inline keyboard callbacks for pair alert management.
func (h *Handler) cbSetAlert(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// data format: "setalert:add", "setalert:clearall", "setalert:toggle:EUR"
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 2 {
		return nil
	}

	action := parts[1]

	switch action {
	case "add":
		// Show currency picker keyboard
		return h.sendAlertCurrencyPicker(ctx, chatID, userID)

	case "clearall":
		prefs, err := h.prefsRepo.Get(ctx, userID)
		if err != nil {
			return err
		}
		prefs.PairAlerts = nil
		if err := h.prefsRepo.Set(ctx, userID, prefs); err != nil {
			return err
		}
		_, err = h.bot.SendHTML(ctx, chatID, "🗑 Semua pair alerts dihapus.")
		return err

	case "set":
		// setalert:set:EUR — set alert for currency with defaults
		if len(parts) < 3 {
			return nil
		}
		currency := strings.ToUpper(parts[2])
		if !domain.IsValidAlertCurrency(currency) {
			return nil
		}
		prefs, err := h.prefsRepo.Get(ctx, userID)
		if err != nil {
			return err
		}
		return h.upsertPairAlert(ctx, chatID, userID, prefs, domain.PairAlert{
			Currency: currency,
			Enabled:  true,
		})
	}

	return nil
}

// sendAlertCurrencyPicker shows a keyboard with all valid currencies for pair alerts.
func (h *Handler) sendAlertCurrencyPicker(ctx context.Context, chatID string, userID int64) error {
	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return err
	}

	// Build set of already-alerted currencies
	alerted := make(map[string]bool)
	for _, pa := range prefs.PairAlerts {
		alerted[pa.Currency] = true
	}

	currencies := domain.ValidAlertCurrencies()
	var rows [][]ports.InlineButton
	var row []ports.InlineButton
	for _, c := range currencies {
		label := c
		if alerted[c] {
			label = "✅ " + c
		}
		row = append(row, ports.InlineButton{
			Text:         label,
			CallbackData: "setalert:set:" + c,
		})
		if len(row) == 4 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	kb := ports.InlineKeyboard{Rows: rows}
	_, err = h.bot.SendWithKeyboard(ctx, chatID,
		"🔔 <b>Pilih Currency untuk Alert</b>\n\n"+
			"Tap currency untuk menambah alert (default: semua perubahan).\n"+
			"Untuk opsi lanjutan: <code>/setalert EUR 2.0</code> atau <code>/setalert EUR flip</code>",
		kb)
	return err
}
