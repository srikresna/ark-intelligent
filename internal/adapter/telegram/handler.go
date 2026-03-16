package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

// ---------------------------------------------------------------------------
// Handler — Wires services to Telegram commands
// ---------------------------------------------------------------------------

// Handler holds all service dependencies and registers commands on the bot.
type Handler struct {
	bot        *Bot
	fmt        *Formatter
	kb         *KeyboardBuilder

	// Repositories
	eventRepo    ports.EventRepository
	cotRepo      ports.COTRepository
	prefsRepo    ports.PrefsRepository

	aiAnalyzer ports.AIAnalyzer
}

// NewHandler creates a handler and registers all commands on the bot.
func NewHandler(
	bot *Bot,
	eventRepo ports.EventRepository,
	cotRepo ports.COTRepository,
	prefsRepo ports.PrefsRepository,
	aiAnalyzer ports.AIAnalyzer,
) *Handler {
	h := &Handler{
		bot:          bot,
		fmt:          NewFormatter(),
		kb:           NewKeyboardBuilder(),
		eventRepo:    eventRepo,
		cotRepo:      cotRepo,
		prefsRepo:    prefsRepo,
		aiAnalyzer:   aiAnalyzer,
	}

	// Register all commands
	bot.RegisterCommand("/start", h.cmdStart)
	bot.RegisterCommand("/help", h.cmdHelp)
	bot.RegisterCommand("/settings", h.cmdSettings)
	bot.RegisterCommand("/status", h.cmdStatus)
	bot.RegisterCommand("/cot", h.cmdCOT)
	bot.RegisterCommand("/outlook", h.cmdOutlook)

	// Register callback handlers
	bot.RegisterCallback("cot:", h.cbCOTDetail)
	bot.RegisterCallback("alert:", h.cbAlertToggle)
	bot.RegisterCallback("set:", h.cbSettings)

	log.Printf("[HANDLER] Registered 13 commands and 4 callback prefixes")
	return h
}

// ---------------------------------------------------------------------------
// /start & /help — Onboarding
// ---------------------------------------------------------------------------

func (h *Handler) cmdStart(ctx context.Context, chatID string, userID int64, args string) error {
	html := `<b>FF Calendar Bot v2</b>

Institutional-grade forex fundamental analysis (COT Focus):

<b>Analysis Commands:</b>
/cot - COT positioning analysis
/outlook - AI weekly market outlook

<b>System Commands:</b>
/settings - Alert preferences
/status - Bot health status
/help - Show this menu`

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
		_, err := h.bot.SendHTML(ctx, chatID,
			"AI outlook is unavailable. Gemini API key not configured.")
		return err
	}

	// Send "generating..." placeholder
	placeholderID, _ := h.bot.SendHTML(ctx, chatID, "Generating weekly outlook... (this may take 10-15s)")

	// Gather all data
	now := timeutil.NowWIB()
	cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
	weeklyData := ports.WeeklyData{
		COTAnalyses: cotAnalyses,
	}

	outlook, err := h.aiAnalyzer.GenerateWeeklyOutlook(ctx, weeklyData)
	if err != nil {
		_ = h.bot.EditMessage(ctx, chatID, placeholderID,
			fmt.Sprintf("Failed to generate outlook: %v", err))
		return err
	}

	html := h.fmt.FormatWeeklyOutlook(outlook, now)

	// Delete placeholder and send full outlook
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

<b>Version:</b> v2.0.0`,
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
		"EUR": "099741", // Euro FX
		"GBP": "096742", // British Pound
		"JPY": "097741", // Japanese Yen
		"AUD": "232741", // Australian Dollar
		"NZD": "112741", // New Zealand Dollar
		"CAD": "090741", // Canadian Dollar
		"CHF": "092741", // Swiss Franc
		"USD": "098662", // US Dollar Index
		"GOLD": "088691", // Gold
		"XAU":  "088691", // Gold alias
		"OIL":  "067651", // Crude Oil
	}

	if code, ok := mapping[strings.ToUpper(currency)]; ok {
		return code
	}
	return currency // Return as-is if not mapped
}
