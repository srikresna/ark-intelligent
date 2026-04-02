package telegram

// handler_briefing.go — /briefing & /br commands: daily morning market summary

import (
	"context"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// /briefing — Daily Morning Market Summary
// ---------------------------------------------------------------------------

// cmdBriefing handles the /briefing and /br commands.
// Aggregates today's High/Medium calendar events, top 3 COT conviction scores,
// and a currency bias one-liner into a compact ≤15-line summary.
func (h *Handler) cmdBriefing(ctx context.Context, chatID string, userID int64, args string) error {
	h.bot.SendTyping(ctx, chatID)

	now := timeutil.NowWIB()
	data, err := h.buildBriefingData(ctx, now)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "briefing")
		return nil
	}

	html := h.fmt.FormatBriefing(data)
	kb := h.kb.BriefingMenu()
	_, sendErr := h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return sendErr
}

// cbBriefingRefresh handles the "briefing:refresh" callback for the Refresh button.
func (h *Handler) cbBriefingRefresh(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	now := timeutil.NowWIB()
	bd, err := h.buildBriefingData(ctx, now)
	if err != nil {
		_ = h.bot.AnswerCallback(ctx, data, "⚠️ Gagal memuat data")
		return nil
	}

	html := h.fmt.FormatBriefing(bd)
	kb := h.kb.BriefingMenu()
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
}

// buildBriefingData fetches all data needed for the daily briefing.
// Non-fatal errors (e.g. missing COT) result in empty sections, not failures.
func (h *Handler) buildBriefingData(ctx context.Context, now time.Time) (BriefingData, error) {
	bd := BriefingData{Now: now}

	// ── 1. Calendar events (High + Medium impact, today) ────────────────────
	dateStr := now.Format("20060102")
	if events, err := h.newsRepo.GetByDate(ctx, dateStr); err == nil {
		bd.Events = events
	}
	// If newsRepo fails, bd.Events stays nil → FormatBriefing shows "no events"

	// ── 2. COT conviction scores (parallel, best-effort) ────────────────────
	analyses, cotErr := h.cotRepo.GetAllLatestAnalyses(ctx)
	if cotErr == nil && len(analyses) > 0 {
		bd.Convictions = h.buildConvictionScores(ctx, analyses)
		bd.BiasSummary = FormatBriefingBiasSummary(bd.Convictions)
	}

	return bd, nil
}

// buildConvictionScores computes ConvictionScoreV3 for all available COT analyses.
// Mirrors the logic in cmdCOT overview — best-effort, nil-safe.
func (h *Handler) buildConvictionScores(ctx context.Context, analyses []domain.COTAnalysis) []cot.ConvictionScore {
	// Fetch FRED macro data (cached)
	macroData, fredErr := fred.GetCachedOrFetch(ctx)
	if fredErr != nil || macroData == nil {
		return nil
	}

	composites := fred.ComputeComposites(macroData)
	regime := fred.ClassifyMacroRegime(macroData, composites)

	// Build price contexts for ATR volatility multiplier (optional)
	var priceCtxs map[string]*domain.PriceContext
	if h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pcs, err := ctxBuilder.BuildAll(ctx); err == nil {
			priceCtxs = pcs
		}
	}

	_ = regime // used via ComputeConvictionScoreV3 below

	var scores []cot.ConvictionScore
	for _, a := range analyses {
		surpriseSigma := 0.0
		if h.newsScheduler != nil {
			surpriseSigma = h.newsScheduler.GetSurpriseSigma(a.Contract.Currency)
		}

		var pc *domain.PriceContext
		if priceCtxs != nil {
			pc = priceCtxs[a.Contract.Code]
		}

		cs := cot.ComputeConvictionScoreV3(a, regime, surpriseSigma, "", macroData, pc)
		scores = append(scores, cs)
	}

	return scores
}

// BuildBriefingHTML is a helper for the scheduler to build a briefing HTML string
// directly from available repos — used for automated morning push.
// Returns "" if no meaningful data is available.
func (h *Handler) BuildBriefingHTML(ctx context.Context) string {
	now := timeutil.NowWIB()
	data, err := h.buildBriefingData(ctx, now)
	if err != nil {
		return ""
	}
	// Only send if there's something meaningful (events OR COT data)
	hasContent := len(filterBriefingEvents(data.Events)) > 0 || len(data.Convictions) > 0
	if !hasContent {
		return ""
	}
	return h.fmt.FormatBriefing(data)
}

// sendBriefingToUser sends a briefing push to a specific user by chatID.
// Used by the scheduler for morning auto-push. Returns true if sent successfully.
func (h *Handler) sendBriefingToUser(ctx context.Context, chatID string) bool {
	html := h.BuildBriefingHTML(ctx)
	if html == "" {
		return false
	}
	// Strip keyboard for push sends — keep it minimal
	header := "🌅 <b>Good Morning! ARK Daily Briefing siap:</b>\n\n"
	kb := h.kb.BriefingMenu()
	_, err := h.bot.SendWithKeyboard(ctx, chatID, header+html, kb)
	return err == nil
}

// DayOfWeekLabel returns a human-readable day in Indonesian for the given time.
func DayOfWeekLabel(t time.Time) string {
	days := map[time.Weekday]string{
		time.Monday:    "Senin",
		time.Tuesday:   "Selasa",
		time.Wednesday: "Rabu",
		time.Thursday:  "Kamis",
		time.Friday:    "Jumat",
		time.Saturday:  "Sabtu",
		time.Sunday:    "Minggu",
	}
	if d, ok := days[t.Weekday()]; ok {
		return d
	}
	return t.Weekday().String()
}

// briefingCurrencyFromArgs extracts an optional currency filter from command args.
// e.g. "/briefing USD" → "USD", "/briefing" → "".
func briefingCurrencyFromArgs(args string) string {
	return strings.ToUpper(strings.TrimSpace(args))
}
