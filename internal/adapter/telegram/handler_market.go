package telegram

// /market — Cross-Asset Market Dashboard (Finviz via Firecrawl)

import (
	"context"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/finviz"
)

// cmdMarket handles the /market command — cross-asset performance dashboard.
func (h *Handler) cmdMarket(ctx context.Context, chatID string, userID int64, args string) error {
	forceRefresh := strings.EqualFold(strings.TrimSpace(args), "refresh")
	if forceRefresh {
		if !h.requireAdmin(ctx, chatID, userID) {
			return nil
		}
		finviz.InvalidateCache()
	}

	cacheStatus := "📊 Fetching cross-asset data... ⏳"
	if !forceRefresh && finviz.CacheAge() >= 0 {
		cacheStatus = "📊 Loading market data (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, cacheStatus)

	data, err := finviz.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("finviz data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"Failed to fetch cross-asset data. Please try again later.")
	}

	if !data.Available {
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"⚠️ Cross-asset data currently unavailable. Ensure FIRECRAWL_API_KEY is configured.")
	}

	htmlMsg := h.fmt.FormatMarket(data)
	kb := h.kb.RelatedCommandsKeyboard("market", "")
	if len(kb.Rows) > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	}
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}
