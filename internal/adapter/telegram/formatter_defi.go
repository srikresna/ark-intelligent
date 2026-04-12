package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/defi"
)

// fmtLargeUSD formats a large dollar amount with B/M/K suffixes.
func fmtLargeUSD(v float64) string {
	switch {
	case v >= 1e12:
		return fmt.Sprintf("%.2fT", v/1e12)
	case v >= 1e9:
		return fmt.Sprintf("%.2fB", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.0fK", v/1e3)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

// formatDeFiReport formats the DeFi dashboard for Telegram.
func formatDeFiReport(report *defi.DeFiReport) string {
	if report == nil || !report.Available {
		return "⚠️ <b>DeFi data tidak tersedia</b>\nCoba lagi nanti."
	}

	var b strings.Builder

	// Header
	b.WriteString("🌐 <b>DeFi Health Dashboard</b>\n")
	b.WriteString(fmt.Sprintf("<i>Updated: %s</i>\n\n", report.FetchedAt.Format("02 Jan 15:04 UTC")))

	// Signals first (most important)
	if len(report.Signals) > 0 {
		b.WriteString("📡 <b>Signals</b>\n")
		for _, s := range report.Signals {
			b.WriteString(fmt.Sprintf("  %s\n", s.Message))
		}
		b.WriteString("\n")
	}

	// TVL Overview
	if report.TotalTVL > 0 {
		b.WriteString("🏦 <b>Total Value Locked</b>\n")
		b.WriteString(fmt.Sprintf("  TVL: <b>$%s</b>", fmtLargeUSD(report.TotalTVL)))
		if report.TVLChange24h != 0 {
			arrow := "📈"
			if report.TVLChange24h < 0 {
				arrow = "📉"
			}
			b.WriteString(fmt.Sprintf("  %s %.1f%%", arrow, report.TVLChange24h))
		}
		b.WriteString("\n\n")
	}

	// Top Protocols by TVL
	if len(report.TopProtocols) > 0 {
		b.WriteString("🏆 <b>Top Protocols by TVL</b>\n")
		for i, p := range report.TopProtocols {
			change := ""
			if p.Change1D != 0 {
				sign := "+"
				if p.Change1D < 0 {
					sign = ""
				}
				change = fmt.Sprintf(" (%s%.1f%%)", sign, p.Change1D)
			}
			b.WriteString(fmt.Sprintf("  %d. %s — $%s%s\n",
				i+1, p.Name,
				fmtLargeUSD(p.TVL),
				change))
		}
		b.WriteString("\n")
	}

	// Top Chains by TVL
	if len(report.TopChains) > 0 {
		b.WriteString("⛓ <b>TVL by Chain</b>\n")
		for _, c := range report.TopChains {
			b.WriteString(fmt.Sprintf("  • %s: $%s\n", c.Name, fmtLargeUSD(c.TVL)))
		}
		b.WriteString("\n")
	}

	// DEX Volume
	if report.DEX.TotalVolume24h > 0 {
		b.WriteString("📊 <b>DEX Volume (24h)</b>\n")
		b.WriteString(fmt.Sprintf("  Total: <b>$%s</b>", fmtLargeUSD(report.DEX.TotalVolume24h)))
		if report.DEX.Change24h != 0 {
			sign := "+"
			if report.DEX.Change24h < 0 {
				sign = ""
			}
			b.WriteString(fmt.Sprintf(" (%s%.0f%%)", sign, report.DEX.Change24h))
		}
		b.WriteString("\n")
		for _, d := range report.DEX.TopProtocols {
			b.WriteString(fmt.Sprintf("  • %s: $%s\n", d.Name, fmtLargeUSD(d.Volume24h)))
		}
		b.WriteString("\n")
	}

	// Stablecoins
	if len(report.Stablecoins) > 0 {
		b.WriteString("💵 <b>Stablecoin Supply</b>\n")
		b.WriteString(fmt.Sprintf("  Total: <b>$%s</b>\n", fmtLargeUSD(report.TotalStablecoinSupply)))
		for _, s := range report.Stablecoins {
			b.WriteString(fmt.Sprintf("  • %s: $%s\n", s.Symbol, fmtLargeUSD(s.TotalSupply)))
		}
		b.WriteString("\n")
	}

	return truncateMsg(b.String())
}
