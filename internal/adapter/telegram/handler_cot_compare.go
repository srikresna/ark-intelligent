package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ---------------------------------------------------------------------------
// /compare — Side-by-side COT positioning comparison
// ---------------------------------------------------------------------------

// cmdCompare handles /compare EUR GBP — side-by-side COT positioning.
func (h *Handler) cmdCompare(ctx context.Context, chatID string, userID int64, args string) error {
	h.bot.SendTyping(ctx, chatID)

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) < 2 {
		_, err := h.bot.SendHTML(ctx, chatID,
			"⚖️ <b>COT Compare</b>\n\nUsage: <code>/compare EUR GBP</code>\n\nBandingkan positioning dua aset secara side-by-side.")
		return err
	}

	currA, currB := parts[0], parts[1]
	codeA := currencyToContractCode(currA)
	codeB := currencyToContractCode(currB)

	recsA, errA := h.cotRepo.GetHistory(ctx, codeA, 1)
	recsB, errB := h.cotRepo.GetHistory(ctx, codeB, 1)
	if errA != nil || len(recsA) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("no data for %s", currA), "compare")
		return nil
	}
	if errB != nil || len(recsB) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("no data for %s", currB), "compare")
		return nil
	}

	rA, rB := recsA[0], recsB[0]

	// Detect report type per contract for correct net calculation
	rtA, rtB := "TFF", "TFF"
	analyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
	for _, a := range analyses {
		if a.Contract.Code == codeA {
			rtA = a.Contract.ReportType
		}
		if a.Contract.Code == codeB {
			rtB = a.Contract.ReportType
		}
	}
	netA := rA.GetSmartMoneyNet(rtA)
	netB := rB.GetSmartMoneyNet(rtB)

	biasA, iconA := cotBiasLabel(netA)
	biasB, iconB := cotBiasLabel(netB)
	chgLabelA := cotFormatChg(rA.NetChange)
	chgLabelB := cotFormatChg(rB.NetChange)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("⚖️ <b>COT Compare — %s vs %s</b>\n", currA, currB))
	b.WriteString(fmt.Sprintf("<i>Report: %s</i>\n\n", rA.ReportDate.Format("02 Jan 2006")))
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-12s  %-15s  %-15s\n", "", currA, currB))
	b.WriteString(fmt.Sprintf("%-12s  %-15s  %-15s\n", "Net Pos", fmt.Sprintf("%+.0f", netA), fmt.Sprintf("%+.0f", netB)))
	b.WriteString(fmt.Sprintf("%-12s  %-15s  %-15s\n", "WoW Chg", chgLabelA, chgLabelB))
	b.WriteString(fmt.Sprintf("%-12s  %-15s  %-15s\n", "Bias", biasA, biasB))
	b.WriteString("</pre>\n")
	b.WriteString(fmt.Sprintf("\n%s <b>%s</b> %s   |   %s <b>%s</b> %s",
		iconA, currA, biasA, iconB, currB, biasB))

	_, err := h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// cotBiasLabel returns a human-readable bias label and icon for a net position value.
func cotBiasLabel(net float64) (string, string) {
	if net > 5000 {
		return "BULLISH", "🟢"
	}
	if net < -5000 {
		return "BEARISH", "🔴"
	}
	return "NEUTRAL", "🟡"
}

// cotFormatChg formats a WoW change value with sign and K-suffix for readability.
func cotFormatChg(chg float64) string {
	if chg == 0 {
		return "N/A"
	}
	if chg >= 1000 || chg <= -1000 {
		return fmt.Sprintf("%+.1fK", chg/1000)
	}
	return fmt.Sprintf("%+.0f", chg)
}

// ---------------------------------------------------------------------------
// cbHistoryNav — History view navigation (week range toggle)
// ---------------------------------------------------------------------------

// cbHistoryNav handles "hist:<currency>:<weeks>" callbacks from inline keyboard buttons.
func (h *Handler) cbHistoryNav(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// data format: "hist:EUR:8"
	trimmed := strings.TrimPrefix(data, "hist:")
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	currency := strings.ToUpper(parts[0])
	weeks := 4
	if w, err := strconv.Atoi(parts[1]); err == nil && w >= 2 && w <= 52 {
		weeks = w
	}

	contractCode := currencyToContractCode(currency)
	records, err := h.cotRepo.GetHistory(ctx, contractCode, weeks)
	if err != nil || len(records) == 0 {
		return h.bot.EditMessage(ctx, chatID, msgID,
			fmt.Sprintf("⚠️ Tidak ada data history untuk %s.", currency))
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📊 <b>COT History — %s (%d weeks)</b>\n", currency, len(records)))
	b.WriteString(fmt.Sprintf("<i>%s → %s</i>\n\n",
		records[len(records)-1].ReportDate.Format("02 Jan"),
		records[0].ReportDate.Format("02 Jan 2006")))

	netPositions := make([]float64, len(records))
	for i, r := range records {
		netPositions[i] = r.GetSmartMoneyNet("TFF")
	}
	for i, j := 0, len(netPositions)-1; i < j; i, j = i+1, j-1 {
		netPositions[i], netPositions[j] = netPositions[j], netPositions[i]
	}
	b.WriteString("📈 Trend: <code>")
	b.WriteString(sparkLine(netPositions))
	b.WriteString("</code>\n\n")

	b.WriteString("<pre>")
	b.WriteString("Date       | Net Pos   | Chg      | L/S\n")
	b.WriteString("───────────┼───────────┼──────────┼────\n")
	for i, r := range records {
		net := int64(r.GetSmartMoneyNet("TFF"))
		var chg int64
		if i+1 < len(records) {
			prevNet := int64(records[i+1].GetSmartMoneyNet("TFF"))
			chg = net - prevNet
		}
		ratio := 0.0
		if r.LevFundShort > 0 {
			ratio = r.LevFundLong / r.LevFundShort
		}
		b.WriteString(fmt.Sprintf("%-10s | %+9d | %+8d | %.2f\n",
			r.ReportDate.Format("02 Jan"), net, chg, ratio))
	}
	b.WriteString("</pre>")

	navKB := ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				historyNavBtn(currency, 4, weeks),
				historyNavBtn(currency, 8, weeks),
				historyNavBtn(currency, 12, weeks),
			},
		},
	}
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, b.String(), navKB)
}

// historyNavBtn creates a history navigation button; marks the active week range with ✓.
func historyNavBtn(currency string, targetWeeks, activeWeeks int) ports.InlineButton {
	label := fmt.Sprintf("%dW", targetWeeks)
	if targetWeeks == activeWeeks {
		label += " ✓"
	}
	return ports.InlineButton{
		Text:         label,
		CallbackData: fmt.Sprintf("hist:%s:%d", currency, targetWeeks),
	}
}
