package telegram

import (
	"fmt"
	"strings"
	"github.com/arkcode369/ark-intelligent/internal/service/dvol"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
	"github.com/arkcode369/ark-intelligent/pkg/fmtutil"
)

// sentimentGauge builds a visual gauge bar for Fear & Greed (0-100).
func sentimentGauge(score float64, width int) string {
	pos := int(score / 100 * float64(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}

	bar := make([]byte, width)
	for i := range bar {
		bar[i] = '-'
	}
	bar[pos] = '|'

	return "Fear " + string(bar) + " Greed"
}

// fearGreedEmoji returns an emoji indicator for the CNN F&G score.
func fearGreedEmoji(score float64) string {
	switch {
	case score <= 25:
		return "😱"
	case score <= 45:
		return "😟"
	case score <= 55:
		return "😐"
	case score <= 75:
		return "😏"
	default:
		return "🤑"
	}
}

// sentimentBar builds a compact visual bar for a percentage (0-100).
func sentimentBar(pct float64, emoji string) string {
	const barWidth = 10
	filled := int(pct / 100 * barWidth)
	if filled > barWidth {
		filled = barWidth
	}
	return strings.Repeat(emoji, filled)
}

// FormatSentiment renders the sentiment survey dashboard as Telegram HTML.
// macroRegime is the current FRED regime name (e.g. "GOLDILOCKS"); pass "" to skip regime context.
func (f *Formatter) FormatSentiment(data *sentiment.SentimentData, macroRegime string) string {
	var b strings.Builder

	b.WriteString("🧠 <b>SENTIMENT SURVEY DASHBOARD</b>\n")
	b.WriteString(fmt.Sprintf("<i>Updated %s</i>\n", fmtutil.FormatDateTimeWIB(data.FetchedAt)))

	// --- CNN Fear & Greed Index ---
	b.WriteString("\n<b>CNN Fear &amp; Greed Index</b>\n")
	if data.CNNAvailable {
		gauge := sentimentGauge(data.CNNFearGreed, 15)
		emoji := fearGreedEmoji(data.CNNFearGreed)
		b.WriteString(fmt.Sprintf("<code>[%s]</code>\n", gauge))
		b.WriteString(fmt.Sprintf("<code>Score : %.0f / 100  %s %s</code>\n", data.CNNFearGreed, emoji, data.CNNFearGreedLabel))

		// Trend comparison — show how sentiment has changed
		b.WriteString("<code>Trend :</code>")
		if data.CNNPrev1Week > 0 {
			delta1w := data.CNNFearGreed - data.CNNPrev1Week
			b.WriteString(fmt.Sprintf(" <code>1W: %+.0f</code>", delta1w))
		}
		if data.CNNPrev1Month > 0 {
			delta1m := data.CNNFearGreed - data.CNNPrev1Month
			b.WriteString(fmt.Sprintf(" <code>| 1M: %+.0f</code>", delta1m))
		}
		if data.CNNPrev1Year > 0 {
			delta1y := data.CNNFearGreed - data.CNNPrev1Year
			b.WriteString(fmt.Sprintf(" <code>| 1Y: %+.0f</code>", delta1y))
		}
		b.WriteString("\n")

		// Velocity alert — rapid shift in sentiment
		if data.CNNPrev1Month > 0 {
			monthDelta := data.CNNFearGreed - data.CNNPrev1Month
			if monthDelta < -30 {
				b.WriteString("⚠️ <i>Penurunan tajam dari sebulan lalu — pasar bergeser ke fear cepat</i>\n")
			} else if monthDelta > 30 {
				b.WriteString("⚠️ <i>Lonjakan tajam dari sebulan lalu — euforia meningkat cepat</i>\n")
			}
		}

		// Contrarian signal
		if data.CNNFearGreed <= 25 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Extreme fear sering mendahului kenaikan\n")
		} else if data.CNNFearGreed >= 75 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Extreme greed sering mendahului koreksi\n")
		}
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}

	// --- Crypto Fear & Greed Index (alternative.me) ---
	b.WriteString("\n<b>Crypto Fear &amp; Greed Index</b>\n")
	if data.CryptoFearGreedAvailable {
		gauge := sentimentGauge(data.CryptoFearGreed, 15)
		emoji := fearGreedEmoji(data.CryptoFearGreed)
		b.WriteString(fmt.Sprintf("<code>[%s]</code>\n", gauge))
		b.WriteString(fmt.Sprintf("<code>Score : %.0f / 100  %s %s</code>\n", data.CryptoFearGreed, emoji, data.CryptoFearGreedLabel))
		if data.CryptoFearGreed <= 25 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Extreme fear di crypto bisa jadi zona akumulasi\n")
		} else if data.CryptoFearGreed >= 75 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Extreme greed di crypto sering mendahului koreksi\n")
		}
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}


	// --- Crypto Global Market Data (alternative.me v2) ---
	if data.CryptoGlobalAvailable {
		b.WriteString("\n<b>🌐 Crypto Global Market</b>\n")
		mcapT := data.CryptoTotalMarketCap / 1e12
		b.WriteString(fmt.Sprintf("<code>Total Mcap   : $%.2fT</code>\n", mcapT))
		b.WriteString(fmt.Sprintf("<code>BTC Dominance: %.1f%%</code>", data.CryptoBTCDominance))
		if data.CryptoBTCDominance >= 55 {
			b.WriteString(" — BTC season")
		} else if data.CryptoBTCDominance <= 40 {
			b.WriteString(" — 🔥 Alt season")
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("<code>Currencies   : %d</code>\n", data.CryptoActiveCurrencies))
		b.WriteString(fmt.Sprintf("<code>Markets      : %d</code>\n", data.CryptoActiveMarkets))
	}

	// --- Crypto Top Movers (alternative.me v2) ---
	if data.CryptoTickersAvailable && len(data.CryptoTopTickers) > 0 {
		b.WriteString("\n<b>📊 Crypto Top 10 (24h)</b>\n")
		topN := 10
		if len(data.CryptoTopTickers) < topN {
			topN = len(data.CryptoTopTickers)
		}
		for _, t := range data.CryptoTopTickers[:topN] {
			arrow := "⬆️"
			if t.PercentChange24h < 0 {
				arrow = "⬇️"
			}
			priceStr := ""
			if t.PriceUSD >= 1000 {
				priceStr = fmt.Sprintf("$%.0f", t.PriceUSD)
			} else if t.PriceUSD >= 1 {
				priceStr = fmt.Sprintf("$%.2f", t.PriceUSD)
			} else {
				priceStr = fmt.Sprintf("$%.4f", t.PriceUSD)
			}
			b.WriteString(fmt.Sprintf("<code>%s %-5s %8s %+.1f%%</code>\n", arrow, t.Symbol, priceStr, t.PercentChange24h))
		}
	}

	// --- AAII Investor Sentiment Survey ---
	b.WriteString("\n<b>AAII Investor Sentiment Survey</b>\n")
	if data.AAIIAvailable {
		if data.AAIIWeekDate != "" {
			b.WriteString(fmt.Sprintf("<i>Minggu berakhir %s</i>\n", data.AAIIWeekDate))
		}
		b.WriteString(fmt.Sprintf("<code>Bullish : %5.1f%%</code>  %s\n", data.AAIIBullish, sentimentBar(data.AAIIBullish, "🟢")))
		b.WriteString(fmt.Sprintf("<code>Neutral : %5.1f%%</code>  %s\n", data.AAIINeutral, sentimentBar(data.AAIINeutral, "⚪")))
		b.WriteString(fmt.Sprintf("<code>Bearish : %5.1f%%</code>  %s\n", data.AAIIBearish, sentimentBar(data.AAIIBearish, "🔴")))
		b.WriteString(fmt.Sprintf("<code>Bull/Bear: %.2f</code>", data.AAIIBullBear))
		if data.AAIIBullBear > 0 {
			if data.AAIIBullBear >= 2.0 {
				b.WriteString("  — ⚠️ Optimisme tinggi")
			} else if data.AAIIBullBear <= 0.5 {
				b.WriteString("  — 🟢 Pesimisme dalam (contrarian bullish)")
			}
		}
		b.WriteString("\n")

		// Historical context: AAII long-term averages are ~37.5% bull, 31% bear, 31.5% neutral
		if data.AAIIBullish >= 50 {
			b.WriteString("<code>Catatan: Bullish jauh di atas rata-rata historis (~37.5%%)</code>\n")
		} else if data.AAIIBearish >= 50 {
			b.WriteString("<code>Catatan: Bearish jauh di atas rata-rata historis (~31%%)</code>\n")
		}
	} else {
		b.WriteString("<code>Data tidak tersedia — set FIRECRAWL_API_KEY untuk mengaktifkan</code>\n")
	}

	// --- AAII contrarian signal ---
	if data.AAIIAvailable {
		if data.AAIIBearish >= 50 {
			b.WriteString("<code>Signal: </code>🟢 <b>Contrarian BUY</b> — Bearish >50%% secara historis mendahului rally\n")
		} else if data.AAIIBullish >= 50 {
			b.WriteString("<code>Signal: </code>🔴 <b>Contrarian SELL</b> — Bullish >50%% secara historis mendahului koreksi\n")
		}
	}

	// --- CBOE Put/Call Ratios ---
	b.WriteString("\n<b>CBOE Put/Call Ratios</b>\n")
	if data.PutCallAvailable {
		b.WriteString(fmt.Sprintf("<code>Total P/C : %.2f</code>\n", data.PutCallTotal))
		if data.PutCallEquity > 0 {
			b.WriteString(fmt.Sprintf("<code>Equity P/C: %.2f</code>\n", data.PutCallEquity))
		}
		if data.PutCallIndex > 0 {
			b.WriteString(fmt.Sprintf("<code>Index P/C : %.2f</code>\n", data.PutCallIndex))
		}
		if data.PutCallSignal != "" {
			signalEmoji := "🟡"
			switch data.PutCallSignal {
			case "EXTREME FEAR":
				signalEmoji = "🟢"
			case "FEAR":
				signalEmoji = "🟢"
			case "EXTREME COMPLACENCY":
				signalEmoji = "🔴"
			case "COMPLACENCY":
				signalEmoji = "🟠"
			}
			b.WriteString(fmt.Sprintf("<code>Signal    : %s %s</code>\n", signalEmoji, data.PutCallSignal))
		}
		// Context interpretation
		if data.PutCallIndex > 0 && data.PutCallEquity > 0 {
			if data.PutCallIndex > 1.0 && data.PutCallEquity < 0.8 {
				b.WriteString("<i>Index P/C tinggi → institusi melakukan hedging. Equity P/C normal → retail belum panik.</i>\n")
			} else if data.PutCallTotal >= 1.2 {
				b.WriteString("<i>Pembelian put ekstrem di semua instrumen — sinyal contrarian bullish kuat.</i>\n")
			} else if data.PutCallTotal < 0.7 {
				b.WriteString("<i>Pembelian proteksi sangat rendah — peringatan complacency. Contrarian bearish.</i>\n")
			}
		}
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}

	// --- Myfxbook Retail Positioning ---
	b.WriteString("\n<b>Retail Positioning (Myfxbook)</b>\n")
	if data.MyfxbookAvailable && len(data.MyfxbookPairs) > 0 {
		for _, mp := range data.MyfxbookPairs {
			var signalEmoji string
			switch mp.Signal {
			case "CONTRARIAN_BULLISH":
				signalEmoji = "🟢"
			case "LEAN_BULLISH":
				signalEmoji = "🟢"
			case "CONTRARIAN_BEARISH":
				signalEmoji = "🔴"
			case "LEAN_BEARISH":
				signalEmoji = "🔴"
			default:
				signalEmoji = "⚪"
			}
			signalLabel := mp.Signal
			if signalLabel == "CONTRARIAN_BULLISH" {
				signalLabel = "Contrarian Bullish"
			} else if signalLabel == "CONTRARIAN_BEARISH" {
				signalLabel = "Contrarian Bearish"
			} else if signalLabel == "LEAN_BULLISH" {
				signalLabel = "Lean Bullish"
			} else if signalLabel == "LEAN_BEARISH" {
				signalLabel = "Lean Bearish"
			} else {
				signalLabel = "Netral"
			}
			b.WriteString(fmt.Sprintf("<code>%-8s: %4.1f%% L / %4.1f%% S</code> %s %s\n", mp.Symbol, mp.LongPct, mp.ShortPct, signalEmoji, signalLabel))
		}
		b.WriteString("<i>Retail positioning adalah indikator contrarian — pembacaan ekstrem mengindikasikan potensi reversal.</i>\n")
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}

	// --- VIX Term Structure (CBOE) ---
	b.WriteString("\n<b>VIX Term Structure</b>\n")
	if data.VIXAvailable {
		b.WriteString(fmt.Sprintf("<code>Spot  : %.2f</code>\n", data.VIXSpot))
		if data.VIXM1 > 0 {
			b.WriteString(fmt.Sprintf("<code>M1    : %.2f</code>\n", data.VIXM1))
		}
		if data.VIXM2 > 0 {
			b.WriteString(fmt.Sprintf("<code>M2    : %.2f</code>\n", data.VIXM2))
		}
		if data.VVIX > 0 {
			b.WriteString(fmt.Sprintf("<code>VVIX  : %.1f</code>\n", data.VVIX))
		}
		var structLabel, structEmoji string
		if data.VIXContango {
			structLabel = "CONTANGO"
			structEmoji = "✅"
		} else {
			structLabel = "BACKWARDATION"
			structEmoji = "🔴"
		}
		if data.VIXSlopePct != 0 {
			b.WriteString(fmt.Sprintf("<code>Shape : %s (%+.1f%%) %s</code>\n", structLabel, data.VIXSlopePct, structEmoji))
		} else {
			b.WriteString(fmt.Sprintf("<code>Shape : %s %s</code>\n", structLabel, structEmoji))
		}
		if data.VIXRegime != "" {
			var regimeEmoji string
			switch data.VIXRegime {
			case "EXTREME_FEAR":
				regimeEmoji = "😱"
			case "FEAR":
				regimeEmoji = "😟"
			case "ELEVATED":
				regimeEmoji = "⚠️"
			case "RISK_ON_NORMAL":
				regimeEmoji = "🟢"
			case "RISK_ON_COMPLACENT":
				regimeEmoji = "😏"
			default:
				regimeEmoji = "🟡"
			}
			b.WriteString(fmt.Sprintf("<code>Regime: %s %s</code>\n", data.VIXRegime, regimeEmoji))
		}
		switch data.VIXRegime {
		case "EXTREME_FEAR":
			b.WriteString("<i>VIX backwardation ekstrem — pasar panik, hedging demand tinggi. Historically contrarian bullish.</i>\n")
		case "FEAR":
			b.WriteString("<i>VIX backwardation — ketakutan jangka pendek tinggi, pasar memperhitungkan risiko dekat.</i>\n")
		case "RISK_ON_COMPLACENT":
			b.WriteString("<i>Steep contango — pasar complacent, VIX ETPs merugi. Bullish ekuitas tapi waspada pembalikan mendadak.</i>\n")
		}
		// --- MOVE Index (bond volatility) ---
		if data.MOVEAvailable {
			b.WriteString("\n<b>MOVE Index (Bond Vol)</b>\n")
			b.WriteString(fmt.Sprintf("<code>MOVE  : %.1f (%+.1f%%)</code>\n", data.MOVELevel, data.MOVEChangePct))
			if data.VIXMOVERatio > 0 {
				var ratioEmoji string
				switch {
				case data.VIXMOVERatio > 0.35:
					ratioEmoji = "📈" // equity vol elevated
				case data.VIXMOVERatio < 0.12:
					ratioEmoji = "📉" // bond vol elevated
				default:
					ratioEmoji = "↔️"
				}
				b.WriteString(fmt.Sprintf("<code>VIX/MOVE: %.3f %s</code>\n", data.VIXMOVERatio, ratioEmoji))
			}
			switch data.MOVEDivergence {
			case "EQUITY_FEAR":
				b.WriteString("<i>VIX tinggi vs MOVE rendah — ketakutan spesifik ekuitas, bukan sistemik.</i>\n")
			case "BOND_STRESS":
				b.WriteString("<i>MOVE tinggi vs VIX rendah — stres obligasi / risiko carry FX.</i>\n")
			case "SYSTEMIC_STRESS":
				b.WriteString("<i>VIX dan MOVE keduanya tinggi — stres sistemik luas.</i>\n")
			}
		}
	} else {
		b.WriteString("<code>Data tidak tersedia</code>\n")
	}


	// --- Deribit DVOL - Crypto Volatility Index ---
	if data.DVOLAvailable {
		b.WriteString("\n<b>Crypto Volatility (Deribit DVOL)</b>\n")

		formatDVOLCurrency := func(label string, current, change24hPct, high24h, low24h, hv, ivhvSpread, ivhvRatio float64, spike, available bool) {
			if !available {
				return
			}
			changeArrow := "\u2192"
			changeEmoji := ""
			if change24hPct > 5 {
				changeArrow = "\u2191"
				changeEmoji = "\U0001f534" // red circle for vol up
			} else if change24hPct < -5 {
				changeArrow = "\u2193"
				changeEmoji = "\U0001f7e2" // green circle for vol down
			}
			b.WriteString(fmt.Sprintf("<code>%s DVOL : %.1f%%  %s %+.1f%% %s</code>\n", label, current, changeArrow, change24hPct, changeEmoji))
			b.WriteString(fmt.Sprintf("<code>  24h   : %.1f - %.1f</code>\n", low24h, high24h))
			if hv > 0 {
				spreadLabel := dvol.SpreadSignal(ivhvRatio)
				b.WriteString(fmt.Sprintf("<code>  IV/HV : %.1f%% / %.1f%% (spread: %+.1f)</code>\n", current, hv, ivhvSpread))
				b.WriteString(fmt.Sprintf("<code>  Signal: %s</code>\n", spreadLabel))
			}
			if spike {
				b.WriteString(fmt.Sprintf("\u26a0\ufe0f <i>%s DVOL spike >20%% dalam 24h \u2014 volatility surge!</i>\n", label))
			}
		}

		formatDVOLCurrency("BTC", data.DVOLBTCCurrent, data.DVOLBTCChange24hPct, data.DVOLBTCHigh24h, data.DVOLBTCLow24h, data.DVOLBTCHV, data.DVOLBTCIVHVSpread, data.DVOLBTCIVHVRatio, data.DVOLBTCSpike, data.DVOLBTCAvailable)
		formatDVOLCurrency("ETH", data.DVOLETHCurrent, data.DVOLETHChange24hPct, data.DVOLETHHigh24h, data.DVOLETHLow24h, data.DVOLETHHV, data.DVOLETHIVHVSpread, data.DVOLETHIVHVRatio, data.DVOLETHSpike, data.DVOLETHAvailable)

		// Cross-asset vol comparison: DVOL vs CBOE VIX
		if data.DVOLBTCAvailable && data.VIXAvailable && data.VIXSpot > 0 {
			dvolVixRatio := data.DVOLBTCCurrent / data.VIXSpot
			b.WriteString(fmt.Sprintf("\n<code>BTC DVOL/VIX: %.1fx</code>", dvolVixRatio))
			if dvolVixRatio > 5 {
				b.WriteString(" \u2014 <i>Crypto vol jauh melebihi ekuitas</i>")
			} else if dvolVixRatio < 2 {
				b.WriteString(" \u2014 <i>Crypto vol relatif rendah vs ekuitas</i>")
			}
			b.WriteString("\n")
		}
	}

	// --- Cross-Asset Volatility Suite (CBOE) ---
	if data.VolSuiteAvail {
		b.WriteString("\n<b>Vol Suite (CBOE)</b>\n")
		if data.VolSKEW > 0 {
			var skewEmoji string
			switch {
			case data.VolSKEW > 140:
				skewEmoji = "🔴"
			case data.VolSKEW > 130:
				skewEmoji = "⚠️"
			default:
				skewEmoji = "✅"
			}
			b.WriteString(fmt.Sprintf("<code>SKEW  : %.1f %s</code>\n", data.VolSKEW, skewEmoji))
		}
		if data.VolOVX > 0 {
			b.WriteString(fmt.Sprintf("<code>OVX   : %.1f</code>\n", data.VolOVX))
		}
		if data.VolGVZ > 0 {
			b.WriteString(fmt.Sprintf("<code>GVZ   : %.1f</code>\n", data.VolGVZ))
		}
		if data.VolRVX > 0 {
			var rvxEmoji string
			if data.RVXVIXRatio > 1.3 {
				rvxEmoji = " ⚠️"
			}
			b.WriteString(fmt.Sprintf("<code>RVX   : %.1f%s</code>\n", data.VolRVX, rvxEmoji))
		}
		if data.VolVIX9D > 0 {
			var v9dEmoji string
			if data.VIX9D30Ratio > 1.1 {
				v9dEmoji = " ⚠️"
			}
			b.WriteString(fmt.Sprintf("<code>VIX9D : %.2f%s</code>\n", data.VolVIX9D, v9dEmoji))
		}
		if data.SKEWVIXRatio > 0 {
			var ratioEmoji string
			if data.SKEWVIXRatio > 8.0 {
				ratioEmoji = " 🔴"
			}
			pctStr := ""
			if data.SKEWVIXPctile > 0 {
				pctStr = fmt.Sprintf(" (P%.0f)", data.SKEWVIXPctile)
			}
			b.WriteString(fmt.Sprintf("<code>SKEW/VIX: %.1f%s%s</code>\n", data.SKEWVIXRatio, pctStr, ratioEmoji))
		}
		if data.SKEWPctile > 0 && data.VolSKEW > 0 {
			b.WriteString(fmt.Sprintf("<code>SKEW Pct: P%.0f</code>\n", data.SKEWPctile))
		}
		if data.RVXVIXRatio > 0 {
			b.WriteString(fmt.Sprintf("<code>RVX/VIX : %.2f</code>\n", data.RVXVIXRatio))
		}
		switch data.VolTailRisk {
		case "EXTREME":
			b.WriteString("<i>🔴 TAIL RISK EXTREME — SKEW/VIX historically dangerous.</i>\n")
			if data.TailRiskCtx != "" {
				b.WriteString(fmt.Sprintf("<i>%s</i>\n", data.TailRiskCtx))
			}
		case "ELEVATED":
			b.WriteString("<i>⚠️ Tail risk elevated — SKEW tinggi vs VIX rendah.</i>\n")
			if data.TailRiskCtx != "" {
				b.WriteString(fmt.Sprintf("<i>%s</i>\n", data.TailRiskCtx))
			}
		}
		for _, d := range data.VolDivergences {
			b.WriteString(fmt.Sprintf("<i>📊 %s</i>\n", d))
		}
	}

	// --- Composite reading ---
	b.WriteString("\n<b>Pembacaan Gabungan</b>\n")
	compositeWritten := false

	// Cross-source agreement amplifies the signal
	if data.CNNAvailable && data.AAIIAvailable {
		cnnFear := data.CNNFearGreed <= 25
		cnnGreed := data.CNNFearGreed >= 75
		aaiiFear := data.AAIIBearish >= 50
		aaiiGreed := data.AAIIBullish >= 50

		if cnnFear && aaiiFear {
			b.WriteString("🟢 <b>STRONG CONTRARIAN BUY</b>\n")
			b.WriteString("<i>Kedua sumber menunjukkan ketakutan ekstrem — secara historis ini sinyal beli yang kuat. Pasar sering rebound dari level ini.</i>\n")
			compositeWritten = true
		} else if cnnGreed && aaiiGreed {
			b.WriteString("🔴 <b>STRONG CONTRARIAN SELL</b>\n")
			b.WriteString("<i>Kedua sumber menunjukkan keserakahan ekstrem — waspada koreksi. Euforia berlebihan jarang bertahan lama.</i>\n")
			compositeWritten = true
		} else if (cnnFear && !aaiiFear) || (!cnnFear && aaiiFear) {
			b.WriteString("🟡 <b>MIXED FEAR</b>\n")
			b.WriteString("<i>Hanya salah satu sumber menunjukkan fear ekstrem — sinyal belum sekuat jika keduanya sepakat.</i>\n")
			compositeWritten = true
		} else if (cnnGreed && !aaiiGreed) || (!cnnGreed && aaiiGreed) {
			b.WriteString("🟡 <b>MIXED GREED</b>\n")
			b.WriteString("<i>Hanya salah satu sumber menunjukkan greed ekstrem — belum cukup kuat untuk sinyal jual.</i>\n")
			compositeWritten = true
		}
	}

	if !compositeWritten {
		b.WriteString("<i>Sentiment survey adalah indikator contrarian.\n")
		b.WriteString("Pembacaan ekstrem sering menandai titik balik.</i>\n")
	}

	// --- Regime context ---
	if macroRegime != "" {
		b.WriteString(fmt.Sprintf("\n<b>Konteks Regime: %s</b>\n", macroRegime))
		sentimentFearish := (data.CNNAvailable && data.CNNFearGreed <= 35) || (data.AAIIAvailable && data.AAIIBearish >= 45)
		sentimentGreedish := (data.CNNAvailable && data.CNNFearGreed >= 65) || (data.AAIIAvailable && data.AAIIBullish >= 45)

		switch {
		case sentimentFearish && (macroRegime == "GOLDILOCKS" || macroRegime == "DISINFLATIONARY"):
			b.WriteString("<i>Fear di tengah ekonomi yang sehat — peluang beli lebih kredibel.</i>\n")
		case sentimentFearish && (macroRegime == "RECESSION" || macroRegime == "STRESS"):
			b.WriteString("<i>Fear di tengah tekanan makro nyata — ketakutan mungkin memang tepat. Hati-hati mengandalkan sinyal contrarian.</i>\n")
		case sentimentGreedish && (macroRegime == "RECESSION" || macroRegime == "STRESS"):
			b.WriteString("<i>Greed di tengah resesi/stress — disconnected dari fundamental. Risiko koreksi tinggi.</i>\n")
		case sentimentGreedish && macroRegime == "GOLDILOCKS":
			b.WriteString("<i>Greed di kondisi ideal — wajar tapi tetap waspada jika sudah terlalu jauh.</i>\n")
		default:
			b.WriteString("<i>Sentiment sejalan dengan kondisi makro saat ini — tidak ada divergensi signifikan.</i>\n")
		}
	}

	return b.String()
}

