package telegram

import (
	"context"
	"fmt"
	"html"
	"os"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// cmdSeasonal handles /seasonal [currency] — advanced seasonal pattern analysis.
// Enriches base statistics with regime context, COT alignment, event density,
// volatility regime, cross-asset checks, EIA data, and confluence scoring.
func (h *Handler) cmdSeasonal(ctx context.Context, chatID string, _ int64, args string) error {
	if h.priceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet. Prices are fetched periodically.")
		return err
	}

	analyzer := pricesvc.NewSeasonalAnalyzer(h.priceRepo)
	args = strings.TrimSpace(strings.ToUpper(args))

	// Build context dependencies for advanced analysis
	deps := h.buildSeasonalDeps(ctx)

	if args != "" {
		// Single contract mode
		mapping := domain.FindPriceMappingByCurrency(args)
		if mapping == nil || mapping.RiskOnly {
			_, err := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("Unknown currency: <code>%s</code>\n\nUsage: <code>/seasonal</code> (all) or <code>/seasonal EUR</code>",
					html.EscapeString(args)))
			return err
		}

		loadingID, _ := h.bot.SendLoading(ctx, chatID,
			fmt.Sprintf("📅 Menganalisis seasonal pattern untuk <b>%s</b>... ⏳", html.EscapeString(args)))

		pattern, err := analyzer.AnalyzeContractAdvanced(ctx, mapping.ContractCode, mapping.Currency, deps)
		if err != nil {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			h.sendUserError(ctx, chatID, err, "seasonal")
			return nil
		}

		htmlOut := h.fmt.FormatSeasonalSingle(*pattern)
		kb := h.kb.SeasonalDetailMenu(mapping.Currency)
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, kb)
		return err
	}

	// All contracts mode
	loadingID, _ := h.bot.SendLoading(ctx, chatID, "📅 Menganalisis seasonal patterns... ⏳")

	patterns, err := analyzer.AnalyzeAllAdvanced(ctx, deps)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "seasonal")
		return nil
	}

	htmlOut := h.fmt.FormatSeasonalPatterns(patterns)
	kb := h.kb.SeasonalMenu()
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, kb)
	return err
}

// buildSeasonalDeps assembles all available context for advanced seasonal analysis.
// Any unavailable dependency is nil — the analyzer gracefully degrades.
func (h *Handler) buildSeasonalDeps(ctx context.Context) *pricesvc.SeasonalContextDeps {
	deps := &pricesvc.SeasonalContextDeps{
		PriceRepo: h.priceRepo,
	}

	// Wire COT repo
	if h.cotRepo != nil {
		deps.COTRepo = h.cotRepo
	}

	// Wire news repo for event density
	if h.newsRepo != nil {
		deps.NewsRepo = h.newsRepo
	}

	// Fetch current FRED macro data (non-fatal if unavailable)
	macroData, err := fred.FetchMacroData(ctx)
	if err == nil && macroData != nil {
		deps.MacroData = macroData
		deps.VIXPrice = macroData.VIX
	}

	// Fetch historical regimes for regime-filtered seasonal (extend to 260 weeks)
	regimes, err := fred.FetchHistoricalRegimes(ctx, 260)
	if err == nil {
		deps.Regimes = regimes
	}

	// EIA data for energy pairs
	eiaKey := os.Getenv("EIA_API_KEY")
	if eiaKey != "" {
		eiaClient := pricesvc.NewEIAClient(eiaKey)
		eiaData, err := eiaClient.FetchSeasonalData(ctx)
		if err == nil {
			deps.EIAData = eiaData
		}
	}

	return deps
}
