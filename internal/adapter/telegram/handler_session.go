package telegram

// handler_session.go — /session command: Trading Session Analysis
//   /session [SYMBOL]  — Session behavior stats + current session context

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// sessionCache caches session analysis results by currency (TTL 1h).
type sessionCache struct {
	result    *ta.SessionAnalysisResult
	fetchedAt time.Time
}

var (
	sessionAnalysisCache   = map[string]*sessionCache{}
	sessionCacheMutex      sync.RWMutex // protects sessionAnalysisCache
	sessionCacheTTL        = 1 * time.Hour
)

// cmdSession handles /session [SYMBOL].
func (h *Handler) cmdSession(ctx context.Context, chatID string, _ int64, args string) error {
	if h.intradayRepo == nil {
		_, err := h.bot.SendHTML(ctx, chatID,
			"📊 <b>Session Analysis</b>\n\n<i>Intraday data not available. Requires TwelveData key.</i>")
		return err
	}

	args = strings.TrimSpace(strings.ToUpper(args))

	if args == "" {
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			"🕐 <b>Trading Session Analysis</b>\n\n"+
				"Analisis karakter London/NY/Tokyo per pair:\n"+
				"• ADX rata-rata per session\n"+
				"• % waktu trending vs ranging\n"+
				"• Rekomendasi strategi\n\n"+
				"Pilih pair:",
			h.kb.SessionMenu())
		return err
	}

	mapping := domain.FindPriceMappingByCurrency(args)
	if mapping == nil {
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("Unknown symbol: <code>%s</code>\n\nUsage: <code>/session EUR</code>",
				html.EscapeString(args)))
		return err
	}

	// Check cache (thread-safe read)
	sessionCacheMutex.RLock()
	cached, ok := sessionAnalysisCache[mapping.Currency]
	sessionCacheMutex.RUnlock()
	
	if ok && time.Since(cached.fetchedAt) < sessionCacheTTL {
		text := formatSessionAnalysis(cached.result)
		kb := h.kb.SessionDetailMenu(mapping.Currency)
		_, err := h.bot.SendWithKeyboard(ctx, chatID, text, kb)
		return err
	}

	loadingID, _ := h.bot.SendLoading(ctx, chatID,
		fmt.Sprintf("🕐 Menganalisis session behavior untuk <b>%s</b>... ⏳", html.EscapeString(args)))

	// Fetch 1H intraday bars (720 bars ≈ 30 days)
	bars1h, err := h.intradayRepo.GetHistory(ctx, mapping.ContractCode, "1h", 720)
	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	if err != nil || len(bars1h) < 20 {
		msg := fmt.Sprintf("Insufficient intraday data for %s", mapping.Currency)
		if err != nil {
			msg = err.Error()
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("⚠️ <b>Session Analysis</b>\n\n<i>%s</i>", html.EscapeString(msg)))
		return sendErr
	}

	ohlcvBars := ta.IntradayBarsToOHLCV(bars1h)

	analyzer := ta.NewSessionAnalyzer()
	result, err := analyzer.Analyze(ctx, mapping.Currency, ohlcvBars)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("⚠️ Session analysis failed for %s: <i>%s</i>",
				html.EscapeString(mapping.Currency), html.EscapeString(err.Error())))
		return sendErr
	}

	// Cache result (thread-safe write)
	sessionCacheMutex.Lock()
	sessionAnalysisCache[mapping.Currency] = &sessionCache{result: result, fetchedAt: time.Now()}
	sessionCacheMutex.Unlock()

	text := formatSessionAnalysis(result)
	kb := h.kb.SessionDetailMenu(mapping.Currency)
	_, err = h.bot.SendWithKeyboard(ctx, chatID, text, kb)
	return err
}

// formatSessionAnalysis renders the session analysis as HTML.
func formatSessionAnalysis(r *ta.SessionAnalysisResult) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("🕐 <b>Session Analysis — %s</b>\n", html.EscapeString(r.Currency)))
	b.WriteString(fmt.Sprintf("<i>%s UTC — based on 20 sessions</i>\n\n",
		r.CurrentTime.Format("02 Jan 15:04")))

	// Current session status
	sessLabel := sessionDisplayName(r.CurrentSession)
	b.WriteString(fmt.Sprintf("📍 <b>Current:</b> %s", sessLabel))
	if r.MinUntilNext > 0 {
		nextLabel := sessionDisplayName(r.NextSession)
		h := r.MinUntilNext / 60
		m := r.MinUntilNext % 60
		if h > 0 {
			b.WriteString(fmt.Sprintf(" → %s in %dh%02dm", nextLabel, h, m))
		} else {
			b.WriteString(fmt.Sprintf(" → %s in %dm", nextLabel, m))
		}
	}
	b.WriteString("\n\n")

	// Session stats
	sessions := []*ta.SessionStats{r.Tokyo, r.London, r.NewYork}
	for _, s := range sessions {
		if s == nil || s.SampleCount == 0 {
			continue
		}
		b.WriteString(formatSessionBlock(s))
	}

	b.WriteString("\n<i>Strategi: TREND=trend following, BREAKOUT=momentum entry, MEAN_REVERSION=fade extremes</i>")
	return b.String()
}

func formatSessionBlock(s *ta.SessionStats) string {
	var b strings.Builder

	charEmoji := sessionCharEmoji(s.Character)
	stratEmoji := sessionStratEmoji(s.Strategy)

	b.WriteString(fmt.Sprintf("%s <b>%s</b> — <code>%s</code>\n", charEmoji, s.Label, string(s.Character)))
	b.WriteString(fmt.Sprintf(
		"<code>ADX avg : %.1f  |  Range: %.0f pips</code>\n",
		s.ADXAvg, s.RangeAvgPips))
	b.WriteString(fmt.Sprintf(
		"<code>Trending: %.0f%%  |  Ranging: %.0f%%</code>\n",
		s.PctTrending, s.PctRanging))
	b.WriteString(fmt.Sprintf(
		"%s Strategi: <b>%s</b>  <i>(%d sessions)</i>\n\n",
		stratEmoji, string(s.Strategy), s.SampleCount))
	return b.String()
}

func sessionDisplayName(session string) string {
	switch session {
	case ta.SessionTokyo:
		return "🌏 Tokyo/Asia"
	case ta.SessionLondon:
		return "🇬🇧 London"
	case ta.SessionNewYork:
		return "🗽 New York"
	case ta.SessionOverlap:
		return "⚡ London–NY Overlap"
	default:
		return "😴 Off Hours"
	}
}

func sessionCharEmoji(c ta.SessionCharacter) string {
	switch c {
	case ta.CharTrending:
		return "📈"
	case ta.CharRanging:
		return "↔️"
	case ta.CharVolatile:
		return "⚡"
	case ta.CharCalm:
		return "😴"
	default:
		return "🔀"
	}
}

func sessionStratEmoji(s ta.SessionStrategy) string {
	switch s {
	case ta.StratTrend:
		return "🏹"
	case ta.StratBreakout:
		return "💥"
	case ta.StratMeanRevert:
		return "🔄"
	default:
		return "⚖️"
	}
}
