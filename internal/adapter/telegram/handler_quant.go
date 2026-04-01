package telegram

// handler_quant.go — /quant command: Econometric/Statistical Analysis dashboard
//   /quant [SYMBOL] [TIMEFRAME]  — Quant dashboard with inline keyboard

import (
	"bytes"
	"github.com/arkcode369/ark-intelligent/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// QuantServices — dependencies for the /quant command
// ---------------------------------------------------------------------------

type QuantServices struct {
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
	PriceMapping   []domain.PriceSymbolMapping
}

// ---------------------------------------------------------------------------
// quantState — cached per-chat state
// ---------------------------------------------------------------------------

type quantState struct {
	symbol    string
	currency  string
	timeframe string
	bars      map[string][]ta.OHLCV // tf → bars
	createdAt time.Time
}

var quantStateTTL = config.QuantStateTTL

// ---------------------------------------------------------------------------
// quantStateCache
// ---------------------------------------------------------------------------

type quantStateCache struct {
	mu    sync.Mutex
	store map[string]*quantState // chatID → state
}

func newQuantStateCache() *quantStateCache {
	return &quantStateCache{store: make(map[string]*quantState)}
}

func (c *quantStateCache) get(chatID string) *quantState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.createdAt) > quantStateTTL {
		delete(c.store, chatID)
		return nil
	}
	return s
}

func (c *quantStateCache) set(chatID string, s *quantState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	// Evict old entries
	for k, v := range c.store {
		if now.Sub(v.createdAt) > quantStateTTL*2 {
			delete(c.store, k)
		}
	}
	c.store[chatID] = s
}

// ---------------------------------------------------------------------------
// WithQuant injects QuantServices and registers commands
// ---------------------------------------------------------------------------

func (h *Handler) WithQuant(q *QuantServices) *Handler {
	h.quant = q
	if q != nil {
		h.quantCache = newQuantStateCache()
		h.registerQuantCommands()
	}
	return h
}

func (h *Handler) registerQuantCommands() {
	h.bot.RegisterCommand("/quant", h.cmdQuant)
	h.bot.RegisterCallback("quant:", h.handleQuantCallback)
}

// ---------------------------------------------------------------------------
// /quant — Main command
// ---------------------------------------------------------------------------

func (h *Handler) cmdQuant(ctx context.Context, chatID string, userID int64, args string) error {
	if h.quant == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Quant Engine not configured.")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) == 0 {
		// Fallback to last currency if available
		if lc := h.getLastCurrency(ctx, userID); lc != "" {
			return h.cmdQuant(ctx, chatID, userID, lc)
		}
		// Show symbol selector with description
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`🔬 <b>Quant Engine — Econometric Analysis</b>

Analisis statistik & ekonometrik institutional:

📊 <b>Stats</b> — Distribusi return, VaR, Sharpe, QQ plot
📈 <b>GARCH</b> — Volatility clustering & forecast
🔗 <b>Correlation</b> — Heatmap multi-asset
🎭 <b>Regime</b> — Hidden Markov Model (bull/bear/transition)
📅 <b>Seasonal</b> — Day-of-week & month patterns
🔄 <b>Mean Revert</b> — ADF, Hurst, half-life
⚡ <b>Granger</b> — Kausalitas antar aset
🔗 <b>Cointegration</b> — Pair trading analysis
🧬 <b>PCA</b> — Factor analysis multi-asset
🌐 <b>VAR</b> — Multi-asset forecast
⚠️ <b>Risk</b> — VaR/CVaR historical + parametric
📋 <b>Full Report</b> — Semua model → LONG/SHORT/FLAT

Pilih aset:`, h.kb.QuantSymbolMenu())
		return err
	}

	symbol := parts[0]
	timeframe := "daily"
	if len(parts) > 1 {
		timeframe = strings.ToLower(parts[1])
	}

	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/quant EUR</code>, <code>/quant XAU 4h</code>",
			html.EscapeString(symbol),
		))
		return err
	}

	// Save last currency for context carry-over
	h.saveLastCurrency(ctx, userID, mapping.Currency)

	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf("📊 Computing Quant Analysis for <b>%s</b> (%s)... ⏳", html.EscapeString(mapping.Currency), timeframe))

	state, err := h.computeQuantState(ctx, mapping, timeframe)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "quant")
		return nil
	}

	h.quantCache.set(chatID, state)

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}

	// Send dashboard
	dashboard := h.formatQuantDashboard(state)
	kb := h.kb.QuantMenu()
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, dashboard, kb)
	return err
}

// ---------------------------------------------------------------------------
// computeQuantState fetches bars for primary symbol + multi-asset
// ---------------------------------------------------------------------------

func (h *Handler) computeQuantState(ctx context.Context, mapping *domain.PriceSymbolMapping, timeframe string) (*quantState, error) {
	code := mapping.ContractCode
	barsByTF := make(map[string][]ta.OHLCV)

	// Fetch daily bars (always needed for most models)
	dailyRecords, err := h.quant.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
	if err != nil || len(dailyRecords) < 30 {
		return nil, fmt.Errorf("insufficient daily data for %s (%d bars)", mapping.Currency, len(dailyRecords))
	}
	barsByTF["daily"] = ta.DailyPricesToOHLCV(dailyRecords)

	// Fetch intraday if requested
	if timeframe != "daily" && h.quant.IntradayRepo != nil {
		count := 500
		if timeframe == "15m" || timeframe == "30m" {
			count = 2000
		}
		intradayBars, iErr := h.quant.IntradayRepo.GetHistory(ctx, code, timeframe, count)
		if iErr == nil && len(intradayBars) > 30 {
			barsByTF[timeframe] = ta.IntradayBarsToOHLCV(intradayBars)
		}
	}

	return &quantState{
		symbol:    mapping.Currency,
		currency:  mapping.Currency,
		timeframe: timeframe,
		bars:      barsByTF,
		createdAt: time.Now(),
	}, nil
}

// ---------------------------------------------------------------------------
// formatQuantDashboard — quick summary dashboard
// ---------------------------------------------------------------------------

func (h *Handler) formatQuantDashboard(state *quantState) string {
	return fmt.Sprintf(`📊 <b>QUANT DASHBOARD: %s</b>
📅 %s — %s

Pilih model analisis di bawah.
Setiap model akan menghasilkan chart + analisis detail.

<b>📊 Foundation:</b>
  Stats · GARCH · Correlation

<b>📈 Time Series:</b>
  ARIMA · Mean Reversion · Granger

<b>🎭 Advanced:</b>
  HMM Regime Detection

Klik tombol untuk mulai analisis.`,
		html.EscapeString(state.symbol),
		time.Now().Format("02 Jan 2006"),
		state.timeframe,
	)
}

// ---------------------------------------------------------------------------
// handleQuantCallback — inline button handler
// ---------------------------------------------------------------------------

func (h *Handler) handleQuantCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	// Format: quant:{action}
	parts := strings.SplitN(data, ":", 2)
	if len(parts) < 2 {
		return nil
	}
	action := parts[1]

	// Symbol selection from QuantSymbolMenu (before state check)
	if strings.HasPrefix(action, "sym:") {
		sym := strings.TrimPrefix(action, "sym:")
		return h.cmdQuant(ctx, chatID, 0, sym)
	}

	state := h.quantCache.get(chatID)
	if state == nil {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendHTML(ctx, chatID, sessionExpiredMessage("quant"))
		return err
	}

	// Timeframe switch
	if strings.HasPrefix(action, "tf:") {
		newTF := strings.TrimPrefix(action, "tf:")
		state.timeframe = newTF
		h.quantCache.set(chatID, state)
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		dashboard := h.formatQuantDashboard(state)
		kb := h.kb.QuantMenu()
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, dashboard, kb)
		return err
	}

	// Back to dashboard
	if action == "back" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		dashboard := h.formatQuantDashboard(state)
		kb := h.kb.QuantMenu()
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, dashboard, kb)
		return err
	}

	// Model-specific actions
	validModes := map[string]bool{
		"stats": true, "garch": true, "corr": true, "regime": true,
		"seasonal": true, "meanrevert": true, "granger": true,
		"coint": true, "pca": true, "var": true, "risk": true, "full": true,
	}

	mode := action
	// Alias
	if mode == "corr" {
		mode = "correlation"
	}
	if mode == "mr" {
		mode = "meanrevert"
	}
	if mode == "coint" {
		mode = "cointegration"
	}

	if !validModes[action] {
		return nil
	}

	// Send loading
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	loadingID, _ := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("⏳ Running %s for <b>%s</b> (%s)...", action, html.EscapeString(state.symbol), state.timeframe))

	// Run quant engine
	result, err := h.runQuantEngine(state, mode)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	if err != nil {
		h.sendUserError(ctx, chatID, err, "quant")
		return nil
	}

	kb := h.kb.QuantDetailMenu()

	// Send chart if available
	if result.ChartPath != "" {
		chartData, readErr := os.ReadFile(result.ChartPath)
		if readErr == nil && len(chartData) > 0 {
			shortCaption := fmt.Sprintf("📊 %s — %s — %s", strings.ToUpper(action), html.EscapeString(state.symbol), state.timeframe)
			_, _ = h.bot.SendPhoto(ctx, chatID, chartData, shortCaption)
		} else if readErr != nil {
			log.Warn().Err(readErr).Str("chart_path", result.ChartPath).
				Str("symbol", state.symbol).Str("timeframe", state.timeframe).
				Msg("quant: chart file unreadable")
		}
		os.Remove(result.ChartPath) // cleanup
	}

	// Send text with chart-failure note if chart was expected but unavailable
	textOut := result.TextOutput
	if textOut != "" {
		_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, textOut, kb)
	} else if !result.Success {
		_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, "❌ "+html.EscapeString(result.Error), kb)
	}
	return err
}

// ---------------------------------------------------------------------------
// quantEngineResult — parsed output from Python
// ---------------------------------------------------------------------------

type quantEngineResult struct {
	Mode       string                 `json:"mode"`
	Symbol     string                 `json:"symbol"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error"`
	Result     map[string]any `json:"result"`
	TextOutput string                 `json:"text_output"`
	ChartPath  string                 `json:"chart_path"`
}

// ---------------------------------------------------------------------------
// quantEngineInput — JSON sent to Python
// ---------------------------------------------------------------------------

type quantEngineInput struct {
	Mode       string                       `json:"mode"`
	Symbol     string                       `json:"symbol"`
	Timeframe  string                       `json:"timeframe"`
	Bars       []chartBar                   `json:"bars"`
	MultiAsset map[string][]quantAssetClose `json:"multi_asset,omitempty"`
	Params     map[string]any       `json:"params,omitempty"`
}

type quantAssetClose struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
}

// ---------------------------------------------------------------------------
// runQuantEngine — execute Python quant_engine.py
// ---------------------------------------------------------------------------

func (h *Handler) runQuantEngine(state *quantState, mode string) (*quantEngineResult, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Interface("panic", r).
				Msg("panic in runQuantEngine — subprocess may have failed")
		}
	}()
	tf := state.timeframe
	bars, ok := state.bars[tf]
	if !ok || len(bars) == 0 {
		// Fallback to daily
		bars, ok = state.bars["daily"]
		if !ok || len(bars) == 0 {
			return nil, fmt.Errorf("no bars available for %s", state.symbol)
		}
		tf = "daily"
	}

	// Convert bars (newest-first → oldest-first) — use date-only format for alignment
	n := len(bars)
	chartBars := make([]chartBar, n)
	for i, b := range bars {
		chartBars[n-1-i] = chartBar{
			Date:   b.Date.Format("2006-01-02"),
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		}
	}

	input := quantEngineInput{
		Mode:      mode,
		Symbol:    state.symbol,
		Timeframe: tf,
		Bars:      chartBars,
		Params: map[string]any{
			"lookback":         120,
			"forecast_horizon": 5,
			"confidence_level": 0.95,
		},
	}

	// Multi-asset data for correlation/granger/cointegration/pca/var/full
	needsMultiAsset := mode == "correlation" || mode == "granger" || mode == "cointegration" || mode == "pca" || mode == "var" || mode == "full"
	if needsMultiAsset {
		multiAsset, maErr := h.fetchMultiAssetCloses(state.symbol, tf)
		if maErr == nil && len(multiAsset) > 0 {
			input.MultiAsset = multiAsset
		}
	}

	// Marshal + execute
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal quant input: %w", err)
	}

	tmpDir := os.TempDir()
	ts := time.Now().UnixNano()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("quant_input_%d.json", ts))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("quant_output_%d.json", ts))
	chartPath := filepath.Join(tmpDir, fmt.Sprintf("quant_chart_%d.png", ts))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write quant input: %w", err)
	}
	defer os.Remove(inputPath)

	scriptPath := findQuantScript()

	// Timeout: 60s for complex models
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath, chartPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Error().Err(err).
			Str("stderr", stderr.String()).
			Str("symbol", state.symbol).
			Str("mode", mode).
			Msg("quant engine subprocess failed")
		os.Remove(chartPath) // cleanup chart on failure
		os.Remove(outputPath)
		return nil, fmt.Errorf("quant engine failed: %w", err)
	}

	// Read output
	outData, err := os.ReadFile(outputPath)
	os.Remove(outputPath)
	if err != nil {
		os.Remove(chartPath) // cleanup chart on failure
		return nil, fmt.Errorf("read quant output: %w", err)
	}

	var result quantEngineResult
	if err := json.Unmarshal(outData, &result); err != nil {
		os.Remove(chartPath) // cleanup chart on failure
		return nil, fmt.Errorf("parse quant output: %w", err)
	}

	// Check if chart was actually generated
	if fi, err := os.Stat(chartPath); err == nil {
		if fi.Size() > 0 {
			result.ChartPath = chartPath
		} else {
			log.Warn().Str("chart_path", chartPath).Msg("chart renderer produced 0-byte file, skipping")
			os.Remove(chartPath)
		}
	}

	return &result, nil
}

// ---------------------------------------------------------------------------
// fetchMultiAssetCloses — get daily closes for all tracked symbols
// ---------------------------------------------------------------------------

func (h *Handler) fetchMultiAssetCloses(excludeSymbol string, tf string) (map[string][]quantAssetClose, error) {
	ctx := context.Background()
	result := make(map[string][]quantAssetClose)

	// Use ALL tracked symbols from price mappings
	for _, mapping := range domain.DefaultPriceSymbolMappings {
		sym := mapping.Currency
		if strings.EqualFold(sym, excludeSymbol) {
			continue
		}
		if mapping.RiskOnly {
			continue // skip VIX, SPX risk-only symbols
		}

		records, err := h.quant.DailyPriceRepo.GetDailyHistory(ctx, mapping.ContractCode, 300)
		if err != nil || len(records) < 30 {
			continue
		}

		closes := make([]quantAssetClose, len(records))
		for i, r := range records {
			// records are newest-first, reverse for Python
			closes[len(records)-1-i] = quantAssetClose{
				Date:  r.Date.Format("2006-01-02"),
				Close: r.Close,
			}
		}
		result[sym] = closes
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// findQuantScript locates the quant_engine.py script
// ---------------------------------------------------------------------------

func findQuantScript() string {
	candidates := []string{
		"scripts/quant_engine.py",
		"../scripts/quant_engine.py",
		"/home/mulerun/.openclaw/workspace/ark-intelligent/scripts/quant_engine.py",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "scripts/quant_engine.py"
}
