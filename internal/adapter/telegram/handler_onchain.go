package telegram

import (
	"context"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/onchain"
)

// cmdOnChain handles the /onchain command — shows BTC network health (Blockchain.com)
// and BTC + ETH exchange flow data (CoinMetrics).
func (h *Handler) cmdOnChain(ctx context.Context, chatID string, _ int64, _ string) error {
	h.bot.SendTyping(ctx, chatID)

	// Add timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	report := onchain.GetCachedOrFetch(ctx)
	btcHealth := onchain.GetBTCHealth(ctx)

	txt := formatOnChainReport(report, btcHealth)
	kb := h.kb.RelatedCommandsKeyboard("sentiment", "")
	_, err := h.bot.SendWithKeyboard(ctx, chatID, txt, kb)
	return err
}
