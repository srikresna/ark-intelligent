package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	regimesvc "github.com/arkcode369/ark-intelligent/internal/service/regime"
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

	// feedbackRepo stores thumbs-up/down feedback on AI analyses.
	// May be nil — feedback buttons disabled if not wired.
	feedbackRepo *storage.FeedbackRepo

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
	// deepLinks caches command intents from deep link parameters (t.me/bot?start=cmd_cot_EUR).
	// Intent is auto-executed after onboarding completes. TTL 10 minutes.
	deepLinks *deepLinkCache

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

	// regimeEngine computes the unified market regime overlay.
	// May be nil — overlay headers silently omitted if not configured.
	regimeEngine *regimesvc.OverlayEngine

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
	// wyckoffCache stores last Wyckoff analysis state per chat for callback navigation.
	wyckoffCache *wyckoffStateCache

	// elliott holds optional Elliott Wave engine services.
	// May be nil — /elliott command disabled if not configured.
	elliott *ElliottServices

	// adminConfirm stores pending admin action confirmations with TTL.
	adminConfirm *adminConfirmStore

	// orderFlow holds optional price repos for the /orderflow command.
	// May be nil — /orderflow command disabled if not configured.
	orderFlow *OrderFlowServices

	// regimeProvider exposes regime state data for the /regime command.
	// May be nil — regime feature disabled if scheduler not wired.
	regimeProvider RegimeAlertProvider
}

// HandlerDeps holds all dependencies needed by the Handler.
type HandlerDeps struct {
	Bot            *Bot
	EventRepo      ports.EventRepository
	COTRepo        ports.COTRepository
	PrefsRepo      ports.PrefsRepository
	NewsRepo       ports.NewsRepository
	NewsFetcher    ports.NewsFetcher
	AIAnalyzer     ports.AIAnalyzer
	Changelog      string
	NewsScheduler  SurpriseProvider
	Middleware     *Middleware
	PriceRepo      ports.PriceRepository
	SignalRepo     ports.SignalRepository
	ChatService    *aisvc.ChatService
	ClaudeAnalyzer *aisvc.ClaudeAnalyzer
	ImpactProvider ImpactProvider
	DailyPriceRepo pricesvc.DailyPriceStore
	IntradayRepo   pricesvc.IntradayStore
}

// NewHandler creates a handler and registers all commands on the bot.
// newsScheduler, chatService, and claudeAnalyzer may be nil; all callers guard with nil checks before use.
func NewHandler(d HandlerDeps) *Handler {
	h := &Handler{
		bot:            d.Bot,
		fmt:            NewFormatter(),
		kb:             NewKeyboardBuilder(),
		eventRepo:      d.EventRepo,
		cotRepo:        d.COTRepo,
		prefsRepo:      d.PrefsRepo,
		newsRepo:       d.NewsRepo,
		newsFetcher:    d.NewsFetcher,
		aiAnalyzer:     d.AIAnalyzer,
		changelog:      d.Changelog,
		newsScheduler:  d.NewsScheduler,
		aiCooldown:     make(map[int64]time.Time),
		chatCooldown:   make(map[int64]time.Time),
		deepLinks:      newDeepLinkCache(),
		middleware:     d.Middleware,
		priceRepo:      d.PriceRepo,
		signalRepo:     d.SignalRepo,
		chatService:    d.ChatService,
		claudeAnalyzer: d.ClaudeAnalyzer,
		impactProvider: d.ImpactProvider,
		dailyPriceRepo: d.DailyPriceRepo,
		intradayRepo:   d.IntradayRepo,
		adminConfirm:   newAdminConfirmStore(),
	}

	// Register all commands
	d.Bot.RegisterCommand("/start", h.cmdStart)
	d.Bot.RegisterCommand("/help", h.cmdHelp)
	d.Bot.RegisterCommand("/onboarding", h.cmdOnboarding) // TASK-001-EXT: Restart onboarding
	d.Bot.RegisterCommand("/settings", h.cmdSettings)
	d.Bot.RegisterCommand("/status", h.cmdStatus)
	d.Bot.RegisterCommand("/cot", h.cmdCOT)
	d.Bot.RegisterCommand("/outlook", h.cmdOutlook)
	d.Bot.RegisterCommand("/calendar", h.cmdCalendar)
	d.Bot.RegisterCommand("/rank", h.cmdRank)
	d.Bot.RegisterCommand("/macro", h.cmdMacro)
	d.Bot.RegisterCommand("/ecb", h.cmdECB)           // ECB monetary policy dashboard (SDW)
	d.Bot.RegisterCommand("/leading", h.cmdLeading)    // OECD Composite Leading Indicators
	d.Bot.RegisterCommand("/eurostat", h.cmdEurostat)  // EU economy dashboard (Eurostat)
	d.Bot.RegisterCommand("/eu", h.cmdEurostat)        // EU economy alias
	d.Bot.RegisterCommand("/snb", h.cmdSNB)           // SNB balance sheet / FX intervention proxy
	d.Bot.RegisterCommand("/swaps", h.cmdSwaps)        // DTCC FX swap institutional flows
	d.Bot.RegisterCommand("/tedge", h.cmdTEdge)        // TradingEconomics global macro dashboard
	d.Bot.RegisterCommand("/globalm", h.cmdTEdge)      // alias for /tedge
	d.Bot.RegisterCommand("/bias", h.cmdBias)
	d.Bot.RegisterCommand("/backtest", h.cmdBacktest)
	d.Bot.RegisterCommand("/accuracy", h.cmdAccuracy)
	d.Bot.RegisterCommand("/report", h.cmdReport)
	d.Bot.RegisterCommand("/impact", h.cmdImpact)
	d.Bot.RegisterCommand("/sentiment", h.cmdSentiment)
	d.Bot.RegisterCommand("/vix", h.cmdVix)                // CBOE volatility index dashboard (VIX + vol suite)
	d.Bot.RegisterCommand("/seasonal", h.cmdSeasonal)
	d.Bot.RegisterCommand("/price", h.cmdPrice)             // Daily price context
	d.Bot.RegisterCommand("/levels", h.cmdLevels)           // Support/resistance levels + position sizing
	d.Bot.RegisterCommand("/intermarket", h.cmdIntermarket) // Intermarket correlation signals
	d.Bot.RegisterCommand("/flows", h.cmdFlows)             // Cross-asset flow divergence detection
	d.Bot.RegisterCommand("/treasury", h.cmdTreasury)     // US Treasury auction results
	d.Bot.RegisterCommand("/13f", h.cmdSEC)             // SEC EDGAR 13F institutional holdings
	d.Bot.RegisterCommand("/signal", h.cmdSignal)         // Unified directional signal (COT+CTA+Quant+Sentiment+Seasonal)
	d.Bot.RegisterCommand("/setalert", h.cmdSetAlert)  // Per-pair COT alert management
	d.Bot.RegisterCommand("/onchain", h.cmdOnChain)    // On-chain exchange flow metrics (CoinMetrics)
	d.Bot.RegisterCommand("/defi", h.cmdDeFi)          // DeFi health dashboard (DefiLlama)
	d.Bot.RegisterCommand("/carry", h.cmdCarry)         // Carry trade monitor & unwind detector
	d.Bot.RegisterCommand("/bis", h.cmdBIS)            // BIS Statistics: CB policy rates + credit gaps + REER
	d.Bot.RegisterCommand("/cbrates", h.cmdBIS)        // Central bank policy rates (alias for /bis)
	d.Bot.RegisterCommand("/orderflow", h.cmdOrderFlow)   // Estimated delta & order flow analysis
	d.Bot.RegisterCommand("/market", h.cmdMarket)      // Cross-asset market overview (Finviz via Firecrawl)
	d.Bot.RegisterCommand("/session", h.cmdSession)       // Trading session behavior analysis (London/NY/Tokyo)
	d.Bot.RegisterCommand("/scenario", h.cmdScenario)    // Monte Carlo price scenario generator
	d.Bot.RegisterCommand("/regime", h.cmdRegime)        // Multi-asset regime dashboard (HMM states)

	// Membership & upgrade info
	d.Bot.RegisterCommand("/membership", h.cmdMembership)

	// Chat history management
	d.Bot.RegisterCommand("/clear", h.cmdClearChat)

	// Pinned commands (TASK-078)
	d.Bot.RegisterCommand("/pin", h.cmdPin)
	d.Bot.RegisterCommand("/unpin", h.cmdUnpin)
	d.Bot.RegisterCommand("/pins", h.cmdPins)

	// Admin commands (access enforced inside handlers)
	d.Bot.RegisterCommand("/users", h.cmdUsers)
	d.Bot.RegisterCommand("/setrole", h.cmdSetRole)
	d.Bot.RegisterCommand("/ban", h.cmdBan)
	d.Bot.RegisterCommand("/unban", h.cmdUnban)

	// Short aliases for power users (mobile-friendly)
	d.Bot.RegisterCommand("/c", h.cmdCOT)
	d.Bot.RegisterCommand("/cal", h.cmdCalendar)
	d.Bot.RegisterCommand("/out", h.cmdOutlook)
	d.Bot.RegisterCommand("/m", h.cmdMacro)
	d.Bot.RegisterCommand("/b", h.cmdBias)
	d.Bot.RegisterCommand("/q", h.cmdQuant)
	d.Bot.RegisterCommand("/bt", h.cmdBacktest)
	d.Bot.RegisterCommand("/r", h.cmdRank)
	d.Bot.RegisterCommand("/s", h.cmdSentiment)
	d.Bot.RegisterCommand("/p", h.cmdPrice)
	d.Bot.RegisterCommand("/l", h.cmdLevels)
	d.Bot.RegisterCommand("/history", h.cmdHistory)
	d.Bot.RegisterCommand("/h", h.cmdHistory)
	d.Bot.RegisterCommand("/compare", h.cmdCompare) // COT side-by-side comparison

	// Daily briefing command (TASK-029)
	d.Bot.RegisterCommand("/briefing", h.cmdBriefing)
	d.Bot.RegisterCommand("/br", h.cmdBriefing) // short alias

	// Multi-word command+arg aliases for power users (TASK-203)
	// These combine command + default argument for the most common workflows.
	d.Bot.RegisterCommand("/ce", h.cmdCOT)            // /ce EUR = /cot EUR
	d.Bot.RegisterCommand("/ca", h.cmdCTA)             // /ca EUR = /cta EUR
	d.Bot.RegisterCommand("/qe", h.cmdQuant)           // /qe EUR = /quant EUR
	d.Bot.RegisterCommand("/bta", h.cmdBacktestAll)    // /bta    = /backtest all
	d.Bot.RegisterCommand("/of", h.cmdOutlookFRED)     // /of     = /outlook fred

	// Register callback handlers
	d.Bot.RegisterCallback("cot:", h.cbCOTDetail)
	d.Bot.RegisterCallback("alert:", h.cbAlertToggle)
	d.Bot.RegisterCallback("set:", h.cbSettings)
	d.Bot.RegisterCallback("alertmgr:", h.cbAlertMgr)
	d.Bot.RegisterCallback("cal:filter:", h.cbNewsFilter)
	d.Bot.RegisterCallback("out:", h.cbOutlook)
	d.Bot.RegisterCallback("cal:nav:", h.cbNewsNav)
	d.Bot.RegisterCallback("cmd:", h.cbQuickCommand)
	d.Bot.RegisterCallback("onboard:", h.cbOnboard)
	d.Bot.RegisterCallback("tutorial:", h.cbTutorial) // TASK-001-EXT: Tutorial navigation
	d.Bot.RegisterCallback("macro:", h.cbMacro)
	d.Bot.RegisterCallback("imp:", h.cbImpact)
	d.Bot.RegisterCallback("nav:", h.cbNav)
	d.Bot.RegisterCallback("help:", h.cbHelp)
	d.Bot.RegisterCallback("setalert:", h.cbSetAlert) // Per-pair alert management keyboard
	d.Bot.RegisterCallback("share:", h.cbShare)
	d.Bot.RegisterCallback("adm_cf:", h.cbAdminConfirm)
	d.Bot.RegisterCallback("briefing:", h.cbBriefingRefresh)
	d.Bot.RegisterCallback("hist:", h.cbHistory)

	// Onboarding completion tracking (TASK-204)
	h.registerOnboardingProgress()

	log.Info().Int("commands", 52).Int("callbacks", 12).Msg("registered commands and callback prefixes")
	return h
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
		"EUR":  string(domain.ContractEUR),
		"GBP":  string(domain.ContractGBP),
		"JPY":  string(domain.ContractJPY),
		"AUD":  string(domain.ContractAUD),
		"NZD":  string(domain.ContractNZD),
		"CAD":  string(domain.ContractCAD),
		"CHF":  string(domain.ContractCHF),
		"USD":  string(domain.ContractDXY),
		"GOLD": string(domain.ContractGold),
		"XAU":  string(domain.ContractGold),
		"OIL":  string(domain.ContractOil),
	}

	if code, ok := mapping[strings.ToUpper(currency)]; ok {
		return code
	}
	return currency // Return as-is if not mapped
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

// cmdBacktestAll is a convenience alias: /bta → /backtest all (TASK-203).
func (h *Handler) cmdBacktestAll(ctx context.Context, chatID string, userID int64, args string) error {
	if strings.TrimSpace(args) == "" {
		args = "all"
	}
	return h.cmdBacktest(ctx, chatID, userID, args)
}

// cmdOutlookFRED is a convenience alias: /of → /outlook fred (TASK-203).
func (h *Handler) cmdOutlookFRED(ctx context.Context, chatID string, userID int64, args string) error {
	if strings.TrimSpace(args) == "" {
		args = "fred"
	}
	return h.cmdOutlook(ctx, chatID, userID, args)
}

// ---------------------------------------------------------------------------
// Regime Overlay Engine — optional injection
// ---------------------------------------------------------------------------

// WithRegimeEngine injects the regime overlay engine into the handler.
// If nil is passed, regime overlay headers are silently skipped.
func (h *Handler) WithRegimeEngine(e *regimesvc.OverlayEngine) *Handler {
	h.regimeEngine = e
	return h
}
