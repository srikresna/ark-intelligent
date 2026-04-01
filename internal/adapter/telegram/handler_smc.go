package telegram

// handler_smc.go — /smc command: Smart Money Concepts (SMC) Structure Dashboard
//   /smc [SYMBOL] [TIMEFRAME]
//
// Combines ta.SMCResult (from ta/smc.go: BOS/CHOCH/zones) with
// ICT analysis (FVG, Order Blocks) from service/ict.

import (
	"context"
	"fmt"
	"html"
	"math"
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
// SMCServices — dependencies
// ---------------------------------------------------------------------------

// SMCServices holds all dependencies needed by the /smc handler.
type SMCServices struct {
	ICTEngine      *ictsvc.Engine
	TAEngine       *ta.Engine
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore // may be nil
}

// ---------------------------------------------------------------------------
// smcState — per-chat cached state
// ---------------------------------------------------------------------------

type smcState struct {
	symbol    string
	timeframe string
	ictResult *ictsvc.ICTResult
	smcResult *ta.SMCResult
	createdAt time.Time
}

const smcStateTTL = 5 * time.Minute

type smcStateCache struct {
	mu    sync.Mutex
	store map[string]*smcState
}

func newSMCStateCache() *smcStateCache {
	return &smcStateCache{store: make(map[string]*smcState)}
}

func (c *smcStateCache) get(chatID string) *smcState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.createdAt) > smcStateTTL {
		delete(c.store, chatID)
		return nil
	}
	return s
}

func (c *smcStateCache) set(chatID string, s *smcState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.store {
		if now.Sub(v.createdAt) > smcStateTTL*2 {
			delete(c.store, k)
		}
	}
	c.store[chatID] = s
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------

// WithSMC injects SMCServices and registers the /smc command.
func (h *Handler) WithSMC(svc *SMCServices) *Handler {
	h.smc = svc
	if svc != nil {
		h.smcCache = newSMCStateCache()
		h.registerSMCCommands()
	}
	return h
}

func (h *Handler) registerSMCCommands() {
	h.bot.RegisterCommand("/smc", h.cmdSMC)
	h.bot.RegisterCallback("smc:", h.handleSMCCallback)
}

// ---------------------------------------------------------------------------
// /smc — Main command
// ---------------------------------------------------------------------------

func (h *Handler) cmdSMC(ctx context.Context, chatID string, _ int64, args string) error {
	if h.smc == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ SMC Engine not configured.")
		return err
	}

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))

	// No args → show symbol selector
	if len(parts) == 0 {
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`📐 <b>SMC — Smart Money Concepts Dashboard</b>

Market structure analysis:
• BOS &amp; CHoCH (Break of Structure / Change of Character)
• Premium / Discount / Equilibrium zones
• ICT Fair Value Gaps (FVG)
• Order Blocks &amp; Breaker Blocks
• Liquidity sweeps

Pilih pair:`,
			smcSymbolKeyboard())
		return err
	}

	symbol := parts[0]
	timeframe := "4h"
	if len(parts) >= 2 {
		switch strings.ToLower(parts[1]) {
		case "15m", "m15":
			timeframe = "15m"
		case "1h", "h1":
			timeframe = "1h"
		case "4h", "h4":
			timeframe = "4h"
		case "1d", "d1", "daily":
			timeframe = "daily"
		}
	}

	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/smc EUR</code>, <code>/smc EURUSD 4H</code>",
			html.EscapeString(symbol)))
		return err
	}

	// Loading indicator
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf(
		"📐 Analyzing SMC structure for <b>%s</b> %s...",
		html.EscapeString(mapping.Currency), strings.ToUpper(timeframe)))

	state, err := h.computeSMCState(ctx, mapping, timeframe)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	if err != nil {
		h.sendUserError(ctx, chatID, err, "smc")
		return nil
	}

	h.smcCache.set(chatID, state)
	msg := formatSMCOutput(state)
	kb := smcNavKeyboard(symbol, timeframe)
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, msg, kb)
	return err
}

// computeSMCState fetches price data and runs both ICT and SMC engines.
func (h *Handler) computeSMCState(ctx context.Context, mapping *domain.PriceSymbolMapping, timeframe string) (*smcState, error) {
	code := mapping.ContractCode
	symbol := mapping.Currency

	var bars []ta.OHLCV

	switch timeframe {
	case "daily":
		records, err := h.smc.DailyPriceRepo.GetDailyHistory(ctx, code, 200)
		if err != nil || len(records) == 0 {
			return nil, fmt.Errorf("insufficient daily data for %s", symbol)
		}
		bars = ta.DailyPricesToOHLCV(records)
	default:
		if h.smc.IntradayRepo == nil {
			return nil, fmt.Errorf("intraday data not configured")
		}
		intBars, err := h.smc.IntradayRepo.GetHistory(ctx, code, timeframe, 200)
		if err != nil || len(intBars) == 0 {
			return nil, fmt.Errorf("insufficient %s data for %s", timeframe, symbol)
		}
		bars = ta.IntradayBarsToOHLCV(intBars)
	}

	if len(bars) < 30 {
		return nil, fmt.Errorf("data tidak cukup untuk analisis SMC (minimal 30 bar, tersedia %d)", len(bars))
	}

	// ATR for impulse sizing
	atr := ta.CalcATR(bars, 14)

	// SMC from ta package
	smcResult := ta.CalcSMC(bars, atr)

	// ICT from ict service (FVG, Order Blocks, Killzone)
	tfLabel := smcTFLabel(timeframe)
	ictResult := h.smc.ICTEngine.Analyze(bars, symbol, tfLabel)

	return &smcState{
		symbol:    symbol,
		timeframe: timeframe,
		ictResult: ictResult,
		smcResult: smcResult,
		createdAt: time.Now(),
	}, nil
}

// ---------------------------------------------------------------------------
// Callback handler
// ---------------------------------------------------------------------------

func (h *Handler) handleSMCCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	parts := strings.SplitN(strings.TrimPrefix(data, "smc:"), ":", 2)
	action := parts[0]
	payload := ""
	if len(parts) > 1 {
		payload = parts[1]
	}

	switch action {
	case "sym":
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdSMC(ctx, chatID, 0, payload)

	case "tf":
		// payload = "SYMBOL:TIMEFRAME"
		p2 := strings.SplitN(payload, ":", 2)
		if len(p2) < 2 {
			return nil
		}
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdSMC(ctx, chatID, 0, p2[0]+" "+p2[1])

	case "refresh":
		state := h.smcCache.get(chatID)
		if state == nil {
			_, err := h.bot.SendHTML(ctx, chatID, sessionExpiredMessage("smc"))
			return err
		}
		mapping := domain.FindPriceMappingByCurrency(state.symbol)
		if mapping == nil {
			return nil
		}
		newState, err := h.computeSMCState(ctx, mapping, state.timeframe)
		if err != nil {
			h.sendUserError(ctx, chatID, err, "smc")
			return nil
		}
		h.smcCache.set(chatID, newState)

		msg := formatSMCOutput(newState)
		kb := smcNavKeyboard(state.symbol, state.timeframe)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, msg, kb)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Output formatter
// ---------------------------------------------------------------------------

// formatSMCOutput builds the HTML message for the SMC dashboard.
func formatSMCOutput(s *smcState) string {
	var sb strings.Builder

	tfDisp := strings.ToUpper(s.timeframe)
	if tfDisp == "DAILY" {
		tfDisp = "D1"
	}

	sb.WriteString(fmt.Sprintf("📐 <b>SMC — %s %s</b>\n",
		html.EscapeString(s.symbol), tfDisp))
	sb.WriteString(fmt.Sprintf("<code>%s UTC</code>",
		time.Now().UTC().Format("2006-01-02 15:04")))

	// Killzone from ICT
	if s.ictResult != nil && s.ictResult.Killzone != "" {
		sb.WriteString(fmt.Sprintf(" | ⏰ %s", html.EscapeString(s.ictResult.Killzone)))
	}
	sb.WriteString("\n\n")

	// ── Market Structure (from ta.SMCResult) ──────────────────────────────
	sb.WriteString("🏗 <b>Market Structure</b>\n")
	if s.smcResult != nil {
		structEmoji := "⬛"
		switch s.smcResult.Structure {
		case ta.StructureBullish:
			structEmoji = "🟢"
		case ta.StructureBearish:
			structEmoji = "🔴"
		}
		sb.WriteString(fmt.Sprintf("Trend: %s <b>%s</b>\n", structEmoji, s.smcResult.Trend))

		if len(s.smcResult.RecentBOS) > 0 {
			bos := s.smcResult.RecentBOS[0]
			bosDir := "↑"
			if bos.Dir == "BEARISH" {
				bosDir = "↓"
			}
			sb.WriteString(fmt.Sprintf("Last BOS %s: <code>%.5f</code> <i>(%d bars ago)</i>\n",
				bosDir, bos.Price, bos.BarIndex))
		}

		if len(s.smcResult.RecentCHOCH) > 0 {
			ch := s.smcResult.RecentCHOCH[0]
			sb.WriteString(fmt.Sprintf("Last CHoCH: <code>%.5f</code> %s <i>(%d bars ago)</i>\n",
				ch.Price, ch.Dir, ch.BarIndex))
		}
	} else if s.ictResult != nil && len(s.ictResult.Structure) > 0 {
		// Fallback to ICT structure
		ev := s.ictResult.Structure[len(s.ictResult.Structure)-1]
		icon := "↑"
		if ev.Direction == "BEARISH" {
			icon = "↓"
		}
		sb.WriteString(fmt.Sprintf("Last %s %s: <code>%.5f</code>\n", ev.Kind, icon, ev.Level))
	}
	sb.WriteString("\n")

	// ── ICT Fair Value Gaps ────────────────────────────────────────────────
	if s.ictResult != nil && len(s.ictResult.FVGZones) > 0 {
		shown := 0
		sb.WriteString("⚡ <b>Fair Value Gaps</b>\n")
		for i := len(s.ictResult.FVGZones) - 1; i >= 0 && shown < 3; i-- {
			fvg := s.ictResult.FVGZones[i]
			icon := "🟢"
			if fvg.Kind == "BEARISH" {
				icon = "🔴"
			}
			fillStr := ""
			if fvg.Filled {
				fillStr = fmt.Sprintf(" (%.0f%% filled)", fvg.FillPct)
			} else {
				fillStr = " <i>unfilled</i>"
			}
			sb.WriteString(fmt.Sprintf("• %s %s FVG: <code>%.5f – %.5f</code>%s\n",
				icon, fvg.Kind, fvg.Bottom, fvg.Top, fillStr))
			shown++
		}
		sb.WriteString("\n")
	}

	// ── Order Blocks ──────────────────────────────────────────────────────
	if s.ictResult != nil && len(s.ictResult.OrderBlocks) > 0 {
		sb.WriteString("🔲 <b>Order Blocks</b>\n")
		shown := 0
		for i := len(s.ictResult.OrderBlocks) - 1; i >= 0 && shown < 3; i-- {
			ob := s.ictResult.OrderBlocks[i]
			icon := "🟢"
			if ob.Kind == "BEARISH" {
				icon = "🔴"
			}
			status := "unmitigated ✅"
			if ob.Broken {
				icon = "⬛"
				status = "breaker ⚡"
			}
			sb.WriteString(fmt.Sprintf("• %s %s OB: <code>%.5f – %.5f</code> %s\n",
				icon, ob.Kind, ob.Bottom, ob.Top, status))
			shown++
		}
		sb.WriteString("\n")
	}

	// ── Liquidity Pools ────────────────────────────────────────────────────
	if s.smcResult != nil && len(s.smcResult.InternalLiq) > 0 {
		sb.WriteString("💧 <b>Liquidity Pools</b>\n")
		for i, liq := range s.smcResult.InternalLiq {
			if i >= 3 {
				break
			}
			midpoint := (liq.High + liq.Low) / 2
			swept := ""
			if liq.Swept {
				swept = " ← swept ✓"
			}
			sb.WriteString(fmt.Sprintf("• <code>%.5f</code> (%s)%s\n",
				midpoint, liq.Type, swept))
		}
		sb.WriteString("\n")
	} else if s.ictResult != nil && len(s.ictResult.Sweeps) > 0 {
		sb.WriteString("💧 <b>Liquidity Sweeps</b>\n")
		for i, sw := range s.ictResult.Sweeps {
			if i >= 3 {
				break
			}
			dir := "↑"
			if sw.Kind == "SWEEP_LOW" {
				dir = "↓"
			}
			rev := ""
			if sw.Reversed {
				rev = " ✓ reversed"
			}
			sb.WriteString(fmt.Sprintf("• %s <code>%.5f</code>%s\n", dir, sw.Level, rev))
		}
		sb.WriteString("\n")
	}

	// ── Premium / Discount Zone ────────────────────────────────────────────
	if s.smcResult != nil {
		zoneEmoji := "⚖️"
		switch s.smcResult.CurrentZone {
		case "PREMIUM":
			zoneEmoji = "🔴"
		case "DISCOUNT":
			zoneEmoji = "🟢"
		}

		// Compute zone % position
		swingRange := s.smcResult.LastSwingHigh - s.smcResult.LastSwingLow
		currentPct := 0.0
		if swingRange > 0 {
			// approximate current price from last BOS or equilibrium
			currentPct = (s.smcResult.Equilibrium - s.smcResult.LastSwingLow) / swingRange * 100
			// use the actual zone position
			if s.smcResult.CurrentZone == "PREMIUM" {
				currentPct = 62.0
			} else if s.smcResult.CurrentZone == "DISCOUNT" {
				currentPct = 38.0
			} else {
				currentPct = 50.0
			}
		}

		sb.WriteString(fmt.Sprintf("📊 <b>Zone: %s %s</b> (%.0f%% of range)\n",
			zoneEmoji, s.smcResult.CurrentZone, math.Round(currentPct)))
		sb.WriteString(fmt.Sprintf("EQ: <code>%.5f</code> | Premium: &gt;<code>%.5f</code> | Discount: &lt;<code>%.5f</code>\n",
			s.smcResult.Equilibrium, s.smcResult.Equilibrium, s.smcResult.Equilibrium))
	}

	return sb.String()
}

// ---------------------------------------------------------------------------
// Keyboards
// ---------------------------------------------------------------------------

func smcSymbolKeyboard() ports.InlineKeyboard {
	pairs := []string{"EUR", "GBP", "JPY", "CHF", "AUD", "NZD", "CAD", "XAU"}
	var rows [][]ports.InlineButton
	row := make([]ports.InlineButton, 0, 4)
	for i, p := range pairs {
		row = append(row, ports.InlineButton{
			Text:         p,
			CallbackData: "smc:sym:" + p,
		})
		if len(row) == 4 || i == len(pairs)-1 {
			rows = append(rows, row)
			row = make([]ports.InlineButton, 0, 4)
		}
	}
	return ports.InlineKeyboard{Rows: rows}
}

func smcNavKeyboard(symbol, currentTF string) ports.InlineKeyboard {
	tfRow := []ports.InlineButton{
		{Text: smcTFButtonLabel("1H", currentTF), CallbackData: "smc:tf:" + symbol + ":1h"},
		{Text: smcTFButtonLabel("4H", currentTF), CallbackData: "smc:tf:" + symbol + ":4h"},
		{Text: smcTFButtonLabel("D1", currentTF), CallbackData: "smc:tf:" + symbol + ":daily"},
	}
	actionRow := []ports.InlineButton{
		{Text: "🔄 Refresh", CallbackData: "smc:refresh:"},
		{Text: "📈 ICT", CallbackData: "ict:sym:" + symbol},
	}
	return ports.InlineKeyboard{Rows: [][]ports.InlineButton{tfRow, actionRow}}
}

func smcTFButtonLabel(label, currentTF string) string {
	norm := strings.ToUpper(currentTF)
	switch norm {
	case "1H":
		norm = "1H"
	case "4H":
		norm = "4H"
	case "DAILY":
		norm = "D1"
	}
	if label == norm {
		return "✅ " + label
	}
	return label
}

func smcTFLabel(tf string) string {
	switch strings.ToLower(tf) {
	case "1h":
		return "H1"
	case "4h":
		return "H4"
	case "daily":
		return "D1"
	case "15m":
		return "M15"
	default:
		return strings.ToUpper(tf)
	}
}
