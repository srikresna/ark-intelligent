package telegram

// handler_qbacktest.go — Command handler untuk /qbacktest
//   /qbacktest [SYMBOL] [MODEL] — Run backtest untuk quant model

import (
	"context"
	"fmt"
	"html"
	"strings"
)

// registerQuantBacktestCommands register /qbacktest command
func (h *Handler) registerQuantBacktestCommands() {
	h.bot.RegisterCommand("/qbacktest", h.cmdQBacktest)
	h.bot.RegisterCommand("/quantbacktest", h.cmdQBacktest) // alias
	h.bot.RegisterCallback("qbacktest:", h.handleQBacktestCallback)
}

// cmdQBacktest handles /qbacktest [SYMBOL] [MODEL]
func (h *Handler) cmdQBacktest(ctx context.Context, chatID string, userID int64, args string) error {
	_ = h.bot.SendChatAction(ctx, chatID, "typing")

	if h.quant == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Quant Engine not configured.")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	
	// If no args, show dashboard or use last symbol
	if len(parts) == 0 {
		// Try last currency
		if lc := h.getLastCurrency(ctx, userID); lc != "" {
			return h.cmdQBacktest(ctx, chatID, userID, lc)
		}
		// Show symbol selector
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`📊 <b>QUANT BACKTEST DASHBOARD</b>

Backtest untuk model ekonometrik:

📈 Stats (Distribusi return, Sharpe, VaR)
📉 GARCH (Volatility clustering)
🔗 Correlation (Multi-asset)
🎭 Regime (HMM Bull/Bear)
🔄 MeanRevert (ADF, Hurst)
⚡ Granger (Kausalitas)
🔗 Cointegration (Pair trading)
🧬 PCA (Factor analysis)
🌐 VAR (Multi-asset forecast)
⚠️ Risk (VaR/CVaR)

Atau pilih simbol:`, h.kb.QuantSymbolMenu())
		return err
	}

	symbol := parts[0]
	model := ""
	if len(parts) > 1 {
		model = strings.ToLower(parts[1])
	}

	// Validate symbol
	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/qbacktest EUR</code>, <code>/qbacktest XAU garch</code>",
			html.EscapeString(symbol),
		))
		return err
	}

	h.saveLastCurrency(ctx, userID, mapping.Currency)

	sym := html.EscapeString(mapping.Currency)
	
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf("📊 Running backtest for <b>%s</b>...", sym))

	analyzer := NewQuantBacktestAnalyzer(h.quant)
	stats, err := analyzer.Analyze(ctx, symbol, model)
	
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	
	if err != nil {
		h.sendUserError(ctx, chatID, err, "qbacktest")
		return nil
	}

	if len(stats.Models) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ Tidak ada hasil backtest untuk <b>%s</b>.\n\nCoba model lain atau pastikan ada cukup data historis.",
			sym,
		))
		return err
	}

	// Format output
	htmlOut := h.formatQBacktestStats(stats)
	
	kb := h.kb.QBacktestMenu()
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, htmlOut, kb)
	return err
}

// formatQBacktestStats format backtest results
func (h *Handler) formatQBacktestStats(stats *QuantBacktestStats) string {
	var b strings.Builder
	
	b.WriteString(fmt.Sprintf("📊 <b>QUANT BACKTEST: %s</b>\n\n", html.EscapeString(stats.Symbol)))
	
	if len(stats.Models) == 1 {
		// Single model detail view
		m := stats.Models[0]
		b.WriteString(h.formatSingleModelBacktest(&m))
	} else {
		// Multi-model summary
		b.WriteString(h.formatMultiModelSummary(stats.Models))
	}
	
	return b.String()
}

func (h *Handler) formatSingleModelBacktest(m *QuantBacktestResult) string {
	var b strings.Builder
	
	b.WriteString(fmt.Sprintf("<b>Model:</b> %s\n", m.Model))
	b.WriteString(fmt.Sprintf("<b>Symbol:</b> %s\n\n", m.Symbol))
	
	b.WriteString("<b>Performance Summary</b>\n")
	b.WriteString(fmt.Sprintf("<code>Signals     :</code> %d (evaluated: %d)\n", m.TotalSignals, m.SampleSize))
	b.WriteString(fmt.Sprintf("<code>Confidence  :</code> %.0f%%\n\n", m.Confidence))
	
	b.WriteString("<b>Win Rate</b>\n")
	b.WriteString(fmt.Sprintf("<code>1 Week :</code> %.1f%%\n", m.WinRate1W))
	b.WriteString(fmt.Sprintf("<code>2 Week :</code> %.1f%%\n", m.WinRate2W))
	b.WriteString(fmt.Sprintf("<code>4 Week :</code> %.1f%%\n\n", m.WinRate4W))
	
	b.WriteString("<b>Average Return</b>\n")
	b.WriteString(fmt.Sprintf("<code>1 Week :</code> %+.4f%%\n", m.AvgReturn1W))
	b.WriteString(fmt.Sprintf("<code>2 Week :</code> %+.4f%%\n", m.AvgReturn2W))
	b.WriteString(fmt.Sprintf("<code>4 Week :</code> %+.4f%%\n\n", m.AvgReturn4W))
	
	b.WriteString("<b>Risk Metrics</b>\n")
	b.WriteString(fmt.Sprintf("<code>Sharpe Ratio :</code> %.2f\n", m.SharpeRatio))
	b.WriteString(fmt.Sprintf("<code>Max Drawdown :</code> %.2f%%\n", m.MaxDrawdown))
	if m.ProfitFactor > 0 {
		b.WriteString(fmt.Sprintf("<code>Profit Factor:</code> %.2f\n", m.ProfitFactor))
	}
	
	// Recommendation
	b.WriteString("\n")
	if m.SampleSize < 30 {
		b.WriteString("⚠️ <i>Sample size kecil — hasil belum statistically significant.</i>\n")
	} else if m.WinRate4W > 55 && m.SharpeRatio > 1.0 {
		b.WriteString("✅ <i>Strong edge detected — model ini menunjukkan performa konsisten.</i>\n")
	} else if m.WinRate4W > 50 && m.SharpeRatio > 0.5 {
		b.WriteString("⚠️ <i>Moderate edge — monitor performance lebih lanjut.</i>\n")
	} else if m.SharpeRatio > 0 {
		b.WriteString("🔴 <i>Weak edge — model belum cukup robust.</i>\n")
	} else {
		b.WriteString("❌ <i>No edge detected — model tidak profitable di backtest.</i>\n")
	}
	
	return b.String()
}

func (h *Handler) formatMultiModelSummary(models []QuantBacktestResult) string {
	var b strings.Builder
	
	b.WriteString("<b>Model Performance Summary</b>\n\n")
	b.WriteString("<code>Model          WR4W   Sharpe   Sample  Conf</code>\n")
	b.WriteString("<code>──────────────────────────────────────────</code>\n")
	
	for _, m := range models {
		confIcon := "🟢"
		if m.Confidence < 75 {
			confIcon = "🟡"
		}
		if m.Confidence < 60 {
			confIcon = "🔴"
		}
		
		b.WriteString(fmt.Sprintf("<code>%-15s %5.1f%%  %6.2f   %5d   %s</code>\n",
			strings.ToUpper(m.Model),
			m.WinRate4W,
			m.SharpeRatio,
			m.SampleSize,
			confIcon,
		))
	}
	
	b.WriteString("\n<i>🟢 High Confidence | 🟡 Medium | 🔴 Low</i>\n\n")
	
	// Find best model
	bestModel := ""
	bestScore := -999.0
	for _, m := range models {
		if m.SampleSize < 30 {
			continue
		}
		score := m.WinRate4W + (m.SharpeRatio * 10)
		if score > bestScore {
			bestScore = score
			bestModel = m.Model
		}
	}
	
	if bestModel != "" {
		b.WriteString(fmt.Sprintf("🏆 <b>Best Model:</b> %s (score: %.1f)\n", strings.ToUpper(bestModel), bestScore))
	}
	
	return b.String()
}

// handleQBacktestCallback handle callback untuk backtest menu
func (h *Handler) handleQBacktestCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 3 {
		return nil
	}
	// data format: "qbacktest:model:STATS" or "qbacktest:sym:EUR"
	secondPart := parts[1]
	thirdPart := ""
	if len(parts) > 2 {
		thirdPart = parts[2]
	}
	
	// Model selection: qbacktest:model:MODELNAME
	if secondPart == "model" && thirdPart != "" {
		model := strings.ToLower(thirdPart)
		// Get current symbol from cache or last currency
		if lc := h.getLastCurrency(ctx, 0); lc != "" {
			return h.cmdQBacktest(ctx, chatID, 0, lc+" "+model)
		}
	}
	
	// Symbol selection: qbacktest:sym:SYMBOL
	if secondPart == "sym" && thirdPart != "" {
		sym := thirdPart
		return h.cmdQBacktest(ctx, chatID, 0, sym)
	}
	
	// Back to dashboard
	if secondPart == "back" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendHTML(ctx, chatID, "📊 Quant Backtest Dashboard\n\nGunakan /qbacktest [SYMBOL] [MODEL]")
		return err
	}
	
	return nil
}
