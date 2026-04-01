package telegram

// handler_cta.go — /cta command: Classical Technical Analysis dashboard
//   /cta [SYMBOL] [TIMEFRAME]  — TA dashboard with chart + inline keyboard

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
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// ---------------------------------------------------------------------------
// CTAServices — dependencies for the /cta command
// ---------------------------------------------------------------------------

// CTAServices holds the services required for the CTA command.
type CTAServices struct {
	TAEngine       *ta.Engine
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
	PriceMapping   []domain.PriceSymbolMapping
}

// ---------------------------------------------------------------------------
// ctaState — cached computation results
// ---------------------------------------------------------------------------

type ctaState struct {
	symbol     string
	currency   string
	daily      *ta.FullResult
	h4         *ta.FullResult
	h1         *ta.FullResult
	m15        *ta.FullResult
	m30        *ta.FullResult
	h6         *ta.FullResult
	h12        *ta.FullResult
	weekly     *ta.FullResult
	mtf        *ta.MTFResult
	bars       map[string][]ta.OHLCV // timeframe -> bars
	chartData  map[string][]byte     // timeframe -> PNG bytes (lazy-generated)
	computedAt time.Time
}

const ctaStateTTL = 120 * time.Second

// ctaStateCache stores per-chat CTA state with TTL.
type ctaStateCache struct {
	mu    sync.Mutex
	store map[string]*ctaState // chatID -> state
}

func newCTAStateCache() *ctaStateCache {
	return &ctaStateCache{
		store: make(map[string]*ctaState),
	}
}

func (c *ctaStateCache) get(chatID string) *ctaState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.computedAt) > ctaStateTTL {
		return nil
	}
	return s
}

func (c *ctaStateCache) set(chatID string, s *ctaState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Opportunistic cleanup
	if len(c.store) > 50 {
		now := time.Now()
		for k, v := range c.store {
			if now.Sub(v.computedAt) > ctaStateTTL*2 {
				delete(c.store, k)
			}
		}
	}
	c.store[chatID] = s
}

// ---------------------------------------------------------------------------
// Handler fields (stored in Handler struct)
// ---------------------------------------------------------------------------

// WithCTA injects CTAServices into the handler and registers CTA commands.
func (h *Handler) WithCTA(c *CTAServices) *Handler {
	h.cta = c
	if c != nil {
		h.ctaCache = newCTAStateCache()
		h.registerCTACommands()
	}
	return h
}

// registerCTACommands wires the CTA commands into the bot.
func (h *Handler) registerCTACommands() {
	h.bot.RegisterCommand("/cta", h.cmdCTA)
	h.bot.RegisterCallback("cta:", h.handleCTACallback)
}

// ---------------------------------------------------------------------------
// /cta — Main CTA Command
// ---------------------------------------------------------------------------

func (h *Handler) cmdCTA(ctx context.Context, chatID string, _ int64, args string) error {
	if h.cta == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ CTA Engine not configured.")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) == 0 {
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`📈 <b>CTA — Classical Technical Analysis</b>

Multi-timeframe TA dashboard dengan 6 tools:

📊 <b>Chart</b> — Candlestick + indikator per TF (15m-daily)
🏯 <b>Ichimoku</b> — Cloud, Tenkan/Kijun, signal
📐 <b>Fibonacci</b> — Swing levels + Golden Zone
🕯 <b>Patterns</b> — Candlestick pattern detection
⚡ <b>Confluence</b> — Multi-indicator agreement score
📱 <b>Multi-TF</b> — Alignment semua timeframe
🎯 <b>Zones</b> — Entry/SL/TP otomatis

Pilih aset:`, h.kb.CTASymbolMenu())
		return err
	}

	symbol := parts[0]

	// Resolve symbol to contract code
	mapping := h.resolveCTAMapping(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/cta EUR</code>, <code>/cta XAU</code>, <code>/cta BTC</code>",
			html.EscapeString(symbol),
		))
		return err
	}

	// Send loading indicator
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf("⚡ Computing TA for <b>%s</b>... ⏳", html.EscapeString(mapping.Currency)))

	// Compute CTA state
	state, err := h.computeCTAState(ctx, mapping)
	if err != nil {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		h.sendUserError(ctx, chatID, err, "cta")
		return nil
	}

	h.ctaCache.set(chatID, state)

	// Generate chart for daily timeframe
	chartPNG, chartErr := h.generateCTAChart(state, "daily")
	if chartErr != nil {
		log.Warn().Err(chartErr).Str("symbol", symbol).Msg("chart generation failed")
	}

	// Delete loading message
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}

	// Format summary
	summary := formatCTASummary(state)
	kb := h.kb.CTAMenu()

	// Send photo with keyboard if chart available, otherwise text
	if chartPNG != nil && len(chartPNG) > 0 {
		state.chartData["daily"] = chartPNG

		// Photo caption limited to 1024 chars by Telegram.
		// Send chart with short caption, then full analysis as separate message.
		shortCaption := fmt.Sprintf("⚡ <b>CTA: %s</b> — Daily", html.EscapeString(mapping.Currency))
		_, photoErr := h.bot.SendPhotoWithKeyboard(ctx, chatID, chartPNG, shortCaption, kb)
		if photoErr != nil {
			log.Warn().Err(photoErr).Msg("send CTA photo failed, falling back to text")
			_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
			return err
		}
		// Send full analysis as text
		_, err = h.bot.SendHTML(ctx, chatID, summary)
		return err
	}

	// Fallback: send text only
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
	return err
}

// ---------------------------------------------------------------------------
// Callback Handler
// ---------------------------------------------------------------------------

func (h *Handler) handleCTACallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	action := strings.TrimPrefix(data, "cta:")

	// Symbol selection from CTASymbolMenu (before state check)
	if strings.HasPrefix(action, "sym:") {
		sym := strings.TrimPrefix(action, "sym:")
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdCTA(ctx, chatID, 0, sym)
	}

	// Get or recompute state
	state := h.ctaCache.get(chatID)
	if state == nil {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendHTML(ctx, chatID, "⏳ Data expired. Gunakan /cta untuk refresh.")
		return err
	}

	switch {
	case action == "back":
		return h.ctaShowSummaryChart(ctx, chatID, msgID, state, "daily")

	case action == "refresh":
		// Recompute
		mapping := h.resolveCTAMapping(state.currency)
		if mapping == nil {
			return h.bot.EditMessage(ctx, chatID, msgID, "❌ Symbol not found.")
		}
		newState, err := h.computeCTAState(ctx, mapping)
		if err != nil {
			h.editUserError(ctx, chatID, msgID, err, "cta")
			return nil
		}
		h.ctaCache.set(chatID, newState)
		return h.ctaShowSummaryChart(ctx, chatID, msgID, newState, "daily")

	case strings.HasPrefix(action, "tf:"):
		tf := strings.TrimPrefix(action, "tf:")
		return h.ctaShowTimeframe(ctx, chatID, msgID, state, tf)

	case action == "ichi":
		txt := formatCTAIchimoku(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		chartPNG, chartErr := h.generateCTADetailChart(state, "daily", "ichimoku")
		if chartErr == nil && len(chartPNG) > 0 {
			shortCaption := fmt.Sprintf("🏯 Ichimoku Cloud — %s", html.EscapeString(state.symbol))
			_, _ = h.bot.SendPhoto(ctx, chatID, chartPNG, shortCaption)
		}
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "fib":
		txt := formatCTAFibonacci(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		chartPNG, chartErr := h.generateCTADetailChart(state, "daily", "fibonacci")
		if chartErr == nil && len(chartPNG) > 0 {
			shortCaption := fmt.Sprintf("📐 Fibonacci — %s", html.EscapeString(state.symbol))
			_, _ = h.bot.SendPhoto(ctx, chatID, chartPNG, shortCaption)
		}
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "patterns":
		txt := formatCTAPatterns(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "confluence":
		txt := formatCTAConfluence(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "mtf":
		txt := formatCTAMTF(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "zones":
		txt := formatCTAZones(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		chartPNG, chartErr := h.generateCTADetailChart(state, "daily", "zones")
		if chartErr == nil && len(chartPNG) > 0 {
			shortCaption := fmt.Sprintf("🎯 Trade Setup — %s", html.EscapeString(state.symbol))
			_, _ = h.bot.SendPhoto(ctx, chatID, chartPNG, shortCaption)
		}
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err
	}

	return nil
}

// ctaShowSummaryChart deletes old message and sends new photo (can't edit text→photo).
func (h *Handler) ctaShowSummaryChart(ctx context.Context, chatID string, msgID int, state *ctaState, tf string) error {
	chartPNG, err := h.getCTAChart(state, tf)
	if err != nil || len(chartPNG) == 0 {
		// Fallback to text
		summary := formatCTASummary(state)
		kb := h.kb.CTAMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, summary, kb)
	}

	// Delete old and send new photo
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	summary := formatCTASummary(state)
	kb := h.kb.CTAMenu()
	shortCaption := fmt.Sprintf("⚡ <b>CTA: %s</b> — Daily", html.EscapeString(state.symbol))
	_, photoErr := h.bot.SendPhotoWithKeyboard(ctx, chatID, chartPNG, shortCaption, kb)
	if photoErr != nil {
		_, sendErr := h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
		return sendErr
	}
	_, sendErr := h.bot.SendHTML(ctx, chatID, summary)
	return sendErr
}

// ctaShowTimeframe shows a specific timeframe detail + chart.
func (h *Handler) ctaShowTimeframe(ctx context.Context, chatID string, msgID int, state *ctaState, tf string) error {
	result := h.getCTAResult(state, tf)
	if result == nil {
		return h.bot.EditMessage(ctx, chatID, msgID, fmt.Sprintf("⚠️ Data %s tidak tersedia.", tf))
	}

	chartPNG, _ := h.getCTAChart(state, tf)

	txt := formatCTATimeframeDetail(state, tf, result)
	kb := h.kb.CTATimeframeMenu()

	if chartPNG != nil && len(chartPNG) > 0 {
		// Delete old and send photo with short caption + full text separately
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		shortCaption := fmt.Sprintf("⚡ <b>CTA: %s</b> — %s", html.EscapeString(state.symbol), strings.ToUpper(tf))
		_, photoErr := h.bot.SendPhotoWithKeyboard(ctx, chatID, chartPNG, shortCaption, kb)
		if photoErr != nil {
			_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
			return err
		}
		_, err := h.bot.SendHTML(ctx, chatID, txt)
		return err
	}

	return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)
}

// ---------------------------------------------------------------------------
// Data Fetching & Computation
// ---------------------------------------------------------------------------

func (h *Handler) resolveCTAMapping(symbol string) *domain.PriceSymbolMapping {
	return domain.FindPriceMappingByCurrency(strings.ToUpper(symbol))
}

func (h *Handler) computeCTAState(ctx context.Context, mapping *domain.PriceSymbolMapping) (*ctaState, error) {
	code := mapping.ContractCode
	barsByTF := make(map[string][]ta.OHLCV)

	// Fetch daily bars
	dailyRecords, err := h.cta.DailyPriceRepo.GetDailyHistory(ctx, code, 300)
	if err != nil || len(dailyRecords) < 20 {
		return nil, fmt.Errorf("insufficient daily data for %s (%d bars)", mapping.Currency, len(dailyRecords))
	}
	dailyBars := ta.DailyPricesToOHLCV(dailyRecords)
	barsByTF["daily"] = dailyBars

	// Fetch intraday bars (best-effort, non-fatal)
	if h.cta.IntradayRepo != nil {
		for _, spec := range []struct {
			interval string
			count    int
		}{
			{"15m", 500},
			{"30m", 312},
			{"1h", 200},
			{"4h", 312},
			{"6h", 200},
			{"12h", 200},
		} {
			intradayBars, iErr := h.cta.IntradayRepo.GetHistory(ctx, code, spec.interval, spec.count)
			if iErr == nil && len(intradayBars) > 10 {
				barsByTF[spec.interval] = ta.IntradayBarsToOHLCV(intradayBars)
			}
		}
	}

	engine := h.cta.TAEngine

	// Compute FullResult per timeframe
	var daily, h4, h1, m15, m30, h6, h12, weekly *ta.FullResult

	daily = engine.ComputeFull(barsByTF["daily"])
	if b, ok := barsByTF["4h"]; ok {
		h4 = engine.ComputeFull(b)
	}
	if b, ok := barsByTF["1h"]; ok {
		h1 = engine.ComputeFull(b)
	}
	if b, ok := barsByTF["15m"]; ok {
		m15 = engine.ComputeFull(b)
	}
	if b, ok := barsByTF["30m"]; ok {
		m30 = engine.ComputeFull(b)
	}
	if b, ok := barsByTF["6h"]; ok {
		h6 = engine.ComputeFull(b)
	}
	if b, ok := barsByTF["12h"]; ok {
		h12 = engine.ComputeFull(b)
	}

	// Weekly: aggregate from daily (simple: take every 5th bar as weekly candle)
	// For now, skip weekly — not enough data typically
	_ = weekly

	// Compute MTF
	mtfBars := make(map[string][]ta.OHLCV)
	for tf, bars := range barsByTF {
		mtfBars[tf] = bars
	}
	mtf := engine.ComputeMTF(mtfBars)

	// Determine display symbol
	displaySymbol := mapping.Currency
	if mapping.TwelveData != "" {
		displaySymbol = mapping.TwelveData
	}

	return &ctaState{
		symbol:     displaySymbol,
		currency:   mapping.Currency,
		daily:      daily,
		h4:         h4,
		h1:         h1,
		m15:        m15,
		m30:        m30,
		h6:         h6,
		h12:        h12,
		weekly:     weekly,
		mtf:        mtf,
		bars:       barsByTF,
		chartData:  make(map[string][]byte),
		computedAt: time.Now(),
	}, nil
}

func (h *Handler) getCTAResult(state *ctaState, tf string) *ta.FullResult {
	switch tf {
	case "daily", "d":
		return state.daily
	case "4h":
		return state.h4
	case "1h":
		return state.h1
	case "15m":
		return state.m15
	case "30m":
		return state.m30
	case "6h":
		return state.h6
	case "12h":
		return state.h12
	case "weekly", "w":
		return state.weekly
	default:
		return state.daily
	}
}

// getCTAChart returns cached chart or generates it.
func (h *Handler) getCTAChart(state *ctaState, tf string) ([]byte, error) {
	if data, ok := state.chartData[tf]; ok && len(data) > 0 {
		return data, nil
	}
	data, err := h.generateCTAChart(state, tf)
	if err != nil {
		return nil, err
	}
	state.chartData[tf] = data
	return data, nil
}

// ---------------------------------------------------------------------------
// Chart Generation
// ---------------------------------------------------------------------------

// chartInput is the JSON structure passed to the Python chart renderer.
type chartInput struct {
	Symbol     string            `json:"symbol"`
	Timeframe  string            `json:"timeframe"`
	Mode       string            `json:"mode,omitempty"`
	Bars       []chartBar        `json:"bars"`
	Indicators chartIndicators   `json:"indicators"`
	Fibonacci  chartFib          `json:"fibonacci"`
	Patterns   []chartPattern    `json:"patterns"`
	Ichimoku   *chartIchimoku    `json:"ichimoku,omitempty"`
	Zones      *chartZones       `json:"zones,omitempty"`
}

type chartIchimoku struct {
	TenkanSen  []float64 `json:"tenkan_sen"`
	KijunSen   []float64 `json:"kijun_sen"`
	SenkouSpanA []float64 `json:"senkou_span_a"`
	SenkouSpanB []float64 `json:"senkou_span_b"`
	ChikouSpan []float64 `json:"chikou_span"`
}

type chartZones struct {
	Direction   string  `json:"direction"`
	EntryHigh   float64 `json:"entry_high"`
	EntryLow    float64 `json:"entry_low"`
	StopLoss    float64 `json:"stop_loss"`
	TakeProfit1 float64 `json:"take_profit_1"`
	TakeProfit2 float64 `json:"take_profit_2"`
	RiskReward1 float64 `json:"risk_reward_1"`
	RiskReward2 float64 `json:"risk_reward_2"`
	Confidence  string  `json:"confidence"`
}

type chartBar struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type chartIndicators struct {
	EMA9          []float64 `json:"ema9,omitempty"`
	EMA21         []float64 `json:"ema21,omitempty"`
	EMA55         []float64 `json:"ema55,omitempty"`
	BBUpper       []float64 `json:"bb_upper,omitempty"`
	BBMiddle      []float64 `json:"bb_middle,omitempty"`
	BBLower       []float64 `json:"bb_lower,omitempty"`
	SuperTrend    []float64 `json:"supertrend,omitempty"`
	SuperTrendDir []string  `json:"supertrend_dir,omitempty"`
	RSI           []float64 `json:"rsi,omitempty"`
	MACD          []float64 `json:"macd,omitempty"`
	MACDSignal    []float64 `json:"macd_signal,omitempty"`
	MACDHistogram []float64 `json:"macd_histogram,omitempty"`
}

type chartFib struct {
	Levels       map[string]float64 `json:"levels,omitempty"`
	TrendDir     string             `json:"trend_dir,omitempty"`
	SwingHigh    float64            `json:"swing_high,omitempty"`
	SwingLow     float64            `json:"swing_low,omitempty"`
	SwingHighIdx int                `json:"swing_high_idx,omitempty"`
	SwingLowIdx  int                `json:"swing_low_idx,omitempty"`
}

type chartPattern struct {
	Name      string `json:"name"`
	BarIndex  int    `json:"bar_index"`
	Direction string `json:"direction"`
}

func (h *Handler) generateCTAChart(state *ctaState, timeframe string) ([]byte, error) {
	ctx := context.Background()
	bars, ok := state.bars[timeframe]
	if !ok || len(bars) == 0 {
		return nil, fmt.Errorf("no bars for timeframe %s", timeframe)
	}

	result := h.getCTAResult(state, timeframe)

	// Convert bars to chart format (bars are newest-first, chart expects oldest-first)
	n := len(bars)
	chartBars := make([]chartBar, n)
	for i, b := range bars {
		chartBars[n-1-i] = chartBar{
			Date:   b.Date.Format(time.RFC3339),
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		}
	}

	// Build indicator arrays
	ind := chartIndicators{}

	// Compute indicator series from bars
	// EMA series
	closes := make([]float64, n)
	for i, b := range bars {
		closes[i] = b.Close
	}

	ema9Series := ta.CalcEMA(closes, 9)
	ema21Series := ta.CalcEMA(closes, 21)
	ema55Series := ta.CalcEMA(closes, 55)
	if ema9Series != nil {
		ind.EMA9 = reverseToOldestFirst(ema9Series)
	}
	if ema21Series != nil {
		ind.EMA21 = reverseToOldestFirst(ema21Series)
	}
	if ema55Series != nil {
		ind.EMA55 = reverseToOldestFirst(ema55Series)
	}

	// Bollinger Bands
	bbU, bbM, bbL := ta.CalcBollingerSeries(bars, 20, 2.0)
	if bbU != nil {
		ind.BBUpper = reverseToOldestFirst(bbU)
		ind.BBMiddle = reverseToOldestFirst(bbM)
		ind.BBLower = reverseToOldestFirst(bbL)
	}

	// SuperTrend
	if result != nil && result.Snapshot != nil && result.Snapshot.SuperTrend != nil {
		st := result.Snapshot.SuperTrend
		if st.Series != nil {
			ind.SuperTrend = reverseToOldestFirst(st.Series) // NaN sanitized by reverseToOldestFirst
		}
		if st.DirectionSeries != nil {
			ind.SuperTrendDir = reverseStringsToOldestFirst(st.DirectionSeries)
		}
	}

	// RSI
	rsiSeries := ta.CalcRSISeries(bars, 14)
	if rsiSeries != nil {
		ind.RSI = reverseToOldestFirst(rsiSeries)
	}

	// MACD
	macdLine, signalLine, histogram := ta.CalcMACDSeries(bars, 12, 26, 9)
	if macdLine != nil {
		ind.MACD = reverseToOldestFirst(macdLine)
		ind.MACDSignal = reverseToOldestFirst(signalLine)
		ind.MACDHistogram = reverseToOldestFirst(histogram)
	}

	// Fibonacci levels
	fibData := chartFib{}
	if result != nil && result.Snapshot != nil && result.Snapshot.Fibonacci != nil {
		// Sanitize Fibonacci levels (remove NaN/Inf)
		sanitizedLevels := make(map[string]float64)
		for k, v := range result.Snapshot.Fibonacci.Levels {
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				sanitizedLevels[k] = v
			}
		}
		fibData.Levels = sanitizedLevels
	}

	// Patterns
	var chartPatterns []chartPattern
	if result != nil && result.Patterns != nil {
		for _, p := range result.Patterns {
			chartPatterns = append(chartPatterns, chartPattern{
				Name:      p.Name,
				BarIndex:  p.BarIndex,
				Direction: p.Direction,
			})
		}
	}

	// Build input JSON
	input := chartInput{
		Symbol:     state.symbol,
		Timeframe:  timeframe,
		Bars:       chartBars,
		Indicators: ind,
		Fibonacci:  fibData,
		Patterns:   chartPatterns,
	}

	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal chart input: %w", err)
	}

	// Write to temp file
	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("cta_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("cta_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write chart input: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	// Find the script path relative to the binary
	scriptPath := findCTAScript()

	// Execute Python script
	cmd := exec.CommandContext(ctx, "python3", scriptPath, inputPath, outputPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("chart renderer failed: %w", err)
	}

	// Read output PNG
	pngData, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read chart output: %w", err)
	}

	return pngData, nil
}

// runChartScript marshals input to JSON, runs cta_chart.py, and returns PNG bytes.
func runChartScript(input interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal chart input: %w", err)
	}

	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("cta_input_%d.json", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("cta_output_%d.png", time.Now().UnixNano()))

	if err := os.WriteFile(inputPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("write chart input: %w", err)
	}
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)

	scriptPath := findCTAScript()
	cmd := exec.CommandContext(context.Background(), "python3", scriptPath, inputPath, outputPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("chart renderer failed: %w", err)
	}

	return os.ReadFile(outputPath)
}

// generateCTADetailChart creates a mode-specific chart (ichimoku, fibonacci, zones).
func (h *Handler) generateCTADetailChart(state *ctaState, timeframe string, mode string) ([]byte, error) {
	bars, ok := state.bars[timeframe]
	if !ok || len(bars) == 0 {
		// Fallback to daily
		bars, ok = state.bars["daily"]
		if !ok || len(bars) == 0 {
			return nil, fmt.Errorf("no bars available")
		}
		timeframe = "daily"
	}

	result := h.getCTAResult(state, timeframe)

	// Convert bars to chart format (newest-first → oldest-first)
	n := len(bars)
	chartBars := make([]chartBar, n)
	for i, b := range bars {
		chartBars[n-1-i] = chartBar{
			Date:   b.Date.Format(time.RFC3339),
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		}
	}

	input := chartInput{
		Symbol:    state.symbol,
		Timeframe: timeframe,
		Mode:      mode,
		Bars:      chartBars,
	}

	switch mode {
	case "ichimoku":
		series := ta.CalcIchimokuSeries(bars)
		if series != nil {
			input.Ichimoku = &chartIchimoku{
				TenkanSen:   reverseToOldestFirst(series.Tenkan),
				KijunSen:    reverseToOldestFirst(series.Kijun),
				SenkouSpanA: reverseToOldestFirst(series.SenkouA),
				SenkouSpanB: reverseToOldestFirst(series.SenkouB),
				ChikouSpan:  reverseToOldestFirst(series.Chikou),
			}
		}

	case "fibonacci":
		if result != nil && result.Snapshot != nil && result.Snapshot.Fibonacci != nil {
			fib := result.Snapshot.Fibonacci
			sanitizedLevels := make(map[string]float64)
			for k, v := range fib.Levels {
				if !math.IsNaN(v) && !math.IsInf(v, 0) {
					sanitizedLevels[k] = v
				}
			}
			input.Fibonacci = chartFib{
				Levels:       sanitizedLevels,
				TrendDir:     fib.TrendDir,
				SwingHigh:    fib.SwingHigh,
				SwingLow:     fib.SwingLow,
				SwingHighIdx: fib.SwingHighIdx,
				SwingLowIdx:  fib.SwingLowIdx,
			}
		}

	case "zones":
		if result != nil && result.Zones != nil && result.Zones.Valid {
			z := result.Zones
			input.Zones = &chartZones{
				Direction:   z.Direction,
				EntryHigh:   z.EntryHigh,
				EntryLow:    z.EntryLow,
				StopLoss:    z.StopLoss,
				TakeProfit1: z.TakeProfit1,
				TakeProfit2: z.TakeProfit2,
				RiskReward1: z.RiskReward1,
				RiskReward2: z.RiskReward2,
				Confidence:  z.Confidence,
			}
		}
	}

	return runChartScript(input)
}

// findCTAScript locates the cta_chart.py script.
func findCTAScript() string {
	candidates := []string{
		"scripts/cta_chart.py",
		"../scripts/cta_chart.py",
		"/home/mulerun/.openclaw/workspace/ark-intelligent/scripts/cta_chart.py",
	}

	// Check relative to current working dir
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	// Check relative to executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		rel := filepath.Join(execDir, "scripts", "cta_chart.py")
		if _, err := os.Stat(rel); err == nil {
			return rel
		}
		rel = filepath.Join(execDir, "..", "scripts", "cta_chart.py")
		if _, err := os.Stat(rel); err == nil {
			abs, _ := filepath.Abs(rel)
			return abs
		}
	}

	// Fallback
	return "scripts/cta_chart.py"
}

// sanitizeFloats replaces NaN and Inf with 0 so JSON marshaling works.
func sanitizeFloats(s []float64) []float64 {
	for i, v := range s {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			s[i] = 0
		}
	}
	return s
}

// reverseToOldestFirst converts a newest-first slice to oldest-first and sanitizes NaN/Inf.
func reverseToOldestFirst(s []float64) []float64 {
	n := len(s)
	out := make([]float64, n)
	for i, v := range s {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			out[n-1-i] = 0
		} else {
			out[n-1-i] = v
		}
	}
	return out
}

// reverseStringsToOldestFirst converts a newest-first string slice to oldest-first.
func reverseStringsToOldestFirst(s []string) []string {
	n := len(s)
	out := make([]string, n)
	for i, v := range s {
		out[n-1-i] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// Formatters — Indonesian language
// ---------------------------------------------------------------------------

func formatCTASummary(state *ctaState) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<b>⚡ Technical Analysis: %s</b>\n", html.EscapeString(state.symbol)))
	sb.WriteString(fmt.Sprintf("📅 <i>%s</i>\n\n", state.computedAt.UTC().Format("02 Jan 2006 15:04 UTC")))

	d := state.daily
	if d == nil || d.Confluence == nil {
		sb.WriteString("⚠️ Data tidak cukup untuk analisis.\n")
		return sb.String()
	}

	conf := d.Confluence

	// Decision with grade-aware language
	dirEmoji := "🔵"
	dirLabel := conf.Direction
	if conf.Direction == "BULLISH" {
		dirEmoji = "🟢"
	} else if conf.Direction == "BEARISH" {
		dirEmoji = "🔴"
	}

	sb.WriteString(fmt.Sprintf("🎯 <b>KEPUTUSAN: %s %s</b> (Skor: %+.0f/100, Grade: %s)\n",
		dirEmoji, dirLabel, conf.Score, conf.Grade))

	// Plain-language grade explanation
	switch conf.Grade {
	case "A":
		sb.WriteString("<i>Sinyal sangat kuat — kepercayaan tinggi.</i>\n")
	case "B":
		sb.WriteString("<i>Sinyal cukup kuat — bisa dijadikan acuan.</i>\n")
	case "C":
		sb.WriteString("<i>Sinyal moderate — perlu konfirmasi tambahan.</i>\n")
	case "D":
		sb.WriteString("<i>⚠️ Sinyal sangat lemah — belum ada arah yang jelas. Hindari entry.</i>\n")
	default:
		sb.WriteString("<i>⚠️ Tidak ada sinyal berarti — pasar belum menunjukkan arah.</i>\n")
	}

	sb.WriteString(fmt.Sprintf("Confluence: %d/%d indikator %s\n\n",
		conf.BullishCount, conf.TotalIndicators, strings.ToLower(dirLabel)))

	// Summary
	sb.WriteString("📊 <b>Ringkasan Teknikal:</b>\n")
	snap := d.Snapshot

	// Trend
	if snap != nil && snap.EMA != nil {
		trendNote := "Mixed — belum ada arah tren yang jelas"
		if snap.EMA.RibbonAlignment == "BULLISH" {
			trendNote = "Bullish — semua EMA tersusun naik (harga di atas moving average)"
		} else if snap.EMA.RibbonAlignment == "BEARISH" {
			trendNote = "Bearish — semua EMA tersusun turun (harga di bawah moving average)"
		}
		if snap.SuperTrend != nil {
			if snap.SuperTrend.Direction == "UP" {
				trendNote += "\n  ↳ SuperTrend: ✅ Bullish — harga di atas support dinamis"
			} else {
				trendNote += "\n  ↳ SuperTrend: ❌ Bearish — harga di bawah resistance dinamis"
			}
		}
		sb.WriteString(fmt.Sprintf("• <b>Tren:</b> %s\n", trendNote))
	}

	// Momentum
	if snap != nil && snap.RSI != nil {
		rsiVal := snap.RSI.Value
		rsiNote := fmt.Sprintf("RSI %.0f", rsiVal)
		if snap.RSI.Zone == "OVERBOUGHT" {
			rsiNote += " — ⚠️ jenuh beli (overbought), hati-hati momentum bisa melambat"
		} else if snap.RSI.Zone == "OVERSOLD" {
			rsiNote += " — 💡 jenuh jual (oversold), potensi rebound"
		} else if rsiVal >= 55 {
			rsiNote += " — momentum cenderung bullish"
		} else if rsiVal <= 45 {
			rsiNote += " — momentum cenderung bearish"
		} else {
			rsiNote += " — netral, belum ada momentum kuat"
		}

		macdNote := ""
		if snap.MACD != nil {
			if snap.MACD.BullishCross {
				macdNote = "\n  ↳ MACD baru saja bullish cross — sinyal momentum naik baru dimulai ✅"
			} else if snap.MACD.BearishCross {
				macdNote = "\n  ↳ MACD baru saja bearish cross — sinyal momentum turun baru dimulai ❌"
			} else if snap.MACD.Histogram > 0 {
				macdNote = "\n  ↳ MACD histogram positif — momentum masih mendukung kenaikan"
			} else if snap.MACD.Histogram < 0 {
				macdNote = "\n  ↳ MACD histogram negatif — momentum masih mendukung penurunan"
			}
		}
		sb.WriteString(fmt.Sprintf("• <b>Momentum:</b> %s%s\n", rsiNote, macdNote))
	}

	// Volume
	if snap != nil && snap.OBV != nil {
		obvNote := "flat — volume tidak menunjukkan arah"
		if snap.OBV.Trend == "RISING" {
			obvNote = "naik — volume mendukung pergerakan harga (konfirmasi tren)"
		} else if snap.OBV.Trend == "FALLING" {
			obvNote = "turun — volume melemah (potensi divergensi)"
		}
		sb.WriteString(fmt.Sprintf("• <b>Volume:</b> OBV %s\n", obvNote))
	}

	// Volatility
	if snap != nil && snap.Bollinger != nil {
		bbNote := fmt.Sprintf("BB %%B=%.2f", snap.Bollinger.PercentB)
		if snap.Bollinger.Squeeze {
			bbNote += " — 💥 squeeze terdeteksi! Harga terkompresi, potensi breakout besar"
		} else if snap.Bollinger.PercentB > 0.8 {
			bbNote += " — harga mendekati band atas (potensi jenuh)"
		} else if snap.Bollinger.PercentB < 0.2 {
			bbNote += " — harga mendekati band bawah (potensi pantulan)"
		} else {
			bbNote += " — volatilitas normal"
		}
		sb.WriteString(fmt.Sprintf("• <b>Volatilitas:</b> %s\n", bbNote))
	}

	sb.WriteString("\n")

	// Ichimoku
	if snap != nil && snap.Ichimoku != nil {
		ich := snap.Ichimoku
		ichNote := ""
		switch ich.Overall {
		case "STRONG_BULLISH":
			ichNote = "Semua sinyal Ichimoku sejajar bullish — tren naik sangat kuat"
		case "BULLISH":
			ichNote = "Mayoritas sinyal Ichimoku bullish — tren cenderung naik"
		case "STRONG_BEARISH":
			ichNote = "Semua sinyal Ichimoku sejajar bearish — tren turun sangat kuat"
		case "BEARISH":
			ichNote = "Mayoritas sinyal Ichimoku bearish — tren cenderung turun"
		default:
			ichNote = "Sinyal campur — pasar belum menunjukkan arah jelas"
		}
		sb.WriteString(fmt.Sprintf("🏯 <b>Ichimoku:</b> %s (%s)\n", ich.Overall, ichNote))
		if ich.KumoBreakout == "BULLISH_BREAKOUT" {
			sb.WriteString("  ↳ Harga di atas awan (cloud) — zona bullish ✅\n")
		} else if ich.KumoBreakout == "BEARISH_BREAKOUT" {
			sb.WriteString("  ↳ Harga di bawah awan (cloud) — zona bearish ❌\n")
		} else if ich.KumoBreakout == "INSIDE_CLOUD" {
			sb.WriteString("  ↳ Harga di dalam awan — zona netral/transisi ⚠️\n")
		}
	}

	// Fibonacci
	if snap != nil && snap.Fibonacci != nil {
		fib := snap.Fibonacci
		sb.WriteString(fmt.Sprintf("📐 <b>Fibonacci:</b> Level terdekat %s%% di %.4f\n", fib.NearestLevel, fib.NearestPrice))
		sb.WriteString("  ↳ <i>Level Fibonacci = area support/resistance kunci berdasarkan swing harga</i>\n")
	}

	// Patterns
	if len(d.Patterns) > 0 {
		p := d.Patterns[0]
		stars := strings.Repeat("★", p.Reliability) + strings.Repeat("☆", 3-p.Reliability)
		patDir := "netral"
		if p.Direction == "BULLISH" {
			patDir = "potensi naik"
		} else if p.Direction == "BEARISH" {
			patDir = "potensi turun"
		}
		sb.WriteString(fmt.Sprintf("🕯 <b>Pola:</b> %s %s — %s\n", p.Name, stars, patDir))
	}

	// Divergences
	if len(d.Divergences) > 0 {
		div := d.Divergences[0]
		divNote := ""
		switch div.Type {
		case "REGULAR_BULLISH":
			divNote = "harga turun tapi indikator naik — potensi reversal naik 🔄"
		case "REGULAR_BEARISH":
			divNote = "harga naik tapi indikator turun — potensi reversal turun 🔄"
		case "HIDDEN_BULLISH":
			divNote = "sinyal kelanjutan tren naik"
		case "HIDDEN_BEARISH":
			divNote = "sinyal kelanjutan tren turun"
		}
		sb.WriteString(fmt.Sprintf("⚡ <b>Divergence:</b> %s (%s) — %s\n", div.Type, div.Indicator, divNote))
	}

	sb.WriteString("\n")

	// Zones — with safeguard messaging
	if d.Zones != nil && d.Zones.Valid {
		z := d.Zones
		confLabel := ""
		switch z.Confidence {
		case "HIGH":
			confLabel = "Kepercayaan: ✅ Tinggi"
		case "MEDIUM":
			confLabel = "Kepercayaan: 🟡 Sedang"
		default:
			confLabel = "Kepercayaan: ⚠️ Rendah — gunakan dengan hati-hati"
		}
		sb.WriteString(fmt.Sprintf("🎯 <b>Setup %s</b> (%s)\n", z.Direction, confLabel))
		sb.WriteString(fmt.Sprintf("  Entry: %.4f – %.4f\n", z.EntryLow, z.EntryHigh))
		sb.WriteString(fmt.Sprintf("  🛑 Stop Loss: %.4f\n", z.StopLoss))
		sb.WriteString(fmt.Sprintf("  ✅ TP1: %.4f (R:R 1:%.1f) | TP2: %.4f (R:R 1:%.1f)\n",
			z.TakeProfit1, z.RiskReward1, z.TakeProfit2, z.RiskReward2))
		sb.WriteString("<i>  ↳ R:R = rasio potensi profit vs risiko. Semakin tinggi semakin baik.</i>\n")
		sb.WriteString("\n")
	} else if d.Zones != nil && !d.Zones.Valid {
		sb.WriteString("🎯 <b>Setup:</b> ❌ Belum ada setup valid\n")
		sb.WriteString(fmt.Sprintf("  <i>%s</i>\n\n", html.EscapeString(d.Zones.Reasoning)))
	}

	// MTF
	if state.mtf != nil && len(state.mtf.Matrix) > 0 {
		sb.WriteString("📊 <b>Multi-Timeframe:</b>\n")
		var mtfParts []string
		for _, row := range state.mtf.Matrix {
			dirE := "⚪"
			if row.Direction == "BULLISH" {
				dirE = "🟢"
			} else if row.Direction == "BEARISH" {
				dirE = "🔴"
			}
			mtfParts = append(mtfParts, fmt.Sprintf("%s%s %s", dirE, row.Timeframe, row.Grade))
		}
		sb.WriteString(strings.Join(mtfParts, " | "))

		mtfNote := ""
		switch state.mtf.Alignment {
		case "STRONG_BULLISH":
			mtfNote = "semua timeframe sejajar bullish — sinyal sangat kuat"
		case "BULLISH":
			mtfNote = "mayoritas timeframe bullish"
		case "STRONG_BEARISH":
			mtfNote = "semua timeframe sejajar bearish — sinyal sangat kuat"
		case "BEARISH":
			mtfNote = "mayoritas timeframe bearish"
		default:
			mtfNote = "sinyal campur antar timeframe — berhati-hati"
		}
		sb.WriteString(fmt.Sprintf("\n<i>%s (Skor MTF: %+.0f, Grade %s)</i>",
			mtfNote, state.mtf.WeightedScore, state.mtf.WeightedGrade))
	}

	// Disclaimer
	sb.WriteString("\n\n<i>⚠️ Ini analisis teknikal otomatis, bukan saran keuangan. Selalu kelola risiko Anda.</i>")

	return sb.String()
}

func formatCTAIchimoku(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>🏯 Ichimoku Cloud Detail</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Ichimoku = sistem tren lengkap: mengukur momentum, support/resistance, dan arah tren sekaligus.</i>\n\n")

	for _, entry := range []struct {
		tf     string
		result *ta.FullResult
	}{
		{"Daily", state.daily},
		{"4H", state.h4},
		{"1H", state.h1},
		{"15m", state.m15},
	} {
		if entry.result == nil || entry.result.Snapshot == nil || entry.result.Snapshot.Ichimoku == nil {
			continue
		}
		ich := entry.result.Snapshot.Ichimoku

		overallEmoji := "⚪"
		switch ich.Overall {
		case "STRONG_BULLISH":
			overallEmoji = "🟢🟢"
		case "BULLISH":
			overallEmoji = "🟢"
		case "STRONG_BEARISH":
			overallEmoji = "🔴🔴"
		case "BEARISH":
			overallEmoji = "🔴"
		}

		sb.WriteString(fmt.Sprintf("<b>%s: %s %s</b>\n", entry.tf, overallEmoji, ich.Overall))
		sb.WriteString(fmt.Sprintf("  Tenkan: %.4f | Kijun: %.4f\n", ich.Tenkan, ich.Kijun))

		// TK Cross explanation
		switch ich.TKCross {
		case "BULLISH_CROSS":
			sb.WriteString("  TK Cross: ✅ Bullish — <i>garis cepat memotong ke atas garis lambat (sinyal beli)</i>\n")
		case "BEARISH_CROSS":
			sb.WriteString("  TK Cross: ❌ Bearish — <i>garis cepat memotong ke bawah garis lambat (sinyal jual)</i>\n")
		default:
			sb.WriteString("  TK Cross: Tidak ada\n")
		}

		// Kumo explanation
		switch ich.KumoBreakout {
		case "BULLISH_BREAKOUT":
			sb.WriteString("  Kumo: ✅ Di atas awan — <i>harga berada di zona bullish, awan menjadi support</i>\n")
		case "BEARISH_BREAKOUT":
			sb.WriteString("  Kumo: ❌ Di bawah awan — <i>harga berada di zona bearish, awan menjadi resistance</i>\n")
		case "INSIDE_CLOUD":
			sb.WriteString("  Kumo: ⚠️ Di dalam awan — <i>zona transisi/netral, arah belum jelas</i>\n")
		default:
			sb.WriteString(fmt.Sprintf("  Kumo: %s\n", ich.KumoBreakout))
		}

		sb.WriteString(fmt.Sprintf("  Cloud: %s | Chikou: %s\n\n", ich.CloudColor, ich.ChikouSignal))
	}

	return sb.String()
}

func formatCTAFibonacci(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>📐 Fibonacci Retracement</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Level Fibonacci = area support/resistance kunci. Harga sering berbalik arah di level 38.2%, 50%, atau 61.8%.</i>\n\n")

	for _, entry := range []struct {
		tf     string
		result *ta.FullResult
	}{
		{"Daily", state.daily},
		{"4H", state.h4},
	} {
		if entry.result == nil || entry.result.Snapshot == nil || entry.result.Snapshot.Fibonacci == nil {
			continue
		}
		fib := entry.result.Snapshot.Fibonacci

		trendLabel := "⬆️ Uptrend"
		if fib.TrendDir == "DOWN" {
			trendLabel = "⬇️ Downtrend"
		}

		sb.WriteString(fmt.Sprintf("<b>%s:</b> %s\n", entry.tf, trendLabel))
		sb.WriteString(fmt.Sprintf("  Swing High: %.4f | Swing Low: %.4f\n", fib.SwingHigh, fib.SwingLow))
		sb.WriteString(fmt.Sprintf("  Level terdekat: <b>%s%%</b> = %.4f\n", fib.NearestLevel, fib.NearestPrice))

		for _, lvl := range []string{"23.6", "38.2", "50", "61.8", "78.6"} {
			if val, ok := fib.Levels[lvl]; ok {
				marker := ""
				if lvl == fib.NearestLevel {
					marker = " ◀ <b>terdekat</b>"
				}
				// Add contextual notes for key levels
				note := ""
				switch lvl {
				case "38.2":
					note = " <i>(shallow retracement)</i>"
				case "50":
					note = " <i>(mid-level kunci)</i>"
				case "61.8":
					note = " <i>(golden ratio — level terpenting)</i>"
				}
				sb.WriteString(fmt.Sprintf("  %s%%: %.4f%s%s\n", lvl, val, marker, note))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatCTAPatterns(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>🕯 Pola Candlestick</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Pola candlestick = formasi harga yang mengindikasikan potensi pergerakan selanjutnya. ★ = tingkat keandalan.</i>\n\n")

	found := false
	for _, entry := range []struct {
		tf     string
		result *ta.FullResult
	}{
		{"Daily", state.daily},
		{"4H", state.h4},
		{"1H", state.h1},
		{"15m", state.m15},
	} {
		if entry.result == nil || len(entry.result.Patterns) == 0 {
			continue
		}
		found = true
		sb.WriteString(fmt.Sprintf("<b>%s:</b>\n", entry.tf))
		for _, p := range entry.result.Patterns {
			dirEmoji := "⚪"
			if p.Direction == "BULLISH" {
				dirEmoji = "🟢"
			} else if p.Direction == "BEARISH" {
				dirEmoji = "🔴"
			}
			stars := strings.Repeat("★", p.Reliability) + strings.Repeat("☆", 3-p.Reliability)
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b> %s\n", dirEmoji, p.Name, stars))
			sb.WriteString(fmt.Sprintf("    <i>%s</i>\n", p.Description))
		}
		sb.WriteString("\n")
	}

	if !found {
		sb.WriteString("Tidak ada pola candlestick terdeteksi saat ini.\n")
		sb.WriteString("<i>Ini normal — pola hanya muncul pada kondisi pasar tertentu.</i>\n")
	}

	return sb.String()
}

func formatCTAConfluence(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>⚡ Detail Confluence</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s — Daily</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Confluence = skor gabungan dari semua indikator. Semakin banyak yang sejajar, semakin kuat sinyal.</i>\n\n")

	if state.daily == nil || state.daily.Confluence == nil {
		sb.WriteString("⚠️ Data confluence belum tersedia.\n")
		return sb.String()
	}

	conf := state.daily.Confluence
	dirEmoji := "⚪"
	if conf.Direction == "BULLISH" {
		dirEmoji = "🟢"
	} else if conf.Direction == "BEARISH" {
		dirEmoji = "🔴"
	}

	sb.WriteString(fmt.Sprintf("%s Skor: <b>%+.1f</b> | Grade: <b>%s</b> | Arah: <b>%s</b>\n",
		dirEmoji, conf.Score, conf.Grade, conf.Direction))
	sb.WriteString(fmt.Sprintf("Bullish: %d | Bearish: %d | Netral: %d dari %d indikator\n\n",
		conf.BullishCount, conf.BearishCount, conf.NeutralCount, conf.TotalIndicators))

	sb.WriteString("<b>Sinyal per-indikator:</b>\n")
	for _, sig := range conf.Signals {
		signalEmoji := "⚪"
		if sig.Value > 0.05 {
			signalEmoji = "🟢"
		} else if sig.Value < -0.05 {
			signalEmoji = "🔴"
		}
		sb.WriteString(fmt.Sprintf("  %s <b>%s</b>: %+.2f (bobot: %.0f%%)\n",
			signalEmoji, sig.Indicator, sig.Value, sig.Weight*100))
		if sig.Note != "" {
			sb.WriteString(fmt.Sprintf("    <i>%s</i>\n", html.EscapeString(sig.Note)))
		}
	}

	// Divergences
	if state.daily.Divergences != nil && len(state.daily.Divergences) > 0 {
		sb.WriteString("\n<b>Divergence:</b>\n")
		for _, div := range state.daily.Divergences {
			divEmoji := "🔄"
			if strings.Contains(div.Type, "BULLISH") {
				divEmoji = "🟢🔄"
			} else if strings.Contains(div.Type, "BEARISH") {
				divEmoji = "🔴🔄"
			}
			sb.WriteString(fmt.Sprintf("  %s %s (%s) — kekuatan: %.0f%%\n",
				divEmoji, div.Type, div.Indicator, div.Strength*100))
			sb.WriteString(fmt.Sprintf("    <i>%s</i>\n", div.Description))
		}
	}

	return sb.String()
}

func formatCTAMTF(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>📊 Multi-Timeframe Matrix</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>MTF = membandingkan sinyal dari berbagai timeframe. Jika semua sejajar, sinyal lebih kuat.</i>\n\n")

	if state.mtf == nil || len(state.mtf.Matrix) == 0 {
		sb.WriteString("⚠️ Data MTF belum tersedia.\n")
		return sb.String()
	}

	// Alignment with explanation
	alignEmoji := "⚪"
	alignNote := ""
	switch state.mtf.Alignment {
	case "STRONG_BULLISH":
		alignEmoji = "🟢🟢"
		alignNote = "Semua timeframe bullish — kepercayaan sangat tinggi"
	case "BULLISH":
		alignEmoji = "🟢"
		alignNote = "Mayoritas timeframe bullish"
	case "STRONG_BEARISH":
		alignEmoji = "🔴🔴"
		alignNote = "Semua timeframe bearish — kepercayaan sangat tinggi"
	case "BEARISH":
		alignEmoji = "🔴"
		alignNote = "Mayoritas timeframe bearish"
	default:
		alignEmoji = "🟡"
		alignNote = "Sinyal campur — berhati-hati, arah belum jelas"
	}

	sb.WriteString(fmt.Sprintf("%s Alignment: <b>%s</b>\n", alignEmoji, state.mtf.Alignment))
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", alignNote))
	sb.WriteString(fmt.Sprintf("Skor Tertimbang: <b>%+.1f</b> (Grade %s)\n\n", state.mtf.WeightedScore, state.mtf.WeightedGrade))

	// Matrix as clean list (better for mobile than <code> table)
	for _, row := range state.mtf.Matrix {
		dirEmoji := "⚪"
		if row.Direction == "BULLISH" {
			dirEmoji = "🟢"
		} else if row.Direction == "BEARISH" {
			dirEmoji = "🔴"
		}
		weightStr := ""
		if row.Weight > 0 {
			weightStr = fmt.Sprintf(" (bobot: %.0f%%)", row.Weight*100)
		}
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>: %s Skor %+.0f Grade %s%s\n",
			dirEmoji, row.Timeframe, row.Direction, row.Score, row.Grade, weightStr))
	}

	return sb.String()
}

func formatCTAZones(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>🎯 Entry/Exit Zones</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Zone = area masuk/keluar yang dihitung dari level support, resistance, dan ATR.</i>\n\n")

	for _, entry := range []struct {
		tf     string
		result *ta.FullResult
	}{
		{"Daily", state.daily},
		{"4H", state.h4},
	} {
		if entry.result == nil || entry.result.Zones == nil {
			continue
		}
		z := entry.result.Zones
		sb.WriteString(fmt.Sprintf("<b>%s:</b>\n", entry.tf))
		if !z.Valid {
			sb.WriteString(fmt.Sprintf("  ❌ Tidak ada setup valid.\n"))
			sb.WriteString(fmt.Sprintf("  <i>%s</i>\n\n", html.EscapeString(z.Reasoning)))
			continue
		}

		dirEmoji := "🟢"
		dirNote := "BUY (masuk posisi beli)"
		if z.Direction == "SHORT" {
			dirEmoji = "🔴"
			dirNote = "SELL (masuk posisi jual)"
		}

		confLabel := ""
		switch z.Confidence {
		case "HIGH":
			confLabel = "✅ Tinggi"
		case "MEDIUM":
			confLabel = "🟡 Sedang"
		default:
			confLabel = "⚠️ Rendah"
		}

		sb.WriteString(fmt.Sprintf("  %s <b>%s</b> — %s\n", dirEmoji, z.Direction, dirNote))
		sb.WriteString(fmt.Sprintf("  Kepercayaan: %s\n", confLabel))
		sb.WriteString(fmt.Sprintf("  📍 Entry: %.4f – %.4f\n", z.EntryLow, z.EntryHigh))
		sb.WriteString(fmt.Sprintf("  🛑 Stop Loss: %.4f\n", z.StopLoss))
		sb.WriteString(fmt.Sprintf("  ✅ TP1: %.4f (R:R 1:%.1f)\n", z.TakeProfit1, z.RiskReward1))
		sb.WriteString(fmt.Sprintf("  ✅ TP2: %.4f (R:R 1:%.1f)\n", z.TakeProfit2, z.RiskReward2))

		if z.Confidence == "LOW" {
			sb.WriteString("  <i>⚠️ Setup berkepercayaan rendah — pertimbangkan menunggu konfirmasi lebih lanjut.</i>\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("<i>R:R = Risk:Reward. Contoh 1:2.0 = setiap $1 risiko, potensi profit $2.</i>\n")

	return sb.String()
}

func formatCTATimeframeDetail(state *ctaState, tf string, result *ta.FullResult) string {
	var sb strings.Builder
	tfLabel := strings.ToUpper(tf)
	sb.WriteString(fmt.Sprintf("<b>⚡ TA: %s — %s</b>\n\n", html.EscapeString(state.symbol), tfLabel))

	if result == nil || result.Confluence == nil {
		sb.WriteString("⚠️ Data tidak cukup.\n")
		return sb.String()
	}

	conf := result.Confluence
	dirEmoji := "🔵"
	if conf.Direction == "BULLISH" {
		dirEmoji = "🟢"
	} else if conf.Direction == "BEARISH" {
		dirEmoji = "🔴"
	}

	sb.WriteString(fmt.Sprintf("%s <b>%s</b> Skor: %+.0f Grade: %s\n",
		dirEmoji, conf.Direction, conf.Score, conf.Grade))
	sb.WriteString(fmt.Sprintf("Confluence: %d bull / %d bear / %d netral\n\n",
		conf.BullishCount, conf.BearishCount, conf.NeutralCount))

	snap := result.Snapshot
	if snap != nil {
		sb.WriteString("<b>Indikator:</b>\n")

		if snap.RSI != nil {
			rsiNote := ""
			if snap.RSI.Zone == "OVERBOUGHT" {
				rsiNote = " — ⚠️ jenuh beli"
			} else if snap.RSI.Zone == "OVERSOLD" {
				rsiNote = " — 💡 jenuh jual, potensi rebound"
			} else if snap.RSI.Value >= 55 {
				rsiNote = " — momentum bullish"
			} else if snap.RSI.Value <= 45 {
				rsiNote = " — momentum bearish"
			} else {
				rsiNote = " — netral"
			}
			sb.WriteString(fmt.Sprintf("• RSI(14): <b>%.1f</b>%s\n", snap.RSI.Value, rsiNote))
		}
		if snap.MACD != nil {
			sb.WriteString(fmt.Sprintf("• MACD: %.4f | Signal: %.4f | Hist: %+.4f\n",
				snap.MACD.MACD, snap.MACD.Signal, snap.MACD.Histogram))
			if snap.MACD.BullishCross {
				sb.WriteString("  ↳ ✅ Bullish cross — <i>momentum naik baru dimulai</i>\n")
			} else if snap.MACD.BearishCross {
				sb.WriteString("  ↳ ❌ Bearish cross — <i>momentum turun baru dimulai</i>\n")
			}
		}
		if snap.Stochastic != nil {
			stochNote := ""
			if snap.Stochastic.Zone == "OVERBOUGHT" {
				stochNote = " ⚠️ overbought"
			} else if snap.Stochastic.Zone == "OVERSOLD" {
				stochNote = " 💡 oversold"
			}
			sb.WriteString(fmt.Sprintf("• Stoch: %%K=%.1f %%D=%.1f%s\n",
				snap.Stochastic.K, snap.Stochastic.D, stochNote))
		}
		if snap.ADX != nil {
			adxNote := ""
			switch snap.ADX.TrendStrength {
			case "STRONG":
				adxNote = " — tren sangat kuat"
			case "MODERATE":
				adxNote = " — tren moderate"
			default:
				adxNote = " — tidak trending (sideways)"
			}
			di := "↗️"
			if snap.ADX.MinusDI > snap.ADX.PlusDI {
				di = "↘️"
			}
			sb.WriteString(fmt.Sprintf("• ADX: %.1f %s (+DI=%.1f -DI=%.1f)%s\n",
				snap.ADX.ADX, di, snap.ADX.PlusDI, snap.ADX.MinusDI, adxNote))
		}
		if snap.Bollinger != nil {
			bbNote := ""
			if snap.Bollinger.Squeeze {
				bbNote = " 💥 SQUEEZE"
			}
			sb.WriteString(fmt.Sprintf("• BB: %%B=%.2f BW=%.2f%s\n",
				snap.Bollinger.PercentB, snap.Bollinger.Bandwidth, bbNote))
		}
		if snap.WilliamsR != nil {
			sb.WriteString(fmt.Sprintf("• Williams %%R: %.1f (%s)\n",
				snap.WilliamsR.Value, snap.WilliamsR.Zone))
		}
		if snap.CCI != nil {
			sb.WriteString(fmt.Sprintf("• CCI: %.1f (%s)\n", snap.CCI.Value, snap.CCI.Zone))
		}
	}

	if len(result.Patterns) > 0 {
		sb.WriteString("\n🕯 <b>Pola:</b>\n")
		for _, p := range result.Patterns {
			dirE := "⚪"
			if p.Direction == "BULLISH" {
				dirE = "🟢"
			} else if p.Direction == "BEARISH" {
				dirE = "🔴"
			}
			stars := strings.Repeat("★", p.Reliability) + strings.Repeat("☆", 3-p.Reliability)
			sb.WriteString(fmt.Sprintf("  %s %s %s\n", dirE, p.Name, stars))
		}
	}

	if result.Zones != nil && result.Zones.Valid {
		z := result.Zones
		sb.WriteString(fmt.Sprintf("\n🎯 <b>%s Setup:</b>\n", z.Direction))
		sb.WriteString(fmt.Sprintf("  Entry: %.4f – %.4f | SL: %.4f\n",
			z.EntryLow, z.EntryHigh, z.StopLoss))
		sb.WriteString(fmt.Sprintf("  TP1: %.4f (R:R 1:%.1f) | TP2: %.4f (R:R 1:%.1f)\n",
			z.TakeProfit1, z.RiskReward1, z.TakeProfit2, z.RiskReward2))
	} else if result.Zones != nil && !result.Zones.Valid {
		sb.WriteString("\n🎯 <b>Setup:</b> ❌ Tidak ada setup valid\n")
	}

	return sb.String()
}

// Unused import guard
var _ = math.Abs
