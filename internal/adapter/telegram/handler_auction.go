package telegram

// handler_auction.go — /auction command: AMT Day Type + Opening Type Analysis.
// Requires VP services to be configured (IntradayRepo for 30-minute bars).

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// /auction command registration (called from WithVP)
// ---------------------------------------------------------------------------

func (h *Handler) registerAuctionCommand() {
	h.bot.RegisterCommand("/auction", h.cmdAuction)
}

// ---------------------------------------------------------------------------
// /auction command handler
// ---------------------------------------------------------------------------

func (h *Handler) cmdAuction(ctx context.Context, chatID string, userID int64, args string) error {
	if h.vp == nil || h.vp.IntradayRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚙️ Auction Market Theory engine tidak tersedia.\n\nPastikan VP services dikonfigurasi dengan IntradayRepo.")
		return err
	}

	parts := strings.Fields(strings.TrimSpace(strings.ToUpper(args)))
	if len(parts) == 0 {
		if lc := h.getLastCurrency(ctx, userID); lc != "" {
			return h.cmdAuction(ctx, chatID, userID, lc)
		}
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			`🏛️ <b>Auction Market Theory — Day Type Analysis</b>

Klasifikasi Dalton's 6 Day Types untuk setiap hari trading:

🟢 <b>Normal Day</b> — IB ≥85% range. Balanced, fade extremes.
🟡 <b>Normal Variation</b> — IB 70–85%. One-sided extension.
🔴 <b>Trend Day</b> — IB &lt;50% range. Strong conviction.
🔵 <b>Double Distribution</b> — Two value clusters.
📈 <b>P-shape</b> — Heavy upper volume, long lower tail.
📉 <b>b-shape</b> — Heavy lower volume, long upper tail.

Pilih aset:`, h.kb.QuantSymbolMenu())
		return err
	}

	symbol := parts[0]
	mapping := domain.FindPriceMappingByCurrency(symbol)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf(
			"❌ Symbol <code>%s</code> tidak ditemukan.\nContoh: <code>/auction EUR</code>, <code>/auction XAU</code>",
			html.EscapeString(symbol),
		))
		return err
	}

	h.saveLastCurrency(ctx, userID, mapping.Currency)
	loadingID, _ := h.bot.SendLoading(ctx, chatID, fmt.Sprintf(
		"🏛️ Analysing Day Types for <b>%s</b>...", html.EscapeString(mapping.Currency),
	))

	// Fetch 30m bars (≥10 days × ~48 bars/day = ~480 bars).
	intradayBars, err := h.vp.IntradayRepo.GetHistory(ctx, mapping.ContractCode, "30m", 500)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	if err != nil || len(intradayBars) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("insufficient 30m data for %s: %w", mapping.Currency, err), "auction")
		return nil
	}

	bars := ta.IntradayBarsToOHLCV(intradayBars) // newest-first
	result := ta.ClassifyDayTypes(bars, 2, 6)    // ibPeriods=2 (1h IB), maxDays=6
	if result == nil || len(result.Days) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("insufficient bars for day type classification"), "auction")
		return nil
	}

	// Opening type analysis (Module 2) — best-effort, nil-safe.
	openingResult := ta.ClassifyOpening(bars, 2, 10)

	// Module 3: Rotation Factor — best-effort, nil-safe.
	rotationResult := ta.ClassifyRotation(bars, 2, 6)

	// Module 4: Close Location — best-effort, nil-safe.
	closeResult := ta.ClassifyClose(bars, 10)

	// Module 5: Multi-Day Migration + MGI — best-effort, nil-safe.
	migrationResult := ta.ClassifyMigration(bars, 10)

	msg := formatAuctionResult(mapping.Currency, result, openingResult)

	// Append Modules 3-5 if available (split into second message if too long).
	mod35 := formatAuctionModules35(mapping.Currency, rotationResult, closeResult, migrationResult)
	if mod35 != "" {
		if len(msg)+len(mod35) < 3800 {
			msg += "\n" + mod35
		} else {
			// Send first message, then second.
			_, _ = h.bot.SendHTML(ctx, chatID, msg)
			_, err = h.bot.SendHTML(ctx, chatID, mod35)
			return err
		}
	}

	_, err = h.bot.SendHTML(ctx, chatID, msg)
	return err
}

// ---------------------------------------------------------------------------
// Formatter
// ---------------------------------------------------------------------------

func formatAuctionResult(symbol string, r *ta.AMTDayTypeResult, opening *ta.AMTOpeningResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🏛️ <b>AMT DAY TYPE: %s</b>\n", html.EscapeString(symbol)))
	sb.WriteString(fmt.Sprintf("📅 %s\n\n", time.Now().UTC().Format("02 Jan 2006 15:04 UTC")))

	for i := len(r.Days) - 1; i >= 0; i-- {
		d := r.Days[i]
		emoji := dayTypeEmoji(d.Type)
		sb.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n",
			emoji,
			d.Date.Format("Mon 02 Jan"),
			html.EscapeString(string(d.Type)),
		))
		sb.WriteString(fmt.Sprintf("  IB: <code>%.1f%%</code> of range", d.IBPercent))
		if d.ExtensionUp > 0 || d.ExtensionDown > 0 {
			netDir := "↑"
			if d.NetExtension < 0 {
				netDir = "↓"
			}
			sb.WriteString(fmt.Sprintf(" | Ext: <code>%s%.0f pips</code>", netDir, absVal(d.NetExtension)/0.0001))
		}
		sb.WriteByte('\n')
		sb.WriteString(fmt.Sprintf("  <i>%s</i>\n\n", html.EscapeString(d.Description)))
	}

	// Pattern summary
	sb.WriteString("─────────────────────\n")
	biasEmoji := "⚪ Neutral"
	switch r.Bias {
	case "BULLISH":
		biasEmoji = "🟢 Bullish"
	case "BEARISH":
		biasEmoji = "🔴 Bearish"
	}
	sb.WriteString(fmt.Sprintf("%s <b>Bias:</b> %s\n", biasEmoji, r.Bias))

	migEmoji := "➡️"
	switch r.ValueMigration {
	case "HIGHER":
		migEmoji = "📈"
	case "LOWER":
		migEmoji = "📉"
	}
	sb.WriteString(fmt.Sprintf("%s <b>Value Migration:</b> %s\n", migEmoji, r.ValueMigration))

	if r.ConsecutiveTrendDays >= 2 {
		sb.WriteString(fmt.Sprintf("⚡ <b>%d consecutive Trend Days</b> — strong directional conviction\n", r.ConsecutiveTrendDays))
	}

	// Opening Type Analysis (Module 2) — only if available.
	if opening != nil {
		sb.WriteString("\n─────────────────────\n")
		sb.WriteString("🚪 <b>OPENING TYPE ANALYSIS</b>\n\n")
		oc := opening.Today
		locEmoji := "➡️"
		switch oc.OpenLocation {
		case "ABOVE_VA":
			locEmoji = "⬆️"
		case "BELOW_VA":
			locEmoji = "⬇️"
		}
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>\n", locEmoji, html.EscapeString(string(oc.Type))))
		sb.WriteString(fmt.Sprintf("  Open: <code>%.5f</code> | Location: <i>%s</i>\n", oc.OpenPrice, html.EscapeString(oc.OpenLocation)))
		if oc.Implication != "" {
			sb.WriteString(fmt.Sprintf("  💡 %s\n", html.EscapeString(oc.Implication)))
		}
		if oc.Confidence != "" {
			confEmoji := "🟡"
			if oc.Confidence == "HIGH" {
				confEmoji = "🟢 High"
			} else if oc.Confidence == "LOW" {
				confEmoji = "🔴 Low"
			}
			sb.WriteString(fmt.Sprintf("  %s Confidence: <b>%s</b>\n", confEmoji, oc.Confidence))
		}
		if len(opening.WinRates) > 0 {
			if wr, ok := opening.WinRates[oc.Type]; ok {
				sb.WriteString(fmt.Sprintf("  📊 Historical win rate: <b>%.0f%%</b>\n", wr*100))
			}
		}
	}

	return sb.String()
}

func dayTypeEmoji(t ta.DayType) string {
	switch t {
	case ta.DayTypeNormal:
		return "🟢 Up"
	case ta.DayTypeNormalVariation:
		return "🟡"
	case ta.DayTypeTrend:
		return "🔴 Down"
	case ta.DayTypeDoubleDistribution:
		return "🔵"
	case ta.DayTypePShape:
		return "📈"
	case ta.DayTypeBShape:
		return "📉"
	default:
		return "⚪ Flat"
	}
}

func absVal(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---------------------------------------------------------------------------
// Modules 3–5 Formatter
// ---------------------------------------------------------------------------

func formatAuctionModules35(symbol string, rot *ta.AMTRotationResult, cl *ta.AMTCloseResult, mig *ta.AMTMigrationResult) string {
	var sb strings.Builder
	hasContent := false

	// Module 3: Rotation Factor
	if rot != nil && len(rot.Days) > 0 {
		hasContent = true
		sb.WriteString("─────────────────────\n")
		sb.WriteString("🔄 <b>ROTATION FACTOR</b>\n\n")

		trendIcon := "➡️"
		switch rot.Trend {
		case "INCREASING":
			trendIcon = "📈"
		case "DECREASING":
			trendIcon = "📉"
		}
		sb.WriteString(fmt.Sprintf("📊 Avg RF: <b>%.1f</b> | Trend: %s %s\n\n", rot.AvgRotation, trendIcon, rot.Trend))

		shown := len(rot.Days)
		if shown > 5 {
			shown = 5
		}
		for i := len(rot.Days) - shown; i < len(rot.Days); i++ {
			d := rot.Days[i]
			rfIcon := "⚖️"
			if d.Interpretation == "DIRECTIONAL" {
				rfIcon = "🚀"
			} else if d.Interpretation == "TRANSITIONAL" {
				rfIcon = "⚡"
			}
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b> RF=%d | Time in VA: %.0f%%\n",
				rfIcon, d.Date.Format("Jan02"), d.RotationFactor, d.TimeInVA*100))
		}
		sb.WriteString(fmt.Sprintf("\n💡 <i>%s</i>\n", html.EscapeString(rot.Days[len(rot.Days)-1].Description)))
	}

	// Module 4: Close Location
	if cl != nil && len(cl.Days) > 0 {
		hasContent = true
		sb.WriteString("\n─────────────────────\n")
		sb.WriteString("📍 <b>CLOSE LOCATION</b>\n\n")

		shown := len(cl.Days)
		if shown > 5 {
			shown = 5
		}
		for i := len(cl.Days) - shown; i < len(cl.Days); i++ {
			d := cl.Days[i]
			icon := closeLocIcon(d.Location)
			ft := ""
			if d.NextDayDirection != "" {
				if d.FollowedThrough {
					ft = " ✅"
				} else {
					ft = " ❌"
				}
			}
			sb.WriteString(fmt.Sprintf("  %s <b>%s</b> %s%s\n",
				icon, d.Date.Format("Jan02"), string(d.Location), ft))
		}

		// Follow-through rates
		if len(cl.FollowThroughRates) > 0 {
			sb.WriteString("\n📊 <b>Follow-Through Rates:</b>\n")
			for _, loc := range []ta.CloseLocation{ta.CloseAboveVAH, ta.CloseBelowVAL, ta.CloseAtPOC, ta.CloseInsideVA} {
				if rate, ok := cl.FollowThroughRates[loc]; ok {
					sb.WriteString(fmt.Sprintf("  %s %s: <b>%.0f%%</b>\n", closeLocIcon(loc), string(loc), rate*100))
				}
			}
		}

		if cl.TodayImplication != "" {
			sb.WriteString(fmt.Sprintf("\n💡 <i>%s</i>\n", html.EscapeString(cl.TodayImplication)))
		}
	}

	// Module 5: Migration + MGI
	if mig != nil && len(mig.Days) >= 2 {
		hasContent = true
		sb.WriteString("\n─────────────────────\n")
		sb.WriteString("🗺 <b>VALUE MIGRATION</b>\n\n")

		migIcon := "➡️"
		switch mig.NetDirection {
		case ta.MigrationUp:
			migIcon = "📈"
		case ta.MigrationDown:
			migIcon = "📉"
		case ta.MigrationBalanced:
			migIcon = "⚖️"
		}
		sb.WriteString(fmt.Sprintf("%s Direction: <b>%s</b> | Score: <b>%.0f</b>\n\n",
			migIcon, string(mig.NetDirection), mig.MigrationScore))

		// POC migration chart
		if mig.MigrationChart != "" {
			sb.WriteString(mig.MigrationChart)
			sb.WriteString("\n\n")
		}

		// MGI for latest day
		if len(mig.Days) > 0 {
			latest := mig.Days[len(mig.Days)-1]
			if len(latest.MGILevels) > 0 {
				sb.WriteString("🔍 <b>MGI (Today):</b>\n")
				for _, lvl := range latest.MGILevels {
					icon := "❌"
					if lvl.Accepted {
						icon = "✅"
					}
					sb.WriteString(fmt.Sprintf("  %s %s\n", icon, html.EscapeString(lvl.Description)))
				}
				sb.WriteByte('\n')
			}
		}

		// Composite VA
		if mig.WeeklyVA != nil {
			sb.WriteString(fmt.Sprintf("📦 <b>Weekly VA:</b> <code>%.5f — %.5f</code> (POC: <code>%.5f</code>)\n",
				mig.WeeklyVA.VAL, mig.WeeklyVA.VAH, mig.WeeklyVA.POC))
		}
		if mig.MonthlyVA != nil {
			sb.WriteString(fmt.Sprintf("📦 <b>Monthly VA:</b> <code>%.5f — %.5f</code> (POC: <code>%.5f</code>)\n",
				mig.MonthlyVA.VAL, mig.MonthlyVA.VAH, mig.MonthlyVA.POC))
		}

		if mig.Summary != "" {
			sb.WriteString(fmt.Sprintf("\n💡 <i>%s</i>\n", html.EscapeString(mig.Summary)))
		}
	}

	if !hasContent {
		return ""
	}
	return sb.String()
}

func closeLocIcon(loc ta.CloseLocation) string {
	switch loc {
	case ta.CloseAboveVAH:
		return "⬆️"
	case ta.CloseBelowVAL:
		return "⬇️"
	case ta.CloseAtPOC:
		return "🎯"
	case ta.CloseInsideVA:
		return "📦"
	default:
		return "❓"
	}
}
