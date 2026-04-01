package telegram

// handler_orderflow.go — /orderflow command: Estimated Delta & Order Flow Analysis
//   /orderflow [SYMBOL] [TIMEFRAME]
//   Example: /orderflow EUR 4h
//            /orderflow BTC daily

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/orderflow"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// OrderFlowServices holds the price data repositories needed for /orderflow.
type OrderFlowServices struct {
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore // may be nil — falls back to daily bars
}

// WithOrderFlow injects OrderFlowServices into the handler and registers commands.
func (h *Handler) WithOrderFlow(s *OrderFlowServices) *Handler {
	h.orderFlow = s
	if s != nil {
		h.registerOrderFlowCommands()
	}
	return h
}

// registerOrderFlowCommands wires the /orderflow and /of2 commands.
func (h *Handler) registerOrderFlowCommands() {
	h.bot.RegisterCommand("/orderflow", h.cmdOrderFlow)
}

// cmdOrderFlow handles /orderflow [SYMBOL] [TIMEFRAME].
func (h *Handler) cmdOrderFlow(ctx context.Context, chatID string, _ int64, args string) error {
	if h.orderFlow == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Order Flow service not configured.")
		return err
	}

	symbol, timeframe := parseOrderFlowArgs(args)

	// Resolve the price mapping.
	mapping := domain.FindPriceMappingByCurrency(strings.ToUpper(symbol))
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/orderflow EUR</code>, <code>/orderflow BTC 4h</code>",
			symbol))
		return err
	}

	h.bot.SendTyping(ctx, chatID)

	bars, tf, err := h.fetchOrderFlowBars(ctx, mapping, timeframe)
	if err != nil || len(bars) < 3 {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ Tidak cukup data untuk <b>%s %s</b>. Coba lagi nanti.", mapping.Currency, timeframe))
		if sendErr != nil {
			return sendErr
		}
		return nil
	}

	result := orderflow.Analyze(bars, mapping.Currency, tf)
	txt := formatOrderFlowResult(result)
	_, err = h.bot.SendHTML(ctx, chatID, txt)
	return err
}

// parseOrderFlowArgs splits raw command args into (symbol, timeframe).
// Defaults: symbol="EUR", timeframe="4h".
func parseOrderFlowArgs(args string) (symbol, timeframe string) {
	symbol = "EUR"
	timeframe = "4h"

	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) >= 1 {
		symbol = parts[0]
	}
	if len(parts) >= 2 {
		timeframe = strings.ToLower(parts[1])
	}
	return symbol, timeframe
}

// fetchOrderFlowBars retrieves OHLCV bars for the requested timeframe.
// Falls back to daily bars if intraday is unavailable or not requested.
func (h *Handler) fetchOrderFlowBars(
	ctx context.Context,
	mapping *domain.PriceSymbolMapping,
	timeframe string,
) ([]ta.OHLCV, string, error) {
	code := mapping.ContractCode

	if timeframe == "daily" || timeframe == "1d" || h.orderFlow.IntradayRepo == nil {
		records, err := h.orderFlow.DailyPriceRepo.GetDailyHistory(ctx, code, orderflow.MaxBars+5)
		if err != nil {
			return nil, "daily", err
		}
		return ta.DailyPricesToOHLCV(records), "daily", nil
	}

	// Try intraday.
	bars, err := h.orderFlow.IntradayRepo.GetHistory(ctx, code, timeframe, orderflow.MaxBars+5)
	if err != nil || len(bars) < 3 {
		// Fallback to daily.
		records, dailyErr := h.orderFlow.DailyPriceRepo.GetDailyHistory(ctx, code, orderflow.MaxBars+5)
		if dailyErr != nil {
			return nil, "daily", dailyErr
		}
		return ta.DailyPricesToOHLCV(records), "daily", nil
	}
	return ta.IntradayBarsToOHLCV(bars), timeframe, nil
}
