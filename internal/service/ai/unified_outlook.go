package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
	"github.com/arkcode369/ark-intelligent/internal/service/worldbank"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// UnifiedOutlookData holds ALL available data sources for the comprehensive
// unified outlook prompt. Each field is optional — the prompt builder only
// includes sections for which data is present.
type UnifiedOutlookData struct {
	COTAnalyses        []domain.COTAnalysis
	NewsEvents         []domain.NewsEvent
	MacroData          *fred.MacroData
	MacroRegime        *fred.MacroRegime
	MacroComposites    *domain.MacroComposites
	PriceContexts      map[string]*domain.PriceContext
	DailyPriceContexts map[string]*domain.DailyPriceContext
	RiskContext         *domain.RiskContext
	SentimentData      *sentiment.SentimentData
	SeasonalData       map[string]*pricesvc.SeasonalPattern
	BacktestStats      *domain.BacktestStats
	CurrencyStrength   []pricesvc.CurrencyStrength
	WorldBankData      *worldbank.WorldBankData
	Language           string
}

// BuildUnifiedOutlookPrompt builds a comprehensive prompt that fuses ALL
// available data sources into a single analysis request. Sections are
// dynamically numbered and only included when data is present.
func BuildUnifiedOutlookPrompt(data UnifiedOutlookData) string {
	var b strings.Builder
	now := time.Now().UTC().Add(7 * time.Hour) // WIB
	b.WriteString("Generate a comprehensive UNIFIED MARKET OUTLOOK fusing all available data sources.\n")
	b.WriteString(fmt.Sprintf("Analysis date: %s (WIB).\n", now.Format("02 January 2006")))

	if data.Language == "en" {
		b.WriteString("PLEASE RESPOND IN ENGLISH.\n\n")
	} else {
		b.WriteString("PLEASE RESPOND IN INDONESIAN (Bahasa Indonesia).\n\n")
	}

	section := 1

	// -----------------------------------------------------------------------
	// Section 1: COT Positioning
	// -----------------------------------------------------------------------
	if len(data.COTAnalyses) > 0 {
		b.WriteString(fmt.Sprintf("=== %d. COT POSITIONING ===\n", section))
		section++
		for _, a := range data.COTAnalyses {
			b.WriteString(fmt.Sprintf("%s (Report: %s): SpecNet=%s COTIdx=%.0f CommSignal=%s Crowding=%.1f",
				a.Contract.Currency,
				a.ReportDate.Format("2006-01-02"),
				fmtutil.FmtNumSigned(a.NetPosition, 0),
				a.COTIndex, a.CommercialSignal, a.CrowdingIndex))
			if a.AssetMgrAlert {
				b.WriteString(fmt.Sprintf(" [AssetMgrAlert Z=%.1f]", a.AssetMgrZScore))
			}
			b.WriteString("\n")

			// Options data if available
			if a.OptionsNetPosition != 0 || a.OptionsSmartBias != "" {
				b.WriteString(fmt.Sprintf("  Options: NetPos=%s PctOfOI=%.1f%% SmartBias=%s\n",
					fmtutil.FmtNumSigned(a.OptionsNetPosition, 0),
					a.OptionsPctOfTotalOI,
					a.OptionsSmartBias))
			}
		}
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 2: Price Context
	// -----------------------------------------------------------------------
	if len(data.PriceContexts) > 0 {
		b.WriteString(fmt.Sprintf("=== %d. PRICE CONTEXT (Weekly Closes) ===\n", section))
		section++
		// Iterate in COT order when available, otherwise iterate the map directly.
		if len(data.COTAnalyses) > 0 {
			for _, a := range data.COTAnalyses {
				if pc, ok := data.PriceContexts[a.Contract.Code]; ok {
					writePriceContextLine(&b, a.Contract.Currency, pc)
				}
			}
		} else {
			for code, pc := range data.PriceContexts {
				writePriceContextLine(&b, code, pc)
			}
		}
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 2b: Daily Price Context (if available)
	// -----------------------------------------------------------------------
	if len(data.DailyPriceContexts) > 0 {
		b.WriteString(fmt.Sprintf("=== %d. DAILY PRICE CONTEXT (Technical) ===\n", section))
		section++
		if len(data.COTAnalyses) > 0 {
			for _, a := range data.COTAnalyses {
				if dc, ok := data.DailyPriceContexts[a.Contract.Code]; ok {
					writeDailyPriceContextLine(&b, a.Contract.Currency, dc)
				}
			}
		} else {
			for _, dc := range data.DailyPriceContexts {
				writeDailyPriceContextLine(&b, dc.Currency, dc)
			}
		}
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 3: Economic Calendar
	// -----------------------------------------------------------------------
	if len(data.NewsEvents) > 0 {
		b.WriteString(fmt.Sprintf("=== %d. ECONOMIC CALENDAR (HIGH IMPACT) ===\n", section))
		section++
		for _, e := range data.NewsEvents {
			if e.Impact == "high" || e.Impact == "medium" {
				line := fmt.Sprintf("%s | %s - %s | Impact: %s | Fcast: %s | Prev: %s | Act: %s",
					e.Date, e.Currency, e.Event, e.Impact,
					e.Forecast, e.Previous, e.Actual)
				if e.SurpriseScore != 0 {
					line += fmt.Sprintf(" | Surprise: %.1f\u03c3 %s", e.SurpriseScore, e.SurpriseLabel)
				}
				if e.OldPrevious != "" && e.OldPrevious != e.Previous {
					line += fmt.Sprintf(" | Rev: %s\u2192%s", e.OldPrevious, e.Previous)
				}
				b.WriteString(line + "\n")
			}
		}
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 4: FRED Macro Backdrop
	// -----------------------------------------------------------------------
	if data.MacroData != nil && data.MacroRegime != nil {
		b.WriteString(fmt.Sprintf("=== %d. FRED MACRO BACKDROP ===\n", section))
		section++
		m := data.MacroData
		regime := data.MacroRegime

		b.WriteString(fmt.Sprintf("Macro Regime: %s (Risk-Off Score: %d/100 | Recession Risk: %s)\n",
			regime.Name, regime.Score, regime.RecessionRisk))

		// Full yield curve
		b.WriteString(fmt.Sprintf("Yields: 3M=%.2f%% 2Y=%.2f%% 5Y=%.2f%% 10Y=%.2f%% 30Y=%.2f%%\n",
			m.Yield3M, m.Yield2Y, m.Yield5Y, m.Yield10Y, m.Yield30Y))
		b.WriteString(fmt.Sprintf("2Y-10Y Spread: %+.2f%% %s (%s)\n",
			m.YieldSpread, m.YieldSpreadTrend.Arrow(), regime.YieldCurve))
		if m.Yield3M > 0 {
			b.WriteString(fmt.Sprintf("3M-10Y Spread: %+.2f%% (%s)\n", m.Spread3M10Y, regime.Yield3M10Y))
		}
		if m.Spread2Y30Y != 0 {
			b.WriteString(fmt.Sprintf("2Y-30Y Spread: %+.2f%% (%s)\n", m.Spread2Y30Y, regime.Yield2Y30Y))
		}

		// Inflation
		if m.CorePCE > 0 {
			b.WriteString(fmt.Sprintf("Core PCE: %.2f%% %s | ", m.CorePCE, m.CorePCETrend.Arrow()))
		}
		if m.CPI > 0 {
			b.WriteString(fmt.Sprintf("CPI: %.2f%% %s\n", m.CPI, m.CPITrend.Arrow()))
		} else if m.CorePCE > 0 {
			b.WriteString("\n")
		}

		// Monetary policy
		if m.FedFundsRate > 0 {
			realRate := m.FedFundsRate - m.Breakeven5Y
			b.WriteString(fmt.Sprintf("FFR: %.2f%% (Real: %+.2f%%)", m.FedFundsRate, realRate))
		}
		if m.SOFR > 0 && m.IORB > 0 {
			b.WriteString(fmt.Sprintf(" | SOFR: %.2f%% IORB: %.2f%%", m.SOFR, m.IORB))
		}
		if m.FedFundsRate > 0 || (m.SOFR > 0 && m.IORB > 0) {
			b.WriteString("\n")
		}

		// Financial stress
		b.WriteString(fmt.Sprintf("NFCI: %.3f %s (%s)\n", m.NFCI, m.NFCITrend.Arrow(), regime.FinStress))

		// Labor
		if m.InitialClaims > 0 {
			b.WriteString(fmt.Sprintf("Claims: %.0fK %s | ", m.InitialClaims/1_000, m.ClaimsTrend.Arrow()))
		}
		if m.UnemployRate > 0 {
			b.WriteString(fmt.Sprintf("U-Rate: %.1f%%\n", m.UnemployRate))
		} else if m.InitialClaims > 0 {
			b.WriteString("\n")
		}

		// Sahm Rule
		if m.SahmRule > 0 {
			b.WriteString(fmt.Sprintf("Sahm Rule: %.2f (%s)\n", m.SahmRule, regime.SahmLabel))
		}

		// Growth & money supply
		if m.GDPGrowth != 0 {
			b.WriteString(fmt.Sprintf("GDP Growth: %.1f%% QoQ ann.\n", m.GDPGrowth))
		}
		if m.M2Growth != 0 {
			b.WriteString(fmt.Sprintf("M2 YoY: %+.1f%% %s (%s)\n",
				m.M2Growth, m.M2GrowthTrend.Arrow(), regime.M2Label))
		}

		// Fed balance sheet
		if m.FedBalSheet > 0 {
			b.WriteString(fmt.Sprintf("Fed Balance: $%.2fT %s (%s)\n",
				m.FedBalSheet/1_000, m.FedBalSheetTrend.Arrow(), regime.FedBalance))
		}

		// USD
		if m.DXY > 0 {
			b.WriteString(fmt.Sprintf("DXY: %.1f (%s)\n", m.DXY, regime.USDStrength))
		}
		b.WriteString(fmt.Sprintf("Implied Bias: %s\n", regime.Bias))
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section X: Macro Composite Scores
	// -----------------------------------------------------------------------
	if data.MacroComposites != nil {
		b.WriteString(fmt.Sprintf("=== %d. MACRO COMPOSITE SCORES ===\n", section))
		section++
		comp := data.MacroComposites
		b.WriteString(fmt.Sprintf("Labor Health: %.0f/100 (%s)\n", comp.LaborHealth, comp.LaborLabel))
		b.WriteString(fmt.Sprintf("Inflation Momentum: %+.2f (%s)\n", comp.InflationMomentum, comp.InflationLabel))
		b.WriteString(fmt.Sprintf("Yield Curve: %s\n", comp.YieldCurveSignal))
		b.WriteString(fmt.Sprintf("Credit Stress: %.0f/100 (%s)\n", comp.CreditStress, comp.CreditLabel))
		b.WriteString(fmt.Sprintf("Housing Pulse: %s\n", comp.HousingPulse))
		b.WriteString(fmt.Sprintf("Financial Conditions: %+.2f\n", comp.FinConditions))
		b.WriteString(fmt.Sprintf("Sentiment Composite: %+.0f (%s)\n", comp.SentimentComposite, comp.SentimentLabel))
		if comp.VIXTermRegime != "" && comp.VIXTermRegime != "N/A" {
			b.WriteString(fmt.Sprintf("VIX Term Structure: %s (ratio: %.3f)\n", comp.VIXTermRegime, comp.VIXTermRatio))
		}
		b.WriteString(fmt.Sprintf("Country Scores: US=%+.0f EZ=%+.0f UK=%+.0f JP=%+.0f AU=%+.0f CA=%+.0f NZ=%+.0f\n",
			comp.USScore, comp.EZScore, comp.UKScore, comp.JPScore, comp.AUScore, comp.CAScore, comp.NZScore))
		b.WriteString("Use these composite scores as the PRIMARY basis for macro assessment — they synthesize 80+ underlying FRED series.\n")
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 5: Risk Sentiment (VIX/SPX)
	// -----------------------------------------------------------------------
	if data.RiskContext != nil {
		rc := data.RiskContext
		b.WriteString(fmt.Sprintf("=== %d. RISK SENTIMENT (VIX/SPX) ===\n", section))
		section++
		b.WriteString(fmt.Sprintf("VIX: %.2f (4W Avg: %.2f) | Trend: %s | Regime: %s\n",
			rc.VIXLevel, rc.VIX4WAvg, rc.VIXTrend, string(rc.Regime)))
		b.WriteString(fmt.Sprintf("SPX: Wk %+.2f%% | Mo %+.2f%% | Above MA4W: %s\n",
			rc.SPXWeeklyChg, rc.SPXMonthlyChg, maLabel(rc.SPXAboveMA4W)))
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 6: Market Sentiment (CNN Fear & Greed)
	// -----------------------------------------------------------------------
	if data.SentimentData != nil {
		sd := data.SentimentData
		if sd.CNNAvailable {
			b.WriteString(fmt.Sprintf("=== %d. MARKET SENTIMENT ===\n", section))
			section++
			b.WriteString(fmt.Sprintf("CNN Fear & Greed: %.0f/100 (%s)\n", sd.CNNFearGreed, sd.CNNFearGreedLabel))
			if sd.CryptoFearGreedAvailable {
				b.WriteString(fmt.Sprintf("Crypto Fear & Greed: %.0f/100 (%s)\n", sd.CryptoFearGreed, sd.CryptoFearGreedLabel))
			}
			if sd.AAIIAvailable {
				b.WriteString(fmt.Sprintf("AAII: Bull=%.1f%% Bear=%.1f%% Neutral=%.1f%% (B/B Ratio=%.2f)\n",
					sd.AAIIBullish, sd.AAIIBearish, sd.AAIINeutral, sd.AAIIBullBear))
			}
			b.WriteString("\n")
		}
	}

	// -----------------------------------------------------------------------
	// Section 7: Seasonal Patterns
	// -----------------------------------------------------------------------
	if len(data.SeasonalData) > 0 {
		b.WriteString(fmt.Sprintf("=== %d. SEASONAL PATTERNS ===\n", section))
		section++
		for _, a := range data.COTAnalyses {
			if sp, ok := data.SeasonalData[a.Contract.Code]; ok {
				curIdx := sp.CurrentMonth - 1
				if curIdx >= 0 && curIdx < 12 {
					ms := sp.Monthly[curIdx]
					b.WriteString(fmt.Sprintf("%s: %s bias (Month: %s | AvgRet: %+.2f%% | WinRate: %.0f%% | N=%d)\n",
						sp.Currency, sp.CurrentBias, ms.Month, ms.AvgReturn, ms.WinRate, ms.SampleSize))
				}
			}
		}
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 8: Currency Strength Index
	// -----------------------------------------------------------------------
	if len(data.CurrencyStrength) > 0 {
		b.WriteString(fmt.Sprintf("=== %d. CURRENCY STRENGTH INDEX ===\n", section))
		section++
		for _, cs := range data.CurrencyStrength {
			line := fmt.Sprintf("#%d %s: Combined=%.1f (Price=%.1f #%d | COT=%.1f #%d)",
				cs.CombinedRank, cs.Currency,
				cs.CombinedScore, cs.PriceScore, cs.PriceRank,
				cs.COTScore, cs.COTRank)
			if cs.Divergence {
				line += fmt.Sprintf(" DIVERGENCE: %s", cs.DivergenceMsg)
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 9: Backtest Statistics
	// -----------------------------------------------------------------------
	if data.BacktestStats != nil && data.BacktestStats.Evaluated > 0 {
		bs := data.BacktestStats
		b.WriteString(fmt.Sprintf("=== %d. SIGNAL ACCURACY CONTEXT ===\n", section))
		section++
		b.WriteString(fmt.Sprintf("Historical signals: %d evaluated\n", bs.Evaluated))
		b.WriteString(fmt.Sprintf("Win rates: 1W=%.0f%% 2W=%.0f%% 4W=%.0f%%\n", bs.WinRate1W, bs.WinRate2W, bs.WinRate4W))
		b.WriteString(fmt.Sprintf("Best holding period: %s (%.0f%% win rate)\n", bs.BestPeriod, bs.BestWinRate))
		if bs.HighStrengthCount > 0 {
			b.WriteString(fmt.Sprintf("High-strength signals (4-5): %.0f%% win rate\n", bs.HighStrengthWinRate))
		}
		b.WriteString("NOTE: Weight your conviction based on historical accuracy. High-strength signals have proven more reliable.\n")
		b.WriteString("\n")
	}

	// -----------------------------------------------------------------------
	// Section 10: Cross-Country Macro Fundamentals (World Bank)
	// -----------------------------------------------------------------------
	if data.WorldBankData != nil {
		available := make([]worldbank.CountryMacro, 0, len(data.WorldBankData.Countries))
		for _, c := range data.WorldBankData.Countries {
			if c.Available {
				available = append(available, c)
			}
		}
		if len(available) > 0 {
			b.WriteString(fmt.Sprintf("=== %d. CROSS-COUNTRY MACRO FUNDAMENTALS (World Bank, Annual) ===\n", section))
			section++ //nolint:ineffassign // section may be used in future extensions
			b.WriteString(fmt.Sprintf("%-6s  %6s  %12s  %8s  %4s\n",
				"CCY", "GDP%", "CA(USDbn)", "CPI%", "Year"))
			b.WriteString(strings.Repeat("-", 46) + "\n")
			for _, c := range available {
				caSign := "+"
				if c.CurrentAccount < 0 {
					caSign = ""
				}
				b.WriteString(fmt.Sprintf("%-6s  %+6.2f  %s%11.1f  %+8.2f  %4d\n",
					c.Currency, c.GDPGrowth, caSign, c.CurrentAccount, c.CPIInflation, c.Year))
			}
			b.WriteString("\n")
			b.WriteString("NOTE: Use differentials for fundamental FX bias:\n")
			b.WriteString("  - Higher GDP growth differential → currency structural tailwind\n")
			b.WriteString("  - Current Account surplus → structural currency demand\n")
			b.WriteString("  - Lower inflation → PPP-based currency strength over time\n")
			b.WriteString("\n")
		}
	}

	// -----------------------------------------------------------------------
	// Analysis Request
	// -----------------------------------------------------------------------
	b.WriteString("=== ANALYSIS REQUESTED ===\n")
	if data.Language == "en" {
		b.WriteString("Provide a comprehensive UNIFIED OUTLOOK covering:\n\n")
		b.WriteString("1. MACRO REGIME & MARKET CONTEXT\n")
		b.WriteString("   Synthesize all macro data (FRED, risk sentiment, market sentiment, seasonal) into a coherent narrative.\n")
		b.WriteString("   What is the dominant regime? Risk-on or risk-off? What is the macro trajectory?\n\n")
		b.WriteString("2. CURRENCY-BY-CURRENCY ANALYSIS\n")
		b.WriteString("   For each G8 currency, synthesize COT positioning + price action + seasonal pattern + sentiment into:\n")
		b.WriteString("   - Directional bias: BULLISH / BEARISH / NEUTRAL\n")
		b.WriteString("   - Conviction level: 1-5 (5 = highest)\n")
		b.WriteString("   - Key supporting/conflicting signals\n\n")
		b.WriteString("3. TOP 3 TRADE SETUPS\n")
		b.WriteString("   Highest conviction setups with entry logic, supported by multiple data points.\n")
		b.WriteString("   Each setup must cite at least 3 confirming signals from different data sources.\n\n")
		b.WriteString("4. CROSS-MARKET SIGNALS\n")
		b.WriteString("   Risk-on/risk-off assessment. Gold/Oil/Bond implications.\n")
		b.WriteString("   Safe-haven flow analysis (JPY, CHF, Gold vs risk FX).\n\n")
		b.WriteString("5. KEY RISKS & CATALYSTS\n")
		b.WriteString("   What upcoming events could change the thesis?\n")
		b.WriteString("   Identify crowded-exit risks where heavy positioning faces catalyst risk.\n\n")
		b.WriteString("6. WEB SEARCH ENRICHMENT\n")
		b.WriteString("   Use web_search to verify current prices, check for breaking news, and validate your analysis with real-time data.\n")
		b.WriteString("   Search for latest central bank statements, geopolitical developments, or any market-moving news not captured in the data above.\n\n")
	} else {
		b.WriteString("Berikan UNIFIED OUTLOOK komprehensif yang mencakup:\n\n")
		b.WriteString("1. REGIME MAKRO & KONTEKS PASAR\n")
		b.WriteString("   Sintesiskan seluruh data makro (FRED, sentimen risiko, sentimen pasar, seasonal) menjadi narasi yang koheren.\n")
		b.WriteString("   Apa regime dominan? Risk-on atau risk-off? Apa trajectory makro?\n\n")
		b.WriteString("2. ANALISIS PER MATA UANG\n")
		b.WriteString("   Untuk setiap mata uang G8, sintesiskan positioning COT + price action + pola seasonal + sentimen menjadi:\n")
		b.WriteString("   - Bias arah: BULLISH / BEARISH / NEUTRAL\n")
		b.WriteString("   - Level konviksi: 1-5 (5 = tertinggi)\n")
		b.WriteString("   - Sinyal pendukung/bertentangan utama\n\n")
		b.WriteString("3. TOP 3 SETUP TRADING\n")
		b.WriteString("   Setup dengan konviksi tertinggi beserta logika entry, didukung oleh beberapa data point.\n")
		b.WriteString("   Setiap setup harus mengutip minimal 3 sinyal konfirmasi dari sumber data berbeda.\n\n")
		b.WriteString("4. SINYAL LINTAS PASAR\n")
		b.WriteString("   Penilaian risk-on/risk-off. Implikasi Gold/Oil/Bond.\n")
		b.WriteString("   Analisis aliran safe-haven (JPY, CHF, Gold vs risk FX).\n\n")
		b.WriteString("5. RISIKO UTAMA & KATALIS\n")
		b.WriteString("   Event mendatang apa yang bisa mengubah tesis?\n")
		b.WriteString("   Identifikasi risiko crowded-exit di mana positioning berat menghadapi risiko katalis.\n\n")
		b.WriteString("6. PENGAYAAN WEB SEARCH\n")
		b.WriteString("   Gunakan web_search untuk memverifikasi harga terkini, cek berita terbaru, dan validasi analisis dengan data real-time.\n")
		b.WriteString("   Cari pernyataan bank sentral terbaru, perkembangan geopolitik, atau berita penggerak pasar yang belum tercakup data di atas.\n\n")
	}

	b.WriteString("You have access to web_search and web_fetch tools. USE THEM to:\n")
	b.WriteString("- Verify current market prices and any moves since the data above was collected\n")
	b.WriteString("- Check for breaking news or geopolitical events\n")
	b.WriteString("- Look up recent central bank speeches or meeting minutes\n")
	b.WriteString("- Validate any assumptions with real-time information\n")
	b.WriteString("This enrichment makes your analysis significantly more valuable.\n")

	return b.String()
}

// writePriceContextLine writes a single price context line for a currency.
func writePriceContextLine(b *strings.Builder, label string, pc *domain.PriceContext) {
	line := fmt.Sprintf("%s: %.5f | Wk %+.2f%% | Mo %+.2f%% | Trend4W: %s | MA4W: %s MA13W: %s",
		label,
		pc.CurrentPrice, pc.WeeklyChgPct, pc.MonthlyChgPct,
		pc.Trend4W, maLabel(pc.AboveMA4W), maLabel(pc.AboveMA13W))
	if pc.ADX > 0 {
		line += fmt.Sprintf(" | ADX: %.0f", pc.ADX)
	}
	if pc.PriceRegime != "" {
		line += fmt.Sprintf(" | Regime: %s", pc.PriceRegime)
	}
	if pc.VolatilityRegime != "" {
		line += fmt.Sprintf(" | Vol: %s", pc.VolatilityRegime)
	}
	b.WriteString(line + "\n")
}

func writeDailyPriceContextLine(b *strings.Builder, label string, dc *domain.DailyPriceContext) {
	line := fmt.Sprintf("%s: Daily %+.2f%% | 5D %+.2f%% | 20D %+.2f%% | DMA20: %s DMA50: %s DMA200: %s | MA: %s",
		label,
		dc.DailyChgPct, dc.WeeklyChgPct, dc.MonthlyChgPct,
		maLabel(dc.AboveDMA20), maLabel(dc.AboveDMA50), maLabel(dc.AboveDMA200),
		dc.MATrendDaily())
	if dc.DailyATR > 0 {
		line += fmt.Sprintf(" | ATR: %.2f%%", dc.NormalizedATR)
	}
	if dc.ConsecDays >= 2 {
		line += fmt.Sprintf(" | Streak: %d%s", dc.ConsecDays, dc.ConsecDir[:1])
	}
	line += fmt.Sprintf(" | Mom5D: %+.2f%%", dc.Momentum5D)
	b.WriteString(line + "\n")
}
