package telegram

// handler_wyckoff.go — /wyckoff command handler with inline keyboard navigation.
// Implements Wyckoff Method structure detection (Accumulation/Distribution).

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	"github.com/arkcode369/ark-intelligent/internal/service/wyckoff"
)

// ---------------------------------------------------------------------------
// Wyckoff state cache — tracks last analysis per chat for callbacks
// ---------------------------------------------------------------------------

type wyckoffState struct {
	symbol    string
	timeframe string
	result    *wyckoff.WyckoffResult
	createdAt time.Time
}

const wyckoffStateTTL = 5 * time.Minute

type wyckoffStateCache struct {
	mu    sync.Mutex
	store map[string]*wyckoffState // chatID → state
}

func newWyckoffStateCache() *wyckoffStateCache {
	return &wyckoffStateCache{store: make(map[string]*wyckoffState)}
}

func (c *wyckoffStateCache) get(chatID string) *wyckoffState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.createdAt) > wyckoffStateTTL {
		delete(c.store, chatID)
		return nil
	}
	return s
}

func (c *wyckoffStateCache) set(chatID string, s *wyckoffState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	// Evict stale entries
	for k, v := range c.store {
		if now.Sub(v.createdAt) > wyckoffStateTTL*2 {
			delete(c.store, k)
		}
	}
	c.store[chatID] = s
}

// ---------------------------------------------------------------------------
// WyckoffServices — injected dependencies
// ---------------------------------------------------------------------------

// WyckoffServices holds dependencies for the /wyckoff command.
type WyckoffServices struct {
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore // may be nil
	WyckoffEngine  *wyckoff.Engine
}

// ---------------------------------------------------------------------------
// WithWyckoff wires the Wyckoff handler into the main Handler.
// ---------------------------------------------------------------------------

func (h *Handler) WithWyckoff(svc WyckoffServices) {
	h.wyckoff = &svc
	h.wyckoffCache = newWyckoffStateCache()
	h.bot.RegisterCommand("/wyckoff", h.cmdWyckoff)
	h.bot.RegisterCallback("wck:", h.handleWyckoffCallback)
}

// ---------------------------------------------------------------------------
// Top forex symbols for the symbol picker
// ---------------------------------------------------------------------------

var wyckoffSymbols = []string{"EURUSD", "GBPUSD", "USDJPY", "AUDUSD", "USDCHF", "XAUUSD"}

// ---------------------------------------------------------------------------
// /wyckoff command
// ---------------------------------------------------------------------------

// cmdWyckoff handles /wyckoff [SYMBOL] [TIMEFRAME]
func (h *Handler) cmdWyckoff(ctx context.Context, chatID string, userID int64, args string) error {
	if h.wyckoff == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚠️ Wyckoff engine tidak tersedia.")
		return err
	}

	parts := strings.Fields(strings.TrimSpace(strings.ToUpper(args)))
	if len(parts) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, `📊 <b>Wyckoff Structure Analysis</b>

Gunakan: <code>/wyckoff [SYMBOL] [TIMEFRAME]</code>

Contoh:
  <code>/wyckoff EURUSD</code>
  <code>/wyckoff XAUUSD H4</code>
  <code>/wyckoff BTCUSD daily</code>

Timeframe yang didukung: <code>daily</code>, <code>4h</code>, <code>1h</code>`)
		return err
	}

	currency := parts[0]
	timeframe := "daily"
	if len(parts) > 1 {
		timeframe = normalizeWyckoffTF(parts[1])
	}

	mapping := domain.FindPriceMappingByCurrency(currency)
	if mapping == nil || mapping.RiskOnly {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("❌ Symbol tidak dikenal: <code>%s</code>", html.EscapeString(currency)))
		return err
	}

	// Loading indicator
	msgID, _ := h.bot.SendLoading(ctx, chatID,
		fmt.Sprintf("⏳ Menganalisis Wyckoff Structure <b>%s</b> (%s)...",
			html.EscapeString(mapping.Currency), timeframe))

	bars, err := h.fetchWyckoffBars(ctx, mapping, timeframe)
	if err != nil || len(bars) == 0 {
		errMsg := fmt.Sprintf("❌ Gagal mengambil data harga untuk <b>%s</b>: %s",
			html.EscapeString(mapping.Currency), html.EscapeString(fmt.Sprintf("%v", err)))
		if msgID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	result := h.wyckoff.WyckoffEngine.Analyze(mapping.Currency, strings.ToUpper(timeframe), bars)

	// Cache state for callback navigation
	h.wyckoffCache.set(chatID, &wyckoffState{
		symbol:    mapping.Currency,
		timeframe: timeframe,
		result:    result,
		createdAt: time.Now(),
	})

	output := h.fmt.FormatWyckoffResult(result)
	kb := wyckoffNavKeyboard(mapping.Currency, timeframe)

	if msgID > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, output, kb)
	}
	_, sendErr := h.bot.SendWithKeyboard(ctx, chatID, output, kb)
	return sendErr
}

// ---------------------------------------------------------------------------
// Keyboard builder
// ---------------------------------------------------------------------------

func wyckoffNavKeyboard(currentSym, currentTF string) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	// Row 1: Symbol selector (top forex pairs)
	var symRow []ports.InlineButton
	for _, s := range wyckoffSymbols {
		label := s
		if strings.EqualFold(s, currentSym) {
			label = "● " + s
		}
		symRow = append(symRow, ports.InlineButton{
			Text:         label,
			CallbackData: "wck:sym:" + s,
		})
	}
	// Split symbol row into 2 rows of 3 for better mobile layout
	if len(symRow) > 3 {
		rows = append(rows, symRow[:3])
		rows = append(rows, symRow[3:])
	} else {
		rows = append(rows, symRow)
	}

	// Row 3: Timeframe toggle
	tfRow := []ports.InlineButton{
		{Text: wyckoffTFLabel("H1", currentTF), CallbackData: "wck:tf:" + currentSym + ":1h"},
		{Text: wyckoffTFLabel("H4", currentTF), CallbackData: "wck:tf:" + currentSym + ":4h"},
		{Text: wyckoffTFLabel("D1", currentTF), CallbackData: "wck:tf:" + currentSym + ":daily"},
	}
	rows = append(rows, tfRow)

	// Row 4: Refresh + related commands
	relatedRow := []ports.InlineButton{
		{Text: "🔄 Refresh", CallbackData: "wck:refresh:"},
		{Text: "📊 CTA", CallbackData: "wck:goto:cta:" + currentSym},
		{Text: "🏗️ ICT", CallbackData: "wck:goto:ict:" + currentSym},
	}
	rows = append(rows, relatedRow)

	return ports.InlineKeyboard{Rows: rows}
}

// wyckoffTFLabel marks the active timeframe with a checkmark.
func wyckoffTFLabel(label, currentTF string) string {
	norm := strings.ToUpper(currentTF)
	switch norm {
	case "1H":
		norm = "H1"
	case "4H":
		norm = "H4"
	case "DAILY":
		norm = "D1"
	}
	if label == norm {
		return "✅ " + label
	}
	return label
}

// normalizeWyckoffTF normalizes timeframe aliases.
func normalizeWyckoffTF(tf string) string {
	switch strings.ToLower(tf) {
	case "h4", "4hour", "4h":
		return "4h"
	case "h1", "1hour", "1h":
		return "1h"
	default:
		return "daily"
	}
}

// ---------------------------------------------------------------------------
// Callback handler
// ---------------------------------------------------------------------------

// handleWyckoffCallback handles inline keyboard callbacks for /wyckoff.
// data format: "wck:<action>:<payload>"
func (h *Handler) handleWyckoffCallback(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	if h.wyckoff == nil {
		return nil
	}

	parts := strings.SplitN(strings.TrimPrefix(data, "wck:"), ":", 2)
	action := parts[0]
	payload := ""
	if len(parts) > 1 {
		payload = parts[1]
	}

	switch action {
	case "sym":
		// User selected a symbol — re-run with cached or default timeframe
		tf := "daily"
		if state := h.wyckoffCache.get(chatID); state != nil {
			tf = state.timeframe
		}
		return h.runWyckoffAnalysis(ctx, chatID, msgID, payload, tf)

	case "tf":
		// User changed timeframe. payload = "SYMBOL:TIMEFRAME"
		p2 := strings.SplitN(payload, ":", 2)
		if len(p2) < 2 {
			return nil
		}
		return h.runWyckoffAnalysis(ctx, chatID, msgID, p2[0], normalizeWyckoffTF(p2[1]))

	case "refresh":
		// Refresh current analysis
		state := h.wyckoffCache.get(chatID)
		if state == nil {
			_, err := h.bot.SendHTML(ctx, chatID, sessionExpiredMessage("wyckoff"))
			return err
		}
		return h.runWyckoffAnalysis(ctx, chatID, msgID, state.symbol, state.timeframe)

	case "goto":
		// Navigate to related command. payload = "COMMAND:SYMBOL"
		p2 := strings.SplitN(payload, ":", 2)
		if len(p2) < 2 {
			return nil
		}
		cmd, sym := p2[0], p2[1]
		switch cmd {
		case "cta":
			return h.cmdCTA(ctx, chatID, userID, sym)
		case "ict":
			return h.cmdICT(ctx, chatID, userID, sym)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// runWyckoffAnalysis — shared analysis + keyboard update
// ---------------------------------------------------------------------------

func (h *Handler) runWyckoffAnalysis(ctx context.Context, chatID string, msgID int, symbol, timeframe string) error {
	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil || mapping.RiskOnly {
		return nil
	}

	bars, err := h.fetchWyckoffBars(ctx, mapping, timeframe)
	if err != nil || len(bars) == 0 {
		errMsg := fmt.Sprintf("⚠️ Gagal mengambil data <b>%s</b>: %v",
			html.EscapeString(mapping.Currency), err)
		kb := wyckoffNavKeyboard(mapping.Currency, timeframe)
		_ = h.bot.EditWithKeyboard(ctx, chatID, msgID, errMsg, kb)
		return nil
	}

	result := h.wyckoff.WyckoffEngine.Analyze(mapping.Currency, strings.ToUpper(timeframe), bars)

	h.wyckoffCache.set(chatID, &wyckoffState{
		symbol:    mapping.Currency,
		timeframe: timeframe,
		result:    result,
		createdAt: time.Now(),
	})

	output := h.fmt.FormatWyckoffResult(result)
	kb := wyckoffNavKeyboard(mapping.Currency, timeframe)
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, output, kb)
}

// ---------------------------------------------------------------------------
// fetchWyckoffBars — fetch OHLCV bars for Wyckoff analysis
// ---------------------------------------------------------------------------

func (h *Handler) fetchWyckoffBars(ctx context.Context, mapping *domain.PriceSymbolMapping, timeframe string) ([]ta.OHLCV, error) {
	code := mapping.ContractCode

	switch timeframe {
	case "4h", "1h":
		if h.wyckoff.IntradayRepo == nil {
			return nil, fmt.Errorf("intraday data tidak tersedia")
		}
		count := 300
		intradayBars, err := h.wyckoff.IntradayRepo.GetHistory(ctx, code, timeframe, count)
		if err != nil {
			return nil, fmt.Errorf("fetch intraday bars: %w", err)
		}
		return ta.IntradayBarsToOHLCV(intradayBars), nil

	default: // "daily"
		dailyRecords, err := h.wyckoff.DailyPriceRepo.GetDailyHistory(ctx, code, 300)
		if err != nil {
			return nil, fmt.Errorf("fetch daily bars: %w", err)
		}
		return ta.DailyPricesToOHLCV(dailyRecords), nil
	}
}
