package telegram

// /intermarket — Intermarket Correlation Signal Engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/intermarket"
)

// cmdIntermarket handles /intermarket — shows intermarket correlation divergences.
func (h *Handler) cmdIntermarket(ctx context.Context, chatID string, _ int64, args string) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet. Please try again later.")
		return err
	}

	forceRefresh := strings.ToUpper(strings.TrimSpace(args)) == "REFRESH"
	if forceRefresh {
		intermarket.InvalidateCache()
	}

	loadingMsg := "🔗 Menganalisis intermarket correlations... ⏳"
	if !forceRefresh {
		loadingMsg = "🔗 Memuat intermarket correlations (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, loadingMsg)

	engine := intermarket.NewEngine(h.dailyPriceRepo)
	result, err := engine.GetCachedOrAnalyze(ctx)
	if err != nil {
		if placeholderID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, placeholderID)
		}
		log.Error().Err(err).Msg("intermarket analysis failed")
		_, sendErr := h.bot.SendHTML(ctx, chatID, "⚠️ Gagal mengambil data intermarket. Silakan coba lagi.")
		return sendErr
	}

	text := formatIntermarketResult(result)
	if placeholderID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, placeholderID)
	}
	kb := h.kb.RelatedCommandsKeyboard("intermarket", "")
	if len(kb.Rows) > 0 {
		_, err = h.bot.SendWithKeyboard(ctx, chatID, text, kb)
	} else {
		_, err = h.bot.SendHTML(ctx, chatID, text)
	}
	return err
}

// formatIntermarketResult renders an IntermarketResult as Telegram HTML.
func formatIntermarketResult(r *intermarket.IntermarketResult) string {
	var sb strings.Builder

	dateStr := r.AsOf.Format("2006-01-02")
	sb.WriteString(fmt.Sprintf("🔗 <b>Intermarket Correlation</b> [%s]\n\n", dateStr))

	// Count aligned vs diverging (excluding insufficient)
	aligned := filterSignals(r.Signals, intermarket.StatusAligned)
	diverging := filterSignals(r.Signals, intermarket.StatusDiverging)
	broken := filterSignals(r.Signals, intermarket.StatusBroken)

	totalValid := countValid(r.Signals)
	alignedCount := len(aligned)

	if alignedCount > 0 {
		sb.WriteString(fmt.Sprintf("🟢 <b>ALIGNED (%d/%d relationships on track):</b>\n", alignedCount, totalValid))
		for _, s := range aligned {
			corrStr := fmt.Sprintf("%.2f", s.ActualCorr)
			dirEmoji := "↗"
			if s.ActualCorr < 0 {
				dirEmoji = "↘"
			}
			sb.WriteString(fmt.Sprintf("  • %s: %s%s corr ✅\n", s.Rule.Label, dirEmoji, corrStr))
		}
		sb.WriteString("\n")
	}

	divCount := len(diverging) + len(broken)
	if divCount > 0 {
		sb.WriteString(fmt.Sprintf("🔴 <b>DIVERGING (%d/%d relationships breaking):</b>\n", divCount, totalValid))
		for _, s := range broken {
			sb.WriteString(fmt.Sprintf("  • %s: %+.2f corr 🚨 (expected %s)\n",
				s.Rule.Label, s.ActualCorr, directionLabel(s.Rule.Direction)))
			sb.WriteString(fmt.Sprintf("    → <i>%s</i>\n", s.Implication))
		}
		for _, s := range diverging {
			sb.WriteString(fmt.Sprintf("  • %s: %+.2f corr ⚠️ (expected %s)\n",
				s.Rule.Label, s.ActualCorr, directionLabel(s.Rule.Direction)))
			sb.WriteString(fmt.Sprintf("    → <i>%s</i>\n", s.Implication))
		}
		sb.WriteString("\n")
	}

	// Insufficient data
	var insufficient []intermarket.IntermarketSignal
	for _, s := range r.Signals {
		if s.Insufficient {
			insufficient = append(insufficient, s)
		}
	}
	if len(insufficient) > 0 {
		var labels []string
		for _, s := range insufficient {
			labels = append(labels, s.Rule.Label)
		}
		sb.WriteString(fmt.Sprintf("⚪ <i>Data tidak cukup untuk: %s</i>\n\n", strings.Join(labels, ", ")))
	}

	// Risk regime
	regimeEmoji, regimeText := formatRiskRegime(r.RiskRegime, divCount)
	sb.WriteString(fmt.Sprintf("📊 <b>Risk Regime: %s %s</b>\n", regimeEmoji, r.RiskRegime))
	sb.WriteString(fmt.Sprintf("   → <i>%s</i>\n", regimeText))

	// Cache hint
	sb.WriteString(fmt.Sprintf("\n<i>Data: %s UTC • Gunakan /intermarket refresh untuk update</i>",
		r.AsOf.UTC().Format("15:04")))

	return sb.String()
}

func filterSignals(signals []intermarket.IntermarketSignal, status intermarket.CorrelationStatus) []intermarket.IntermarketSignal {
	var out []intermarket.IntermarketSignal
	for _, s := range signals {
		if !s.Insufficient && s.Status == status {
			out = append(out, s)
		}
	}
	return out
}

func countValid(signals []intermarket.IntermarketSignal) int {
	n := 0
	for _, s := range signals {
		if !s.Insufficient {
			n++
		}
	}
	return n
}

func directionLabel(d int) string {
	if d > 0 {
		return "positive"
	}
	return "negative"
}

func formatRiskRegime(regime string, divergeCount int) (string, string) {
	switch regime {
	case "RISK_ON":
		return "🟢 Risk-On", "Cross-asset signals aligned for risk appetite — prefer risk-on currencies (AUD, NZD, CAD)"
	case "RISK_OFF":
		return "🔴 Risk-Off", "Safe-haven demand confirmed across markets — prefer JPY, CHF, Gold"
	default: // MIXED
		if divergeCount >= 3 {
			return "🟡", fmt.Sprintf("%d divergences aktif — trade individual pairs daripada broad regime bias", divergeCount)
		}
		return "🟡", "Sinyal conflicting — monitor divergences dan tunggu konfirmasi"
	}
}
