package telegram

// Handler — Core struct, interfaces, NewHandler, shared helpers

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// ---------------------------------------------------------------------------
// Handler — Wires services to Telegram commands
// ---------------------------------------------------------------------------

// SurpriseProvider is a minimal interface allowing the handler to read
// the per-currency accumulated surprise sigma from the news scheduler.
type SurpriseProvider interface {
	GetSurpriseSigma(currency string) float64
}

// ImpactProvider exposes event impact data for the /impact command.
type ImpactProvider interface {
	GetEventImpactSummary(ctx context.Context, eventTitle string) ([]domain.EventImpactSummary, error)
	GetTrackedEvents(ctx context.Context) ([]string, error)
}

// Handler holds all service dependencies and registers commands on the bot.
type Handler struct {
	bot *Bot
	fmt *Formatter
	kb  *KeyboardBuilder

	// Repositories
	eventRepo      ports.EventRepository
	cotRepo        ports.COTRepository
	prefsRepo      ports.PrefsRepository
	newsRepo       ports.NewsRepository
	newsFetcher    ports.NewsFetcher
	priceRepo      ports.PriceRepository
	signalRepo     ports.SignalRepository
	dailyPriceRepo pricesvc.DailyPriceStore
	intradayRepo   pricesvc.IntradayStore // 4H intraday — may be nil

	aiAnalyzer ports.AIAnalyzer

	// newsScheduler provides access to per-currency surprise sigma for conviction scoring.
	// May be nil — all callers guard with a nil check.
	newsScheduler SurpriseProvider

	// impactProvider exposes event impact data for the /impact command.
	// May be nil — impact feature disabled if not wired.
	impactProvider ImpactProvider

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

	// claudeAnalyzer is an AIAnalyzer backed by Claude.
	// Used by /outlook when the user's PreferredModel is "claude".
	// May be nil if Claude is not configured.
	claudeAnalyzer *aisvc.ClaudeAnalyzer

	// alpha holds optional Factor/Strategy/Microstructure engine services.
	// May be nil — all alpha commands degrade gracefully.
	alpha *AlphaServices

	// alphaCache stores per-chat alpha state with TTL for unified /alpha dashboard.
	// Initialized by WithAlpha; nil if alpha services are not configured.
	alphaCache *alphaStateCache

	// cta holds optional Classical TA engine services.
	// May be nil — /cta command disabled if not configured.
	cta *CTAServices

	// ctaCache stores per-chat CTA state with TTL for the /cta dashboard.
	// Initialized by WithCTA; nil if CTA services are not configured.
	ctaCache *ctaStateCache

	// quant holds optional Quant/Econometric engine services.
	// May be nil — /quant command disabled if not configured.
	quant *QuantServices

	// quantCache stores per-chat Quant state with TTL.
	quantCache *quantStateCache

	// vp holds optional Volume Profile engine services.
	// May be nil — /vp command disabled if not configured.
	vp *VPServices

	// vpCache stores per-chat VP state with TTL.
	vpCache *vpStateCache

	// ctabt holds optional CTA Backtest engine services.
	// May be nil — /ctabt command disabled if not configured.
	ctabt *CTABTServices

	// ict holds optional ICT/SMC analysis engine services.
	// May be nil — /ict command disabled if not configured.
	ict      *ICTServices
	ictCache *ictStateCache

	// smc holds optional SMC analysis engine services.
	// May be nil — /smc command disabled if not configured.
	smc      *SMCServices
	smcCache *smcStateCache

	// gex holds the GEX engine for /gex command.
	// May be nil — /gex command disabled if not configured.
	gex *GEXServices

	// wyckoff holds optional Wyckoff analysis engine services.
	// May be nil — /wyckoff command disabled if not configured.
	wyckoff *WyckoffServices

	// elliott holds optional Elliott Wave engine services.
	// May be nil — /elliott command disabled if not configured.
	elliott *ElliottServices
}

// NewHandler creates a handler and registers all commands on the bot.
// newsScheduler, chatService, and claudeAnalyzer may be nil; all callers guard with nil checks before use.
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
	claudeAnalyzer *aisvc.ClaudeAnalyzer,
	impactProvider ImpactProvider,
	dailyPriceRepo pricesvc.DailyPriceStore,
	intradayRepo pricesvc.IntradayStore,
) *Handler {
	h := &Handler{
		bot:            bot,
		fmt:            NewFormatter(),
		kb:             NewKeyboardBuilder(),
		eventRepo:      eventRepo,
		cotRepo:        cotRepo,
		prefsRepo:      prefsRepo,
		newsRepo:       newsRepo,
		newsFetcher:    newsFetcher,
		aiAnalyzer:     aiAnalyzer,
		changelog:      changelog,
		newsScheduler:  newsScheduler,
		aiCooldown:     make(map[int64]time.Time),
		chatCooldown:   make(map[int64]time.Time),
		middleware:     middleware,
		priceRepo:      priceRepo,
		signalRepo:     signalRepo,
		chatService:    chatService,
		claudeAnalyzer: claudeAnalyzer,
		impactProvider: impactProvider,
		dailyPriceRepo: dailyPriceRepo,
		intradayRepo:   intradayRepo,
	}

	// Register all commands
	bot.RegisterCommand("/start", h.cmdStart)
	bot.RegisterCommand("/help", h.cmdHelp)
	bot.RegisterCommand("/settings", h.cmdSettings)
	bot.RegisterCommand("/status", h.cmdStatus)
	bot.RegisterCommand("/cot", h.cmdCOT)
	bot.RegisterCommand("/outlook", h.cmdOutlook)
	bot.RegisterCommand("/calendar", h.cmdCalendar)
	bot.RegisterCommand("/rank", h.cmdRank)
	bot.RegisterCommand("/macro", h.cmdMacro)
	bot.RegisterCommand("/bias", h.cmdBias)
	bot.RegisterCommand("/backtest", h.cmdBacktest)
	bot.RegisterCommand("/accuracy", h.cmdAccuracy)
	bot.RegisterCommand("/report", h.cmdReport)
	bot.RegisterCommand("/impact", h.cmdImpact)
	bot.RegisterCommand("/sentiment", h.cmdSentiment)
	bot.RegisterCommand("/seasonal", h.cmdSeasonal)
	bot.RegisterCommand("/price", h.cmdPrice)   // Daily price context
	bot.RegisterCommand("/levels", h.cmdLevels) // Support/resistance levels + position sizing

	// Membership & upgrade info
	bot.RegisterCommand("/membership", h.cmdMembership)

	// Chat history management
	bot.RegisterCommand("/clear", h.cmdClearChat)

	// Admin commands (access enforced inside handlers)
	bot.RegisterCommand("/users", h.cmdUsers)
	bot.RegisterCommand("/setrole", h.cmdSetRole)
	bot.RegisterCommand("/ban", h.cmdBan)
	bot.RegisterCommand("/unban", h.cmdUnban)

	// Short aliases for power users (mobile-friendly)
	bot.RegisterCommand("/c", h.cmdCOT)
	bot.RegisterCommand("/cal", h.cmdCalendar)
	bot.RegisterCommand("/out", h.cmdOutlook)
	bot.RegisterCommand("/m", h.cmdMacro)
	bot.RegisterCommand("/b", h.cmdBias)
	bot.RegisterCommand("/q", h.cmdQuant)
	bot.RegisterCommand("/bt", h.cmdBacktest)
	bot.RegisterCommand("/r", h.cmdRank)
	bot.RegisterCommand("/s", h.cmdSentiment)
	bot.RegisterCommand("/p", h.cmdPrice)
	bot.RegisterCommand("/l", h.cmdLevels)
	bot.RegisterCommand("/history", h.cmdHistory)
	bot.RegisterCommand("/h", h.cmdHistory)

	// Register callback handlers
	bot.RegisterCallback("cot:", h.cbCOTDetail)
	bot.RegisterCallback("alert:", h.cbAlertToggle)
	bot.RegisterCallback("set:", h.cbSettings)
	bot.RegisterCallback("cal:filter:", h.cbNewsFilter)
	bot.RegisterCallback("out:", h.cbOutlook)
	bot.RegisterCallback("cal:nav:", h.cbNewsNav)
	bot.RegisterCallback("cmd:", h.cbQuickCommand)
	bot.RegisterCallback("onboard:", h.cbOnboard)
	bot.RegisterCallback("macro:", h.cbMacro)
	bot.RegisterCallback("imp:", h.cbImpact)
	bot.RegisterCallback("nav:", h.cbNav)
	bot.RegisterCallback("help:", h.cbHelp)

	log.Info().Int("commands", 48).Int("callbacks", 10).Msg("registered commands and callback prefixes")
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

	// If user already has experience level set, show normal help.
	if prefs.ExperienceLevel != "" {
		return h.sendHelp(ctx, chatID, userID)
	}

	// New user → interactive onboarding with role selector.
	welcome := `🦅 <b>Selamat datang di ARK Intelligence!</b>
<i>Institutional Flow &amp; Macro Analytics</i>

Sebelum mulai, pilih level pengalaman trading kamu:

🌱 <b>Pemula</b> — Baru mulai trading, ingin belajar dasar
📈 <b>Intermediate</b> — Sudah trading aktif, ingin tools analisis
🏛 <b>Pro</b> — Trader berpengalaman, butuh data institusional`

	_, err := h.bot.SendWithKeyboard(ctx, chatID, welcome, h.kb.OnboardingRoleMenu())
	return err
}

// cbOnboard handles the onboarding flow callbacks (role selection + tutorial).
func (h *Handler) cbOnboard(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "onboard:")

	// "showhelp" → show full help menu
	if action == "showhelp" {
		_ = h.bot.DeleteMessage(ctx, chatID, msgID)
		return h.sendHelp(ctx, chatID, userID)
	}

	// Role selection: beginner / intermediate / pro
	level := action
	if level != "beginner" && level != "intermediate" && level != "pro" {
		return nil
	}

	// Persist experience level
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	prefs.ExperienceLevel = level
	_ = h.prefsRepo.Set(ctx, userID, prefs)

	// Delete the role selector message
	_ = h.bot.DeleteMessage(ctx, chatID, msgID)

	// Send tutorial steps based on level
	var tutorial string
	switch level {
	case "beginner":
		tutorial = `✅ <b>Level: Pemula</b>

<b>🎓 3 Langkah Memulai:</b>

<b>1️⃣ Cek COT Data</b>
Ketik <code>/cot EUR</code> — lihat posisi big player di Euro

<b>2️⃣ Cek Kalender</b>
Ketik <code>/calendar</code> — jadwal rilis data ekonomi

<b>3️⃣ Cek Harga</b>
Ketik <code>/price EUR</code> — harga terkini + perubahan

Ini menu kamu — klik untuk mulai:`

	case "intermediate":
		tutorial = `✅ <b>Level: Intermediate</b>

<b>🎓 3 Langkah Memulai:</b>

<b>1️⃣ CTA Dashboard</b>
Ketik <code>/cta EUR</code> — analisis teknikal lengkap dengan chart

<b>2️⃣ AI Outlook</b>
Ketik <code>/outlook</code> — analisis gabungan AI (data + sentiment + web)

<b>3️⃣ Macro Regime</b>
Ketik <code>/macro</code> — kondisi makro ekonomi global + dampak ke trading

Ini menu kamu — klik untuk mulai:`

	case "pro":
		tutorial = `✅ <b>Level: Pro / Institutional</b>

<b>🎓 3 Langkah Memulai:</b>

<b>1️⃣ Alpha Engine</b>
Ketik <code>/alpha</code> — factor ranking + playbook + risk dashboard

<b>2️⃣ Volume Profile</b>
Ketik <code>/vp EUR</code> — 10 mode VP termasuk AMT institutional-grade

<b>3️⃣ Quant Analysis</b>
Ketik <code>/quant EUR</code> — 12 model econometric (GARCH, regime, PCA, dll)

Ini menu kamu — klik untuk mulai:`
	}

	_, err := h.bot.SendWithKeyboard(ctx, chatID, tutorial, h.kb.StarterKitMenu(level))
	return err
}

func (h *Handler) cmdHelp(ctx context.Context, chatID string, userID int64, args string) error {
	// Support /help <category> to directly expand a sub-category
	category := strings.ToLower(strings.TrimSpace(args))
	if category != "" {
		switch category {
		case "market", "research", "ai", "signals", "settings", "admin", "changelog":
			return h.sendHelpSubCategory(ctx, chatID, userID, category, 0)
		}
	}
	return h.sendHelp(ctx, chatID, userID)
}

// sendHelp sends the interactive category-based help menu.
func (h *Handler) sendHelp(ctx context.Context, chatID string, userID int64) error {
	// Determine user role
	isAdmin := h.bot.isOwner(userID)
	if !isAdmin && h.middleware != nil {
		role := h.middleware.GetUserRole(ctx, userID)
		isAdmin = domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin)
	}

	header := `🦅 <b>ARK Intelligence Terminal</b>
<i>Institutional Flow &amp; Macro Analytics</i>

<i>Pilih kategori untuk melihat commands tersedia:</i>`

	var kb ports.InlineKeyboard
	if isAdmin {
		kb = h.kb.HelpCategoryMenuWithAdmin()
	} else {
		kb = h.kb.HelpCategoryMenu()
	}

	_, err := h.bot.SendWithKeyboard(ctx, chatID, header, kb)
	return err
}

// sendHelpSubCategory sends or edits the help sub-category message for a given category.
func (h *Handler) sendHelpSubCategory(ctx context.Context, chatID string, userID int64, category string, editMsgID int) error {
	var text string

	switch category {
	case "market":
		text = `📊 <b>Market &amp; Data Commands</b>

/cot — COT institutional positioning · <code>/cot EUR</code>
/rank — Currency strength ranking
/bias — Directional bias summary · <code>/bias EUR</code>
/calendar — Economic calendar · <code>/calendar week</code>
/price — Daily OHLC price context · <code>/price EUR</code>
/levels — Support/resistance levels · <code>/levels EUR</code>`

	case "research":
		text = `🔬 <b>Research &amp; Alpha Commands</b>

/alpha — Dashboard lengkap (factor + playbook + risk)
/cta — Classical TA dashboard · <code>/cta EUR</code> · <code>/cta EUR 4h</code>
/ctabt — Backtest Classical TA · <code>/ctabt EUR</code> · <code>/ctabt EUR 4h</code>
/quant — Econometric analysis · <code>/quant EUR</code> · <code>/quant XAU 4h</code>
/vp — Volume Profile institutional · <code>/vp EUR</code> · <code>/vp XAU 4h</code>
/ict — ICT/SMC Smart Money Concepts · <code>/ict EURUSD</code> · <code>/ict XAUUSD H4</code>
/gex — Gamma Exposure (crypto options) · <code>/gex BTC</code> · <code>/gex ETH</code>
/backtest — Backtest dashboard (17 sub-views)
/accuracy — Win rate summary
/report — Weekly signal performance`

	case "ai":
		text = `🧠 <b>AI &amp; Outlook Commands</b>

/outlook — Unified AI analysis (all data + web search)
/macro — FRED macro regime + asset performance
/impact — Event impact database · <code>/impact NFP</code>
/sentiment — Sentiment surveys (CNN F&amp;G, AAII, P/C)
/seasonal — Seasonal patterns · <code>/seasonal EUR</code>`

	case "signals":
		text = `⚡ <b>Signals &amp; Alerts</b>

/bias — Directional bias signals · <code>/bias EUR</code>
/cot — COT positioning + conviction score · <code>/cot EUR</code>
/rank — Currency strength ranking

<b>Alert Settings:</b>
Use /settings to configure:
• COT release alerts
• News event alerts (High/Med/All impact)
• Currency filter for alerts
• Alert timing (60/15/5, 15/5/1, 5/1 min)`

	case "settings":
		text = `⚙️ <b>Settings &amp; Preferences</b>

/settings — Preferences dashboard (alerts, language, model)
/membership — Tier info + upgrade · <code>/membership</code>
/clear — Clear AI chat history

<b>Available settings:</b>
• Language: Indonesian / English
• AI Provider: Claude / Gemini
• Claude Model: Opus / Sonnet / Haiku
• COT &amp; AI report alerts on/off
• Currency filter for alerts
• Alert timing presets`

	case "admin":
		// Only show admin section to admins
		isAdmin := h.bot.isOwner(userID)
		if !isAdmin && h.middleware != nil {
			role := h.middleware.GetUserRole(ctx, userID)
			isAdmin = domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin)
		}
		if !isAdmin {
			text = "⛔ Admin commands hanya tersedia untuk Admin+"
		} else {
			text = `🔐 <b>Admin Commands</b>

/users — List all registered users with roles
/setrole — Change user role · <code>/setrole &lt;userID&gt; &lt;role&gt;</code>
/ban — Ban a user · <code>/ban &lt;userID&gt;</code>
/unban — Unban a user · <code>/unban &lt;userID&gt;</code>

<b>Roles:</b> owner · admin · member · free · banned`
		}

	case "changelog":
		if h.changelog == "" {
			text = "📋 <b>Changelog</b>\n\n<i>Changelog tidak tersedia.</i>"
		} else {
			// Show a reasonable portion of the changelog
			cl := h.changelog
			if len(cl) > 3500 {
				cl = cl[:3500] + "\n\n<i>... (lihat selengkapnya di /settings → View Changelog)</i>"
			}
			text = "🆕 <b>What's New</b>\n\n" + cl
		}

	default:
		return h.sendHelp(ctx, chatID, userID)
	}

	kb := h.kb.HelpSubMenu()

	if editMsgID > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, editMsgID, text, kb)
	}
	_, err := h.bot.SendWithKeyboard(ctx, chatID, text, kb)
	return err
}

// cbHelp handles "help:" prefixed callbacks for the interactive help menu.
func (h *Handler) cbHelp(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "help:")

	if action == "back" {
		// Return to category menu
		isAdmin := h.bot.isOwner(userID)
		if !isAdmin && h.middleware != nil {
			role := h.middleware.GetUserRole(ctx, userID)
			isAdmin = domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin)
		}

		header := `🦅 <b>ARK Intelligence Terminal</b>
<i>Institutional Flow &amp; Macro Analytics</i>

<i>Pilih kategori untuk melihat commands tersedia:</i>`

		var kb ports.InlineKeyboard
		if isAdmin {
			kb = h.kb.HelpCategoryMenuWithAdmin()
		} else {
			kb = h.kb.HelpCategoryMenu()
		}
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, header, kb)
	}

	return h.sendHelpSubCategory(ctx, chatID, userID, action, msgID)
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
			h.saveLastCurrency(ctx, userID, code)
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

	h.bot.SendTyping(ctx, chatID)

	// Build conviction scores for overview (best-effort, non-fatal)
	var overviewConvictions []cot.ConvictionScore
	macroDataOv, fredErrOv := fred.GetCachedOrFetch(ctx)
	if fredErrOv == nil && macroDataOv != nil {
		compositeOv := fred.ComputeComposites(macroDataOv)
		regimeOv := fred.ClassifyMacroRegime(macroDataOv, compositeOv)
		var priceCtxsOv map[string]*domain.PriceContext
		if h.priceRepo != nil {
			ctxBuilderOv := pricesvc.NewContextBuilder(h.priceRepo)
			if pcs, pcErr := ctxBuilderOv.BuildAll(ctx); pcErr == nil {
				priceCtxsOv = pcs
			}
		}
		for _, a := range analyses {
			surpriseSigma := 0.0
			if h.newsScheduler != nil {
				surpriseSigma = h.newsScheduler.GetSurpriseSigma(a.Contract.Currency)
			}
			var pc *domain.PriceContext
			if priceCtxsOv != nil {
				pc = priceCtxsOv[a.Contract.Code]
			}
			cs := cot.ComputeConvictionScoreV3(a, regimeOv, surpriseSigma, "", macroDataOv, pc)
			overviewConvictions = append(overviewConvictions, cs)
		}
	}

	html := h.fmt.FormatCOTOverview(analyses, overviewConvictions)
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
	if h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pc, pcErr := ctxBuilder.Build(ctx, contractCode, displayCode); pcErr == nil && pc != nil {
			priceCtxMap = map[string]*domain.PriceContext{contractCode: pc}
			html += h.fmt.FormatPriceContext(pc)

			// Always show price-COT relationship — divergence warning OR alignment confirmation
			divs := pricesvc.DetectPriceCOTDivergences(priceCtxMap, []domain.COTAnalysis{*analysis})
			if len(divs) > 0 {
				html += h.fmt.FormatPriceCOTDivergence(divs[0])
			} else {
				html += h.fmt.FormatPriceCOTAlignment(pc, *analysis)
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
			composites := fred.ComputeComposites(macroData)
			regime := fred.ClassifyMacroRegime(macroData, composites)
			fredCtx := h.fmt.FormatFREDContext(macroData, regime)
			if fredCtx != "" {
				html += fredCtx
			}
		}
	}

	// Conviction Score — always shown, uses whatever data is available (FRED optional)
	if editMsgID == 0 && analysis != nil {
		surpriseSigma2 := 0.0
		if h.newsScheduler != nil {
			surpriseSigma2 = h.newsScheduler.GetSurpriseSigma(analysis.Contract.Currency)
		}
		var pc2 *domain.PriceContext
		if h.priceRepo != nil {
			ctxBuilder2 := pricesvc.NewContextBuilder(h.priceRepo)
			if pcs2, pcErr2 := ctxBuilder2.BuildAll(ctx); pcErr2 == nil {
				pc2 = pcs2[contractCode]
			}
		}
		macroData2, fredErr2 := fred.GetCachedOrFetch(ctx)
		if fredErr2 == nil && macroData2 != nil {
			composites2 := fred.ComputeComposites(macroData2)
			regime2 := fred.ClassifyMacroRegime(macroData2, composites2)
			cs := cot.ComputeConvictionScoreV3(*analysis, regime2, surpriseSigma2, "", macroData2, pc2)
			html += h.fmt.FormatConvictionBlock(cs)
		} else {
			// FRED unavailable — compute conviction with COT + price only (regime = zero value)
			cs := cot.ComputeConvictionScoreV3(*analysis, fred.MacroRegime{}, surpriseSigma2, "", nil, pc2)
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
			signals := recalDet.DetectAll([]domain.COTAnalysis{*analysis}, histMap, rCtx, priceCtxMap)
			if len(signals) > 0 {
				html += h.fmt.FormatBiasSummary(signals)
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
		// Build conviction scores for overview (best-effort, non-fatal)
		var cbConvictions []cot.ConvictionScore
		cbMacro, cbFredErr := fred.GetCachedOrFetch(ctx)
		if cbFredErr == nil && cbMacro != nil {
			cbComposites := fred.ComputeComposites(cbMacro)
		cbRegime := fred.ClassifyMacroRegime(cbMacro, cbComposites)
			var cbPriceCtxs map[string]*domain.PriceContext
			if h.priceRepo != nil {
				cbBuilder := pricesvc.NewContextBuilder(h.priceRepo)
				if pcs, pcErr := cbBuilder.BuildAll(ctx); pcErr == nil {
					cbPriceCtxs = pcs
				}
			}
			for _, a := range analyses {
				surpriseSigma := 0.0
				if h.newsScheduler != nil {
					surpriseSigma = h.newsScheduler.GetSurpriseSigma(a.Contract.Currency)
				}
				var pc *domain.PriceContext
				if cbPriceCtxs != nil {
					pc = cbPriceCtxs[a.Contract.Code]
				}
				cs := cot.ComputeConvictionScoreV3(a, cbRegime, surpriseSigma, "", cbMacro, pc)
				cbConvictions = append(cbConvictions, cs)
			}
		}
		html := h.fmt.FormatCOTOverview(analyses, cbConvictions)
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

	// Per-user AI quota check via middleware
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

	return h.generateOutlook(ctx, chatID, userID, 0)
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
	return h.generateOutlook(ctx, chatID, userID, msgID)
}

func (h *Handler) generateOutlook(ctx context.Context, chatID string, userID int64, editMsgID int) error {
	prefs, err := h.prefsRepo.Get(ctx, userID)
	if err != nil {
		prefs = domain.DefaultPrefs()
	}

	placeholderID := 0
	if editMsgID > 0 {
		_ = h.bot.EditMessage(ctx, chatID, editMsgID, "Generating unified intelligence report... ⏳\n(collecting all data sources + web search)")
		placeholderID = editMsgID
	} else {
		placeholderID, _ = h.bot.SendLoading(ctx, chatID, "Generating unified intelligence report... ⏳\n(collecting all data sources + web search)")
	}

	now := timeutil.NowWIB()

	// ---------- Collect ALL data sources (best-effort, non-fatal) ----------

	// COT
	cotAnalyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)

	// News
	weekEvts, _ := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))

	// FRED Macro
	macroData, _ := fred.GetCachedOrFetch(ctx)
	var macroRegime *fred.MacroRegime
	if macroData != nil {
		comp := fred.ComputeComposites(macroData)
		r := fred.ClassifyMacroRegime(macroData, comp)
		macroRegime = &r
	}

	// Price contexts
	var priceCtxs map[string]*domain.PriceContext
	if h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pc, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil && len(pc) > 0 {
			priceCtxs = pc
		}
	}

	// VIX/SPX risk context
	var riskCtx *domain.RiskContext
	if h.priceRepo != nil {
		riskBuilder := pricesvc.NewRiskContextBuilder(h.priceRepo)
		riskCtx, _ = riskBuilder.Build(ctx)
		if riskCtx != nil && macroData != nil {
			pricesvc.EnrichWithTermStructure(riskCtx, macroData.VIX3M)
		}
	}

	// Sentiment (CNN Fear & Greed)
	sentimentData, _ := sentiment.GetCachedOrFetch(ctx)

	// Seasonal patterns
	var seasonalData map[string]*pricesvc.SeasonalPattern
	if h.priceRepo != nil {
		sa := pricesvc.NewSeasonalAnalyzer(h.priceRepo)
		if patterns, saErr := sa.Analyze(ctx); saErr == nil && len(patterns) > 0 {
			seasonalData = make(map[string]*pricesvc.SeasonalPattern, len(patterns))
			for i := range patterns {
				seasonalData[patterns[i].ContractCode] = &patterns[i]
			}
		}
	}

	// Currency strength
	var currencyStrength []pricesvc.CurrencyStrength
	if len(priceCtxs) > 0 && len(cotAnalyses) > 0 {
		currencyStrength = pricesvc.ComputeCurrencyStrengthIndex(priceCtxs, cotAnalyses)
	}

	// Backtest stats
	var backtestStats *domain.BacktestStats
	if h.signalRepo != nil {
		sc := backtestsvc.NewStatsCalculator(h.signalRepo)
		if stats, bErr := sc.ComputeAll(ctx); bErr == nil {
			backtestStats = stats
		}
	}

	// World Bank cross-country macro fundamentals (graceful degradation on error)
	wbData, _ := worldbank.GetCachedOrFetch(ctx)

	// BIS REER/NEER currency valuation (graceful degradation on error)
	bisData, _ := bis.GetCachedOrFetch(ctx)

	// Daily price contexts (for daily technical analysis in outlook)
	var dailyPriceCtxs map[string]*domain.DailyPriceContext
	if h.dailyPriceRepo != nil {
		dailyBuilder := pricesvc.NewDailyContextBuilder(h.dailyPriceRepo)
		if dpc, dpcErr := dailyBuilder.BuildAll(ctx); dpcErr == nil && len(dpc) > 0 {
			dailyPriceCtxs = dpc
		}
	}

	// ---------- Build unified data ----------
	var macroComposites *domain.MacroComposites
	if macroData != nil {
		// Merge sentiment data into MacroData before computing composites,
		// so SentimentComposite includes CNN F&G, AAII, and CBOE P/C.
		if sentimentData != nil {
			fred.MergeSentiment(macroData,
				sentimentData.CNNFearGreed,
				sentimentData.AAIIBullBear,
				sentimentData.PutCallTotal,
				sentimentData.PutCallEquity,
				sentimentData.PutCallIndex,
			)
		}
		macroComposites = fred.ComputeComposites(macroData)
	}

	unifiedData := aisvc.UnifiedOutlookData{
		COTAnalyses:        cotAnalyses,
		NewsEvents:         weekEvts,
		MacroData:          macroData,
		MacroRegime:        macroRegime,
		MacroComposites:    macroComposites,
		PriceContexts:      priceCtxs,
		DailyPriceContexts: dailyPriceCtxs,
		RiskContext:         riskCtx,
		SentimentData:      sentimentData,
		SeasonalData:       seasonalData,
		BacktestStats:      backtestStats,
		CurrencyStrength:   currencyStrength,
		WorldBankData:      wbData,
		BISData:            bisData,
		Language:           prefs.Language,
	}

	// ---------- Route based on user's PreferredModel setting ----------
	var result string
	useClaude := prefs.PreferredModel != "gemini" && h.claudeAnalyzer != nil && h.claudeAnalyzer.IsAvailable()

	if useClaude {
		// Claude path: multi-phase unified outlook with thinking + web_search
		modelOverride := ""
		if prefs.ClaudeModel != "" && domain.IsValidClaudeModel(prefs.ClaudeModel) {
			modelOverride = string(prefs.ClaudeModel)
		}
		analyzer := h.claudeAnalyzer.WithModel(modelOverride)
		log.Info().
			Str("model", modelOverride).
			Int64("user_id", userID).
			Msg("/outlook unified routed to Claude (multi-phase)")
		result, err = analyzer.GenerateUnifiedOutlook(ctx, unifiedData)

		// If Claude fails (e.g. Vercel timeout on all phases), fall back to Gemini
		if err != nil || result == "" {
			log.Warn().Err(err).Msg("/outlook Claude failed, falling back to Gemini")
			weeklyData := ports.WeeklyData{
				COTAnalyses:   cotAnalyses,
				NewsEvents:    weekEvts,
				MacroData:     macroData,
				BacktestStats: backtestStats,
				PriceContexts: priceCtxs,
				Language:      prefs.Language,
			}
			result, err = h.aiAnalyzer.AnalyzeCombinedOutlook(ctx, weeklyData)
		}
	} else {
		// Gemini path: direct combined outlook (no web search capability)
		log.Info().Int64("user_id", userID).Msg("/outlook routed to Gemini")
		weeklyData := ports.WeeklyData{
			COTAnalyses:   cotAnalyses,
			NewsEvents:    weekEvts,
			MacroData:     macroData,
			BacktestStats: backtestStats,
			PriceContexts: priceCtxs,
			Language:      prefs.Language,
		}
		result, err = h.aiAnalyzer.AnalyzeCombinedOutlook(ctx, weeklyData)
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
	case "model_claude":
		prefs.PreferredModel = "claude"
	case "model_gemini":
		prefs.PreferredModel = "gemini"
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
		} else if strings.HasPrefix(action, "claude_model:") {
			// Handle set:claude_model:claude-opus-4-5 etc (specific Claude variant)
			modelID := domain.ClaudeModelID(strings.TrimPrefix(action, "claude_model:"))
			if domain.IsValidClaudeModel(modelID) {
				prefs.ClaudeModel = modelID
				// Automatically switch provider to Claude when a Claude model is selected
				prefs.PreferredModel = "claude"
				log.Info().Str("model", string(modelID)).Int64("user_id", userID).Msg("user selected Claude model variant")
			} else {
				log.Warn().Str("model", string(modelID)).Msg("unknown Claude model ID in settings callback")
				return nil
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
// /bias — COT Directional Bias
// ---------------------------------------------------------------------------

func (h *Handler) cmdBias(ctx context.Context, chatID string, userID int64, args string) error {
	loadingID, _ := h.bot.SendHTML(ctx, chatID, "🎯 Mendeteksi directional bias... ⏳")
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err = h.bot.SendHTML(ctx, chatID, "No COT data available for bias detection.")
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
	var priceCtxsBias map[string]*domain.PriceContext
	if h.priceRepo != nil {
		rb := pricesvc.NewRiskContextBuilder(h.priceRepo)
		riskCtx, _ = rb.Build(ctx)
		// Build price contexts for ATR volatility multiplier
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pcs, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil {
			priceCtxsBias = pcs
		}
	}
	signals := recalDetector.DetectAll(analyses, historyMap, riskCtx, priceCtxsBias)

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

	html := h.fmt.FormatBiasHTML(signals, filterCurrency)

	// Append USD Aggregate COT signal when showing all currencies (no filter).
	if filterCurrency == "" {
		usdAgg := cot.ComputeUSDAggregate(analyses, domain.DefaultCOTContracts)
		if aggHTML := cot.FormatUSDAggregate(usdAgg); aggHTML != "" {
			html += aggHTML
		}
	}

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------// AI cooldown helper
// ---------------------------------------------------------------------------

// aiCooldownDuration is the minimum interval between AI-heavy commands per user.
var aiCooldownDuration = config.AICooldownDefault

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
