package cryptocompare

import (
	"fmt"
	"strings"
)

// FormatExchangeVolumeSection formats exchange volume data for the /cryptoalpha output.
// Output is HTML suitable for Telegram.
func FormatExchangeVolumeSection(s *VolumeSummary) string {
	if s == nil || !s.Available || len(s.Exchanges) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("\n<b>📊 Exchange Volume (CryptoCompare)</b>\n")
	sb.WriteString(fmt.Sprintf("Total: <b>%s</b>\n\n", FormatVolumeUSD(s.TotalUSD)))

	for _, ex := range s.Exchanges {
		changeEmoji := "➡️"
		if ex.Change7D > 10 {
			changeEmoji = "📈"
		} else if ex.Change7D < -10 {
			changeEmoji = "📉"
		}

		shareBar := volumeShareBar(ex.Share)
		sb.WriteString(fmt.Sprintf("%s <b>%s</b> %s (%.1f%%)\n",
			changeEmoji, ex.Exchange, FormatVolumeUSD(ex.VolumeUSD), ex.Share))
		sb.WriteString(fmt.Sprintf("  %s 24h: %+.1f%% | 7d: %+.1f%%\n",
			shareBar, ex.Change24H, ex.Change7D))
	}

	// Divergence alert
	if s.Divergence != "NONE" && s.DivDetail != "" {
		divEmoji := "⚠️"
		if s.Divergence == "SIGNIFICANT" {
			divEmoji = "🚨"
		}
		sb.WriteString(fmt.Sprintf("\n%s <b>Volume Divergence: %s</b>\n", divEmoji, s.Divergence))
		sb.WriteString(fmt.Sprintf("<i>%s</i>\n", s.DivDetail))
	}

	// Top 10 assets by volume
	if len(s.TopAssets) > 0 {
		sb.WriteString("\n<b>🏆 Top by Volume (24h)</b>\n")
		limit := 10
		if len(s.TopAssets) < limit {
			limit = len(s.TopAssets)
		}
		for i, a := range s.TopAssets[:limit] {
			changeStr := fmt.Sprintf("%+.1f%%", a.Change24H)
			priceStr := ""
			if a.Price > 0 {
				priceStr = fmt.Sprintf(" @ $%s", formatPrice(a.Price))
			}
			sb.WriteString(fmt.Sprintf("  %2d. <b>%s</b> %s%s (%s)\n",
				i+1, a.Symbol, FormatVolumeUSD(a.VolumeUSD), priceStr, changeStr))
		}
	}

	return sb.String()
}

// volumeShareBar creates a proportional bar for market share (0-100%).
func volumeShareBar(share float64) string {
	bars := int(share / 10)
	if bars > 10 {
		bars = 10
	}
	if bars < 0 {
		bars = 0
	}
	return strings.Repeat("█", bars) + strings.Repeat("░", 10-bars)
}

// formatPrice formats price for display.
func formatPrice(p float64) string {
	switch {
	case p >= 1000:
		return fmt.Sprintf("%.0f", p)
	case p >= 1:
		return fmt.Sprintf("%.2f", p)
	case p >= 0.01:
		return fmt.Sprintf("%.4f", p)
	default:
		return fmt.Sprintf("%.6f", p)
	}
}
