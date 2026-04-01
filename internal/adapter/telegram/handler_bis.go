package telegram

// /bis — BIS Statistics Dashboard: central bank policy rates, credit-to-GDP gaps,
// and global liquidity indicators from the Bank for International Settlements.

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/bis"
)

// cmdBIS handles the /bis command — BIS macro institutional data.
func (h *Handler) cmdBIS(ctx context.Context, chatID string, _ int64, _ string) error {
	cacheStatus := "🏦 Fetching BIS data... ⏳ (5-15s)"
	if bis.SummaryCacheAge() >= 0 {
		cacheStatus = "🏦 Loading BIS data (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, cacheStatus)

	data, err := bis.GetBISSummary(ctx)
	if err != nil {
		log.Error().Err(err).Msg("BIS summary fetch failed")
		errMsg := "❌ Failed to fetch BIS data. Please try again later."
		if placeholderID > 0 {
			return h.bot.EditMessage(ctx, chatID, placeholderID, errMsg)
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	htmlMsg := h.fmt.FormatBISSummary(data)
	if placeholderID > 0 {
		return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
	}
	_, sendErr := h.bot.SendHTML(ctx, chatID, htmlMsg)
	return sendErr
}
