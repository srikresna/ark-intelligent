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
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, cacheStatus)

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
			
			return h.bot.EditMessage(ctx, chatID, placeholderID,
				fmt.Sprintf("⚠️ <b>Data terbaru tidak tersedia</b>\n\n"+
					"Sistem sedang mengalami gangguan, menampilkan data terakhir (%s):\n\n"+
					"<i>Data akan diperbarui otomatis begitu layanan kembali normal.</i>", ageStr) +
				h.fmt.FormatSentiment(staleData, h.currentMacroRegimeName(ctx)))
		}
		
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"❌ <b>Gagal mengambil data sentiment</b>\n\n"+
				"Silakan coba lagi dalam beberapa menit.\n\n"+
				<i>Technical details:</i> <code>%v</code>", err)
	}

	if !data.CNNAvailable && !data.AAIIAvailable && !data.VIXAvailable {
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"⚠️ <b>Sentiment data unavailable</b>\n\n"+
				"All data sources are currently unavailable. Try again later.")
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
	
	if len(kb.Rows) > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	}
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}

// cbSentimentRefresh handles the refresh button callback for sentiment data.
func (h *Handler) cbSentimentRefresh(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// Invalidate cache and re-fetch
	sentiment.InvalidateCache()
	
	// Show loading
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, "🧠 Refreshing sentiment data... ⏳")
	
	// Fetch with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	freshData, err := sentiment.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("sentiment refresh failed")
		
		// Try stale cache
		if staleData, ok := sentiment.GetStaleCache(); ok {
			return h.bot.SendWithKeyboard(ctx, chatID,
				fmt.Sprintf("⚠️ <b>Refresh gagal, menampilkan data terakhir</b>\n\n"+
					"<i>Data terakhir: %s</i>\n\n"+
					"%s", staleData.FetchedAt.Format("02 Jan 2006 15:04"),
					h.fmt.FormatSentiment(staleData, h.currentMacroRegimeName(ctx))),
				h.kb.RelatedCommandsKeyboard("sentiment", ""))
		}
		
		return h.bot.SendMessage(ctx, chatID,
			"❌ <b>Gagal refresh data sentiment</b>\n\n"+
				"Silakan coba lagi dalam beberapa menit.")
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
	
	return h.bot.SendWithKeyboard(ctx, chatID, htmlMsg, kb)
}
