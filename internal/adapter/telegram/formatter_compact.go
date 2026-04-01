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

		arrow := "➡"
		if a.NetChange > 0 {
			arrow = "🟢"
		} else if a.NetChange < 0 {
			arrow = "🔴"
		}

		signal := ""
		if conv, ok := convMap[code]; ok && conv.Direction != "" {
			signal = fmt.Sprintf(" | %s %s", conv.Direction, strengthDot(conv.Score))
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b>: %+.0fk (%+.0fk)%s\n",
			arrow, code, a.NetPosition/1000, a.NetChange/1000, signal))
	}

	b.WriteString("\n💡 <i>Tekan 📖 Detail Lengkap untuk analisis penuh.</i>")
	return b.String()
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
	return b.String()
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
