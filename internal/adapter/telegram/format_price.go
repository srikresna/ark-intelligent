package telegram

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// momentumLabel converts MomentumDirection to readable label.
func (f *Formatter) momentumLabel(m domain.MomentumDirection) string {
	switch m {
	case "STRONG_UP":
		return "Strong Bullish"
	case "UP":
		return "Bullish"
	case "FLAT":
		return "Neutral"
	case "DOWN":
		return "Bearish"
	case "STRONG_DOWN":
		return "Strong Bearish"
	default:
		return string(m)
	}
}

// FormatPriceContext formats price context for a single contract.
// Uses plain language so non-finance users can understand each metric.
func (f *Formatter) FormatPriceContext(pc *domain.PriceContext) string {
	if pc == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n💰 <b>KONDISI HARGA SAAT INI</b>\n")
	b.WriteString(fmt.Sprintf("<code>Harga       : %.5f</code>\n", pc.CurrentPrice))

	// Weekly change with plain explanation
	wIcon := "🟢"
	wDesc := "naik minggu ini"
	if pc.WeeklyChgPct < 0 {
		wIcon = "🔴"
		wDesc = "turun minggu ini"
	} else if pc.WeeklyChgPct == 0 {
		wIcon = "⚪"
		wDesc = "flat minggu ini"
	}
	b.WriteString(fmt.Sprintf("<code>Perubahan 1W: </code>%s <b>%+.2f%%</b> <i>(%s)</i>\n", wIcon, pc.WeeklyChgPct, wDesc))

	// Monthly change
	mIcon := "🟢"
	mDesc := "naik sebulan terakhir"
	if pc.MonthlyChgPct < 0 {
		mIcon = "🔴"
		mDesc = "turun sebulan terakhir"
	} else if pc.MonthlyChgPct == 0 {
		mIcon = "⚪"
		mDesc = "flat sebulan terakhir"
	}
	b.WriteString(fmt.Sprintf("<code>Perubahan 1M: </code>%s <b>%+.2f%%</b> <i>(%s)</i>\n", mIcon, pc.MonthlyChgPct, mDesc))

	// 4-week trend with plain explanation
	trendIcon := "➡️"
	trendDesc := "bergerak sideways (tidak ada arah jelas)"
	if pc.Trend4W == "UP" {
		trendIcon = "⬆️"
		trendDesc = "tren 4 minggu ke ATAS"
	} else if pc.Trend4W == "DOWN" {
		trendIcon = "⬇️"
		trendDesc = "tren 4 minggu ke BAWAH"
	}
	b.WriteString(fmt.Sprintf("<code>Tren 4 Minggu:</code> %s <i>%s</i>\n", trendIcon, trendDesc))

	// MA explanation — simplified
	b.WriteString("\n<b>Posisi vs Rata-rata Harga:</b>\n")

	ma4wPos := "di BAWAH"
	ma4wIcon := "🔴"
	ma4wMeaning := "bearish jangka pendek"
	if pc.AboveMA4W {
		ma4wPos = "di ATAS"
		ma4wIcon = "🟢"
		ma4wMeaning = "bullish jangka pendek"
	}
	b.WriteString(fmt.Sprintf("<code>  Rata2 4-minggu : </code>%s %s (%.5f) — <i>%s</i>\n",
		ma4wIcon, ma4wPos, pc.PriceMA4W, ma4wMeaning))

	ma13wPos := "di BAWAH"
	ma13wIcon := "🔴"
	ma13wMeaning := "tren besar masih turun"
	if pc.AboveMA13W {
		ma13wPos = "di ATAS"
		ma13wIcon = "🟢"
		ma13wMeaning = "tren besar masih naik"
	}
	b.WriteString(fmt.Sprintf("<code>  Rata2 13-minggu: </code>%s %s (%.5f) — <i>%s</i>\n",
		ma13wIcon, ma13wPos, pc.PriceMA13W, ma13wMeaning))

	// MA alignment summary
	if pc.AboveMA4W && pc.AboveMA13W {
		b.WriteString("<i>  → Harga di atas kedua rata-rata = sinyal naik kuat</i>\n")
	} else if !pc.AboveMA4W && !pc.AboveMA13W {
		b.WriteString("<i>  → Harga di bawah kedua rata-rata = sinyal turun kuat</i>\n")
	} else if pc.AboveMA4W && !pc.AboveMA13W {
		b.WriteString("<i>  → Baru mulai rebound, tapi tren besar masih bearish</i>\n")
	} else {
		b.WriteString("<i>  → Mulai melemah dari tren naik, perlu waspada</i>\n")
	}

	// Volatility with plain explanation
	if pc.VolatilityRegime != "" {
		volIcon := "🟡"
		volDesc := "volatilitas normal — pergerakan harga wajar"
		switch pc.VolatilityRegime {
		case "EXPANDING":
			volIcon = "🔴"
			volDesc = "volatilitas TINGGI — harga sedang bergerak liar, risiko lebih besar"
		case "CONTRACTING":
			volIcon = "🟢"
			volDesc = "volatilitas RENDAH — harga sedang tenang, breakout mungkin segera terjadi"
		}
		b.WriteString(fmt.Sprintf("\n<code>Volatilitas: </code>%s <i>%s</i>\n", volIcon, volDesc))
		b.WriteString(fmt.Sprintf("<code>  ATR: %.5f (%.2f%% dari harga)</code>\n", pc.ATR, pc.NormalizedATR))
	}

	return b.String()
}

// FormatSeasonalPatterns formats seasonal analysis results as a compact HTML table.
// Enhanced: shows confidence bar for current month and regime context header.
func (f *Formatter) FormatSeasonalPatterns(patterns []pricesvc.SeasonalPattern) string {
	var b strings.Builder

	b.WriteString("\xF0\x9F\x93\x85 <b>ADVANCED SEASONAL ANALYSIS</b>\n")
	b.WriteString("<i>Statistical monthly bias (up to 5 years, min n\xe2\x89\xa53)</i>\n")

	// Show regime context if available
	if len(patterns) > 0 && patterns[0].RegimeStats != nil {
		b.WriteString(fmt.Sprintf("<i>Regime: %s</i>\n", patterns[0].RegimeStats.RegimeName))
	}
	b.WriteString("\n")

	// Compact grid: currency + 12 months + confluence
	b.WriteString("<pre>")
	shortMonths := [12]string{"J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"}
	// Determine current month for header alignment (bracket current month to match data rows)
	curMonth := 0
	if len(patterns) > 0 {
		curMonth = patterns[0].CurrentMonth
	}
	b.WriteString(fmt.Sprintf("%-6s", "CCY"))
	for i, m := range shortMonths {
		if i+1 == curMonth {
			b.WriteString(fmt.Sprintf("[%s]", m))
		} else {
			b.WriteString(fmt.Sprintf(" %s", m))
		}
	}
	b.WriteString(" Cf\n")
	b.WriteString(strings.Repeat("\xe2\x94\x80", 35) + "\n")

	for _, p := range patterns {
		b.WriteString(fmt.Sprintf("%-6s", p.Currency))
		for i := 0; i < 12; i++ {
			icon := "\xc2\xb7"
			switch p.Monthly[i].Bias {
			case "BULLISH":
				icon = "\xe2\x96\xb2"
			case "BEARISH":
				icon = "\xe2\x96\xbc"
			}
			if i+1 == p.CurrentMonth {
				b.WriteString(fmt.Sprintf("[%s]", icon))
			} else {
				b.WriteString(fmt.Sprintf(" %s", icon))
			}
		}
		// Confluence score for current month
		if p.Confluence != nil {
			b.WriteString(fmt.Sprintf(" %d/%d", p.Confluence.Score, p.Confluence.MaxScore))
		} else {
			b.WriteString("  -")
		}
		b.WriteString("\n")
	}
	b.WriteString("</pre>\n")

	b.WriteString("<i>\xe2\x96\xb2=Bullish \xe2\x96\xbc=Bearish \xc2\xb7=Neutral [x]=now Cf=confluence</i>\n")

	// Strongest tendencies with confidence
	type tendency struct {
		currency   string
		month      string
		avgRet     float64
		winRate    float64
		bias       string
		confidence pricesvc.ConfidenceTier
	}
	var strong []tendency
	for _, p := range patterns {
		for i := 0; i < 12; i++ {
			ms := p.Monthly[i]
			if ms.Bias != "NEUTRAL" && ms.SampleSize >= 3 {
				strong = append(strong, tendency{
					currency: p.Currency, month: ms.Month,
					avgRet: ms.AvgReturn, winRate: ms.WinRate,
					bias: ms.Bias, confidence: ms.Confidence,
				})
			}
		}
	}

	sort.Slice(strong, func(i, j int) bool {
		return math.Abs(strong[i].avgRet) > math.Abs(strong[j].avgRet)
	})

	if len(strong) > 0 {
		b.WriteString("\n\xF0\x9F\x94\xA5 <b>Strongest Tendencies:</b>\n")
		limit := 5
		if len(strong) < limit {
			limit = len(strong)
		}
		for _, t := range strong[:limit] {
			icon := "\xF0\x9F\x9F\xA2"
			if t.bias == "BEARISH" {
				icon = "\xF0\x9F\x94\xB4"
			}
			confTag := ""
			if t.confidence == pricesvc.ConfidenceStrong {
				confTag = " \xe2\x9c\xa8"
			}
			b.WriteString(fmt.Sprintf("%s %s %s: %+.2f%% (%.0f%% WR)%s\n",
				icon, t.currency, t.month, t.avgRet, t.winRate, confTag))
		}
		b.WriteString("<i>\xe2\x9c\xa8 = STRONG confidence</i>\n")
	}

	b.WriteString("\n<i>Use <code>/seasonal CCY</code> for deep dive</i>")

	return b.String()
}

// FormatSeasonalSingle formats the advanced seasonal deep-dive for a single contract.
func (f *Formatter) FormatSeasonalSingle(p pricesvc.SeasonalPattern) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x85 <b>%s \xe2\x80\x94 SEASONAL DEEP DIVE</b>\n", html.EscapeString(p.Currency)))
	b.WriteString("<i>Up to 5 years, regime-aware, multi-factor</i>\n\n")

	// --- STATISTICAL SUMMARY TABLE ---
	b.WriteString("<b>MONTHLY STATISTICS</b>\n<pre>")
	b.WriteString(fmt.Sprintf("%-4s %6s %6s %5s %4s %3s %s\n", "Mon", "Avg", "Med", "WR", "Wgt", "N", ""))
	b.WriteString(strings.Repeat("\xe2\x94\x80", 38) + "\n")

	for i := 0; i < 12; i++ {
		ms := p.Monthly[i]
		marker := " "
		if i+1 == p.CurrentMonth {
			marker = "\xe2\x96\xb6"
		}

		biasIcon := "\xc2\xb7"
		switch ms.Bias {
		case "BULLISH":
			biasIcon = "\xe2\x96\xb2"
		case "BEARISH":
			biasIcon = "\xe2\x96\xbc"
		}

		wrStr := fmt.Sprintf("%.0f%%", ms.WinRate)
		wgtStr := fmt.Sprintf("%.0f%%", ms.WeightedWR)
		if ms.SampleSize == 0 {
			wrStr = "  -"
			wgtStr = "  -"
		}

		b.WriteString(fmt.Sprintf("%s%-3s %+5.1f%% %+5.1f%% %4s %4s %2d %s\n",
			marker, ms.Month, ms.AvgReturn, ms.MedianRet, wrStr, wgtStr, ms.SampleSize, biasIcon))
	}
	b.WriteString("</pre>\n")

	// Guard: CurrentMonth may be 0 for new contracts with no history
	if p.CurrentMonth < 1 || p.CurrentMonth > 12 {
		b.WriteString("<i>Insufficient history for seasonal analysis.</i>\n")
		return b.String()
	}

	curMs := p.Monthly[p.CurrentMonth-1]

	// --- CURRENT MONTH SUMMARY ---
	biasEmoji := "\xe2\x9a\xaa"
	switch p.CurrentBias {
	case "BULLISH":
		biasEmoji = "\xF0\x9F\x9F\xA2"
	case "BEARISH":
		biasEmoji = "\xF0\x9F\x94\xB4"
	}
	b.WriteString(fmt.Sprintf("\n%s <b>%s (%s):</b> %+.2f%% avg, %.0f%% WR",
		biasEmoji, curMs.Month, p.CurrentBias, curMs.AvgReturn, curMs.WinRate))
	if curMs.StdDev > 0 {
		b.WriteString(fmt.Sprintf(", \xcf\x83=%.1f%%", curMs.StdDev))
	}
	b.WriteString(fmt.Sprintf(" (n=%d)\n", curMs.SampleSize))
	if curMs.WeightedAvg != 0 {
		b.WriteString(fmt.Sprintf("<i>Recency-weighted: %+.2f%% avg, %.0f%% WR</i>\n", curMs.WeightedAvg, curMs.WeightedWR))
	}

	// Year-by-year returns
	if len(curMs.YearReturns) > 0 {
		b.WriteString("<pre>")
		for _, yr := range curMs.YearReturns {
			icon := "\xe2\x96\xb2"
			if yr.Return < 0 {
				icon = "\xe2\x96\xbc"
			}
			b.WriteString(fmt.Sprintf("  %d: %+6.2f%% %s\n", yr.Year, yr.Return, icon))
		}
		b.WriteString("</pre>\n")
	}

	// --- REGIME CONTEXT (Phase 2) ---
	if p.RegimeStats != nil {
		b.WriteString("\n\xF0\x9F\x8F\x9B <b>REGIME CONTEXT</b>\n")
		b.WriteString(fmt.Sprintf("<code>Current : %s</code>\n", p.RegimeStats.RegimeName))
		if p.RegimeStats.SampleSize > 0 {
			b.WriteString(fmt.Sprintf("<code>In regime: %+.1f%% avg, %.0f%% WR (n=%d)</code>\n",
				p.RegimeStats.AvgReturn, p.RegimeStats.WinRate, p.RegimeStats.SampleSize))
		} else {
			b.WriteString("<code>In regime: no historical data in same regime</code>\n")
		}
		b.WriteString(fmt.Sprintf("<code>Driver  : %s</code>\n", p.RegimeStats.PrimaryFREDDriver))
		driverIcon := "\xe2\x9e\x96"
		switch p.RegimeStats.DriverAlignment {
		case "SUPPORTIVE":
			driverIcon = "\xe2\x9c\x85"
		case "HEADWIND":
			driverIcon = "\xe2\x9d\x8c"
		}
		b.WriteString(fmt.Sprintf("<code>Outlook : %s %s</code>\n", p.RegimeStats.DriverAlignment, driverIcon))
	} else {
		b.WriteString("\n\xF0\x9F\x8F\x9B <b>REGIME CONTEXT</b>\n")
		b.WriteString("<code>Insufficient history for regime analysis</code>\n")
	}

	// --- COT ALIGNMENT (Phase 3a) ---
	if p.COTAlignment != nil {
		b.WriteString("\n\xF0\x9F\x93\x8A <b>COT ALIGNMENT</b>\n")
		alignIcon := "\xe2\x9d\x8c"
		if p.COTAlignment.CurrentAligned {
			alignIcon = "\xe2\x9c\x85"
		}
		b.WriteString(fmt.Sprintf("<code>Current : COT %s %s</code>\n", p.COTAlignment.CurrentCOTBias, alignIcon))
		b.WriteString(fmt.Sprintf("<code>Interp  : %s</code>\n", p.COTAlignment.Interpretation))
	} else {
		b.WriteString("\n\xF0\x9F\x93\x8A <b>COT ALIGNMENT</b>\n")
		b.WriteString("<code>Insufficient history for COT analysis</code>\n")
	}

	// --- EVENT DENSITY (Phase 3b) ---
	if p.EventDensity != nil {
		b.WriteString("\n\xF0\x9F\x93\x86 <b>EVENT DENSITY</b>\n")
		evIcon := "\xF0\x9F\x9F\xA2"
		if p.EventDensity.Rating == "HIGH" {
			evIcon = "\xF0\x9F\x94\xB4"
		} else if p.EventDensity.Rating == "MEDIUM" {
			evIcon = "\xF0\x9F\x9F\xA1"
		}
		b.WriteString(fmt.Sprintf("<code>Rating  : %s %s (%d high-impact)</code>\n",
			p.EventDensity.Rating, evIcon, p.EventDensity.HighImpactEvents))
		if p.EventDensity.KeyEvents != "" {
			b.WriteString(fmt.Sprintf("<code>Events  : %s</code>\n", html.EscapeString(p.EventDensity.KeyEvents)))
		}
	} else {
		b.WriteString("\n\xF0\x9F\x93\x86 <b>EVENT DENSITY</b>\n")
		b.WriteString("<code>Insufficient history for event analysis</code>\n")
	}

	// --- VOLATILITY CONTEXT (Phase 3c) ---
	if p.VolContext != nil && p.VolContext.AvgATR > 0 {
		b.WriteString("\n\xF0\x9F\x93\x89 <b>VOLATILITY</b>\n")
		b.WriteString(fmt.Sprintf("<code>Month vol: %.1f%% (%.1fx avg)</code>\n",
			p.VolContext.HistoricalATR, p.VolContext.VolRatio))
		if p.VolContext.CurrentVIXRegime != "N/A" {
			b.WriteString(fmt.Sprintf("<code>VIX     : %s (sensitivity: %s)</code>\n",
				p.VolContext.CurrentVIXRegime, p.VolContext.VIXSensitivity))
		}
		if p.VolContext.Assessment != "" {
			b.WriteString(fmt.Sprintf("<i>%s</i>\n", p.VolContext.Assessment))
		}
	} else if p.VolContext == nil {
		b.WriteString("\n\xF0\x9F\x93\x89 <b>VOLATILITY</b>\n")
		b.WriteString("<code>Insufficient history for volatility analysis</code>\n")
	}

	// --- CROSS-ASSET (Phase 3d) ---
	if p.CrossAsset != nil && len(p.CrossAsset.Correlations) > 0 {
		b.WriteString("\n\xF0\x9F\x94\x97 <b>CROSS-ASSET</b>\n")
		for _, cc := range p.CrossAsset.Correlations {
			checkIcon := "\xe2\x9c\x85"
			if !cc.IsAligned {
				checkIcon = "\xe2\x9a\xa0\xef\xb8\x8f"
			}
			b.WriteString(fmt.Sprintf("<code>%-6s %s seasonal %s</code> %s\n",
				cc.Asset, cc.Relation, cc.TheirBias, checkIcon))
		}
		b.WriteString(fmt.Sprintf("<code>Assessment: %s</code>\n", p.CrossAsset.Assessment))
	} else if p.CrossAsset == nil {
		b.WriteString("\n\xF0\x9F\x94\x97 <b>CROSS-ASSET</b>\n")
		b.WriteString("<code>Insufficient history for cross-asset analysis</code>\n")
	}

	// --- EIA ENERGY CONTEXT (Phase 4) ---
	if p.EIACtx != nil {
		b.WriteString("\n\xF0\x9F\x9B\xA2 <b>EIA ENERGY CONTEXT</b>\n")
		if p.EIACtx.InventoryTrend != "" {
			trendIcon := "\xe2\x9e\x96"
			switch p.EIACtx.InventoryTrend {
			case "BUILD":
				trendIcon = "\xF0\x9F\x93\x88"
			case "DRAW":
				trendIcon = "\xF0\x9F\x93\x89"
			}
			unit := "M bbl/wk"
			if p.Currency == "NG" || p.Currency == "NATGAS" {
				unit = "BCF/wk"
			}
			b.WriteString(fmt.Sprintf("<code>Inventory: %s %s (avg %+.1f %s)</code>\n",
				p.EIACtx.InventoryTrend, trendIcon, p.EIACtx.AvgWeeklyChange, unit))
		}
		if p.EIACtx.RefineryUtil > 0 {
			b.WriteString(fmt.Sprintf("<code>Refinery : %.1f%% utilization</code>\n", p.EIACtx.RefineryUtil))
		}
		if p.EIACtx.CurrentVs5YrAvg != "" {
			b.WriteString(fmt.Sprintf("<code>vs 5yr   : %s seasonal average</code>\n", p.EIACtx.CurrentVs5YrAvg))
		}
		if p.EIACtx.Assessment != "" {
			b.WriteString(fmt.Sprintf("<i>%s</i>\n", p.EIACtx.Assessment))
		}
	}

	// --- CONFLUENCE SCORE (Phase 5) ---
	if p.Confluence != nil {
		b.WriteString("\n\xF0\x9F\x8E\xAF <b>CONFLUENCE SCORE</b>\n")

		// Visual bar
		filled := p.Confluence.Score
		empty := p.Confluence.MaxScore - filled
		bar := strings.Repeat("\xe2\x96\x88", filled) + strings.Repeat("\xe2\x96\x91", empty)
		b.WriteString(fmt.Sprintf("<code>%s %d/%d</code>\n", bar, p.Confluence.Score, p.Confluence.MaxScore))

		// Factor details
		for _, factor := range p.Confluence.Factors {
			checkIcon := "\xe2\x9c\x97"
			if factor.Aligned {
				checkIcon = "\xe2\x9c\x93"
			}
			b.WriteString(fmt.Sprintf("<code>%s %-11s %s</code>\n", checkIcon, factor.Name, html.EscapeString(factor.Detail)))
		}

		b.WriteString(fmt.Sprintf("\n<b>Verdict: %s</b>\n", p.Confluence.Verdict))
	} else {
		b.WriteString("\n\xF0\x9F\x8E\xAF <b>CONFLUENCE SCORE</b>\n")
		b.WriteString("<code>Insufficient history for confluence scoring</code>\n")
	}

	return b.String()
}

// FormatDailyPrice formats a DailyPriceContext for Telegram display.
func (f *Formatter) FormatDailyPrice(dc *domain.DailyPriceContext) string {
	var b strings.Builder

	// Header with price and daily change
	arrow := "→"
	if dc.DailyChgPct > 0 {
		arrow = "▲"
	} else if dc.DailyChgPct < 0 {
		arrow = "▼"
	}

	b.WriteString(fmt.Sprintf("💹 <b>%s — %s %s</b>\n\n",
		dc.Currency, formatDailyPrice(dc.CurrentPrice, dc.Currency), arrow))

	// Change section
	b.WriteString("<b>📊 Price Changes</b>\n")
	b.WriteString(fmt.Sprintf("<code>Daily  : %+.2f%%</code>\n", dc.DailyChgPct))
	b.WriteString(fmt.Sprintf("<code>5-Day  : %+.2f%%</code>\n", dc.WeeklyChgPct))
	b.WriteString(fmt.Sprintf("<code>20-Day : %+.2f%%</code>\n", dc.MonthlyChgPct))

	// Consecutive days
	if dc.ConsecDays >= 2 {
		dirEmoji := "📈"
		if dc.ConsecDir == "DOWN" {
			dirEmoji = "📉"
		}
		b.WriteString(fmt.Sprintf("<code>Streak : %d days %s</code> %s\n", dc.ConsecDays, dc.ConsecDir, dirEmoji))
	}

	// Moving Averages
	b.WriteString("\n<b>📐 Moving Averages</b>\n")

	maStatus := func(price, ma float64, label string) string {
		if ma == 0 {
			return fmt.Sprintf("<code>%s: N/A</code>", label)
		}
		icon := "✅"
		pos := "above"
		if price < ma {
			icon = "❌"
			pos = "below"
		}
		return fmt.Sprintf("<code>%s: %s</code> %s (%s)", label, formatDailyPrice(ma, dc.Currency), icon, pos)
	}

	b.WriteString(maStatus(dc.CurrentPrice, dc.DMA20, "20 DMA ") + "\n")
	b.WriteString(maStatus(dc.CurrentPrice, dc.DMA50, "50 DMA ") + "\n")
	b.WriteString(maStatus(dc.CurrentPrice, dc.DMA200, "200 DMA") + "\n")

	// MA Trend alignment
	maTrend := dc.MATrendDaily()
	trendEmoji := "⚪"
	switch maTrend {
	case "BULLISH":
		trendEmoji = "🟢"
	case "BEARISH":
		trendEmoji = "🔴"
	}
	b.WriteString(fmt.Sprintf("<code>Alignment: %s</code> %s\n", maTrend, trendEmoji))

	// Volatility
	if dc.DailyATR > 0 {
		b.WriteString("\n<b>📏 Volatility</b>\n")
		b.WriteString(fmt.Sprintf("<code>Daily ATR : %s (%.2f%%)</code>\n",
			formatDailyPrice(dc.DailyATR, dc.Currency), dc.NormalizedATR))
	}

	// Momentum
	b.WriteString("\n<b>🚀 Momentum</b>\n")
	b.WriteString(fmt.Sprintf("<code>5D  ROC: %+.2f%%</code>\n", dc.Momentum5D))
	b.WriteString(fmt.Sprintf("<code>10D ROC: %+.2f%%</code>\n", dc.Momentum10D))
	b.WriteString(fmt.Sprintf("<code>20D ROC: %+.2f%%</code>\n", dc.Momentum20D))

	// Daily trend
	trendIcon := "➡️"
	switch dc.DailyTrend {
	case "UP":
		trendIcon = "📈"
	case "DOWN":
		trendIcon = "📉"
	}
	b.WriteString(fmt.Sprintf("\n<code>Trend: %s</code> %s\n", dc.DailyTrend, trendIcon))

	return b.String()
}

// formatDailyPrice is a local helper for FormatDailyPrice formatting.
func formatDailyPrice(price float64, currency string) string {
	switch {
	case currency == "JPY":
		return fmt.Sprintf("%.3f", price)
	case currency == "XAU" || currency == "XAG":
		return fmt.Sprintf("%.2f", price)
	case currency == "BTC" || currency == "ETH":
		return fmt.Sprintf("%.0f", price)
	case currency == "OIL" || currency == "COPPER":
		return fmt.Sprintf("%.2f", price)
	case strings.HasPrefix(currency, "BOND") || currency == "SPX500" || currency == "NDX" || currency == "DJI" || currency == "RUT":
		return fmt.Sprintf("%.2f", price)
	default:
		if price > 10 {
			return fmt.Sprintf("%.4f", price)
		}
		return fmt.Sprintf("%.5f", price)
	}
}

// FormatDailyMomentumSnapshot formats a compact daily momentum view for /rank.
func (f *Formatter) FormatDailyMomentumSnapshot(dailyCtxs map[string]*domain.DailyPriceContext) string {
	if len(dailyCtxs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n📈 <b>Daily Momentum</b>\n<pre>")
	b.WriteString("Pair   Day%   5D%    MA   Strk\n")
	b.WriteString("─────────────────────────────\n")

	// Sort by daily change descending
	type entry struct {
		currency string
		dc       *domain.DailyPriceContext
	}
	var entries []entry
	for _, dc := range dailyCtxs {
		entries = append(entries, entry{dc.Currency, dc})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].dc.DailyChgPct > entries[j].dc.DailyChgPct
	})

	for _, e := range entries {
		dc := e.dc
		// Skip non-core instruments for compact view
		if strings.HasPrefix(e.currency, "BOND") || e.currency == "ULSD" || e.currency == "RBOB" {
			continue
		}

		maTrend := dc.MATrendDaily()
		maIcon := "·"
		switch maTrend {
		case "BULLISH":
			maIcon = "▲"
		case "BEARISH":
			maIcon = "▼"
		}

		streak := "  "
		if dc.ConsecDays >= 2 {
			dir := "↑"
			if dc.ConsecDir == "DOWN" {
				dir = "↓"
			}
			streak = fmt.Sprintf("%d%s", dc.ConsecDays, dir)
		}

		b.WriteString(fmt.Sprintf("%-6s %+5.1f%% %+5.1f%%  %s   %s\n",
			dc.Currency, dc.DailyChgPct, dc.WeeklyChgPct, maIcon, streak))
	}
	b.WriteString("</pre>")

	return b.String()
}

// FormatLevels formats support/resistance levels and pivot points.
func (f *Formatter) FormatLevels(lc *pricesvc.LevelsContext, currency string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\xF0\x9F\x93\x8F <b>KEY LEVELS: %s</b>\n\n", currency))

	b.WriteString(fmt.Sprintf("<code>Price    :</code> %s\n", formatPrice(lc.CurrentPrice, currency)))
	if lc.DailyATR > 0 {
		b.WriteString(fmt.Sprintf("<code>Daily ATR:</code> %s (%.2f%%)\n\n",
			formatPrice(lc.DailyATR, currency),
			lc.DailyATR/lc.CurrentPrice*100))
	}

	// Pivot points
	b.WriteString("<b>Daily Pivots</b>\n")
	b.WriteString(fmt.Sprintf("<code>R2    :</code> %s\n", formatPrice(lc.PivotR2, currency)))
	b.WriteString(fmt.Sprintf("<code>R1    :</code> %s\n", formatPrice(lc.PivotR1, currency)))
	b.WriteString(fmt.Sprintf("<code>Pivot :</code> %s\n", formatPrice(lc.DailyPivot, currency)))
	b.WriteString(fmt.Sprintf("<code>S1    :</code> %s\n", formatPrice(lc.PivotS1, currency)))
	b.WriteString(fmt.Sprintf("<code>S2    :</code> %s\n\n", formatPrice(lc.PivotS2, currency)))

	// Key S/R levels (top 10 by proximity)
	maxLevels := 10
	if len(lc.Levels) < maxLevels {
		maxLevels = len(lc.Levels)
	}

	if maxLevels > 0 {
		b.WriteString("<b>Support / Resistance</b>\n")
		b.WriteString("<pre>")
		b.WriteString(fmt.Sprintf("%-12s %-5s %7s %s\n", "Level", "Type", "Dist", "Source"))
		for i := 0; i < maxLevels; i++ {
			l := lc.Levels[i]
			typeIcon := "S"
			if l.Type == "RESISTANCE" {
				typeIcon = "R"
			}
			stars := strings.Repeat("*", l.Strength)
			b.WriteString(fmt.Sprintf("%-12s %-5s %+6.2f%% %s\n",
				formatPrice(l.Price, currency), typeIcon+stars, l.Distance, l.Source))
		}
		b.WriteString("</pre>\n")
	}

	// Nearest S/R summary
	if lc.NearestSupport != nil {
		b.WriteString(fmt.Sprintf("\xF0\x9F\x9F\xA2 <b>Nearest Support:</b> %s (%+.2f%%) — %s\n",
			formatPrice(lc.NearestSupport.Price, currency),
			lc.NearestSupport.Distance,
			lc.NearestSupport.Source))
	}
	if lc.NearestResistance != nil {
		b.WriteString(fmt.Sprintf("\xF0\x9F\x94\xB4 <b>Nearest Resistance:</b> %s (%+.2f%%) — %s\n",
			formatPrice(lc.NearestResistance.Price, currency),
			lc.NearestResistance.Distance,
			lc.NearestResistance.Source))
	}

	return b.String()
}
