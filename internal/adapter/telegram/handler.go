package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
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
	priceRepo   ports.PriceRepository
	signalRepo  ports.SignalRepository

	aiAnalyzer ports.AIAnalyzer

	// newsScheduler provides access to per-currency surprise sigma for conviction scoring.
	// May be nil — all callers guard with a nil check.
	newsScheduler SurpriseProvider

	// changelog is the embedded CHANGELOG.md content, injected at startup.
	changelog string

	// Per-user AI cooldown to prevent rapid-fire expensive commands.
	aiCooldownMu sync.Mutex
	aiCooldown   map[int64]time.Time // userID -> last AI command time

	// Per-user chat cooldown (separate from AI command cooldown to avoid interference).
	chatCooldownMu sync.Mutex
	chatCooldown   map[int64]time.Time // userID -> last chat message time

	// Authorization middleware for tiered access control.
	middleware *Middleware

	// chatService handles free-text (chatbot) messages via Claude.
	// May be nil — chatbot mode disabled if Claude endpoint not configured.
	chatService *aisvc.ChatService
}

// NewHandler creates a handler and registers all commands on the bot.
// newsScheduler and chatService may be nil; all callers guard with nil checks before use.
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
	middleware *Middleware,
	priceRepo ports.PriceRepository,
	signalRepo ports.SignalRepository,
	chatService *aisvc.ChatService,
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
		aiCooldown:    make(map[int64]time.Time),
		chatCooldown:  make(map[int64]time.Time),
		middleware:    middleware,
		priceRepo:     priceRepo,
		signalRepo:    signalRepo,
		chatService:   chatService,
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
	bot.RegisterCommand("/macro", h.cmdMacro)     // P3.2 — FRED Macro Regime Dashboard
	bot.RegisterCommand("/signals", h.cmdSignals) // COT Signal Detection
	bot.RegisterCommand("/backtest", h.cmdBacktest)  // Backtest stats
	bot.RegisterCommand("/accuracy", h.cmdAccuracy)  // Quick accuracy summary

	// Membership & upgrade info
	bot.RegisterCommand("/membership", h.cmdMembership)

	// Chat history management
	bot.RegisterCommand("/clear", h.cmdClearChat)

	// Admin commands (access enforced inside handlers)
	bot.RegisterCommand("/users", h.cmdUsers)
	bot.RegisterCommand("/setrole", h.cmdSetRole)
	bot.RegisterCommand("/ban", h.cmdBan)
	bot.RegisterCommand("/unban", h.cmdUnban)

	// Register callback handlers
	bot.RegisterCallback("cot:", h.cbCOTDetail)
	bot.RegisterCallback("alert:", h.cbAlertToggle)
	bot.RegisterCallback("set:", h.cbSettings)
	bot.RegisterCallback("cal:filter:", h.cbNewsFilter)
	bot.RegisterCallback("out:", h.cbOutlook)
	bot.RegisterCallback("cal:nav:", h.cbNewsNav)

	log.Info().Int("commands", 18).Int("callbacks", 6).Msg("registered commands and callback prefixes")
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
<i>Institutional Flow &amp; Macro Analytics</i>

<b>📊 Market Data</b>
/cot — COT overview · <code>/cot EUR</code> detail · <code>/cot raw EUR</code>
/rank — Currency strength ranking
/signals — COT signal detection (7 types)
/calendar — Economic calendar · <code>/calendar week</code>

<b>🧠 AI Outlook</b>
/outlook cot · news · fred · combine · cross

<b>📈 Backtest</b>
/backtest — Stats · <code>/backtest signals</code> · <code>/backtest EUR</code>
/accuracy — Quick accuracy summary

<b>🏛 Macro</b>
/macro — FRED regime dashboard (7 indicators)

<b>⚙️ Settings</b>
/settings · /membership · /status

<b>🔐 Admin</b>
/users · /setrole · /ban · /unban

<code>ARK v3.2.0</code>`

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
	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) > 0 {
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
		// Try GetHistory first (up to 4 weeks back to avoid missing data
		// when CFTC report is older than 7 days due to delayed release).
		records, err := h.cotRepo.GetHistory(ctx, contractCode, 4)
		if err != nil || len(records) == 0 {
			// Fallback: try GetLatest directly (reverse-scans the whole prefix)
			latest, latestErr := h.cotRepo.GetLatest(ctx, contractCode)
			if latestErr != nil || latest == nil {
				msg := fmt.Sprintf("No COT data for %s", displayCode)
				if editMsgID > 0 {
					return h.bot.EditMessage(ctx, chatID, editMsgID, msg)
				}
				_, e := h.bot.SendHTML(ctx, chatID, msg)
				return e
			}
			records = []domain.COTRecord{*latest}
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

	// Build price context for this contract (best-effort, non-fatal)
	var priceCtxMap map[string]*domain.PriceContext
	if editMsgID == 0 && h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pc, pcErr := ctxBuilder.Build(ctx, contractCode, displayCode); pcErr == nil && pc != nil {
			priceCtxMap = map[string]*domain.PriceContext{contractCode: pc}
			html += h.fmt.FormatPriceContext(pc)

			// Price-COT divergence detection
			divs := pricesvc.DetectPriceCOTDivergences(priceCtxMap, []domain.COTAnalysis{*analysis})
			if len(divs) > 0 {
				html += h.fmt.FormatPriceCOTDivergence(divs[0])
			}
		} else if pcErr != nil {
			// Notify owner about price context failure (non-blocking)
			h.notifyOwnerDebug(ctx, fmt.Sprintf("⚠️ Price context failed for <b>%s</b>\n<code>%s</code>", displayCode, pcErr.Error()))
		}
	}

	// Add AI interpretation with price context if available
	if editMsgID == 0 && h.aiAnalyzer != nil && h.aiAnalyzer.IsAvailable() {
		narrative, aiErr := h.aiAnalyzer.AnalyzeCOTWithPrice(ctx, []domain.COTAnalysis{*analysis}, priceCtxMap)
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

	// Signal detection for this currency
	if editMsgID == 0 && analysis != nil {
		records, histErr := h.cotRepo.GetHistory(ctx, contractCode, 8)
		if histErr == nil && len(records) > 0 {
			histMap := map[string][]domain.COTRecord{contractCode: records}
			recalDet := cot.NewRecalibratedDetector(h.signalRepo)
			if h.signalRepo != nil {
				_ = recalDet.LoadTypeStats(ctx)
			}
			var rCtx *domain.RiskContext
			if h.priceRepo != nil {
				rb := pricesvc.NewRiskContextBuilder(h.priceRepo)
				rCtx, _ = rb.Build(ctx)
			}
			signals := recalDet.DetectAll([]domain.COTAnalysis{*analysis}, histMap, rCtx)
			if len(signals) > 0 {
				html += h.fmt.FormatSignalsSummary(signals)
			}
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

	// Check subcmd first — show menu without consuming AI quota if no args
	subcmd := strings.ToLower(strings.TrimSpace(args))
	if subcmd == "" {
		html := "🦅 <b>ARK Intelligence Outlook</b>\nSelect the type of market analysis you want to generate:\n\n" +
			"<i>Tip: </i><code>/outlook cot</code> | <code>/outlook news</code> | <code>/outlook fred</code> | <code>/outlook combine</code> | <code>/outlook cross</code>"
		kb := h.kb.OutlookMenu()
		_, err := h.bot.SendWithKeyboard(ctx, chatID, html, kb)
		return err
	}

	// Per-user AI quota check via middleware (only consumed when AI will actually be invoked)
	if h.middleware != nil {
		allowed, reason := h.middleware.CheckAIQuota(ctx, userID)
		if !allowed {
			_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("⛔ %s", reason))
			return err
		}

		// Tiered cooldown check (Owner=0s, Admin=10s, Member/Free=30s)
		cooldown := h.middleware.GetAICooldown(ctx, userID)
		if cooldown > 0 && !h.checkAICooldownDynamic(userID, cooldown) {
			_, err := h.bot.SendHTML(ctx, chatID, "Please wait before requesting another AI analysis.")
			return err
		}
	} else {
		// Legacy fallback
		if !h.bot.isOwner(userID) && !h.checkAICooldown(userID) {
			_, err := h.bot.SendHTML(ctx, chatID, "Please wait before requesting another AI analysis.")
			return err
		}
	}

	return h.generateOutlook(ctx, chatID, userID, subcmd, 0)
}

func (h *Handler) cbOutlook(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// AI quota + cooldown check for callback-triggered outlook (same as /outlook command)
	if h.aiAnalyzer == nil || !h.aiAnalyzer.IsAvailable() {
		return h.bot.EditMessage(ctx, chatID, msgID, "AI outlook is unavailable.")
	}
	if h.middleware != nil {
		allowed, reason := h.middleware.CheckAIQuota(ctx, userID)
		if !allowed {
			return h.bot.EditMessage(ctx, chatID, msgID, fmt.Sprintf("\xe2\x9b\x94 %s", reason))
		}
		cooldown := h.middleware.GetAICooldown(ctx, userID)
		if cooldown > 0 && !h.checkAICooldownDynamic(userID, cooldown) {
			return h.bot.EditMessage(ctx, chatID, msgID, "Please wait before requesting another AI analysis.")
		}
	} else {
		// Legacy fallback
		if !h.bot.isOwner(userID) && !h.checkAICooldown(userID) {
			return h.bot.EditMessage(ctx, chatID, msgID, "Please wait before requesting another AI analysis.")
		}
	}
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
		// Price contexts (best-effort — non-fatal if unavailable)
		if h.priceRepo != nil {
			ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
			if priceCtxs, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil && len(priceCtxs) > 0 {
				weeklyData.PriceContexts = priceCtxs
			}
		}
		result, err = h.aiAnalyzer.AnalyzeCombinedOutlook(ctx, weeklyData)
	} else if subcmd == "cross" {
		cotSlice, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
		cotMap := make(map[string]*domain.COTAnalysis, len(cotSlice))
		for i := range cotSlice {
			cotMap[cotSlice[i].Contract.Code] = &cotSlice[i]
		}
		result, err = h.aiAnalyzer.AnalyzeCrossMarket(ctx, cotMap)
	} else { // "cot" or default
		cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
		// Gap E — pass MacroData so FRED regime context is injected into the /outlook cot prompt
		macroData, _ := fred.GetCachedOrFetch(ctx)
		weeklyData := ports.WeeklyData{
			COTAnalyses: cotAnalyses,
			MacroData:   macroData,
			Language:    prefs.Language,
		}
		// Price contexts (best-effort — non-fatal if unavailable)
		if h.priceRepo != nil {
			ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
			if priceCtxs, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil && len(priceCtxs) > 0 {
				weeklyData.PriceContexts = priceCtxs
			}
		}
		result, err = h.aiAnalyzer.GenerateWeeklyOutlook(ctx, weeklyData)
	}

	if err != nil {
		log.Error().Err(err).Msg("AI generation failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID, "AI generation failed. Please try again later.")
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
			log.Warn().Str("action", action).Msg("unknown settings action")
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
	case "mute_1h", "disable":
		// Disable alerts until manually re-enabled via /settings.
		// Note: "mute_1h" is a legacy callback key retained for backward compatibility.
		prefs.AlertsEnabled = false
		_ = h.prefsRepo.Set(ctx, userID, prefs)
		return h.bot.EditMessage(ctx, chatID, msgID,
			"Alerts disabled. Use /settings to re-enable.")
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

<b>Version:</b> v3.0.0`,
		now.Format("15:04:05"),
		len(cotAnalyses),
		aiStatus,
	)

	_, err := h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------
// /signals — COT Signal Detection
// ---------------------------------------------------------------------------

func (h *Handler) cmdSignals(ctx context.Context, chatID string, userID int64, args string) error {
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		_, err = h.bot.SendHTML(ctx, chatID, "No COT data available for signal detection.")
		return err
	}

	// Build history map (8 weeks needed for momentum/divergence detection)
	historyMap := make(map[string][]domain.COTRecord, len(analyses))
	for _, a := range analyses {
		records, hErr := h.cotRepo.GetHistory(ctx, a.Contract.Code, 8)
		if hErr == nil && len(records) > 0 {
			historyMap[a.Contract.Code] = records
		}
	}

	// Use recalibrated detector with historical win rates + VIX filter
	recalDetector := cot.NewRecalibratedDetector(h.signalRepo)
	if h.signalRepo != nil {
		_ = recalDetector.LoadTypeStats(ctx)
	}
	var riskCtx *domain.RiskContext
	if h.priceRepo != nil {
		rb := pricesvc.NewRiskContextBuilder(h.priceRepo)
		riskCtx, _ = rb.Build(ctx)
	}
	signals := recalDetector.DetectAll(analyses, historyMap, riskCtx)

	// Filter by currency if specified
	filterCurrency := strings.ToUpper(strings.TrimSpace(args))
	if filterCurrency != "" {
		var filtered []cot.Signal
		for _, s := range signals {
			if s.Currency == filterCurrency {
				filtered = append(filtered, s)
			}
		}
		signals = filtered
	}

	html := h.fmt.FormatSignalsHTML(signals, filterCurrency)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------
// AI cooldown helper
// ---------------------------------------------------------------------------

// aiCooldownDuration is the minimum interval between AI-heavy commands per user.
const aiCooldownDuration = 30 * time.Second

// checkAICooldown returns true if the user is allowed to make an AI call,
// and records the current time. Returns false if the user is still in cooldown.
func (h *Handler) checkAICooldown(userID int64) bool {
	h.aiCooldownMu.Lock()
	defer h.aiCooldownMu.Unlock()

	now := time.Now()

	// Opportunistic cleanup: remove entries older than 5 minutes (max cooldown is 30s,
	// so anything >5m is stale). Only runs when map exceeds 100 entries to amortize cost.
	if len(h.aiCooldown) > 100 {
		cutoff := now.Add(-5 * time.Minute)
		for uid, ts := range h.aiCooldown {
			if ts.Before(cutoff) {
				delete(h.aiCooldown, uid)
			}
		}
	}

	if last, ok := h.aiCooldown[userID]; ok {
		if now.Sub(last) < aiCooldownDuration {
			return false
		}
	}
	h.aiCooldown[userID] = now
	return true
}

// checkAICooldownDynamic is like checkAICooldown but with a configurable duration per tier.
func (h *Handler) checkAICooldownDynamic(userID int64, cooldown time.Duration) bool {
	h.aiCooldownMu.Lock()
	defer h.aiCooldownMu.Unlock()

	now := time.Now()
	if last, ok := h.aiCooldown[userID]; ok {
		if now.Sub(last) < cooldown {
			return false
		}
	}
	h.aiCooldown[userID] = now
	return true
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

// handleMonthNav handles prevmonth / thismonth / nextmonth navigation.
// dateStr is the reference date from the callback (e.g. "20260301") to compute relative months.
func (h *Handler) handleMonthNav(ctx context.Context, chatID string, msgID int, navType, dateStr string) error {
	// Parse the reference date from the callback; fall back to "now" if invalid.
	refDate, parseErr := time.Parse("20060102", dateStr)
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

	// Dual price + COT strength ranking (best-effort, non-fatal)
	if h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if priceCtxs, err := ctxBuilder.BuildAll(ctx); err == nil && len(priceCtxs) > 0 {
			strengths := pricesvc.ComputeCurrencyStrengthIndex(priceCtxs, analyses)
			if len(strengths) > 0 {
				html += h.fmt.FormatStrengthRanking(strengths)
			}
		}
	}

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
		// Only admin+ can force-refresh (prevents FRED API quota abuse)
		if !h.requireAdmin(ctx, chatID, userID) {
			return nil
		}
		fred.InvalidateCache()
	}

	cacheStatus := "🏦 Fetching FRED macro data... ⏳ (5-15s)"
	if !forceRefresh && fred.CacheAge() >= 0 {
		cacheStatus = "🏦 Loading FRED macro data (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendHTML(ctx, chatID, cacheStatus)

	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("FRED data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"Failed to fetch macro data. Please try again later.")
	}

	regime := fred.ClassifyMacroRegime(data)
	html := h.fmt.FormatMacroRegime(regime, data)

	return h.bot.EditMessage(ctx, chatID, placeholderID, html)
}

// ---------------------------------------------------------------------------
// /membership — Tier comparison & upgrade info
// ---------------------------------------------------------------------------

// cmdMembership shows the tier comparison and how to upgrade.
func (h *Handler) cmdMembership(ctx context.Context, chatID string, userID int64, args string) error {
	// Determine caller's current tier
	currentRole := domain.RoleFree
	if h.middleware != nil {
		currentRole = h.middleware.GetUserRole(ctx, userID)
	} else if h.bot.isOwner(userID) {
		currentRole = domain.RoleOwner
	}

	currentLabel := strings.ToUpper(string(currentRole))

	html := fmt.Sprintf(""+
		"\xf0\x9f\xa6\x85 <b>ARK Intelligence Membership</b>\n"+
		"Your tier: <b>%s</b>\n\n", currentLabel)

	html += "" +
		"<b>\xf0\x9f\x86\x93 FREE</b>\n" +
		"<code>Commands   : 10/day</code>\n" +
		"<code>AI Analysis: 3/day (30s cooldown)</code>\n" +
		"<code>News Alert : USD only, High impact</code>\n" +
		"<code>FRED Macro : </code>\xe2\x9d\x8c\n" +
		"<code>COT Data   : </code>\xe2\x9c\x85 Full access\n" +
		"<code>Calendar   : </code>\xe2\x9c\x85 Full access\n\n"

	html += "" +
		"<b>\xe2\xad\x90 MEMBER</b>\n" +
		"<code>Commands   : 15/min (no daily cap)</code>\n" +
		"<code>AI Analysis: 10/day (30s cooldown)</code>\n" +
		"<code>News Alert : All currencies &amp; impacts</code>\n" +
		"<code>FRED Macro : </code>\xe2\x9c\x85 Regime alerts\n" +
		"<code>COT Data   : </code>\xe2\x9c\x85 Full access\n" +
		"<code>Calendar   : </code>\xe2\x9c\x85 Full access\n\n"

	// Only show ADMIN tier to admins and owners
	isAdmin := domain.RoleHierarchy(currentRole) >= domain.RoleHierarchy(domain.RoleAdmin)
	if isAdmin {
		html += "" +
			"<b>\xf0\x9f\x9b\xa1 ADMIN</b>\n" +
			"<code>Commands   : 30/min (no daily cap)</code>\n" +
			"<code>AI Analysis: 50/day (10s cooldown)</code>\n" +
			"<code>News Alert : All currencies &amp; impacts</code>\n" +
			"<code>FRED Macro : </code>\xe2\x9c\x85 Regime alerts\n" +
			"<code>User Mgmt  : </code>\xe2\x9c\x85 /users, /ban, /setrole\n\n"
	}

	// Show upgrade CTA for non-owner users
	if currentRole == domain.RoleFree || currentRole == domain.RoleMember {
		ownerID := h.bot.OwnerID()
		if ownerID > 0 {
			html += fmt.Sprintf(
				"\xf0\x9f\x94\x91 <b>Upgrade to Member</b>\n"+
					"Contact the owner to upgrade your access:\n"+
					"\xe2\x9e\xa1 <a href=\"tg://user?id=%d\">Contact Owner</a>\n\n"+
					"<i>Include your User ID: <code>%d</code></i>",
				ownerID, userID)
		} else {
			html += fmt.Sprintf(
				"\xf0\x9f\x94\x91 <b>Upgrade to Member</b>\n"+
					"Contact the group admin to upgrade your access.\n\n"+
					"<i>Your User ID: <code>%d</code></i>",
				userID)
		}
	} else if currentRole == domain.RoleOwner {
		html += "<i>You have unlimited access as Owner.</i>"
	}

	_, err := h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------
// Admin Commands — /users, /setrole, /ban, /unban
// ---------------------------------------------------------------------------

// requireAdmin checks that the caller is Owner or Admin. Returns false and sends an error if not.
func (h *Handler) requireAdmin(ctx context.Context, chatID string, userID int64) bool {
	if h.middleware == nil {
		return h.bot.isOwner(userID) // fallback
	}
	role := h.middleware.GetUserRole(ctx, userID)
	if domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin) {
		return true
	}
	_, _ = h.bot.SendHTML(ctx, chatID, "This command requires Admin privileges.")
	return false
}

// cmdUsers lists all registered users with their roles and usage stats.
func (h *Handler) cmdUsers(ctx context.Context, chatID string, userID int64, args string) error {
	if !h.requireAdmin(ctx, chatID, userID) {
		return nil
	}
	if h.middleware == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "User management not available.")
		return err
	}

	users, err := h.middleware.GetAllUsers(ctx)
	if err != nil {
		log.Error().Err(err).Int64("caller", userID).Msg("cmdUsers: failed to list users")
		_, err = h.bot.SendHTML(ctx, chatID, "Failed to list users. Check server logs.")
		return err
	}

	html := FormatUserList(users)
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// cmdSetRole changes a user's role.
// Usage: /setrole <userID> <role>
func (h *Handler) cmdSetRole(ctx context.Context, chatID string, userID int64, args string) error {
	if !h.requireAdmin(ctx, chatID, userID) {
		return nil
	}
	if h.middleware == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "User management not available.")
		return err
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		_, err := h.bot.SendHTML(ctx, chatID,
			"Usage: <code>/setrole &lt;userID&gt; &lt;role&gt;</code>\nRoles: owner, admin, member, free, banned")
		return err
	}

	var targetID int64
	if _, err := fmt.Sscanf(parts[0], "%d", &targetID); err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Invalid user ID. Must be a number.")
		return err
	}

	newRole := domain.UserRole(strings.ToLower(parts[1]))
	switch newRole {
	case domain.RoleOwner, domain.RoleAdmin, domain.RoleMember, domain.RoleFree, domain.RoleBanned:
		// valid
	default:
		_, err := h.bot.SendHTML(ctx, chatID,
			fmt.Sprintf("Unknown role <code>%s</code>. Valid: owner, admin, member, free, banned", html.EscapeString(parts[1])))
		return err
	}

	// Prevent non-owner from setting privileged roles (owner or admin)
	callerRole := h.middleware.GetUserRole(ctx, userID)
	if (newRole == domain.RoleOwner || newRole == domain.RoleAdmin) && callerRole != domain.RoleOwner {
		_, err := h.bot.SendHTML(ctx, chatID, "Only the Owner can assign Owner or Admin roles.")
		return err
	}

	// Prevent banning/demoting the Owner
	if h.bot.isOwner(targetID) && newRole != domain.RoleOwner {
		_, err := h.bot.SendHTML(ctx, chatID, "Cannot change the Owner's role.")
		return err
	}

	// Prevent Admin from modifying users with equal or higher privilege
	targetRole := h.middleware.GetUserRole(ctx, targetID)
	if callerRole != domain.RoleOwner && domain.RoleHierarchy(targetRole) >= domain.RoleHierarchy(callerRole) {
		_, err := h.bot.SendHTML(ctx, chatID, "You cannot modify a user with equal or higher privileges.")
		return err
	}

	if err := h.middleware.SetUserRole(ctx, targetID, newRole); err != nil {
		log.Error().Err(err).Int64("target", targetID).Str("role", string(newRole)).Msg("cmdSetRole: failed")
		_, err = h.bot.SendHTML(ctx, chatID, "Failed to set role. Check server logs.")
		return err
	}

	_, err := h.bot.SendHTML(ctx, chatID,
		fmt.Sprintf("User <code>%d</code> role set to <b>%s</b>.", targetID, newRole))
	return err
}

// cmdBan bans a user.
// Usage: /ban <userID>
func (h *Handler) cmdBan(ctx context.Context, chatID string, userID int64, args string) error {
	if !h.requireAdmin(ctx, chatID, userID) {
		return nil
	}
	if h.middleware == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "User management not available.")
		return err
	}

	targetStr := strings.TrimSpace(args)
	if targetStr == "" {
		_, err := h.bot.SendHTML(ctx, chatID, "Usage: <code>/ban &lt;userID&gt;</code>")
		return err
	}

	var targetID int64
	if _, err := fmt.Sscanf(targetStr, "%d", &targetID); err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Invalid user ID.")
		return err
	}

	// Prevent banning the Owner
	if h.bot.isOwner(targetID) {
		_, err := h.bot.SendHTML(ctx, chatID, "Cannot ban the Owner.")
		return err
	}

	// Prevent Admin from banning users with equal or higher privilege
	callerRole := h.middleware.GetUserRole(ctx, userID)
	targetRole := h.middleware.GetUserRole(ctx, targetID)
	if callerRole != domain.RoleOwner && domain.RoleHierarchy(targetRole) >= domain.RoleHierarchy(callerRole) {
		_, err := h.bot.SendHTML(ctx, chatID, "You cannot ban a user with equal or higher privileges.")
		return err
	}

	if err := h.middleware.SetUserRole(ctx, targetID, domain.RoleBanned); err != nil {
		log.Error().Err(err).Int64("target", targetID).Msg("cmdBan: failed")
		_, err = h.bot.SendHTML(ctx, chatID, "Failed to ban user. Check server logs.")
		return err
	}

	_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("User <code>%d</code> has been banned.", targetID))
	return err
}

// cmdUnban unbans a user (sets them back to Free).
// Usage: /unban <userID>
func (h *Handler) cmdUnban(ctx context.Context, chatID string, userID int64, args string) error {
	if !h.requireAdmin(ctx, chatID, userID) {
		return nil
	}
	if h.middleware == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "User management not available.")
		return err
	}

	targetStr := strings.TrimSpace(args)
	if targetStr == "" {
		_, err := h.bot.SendHTML(ctx, chatID, "Usage: <code>/unban &lt;userID&gt;</code>")
		return err
	}

	var targetID int64
	if _, err := fmt.Sscanf(targetStr, "%d", &targetID); err != nil {
		_, err = h.bot.SendHTML(ctx, chatID, "Invalid user ID.")
		return err
	}

	if err := h.middleware.SetUserRole(ctx, targetID, domain.RoleFree); err != nil {
		log.Error().Err(err).Int64("target", targetID).Msg("cmdUnban: failed")
		_, err = h.bot.SendHTML(ctx, chatID, "Failed to unban user. Check server logs.")
		return err
	}

	_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("User <code>%d</code> has been unbanned (set to Free).", targetID))
	return err
}

// notifyOwnerDebug sends a debug message to the bot owner (non-blocking, best-effort).
// Does nothing if OwnerID is not set.
func (h *Handler) notifyOwnerDebug(ctx context.Context, html string) {
	ownerID := h.bot.OwnerID()
	if ownerID <= 0 {
		return
	}
	go func() {
		_, _ = h.bot.SendHTML(ctx, fmt.Sprintf("%d", ownerID), html)
	}()
}

// ---------------------------------------------------------------------------
// Chatbot — Free-text message handling via Claude
// ---------------------------------------------------------------------------

// HandleFreeText processes non-command messages through the Claude chatbot pipeline.
// This is registered as the Bot's FreeTextHandler during wiring.
func (h *Handler) HandleFreeText(ctx context.Context, chatID string, userID int64, username string, text string, contentBlocks []ports.ContentBlock) error {
	if h.chatService == nil {
		// No chatbot configured — send a helpful hint
		_, err := h.bot.SendHTML(ctx, chatID,
			"I only respond to commands for now. Type /help for available commands.")
		return err
	}

	// Check per-user chat cooldown BEFORE quota check to avoid consuming
	// AI quota on cooldown-blocked requests (owner bypassed — unlimited access).
	if !h.bot.isOwner(userID) && !h.checkChatCooldown(userID) {
		_, err := h.bot.SendHTML(ctx, chatID,
			"\u23f3 Please wait a moment before sending another message.")
		return err
	}

	// Check AI quota via middleware (after cooldown so blocked requests don't waste quota)
	if h.middleware != nil {
		allowed, reason := h.middleware.CheckAIQuota(ctx, userID)
		if !allowed {
			_, err := h.bot.SendHTML(ctx, chatID, fmt.Sprintf("\u26d4 %s", reason))
			return err
		}
	}

	// Send "thinking" indicator
	thinkMsgID, _ := h.bot.SendMessage(ctx, chatID, "\u2699\ufe0f Thinking...")

	// Get user role for tool resolution
	role := domain.RoleFree
	if h.middleware != nil {
		profile := h.middleware.GetUserProfile(ctx, userID)
		if profile != nil {
			role = profile.Role
		}
	}

	// Call chat service with a per-request timeout to prevent unbounded waits.
	// Extended thinking + server tools (web search, code execution) can take
	// significantly longer than text-only responses. Allow 120s for full pipeline.
	chatCtx, chatCancel := context.WithTimeout(ctx, 120*time.Second)
	defer chatCancel()
	response, err := h.chatService.HandleMessage(chatCtx, userID, text, role, contentBlocks)

	// Delete "thinking" indicator
	if thinkMsgID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, thinkMsgID)
	}

	// Handle template fallback: still send the response but refund the AI quota
	// since no real AI call succeeded.
	if errors.Is(err, aisvc.ErrAIFallback) {
		if h.middleware != nil {
			h.middleware.RefundAIQuota(ctx, userID)
		}
		// Send the template fallback content (err contains ErrAIFallback but response is valid)
		if _, sendErr := h.bot.SendHTML(ctx, chatID, response); sendErr != nil {
			_, sendErr = h.bot.SendMessage(ctx, chatID, response)
			return sendErr
		}
		return nil
	}

	if err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("chat service error")
		// Refund AI quota on total failure (context timeout, etc.)
		if h.middleware != nil {
			h.middleware.RefundAIQuota(ctx, userID)
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID,
			"Error processing your message. Please try again later or use /help.")
		return sendErr
	}

	// Send response (Claude follows HTML constraints via system prompt).
	// Fall back to plain text if Telegram rejects the HTML (malformed tags, etc.)
	if _, err = h.bot.SendHTML(ctx, chatID, response); err != nil {
		log.Warn().Err(err).Int64("user_id", userID).Msg("SendHTML failed for chat response, falling back to plain text")
		_, err = h.bot.SendMessage(ctx, chatID, response)
	}
	return err
}

// checkChatCooldown returns true if the user is allowed to send a chat message (cooldown elapsed).
// Updates the cooldown timestamp if allowed.
// Uses a separate map from AI command cooldown to avoid interference.
func (h *Handler) checkChatCooldown(userID int64) bool {
	h.chatCooldownMu.Lock()
	defer h.chatCooldownMu.Unlock()

	now := time.Now()

	// Opportunistic cleanup: remove stale entries when map grows large.
	if len(h.chatCooldown) > 100 {
		cutoff := now.Add(-5 * time.Minute)
		for uid, ts := range h.chatCooldown {
			if ts.Before(cutoff) {
				delete(h.chatCooldown, uid)
			}
		}
	}

	if last, ok := h.chatCooldown[userID]; ok {
		// Use a 5-second cooldown for chat messages
		if now.Sub(last) < 5*time.Second {
			return false
		}
	}
	h.chatCooldown[userID] = now
	return true
}

// cmdClearChat handles the /clear command to wipe conversation history.
func (h *Handler) cmdClearChat(ctx context.Context, chatID string, userID int64, _ string) error {
	if h.chatService == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Chat mode is not enabled.")
		return err
	}

	if err := h.chatService.ClearHistory(ctx, userID); err != nil {
		log.Error().Err(err).Int64("user_id", userID).Msg("clear chat history failed")
		_, sendErr := h.bot.SendHTML(ctx, chatID, "Failed to clear chat history. Please try again.")
		return sendErr
	}

	_, err := h.bot.SendHTML(ctx, chatID,
		"\u2705 Chat history cleared. Starting fresh conversation.")
	return err
}
