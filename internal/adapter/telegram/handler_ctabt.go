package telegram

// handler_ctabt.go — /ctabt command: CTA Backtest dashboard
//   /ctabt [SYMBOL] [TIMEFRAME] [GRADE]  — run CTA backtest with chart + inline keyboard

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// ---------------------------------------------------------------------------
// CTABTServices — dependencies for the /ctabt command
// ---------------------------------------------------------------------------

// CTABTServices holds the services required for the CTA Backtest command.
type CTABTServices struct {
	TAEngine       *ta.Engine
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
}

// ---------------------------------------------------------------------------
// Handler wiring
// ---------------------------------------------------------------------------

// WithCTABT injects CTABTServices into the handler and registers CTABT commands.
func (h *Handler) WithCTABT(c *CTABTServices) *Handler {
	h.ctabt = c
	if c != nil {
		h.registerCTABTCommands()
	}
	return h
}

// registerCTABTCommands wires the CTABT commands into the bot.
func (h *Handler) registerCTABTCommands() {
	h.bot.RegisterCommand("/ctabt", h.cmdCTABT)
	h.bot.RegisterCallback("ctabt:", h.handleCTABTCallback)
}

// ---------------------------------------------------------------------------
// /ctabt — Main CTA Backtest Command
// ---------------------------------------------------------------------------

func (h *Handler) cmdCTABT(ctx context.Context, chatID string, _ int64, args string) error {
	if h.ctabt == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ CTA Backtest Engine not configured.")
		return err
	}

	// Parse args: [SYMBOL] [TIMEFRAME] [GRADE]
	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))

	if len(parts) == 0 {
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`📊 <b>CTA Backtest — Strategi Backtest</b>

Backtest strategi CTA di semua timeframe:

📊 <b>7 Timeframe:</b> 15m, 30m, 1h, 4h, 6h, 12h, daily
📈 <b>Grade Filter:</b> A (best), B, C (all trades)
📋 <b>Detail Trades:</b> Entry/exit/PnL setiap trade
🎯 <b>Metrics:</b> Win rate, Sharpe, drawdown, profit factor

Pilih aset:`, h.kb.CTABTSymbolMenu())
		return err
	}

	symbol := parts[0]
	timeframe := "daily"
	grade := "C"
	if len(parts) > 1 {
		timeframe = normalizeTimeframe(parts[1])
	}
	if len(parts) > 2 {
		g := parts[2]
		if g == "A" || g == "B" || g == "C" {
			grade = g
		}
	}

	return h.runCTABacktest(ctx, chatID, symbol, timeframe, grade, 0)
}

// ---------------------------------------------------------------------------
// Callback Handler
// ---------------------------------------------------------------------------

func (h *Handler) handleCTABTCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	action := strings.TrimPrefix(data, "ctabt:")

	// Symbol selection from CTABTSymbolMenu (before any other processing)
	if strings.HasPrefix(action, "sym:") {
		sym := strings.TrimPrefix(action, "sym:")
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdCTABT(ctx, chatID, 0, sym)
	}

	// Default params (symbol from last run isn't cached, use EUR)
	symbol := "EUR"
	timeframe := "daily"
	grade := "C"

	switch {
	case action == "daily":
		timeframe = "daily"
	case action == "12h":
		timeframe = "12h"
	case action == "6h":
		timeframe = "6h"
	case action == "4h":
		timeframe = "4h"
	case action == "1h":
		timeframe = "1h"
	case action == "30m":
		timeframe = "30m"
	case action == "15m":
		timeframe = "15m"
	case action == "gradeA":
		grade = "A"
	case action == "gradeB":
		grade = "B"
	case action == "gradeC":
		grade = "C"
	case action == "refresh":
		// refresh uses defaults
	case action == "trades":
		return h.showCTABTTrades(ctx, chatID, msgID, symbol, timeframe, grade)
	default:
		return nil
	}

	// Delete old message and send new one
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	return h.runCTABacktest(ctx, chatID, symbol, timeframe, grade, 0)
}

// ---------------------------------------------------------------------------
// Core backtest execution
// ---------------------------------------------------------------------------

func (h *Handler) runCTABacktest(ctx context.Context, chatID string, symbol, timeframe, grade string, editMsgID int) error {
	// Resolve symbol to contract code
	mapping := h.resolveCTAMapping(symbol)
	if mapping == nil {
		msg := fmt.Sprintf("❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/ctabt EUR</code>, <code>/ctabt XAU</code>",
			html.EscapeString(symbol))
		if editMsgID > 0 {
			return h.bot.EditMessage(ctx, chatID, editMsgID, msg)
		}
		_, err := h.bot.SendHTML(ctx, chatID, msg)
		return err
	}

	// Send loading
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf(
		"⏳ Menjalankan backtest <b>%s</b> (%s, Grade ≥ %s)...\n<i>Ini bisa memakan waktu 10-30 detik.</i>",
		html.EscapeString(mapping.Currency), timeframe, grade,
	))

	// Fetch bars
	var bars []ta.OHLCV
	code := mapping.ContractCode

	switch timeframe {
	case "daily":
		dailyRecords, err := h.ctabt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
		if err != nil || len(dailyRecords) < 50 {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			cnt := 0
			if dailyRecords != nil {
				cnt = len(dailyRecords)
			}
			_, err2 := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("❌ Data daily tidak cukup untuk %s (%d bars, minimal 65).", mapping.Currency, cnt))
			return err2
		}
		bars = ta.DailyPricesToOHLCV(dailyRecords)
	case "12h", "6h", "4h", "1h", "30m", "15m":
		if h.ctabt.IntradayRepo == nil {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			_, err := h.bot.SendHTML(ctx, chatID, "❌ Intraday data repository not configured.")
			return err
		}
		// Determine bar count based on timeframe granularity
		count := 600
		switch timeframe {
		case "15m":
			count = 5000 // ~52 days of 15m bars
		case "30m":
			count = 2500
		case "1h":
			count = 1200
		case "4h":
			count = 600
		case "6h":
			count = 400
		case "12h":
			count = 300
		}
		intradayBars, err := h.ctabt.IntradayRepo.GetHistory(ctx, code, timeframe, count)
		if err != nil || len(intradayBars) < 50 {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			cnt := 0
			if intradayBars != nil {
				cnt = len(intradayBars)
			}
			_, err2 := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("❌ Data %s tidak cukup untuk %s (%d bars, minimal 65).", timeframe, mapping.Currency, cnt))
			return err2
		}
		bars = ta.IntradayBarsToOHLCV(intradayBars)
	default:
		// fallback to daily
		timeframe = "daily"
		dailyRecords, err := h.ctabt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
		if err != nil || len(dailyRecords) < 50 {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			_, err2 := h.bot.SendHTML(ctx, chatID,
				fmt.Sprintf("❌ Data daily tidak cukup untuk %s.", mapping.Currency))
			return err2
		}
		bars = ta.DailyPricesToOHLCV(dailyRecords)
	}

	// Build params
	params := ta.DefaultBacktestParams()
	params.Symbol = mapping.Currency
	params.Timeframe = timeframe
	params.MinGrade = grade

	// Run backtest
	result := ta.RunBacktest(bars, params)
	if result == nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("❌ Data tidak cukup untuk backtest %s (%s). Minimal %d bars diperlukan.",
				mapping.Currency, timeframe, params.WarmupBars+10))
		return err
	}

	// Generate chart
	chartPNG, chartErr := h.generateBacktestChart(ctx, result, mapping.Currency, timeframe)
	if chartErr != nil {
		log.Warn().Err(chartErr).Str("symbol", symbol).Msg("backtest chart generation failed")
	}

	// Delete loading
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}

	// Format result
	summary := formatBacktestResult(result)
	kb := h.kb.CTABTMenu()

	// Send chart + caption with keyboard
	if chartPNG != nil && len(chartPNG) > 0 {
		_, err := h.bot.SendPhotoWithKeyboard(ctx, chatID, chartPNG, summary, kb)
		return err
	}

	// Chart unavailable: prepend notification so user knows chart exists but failed
	if chartErr != nil {
		summary = "📊 <i>Chart sementara tidak tersedia. Menampilkan analisis teks.</i>\n\n" + summary
	}
	_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
	return err
}

// ---------------------------------------------------------------------------
// Trade detail view
// ---------------------------------------------------------------------------

func (h *Handler) showCTABTTrades(ctx context.Context, chatID string, msgID int, symbol, timeframe, grade string) error {
	// Re-run backtest to get trades (lightweight enough since data is cached)
	mapping := h.resolveCTAMapping(symbol)
	if mapping == nil {
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Symbol not found.")
	}

	code := mapping.ContractCode
	var bars []ta.OHLCV

	switch timeframe {
	case "daily":
		records, err := h.ctabt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
		if err != nil || len(records) < 50 {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Insufficient data.")
		}
		bars = ta.DailyPricesToOHLCV(records)
	case "12h", "6h", "4h", "1h", "30m", "15m":
		if h.ctabt.IntradayRepo == nil {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Intraday not available.")
		}
		count := 600
		switch timeframe {
		case "15m":
			count = 5000
		case "30m":
			count = 2500
		case "1h":
			count = 1200
		case "4h":
			count = 600
		case "6h":
			count = 400
		case "12h":
			count = 300
		}
		intBars, err := h.ctabt.IntradayRepo.GetHistory(ctx, code, timeframe, count)
		if err != nil || len(intBars) < 50 {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Insufficient data.")
		}
		bars = ta.IntradayBarsToOHLCV(intBars)
	default:
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Invalid timeframe.")
	}

	params := ta.DefaultBacktestParams()
	params.Symbol = mapping.Currency
	params.Timeframe = timeframe
	params.MinGrade = grade

	result := ta.RunBacktest(bars, params)
	if result == nil || len(result.Trades) == 0 {
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Tidak ada trade yang dihasilkan.")
	}

	// Format last 10 trades
	txt := formatTradeList(result, mapping.Currency, timeframe)
	kb := h.kb.CTABTMenu()

	// Delete old and send new (might be too long for edit)
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
	return err
}

// ---------------------------------------------------------------------------
// Chart generation
// ---------------------------------------------------------------------------

// backtestChartInput is the JSON structure for the backtest chart Python script.
type backtestChartInput struct {
	EquityCurve []float64          `json:"equity_curve"`
	TradeDates  []string           `json:"trade_dates"`
	TradePnL    []float64          `json:"trade_pnl"`
	Drawdown    []float64          `json:"drawdown"`
	Symbol      string             `json:"symbol"`
	Timeframe   string             `json:"timeframe"`
	Params      backtestChartParams `json:"params"`
}

type backtestChartParams struct {
	StartEquity float64 `json:"start_equity"`
	TotalTrades int     `json:"total_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalReturn float64 `json:"total_return"`
	MaxDD       float64 `json:"max_dd"`
	Sharpe      float64 `json:"sharpe"`
	PF          float64 `json:"pf"`
}

func (h *Handler) generateBacktestChart(ctx context.Context, result *ta.BacktestResult, symbol, timeframe string) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("no backtest result")
	}

	// If no trades, create a flat equity line with start equity
	equityCurve := result.EquityCurve
	if len(equityCurve) < 2 {
		equityCurve = []float64{result.Params.StartEquity, result.Params.StartEquity}
	}

	// Build trade dates and PnL arrays (one entry per trade / equity point)
	tradeDates := make([]string, len(result.Trades))
	tradePnL := make([]float64, len(result.Trades))
	for i, t := range result.Trades {
		tradeDates[i] = t.ExitDate.Format("2006-01-02")
		tradePnL[i] = t.PnLPercent
	}

	// Compute drawdown from equity curve
	drawdown := make([]float64, len(equityCurve))
	peak := equityCurve[0]
	for i, eq := range equityCurve {
		if eq > peak {
			peak = eq
		}
		if peak > 0 {
			drawdown[i] = (eq - peak) / peak * 100.0
		}
	}

	input := backtestChartInput{
		EquityCurve: equityCurve,
		TradeDates:  tradeDates,
		TradePnL:    tradePnL,
		Drawdown:    drawdown,
		Symbol:      symbol,
		Timeframe:   timeframe,
		Params: backtestChartParams{
			StartEquity: result.Params.StartEquity,
			TotalTrades: result.TotalTrades,
			WinRate:     result.WinRate,
			TotalReturn: result.TotalPnLPercent,
			MaxDD:       -result.MaxDrawdown,
			Sharpe:      result.SharpeRatio,
			PF:          result.ProfitFactor,
		},
	}

	// Sanitize NaN/Inf for JSON marshaling
	sanitizeBTFloat := func(v float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return v
	}
	for i := range input.EquityCurve {
		input.EquityCurve[i] = sanitizeBTFloat(input.EquityCurve[i])
	}
	for i := range input.TradePnL {
		input.TradePnL[i] = sanitizeBTFloat(input.TradePnL[i])
	}
	for i := range input.Drawdown {
		input.Drawdown[i] = sanitizeBTFloat(input.Drawdown[i])
	}
	input.Params.WinRate = sanitizeBTFloat(input.Params.WinRate)
	input.Params.TotalReturn = sanitizeBTFloat(input.Params.TotalReturn)
	input.Params.MaxDD = sanitizeBTFloat(input.Params.MaxDD)
	input.Params.Sharpe = sanitizeBTFloat(input.Params.Sharpe)
	input.Params.PF = sanitizeBTFloat(input.Params.PF)

	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal chart input: %w", err)
	}

	// Write temp files
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("ctabt_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("ctabt_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write chart input: %w", err)
	}
	defer os.Remove(inputPath)

	scriptPath := findBacktestScript()

	cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(outputPath)
		return nil, fmt.Errorf("backtest chart renderer failed (timeout 90s): %w", err)
	}

	pngData, err := os.ReadFile(outputPath)
	os.Remove(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read chart output: %w", err)
	}

	return pngData, nil
}

// findBacktestScript locates the backtest_chart.py script.
func findBacktestScript() string {
	candidates := []string{
		"scripts/backtest_chart.py",
		"../scripts/backtest_chart.py",
		"/home/mulerun/.openclaw/workspace/ark-intelligent/scripts/backtest_chart.py",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		rel := filepath.Join(execDir, "scripts", "backtest_chart.py")
		if _, err := os.Stat(rel); err == nil {
			return rel
		}
		rel = filepath.Join(execDir, "..", "scripts", "backtest_chart.py")
		if _, err := os.Stat(rel); err == nil {
			abs, _ := filepath.Abs(rel)
			return abs
		}
	}
	return "scripts/backtest_chart.py"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// normalizeTimeframe normalizes user input to a recognized timeframe key.
func normalizeTimeframe(tf string) string {
	switch strings.ToLower(tf) {
	case "daily", "d", "1d", "day":
		return "daily"
	case "12h", "h12":
		return "12h"
	case "6h", "h6":
		return "6h"
	case "4h", "h4":
		return "4h"
	case "1h", "h1":
		return "1h"
	case "30m", "m30":
		return "30m"
	case "15m", "m15":
		return "15m"
	default:
		return "daily"
	}
}



// ---------------------------------------------------------------------------
// Formatters — Indonesian language
// ---------------------------------------------------------------------------

func formatBacktestResult(bt *ta.BacktestResult) string {
	if bt == nil {
		return "❌ Tidak ada hasil backtest."
	}

	var sb strings.Builder

	// Header
	tfLabel := strings.ToUpper(bt.Params.Timeframe)
	sb.WriteString(fmt.Sprintf("📊 <b>Hasil Backtest: %s (%s)</b>\n", html.EscapeString(bt.Params.Symbol), tfLabel))

	startStr := bt.StartDate.Format("02 Jan 2006")
	endStr := bt.EndDate.Format("02 Jan 2006")
	sb.WriteString(fmt.Sprintf("📅 %s — %s | %d trade\n\n", startStr, endStr, bt.TotalTrades))

	if bt.TotalTrades == 0 {
		sb.WriteString("⚠️ Tidak ada trade yang memenuhi kriteria selama periode ini.\n")
		sb.WriteString(fmt.Sprintf("Grade minimum: %s | R:R minimum: %.1f\n", bt.Params.MinGrade, bt.Params.MinRR))
		return sb.String()
	}

	// Performance summary
	wins := int(math.Round(bt.WinRate / 100.0 * float64(bt.TotalTrades)))
	losses := bt.TotalTrades - wins

	sb.WriteString("━━━━ RINGKASAN PERFORMA ━━━━\n")
	sb.WriteString(fmt.Sprintf("✅ Win Rate: %.1f%% (%d menang / %d kalah)\n", bt.WinRate, wins, losses))
	sb.WriteString(fmt.Sprintf("  → Artinya: dari 10 sinyal, rata-rata %.0f berhasil profit\n\n", bt.WinRate/10.0))

	returnSign := "+"
	if bt.TotalPnLPercent < 0 {
		returnSign = ""
	}
	sb.WriteString(fmt.Sprintf("💰 Total Return: %s%.1f%% ($%s dari $%s)\n",
		returnSign, bt.TotalPnLPercent,
		formatMoney(bt.TotalPnLDollar), formatMoney(bt.Params.StartEquity)))
	sb.WriteString(fmt.Sprintf("📉 Max Drawdown: -%.1f%% (penurunan modal terbesar)\n", bt.MaxDrawdown))

	pfStr := fmt.Sprintf("%.1f×", bt.ProfitFactor)
	if bt.ProfitFactor >= 999 {
		pfStr = "∞"
	}
	sb.WriteString(fmt.Sprintf("⚡ Profit Factor: %s (setiap $1 rugi → $%s untung)\n", pfStr, pfStr))
	sb.WriteString(fmt.Sprintf("📊 Sharpe Ratio: %.2f (>1 = baik, >2 = sangat baik)\n", bt.SharpeRatio))
	sb.WriteString(fmt.Sprintf("🎯 Expected Value: %+.1f%% per trade\n\n", bt.ExpectedValue))

	// Detail stats
	sb.WriteString("━━━━ STATISTIK DETAIL ━━━━\n")
	sb.WriteString(fmt.Sprintf("Avg Win: +%.1f%% | Avg Loss: -%.1f%%\n", bt.AvgWin, bt.AvgLoss))
	sb.WriteString(fmt.Sprintf("Best Trade: %+.1f%% | Worst: %+.1f%%\n", bt.BestTrade, bt.WorstTrade))
	sb.WriteString(fmt.Sprintf("Max Streak: %d wins / %d losses berturut\n\n", bt.ConsecWins, bt.ConsecLosses))

	// Rating
	sb.WriteString("━━━━ PENILAIAN ━━━━\n")
	ratingEmoji, ratingGrade, ratingDesc := rateStrategy(bt)
	sb.WriteString(fmt.Sprintf("%s Grade: %s — %s\n", ratingEmoji, ratingGrade, ratingDesc))

	return sb.String()
}

func formatTradeList(bt *ta.BacktestResult, symbol, timeframe string) string {
	if bt == nil || len(bt.Trades) == 0 {
		return "❌ Tidak ada trade."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 <b>Detail Trade: %s (%s)</b>\n",
		html.EscapeString(symbol), strings.ToUpper(timeframe)))
	sb.WriteString(fmt.Sprintf("Menampilkan %d trade terakhir\n\n", min(10, len(bt.Trades))))

	// Show last 10 trades
	start := len(bt.Trades) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(bt.Trades); i++ {
		t := bt.Trades[i]
		dirEmoji := "🟢"
		if t.Direction == "SHORT" {
			dirEmoji = "🔴"
		}
		resultEmoji := "✅"
		if t.PnLDollar <= 0 {
			resultEmoji = "❌"
		}

		sb.WriteString(fmt.Sprintf("%s <b>#%d %s</b> (Grade %s)\n", dirEmoji, i+1, t.Direction, t.Grade))
		sb.WriteString(fmt.Sprintf("  Entry: %.4f → Exit: %.4f\n", t.EntryPrice, t.ExitPrice))
		sb.WriteString(fmt.Sprintf("  SL: %.4f | TP: %.4f | R:R: %.1f\n", t.StopLoss, t.TakeProfit, t.RR))
		sb.WriteString(fmt.Sprintf("  %s P&L: %+.1f%% ($%+.0f) — %s\n",
			resultEmoji, t.PnLPercent, t.PnLDollar, t.ExitReason))
		sb.WriteString(fmt.Sprintf("  📅 %s → %s\n\n",
			t.EntryDate.Format("02 Jan"), t.ExitDate.Format("02 Jan 2006")))
	}

	return sb.String()
}

// rateStrategy returns emoji, grade, and description based on strategy performance.
func rateStrategy(bt *ta.BacktestResult) (emoji, grade, desc string) {
	wr := bt.WinRate
	pf := bt.ProfitFactor
	sr := bt.SharpeRatio

	if sr > 2 && pf > 2.5 && wr > 60 {
		return "⭐⭐⭐⭐⭐", "S", "Sangat Luar Biasa — strategi ini konsisten dan highly profitable."
	}
	if sr > 1.5 && pf > 2 && wr > 55 {
		return "⭐⭐⭐⭐", "A", "Sangat Baik — strategi ini profitable dengan risk management solid."
	}
	if sr > 1 && pf > 1.5 && wr > 50 {
		return "⭐⭐⭐", "B", "Baik — strategi ini profitable tapi perlu risk management ketat."
	}
	if pf > 1 {
		return "⭐⭐", "C", "Cukup — masih profitable tapi margin tipis."
	}
	return "⭐", "D", "Buruk — strategi merugi di periode ini. Evaluasi parameter."
}

// formatMoney formats a float as a human-readable money string.
func formatMoney(v float64) string {
	if v < 0 {
		return fmt.Sprintf("-%.0f", -v)
	}
	return fmt.Sprintf("%.0f", v)
}

// Unused import guard
var _ = math.Abs
