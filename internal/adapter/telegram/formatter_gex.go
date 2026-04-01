package telegram

// formatter_gex.go — GEX (Gamma Exposure) result formatting for Telegram HTML messages.

import (
	"fmt"
	"math"
	"sort"
	"strings"

	gexsvc "github.com/arkcode369/ark-intelligent/internal/service/gex"
)

// FormatGEXResult formats a GEXResult as a Telegram HTML message.
// Output is kept readable on mobile (~2500 chars).
func FormatGEXResult(r *gexsvc.GEXResult) string {
	var sb strings.Builder

	// Header
	regimeEmoji := "🟢"
	if r.Regime == "NEGATIVE_GEX" {
		regimeEmoji = "🔴"
	}

	sb.WriteString(fmt.Sprintf("📊 <b>GAMMA EXPOSURE — %s</b>\n", r.Symbol))
	sb.WriteString(fmt.Sprintf("💰 Spot: <code>%s</code>\n", gexFormatPrice(r.SpotPrice)))
	sb.WriteString(fmt.Sprintf("📅 %s UTC\n\n", r.AnalyzedAt.Format("2006-01-02 15:04")))

	// GEX Regime
	sb.WriteString(fmt.Sprintf("🌡️ <b>GEX REGIME:</b> %s %s (<code>%s</code>)\n",
		regimeEmoji, r.Regime, gexFormatGEXValue(r.TotalGEX)))
	if r.Regime == "POSITIVE_GEX" {
		sb.WriteString("✅ Dealers net long gamma — range-bound / volatility damping\n\n")
	} else {
		sb.WriteString("⚠️ Dealers net short gamma — volatility amplifying / trending\n\n")
	}

	// Key Levels
	sb.WriteString("🎯 <b>KEY LEVELS:</b>\n")
	if r.GEXFlipLevel > 0 {
		flipArrow := "🔼"
		if r.SpotPrice > r.GEXFlipLevel {
			flipArrow = "🔽"
		}
		sb.WriteString(fmt.Sprintf("  🔀 GEX Flip: <code>%s</code> %s\n",
			gexFormatPrice(r.GEXFlipLevel), flipArrow))
	}
	if r.MaxPain > 0 {
		sb.WriteString(fmt.Sprintf("  📌 Max Pain:   <code>%s</code>\n", gexFormatPrice(r.MaxPain)))
	}
	if r.GammaWall > 0 {
		sb.WriteString(fmt.Sprintf("  📌 Gamma Wall: <code>%s</code> (call resistance)\n", gexFormatPrice(r.GammaWall)))
	}
	if r.PutWall > 0 {
		sb.WriteString(fmt.Sprintf("  📌 Put Wall:   <code>%s</code> (put support)\n", gexFormatPrice(r.PutWall)))
	}
	sb.WriteString("\n")

	// GEX Profile (mini bar chart around spot, up to 10 strikes)
	if len(r.Levels) > 0 {
		sb.WriteString("📊 <b>GEX PROFILE</b> (±20% of spot):\n")
		sb.WriteString(gexProfileBars(r.Levels, r.SpotPrice, 10))
		sb.WriteString("\n")
	}

	// Implication
	sb.WriteString("💡 <b>IMPLICATION:</b>\n")
	sb.WriteString(gexWrapText(r.Implication, 300))

	return sb.String()
}

// gexFormatPrice formats a price for the GEX display.
func gexFormatPrice(p float64) string {
	if p == 0 {
		return "N/A"
	}
	if p >= 10000 {
		return fmt.Sprintf("$%s", gexCommaSep(int64(math.Round(p))))
	}
	if p >= 100 {
		return fmt.Sprintf("$%.2f", p)
	}
	return fmt.Sprintf("$%.4f", p)
}

// gexFormatGEXValue formats a GEX dollar value in billions/millions.
func gexFormatGEXValue(v float64) string {
	abs := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}
	switch {
	case abs >= 1e9:
		return fmt.Sprintf("%s$%.1fB", sign, abs/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("%s$%.0fM", sign, abs/1e6)
	default:
		return fmt.Sprintf("%s$%.0f", sign, abs)
	}
}

// gexCommaSep formats an integer with comma thousand separators.
func gexCommaSep(n int64) string {
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// gexProfileBars renders a mini bar chart of GEX levels around spot.
// Shows up to maxLines strikes closest to spot.
func gexProfileBars(levels []gexsvc.GEXLevel, spot float64, maxLines int) string {
	if len(levels) == 0 {
		return "  (no data)\n"
	}

	// Sort by distance to spot and take closest maxLines
	sorted := make([]gexsvc.GEXLevel, len(levels))
	copy(sorted, levels)
	sort.Slice(sorted, func(i, j int) bool {
		return math.Abs(sorted[i].Strike-spot) < math.Abs(sorted[j].Strike-spot)
	})
	if len(sorted) > maxLines {
		sorted = sorted[:maxLines]
	}
	// Re-sort ascending by strike for display
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Strike < sorted[j].Strike })

	// Find max abs GEX for scaling
	maxAbs := 1.0
	for _, l := range sorted {
		if a := math.Abs(l.NetGEX); a > maxAbs {
			maxAbs = a
		}
	}

	var sb strings.Builder
	barWidth := 8
	for _, l := range sorted {
		priceStr := gexFormatPrice(l.Strike)
		ratio := l.NetGEX / maxAbs
		filled := int(math.Round(math.Abs(ratio) * float64(barWidth)))
		if filled > barWidth {
			filled = barWidth
		}

		bar := ""
		if l.NetGEX >= 0 {
			bar = strings.Repeat("▓", filled) + strings.Repeat("░", barWidth-filled)
		} else {
			bar = strings.Repeat("░", barWidth-filled) + strings.Repeat("▓", filled)
		}

		spotMarker := " "
		if math.Abs(l.Strike-spot)/spot < 0.003 {
			spotMarker = "◀"
		}

		typeStr := "  "
		if l.CallGEX > math.Abs(l.PutGEX) {
			typeStr = "📈"
		} else if math.Abs(l.PutGEX) > l.CallGEX {
			typeStr = "📉"
		}

		sb.WriteString(fmt.Sprintf("  %s [%s] <code>%s</code> %s %s\n",
			typeStr, bar, priceStr, spotMarker, gexFormatGEXValue(l.NetGEX)))
	}
	return sb.String()
}

// gexWrapText wraps a long text to maxLen characters, breaking at word boundaries.
func gexWrapText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text + "\n"
	}
	// Find last space before maxLen
	cut := maxLen
	for cut > 0 && text[cut-1] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = maxLen
	}
	return text[:cut] + "…\n"
}
