package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/finviz"
)

// formatMarketOverview builds the Telegram HTML output for /market.
func formatMarketOverview(o *finviz.MarketOverview) string {
	if o == nil || !o.Available {
		return "🌐 <b>Cross-Asset Market Overview</b>\n\n⚠️ Data not available. Ensure FIRECRAWL_API_KEY is configured."
	}

	var sb strings.Builder
	sb.WriteString("🌐 <b>Cross-Asset Market Overview</b>\n")
	sb.WriteString(fmt.Sprintf("📅 %s\n\n", o.FetchedAt.UTC().Format("2006-01-02 15:04 UTC")))

	// Risk tone banner.
	toneEmoji := "⚖️"
	switch o.RiskTone {
	case "RISK-ON":
		toneEmoji = "🟢"
	case "RISK-OFF":
		toneEmoji = "🔴"
	}
	sb.WriteString(fmt.Sprintf("%s <b>Market Tone: %s</b>\n\n", toneEmoji, o.RiskTone))

	// Futures by group.
	if len(o.Futures) > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("📊 <b>Futures</b>\n\n")

		groups := []string{"indices", "energy", "metals", "currencies", "bonds", "agriculture"}
		groupEmoji := map[string]string{
			"indices":     "📈",
			"energy":      "🛢",
			"metals":      "🥇",
			"currencies":  "💱",
			"bonds":       "🏛",
			"agriculture": "🌾",
		}
		groupTitle := map[string]string{
			"indices":     "Indices",
			"energy":      "Energy",
			"metals":      "Metals",
			"currencies":  "Currencies",
			"bonds":       "Bonds",
			"agriculture": "Agriculture",
		}

		for _, g := range groups {
			items := filterByGroup(o.Futures, g)
			if len(items) == 0 {
				continue
			}
			emoji := groupEmoji[g]
			sb.WriteString(fmt.Sprintf("%s <b>%s</b>\n", emoji, groupTitle[g]))
			for _, f := range items {
				arrow := "🟢"
				if f.Change < 0 {
					arrow = "🔴"
				} else if f.Change == 0 {
					arrow = "⚪"
				}
				name := f.Name
				if len(name) > 18 {
					name = name[:18]
				}
				sb.WriteString(fmt.Sprintf("  %s %-18s %+.2f%%\n", arrow, name, f.Change))
			}
			sb.WriteString("\n")
		}

		// Summary: green vs red.
		green, red := countGreenRed(o.Futures)
		sb.WriteString(fmt.Sprintf("  🟢 %d green  🔴 %d red\n\n", green, red))
	}

	// Sector performance.
	if len(o.Sectors) > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("🏭 <b>Sector Performance</b>\n\n")

		for _, s := range o.Sectors {
			arrow := "🟢"
			if s.Change1D < 0 {
				arrow = "🔴"
			}
			name := s.Name
			if len(name) > 20 {
				name = name[:20]
			}
			sb.WriteString(fmt.Sprintf("  %s %-20s 1D: %+.2f%% | 1W: %+.2f%% | 1M: %+.2f%%\n",
				arrow, name, s.Change1D, s.Change1W, s.Change1M))
		}

		// Leaders / laggards.
		if len(o.Sectors) >= 3 {
			sb.WriteString("\n")
			best, worst := topBottomSectors(o.Sectors)
			if best.Name != "" {
				sb.WriteString(fmt.Sprintf("  🏆 Leader: %s (%+.2f%%)\n", best.Name, best.Change1D))
			}
			if worst.Name != "" {
				sb.WriteString(fmt.Sprintf("  📉 Laggard: %s (%+.2f%%)\n", worst.Name, worst.Change1D))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("💡 <i>Risk-on = equities ↑ gold ↓ yields ↑\nRisk-off = equities ↓ gold ↑ yields ↓</i>\n")
	sb.WriteString("📊 Data: Finviz (delayed ~15 min)")

	return sb.String()
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func filterByGroup(items []finviz.FuturesItem, group string) []finviz.FuturesItem {
	var out []finviz.FuturesItem
	for _, f := range items {
		if strings.EqualFold(f.Group, group) {
			out = append(out, f)
		}
	}
	return out
}

func countGreenRed(items []finviz.FuturesItem) (green, red int) {
	for _, f := range items {
		if f.Change > 0 {
			green++
		} else if f.Change < 0 {
			red++
		}
	}
	return
}

func topBottomSectors(sectors []finviz.SectorItem) (best, worst finviz.SectorItem) {
	if len(sectors) == 0 {
		return
	}
	best = sectors[0]
	worst = sectors[0]
	for _, s := range sectors[1:] {
		if s.Change1D > best.Change1D {
			best = s
		}
		if s.Change1D < worst.Change1D {
			worst = s
		}
	}
	return
}
