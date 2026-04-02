package telegram

// handler_signal_cmd.go — /signal command: Unified Directional Signal (TASK-113)
// Aggregates COT + CTA + Quant + Sentiment + Seasonal into a single scored recommendation.

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/analysis"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// cmdSignal handles /signal [CURRENCY] — Unified Directional Signal.
// Shows a fused score from COT + CTA + Quant + Sentiment + Seasonal with
// conflict detection and VIX dampening.
func (h *Handler) cmdSignal(ctx context.Context, chatID string, userID int64, args string) error {
	currency := strings.ToUpper(strings.TrimSpace(args))

	if currency == "" {
		lc := h.getLastCurrency(ctx, userID)
		if lc != "" {
			currency = lc
		}
	}

	if currency == "" || currency == "ALL" {
		return h.cmdSignalAll(ctx, chatID, userID)
	}

	h.saveLastCurrency(ctx, userID, currency)
	return h.sendSignalForCurrency(ctx, chatID, currency)
}

// sendSignalForCurrency computes and sends the unified signal for one currency.
func (h *Handler) sendSignalForCurrency(ctx context.Context, chatID, currency string) error {
	loadingID, _ := h.bot.SendHTML(ctx, chatID, "🎯 Computing unified signal for <b>"+currency+"</b>... ⏳")

	sig, err := h.computeUnifiedSignal(ctx, currency)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			"❌ <b>Signal error</b>\n\n<code>"+err.Error()+"</code>")
		return sendErr
	}

	text := h.fmt.FormatUnifiedSignal(sig)
	kb := h.kb.RelatedCommandsKeyboard("signal", currency)
	if len(kb.Rows) > 0 {
		_, err = h.bot.SendWithKeyboard(ctx, chatID, text, kb)
	} else {
		_, err = h.bot.SendHTML(ctx, chatID, text)
	}
	return err
}

// cmdSignalAll computes the unified signal for all tracked currencies.
func (h *Handler) cmdSignalAll(ctx context.Context, chatID string, userID int64) error {
	loadingID, _ := h.bot.SendHTML(ctx, chatID, "🎯 Computing unified signals for all currencies... ⏳")

	currencies := []string{"EUR", "GBP", "JPY", "CHF", "AUD", "CAD", "NZD"}
	var results []*analysis.UnifiedSignalV2
	for _, c := range currencies {
		sig, err := h.computeUnifiedSignal(ctx, c)
		if err == nil {
			results = append(results, sig)
		}
	}

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}

	if len(results) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("no unified signal data available"), "signal")
		return nil
	}

	text := h.fmt.FormatUnifiedSignalOverview(results)
	_, err := h.bot.SendHTML(ctx, chatID, text)
	return err
}

// computeUnifiedSignal gathers all inputs and calls analysis.ComputeUnifiedSignal.
func (h *Handler) computeUnifiedSignal(ctx context.Context, currency string) (*analysis.UnifiedSignalV2, error) {
	contractCode := currencyToContractCode(currency)

	// --- COT component (required) ---
	cotAnalysis, err := h.cotRepo.GetLatestAnalysis(ctx, contractCode)
	if err != nil {
		return nil, err
	}

	// --- FRED macro regime + macro data ---
	macroData, _ := fred.GetCachedOrFetch(ctx)
	var regime fred.MacroRegime
	if macroData != nil {
		regime = fred.ClassifyMacroRegime(macroData)
	}

	// --- Surprise sigma ---
	surpriseSigma := 0.0
	if h.newsScheduler != nil {
		surpriseSigma = h.newsScheduler.GetSurpriseSigma(currency)
	}

	// --- CTA confluence (optional) ---
	var ctaConfl *ta.ConfluenceResult
	if h.cta != nil && h.cta.DailyPriceRepo != nil {
		dailyRecords, pErr := h.cta.DailyPriceRepo.GetDailyHistory(ctx, contractCode, 300)
		if pErr == nil && len(dailyRecords) >= 50 {
			bars := ta.DailyPricesToOHLCV(dailyRecords)
			if h.cta.TAEngine != nil {
				full := h.cta.TAEngine.ComputeFull(bars)
				if full != nil {
					ctaConfl = full.Confluence
				}
			}
		}
	}

	// --- HMM + GARCH (optional, requires DailyPrice → PriceRecord conversion) ---
	var hmmResult *pricesvc.HMMResult
	var garchResult *pricesvc.GARCHResult
	if h.quant != nil && h.quant.DailyPriceRepo != nil {
		dailyPrices, pErr := h.quant.DailyPriceRepo.GetDailyHistory(ctx, contractCode, 500)
		if pErr == nil && len(dailyPrices) >= 50 {
			priceRecords := dailyPricesToPriceRecords(dailyPrices)
			if hmm, hErr := pricesvc.EstimateHMMRegime(priceRecords); hErr == nil {
				hmmResult = hmm
			}
			if garch, gErr := pricesvc.EstimateGARCH(priceRecords); gErr == nil {
				garchResult = garch
			}
		}
	}

	// --- Risk context (VIX / sentiment) ---
	var riskCtx *domain.RiskContext
	if h.priceRepo != nil {
		rb := pricesvc.NewRiskContextBuilder(h.priceRepo)
		riskCtx, _ = rb.Build(ctx)
	}

	// --- Seasonal (optional) ---
	var seasonal *pricesvc.SeasonalPattern
	if h.priceRepo != nil {
		sa := pricesvc.NewSeasonalAnalyzer(h.priceRepo)
		if sp, sErr := sa.AnalyzeContract(ctx, contractCode, currency); sErr == nil {
			seasonal = sp
		}
	}

	return analysis.ComputeUnifiedSignalForCurrency(
		ctx, currency, cotAnalysis, regime, macroData,
		surpriseSigma, ctaConfl, hmmResult, garchResult, riskCtx, seasonal,
	), nil
}

// dailyPricesToPriceRecords converts []domain.DailyPrice to []domain.PriceRecord
// for use with HMM and GARCH estimators.
func dailyPricesToPriceRecords(daily []domain.DailyPrice) []domain.PriceRecord {
	out := make([]domain.PriceRecord, len(daily))
	for i, d := range daily {
		out[i] = domain.PriceRecord{
			ContractCode: d.ContractCode,
			Symbol:       d.Symbol,
			Date:         d.Date,
			Open:         d.Open,
			High:         d.High,
			Low:          d.Low,
			Close:        d.Close,
			Volume:       d.Volume,
			Source:       d.Source,
		}
	}
	return out
}
