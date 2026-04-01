package telegram

// handler_wyckoff.go — /wyckoff command handler.
// Implements Wyckoff Method structure detection (Accumulation/Distribution).

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	"github.com/arkcode369/ark-intelligent/internal/service/wyckoff"
)

// ---------------------------------------------------------------------------
// WyckoffServices — injected dependencies
// ---------------------------------------------------------------------------

// WyckoffServices holds dependencies for the /wyckoff command.
type WyckoffServices struct {
	DailyPriceRepo  pricesvc.DailyPriceStore
	IntradayRepo    pricesvc.IntradayStore // may be nil
	WyckoffEngine   *wyckoff.Engine
}

// ---------------------------------------------------------------------------
// WithWyckoff wires the Wyckoff handler into the main Handler.
// ---------------------------------------------------------------------------

func (h *Handler) WithWyckoff(svc WyckoffServices) {
	h.wyckoff = &svc
	h.bot.RegisterCommand("/wyckoff", h.cmdWyckoff)
}

// ---------------------------------------------------------------------------
// /wyckoff command
// ---------------------------------------------------------------------------

// cmdWyckoff handles /wyckoff [SYMBOL] [TIMEFRAME]
// Examples:
//   /wyckoff EURUSD
//   /wyckoff XAUUSD H4
//   /wyckoff BTCUSD daily
func (h *Handler) cmdWyckoff(ctx context.Context, chatID string, userID int64, args string) error {
	if h.wyckoff == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚠️ Wyckoff engine tidak tersedia.")
		return err
	}

	parts := strings.Fields(strings.TrimSpace(strings.ToUpper(args)))
	if len(parts) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, `📊 <b>Wyckoff Structure Analysis</b>

Gunakan: <code>/wyckoff [SYMBOL] [TIMEFRAME]</code>

Contoh:
  <code>/wyckoff EURUSD</code>
  <code>/wyckoff XAUUSD H4</code>
  <code>/wyckoff BTCUSD daily</code>

Timeframe yang didukung: <code>daily</code>, <code>4h</code>, <code>1h</code>`)
		return err
	}

	currency := parts[0]
	timeframe := "daily"
	if len(parts) > 1 {
		timeframe = strings.ToLower(parts[1])
		// Normalize timeframe aliases
		switch timeframe {
		case "h4", "4hour", "4h":
			timeframe = "4h"
		case "h1", "1hour", "1h":
			timeframe = "1h"
		case "d", "d1", "daily":
			timeframe = "daily"
		}
	}

	mapping := domain.FindPriceMappingByCurrency(currency)
	if mapping == nil || mapping.RiskOnly {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("❌ Symbol tidak dikenal: <code>%s</code>", html.EscapeString(currency)))
		return err
	}

	// Loading indicator
	msgID, _ := h.bot.SendLoading(ctx, chatID,
		fmt.Sprintf("⏳ Menganalisis Wyckoff Structure <b>%s</b> (%s)...",
			html.EscapeString(mapping.Currency), timeframe))

	bars, err := h.fetchWyckoffBars(ctx, mapping, timeframe)
	if err != nil || len(bars) == 0 {
		errMsg := fmt.Sprintf("❌ Gagal mengambil data harga untuk <b>%s</b>: %s",
			html.EscapeString(mapping.Currency), html.EscapeString(fmt.Sprintf("%v", err)))
		if msgID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	result := h.wyckoff.WyckoffEngine.Analyze(mapping.Currency, strings.ToUpper(timeframe), bars)
	output := h.fmt.FormatWyckoffResult(result)

	if msgID > 0 {
		return h.bot.EditMessage(ctx, chatID, msgID, output)
	}
	_, sendErr := h.bot.SendHTML(ctx, chatID, output)
	return sendErr
}

// ---------------------------------------------------------------------------
// fetchWyckoffBars — fetch OHLCV bars for Wyckoff analysis
// ---------------------------------------------------------------------------

func (h *Handler) fetchWyckoffBars(ctx context.Context, mapping *domain.PriceSymbolMapping, timeframe string) ([]ta.OHLCV, error) {
	code := mapping.ContractCode

	switch timeframe {
	case "4h", "1h":
		if h.wyckoff.IntradayRepo == nil {
			return nil, fmt.Errorf("intraday data tidak tersedia")
		}
		count := 300
		intradayBars, err := h.wyckoff.IntradayRepo.GetHistory(ctx, code, timeframe, count)
		if err != nil {
			return nil, fmt.Errorf("fetch intraday bars: %w", err)
		}
		return ta.IntradayBarsToOHLCV(intradayBars), nil

	default: // "daily"
		dailyRecords, err := h.wyckoff.DailyPriceRepo.GetDailyHistory(ctx, code, 300)
		if err != nil {
			return nil, fmt.Errorf("fetch daily bars: %w", err)
		}
		return ta.DailyPricesToOHLCV(dailyRecords), nil
	}
}
