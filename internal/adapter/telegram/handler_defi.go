package telegram

import (
	"context"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/defi"
)

// registerDeFiCommands wires the /defi command.
func (h *Handler) registerDeFiCommands() {
	h.bot.RegisterCommand("/defi", h.cmdDeFi)
}

// cmdDeFi handles /defi — shows DeFi health dashboard with TVL, DEX volume, and stablecoin supply.
func (h *Handler) cmdDeFi(ctx context.Context, chatID string, _ int64, _ string) error {
	h.bot.SendTyping(ctx, chatID)

	// Add timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	report := defi.GetCachedOrFetch(ctx)

	txt := formatDeFiReport(report)
	kb := h.kb.RelatedCommandsKeyboard("sentiment", "")
	_, err := h.bot.SendWithKeyboard(ctx, chatID, txt, kb)
	return err
}
