package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// cmdPrice handles /price [currency] — daily price quote with technical context.
func (h *Handler) cmdPrice(ctx context.Context, chatID string, userID int64, args string) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Daily price data not available yet. Price tracking is being initialized.")
		return err
	}

	args = strings.TrimSpace(strings.ToUpper(args))

	if args == "" {
		return h.priceOverview(ctx, chatID)
	}

	// Look up currency
	mapping := domain.FindPriceMappingByCurrency(args)
	if mapping == nil {
		kb := h.kb.PriceMenu()
		_, err := h.bot.SendWithKeyboard(ctx, chatID, fmt.Sprintf(
			"Unknown instrument: <code>%s</code>\n\nTap a button below or type e.g. <code>/price EUR</code>",
			html.EscapeString(args),
		), kb)
		return err
	}

	return h.priceDetail(ctx, chatID, mapping)
}

// priceOverview shows a categorized snapshot of all major instruments.
func (h *Handler) priceOverview(ctx context.Context, chatID string) error {
	h.bot.SendTyping(ctx, chatID)

	builder := pricesvc.NewDailyContextBuilder(h.dailyPriceRepo)

	type section struct {
		title      string
		currencies []string
	}
	sections := []section{
		{"💱 FX Majors", []string{"EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF", "USD"}},
		{"🪙 Metals & Energy", []string{"XAU", "XAG", "OIL", "COPPER"}},
		{"📈 Indices", []string{"SPX500", "NDX", "DJI", "RUT"}},
		{"₿ Crypto", []string{"BTC", "ETH"}},
	}

	var blocks []string
	for _, sec := range sections {
		var lines []string
		for _, cur := range sec.currencies {
			mapping := domain.FindPriceMappingByCurrency(cur)
			if mapping == nil {
				continue
			}
			dc, err := builder.Build(ctx, mapping.ContractCode, mapping.Currency)
			if err != nil {
				continue
			}

			arrow := "→"
			if dc.DailyChgPct > 0 {
				arrow = "▲"
			} else if dc.DailyChgPct < 0 {
				arrow = "▼"
			}

			lines = append(lines, fmt.Sprintf(
				"<code>%-7s %s %+.2f%%</code> %s",
				dc.Currency, formatPrice(dc.CurrentPrice, dc.Currency), dc.DailyChgPct, arrow,
			))
		}
		if len(lines) > 0 {
			blocks = append(blocks, sec.title+"\n"+strings.Join(lines, "\n"))
		}
	}

	if len(blocks) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No daily price data available yet. Data is fetched every 6 hours.")
		return err
	}

	msg := "💹 <b>PRICE OVERVIEW</b>\n\n" +
		strings.Join(blocks, "\n\n") +
		"\n\n<i>Tap below for detailed view</i>"

	kb := h.kb.PriceMenu()
	_, err := h.bot.SendWithKeyboard(ctx, chatID, msg, kb)
	return err
}

// priceDetail shows detailed daily price context for a single instrument.
func (h *Handler) priceDetail(ctx context.Context, chatID string, mapping *domain.PriceSymbolMapping) error {
	builder := pricesvc.NewDailyContextBuilder(h.dailyPriceRepo)
	dc, err := builder.Build(ctx, mapping.ContractCode, mapping.Currency)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"No daily price data for <code>%s</code> yet.\nData is fetched every 6 hours.",
			mapping.Currency,
		))
		return sendErr
	}

	htmlOut := h.fmt.FormatDailyPrice(dc)
	kb := h.kb.PriceMenu()
	_, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, kb)
	return err
}

// formatPrice formats a price with appropriate decimal places based on instrument.
func formatPrice(price float64, currency string) string {
	switch {
	case currency == "JPY":
		return fmt.Sprintf("%.3f", price)
	case currency == "XAU" || currency == "XAG":
		return fmt.Sprintf("%.2f", price)
	case currency == "BTC" || currency == "ETH":
		return fmt.Sprintf("%.0f", price)
	case currency == "OIL" || currency == "COPPER":
		return fmt.Sprintf("%.2f", price)
	case strings.HasPrefix(currency, "BOND") || currency == "SPX500" || currency == "NDX" || currency == "DJI" || currency == "RUT":
		return fmt.Sprintf("%.2f", price)
	default:
		// FX pairs — 5 decimal places for standard, fewer for others
		if price > 10 {
			return fmt.Sprintf("%.4f", price)
		}
		return fmt.Sprintf("%.5f", price)
	}
}
