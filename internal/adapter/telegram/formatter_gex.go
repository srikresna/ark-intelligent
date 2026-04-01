package telegram

// formatter_gex.go — GEX (Gamma Exposure) + IV Surface formatting for Telegram HTML messages.

import (
	"fmt"
	"math"
	"sort"
	"strings"

	gexsvc "github.com/arkcode369/ark-intelligent/internal/service/gex"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatGEXResult formats a GEXResult as a Telegram HTML message.
// Output is kept readable on mobile (~2500 chars).
func FormatGEXResult(r *gexsvc.GEXResult) string {
	var sb strings.Builder

	// Header — uses fmtutil.AnalysisHeader.
	sb.WriteString(fmtutil.AnalysisHeader("📊", "GAMMA EXPOSURE", r.Symbol, ""))
	sb.WriteString(fmt.Sprintf("💰 Spot: <code>%s</code>\n", gexFormatPrice(r.SpotPrice)))
	sb.WriteString(fmt.Sprintf("📅 %s UTC\n", r.AnalyzedAt.Format("2006-01-02 15:04")))
	if r.LowLiquidity {
		sb.WriteString("⚠️ <i>Low liquidity — data may be less reliable</i>\n")
	}
	sb.WriteString("\n")

	// GEX Regime — uses fmtutil.RegimeEmoji.
	regimeEmoji := fmtutil.RegimeEmoji(r.Regime)
	sb.WriteString(fmt.Sprintf("🌡️ <b>GEX REGIME:</b> %s %s (<code>%s</code>)\n",
		regimeEmoji, r.Regime, gexFormatGEXValue(r.TotalGEX)))
	if r.Regime == "POSITIVE_GEX" {
		sb.WriteString("✅ Dealers net long gamma — range-bound / peredam volatilitas\n")
	} else {
		sb.WriteString("⚠️ Dealers net short gamma — volatilitas meningkat / trending\n")
	}
	sb.WriteString("<i>📖 GEX (Gamma Exposure): ukuran sensitivitas harga terhadap posisi options dealer</i>\n\n")

	// Key Levels
	sb.WriteString("🎯 <b>LEVEL KUNCI:</b>\n")
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
		sb.WriteString("     <i>↳ harga di mana options expiry menyebabkan kerugian terkecil bagi option holders</i>\n")
	}
	if r.GammaWall > 0 {
		sb.WriteString(fmt.Sprintf("  📌 Gamma Wall: <code>%s</code> (resistansi call)\n", gexFormatPrice(r.GammaWall)))
		sb.WriteString("     <i>↳ strike dengan gamma tertinggi — bertindak sebagai magnet harga / resistansi kuat</i>\n")
	}
	if r.PutWall > 0 {
		sb.WriteString(fmt.Sprintf("  📌 Put Wall:   <code>%s</code> (support put)\n", gexFormatPrice(r.PutWall)))
		sb.WriteString("     <i>↳ level support terkuat dari konsentrasi posisi put options dealer</i>\n")
	}
	sb.WriteString("\n")

	// GEX Profile (mini bar chart around spot, up to 10 strikes)
	if len(r.Levels) > 0 {
		sb.WriteString("📊 <b>PROFIL GEX</b> (±20% dari spot):\n")
		sb.WriteString(gexProfileBars(r.Levels, r.SpotPrice, 10))
		sb.WriteString("\n")
	}

	// Implication
	sb.WriteString("💡 <b>IMPLIKASI:</b>\n")
	sb.WriteString(gexWrapText(r.Implication, 300))

	return sb.String()
}

// gexFormatPrice formats a price for the GEX display.
// Uses fmtutil.FmtNum for the integer portion when p >= 10000.
func gexFormatPrice(p float64) string {
	if p == 0 {
		return "N/A"
	}
	if p >= 10000 {
		return fmt.Sprintf("$%s", fmtutil.FmtNum(math.Round(p), 0))
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

// gexProfileBars renders a mini bar chart of GEX levels around spot.
// Shows up to maxLines strikes closest to spot.
// Uses fmtutil.ProgressBar for the bar rendering.
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

	const barWidth = 8
	var sb strings.Builder
	for _, l := range sorted {
		priceStr := gexFormatPrice(l.Strike)
		absRatio := math.Abs(l.NetGEX) / maxAbs

		var bar string
		if l.NetGEX >= 0 {
			bar = fmtutil.ProgressBar(absRatio, 1, barWidth, "▓", "░")
		} else {
			// Negative GEX: fill from the right side.
			filled := int(math.Round(absRatio * float64(barWidth)))
			if filled > barWidth {
				filled = barWidth
			}
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

// ---------------------------------------------------------------------------
// IV Surface formatter
// ---------------------------------------------------------------------------

// FormatIVSurface formats an IVSurfaceResult as a Telegram HTML message.
// Sections: header, market signal, term structure, per-expiry skew table.
func FormatIVSurface(r *gexsvc.IVSurfaceResult) string {
	var sb strings.Builder

	sb.WriteString(fmtutil.AnalysisHeader("📈", "IV SURFACE", r.Symbol, ""))
	sb.WriteString(fmt.Sprintf("💰 Spot: <code>%s</code>\n", gexFormatPrice(r.SpotPrice)))
	sb.WriteString(fmt.Sprintf("📅 %s UTC\n\n", r.AnalyzedAt.Format("2006-01-02 15:04")))

	// Market signal
	signalEmoji := ivSignalEmoji(r.MarketSignal)
	sb.WriteString(fmt.Sprintf("🌡️ <b>IV SIGNAL:</b> %s %s\n", signalEmoji, r.MarketSignal))
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n\n", r.SignalReason))

	// Term structure summary
	sb.WriteString("📉 <b>TERM STRUCTURE</b>")
	if r.Backwardation {
		sb.WriteString(" ⚠️ <i>(backwardation)</i>")
	}
	sb.WriteString("\n")
	if len(r.TermStructure) == 0 {
		sb.WriteString("  <i>no ATM IV data</i>\n")
	} else {
		sb.WriteString(ivTermStructureChart(r.TermStructure))
	}
	sb.WriteString("\n")

	// Per-expiry skew table (up to 8 expiries)
	sb.WriteString("🔀 <b>SKEW PER EXPIRY</b>\n")
	sb.WriteString("<code>Expiry     DTE  ATM-IV  Skew  Signal</code>\n")
	count := 0
	for _, sl := range r.Expiries {
		if sl.ATMIV <= 0 && sl.PointCount < 5 {
			continue
		}
		if count >= 8 {
			break
		}
		sb.WriteString(ivSkewRow(sl))
		count++
	}
	if count == 0 {
		sb.WriteString("  <i>insufficient data</i>\n")
	}
	sb.WriteString("\n")

	// Smile legend
	sb.WriteString("<i>📖 Skew = Put wing IV − Call wing IV. Positive = bearish fear (put demand). Negative = call demand / bullish.</i>\n")

	return sb.String()
}

// ivSignalEmoji returns an emoji for the IV market signal.
func ivSignalEmoji(signal string) string {
	switch signal {
	case "FEAR":
		return "🔴"
	case "GREED":
		return "🟢"
	default:
		return "🟡"
	}
}

// ivTermStructureChart renders a compact ASCII bar chart of ATM IV vs DTE.
// Shows up to 8 data points.
func ivTermStructureChart(pts []gexsvc.TermPoint) string {
	if len(pts) == 0 {
		return "  <i>no data</i>\n"
	}
	// Find max IV for scaling.
	maxIV := 1.0
	for _, p := range pts {
		if p.ATMIV > maxIV {
			maxIV = p.ATMIV
		}
	}

	limit := 8
	if len(pts) < limit {
		limit = len(pts)
	}

	var sb strings.Builder
	for i := 0; i < limit; i++ {
		p := pts[i]
		ratio := p.ATMIV / maxIV
		bar := fmtutil.ProgressBar(ratio, 1, 10, "▓", "░")
		sb.WriteString(fmt.Sprintf("  %3dD [%s] <code>%.0f%%</code>\n", p.DTE, bar, p.ATMIV))
	}
	return sb.String()
}

// ivSkewRow formats a single expiry slice as a table row.
func ivSkewRow(sl gexsvc.ExpirySlice) string {
	expiryStr := sl.Expiry.Format("02Jan")
	atmStr := "  N/A"
	if sl.ATMIV > 0 {
		atmStr = fmt.Sprintf("%5.0f%%", sl.ATMIV)
	}
	skewStr := "  N/A"
	if sl.PutWingIV > 0 || sl.CallWingIV > 0 {
		skewStr = fmt.Sprintf("%+5.1f%%", sl.Skew25Delta)
	}
	smileEmoji := ivSmileEmoji(sl.SmileLabel)
	return fmt.Sprintf("<code>%-9s %3d  %s  %s</code> %s\n",
		expiryStr, sl.DTE, atmStr, skewStr, smileEmoji)
}

// ivSmileEmoji maps SmileLabel to an emoji indicator.
func ivSmileEmoji(label string) string {
	switch label {
	case "PUT_SKEW":
		return "📉 PUT"
	case "CALL_SKEW":
		return "📈 CALL"
	default:
		return "➖ FLAT"
	}
}
