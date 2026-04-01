package telegram

// /treasury — US Treasury Auction Results Dashboard.
// Shows recent auction bid-to-cover ratios, indirect bidder %, and trend analysis.
// Data from TreasuryDirect API (no auth required).

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/treasury"
)

// cmdTreasury handles the /treasury command.
func (h *Handler) cmdTreasury(ctx context.Context, chatID string, _ int64, _ string) error {
	cacheStatus := "🏛️ Fetching Treasury auction data... ⏳"
	if treasury.CacheAge() >= 0 {
		cacheStatus = "🏛️ Loading Treasury auction data (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, cacheStatus)

	data, err := treasury.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Treasury auction fetch failed")
		if placeholderID > 0 {
			_ = h.bot.EditMessage(ctx, chatID, placeholderID,
				"❌ Failed to fetch Treasury auction data. Please try again later.")
		}
		return nil
	}

	htmlMsg := h.fmt.FormatTreasury(data)

	if placeholderID > 0 {
		return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
	}
	_, err = h.bot.SendHTML(ctx, chatID, htmlMsg)
	return err
}
