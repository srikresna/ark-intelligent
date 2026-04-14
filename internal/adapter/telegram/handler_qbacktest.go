package telegram

// handler_qbacktest.go — /qbacktest command: Quant Backtest Dashboard
// Flow (like /ctabt):
//   1. /qbacktest           → symbol selector
//   2. qbacktest:sym:EUR    → model menu with TF buttons
//   3. qbacktest:model:stats → run backtest
//   4. qbacktest:tf:4h      → change TF, re-run last model

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// State cache
// ---------------------------------------------------------------------------

type qbacktestState struct {
	symbol    string
	timeframe string
	model     string
	createdAt time.Time
}

type qbacktestStateCache struct {
	mu    sync.Mutex
	store map[string]*qbacktestState
}

func newQBacktestStateCache() *qbacktestStateCache {
	return &qbacktestStateCache{store: make(map[string]*qbacktestState)}
}

const qbacktestStateTTL = 30 * time.Minute

func (c *qbacktestStateCache) get(chatID string) *qbacktestState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.createdAt) > qbacktestStateTTL {
		delete(c.store, chatID)
		return nil
	}
	return s
}

func (c *qbacktestStateCache) set(chatID string, s *qbacktestState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.store {
		if now.Sub(v.createdAt) > qbacktestStateTTL*2 {
			delete(c.store, k)
		}
	}
	c.store[chatID] = s
}

// ---------------------------------------------------------------------------
// Handler registration
// ---------------------------------------------------------------------------

func (h *Handler) registerQuantBacktestCommands() {
	h.qbacktestCache = newQBacktestStateCache()
	h.bot.RegisterCommand("/qbacktest", h.cmdQBacktest)
	h.bot.RegisterCommand("/quantbacktest", h.cmdQBacktest)
	h.bot.RegisterCallback("qbacktest:", h.handleQBacktestCallback)
}

// ---------------------------------------------------------------------------
// /qbacktest command
// ---------------------------------------------------------------------------

func (h *Handler) cmdQBacktest(ctx context.Context, chatID string, userID int64, args string) error {
	_ = h.bot.SendChatAction(ctx, chatID, "typing")
	if h.quant == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ <b>Quant Engine not configured.</b>")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) == 0 {
		return h.showQBTDashboard(ctx, chatID, userID)
	}

	symbol := parts[0]
	model := ""
	timeframe := "daily"
	if len(parts) > 1 {
		model = strings.ToLower(parts[1])
	}
	if len(parts) > 2 {
		timeframe = strings.ToLower(parts[2])
	}

	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/qbacktest EUR stats daily</code>",
			html.EscapeString(symbol),
		))
		return err
	}

	st := &qbacktestState{
		symbol:    mapping.Currency,
		timeframe: timeframe,
		model:     model,
		createdAt: time.Now(),
	}
	h.qbacktestCache.set(chatID, st)
	h.saveLastCurrency(ctx, userID, mapping.Currency)

	if model == "" {
		return h.showQBTModelMenu(ctx, chatID, st)
	}
	return h.runQBacktest(ctx, chatID, st)
}

// ---------------------------------------------------------------------------
// Callback handler
// ---------------------------------------------------------------------------

func (h *Handler) handleQBacktestCallback(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "qbacktest:")
	parts := strings.SplitN(action, ":", 2)
	cmd := parts[0]
	param := ""
	if len(parts) > 1 {
		param = parts[1]
	}

	// Symbol selection
	if cmd == "sym" && param != "" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdQBacktest(ctx, chatID, userID, param)
	}

	state := h.qbacktestCache.get(chatID)

	// Timeframe change
	if cmd == "tf" && param != "" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		if state == nil {
			return h.showQBTDashboard(ctx, chatID, userID)
		}
		state.timeframe = strings.ToLower(param)
		state.createdAt = time.Now()
		h.qbacktestCache.set(chatID, state)
		if state.model != "" {
			return h.runQBacktest(ctx, chatID, state)
		}
		return h.showQBTModelMenu(ctx, chatID, state)
	}

	// Model selection
	if cmd == "model" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		if state == nil {
			return h.showQBTDashboard(ctx, chatID, userID)
		}
		if param == "" {
			// Show model description
			_, _ = h.bot.SendHTML(ctx, chatID,
				"📋 <b>Model Quant Backtest</b>\n\n"+
					"• <b>Stats</b> — Return percentile momentum\n"+
					"• <b>GARCH</b> — Volatility regime breakout\n"+
					"• <b>Correlation</b> — Multi-TF momentum alignment\n"+
					"• <b>Regime</b> — 50/200 SMA golden/death cross\n"+
					"• <b>Seasonal</b> — Day-of-week pattern\n"+
					"• <b>Mean Revert</b> — Bollinger Band z-score fade\n"+
					"• <b>Granger</b> — ROC percentile momentum\n"+
					"• <b>Cointegration</b> — Long-term mean reversion 60-bar\n"+
					"• <b>PCA</b> — Multi-factor composite signal\n"+
					"• <b>VaR</b> — Tail-risk regime signal\n"+
					"• <b>Risk</b> — Sortino ratio signal")
			return h.showQBTModelMenu(ctx, chatID, state)
		}
		state.model = strings.ToLower(param)
		state.createdAt = time.Now()
		h.qbacktestCache.set(chatID, state)
		return h.runQBacktest(ctx, chatID, state)
	}

	// Back to model menu
	if cmd == "back" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		if state == nil {
			return h.showQBTDashboard(ctx, chatID, userID)
		}
		return h.showQBTModelMenu(ctx, chatID, state)
	}

	// Refresh last model
	if cmd == "refresh" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		if state == nil || state.model == "" {
			return h.showQBTDashboard(ctx, chatID, userID)
		}
		return h.runQBacktest(ctx, chatID, state)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Dashboard (symbol selector)
// ---------------------------------------------------------------------------

func (h *Handler) showQBTDashboard(ctx context.Context, chatID string, userID int64) error {
	if lc := h.getLastCurrency(ctx, userID); lc != "" {
		return h.cmdQBacktest(ctx, chatID, userID, lc)
	}
	_, err := h.bot.SendWithKeyboard(ctx, chatID,
		`🔬 <b>QUANT BACKTEST DASHBOARD</b>

Backtest 11 model ekonometrik pada data historis.
Setiap model menggunakan strategi berbeda:

<b>Foundation:</b> Stats · GARCH · Correlation
<b>Time Series:</b> Regime · Mean Revert · Granger
<b>Advanced:</b> Cointegration · PCA · VaR · Risk · Seasonal

<b>Timeframe:</b> 15m, 30m, 1h, 4h, 6h, 12h, daily

Pilih aset:`, h.kb.QuantSymbolMenu())
	return err
}

// ---------------------------------------------------------------------------
// Model menu
// ---------------------------------------------------------------------------

func (h *Handler) showQBTModelMenu(ctx context.Context, chatID string, st *qbacktestState) error {
	sym := html.EscapeString(st.symbol)
	tf := strings.ToUpper(st.timeframe)
	text := fmt.Sprintf(`🔬 <b>QUANT BACKTEST: %s</b>
📅 Timeframe: <b>%s</b>

Pilih model analisis di bawah.
<i>Tip: Coba beberapa model untuk membandingkan edge yang berbeda.</i>`, sym, tf)

	kb := h.kb.QBacktestMenuWithTF(st.symbol, st.timeframe)
	_, err := h.bot.SendWithKeyboard(ctx, chatID, text, kb)
	return err
}

// ---------------------------------------------------------------------------
// Run backtest
// ---------------------------------------------------------------------------

func (h *Handler) runQBacktest(ctx context.Context, chatID string, st *qbacktestState) error {
	sym := html.EscapeString(st.symbol)
	modelStr := strings.ToUpper(st.model)

	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf(
		"🔬 Running <b>%s</b> backtest for <b>%s</b> (%s)...",
		modelStr, sym, st.timeframe,
	))

	analyzer := NewSimpleQuantBacktestAnalyzer(h.quant)
	result, err := analyzer.Analyze(ctx, st.symbol, st.model, st.timeframe)

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}

	if err != nil {
		h.sendUserError(ctx, chatID, err, "qbacktest")
		return nil
	}

	htmlOut := h.formatQBTResult(result)
	kb := h.kb.QBacktestResultMenu(st.symbol, st.timeframe, st.model)
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, htmlOut, kb)
	return err
}

// ---------------------------------------------------------------------------
// Result formatter
// ---------------------------------------------------------------------------

func (h *Handler) formatQBTResult(r *SimpleQuantBacktestResult) string {
	var b strings.Builder

	// ── Header ──
	b.WriteString(fmt.Sprintf("📊 <b>QUANT BACKTEST: %s — %s</b>\n", html.EscapeString(r.Symbol), html.EscapeString(r.Model)))
	b.WriteString(fmt.Sprintf("<i>Timeframe: %s | %d bars historis</i>\n", r.Timeframe, r.TotalBars))

	// ── Signal Logic (transparansi decision making) ──
	if r.SignalLogic != "" {
		b.WriteString(fmt.Sprintf("\n🔬 <b>%s</b>\n", html.EscapeString(r.SignalLogic)))
		b.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(r.Criteria)))
	}

	b.WriteString("\n━━━━━━━━━━━━━━━\n")

	// ── Signal Summary ──
	b.WriteString("📋 <b>Signal Summary</b>\n")
	b.WriteString(fmt.Sprintf("Total Signals : %d\n", r.SignalCount))
	b.WriteString(fmt.Sprintf("  📈 LONG     : %d\n", r.LongCount))
	b.WriteString(fmt.Sprintf("  📉 SHORT    : %d\n", r.ShortCount))
	b.WriteString(fmt.Sprintf("Confidence    : %.0f%%\n", r.Confidence))

	// ── Win Rate Breakdown ──
	b.WriteString("\n🎯 <b>Win Rate Breakdown</b>\n")
	b.WriteString(fmt.Sprintf("Overall  : %s\n", h.qbtWinRateStr(r.WinRate)))
	if r.LongCount > 0 {
		b.WriteString(fmt.Sprintf("LONG     : %.1f%%  (%d/%d)\n", r.LongWinRate, r.LongWins, r.LongCount))
	}
	if r.ShortCount > 0 {
		b.WriteString(fmt.Sprintf("SHORT    : %.1f%%  (%d/%d)\n", r.ShortWinRate, r.ShortWins, r.ShortCount))
	}

	// ── Return Analysis ──
	b.WriteString("\n💰 <b>Return Analysis</b>\n")
	b.WriteString(fmt.Sprintf("Avg Return   : %+.4f%%\n", r.AvgReturn))
	b.WriteString(fmt.Sprintf("Avg Win      : %+.4f%%\n", r.AvgWinReturn))
	b.WriteString(fmt.Sprintf("Avg Loss     : %+.4f%%\n", r.AvgLossReturn))
	b.WriteString(fmt.Sprintf("Best Trade   : %+.4f%%\n", r.BestTrade))
	b.WriteString(fmt.Sprintf("Worst Trade  : %+.4f%%\n", r.WorstTrade))

	// ── Risk Metrics ──
	b.WriteString("\n⚠️ <b>Risk Metrics</b>\n")
	b.WriteString(fmt.Sprintf("Sharpe Ratio  : %s\n", h.qbtSharpeStr(r.Sharpe)))
	b.WriteString(fmt.Sprintf("Profit Factor : %.2f  %s\n", r.ProfitFactor, h.qbtPFEmoji(r.ProfitFactor)))
	b.WriteString(fmt.Sprintf("Max Drawdown  : %.2f%%  %s\n", -r.MaxDD, h.qbtDDEmoji(r.MaxDD)))

	b.WriteString("\n" + h.qbtRecommendation(r))
	return b.String()
}

func (h *Handler) qbtWinRateStr(wr float64) string {
	emoji := "❌"
	if wr >= 55 {
		emoji = "✅"
	} else if wr >= 50 {
		emoji = "⚠️"
	}
	return fmt.Sprintf("%.1f%%  %s", wr, emoji)
}

func (h *Handler) qbtSharpeStr(s float64) string {
	emoji := "❌"
	if s >= 1.0 {
		emoji = "✅"
	} else if s >= 0.5 {
		emoji = "⚠️"
	}
	return fmt.Sprintf("%.2f  %s", s, emoji)
}

func (h *Handler) qbtDDEmoji(dd float64) string {
	if dd > -10 {
		return "✅"
	} else if dd > -20 {
		return "⚠️"
	}
	return "❌"
}

func (h *Handler) qbtPFEmoji(pf float64) string {
	if pf >= 1.5 {
		return "✅"
	} else if pf >= 1.0 {
		return "⚠️"
	}
	return "❌"
}

func (h *Handler) qbtRecommendation(r *SimpleQuantBacktestResult) string {
	var b strings.Builder
	b.WriteString("💡 <b>Recommendation:</b>\n")
	if r.SignalCount < 20 {
		b.WriteString("⚠️ <i>Sample size too small. Collect more data.</i>")
		return b.String()
	}
	if r.WinRate >= 55 && r.Sharpe >= 1.0 && r.MaxDD > -15 {
		b.WriteString("✅ <b>Strong Edge</b>\n")
		b.WriteString(fmt.Sprintf("<i>Win %.1f%%, Sharpe %.2f — consider live with proper sizing.</i>", r.WinRate, r.Sharpe))
	} else if r.WinRate >= 50 && r.Sharpe >= 0.5 {
		b.WriteString("⚠️ <b>Moderate Edge</b>\n")
		b.WriteString("<i>Promising, needs monitoring. Use small position size.</i>")
	} else if r.Sharpe > 0 {
		b.WriteString("🔴 <b>Weak Edge</b>\n")
		b.WriteString("<i>Barely profitable. Not recommended for live trading.</i>")
	} else {
		b.WriteString("❌ <b>No Edge Detected</b>\n")
		b.WriteString(fmt.Sprintf("<i>Negative expected value: %+.4f%%. Do not trade.</i>", r.AvgReturn))
	}
	return b.String()
}

// showBacktestDashboard is an alias called from handler_quant.go (quant:backtest callback).
func (h *Handler) showBacktestDashboard(ctx context.Context, chatID string, userID int64) error {
	return h.showQBTDashboard(ctx, chatID, userID)
}
