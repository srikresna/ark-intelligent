package telegram

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/finviz"
)

// cmdMarket handles the /market command — cross-asset overview via Finviz.
func (h *Handler) cmdMarket(ctx context.Context, chatID string, _ int64, _ string) error {
	h.bot.SendTyping(ctx, chatID)

	overview := finviz.GetCachedOrFetch(ctx)
	txt := formatMarketOverview(overview)
	_, err := h.bot.SendHTML(ctx, chatID, txt)
	return err
}
