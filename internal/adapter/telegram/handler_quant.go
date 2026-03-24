package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	fredSvc "github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// cmdCorr handles /corr — cross-pair correlation matrix.
func (h *Handler) cmdCorr(ctx context.Context, chatID string, userID int64, args string) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Correlation data not available yet. Daily prices are being initialized.")
		return err
	}

	engine := pricesvc.NewCorrelationEngine(h.dailyPriceRepo)
	matrix, err := engine.BuildWithBreakdowns(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Failed to compute correlation: %s", err.Error()))
		return sendErr
	}

	htmlOut := h.fmt.FormatCorrelationMatrix(matrix)

	// Detect clusters
	clusters := engine.DetectClusters(matrix, 0.70)
	if len(clusters) > 0 {
		htmlOut += "\n" + h.fmt.FormatCorrelationClusters(clusters)
	}

	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// cmdCarry handles /carry — interest rate differential and carry trade ranking.
func (h *Handler) cmdCarry(ctx context.Context, chatID string, userID int64, args string) error {
	_, _ = h.bot.SendHTML(ctx, chatID, "Fetching central bank rates...")

	engine := fredSvc.NewRateDifferentialEngine()
	ranking, err := engine.FetchCarryRanking(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Failed to fetch carry data: %s", err.Error()))
		return sendErr
	}

	htmlOut := h.fmt.FormatCarryRanking(ranking)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// cmdIntraday handles /intraday [currency] — 4H price context.
func (h *Handler) cmdIntraday(ctx context.Context, chatID string, userID int64, args string) error {
	if h.intradayRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Intraday data not available yet. 4H data is being initialized.")
		return err
	}

	args = strings.TrimSpace(strings.ToUpper(args))

	if args == "" {
		return h.intradayOverview(ctx, chatID)
	}

	mapping := domain.FindPriceMappingByCurrency(args)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Unknown instrument: <code>%s</code>\n\nUsage: <code>/intraday EUR</code>",
			args,
		))
		return err
	}

	return h.intradayDetail(ctx, chatID, mapping)
}

// intradayOverview shows a quick 4H snapshot of key instruments.
func (h *Handler) intradayOverview(ctx context.Context, chatID string) error {
	builder := pricesvc.NewIntradayContextBuilder(h.intradayRepo)

	currencies := []string{"EUR", "GBP", "JPY", "AUD", "XAU", "OIL", "BTC", "SPX500"}
	var lines []string

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			continue
		}
		ic, err := builder.Build(ctx, mapping.ContractCode, mapping.Currency)
		if err != nil {
			continue
		}

		arrow := "→"
		if ic.Chg4H > 0 {
			arrow = "▲"
		} else if ic.Chg4H < 0 {
			arrow = "▼"
		}

		trend := ic.IntradayMATrend()
		trendIcon := "⚪"
		switch trend {
		case "BULLISH":
			trendIcon = "🟢"
		case "BEARISH":
			trendIcon = "🔴"
		}

		lines = append(lines, fmt.Sprintf(
			"<code>%-6s %+.2f%% 24h:%+.2f%%</code> %s %s",
			ic.Currency, ic.Chg4H, ic.Chg24H, arrow, trendIcon,
		))
	}

	if len(lines) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "No intraday data available yet. 4H data is fetched every 4 hours.")
		return err
	}

	msg := "⏰ <b>4H INTRADAY OVERVIEW</b>\n\n" +
		strings.Join(lines, "\n") +
		"\n\n<i>Use</i> <code>/intraday EUR</code> <i>for detailed view</i>"

	_, err := h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// intradayDetail shows detailed 4H context for a single instrument.
func (h *Handler) intradayDetail(ctx context.Context, chatID string, mapping *domain.PriceSymbolMapping) error {
	builder := pricesvc.NewIntradayContextBuilder(h.intradayRepo)
	ic, err := builder.Build(ctx, mapping.ContractCode, mapping.Currency)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"No intraday data for <code>%s</code> yet.\n4H data is fetched every 4 hours.",
			mapping.Currency,
		))
		return sendErr
	}

	htmlOut := h.fmt.FormatIntradayContext(ic)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// ---------------------------------------------------------------------------
// GARCH(1,1) Volatility Forecast
// ---------------------------------------------------------------------------

// cmdGarch handles /garch [currency] — GARCH(1,1) volatility forecast.
func (h *Handler) cmdGarch(ctx context.Context, chatID string, userID int64, args string) error {
	args = strings.TrimSpace(strings.ToUpper(args))

	if args == "" {
		return h.garchOverview(ctx, chatID)
	}

	mapping := domain.FindPriceMappingByCurrency(args)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Unknown instrument: <code>%s</code>\n\nUsage: <code>/garch EUR</code>",
			args,
		))
		return err
	}

	return h.garchDetail(ctx, chatID, mapping)
}

// garchOverview shows GARCH vol forecast for key instruments.
func (h *Handler) garchOverview(ctx context.Context, chatID string) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet.")
		return err
	}

	currencies := []string{"EUR", "GBP", "JPY", "AUD", "XAU", "OIL", "BTC", "SPX500"}
	var lines []string

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			continue
		}
		prices, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 120)
		if err != nil || len(prices) < 30 {
			continue
		}

		records := dailyToPriceRecords(prices)
		garch, err := pricesvc.EstimateGARCH(records)
		if err != nil {
			continue
		}

		icon := "⚪"
		switch garch.VolForecast {
		case "INCREASING":
			icon = "🔴"
		case "DECREASING":
			icon = "🟢"
		}

		lines = append(lines, fmt.Sprintf(
			"<code>%-6s Vol:%.4f%% Fwd:%.4f%% R:%.2f</code> %s",
			cur, garch.CurrentVol*100, garch.ForecastVol1*100, garch.VolRatio, icon,
		))
	}

	if len(lines) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "Insufficient price data for GARCH estimation (need ≥30 daily bars).")
		return err
	}

	msg := "📊 <b>GARCH(1,1) VOLATILITY FORECAST</b>\n\n" +
		strings.Join(lines, "\n") +
		"\n\n🔴 Increasing  ⚪ Stable  🟢 Decreasing\n" +
		"<i>Use</i> <code>/garch EUR</code> <i>for detailed view</i>"

	_, err := h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// garchDetail shows detailed GARCH output for a single instrument.
func (h *Handler) garchDetail(ctx context.Context, chatID string, mapping *domain.PriceSymbolMapping) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet.")
		return err
	}

	prices, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 120)
	if err != nil || len(prices) < 30 {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Insufficient data for <code>%s</code> GARCH (need ≥30 daily bars, got %d).",
			mapping.Currency, len(prices),
		))
		return sendErr
	}

	records := dailyToPriceRecords(prices)
	garch, err := pricesvc.EstimateGARCH(records)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("GARCH estimation failed: %s", err.Error()))
		return sendErr
	}

	htmlOut := h.fmt.FormatGARCH(mapping.Currency, garch)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// ---------------------------------------------------------------------------
// Hurst Exponent — Regime Analysis
// ---------------------------------------------------------------------------

// cmdHurst handles /hurst [currency] — Hurst exponent analysis.
func (h *Handler) cmdHurst(ctx context.Context, chatID string, userID int64, args string) error {
	args = strings.TrimSpace(strings.ToUpper(args))

	if args == "" {
		return h.hurstOverview(ctx, chatID)
	}

	mapping := domain.FindPriceMappingByCurrency(args)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Unknown instrument: <code>%s</code>\n\nUsage: <code>/hurst EUR</code>",
			args,
		))
		return err
	}

	return h.hurstDetail(ctx, chatID, mapping)
}

// hurstOverview shows Hurst exponent for key instruments.
func (h *Handler) hurstOverview(ctx context.Context, chatID string) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet.")
		return err
	}

	currencies := []string{"EUR", "GBP", "JPY", "AUD", "CAD", "CHF", "NZD", "XAU", "OIL", "BTC"}
	var lines []string

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			continue
		}
		prices, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 200)
		if err != nil || len(prices) < 50 {
			continue
		}

		records := dailyToPriceRecords(prices)
		hurst, err := pricesvc.ComputeHurstExponent(records)
		if err != nil {
			continue
		}

		icon := "⚪"
		switch hurst.Classification {
		case "TRENDING":
			icon = "📈"
		case "MEAN_REVERTING":
			icon = "🔄"
		}

		lines = append(lines, fmt.Sprintf(
			"<code>%-6s H=%.3f %-15s R²=%.2f</code> %s",
			cur, hurst.H, hurst.Classification, hurst.RSquared, icon,
		))
	}

	if len(lines) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "Insufficient price data for Hurst estimation (need ≥50 daily bars).")
		return err
	}

	msg := "📐 <b>HURST EXPONENT — REGIME ANALYSIS</b>\n\n" +
		strings.Join(lines, "\n") +
		"\n\n📈 Trending  ⚪ Random Walk  🔄 Mean-Reverting\n" +
		"<i>Use</i> <code>/hurst EUR</code> <i>for detailed view</i>"

	_, err := h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// hurstDetail shows detailed Hurst analysis for a single instrument.
func (h *Handler) hurstDetail(ctx context.Context, chatID string, mapping *domain.PriceSymbolMapping) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet.")
		return err
	}

	prices, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 200)
	if err != nil || len(prices) < 50 {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Insufficient data for <code>%s</code> Hurst (need ≥50 daily bars, got %d).",
			mapping.Currency, len(prices),
		))
		return sendErr
	}

	records := dailyToPriceRecords(prices)
	hurst, err := pricesvc.ComputeHurstExponent(records)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Hurst estimation failed: %s", err.Error()))
		return sendErr
	}

	// Also get ADX regime for combined view
	var regime *pricesvc.HurstRegimeContext
	if len(records) >= 14 {
		adxRegime := pricesvc.ClassifyPriceRegime(records, nil)
		regime = pricesvc.CombineRegimeClassification(adxRegime, hurst)
	}

	htmlOut := h.fmt.FormatHurst(mapping.Currency, hurst, regime)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// dailyToPriceRecords converts DailyPrice slice to PriceRecord slice.
func dailyToPriceRecords(prices []domain.DailyPrice) []domain.PriceRecord {
	records := make([]domain.PriceRecord, len(prices))
	for i, p := range prices {
		records[i] = domain.PriceRecord{
			Date:  p.Date,
			Open:  p.Open,
			High:  p.High,
			Low:   p.Low,
			Close: p.Close,
		}
	}
	return records
}
