package telegram

import (
	"fmt"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// FormatCarryMonitor formats the carry trade monitor dashboard with unwind detection.
func (f *Formatter) FormatCarryMonitor(r *domain.CarryMonitorResult) string {
	var b strings.Builder

	b.WriteString("💱 <b>CARRY TRADE MONITOR</b>\n")
	b.WriteString(fmt.Sprintf("<code>%s | Risk: %s %s</code>\n\n",
		r.AsOf, riskEmoji(r.Risk), string(r.Risk)))

	// Pair rankings
	for i, p := range r.Pairs {
		medal := fmt.Sprintf("%d.", i+1)
		if i == 0 {
			medal = "🥇"
		} else if i == 1 {
			medal = "🥈"
		} else if i == 2 {
			medal = "🥉"
		}

		dirIcon := "🟢 Long"
		if p.Spread < 0 {
			dirIcon = "🔴 Short"
		}

		bar := spreadBar(p.Spread)
		b.WriteString(fmt.Sprintf("%s <code>%s/USD  Spread:%+.0fbps  Daily:%+.2fbps</code> %s\n",
			medal, p.Currency, p.Spread, p.DailyAccrual, dirIcon))
		b.WriteString(fmt.Sprintf("   <code>%s %s</code>\n", bar, p.Direction))
	}

	// Spread range & unwind detection
	b.WriteString("\n<b>📊 Unwind Detection</b>\n")
	b.WriteString(fmt.Sprintf("<code>Spread Range : %.0f bps</code>\n", r.SpreadRange))
	if r.PrevRange > 0 {
		b.WriteString(fmt.Sprintf("<code>Prev Range   : %.0f bps</code>\n", r.PrevRange))
		b.WriteString(fmt.Sprintf("<code>Range Change  : %+.1f%%</code>\n", r.RangeChange))
	}
	b.WriteString(fmt.Sprintf("<code>Risk Level   : %s %s</code>\n",
		riskEmoji(r.Risk), riskLabel(r.Risk)))

	// Summary
	b.WriteString("\n<b>📋 Summary</b>\n")
	if r.BestCarry != "" {
		b.WriteString(fmt.Sprintf("<code>Best Carry : %s (most attractive long)</code>\n", r.BestCarry))
	}
	if r.WorstCarry != "" {
		b.WriteString(fmt.Sprintf("<code>Worst Carry: %s (most costly long)</code>\n", r.WorstCarry))
	}

	b.WriteString("\n<i>Spread = annualized rate differential in basis points</i>\n")
	b.WriteString("<i>Unwind risk: range compression signals carry trade reversal</i>")

	return b.String()
}

// spreadBar creates a visual bar for carry spread (in bps, roughly -500 to +500).
func spreadBar(spreadBps float64) string {
	const width = 10
	// Normalize: 500bps = full bar
	filled := int(math.Abs(spreadBps) / 500 * float64(width))
	if filled > width {
		filled = width
	}
	if spreadBps >= 0 {
		return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
	}
	return "[" + strings.Repeat("░", width-filled) + strings.Repeat("█", filled) + "]"
}

// riskEmoji returns an emoji for the unwind risk level.
func riskEmoji(risk domain.UnwindRisk) string {
	switch risk {
	case domain.UnwindAlert:
		return "🔴 Alert"
	case domain.UnwindNarrow:
		return "🟡 Narrow"
	default:
		return "🟢 Safe"
	}
}

// riskLabel returns a human-readable label for the unwind risk level.
func riskLabel(risk domain.UnwindRisk) string {
	switch risk {
	case domain.UnwindAlert:
		return "Carry Unwind Alert — spreads collapsing"
	case domain.UnwindNarrow:
		return "Narrowing — early warning, monitor closely"
	default:
		return "Normal — carry positions stable"
	}
}
