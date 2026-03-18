package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/internal/service/cot"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// Handler — Wires services to Telegram commands
// ---------------------------------------------------------------------------

// SurpriseProvider is a minimal interface allowing the handler to read
// the per-currency accumulated surprise sigma from the news scheduler.
type SurpriseProvider interface {
	GetSurpriseSigma(currency string) float64
}

// Handler holds all service dependencies and registers commands on the bot.
type Handler struct {
	bot *Bot
	fmt *Formatter
	kb  *KeyboardBuilder

	// Repositories
	eventRepo   ports.EventRepository
	cotRepo     ports.COTRepository
	prefsRepo   ports.PrefsRepository
	newsRepo    ports.NewsRepository
	newsFetcher ports.NewsFetcher

	aiAnalyzer ports.AIAnalyzer

	// newsScheduler provides access to per-currency surprise sigma for conviction scoring.
	// May be nil — all callers guard with a nil check.
	newsScheduler SurpriseProvider

	// changelog is the embedded CHANGELOG.md content, injected at startup.
	changelog string
}

// NewHandler creates a handler and registers all commands on the bot.
// newsScheduler may be nil; all callers guard with nil checks before use.
func NewHandler(
	bot *Bot,
	eventRepo ports.EventRepository,
	cotRepo ports.COTRepository,
	prefsRepo ports.PrefsRepository,
	newsRepo ports.NewsRepository,
	newsFetcher ports.NewsFetcher,
	aiAnalyzer ports.AIAnalyzer,
	changelog string,
	newsScheduler SurpriseProvider,
) *Handler {
	h := &Handler{
		bot:           bot,
		fmt:           NewFormatter(),
		kb:            NewKeyboardBuilder(),
		eventRepo:     eventRepo,
		cotRepo:       cotRepo,
		prefsRepo:     prefsRepo,
		newsRepo:      newsRepo,
		newsFetcher:   newsFetcher,
		aiAnalyzer:    aiAnalyzer,
		changelog:     changelog,
		newsScheduler: newsScheduler,
	}

	// Register all commands
	bot.RegisterCommand("/start", h.cmdStart)
	bot.RegisterCommand("/help", h.cmdHelp)
	bot.RegisterCommand("/settings", h.cmdSettings)
	bot.RegisterCommand("/status", h.cmdStatus)
	bot.RegisterCommand("/cot", h.cmdCOT)
	bot.RegisterCommand("/outlook", h.cmdOutlook)
	bot.RegisterCommand("/calendar", h.cmdCalendar)
	bot.RegisterCommand("/rank", h.cmdRank)   // P1.3 — Currency Strength Ranking
	bot.RegisterCommand("/macro", h.cmdMacro) // P3.2 — FRED Macro Regime Dashboard

	// Register callback handlers
	bot.RegisterCallback("cot:", h.cbCOTDetail)
	bot.RegisterCallback("alert:", h.cbAlertToggle)
	bot.RegisterCallback("set:", h.cbSettings)
	bot.RegisterCallback("cal:filter:", h.cbNewsFilter)
	bot.RegisterCallback("out:", h.cbOutlook)
	bot.RegisterCallback("cal:nav:", h.cbNewsNav)

	log.Printf("[HANDLER] Registered 9 commands and 6 callback prefixes")
	return h
}

// ---------------------------------------------------------------------------
// /start & /help — Onboarding
// ---------------------------------------------------------------------------

func (h *Handler) cmdStart(ctx context.Context, chatID string, userID int64, args string) error {
	// Persist chatID so the scheduler can push alerts to this user.
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	if prefs.ChatID != chatID {
		prefs.ChatID = chatID
		_ = h.prefsRepo.Set(ctx, userID, prefs)
	}

	html := `🦅 <b>ARK Intelligence Terminal</b>
<i>Institutional Flow & Macro Analytics</i>

<b>📊 COT Positioning</b>
/cot - Overview semua currency
/cot USD - Detail + Upcoming Catalysts 48h
/cot raw USD - Raw data positioning

<b>🏆 Rankings & Regime</b>
/rank - Currency strength ranking mingguan
/macro - FRED Macro regime dashboard (7 indicators)

<b>📅 Economic Calendar</b>
/calendar - Agenda hari ini
/calendar week - Agenda minggu ini
<i>Gunakan tombol navigasi untuk pindah hari/minggu</i>

<b>🧠 AI Intelligence Outlook</b>
/outlook cot - COT Positioning structural analysis
/outlook news - News catalysts, Storm Days &amp; Central Bank
/outlook fred - FRED Macro deep-dive (Fed policy, real rates, DXY)
/outlook combine - Fused COT + News + FRED macro triggers

<b>⚙️ Operations</b>
/settings - Preference management
/status - System status

<code>ARK Interface v2.1.0</code>`

	_, err := h.bot.SendHTML(ctx, chatID, html)
	return err
}

func (h *Handler) cmdHelp(ctx context.Context, chatID string, userID int64, args string) error {
	return h.cmdStart(ctx, chatID, userID, args)
}

// ---------------------------------------------------------------------------
// /cot — COT positioning analysis
// ---------------------------------------------------------------------------

func (h *Handler) cmdCOT(ctx context.Context, chatID string, userID int64, args string) error {
	// If specific currency requested: /cot USD or /cot raw USD
	if args != "" {
		parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
		isRaw := false
		code := parts[0]

		if parts[0] == "RAW" {
			isRaw = true
			if len(parts) > 1 {
				code = parts[1]
			} else {
				code = ""
			}
		} else if parts[0] == "ANALYSIS" {
			if len(parts) > 1 {
				code = parts[1]
			} else {
				code = ""
			}
		} else if len(parts) > 1 && parts[1] == "RAW" {
			isRaw = true
		}

		if code != "" {
			contractCode := currencyToContractCode(code)
			return h.sendCOTDetail(ctx, chatID, contractCode, code, isRaw, 0)
		}
	}

	// Overview: all currencies
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil {
		return fmt.Errorf("get all COT analyses: %w", err)
	}

	if len(analyses) == 0 {
		_, err = h.bot.SendHTML(ctx, chatID,
			"No COT data available yet. Data is fetched from CFTC every Friday.")
		return err
	}

	html := h.fmt.FormatCOTOverview(analyses)
	kb := h.kb.COTCurrencySelector(analyses)
	_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

func (h *Handler) sendCOTDetail(ctx context.Context, chatID string, contractCode, displayCode string, isRaw bool, editMsgID int) error {
	if isRaw {
		records, err := h.cotRepo.GetHistory(ctx, contractCode, 1)
		if err != nil || len(records) == 0 {
			msg := fmt.Sprintf("No COT data for %s", displayCode)
			if editMsgID > 0 {
				return h.bot.EditMessage(ctx, chatID, editMsgID, msg)
			}
			_, e := h.bot.SendHTML(ctx, chatID, msg)
			return e
		}

		html := h.fmt.FormatCOTRaw(records[0])
		kb := h.kb.COTDetailMenu(contractCode, true)
		if editMsgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, editMsgID, html, kb)
		}
		_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return err
	}

	analysis, err := h.cotRepo.GetLatestAnalysis(ctx, contractCode)
	if err != nil || analysis == nil {
		msg := fmt.Sprintf("No COT data for %s", displayCode)
		if editMsgID > 0 {
			return h.bot.EditMessage(ctx, chatID, editMsgID, msg)
		}
		_, e := h.bot.SendHTML(ctx, chatID, msg)
		return e
	}

	html := h.fmt.FormatCOTDetailWithCode(*analysis, displayCode)

	// Add AI interpretation if it's a new message
	if editMsgID == 0 && h.aiAnalyzer != nil && h.aiAnalyzer.IsAvailable() {
		narrative, aiErr := h.aiAnalyzer.AnalyzeCOT(ctx, []domain.COTAnalysis{*analysis})
		if aiErr == nil && narrative != "" {
			html += "\n\n" + h.fmt.FormatAIInsight("COT Analysis", narrative)
		}
	}

	// Inject FRED macro context (non-fatal if fails — uses cache if available)
	if editMsgID == 0 {
		macroData, fredErr := fred.GetCachedOrFetch(ctx)
		if fredErr == nil && macroData != nil {
			regime := fred.ClassifyMacroRegime(macroData)
			fredCtx := h.fmt.FormatFREDContext(macroData, regime)
			if fredCtx != "" {
				html += fredCtx
			}
		}
	}

	// Gap D — Conviction Score for this currency (COT + FRED + Calendar fused)
	if editMsgID == 0 && analysis != nil {
		macroData2, fredErr2 := fred.GetCachedOrFetch(ctx)
		if fredErr2 == nil && macroData2 != nil {
			regime2 := fred.ClassifyMacroRegime(macroData2)
			surpriseSigma2 := 0.0
			if h.newsScheduler != nil {
				surpriseSigma2 = h.newsScheduler.GetSurpriseSigma(analysis.Contract.Currency)
			}
			cs := cot.ComputeConvictionScore(*analysis, regime2, surpriseSigma2, "", macroData2)
			html += h.fmt.FormatConvictionBlock(cs)
		}
	}

	// P1.4 — Upcoming Catalysts: fetch events for next 48h for this currency
	if editMsgID == 0 && h.newsRepo != nil {
		now := timeutil.NowWIB()
		today := now.Format("20060102")
		tomorrow := now.AddDate(0, 0, 1).Format("20060102")

		todayEvts, _ := h.newsRepo.GetByDate(ctx, today)
		tomorrowEvts, _ := h.newsRepo.GetByDate(ctx, tomorrow)

		upcoming := append(todayEvts, tomorrowEvts...) //nolint:gocritic
		currency := analysis.Contract.Currency
		catalysts := h.fmt.FormatUpcomingCatalysts(currency, upcoming)
		if catalysts != "" {
			html += catalysts
		}
	}

	kb := h.kb.COTDetailMenu(contractCode, false)
	if editMsgID > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, editMsgID, html, kb)
	}
	_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

// cbCOTDetail handles inline keyboard callback for COT detail view.
func (h *Handler) cbCOTDetail(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// data format: "cot:analysis:099741", "cot:raw:099741", "cot:overview"
	action := strings.TrimPrefix(data, "cot:")

	if action == "overview" {
		analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
		if err != nil || len(analyses) == 0 {
			return h.bot.EditMessage(ctx, chatID, msgID, "No COT data available.")
		}
		html := h.fmt.FormatCOTOverview(analyses)
		kb := h.kb.COTCurrencySelector(analyses)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	}

	parts := strings.Split(action, ":")
	if len(parts) != 2 {
		return nil
	}

	isRaw := parts[0] == "raw"
	contractCode := parts[1]

	return h.sendCOTDetail(ctx, chatID, contractCode, contractCode, isRaw, msgID)
}

// ---------------------------------------------------------------------------
// /outlook — AI weekly market outlook
// ---------------------------------------------------------------------------

func (h *Handler) cmdOutlook(ctx context.Context, chatID string, userID int64, args string) error {
	if h.aiAnalyzer == nil || !h.aiAnalyzer.IsAvailable() {
		_, err := h.bot.SendHTML(ctx, chatID, "AI outlook is unavailable. Gemini API key not configured.")
		return err
	}

	subcmd := strings.ToLower(strings.TrimSpace(args))
	if subcmd == "" {
		html := "🦅 <b>ARK Intelligence Outlook</b>\nSelect the type of market analysis you want to generate:\n\n" +
			"<i>Tip: </i><code>/outlook cot</code> | <code>/outlook news</code> | <code>/outlook fred</code> | <code>/outlook combine</code>"
		kb := h.kb.OutlookMenu()
		_, err := h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return err
	}

	return h.generateOutlook(ctx, chatID, userID, subcmd, 0)
}

func (h *Handler) cbOutlook(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "out:") // cot, news, combine
	return h.generateOutlook(ctx, chatID, userID, action, msgID)
}

func (h *Handler) generateOutlook(ctx context.Context, chatID string, userID int64, subcmd string, editMsgID int) error {
	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		prefs = domain.DefaultPrefs()
	}

	placeholderID := 0
	if editMsgID > 0 {
		_ = h.bot.EditMessage(ctx, chatID, editMsgID, "Generating intelligence report... (10-15s) ⏳")
		placeholderID = editMsgID
	} else {
		placeholderID, _ = h.bot.SendHTML(ctx, chatID, "Generating intelligence report... (10-15s) ⏳")
	}

	now := timeutil.NowWIB()
	var result string

	if subcmd == "news" {
		weekEvts, fetchErr := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))
		if fetchErr != nil {
			_ = h.bot.EditMessage(ctx, chatID, placeholderID, "Failed to load news for analysis.")
			return fetchErr
		}
		result, err = h.aiAnalyzer.AnalyzeNewsOutlook(ctx, weekEvts, prefs.Language)
	} else if subcmd == "fred" {
		// Use cached FRED data (or fetch fresh if stale) then run AI analysis
		macroData, fredErr := fred.GetCachedOrFetch(ctx)
		if fredErr != nil || macroData == nil {
			_ = h.bot.EditMessage(ctx, chatID, placeholderID, "Failed to fetch FRED macro data. Check FRED_API_KEY.")
			return fredErr
		}
		result, err = h.aiAnalyzer.AnalyzeFREDOutlook(ctx, macroData, prefs.Language)
	} else if subcmd == "combine" {
		cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
		weekEvts, _ := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))
		// Use cached FRED data — non-fatal if it fails
		macroData, _ := fred.GetCachedOrFetch(ctx)
		weeklyData := ports.WeeklyData{
			COTAnalyses: cotAnalyses,
			NewsEvents:  weekEvts,
			MacroData:   macroData,
			Language:    prefs.Language,
		}
		result, err = h.aiAnalyzer.AnalyzeCombinedOutlook(ctx, weeklyData)
	} else { // "cot" or default
		cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
		// Gap E — pass MacroData so FRED regime context is injected into the /outlook cot prompt
		macroData, _ := fred.GetCachedOrFetch(ctx)
		weeklyData := ports.WeeklyData{
			COTAnalyses: cotAnalyses,
			MacroData:   macroData,
			Language:    prefs.Language,
		}
		result, err = h.aiAnalyzer.GenerateWeeklyOutlook(ctx, weeklyData)
	}

	if err != nil {
		return h.bot.EditMessage(ctx, chatID, placeholderID, fmt.Sprintf("AI Generation failed: %v", err))
	}

	html := h.fmt.FormatWeeklyOutlook(result, now)
	if editMsgID > 0 {
		return h.bot.EditMessage(ctx, chatID, editMsgID, html)
	}
	_ = h.bot.DeleteMessage(ctx, chatID, placeholderID)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------
// /settings — User preferences
// ---------------------------------------------------------------------------

func (h *Handler) cmdSettings(ctx context.Context, chatID string, userID int64, args string) error {
	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get preferences: %w", err)
	}

	html := h.fmt.FormatSettings(prefs)
	kb := h.kb.SettingsMenu(prefs)
	_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

// cbSettings handles settings toggle callbacks.
func (h *Handler) cbSettings(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "set:")

	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return err
	}

	switch action {
	case "lang_toggle":
		if prefs.Language == "en" {
			prefs.Language = "id"
		} else {
			prefs.Language = "en"
		}
	case "changelog_view":
		if h.changelog == "" {
			return h.bot.EditMessage(ctx, chatID, msgID, "Changelog unavailable.")
		}
		html := fmt.Sprintf("🦅 <b>ARK Intelligence Changelog</b>\n\n%s", h.changelog)
		kb := h.kb.SettingsMenu(prefs)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)

	case "alerts_toggle":
		prefs.AlertsEnabled = !prefs.AlertsEnabled
	case "cot_toggle":
		prefs.COTAlertsEnabled = !prefs.COTAlertsEnabled
	case "ai_toggle":
		prefs.AIReportsEnabled = !prefs.AIReportsEnabled
	case "impact_high_only":
		prefs.AlertImpacts = []string{"High"}
	case "impact_high_med":
		prefs.AlertImpacts = []string{"High", "Medium"}
	case "impact_all":
		prefs.AlertImpacts = []string{"High", "Medium", "Low"}
	case "time_60_15_5":
		prefs.AlertMinutes = []int{60, 15, 5}
	case "time_15_5_1":
		prefs.AlertMinutes = []int{15, 5, 1}
	case "time_5_1":
		prefs.AlertMinutes = []int{5, 1}
	case "cur_reset":
		prefs.CurrencyFilter = nil
	default:
		// Handle cur_toggle:XXX dynamically
		if strings.HasPrefix(action, "cur_toggle:") {
			cur := strings.ToUpper(strings.TrimPrefix(action, "cur_toggle:"))
			if cur != "" {
				found := false
				newFilter := make([]string, 0, len(prefs.CurrencyFilter))
				for _, c := range prefs.CurrencyFilter {
					if strings.ToUpper(c) == cur {
						found = true
						// Skip it (remove)
					} else {
						newFilter = append(newFilter, c)
					}
				}
				if !found {
					newFilter = append(newFilter, cur)
				}
				prefs.CurrencyFilter = newFilter
			}
		} else {
			log.Printf("[HANDLER] Unknown settings action: %s", action)
			return nil
		}
	}

	if err := h.prefsRepo.Set(ctx, userID, prefs); err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}

	// Update the message with new settings state
	html := h.fmt.FormatSettings(prefs)
	kb := h.kb.SettingsMenu(prefs)
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
}

// cbAlertToggle handles quick alert toggle from notification messages.
func (h *Handler) cbAlertToggle(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "alert:")

	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		return err
	}

	switch action {
	case "mute_1h":
		// Temporarily disable alerts (would need a timer mechanism)
		prefs.AlertsEnabled = false
		_ = h.prefsRepo.Set(ctx, userID, prefs)
		return h.bot.EditMessage(ctx, chatID, msgID,
			"Alerts muted. Use /settings to re-enable.")
	case "dismiss":
		return h.bot.DeleteMessage(ctx, chatID, msgID)
	}

	return nil
}

func (h *Handler) cmdStatus(ctx context.Context, chatID string, userID int64, args string) error {
	now := timeutil.NowWIB()

	// Check data freshness
	cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)

	// AI status
	aiStatus := "Not configured"
	if h.aiAnalyzer != nil {
		if h.aiAnalyzer.IsAvailable() {
			aiStatus = "Available"
		} else {
			aiStatus = "Configured but unavailable"
		}
	}

	html := fmt.Sprintf(`<b>System Status</b>
<code>Time:       %s WIB</code>

<b>Data Sources:</b>
<code>COT:        %d contracts</code>

<b>Services:</b>
<code>AI Engine:  %s</code>

<b>Version:</b> v1.0.0`,
		now.Format("15:04:05"),
		len(cotAnalyses),
		aiStatus,
	)

	_, err := h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------
// Currency-to-contract mapping
// ---------------------------------------------------------------------------

// currencyToContractCode maps 3-letter currency codes to CFTC contract codes.
func currencyToContractCode(currency string) string {
	mapping := map[string]string{
		"EUR":  "099741", // Euro FX
		"GBP":  "096742", // British Pound
		"JPY":  "097741", // Japanese Yen
		"AUD":  "232741", // Australian Dollar
		"NZD":  "112741", // New Zealand Dollar
		"CAD":  "090741", // Canadian Dollar
		"CHF":  "092741", // Swiss Franc
		"USD":  "098662", // US Dollar Index
		"GOLD": "088691", // Gold
		"XAU":  "088691", // Gold alias
		"OIL":  "067651", // Crude Oil
	}

	if code, ok := mapping[strings.ToUpper(currency)]; ok {
		return code
	}
	return currency // Return as-is if not mapped
}

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
			_, err = h.bot.SendHTML(ctx, chatID, "Failed to get weekly calendar")
			return err
		}
		html := h.fmt.FormatCalendarWeek(now.Format("Jan 02, 2006"), events, savedFilter)
		kb := h.kb.CalendarFilter(savedFilter, now.Format("20060102"), true)
		return h.sendCalendarChunked(ctx, chatID, 0, html, kb)
	}

	dateStr := now.Format("20060102")
	events, err := h.newsRepo.GetByDate(ctx, dateStr)
	if err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Failed to get today's calendar")
		return err
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

	t, _ := time.Parse("20060102", dateStr)
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

	t, err := time.Parse("20060102", dateStr)
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

	activeFilter := "all"
	var html string
	if isWeek {
		html = h.fmt.FormatCalendarWeek(targetDate.Format("Jan 02, 2006"), events, activeFilter)
	} else {
		html = h.fmt.FormatCalendarDay(targetDate.Format("Mon Jan 02, 2006"), events, activeFilter)
	}

	kb := h.kb.CalendarFilter(activeFilter, targetDateStr, isWeek)
	return h.sendCalendarChunked(ctx, chatID, msgID, html, kb)
}

// handleMonthNav handles prevmonth / thismonth / nextmonth navigation.
func (h *Handler) handleMonthNav(ctx context.Context, chatID string, msgID int, navType, _ string) error {
	var monthType string
	switch navType {
	case "prevmonth":
		monthType = "prev"
	case "nextmonth":
		monthType = "next"
	default: // "thismonth"
		monthType = "current"
	}

	now := timeutil.NowWIB()
	var targetYear int
	var targetMonth time.Month
	switch monthType {
	case "prev":
		prev := now.AddDate(0, -1, 0)
		targetYear, targetMonth = prev.Year(), prev.Month()
	case "next":
		next := now.AddDate(0, 1, 0)
		targetYear, targetMonth = next.Year(), next.Month()
	default:
		targetYear, targetMonth = now.Year(), now.Month()
	}

	yearMonth := fmt.Sprintf("%04d%02d", targetYear, targetMonth)
	// Representative dateStr = first day of that month (for keyboard callbacks)
	targetDateStr := fmt.Sprintf("%04d%02d01", targetYear, targetMonth)

	// Try cache first
	events, _ := h.newsRepo.GetByMonth(ctx, yearMonth)

	if len(events) == 0 {
		_ = h.bot.EditMessage(ctx, chatID, msgID, "Fetching monthly calendar from MQL5... (15-20s) ⏳")
		fetched, err := h.newsFetcher.ScrapeMonth(ctx, monthType)
		if err != nil {
			return h.bot.EditMessage(ctx, chatID, msgID, fmt.Sprintf("Failed to fetch month: %v", err))
		}
		_ = h.newsRepo.SaveEvents(ctx, fetched)
		events, _ = h.newsRepo.GetByMonth(ctx, yearMonth)
	}

	monthLabel := time.Date(targetYear, targetMonth, 1, 0, 0, 0, 0, time.UTC).Format("January 2006")
	html := h.fmt.FormatCalendarMonth(monthLabel, events, "all")
	kb := h.kb.CalendarFilter("all", targetDateStr, true)
	return h.sendCalendarChunked(ctx, chatID, msgID, html, kb)
}

// ---------------------------------------------------------------------------
// P1.3 — /rank — Currency Strength Ranking
// ---------------------------------------------------------------------------

// cmdRank handles the /rank command — weekly currency strength ranking.
// Ranks 8 major currencies by COT SentimentScore and shows conviction scores (COT + FRED + Calendar).
func (h *Handler) cmdRank(ctx context.Context, chatID string, userID int64, args string) error {
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		_, err = h.bot.SendHTML(ctx, chatID,
			"No COT data available for ranking. Data is fetched from CFTC every Friday.")
		return err
	}

	// Fetch FRED regime for conviction scoring (best-effort, non-fatal)
	var macroData *fred.MacroData
	var regime *fred.MacroRegime
	if md, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && md != nil {
		macroData = md
		r := fred.ClassifyMacroRegime(md)
		regime = &r
	}

	// Compute conviction scores for each currency (full 3-source: COT + FRED + Calendar)
	convictions := make([]cot.ConvictionScore, 0, len(analyses))
	for _, a := range analyses {
		var r fred.MacroRegime
		if regime != nil {
			r = *regime
		}
		// Pull per-currency weekly surprise sigma from accumulator (0.0 if not available)
		surpriseSigma := 0.0
		if h.newsScheduler != nil {
			surpriseSigma = h.newsScheduler.GetSurpriseSigma(a.Contract.Currency)
		}
		cs := cot.ComputeConvictionScore(a, r, surpriseSigma, "", macroData)
		convictions = append(convictions, cs)
	}

	now := timeutil.NowWIB()
	html := h.fmt.FormatRankingWithConviction(analyses, convictions, regime, now)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------
// P3.2 — /macro — FRED Macro Regime Dashboard
// ---------------------------------------------------------------------------

// cmdMacro handles the /macro command — fetches FRED data and displays macro regime.
// Usage: /macro (uses cache) or /macro refresh (force re-fetch from FRED).
func (h *Handler) cmdMacro(ctx context.Context, chatID string, userID int64, args string) error {
	forceRefresh := strings.EqualFold(strings.TrimSpace(args), "refresh")
	if forceRefresh {
		fred.InvalidateCache()
	}

	cacheStatus := "🏦 Fetching FRED macro data... ⏳ (5-15s)"
	if !forceRefresh && fred.CacheAge() >= 0 {
		cacheStatus = "🏦 Loading FRED macro data (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendHTML(ctx, chatID, cacheStatus)

	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil {
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			fmt.Sprintf("Failed to fetch FRED data: %v\n\nMake sure FRED_API_KEY is set in .env", err))
	}

	regime := fred.ClassifyMacroRegime(data)
	html := h.fmt.FormatMacroRegime(regime, data)

	return h.bot.EditMessage(ctx, chatID, placeholderID, html)
}
