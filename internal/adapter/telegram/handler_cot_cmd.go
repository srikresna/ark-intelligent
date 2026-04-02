package telegram

// /cot, /bias, /rank, /history — COT Positioning Domain

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// /cot — COT positioning analysis
// ---------------------------------------------------------------------------

func (h *Handler) cmdCOT(ctx context.Context, chatID string, userID int64, args string) error {
	// If specific currency requested: /cot USD or /cot raw USD
	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) > 0 {
		isRaw := false
		code := parts[0]

		if parts[0] == "RAW" {
			isRaw = true
			if len(parts) > 1 {
				code = parts[1]
			} else {
				code = ""
			}
		} else if parts[0] == "ANALYSIS" {
			if len(parts) > 1 {
				code = parts[1]
			} else {
				code = ""
			}
		} else if len(parts) > 1 && parts[1] == "RAW" {
			isRaw = true
		}

		if code != "" {
			h.saveLastCurrency(ctx, userID, code)
			contractCode := currencyToContractCode(code)
			return h.sendCOTDetail(ctx, chatID, contractCode, code, isRaw, 0)
		}
	}

	// Overview: all currencies
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil {
		return fmt.Errorf("get all COT analyses: %w", err)
	}

	if len(analyses) == 0 {
		_, err = h.bot.SendHTML(ctx, chatID,
			"No COT data available yet. Data is fetched from CFTC every Friday.")
		return err
	}

	// Build conviction scores for overview (best-effort, non-fatal)
	var overviewConvictions []cot.ConvictionScore
	macroDataOv, fredErrOv := fred.GetCachedOrFetch(ctx)
	if fredErrOv == nil && macroDataOv != nil {
		compositeOv := fred.ComputeComposites(macroDataOv)
		regimeOv := fred.ClassifyMacroRegime(macroDataOv, compositeOv)
		var priceCtxsOv map[string]*domain.PriceContext
		if h.priceRepo != nil {
			ctxBuilderOv := pricesvc.NewContextBuilder(h.priceRepo)
			if pcs, pcErr := ctxBuilderOv.BuildAll(ctx); pcErr == nil {
				priceCtxsOv = pcs
			}
		}
		for _, a := range analyses {
			surpriseSigma := 0.0
			if h.newsScheduler != nil {
				surpriseSigma = h.newsScheduler.GetSurpriseSigma(a.Contract.Currency)
			}
			var pc *domain.PriceContext
			if priceCtxsOv != nil {
				pc = priceCtxsOv[a.Contract.Code]
			}
			cs := cot.ComputeConvictionScoreV3(a, regimeOv, surpriseSigma, "", macroDataOv, pc)
			overviewConvictions = append(overviewConvictions, cs)
		}
	}

	html := h.fmt.FormatCOTOverview(analyses, overviewConvictions)
	kb := h.kb.COTCurrencySelector(analyses)
	_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

func (h *Handler) sendCOTDetail(ctx context.Context, chatID string, contractCode, displayCode string, isRaw bool, editMsgID int) error {
	if isRaw {
		// Try GetHistory first (up to 4 weeks back to avoid missing data
		// when CFTC report is older than 7 days due to delayed release).
		records, err := h.cotRepo.GetHistory(ctx, contractCode, 4)
		if err != nil || len(records) == 0 {
			// Fallback: try GetLatest directly (reverse-scans the whole prefix)
			latest, latestErr := h.cotRepo.GetLatest(ctx, contractCode)
			if latestErr != nil || latest == nil {
				msg := fmt.Sprintf("No COT data for %s", displayCode)
				if editMsgID > 0 {
					return h.bot.EditMessage(ctx, chatID, editMsgID, msg)
				}
				_, e := h.bot.SendHTML(ctx, chatID, msg)
				return e
			}
			records = []domain.COTRecord{*latest}
		}

		html := h.fmt.FormatCOTRaw(records[0])
		kb := h.kb.COTDetailMenu(contractCode, true)
		if editMsgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, editMsgID, html, kb)
		}
		_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return err
	}

	analysis, err := h.cotRepo.GetLatestAnalysis(ctx, contractCode)
	if err != nil || analysis == nil {
		msg := fmt.Sprintf("No COT data for %s", displayCode)
		if editMsgID > 0 {
			return h.bot.EditMessage(ctx, chatID, editMsgID, msg)
		}
		_, e := h.bot.SendHTML(ctx, chatID, msg)
		return e
	}

	html := h.fmt.FormatCOTDetailWithCode(*analysis, displayCode)

	// Build price context for this contract (best-effort, non-fatal)
	var priceCtxMap map[string]*domain.PriceContext
	if h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pc, pcErr := ctxBuilder.Build(ctx, contractCode, displayCode); pcErr == nil && pc != nil {
			priceCtxMap = map[string]*domain.PriceContext{contractCode: pc}
			html += h.fmt.FormatPriceContext(pc)

			// Always show price-COT relationship — divergence warning OR alignment confirmation
			divs := pricesvc.DetectPriceCOTDivergences(priceCtxMap, []domain.COTAnalysis{*analysis})
			if len(divs) > 0 {
				html += h.fmt.FormatPriceCOTDivergence(divs[0])
			} else {
				html += h.fmt.FormatPriceCOTAlignment(pc, *analysis)
			}
		} else if pcErr != nil {
			// Notify owner about price context failure (non-blocking)
			h.notifyOwnerDebug(ctx, fmt.Sprintf("⚠️ Price context failed for <b>%s</b>\n<code>%s</code>", displayCode, pcErr.Error()))
		}
	}

	// Add AI interpretation with price context if available
	if editMsgID == 0 && h.aiAnalyzer != nil && h.aiAnalyzer.IsAvailable() {
		narrative, aiErr := h.aiAnalyzer.AnalyzeCOTWithPrice(ctx, []domain.COTAnalysis{*analysis}, priceCtxMap)
		if aiErr == nil && narrative != "" {
			html += "\n\n" + h.fmt.FormatAIInsight("COT Analysis", narrative)
		}
	}

	// Inject FRED macro context (non-fatal if fails — uses cache if available)
	if editMsgID == 0 {
		macroData, fredErr := fred.GetCachedOrFetch(ctx)
		if fredErr == nil && macroData != nil {
			composites := fred.ComputeComposites(macroData)
			regime := fred.ClassifyMacroRegime(macroData, composites)
			fredCtx := h.fmt.FormatFREDContext(macroData, regime)
			if fredCtx != "" {
				html += fredCtx
			}
		}
	}

	// Conviction Score — always shown, uses whatever data is available (FRED optional)
	if editMsgID == 0 && analysis != nil {
		surpriseSigma2 := 0.0
		if h.newsScheduler != nil {
			surpriseSigma2 = h.newsScheduler.GetSurpriseSigma(analysis.Contract.Currency)
		}
		var pc2 *domain.PriceContext
		if h.priceRepo != nil {
			ctxBuilder2 := pricesvc.NewContextBuilder(h.priceRepo)
			if pcs2, pcErr2 := ctxBuilder2.BuildAll(ctx); pcErr2 == nil {
				pc2 = pcs2[contractCode]
			}
		}
		macroData2, fredErr2 := fred.GetCachedOrFetch(ctx)
		if fredErr2 == nil && macroData2 != nil {
			composites2 := fred.ComputeComposites(macroData2)
			regime2 := fred.ClassifyMacroRegime(macroData2, composites2)
			cs := cot.ComputeConvictionScoreV3(*analysis, regime2, surpriseSigma2, "", macroData2, pc2)
			html += h.fmt.FormatConvictionBlock(cs)
		} else {
			// FRED unavailable — compute conviction with COT + price only (regime = zero value)
			cs := cot.ComputeConvictionScoreV3(*analysis, fred.MacroRegime{}, surpriseSigma2, "", nil, pc2)
			html += h.fmt.FormatConvictionBlock(cs)
		}
	}

	// Signal detection for this currency
	if editMsgID == 0 && analysis != nil {
		records, histErr := h.cotRepo.GetHistory(ctx, contractCode, 8)
		if histErr == nil && len(records) > 0 {
			histMap := map[string][]domain.COTRecord{contractCode: records}
			recalDet := cot.NewRecalibratedDetector(h.signalRepo)
			if h.signalRepo != nil {
				_ = recalDet.LoadTypeStats(ctx)
			}
			var rCtx *domain.RiskContext
			if h.priceRepo != nil {
				rb := pricesvc.NewRiskContextBuilder(h.priceRepo)
				rCtx, _ = rb.Build(ctx)
			}
			signals := recalDet.DetectAll([]domain.COTAnalysis{*analysis}, histMap, rCtx, priceCtxMap)
			if len(signals) > 0 {
				html += h.fmt.FormatBiasSummary(signals)
			}
		}
	}

	// P1.4 — Upcoming Catalysts: fetch events for next 48h for this currency
	if editMsgID == 0 && h.newsRepo != nil {
		now := timeutil.NowWIB()
		today := now.Format("20060102")
		tomorrow := now.AddDate(0, 0, 1).Format("20060102")

		todayEvts, _ := h.newsRepo.GetByDate(ctx, today)
		tomorrowEvts, _ := h.newsRepo.GetByDate(ctx, tomorrow)

		upcoming := append(todayEvts, tomorrowEvts...) //nolint:gocritic
		currency := analysis.Contract.Currency
		catalysts := h.fmt.FormatUpcomingCatalysts(currency, upcoming)
		if catalysts != "" {
			html += catalysts
		}
	}

	kb := h.kb.COTDetailMenu(contractCode, false)
	kb = AppendFeedbackRow(kb, h.kb, "fb:cot:"+contractCode, h.feedbackEnabled())
	if editMsgID > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, editMsgID, html, kb)
	}
	_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

// cbCOTDetail handles inline keyboard callback for COT detail view.
func (h *Handler) cbCOTDetail(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// data format: "cot:analysis:099741", "cot:raw:099741", "cot:overview"
	action := strings.TrimPrefix(data, "cot:")

	if action == "overview" {
		analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
		if err != nil || len(analyses) == 0 {
			return h.bot.EditMessage(ctx, chatID, msgID, "No COT data available.")
		}
		// Build conviction scores for overview (best-effort, non-fatal)
		var cbConvictions []cot.ConvictionScore
		cbMacro, cbFredErr := fred.GetCachedOrFetch(ctx)
		if cbFredErr == nil && cbMacro != nil {
			cbComposites := fred.ComputeComposites(cbMacro)
			cbRegime := fred.ClassifyMacroRegime(cbMacro, cbComposites)
			var cbPriceCtxs map[string]*domain.PriceContext
			if h.priceRepo != nil {
				cbBuilder := pricesvc.NewContextBuilder(h.priceRepo)
				if pcs, pcErr := cbBuilder.BuildAll(ctx); pcErr == nil {
					cbPriceCtxs = pcs
				}
			}
			for _, a := range analyses {
				surpriseSigma := 0.0
				if h.newsScheduler != nil {
					surpriseSigma = h.newsScheduler.GetSurpriseSigma(a.Contract.Currency)
				}
				var pc *domain.PriceContext
				if cbPriceCtxs != nil {
					pc = cbPriceCtxs[a.Contract.Code]
				}
				cs := cot.ComputeConvictionScoreV3(a, cbRegime, surpriseSigma, "", cbMacro, pc)
				cbConvictions = append(cbConvictions, cs)
			}
		}
		html := h.fmt.FormatCOTOverview(analyses, cbConvictions)
		kb := h.kb.COTCurrencySelector(analyses)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	}

	parts := strings.Split(action, ":")
	if len(parts) != 2 {
		return nil
	}

	isRaw := parts[0] == "raw"
	contractCode := parts[1]

	return h.sendCOTDetail(ctx, chatID, contractCode, contractCode, isRaw, msgID)
}

// ---------------------------------------------------------------------------
// /bias — COT Directional Bias
// ---------------------------------------------------------------------------

func (h *Handler) cmdBias(ctx context.Context, chatID string, userID int64, args string) error {
	loadingID, _ := h.bot.SendHTML(ctx, chatID, "🎯 Mendeteksi directional bias... ⏳")
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err = h.bot.SendHTML(ctx, chatID, "No COT data available for bias detection.")
		return err
	}

	// Build history map (8 weeks needed for momentum/divergence detection)
	historyMap := make(map[string][]domain.COTRecord, len(analyses))
	for _, a := range analyses {
		records, hErr := h.cotRepo.GetHistory(ctx, a.Contract.Code, 8)
		if hErr == nil && len(records) > 0 {
			historyMap[a.Contract.Code] = records
		}
	}

	// Use recalibrated detector with historical win rates + VIX filter
	recalDetector := cot.NewRecalibratedDetector(h.signalRepo)
	if h.signalRepo != nil {
		_ = recalDetector.LoadTypeStats(ctx)
	}
	var riskCtx *domain.RiskContext
	var priceCtxsBias map[string]*domain.PriceContext
	if h.priceRepo != nil {
		rb := pricesvc.NewRiskContextBuilder(h.priceRepo)
		riskCtx, _ = rb.Build(ctx)
		// Build price contexts for ATR volatility multiplier
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pcs, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil {
			priceCtxsBias = pcs
		}
	}
	signals := recalDetector.DetectAll(analyses, historyMap, riskCtx, priceCtxsBias)

	// Filter by currency if specified
	filterCurrency := strings.ToUpper(strings.TrimSpace(args))
	if filterCurrency != "" {
		var filtered []cot.Signal
		for _, s := range signals {
			if s.Currency == filterCurrency {
				filtered = append(filtered, s)
			}
		}
		signals = filtered
	}

	html := h.fmt.FormatBiasHTML(signals, filterCurrency)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	kb := h.kb.RelatedCommandsKeyboard("bias", filterCurrency)
	if len(kb.Rows) > 0 {
		_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	} else {
		_, err = h.bot.SendHTML(ctx, chatID, html)
	}
	return err
}

// cmdHistory shows COT positioning history for a currency.
// Usage: /history EUR [4|8|12] — weeks of history (default 4)
func (h *Handler) cmdHistory(ctx context.Context, chatID string, userID int64, args string) error {
	h.bot.SendTyping(ctx, chatID)

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) == 0 {
		// Try last currency
		lc := h.getLastCurrency(ctx, userID)
		if lc != "" {
			parts = []string{lc}
		} else {
			_, err := h.bot.SendHTML(ctx, chatID,
				"📊 <b>COT History</b>\n\nUsage: <code>/history EUR</code> atau <code>/h GBP 8</code>\n\nTampilkan positioning history 4-12 minggu terakhir.")
			return err
		}
	}

	currency := parts[0]
	weeks := 4
	if len(parts) > 1 {
		if w, err := strconv.Atoi(parts[1]); err == nil && w >= 2 && w <= 52 {
			weeks = w
		}
	}

	h.saveLastCurrency(ctx, userID, currency)
	contractCode := currencyToContractCode(currency)

	records, err := h.cotRepo.GetHistory(ctx, contractCode, weeks)
	if err != nil || len(records) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("no history for %s", currency), "history")
		return nil
	}

	// Build history view
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📊 <b>COT History — %s (%d weeks)</b>\n", currency, len(records)))
	b.WriteString(fmt.Sprintf("<i>%s → %s</i>\n\n", records[len(records)-1].ReportDate.Format("02 Jan"), records[0].ReportDate.Format("02 Jan 2006")))

	// Sparkline of net position
	netPositions := make([]float64, len(records))
	for i, r := range records {
		netPositions[i] = r.GetSmartMoneyNet("TFF")
	}
	// Reverse for sparkline (oldest first)
	for i, j := 0, len(netPositions)-1; i < j; i, j = i+1, j-1 {
		netPositions[i], netPositions[j] = netPositions[j], netPositions[i]
	}
	b.WriteString("📈 Net Position Trend: <code>")
	b.WriteString(sparkLine(netPositions))
	b.WriteString("</code>\n\n")

	// Table
	b.WriteString("<pre>")
	b.WriteString("Date       | Net Pos   | Chg      | L/S\n")
	b.WriteString("───────────┼───────────┼──────────┼────\n")
	for i, r := range records {
		net := int64(r.GetSmartMoneyNet("TFF"))
		var chg int64
		if i+1 < len(records) {
			prevNet := int64(records[i+1].GetSmartMoneyNet("TFF"))
			chg = net - prevNet
		}
		ratio := 0.0
		if r.LevFundShort > 0 {
			ratio = r.LevFundLong / r.LevFundShort
		}
		b.WriteString(fmt.Sprintf("%-10s | %+9d | %+8d | %.2f\n",
			r.ReportDate.Format("02 Jan"), net, chg, ratio))
	}
	b.WriteString("</pre>")

	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// sparkLine generates a Unicode sparkline from a slice of values.
func sparkLine(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	if span == 0 {
		return strings.Repeat("▄", len(values))
	}
	var result []rune
	for _, v := range values {
		idx := int((v - min) / span * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		result = append(result, blocks[idx])
	}
	return string(result)
}

// saveLastCurrency persists the user's last viewed currency for context carry-over.
func (h *Handler) saveLastCurrency(ctx context.Context, userID int64, currency string) {
	if currency == "" {
		return
	}
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	prefs.LastCurrency = strings.ToUpper(currency)
	_ = h.prefsRepo.Set(ctx, userID, prefs)
}

// getLastCurrency returns the user's last viewed currency, or empty string.
func (h *Handler) getLastCurrency(ctx context.Context, userID int64) string {
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	return prefs.LastCurrency
}

// resolveOrLastCurrency returns the given currency if non-empty, otherwise the user's last currency.
func (h *Handler) resolveOrLastCurrency(ctx context.Context, userID int64, currency string) string {
	if currency != "" {
		return currency
	}
	return h.getLastCurrency(ctx, userID)
}

// cbViewToggle handles compact/full view toggle callbacks.
// Callback data format: "view:<action>:<command>"
func (h *Handler) cbViewToggle(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	parts := strings.SplitN(strings.TrimPrefix(data, "view:"), ":", 2)
	if len(parts) < 2 {
		return nil
	}
	action, command := parts[0], parts[1]

	prefs, _ := h.prefsRepo.Get(ctx, userID)

	switch action {
	case "full":
		prefs.OutputMode = domain.OutputFull
	case "compact":
		prefs.OutputMode = domain.OutputCompact
	default:
		return nil
	}

	_ = h.prefsRepo.Set(ctx, userID, prefs)

	switch command {
	case "cot":
		return h.renderCOTOverview(ctx, chatID, userID, msgID)
	case "macro":
		return h.renderMacroSummary(ctx, chatID, userID, msgID)
	default:
		return nil
	}
}

// renderCOTOverview renders COT overview in compact or full mode based on prefs.
func (h *Handler) renderCOTOverview(ctx context.Context, chatID string, userID int64, editMsgID int) error {
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		return nil
	}

	// Build convictions (best-effort)
	var convictions []cot.ConvictionScore
	macroData, fredErr := fred.GetCachedOrFetch(ctx)
	if fredErr == nil && macroData != nil {
		composites := fred.ComputeComposites(macroData)
		regime := fred.ClassifyMacroRegime(macroData, composites)
		for _, a := range analyses {
			cs := cot.ComputeConvictionScoreV3(a, regime, 0, "", macroData, nil)
			convictions = append(convictions, cs)
		}
	}

	var htmlOut string
	var toggleBtn ports.InlineButton
	if prefs.MobileMode {
		// Mobile mode: sparkline + one-liner per currency (no wide tables)
		htmlOut = h.fmt.FormatCOTOverviewSparkline(analyses, convictions)
		toggleBtn = ports.InlineButton{Text: btnExpand, CallbackData: "view:full:cot"}
	} else if prefs.OutputMode == domain.OutputFull {
		htmlOut = h.fmt.FormatCOTOverview(analyses, convictions)
		toggleBtn = ports.InlineButton{Text: btnCompact, CallbackData: "view:compact:cot"}
	} else {
		htmlOut = h.fmt.FormatCOTOverviewCompact(analyses, convictions)
		toggleBtn = ports.InlineButton{Text: btnExpand, CallbackData: "view:full:cot"}
	}

	kb := h.kb.COTCurrencySelector(analyses)
	toggleRow := []ports.InlineButton{toggleBtn}
	kb.Rows = append([][]ports.InlineButton{toggleRow}, kb.Rows...)

	if editMsgID > 0 {
		return h.bot.EditWithKeyboardChunked(ctx, chatID, editMsgID, htmlOut, kb)
	}
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, htmlOut, kb)
	return err
}

// ---------------------------------------------------------------------------
// P1.3 — /rank — Currency Strength Ranking
// ---------------------------------------------------------------------------

// cmdRank handles the /rank command — weekly currency strength ranking.
// Ranks 8 major currencies by COT SentimentScore and shows conviction scores (COT + FRED + Calendar).
func (h *Handler) cmdRank(ctx context.Context, chatID string, userID int64, args string) error {
	loadingID, _ := h.bot.SendHTML(ctx, chatID, "📈 Menghitung currency strength ranking... ⏳")
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err = h.bot.SendHTML(ctx, chatID,
			"No COT data available for ranking. Data is fetched from CFTC every Friday.")
		return err
	}

	// Fetch FRED regime for conviction scoring (best-effort, non-fatal)
	var macroData *fred.MacroData
	var regime *fred.MacroRegime
	if md, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && md != nil {
		macroData = md
		comp := fred.ComputeComposites(md)
		r := fred.ClassifyMacroRegime(md, comp)
		regime = &r
	}

	// Build price contexts for V3 conviction scoring + strength ranking (best-effort)
	var priceCtxs map[string]*domain.PriceContext
	if h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pcs, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil && len(pcs) > 0 {
			priceCtxs = pcs
		}
	}

	// Compute conviction scores for each currency (full 5-source V3: COT + Calendar + Stress + FRED + Price)
	convictions := make([]cot.ConvictionScore, 0, len(analyses))
	for _, a := range analyses {
		var r fred.MacroRegime
		if regime != nil {
			r = *regime
		}
		// Pull per-currency weekly surprise sigma from accumulator (0.0 if not available)
		surpriseSigma := 0.0
		if h.newsScheduler != nil {
			surpriseSigma = h.newsScheduler.GetSurpriseSigma(a.Contract.Currency)
		}
		var pc *domain.PriceContext
		if priceCtxs != nil {
			pc = priceCtxs[a.Contract.Code]
		}
		cs := cot.ComputeConvictionScoreV3(a, r, surpriseSigma, "", macroData, pc)
		convictions = append(convictions, cs)
	}

	now := timeutil.NowWIB()
	html := h.fmt.FormatRankingWithConviction(analyses, convictions, regime, now)

	// Dual price + COT strength ranking (best-effort, non-fatal)
	if priceCtxs != nil {
		strengths := pricesvc.ComputeCurrencyStrengthIndex(priceCtxs, analyses)
		if len(strengths) > 0 {
			html += h.fmt.FormatStrengthRanking(strengths)
		}
	}

	// Daily momentum snapshot (best-effort, non-fatal)
	if h.dailyPriceRepo != nil {
		dailyBuilder := pricesvc.NewDailyContextBuilder(h.dailyPriceRepo)
		if dailyCtxs, dErr := dailyBuilder.BuildAll(ctx); dErr == nil && len(dailyCtxs) > 0 {
			html += h.fmt.FormatDailyMomentumSnapshot(dailyCtxs)
		}
	}

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	kb := h.kb.RelatedCommandsKeyboard("rank", "")
	if len(kb.Rows) > 0 {
		_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	} else {
		_, err = h.bot.SendHTML(ctx, chatID, html)
	}
	return err
}
