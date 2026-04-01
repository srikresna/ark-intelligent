package telegram

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/treasury"
)

// FormatTreasury formats the Treasury auction results dashboard.
func (f *Formatter) FormatTreasury(data *treasury.TreasuryData) string {
	var b strings.Builder

	b.WriteString("🏛️ <b>US TREASURY AUCTION RESULTS</b>\n")
	b.WriteString(fmt.Sprintf("📅 %s\n\n", time.Now().UTC().Format("02 Jan 2006 15:04 UTC")))

	// Show trend analyses for key tenors.
	if len(data.Analyses) > 0 {
		b.WriteString("<code>━━━ Demand Analysis ━━━</code>\n\n")

		for _, a := range data.Analyses {
			if a.Count < 2 {
				continue
			}

			demandEmoji := "⚪"
			switch a.DemandSignal {
			case "STRONG":
				demandEmoji = "🟢"
			case "WEAK":
				demandEmoji = "🔴"
			}

			b.WriteString(fmt.Sprintf("%s <b>%s</b> (%d auctions)\n",
				demandEmoji, html.EscapeString(a.SecurityTerm), a.Count))

			// Bid-to-cover.
			btcArrow := "→"
			switch a.BidToCoverTrend {
			case "IMPROVING":
				btcArrow = "↑"
			case "DETERIORATING":
				btcArrow = "↓"
			}
			b.WriteString(fmt.Sprintf("  B/C: <code>%.2f</code> %s (avg: <code>%.2f</code>) %s\n",
				a.LatestBidToCover, btcArrow, a.AvgBidToCover, a.BidToCoverTrend))

			// Indirect bidders.
			if a.LatestIndirect > 0 {
				indArrow := "→"
				switch a.IndirectTrend {
				case "RISING":
					indArrow = "↑"
				case "FALLING":
					indArrow = "↓"
				}
				b.WriteString(fmt.Sprintf("  Indirect: <code>%.1f%%</code> %s (avg: <code>%.1f%%</code>) %s\n",
					a.LatestIndirect, indArrow, a.AvgIndirect, a.IndirectTrend))
			}

			b.WriteString("\n")
		}
	}

	// Show recent individual auctions (top 10).
	b.WriteString("<code>━━━ Recent Auctions ━━━</code>\n\n")

	shown := 0
	for _, a := range data.Auctions {
		if shown >= 10 {
			break
		}
		if a.BidToCover <= 0 {
			continue // skip auctions without results yet
		}

		demandEmoji := "⚪"
		if a.BidToCover >= 2.5 {
			demandEmoji = "🟢"
		} else if a.BidToCover < 2.0 {
			demandEmoji = "🔴"
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n",
			demandEmoji,
			html.EscapeString(a.SecurityTerm),
			a.AuctionDate.Format("02 Jan")))
		b.WriteString(fmt.Sprintf("  Yield: <code>%.3f%%</code> | B/C: <code>%.2f</code>",
			a.HighYield, a.BidToCover))
		if a.IndirectPct > 0 {
			b.WriteString(fmt.Sprintf(" | Indirect: <code>%.1f%%</code>", a.IndirectPct))
		}
		b.WriteString("\n")

		shown++
	}

	if shown == 0 {
		b.WriteString("No recent auction results available.\n")
	}

	// Add interpretation guide.
	b.WriteString("\n<code>━━━ Legend ━━━</code>\n")
	b.WriteString("🟢 Strong demand (B/C ≥ 2.5)\n")
	b.WriteString("🔴 Weak demand (B/C < 2.0)\n")
	b.WriteString("Indirect = foreign central banks (USD demand proxy)\n")
	b.WriteString("B/C trend ↑ = improving demand | ↓ = deteriorating\n")

	// Weak auction alert.
	for _, a := range data.Analyses {
		if a.DemandSignal == "WEAK" {
			b.WriteString(fmt.Sprintf("\n⚠️ <b>WEAK AUCTION ALERT:</b> %s B/C <code>%.2f</code> below threshold\n",
				html.EscapeString(a.SecurityTerm), a.LatestBidToCover))
		}
	}

	return b.String()
}
