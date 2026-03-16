package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/arkcode369/ff-calendar-bot/internal/adapter/renderer"
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
	bot.RegisterCallback("cot:", h.cbCOT) // changed to a unified cbCOT
	bot.RegisterCallback("alert:", h.cbAlertToggle)
	bot.RegisterCallback("set:", h.cbSettings)
	bot.RegisterCallback("nav:", h.cbNav)

	log.Printf("[HANDLER] Registered commands and callbacks")
	return h
}

// cbNav handles simple navigation shortcuts
func (h *Handler) cbNav(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	switch data {
	case "nav:cot_menu":
		html := "<b>COT Data & Analysis</b>\n\nPilih menu di bawah ini:"
		kb := h.kb.COTMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	case "nav:outlook":
		// Can't edit a message to an outlook due to length properly here, we call cmdOutlook
		_ = h.bot.DeleteMessage(ctx, chatID, msgID) // remove menu
		return h.cmdOutlook(ctx, chatID, userID, "")
	}
	return nil
}

// ---------------------------------------------------------------------------
// /start & /help — Onboarding
// ---------------------------------------------------------------------------

func (h *Handler) cmdStart(ctx context.Context, chatID string, userID int64, args string) error {
	html := `<b>FF Calendar Bot v2</b>

Institutional-grade forex fundamental analysis (COT Focus):

<b>Analysis Commands:</b>
/cot - COT positioning menu
/cot raw [asset] - View raw data chart
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
	args = strings.TrimSpace(strings.ToLower(args))

	if args == "" {
		html := "<b>COT Data & Analysis</b>\n\nPilih menu di bawah ini:"
		kb := h.kb.COTMenu()
		_, err := h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return err
	}

	if args == "analysis" {
		return h.handleCOTAnalysisRequest(ctx, chatID, 0, false)
	}

	if strings.HasPrefix(args, "raw ") {
		code := strings.TrimSpace(strings.TrimPrefix(args, "raw "))
		return h.sendCOTRawImage(ctx, chatID, code)
	}

	// fallback to raw image
	return h.sendCOTRawImage(ctx, chatID, args)
}

func (h *Handler) handleCOTAnalysisRequest(ctx context.Context, chatID string, msgID int, isEdit bool) error {
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		msg := "No COT data available yet."
		if isEdit {
			return h.bot.EditMessage(ctx, chatID, msgID, msg)
		}
		_, err = h.bot.SendHTML(ctx, chatID, msg)
		return err
	}

	html := h.fmt.FormatCOTOverview(analyses)

	// Keep existing AI overview logic
	if h.aiAnalyzer != nil && h.aiAnalyzer.IsAvailable() {
		narrative, aiErr := h.aiAnalyzer.AnalyzeCOT(ctx, analyses)
		if aiErr == nil && narrative != "" {
			html += "\n\n" + h.fmt.FormatAIInsight("Global COT Analysis", narrative)
		}
	}

	// Back to menu button
	kb := h.kb.BackToOverview("nav:cot_menu") // we can re-use BackToOverview slightly or just use a basic inline row
	kb = ports.InlineKeyboard{Rows: [][]ports.InlineButton{{{Text: "« Back", CallbackData: "nav:cot_menu"}}}}

	if isEdit {
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	}
	_, err = h.bot.SendWithKeyboard(ctx, chatID, html, kb)
	return err
}

func (h *Handler) sendCOTRawImage(ctx context.Context, chatID string, rawCode string) error {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	contractCode := currencyToContractCode(code)
	analysis, err := h.cotRepo.GetLatestAnalysis(ctx, contractCode)
	if err != nil || analysis == nil {
		_, err = h.bot.SendHTML(ctx, chatID, fmt.Sprintf("No raw data for %s", code))
		return err
	}

	// Generate image
	imgBytes, err := renderer.GenerateCOTRawImage(*analysis)
	if err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Failed to render COT image: "+err.Error())
		return err
	}

	_, err = h.bot.SendPhoto(ctx, chatID, imgBytes, fmt.Sprintf("%s COT Data", analysis.Contract.Name))
	return err
}

// cbCOT handles all inline keyboard callbacks for the cot prefix.
func (h *Handler) cbCOT(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// e.g. "cot:analysis" or "cot:raw_list" or "cot:raw:099741"
	action := strings.TrimPrefix(data, "cot:")

	if action == "analysis" {
		return h.handleCOTAnalysisRequest(ctx, chatID, msgID, true)
	}

	if action == "raw_list" {
		analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
		if err != nil || len(analyses) == 0 {
			return h.bot.EditMessage(ctx, chatID, msgID, "No COT data available.")
		}
		
		html := "<b>Select Asset for Raw Data</b>\n\nPilih aset untuk melihat visualisasi data mentah:"
		kb := h.kb.COTRawList(analyses)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, html, kb)
	}

	if strings.HasPrefix(action, "raw:") {
		// e.g. "cot:raw:099741"
		contractCode := strings.TrimPrefix(action, "raw:")
		// Send a photo into chat directly
		// It's a new message, so we just answer the callback and send the photo
		_ = h.bot.AnswerCallback(ctx, cbIDFromContext(h, ctx), "Generating image...") // A bit hacky without exact ID, ignoring Answer
		analysis, err := h.cotRepo.GetLatestAnalysis(ctx, contractCode)
		if err == nil && analysis != nil {
			imgBytes, err := renderer.GenerateCOTRawImage(*analysis)
			if err == nil {
				_, _ = h.bot.SendPhoto(ctx, chatID, imgBytes, fmt.Sprintf("%s COT Data", analysis.Contract.Name))
				return nil
			}
		}
		return nil
	}

	return nil
}

// cbIDFromContext is a dummy helper
func cbIDFromContext(h *Handler, ctx context.Context) string { return "" }




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
