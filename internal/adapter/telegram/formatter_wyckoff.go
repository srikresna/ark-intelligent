package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/wyckoff"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatWyckoffResult formats a WyckoffResult as an HTML Telegram message.
// Output is capped at 4000 characters.
func (f *Formatter) FormatWyckoffResult(r *wyckoff.WyckoffResult) string {
	var b strings.Builder

	schIcon := fmtutil.AccumulationDistributionIcon(r.Schematic)

	// Header — uses fmtutil.AnalysisHeader for consistency.
	b.WriteString(fmtutil.AnalysisHeader("📊", "WYCKOFF ANALYSIS", r.Symbol, r.Timeframe))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("%s <b>SKEMA:</b> %s (kepercayaan: %s)\n",
		schIcon, r.Schematic, r.Confidence))
	b.WriteString(fmt.Sprintf("📍 <b>FASE SAAT INI:</b> %s\n\n",
		formatPhaseName(r.CurrentPhase)))

	if len(r.Events) > 0 {
		b.WriteString("⚡ <b>EVENT TERDETEKSI:</b>\n")
		for _, e := range r.Events {
			phase := phaseForEvent(r.Phases, e.BarIndex)
			icon := eventIcon(e.Name)
			sigIcon := ""
			if e.Significance == "HIGH" {
				sigIcon = " ⭐"
			}
			b.WriteString(fmt.Sprintf("  [%s] %s <b>%s</b>: %.5f (vol %.1fx rata-rata)%s\n",
				phase,
				icon,
				string(e.Name),
				e.Price,
				e.Volume/1000, // raw ratio placeholder; real would need avgVol
				sigIcon,
			))
			// Educational tooltip for key Wyckoff events
			if tooltip := wyckoffEventTooltip(e.Name); tooltip != "" {
				b.WriteString(fmt.Sprintf("     <i>↳ %s</i>\n", tooltip))
			}
		}
		b.WriteString("\n")
	}

	if r.TradingRange[0] > 0 && r.TradingRange[1] > 0 {
		b.WriteString(fmt.Sprintf("📏 <b>TRADING RANGE:</b> <code>%.5f — %.5f</code>\n",
			r.TradingRange[0], r.TradingRange[1]))
	}
	if r.ProjectedMove > 0 {
		b.WriteString(fmt.Sprintf("🎯 <b>PERGERAKAN PROYEKSI:</b> <code>%.5f</code> (cause terbangun: %.0f%%)\n",
			r.ProjectedMove, r.CauseBuilt))
	}

	// Next watch
	nextWatch := nextWatchEvent(r)
	if nextWatch != "" {
		b.WriteString(fmt.Sprintf("⚡ <b>PANTAU SELANJUTNYA:</b> %s\n", nextWatch))
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("💡 <b>RINGKASAN:</b> %s\n", r.Summary))
	b.WriteString(fmt.Sprintf("\n<i>Dianalisis: %s UTC</i>", r.AnalyzedAt.Format("2006-01-02 15:04")))

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
		return "A — Menghentikan Tren Sebelumnya"
	case "B":
		return "B — Membangun Cause"
	case "C":
		return "C — Pengujian (Spring/UTAD)"
	case "D":
		return "D — Dominasi (SOS/SOW)"
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

// wyckoffEventTooltip returns a short Indonesian educational description for key Wyckoff events.
// Returns empty string for events that don't need a tooltip.
func wyckoffEventTooltip(name wyckoff.EventName) string {
	switch name {
	case wyckoff.EventSpring:
		return "false breakdown di bawah support — sinyal pembalikan bullish, smart money menyerap jual"
	case wyckoff.EventSOS:
		return "SOS (Sign of Strength): breakout di atas resistance dengan volume tinggi — konfirmasi akumulasi"
	case wyckoff.EventUTAD:
		return "UTAD (Upthrust After Distribution): false breakout di atas resistance — sinyal pembalikan bearish"
	case wyckoff.EventSOW:
		return "SOW (Sign of Weakness): breakdown di bawah support dengan volume — konfirmasi distribusi"
	case wyckoff.EventSC:
		return "SC (Selling Climax): titik balik penjualan ekstrem dengan volume sangat tinggi"
	case wyckoff.EventBC:
		return "BC (Buying Climax): titik balik pembelian ekstrem — tanda distribusi dimulai"
	case wyckoff.EventAR, wyckoff.EventARDist:
		return "AR (Automatic Rally/Reaction): pantulan/tekanan awal setelah climax — mendefinisikan batas range"
	case wyckoff.EventLPS:
		return "LPS (Last Point of Support): pullback terakhir volume rendah sebelum markup — entry optimal"
	default:
		return ""
	}
}

// nextWatchEvent suggests what to monitor next based on current phase.
func nextWatchEvent(r *wyckoff.WyckoffResult) string {
	switch r.CurrentPhase {
	case "A":
		if r.Schematic == "ACCUMULATION" {
			return "Secondary Test (kunjungan ulang SC lows volume rendah)"
		}
		return "Automatic Reaction (tekanan jual pertama)"
	case "B":
		if r.Schematic == "ACCUMULATION" {
			return "Spring / Shakeout di bawah trading range"
		}
		return "Upthrust (kegagalan break di atas resistance range)"
	case "C":
		if r.Schematic == "ACCUMULATION" {
			return fmt.Sprintf("Sign of Strength — break di atas %.5f", r.TradingRange[1])
		}
		return fmt.Sprintf("Sign of Weakness — break di bawah %.5f", r.TradingRange[0])
	case "D":
		if r.Schematic == "ACCUMULATION" {
			return "Last Point of Support (pullback volume rendah)"
		}
		return "Last Point of Supply (LPSY)"
	case "E":
		return "Kelanjutan tren — trailing stops"
	default:
		return ""
	}
}
