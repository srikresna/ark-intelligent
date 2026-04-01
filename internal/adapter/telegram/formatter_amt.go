package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// AMT Formatter — formats AMT Day Type and Opening Type analysis
// ---------------------------------------------------------------------------

// FormatAMTDayType formats an AMTDayTypeResult as an HTML Telegram message.
func (f *Formatter) FormatAMTDayType(symbol string, r *ta.AMTDayTypeResult) string {
	if r == nil || len(r.Days) == 0 {
		return "❌ Tidak cukup data intraday untuk klasifikasi AMT."
	}

	var b strings.Builder

	biasIcon := "⚖️"
	switch r.Bias {
	case "BULLISH":
		biasIcon = "🟢"
	case "BEARISH":
		biasIcon = "🔴"
	}

	migIcon := "➡️"
	switch r.ValueMigration {
	case "HIGHER":
		migIcon = "⬆️"
	case "LOWER":
		migIcon = "⬇️"
	case "MIXED":
		migIcon = "↔️"
	}

	b.WriteString(fmt.Sprintf("🏛 <b>AMT DAY TYPE — %s</b>\n\n", symbol))

	// Summary
	b.WriteString(fmt.Sprintf("%s <b>Bias:</b> %s\n", biasIcon, r.Bias))
	b.WriteString(fmt.Sprintf("%s <b>Value Migration:</b> %s\n", migIcon, r.ValueMigration))
	if r.ConsecutiveTrendDays > 1 {
		b.WriteString(fmt.Sprintf("🔥 <b>Trend berurutan:</b> %d hari\n", r.ConsecutiveTrendDays))
	}
	b.WriteString("\n")

	// Per-day table (newest first = Days[0])
	b.WriteString("📅 <b>Klasifikasi Hari:</b>\n")
	shown := len(r.Days)
	if shown > 6 {
		shown = 6
	}
	for i := 0; i < shown; i++ {
		d := r.Days[i]
		icon := dayTypeIcon(d.Type)
		dateStr := d.Date.Format("Jan 02")
		b.WriteString(fmt.Sprintf("  <b>%s</b> %s <code>%s</code> — IB:%.0f%% %s\n",
			dateStr, icon, string(d.Type), d.IBPercent, d.Description))
	}
	b.WriteString("\n")

	// Today detail
	today := r.Days[0]
	b.WriteString("🔍 <b>Detail Hari Ini:</b>\n")
	b.WriteString(fmt.Sprintf("  Range: <code>%.5f — %.5f</code> (%.5f)\n",
		today.DayLow, today.DayHigh, today.DayRange))
	b.WriteString(fmt.Sprintf("  IB: <code>%.5f — %.5f</code> (%.0f%% dari range)\n",
		today.IBLow, today.IBHigh, today.IBPercent))
	if today.ExtensionUp > 0 {
		b.WriteString(fmt.Sprintf("  ⬆ Ekstensi Atas: <code>%.5f</code>\n", today.ExtensionUp))
	}
	if today.ExtensionDown > 0 {
		b.WriteString(fmt.Sprintf("  ⬇ Ekstensi Bawah: <code>%.5f</code>\n", today.ExtensionDown))
	}
	b.WriteString(fmt.Sprintf("  Volume: atas <b>%.0f%%</b> / bawah <b>%.0f%%</b>\n",
		today.UpperVolumeRatio*100, today.LowerVolumeRatio*100))

	b.WriteString(fmt.Sprintf("\n💡 <i>%s</i>", today.Description))
	b.WriteString(fmt.Sprintf("\n\n<i>Sumber: 30m bars. IB = 2 periode pertama.</i>"))

	out := b.String()
	if len(out) > 4000 {
		out = out[:3997] + "…"
	}
	return out
}

// FormatAMTOpening formats an AMTOpeningResult as an HTML Telegram message.
func (f *Formatter) FormatAMTOpening(symbol string, r *ta.AMTOpeningResult) string {
	if r == nil {
		return "❌ Tidak cukup data untuk analisis opening AMT."
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("🌅 <b>AMT OPENING TYPE — %s</b>\n\n", symbol))

	today := r.Today
	icon := openingTypeIcon(today.Type)
	confIcon := confidenceIcon(today.Confidence)

	b.WriteString(fmt.Sprintf("%s <b>Tipe Opening:</b> %s %s (%s)\n",
		icon, string(today.Type), confIcon, today.Confidence))
	b.WriteString(fmt.Sprintf("📍 <b>Posisi Open:</b> %s\n", formatOpenLocation(today.OpenLocation)))
	b.WriteString(fmt.Sprintf("💰 <b>Open:</b> <code>%.5f</code>\n\n", today.OpenPrice))

	// Yesterday's Value Area
	va := today.YesterdayVA
	if va.POC > 0 {
		b.WriteString("📊 <b>Value Area Kemarin:</b>\n")
		b.WriteString(fmt.Sprintf("  VAH: <code>%.5f</code>\n", va.VAH))
		b.WriteString(fmt.Sprintf("  POC: <code>%.5f</code>\n", va.POC))
		b.WriteString(fmt.Sprintf("  VAL: <code>%.5f</code>\n\n", va.VAL))
	}

	// Trading implication
	b.WriteString(fmt.Sprintf("🎯 <b>Implikasi:</b> %s\n", today.Implication))

	// Win rates if available
	if len(r.WinRates) > 0 {
		b.WriteString("\n📈 <b>Win Rate Historis (20 hari):</b>\n")
		for _, ot := range []ta.OpeningType{
			ta.OpenDrive, ta.OpenTestDrive,
			ta.OpenRejectionReverse, ta.OpenAuction,
		} {
			wr, ok := r.WinRates[ot]
			if !ok {
				continue
			}
			bar := winRateBar(wr)
			b.WriteString(fmt.Sprintf("  %s <b>%s:</b> %.0f%% %s\n",
				openingTypeIcon(ot), string(ot), wr*100, bar))
		}
	}

	// History summary
	if len(r.History) > 0 {
		b.WriteString("\n📅 <b>Riwayat Opening (5 hari):</b>\n")
		start := len(r.History) - 1
		count := 0
		for i := start; i >= 0 && count < 5; i-- {
			h := r.History[i]
			b.WriteString(fmt.Sprintf("  %s <b>%s</b> — %s %s\n",
				openingTypeIcon(h.Type),
				h.Date.Format("Jan 02"),
				string(h.Type),
				confidenceIcon(h.Confidence)))
			count++
		}
	}

	b.WriteString(fmt.Sprintf("\n<i>Available 30 menit setelah market open.</i>"))

	out := b.String()
	if len(out) > 4000 {
		out = out[:3997] + "…"
	}
	return out
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func dayTypeIcon(dt ta.DayType) string {
	switch dt {
	case ta.DayTypeTrend:
		return "🚀"
	case ta.DayTypeNormal:
		return "⚖️"
	case ta.DayTypeNormalVariation:
		return "📊"
	case ta.DayTypeDoubleDistribution:
		return "🎭"
	case ta.DayTypePShape:
		return "🅿️"
	case ta.DayTypeBShape:
		return "🅱️"
	default:
		return "❓"
	}
}

func openingTypeIcon(ot ta.OpeningType) string {
	switch ot {
	case ta.OpenDrive:
		return "🚀"
	case ta.OpenTestDrive:
		return "🔄"
	case ta.OpenRejectionReverse:
		return "↩️"
	case ta.OpenAuction:
		return "⚖️"
	default:
		return "❓"
	}
}

func confidenceIcon(c string) string {
	switch c {
	case "HIGH":
		return "🟢"
	case "MEDIUM":
		return "🟡"
	case "LOW":
		return "🔴"
	default:
		return ""
	}
}

func formatOpenLocation(loc string) string {
	switch loc {
	case "ABOVE_VA":
		return "Di atas Value Area (premium)"
	case "BELOW_VA":
		return "Di bawah Value Area (discount)"
	case "INSIDE_VA":
		return "Di dalam Value Area"
	default:
		return loc
	}
}

func winRateBar(wr float64) string {
	pct := int(wr * 10)
	if pct > 10 {
		pct = 10
	}
	return strings.Repeat("█", pct) + strings.Repeat("░", 10-pct)
}
