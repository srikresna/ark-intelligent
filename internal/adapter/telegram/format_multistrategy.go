package telegram

import (
	"fmt"
	"math"
	"sort"
	"strings"

	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
)

// FormatMultiStrategy formats the multi-strategy composer output.
func (f *Formatter) FormatMultiStrategy(result *backtestsvc.MultiStrategyResult) string {
	var b strings.Builder

	b.WriteString("📊 <b>MULTI-STRATEGY BACKTESTER</b>\n\n")

	if len(result.Strategies) == 0 {
		b.WriteString("<i>Not enough signal data for multi-strategy analysis.\nRun more signals and re-evaluate outcomes first.</i>")
		return b.String()
	}

	// Per-strategy table.
	b.WriteString("<code>━━━ Per-Strategy Performance ━━━</code>\n\n")
	b.WriteString(fmt.Sprintf("<code>%-12s %5s %5s %6s %7s %5s</code>\n",
		"Strategy", "N", "WR%", "Sharpe", "MaxDD%", "PF"))
	b.WriteString("<code>────────────────────────────────────</code>\n")

	for _, s := range result.Strategies {
		sharpeStr := fmt.Sprintf("%.2f", s.Sharpe)
		ddStr := fmt.Sprintf("%.1f", s.MaxDrawdown)
		pfStr := fmt.Sprintf("%.2f", s.ProfitFactor)
		wrStr := fmt.Sprintf("%.1f", s.WinRate)

		best := s.Name == result.BestStrategy
		prefix := " "
		if best {
			prefix = "★"
		}
		b.WriteString(fmt.Sprintf("<code>%s%-11s %5d %5s %6s %7s %5s</code>\n",
			prefix, truncate(s.Name, 11), s.SignalCount, wrStr, sharpeStr, ddStr, pfStr))
	}
	b.WriteString("\n")

	// Best/worst.
	b.WriteString(fmt.Sprintf("⭐ Best:  <b>%s</b> (Sharpe <code>%.2f</code>)\n",
		result.BestStrategy, result.BestSharpe))
	b.WriteString(fmt.Sprintf("⚠️ Worst: <b>%s</b> (Sharpe <code>%.2f</code>)\n\n",
		result.WorstStrategy, result.WorstSharpe))

	// Correlation matrix (top pairs only, skip trivial/zero).
	if len(result.Correlations) > 0 {
		b.WriteString("<code>━━━ Inter-Strategy Correlations ━━━</code>\n\n")
		shown := 0
		for _, c := range result.Correlations {
			if math.Abs(c.Corr) < 0.1 {
				continue
			}
			emoji := corrEmoji(c.Corr)
			b.WriteString(fmt.Sprintf("%s <code>%-11s × %-11s</code> <code>%+.2f</code>\n",
				emoji, truncate(c.StratA, 11), truncate(c.StratB, 11), c.Corr))
			shown++
			if shown >= 6 {
				break
			}
		}
		b.WriteString("\n")
	}

	// Portfolio compositions.
	if len(result.Portfolios) > 0 {
		b.WriteString("<code>━━━ Portfolio Compositions ━━━</code>\n\n")

		// Find best portfolio by Sharpe.
		bestIdx := 0
		for i, p := range result.Portfolios {
			if p.CombinedSharpe > result.Portfolios[bestIdx].CombinedSharpe {
				bestIdx = i
			}
		}

		for i, p := range result.Portfolios {
			marker := ""
			if i == bestIdx {
				marker = " ★"
			}
			b.WriteString(fmt.Sprintf("<b>%s</b>%s\n", p.Name, marker))
			b.WriteString(fmt.Sprintf("  Sharpe: <code>%.2f</code>  MaxDD: <code>%.1f%%</code>  DivRatio: <code>%.2f</code>\n",
				p.CombinedSharpe, p.CombinedMaxDD, p.DiversificationRatio))

			// Top 3 weights.
			type wEntry struct {
				name   string
				weight float64
			}
			var entries []wEntry
			for k, v := range p.Weights {
				entries = append(entries, wEntry{k, v})
			}
			sort.Slice(entries, func(x, y int) bool {
				return entries[x].weight > entries[y].weight
			})
			parts := make([]string, 0, 3)
			for j, e := range entries {
				if j >= 3 {
					break
				}
				parts = append(parts, fmt.Sprintf("%s:%.0f%%", truncate(e.name, 8), e.weight*100))
			}
			b.WriteString(fmt.Sprintf("  Weights: <code>%s</code>\n\n", strings.Join(parts, " | ")))
		}
	}

	// Legend.
	b.WriteString("<code>━━━ Legend ━━━</code>\n")
	b.WriteString("WR% = 1W Win Rate | PF = Profit Factor\n")
	b.WriteString("DivRatio > 1 = diversification benefit\n")
	b.WriteString("★ = best by Sharpe ratio\n")

	return b.String()
}

// corrEmoji returns a correlation strength indicator emoji.
func corrEmoji(c float64) string {
	switch {
	case c >= 0.7:
		return "🔴" // high positive correlation
	case c >= 0.3:
		return "🟡" // moderate positive
	case c <= -0.7:
		return "🟢" // high negative (diversifying)
	case c <= -0.3:
		return "🟢" // moderate negative
	default:
		return "⚪" // low correlation
	}
}

