package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/bis"
)

// FormatBISSummary formats the BIS Statistics dashboard for Telegram HTML.
func (f *Formatter) FormatBISSummary(data *bis.BISSummaryData) string {
	var b strings.Builder

	b.WriteString("🏦 <b>BIS STATISTICS DASHBOARD</b>\n")
	b.WriteString(fmt.Sprintf("📅 %s\n\n", time.Now().UTC().Format("02 Jan 2006 15:04 UTC")))

	// ── Section 1: Central Bank Policy Rates ──────────────────────────────
	b.WriteString("<code>━━━ Central Bank Policy Rates ━━━</code>\n\n")
	for _, r := range data.PolicyRates {
		if !r.Available {
			b.WriteString(fmt.Sprintf("⚪ <b>%s</b> — N/A\n", r.Label))
			continue
		}
		emoji := rateEmoji(r.Rate)
		b.WriteString(fmt.Sprintf("%s <b>%s</b>: <code>%.2f%%</code>",
			emoji, r.Label, r.Rate))
		if r.Period != "" {
			b.WriteString(fmt.Sprintf(" <i>(%s)</i>", r.Period))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// ── Section 2: Credit-to-GDP Gaps ─────────────────────────────────────
	b.WriteString("<code>━━━ Credit-to-GDP Gaps ━━━</code>\n")
	b.WriteString("<i>Source: BIS WS_CREDIT_GAP | &gt;2pp = WARNING</i>\n\n")

	hasAnyGap := false
	for _, g := range data.CreditGaps {
		if !g.Available {
			continue
		}
		hasAnyGap = true
		emoji := gapEmoji(g.Signal)
		sign := "+"
		if g.Gap < 0 {
			sign = ""
		}
		b.WriteString(fmt.Sprintf("%s <b>%s</b>: <code>%s%.1fpp</code> %s",
			emoji, g.Label, sign, g.Gap, signalBadge(g.Signal)))
		if g.Period != "" {
			b.WriteString(fmt.Sprintf(" <i>(%s)</i>", g.Period))
		}
		b.WriteString("\n")
	}
	if !hasAnyGap {
		b.WriteString("⚪ Credit gap data unavailable\n")
	}
	b.WriteString("\n")

	// ── Section 3: Global Liquidity (WS_GLI) ──────────────────────────────
	if len(data.GLIndicators) > 0 {
		b.WriteString("<code>━━━ Global Liquidity Indicators ━━━</code>\n\n")
		for _, g := range data.GLIndicators {
			if !g.Available {
				continue
			}
			b.WriteString(fmt.Sprintf("💧 <b>%s</b>: <code>$%.0fBn</code>",
				g.Label, g.ValueBn))
			if g.Period != "" {
				b.WriteString(fmt.Sprintf(" <i>(%s)</i>", g.Period))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// ── Footer ─────────────────────────────────────────────────────────────
	b.WriteString("<i>Source: BIS SDMX REST API • No-auth, monthly data</i>\n")
	b.WriteString(fmt.Sprintf("<i>Fetched: %s</i>", data.FetchedAt.UTC().Format("02 Jan 2006 15:04 UTC")))

	return b.String()
}

func rateEmoji(rate float64) string {
	switch {
	case rate >= 5.0:
		return "🔴"
	case rate >= 3.0:
		return "🟡"
	case rate >= 0.5:
		return "🟢"
	default:
		return "🔵" // near-zero / negative (accommodative)
	}
}

func gapEmoji(signal string) string {
	switch signal {
	case "WARNING":
		return "🔴"
	case "ELEVATED":
		return "🟡"
	default:
		return "🟢"
	}
}

func signalBadge(signal string) string {
	switch signal {
	case "WARNING":
		return "⚠️"
	case "ELEVATED":
		return "📈"
	default:
		return "✅"
	}
}
