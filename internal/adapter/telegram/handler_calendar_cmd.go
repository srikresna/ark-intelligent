package telegram

// /calendar — Economic Calendar & Navigation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// /calendar & Callbacks — Economic Calendar
// ---------------------------------------------------------------------------

// sendCalendarChunked sends or edits a calendar message, automatically chunking
// if the HTML exceeds Telegram's 4096-char limit. The keyboard is always
// attached to the last chunk.
//   - msgID == 0 → new message (send)
//   - msgID >  0 → edit existing message, overflow as new messages
func (h *Handler) sendCalendarChunked(ctx context.Context, chatID string, msgID int, html string, kb ports.InlineKeyboard) error {
	if msgID > 0 {
		return h.bot.EditWithKeyboardChunked(ctx, chatID, msgID, html, kb)
	}
	_, err := h.bot.SendWithKeyboardChunked(ctx, chatID, html, kb)
	return err
}

func (h *Handler) cmdCalendar(ctx context.Context, chatID string, userID int64, args string) error {
	now := timeutil.NowWIB()

	// Load saved filter preference (fallback to "all")
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	savedFilter := prefs.CalendarFilter
	if savedFilter == "" {
		savedFilter = "all"
	}

	if strings.ToLower(strings.TrimSpace(args)) == "week" {
		events, err := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))
		if err != nil {
			h.sendUserError(ctx, chatID, err, "calendar")
			return nil
		}
		html := h.fmt.FormatCalendarWeek(now.Format("Jan 02, 2006"), events, savedFilter)
		kb := h.kb.CalendarFilter(savedFilter, now.Format("20060102"), true)
		return h.sendCalendarChunked(ctx, chatID, 0, html, kb)
	}

	dateStr := now.Format("20060102")
	events, err := h.newsRepo.GetByDate(ctx, dateStr)
	if err != nil {
		h.sendUserError(ctx, chatID, err, "calendar")
		return nil
	}

	html := h.fmt.FormatCalendarDay(now.Format("Mon Jan 02, 2006"), events, savedFilter)
	kb := h.kb.CalendarFilter(savedFilter, dateStr, false)
	return h.sendCalendarChunked(ctx, chatID, 0, html, kb)
}

func (h *Handler) cbNewsFilter(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// Callback formats:
	//   cal:filter:all:20260317:day
	//   cal:filter:high:20260317:week
	//   cal:filter:med:20260317:day
	//   cal:filter:cur:USD:20260317:week   ← currency filter has extra segment
	action := strings.TrimPrefix(data, "cal:filter:")
	parts := strings.Split(action, ":")

	filter := "all"
	dateStr := timeutil.NowWIB().Format("20060102")
	isWeek := false

	if len(parts) == 0 {
		// nothing to parse
	} else if parts[0] == "cur" && len(parts) >= 4 {
		// cal:filter:cur:USD:20260317:week
		filter = "cur:" + parts[1]
		dateStr = parts[2]
		isWeek = len(parts) > 3 && parts[3] == "week"
	} else if len(parts) >= 3 {
		// cal:filter:all:20260317:day  or  cal:filter:high:20260317:week
		filter = parts[0]
		dateStr = parts[1]
		isWeek = parts[2] == "week"
	} else if len(parts) >= 2 {
		filter = parts[0]
		dateStr = parts[1]
	} else {
		filter = parts[0]
	}

	// Persist the chosen filter for this user
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	prefs.CalendarFilter = filter
	if isWeek {
		prefs.CalendarView = "week"
	} else {
		prefs.CalendarView = "day"
	}
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	var events []domain.NewsEvent
	var err error
	if isWeek {
		events, err = h.newsRepo.GetByWeek(ctx, dateStr)
	} else {
		events, err = h.newsRepo.GetByDate(ctx, dateStr)
	}
	if err != nil {
		return h.bot.EditMessage(ctx, chatID, msgID, "Failed to refresh calendar")
	}

	// BUG #6 FIX: parse dateStr in WIB timezone so the label matches the user's date.
	// time.Parse() returns UTC — on a WIB system the formatted label can be off by 1 day
	// at midnight boundary (e.g. 00:30 WIB = 17:30 UTC prev day).
	t, _ := time.ParseInLocation("20060102", dateStr, timeutil.WIB)
	var html string
	if isWeek {
		html = h.fmt.FormatCalendarWeek(t.Format("Jan 02, 2006"), events, filter)
	} else {
		html = h.fmt.FormatCalendarDay(t.Format("Mon Jan 02, 2006"), events, filter)
	}

	kb := h.kb.CalendarFilter(filter, dateStr, isWeek)
	return h.sendCalendarChunked(ctx, chatID, msgID, html, kb)
}

func (h *Handler) cbNewsNav(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "cal:nav:")
	parts := strings.Split(action, ":")
	if len(parts) < 2 {
		return nil
	}
	navType := parts[0]
	dateStr := parts[1]

	// Handle month navigation separately (no day-level targetDate needed)
	if navType == "prevmonth" || navType == "thismonth" || navType == "nextmonth" {
		return h.handleMonthNav(ctx, chatID, msgID, navType, dateStr)
	}

	// BUG #6 FIX: parse with WIB timezone to keep label consistent with WIB date boundary.
	t, err := time.ParseInLocation("20060102", dateStr, timeutil.WIB)
	if err != nil {
		return nil
	}

	isWeek := false
	targetDate := t

	switch navType {
	case "prev":
		targetDate = t.AddDate(0, 0, -1)
	case "next":
		targetDate = t.AddDate(0, 0, 1)
	case "week":
		isWeek = true
	case "prevwk":
		isWeek = true
		targetDate = t.AddDate(0, 0, -7)
	case "nextwk":
		isWeek = true
		targetDate = t.AddDate(0, 0, 7)
	case "day":
		isWeek = false
	}

	targetDateStr := targetDate.Format("20060102")

	var events []domain.NewsEvent
	if isWeek {
		events, _ = h.newsRepo.GetByWeek(ctx, targetDateStr)
	} else {
		events, _ = h.newsRepo.GetByDate(ctx, targetDateStr)
	}

	if len(events) == 0 {
		_ = h.bot.EditMessage(ctx, chatID, msgID, "Fetching calendar from MQL5... (15s) ⏳")
		if isWeek {
			rangeType := "this"
			if targetDate.After(timeutil.NowWIB()) {
				rangeType = "next"
			}
			events, _ = h.newsFetcher.ScrapeCalendar(ctx, rangeType)
			_ = h.newsRepo.SaveEvents(ctx, events)
			events, _ = h.newsRepo.GetByWeek(ctx, targetDateStr)
		} else {
			events, _ = h.newsFetcher.ScrapeActuals(ctx, targetDateStr)
			_ = h.newsRepo.SaveEvents(ctx, events)
			events, _ = h.newsRepo.GetByDate(ctx, targetDateStr)
		}
	}

	// Load saved filter preference (instead of resetting to "all" on nav)
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	activeFilter := prefs.CalendarFilter
	if activeFilter == "" {
		activeFilter = "all"
	}
	var htmlStr string
	if isWeek {
		htmlStr = h.fmt.FormatCalendarWeek(targetDate.Format("Jan 02, 2006"), events, activeFilter)
	} else {
		htmlStr = h.fmt.FormatCalendarDay(targetDate.Format("Mon Jan 02, 2006"), events, activeFilter)
	}

	kb := h.kb.CalendarFilter(activeFilter, targetDateStr, isWeek)
	return h.sendCalendarChunked(ctx, chatID, msgID, htmlStr, kb)
}

// cbQuickCommand handles "cmd:" prefixed callbacks, routing them to the
// corresponding command handler. This enables inline keyboard buttons to
// invoke the same logic as slash commands.
func (h *Handler) cbQuickCommand(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "cmd:")

	// Check for commands with arguments (e.g. "seasonal:EUR")
	var cmd, args string
	if idx := strings.Index(action, ":"); idx >= 0 {
		cmd = action[:idx]
		args = action[idx+1:]
	} else {
		cmd = action
	}

	switch cmd {
	case "bias":
		return h.cmdBias(ctx, chatID, userID, args)
	case "macro":
		return h.cmdMacro(ctx, chatID, userID, args)
	case "rank":
		return h.cmdRank(ctx, chatID, userID, args)
	case "calendar":
		return h.cmdCalendar(ctx, chatID, userID, args)
	case "accuracy":
		return h.cmdAccuracy(ctx, chatID, userID, args)
	case "sentiment":
		return h.cmdSentiment(ctx, chatID, userID, args)
	case "seasonal":
		return h.cmdSeasonal(ctx, chatID, userID, args)
	case "backtest":
		return h.cmdBacktest(ctx, chatID, userID, args)
	case "price":
		return h.cmdPrice(ctx, chatID, userID, args)
	case "levels":
		return h.cmdLevels(ctx, chatID, userID, args)
	case "corr", "carry", "intraday", "garch", "hurst", "regime", "factors", "wfopt":
		// These are now handled by /quant
		return h.cmdQuant(ctx, chatID, userID, args)
	case "quant":
		return h.cmdQuant(ctx, chatID, userID, args)
	case "vp":
		return h.cmdVP(ctx, chatID, userID, args)
	default:
		return nil
	}
}

// handleMonthNav handles prevmonth / thismonth / nextmonth navigation.
// dateStr is the reference date from the callback (e.g. "20260301") to compute relative months.

// cbNav handles navigation callbacks (e.g. home button).
func (h *Handler) cbNav(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "nav:")
	switch action {
	case "home":
		// Delete the current message and show the main menu
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.cmdStart(ctx, chatID, userID, "")
	default:
		return nil
	}
}

func (h *Handler) handleMonthNav(ctx context.Context, chatID string, msgID int, navType, dateStr string) error {
	// Parse the reference date from the callback; fall back to "now" if invalid.
	// BUG #6 FIX: parse with WIB timezone for consistency with month boundary in WIB.
	refDate, parseErr := time.ParseInLocation("20060102", dateStr, timeutil.WIB)
	if parseErr != nil {
		refDate = timeutil.NowWIB()
	}

	var targetYear int
	var targetMonth time.Month
	switch navType {
	case "prevmonth":
		prev := refDate.AddDate(0, -1, 0)
		targetYear, targetMonth = prev.Year(), prev.Month()
	case "nextmonth":
		next := refDate.AddDate(0, 1, 0)
		targetYear, targetMonth = next.Year(), next.Month()
	default: // "thismonth"
		now := timeutil.NowWIB()
		targetYear, targetMonth = now.Year(), now.Month()
	}

	yearMonth := fmt.Sprintf("%04d%02d", targetYear, targetMonth)
	// Representative dateStr = first day of that month (for keyboard callbacks)
	targetDateStr := fmt.Sprintf("%04d%02d01", targetYear, targetMonth)

	// Try cache first
	events, _ := h.newsRepo.GetByMonth(ctx, yearMonth)

	if len(events) == 0 {
		_ = h.bot.EditMessage(ctx, chatID, msgID, "Fetching monthly calendar from MQL5... (15-20s) ⏳")
		// Map navType to ScrapeMonth range type
		scrapeRange := "current"
		now := timeutil.NowWIB()
		targetFirst := time.Date(targetYear, targetMonth, 1, 0, 0, 0, 0, time.UTC)
		nowFirst := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		if targetFirst.Before(nowFirst) {
			scrapeRange = "prev"
		} else if targetFirst.After(nowFirst) {
			scrapeRange = "next"
		}
		fetched, err := h.newsFetcher.ScrapeMonth(ctx, scrapeRange)
		if err != nil {
			log.Error().Err(err).Str("range", scrapeRange).Msg("month scrape failed")
			return h.bot.EditMessage(ctx, chatID, msgID, "Failed to fetch monthly calendar. Please try again later.")
		}
		_ = h.newsRepo.SaveEvents(ctx, fetched)
		events, _ = h.newsRepo.GetByMonth(ctx, yearMonth)
	}

	monthLabel := time.Date(targetYear, targetMonth, 1, 0, 0, 0, 0, time.UTC).Format("January 2006")
	html := h.fmt.FormatCalendarMonth(monthLabel, events, "all")
	kb := h.kb.CalendarFilter("all", targetDateStr, true)
	return h.sendCalendarChunked(ctx, chatID, msgID, html, kb)
}
