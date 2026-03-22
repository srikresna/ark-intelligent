package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// cmdSeasonal handles /seasonal [currency] — historical monthly return patterns.
func (h *Handler) cmdSeasonal(ctx context.Context, chatID string, _ int64, args string) error {
	if h.priceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet. Prices are fetched periodically.")
		return err
	}

	analyzer := pricesvc.NewSeasonalAnalyzer(h.priceRepo)
	args = strings.TrimSpace(strings.ToUpper(args))

	if args != "" {
		// Single contract mode — look up by currency code
		mapping := domain.FindPriceMappingByCurrency(args)
		if mapping == nil || mapping.RiskOnly {
			_, err := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("Unknown currency: <code>%s</code>\n\nUsage: <code>/seasonal</code> (all) or <code>/seasonal EUR</code>",
					html.EscapeString(args)))
			return err
		}

		pattern, err := analyzer.AnalyzeContract(ctx, mapping.ContractCode, mapping.Currency)
		if err != nil {
			_, sendErr := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("No seasonal data for %s: %s", html.EscapeString(args), html.EscapeString(err.Error())))
			return sendErr
		}

		htmlOut := h.fmt.FormatSeasonalSingle(*pattern)
		_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
		return err
	}

	// All contracts mode
	patterns, err := analyzer.Analyze(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("Seasonal analysis unavailable: %s", html.EscapeString(err.Error())))
		return sendErr
	}

	htmlOut := h.fmt.FormatSeasonalPatterns(patterns)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}
