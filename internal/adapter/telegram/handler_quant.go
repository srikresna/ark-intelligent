package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
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
		errMsg := fmt.Sprintf(
			"<b>Correlation matrix unavailable</b>\n\n%s\n\n<i>Daily prices may still be loading. Try again in a few minutes.</i>",
			html.EscapeString(err.Error()),
		)
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	htmlOut := h.fmt.FormatCorrelationMatrix(matrix)

	// Note if the matrix fell back to a shorter period than the default 20-day
	if matrix.Period < 20 {
		htmlOut += fmt.Sprintf("\n<i>Note: Insufficient data for 20-day window; using %d-day fallback.</i>\n", matrix.Period)
	}

	// Report which currencies are included vs excluded from the matrix
	allCurrencies := domain.DefaultCorrelationCurrencies()
	includedSet := make(map[string]bool, len(matrix.Currencies))
	for _, c := range matrix.Currencies {
		includedSet[c] = true
	}
	var excluded []string
	for _, c := range allCurrencies {
		if !includedSet[c] {
			excluded = append(excluded, c)
		}
	}
	if len(excluded) > 0 {
		htmlOut += fmt.Sprintf("\n<i>Included: %s</i>", strings.Join(matrix.Currencies, ", "))
		htmlOut += fmt.Sprintf("\n<i>Excluded (insufficient data): %s</i>\n", strings.Join(excluded, ", "))
	}

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
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Failed to fetch carry data: %s", html.EscapeString(err.Error())))
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

	currencies := []string{"EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF", "XAU", "XAG", "OIL", "BTC", "ETH", "SPX500"}
	var lines []string
	skipped := 0

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			skipped++
			continue
		}
		ic, err := builder.Build(ctx, mapping.ContractCode, mapping.Currency)
		if err != nil {
			skipped++
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
	if skipped > 0 {
		msg += fmt.Sprintf("\n\n⚠️ %d instruments unavailable (insufficient data)", skipped)
	}

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

	currencies := []string{"EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF", "XAU", "XAG", "OIL", "BTC", "ETH", "SPX500"}
	var lines []string
	skipped := 0

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			skipped++
			continue
		}
		prices, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 120)
		if err != nil || len(prices) < 30 {
			skipped++
			continue
		}

		records := dailyToPriceRecords(prices)
		garch, err := pricesvc.EstimateGARCH(records)
		if err != nil {
			skipped++
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
	if skipped > 0 {
		msg += fmt.Sprintf("\n\n⚠️ %d instruments unavailable (insufficient data)", skipped)
	}

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
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("GARCH estimation failed: %s", html.EscapeString(err.Error())))
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

	currencies := []string{"EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF", "XAU", "XAG", "OIL", "BTC", "ETH", "SPX500"}
	var lines []string
	skipped := 0

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			skipped++
			continue
		}
		prices, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 200)
		if err != nil || len(prices) < 50 {
			skipped++
			continue
		}

		records := dailyToPriceRecords(prices)
		hurst, err := pricesvc.ComputeHurstExponent(records)
		if err != nil {
			skipped++
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
	if skipped > 0 {
		msg += fmt.Sprintf("\n\n⚠️ %d instruments unavailable (insufficient data)", skipped)
	}

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
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Hurst estimation failed: %s", html.EscapeString(err.Error())))
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

// ---------------------------------------------------------------------------
// HMM Regime-Switching Model
// ---------------------------------------------------------------------------

// cmdRegime handles /regime [currency] — HMM regime analysis.
func (h *Handler) cmdRegime(ctx context.Context, chatID string, userID int64, args string) error {
	args = strings.TrimSpace(strings.ToUpper(args))

	if args == "" {
		return h.regimeOverview(ctx, chatID)
	}

	mapping := domain.FindPriceMappingByCurrency(args)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Unknown instrument: <code>%s</code>\n\nUsage: <code>/regime EUR</code>",
			args,
		))
		return err
	}

	return h.regimeDetail(ctx, chatID, mapping)
}

// regimeOverview shows HMM regime for key instruments.
func (h *Handler) regimeOverview(ctx context.Context, chatID string) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet.")
		return err
	}

	currencies := []string{"EUR", "GBP", "JPY", "AUD", "NZD", "CAD", "CHF", "XAU", "XAG", "OIL", "BTC", "ETH", "SPX500"}
	var lines []string
	skipped := 0

	for _, cur := range currencies {
		mapping := domain.FindPriceMappingByCurrency(cur)
		if mapping == nil {
			skipped++
			continue
		}
		prices, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 120)
		if err != nil || len(prices) < 60 {
			skipped++
			continue
		}

		records := dailyToPriceRecords(prices)
		hmm, err := pricesvc.EstimateHMMRegime(records)
		if err != nil {
			skipped++
			continue
		}

		icon := "⚪"
		switch hmm.CurrentState {
		case pricesvc.HMMRiskOn:
			icon = "🟢"
		case pricesvc.HMMRiskOff:
			icon = "🟡"
		case pricesvc.HMMCrisis:
			icon = "🔴"
		}

		warning := ""
		if hmm.TransitionWarning != "" {
			warning = " ⚠️"
		}

		lines = append(lines, fmt.Sprintf(
			"<code>%-6s %-9s P:%.0f%%</code> %s%s",
			cur, hmm.CurrentState,
			hmm.StateProbabilities[stateIndex(hmm.CurrentState)]*100,
			icon, warning,
		))
	}

	if len(lines) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, "Insufficient price data for HMM regime estimation (need ≥60 daily bars).")
		return err
	}

	msg := "🔀 <b>HMM REGIME-SWITCHING MODEL</b>\n\n" +
		strings.Join(lines, "\n") +
		"\n\n🟢 Risk-On  🟡 Risk-Off  🔴 Crisis  ⚠️ Transition\n" +
		"<i>Use</i> <code>/regime EUR</code> <i>for detailed view</i>"
	if skipped > 0 {
		msg += fmt.Sprintf("\n\n⚠️ %d instruments unavailable (insufficient data)", skipped)
	}

	_, err := h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// regimeDetail shows detailed HMM regime for a single instrument.
func (h *Handler) regimeDetail(ctx context.Context, chatID string, mapping *domain.PriceSymbolMapping) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet.")
		return err
	}

	prices, err := h.dailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 120)
	if err != nil || len(prices) < 60 {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"Insufficient data for <code>%s</code> HMM (need ≥60 daily bars, got %d).",
			mapping.Currency, len(prices),
		))
		return sendErr
	}

	records := dailyToPriceRecords(prices)
	hmm, err := pricesvc.EstimateHMMRegime(records)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("HMM estimation failed: %s", html.EscapeString(err.Error())))
		return sendErr
	}

	htmlOut := h.fmt.FormatHMMRegime(mapping.Currency, hmm)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// stateIndex returns the numeric index for an HMM state label.
func stateIndex(state string) int {
	switch state {
	case pricesvc.HMMRiskOn:
		return 0
	case pricesvc.HMMRiskOff:
		return 1
	case pricesvc.HMMCrisis:
		return 2
	default:
		return 1
	}
}

// ---------------------------------------------------------------------------
// Factor Decomposition
// ---------------------------------------------------------------------------

// cmdFactors handles /factors — factor return decomposition.
func (h *Handler) cmdFactors(ctx context.Context, chatID string, userID int64, args string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Signal repository not available.")
		return err
	}

	_, _ = h.bot.SendHTML(ctx, chatID, "Running factor decomposition...")

	decomposer := backtestsvc.NewFactorDecomposer(h.signalRepo)
	result, err := decomposer.Decompose(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Factor decomposition failed: %s", html.EscapeString(err.Error())))
		return sendErr
	}

	htmlOut := h.fmt.FormatFactorDecomposition(result)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// ---------------------------------------------------------------------------
// Walk-Forward Optimization
// ---------------------------------------------------------------------------

// cmdWFOpt handles /wfopt — walk-forward weight optimization.
func (h *Handler) cmdWFOpt(ctx context.Context, chatID string, userID int64, args string) error {
	if h.signalRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Signal repository not available.")
		return err
	}

	_, _ = h.bot.SendHTML(ctx, chatID, "Running walk-forward optimization (26W train → 4W test)...")

	optimizer := backtestsvc.NewWalkForwardOptimizer(h.signalRepo)
	result, err := optimizer.Optimize(ctx)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("Walk-forward optimization failed: %s", html.EscapeString(err.Error())))
		return sendErr
	}

	htmlOut := h.fmt.FormatWFOptimization(result)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}
