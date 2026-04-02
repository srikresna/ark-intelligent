package telegram

// handler_flows.go — /flows command: Cross-Asset Flow Divergence (TASK-162)

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/factors"
)

// cmdFlows handles /flows — shows cross-asset flow divergence analysis.
func (h *Handler) cmdFlows(ctx context.Context, chatID string, _ int64, args string) error {
	if h.dailyPriceRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Price data not available yet. Please try again later.")
		return err
	}

	forceRefresh := strings.ToUpper(strings.TrimSpace(args)) == "REFRESH"
	if forceRefresh {
		factors.InvalidateFlowCache()
	}

	loadMsg := "🔀 Menganalisis cross-asset flow divergences... ⏳"
	if !forceRefresh {
		loadMsg = "🔀 Memuat flow divergences (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, loadMsg)

	engine := factors.NewFlowDivergenceEngine(h.dailyPriceRepo)
	result, err := engine.GetCachedOrAnalyze(ctx)
	if err != nil {
		if placeholderID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, placeholderID)
		}
		h.sendUserError(ctx, chatID, err, "flows")
		return nil
	}

	text := formatFlowDivergenceResult(result)
	if placeholderID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, placeholderID)
	}
	kb := h.kb.RelatedCommandsKeyboard("flows", "")
	if len(kb.Rows) > 0 {
		_, err = h.bot.SendWithKeyboard(ctx, chatID, text, kb)
	} else {
		_, err = h.bot.SendHTML(ctx, chatID, text)
	}
	return err
}

// formatFlowDivergenceResult renders a FlowDivergenceResult as Telegram HTML.
func formatFlowDivergenceResult(r *factors.FlowDivergenceResult) string {
	var sb strings.Builder

	dateStr := r.ComputedAt.Format("2006-01-02 15:04")
	sb.WriteString(fmt.Sprintf("🔀 <b>Cross-Asset Flow Divergence</b> [%s UTC]\n\n", dateStr))

	// Regime stability bar.
	stabilityPct := int(r.RegimeStability * 100)
	stabilityEmoji := regimeStabilityEmoji(r.RegimeStability)
	sb.WriteString(fmt.Sprintf("%s <b>Regime Stability: %d%%</b> (%d/%d pairs aligned)\n\n",
		stabilityEmoji, stabilityPct,
		flowCountStable(r.Pairs), flowCountValid(r.Pairs)))

	// Top divergences (alert-worthy).
	if len(r.TopDivergences) > 0 {
		sb.WriteString("🚨 <b>ACTIVE DIVERGENCES:</b>\n")
		for _, pd := range r.TopDivergences {
			sb.WriteString(formatPairDivergence(pd))
		}
		sb.WriteString("\n")
	}

	// Aligned pairs summary.
	aligned := flowCountStable(r.Pairs)
	valid := flowCountValid(r.Pairs)
	if aligned > 0 {
		sb.WriteString(fmt.Sprintf("✅ <b>ALIGNED (%d/%d):</b> ", aligned, valid))
		var labels []string
		for _, pd := range r.Pairs {
			if !pd.Insufficient && !pd.IsDiverging {
				labels = append(labels, pd.Pair.Label)
			}
		}
		sb.WriteString(strings.Join(labels, ", "))
		sb.WriteString("\n\n")
	}

	// Insufficient data.
	var missing []string
	for _, pd := range r.Pairs {
		if pd.Insufficient {
			missing = append(missing, pd.Pair.Label)
		}
	}
	if len(missing) > 0 {
		sb.WriteString(fmt.Sprintf("⚪ <i>Data tidak cukup: %s</i>\n\n", strings.Join(missing, ", ")))
	}

	// Trading note.
	if len(r.TopDivergences) >= 3 {
		sb.WriteString("⚡ <i>Multiple divergences aktif — trade individual pairs, bukan broad regime bias.</i>\n\n")
	} else if len(r.TopDivergences) == 0 {
		sb.WriteString("💡 <i>Cross-asset relationships normal — regime signals reliable.</i>\n\n")
	}

	sb.WriteString(fmt.Sprintf("<i>Data: %s UTC • /flows refresh untuk update</i>", r.ComputedAt.UTC().Format("15:04")))
	return sb.String()
}

// formatPairDivergence renders a single PairDivergence entry.
func formatPairDivergence(pd factors.PairDivergence) string {
	zAbs := math.Abs(pd.DivergenceZ)
	zIcon := "⚠️"
	if zAbs > 3.0 {
		zIcon = "🚨"
	}

	dirLabel := "pos"
	if pd.Pair.Direction < 0 {
		dirLabel = "inv"
	}

	line := fmt.Sprintf("  %s <b>%s</b>: r=<code>%.2f</code> z=<code>%.1f</code> (baseline %.2f±%.2f, %s)\n",
		zIcon, pd.Pair.Label,
		pd.CurrentCorr, pd.DivergenceZ,
		pd.BaselineMean, pd.BaselineStd, dirLabel)

	if pd.LeadLag.LeadAsset != "" && pd.LeadLag.LeadAsset != "simultaneous" {
		line += fmt.Sprintf("    📊 Lead-lag: %s\n", pd.LeadLag.LeadAsset)
	}

	line += fmt.Sprintf("    → <i>%s</i>\n", pd.Pair.Implication)
	return line
}

func regimeStabilityEmoji(s float64) string {
	switch {
	case s >= 0.8:
		return "🟢 Stable"
	case s >= 0.5:
		return "🟡 Moderate"
	default:
		return "🔴 Unstable"
	}
}

func flowCountValid(pairs []factors.PairDivergence) int {
	n := 0
	for _, p := range pairs {
		if !p.Insufficient {
			n++
		}
	}
	return n
}

func flowCountStable(pairs []factors.PairDivergence) int {
	n := 0
	for _, p := range pairs {
		if !p.Insufficient && !p.IsDiverging {
			n++
		}
	}
	return n
}
