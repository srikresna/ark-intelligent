package telegram

// /macro — FRED Macro Regime Dashboard
// /ecb   — ECB Statistical Data Warehouse Dashboard

import (
	"context"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/internal/service/macro"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
)

// ---------------------------------------------------------------------------
// P3.2 — /macro — FRED Macro Regime Dashboard
// ---------------------------------------------------------------------------

// cmdMacro handles the /macro command — shows plain-language summary with inline navigation.
// Subcommands: /macro detail, /macro explain, /macro matrix|performance, /macro refresh (admin).
func (h *Handler) cmdMacro(ctx context.Context, chatID string, userID int64, args string) error {
	upper := strings.ToUpper(strings.TrimSpace(args))

	// Subcommand routing
	if upper == "MATRIX" || upper == "PERFORMANCE" {
		return h.macroRegimePerformance(ctx, chatID, 0)
	}

	forceRefresh := upper == "REFRESH"
	if forceRefresh {
		if !h.requireAdmin(ctx, chatID, userID) {
			return nil
		}
		fred.InvalidateCache()
	}

	cacheStatus := "🏦 Fetching FRED macro data... ⏳ (5-15s)"
	if !forceRefresh && fred.CacheAge() >= 0 {
		cacheStatus = "🏦 Loading FRED macro data (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, cacheStatus)

	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("FRED data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"Failed to fetch macro data. Please try again later.")
	}

	// Merge sentiment data into MacroData for complete composite scoring
	if sentData, sentErr := sentiment.GetCachedOrFetch(ctx); sentErr == nil && sentData != nil {
		fred.MergeSentiment(data,
			sentData.CNNFearGreed,
			sentData.AAIIBullBear,
			sentData.PutCallTotal,
			sentData.PutCallEquity,
			sentData.PutCallIndex,
		)
	}

	composites := fred.ComputeComposites(data)
	regime := fred.ClassifyMacroRegime(data, composites)

	// Route to specific view
	switch upper {
	case "DETAIL":
		return h.macroSendDetail(ctx, chatID, placeholderID, regime, data)
	case "EXPLAIN":
		htmlMsg := h.fmt.FormatMacroExplain(regime, data)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	case "COMPOSITES":
		htmlMsg := h.fmt.FormatMacroComposites(composites, data)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	case "GLOBAL":
		htmlMsg := h.fmt.FormatMacroGlobal(composites, data)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	case "LABOR":
		htmlMsg := h.fmt.FormatMacroLabor(composites, data)
		kb := h.kb.MacroDrillDownMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	case "INFLATION":
		htmlMsg := h.fmt.FormatMacroInflation(composites, data)
		kb := h.kb.MacroDrillDownMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	}

	// Default: plain-language summary with inline keyboard
	return h.macroSendSummary(ctx, chatID, placeholderID, userID, regime, data)
}

// macroSendSummary sends the plain-language macro summary with inline navigation buttons.
func (h *Handler) macroSendSummary(ctx context.Context, chatID string, msgID int, userID int64, regime fred.MacroRegime, data *fred.MacroData) error {
	implications := fred.DeriveTradingImplications(regime, data)
	htmlMsg := h.fmt.FormatMacroSummary(regime, data, implications)

	isAdmin := false
	if h.middleware != nil {
		role := h.middleware.GetUserRole(ctx, userID)
		isAdmin = domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin)
	} else {
		isAdmin = h.bot.isOwner(userID)
	}

	kb := h.kb.MacroMenu(isAdmin)
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)
}

// macroSendDetail sends the full technical dashboard with back-navigation.
func (h *Handler) macroSendDetail(ctx context.Context, chatID string, msgID int, regime fred.MacroRegime, data *fred.MacroData) error {
	htmlMsg := h.fmt.FormatMacroRegime(regime, data)

	// Append regime-asset performance insight if price data is available.
	if h.priceRepo != nil {
		insight := h.buildRegimeAssetInsight(ctx, data, regime)
		if formatted := h.fmt.FormatRegimeAssetInsight(insight); formatted != "" {
			htmlMsg += formatted
		}
	}

	kb := h.kb.MacroDetailMenu()
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)
}

// cbMacro handles inline keyboard callbacks for the macro dashboard navigation.
func (h *Handler) cbMacro(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "macro:")

	// Get cached FRED data (should already be in cache from initial /macro call)
	macroData, err := fred.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("FRED data fetch failed in macro callback")
		return h.bot.EditMessage(ctx, chatID, msgID, "Failed to load macro data. Ketik /macro untuk coba lagi.")
	}
	composites := fred.ComputeComposites(macroData)
	regime := fred.ClassifyMacroRegime(macroData, composites)

	switch action {
	case "detail":
		return h.macroSendDetail(ctx, chatID, msgID, regime, macroData)

	case "explain":
		htmlMsg := h.fmt.FormatMacroExplain(regime, macroData)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "summary":
		return h.macroSendSummary(ctx, chatID, msgID, userID, regime, macroData)

	case "performance":
		return h.macroRegimePerformance(ctx, chatID, msgID)

	case "composites":
		htmlMsg := h.fmt.FormatMacroComposites(composites, macroData)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "global":
		htmlMsg := h.fmt.FormatMacroGlobal(composites, macroData)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "labor":
		htmlMsg := h.fmt.FormatMacroLabor(composites, macroData)
		kb := h.kb.MacroDrillDownMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "inflation":
		htmlMsg := h.fmt.FormatMacroInflation(composites, macroData)
		kb := h.kb.MacroDrillDownMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "refresh":
		if !h.requireAdmin(ctx, chatID, userID) {
			return nil
		}
		fred.InvalidateCache()
		freshData, err := fred.GetCachedOrFetch(ctx)
		if err != nil {
			return h.bot.EditMessage(ctx, chatID, msgID, "Failed to refresh macro data.")
		}
		freshComposites := fred.ComputeComposites(freshData)
		freshRegime := fred.ClassifyMacroRegime(freshData, freshComposites)
		return h.macroSendSummary(ctx, chatID, msgID, userID, freshRegime, freshData)
	}

	return nil
}

// buildRegimeAssetInsight computes the regime-asset matrix from stored price
// history and returns insight for the current regime.
func (h *Handler) buildRegimeAssetInsight(ctx context.Context, data *fred.MacroData, regime fred.MacroRegime) fred.RegimeInsight {
	const lookbackWeeks = 52

	// Build regime history from current FRED data.
	regimeHistory := fred.BuildRegimeHistoryFromCurrent(data, lookbackWeeks)
	if len(regimeHistory) == 0 {
		return fred.RegimeInsight{Regime: regime.Name}
	}

	// Fetch price history for all COT-tracked contracts.
	priceHistory := make(map[string][]domain.PriceRecord)
	for _, m := range domain.COTPriceSymbolMappings() {
		records, err := h.priceRepo.GetHistory(ctx, m.ContractCode, lookbackWeeks)
		if err != nil || len(records) == 0 {
			continue
		}
		priceHistory[m.ContractCode] = records
	}

	if len(priceHistory) == 0 {
		return fred.RegimeInsight{Regime: regime.Name}
	}

	matrix := fred.ComputeRegimeAssetMatrix(regimeHistory, priceHistory)
	return fred.GetCurrentRegimeInsight(regime.Name, matrix)
}

// macroRegimePerformance builds and sends the regime-asset performance matrix
// from historical persisted signals with FRED regime labels.
// If msgID > 0, edits the existing message; otherwise sends a new message.
func (h *Handler) macroRegimePerformance(ctx context.Context, chatID string, msgID int) error {
	if h.signalRepo == nil {
		msg := "Regime performance requires signal history with FRED regime data."
		if msgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID, msg, h.kb.MacroDetailMenu())
		}
		_, err := h.bot.SendHTML(ctx, chatID, msg)
		return err
	}

	builder := fred.NewRegimePerformanceBuilder(h.signalRepo)
	matrix, err := builder.Build(ctx)
	if err != nil {
		errMsg := userFriendlyError(err, "macro")
		if msgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID, errMsg, h.kb.MacroDetailMenu())
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	htmlOut := h.fmt.FormatRegimePerformance(matrix)
	kb := h.kb.MacroDetailMenu()
	if msgID > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlOut, kb)
	}
	// Fallback: send as new message with keyboard
	_, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, kb)
	return err
}

// currentMacroRegimeName returns the current FRED macro regime name from cache.
// Returns "" if FRED data is unavailable (never blocks on a network fetch).
func (h *Handler) currentMacroRegimeName(ctx context.Context) string {
	// Only use cached data — don't trigger a FRED fetch just for sentiment context
	if fred.CacheAge() < 0 {
		return ""
	}
	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil || data == nil {
		return ""
	}
	composites := fred.ComputeComposites(data)
	regime := fred.ClassifyMacroRegime(data, composites)
	return regime.Name
}

func (h *Handler) renderMacroSummary(ctx context.Context, chatID string, userID int64, editMsgID int) error {
	prefs, _ := h.prefsRepo.Get(ctx, userID)

	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil || data == nil {
		return nil
	}
	composites := fred.ComputeComposites(data)
	regime := fred.ClassifyMacroRegime(data, composites)

	var htmlOut string
	var toggleBtn ports.InlineButton
	if prefs.OutputMode == domain.OutputFull {
		implications := fred.DeriveTradingImplications(regime, data)
		htmlOut = h.fmt.FormatMacroSummary(regime, data, implications)
		toggleBtn = ports.InlineButton{Text: btnCompact, CallbackData: "view:compact:macro"}
	} else {
		htmlOut = h.fmt.FormatMacroSummaryCompact(regime, data)
		toggleBtn = ports.InlineButton{Text: btnExpand, CallbackData: "view:full:macro"}
	}

	kb := h.kb.MacroMenu(false)
	toggleRow := []ports.InlineButton{toggleBtn}
	kb.Rows = append([][]ports.InlineButton{toggleRow}, kb.Rows...)

	if editMsgID > 0 {
		return h.bot.EditWithKeyboardChunked(ctx, chatID, editMsgID, htmlOut, kb)
	}
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, htmlOut, kb)
	return err
}

// ---------------------------------------------------------------------------
// /ecb — ECB Statistical Data Warehouse Dashboard
// ---------------------------------------------------------------------------

// cmdECB handles the /ecb command — fetches and displays ECB monetary policy data
// (key rate, M3 money supply, EUR/USD official rate) from the ECB SDW API.
func (h *Handler) cmdECB(ctx context.Context, chatID string, _ int64, _ string) error {
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, "🏦 Fetching ECB monetary policy data... ⏳")

	data, err := macro.GetECBData(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ECB data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"❌ Gagal mengambil data ECB. Silakan coba lagi nanti.")
	}

	htmlMsg := macro.FormatECBData(data)
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}

// ---------------------------------------------------------------------------
// /snb — SNB Balance Sheet / FX Intervention Proxy Dashboard
// ---------------------------------------------------------------------------

// cmdSNB handles the /snb command — fetches and displays SNB balance sheet data,
// focusing on foreign currency investments as a CHF intervention proxy.
func (h *Handler) cmdSNB(ctx context.Context, chatID string, _ int64, _ string) error {
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, "🏦 Fetching SNB balance sheet data... ⏳")

	data, err := macro.GetSNBData(ctx)
	if err != nil {
		log.Error().Err(err).Msg("SNB data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"❌ Gagal mengambil data SNB. Silakan coba lagi nanti.")
	}

	htmlMsg := macro.FormatSNBData(data)
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}

// ---------------------------------------------------------------------------
// /leading — OECD Composite Leading Indicators Dashboard
// ---------------------------------------------------------------------------

// cmdLeading handles the /leading command — fetches and displays OECD CLI data
// showing economic growth momentum across G7+ countries with FX divergence signals.
func (h *Handler) cmdLeading(ctx context.Context, chatID string, _ int64, _ string) error {
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, "📊 Fetching OECD leading indicators... ⏳")

	data, err := macro.GetOECDCLIData(ctx)
	if err != nil {
		log.Error().Err(err).Msg("OECD CLI data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"❌ Gagal mengambil data OECD CLI. Silakan coba lagi nanti.")
	}

	htmlMsg := macro.FormatOECDCLIData(data)
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}

// ---------------------------------------------------------------------------
// /swaps — DTCC FX Swap Institutional Flows
// ---------------------------------------------------------------------------

// cmdSwaps handles the /swaps command — fetches and displays DTCC PPD FX swap
// volume data showing institutional hedging flows and positioning per currency pair.
func (h *Handler) cmdSwaps(ctx context.Context, chatID string, _ int64, _ string) error {
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, "🏛 Fetching DTCC FX swap institutional data... ⏳")

	data, err := macro.GetDTCCData(ctx)
	if err != nil {
		log.Error().Err(err).Msg("DTCC FX swap data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"❌ Gagal mengambil data DTCC FX swap. Silakan coba lagi nanti.")
	}

	htmlMsg := macro.FormatDTCCData(data)
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
}
