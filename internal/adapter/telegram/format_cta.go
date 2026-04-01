package telegram

// format_cta.go — CTA formatting functions (extracted from handler_cta.go)
// Pure formatting: no handler logic, no side effects.

import (
	"fmt"
	"html"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

func formatCTASummary(state *ctaState) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<b>⚡ Technical Analysis: %s</b>\n", html.EscapeString(state.symbol)))
	sb.WriteString(fmt.Sprintf("📅 <i>%s</i>\n\n", state.computedAt.UTC().Format("02 Jan 2006 15:04 UTC")))

	d := state.daily
	if d == nil || d.Confluence == nil {
		sb.WriteString("⚠️ Data tidak cukup untuk analisis.\n")
		return sb.String()
	}

	conf := d.Confluence

	// Decision with grade-aware language
	dirEmoji := "🔵"
	dirLabel := conf.Direction
	if conf.Direction == "BULLISH" {
		dirEmoji = "🟢"
	} else if conf.Direction == "BEARISH" {
		dirEmoji = "🔴"
	}

	sb.WriteString(fmt.Sprintf("🎯 <b>KEPUTUSAN: %s %s</b> (Skor: %+.0f/100, Grade: %s)\n",
		dirEmoji, dirLabel, conf.Score, conf.Grade))

	// Plain-language grade explanation
	switch conf.Grade {
	case "A":
		sb.WriteString("<i>Sinyal sangat kuat — kepercayaan tinggi.</i>\n")
	case "B":
		sb.WriteString("<i>Sinyal cukup kuat — bisa dijadikan acuan.</i>\n")
	case "C":
		sb.WriteString("<i>Sinyal moderate — perlu konfirmasi tambahan.</i>\n")
	case "D":
		sb.WriteString("<i>⚠️ Sinyal sangat lemah — belum ada arah yang jelas. Hindari entry.</i>\n")
	default:
		sb.WriteString("<i>⚠️ Tidak ada sinyal berarti — pasar belum menunjukkan arah.</i>\n")
	}

	sb.WriteString(fmt.Sprintf("Confluence: %d/%d indikator %s\n\n",
		conf.BullishCount, conf.TotalIndicators, strings.ToLower(dirLabel)))

	// Summary
	sb.WriteString("📊 <b>Ringkasan Teknikal:</b>\n")
	snap := d.Snapshot

	// Trend
	if snap != nil && snap.EMA != nil {
		trendNote := "Mixed — belum ada arah tren yang jelas"
		if snap.EMA.RibbonAlignment == "BULLISH" {
			trendNote = "Bullish — semua EMA tersusun naik (harga di atas moving average)"
		} else if snap.EMA.RibbonAlignment == "BEARISH" {
			trendNote = "Bearish — semua EMA tersusun turun (harga di bawah moving average)"
		}
		if snap.SuperTrend != nil {
			if snap.SuperTrend.Direction == "UP" {
				trendNote += "\n  ↳ SuperTrend: ✅ Bullish — harga di atas support dinamis"
			} else {
				trendNote += "\n  ↳ SuperTrend: ❌ Bearish — harga di bawah resistance dinamis"
			}
		}
		sb.WriteString(fmt.Sprintf("• <b>Tren:</b> %s\n", trendNote))
	}

	// Momentum
	if snap != nil && snap.RSI != nil {
		rsiVal := snap.RSI.Value
		rsiNote := fmt.Sprintf("RSI %.0f", rsiVal)
		if snap.RSI.Zone == "OVERBOUGHT" {
			rsiNote += " — ⚠️ jenuh beli (overbought), hati-hati momentum bisa melambat"
		} else if snap.RSI.Zone == "OVERSOLD" {
			rsiNote += " — 💡 jenuh jual (oversold), potensi rebound"
		} else if rsiVal >= 55 {
			rsiNote += " — momentum cenderung bullish"
		} else if rsiVal <= 45 {
			rsiNote += " — momentum cenderung bearish"
		} else {
			rsiNote += " — netral, belum ada momentum kuat"
		}

		macdNote := ""
		if snap.MACD != nil {
			if snap.MACD.BullishCross {
				macdNote = "\n  ↳ MACD baru saja bullish cross — sinyal momentum naik baru dimulai ✅"
			} else if snap.MACD.BearishCross {
				macdNote = "\n  ↳ MACD baru saja bearish cross — sinyal momentum turun baru dimulai ❌"
			} else if snap.MACD.Histogram > 0 {
				macdNote = "\n  ↳ MACD histogram positif — momentum masih mendukung kenaikan"
			} else if snap.MACD.Histogram < 0 {
				macdNote = "\n  ↳ MACD histogram negatif — momentum masih mendukung penurunan"
			}
		}
		sb.WriteString(fmt.Sprintf("• <b>Momentum:</b> %s%s\n", rsiNote, macdNote))
	}

	// Volume
	if snap != nil && snap.OBV != nil {
		obvNote := "flat — volume tidak menunjukkan arah"
		if snap.OBV.Trend == "RISING" {
			obvNote = "naik — volume mendukung pergerakan harga (konfirmasi tren)"
		} else if snap.OBV.Trend == "FALLING" {
			obvNote = "turun — volume melemah (potensi divergensi)"
		}
		sb.WriteString(fmt.Sprintf("• <b>Volume:</b> OBV %s\n", obvNote))
	}

	// Volatility
	if snap != nil && snap.Bollinger != nil {
		bbNote := fmt.Sprintf("BB %%B=%.2f", snap.Bollinger.PercentB)
		if snap.Bollinger.Squeeze {
			bbNote += " — 💥 squeeze terdeteksi! Harga terkompresi, potensi breakout besar"
		} else if snap.Bollinger.PercentB > 0.8 {
			bbNote += " — harga mendekati band atas (potensi jenuh)"
		} else if snap.Bollinger.PercentB < 0.2 {
			bbNote += " — harga mendekati band bawah (potensi pantulan)"
		} else {
			bbNote += " — volatilitas normal"
		}
		sb.WriteString(fmt.Sprintf("• <b>Volatilitas:</b> %s\n", bbNote))
	}

	sb.WriteString("\n")

	// Ichimoku
	if snap != nil && snap.Ichimoku != nil {
		ich := snap.Ichimoku
		ichNote := ""
		switch ich.Overall {
		case "STRONG_BULLISH":
			ichNote = "Semua sinyal Ichimoku sejajar bullish — tren naik sangat kuat"
		case "BULLISH":
			ichNote = "Mayoritas sinyal Ichimoku bullish — tren cenderung naik"
		case "STRONG_BEARISH":
			ichNote = "Semua sinyal Ichimoku sejajar bearish — tren turun sangat kuat"
		case "BEARISH":
			ichNote = "Mayoritas sinyal Ichimoku bearish — tren cenderung turun"
		default:
			ichNote = "Sinyal campur — pasar belum menunjukkan arah jelas"
		}
		sb.WriteString(fmt.Sprintf("🏯 <b>Ichimoku:</b> %s (%s)\n", ich.Overall, ichNote))
		if ich.KumoBreakout == "BULLISH_BREAKOUT" {
			sb.WriteString("  ↳ Harga di atas awan (cloud) — zona bullish ✅\n")
		} else if ich.KumoBreakout == "BEARISH_BREAKOUT" {
			sb.WriteString("  ↳ Harga di bawah awan (cloud) — zona bearish ❌\n")
		} else if ich.KumoBreakout == "INSIDE_CLOUD" {
			sb.WriteString("  ↳ Harga di dalam awan — zona netral/transisi ⚠️\n")
		}
	}

	// Fibonacci
	if snap != nil && snap.Fibonacci != nil {
		fib := snap.Fibonacci
		sb.WriteString(fmt.Sprintf("📐 <b>Fibonacci:</b> Level terdekat %s%% di %.4f\n", fib.NearestLevel, fib.NearestPrice))
		sb.WriteString("  ↳ <i>Level Fibonacci = area support/resistance kunci berdasarkan swing harga</i>\n")
	}

	// Patterns
	if len(d.Patterns) > 0 {
		p := d.Patterns[0]
		stars := strings.Repeat("★", p.Reliability) + strings.Repeat("☆", 3-p.Reliability)
		patDir := "netral"
		if p.Direction == "BULLISH" {
			patDir = "potensi naik"
		} else if p.Direction == "BEARISH" {
			patDir = "potensi turun"
		}
		sb.WriteString(fmt.Sprintf("🕯 <b>Pola:</b> %s %s — %s\n", p.Name, stars, patDir))
	}

	// Divergences
	if len(d.Divergences) > 0 {
		div := d.Divergences[0]
		divNote := ""
		switch div.Type {
		case "REGULAR_BULLISH":
			divNote = "harga turun tapi indikator naik — potensi reversal naik 🔄"
		case "REGULAR_BEARISH":
			divNote = "harga naik tapi indikator turun — potensi reversal turun 🔄"
		case "HIDDEN_BULLISH":
			divNote = "sinyal kelanjutan tren naik"
		case "HIDDEN_BEARISH":
			divNote = "sinyal kelanjutan tren turun"
		}
		sb.WriteString(fmt.Sprintf("⚡ <b>Divergence:</b> %s (%s) — %s\n", div.Type, div.Indicator, divNote))
	}

	sb.WriteString("\n")

	// Zones — with safeguard messaging
	if d.Zones != nil && d.Zones.Valid {
		z := d.Zones
		confLabel := ""
		switch z.Confidence {
		case "HIGH":
			confLabel = "Kepercayaan: ✅ Tinggi"
		case "MEDIUM":
			confLabel = "Kepercayaan: 🟡 Sedang"
		default:
			confLabel = "Kepercayaan: ⚠️ Rendah — gunakan dengan hati-hati"
		}
		sb.WriteString(fmt.Sprintf("🎯 <b>Setup %s</b> (%s)\n", z.Direction, confLabel))
		sb.WriteString(fmt.Sprintf("  Entry: %.4f – %.4f\n", z.EntryLow, z.EntryHigh))
		sb.WriteString(fmt.Sprintf("  🛑 Stop Loss: %.4f\n", z.StopLoss))
		sb.WriteString(fmt.Sprintf("  ✅ TP1: %.4f (R:R 1:%.1f) | TP2: %.4f (R:R 1:%.1f)\n",
			z.TakeProfit1, z.RiskReward1, z.TakeProfit2, z.RiskReward2))
		sb.WriteString("<i>  ↳ R:R = rasio potensi profit vs risiko. Semakin tinggi semakin baik.</i>\n")
		sb.WriteString("\n")
	} else if d.Zones != nil && !d.Zones.Valid {
		sb.WriteString("🎯 <b>Setup:</b> ❌ Belum ada setup valid\n")
		sb.WriteString(fmt.Sprintf("  <i>%s</i>\n\n", html.EscapeString(d.Zones.Reasoning)))
	}

	// MTF
	if state.mtf != nil && len(state.mtf.Matrix) > 0 {
		sb.WriteString("📊 <b>Multi-Timeframe:</b>\n")
		var mtfParts []string
		for _, row := range state.mtf.Matrix {
			dirE := "⚪"
			if row.Direction == "BULLISH" {
				dirE = "🟢"
			} else if row.Direction == "BEARISH" {
				dirE = "🔴"
			}
			mtfParts = append(mtfParts, fmt.Sprintf("%s%s %s", dirE, row.Timeframe, row.Grade))
		}
		sb.WriteString(strings.Join(mtfParts, " | "))

		mtfNote := ""
		switch state.mtf.Alignment {
		case "STRONG_BULLISH":
			mtfNote = "semua timeframe sejajar bullish — sinyal sangat kuat"
		case "BULLISH":
			mtfNote = "mayoritas timeframe bullish"
		case "STRONG_BEARISH":
			mtfNote = "semua timeframe sejajar bearish — sinyal sangat kuat"
		case "BEARISH":
			mtfNote = "mayoritas timeframe bearish"
		default:
			mtfNote = "sinyal campur antar timeframe — berhati-hati"
		}
		sb.WriteString(fmt.Sprintf("\n<i>%s (Skor MTF: %+.0f, Grade %s)</i>",
			mtfNote, state.mtf.WeightedScore, state.mtf.WeightedGrade))
	}

	// ICT summary — nearest FVG and Order Block from daily
	if d.ICT != nil {
		sb.WriteString("\n🔷 <b>ICT Key Levels:</b>\n")
		// Nearest unfilled FVG
		nearestFVG := (*ta.FVG)(nil)
		for i := range d.ICT.FairValueGaps {
			if !d.ICT.FairValueGaps[i].Filled {
				nearestFVG = &d.ICT.FairValueGaps[i]
				break
			}
		}
		if nearestFVG != nil {
			fvgDir := "🟢 Bullish"
			if nearestFVG.Type == "BEARISH" {
				fvgDir = "🔴 Bearish"
			}
			fillNote := ""
			if nearestFVG.FillPct > 0 {
				fillNote = fmt.Sprintf(" (%.0f%% terisi)", nearestFVG.FillPct)
			}
			sb.WriteString(fmt.Sprintf("• FVG %s: <code>%.5f – %.5f</code>%s\n",
				fvgDir, nearestFVG.Low, nearestFVG.High, fillNote))
		}
		// Nearest active Order Block
		nearestOB := (*ta.OrderBlock)(nil)
		for i := range d.ICT.OrderBlocks {
			if !d.ICT.OrderBlocks[i].Broken {
				nearestOB = &d.ICT.OrderBlocks[i]
				break
			}
		}
		if nearestOB != nil {
			obDir := "🟢 Demand"
			if nearestOB.Type == "BEARISH" {
				obDir = "🔴 Supply"
			}
			mitigatedNote := ""
			if nearestOB.Mitigated {
				mitigatedNote = " ⚡mitigated"
			}
			sb.WriteString(fmt.Sprintf("• OB %s: <code>%.5f – %.5f</code> (str:%d)%s\n",
				obDir, nearestOB.Low, nearestOB.High, nearestOB.Strength, mitigatedNote))
		}
		if nearestFVG == nil && nearestOB == nil {
			sb.WriteString("<i>Tidak ada FVG/OB aktif terdeteksi.</i>\n")
		}
		// Killzone
		if d.ICT.Killzone != "" && d.ICT.Killzone != "OFF" {
			sb.WriteString(fmt.Sprintf("• Killzone: 🕐 <b>%s</b> (sesi aktif)\n", d.ICT.Killzone))
		}
		// Premium / Discount zone
		if d.ICT.PremiumZone {
			sb.WriteString(fmt.Sprintf("• Zone: 📈 Premium (harga di atas equilibrium %.5f)\n", d.ICT.Equilibrium))
		} else if d.ICT.DiscountZone {
			sb.WriteString(fmt.Sprintf("• Zone: 📉 Discount (harga di bawah equilibrium %.5f)\n", d.ICT.Equilibrium))
		}
	}

	// Disclaimer
	sb.WriteString("\n\n<i>⚠️ Ini analisis teknikal otomatis, bukan saran keuangan. Selalu kelola risiko Anda.</i>")

	return sb.String()
}

func formatCTAIchimoku(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>🏯 Ichimoku Cloud Detail</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Ichimoku = sistem tren lengkap: mengukur momentum, support/resistance, dan arah tren sekaligus.</i>\n\n")

	for _, entry := range []struct {
		tf     string
		result *ta.FullResult
	}{
		{"Daily", state.daily},
		{"4H", state.h4},
		{"1H", state.h1},
		{"15m", state.m15},
	} {
		if entry.result == nil || entry.result.Snapshot == nil || entry.result.Snapshot.Ichimoku == nil {
			continue
		}
		ich := entry.result.Snapshot.Ichimoku

		overallEmoji := "⚪"
		switch ich.Overall {
		case "STRONG_BULLISH":
			overallEmoji = "🟢🟢"
		case "BULLISH":
			overallEmoji = "🟢"
		case "STRONG_BEARISH":
			overallEmoji = "🔴🔴"
		case "BEARISH":
			overallEmoji = "🔴"
		}

		sb.WriteString(fmt.Sprintf("<b>%s: %s %s</b>\n", entry.tf, overallEmoji, ich.Overall))
		sb.WriteString(fmt.Sprintf("  Tenkan: %.4f | Kijun: %.4f\n", ich.Tenkan, ich.Kijun))

		// TK Cross explanation
		switch ich.TKCross {
		case "BULLISH_CROSS":
			sb.WriteString("  TK Cross: ✅ Bullish — <i>garis cepat memotong ke atas garis lambat (sinyal beli)</i>\n")
		case "BEARISH_CROSS":
			sb.WriteString("  TK Cross: ❌ Bearish — <i>garis cepat memotong ke bawah garis lambat (sinyal jual)</i>\n")
		default:
			sb.WriteString("  TK Cross: Tidak ada\n")
		}

		// Kumo explanation
		switch ich.KumoBreakout {
		case "BULLISH_BREAKOUT":
			sb.WriteString("  Kumo: ✅ Di atas awan — <i>harga berada di zona bullish, awan menjadi support</i>\n")
		case "BEARISH_BREAKOUT":
			sb.WriteString("  Kumo: ❌ Di bawah awan — <i>harga berada di zona bearish, awan menjadi resistance</i>\n")
		case "INSIDE_CLOUD":
			sb.WriteString("  Kumo: ⚠️ Di dalam awan — <i>zona transisi/netral, arah belum jelas</i>\n")
		default:
			sb.WriteString(fmt.Sprintf("  Kumo: %s\n", ich.KumoBreakout))
		}

		sb.WriteString(fmt.Sprintf("  Cloud: %s | Chikou: %s\n\n", ich.CloudColor, ich.ChikouSignal))
	}

	return sb.String()
}

func formatCTAFibonacci(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>📐 Fibonacci Retracement</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Level Fibonacci = area support/resistance kunci. Harga sering berbalik arah di level 38.2%, 50%, atau 61.8%.</i>\n\n")

	for _, entry := range []struct {
		tf     string
		result *ta.FullResult
	}{
		{"Daily", state.daily},
		{"4H", state.h4},
	} {
		if entry.result == nil || entry.result.Snapshot == nil || entry.result.Snapshot.Fibonacci == nil {
			continue
		}
		fib := entry.result.Snapshot.Fibonacci

		trendLabel := "⬆️ Uptrend"
		if fib.TrendDir == "DOWN" {
			trendLabel = "⬇️ Downtrend"
		}

		sb.WriteString(fmt.Sprintf("<b>%s:</b> %s\n", entry.tf, trendLabel))
		sb.WriteString(fmt.Sprintf("  Swing High: %.4f | Swing Low: %.4f\n", fib.SwingHigh, fib.SwingLow))
		sb.WriteString(fmt.Sprintf("  Level terdekat: <b>%s%%</b> = %.4f\n", fib.NearestLevel, fib.NearestPrice))

		for _, lvl := range []string{"23.6", "38.2", "50", "61.8", "78.6"} {
			if val, ok := fib.Levels[lvl]; ok {
				marker := ""
				if lvl == fib.NearestLevel {
					marker = " ◀ <b>terdekat</b>"
				}
				// Add contextual notes for key levels
				note := ""
				switch lvl {
				case "38.2":
					note = " <i>(shallow retracement)</i>"
				case "50":
					note = " <i>(mid-level kunci)</i>"
				case "61.8":
					note = " <i>(golden ratio — level terpenting)</i>"
				}
				sb.WriteString(fmt.Sprintf("  %s%%: %.4f%s%s\n", lvl, val, marker, note))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatCTAPatterns(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>🕯 Pola Candlestick</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Pola candlestick = formasi harga yang mengindikasikan potensi pergerakan selanjutnya. ★ = tingkat keandalan.</i>\n\n")

	found := false
	for _, entry := range []struct {
		tf     string
		result *ta.FullResult
	}{
		{"Daily", state.daily},
		{"4H", state.h4},
		{"1H", state.h1},
		{"15m", state.m15},
	} {
		if entry.result == nil || len(entry.result.Patterns) == 0 {
			continue
		}
		found = true
		sb.WriteString(fmt.Sprintf("<b>%s:</b>\n", entry.tf))
		for _, p := range entry.result.Patterns {
			dirEmoji := "⚪"
			if p.Direction == "BULLISH" {
				dirEmoji = "🟢"
			} else if p.Direction == "BEARISH" {
				dirEmoji = "🔴"
			}
			stars := strings.Repeat("★", p.Reliability) + strings.Repeat("☆", 3-p.Reliability)
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b> %s\n", dirEmoji, p.Name, stars))
			sb.WriteString(fmt.Sprintf("    <i>%s</i>\n", p.Description))
		}
		sb.WriteString("\n")
	}

	if !found {
		sb.WriteString("Tidak ada pola candlestick terdeteksi saat ini.\n")
		sb.WriteString("<i>Ini normal — pola hanya muncul pada kondisi pasar tertentu.</i>\n")
	}

	return sb.String()
}

func formatCTAConfluence(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>⚡ Detail Confluence</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s — Daily</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Confluence = skor gabungan dari semua indikator. Semakin banyak yang sejajar, semakin kuat sinyal.</i>\n\n")

	if state.daily == nil || state.daily.Confluence == nil {
		sb.WriteString("⚠️ Data confluence belum tersedia.\n")
		return sb.String()
	}

	conf := state.daily.Confluence
	dirEmoji := "⚪"
	if conf.Direction == "BULLISH" {
		dirEmoji = "🟢"
	} else if conf.Direction == "BEARISH" {
		dirEmoji = "🔴"
	}

	sb.WriteString(fmt.Sprintf("%s Skor: <b>%+.1f</b> | Grade: <b>%s</b> | Arah: <b>%s</b>\n",
		dirEmoji, conf.Score, conf.Grade, conf.Direction))
	sb.WriteString(fmt.Sprintf("Bullish: %d | Bearish: %d | Netral: %d dari %d indikator\n\n",
		conf.BullishCount, conf.BearishCount, conf.NeutralCount, conf.TotalIndicators))

	sb.WriteString("<b>Sinyal per-indikator:</b>\n")
	for _, sig := range conf.Signals {
		signalEmoji := "⚪"
		if sig.Value > 0.05 {
			signalEmoji = "🟢"
		} else if sig.Value < -0.05 {
			signalEmoji = "🔴"
		}
		sb.WriteString(fmt.Sprintf("  %s <b>%s</b>: %+.2f (bobot: %.0f%%)\n",
			signalEmoji, sig.Indicator, sig.Value, sig.Weight*100))
		if sig.Note != "" {
			sb.WriteString(fmt.Sprintf("    <i>%s</i>\n", html.EscapeString(sig.Note)))
		}
	}

	// Divergences
	if state.daily.Divergences != nil && len(state.daily.Divergences) > 0 {
		sb.WriteString("\n<b>Divergence:</b>\n")
		for _, div := range state.daily.Divergences {
			divEmoji := "🔄"
			if strings.Contains(div.Type, "BULLISH") {
				divEmoji = "🟢🔄"
			} else if strings.Contains(div.Type, "BEARISH") {
				divEmoji = "🔴🔄"
			}
			sb.WriteString(fmt.Sprintf("  %s %s (%s) — kekuatan: %.0f%%\n",
				divEmoji, div.Type, div.Indicator, div.Strength*100))
			sb.WriteString(fmt.Sprintf("    <i>%s</i>\n", div.Description))
		}
	}

	return sb.String()
}

func formatCTAMTF(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>📊 Multi-Timeframe Matrix</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>MTF = membandingkan sinyal dari berbagai timeframe. Jika semua sejajar, sinyal lebih kuat.</i>\n\n")

	if state.mtf == nil || len(state.mtf.Matrix) == 0 {
		sb.WriteString("⚠️ Data MTF belum tersedia.\n")
		return sb.String()
	}

	// Alignment with explanation
	alignEmoji := "⚪"
	alignNote := ""
	switch state.mtf.Alignment {
	case "STRONG_BULLISH":
		alignEmoji = "🟢🟢"
		alignNote = "Semua timeframe bullish — kepercayaan sangat tinggi"
	case "BULLISH":
		alignEmoji = "🟢"
		alignNote = "Mayoritas timeframe bullish"
	case "STRONG_BEARISH":
		alignEmoji = "🔴🔴"
		alignNote = "Semua timeframe bearish — kepercayaan sangat tinggi"
	case "BEARISH":
		alignEmoji = "🔴"
		alignNote = "Mayoritas timeframe bearish"
	default:
		alignEmoji = "🟡"
		alignNote = "Sinyal campur — berhati-hati, arah belum jelas"
	}

	sb.WriteString(fmt.Sprintf("%s Alignment: <b>%s</b>\n", alignEmoji, state.mtf.Alignment))
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", alignNote))
	sb.WriteString(fmt.Sprintf("Skor Tertimbang: <b>%+.1f</b> (Grade %s)\n\n", state.mtf.WeightedScore, state.mtf.WeightedGrade))

	// Matrix as clean list (better for mobile than <code> table)
	for _, row := range state.mtf.Matrix {
		dirEmoji := "⚪"
		if row.Direction == "BULLISH" {
			dirEmoji = "🟢"
		} else if row.Direction == "BEARISH" {
			dirEmoji = "🔴"
		}
		weightStr := ""
		if row.Weight > 0 {
			weightStr = fmt.Sprintf(" (bobot: %.0f%%)", row.Weight*100)
		}
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>: %s Skor %+.0f Grade %s%s\n",
			dirEmoji, row.Timeframe, row.Direction, row.Score, row.Grade, weightStr))
	}

	return sb.String()
}

func formatCTAZones(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>🎯 Entry/Exit Zones</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>Zone = area masuk/keluar yang dihitung dari level support, resistance, dan ATR.</i>\n\n")

	for _, entry := range []struct {
		tf     string
		result *ta.FullResult
	}{
		{"Daily", state.daily},
		{"4H", state.h4},
	} {
		if entry.result == nil || entry.result.Zones == nil {
			continue
		}
		z := entry.result.Zones
		sb.WriteString(fmt.Sprintf("<b>%s:</b>\n", entry.tf))
		if !z.Valid {
			sb.WriteString(fmt.Sprintf("  ❌ Tidak ada setup valid.\n"))
			sb.WriteString(fmt.Sprintf("  <i>%s</i>\n\n", html.EscapeString(z.Reasoning)))
			continue
		}

		dirEmoji := "🟢"
		dirNote := "BUY (masuk posisi beli)"
		if z.Direction == "SHORT" {
			dirEmoji = "🔴"
			dirNote = "SELL (masuk posisi jual)"
		}

		confLabel := ""
		switch z.Confidence {
		case "HIGH":
			confLabel = "✅ Tinggi"
		case "MEDIUM":
			confLabel = "🟡 Sedang"
		default:
			confLabel = "⚠️ Rendah"
		}

		sb.WriteString(fmt.Sprintf("  %s <b>%s</b> — %s\n", dirEmoji, z.Direction, dirNote))
		sb.WriteString(fmt.Sprintf("  Kepercayaan: %s\n", confLabel))
		sb.WriteString(fmt.Sprintf("  📍 Entry: %.4f – %.4f\n", z.EntryLow, z.EntryHigh))
		sb.WriteString(fmt.Sprintf("  🛑 Stop Loss: %.4f\n", z.StopLoss))
		sb.WriteString(fmt.Sprintf("  ✅ TP1: %.4f (R:R 1:%.1f)\n", z.TakeProfit1, z.RiskReward1))
		sb.WriteString(fmt.Sprintf("  ✅ TP2: %.4f (R:R 1:%.1f)\n", z.TakeProfit2, z.RiskReward2))

		if z.Confidence == "LOW" {
			sb.WriteString("  <i>⚠️ Setup berkepercayaan rendah — pertimbangkan menunggu konfirmasi lebih lanjut.</i>\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("<i>R:R = Risk:Reward. Contoh 1:2.0 = setiap $1 risiko, potensi profit $2.</i>\n")

	return sb.String()
}

// formatCTAVWAPDelta formats VWAP + estimated delta analysis for /cta output.
func formatCTAVWAPDelta(state *ctaState) string {
	var sb strings.Builder
	sb.WriteString("<b>📏 VWAP + Estimated Delta</b>\n")
	sb.WriteString(fmt.Sprintf("<i>%s — Daily</i>\n", html.EscapeString(state.symbol)))
	sb.WriteString("<i>VWAP = price level weighted by volume. Delta = estimated buy/sell pressure via tick rule.</i>\n\n")

	if state.daily == nil || state.daily.Snapshot == nil {
		sb.WriteString("⚠️ Data tidak cukup untuk VWAP/Delta.\n")
		return sb.String()
	}

	snap := state.daily.Snapshot

	// --- VWAP Section ---
	if snap.VWAP == nil {
		sb.WriteString("📏 <b>VWAP:</b> Data volume tidak tersedia.\n")
	} else {
		sb.WriteString("📏 <b>VWAP (Anchored)</b>\n")

		vwapLine := func(label string, r *ta.VWAPResult) {
			if r == nil {
				return
			}
			posEmoji := "⚪"
			switch r.Position {
			case "ABOVE":
				posEmoji = "🟢"
			case "BELOW":
				posEmoji = "🔴"
			}
			sb.WriteString(fmt.Sprintf("  %s <b>%s:</b> %.5f (%s, %.1fσ)\n",
				posEmoji, label, r.VWAP, r.Position, r.Deviation))
			sb.WriteString(fmt.Sprintf("     Bands ±1σ: [%.5f – %.5f]\n",
				r.Band1Lower, r.Band1Upper))
			sb.WriteString(fmt.Sprintf("     Bands ±2σ: [%.5f – %.5f]\n",
				r.Band2Lower, r.Band2Upper))
		}

		vwapLine("Daily", snap.VWAP.Daily)
		vwapLine("Weekly", snap.VWAP.Weekly)
		if snap.VWAP.SwingLow != nil {
			vwapLine("Swing Low", snap.VWAP.SwingLow)
		}
		if snap.VWAP.SwingHigh != nil {
			vwapLine("Swing High", snap.VWAP.SwingHigh)
		}
	}

	sb.WriteString("\n")

	// --- Delta Section ---
	if snap.Delta == nil {
		sb.WriteString("📈 <b>Delta:</b> Data tidak cukup.\n")
	} else {
		d := snap.Delta

		biasEmoji := "⚪"
		switch d.Bias {
		case "BUYING_PRESSURE":
			biasEmoji = "🟢"
		case "SELLING_PRESSURE":
			biasEmoji = "🔴"
		}

		sb.WriteString("📈 <b>Estimated Delta (Tick Rule)</b>\n")
		sb.WriteString(fmt.Sprintf("  %s <b>Bias:</b> %s (strength: %.0f%%)\n",
			biasEmoji, d.Bias, d.BiasStrength*100))
		sb.WriteString(fmt.Sprintf("  Cumulative Delta: %+.0f\n", d.CumulativeDelta))
		sb.WriteString(fmt.Sprintf("  Current Bar Delta: %+.0f\n", d.CurrentDelta))

		if d.DeltaDivergence != "NONE" {
			divEmoji := "⚠️"
			sb.WriteString(fmt.Sprintf("  %s <b>Divergence:</b> %s\n", divEmoji, d.DeltaDivergence))
			if d.DeltaDivergence == "BEARISH_DIVERGENCE" {
				sb.WriteString("  <i>Price naik tapi delta turun — tekanan beli melemah.</i>\n")
			} else {
				sb.WriteString("  <i>Price turun tapi delta naik — tekanan jual melemah.</i>\n")
			}
		}

		sb.WriteString(fmt.Sprintf("  <i>Bars used: %d</i>\n", d.BarsUsed))
	}

	return sb.String()
}


func formatCTATimeframeDetail(state *ctaState, tf string, result *ta.FullResult) string {
	var sb strings.Builder
	tfLabel := strings.ToUpper(tf)
	sb.WriteString(fmt.Sprintf("<b>⚡ TA: %s — %s</b>\n\n", html.EscapeString(state.symbol), tfLabel))

	if result == nil || result.Confluence == nil {
		sb.WriteString("⚠️ Data tidak cukup.\n")
		return sb.String()
	}

	conf := result.Confluence
	dirEmoji := "🔵"
	if conf.Direction == "BULLISH" {
		dirEmoji = "🟢"
	} else if conf.Direction == "BEARISH" {
		dirEmoji = "🔴"
	}

	sb.WriteString(fmt.Sprintf("%s <b>%s</b> Skor: %+.0f Grade: %s\n",
		dirEmoji, conf.Direction, conf.Score, conf.Grade))
	sb.WriteString(fmt.Sprintf("Confluence: %d bull / %d bear / %d netral\n\n",
		conf.BullishCount, conf.BearishCount, conf.NeutralCount))

	snap := result.Snapshot
	if snap != nil {
		sb.WriteString("<b>Indikator:</b>\n")

		if snap.RSI != nil {
			rsiNote := ""
			if snap.RSI.Zone == "OVERBOUGHT" {
				rsiNote = " — ⚠️ jenuh beli"
			} else if snap.RSI.Zone == "OVERSOLD" {
				rsiNote = " — 💡 jenuh jual, potensi rebound"
			} else if snap.RSI.Value >= 55 {
				rsiNote = " — momentum bullish"
			} else if snap.RSI.Value <= 45 {
				rsiNote = " — momentum bearish"
			} else {
				rsiNote = " — netral"
			}
			sb.WriteString(fmt.Sprintf("• RSI(14): <b>%.1f</b>%s\n", snap.RSI.Value, rsiNote))
		}
		if snap.MACD != nil {
			sb.WriteString(fmt.Sprintf("• MACD: %.4f | Signal: %.4f | Hist: %+.4f\n",
				snap.MACD.MACD, snap.MACD.Signal, snap.MACD.Histogram))
			if snap.MACD.BullishCross {
				sb.WriteString("  ↳ ✅ Bullish cross — <i>momentum naik baru dimulai</i>\n")
			} else if snap.MACD.BearishCross {
				sb.WriteString("  ↳ ❌ Bearish cross — <i>momentum turun baru dimulai</i>\n")
			}
		}
		if snap.Stochastic != nil {
			stochNote := ""
			if snap.Stochastic.Zone == "OVERBOUGHT" {
				stochNote = " ⚠️ overbought"
			} else if snap.Stochastic.Zone == "OVERSOLD" {
				stochNote = " 💡 oversold"
			}
			sb.WriteString(fmt.Sprintf("• Stoch: %%K=%.1f %%D=%.1f%s\n",
				snap.Stochastic.K, snap.Stochastic.D, stochNote))
		}
		if snap.ADX != nil {
			adxNote := ""
			switch snap.ADX.TrendStrength {
			case "STRONG":
				adxNote = " — tren sangat kuat"
			case "MODERATE":
				adxNote = " — tren moderate"
			default:
				adxNote = " — tidak trending (sideways)"
			}
			di := "↗️"
			if snap.ADX.MinusDI > snap.ADX.PlusDI {
				di = "↘️"
			}
			sb.WriteString(fmt.Sprintf("• ADX: %.1f %s (+DI=%.1f -DI=%.1f)%s\n",
				snap.ADX.ADX, di, snap.ADX.PlusDI, snap.ADX.MinusDI, adxNote))
		}
		if snap.Bollinger != nil {
			bbNote := ""
			if snap.Bollinger.Squeeze {
				bbNote = " 💥 SQUEEZE"
			}
			sb.WriteString(fmt.Sprintf("• BB: %%B=%.2f BW=%.2f%s\n",
				snap.Bollinger.PercentB, snap.Bollinger.Bandwidth, bbNote))
		}
		if snap.WilliamsR != nil {
			sb.WriteString(fmt.Sprintf("• Williams %%R: %.1f (%s)\n",
				snap.WilliamsR.Value, snap.WilliamsR.Zone))
		}
		if snap.CCI != nil {
			sb.WriteString(fmt.Sprintf("• CCI: %.1f (%s)\n", snap.CCI.Value, snap.CCI.Zone))
		}

		// SMC: Smart Money Concepts
		if snap.SMC != nil {
			smc := snap.SMC
			structEmoji := "🔵"
			if string(smc.Structure) == "BULLISH" {
				structEmoji = "🟢"
			} else if string(smc.Structure) == "BEARISH" {
				structEmoji = "🔴"
			}
			sb.WriteString(fmt.Sprintf("\n🏗 <b>SMC Structure:</b> %s %s (zone: %s)\n",
				structEmoji, smc.Structure, smc.CurrentZone))
			if len(smc.RecentCHOCH) > 0 {
				ch := smc.RecentCHOCH[0]
				chEmoji := "🔄"
				if ch.Dir == "BULLISH" {
					chEmoji = "🟢🔄"
				} else if ch.Dir == "BEARISH" {
					chEmoji = "🔴🔄"
				}
				sb.WriteString(fmt.Sprintf("  %s CHOCH %s @ %.4f\n", chEmoji, ch.Dir, ch.Price))
			}
			if len(smc.RecentBOS) > 0 {
				bos := smc.RecentBOS[0]
				bosEmoji := "📈"
				if bos.Dir == "BEARISH" {
					bosEmoji = "📉"
				}
				sb.WriteString(fmt.Sprintf("  %s BOS %s @ %.4f\n", bosEmoji, bos.Dir, bos.Price))
			}
		}
	}

	if len(result.Patterns) > 0 {
		sb.WriteString("\n🕯 <b>Pola:</b>\n")
		for _, p := range result.Patterns {
			dirE := "⚪"
			if p.Direction == "BULLISH" {
				dirE = "🟢"
			} else if p.Direction == "BEARISH" {
				dirE = "🔴"
			}
			stars := strings.Repeat("★", p.Reliability) + strings.Repeat("☆", 3-p.Reliability)
			sb.WriteString(fmt.Sprintf("  %s %s %s\n", dirE, p.Name, stars))
		}
	}

	if result.Zones != nil && result.Zones.Valid {
		z := result.Zones
		sb.WriteString(fmt.Sprintf("\n🎯 <b>%s Setup:</b>\n", z.Direction))
		sb.WriteString(fmt.Sprintf("  Entry: %.4f – %.4f | SL: %.4f\n",
			z.EntryLow, z.EntryHigh, z.StopLoss))
		sb.WriteString(fmt.Sprintf("  TP1: %.4f (R:R 1:%.1f) | TP2: %.4f (R:R 1:%.1f)\n",
			z.TakeProfit1, z.RiskReward1, z.TakeProfit2, z.RiskReward2))
	} else if result.Zones != nil && !result.Zones.Valid {
		sb.WriteString("\n🎯 <b>Setup:</b> ❌ Tidak ada setup valid\n")
	}

	return sb.String()
}
