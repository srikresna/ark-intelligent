package telegram

import (
	"github.com/arkcode369/ark-intelligent/internal/config"
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/sentiment"
	"github.com/arkcode369/ark-intelligent/internal/service/worldbank"
	"github.com/arkcode369/ark-intelligent/internal/service/imf"
	"github.com/arkcode369/ark-intelligent/internal/service/fed"
	"github.com/arkcode369/ark-intelligent/internal/service/bis"
	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/defillama"
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
	bot.RegisterCommand("/price", h.cmdPrice)       // Daily price context
	bot.RegisterCommand("/levels", h.cmdLevels)     // Support/resistance levels + position sizing
	bot.RegisterCommand("/intermarket", h.cmdIntermarket) // Intermarket correlation signals

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
	bot.RegisterCallback("share:", h.cbShare)

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

	// IMF WEO forecasts — forward-looking GDP, CPI, current account (graceful degradation)
	imfData, _ := imf.GetCachedOrFetch(ctx)

	// CME FedWatch implied rate probabilities (graceful degradation)
	fedWatchData := fed.FetchFedWatch(ctx)

	// DeFiLlama TVL — DeFi total value locked (graceful degradation on error)
	tvlData := defillama.GetCachedOrFetch(ctx)

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
		IMFData:            imfData,
		FedWatchData:       fedWatchData,
		DeFiLlamaTVL:       tvlData,
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
	case "output_mode_toggle":
		prefs.OutputMode = domain.NextOutputMode(prefs.OutputMode)
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

// ---------------------------------------------------------------------------
// AI cooldown helper
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



// cmdHistory shows COT positioning history for a currency.
// Usage: /history EUR [4|8|12] — weeks of history (default 4)
func (h *Handler) cmdHistory(ctx context.Context, chatID string, userID int64, args string) error {
	h.bot.SendTyping(ctx, chatID)

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) == 0 {
		// Try last currency
		lc := h.getLastCurrency(ctx, userID)
		if lc != "" {
			parts = []string{lc}
		} else {
			_, err := h.bot.SendHTML(ctx, chatID,
				"📊 <b>COT History</b>\n\nUsage: <code>/history EUR</code> atau <code>/h GBP 8</code>\n\nTampilkan positioning history 4-12 minggu terakhir.")
			return err
		}
	}

	currency := parts[0]
	weeks := 4
	if len(parts) > 1 {
		if w, err := strconv.Atoi(parts[1]); err == nil && w >= 2 && w <= 52 {
			weeks = w
		}
	}

	h.saveLastCurrency(ctx, userID, currency)
	contractCode := currencyToContractCode(currency)

	records, err := h.cotRepo.GetHistory(ctx, contractCode, weeks)
	if err != nil || len(records) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("no history for %s", currency), "history")
		return nil
	}

	// Build history view
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📊 <b>COT History — %s (%d weeks)</b>\n", currency, len(records)))
	b.WriteString(fmt.Sprintf("<i>%s → %s</i>\n\n", records[len(records)-1].ReportDate.Format("02 Jan"), records[0].ReportDate.Format("02 Jan 2006")))

	// Sparkline of net position
	netPositions := make([]float64, len(records))
	for i, r := range records {
		netPositions[i] = r.GetSmartMoneyNet("TFF")
	}
	// Reverse for sparkline (oldest first)
	for i, j := 0, len(netPositions)-1; i < j; i, j = i+1, j-1 {
		netPositions[i], netPositions[j] = netPositions[j], netPositions[i]
	}
	b.WriteString("📈 Net Position Trend: <code>")
	b.WriteString(sparkLine(netPositions))
	b.WriteString("</code>\n\n")

	// Table
	b.WriteString("<pre>")
	b.WriteString("Date       | Net Pos   | Chg      | L/S\n")
	b.WriteString("───────────┼───────────┼──────────┼────\n")
	for i, r := range records {
		net := int64(r.GetSmartMoneyNet("TFF"))
		var chg int64
		if i+1 < len(records) {
			prevNet := int64(records[i+1].GetSmartMoneyNet("TFF"))
			chg = net - prevNet
		}
		ratio := 0.0
		if r.LevFundShort > 0 {
			ratio = r.LevFundLong / r.LevFundShort
		}
		b.WriteString(fmt.Sprintf("%-10s | %+9d | %+8d | %.2f\n",
			r.ReportDate.Format("02 Jan"), net, chg, ratio))
	}
	b.WriteString("</pre>")

	_, err = h.bot.SendHTML(ctx, chatID, b.String())
	return err
}


// sparkLine generates a Unicode sparkline from a slice of values.
func sparkLine(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	if span == 0 {
		return strings.Repeat("▄", len(values))
	}
	var result []rune
	for _, v := range values {
		idx := int((v - min) / span * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		result = append(result, blocks[idx])
	}
	return string(result)
}

// ---------------------------------------------------------------------------
// /compare — Side-by-side COT comparison for two currencies
// ---------------------------------------------------------------------------

// cmdCompare handles /compare EUR GBP — shows side-by-side COT positioning.
func (h *Handler) cmdCompare(ctx context.Context, chatID string, userID int64, args string) error {
	h.bot.SendTyping(ctx, chatID)

	parts := strings.Fields(strings.ToUpper(strings.TrimSpace(args)))
	if len(parts) < 2 {
		_, err := h.bot.SendHTML(ctx, chatID,
			"⚖️ <b>COT Compare</b>\n\nUsage: <code>/compare EUR GBP</code>\n\nBandingkan positioning dua aset secara side-by-side.")
		return err
	}

	currA, currB := parts[0], parts[1]
	codeA := currencyToContractCode(currA)
	codeB := currencyToContractCode(currB)

	recsA, errA := h.cotRepo.GetHistory(ctx, codeA, 1)
	recsB, errB := h.cotRepo.GetHistory(ctx, codeB, 1)
	if errA != nil || len(recsA) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("no data for %s", currA), "compare")
		return nil
	}
	if errB != nil || len(recsB) == 0 {
		h.sendUserError(ctx, chatID, fmt.Errorf("no data for %s", currB), "compare")
		return nil
	}

	rA, rB := recsA[0], recsB[0]

	// Detect report type per contract for correct net calculation
	rtA, rtB := "TFF", "TFF"
	analyses, _ := h.cotRepo.GetAllLatestAnalyses(ctx)
	for _, a := range analyses {
		if a.Contract.Code == codeA {
			rtA = a.Contract.ReportType
		}
		if a.Contract.Code == codeB {
			rtB = a.Contract.ReportType
		}
	}
	netA := rA.GetSmartMoneyNet(rtA)
	netB := rB.GetSmartMoneyNet(rtB)

	biasA, iconA := cotBiasLabel(netA)
	biasB, iconB := cotBiasLabel(netB)
	chgLabelA := cotFormatChg(rA.NetChange)
	chgLabelB := cotFormatChg(rB.NetChange)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("⚖️ <b>COT Compare — %s vs %s</b>\n", currA, currB))
	b.WriteString(fmt.Sprintf("<i>Report: %s</i>\n\n", rA.ReportDate.Format("02 Jan 2006")))
	b.WriteString("<pre>")
	b.WriteString(fmt.Sprintf("%-12s  %-15s  %-15s\n", "", currA, currB))
	b.WriteString(fmt.Sprintf("%-12s  %-15s  %-15s\n", "Net Pos", fmt.Sprintf("%+.0f", netA), fmt.Sprintf("%+.0f", netB)))
	b.WriteString(fmt.Sprintf("%-12s  %-15s  %-15s\n", "WoW Chg", chgLabelA, chgLabelB))
	b.WriteString(fmt.Sprintf("%-12s  %-15s  %-15s\n", "Bias", biasA, biasB))
	b.WriteString("</pre>\n")
	b.WriteString(fmt.Sprintf("\n%s <b>%s</b> %s   |   %s <b>%s</b> %s",
		iconA, currA, biasA, iconB, currB, biasB))

	_, err := h.bot.SendHTML(ctx, chatID, b.String())
	return err
}

// cotBiasLabel returns a human-readable bias label and icon for a net position value.
func cotBiasLabel(net float64) (string, string) {
	if net > 5000 {
		return "BULLISH", "🟢"
	}
	if net < -5000 {
		return "BEARISH", "🔴"
	}
	return "NEUTRAL", "🟡"
}

// cotFormatChg formats a WoW change value with sign and K-suffix for readability.
func cotFormatChg(chg float64) string {
	if chg == 0 {
		return "N/A"
	}
	if chg >= 1000 || chg <= -1000 {
		return fmt.Sprintf("%+.1fK", chg/1000)
	}
	return fmt.Sprintf("%+.0f", chg)
}

// ---------------------------------------------------------------------------
// cbHistoryNav — History view navigation (week range toggle)
// ---------------------------------------------------------------------------

// cbHistoryNav handles "hist:<currency>:<weeks>" callbacks from inline keyboard buttons.
func (h *Handler) cbHistoryNav(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// data format: "hist:EUR:8"
	trimmed := strings.TrimPrefix(data, "hist:")
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	currency := strings.ToUpper(parts[0])
	weeks := 4
	if w, err := strconv.Atoi(parts[1]); err == nil && w >= 2 && w <= 52 {
		weeks = w
	}

	contractCode := currencyToContractCode(currency)
	records, err := h.cotRepo.GetHistory(ctx, contractCode, weeks)
	if err != nil || len(records) == 0 {
		return h.bot.EditMessage(ctx, chatID, msgID,
			fmt.Sprintf("⚠️ Tidak ada data history untuk %s.", currency))
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📊 <b>COT History — %s (%d weeks)</b>\n", currency, len(records)))
	b.WriteString(fmt.Sprintf("<i>%s → %s</i>\n\n",
		records[len(records)-1].ReportDate.Format("02 Jan"),
		records[0].ReportDate.Format("02 Jan 2006")))

	netPositions := make([]float64, len(records))
	for i, r := range records {
		netPositions[i] = r.GetSmartMoneyNet("TFF")
	}
	for i, j := 0, len(netPositions)-1; i < j; i, j = i+1, j-1 {
		netPositions[i], netPositions[j] = netPositions[j], netPositions[i]
	}
	b.WriteString("📈 Trend: <code>")
	b.WriteString(sparkLine(netPositions))
	b.WriteString("</code>\n\n")

	b.WriteString("<pre>")
	b.WriteString("Date       | Net Pos   | Chg      | L/S\n")
	b.WriteString("───────────┼───────────┼──────────┼────\n")
	for i, r := range records {
		net := int64(r.GetSmartMoneyNet("TFF"))
		var chg int64
		if i+1 < len(records) {
			prevNet := int64(records[i+1].GetSmartMoneyNet("TFF"))
			chg = net - prevNet
		}
		ratio := 0.0
		if r.LevFundShort > 0 {
			ratio = r.LevFundLong / r.LevFundShort
		}
		b.WriteString(fmt.Sprintf("%-10s | %+9d | %+8d | %.2f\n",
			r.ReportDate.Format("02 Jan"), net, chg, ratio))
	}
	b.WriteString("</pre>")

	navKB := ports.InlineKeyboard{
		Rows: [][]ports.InlineButton{
			{
				historyNavBtn(currency, 4, weeks),
				historyNavBtn(currency, 8, weeks),
				historyNavBtn(currency, 12, weeks),
			},
		},
	}
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, b.String(), navKB)
}

// historyNavBtn creates a history navigation button; marks the active week range with ✓.
func historyNavBtn(currency string, targetWeeks, activeWeeks int) ports.InlineButton {
	label := fmt.Sprintf("%dW", targetWeeks)
	if targetWeeks == activeWeeks {
		label += " ✓"
	}
	return ports.InlineButton{
		Text:         label,
		CallbackData: fmt.Sprintf("hist:%s:%d", currency, targetWeeks),
	}
}

// saveLastCurrency persists the user's last viewed currency for context carry-over.
func (h *Handler) saveLastCurrency(ctx context.Context, userID int64, currency string) {
	if currency == "" {
		return
	}
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	prefs.LastCurrency = strings.ToUpper(currency)
	_ = h.prefsRepo.Set(ctx, userID, prefs)
}

// getLastCurrency returns the user's last viewed currency, or empty string.
func (h *Handler) getLastCurrency(ctx context.Context, userID int64) string {
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	return prefs.LastCurrency
}

// resolveOrLastCurrency returns the given currency if non-empty, otherwise the user's last currency.
func (h *Handler) resolveOrLastCurrency(ctx context.Context, userID int64, currency string) string {
	if currency != "" {
		return currency
	}
	return h.getLastCurrency(ctx, userID)
}


// cbShare handles "share:<type>:<key>" callbacks.
// Generates a plain-text, copy-paste friendly version of the analysis
// and sends it wrapped in <code> tags for easy copying in Telegram.
func (h *Handler) cbShare(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	// data format: "share:<type>:<key>"
	trimmed := strings.TrimPrefix(data, "share:")
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) < 2 {
		return nil
	}
	shareType, key := parts[0], parts[1]

	switch shareType {
	case "cot":
		return h.shareCOT(ctx, chatID, key)
	case "outlook":
		// Outlook share — placeholder for future implementation
		_, err := h.bot.SendHTML(ctx, chatID, "<i>Outlook share coming soon.</i>")
		return err
	default:
		return nil
	}
}

// shareCOT generates and sends a plain-text COT share message.
func (h *Handler) shareCOT(ctx context.Context, chatID string, contractCode string) error {
	analysis, err := h.cotRepo.GetLatestAnalysis(ctx, contractCode)
	if err != nil {
		_, sendErr := h.bot.SendHTML(ctx, chatID, "<i>COT data not available for sharing.</i>")
		return sendErr
	}

	shareText := h.fmt.FormatCOTShareText(*analysis)

	// Wrap in <code> block for easy copy on Telegram
	shareHTML := fmt.Sprintf("<code>%s</code>", html.EscapeString(shareText))
	_, err = h.bot.SendHTML(ctx, chatID, shareHTML)
	return err
}

// cbViewToggle handles compact/full view toggle callbacks.
// Callback data format: "view:<action>:<command>"
func (h *Handler) cbViewToggle(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	parts := strings.SplitN(strings.TrimPrefix(data, "view:"), ":", 2)
	if len(parts) < 2 {
		return nil
	}
	action, command := parts[0], parts[1]

	prefs, _ := h.prefsRepo.Get(ctx, userID)

	switch action {
	case "full":
		prefs.OutputMode = domain.OutputFull
	case "compact":
		prefs.OutputMode = domain.OutputCompact
	case "minimal":
		prefs.OutputMode = domain.OutputMinimal
	default:
		return nil
	}

	_ = h.prefsRepo.Set(ctx, userID, prefs)

	switch command {
	case "cot":
		return h.renderCOTOverview(ctx, chatID, userID, msgID)
	case "macro":
		return h.renderMacroSummary(ctx, chatID, userID, msgID)
	default:
		return nil
	}
}

// renderCOTOverview renders COT overview in compact or full mode based on prefs.
func (h *Handler) renderCOTOverview(ctx context.Context, chatID string, userID int64, editMsgID int) error {
	prefs, _ := h.prefsRepo.Get(ctx, userID)
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		return nil
	}

	// Build convictions (best-effort)
	var convictions []cot.ConvictionScore
	macroData, fredErr := fred.GetCachedOrFetch(ctx)
	if fredErr == nil && macroData != nil {
		composites := fred.ComputeComposites(macroData)
		regime := fred.ClassifyMacroRegime(macroData, composites)
		for _, a := range analyses {
			cs := cot.ComputeConvictionScoreV3(a, regime, 0, "", macroData, nil)
			convictions = append(convictions, cs)
		}
	}

	var htmlOut string
	var toggleBtns []ports.InlineButton
	switch prefs.OutputMode {
	case domain.OutputFull:
		htmlOut = h.fmt.FormatCOTOverview(analyses, convictions)
		toggleBtns = []ports.InlineButton{
			{Text: btnCompact, CallbackData: "view:compact:cot"},
			{Text: "⚡ Minimal", CallbackData: "view:minimal:cot"},
		}
	case domain.OutputMinimal:
		htmlOut = h.fmt.FormatCOTOverviewMinimal(analyses, convictions)
		toggleBtns = []ports.InlineButton{
			{Text: btnCompact, CallbackData: "view:compact:cot"},
			{Text: btnExpand, CallbackData: "view:full:cot"},
		}
	default: // compact
		htmlOut = h.fmt.FormatCOTOverviewCompact(analyses, convictions)
		toggleBtns = []ports.InlineButton{
			{Text: btnExpand, CallbackData: "view:full:cot"},
			{Text: "⚡ Minimal", CallbackData: "view:minimal:cot"},
		}
	}

	kb := h.kb.COTCurrencySelector(analyses)
	kb.Rows = append([][]ports.InlineButton{toggleBtns}, kb.Rows...)

	if editMsgID > 0 {
		return h.bot.EditWithKeyboardChunked(ctx, chatID, editMsgID, htmlOut, kb)
	}
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, htmlOut, kb)
	return err
}

// renderMacroSummary renders macro dashboard in compact or full mode.
func (h *Handler) renderMacroSummary(ctx context.Context, chatID string, userID int64, editMsgID int) error {
	prefs, _ := h.prefsRepo.Get(ctx, userID)

	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil || data == nil {
		return nil
	}
	composites := fred.ComputeComposites(data)
	regime := fred.ClassifyMacroRegime(data, composites)

	var htmlOut string
	var toggleBtns []ports.InlineButton
	switch prefs.OutputMode {
	case domain.OutputFull:
		implications := fred.DeriveTradingImplications(regime, data)
		htmlOut = h.fmt.FormatMacroSummary(regime, data, implications)
		toggleBtns = []ports.InlineButton{
			{Text: btnCompact, CallbackData: "view:compact:macro"},
			{Text: "⚡ Minimal", CallbackData: "view:minimal:macro"},
		}
	case domain.OutputMinimal:
		htmlOut = h.fmt.FormatMacroSummaryMinimal(regime, data)
		toggleBtns = []ports.InlineButton{
			{Text: btnCompact, CallbackData: "view:compact:macro"},
			{Text: btnExpand, CallbackData: "view:full:macro"},
		}
	default: // compact
		htmlOut = h.fmt.FormatMacroSummaryCompact(regime, data)
		toggleBtns = []ports.InlineButton{
			{Text: btnExpand, CallbackData: "view:full:macro"},
			{Text: "⚡ Minimal", CallbackData: "view:minimal:macro"},
		}
	}

	kb := h.kb.MacroMenu(false)
	kb.Rows = append([][]ports.InlineButton{toggleBtns}, kb.Rows...)

	if editMsgID > 0 {
		return h.bot.EditWithKeyboardChunked(ctx, chatID, editMsgID, htmlOut, kb)
	}
	_, err = h.bot.SendWithKeyboardChunked(ctx, chatID, htmlOut, kb)
	return err
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

// ---------------------------------------------------------------------------
// P1.3 — /rank — Currency Strength Ranking
// ---------------------------------------------------------------------------

// cmdRank handles the /rank command — weekly currency strength ranking.
// Ranks 8 major currencies by COT SentimentScore and shows conviction scores (COT + FRED + Calendar).
func (h *Handler) cmdRank(ctx context.Context, chatID string, userID int64, args string) error {
	loadingID, _ := h.bot.SendHTML(ctx, chatID, "📈 Menghitung currency strength ranking... ⏳")
	analyses, err := h.cotRepo.GetAllLatestAnalyses(ctx)
	if err != nil || len(analyses) == 0 {
		if loadingID > 0 {
			_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
		}
		_, err = h.bot.SendHTML(ctx, chatID,
			"No COT data available for ranking. Data is fetched from CFTC every Friday.")
		return err
	}

	// Fetch FRED regime for conviction scoring (best-effort, non-fatal)
	var macroData *fred.MacroData
	var regime *fred.MacroRegime
	if md, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && md != nil {
		macroData = md
		comp := fred.ComputeComposites(md)
		r := fred.ClassifyMacroRegime(md, comp)
		regime = &r
	}

	// Build price contexts for V3 conviction scoring + strength ranking (best-effort)
	var priceCtxs map[string]*domain.PriceContext
	if h.priceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(h.priceRepo)
		if pcs, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil && len(pcs) > 0 {
			priceCtxs = pcs
		}
	}

	// Compute conviction scores for each currency (full 5-source V3: COT + Calendar + Stress + FRED + Price)
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
		var pc *domain.PriceContext
		if priceCtxs != nil {
			pc = priceCtxs[a.Contract.Code]
		}
		cs := cot.ComputeConvictionScoreV3(a, r, surpriseSigma, "", macroData, pc)
		convictions = append(convictions, cs)
	}

	now := timeutil.NowWIB()
	html := h.fmt.FormatRankingWithConviction(analyses, convictions, regime, now)

	// Dual price + COT strength ranking (best-effort, non-fatal)
	if priceCtxs != nil {
		strengths := pricesvc.ComputeCurrencyStrengthIndex(priceCtxs, analyses)
		if len(strengths) > 0 {
			html += h.fmt.FormatStrengthRanking(strengths)
		}
	}

	// Daily momentum snapshot (best-effort, non-fatal)
	if h.dailyPriceRepo != nil {
		dailyBuilder := pricesvc.NewDailyContextBuilder(h.dailyPriceRepo)
		if dailyCtxs, dErr := dailyBuilder.BuildAll(ctx); dErr == nil && len(dailyCtxs) > 0 {
			html += h.fmt.FormatDailyMomentumSnapshot(dailyCtxs)
		}
	}

	if loadingID > 0 {
		_ = h.bot.DeleteMessage(ctx, chatID, loadingID)
	}
	_, err = h.bot.SendHTML(ctx, chatID, html)
	return err
}

// ---------------------------------------------------------------------------
// P3.2 — /macro — FRED Macro Regime Dashboard
// ---------------------------------------------------------------------------

// cmdMacro handles the /macro command — shows plain-language summary with inline navigation.
// Subcommands: /macro detail, /macro explain, /macro matrix|performance, /macro refresh (admin).
func (h *Handler) cmdMacro(ctx context.Context, chatID string, userID int64, args string) error {
	upper := strings.ToUpper(strings.TrimSpace(args))

	// Subcommand routing
	if upper == "MATRIX" || upper == "PERFORMANCE" {
		return h.macroRegimePerformance(ctx, chatID, 0)
	}

	forceRefresh := upper == "REFRESH"
	if forceRefresh {
		if !h.requireAdmin(ctx, chatID, userID) {
			return nil
		}
		fred.InvalidateCache()
	}

	cacheStatus := "🏦 Fetching FRED macro data... ⏳ (5-15s)"
	if !forceRefresh && fred.CacheAge() >= 0 {
		cacheStatus = "🏦 Loading FRED macro data (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, cacheStatus)

	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("FRED data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"Failed to fetch macro data. Please try again later.")
	}

	// Merge sentiment data into MacroData for complete composite scoring
	if sentData, sentErr := sentiment.GetCachedOrFetch(ctx); sentErr == nil && sentData != nil {
		fred.MergeSentiment(data,
			sentData.CNNFearGreed,
			sentData.AAIIBullBear,
			sentData.PutCallTotal,
			sentData.PutCallEquity,
			sentData.PutCallIndex,
		)
	}

	composites := fred.ComputeComposites(data)
	regime := fred.ClassifyMacroRegime(data, composites)

	// Route to specific view
	switch upper {
	case "DETAIL":
		return h.macroSendDetail(ctx, chatID, placeholderID, regime, data)
	case "EXPLAIN":
		htmlMsg := h.fmt.FormatMacroExplain(regime, data)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	case "COMPOSITES":
		htmlMsg := h.fmt.FormatMacroComposites(composites, data)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	case "GLOBAL":
		htmlMsg := h.fmt.FormatMacroGlobal(composites, data)
		// Append World Bank annual fundamentals (best-effort; non-blocking on error)
		if wbData, wbErr := fred.GetWorldBankCachedOrFetch(ctx); wbErr == nil && wbData != nil {
			htmlMsg += h.fmt.FormatWorldBankFundamentals(wbData)
		}
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	case "LABOR":
		htmlMsg := h.fmt.FormatMacroLabor(composites, data)
		kb := h.kb.MacroDrillDownMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	case "INFLATION":
		htmlMsg := h.fmt.FormatMacroInflation(composites, data)
		kb := h.kb.MacroDrillDownMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, placeholderID, htmlMsg, kb)
	}

	// Default: plain-language summary with inline keyboard
	return h.macroSendSummary(ctx, chatID, placeholderID, userID, regime, data)
}

// macroSendSummary sends the plain-language macro summary with inline navigation buttons.
func (h *Handler) macroSendSummary(ctx context.Context, chatID string, msgID int, userID int64, regime fred.MacroRegime, data *fred.MacroData) error {
	implications := fred.DeriveTradingImplications(regime, data)
	htmlMsg := h.fmt.FormatMacroSummary(regime, data, implications)

	isAdmin := false
	if h.middleware != nil {
		role := h.middleware.GetUserRole(ctx, userID)
		isAdmin = domain.RoleHierarchy(role) >= domain.RoleHierarchy(domain.RoleAdmin)
	} else {
		isAdmin = h.bot.isOwner(userID)
	}

	kb := h.kb.MacroMenu(isAdmin)
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)
}

// macroSendDetail sends the full technical dashboard with back-navigation.
func (h *Handler) macroSendDetail(ctx context.Context, chatID string, msgID int, regime fred.MacroRegime, data *fred.MacroData) error {
	htmlMsg := h.fmt.FormatMacroRegime(regime, data)

	// Append regime-asset performance insight if price data is available.
	if h.priceRepo != nil {
		insight := h.buildRegimeAssetInsight(ctx, data, regime)
		if formatted := h.fmt.FormatRegimeAssetInsight(insight); formatted != "" {
			htmlMsg += formatted
		}
	}

	kb := h.kb.MacroDetailMenu()
	return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)
}

// cbMacro handles inline keyboard callbacks for the macro dashboard navigation.
func (h *Handler) cbMacro(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	action := strings.TrimPrefix(data, "macro:")

	// Get cached FRED data (should already be in cache from initial /macro call)
	macroData, err := fred.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("FRED data fetch failed in macro callback")
		return h.bot.EditMessage(ctx, chatID, msgID, "Failed to load macro data. Ketik /macro untuk coba lagi.")
	}
	composites := fred.ComputeComposites(macroData)
	regime := fred.ClassifyMacroRegime(macroData, composites)

	switch action {
	case "detail":
		return h.macroSendDetail(ctx, chatID, msgID, regime, macroData)

	case "explain":
		htmlMsg := h.fmt.FormatMacroExplain(regime, macroData)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "summary":
		return h.macroSendSummary(ctx, chatID, msgID, userID, regime, macroData)

	case "performance":
		return h.macroRegimePerformance(ctx, chatID, msgID)

	case "composites":
		htmlMsg := h.fmt.FormatMacroComposites(composites, macroData)
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "global":
		htmlMsg := h.fmt.FormatMacroGlobal(composites, macroData)
		// Append World Bank annual fundamentals (best-effort; non-blocking on error)
		if wbData, wbErr := fred.GetWorldBankCachedOrFetch(ctx); wbErr == nil && wbData != nil {
			htmlMsg += h.fmt.FormatWorldBankFundamentals(wbData)
		}
		kb := h.kb.MacroDetailMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "labor":
		htmlMsg := h.fmt.FormatMacroLabor(composites, macroData)
		kb := h.kb.MacroDrillDownMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "inflation":
		htmlMsg := h.fmt.FormatMacroInflation(composites, macroData)
		kb := h.kb.MacroDrillDownMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlMsg, kb)

	case "refresh":
		if !h.requireAdmin(ctx, chatID, userID) {
			return nil
		}
		fred.InvalidateCache()
		freshData, err := fred.GetCachedOrFetch(ctx)
		if err != nil {
			return h.bot.EditMessage(ctx, chatID, msgID, "Failed to refresh macro data.")
		}
		freshComposites := fred.ComputeComposites(freshData)
		freshRegime := fred.ClassifyMacroRegime(freshData, freshComposites)
		return h.macroSendSummary(ctx, chatID, msgID, userID, freshRegime, freshData)
	}

	return nil
}

// buildRegimeAssetInsight computes the regime-asset matrix from stored price
// history and returns insight for the current regime.
func (h *Handler) buildRegimeAssetInsight(ctx context.Context, data *fred.MacroData, regime fred.MacroRegime) fred.RegimeInsight {
	const lookbackWeeks = 52

	// Build regime history from current FRED data.
	regimeHistory := fred.BuildRegimeHistoryFromCurrent(data, lookbackWeeks)
	if len(regimeHistory) == 0 {
		return fred.RegimeInsight{Regime: regime.Name}
	}

	// Fetch price history for all COT-tracked contracts.
	priceHistory := make(map[string][]domain.PriceRecord)
	for _, m := range domain.COTPriceSymbolMappings() {
		records, err := h.priceRepo.GetHistory(ctx, m.ContractCode, lookbackWeeks)
		if err != nil || len(records) == 0 {
			continue
		}
		priceHistory[m.ContractCode] = records
	}

	if len(priceHistory) == 0 {
		return fred.RegimeInsight{Regime: regime.Name}
	}

	matrix := fred.ComputeRegimeAssetMatrix(regimeHistory, priceHistory)
	return fred.GetCurrentRegimeInsight(regime.Name, matrix)
}

// macroRegimePerformance builds and sends the regime-asset performance matrix
// from historical persisted signals with FRED regime labels.
// If msgID > 0, edits the existing message; otherwise sends a new message.
func (h *Handler) macroRegimePerformance(ctx context.Context, chatID string, msgID int) error {
	if h.signalRepo == nil {
		msg := "Regime performance requires signal history with FRED regime data."
		if msgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID, msg, h.kb.MacroDetailMenu())
		}
		_, err := h.bot.SendHTML(ctx, chatID, msg)
		return err
	}

	builder := fred.NewRegimePerformanceBuilder(h.signalRepo)
	matrix, err := builder.Build(ctx)
	if err != nil {
		errMsg := userFriendlyError(err, "macro")
		if msgID > 0 {
			return h.bot.EditWithKeyboard(ctx, chatID, msgID, errMsg, h.kb.MacroDetailMenu())
		}
		_, sendErr := h.bot.SendHTML(ctx, chatID, errMsg)
		return sendErr
	}

	htmlOut := h.fmt.FormatRegimePerformance(matrix)
	kb := h.kb.MacroDetailMenu()
	if msgID > 0 {
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, htmlOut, kb)
	}
	// Fallback: send as new message with keyboard
	_, err = h.bot.SendWithKeyboard(ctx, chatID, htmlOut, kb)
	return err
}

// currentMacroRegimeName returns the current FRED macro regime name from cache.
// Returns "" if FRED data is unavailable (never blocks on a network fetch).
func (h *Handler) currentMacroRegimeName(ctx context.Context) string {
	// Only use cached data — don't trigger a FRED fetch just for sentiment context
	if fred.CacheAge() < 0 {
		return ""
	}
	data, err := fred.GetCachedOrFetch(ctx)
	if err != nil || data == nil {
		return ""
	}
	composites := fred.ComputeComposites(data)
	regime := fred.ClassifyMacroRegime(data, composites)
	return regime.Name
}

// ---------------------------------------------------------------------------
// /sentiment — Sentiment Survey Dashboard
// ---------------------------------------------------------------------------

func (h *Handler) cmdSentiment(ctx context.Context, chatID string, userID int64, args string) error {
	forceRefresh := strings.EqualFold(strings.TrimSpace(args), "refresh")
	if forceRefresh {
		if !h.requireAdmin(ctx, chatID, userID) {
			return nil
		}
		sentiment.InvalidateCache()
	}

	cacheStatus := "🧠 Fetching sentiment data... ⏳"
	if !forceRefresh && sentiment.CacheAge() >= 0 {
		cacheStatus = "🧠 Loading sentiment data (from cache)... ⏳"
	}
	placeholderID, _ := h.bot.SendLoading(ctx, chatID, cacheStatus)

	data, err := sentiment.GetCachedOrFetch(ctx)
	if err != nil {
		log.Error().Err(err).Msg("sentiment data fetch failed")
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"Failed to fetch sentiment data. Please try again later.")
	}

	if !data.CNNAvailable && !data.AAIIAvailable {
		return h.bot.EditMessage(ctx, chatID, placeholderID,
			"⚠️ Sentiment data currently unavailable from all sources. Try again later.")
	}

	htmlMsg := h.fmt.FormatSentiment(data, h.currentMacroRegimeName(ctx))
	return h.bot.EditMessage(ctx, chatID, placeholderID, htmlMsg)
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

	// Progress callback: updates the "thinking" message with tool activity status.
	// This lets the user see what the model is doing (e.g. "Searching the web...")
	// instead of staring at a static "Thinking..." message.
	onProgress := func(status string) {
		if thinkMsgID > 0 {
			_ = h.bot.EditMessage(ctx, chatID, thinkMsgID, status)
		}
	}

	// Get user role and preferred model for routing
	role := domain.RoleFree
	preferredModel := ""
	if h.middleware != nil {
		profile := h.middleware.GetUserProfile(ctx, userID)
		if profile != nil {
			role = profile.Role
		}
	}
	var claudeModelOverride string
	if prefs, err := h.prefsRepo.Get(ctx, userID); err == nil {
		preferredModel = prefs.PreferredModel
		// Pass specific Claude model variant if user selected one
		if prefs.ClaudeModel != "" && domain.IsValidClaudeModel(prefs.ClaudeModel) {
			claudeModelOverride = string(prefs.ClaudeModel)
		}
	}

	// Call chat service. No blanket timeout — the Claude HTTP client already has
	// a per-request timeout (default 120s) that handles hung requests.
	// As long as Claude keeps responding (tool round-trips), let it work freely.
	response, err := h.chatService.HandleMessage(ctx, userID, text, role, contentBlocks, onProgress, preferredModel, claudeModelOverride)

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

// ---------------------------------------------------------------------------
// /impact — Event Impact Database
// ---------------------------------------------------------------------------

// cmdImpact handles the /impact command.
// /impact        — lists all tracked events
// /impact <name> — shows historical price impact by sigma bucket
func (h *Handler) cmdImpact(ctx context.Context, chatID string, _ int64, args string) error {
	if h.impactProvider == nil {
		_, err := h.bot.SendHTML(ctx, chatID, "Event impact tracking is not available.")
		return err
	}

	query := strings.TrimSpace(args)

	// Resolve common abbreviations to full event names
	query = resolveEventAlias(query)

	// No arguments: show category keyboard
	if query == "" {
		kb := h.kb.ImpactCategoryMenu()
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			"📋 <b>EVENT IMPACT DATABASE</b>\n<i>Select a category to view event impacts:</i>\n\n<i>Or type directly:</i> <code>/impact NFP</code>",
			kb)
		return err
	}

	// Argument provided: look up impact summary for that event
	summaries, err := h.impactProvider.GetEventImpactSummary(ctx, query)
	if err != nil {
		log.Error().Err(err).Str("query", query).Msg("cmdImpact: get summary failed")
		_, sendErr := h.bot.SendHTML(ctx, chatID, "Failed to load impact data.")
		return sendErr
	}

	// If no results, try substring matching against tracked events
	if len(summaries) == 0 {
		events, listErr := h.impactProvider.GetTrackedEvents(ctx)
		if listErr == nil {
			matched := fuzzyMatchEvent(query, events)
			if matched != "" {
				summaries, err = h.impactProvider.GetEventImpactSummary(ctx, matched)
				if err != nil {
					log.Error().Err(err).Str("query", matched).Msg("cmdImpact: fuzzy match summary failed")
					_, sendErr := h.bot.SendHTML(ctx, chatID, "Failed to load impact data.")
					return sendErr
				}
				query = matched
			}
		}
	}

	if len(summaries) == 0 {
		kb := h.kb.ImpactCategoryMenu()
		_, err := h.bot.SendWithKeyboard(ctx, chatID,
			fmt.Sprintf("❌ Event <b>%s</b> not found in impact database.\n\n<i>Select from categories below or use known aliases:</i>\n<code>NFP, CPI, FOMC, BOE, GDP, PMI...</code>", html.EscapeString(query)),
			kb)
		return err
	}

	htmlOut := h.fmt.FormatEventImpact(query, summaries)
	_, err = h.bot.SendHTML(ctx, chatID, htmlOut)
	return err
}

// eventAliases maps common abbreviations to full event names.
var eventAliases = map[string]string{
	"NFP":      "Non-Farm Employment Change",
	"NONFARM":  "Non-Farm Employment Change",
	"CPI":      "CPI m/m",
	"CORE CPI": "Core CPI m/m",
	"PPI":      "PPI m/m",
	"FOMC":     "Federal Funds Rate",
	"FED":      "Federal Funds Rate",
	"BOE":      "Official Bank Rate",
	"ECB":      "Main Refinancing Rate",
	"BOJ":      "BOJ Policy Rate",
	"RBA":      "Cash Rate",
	"BOC":      "Overnight Rate",
	"RBNZ":     "Official Cash Rate",
	"SNB":      "SNB Policy Rate",
	"GDP":      "GDP q/q",
	"PMI":      "ISM Manufacturing PMI",
	"RETAIL":   "Core Retail Sales m/m",
	"CLAIMS":   "Unemployment Claims",
	"JOBLESS":  "Unemployment Claims",
	"PCE":      "Core PCE Price Index m/m",
	"ISM":      "ISM Manufacturing PMI",
	"ADP":      "ADP Non-Farm Employment Change",
	"WAGES":    "Average Hourly Earnings m/m",
	"CORE_CPI":    "Core CPI m/m",
	"CB_CONSUMER": "CB Consumer Confidence Index",
	"PRICE_EXP":   "Consumer Price Expectations",
	"HOME_SALES":  "Existing Home Sales",
	"PERMITS":     "Building Permits",
}

// resolveEventAlias resolves a known abbreviation to its full event name.
// Returns the input unchanged if no alias matches.
func resolveEventAlias(query string) string {
	upper := strings.ToUpper(strings.TrimSpace(query))
	if full, ok := eventAliases[upper]; ok {
		return full
	}
	return query
}

// fuzzyMatchEvent finds the first tracked event whose name contains the query (case-insensitive).
// Returns the matched event name, or empty string if no match found.
func fuzzyMatchEvent(query string, events []string) string {
	lower := strings.ToLower(query)
	for _, ev := range events {
		if strings.Contains(strings.ToLower(ev), lower) {
			return ev
		}
	}
	return ""
}

// cbImpact handles "imp:" prefixed callbacks for event impact navigation.
func (h *Handler) cbImpact(ctx context.Context, chatID string, msgID int, userID int64, data string) error {
	if h.impactProvider == nil {
		return nil
	}

	action := strings.TrimPrefix(data, "imp:")

	switch {
	case strings.HasPrefix(action, "cat:"):
		// Show events in category
		category := strings.TrimPrefix(action, "cat:")
		kb := h.kb.ImpactEventMenu(category)
		return h.bot.EditWithKeyboard(ctx, chatID, msgID,
			"📋 <b>EVENT IMPACT DATABASE</b>\n<i>Select an event:</i>",
			kb)

	case action == "back":
		// Back to category menu
		kb := h.kb.ImpactCategoryMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID,
			"📋 <b>EVENT IMPACT DATABASE</b>\n<i>Select a category to view event impacts:</i>\n\n<i>Or type directly:</i> <code>/impact NFP</code>",
			kb)

	case strings.HasPrefix(action, "ev:"):
		// Show impact for specific event
		alias := strings.TrimPrefix(action, "ev:")
		// Resolve alias (need to handle underscores -> spaces for multi-word aliases)
		query := strings.ReplaceAll(alias, "_", " ")
		query = resolveEventAlias(query)

		summaries, err := h.impactProvider.GetEventImpactSummary(ctx, query)
		if err != nil {
			return err
		}

		impactHTML := h.fmt.FormatEventImpact(query, summaries)
		// Edit existing message with impact data + back button
		kb := h.kb.ImpactBackMenu()
		return h.bot.EditWithKeyboard(ctx, chatID, msgID, impactHTML, kb)
	}

	return nil
}
