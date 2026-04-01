package telegram

// handler_elliott.go — /elliott command handler.
// Implements Elliott Wave counting and projection.

import (
	"context"
	"fmt"
	"html"
	"math"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/elliott"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// ElliottServices — injected dependencies
// ---------------------------------------------------------------------------

// ElliottServices holds dependencies for the /elliott command.
type ElliottServices struct {
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore // may be nil
	Engine         *elliott.Engine
}

// ---------------------------------------------------------------------------
// WithElliott wires the Elliott Wave handler into the main Handler.
// ---------------------------------------------------------------------------

// WithElliott registers the /elliott command on the Handler.
func (h *Handler) WithElliott(svc ElliottServices) {
	h.elliott = &svc
	h.bot.RegisterCommand("/elliott", h.cmdElliott)
}

// ---------------------------------------------------------------------------
// /elliott command
// ---------------------------------------------------------------------------

// cmdElliott handles /elliott [SYMBOL] [TIMEFRAME]
// Examples:
//
//	/elliott EURUSD
//	/elliott XAUUSD H4
//	/elliott BTCUSD daily
func (h *Handler) cmdElliott(ctx context.Context, chatID string, userID int64, args string) error {
	if h.elliott == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "⚠️ Elliott Wave engine tidak tersedia.")
		return err
	}

	parts := strings.Fields(strings.TrimSpace(strings.ToUpper(args)))
	if len(parts) == 0 {
		_, err := h.bot.SendHTML(ctx, chatID, `〽️ <b>Elliott Wave Analysis</b>

Gunakan: <code>/elliott [SYMBOL] [TIMEFRAME]</code>

Contoh:
  <code>/elliott EURUSD</code>
  <code>/elliott XAUUSD H4</code>
  <code>/elliott BTCUSD daily</code>

Timeframe yang didukung: <code>daily</code>, <code>4h</code>, <code>1h</code>`)
		return err
	}

	currency := parts[0]
	timeframe := "daily"
	if len(parts) > 1 {
		tf := strings.ToLower(parts[1])
		switch tf {
		case "h4", "4hour", "4h":
			timeframe = "4h"
		case "h1", "1hour", "1h":
			timeframe = "1h"
		case "d", "d1", "daily":
			timeframe = "daily"
		default:
			timeframe = tf
		}
	}

	mapping := domain.FindPriceMappingByCurrency(currency)
	if mapping == nil || mapping.RiskOnly {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("❌ Symbol tidak dikenal: <code>%s</code>", html.EscapeString(currency)))
		return err
	}

	// Loading indicator.
	msgID, _ := h.bot.SendLoading(ctx, chatID,
		fmt.Sprintf("⏳ Menganalisis Elliott Wave <b>%s</b> (%s)...",
			html.EscapeString(mapping.Currency), strings.ToUpper(timeframe)))

	bars, err := h.fetchElliottBars(ctx, mapping, timeframe)
	if err != nil || len(bars) == 0 {
		errMsg := fmt.Sprintf("❌ Gagal mengambil data harga untuk <b>%s</b>: %s",
			html.EscapeString(mapping.Currency), html.EscapeString(fmt.Sprintf("%v", err)))
		if msgID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	result := h.elliott.Engine.Analyze(bars, mapping.Currency, strings.ToUpper(timeframe))
	output := formatElliottResult(result, mapping.Currency, strings.ToUpper(timeframe))

	if msgID > 0 {
		return h.bot.EditMessage(ctx, chatID, msgID, output)
	}
	_, sendErr := h.bot.SendHTML(ctx, chatID, output)
	return sendErr
}

// ---------------------------------------------------------------------------
// fetchElliottBars — fetch OHLCV bars
// ---------------------------------------------------------------------------

func (h *Handler) fetchElliottBars(ctx context.Context, mapping *domain.PriceSymbolMapping, timeframe string) ([]ta.OHLCV, error) {
	code := mapping.ContractCode

	switch timeframe {
	case "4h", "1h":
		if h.elliott.IntradayRepo == nil {
			return nil, fmt.Errorf("intraday data tidak tersedia")
		}
		bars, err := h.elliott.IntradayRepo.GetHistory(ctx, code, timeframe, 300)
		if err != nil {
			return nil, fmt.Errorf("fetch intraday bars: %w", err)
		}
		return ta.IntradayBarsToOHLCV(bars), nil

	default: // "daily"
		records, err := h.elliott.DailyPriceRepo.GetDailyHistory(ctx, code, 300)
		if err != nil {
			return nil, fmt.Errorf("fetch daily bars: %w", err)
		}
		return ta.DailyPricesToOHLCV(records), nil
	}
}

// ---------------------------------------------------------------------------
// formatElliottResult — Telegram HTML formatter
// ---------------------------------------------------------------------------

func formatElliottResult(r *elliott.WaveCountResult, symbol, timeframe string) string {
	if r == nil {
		return "❌ Tidak ada data Elliott Wave untuk simbol ini."
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("〽️ <b>ELLIOTT WAVE — %s %s</b>\n\n",
		html.EscapeString(symbol), html.EscapeString(timeframe)))

	// Current wave summary
	confEmoji := confidenceEmoji(r.Confidence)
	sb.WriteString(fmt.Sprintf("📊 <b>CURRENT COUNT:</b> Wave %s (%s degree)\n",
		html.EscapeString(r.CurrentWave), html.EscapeString(r.Degree)))
	sb.WriteString(fmt.Sprintf("📍 <b>Progress:</b> ~%.0f%% complete\n", r.WaveProgress))
	sb.WriteString(fmt.Sprintf("%s <b>Confidence:</b> %s\n", confEmoji, html.EscapeString(r.Confidence)))

	// Wave structure
	if len(r.Waves) > 0 {
		sb.WriteString("\n〽️ <b>WAVE STRUCTURE:</b>\n")
		for _, w := range r.Waves {
			waveIcon := waveNumberEmoji(w.Number)
			validMark := "✅"
			if !w.Valid {
				validMark = "⚠️"
			}
			ongoing := w.EndBar == -1
			endStr := fmt.Sprintf("%.4f", w.End)
			if ongoing {
				endStr = "ongoing"
			}
			moveStr := pipsDesc(w.Start, w.End, w.Direction)
			retStr := ""
			if w.Retracement > 0 {
				retStr = fmt.Sprintf(", %.1f%% ret", w.Retracement*100)
			}
			fibStr := ""
			if w.FibRatio > 0 {
				fibStr = fmt.Sprintf(", %.2f×W1", w.FibRatio)
			}
			indicator := ""
			if ongoing {
				indicator = " ← CURRENT"
			}
			sb.WriteString(fmt.Sprintf("  %s <code>%.4f → %s</code> (%s%s%s) %s%s\n",
				waveIcon, w.Start, endStr, moveStr, retStr, fibStr, validMark, indicator))

			if !w.Valid && w.Violation != "" {
				sb.WriteString(fmt.Sprintf("      ⛔ %s\n", html.EscapeString(w.Violation)))
			}
		}
	}

	// Projections
	if r.Target1 != 0 || r.Target2 != 0 {
		sb.WriteString("\n🎯 <b>PROJECTIONS:</b>\n")
		if r.Target1 != 0 {
			sb.WriteString(fmt.Sprintf("  Conservative: <code>%.4f</code>\n", r.Target1))
		}
		if r.Target2 != 0 {
			sb.WriteString(fmt.Sprintf("  Aggressive:   <code>%.4f</code>\n", r.Target2))
		}
	}

	// Invalidation
	if r.InvalidationLevel != 0 {
		sb.WriteString(fmt.Sprintf("\n🚫 <b>INVALIDATION:</b> Below <code>%.4f</code>\n", r.InvalidationLevel))
	}

	// Alternate count
	if r.AlternateCount != nil {
		sb.WriteString(fmt.Sprintf("\n⚠️  <b>ALTERNATE:</b> %s\n", html.EscapeString(r.AlternateCount.Summary)))
	}

	// Summary
	if r.Summary != "" {
		sb.WriteString(fmt.Sprintf("\n💡 <b>SUMMARY:</b> %s\n", html.EscapeString(r.Summary)))
	}

	// Timestamp
	sb.WriteString(fmt.Sprintf("\n<i>%s</i>", r.AnalyzedAt.Format("02 Jan 2006 15:04 UTC")))

	return sb.String()
}

// ---------------------------------------------------------------------------
// formatting helpers
// ---------------------------------------------------------------------------

func confidenceEmoji(c string) string {
	switch c {
	case "HIGH":
		return "🟢"
	case "MEDIUM":
		return "🟡"
	default:
		return "🔴"
	}
}

func waveNumberEmoji(n string) string {
	switch n {
	case "1":
		return "①"
	case "2":
		return "②"
	case "3":
		return "③"
	case "4":
		return "④"
	case "5":
		return "⑤"
	case "A":
		return "Ⓐ"
	case "B":
		return "Ⓑ"
	case "C":
		return "Ⓒ"
	default:
		return "◎"
	}
}

// pipsDesc returns a short description of price movement, e.g. "+250 pips".
func pipsDesc(start, end float64, dir string) string {
	delta := end - start
	abs := math.Abs(delta)
	sign := "+"
	if delta < 0 {
		sign = "-"
	}
	// Use pips for forex-like values (price < 10000 and 4 decimal places)
	if start > 0 && start < 20000 && abs < 100 {
		pips := abs * 10000 // 1 pip = 0.0001
		return fmt.Sprintf("%s%.0f pips", sign, pips)
	}
	return fmt.Sprintf("%s%.2f pts", sign, abs)
}
