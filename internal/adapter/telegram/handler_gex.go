package telegram

// handler_gex.go — /gex command: Gamma Exposure (GEX) analysis via Deribit public API.
//   /gex [SYMBOL]   — e.g. /gex BTC  (default: BTC)

import (
	"context"
	"fmt"
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
	}
	return h
}

// ---------------------------------------------------------------------------
// /gex — Main command
// ---------------------------------------------------------------------------

// validGEXSymbols lists the crypto symbols supported by Deribit options.
var validGEXSymbols = map[string]struct{}{
	"BTC": {},
	"ETH": {},
}

// cmdGEX handles the /gex [SYMBOL] command.
func (h *Handler) cmdGEX(ctx context.Context, chatID string, userID int64, args string) error {
	if h.gex == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚠️ GEX engine is not configured.")
		return err
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
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"⚠️ Unsupported symbol <code>%s</code>.\n"+
				"Supported: <code>%s</code>\n\n"+
				"Usage: <code>/gex BTC</code> or <code>/gex ETH</code>",
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
	symbols := []string{"BTC", "ETH"}
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
			return err
		}
		html := FormatGEXResult(result)
		kb := gexKeyboard(sym)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	}
	return nil
}
