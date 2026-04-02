package telegram

// handler_gex.go — /gex and /ivol commands: Gamma Exposure + IV Surface via Deribit public API.
//   /gex [SYMBOL]   — e.g. /gex BTC  (default: BTC)
//   /ivol [SYMBOL]  — e.g. /ivol BTC (default: BTC)

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	gexsvc "github.com/arkcode369/ark-intelligent/internal/service/gex"
)

// ---------------------------------------------------------------------------
// GEXServices — dependencies for the /gex command
// ---------------------------------------------------------------------------

// GEXServices holds the dependencies needed by the /gex handler.
type GEXServices struct {
	Engine *gexsvc.Engine
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------

// WithGEX injects GEXServices into the handler and registers the /gex command
// and its inline keyboard callbacks.
func (h *Handler) WithGEX(svc *GEXServices) *Handler {
	h.gex = svc
	if svc != nil {
		h.bot.RegisterCommand("/gex", h.cmdGEX)
		h.bot.RegisterCallback("gex:", h.handleGEXCallback)
		h.bot.RegisterCommand("/ivol", h.cmdIVSurface)
		h.bot.RegisterCallback("ivol:", h.handleIVolCallback)
		h.bot.RegisterCommand("/skew", h.cmdSkew)
		h.bot.RegisterCallback("skew:", h.handleSkewCallback)
	}
	return h
}

// ---------------------------------------------------------------------------
// /gex — Main command
// ---------------------------------------------------------------------------

// validGEXSymbols lists the crypto symbols supported by Deribit options.
var validGEXSymbols = map[string]struct{}{
	"BTC":  {},
	"ETH":  {},
	"SOL":  {},
	"AVAX": {},
	"XRP":  {},
}

// cmdGEX handles the /gex [SYMBOL] command.
func (h *Handler) cmdGEX(ctx context.Context, chatID string, userID int64, args string) error {
	if h.gex == nil {
		h.sendUserError(ctx, chatID, fmt.Errorf("GEX engine not available"), "gex")
		return nil
	}

	// Parse symbol from args (default: BTC)
	sym := "BTC"
	if parts := strings.Fields(args); len(parts) > 0 {
		sym = strings.ToUpper(strings.TrimSpace(parts[0]))
	}

	// Validate symbol
	if _, ok := validGEXSymbols[sym]; !ok {
		keys := make([]string, 0, len(validGEXSymbols))
		for k := range validGEXSymbols {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ Symbol <code>%s</code> tidak didukung.\n"+
				"Tersedia: <code>%s</code>\n\n"+
				"Contoh: <code>/gex BTC</code>, <code>/gex SOL</code>",
			sym, strings.Join(keys, "</code>, <code>"),
		))
		return err
	}

	// Show loading indicator
	loadingMsg := fmt.Sprintf("⏳ Fetching GEX data for <b>%s</b> from Deribit...\n"+
		"<i>This may take 10–30 seconds (fetching option Greeks)</i>", sym)
	loadID, err := h.bot.SendLoading(ctx, chatID, loadingMsg)
	if err != nil {
		return fmt.Errorf("gex: send loading: %w", err)
	}

	// Run analysis
	result, err := h.gex.Engine.Analyze(ctx, sym)
	if err != nil {
		h.editUserError(ctx, chatID, loadID, err, "gex")
		return nil
	}

	// Format and send result with navigation keyboard
	html := FormatGEXResult(result)
	kb := gexKeyboard(sym)

	if err := h.bot.EditWithKeyboard(ctx, chatID, loadID, html, kb); err != nil {
		// Fallback: send as new message
		_, sendErr := h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return sendErr
	}
	return nil
}

// gexKeyboard builds the inline keyboard for the /gex response.
func gexKeyboard(currentSym string) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	// Symbol switcher
	symbols := []string{"BTC", "ETH", "SOL", "XRP", "AVAX"}
	var symbolRow []ports.InlineButton
	for _, s := range symbols {
		label := s
		if s == currentSym {
			label = "● " + s
		}
		symbolRow = append(symbolRow, ports.InlineButton{
			Text:         label,
			CallbackData: "gex:sym:" + s,
		})
	}
	rows = append(rows, symbolRow)

	// Refresh button
	rows = append(rows, []ports.InlineButton{
		{Text: "🔄 Refresh", CallbackData: "gex:refresh:" + currentSym},
		{Text: "🔀 Skew", CallbackData: "skew:sym:" + currentSym},
	})

	return ports.InlineKeyboard{Rows: rows}
}

// ---------------------------------------------------------------------------
// Callback handler
// ---------------------------------------------------------------------------

// handleGEXCallback handles /gex inline keyboard presses.
// data format: "gex:<action>:<symbol>"  e.g. "gex:sym:ETH", "gex:refresh:BTC"
func (h *Handler) handleGEXCallback(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	if h.gex == nil {
		return nil
	}

	// data: "gex:sym:BTC" | "gex:refresh:BTC"
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 3 {
		return nil
	}
	action := parts[1]
	sym := strings.ToUpper(parts[2])

	if _, ok := validGEXSymbols[sym]; !ok {
		return nil
	}

	switch action {
	case "sym", "refresh":
		result, err := h.gex.Engine.Analyze(ctx, sym)
		if err != nil {
			errHTML := fmt.Sprintf("\u26a0\ufe0f <b>GEX analysis failed for %s</b>\n\n<i>%s</i>\n\nThis may be temporary \u2014 try again in a few minutes.", sym, err.Error())
			kb := gexKeyboard(sym)
			_ = h.bot.EditWithKeyboard(ctx, chatID, msgID, errHTML, kb)
			return nil
		}
		html := FormatGEXResult(result)
		kb := gexKeyboard(sym)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	}
	return nil
}

// ---------------------------------------------------------------------------
// /ivol — IV Surface command
// ---------------------------------------------------------------------------

// cmdIVSurface handles the /ivol [SYMBOL] command.
func (h *Handler) cmdIVSurface(ctx context.Context, chatID string, userID int64, args string) error {
	if h.gex == nil {
		h.sendUserError(ctx, chatID, fmt.Errorf("IV Surface engine not available"), "ivol")
		return nil
	}

	sym := "BTC"
	if parts := strings.Fields(args); len(parts) > 0 {
		sym = strings.ToUpper(strings.TrimSpace(parts[0]))
	}

	if _, ok := validGEXSymbols[sym]; !ok {
		keys := make([]string, 0, len(validGEXSymbols))
		for k := range validGEXSymbols {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ Symbol <code>%s</code> tidak didukung.\n"+
				"Tersedia: <code>%s</code>\n\n"+
				"Contoh: <code>/ivol BTC</code>, <code>/ivol ETH</code>",
			sym, strings.Join(keys, "</code>, <code>"),
		))
		return err
	}

	loadID, err := h.bot.SendLoading(ctx, chatID,
		fmt.Sprintf("⏳ Fetching IV Surface for <b>%s</b> from Deribit...\n<i>Analysing implied volatility across all strikes and expiries</i>", sym))
	if err != nil {
		return fmt.Errorf("ivol: send loading: %w", err)
	}

	result, err := h.gex.Engine.AnalyzeIVSurface(ctx, sym)
	if err != nil {
		h.editUserError(ctx, chatID, loadID, err, "ivol")
		return nil
	}

	html := FormatIVSurface(result)
	kb := ivolKeyboard(sym)

	if err := h.bot.EditWithKeyboard(ctx, chatID, loadID, html, kb); err != nil {
		_, sendErr := h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return sendErr
	}
	return nil
}

// ivolKeyboard builds the inline keyboard for the /ivol response.
func ivolKeyboard(currentSym string) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	symbols := []string{"BTC", "ETH", "SOL", "XRP", "AVAX"}
	var symRow []ports.InlineButton
	for _, s := range symbols {
		label := s
		if s == currentSym {
			label = "● " + s
		}
		symRow = append(symRow, ports.InlineButton{
			Text:         label,
			CallbackData: "ivol:sym:" + s,
		})
	}
	rows = append(rows, symRow)
	rows = append(rows, []ports.InlineButton{
		{Text: "🔄 Refresh", CallbackData: "ivol:refresh:" + currentSym},
		{Text: "🔀 Skew", CallbackData: "skew:sym:" + currentSym},
		{Text: "📊 GEX", CallbackData: "gex:sym:" + currentSym},
	})
	return ports.InlineKeyboard{Rows: rows}
}

// handleIVolCallback handles /ivol inline keyboard presses.
// data format: "ivol:<action>:<symbol>"
func (h *Handler) handleIVolCallback(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	if h.gex == nil {
		return nil
	}
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 3 {
		return nil
	}
	sym := strings.ToUpper(parts[2])
	if _, ok := validGEXSymbols[sym]; !ok {
		return nil
	}
	result, err := h.gex.Engine.AnalyzeIVSurface(ctx, sym)
	if err != nil {
		errHTML := fmt.Sprintf("⚠️ <b>IV Surface failed for %s</b>\n\n<i>%s</i>", sym, err.Error())
		kb := ivolKeyboard(sym)
		_ = h.bot.EditWithKeyboard(ctx, chatID, msgID, errHTML, kb)
		return nil
	}
	html := FormatIVSurface(result)
	kb := ivolKeyboard(sym)
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
}

// ---------------------------------------------------------------------------
// /skew — IV Skew / Smile Analysis command
// ---------------------------------------------------------------------------

// cmdSkew handles the /skew [SYMBOL] command.
func (h *Handler) cmdSkew(ctx context.Context, chatID string, userID int64, args string) error {
	if h.gex == nil {
		h.sendUserError(ctx, chatID, fmt.Errorf("Skew Analysis engine not available"), "skew")
		return nil
	}

	sym := "BTC"
	if parts := strings.Fields(args); len(parts) > 0 {
		sym = strings.ToUpper(strings.TrimSpace(parts[0]))
	}

	if _, ok := validGEXSymbols[sym]; !ok {
		keys := make([]string, 0, len(validGEXSymbols))
		for k := range validGEXSymbols {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ Symbol <code>%s</code> tidak didukung.\n"+
				"Tersedia: <code>%s</code>\n\n"+
				"Contoh: <code>/skew BTC</code>, <code>/skew ETH</code>",
			sym, strings.Join(keys, "</code>, <code>"),
		))
		return err
	}

	loadID, err := h.bot.SendLoading(ctx, chatID,
		fmt.Sprintf("⏳ Analysing IV Skew for <b>%s</b>...\n<i>Computing smile curves, put/call ratio, flip detection</i>", sym))
	if err != nil {
		return fmt.Errorf("skew: send loading: %w", err)
	}

	result, err := h.gex.Engine.AnalyzeSkew(ctx, sym)
	if err != nil {
		h.editUserError(ctx, chatID, loadID, err, "skew")
		return nil
	}

	html := FormatSkewResult(result)
	kb := skewKeyboard(sym)
	if err := h.bot.EditWithKeyboard(ctx, chatID, loadID, html, kb); err != nil {
		_, sendErr := h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return sendErr
	}
	return nil
}

// skewKeyboard builds the inline keyboard for the /skew response.
func skewKeyboard(currentSym string) ports.InlineKeyboard {
	var rows [][]ports.InlineButton

	symbols := []string{"BTC", "ETH", "SOL", "XRP", "AVAX"}
	var symRow []ports.InlineButton
	for _, s := range symbols {
		label := s
		if s == currentSym {
			label = "● " + s
		}
		symRow = append(symRow, ports.InlineButton{
			Text:         label,
			CallbackData: "skew:sym:" + s,
		})
	}
	rows = append(rows, symRow)
	rows = append(rows, []ports.InlineButton{
		{Text: "🔄 Refresh", CallbackData: "skew:refresh:" + currentSym},
		{Text: "📈 IV Surface", CallbackData: "ivol:sym:" + currentSym},
		{Text: "📊 GEX", CallbackData: "gex:sym:" + currentSym},
	})
	return ports.InlineKeyboard{Rows: rows}
}

// handleSkewCallback handles /skew inline keyboard presses.
// data format: "skew:<action>:<symbol>"
func (h *Handler) handleSkewCallback(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	if h.gex == nil {
		return nil
	}
	parts := strings.SplitN(data, ":", 3)
	if len(parts) < 3 {
		return nil
	}
	sym := strings.ToUpper(parts[2])
	if _, ok := validGEXSymbols[sym]; !ok {
		return nil
	}
	result, err := h.gex.Engine.AnalyzeSkew(ctx, sym)
	if err != nil {
		errHTML := fmt.Sprintf("⚠️ <b>Skew analysis failed for %s</b>\n\n<i>%s</i>", sym, err.Error())
		kb := skewKeyboard(sym)
		_ = h.bot.EditWithKeyboard(ctx, chatID, msgID, errHTML, kb)
		return nil
	}
	html := FormatSkewResult(result)
	kb := skewKeyboard(sym)
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
}
