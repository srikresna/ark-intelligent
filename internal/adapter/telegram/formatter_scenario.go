package telegram

import (
	"fmt"
	"math"
	"strings"

	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// formatScenarioResult renders a Monte Carlo scenario result as Telegram HTML.
func formatScenarioResult(r *pricesvc.ScenarioResult, currency string) string {
	var sb strings.Builder

	sb.WriteString("📊 <b>MONTE CARLO SCENARIO</b>\n")
	sb.WriteString(fmt.Sprintf("%s | %d-day horizon | %d paths\n\n", r.Symbol, r.HorizonDays, r.NumPaths))

	// Current price + regime
	sb.WriteString(fmt.Sprintf("💰 Current: <code>%s</code>\n", formatScenarioPrice(r.CurrentPrice, currency)))
	sb.WriteString(fmt.Sprintf("🔄 Regime: <b>%s</b> | Vol: %.1f%% ann.\n\n", r.Regime, r.VolEstimate*100))

	// Price distribution table
	sb.WriteString("📈 <b>Price Distribution</b>\n")
	sb.WriteString("<pre>")
	sb.WriteString("Pctl   Price       Return\n")
	sb.WriteString("─────────────────────────\n")

	for _, p := range r.Percentiles {
		label := fmt.Sprintf("P%02.0f", p.Percentile*100)
		arrow := returnArrow(p.Return)
		sb.WriteString(fmt.Sprintf("%-4s   %-10s  %s%+.2f%%\n",
			label,
			formatScenarioPrice(p.Price, currency),
			arrow,
			p.Return,
		))
	}
	sb.WriteString("</pre>\n\n")

	// Risk metrics
	sb.WriteString("⚠️ <b>Risk Metrics</b>\n")
	sb.WriteString(fmt.Sprintf("• VaR 95%%: <code>%+.2f%%</code> (5%% chance of worse loss)\n", r.VaR95))
	sb.WriteString(fmt.Sprintf("• VaR 99%%: <code>%+.2f%%</code> (1%% chance of worse loss)\n", r.VaR99))
	sb.WriteString(fmt.Sprintf("• CVaR 95%%: <code>%+.2f%%</code> (expected loss in worst 5%%)\n", r.CVaR95))
	sb.WriteString(fmt.Sprintf("• Mean return: <code>%+.2f%%</code>\n\n", r.MeanReturn))

	// Interpretation
	sb.WriteString("💡 <b>Interpretation</b>\n")
	sb.WriteString(scenarioInterpretation(r))

	return truncateMsg(sb.String())
}

// formatScenarioPrice formats a price based on currency conventions.
func formatScenarioPrice(price float64, currency string) string {
	switch strings.ToUpper(currency) {
	case "JPY":
		return fmt.Sprintf("%.3f", price)
	case "XAU", "GOLD":
		return fmt.Sprintf("%.2f", price)
	case "BTC":
		return fmt.Sprintf("%.0f", price)
	case "OIL", "WTI":
		return fmt.Sprintf("%.2f", price)
	default:
		return fmt.Sprintf("%.5f", price)
	}
}

// returnArrow returns a visual indicator for the return direction.
func returnArrow(ret float64) string {
	switch {
	case ret > 2:
		return "🟢 "
	case ret > 0:
		return "🔹 "
	case ret > -2:
		return "🔸 "
	default:
		return "🔴 "
	}
}

// scenarioInterpretation provides a human-readable summary.
func scenarioInterpretation(r *pricesvc.ScenarioResult) string {
	var lines []string

	// P50 direction
	var medianReturn float64
	for _, p := range r.Percentiles {
		if p.Percentile == 0.50 {
			medianReturn = p.Return
			break
		}
	}

	if medianReturn > 0.5 {
		lines = append(lines, fmt.Sprintf("Median outcome: <b>+%.1f%%</b> — slight upside bias", medianReturn))
	} else if medianReturn < -0.5 {
		lines = append(lines, fmt.Sprintf("Median outcome: <b>%.1f%%</b> — slight downside bias", medianReturn))
	} else {
		lines = append(lines, "Median outcome: roughly <b>flat</b> over this horizon")
	}

	// Tail risk
	if math.Abs(r.VaR95) > 5 {
		lines = append(lines, fmt.Sprintf("⚠️ Significant tail risk: 5%% chance of >%.1f%% drawdown", math.Abs(r.VaR95)))
	}

	// Regime context
	switch r.Regime {
	case pricesvc.HMMRiskOn:
		lines = append(lines, "🟢 Risk-On regime: historically favours trend continuation")
	case pricesvc.HMMRiskOff:
		lines = append(lines, "🟡 Risk-Off regime: expect mean-reversion, range-bound action")
	case pricesvc.HMMCrisis:
		lines = append(lines, "🔴 Crisis regime: elevated vol, wider distribution tails")
	}

	return strings.Join(lines, "\n")
}
