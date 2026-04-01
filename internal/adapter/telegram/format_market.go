package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/finviz"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// FormatMarket renders the Finviz cross-asset dashboard as Telegram HTML.
func (f *Formatter) FormatMarket(data *finviz.CrossAssetData) string {
	var b strings.Builder

	// Header with risk tone.
	toneEmoji := riskToneEmoji(data.RiskTone)
	b.WriteString(fmt.Sprintf("<b>📊 Cross-Asset Dashboard</b> %s <b>%s</b>\n\n", toneEmoji, data.RiskTone))

	// Futures green/red summary.
	green, red := finviz.GreenRedCount(data.Futures)
	if len(data.Futures) > 0 {
		b.WriteString(fmt.Sprintf("Futures: 🟢 %d green · 🔴 %d red (of %d)\n\n", green, red, len(data.Futures)))
	}

	// Futures by category.
	categories := []struct {
		label    string
		emoji    string
		category string
	}{
		{"Indices", "📈", "indices"},
		{"Energy", "🛢", "energy"},
		{"Metals", "🥇", "metals"},
		{"Currencies", "💱", "currencies"},
		{"Bonds", "🏛", "bonds"},
	}

	for _, cat := range categories {
		items := finviz.FuturesByCategory(data.Futures, cat.category)
		if len(items) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("<b>%s %s</b>\n", cat.emoji, cat.label))
		for _, item := range items {
			arrow := changeArrow(item.Change)
			b.WriteString(fmt.Sprintf("  %s %s: %s\n", arrow, item.Name, fmtutil.FmtPct(item.Change)))
		}
		b.WriteString("\n")
	}

	// Sector performance.
	if len(data.Sectors) > 0 {
		b.WriteString("<b>🏢 S&amp;P Sectors</b>\n")

		top := finviz.TopSectors(data.Sectors, 3)
		bottom := finviz.BottomSectors(data.Sectors, 3)

		b.WriteString("<i>Leaders:</i>\n")
		for _, s := range top {
			b.WriteString(fmt.Sprintf("  🟢 %s: %s (1W: %s)\n",
				s.Name, fmtutil.FmtPct(s.Change1D), fmtutil.FmtPct(s.Change1W)))
		}
		b.WriteString("<i>Laggards:</i>\n")
		for _, s := range bottom {
			b.WriteString(fmt.Sprintf("  🔴 %s: %s (1W: %s)\n",
				s.Name, fmtutil.FmtPct(s.Change1D), fmtutil.FmtPct(s.Change1W)))
		}
		b.WriteString("\n")
	}

	// Footer.
	b.WriteString(fmt.Sprintf("<i>Source: Finviz · %s</i>",
		fmtutil.FormatDateTimeWIB(data.FetchedAt)))

	return b.String()
}

// riskToneEmoji returns an emoji for the risk tone.
func riskToneEmoji(tone string) string {
	switch tone {
	case "RISK-ON":
		return "🟢"
	case "RISK-OFF":
		return "🔴"
	default:
		return "🟡"
	}
}

// changeArrow returns a directional emoji for a % change.
func changeArrow(change float64) string {
	switch {
	case change > 0.5:
		return "🟢"
	case change < -0.5:
		return "🔴"
	default:
		return "⚪"
	}
}
