package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	vixsvc "github.com/arkcode369/ark-intelligent/internal/service/vix"
)

// cmdVix handles the /vix command, displaying a full CBOE volatility index
// dashboard: VIX term structure, SKEW tail risk, OVX/GVZ/RVX cross-asset
// comparison, VIX9D event pricing, COR3M dispersion, and regime classification.
//
// Cache TTL: 12 hours (EOD data from CBOE).
func (h *Handler) cmdVix(ctx context.Context, chatID string, _ int64, _ string) error {
	h.bot.SendTyping(ctx, chatID)

	fetchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	ts, err := vixsvc.FetchTermStructure(fetchCtx)
	text := formatVIXDashboard(ts, err)

	_, sendErr := h.bot.SendHTML(ctx, chatID, text)
	return sendErr
}

// formatVIXDashboard formats the full VIX volatility suite dashboard for Telegram HTML.
func formatVIXDashboard(ts *vixsvc.VIXTermStructure, fetchErr error) string {
	if fetchErr != nil || ts == nil || !ts.Available {
		errMsg := "data tidak tersedia"
		if ts != nil && ts.Error != "" {
			errMsg = ts.Error
		} else if fetchErr != nil {
			errMsg = fetchErr.Error()
		}
		return fmt.Sprintf("❌ <b>VIX Dashboard</b>\n\nGagal mengambil data CBOE: <code>%s</code>", errMsg)
	}

	var b strings.Builder

	// Header
	b.WriteString("<b>📊 CBOE Volatility Dashboard</b>\n")
	b.WriteString(fmt.Sprintf("<i>Data: %s UTC</i>\n\n", ts.AsOf.Format("02 Jan 2006 15:04")))

	// VIX spot + regime
	regimeEmoji := vixRegimeEmoji(ts.Regime)
	b.WriteString("<b>VIX Spot &amp; Regime</b>\n")
	b.WriteString(fmt.Sprintf("<code>VIX Spot : %.2f</code>\n", ts.Spot))
	b.WriteString(fmt.Sprintf("<code>Regime   : %s %s</code>\n", ts.Regime, regimeEmoji))
	if ts.VVIX > 0 {
		b.WriteString(fmt.Sprintf("<code>VVIX     : %.2f</code>  <i>(vol-of-vol)</i>\n", ts.VVIX))
	}

	// Term structure
	if ts.M1 > 0 {
		termEmoji := "✅ Contango"
		if ts.Backwardation {
			termEmoji = "🔴 Backwardation"
		}
		b.WriteString("\n<b>Term Structure</b>\n")
		b.WriteString(fmt.Sprintf("<code>M1  : %.2f  %s</code>\n", ts.M1, ts.M1Symbol))
		if ts.M2 > 0 {
			b.WriteString(fmt.Sprintf("<code>M2  : %.2f  %s</code>\n", ts.M2, ts.M2Symbol))
		}
		if ts.M3 > 0 {
			b.WriteString(fmt.Sprintf("<code>M3  : %.2f  %s</code>\n", ts.M3, ts.M3Symbol))
		}
		slopeSign := "+"
		if ts.SlopePct < 0 {
			slopeSign = ""
		}
		b.WriteString(fmt.Sprintf("<code>Slope: %s%.1f%%</code>  %s\n", slopeSign, ts.SlopePct, termEmoji))
		if ts.RollYield != 0 {
			rollSign := "+"
			if ts.RollYield < 0 {
				rollSign = ""
			}
			b.WriteString(fmt.Sprintf("<code>Roll : %s%.1f%% /mo</code>\n", rollSign, ts.RollYield))
		}
	}

	// Vol Suite (SKEW, OVX, GVZ, RVX, VIX9D, COR3M)
	vs := ts.VolSuite
	if vs != nil && vs.Available {
		b.WriteString("\n<b>CBOE Vol Suite</b>\n")

		if vs.SKEW > 0 {
			skewEmoji := vixVolLevelEmoji(vs.SKEW, 130, 140)
			pctStr := ""
			if vs.SKEWPercentile > 0 {
				pctStr = fmt.Sprintf(" P%.0f", vs.SKEWPercentile)
			}
			b.WriteString(fmt.Sprintf("<code>SKEW  : %.1f%s %s</code>\n", vs.SKEW, pctStr, skewEmoji))
		}
		if vs.OVX > 0 {
			b.WriteString(fmt.Sprintf("<code>OVX   : %.1f</code>  <i>(oil vol)</i>\n", vs.OVX))
		}
		if vs.GVZ > 0 {
			b.WriteString(fmt.Sprintf("<code>GVZ   : %.1f</code>  <i>(gold vol)</i>\n", vs.GVZ))
		}
		if vs.RVX > 0 {
			rvxEmoji := ""
			if vs.RVXVIXRatio > 1.3 {
				rvxEmoji = " ⚠️"
			}
			b.WriteString(fmt.Sprintf("<code>RVX   : %.1f%s</code>  <i>(small cap vol)</i>\n", vs.RVX, rvxEmoji))
		}
		if vs.VIX9D > 0 {
			v9dEmoji := ""
			if vs.VIX9D30Ratio > 1.1 {
				v9dEmoji = " ⚠️"
			}
			b.WriteString(fmt.Sprintf("<code>VIX9D : %.2f%s</code>  <i>(9-day event vol)</i>\n", vs.VIX9D, v9dEmoji))
		}
		if vs.COR3M > 0 {
			cor3mEmoji := ""
			cor3mNote := "implied correlation"
			switch {
			case vs.COR3M < 20:
				cor3mEmoji = " 📈"
				cor3mNote = "high dispersion"
			case vs.COR3M > 70:
				cor3mEmoji = " ⚠️"
				cor3mNote = "macro correlation regime"
			}
			b.WriteString(fmt.Sprintf("<code>COR3M : %.0f%s</code>  <i>(%s)</i>\n", vs.COR3M, cor3mEmoji, cor3mNote))
		}

		// Ratios
		hasRatios := vs.SKEWVIXRatio > 0 || vs.OVXVIXRatio > 0 || vs.GVZVIXRatio > 0 || vs.RVXVIXRatio > 0 || vs.VIX9D30Ratio > 0
		if hasRatios {
			b.WriteString("\n<b>Vol Ratios vs VIX</b>\n")
			if vs.SKEWVIXRatio > 0 {
				ratioEmoji := ""
				if vs.SKEWVIXRatio > 8.0 {
					ratioEmoji = " 🔴"
				}
				pctStr := ""
				if vs.SKEWVIXPercentile > 0 {
					pctStr = fmt.Sprintf(" (P%.0f)", vs.SKEWVIXPercentile)
				}
				b.WriteString(fmt.Sprintf("<code>SKEW/VIX : %.1f%s%s</code>\n", vs.SKEWVIXRatio, pctStr, ratioEmoji))
			}
			if vs.OVXVIXRatio > 0 {
				b.WriteString(fmt.Sprintf("<code>OVX/VIX  : %.1f</code>\n", vs.OVXVIXRatio))
			}
			if vs.GVZVIXRatio > 0 {
				b.WriteString(fmt.Sprintf("<code>GVZ/VIX  : %.2f</code>\n", vs.GVZVIXRatio))
			}
			if vs.RVXVIXRatio > 0 {
				b.WriteString(fmt.Sprintf("<code>RVX/VIX  : %.2f</code>\n", vs.RVXVIXRatio))
			}
			if vs.VIX9D30Ratio > 0 {
				b.WriteString(fmt.Sprintf("<code>9D/30D   : %.2f</code>\n", vs.VIX9D30Ratio))
			}
		}

		// Tail risk assessment
		if vs.TailRisk != "" && vs.TailRisk != "NORMAL" {
			b.WriteString("\n")
			switch vs.TailRisk {
			case "EXTREME":
				b.WriteString("🔴 <b>TAIL RISK EXTREME</b> — SKEW/VIX historically dangerous\n")
			case "ELEVATED":
				b.WriteString("⚠️ <b>Tail risk elevated</b> — SKEW tinggi vs VIX rendah\n")
			}
			tailCtx := vs.TailRiskContext()
			if tailCtx != "" {
				b.WriteString(fmt.Sprintf("<i>%s</i>\n", tailCtx))
			}
		}

		// Cross-vol dashboard
		if vs.CrossVol != nil {
			b.WriteString(vixsvc.FormatCrossVolDashboard(vs, ts.Spot, vs.CrossVol))
		}

		// Divergences
		if len(vs.Divergences) > 0 {
			b.WriteString("\n<b>Divergensi Vol</b>\n")
			for _, d := range vs.Divergences {
				b.WriteString(fmt.Sprintf("• <i>%s</i>\n", d))
			}
		}
	}

	// MOVE index (bond volatility)
	if ts.MOVE != nil && ts.MOVE.Available {
		b.WriteString("\n<b>MOVE Index (Bond Vol)</b>\n")
		b.WriteString(fmt.Sprintf("<code>MOVE  : %.1f</code>\n", ts.MOVE.Level))
		if ts.MOVE.DailyChangePct != 0 {
			sign := "+"
			if ts.MOVE.DailyChangePct < 0 {
				sign = ""
			}
			b.WriteString(fmt.Sprintf("<code>Daily : %s%.1f%%</code>\n", sign, ts.MOVE.DailyChangePct))
		}
		if ts.MOVE.Divergence != "" {
			b.WriteString(fmt.Sprintf("<i>%s</i>\n", ts.MOVE.Divergence))
		}
	}

	b.WriteString("\n<i>Source: CBOE EOD • Cache: 12h</i>")

	return b.String()
}

// vixRegimeEmoji returns an emoji for a VIX regime string.
func vixRegimeEmoji(regime string) string {
	switch regime {
	case "EXTREME_FEAR":
		return "🔴"
	case "FEAR":
		return "🟠"
	case "ELEVATED":
		return "⚠️"
	case "RISK_ON_NORMAL":
		return "✅"
	case "RISK_ON_COMPLACENT":
		return "🟢"
	default:
		return ""
	}
}

// vixVolLevelEmoji returns an emoji + label based on value vs warn/alert thresholds.
func vixVolLevelEmoji(val, warnThreshold, alertThreshold float64) string {
	switch {
	case val >= alertThreshold:
		return "🔴 Alert"
	case val >= warnThreshold:
		return "⚠️ Warning"
	default:
		return "✅ Normal"
	}
}
