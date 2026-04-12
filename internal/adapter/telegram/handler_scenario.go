package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// cmdScenario handles /scenario [CURRENCY] [DAYS] — Monte Carlo price scenarios.
//
// Examples:
//
//	/scenario EUR       → 30-day EUR/USD scenario
//	/scenario XAU 60    → 60-day gold scenario
//	/scenario BTC 14    → 14-day BTC scenario
func (h *Handler) cmdScenario(ctx context.Context, chatID string, _ int64, args string) error {
	if h.quant == nil || h.quant.DailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID,
			"📊 <b>Scenario Generator</b>\n\n"+
				"<i>Daily price data not available yet. "+
				"Price tracking is being initialized.</i>")
		return err
	}

	parts := strings.Fields(strings.TrimSpace(strings.ToUpper(args)))
	if len(parts) == 0 {
		return h.scenarioHelp(ctx, chatID)
	}

	currency := parts[0]
	horizonDays := 30
	if len(parts) >= 2 {
		if d, err := strconv.Atoi(parts[1]); err == nil && d > 0 && d <= 90 {
			horizonDays = d
		}
	}

	mapping := domain.FindPriceMappingByCurrency(currency)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Unknown currency: <code>%s</code>\n\nUsage: <code>/scenario EUR 30</code>",
			html.EscapeString(currency),
		))
		return err
	}

	h.bot.SendTyping(ctx, chatID)

	// Fetch 500 days of price history for robust GARCH + HMM estimation
	dailyPrices, err := h.quant.DailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 500)
	if err != nil || len(dailyPrices) < 60 {
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			"📊 <b>Scenario Generator</b>\n\n"+
				"<i>Insufficient price history for Monte Carlo simulation. "+
				"Need at least 60 daily observations.</i>")
		return sendErr
	}

	// Convert DailyPrice → PriceRecord for GARCH/HMM
	priceRecords := dailyPricesToPriceRecords(dailyPrices)

	cfg := &pricesvc.ScenarioConfig{
		NumPaths:    1000,
		HorizonDays: horizonDays,
	}

	result, err := pricesvc.GenerateScenario(priceRecords, mapping.Currency, cfg)
	if err != nil {
		log.Error().Err(err).Str("symbol", mapping.Currency).Msg("scenario simulation failed")
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			"📊 <b>Scenario Generator</b>\n\n"+
				"<i>Simulation failed. Insufficient data or internal error — please try again later.</i>")
		return sendErr
	}

	text := formatScenarioResult(result, currency)
	_, sendErr := h.bot.SendHTML(ctx, chatID, text)
	return sendErr
}

// scenarioHelp shows usage for the /scenario command.
func (h *Handler) scenarioHelp(ctx context.Context, chatID string) error {
	_, err := h.bot.SendHTML(ctx, chatID,
		"📊 <b>Monte Carlo Scenario Generator</b>\n\n"+
			"Generate probabilistic price scenarios using GARCH volatility "+
			"and HMM regime-conditional drift.\n\n"+
			"<b>Usage:</b>\n"+
			"<code>/scenario EUR</code> — 30-day EUR/USD scenarios\n"+
			"<code>/scenario XAU 60</code> — 60-day gold scenarios\n"+
			"<code>/scenario BTC 14</code> — 14-day BTC scenarios\n\n"+
			"<b>Output:</b> Price distribution (P5–P95), VaR, CVaR, "+
			"current regime, volatility estimate.\n\n"+
			"<i>1,000 simulated paths per run. Max horizon: 90 days.</i>")
	return err
}
