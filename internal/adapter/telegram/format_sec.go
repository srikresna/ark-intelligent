package telegram

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/sec"
)

// FormatSEC13F formats the SEC EDGAR 13F institutional holdings dashboard.
func (f *Formatter) FormatSEC13F(data *sec.EdgarData) string {
	var b strings.Builder

	b.WriteString("🏛️ <b>SEC EDGAR — 13F INSTITUTIONAL HOLDINGS</b>\n")
	b.WriteString(fmt.Sprintf("📅 %s\n\n", time.Now().UTC().Format("02 Jan 2006 15:04 UTC")))

	if len(data.Reports) == 0 {
		b.WriteString("No 13F data available at this time.\n")
		return b.String()
	}

	for _, report := range data.Reports {
		f.formatInstitutionReport(&b, report)
	}

	// Legend.
	b.WriteString("<code>━━━ Legend ━━━</code>\n")
	b.WriteString("🟢 NEW = new position this quarter\n")
	b.WriteString("🔴 EXIT = completely sold\n")
	b.WriteString("⬆️ INCREASE | ⬇️ DECREASE in shares\n")
	b.WriteString("Values in $thousands (13F convention)\n")

	return b.String()
}

// formatInstitutionReport formats a single institution's 13F report.
func (f *Formatter) formatInstitutionReport(b *strings.Builder, report sec.InstitutionReport) {
	b.WriteString(fmt.Sprintf("<code>━━━ %s ━━━</code>\n", html.EscapeString(report.Institution.Name)))

	if report.LatestFiling == nil {
		b.WriteString("  No recent 13F filing found.\n\n")
		return
	}

	filing := report.LatestFiling
	b.WriteString(fmt.Sprintf("📄 Filed: %s | Period: %s\n",
		filing.FilingDate.Format("02 Jan 2006"),
		filing.ReportDate.Format("Q1 2006")))
	b.WriteString(fmt.Sprintf("💰 Total portfolio: <code>$%s</code>\n\n",
		formatValueK(filing.TotalValue)))

	// Top holdings (show top 5).
	if len(report.TopHoldings) > 0 {
		b.WriteString("<b>Top Holdings:</b>\n")
		shown := 0
		for _, h := range report.TopHoldings {
			if shown >= 5 {
				break
			}
			pct := float64(0)
			if filing.TotalValue > 0 {
				pct = (h.Value / filing.TotalValue) * 100
			}
			suffix := ""
			if h.PutCall != "" {
				suffix = fmt.Sprintf(" (%s)", h.PutCall)
			}
			b.WriteString(fmt.Sprintf("  • %s%s — <code>$%s</code> (%.1f%%)\n",
				html.EscapeString(truncate(h.Issuer, 30)), suffix,
				formatValueK(h.Value), pct))
			shown++
		}
		b.WriteString("\n")
	}

	// Significant moves (new positions + exits + big changes).
	hasChanges := false

	if len(report.NewPositions) > 0 {
		hasChanges = true
		b.WriteString("<b>🟢 New Positions:</b>\n")
		shown := 0
		for _, c := range report.NewPositions {
			if shown >= 3 {
				break
			}
			b.WriteString(fmt.Sprintf("  + %s — <code>$%s</code>\n",
				html.EscapeString(truncate(c.Issuer, 30)),
				formatValueK(c.CurrValue)))
			shown++
		}
		if len(report.NewPositions) > 3 {
			b.WriteString(fmt.Sprintf("  <i>... +%d more new positions</i>\n", len(report.NewPositions)-3))
		}
		b.WriteString("\n")
	}

	if len(report.Exits) > 0 {
		hasChanges = true
		b.WriteString("<b>🔴 Exits (Sold Entirely):</b>\n")
		shown := 0
		for _, c := range report.Exits {
			if shown >= 3 {
				break
			}
			b.WriteString(fmt.Sprintf("  − %s — was <code>$%s</code>\n",
				html.EscapeString(truncate(c.Issuer, 30)),
				formatValueK(c.PrevValue)))
			shown++
		}
		if len(report.Exits) > 3 {
			b.WriteString(fmt.Sprintf("  <i>... +%d more exits</i>\n", len(report.Exits)-3))
		}
		b.WriteString("\n")
	}

	// Show top increases/decreases (skip unchanged).
	if len(report.Changes) > 0 {
		var increases, decreases []sec.PortfolioChange
		for _, c := range report.Changes {
			switch c.ChangeType {
			case "INCREASE":
				increases = append(increases, c)
			case "DECREASE":
				decreases = append(decreases, c)
			}
		}

		if len(increases) > 0 {
			hasChanges = true
			b.WriteString("<b>⬆️ Top Increases:</b>\n")
			shown := 0
			for _, c := range increases {
				if shown >= 3 {
					break
				}
				b.WriteString(fmt.Sprintf("  ↑ %s — <code>%+.0f%%</code> ($%s → $%s)\n",
					html.EscapeString(truncate(c.Issuer, 25)),
					c.PctChange,
					formatValueK(c.PrevValue),
					formatValueK(c.CurrValue)))
				shown++
			}
			b.WriteString("\n")
		}

		if len(decreases) > 0 {
			hasChanges = true
			b.WriteString("<b>⬇️ Top Decreases:</b>\n")
			shown := 0
			for _, c := range decreases {
				if shown >= 3 {
					break
				}
				b.WriteString(fmt.Sprintf("  ↓ %s — <code>%.0f%%</code> ($%s → $%s)\n",
					html.EscapeString(truncate(c.Issuer, 25)),
					c.PctChange,
					formatValueK(c.PrevValue),
					formatValueK(c.CurrValue)))
				shown++
			}
			b.WriteString("\n")
		}
	}

	if !hasChanges && report.PreviousFiling == nil {
		b.WriteString("<i>QoQ comparison not available (only 1 filing found).</i>\n\n")
	}
}

// formatValueK formats a value in thousands to a human-readable string.
// E.g., 15234567 (thousands) → "$15.2B"; 1234 → "$1.2M"; 500 → "$500K".
func formatValueK(valK float64) string {
	if valK >= 1_000_000 { // billions
		return fmt.Sprintf("%.1fB", valK/1_000_000)
	}
	if valK >= 1_000 { // millions
		return fmt.Sprintf("%.1fM", valK/1_000)
	}
	return fmt.Sprintf("%.0fK", valK)
}

