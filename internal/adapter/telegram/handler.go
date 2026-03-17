package telegram

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// Handler — Wires services to Telegram commands
// ---------------------------------------------------------------------------

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
}

// NewHandler creates a handler and registers all commands on the bot.
func NewHandler(
	bot *Bot,
	eventRepo ports.EventRepository,
	cotRepo ports.COTRepository,
	prefsRepo ports.PrefsRepository,
	newsRepo ports.NewsRepository,
	newsFetcher ports.NewsFetcher,
	aiAnalyzer ports.AIAnalyzer,
) *Handler {
	h := &Handler{
		bot:         bot,
		fmt:         NewFormatter(),
		kb:          NewKeyboardBuilder(),
		eventRepo:   eventRepo,
		cotRepo:     cotRepo,
		prefsRepo:   prefsRepo,
		newsRepo:    newsRepo,
		newsFetcher: newsFetcher,
		aiAnalyzer:  aiAnalyzer,
	}

	// Register all commands
	bot.RegisterCommand("/start", h.cmdStart)
	bot.RegisterCommand("/help", h.cmdHelp)
	bot.RegisterCommand("/settings", h.cmdSettings)
	bot.RegisterCommand("/status", h.cmdStatus)
	bot.RegisterCommand("/cot", h.cmdCOT)
	bot.RegisterCommand("/outlook", h.cmdOutlook)
	bot.RegisterCommand("/calendar", h.cmdCalendar)

	// Register callback handlers
	bot.RegisterCallback("cot:", h.cbCOTDetail)
	bot.RegisterCallback("alert:", h.cbAlertToggle)
	bot.RegisterCallback("set:", h.cbSettings)
	bot.RegisterCallback("cal:filter:", h.cbNewsFilter)
	bot.RegisterCallback("out:", h.cbOutlook)
	bot.RegisterCallback("cal:nav:", h.cbNewsNav)

	log.Printf("[HANDLER] Registered 14 commands and 7 callback prefixes")
	return h
}

// ---------------------------------------------------------------------------
// /start & /help — Onboarding
// ---------------------------------------------------------------------------

func (h *Handler) cmdStart(ctx context.Context, chatID string, userID int64, args string) error {
	html := `🦅 <b>ARK Intelligence Terminal</b>
<i>Institutional Flow & Macro Analytics</i>

<b>📊 COT Positioning</b>
/cot - Overview semua currency
/cot USD - Detail spesifik currency
/cot raw USD - Raw data positioning

<b>📅 Economic Calendar</b>
/calendar - Agenda hari ini
/calendar week - Agenda minggu ini
<i>Gunakan tombol navigasi untuk pindah hari/minggu</i>

<b>🧠 AI Intelligence Outlook</b>
/outlook cot - COT Positioning structural analysis
/outlook news - News catalysts, Storm Days & Central Bank
/outlook combine - Fused COT + News catalyst triggers

<b>⚙️ Operations</b>
/settings - Preference management
/status - System status

<code>ARK Interface v1.0.0</code>`

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

	html := h.fmt.FormatCOTDetail(*analysis)

	// Add AI interpretation if it's a new message
	if editMsgID == 0 && h.aiAnalyzer != nil && h.aiAnalyzer.IsAvailable() {
		narrative, aiErr := h.aiAnalyzer.AnalyzeCOT(ctx, []domain.COTAnalysis{*analysis})
		if aiErr == nil && narrative != "" {
			html += "\n\n" + h.fmt.FormatAIInsight("COT Analysis", narrative)
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
		html := "🦅 <b>ARK Intelligence Outlook</b>\nSelect the type of market analysis you want to generate:"
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
	} else if subcmd == "combine" {
		cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
		weekEvts, _ := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))
		weeklyData := ports.WeeklyData{COTAnalyses: cotAnalyses, NewsEvents: weekEvts, Language: prefs.Language}
		result, err = h.aiAnalyzer.AnalyzeCombinedOutlook(ctx, weeklyData)
	} else { // "cot" or default
		cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
		weeklyData := ports.WeeklyData{COTAnalyses: cotAnalyses, Language: prefs.Language}
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
		// Read CHANGELOG.md
		content, err := os.ReadFile("CHANGELOG.md")
		if err != nil {
			log.Printf("[HANDLER] Failed to read CHANGELOG.md: %v", err)
			return h.bot.EditMessage(ctx, chatID, msgID, "Changelog unavailable.")
		}

		html := fmt.Sprintf("🦅 <b>ARK Intelligence Changelog</b>\n\n%s", string(content))
		// Optional: Add a back button to settings
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
	case "time_15_5":
		prefs.AlertMinutes = []int{15, 5}
	case "time_5_1":
		prefs.AlertMinutes = []int{5, 1}
	default:
		log.Printf("[HANDLER] Unknown settings action: %s", action)
		return nil
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

func (h *Handler) cmdCalendar(ctx context.Context, chatID string, userID int64, args string) error {
	now := timeutil.NowWIB()

	if strings.ToLower(strings.TrimSpace(args)) == "week" {
		events, err := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))
		if err != nil {
			_, err = h.bot.SendHTML(ctx, chatID, "Failed to get weekly calendar")
			return err
		}
		html := h.fmt.FormatCalendarWeek(now.Format("Jan 02, 2006"), events, "med")
		kb := h.kb.CalendarFilter("med", now.Format("20060102"), true)
		_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return err
	}

	dateStr := now.Format("20060102")
	events, err := h.newsRepo.GetByDate(ctx, dateStr)
	if err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Failed to get today's calendar")
		return err
	}

	html := h.fmt.FormatCalendarDay(now.Format("Mon Jan 02, 2006"), events, "med")
	kb := h.kb.CalendarFilter("med", dateStr, false)
	_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

func (h *Handler) cbNewsFilter(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "cal:filter:") // e.g., "high:20260317:day"
	parts := strings.Split(action, ":")

	filter := "med"
	dateStr := timeutil.NowWIB().Format("20060102")
	isWeek := false

	if len(parts) > 0 {
		filter = parts[0]
	}
	if len(parts) > 1 {
		dateStr = parts[1]
	}
	if len(parts) > 2 && parts[2] == "week" {
		isWeek = true
	}

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
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
}

func (h *Handler) cbNewsNav(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "cal:nav:")
	parts := strings.Split(action, ":")
	if len(parts) < 2 {
		return nil
	}
	navType := parts[0]
	dateStr := parts[1]

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
		_ = h.bot.EditMessage(ctx, chatID, msgID, "Fetching calendar from Trading Economics... (15s) ⏳")
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

	activeFilter := "med"
	var html string
	if isWeek {
		html = h.fmt.FormatCalendarWeek(targetDate.Format("Jan 02, 2006"), events, activeFilter)
	} else {
		html = h.fmt.FormatCalendarDay(targetDate.Format("Mon Jan 02, 2006"), events, activeFilter)
	}

	kb := h.kb.CalendarFilter(activeFilter, targetDateStr, isWeek)
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
}
