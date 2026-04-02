package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/orderflow"
)

// formatOrderFlowResult renders an OrderFlowResult as Telegram HTML.
// Output is kept under 3000 chars.
func formatOrderFlowResult(r *orderflow.OrderFlowResult) string {
	if r == nil {
		return "⚠️ Tidak ada data order flow."
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("📊 <b>ORDER FLOW — %s %s</b> (%d bars)\n",
		r.Symbol, r.Timeframe, len(r.DeltaBars)))
	sb.WriteString(fmt.Sprintf("<i>%s UTC</i>\n\n", r.AnalyzedAt.UTC().Format("2006-01-02 15:04")))

	// Delta divergence
	switch r.PriceDeltaDivergence {
	case "BULLISH_DIV":
		sb.WriteString("⚡ <b>DELTA: BULLISH DIVERGENCE</b> ⬆️\n")
		sb.WriteString("  Price: Lower Low, Cum. Delta: Higher Low\n")
		sb.WriteString("  → Buyers menyerap tekanan jual\n\n")
	case "BEARISH_DIV":
		sb.WriteString("⚡ <b>DELTA: BEARISH DIVERGENCE</b> ⬇️\n")
		sb.WriteString("  Price: Higher High, Cum. Delta: Lower High\n")
		sb.WriteString("  → Sellers menyerap tekanan beli\n\n")
	default:
		trendEmoji := deltaTrendEmoji(r.DeltaTrend)
		sb.WriteString(fmt.Sprintf("⚡ <b>DELTA TREND: %s</b> %s\n\n", r.DeltaTrend, trendEmoji))
	}

	// Last 5 delta bars
	displayN := 5
	if len(r.DeltaBars) < displayN {
		displayN = len(r.DeltaBars)
	}
	sb.WriteString("📊 <b>Delta Bars (last 5):</b>\n")
	for i := 0; i < displayN; i++ {
		db := r.DeltaBars[i]
		dirEmoji := "🔴 Sell"
		if db.Delta >= 0 {
			dirEmoji = "🟢 Buy"
		}
		direction := "▼"
		if db.OHLCV.Close >= db.OHLCV.Open {
			direction = "▲"
		}
		note := ""
		for _, idx := range r.BullishAbsorption {
			if idx == i {
				note = " (bullish absorption?)"
				break
			}
		}
		for _, idx := range r.BearishAbsorption {
			if idx == i {
				note = " (bearish absorption?)"
				break
			}
		}
		sb.WriteString(fmt.Sprintf("  %s %s %.5g %s delta %+.0f%s\n",
			dirEmoji, direction, db.OHLCV.Close, ordBarLabel(i), db.Delta, note))
	}
	sb.WriteString("\n")

	// Point of Control
	sb.WriteString(fmt.Sprintf("🎯 <b>POINT OF CONTROL:</b> %.5g (highest volume zone)\n\n", r.PointOfControl))

	// Absorption
	if len(r.BullishAbsorption) > 0 {
		sb.WriteString(fmt.Sprintf("🔰 <b>BULLISH ABSORPTION:</b> %d bar terdeteksi\n", len(r.BullishAbsorption)))
		sb.WriteString("   → Heavy selling, range sempit — buyers menyerap supply\n\n")
	}
	if len(r.BearishAbsorption) > 0 {
		sb.WriteString(fmt.Sprintf("🔰 <b>BEARISH ABSORPTION:</b> %d bar terdeteksi\n", len(r.BearishAbsorption)))
		sb.WriteString("   → Heavy buying, range sempit — sellers menyerap demand\n\n")
	}

	// Cumulative delta
	cumEmoji := "➖"
	if r.CumDelta > 0 {
		cumEmoji = "⬆️"
	} else if r.CumDelta < 0 {
		cumEmoji = "⬇️"
	}
	sb.WriteString(fmt.Sprintf("📈 <b>Cumulative Delta:</b> %+.0f %s\n\n", r.CumDelta, cumEmoji))

	// Bias
	biasEmoji := ofBiasEmoji(r.Bias)
	sb.WriteString(fmt.Sprintf("🧭 <b>BIAS: %s</b> %s\n\n", r.Bias, biasEmoji))

	// Summary
	sb.WriteString(fmt.Sprintf("💡 <i>%s</i>", r.Summary))

	return sb.String()
}

func deltaTrendEmoji(trend string) string {
	switch trend {
	case "RISING":
		return "📈"
	case "FALLING":
		return "📉"
	default:
		return "➡️"
	}
}

func ofBiasEmoji(bias string) string {
	switch bias {
	case "BULLISH":
		return "🟢 Bullish"
	case "BEARISH":
		return "🔴 Bearish"
	default:
		return "⚪ Neutral"
	}
}

func ordBarLabel(i int) string {
	if i == 0 {
		return "← current"
	}
	return ""
}
