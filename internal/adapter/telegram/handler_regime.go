package telegram

import (
	"context"

	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/scheduler"
)

// RegimeAlertProvider provides regime state data for the /regime command.
// Implemented by *scheduler.Scheduler.
type RegimeAlertProvider interface {
	GetRegimeStates() []*pricesvc.RegimeState
	GetRegimeDivergence() string
}

// WithRegime wires the regime alert provider for the /regime command.
func (h *Handler) WithRegime(provider RegimeAlertProvider) *Handler {
	h.regimeProvider = provider
	return h
}

// cmdRegime handles the /regime command — shows the multi-asset regime dashboard
// with current HMM states, probabilities, alert tiers, and divergence detection.
func (h *Handler) cmdRegime(ctx context.Context, chatID string, _ int64, _ string) error {
	h.bot.SendTyping(ctx, chatID)

	if h.regimeProvider == nil {
		_, err := h.bot.SendHTML(ctx, chatID,
			"📊 <b>Regime Monitor</b>\n\n"+
				"<i>Regime alert system not configured. "+
				"Requires daily price data.</i>")
		return err
	}

	states := h.regimeProvider.GetRegimeStates()
	divergence := h.regimeProvider.GetRegimeDivergence()

	text := scheduler.FormatRegimeDashboard(states, divergence)
	_, err := h.bot.SendHTML(ctx, chatID, text)
	return err
}
