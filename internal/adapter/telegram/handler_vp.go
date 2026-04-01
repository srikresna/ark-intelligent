package telegram

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
// VP Services (injected via WithVP)
// ---------------------------------------------------------------------------

// VPServices holds dependencies for the Volume Profile command.
type VPServices struct {
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
}

// ---------------------------------------------------------------------------
// VP State Cache
// ---------------------------------------------------------------------------

type vpState struct {
	symbol    string
	currency  string
	timeframe string
	bars      map[string][]ta.OHLCV // tf -> bars
	createdAt time.Time
}

type vpStateCache struct {
	mu    sync.Mutex
	store map[string]*vpState
}

func newVPStateCache() *vpStateCache {
	return &vpStateCache{store: make(map[string]*vpState)}
}

var vpStateTTL = config.VPStateTTL

func (c *vpStateCache) get(chatID string) *vpState {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.store[chatID]
	if !ok || time.Since(s.createdAt) > vpStateTTL {
		return nil
	}
	return s
}

func (c *vpStateCache) set(chatID string, s *vpState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range c.store {
		if time.Since(v.createdAt) > 60*time.Minute {
			delete(c.store, k)
		}
	}
	c.store[chatID] = s
}

// ---------------------------------------------------------------------------
// WithVP wires the VP handler into the main handler.
// ---------------------------------------------------------------------------

func (h *Handler) WithVP(svc VPServices) {
	h.vp = &svc
	h.vpCache = newVPStateCache()
	h.bot.RegisterCommand("/vp", h.cmdVP)
	h.bot.RegisterCallback("vp:", h.handleVPCallback)
}

// ---------------------------------------------------------------------------
// /vp command
// ---------------------------------------------------------------------------

func (h *Handler) cmdVP(ctx context.Context, chatID string, userID int64, args string) error {
	if h.vp == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "VP engine tidak tersedia.")
		return err
	}

	parts := strings.Fields(strings.TrimSpace(strings.ToUpper(args)))
	if len(parts) == 0 {
		// Show symbol selector with description
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`📊 <b>Volume Profile — Institutional Analysis</b>

10 mode analisis volume-at-price:

📊 <b>Profile</b> — POC, VAH/VAL, HVN/LVN zones
🕐 <b>Session</b> — Asian/London/NY split + Naked POC
📐 <b>Shape</b> — P/b/D/B classification
🔀 <b>Composite</b> — Multi-window merged VP
📏 <b>VWAP</b> — VWAP + σ bands + anchored VWAP
⏱ <b>TPO</b> — Time vs Volume POC divergence
📈 <b>Delta</b> — Simulated buy/sell pressure
🏛 <b>Auction</b> — Initiative/Responsive state
🎯 <b>Confluence</b> — Multi-TF level overlap (★★★★★)
📋 <b>Full Report</b> — Decision signal synthesis

Pilih aset:`, h.kb.VPSymbolMenu())
		return err
	}

	currency := parts[0]
	timeframe := "daily"
	if len(parts) > 1 {
		timeframe = strings.ToLower(parts[1])
	}

	mapping := domain.FindPriceMappingByCurrency(currency)
	if mapping == nil || mapping.RiskOnly {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("Symbol tidak dikenal: <code>%s</code>", html.EscapeString(currency)))
		return err
	}

	// Send "computing..." placeholder
	msgID, _ := h.bot.SendLoading(ctx, chatID,
		fmt.Sprintf("⏳ Menghitung Volume Profile <b>%s</b> (%s)...",
			html.EscapeString(mapping.Currency), timeframe))

	state, err := h.computeVPState(ctx, mapping, timeframe)
	if err != nil {
		errMsg := userFriendlyError(err, "vp")
		if msgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID, errMsg, h.kb.VPMenu())
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	h.vpCache.set(chatID, state)
	return h.vpRunMode(ctx, chatID, msgID, state, "profile")
}

// ---------------------------------------------------------------------------
// Callback handler
// ---------------------------------------------------------------------------

func (h *Handler) handleVPCallback(ctx context.Context, chatID string, msgID int, _ int64, data string) error {
	action := strings.TrimPrefix(data, "vp:")

	// Symbol selection from VPSymbolMenu
	if strings.HasPrefix(action, "sym:") {
		symAndTF := strings.TrimPrefix(action, "sym:")
		parts := strings.SplitN(symAndTF, ":", 2)
		currency := parts[0]
		timeframe := "daily"
		if len(parts) > 1 && parts[1] != "" {
			timeframe = parts[1]
		}
		mapping := domain.FindPriceMappingByCurrency(currency)
		if mapping == nil {
			return nil
		}
		_ = h.bot.EditWithKeyboard(ctx, chatID, msgID,
			fmt.Sprintf("⏳ Menghitung VP <b>%s</b> (%s)...",
				html.EscapeString(currency), timeframe), h.kb.VPMenu())
		state, err := h.computeVPState(ctx, mapping, timeframe)
		if err != nil {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID,
				userFriendlyError(err, "vp"), h.kb.VPMenu())
		}
		h.vpCache.set(chatID, state)
		return h.vpRunMode(ctx, chatID, msgID, state, "profile")
	}

	// TF change
	if strings.HasPrefix(action, "tf:") {
		newTF := strings.TrimPrefix(action, "tf:")
		state := h.vpCache.get(chatID)
		if state == nil {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID,
				sessionExpiredMessage("vp"), h.kb.VPMenu())
		}
		if newTF != state.timeframe {
			mapping := domain.FindPriceMappingByCurrency(state.currency)
			if mapping == nil {
				return nil
			}
			newState, err := h.computeVPState(ctx, mapping, newTF)
			if err != nil {
				return h.bot.EditWithKeyboard(ctx, chatID, msgID,
					userFriendlyError(err, "vp"), h.kb.VPMenu())
			}
			h.vpCache.set(chatID, newState)
			state = newState
		}
		return h.vpRunMode(ctx, chatID, msgID, state, "profile")
	}

	if action == "back" {
		state := h.vpCache.get(chatID)
		if state == nil {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID,
				sessionExpiredMessage("vp"), h.kb.VPMenu())
		}
		summary := fmt.Sprintf("📊 <b>Volume Profile: %s — %s</b>\n\nPilih mode analisis:",
			html.EscapeString(state.symbol), state.timeframe)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, summary, h.kb.VPMenu())
	}

	if action == "refresh" {
		state := h.vpCache.get(chatID)
		if state == nil {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID,
				sessionExpiredMessage("vp"), h.kb.VPMenu())
		}
		mapping := domain.FindPriceMappingByCurrency(state.currency)
		if mapping == nil {
			return nil
		}
		newState, err := h.computeVPState(ctx, mapping, state.timeframe)
		if err != nil {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID,
				userFriendlyError(err, "vp"), h.kb.VPMenu())
		}
		h.vpCache.set(chatID, newState)
		return h.vpRunMode(ctx, chatID, msgID, newState, "profile")
	}

	// Mode buttons
	validModes := map[string]bool{
		"profile": true, "session": true, "shape": true, "composite": true,
		"vwap": true, "tpo": true, "delta": true, "auction": true,
		"confluence": true, "full": true,
	}
	if validModes[action] {
		state := h.vpCache.get(chatID)
		if state == nil {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID,
				sessionExpiredMessage("vp"), h.kb.VPMenu())
		}
		return h.vpRunMode(ctx, chatID, msgID, state, action)
	}

	return nil
}

// ---------------------------------------------------------------------------
// computeVPState — fetch bars for all TFs
// ---------------------------------------------------------------------------

func (h *Handler) computeVPState(ctx context.Context, mapping *domain.PriceSymbolMapping, timeframe string) (*vpState, error) {
	code := mapping.ContractCode
	barsByTF := make(map[string][]ta.OHLCV)

	// Daily bars always fetched
	dailyRecords, err := h.vp.DailyPriceRepo.GetDailyHistory(ctx, code, 500)
	if err != nil || len(dailyRecords) < 20 {
		return nil, fmt.Errorf("insufficient daily data for %s (%d bars)", mapping.Currency, len(dailyRecords))
	}
	barsByTF["daily"] = ta.DailyPricesToOHLCV(dailyRecords)

	// All intraday TFs (for confluence/composite/full)
	if h.vp.IntradayRepo != nil {
		for _, tf := range []string{"15m", "30m", "1h", "4h", "6h", "12h"} {
			count := 500
			if tf == "15m" || tf == "30m" {
				count = 2000
			}
			intradayBars, iErr := h.vp.IntradayRepo.GetHistory(ctx, code, tf, count)
			if iErr == nil && len(intradayBars) > 20 {
				barsByTF[tf] = ta.IntradayBarsToOHLCV(intradayBars)
			}
		}
	}

	return &vpState{
		symbol:    mapping.Currency,
		currency:  mapping.Currency,
		timeframe: timeframe,
		bars:      barsByTF,
		createdAt: time.Now(),
	}, nil
}

// ---------------------------------------------------------------------------
// vpRunMode — run a VP engine mode and deliver result
// ---------------------------------------------------------------------------

func (h *Handler) vpRunMode(ctx context.Context, chatID string, msgID int, state *vpState, mode string) error {
	tf := state.timeframe
	bars, ok := state.bars[tf]
	if !ok || len(bars) < 20 {
		errMsg := fmt.Sprintf("❌ Tidak cukup data untuk <b>%s</b> — timeframe <b>%s</b>",
			html.EscapeString(state.symbol), tf)
		if msgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID, errMsg, h.kb.VPMenu())
		}
		_, err := h.bot.SendWithKeyboard(ctx, chatID, errMsg, h.kb.VPMenu())
		return err
	}

	// Build bar JSON
	type barJSON struct {
		Date   string  `json:"date"`
		Open   float64 `json:"open"`
		High   float64 `json:"high"`
		Low    float64 `json:"low"`
		Close  float64 `json:"close"`
		Volume float64 `json:"volume"`
	}

	toBars := func(ohlcv []ta.OHLCV) []barJSON {
		result := make([]barJSON, len(ohlcv))
		for i, b := range ohlcv {
			result[i] = barJSON{
				Date:   b.Date.Format("2006-01-02 15:04:05"),
				Open:   b.Open,
				High:   b.High,
				Low:    b.Low,
				Close:  b.Close,
				Volume: b.Volume,
			}
		}
		return result
	}

	input := map[string]any{
		"mode":      mode,
		"symbol":    state.symbol,
		"timeframe": tf,
		"bars":      toBars(bars),
		"params":    map[string]any{},
	}

	// For multi-TF modes, include all other TF bars
	if mode == "confluence" || mode == "composite" || mode == "full" {
		allTF := make(map[string][]barJSON)
		for tfName, tfBars := range state.bars {
			if tfName == tf {
				continue
			}
			allTF[tfName] = toBars(tfBars)
		}
		input["all_tf_bars"] = allTF
	}

	result, err := h.runVPEngine(input)
	if err != nil {
		errMsg := userFriendlyError(err, "vp")
		if msgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID, errMsg, h.kb.VPMenu())
		}
		_, sendErr := h.bot.SendWithKeyboard(ctx, chatID, errMsg, h.kb.VPMenu())
		return sendErr
	}

	// Delete existing message, then send chart + text
	if msgID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
	}

	// Send chart photo
	chartSent := false
	if result.ChartPath != "" {
		chartData, readErr := os.ReadFile(result.ChartPath)
		if readErr == nil && len(chartData) > 0 {
			caption := fmt.Sprintf("📊 VP %s — %s — %s",
				strings.ToUpper(mode), html.EscapeString(state.symbol), tf)
			_, _ = h.bot.SendPhoto(ctx, chatID, chartData, caption)
			chartSent = true
			_ = os.Remove(result.ChartPath)
		} else {
			if readErr != nil {
				log.Error().Err(readErr).Str("symbol", state.symbol).Str("timeframe", tf).Str("mode", mode).Msg("VP chart read failed, falling back to text")
			}
			_ = os.Remove(result.ChartPath)
		}
	}

	// Send text with keyboard — prepend chart failure notice if chart was expected but not sent
	textToSend := result.TextOutput
	if textToSend == "" {
		textToSend = fmt.Sprintf("📊 Volume Profile: %s — %s\n(No output)", state.symbol, tf)
	}
	if result.ChartPath != "" && !chartSent {
		fallbackNotice := "📊 <i>Chart sementara tidak tersedia. Menampilkan analisis teks.</i>\n\n"
		textToSend = fallbackNotice + textToSend
	}
	_, sendErr := h.bot.SendWithKeyboard(ctx, chatID, textToSend, h.kb.VPMenu())
	return sendErr
}

// ---------------------------------------------------------------------------
// runVPEngine — execute Python vp_engine.py
// ---------------------------------------------------------------------------

type vpEngineResult struct {
	Mode       string          `json:"mode"`
	Symbol     string          `json:"symbol"`
	Success    bool            `json:"success"`
	Error      string          `json:"error"`
	Result     json.RawMessage `json:"result"`
	TextOutput string          `json:"text_output"`
	ChartPath  string          `json:"chart_path"`
}

func (h *Handler) runVPEngine(input map[string]any) (*vpEngineResult, error) {
	defer func() {
		if r := recover(); r != nil {
			// This panic is caught by handleUpdate's outer recovery too,
			// but logging here gives more specific context.
			log.Error().
				Interface("panic", r).
				Msg("panic in runVPEngine — subprocess may have failed")
		}
	}()
	ts := time.Now().UnixNano()
	inputPath := filepath.Join(os.TempDir(), fmt.Sprintf("vp_in_%d.json", ts))
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("vp_out_%d.json", ts))
	chartPath := filepath.Join(os.TempDir(), fmt.Sprintf("vp_chart_%d.png", ts))

	defer os.Remove(inputPath)

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	if err := os.WriteFile(inputPath, inputJSON, 0644); err != nil {
		return nil, fmt.Errorf("write input: %w", err)
	}

	scriptPath := findVPScript()
	cmdCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath, inputPath, outputPath, chartPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "MPLBACKEND=Agg")

	if err := cmd.Run(); err != nil {
		log.Error().Err(err).
			Str("stderr", stderr.String()).
			Msg("VP engine subprocess failed")
		os.Remove(chartPath) // cleanup chart on failure
		os.Remove(outputPath)
		return nil, fmt.Errorf("VP engine failed: %w", err)
	}

	resultJSON, err := os.ReadFile(outputPath)
	os.Remove(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read output: %w", err)
	}

	var result vpEngineResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshal output: %w", err)
	}

	if !result.Success {
		return &result, fmt.Errorf("%s", result.Error)
	}

	if fi, statErr := os.Stat(chartPath); statErr == nil {
		if fi.Size() > 0 {
			result.ChartPath = chartPath
		} else {
			log.Warn().Str("chart_path", chartPath).Msg("chart renderer produced 0-byte file, skipping")
			os.Remove(chartPath)
		}
	}

	return &result, nil
}

// findVPScript locates the vp_engine.py script.
func findVPScript() string {
	candidates := []string{
		"scripts/vp_engine.py",
		"../scripts/vp_engine.py",
		"/home/mulerun/.openclaw/workspace/ark-intelligent/scripts/vp_engine.py",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "scripts/vp_engine.py"
}
