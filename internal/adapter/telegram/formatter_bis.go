package telegram

// formatter_bis.go — Telegram HTML formatter for the /bis command.
// Displays BIS central bank policy rates and credit-to-GDP gaps.

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/bis"
)

// formatBISDashboard builds the HTML message for /bis command output.
func formatBISDashboard(
	reer *bis.BISData,
	policy *bis.PolicyRateSuite,
	creditGap *bis.CreditGapReport,
) string {
	var sb strings.Builder

	sb.WriteString("🏦 <b>BIS Statistics Dashboard</b>\n")
	sb.WriteString("Bank for International Settlements\n\n")

	// Section 1: Central Bank Policy Rates.
	if policy != nil && anyPolicyAvailable(policy.Rates) {
		sb.WriteString("📌 <b>Central Bank Policy Rates</b>\n")
		sb.WriteString(fmt.Sprintf("📅 %s\n\n", policy.FetchedAt.UTC().Format("2006-01-02")))

		for _, r := range policy.Rates {
			if !r.Available {
				continue
			}
			trendIcon := "➖"
			switch r.Trend {
			case "HIKING":
				trendIcon = "📈"
			case "CUTTING":
				trendIcon = "📉"
			}

			changeStr := ""
			if r.Change != 0 {
				sign := "+"
				if r.Change < 0 {
					sign = ""
				}
				changeStr = fmt.Sprintf(" (%s%.2f%%)", sign, r.Change)
			}

			sb.WriteString(fmt.Sprintf("  %s <b>%s</b> (%s): %.2f%%%s %s\n",
				cbFlagEmoji(r.Currency), r.CB, r.Currency, r.Rate, changeStr, trendIcon))
		}
		sb.WriteString("\n")
	}

	// Section 2: Credit-to-GDP Gaps.
	if creditGap != nil && anyCreditGapAvailable(creditGap.Gaps) {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("⚠️ <b>Credit-to-GDP Gap</b>\n")
		sb.WriteString(fmt.Sprintf("📅 %s\n", creditGap.FetchedAt.UTC().Format("2006-01-02")))
		sb.WriteString("<i>Early warning: &gt;2% = elevated risk, &gt;10% = high stress</i>\n\n")

		for _, g := range creditGap.Gaps {
			if !g.Available {
				continue
			}
			sigIcon := "✅"
			switch g.Signal {
			case "WARNING":
				sigIcon = "⚠️"
			case "ALERT":
				sigIcon = "🚨"
			}

			sign := "+"
			if g.Gap < 0 {
				sign = ""
			}
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b>: %s%.1f%%\n",
				sigIcon, g.Country, sign, g.Gap))
		}
		sb.WriteString("\n")
	}

	// Section 3: REER overview (condensed).
	if reer != nil && len(reer.Currencies) > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString("💱 <b>Real Effective Exchange Rates</b>\n")
		sb.WriteString(fmt.Sprintf("📅 %s\n\n", reer.FetchedAt.UTC().Format("2006-01-02")))

		for _, c := range reer.Currencies {
			if !c.Available {
				continue
			}
			sigStr := c.Signal
			sigIcon := "➖"
			switch c.Signal {
			case "OVERVALUED":
				sigIcon = "🔴"
			case "UNDERVALUED":
				sigIcon = "🟢"
			case "FAIR":
				sigIcon = "⚪"
				sigStr = "Fair"
			}

			sign := "+"
			if c.Deviation < 0 {
				sign = ""
			}
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b>: %.1f (%s%.1f%% vs LT avg) — %s\n",
				sigIcon, c.Currency, c.REER, sign, c.Deviation, sigStr))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("📊 <i>Data: BIS Statistics API (free, no key required)\nCache: 24h • Update frequency: monthly/quarterly</i>")

	return sb.String()
}

// cbFlagEmoji returns a flag emoji for the currency's central bank.
func cbFlagEmoji(currency string) string {
	flags := map[string]string{
		"USD": "🇺🇸",
		"EUR": "🇪🇺",
		"GBP": "🇬🇧",
		"JPY": "🇯🇵",
		"CHF": "🇨🇭",
		"AUD": "🇦🇺",
		"CAD": "🇨🇦",
		"NZD": "🇳🇿",
	}
	if f, ok := flags[currency]; ok {
		return f
	}
	return "🏦"
}

// anyPolicyAvailable checks if any policy rate has data.
func anyPolicyAvailable(rates []bis.PolicyRate) bool {
	for _, r := range rates {
		if r.Available {
			return true
		}
	}
	return false
}

// anyCreditGapAvailable checks if any credit gap entry has data.
func anyCreditGapAvailable(gaps []bis.CreditGap) bool {
	for _, g := range gaps {
		if g.Available {
			return true
		}
	}
	return false
}
