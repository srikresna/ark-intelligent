package telegram

// handler_auction.go — /auction command: AMT Day Type Classification (Dalton's 6 types).
// Requires VP services to be configured (IntradayRepo for 30-minute bars).

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// /auction command registration (called from WithVP)
// ---------------------------------------------------------------------------

func (h *Handler) registerAuctionCommand() {
	h.bot.RegisterCommand("/auction", h.cmdAuction)
}

// ---------------------------------------------------------------------------
// /auction command handler
// ---------------------------------------------------------------------------

func (h *Handler) cmdAuction(ctx context.Context, chatID string, userID int64, args string) error {
	if h.vp == nil || h.vp.IntradayRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Auction Market Theory engine tidak tersedia.\n\nPastikan VP services dikonfigurasi dengan IntradayRepo.")
		return err
	}

	parts := strings.Fields(strings.TrimSpace(strings.ToUpper(args)))
	if len(parts) == 0 {
		if lc := h.getLastCurrency(ctx, userID); lc != "" {
			return h.cmdAuction(ctx, chatID, userID, lc)
		}
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`🏛️ <b>Auction Market Theory — Day Type Analysis</b>

Klasifikasi Dalton's 6 Day Types untuk setiap hari trading:

🟢 <b>Normal Day</b> — IB ≥85% range. Balanced, fade extremes.
🟡 <b>Normal Variation</b> — IB 70–85%. One-sided extension.
🔴 <b>Trend Day</b> — IB &lt;50% range. Strong conviction.
🔵 <b>Double Distribution</b> — Two value clusters.
📈 <b>P-shape</b> — Heavy upper volume, long lower tail.
📉 <b>b-shape</b> — Heavy lower volume, long upper tail.

Pilih aset:`, h.kb.QuantSymbolMenu())
		return err
	}

	symbol := parts[0]
	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/auction EUR</code>, <code>/auction XAU</code>",
			html.EscapeString(symbol),
		))
		return err
	}

	h.saveLastCurrency(ctx, userID, mapping.Currency)
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf(
		"🏛️ Analysing Day Types for <b>%s</b>...", html.EscapeString(mapping.Currency),
	))

	// Fetch 30m bars (≥10 days × ~48 bars/day = ~480 bars).
	intradayBars, err := h.vp.IntradayRepo.GetHistory(ctx, mapping.ContractCode, "30m", 500)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	if err != nil || len(intradayBars) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("insufficient 30m data for %s: %w", mapping.Currency, err), "auction")
		return nil
	}

	bars := ta.IntradayBarsToOHLCV(intradayBars) // newest-first
	result := ta.ClassifyDayTypes(bars, 2, 6)    // ibPeriods=2 (1h IB), maxDays=6
	if result == nil || len(result.Days) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("insufficient bars for day type classification"), "auction")
		return nil
	}

	msg := formatAuctionResult(mapping.Currency, result)
	_, err = h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// ---------------------------------------------------------------------------
// Formatter
// ---------------------------------------------------------------------------

func formatAuctionResult(symbol string, r *ta.AMTDayTypeResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🏛️ <b>AMT DAY TYPE: %s</b>\n", html.EscapeString(symbol)))
	sb.WriteString(fmt.Sprintf("📅 %s\n\n", time.Now().UTC().Format("02 Jan 2006 15:04 UTC")))

	for i := len(r.Days) - 1; i >= 0; i-- {
		d := r.Days[i]
		emoji := dayTypeEmoji(d.Type)
		sb.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n",
			emoji,
			d.Date.Format("Mon 02 Jan"),
			html.EscapeString(string(d.Type)),
		))
		sb.WriteString(fmt.Sprintf("  IB: <code>%.1f%%</code> of range", d.IBPercent))
		if d.ExtensionUp > 0 || d.ExtensionDown > 0 {
			netDir := "↑"
			if d.NetExtension < 0 {
				netDir = "↓"
			}
			sb.WriteString(fmt.Sprintf(" | Ext: <code>%s%.0f pips</code>", netDir, absVal(d.NetExtension)/0.0001))
		}
		sb.WriteByte('\n')
		sb.WriteString(fmt.Sprintf("  <i>%s</i>\n\n", html.EscapeString(d.Description)))
	}

	// Pattern summary
	sb.WriteString("─────────────────────\n")
	biasEmoji := "⚪"
	switch r.Bias {
	case "BULLISH":
		biasEmoji = "🟢"
	case "BEARISH":
		biasEmoji = "🔴"
	}
	sb.WriteString(fmt.Sprintf("%s <b>Bias:</b> %s\n", biasEmoji, r.Bias))

	migEmoji := "➡️"
	switch r.ValueMigration {
	case "HIGHER":
		migEmoji = "📈"
	case "LOWER":
		migEmoji = "📉"
	}
	sb.WriteString(fmt.Sprintf("%s <b>Value Migration:</b> %s\n", migEmoji, r.ValueMigration))

	if r.ConsecutiveTrendDays >= 2 {
		sb.WriteString(fmt.Sprintf("⚡ <b>%d consecutive Trend Days</b> — strong directional conviction\n", r.ConsecutiveTrendDays))
	}

	return sb.String()
}

func dayTypeEmoji(t ta.DayType) string {
	switch t {
	case ta.DayTypeNormal:
		return "🟢"
	case ta.DayTypeNormalVariation:
		return "🟡"
	case ta.DayTypeTrend:
		return "🔴"
	case ta.DayTypeDoubleDistribution:
		return "🔵"
	case ta.DayTypePShape:
		return "📈"
	case ta.DayTypeBShape:
		return "📉"
	default:
		return "⚪"
	}
}

func absVal(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
