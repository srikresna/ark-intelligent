package telegram

// handler_cta.go — /cta command: Classical Technical Analysis dashboard
//   /cta [SYMBOL] [TIMEFRAME]  — TA dashboard with chart + inline keyboard

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// CTAServices — dependencies for the /cta command

// CTAServices holds the services required for the CTA command.
type CTAServices struct {
	TAEngine       *ta.Engine
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
	PriceMapping   []domain.PriceSymbolMapping
	RegimeEngine   RegimeOverlayEngine // optional — nil disables overlay header
}

// ctaState — cached computation results

type ctaState struct {
	symbol        string
	currency      string
	contractCode  string
	daily         *ta.FullResult
	h4            *ta.FullResult
	h1            *ta.FullResult
	m15           *ta.FullResult
	m30           *ta.FullResult
	h6            *ta.FullResult
	h12           *ta.FullResult
	weekly        *ta.FullResult
	mtf           *ta.MTFResult
	bars          map[string][]ta.OHLCV // timeframe -> bars
	chartData     map[string][]byte     // timeframe -> PNG bytes (lazy-generated)
	regimeOverlay RegimeHeaderProvider  // optional regime overlay header
	computedAt    time.Time
}

var ctaStateTTL = config.CTAStateTTL

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

// Handler fields (stored in Handler struct)

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

// /cta — Main CTA Command

func (h *Handler) cmdCTA(ctx context.Context, chatID string, userID int64, args string) error {
	if h.cta == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ CTA Engine not configured.")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) == 0 {
		// Auto-reload last currency when no args provided
		if lc := h.getLastCurrency(ctx, userID); lc != "" {
			parts = []string{lc}
			_, _ = h.bot.SendHTML(ctx, chatID, fmt.Sprintf("🔄 Loading <b>%s</b> (last viewed)...", html.EscapeString(lc)))
		} else {
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

	// Multi-step progress indicator
	sym := html.EscapeString(mapping.Currency)
	prog := NewProgress(h.bot, chatID, []string{
		fmt.Sprintf("⏳ Fetching price data for <b>%s</b>...", sym),
		fmt.Sprintf("🔄 Running technical analysis for <b>%s</b>...", sym),
		fmt.Sprintf("📊 Generating charts for <b>%s</b>...", sym),
		fmt.Sprintf("✨ Finalizing CTA report for <b>%s</b>...", sym),
	})
	prog.Start(ctx)

	// Compute CTA state
	state, err := h.computeCTAState(ctx, mapping)
	if err != nil {
		prog.Stop(ctx)
		h.sendUserError(ctx, chatID, err, "cta")
		return nil
	}

	h.ctaCache.set(chatID, state)
	h.saveLastCurrency(ctx, userID, mapping.Currency)

	// Compute regime overlay (best-effort, non-blocking)
	if h.cta.RegimeEngine != nil {
		if overlay, rErr := h.cta.RegimeEngine.ComputeOverlay(ctx, mapping.ContractCode, mapping.Currency, "daily"); rErr == nil {
			state.regimeOverlay = overlay
		}
	}

	// Generate chart for daily timeframe
	chartPNG, chartErr := h.generateCTAChart(state, "daily")
	if chartErr != nil {
		log.Error().Err(chartErr).Str("symbol", symbol).Str("timeframe", "daily").Msg("CTA chart generation failed, falling back to text")
	}

	// Delete progress message
	prog.Stop(ctx)

	// Format summary
	summary := formatCTASummary(state, state.regimeOverlay)
	kb := h.kb.CTAMenu()

	// Send photo with keyboard if chart available, otherwise text + notification
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

	// Chart unavailable: prepend notification so user knows chart exists but failed
	if chartErr != nil {
		summary = "📊 <i>Chart sementara tidak tersedia. Menampilkan analisis teks.</i>\n\n" + summary
	}
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, summary, kb)
	return err
}

// Callback Handler

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
		_, err := h.bot.SendHTML(ctx, chatID, sessionExpiredMessage("cta"))
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
		chartPNG, chartErr := h.generateCTADetailChart(ctx, state, "daily", "ichimoku")
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
		chartPNG, chartErr := h.generateCTADetailChart(ctx, state, "daily", "fibonacci")
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

	case action == "vwap_delta":
		txt := formatCTAVWAPDelta(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, txt, kb)
		return err

	case action == "zones":
		txt := formatCTAZones(state)
		kb := h.kb.CTADetailMenu()
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		chartPNG, chartErr := h.generateCTADetailChart(ctx, state, "daily", "zones")
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
	chartPNG, chartErr := h.getCTAChart(state, tf)
	if chartErr != nil || len(chartPNG) == 0 {
		if chartErr != nil {
			log.Error().Err(chartErr).Str("symbol", state.symbol).Str("timeframe", tf).Msg("CTA summary chart failed, falling back to text")
		}
		// Fallback to text with chart failure notification
		fallbackNotice := "📊 <i>Chart sementara tidak tersedia. Menampilkan analisis teks.</i>\n\n"
		summary := formatCTASummary(state)
		kb := h.kb.CTAMenu()
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, fallbackNotice+summary, kb)
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

	chartPNG, chartErr := h.getCTAChart(state, tf)
	if chartErr != nil {
		log.Error().Err(chartErr).Str("symbol", state.symbol).Str("timeframe", tf).Msg("CTA timeframe chart failed, falling back to text")
	}

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

	// Text fallback with chart failure notification
	if chartErr != nil {
		fallbackNotice := "📊 <i>Chart sementara tidak tersedia. Menampilkan analisis teks.</i>\n\n"
		txt = fallbackNotice + txt
	}
	return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, txt, kb)
}

// Data Fetching & Computation

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

	daily = engine.ComputeFullForTF(barsByTF["daily"], "daily")
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
		symbol:       displaySymbol,
		currency:     mapping.Currency,
		contractCode: mapping.ContractCode,
		daily:        daily,
		h4:           h4,
		h1:           h1,
		m15:          m15,
		m30:          m30,
		h6:           h6,
		h12:          h12,
		weekly:       weekly,
		mtf:          mtf,
		bars:         barsByTF,
		chartData:    make(map[string][]byte),
		computedAt:   time.Now(),
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
