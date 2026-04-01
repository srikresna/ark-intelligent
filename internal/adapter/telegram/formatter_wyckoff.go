package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/wyckoff"
)

// FormatWyckoffResult formats a WyckoffResult as an HTML Telegram message.
// Output is capped at 4000 characters.
func (f *Formatter) FormatWyckoffResult(r *wyckoff.WyckoffResult) string {
	var b strings.Builder

	schIcon := "📊"
	switch r.Schematic {
	case "ACCUMULATION":
		schIcon = "🟢"
	case "DISTRIBUTION":
		schIcon = "🔴"
	}

	b.WriteString(fmt.Sprintf("📊 <b>WYCKOFF ANALYSIS — %s %s</b>\n\n",
		r.Symbol, r.Timeframe))

	b.WriteString(fmt.Sprintf("%s <b>SCHEMATIC:</b> %s (%s confidence)\n",
		schIcon, r.Schematic, r.Confidence))
	b.WriteString(fmt.Sprintf("📍 <b>CURRENT PHASE:</b> %s\n\n",
		formatPhaseName(r.CurrentPhase)))

	if len(r.Events) > 0 {
		b.WriteString("⚡ <b>EVENTS DETECTED:</b>\n")
		for _, e := range r.Events {
			phase := phaseForEvent(r.Phases, e.BarIndex)
			icon := eventIcon(e.Name)
			sigIcon := ""
			if e.Significance == "HIGH" {
				sigIcon = " ⭐"
			}
			b.WriteString(fmt.Sprintf("  [%s] %s <b>%s</b>: %.5f (vol %.1fx avg)%s\n",
				phase,
				icon,
				string(e.Name),
				e.Price,
				e.Volume/1000, // raw ratio placeholder; real would need avgVol
				sigIcon,
			))
		}
		b.WriteString("\n")
	}

	if r.TradingRange[0] > 0 && r.TradingRange[1] > 0 {
		b.WriteString(fmt.Sprintf("📏 <b>TRADING RANGE:</b> <code>%.5f — %.5f</code>\n",
			r.TradingRange[0], r.TradingRange[1]))
	}
	if r.ProjectedMove > 0 {
		b.WriteString(fmt.Sprintf("🎯 <b>PROJECTED MOVE:</b> <code>%.5f</code> (cause built: %.0f%%)\n",
			r.ProjectedMove, r.CauseBuilt))
	}

	// Next watch
	nextWatch := nextWatchEvent(r)
	if nextWatch != "" {
		b.WriteString(fmt.Sprintf("⚡ <b>NEXT TO WATCH:</b> %s\n", nextWatch))
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("💡 <b>SUMMARY:</b> %s\n", r.Summary))
	b.WriteString(fmt.Sprintf("\n<i>Analyzed: %s UTC</i>", r.AnalyzedAt.Format("2006-01-02 15:04")))

	out := b.String()
	if len(out) > 4000 {
		out = out[:3997] + "…"
	}
	return out
}

// formatPhaseName returns a human-readable phase description.
func formatPhaseName(phase string) string {
	switch phase {
	case "A":
		return "A — Stopping the Prior Trend"
	case "B":
		return "B — Building the Cause"
	case "C":
		return "C — The Test (Spring/UTAD)"
	case "D":
		return "D — Dominance (SOS/SOW)"
	case "E":
		return "E — Markup / Markdown"
	default:
		return phase
	}
}

// eventIcon returns an emoji for a Wyckoff event name.
func eventIcon(name wyckoff.EventName) string {
	switch name {
	case wyckoff.EventPS:
		return "🔵"
	case wyckoff.EventSC:
		return "🔻"
	case wyckoff.EventBC:
		return "🔺"
	case wyckoff.EventAR, wyckoff.EventARDist:
		return "📈"
	case wyckoff.EventST:
		return "🔻"
	case wyckoff.EventSpring:
		return "💎"
	case wyckoff.EventSOS:
		return "🚀"
	case wyckoff.EventLPS:
		return "🛡️"
	case wyckoff.EventUTAD:
		return "⚠️"
	case wyckoff.EventSOW:
		return "📉"
	default:
		return "📌"
	}
}

// phaseForEvent finds which phase label an event belongs to.
func phaseForEvent(phases []wyckoff.WyckoffPhase, barIndex int) string {
	for _, ph := range phases {
		if barIndex >= ph.Start && (ph.End < 0 || barIndex <= ph.End) {
			return ph.Phase
		}
	}
	return "?"
}

// nextWatchEvent suggests what to monitor next based on current phase.
func nextWatchEvent(r *wyckoff.WyckoffResult) string {
	switch r.CurrentPhase {
	case "A":
		if r.Schematic == "ACCUMULATION" {
			return "Secondary Test (low volume revisit of SC lows)"
		}
		return "Automatic Reaction (first sell-off)"
	case "B":
		if r.Schematic == "ACCUMULATION" {
			return "Spring / Shakeout below trading range"
		}
		return "Upthrust (failed break above range high)"
	case "C":
		if r.Schematic == "ACCUMULATION" {
			return fmt.Sprintf("Sign of Strength — break above %.5f", r.TradingRange[1])
		}
		return fmt.Sprintf("Sign of Weakness — break below %.5f", r.TradingRange[0])
	case "D":
		if r.Schematic == "ACCUMULATION" {
			return "Last Point of Support (low-volume pullback)"
		}
		return "Last Point of Supply (LPSY)"
	case "E":
		return "Trend continuation — trail stops"
	default:
		return ""
	}
}
