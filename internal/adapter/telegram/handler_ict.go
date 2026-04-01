package telegram

// handler_ict.go — /ict command: ICT/SMC Smart Money Concepts Analysis
//   /ict [SYMBOL] [TIMEFRAME]  — e.g. /ict EURUSD H4

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
	ictsvc "github.com/arkcode369/ark-intelligent/internal/service/ict"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// ICTServices — dependencies for the /ict command
// ---------------------------------------------------------------------------

// ICTServices holds the dependencies needed by the /ict handler.
type ICTServices struct {
	Engine         *ictsvc.Engine
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore // may be nil
}

// ---------------------------------------------------------------------------
// ictState — cached per-chat state
// ---------------------------------------------------------------------------

type ictState struct {
	symbol    string
	timeframe string
	result    *ictsvc.ICTResult
	createdAt time.Time
}

const ictStateTTL = 5 * time.Minute

type ictStateCache struct {
	mu    sync.Mutex
	store map[string]*ictState // chatID → state
}

func newICTStateCache() *ictStateCache {
	return &ictStateCache{store: make(map[string]*ictState)}
}

func (c *ictStateCache) get(chatID string) *ictState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.createdAt) > ictStateTTL {
		delete(c.store, chatID)
		return nil
	}
	return s
}

func (c *ictStateCache) set(chatID string, s *ictState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.store {
		if now.Sub(v.createdAt) > ictStateTTL*2 {
			delete(c.store, k)
		}
	}
	c.store[chatID] = s
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------

// WithICT injects ICTServices into the handler and registers ICT commands.
func (h *Handler) WithICT(svc *ICTServices) *Handler {
	h.ict = svc
	if svc != nil {
		h.ictCache = newICTStateCache()
		h.registerICTCommands()
	}
	return h
}

func (h *Handler) registerICTCommands() {
	h.bot.RegisterCommand("/ict", h.cmdICT)
	h.bot.RegisterCallback("ict:", h.handleICTCallback)
}

// ---------------------------------------------------------------------------
// /ict — Main command
// ---------------------------------------------------------------------------

// cmdICT handles the /ict [SYMBOL] [TIMEFRAME] command.
func (h *Handler) cmdICT(ctx context.Context, chatID string, _ int64, args string) error {
	if h.ict == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ ICT Engine not configured.")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))

	// No args → show symbol selector.
	if len(parts) == 0 {
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`🔷 <b>ICT/SMC Analysis Engine</b>

Analisis Smart Money Concepts:
• Fair Value Gaps (FVG)
• Order Blocks &amp; Breaker Blocks
• Break of Structure (BOS) &amp; CHoCH
• Liquidity Sweeps

Pilih pair:`,
			ictSymbolKeyboard())
		return err
	}

	// Parse symbol and optional timeframe.
	symbol := parts[0]
	timeframe := "4h"
	if len(parts) >= 2 {
		tf := strings.ToLower(parts[1])
		switch tf {
		case "h1", "1h":
			timeframe = "1h"
		case "h4", "4h":
			timeframe = "4h"
		case "d1", "daily", "1d":
			timeframe = "daily"
		}
	}

	// Find price mapping.
	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/ict EUR</code>, <code>/ict EURUSD H4</code>",
			html.EscapeString(symbol)))
		return err
	}

	// Compute ICT analysis.
	result, err := h.computeICTState(ctx, mapping, timeframe)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "ict")
		return err
	}

	state := &ictState{
		symbol:    symbol,
		timeframe: timeframe,
		result:    result,
		createdAt: time.Now(),
	}
	h.ictCache.set(chatID, state)

	msg := FormatICTResult(result)
	kb := ictNavKeyboard(symbol, timeframe)
	_, err = h.bot.SendWithKeyboard(ctx, chatID, msg, kb)
	return err
}

// computeICTState fetches price data and runs the ICT engine.
func (h *Handler) computeICTState(ctx context.Context, mapping *domain.PriceSymbolMapping, timeframe string) (*ictsvc.ICTResult, error) {
	code := mapping.ContractCode
	symbol := mapping.Currency

	var bars []ta.OHLCV

	switch timeframe {
	case "daily":
		records, err := h.ict.DailyPriceRepo.GetDailyHistory(ctx, code, 200)
		if err != nil {
			return nil, fmt.Errorf("fetch daily price: %w", err)
		}
		if len(records) == 0 {
			return nil, fmt.Errorf("no daily price data for %s", symbol)
		}
		bars = ta.DailyPricesToOHLCV(records)
	default:
		// Intraday (1h or 4h).
		if h.ict.IntradayRepo == nil {
			return nil, fmt.Errorf("intraday data not configured")
		}
		count := 200
		intBars, err := h.ict.IntradayRepo.GetHistory(ctx, code, timeframe, count)
		if err != nil {
			return nil, fmt.Errorf("fetch intraday price: %w", err)
		}
		if len(intBars) == 0 {
			return nil, fmt.Errorf("no %s data for %s", timeframe, symbol)
		}
		bars = ta.IntradayBarsToOHLCV(intBars)
	}

	// Run analysis.
	tfLabel := strings.ToUpper(timeframe)
	switch timeframe {
	case "1h":
		tfLabel = "H1"
	case "4h":
		tfLabel = "H4"
	case "daily":
		tfLabel = "D1"
	}

	result := h.ict.Engine.Analyze(bars, symbol, tfLabel)
	return result, nil
}

// ---------------------------------------------------------------------------
// Callback handler
// ---------------------------------------------------------------------------

func (h *Handler) handleICTCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	// data format: "ict:<action>:<payload>"
	parts := strings.SplitN(strings.TrimPrefix(data, "ict:"), ":", 2)
	action := parts[0]
	payload := ""
	if len(parts) > 1 {
		payload = parts[1]
	}

	switch action {
	case "sym":
		// User selected a symbol from the picker.
		return h.cmdICT(ctx, chatID, 0, payload)

	case "tf":
		// User changed timeframe. payload = "SYMBOL:TIMEFRAME"
		p2 := strings.SplitN(payload, ":", 2)
		if len(p2) < 2 {
			return nil
		}
		sym, tf := p2[0], p2[1]
		return h.cmdICT(ctx, chatID, 0, sym+" "+tf)

	case "refresh":
		// Refresh current state.
		state := h.ictCache.get(chatID)
		if state == nil {
			_, err := h.bot.SendHTML(ctx, chatID, "⏰ Session expired. Gunakan <code>/ict</code> lagi.")
			return err
		}
		mapping := domain.FindPriceMappingByCurrency(state.symbol)
		if mapping == nil {
			return nil
		}
		result, err := h.computeICTState(ctx, mapping, state.timeframe)
		if err != nil {
			h.sendUserError(ctx, chatID, err, "ict")
			return err
		}
		state.result = result
		state.createdAt = time.Now()
		h.ictCache.set(chatID, state)

		msg := FormatICTResult(result)
		kb := ictNavKeyboard(state.symbol, state.timeframe)
		err = h.bot.EditWithKeyboard(ctx, chatID, msgID, msg, kb)
		return err
	}

	return nil
}

// ---------------------------------------------------------------------------
// Keyboards
// ---------------------------------------------------------------------------

func ictSymbolKeyboard() ports.InlineKeyboard {
	pairs := []string{"EUR", "GBP", "JPY", "CHF", "AUD", "NZD", "CAD", "XAU"}
	var rows [][]ports.InlineButton
	row := make([]ports.InlineButton, 0, 4)
	for i, p := range pairs {
		row = append(row, ports.InlineButton{
			Text:         p,
			CallbackData: "ict:sym:" + p,
		})
		if len(row) == 4 || i == len(pairs)-1 {
			rows = append(rows, row)
			row = make([]ports.InlineButton, 0, 4)
		}
	}
	return ports.InlineKeyboard{Rows: rows}
}

func ictNavKeyboard(symbol, currentTF string) ports.InlineKeyboard {
	tfRow := []ports.InlineButton{
		{Text: tfLabel("H1", currentTF), CallbackData: "ict:tf:" + symbol + ":1h"},
		{Text: tfLabel("H4", currentTF), CallbackData: "ict:tf:" + symbol + ":4h"},
		{Text: tfLabel("D1", currentTF), CallbackData: "ict:tf:" + symbol + ":daily"},
	}
	actionRow := []ports.InlineButton{
		{Text: "🔄 Refresh", CallbackData: "ict:refresh:"},
		{Text: "◀ Kembali", CallbackData: "ict:sym:"},
	}
	return ports.InlineKeyboard{Rows: [][]ports.InlineButton{tfRow, actionRow}}
}

// tfLabel adds a checkmark to the active timeframe button label.
func tfLabel(label, currentTF string) string {
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
