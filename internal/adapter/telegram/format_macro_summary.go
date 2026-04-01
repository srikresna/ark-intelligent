package telegram

import (
	"fmt"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)


// FormatMacroSummary formats a plain-language executive summary of the macro regime.
// Designed for non-finance users: leads with "so what", follows with "why".
func (f *Formatter) FormatMacroSummary(regime fred.MacroRegime, data *fred.MacroData, implications []fred.TradingImplication) string {
	var b strings.Builder

	riskBar := buildRiskBar(regime.Score, 15)

	b.WriteString("🏦 <b>MACRO SNAPSHOT</b>\n")
	b.WriteString(fmt.Sprintf("<i>Data per %s</i>\n\n", fmtutil.FormatDateTimeWIB(data.FetchedAt)))

	// --- Section 1: Plain-language status ---
	b.WriteString(macroStatusLine("Ekonomi", macroEconomyLabel(regime, data)))
	b.WriteString(macroStatusLine("Inflasi", macroInflationLabel(data)))
	b.WriteString(macroStatusLine("Pasar Kerja", macroLaborLabel(regime, data)))
	b.WriteString(macroStatusLine("Stress", macroStressLabel(data)))
	b.WriteString(macroStatusLine("Resesi", macroRecessionLabel(regime)))
	b.WriteString(fmt.Sprintf("\n<code>Risk: [%s]</code>\n", riskBar))

	// --- Section 2: Trading implications ---
	b.WriteString("\n<b>━━━ APA ARTINYA UNTUK TRADING? ━━━</b>\n")
	for _, imp := range implications {
		b.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n", imp.Icon, imp.Asset, imp.Reason))
	}

	// --- Section 3: Alerts / warnings ---
	alerts := macroAlertLines(regime, data)
	if len(alerts) > 0 {
		b.WriteString("\n⚠️ <b>PERHATIAN:</b>\n")
		for _, alert := range alerts {
			b.WriteString(fmt.Sprintf("<i>%s</i>\n", alert))
		}
	}

	// Cache note
	age := fred.CacheAge()
	cacheNote := "live"
	if age >= 0 {
		cacheNote = fmt.Sprintf("cache %dm", int(age.Minutes()))
	}
	b.WriteString(fmt.Sprintf("\n<i>FRED %s</i>", cacheNote))

	return b.String()
}

// macroStatusLine formats a single status row: "Label : status_text"
func macroStatusLine(label, status string) string {
	// Pad label to 12 chars for alignment
	padded := label + strings.Repeat(" ", 12-len([]rune(label)))
	return fmt.Sprintf("<code>%s: %s</code>\n", padded, status)
}

// macroEconomyLabel produces a plain-language economy description.
func macroEconomyLabel(regime fred.MacroRegime, data *fred.MacroData) string {
	switch regime.Name {
	case "RECESSION":
		return "Resesi — ekonomi menyusut 🔴"
	case "STAGFLATION":
		return "Stagflasi — inflasi tinggi + pertumbuhan lemah 🔴"
	case "STRESS":
		return "Tekanan finansial tinggi 🔴"
	case "INFLATIONARY":
		return "Inflasi tinggi, pertumbuhan masih jalan ⚠️"
	case "GOLDILOCKS":
		if data.GDPGrowth > 2.0 {
			return "Ideal — pertumbuhan kuat, inflasi terjaga ✅"
		}
		return "Cukup baik — stabil ✅"
	case "NEUTRAL":
		if data.CorePCETrend.Direction == "DOWN" {
			return "Inflasi masih tinggi tapi mulai turun ⚠️"
		}
		return "Inflasi di atas target, pertumbuhan campuran ⚠️"
	case "DISINFLATIONARY":
		if data.GDPGrowth > 1.5 {
			return "Melambat tapi masih tumbuh ✅"
		}
		return "Melambat, perlu pantau ⚠️"
	default:
		return "Campuran — sinyal tidak jelas 🟡"
	}
}

// macroInflationLabel produces a plain-language inflation description.
func macroInflationLabel(data *fred.MacroData) string {
	if data.CorePCE <= 0 {
		return "Data tidak tersedia"
	}
	trend := ""
	switch data.CorePCETrend.Direction {
	case "DOWN":
		trend = " dan menurun ↓"
	case "UP":
		trend = " dan naik ↑"
	}

	switch {
	case data.CorePCE < 2.0:
		return fmt.Sprintf("Di bawah target (%.1f%%)%s ✅", data.CorePCE, trend)
	case data.CorePCE < 2.5:
		return fmt.Sprintf("Mendekati target (%.1f%%)%s ✅", data.CorePCE, trend)
	case data.CorePCE < 3.5:
		return fmt.Sprintf("Masih tinggi (%.1f%%)%s ⚠️", data.CorePCE, trend)
	default:
		return fmt.Sprintf("Sangat tinggi (%.1f%%)%s 🔴", data.CorePCE, trend)
	}
}

// macroLaborLabel produces a plain-language labor market description.
func macroLaborLabel(regime fred.MacroRegime, data *fred.MacroData) string {
	if regime.SahmAlert {
		return "Melemah tajam — sinyal resesi! 🔴"
	}
	if data.NFPChange < 0 {
		return "Kehilangan lapangan kerja 🔴"
	}
	if data.InitialClaims > 300000 {
		return fmt.Sprintf("Melemah (klaim: %.0fK) ⚠️", data.InitialClaims/1000)
	}
	if data.InitialClaims > 0 && data.InitialClaims < 250000 {
		trend := ""
		if data.ClaimsTrend.Direction == "UP" {
			trend = ", tapi mulai naik ↑"
		}
		return fmt.Sprintf("Kuat (klaim: %.0fK)%s ✅", data.InitialClaims/1000, trend)
	}
	return "Stabil 🟡"
}

// macroStressLabel produces a plain-language financial stress description.
func macroStressLabel(data *fred.MacroData) string {
	if data.VIX > 30 {
		return fmt.Sprintf("Tinggi — VIX %.0f, pasar takut 🔴", data.VIX)
	}
	if data.NFCI > 0.5 {
		return "Kondisi finansial ketat 🔴"
	}
	if data.NFCI > 0 {
		return "Sedikit tegang ⚠️"
	}
	if data.VIX > 20 {
		return fmt.Sprintf("Waspada — VIX %.0f ⚠️", data.VIX)
	}
	return "Tenang, tidak ada tekanan ✅"
}

// macroRecessionLabel produces a plain-language recession risk label.
func macroRecessionLabel(regime fred.MacroRegime) string {
	switch {
	case regime.SahmAlert:
		return "TINGGI — indikator resesi aktif! 🔴"
	case regime.Score >= 60:
		return "Meningkat ⚠️"
	case regime.Score >= 40:
		return "Sedang 🟡"
	default:
		return "Rendah ✅"
	}
}

// macroAlertLines returns contextual warnings in plain language.
func macroAlertLines(regime fred.MacroRegime, data *fred.MacroData) []string {
	var alerts []string

	if regime.SahmAlert {
		alerts = append(alerts, "Indikator Sahm Rule aktif — secara historis, ini sinyal resesi sudah dimulai. Akurasi 100% sejak 1970.")
	}
	if data.Spread3M10Y < 0 && data.Spread3M10Y != 0 {
		alerts = append(alerts, fmt.Sprintf("Yield curve 3M-10Y terbalik (%.2f%%) — secara historis ini sinyal resesi 6-18 bulan ke depan.", data.Spread3M10Y))
	} else if data.YieldSpread < 0 {
		alerts = append(alerts, fmt.Sprintf("Yield curve 2Y-10Y terbalik (%.2f%%) — sinyal perlambatan ekonomi.", data.YieldSpread))
	}
	if data.VIX > 30 {
		alerts = append(alerts, fmt.Sprintf("VIX di %.0f (>30) — pasar dalam mode ketakutan. Volatilitas tinggi.", data.VIX))
	}
	if data.NFPChange < 0 {
		alerts = append(alerts, "Nonfarm Payrolls negatif — ekonomi AS kehilangan lapangan kerja. Sangat jarang terjadi.")
	}
	if data.WageGrowth > 5 {
		alerts = append(alerts, fmt.Sprintf("Pertumbuhan upah %.1f%% (>5%%) — risiko spiral upah-harga, inflasi bisa bertahan lama.", data.WageGrowth))
	}

	return alerts
}

// FormatMacroExplain formats a plain-language glossary of macro indicators with current values.
func (f *Formatter) FormatMacroExplain(regime fred.MacroRegime, data *fred.MacroData) string {
	var b strings.Builder

	b.WriteString("📖 <b>PANDUAN INDIKATOR MACRO</b>\n")
	b.WriteString("<i>Penjelasan setiap indikator + nilai saat ini</i>\n")

	// Yield Curve
	b.WriteString("\n<b>Yield Curve (Kurva Imbal Hasil)</b>\n")
	b.WriteString("<i>Selisih bunga obligasi jangka pendek vs panjang.")
	b.WriteString(" Jika terbalik (negatif), pasar mengekspektasi resesi.</i>\n")
	b.WriteString(fmt.Sprintf("<code>  2Y-10Y : %s</code>\n", regime.YieldCurve))
	if regime.Yield3M10Y != "N/A" && regime.Yield3M10Y != "" {
		b.WriteString(fmt.Sprintf("<code>  3M-10Y : %s</code>\n", regime.Yield3M10Y))
	}

	// Core PCE
	b.WriteString("\n<b>Core PCE (Inflasi Inti)</b>\n")
	b.WriteString("<i>Ukuran inflasi favorit The Fed, tanpa makanan &amp; energi.")
	b.WriteString(" Target Fed: 2%. Lebih tinggi = Fed ketatkan suku bunga.</i>\n")
	b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.Inflation))

	// Fed Funds Rate
	if data.FedFundsRate > 0 {
		b.WriteString("\n<b>Suku Bunga Fed (Fed Funds Rate)</b>\n")
		b.WriteString("<i>Suku bunga acuan AS. Naik = USD menguat tapi menekan ekonomi.")
		b.WriteString(" Turun = USD melemah, ekonomi didorong.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.MonPolicy))
	}

	// NFCI
	b.WriteString("\n<b>NFCI (Kondisi Finansial)</b>\n")
	b.WriteString("<i>Indeks kondisi keuangan AS dari Chicago Fed.")
	b.WriteString(" Negatif = longgar (bagus). Positif = ketat (tekanan).")
	b.WriteString(" Di atas 0.5 = stress.</i>\n")
	b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.FinStress))

	// VIX
	if data.VIX > 0 {
		var vixStatus string
		switch {
		case data.VIX > 30:
			vixStatus = fmt.Sprintf("Tinggi (%.0f) 🔴 — pasar takut", data.VIX)
		case data.VIX > 20:
			vixStatus = fmt.Sprintf("Waspada (%.0f) ⚠️", data.VIX)
		default:
			vixStatus = fmt.Sprintf("Tenang (%.0f) ✅", data.VIX)
		}
		b.WriteString("\n<b>VIX (Indeks Ketakutan)</b>\n")
		b.WriteString("<i>Mengukur ekspektasi volatilitas pasar saham AS.")
		b.WriteString(" &lt;15 = tenang, 15-20 = normal, 20-30 = waspada, &gt;30 = panik.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", vixStatus))
	}

	// Sahm Rule
	b.WriteString("\n<b>Sahm Rule (Indikator Resesi)</b>\n")
	b.WriteString("<i>Jika naik di atas 0.5, resesi biasanya sudah dimulai.")
	b.WriteString(" Akurasi historis: 100% sejak 1970 (nol sinyal palsu).</i>\n")
	if regime.SahmAlert {
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code> 🚨 AKTIF!\n", regime.SahmLabel))
	} else if regime.SahmLabel != "N/A" && regime.SahmLabel != "" {
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.SahmLabel))
	}

	// Labor
	b.WriteString("\n<b>Pasar Kerja (Initial Claims)</b>\n")
	b.WriteString("<i>Klaim pengangguran mingguan. Makin rendah = makin sehat.")
	b.WriteString(" Di bawah 250K = kuat. Di atas 300K = melemah.</i>\n")
	b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.Labor))

	// GDP
	if regime.Growth != "N/A" && regime.Growth != "" {
		b.WriteString("\n<b>GDP (Pertumbuhan Ekonomi)</b>\n")
		b.WriteString("<i>Perubahan PDB AS per kuartal (tahunan).")
		b.WriteString(" Positif = tumbuh. Negatif 2 kuartal berturut = resesi teknis.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.Growth))
	}

	// Fed Balance Sheet
	if regime.FedBalance != "N/A" && regime.FedBalance != "" {
		b.WriteString("\n<b>Neraca Fed (QE/QT)</b>\n")
		b.WriteString("<i>Total aset Fed. Naik (QE) = cetak uang, likuiditas naik.")
		b.WriteString(" Turun (QT) = likuiditas dikurangi, pasar lebih ketat.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.FedBalance))
	}

	// DXY
	if regime.USDStrength != "N/A" && regime.USDStrength != "" {
		b.WriteString("\n<b>DXY (Kekuatan USD)</b>\n")
		b.WriteString("<i>Indeks dolar AS terhadap mata uang utama.")
		b.WriteString(" Naik = USD menguat. Turun = USD melemah.</i>\n")
		b.WriteString(fmt.Sprintf("<code>  Saat ini: %s</code>\n", regime.USDStrength))
	}

	return b.String()
}

// FormatRegimeLabel formats a COT-based regime result for display.
func (f *Formatter) FormatRegimeLabel(regime string, confidence float64, factors []string) string {
	icon := "⚪"
	switch regime {
	case "RISK-ON":
		icon = "🟢"
	case "RISK-OFF":
		icon = "🔴"
	case "UNCERTAINTY":
		icon = "🟡"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s <b>COT Regime: %s</b> (%.0f%% confidence)\n", icon, regime, confidence))
	if len(factors) > 0 {
		b.WriteString("<i>Signals: ")
		shown := factors
		if len(shown) > 3 {
			shown = factors[:3]
		}
		b.WriteString(strings.Join(shown, " | "))
		b.WriteString("</i>\n")
	}
	return b.String()
}

// FormatRegimePerformance formats the regime-asset performance matrix.
func (f *Formatter) FormatRegimePerformance(matrix *fred.RegimePerformanceMatrix) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x8A <b>REGIME-ASSET PERFORMANCE MATRIX</b>\n")
	b.WriteString("<i>Annualized returns (%) by FRED macro regime</i>\n\n")

	if matrix == nil || len(matrix.Regimes) == 0 {
		b.WriteString("No regime performance data available yet.\n")
		b.WriteString("<i>Data builds as signals accumulate with FRED regime labels.</i>")
		return b.String()
	}

	// For each regime, show a compact table
	for _, regime := range matrix.Regimes {
		returns := matrix.Data[regime]
		if len(returns) == 0 {
			continue
		}

		icon := "\xF0\x9F\x93\x88"
		if regime == "STRESS" || regime == "RECESSION" {
			icon = "\xF0\x9F\x94\xB4"
		} else if regime == "STAGFLATION" {
			icon = "\xF0\x9F\x9F\xA0"
		} else if regime == "GOLDILOCKS" {
			icon = "\xF0\x9F\x9F\xA2"
		}

		currentTag := ""
		if regime == matrix.Current {
			currentTag = " \xe2\x86\x90 CURRENT"
		}

		b.WriteString(fmt.Sprintf("%s <b>%s</b>%s\n<pre>", icon, regime, currentTag))
		b.WriteString(fmt.Sprintf("%-5s %7s %5s %4s\n", "CCY", "Ann.%", "WR%", "N"))
		b.WriteString(strings.Repeat("\xe2\x94\x80", 26) + "\n")

		for _, r := range returns {
			if r.TotalWeeks == 0 {
				continue
			}
			sign := "+"
			if r.AnnualizedReturn < 0 {
				sign = ""
			}
			b.WriteString(fmt.Sprintf("%-5s %s%6.1f %4.0f%% %4d\n",
				r.Currency, sign, r.AnnualizedReturn, r.WinRate, r.TotalWeeks))
		}
		b.WriteString("</pre>\n")
	}

	if matrix.Current != "" {
		b.WriteString(fmt.Sprintf("\nCurrent regime: <b>%s</b>\n", matrix.Current))
	}
	b.WriteString("<i>Ann.% = avg weekly return \xc3\x97 52 | WR% = weeks with positive return</i>")

	return b.String()
}

