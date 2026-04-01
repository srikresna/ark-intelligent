package telegram

// formatter_ict.go — ICT/SMC result formatting for Telegram HTML messages.

import (
	"fmt"
	"strings"

	ictsvc "github.com/arkcode369/ark-intelligent/internal/service/ict"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatICTResult formats an ICTResult as a Telegram HTML message.
// Output is kept under 3000 characters for mobile readability.
func FormatICTResult(r *ictsvc.ICTResult) string {
	var sb strings.Builder

	// Header — uses fmtutil.AnalysisHeader for consistency.
	sb.WriteString(fmtutil.AnalysisHeader("🔷", "ICT/SMC ANALYSIS", r.Symbol, r.Timeframe))
	sb.WriteString(fmt.Sprintf("📅 %s", r.AnalyzedAt.Format("2006-01-02 15:04 UTC")))
	if r.Killzone != "" {
		sb.WriteString(fmt.Sprintf("\n⏰ <b>Killzone:</b> %s", r.Killzone))
	}
	sb.WriteString("\n\n")

	// Market Structure — uses fmtutil.BiasIcon.
	biasEmoji := fmtutil.BiasIcon(r.Bias)
	sb.WriteString(fmt.Sprintf("📐 <b>MARKET STRUCTURE:</b> %s %s\n", biasEmoji, r.Bias))
	structCount := 0
	for i := len(r.Structure) - 1; i >= 0 && structCount < 3; i-- {
		ev := r.Structure[i]
		icon := structureIcon(ev.Kind, ev.Direction)
		sb.WriteString(fmt.Sprintf("  %s %s at %.5f\n", icon, ev.Kind, ev.Level))
		structCount++
	}
	sb.WriteString("\n")

	// Order Blocks
	if len(r.OrderBlocks) > 0 {
		sb.WriteString(fmt.Sprintf("📦 <b>ORDER BLOCKS (%d)</b>\n", len(r.OrderBlocks)))
		for _, ob := range r.OrderBlocks {
			status := "valid"
			suffix := ""
			icon := fmtutil.BiasIcon(ob.Kind)
			if ob.Broken {
				status = "broken → Breaker"
				suffix = " ⚡"
			}
			sb.WriteString(fmt.Sprintf("  %s %s OB: %.5f–%.5f (%s%s)\n",
				icon, ob.Kind, ob.Bottom, ob.Top, status, suffix))
		}
		sb.WriteString("\n")
	}

	// Fair Value Gaps
	if len(r.FVGZones) > 0 {
		// Show only unfilled or partially filled FVGs (most recent 4).
		shown := 0
		fvgLines := make([]string, 0, 4)
		for i := len(r.FVGZones) - 1; i >= 0 && shown < 4; i-- {
			z := r.FVGZones[i]
			fillStr := fmt.Sprintf("%.0f%% filled", z.FillPct)
			if z.Filled {
				fillStr = "100% filled ✓"
			}
			arrow := fmtutil.DirectionIcon(z.Kind)
			fvgLines = append(fvgLines, fmt.Sprintf("  %s %s FVG: %.5f–%.5f (%s)",
				arrow, z.Kind, z.Bottom, z.Top, fillStr))
			shown++
		}
		sb.WriteString(fmt.Sprintf("⬜ <b>FAIR VALUE GAPS (%d)</b>\n", len(r.FVGZones)))
		// Reverse so newest is first.
		for i := len(fvgLines) - 1; i >= 0; i-- {
			sb.WriteString(fvgLines[i] + "\n")
		}
		sb.WriteString("\n")
	}

	// Liquidity Sweeps
	if len(r.Sweeps) > 0 {
		sb.WriteString(fmt.Sprintf("💧 <b>LIQUIDITY SWEEPS (%d)</b>\n", len(r.Sweeps)))
		for _, s := range r.Sweeps {
			revStr := ""
			if s.Reversed {
				revStr = " → Reversed"
				if s.Kind == "SWEEP_LOW" {
					revStr += " 📈 BULLISH"
				} else {
					revStr += " 📉 BEARISH"
				}
			}
			icon := "🔺"
			if s.Kind == "SWEEP_LOW" {
				icon = "🔻"
			}
			sb.WriteString(fmt.Sprintf("  %s %s at %.5f%s\n", icon, s.Kind, s.Level, revStr))
		}
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString(fmt.Sprintf("🎯 <b>SUMMARY:</b> %s\n", r.Summary))

	return sb.String()
}

// structureIcon returns a status icon for a structure event.
func structureIcon(kind, direction string) string {
	switch {
	case kind == "CHOCH" && direction == "BULLISH":
		return "⚠️ "
	case kind == "CHOCH" && direction == "BEARISH":
		return "⚠️ "
	case kind == "BOS" && direction == "BULLISH":
		return "✅"
	case kind == "BOS" && direction == "BEARISH":
		return "❌"
	default:
		return "•"
	}
}
