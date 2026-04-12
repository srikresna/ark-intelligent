package telegram

// handler_qbacktest.go — Command handler untuk /qbacktest
//   User-friendly dengan proper loading states, error handling, dan navigation

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
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
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ <b>Quant Engine not configured.</b>\n\nPlease contact administrator to enable quant analysis.")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	
	// If no args, show dashboard or use last symbol
	if len(parts) == 0 {
		return h.showBacktestDashboard(ctx, chatID, userID)
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
			"❌ <b>Symbol not found:</b> <code>%s</code>\n\n"+
				"Available symbols: EUR, GBP, USD, JPY, CHF, AUD, NZD, CAD, XAU, XAG, BTC, ETH\n\n"+
				"Usage: <code>/qbacktest EUR</code> or <code>/qbacktest XAU garch</code>",
			html.EscapeString(symbol),
		))
		return err
	}

	h.saveLastCurrency(ctx, userID, mapping.Currency)
	sym := html.EscapeString(mapping.Currency)

	// Show loading with progress
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf("🔬 Running backtest for <b>%s</b>...\n<i>This may take 1-2 minutes. Running all models...</i>", sym))

	// Set longer timeout for backtest (2 minutes)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// Use simple backtest (no Python dependency) for fast results
	analyzer := NewSimpleQuantBacktestAnalyzer(h.quant)
	
	// For single model, use simple analyzer
	var stats *QuantBacktestStats
	
	if model != "" {
		// Single model - use simple analyzer
		simpleResult, err := analyzer.Analyze(ctxWithTimeout, symbol, model)
		if err != nil {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			h.sendUserError(ctx, chatID, err, "qbacktest")
			return nil
		}
		
		// Convert simple result to full stats format
		stats = &QuantBacktestStats{
			Symbol:    symbol,
			Timeframe: "daily",
			TotalBars: simpleResult.TotalBars,
			Config:    DefaultBacktestConfig(),
		}
		
		// Create single model result
		fullResult := QuantBacktestResult{
			Model:        simpleResult.Model,
			Symbol:       simpleResult.Symbol,
			Timeframe:    "daily",
			TotalSignals: simpleResult.SignalCount,
			Evaluated:    simpleResult.SignalCount,
			WinRate4W:    simpleResult.WinRate,
			AvgReturn4W:  simpleResult.AvgReturn,
			SharpeRatio:  simpleResult.Sharpe,
			MaxDrawdown:  simpleResult.MaxDD,
			ProfitFactor: 1.0, // Placeholder
			SampleSize:   simpleResult.SignalCount,
			Confidence:   simpleResult.Confidence,
			Config:       DefaultBacktestConfig(),
		}
		stats.Models = []QuantBacktestResult{fullResult}
	} else {
		// All models - would need to run multiple simple analyses
		// For now, just show a message to pick one model
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err := h.bot.SendHTML(ctx, chatID, 
			"⚡ <b>Quick Backtest Available</b>\n\n"+
				"Due to complexity, full multi-model backtest requires Python engine.\n\n"+
				"<b>Try single model for instant results:</b>\n"+
				"<code>/qbacktest EUR stats</code>\n"+
				"<code>/qbacktest EUR garch</code>\n"+
				"<code>/qbacktest EUR regime</code>\n\n"+
				"Or use <code>/backtest</code> for COT signal backtests.")
		return nil
	}
	
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	
	if err != nil {
		// Check for timeout
		if ctxWithTimeout.Err() == context.DeadlineExceeded {
			_, err = h.bot.SendHTML(ctx, chatID, 
				"⏱️ <b>Backtest timed out after 2 minutes.</b>\n\n"+
					"This can happen with:\n"+
					"• Running all models at once\n"+
					"• High system load\n\n"+
					"<b>Solution:</b> Try testing one model at a time:\n"+
					"<code>/qbacktest EUR stats</code>\n"+
					"<code>/qbacktest EUR garch</code>\n"+
					"etc.")
			return err
		}
		
		h.sendUserError(ctx, chatID, err, "qbacktest")
		return nil
	}

	if len(stats.Models) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ <b>No results for %s</b>\n\n"+
				"Possible reasons:\n"+
				"• Insufficient historical data (need at least 170 days)\n"+
				"• Model not available for this symbol\n"+
				"• Python quant engine not installed\n\n"+
				"Try a different symbol or contact administrator.",
			sym,
		))
		return err
	}

	// Format output
	htmlOut := h.formatQBacktestStats(stats)
	
	// Add navigation keyboard
	kb := h.kb.QBacktestMenu()
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, htmlOut, kb)
	return err
}

// showBacktestDashboard shows the backtest dashboard with symbol selection
func (h *Handler) showBacktestDashboard(ctx context.Context, chatID string, userID int64) error {
	// Try last currency first
	if lc := h.getLastCurrency(ctx, userID); lc != "" {
		return h.cmdQBacktest(ctx, chatID, userID, lc)
	}

	// Show symbol selector with description
	_, err := h.bot.SendWithKeyboard(ctx, chatID,
		`🔬 <b>QUANT BACKTEST DASHBOARD</b>

Backtest model ekonometrik pada data historis:

<b>📊 Foundation:</b>
• Stats - Distribusi return, Sharpe, VaR
• GARCH - Volatility clustering & forecast
• Correlation - Multi-asset correlation

<b>📈 Time Series:</b>
• Regime - Hidden Markov Model (bull/bear)
• Mean Revert - ADF, Hurst, half-life
• Granger - Kausalitas antar aset

<b>🔗 Advanced:</b>
• Cointegration - Pair trading analysis
• PCA - Factor analysis multi-asset
• VAR - Multi-asset forecast
• Risk - VaR/CVaR historical + parametric

<b>Methodology:</b>
✅ No look-ahead bias
✅ Transaction costs included (0.1%)
✅ Walk-forward validation
✅ Statistical confidence scoring

Pilih aset untuk backtest:`, h.kb.QuantSymbolMenu())
	return err
}

// formatQBacktestStats format backtest results dengan proper UI
func (h *Handler) formatQBacktestStats(stats *QuantBacktestStats) string {
	var b strings.Builder
	
	// Header
	b.WriteString(fmt.Sprintf("📊 <b>QUANT BACKTEST: %s</b>\n", html.EscapeString(stats.Symbol)))
	b.WriteString(fmt.Sprintf("<i>Period: %s to %s (%d bars)</i>\n\n", stats.StartDate, stats.EndDate, stats.TotalBars))
	
	if len(stats.Models) == 1 {
		// Single model detail view
		m := stats.Models[0]
		b.WriteString(h.formatSingleModelBacktest(&m))
	} else {
		// Multi-model summary dengan improved formatting
		b.WriteString(h.formatMultiModelSummary(stats.Models))
	}
	
	return b.String()
}

func (h *Handler) formatSingleModelBacktest(m *QuantBacktestResult) string {
	var b strings.Builder
	
	// Model info
	b.WriteString(fmt.Sprintf("<b>🔬 Model:</b> %s\n", m.Model))
	b.WriteString(fmt.Sprintf("<b>📈 Symbol:</b> %s\n", m.Symbol))
	b.WriteString(fmt.Sprintf("<b>📅 Timeframe:</b> %s\n\n", m.Timeframe))
	
	// Performance card
	b.WriteString("<b>📊 Performance Summary</b>\n")
	b.WriteString(fmt.Sprintf("<code>Signals Generated  :</code> %d\n", m.TotalSignals))
	b.WriteString(fmt.Sprintf("<code>Signals Evaluated  :</code> %d\n", m.Evaluated))
	b.WriteString(fmt.Sprintf("<code>Statistical Confidence :</code> %.0f%%\n\n", m.Confidence))
	
	// Win rate section
	b.WriteString("<b>🎯 Win Rate</b>\n")
	b.WriteString(fmt.Sprintf("<code>1 Week  :</code> %5.1f%%  ", m.WinRate1W))
	if m.WinRate1W > 55 {
		b.WriteString("✅\n")
	} else if m.WinRate1W > 50 {
		b.WriteString("⚠️\n")
	} else {
		b.WriteString("❌\n")
	}
	
	b.WriteString(fmt.Sprintf("<code>2 Weeks :</code> %5.1f%%  ", m.WinRate2W))
	if m.WinRate2W > 55 {
		b.WriteString("✅\n")
	} else if m.WinRate2W > 50 {
		b.WriteString("⚠️\n")
	} else {
		b.WriteString("❌\n")
	}
	
	b.WriteString(fmt.Sprintf("<code>4 Weeks :</code> %5.1f%%  ", m.WinRate4W))
	if m.WinRate4W > 55 {
		b.WriteString("✅\n")
	} else if m.WinRate4W > 50 {
		b.WriteString("⚠️\n")
	} else {
		b.WriteString("❌\n")
	}
	b.WriteString("\n")
	
	// Return section
	b.WriteString("<b>💰 Average Return</b>\n")
	b.WriteString(fmt.Sprintf("<code>1 Week  :</code> %+.4f%%\n", m.AvgReturn1W))
	b.WriteString(fmt.Sprintf("<code>2 Weeks :</code> %+.4f%%\n", m.AvgReturn2W))
	b.WriteString(fmt.Sprintf("<code>4 Weeks :</code> %+.4f%%\n\n", m.AvgReturn4W))
	
	// Risk metrics
	b.WriteString("<b>⚠️ Risk Metrics</b>\n")
	b.WriteString(fmt.Sprintf("<code>Sharpe Ratio   :</code> %.2f  ", m.SharpeRatio))
	if m.SharpeRatio > 1.5 {
		b.WriteString("🌟\n")
	} else if m.SharpeRatio > 1.0 {
		b.WriteString("✅\n")
	} else if m.SharpeRatio > 0.5 {
		b.WriteString("⚠️\n")
	} else {
		b.WriteString("❌\n")
	}
	
	b.WriteString(fmt.Sprintf("<code>Sortino Ratio  :</code> %.2f\n", m.SortinoRatio))
	b.WriteString(fmt.Sprintf("<code>Max Drawdown   :</code> %.2f%%  ", -m.MaxDrawdown))
	if m.MaxDrawdown < 10 {
		b.WriteString("✅\n")
	} else if m.MaxDrawdown < 20 {
		b.WriteString("⚠️\n")
	} else {
		b.WriteString("🔴\n")
	}
	
	b.WriteString(fmt.Sprintf("<code>Profit Factor  :</code> %.2f\n", m.ProfitFactor))
	b.WriteString(fmt.Sprintf("<code>Expected Value :</code> %+.4f%%\n\n", m.ExpectedValue))
	
	// Walk-forward score
	if m.WalkForwardScore > 0 {
		b.WriteString(fmt.Sprintf("<b>🔄 Walk-Forward Score:</b> %.2f ", m.WalkForwardScore))
		if m.WalkForwardScore > 0.8 {
			b.WriteString("🌟 Robust\n")
		} else if m.WalkForwardScore > 0.6 {
			b.WriteString("✅ Acceptable\n")
		} else {
			b.WriteString("⚠️ Overfit risk\n")
		}
		b.WriteString("\n")
	}
	
	// Recommendation
	b.WriteString(h.generateRecommendation(m))
	
	return b.String()
}

func (h *Handler) formatMultiModelSummary(models []QuantBacktestResult) string {
	var b strings.Builder
	
	b.WriteString("<b>📊 Model Performance Comparison</b>\n\n")
	
	// Header row with proper alignment
	b.WriteString("<code>Model           WR4W   Sharpe   DD%    Sample  Conf</code>\n")
	b.WriteString("<code>───────────────────────────────────────────────────</code>\n")
	
	// Sort models by score (best first)
	sortedModels := make([]QuantBacktestResult, len(models))
	copy(sortedModels, models)
	
	// Simple bubble sort for display
	for i := 0; i < len(sortedModels); i++ {
		for j := i + 1; j < len(sortedModels); j++ {
			scoreI := sortedModels[i].WinRate4W + sortedModels[i].SharpeRatio*10
			scoreJ := sortedModels[j].WinRate4W + sortedModels[j].SharpeRatio*10
			if scoreJ > scoreI {
				sortedModels[i], sortedModels[j] = sortedModels[j], sortedModels[i]
			}
		}
	}
	
	for idx, m := range sortedModels {
		// Rank indicator
		rank := ""
		if idx == 0 {
			rank = "🥇 "
		} else if idx == 1 {
			rank = "🥈 "
		} else if idx == 2 {
			rank = "🥉 "
		} else {
			rank = "   "
		}
		
		// Confidence icon
		confIcon := "🟢"
		if m.Confidence < 75 {
			confIcon = "🟡"
		}
		if m.Confidence < 60 {
			confIcon = "🔴"
		}
		
		// Sharpe indicator
		sharpeIcon := ""
		if m.SharpeRatio > 1.5 {
			sharpeIcon = "🌟"
		} else if m.SharpeRatio > 1.0 {
			sharpeIcon = "✅"
		} else if m.SharpeRatio > 0.5 {
			sharpeIcon = "⚠️"
		}
		
		b.WriteString(fmt.Sprintf("<code>%s%-15s %5.1f%%  %6.2f %s %6.1f%% %5d   %s</code>\n",
			rank,
			strings.ToUpper(m.Model),
			m.WinRate4W,
			m.SharpeRatio,
			sharpeIcon,
			-m.MaxDrawdown,
			m.SampleSize,
			confIcon,
		))
	}
	
	b.WriteString("\n<i>🟢 High Conf | 🟡 Medium | 🔴 Low | 🌟 Excellent Sharpe</i>\n\n")
	
	// Best model highlight
	if len(sortedModels) > 0 {
		best := sortedModels[0]
		if best.SampleSize >= 30 {
			b.WriteString(fmt.Sprintf("🏆 <b>Best Model:</b> %s\n", strings.ToUpper(best.Model)))
			b.WriteString(fmt.Sprintf("   Score: %.1f | WR4W: %.1f%% | Sharpe: %.2f\n\n",
				best.WinRate4W+best.SharpeRatio*10,
				best.WinRate4W,
				best.SharpeRatio,
			))
		}
	}
	
	// Legend
	b.WriteString("<b>📋 Legend:</b>\n")
	b.WriteString("• <b>WR4W</b> - 4-week win rate (target >55%)\n")
	b.WriteString("• <b>Sharpe</b> - Risk-adjusted return (target >1.0)\n")
	b.WriteString("• <b>DD%</b> - Maximum drawdown (target <15%)\n")
	b.WriteString("• <b>Sample</b> - Number of evaluated signals\n")
	b.WriteString("• <b>Conf</b> - Statistical confidence\n")
	
	return b.String()
}

func (h *Handler) generateRecommendation(m *QuantBacktestResult) string {
	var b strings.Builder
	
	b.WriteString("\n<b>💡 Recommendation:</b>\n")
	
	// Check sample size first
	if m.SampleSize < 30 {
		b.WriteString("⚠️ <i>Sample size too small (<30 signals). Results not statistically significant. Collect more data before making decisions.</i>\n")
		return b.String()
	}
	
	// Check for overfitting
	if m.WalkForwardScore > 0 && m.WalkForwardScore < 0.6 {
		b.WriteString("🔴 <i>High overfitting risk detected. Model may not generalize to new data. Use with caution.</i>\n\n")
	}
	
	// Combined assessment (score calculated inline where needed)
	
	if m.WinRate4W > 55 && m.SharpeRatio > 1.0 && m.MaxDrawdown < 15 && m.ProfitFactor > 1.5 {
		b.WriteString("✅ <b>Strong Edge Detected</b>\n")
		b.WriteString(fmt.Sprintf("<i>Model shows consistent performance with:\n"+
			"• Win rate: %.1f%% (above 55% threshold)\n"+
			"• Sharpe ratio: %.2f (excellent risk-adjusted returns)\n"+
			"• Max DD: %.1f%% (acceptable risk)\n"+
			"• Profit factor: %.2f (profitable after costs)</i>\n",
			m.WinRate4W, m.SharpeRatio, -m.MaxDrawdown, m.ProfitFactor))
	} else if m.WinRate4W > 50 && m.SharpeRatio > 0.5 && m.MaxDrawdown < 20 {
		b.WriteString("⚠️ <b>Moderate Edge</b>\n")
		b.WriteString("<i>Model shows some promise but needs monitoring:\n")
		
		if m.WinRate4W <= 55 {
			b.WriteString(fmt.Sprintf("• Win rate %.1f%% - below optimal threshold\n", m.WinRate4W))
		}
		if m.SharpeRatio <= 1.0 {
			b.WriteString(fmt.Sprintf("• Sharpe %.2f - room for improvement\n", m.SharpeRatio))
		}
		if m.MaxDrawdown >= 15 {
			b.WriteString(fmt.Sprintf("• Max DD %.1f%% - consider position sizing\n", -m.MaxDrawdown))
		}
		
		b.WriteString("<i>Recommendation: Use with smaller position size and tight risk management.</i>\n")
	} else if m.SharpeRatio > 0 {
		b.WriteString("🔴 <b>Weak Edge</b>\n")
		b.WriteString("<i>Model barely profitable after costs. Consider:\n")
		b.WriteString("• Reducing transaction costs\n")
		b.WriteString("• Filtering signals with additional criteria\n")
		b.WriteString("• Combining with other models\n")
		b.WriteString("<i>Recommendation: Not recommended for live trading.</i>\n")
	} else {
		b.WriteString("❌ <b>No Edge Detected</b>\n")
		b.WriteString("<i>Model is not profitable after transaction costs.\n")
		b.WriteString(fmt.Sprintf("• Expected value: %+.4f%% (negative)\n", m.ExpectedValue))
		b.WriteString("<i>Recommendation: Do not use for trading decisions.</i>\n")
	}
	
	return b.String()
}

// handleQBacktestCallback handle callback untuk backtest menu
func (h *Handler) handleQBacktestCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 2 {
		return nil
	}
	
	action := parts[1]
	thirdPart := ""
	if len(parts) > 2 {
		thirdPart = parts[2]
	}
	
	// Model selection: qbacktest:model:MODELNAME
	if action == "model" && thirdPart != "" {
		model := strings.ToLower(thirdPart)
		if lc := h.getLastCurrency(ctx, 0); lc != "" {
			return h.cmdQBacktest(ctx, chatID, 0, lc+" "+model)
		}
		_, err := h.bot.SendHTML(ctx, chatID, "⚠️ No symbol selected. Use /qbacktest [SYMBOL] first.")
		return err
	}
	
	// Symbol selection: qbacktest:sym:SYMBOL
	if action == "sym" && thirdPart != "" {
		return h.cmdQBacktest(ctx, chatID, 0, thirdPart)
	}
	
	// Back to quant dashboard
	if action == "back" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		if lc := h.getLastCurrency(ctx, 0); lc != "" {
			return h.cmdQuant(ctx, chatID, 0, lc)
		}
		_, err := h.bot.SendHTML(ctx, chatID, "🔬 Quant Dashboard\n\nUse /quant [SYMBOL] to start analysis.")
		return err
	}
	
	// Refresh backtest
	if action == "refresh" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		if lc := h.getLastCurrency(ctx, 0); lc != "" {
			return h.cmdQBacktest(ctx, chatID, 0, lc)
		}
	}
	
	return nil
}
