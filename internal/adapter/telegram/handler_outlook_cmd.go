package telegram

// /outlook — AI Weekly Market Outlook

import (
	"context"
	"fmt"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	"github.com/arkcode369/ark-intelligent/internal/service/bis"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/internal/service/imf"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
	"github.com/arkcode369/ark-intelligent/internal/service/worldbank"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// /outlook — AI weekly market outlook
// ---------------------------------------------------------------------------

func (h *Handler) cmdOutlook(ctx context.Context, chatID string, userID int64, args string) error {
	if h.aiAnalyzer == nil || !h.aiAnalyzer.IsAvailable() {
		_, err := h.bot.SendHTML(ctx, chatID, "AI outlook is unavailable. Gemini API key not configured.")
		return err
	}

	// Per-user AI quota check via middleware
	if h.middleware != nil {
		allowed, reason := h.middleware.CheckAIQuota(ctx, userID)
		if !allowed {
			_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("⛔ %s", reason))
			return err
		}

		// Tiered cooldown check (Owner=0s, Admin=10s, Member/Free=30s)
		cooldown := h.middleware.GetAICooldown(ctx, userID)
		if cooldown > 0 && !h.checkAICooldownDynamic(userID, cooldown) {
			_, err := h.bot.SendHTML(ctx, chatID, "Please wait before requesting another AI analysis.")
			return err
		}
	} else {
		// Legacy fallback
		if !h.bot.isOwner(userID) && !h.checkAICooldown(userID) {
			_, err := h.bot.SendHTML(ctx, chatID, "Please wait before requesting another AI analysis.")
			return err
		}
	}

	return h.generateOutlook(ctx, chatID, userID, 0)
}

func (h *Handler) cbOutlook(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// AI quota + cooldown check for callback-triggered outlook (same as /outlook command)
	if h.aiAnalyzer == nil || !h.aiAnalyzer.IsAvailable() {
		return h.bot.EditMessage(ctx, chatID, msgID, "AI outlook is unavailable.")
	}
	if h.middleware != nil {
		allowed, reason := h.middleware.CheckAIQuota(ctx, userID)
		if !allowed {
			return h.bot.EditMessage(ctx, chatID, msgID, fmt.Sprintf("\xe2\x9b\x94 %s", reason))
		}
		cooldown := h.middleware.GetAICooldown(ctx, userID)
		if cooldown > 0 && !h.checkAICooldownDynamic(userID, cooldown) {
			return h.bot.EditMessage(ctx, chatID, msgID, "Please wait before requesting another AI analysis.")
		}
	} else {
		// Legacy fallback
		if !h.bot.isOwner(userID) && !h.checkAICooldown(userID) {
			return h.bot.EditMessage(ctx, chatID, msgID, "Please wait before requesting another AI analysis.")
		}
	}
	return h.generateOutlook(ctx, chatID, userID, msgID)
}

func (h *Handler) generateOutlook(ctx context.Context, chatID string, userID int64, editMsgID int) error {
	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		prefs = domain.DefaultPrefs()
	}

	placeholderID := 0
	if editMsgID > 0 {
		_ = h.bot.EditMessage(ctx, chatID, editMsgID, "Generating unified intelligence report... ⏳\n(collecting all data sources + web search)")
		placeholderID = editMsgID
	} else {
		placeholderID, _ = h.bot.SendLoading(ctx, chatID, "Generating unified intelligence report... ⏳\n(collecting all data sources + web search)")
	}

	now := timeutil.NowWIB()

	// ---------- Collect ALL data sources (best-effort, non-fatal) ----------

	// COT
	cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)

	// News
	weekEvts, _ := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))

	// FRED Macro
	macroData, _ := fred.GetCachedOrFetch(ctx)
	var macroRegime *fred.MacroRegime
	if macroData != nil {
		comp := fred.ComputeComposites(macroData)
		r := fred.ClassifyMacroRegime(macroData, comp)
		macroRegime = &r
	}

	// Price contexts
	var priceCtxs map[string]*domain.PriceContext
	if h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pc, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil && len(pc) > 0 {
			priceCtxs = pc
		}
	}

	// VIX/SPX risk context
	var riskCtx *domain.RiskContext
	if h.priceRepo != nil {
		riskBuilder := pricesvc.NewRiskContextBuilder(h.priceRepo)
		riskCtx, _ = riskBuilder.Build(ctx)
		if riskCtx != nil && macroData != nil {
			pricesvc.EnrichWithTermStructure(riskCtx, macroData.VIX3M)
		}
	}

	// Sentiment (CNN Fear & Greed)
	sentimentData, _ := sentiment.GetCachedOrFetch(ctx)

	// Seasonal patterns
	var seasonalData map[string]*pricesvc.SeasonalPattern
	if h.priceRepo != nil {
		sa := pricesvc.NewSeasonalAnalyzer(h.priceRepo)
		if patterns, saErr := sa.Analyze(ctx); saErr == nil && len(patterns) > 0 {
			seasonalData = make(map[string]*pricesvc.SeasonalPattern, len(patterns))
			for i := range patterns {
				seasonalData[patterns[i].ContractCode] = &patterns[i]
			}
		}
	}

	// Currency strength
	var currencyStrength []pricesvc.CurrencyStrength
	if len(priceCtxs) > 0 && len(cotAnalyses) > 0 {
		currencyStrength = pricesvc.ComputeCurrencyStrengthIndex(priceCtxs, cotAnalyses)
	}

	// Backtest stats
	var backtestStats *domain.BacktestStats
	if h.signalRepo != nil {
		sc := backtestsvc.NewStatsCalculator(h.signalRepo)
		if stats, bErr := sc.ComputeAll(ctx); bErr == nil {
			backtestStats = stats
		}
	}

	// World Bank cross-country macro fundamentals (graceful degradation on error)
	wbData, _ := worldbank.GetCachedOrFetch(ctx)

	// BIS REER/NEER currency valuation (graceful degradation on error)
	bisData, _ := bis.GetCachedOrFetch(ctx)

	// IMF WEO forward-looking forecasts (graceful degradation on error)
	imfData, _ := imf.GetCachedOrFetch(ctx)

	// Daily price contexts (for daily technical analysis in outlook)
	var dailyPriceCtxs map[string]*domain.DailyPriceContext
	if h.dailyPriceRepo != nil {
		dailyBuilder := pricesvc.NewDailyContextBuilder(h.dailyPriceRepo)
		if dpc, dpcErr := dailyBuilder.BuildAll(ctx); dpcErr == nil && len(dpc) > 0 {
			dailyPriceCtxs = dpc
		}
	}

	// ---------- Build unified data ----------
	var macroComposites *domain.MacroComposites
	if macroData != nil {
		// Merge sentiment data into MacroData before computing composites,
		// so SentimentComposite includes CNN F&G, AAII, and CBOE P/C.
		if sentimentData != nil {
			fred.MergeSentiment(macroData,
				sentimentData.CNNFearGreed,
				sentimentData.AAIIBullBear,
				sentimentData.PutCallTotal,
				sentimentData.PutCallEquity,
				sentimentData.PutCallIndex,
			)
		}
		macroComposites = fred.ComputeComposites(macroData)
	}

	unifiedData := aisvc.UnifiedOutlookData{
		COTAnalyses:        cotAnalyses,
		NewsEvents:         weekEvts,
		MacroData:          macroData,
		MacroRegime:        macroRegime,
		MacroComposites:    macroComposites,
		PriceContexts:      priceCtxs,
		DailyPriceContexts: dailyPriceCtxs,
		RiskContext:        riskCtx,
		SentimentData:      sentimentData,
		SeasonalData:       seasonalData,
		BacktestStats:      backtestStats,
		CurrencyStrength:   currencyStrength,
		WorldBankData:      wbData,
		BISData:            bisData,
		IMFData:            imfData,
		Language:           prefs.Language,
	}

	// ---------- Route based on user's PreferredModel setting ----------
	var result string
	useClaude := prefs.PreferredModel != "gemini" && h.claudeAnalyzer != nil && h.claudeAnalyzer.IsAvailable()

	if useClaude {
		// Claude path: multi-phase unified outlook with thinking + web_search
		modelOverride := ""
		if prefs.ClaudeModel != "" && domain.IsValidClaudeModel(prefs.ClaudeModel) {
			modelOverride = string(prefs.ClaudeModel)
		}
		analyzer := h.claudeAnalyzer.WithModel(modelOverride)
		log.Info().
			Str("model", modelOverride).
			Int64("user_id", userID).
			Msg("/outlook unified routed to Claude (multi-phase)")
		result, err = analyzer.GenerateUnifiedOutlook(ctx, unifiedData)

		// If Claude fails (e.g. Vercel timeout on all phases), fall back to Gemini
		if err != nil || result == "" {
			log.Warn().Err(err).Msg("/outlook Claude failed, falling back to Gemini")
			weeklyData := ports.WeeklyData{
				COTAnalyses:   cotAnalyses,
				NewsEvents:    weekEvts,
				MacroData:     macroData,
				BacktestStats: backtestStats,
				PriceContexts: priceCtxs,
				Language:      prefs.Language,
			}
			result, err = h.aiAnalyzer.AnalyzeCombinedOutlook(ctx, weeklyData)
		}
	} else {
		// Gemini path: direct combined outlook (no web search capability)
		log.Info().Int64("user_id", userID).Msg("/outlook routed to Gemini")
		weeklyData := ports.WeeklyData{
			COTAnalyses:   cotAnalyses,
			NewsEvents:    weekEvts,
			MacroData:     macroData,
			BacktestStats: backtestStats,
			PriceContexts: priceCtxs,
			Language:      prefs.Language,
		}
		result, err = h.aiAnalyzer.AnalyzeCombinedOutlook(ctx, weeklyData)
	}

	if err != nil {
		log.Error().Err(err).Msg("AI generation failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID, "AI generation failed. Please try again later.")
	}

	html := h.fmt.FormatWeeklyOutlook(result, now)
	if editMsgID > 0 {
		return h.bot.EditMessage(ctx, chatID, editMsgID, html)
	}
	_ = h.bot.DeleteMessage(ctx, chatID, placeholderID)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}
