package telegram

// /eurostat — EU Economy Dashboard from Eurostat API.
// Shows EU HICP inflation (headline + core), unemployment rate, and GDP growth.
// Data from Eurostat free API (no auth required).

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/macro"
)

// cmdEurostat handles the /eurostat command — EU macro data from Eurostat.
func (h *Handler) cmdEurostat(ctx context.Context, chatID string, _ int64, _ string) error {
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, "🇪🇺 Fetching EU macro data from Eurostat... ⏳")

	data, err := macro.GetEurostatData(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Eurostat data fetch failed")
		errMsg := "❌ Failed to fetch Eurostat EU data. Please try again later."
		if placeholderID > 0 {
			return h.bot.EditMessage(ctx, chatID, placeholderID, errMsg)
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	htmlMsg := macro.FormatEurostatData(data)
	if placeholderID > 0 {
		return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
	}
	_, sendErr := h.bot.SendHTML(ctx, chatID, htmlMsg)
	return sendErr
}
