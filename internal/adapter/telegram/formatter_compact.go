package telegram

import (
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
)

// FormatCOTOverviewCompact returns a compact COT summary with key signals only.
// Max ~1500 chars, optimized for mobile traders.
func (f *Formatter) FormatCOTOverviewCompact(analyses []domain.COTAnalysis, convictions []cot.ConvictionScore) string {
	var b strings.Builder
	b.WriteString("📊 <b>COT Positioning (Compact)</b>\n")
	if len(analyses) > 0 {
		b.WriteString(fmt.Sprintf("<i>%s</i>\n", analyses[0].ReportDate.Format("Jan 2, 2006")))
	}
	b.WriteString("\n")

	convMap := make(map[string]cot.ConvictionScore, len(convictions))
	for _, c := range convictions {
		convMap[c.Currency] = c
	}

	for _, a := range analyses {
		code := a.Contract.Currency
		if code == "" {
			code = contractCodeToFriendly(a.Contract.Code)
		}

		arrow := "➡ Flat"
		if a.NetChange > 0 {
			arrow = "🟢 Long"
		} else if a.NetChange < 0 {
			arrow = "🔴 Short"
		}

		signal := ""
		if conv, ok := convMap[code]; ok && conv.Direction != "" {
			signal = fmt.Sprintf(" | %s %s", conv.Direction, strengthDot(conv.Score))
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b>: %+.0fk (%+.0fk)%s\n",
			arrow, code, a.NetPosition/1000, a.NetChange/1000, signal))
	}

	b.WriteString("\n💡 <i>Tekan 📖 Detail Lengkap untuk analisis penuh.</i>")
	return truncateMsg(b.String())
}

// FormatMacroSummaryCompact returns a compact macro dashboard.
func (f *Formatter) FormatMacroSummaryCompact(regime fred.MacroRegime, data *fred.MacroData) string {
	var b strings.Builder
	b.WriteString("🏦 <b>Macro Dashboard (Compact)</b>\n\n")

	// Regime
	b.WriteString(fmt.Sprintf("📌 Regime: <b>%s</b>\n", regime.Name))
	if regime.MonPolicy != "" {
		b.WriteString(fmt.Sprintf("🎯 Policy: %s\n", regime.MonPolicy))
	}

	// Key numbers
	b.WriteString("\n📈 <b>Key Numbers:</b>\n")
	if data != nil {
		if v := data.Yield10Y; v != 0 {
			b.WriteString(fmt.Sprintf("• 10Y Yield: %.2f%%\n", v))
		}
		if v := data.Yield2Y; v != 0 {
			b.WriteString(fmt.Sprintf("• 2Y Yield: %.2f%%\n", v))
		}
		if v := data.YieldSpread; v != 0 {
			b.WriteString(fmt.Sprintf("• 2Y-10Y Spread: %+.0f bps\n", v*100))
		}
		if v := data.CPI; v != 0 {
			b.WriteString(fmt.Sprintf("• CPI YoY: %.1f%%\n", v))
		}
		if v := data.CorePCE; v != 0 {
			b.WriteString(fmt.Sprintf("• Core PCE: %.1f%%\n", v))
		}
		if v := data.UnemployRate; v != 0 {
			b.WriteString(fmt.Sprintf("• Unemployment: %.1f%%\n", v))
		}
	}

	b.WriteString("\n💡 <i>Tekan 📖 Detail Lengkap untuk breakdown lengkap.</i>")
	return truncateMsg(b.String())
}

// strengthDot returns colored dots based on conviction score.
func strengthDot(score float64) string {
	if score >= 0.7 {
		return "●●●"
	} else if score >= 0.4 {
		return "●●○"
	}
	return "●○○"
}

// FormatCOTOverviewSparkline renders one compact line per currency in mobile-friendly format.
// Each line shows: emoji arrow, currency, net position k, WoW change, a COTIndex percentile bar,
// and conviction signal. Output stays under ~80 chars wide — readable on 320px screens.
func (f *Formatter) FormatCOTOverviewSparkline(analyses []domain.COTAnalysis, convictions []cot.ConvictionScore) string {
	var b strings.Builder
	b.WriteString("📱 <b>COT Mobile View</b>\n")
	if len(analyses) > 0 {
		b.WriteString(fmt.Sprintf("<i>%s</i>\n", analyses[0].ReportDate.Format("02 Jan 2006")))
	}
	b.WriteString("\n")

	convMap := make(map[string]cot.ConvictionScore, len(convictions))
	for _, c := range convictions {
		convMap[c.Currency] = c
	}

	for _, a := range analyses {
		code := a.Contract.Currency
		if code == "" {
			code = contractCodeToFriendly(a.Contract.Code)
		}

		// Emoji direction indicator
		arrow := "➡ Flat"
		if a.NetChange > 0 {
			arrow = "🟢 Long"
		} else if a.NetChange < 0 {
			arrow = "🔴 Short"
		}

		// Percentile bar from COTIndex (0-100) → 5-block sparkline
		pctBar := cotIndexBar(a.COTIndex)

		// Conviction label
		signal := ""
		if conv, ok := convMap[code]; ok && conv.Direction != "" {
			signal = fmt.Sprintf(" %s", conv.Direction)
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b>: %+.0fk (%+.0fk) <code>%s</code>%s\n",
			arrow, code, a.NetPosition/1000, a.NetChange/1000, pctBar, signal))
	}

	b.WriteString("\n💡 <i>Tekan 📖 Detail untuk analisis penuh.</i>")
	return truncateMsg(b.String())
}

// cotIndexBar converts a COTIndex (0-100) into a 5-block progress bar using Unicode block chars.
// Example: index=75 → "████░"
func cotIndexBar(index float64) string {
	const total = 5
	if index < 0 {
		index = 0
	}
	if index > 100 {
		index = 100
	}
	filled := int(index / 100 * total)
	bar := make([]rune, total)
	for i := 0; i < total; i++ {
		if i < filled {
			bar[i] = '█'
		} else {
			bar[i] = '░'
		}
	}
	return string(bar)
}
