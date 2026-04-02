package telegram

// /tedge — Global Macro Dashboard via TradingEconomics (Firecrawl scraper)
// /globalm — alias for /tedge

import (
	"context"

	"github.com/arkcode369/ark-intelligent/internal/service/macro"
)

// cmdTEdge handles /tedge [country] — TradingEconomics global macro dashboard.
//
// Without arguments, shows all G10 countries at a glance.
// With a country code, shows detailed indicators for that country.
//
// Examples:
//
//	/tedge        → full G10 macro table
//	/globalm      → alias
//	/tedge US     → detailed US macro indicators
//	/tedge EUR    → detailed Euro Area macro indicators
func (h *Handler) cmdTEdge(ctx context.Context, chatID string, _ int64, args string) error {
	placeholderID, _ := h.bot.SendLoading(ctx, chatID,
		"🌍 Fetching global macro data (TradingEconomics)... ⏳")

	data, err := macro.GetTECachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("TradingEconomics macro fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"❌ Failed to fetch TradingEconomics data. Please try again later.")
	}

	if data == nil || !data.Available {
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"⚠️ Global macro data unavailable.\n"+
				"<i>FIRECRAWL_API_KEY might not be configured.</i>")
	}

	htmlMsg := macro.FormatTEGlobalMacro(data)
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}
