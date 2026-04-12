package telegram

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/sec"
)

// cmdSEC handles the /13f command — SEC EDGAR 13F Institutional Holdings.
func (h *Handler) cmdSEC(ctx context.Context, chatID string, _ int64, _ string) error {
	cacheStatus := "🏛️ Fetching SEC EDGAR 13F data... ⏳"
	if sec.CacheAge() >= 0 {
		cacheStatus = "🏛️ Loading 13F institutional holdings (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, cacheStatus)

	data, err := sec.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("SEC EDGAR 13F fetch failed")
		if placeholderID > 0 {
			_ = h.bot.EditMessage(ctx, chatID, placeholderID,
				"❌ Failed to fetch SEC EDGAR 13F data. Please try again later.")
		}
		return nil
	}

	htmlMsg := h.fmt.FormatSEC13F(data)

	// Alert on significant new positions (>$1B).
	alerts := sec.DetectSignificantMoves(data)
	if len(alerts) > 0 {
		htmlMsg += h.fmt.FormatSEC13FAlerts(alerts)
	}

	if placeholderID > 0 {
		return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
	}
	_, err = h.bot.SendHTML(ctx, chatID, htmlMsg)
	return err
}
