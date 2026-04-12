package telegram

import (
	"context"

	fred "github.com/arkcode369/ark-intelligent/internal/service/fred"
)

// cmdCarry handles the /carry command — shows carry trade monitor with
// ranked pairs and unwind detection.
func (h *Handler) cmdCarry(ctx context.Context, chatID string, _ int64, _ string) error {
	h.bot.SendTyping(ctx, chatID)

	monitor := fred.GetCarryMonitor()
	result, err := monitor.FetchCarryDashboard(ctx)
	if err != nil {
		log.Error().Err(err).Msg("carry monitor fetch failed")
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			"❌ <b>Carry monitor error</b>\n\n"+
				"<i>FRED API may be temporarily unavailable. Please try again later.</i>")
		return sendErr
	}

	text := h.fmt.FormatCarryMonitor(result)
	_, sendErr := h.bot.SendHTML(ctx, chatID, text)
	return sendErr
}
