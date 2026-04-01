package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// cmdLevels handles /levels [currency] — support/resistance levels + position sizing.
func (h *Handler) cmdLevels(ctx context.Context, chatID string, userID int64, args string) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Daily price data not available yet. Price tracking is being initialized.")
		return err
	}

	args = strings.TrimSpace(strings.ToUpper(args))

	if args == "" {
		return h.levelsOverview(ctx, chatID)
	}

	mapping := domain.FindPriceMappingByCurrency(args)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Unknown currency: <code>%s</code>\n\nUsage: <code>/levels [currency]</code>\nExamples: <code>/levels EUR</code> · <code>/levels XAU</code>",
			html.EscapeString(args),
		))
		return err
	}

	return h.levelsDetail(ctx, chatID, mapping)
}

// levelsOverview shows key support/resistance summary for major instruments.
func (h *Handler) levelsOverview(ctx context.Context, chatID string) error {
	h.bot.SendTyping(ctx, chatID)

	builder := pricesvc.NewLevelsBuilder(h.dailyPriceRepo)

	currencies := []string{"EUR", "GBP", "JPY", "AUD", "XAU", "OIL", "BTC"}
	var lines []string

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			continue
		}
		lc, err := builder.Build(ctx, mapping.ContractCode, mapping.Currency)
		if err != nil {
			continue
		}

		supportStr := "—"
		resistStr := "—"
		if lc.NearestSupport != nil {
			supportStr = fmt.Sprintf("%s (%+.2f%%)", formatLevelPrice(lc.NearestSupport.Price, cur), lc.NearestSupport.Distance)
		}
		if lc.NearestResistance != nil {
			resistStr = fmt.Sprintf("%s (%+.2f%%)", formatLevelPrice(lc.NearestResistance.Price, cur), lc.NearestResistance.Distance)
		}

		lines = append(lines, fmt.Sprintf(
			"<code>%-4s</code> S: %s | R: %s",
			cur, supportStr, resistStr,
		))
	}

	if len(lines) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No daily price data available yet for level computation.")
		return err
	}

	msg := "\xF0\x9F\x93\x8F <b>KEY LEVELS OVERVIEW</b>\n\n" +
		strings.Join(lines, "\n") +
		"\n\n<i>Use</i> <code>/levels EUR</code> <i>for detailed S/R + sizing</i>"

	_, err := h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// levelsDetail shows detailed S/R levels, pivots, and position sizing for one instrument.
func (h *Handler) levelsDetail(ctx context.Context, chatID string, mapping *domain.PriceSymbolMapping) error {
	builder := pricesvc.NewLevelsBuilder(h.dailyPriceRepo)
	lc, err := builder.Build(ctx, mapping.ContractCode, mapping.Currency)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"No daily price data for <code>%s</code> yet.\nData is fetched every 6 hours.",
			mapping.Currency,
		))
		return sendErr
	}

	htmlOut := h.fmt.FormatLevels(lc, mapping.Currency)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// formatLevelPrice formats a price for level display.
func formatLevelPrice(price float64, currency string) string {
	return formatPrice(price, currency)
}
