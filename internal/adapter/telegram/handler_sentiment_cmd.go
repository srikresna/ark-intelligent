package telegram

// /sentiment — Sentiment Survey Dashboard

import (
	"context"
	"strings"

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

	data, err := sentiment.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("sentiment data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"Failed to fetch sentiment data. Please try again later.")
	}

	if !data.CNNAvailable && !data.AAIIAvailable {
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"⚠️ Sentiment data currently unavailable from all sources. Try again later.")
	}

	htmlMsg := h.fmt.FormatSentiment(data, h.currentMacroRegimeName(ctx))
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}
