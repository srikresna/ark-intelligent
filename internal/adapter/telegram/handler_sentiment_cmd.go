package telegram

// /sentiment — Sentiment Survey Dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
)

// ---------------------------------------------------------------------------
// /sentiment — Sentiment Survey Dashboard
// ---------------------------------------------------------------------------

func (h *Handler) cmdSentiment(ctx context.Context, chatID string, userID int64, args string) error {
	forceRefresh := strings.EqualFold(strings.TrimSpace(args), "refresh")
	if forceRefresh {
		if !h.requireAdmin(ctx, chatID, userID) {
			return nil
		}
		sentiment.InvalidateCache()
	}

	cacheStatus := "🧠 Fetching sentiment data... ⏳"
	if !forceRefresh && sentiment.CacheAge() >= 0 {
		cacheStatus = "🧠 Loading sentiment data (from cache)... ⏳"
	}
	msgID, err := h.bot.SendLoading(ctx, chatID, cacheStatus)
	if err != nil {
		log.Warn().Err(err).Msg("failed to send loading indicator")
	}

	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	data, err := sentiment.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("sentiment data fetch failed")

		// Try stale cache as fallback
		if staleData, ok := sentiment.GetStaleCache(); ok {
			age := time.Since(staleData.FetchedAt)
			hours := int(age.Hours())
			var ageStr string
			if hours > 0 {
				ageStr = fmt.Sprintf("%d jam yang lalu", hours)
			} else {
				ageStr = fmt.Sprintf("%d menit yang lalu", int(age.Minutes()))
			}

			text := fmt.Sprintf("⚠️ <b>Data terbaru tidak tersedia</b>\n\n"+
				"Sistem sedang mengalami gangguan, menampilkan data terakhir (%s):\n\n"+
				"<i>Data akan diperbarui otomatis begitu layanan kembali normal.</i>\n\n"+
				"%s", ageStr, h.fmt.FormatSentiment(staleData, h.currentMacroRegimeName(ctx)))

			if msgID > 0 {
				return h.bot.EditMessage(ctx, chatID, msgID, text)
			}
			_, sErr := h.bot.SendMessage(ctx, chatID, text)
			return sErr
		}

		text := fmt.Sprintf("❌ <b>Gagal mengambil data sentiment</b>\n\n"+
			"Silakan coba lagi dalam beberapa menit.\n\n"+
			"<i>Technical details:</i> <code>%v</code>", err)
		if msgID > 0 {
			return h.bot.EditMessage(ctx, chatID, msgID, text)
		}
		_, sErr := h.bot.SendMessage(ctx, chatID, text)
		return sErr
	}

	if !data.CNNAvailable && !data.AAIIAvailable && !data.VIXAvailable {
		text := "⚠️ <b>Sentiment data unavailable</b>\n\nAll data sources are currently unavailable. Try again later."
		if msgID > 0 {
			return h.bot.EditMessage(ctx, chatID, msgID, text)
		}
		_, sErr := h.bot.SendMessage(ctx, chatID, text)
		return sErr
	}

	htmlMsg := h.fmt.FormatSentiment(data, h.currentMacroRegimeName(ctx))

	// Add refresh button to keyboard
	kb := h.kb.RelatedCommandsKeyboard("sentiment", "")
	refreshRow := []ports.InlineButton{
		{Text: "🔄 Refresh", CallbackData: "cmd:sentiment:refresh"},
	}
	if len(kb.Rows) > 0 {
		kb.Rows = append(kb.Rows, refreshRow)
	} else {
		kb.Rows = [][]ports.InlineButton{refreshRow}
	}

	if msgID > 0 {
		// Delete placeholder before sending chunked response
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	}
	_, sErr := h.bot.SendWithKeyboardChunked(ctx, chatID, htmlMsg, kb)
	return sErr
}

// cbSentimentRefresh handles the refresh button callback for sentiment data.
func (h *Handler) cbSentimentRefresh(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// Invalidate cache and re-fetch
	sentiment.InvalidateCache()

	// Delete old message
	if msgID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	}

	// Show loading
	loadingMsgID, err := h.bot.SendLoading(ctx, chatID, "🧠 Refreshing sentiment data... ⏳")
	if err != nil {
		log.Warn().Err(err).Msg("failed to send loading indicator")
	}

	// Fetch with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	freshData, err := sentiment.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("sentiment refresh failed")

		// Try stale cache
		if staleData, ok := sentiment.GetStaleCache(); ok {
			text := fmt.Sprintf("⚠️ <b>Refresh gagal, menampilkan data terakhir</b>\n\n"+
				"<i>Data terakhir: %s</i>\n\n"+
				"%s", staleData.FetchedAt.Format("02 Jan 2006 15:04"),
				h.fmt.FormatSentiment(staleData, h.currentMacroRegimeName(ctx)))

			kb := h.kb.RelatedCommandsKeyboard("sentiment", "")
			refreshRow := []ports.InlineButton{
				{Text: "🔄 Refresh", CallbackData: "sentiment:refresh"},
			}
			if len(kb.Rows) > 0 {
				kb.Rows = append(kb.Rows, refreshRow)
			} else {
				kb.Rows = [][]ports.InlineButton{refreshRow}
			}

			if loadingMsgID > 0 {
				return h.bot.EditWithKeyboard(ctx, chatID, loadingMsgID, text, kb)
			}
			_, sErr := h.bot.SendWithKeyboard(ctx, chatID, text, kb)
			return sErr
		}

		text := "❌ <b>Gagal refresh data sentiment</b>\n\nSilakan coba lagi dalam beberapa menit."
		if loadingMsgID > 0 {
			return h.bot.EditMessage(ctx, chatID, loadingMsgID, text)
		}
		_, sErr := h.bot.SendMessage(ctx, chatID, text)
		return sErr
	}

	htmlMsg := h.fmt.FormatSentiment(freshData, h.currentMacroRegimeName(ctx))
	kb := h.kb.RelatedCommandsKeyboard("sentiment", "")
	refreshRow := []ports.InlineButton{
		{Text: "🔄 Refresh", CallbackData: "sentiment:refresh"},
	}
	if len(kb.Rows) > 0 {
		kb.Rows = append(kb.Rows, refreshRow)
	} else {
		kb.Rows = [][]ports.InlineButton{refreshRow}
	}

	if loadingMsgID > 0 {
		// Delete placeholder before sending chunked response
		_ = h.bot.DeleteMessage(ctx, chatID, loadingMsgID)
	}
	_, sErr := h.bot.SendWithKeyboardChunked(ctx, chatID, htmlMsg, kb)
	return sErr
}
