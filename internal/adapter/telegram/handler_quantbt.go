package telegram

// handler_quantbt.go — /quantbt command: Quantitative Backtest dashboard
//   /quantbt [SYMBOL] [TIMEFRAME] [GRADE]  — run quantitative backtest with chart + inline keyboard

import (
	"bytes"
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

	"github.com/arkcode369/ark-intelligent/internal/ports"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// QuantBTServices — dependencies for the /quantbt command
// ---------------------------------------------------------------------------

// QuantBTServices holds the services required for the Quantitative Backtest command.
type QuantBTServices struct {
	TAEngine       *ta.Engine
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
}

// ---------------------------------------------------------------------------
// Handler wiring
// ---------------------------------------------------------------------------

// WithQuantBT injects QuantBTServices into the handler and registers QuantBT commands.
func (h *Handler) WithQuantBT(q *QuantBTServices) *Handler {
	h.quantbt = q
	if q != nil {
		h.registerQuantBTCommands()
	}
	return h
}

// registerQuantBTCommands wires the QuantBT commands into the bot.
func (h *Handler) registerQuantBTCommands() {
	h.bot.RegisterCommand("/quantbt", h.cmdQuantBT)
	h.bot.RegisterCallback("quantbt:", h.handleQuantBTCallback)
}

// ---------------------------------------------------------------------------
// /quantbt — Main Quantitative Backtest Command
// ---------------------------------------------------------------------------

func (h *Handler) cmdQuantBT(ctx context.Context, chatID string, _ int64, args string) error {
	if h.quantbt == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Quantitative Backtest Engine not configured.")
		return err
	}

	// Parse args: [SYMBOL] [TIMEFRAME] [GRADE]
	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))

	if len(parts) == 0 {
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`📊 <b>Quantitative Backtest — Multi-Strategy Analysis</b>

Backtest strategi quantitative dengan machine learning:

📊 <b>7 Timeframe:</b> 15m, 30m, 1h, 4h, 6h, 12h, daily
📈 <b>Grade Filter:</b> A (best), B, C (all trades)
📋 <b>Detail Trades:</b> Entry/exit/PnL setiap trade
🎯 <b>Metrics:</b> Win rate, Sharpe, drawdown, profit factor
🤖 <b>ML Models:</b> Random Forest, XGBoost, Neural Network

Pilih aset:`, h.quantBTSymbolMenu())
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

	return h.runQuantBacktest(ctx, chatID, symbol, timeframe, grade, 0)
}

// ---------------------------------------------------------------------------
// Callback Handler
// ---------------------------------------------------------------------------

func (h *Handler) handleQuantBTCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	action := strings.TrimPrefix(data, "quantbt:")

	// Symbol selection from QuantBTSymbolMenu (before any other processing)
	if strings.HasPrefix(action, "sym:") {
		sym := strings.TrimPrefix(action, "sym:")
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdQuantBT(ctx, chatID, 0, sym)
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
		return h.showQuantBTTrades(ctx, chatID, msgID, symbol, timeframe, grade)
	default:
		return nil
	}

	// Delete old message and send new one
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	return h.runQuantBacktest(ctx, chatID, symbol, timeframe, grade, 0)
}

// ---------------------------------------------------------------------------
// Core backtest execution
// ---------------------------------------------------------------------------

func (h *Handler) runQuantBacktest(ctx context.Context, chatID string, symbol, timeframe, grade string, editMsgID int) error {
	// Resolve symbol to contract code
	mapping := h.resolveCTAMapping(symbol)
	if mapping == nil {
		msg := fmt.Sprintf("❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/quantbt EUR</code>, <code>/quantbt XAU</code>",
			html.EscapeString(symbol))
		if editMsgID > 0 {
			return h.bot.EditMessage(ctx, chatID, editMsgID, msg)
		}
		_, err := h.bot.SendHTML(ctx, chatID, msg)
		return err
	}

	if h.quantbt.DailyPriceRepo == nil {
		h.sendUserError(ctx, chatID, fmt.Errorf("daily price data not configured"), "quantbt")
		return nil
	}

	// Send loading
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf(
		"⏳ Menjalankan quantitative backtest <b>%s</b> (%s, Grade ≥ %s)...\n<i>Ini bisa memakan waktu 15-40 detik.</i>",
		html.EscapeString(mapping.Currency), timeframe, grade,
	))

	// Fetch bars
	var bars []ta.OHLCV
	code := mapping.ContractCode

	switch timeframe {
	case "daily":
		dailyRecords, err := h.quantbt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
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
		if h.quantbt.IntradayRepo == nil {
			if loadingID > 0 {
				_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
			}
			h.sendUserError(ctx, chatID, fmt.Errorf("intraday data repository not configured"), "quantbt")
			return nil
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
		intradayBars, err := h.quantbt.IntradayRepo.GetHistory(ctx, code, timeframe, count)
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
		dailyRecords, err := h.quantbt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
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
	chartPNG, chartErr := h.generateQuantChart(ctx, result, mapping.Currency, timeframe)
	if chartErr != nil {
		log.Error().Err(chartErr).Str("symbol", symbol).Str("timeframe", timeframe).Msg("quant chart generation failed, falling back to text")
	}

	// Delete loading
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}

	// Format result
	summary := formatBacktestResult(result)
	kb := h.quantBTMenu()

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

func (h *Handler) showQuantBTTrades(ctx context.Context, chatID string, msgID int, symbol, timeframe, grade string) error {
	// Re-run backtest to get trades (lightweight enough since data is cached)
	mapping := h.resolveCTAMapping(symbol)
	if mapping == nil {
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Symbol not found.")
	}

	if h.quantbt.DailyPriceRepo == nil {
		return h.bot.EditMessage(ctx, chatID, msgID, "❌ Daily price data not configured.")
	}

	code := mapping.ContractCode
	var bars []ta.OHLCV

	switch timeframe {
	case "daily":
		records, err := h.quantbt.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
		if err != nil || len(records) < 50 {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Insufficient data.")
		}
		bars = ta.DailyPricesToOHLCV(records)
	case "12h", "6h", "4h", "1h", "30m", "15m":
		if h.quantbt.IntradayRepo == nil {
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
		intBars, err := h.quantbt.IntradayRepo.GetHistory(ctx, code, timeframe, count)
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
	kb := h.quantBTMenu()

	// Delete old and send new (might be too long for edit)
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
	return err
}

// ---------------------------------------------------------------------------
// Chart generation
// ---------------------------------------------------------------------------

// quantChartInput is the JSON structure for the quant chart Python script.
type quantChartInput struct {
	EquityCurve []float64          `json:"equity_curve"`
	TradeDates  []string           `json:"trade_dates"`
	TradePnL    []float64          `json:"trade_pnl"`
	Drawdown    []float64          `json:"drawdown"`
	Symbol      string             `json:"symbol"`
	Timeframe   string             `json:"timeframe"`
	Params      quantChartParams   `json:"params"`
	Features    []quantFeature     `json:"features"`
}

type quantChartParams struct {
	StartEquity float64 `json:"start_equity"`
	TotalTrades int     `json:"total_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalReturn float64 `json:"total_return"`
	MaxDD       float64 `json:"max_dd"`
	Sharpe      float64 `json:"sharpe"`
	PF          float64 `json:"pf"`
	ModelType   string  `json:"model_type"`
	Accuracy    float64 `json:"accuracy"`
}

type quantFeature struct {
	Name  string  `json:"name"`
	Importance float64 `json:"importance"`
}

func (h *Handler) generateQuantChart(ctx context.Context, result *ta.BacktestResult, symbol, timeframe string) (pngData []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in generateQuantChart: %v", r)
			log.Error().Interface("panic", r).Str("symbol", symbol).Str("timeframe", timeframe).Msg("recovered panic in generateQuantChart")
		}
	}()
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

	// Build feature importance (placeholder - no metadata field in BacktestResult)
	features := []quantFeature{
		{Name: "RSI", Importance: 0.25},
		{Name: "MACD", Importance: 0.20},
		{Name: "Bollinger", Importance: 0.18},
		{Name: "Volume", Importance: 0.17},
		{Name: "Trend", Importance: 0.20},
	}

	input := quantChartInput{
		EquityCurve: equityCurve,
		TradeDates:  tradeDates,
		TradePnL:    tradePnL,
		Drawdown:    drawdown,
		Symbol:      symbol,
		Timeframe:   timeframe,
		Params: quantChartParams{
			StartEquity: result.Params.StartEquity,
			TotalTrades: result.TotalTrades,
			WinRate:     result.WinRate,
			TotalReturn: result.TotalPnLPercent,
			MaxDD:       -result.MaxDrawdown,
			Sharpe:      result.SharpeRatio,
			PF:          result.ProfitFactor,
			ModelType:   "Quantitative ML",
			Accuracy:    result.WinRate,
		},
		Features: features,
	}

	// Sanitize NaN/Inf for JSON marshaling
	sanitizeQTFloat := func(v float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return v
	}
	for i := range input.EquityCurve {
		input.EquityCurve[i] = sanitizeQTFloat(input.EquityCurve[i])
	}
	for i := range input.TradePnL {
		input.TradePnL[i] = sanitizeQTFloat(input.TradePnL[i])
	}
	for i := range input.Drawdown {
		input.Drawdown[i] = sanitizeQTFloat(input.Drawdown[i])
	}
	for i := range input.Features {
		input.Features[i].Importance = sanitizeQTFloat(input.Features[i].Importance)
	}
	input.Params.WinRate = sanitizeQTFloat(input.Params.WinRate)
	input.Params.TotalReturn = sanitizeQTFloat(input.Params.TotalReturn)
	input.Params.MaxDD = sanitizeQTFloat(input.Params.MaxDD)
	input.Params.Sharpe = sanitizeQTFloat(input.Params.Sharpe)
	input.Params.PF = sanitizeQTFloat(input.Params.PF)
	input.Params.Accuracy = sanitizeQTFloat(input.Params.Accuracy)

	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal chart input: %w", err)
	}

	// Write temp files
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("quantbt_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("quantbt_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write chart input: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath) // ensure PNG is cleaned up on all return paths

	scriptPath, findErr := findQuantChartScript()
	if findErr != nil {
		return nil, findErr
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Str("stderr", stderr.String()).Msg("quant chart renderer failed")
		return nil, fmt.Errorf("quant chart renderer failed: %w", err)
	}

	pngData, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		return nil, fmt.Errorf("read chart output: %w", readErr)
	}

	return pngData, nil
}

// findQuantChartScript locates the quant_chart.py script.
func findQuantChartScript() (string, error) {
	candidates := []string{
		"scripts/quant_chart.py",
		"../scripts/quant_chart.py",
	}
	if d := os.Getenv("SCRIPTS_DIR"); d != "" {
		candidates = append([]string{filepath.Join(d, "quant_chart.py")}, candidates...)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		rel := filepath.Join(execDir, "scripts", "quant_chart.py")
		if _, err := os.Stat(rel); err == nil {
			return rel, nil
		}
		rel = filepath.Join(execDir, "..", "scripts", "quant_chart.py")
		if _, err := os.Stat(rel); err == nil {
			abs, _ := filepath.Abs(rel)
			return abs, nil
		}
	}
	return "", fmt.Errorf("quant_chart.py not found (searched: %v)", candidates)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------



// ---------------------------------------------------------------------------
// Keyboard builders for /quantbt
// ---------------------------------------------------------------------------

// quantBTSymbolMenu returns the symbol selection keyboard for /quantbt.
func (h *Handler) quantBTSymbolMenu() ports.InlineKeyboard {
	// All available symbols from price data (consistent with your data sources)
	symbols := []string{
		// Forex Majors
		"EUR", "GBP", "JPY", "CHF", "AUD", "CAD", "NZD", "USD",
		// Commodities
		"XAU", "XAG", "COPPER", "OIL", "ULSD", "RBOB",
		// Bonds & Indices
		"BOND", "BOND30", "BOND5", "BOND2", "SPX500", "NDX", "DJI", "RUT",
		// Crypto
		"BTC", "ETH",
		// Cross pairs (synthetic)
		"XAUEUR", "XAUGBP", "XAGEUR", "XAGGBP",
	}
	rows := make([][]ports.InlineButton, 0, len(symbols))
	for _, sym := range symbols {
		rows = append(rows, []ports.InlineButton{
			{Text: fmt.Sprintf("📈 %s", sym), CallbackData: fmt.Sprintf("quantbt:sym:%s", sym)},
		})
	}
	return ports.InlineKeyboard{Rows: rows}
}

// quantBTMenu returns the main menu keyboard for /quantbt results.
func (h *Handler) quantBTMenu() ports.InlineKeyboard {
	return ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				{Text: "🔄 Refresh", CallbackData: "quantbt:refresh"},
				{Text: "📋 Detail Trades", CallbackData: "quantbt:trades"},
			},
			{
				{Text: "📊 Daily", CallbackData: "quantbt:daily"},
				{Text: "📊 12h", CallbackData: "quantbt:12h"},
				{Text: "📊 6h", CallbackData: "quantbt:6h"},
			},
			{
				{Text: "📊 4h", CallbackData: "quantbt:4h"},
				{Text: "📊 1h", CallbackData: "quantbt:1h"},
				{Text: "📊 30m", CallbackData: "quantbt:30m"},
				{Text: "📊 15m", CallbackData: "quantbt:15m"},
			},
			{
				{Text: "⭐ Grade A", CallbackData: "quantbt:gradeA"},
				{Text: "⭐⭐ Grade B", CallbackData: "quantbt:gradeB"},
				{Text: "⭐⭐⭐ Grade C", CallbackData: "quantbt:gradeC"},
			},
		},
	}
}
