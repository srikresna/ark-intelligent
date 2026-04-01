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
	bot.RegisterCommand("/ecb", h.cmdECB)           // ECB monetary policy dashboard (SDW)
	bot.RegisterCommand("/bias", h.cmdBias)
	bot.RegisterCommand("/backtest", h.cmdBacktest)
	bot.RegisterCommand("/accuracy", h.cmdAccuracy)
	bot.RegisterCommand("/report", h.cmdReport)
	bot.RegisterCommand("/impact", h.cmdImpact)
	bot.RegisterCommand("/sentiment", h.cmdSentiment)
	bot.RegisterCommand("/seasonal", h.cmdSeasonal)
	bot.RegisterCommand("/price", h.cmdPrice)             // Daily price context
	bot.RegisterCommand("/levels", h.cmdLevels)           // Support/resistance levels + position sizing
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
