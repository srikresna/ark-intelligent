// Package main is the entry point for ARK Community Intelligent.
// It wires all dependencies using manual DI (no framework), starts
// background schedulers, and runs the Telegram long-polling loop.
//
// Shutdown is graceful: SIGINT/SIGTERM stops polling, drains in-flight
// handlers (10s deadline), cancels background jobs, flushes storage,
// then exits.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	tgbot "github.com/arkcode369/ark-intelligent/internal/adapter/telegram"
	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/health"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/scheduler"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	cotsvc "github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	factorsvc "github.com/arkcode369/ark-intelligent/internal/service/factors"
	microsvc "github.com/arkcode369/ark-intelligent/internal/service/microstructure"
	newssvc "github.com/arkcode369/ark-intelligent/internal/service/news"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	strategysvc "github.com/arkcode369/ark-intelligent/internal/service/strategy"
	ta "github.com/arkcode369/ark-intelligent/internal/service/ta"
	ictsvc "github.com/arkcode369/ark-intelligent/internal/service/ict"
	gexsvc "github.com/arkcode369/ark-intelligent/internal/service/gex"
	bybitpkg "github.com/arkcode369/ark-intelligent/internal/service/marketdata/bybit"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

//go:embed CHANGELOG.md
var changelogContent string

var log = logger.Component("main")

const banner = `
╔══════════════════════════════════════════════════╗
║     Institutional Positioning (COT) • Macro Intel ║
║     Built for institutional-grade macro intel     ║
╚══════════════════════════════════════════════════╝`

func main() {
	fmt.Println(banner)

	// -----------------------------------------------------------------------
	// 1. Configuration
	// -----------------------------------------------------------------------
	cfg := config.MustLoad()
	logger.Init(cfg.LogLevel)
	// Re-initialize component logger after Init
	log = logger.Component("main")

	log.Info().
		Str("version", "v3.0.0").
		Str("go", runtime.Version()).
		Str("os", runtime.GOOS).
		Str("arch", runtime.GOARCH).
		Msg("Starting ARK Community Intelligent")

	log.Info().Str("config", cfg.String()).Msg("Config loaded")

	// -----------------------------------------------------------------------
	// 2. Root context with cancellation (drives graceful shutdown)
	// -----------------------------------------------------------------------
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// -----------------------------------------------------------------------
	// 3. Storage layer
	// -----------------------------------------------------------------------
	db, err := storage.Open(cfg.DataDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open storage")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("Storage close error")
		}
	}()

	eventRepo := storage.NewEventRepo(db)
	cotRepo := storage.NewCOTRepo(db)
	prefsRepo := storage.NewPrefsRepo(db)
	newsRepo := storage.NewNewsRepo(db)
	cacheRepo := storage.NewCacheRepo(db)
	userRepo := storage.NewUserRepo(db)
	priceRepo := storage.NewPriceRepo(db)
	signalRepo := storage.NewSignalRepo(db)
	impactRepo := storage.NewImpactRepo(db)
	dailyPriceRepo := storage.NewDailyPriceRepo(db)
	intradayRepo := storage.NewIntradayRepo(db)
	fredRepo := storage.NewFREDRepo(db)
	fredPersistence := fred.NewPersistenceService(&fredPersistAdapter{repo: fredRepo})
	fred.SetPostFetchHook(func(ctx context.Context, data *fred.MacroData) {
		if err := fredPersistence.PersistSnapshot(ctx, data); err != nil {
			log.Warn().Err(err).Msg("FRED snapshot persistence failed (non-fatal)")
		}
	})

	log.Info().Msg("Storage layer initialized")
	logStorageSize(db)

	// -----------------------------------------------------------------------
	// 3b. Health check endpoint
	// -----------------------------------------------------------------------
	healthChecker := health.New(func() error {
		// Simple DB liveness check via Size() — if it panics, DB is dead
		db.Size()
		return nil
	})
	healthAddr := config.GetEnvDefault("HEALTH_ADDR", ":8080")
	go healthChecker.Start(ctx, healthAddr)

	// -----------------------------------------------------------------------
	// 4. Telegram bot
	// -----------------------------------------------------------------------
	bot := tgbot.NewBot(cfg.BotToken, cfg.ChatID)

	// Check Python chart rendering dependencies at startup.
	// Log a warning but do not fail — chart commands gracefully degrade to text.
	if err := tgbot.CheckPythonChartDeps(); err != nil {
		log.Warn().Err(err).Msg("Python chart dependencies check failed — chart rendering disabled")
	} else {
		log.Info().Msg("Python chart dependencies OK")
	}

	// User management middleware (tiered access control + quotas)
	authMiddleware := tgbot.NewMiddleware(userRepo, bot.OwnerID())
	bot.SetMiddleware(authMiddleware)

	log.Info().Msg("Telegram bot created (with user management middleware)")

	// -----------------------------------------------------------------------
	// 5. AI layer (optional — graceful degradation)
	// -----------------------------------------------------------------------
	var aiAnalyzer ports.AIAnalyzer
	var cachedAI *aisvc.CachedInterpreter

	if cfg.HasGemini() {
		gemini, err := aisvc.NewGeminiClient(ctx, cfg.GeminiAPIKey, cfg.GeminiModel, cfg.AIMaxRPM, cfg.AIMaxDaily)
		if err != nil {
			log.Warn().Err(err).Msg("Gemini init failed, AI features disabled")
		} else {
			rawAI := aisvc.NewInterpreter(gemini, eventRepo, cotRepo)
			cachedAI = aisvc.NewCachedInterpreter(rawAI, cacheRepo)
			aiAnalyzer = cachedAI
			log.Info().Msg("Gemini AI initialized (with cache layer)")
		}
	} else {
		log.Info().Msg("No GEMINI_API_KEY — AI features disabled (template fallback active)")
	}

	// -----------------------------------------------------------------------
	// 5b. Claude chatbot layer (optional — graceful degradation)
	// -----------------------------------------------------------------------
	var chatService *aisvc.ChatService
	var claudeAnalyzer *aisvc.ClaudeAnalyzer
	var geminiForFallback *aisvc.GeminiClient

	if cfg.HasClaude() {
		claudeClient := aisvc.NewClaudeClient(cfg.ClaudeEndpoint, cfg.ClaudeTimeout, cfg.ClaudeMaxTokens)
		if cfg.ClaudeModel != "" {
			claudeClient.SetModel(cfg.ClaudeModel)
		}
		if cfg.ClaudeThinkingBudget > 0 {
			claudeClient.SetThinkingBudget(cfg.ClaudeThinkingBudget)
		} else {
			claudeClient.SetThinkingBudget(0) // explicitly disable
		}

		// ClaudeAnalyzer: AIAnalyzer implementation for /outlook when user prefers Claude.
		claudeAnalyzer = aisvc.NewClaudeAnalyzer(claudeClient, eventRepo, cotRepo)
		log.Info().Str("endpoint", cfg.ClaudeEndpoint).Msg("ClaudeAnalyzer initialized for /outlook")

		// Memory tool: per-user file-based memory persisted in BadgerDB
		memoryRepo := storage.NewMemoryRepo(db, 30*24*time.Hour) // 30-day TTL
		memoryStore := aisvc.NewMemoryStore(memoryRepo)
		toolExecutor := aisvc.NewMemoryToolExecutor(memoryStore)
		claudeClient.SetToolExecutor(toolExecutor)

		convRepo := storage.NewConversationRepo(db, cfg.ChatHistoryLimit, cfg.ChatHistoryTTL)
		toolConfig := aisvc.NewToolConfig()
		contextBuilder := aisvc.NewContextBuilder(cotRepo, newsRepo, priceRepo)

		// Reuse existing Gemini client as fallback (if available)
		if cfg.HasGemini() {
			// Create a separate Gemini instance for chat fallback
			geminiForFallback, err = aisvc.NewGeminiClient(ctx, cfg.GeminiAPIKey, cfg.GeminiModel, cfg.AIMaxRPM, cfg.AIMaxDaily)
			if err != nil {
				log.Warn().Err(err).Msg("Gemini fallback init failed — Claude-only mode")
				geminiForFallback = nil
			}
		}

		chatService = aisvc.NewChatService(claudeClient, geminiForFallback, convRepo, contextBuilder, toolConfig)

		// Wire owner notification for AI failure alerts
		chatService.SetOwnerNotify(func(ctx context.Context, html string) {
			ownerChatID := ownerChatIDForScheduler(bot.OwnerID())
			if ownerChatID == "" {
				return
			}
			_, _ = bot.SendHTML(ctx, ownerChatID, html)
		})

		log.Info().Str("endpoint", cfg.ClaudeEndpoint).Msg("Claude chatbot initialized (with memory tool)")
	} else {
		log.Info().Msg("No CLAUDE_ENDPOINT — chatbot mode disabled")
	}

	// -----------------------------------------------------------------------
	// 6. Service layer
	// -----------------------------------------------------------------------

	// COT services
	cotFetcher := cotsvc.NewFetcher()
	cotAnalyzer := cotsvc.NewAnalyzer(cotRepo, cotFetcher)

	// News services (uses MQL5 Economic Calendar API — no API key required)
	newsFetcher := newssvc.NewMQL5Fetcher()

	// Price fetcher (3-layer resilience: TwelveData → AlphaVantage → Yahoo + CoinGecko)
	priceFetcher := pricesvc.NewFetcher(cfg.TwelveDataAPIKeys, cfg.AlphaVantageAPIKeys)
	if cfg.CoinGeckoAPIKey != "" {
		priceFetcher.SetCoinGeckoKey(cfg.CoinGeckoAPIKey)
		log.Info().Msg("CoinGecko API key configured for TOTAL3 market cap data")
	}

	// Backtest evaluator
	signalEvaluator := backtestsvc.NewEvaluator(signalRepo, priceRepo, dailyPriceRepo)

	log.Info().Msg("Service layer initialized")

	// -----------------------------------------------------------------------
	// 6b. Alpha layer — Factor Engine, Strategy Engine, Microstructure Engine
	//     All optional: gracefully disabled if not configured.
	// -----------------------------------------------------------------------
	var alphaServices *tgbot.AlphaServices
	if cfg.EnableFactorEngine {
		profileSvc := factorsvc.NewProfileService(dailyPriceRepo, cotRepo)
		factorEng := factorsvc.NewEngine(factorsvc.DefaultWeights())
		stratEng := strategysvc.NewEngine()

		as := &tgbot.AlphaServices{
			FactorEngine:   factorEng,
			StrategyEngine: stratEng,
			ProfileBuilder: profileSvc,
		}
		// Microstructure: enable only if Bybit is configured
		if cfg.EnableBybitMicrostructure {
			bybitCli := bybitpkg.NewClient(cfg.BybitAPIKey, cfg.BybitAPISecret, cfg.BybitRestBase)
			as.MicroEngine = microsvc.NewEngine(bybitCli)
			log.Info().Msg("Bybit microstructure engine initialized")
		}
		alphaServices = as
		log.Info().Msg("Factor + Strategy engines initialized")
	} else {
		log.Info().Msg("Factor Engine disabled (ENABLE_FACTOR_ENGINE=false)")
	}

	// -----------------------------------------------------------------------
	// 7. Background schedulers
	// -----------------------------------------------------------------------
	sched := scheduler.New(&scheduler.Deps{
		COTAnalyzer:        cotAnalyzer,
		AIAnalyzer:         aiAnalyzer,
		Bot:                bot,
		COTRepo:            cotRepo,
		PrefsRepo:          prefsRepo,
		ChatID:             cfg.ChatID,
		CachedAI:           cachedAI,
		DB:                 db,
		PriceRepo:          priceRepo,
		SignalRepo:         signalRepo,
		PriceFetcher:       priceFetcher,
		Evaluator:          signalEvaluator,
		DailyPriceRepo:     dailyPriceRepo,
		IntradayRepo:       intradayRepo,
		ImpactBootstrapper: newImpactBootstrapper(newsFetcher, priceRepo, impactRepo, priceFetcher, cfg.ImpactBootstrapMonths),
		FREDAlertCheck:     authMiddleware.ShouldReceiveFREDAlerts,
		IsBanned:           authMiddleware.IsUserBanned,
		OwnerChatID:        ownerChatIDForScheduler(bot.OwnerID()),
	})

	sched.Start(ctx, &scheduler.Intervals{
		COTFetch:      cfg.COTFetchInterval,
		PriceFetch:    cfg.PriceFetchInterval,
		IntradayFetch: cfg.IntradayFetchInterval,
	})

	// News Background Scheduler (always starts — uses MQL5 Economic Calendar)
	// P1.1: cotRepo injected for Confluence Alert cross-check on actual releases
	// newsSched is created before NewHandler so the surprise accumulator can be injected.
	newsSched := newssvc.NewScheduler(newsRepo, newsFetcher, aiAnalyzer, bot, prefsRepo, cotRepo)

	// Wire AI cache invalidation on significant news releases
	if cachedAI != nil {
		newsSched.SetNewsInvalidateFunc(cachedAI.InvalidateOnNewsUpdate)
	}

	// Wire tier-based alert filtering (Free → USD + High only)
	newsSched.SetAlertFilterFunc(authMiddleware.EffectiveAlertFilters)

	// Wire ban check for all news broadcast loops
	newsSched.SetIsBannedFunc(authMiddleware.IsUserBanned)

	// Wire impact recorder for Event Impact Database
	impactRecorder := newssvc.NewImpactRecorder(impactRepo, priceRepo, priceFetcher)
	newsSched.SetImpactRecorder(impactRecorder)

	newsSched.Start(ctx)

	// Wire surprise accumulator to main scheduler for ConvictionScoreV3 (fixes BUG-5)
	sched.SetSurpriseProvider(newsSched)
	log.Info().Msg("News Background scheduler started")

	// -----------------------------------------------------------------------
	// 8. Telegram handler (registers commands on bot)
	// -----------------------------------------------------------------------
	// Handler is wired after newsSched so it can receive the surprise accumulator.
	// newsSched implements SurpriseProvider via GetSurpriseSigma — enables full
	// 3-source conviction scoring (COT + FRED + Calendar) in /rank and /cot detail.
	handler := tgbot.NewHandler(
		bot,
		eventRepo,
		cotRepo,
		prefsRepo,
		newsRepo,
		newsFetcher,
		aiAnalyzer,     // nil-safe: handler checks IsAvailable()
		changelogContent,
		newsSched,       // SurpriseProvider: weekly per-currency surprise accumulator
		authMiddleware,  // User management middleware
		priceRepo,       // Price data for backtest/context (nil-safe)
		signalRepo,      // Signal persistence for backtest (nil-safe)
		chatService,     // Claude chatbot service (nil-safe)
		claudeAnalyzer,  // Claude AIAnalyzer for /outlook (nil-safe)
		impactRepo,      // Event Impact Database (nil-safe)
		dailyPriceRepo,  // Daily price data for /price command (nil-safe)
		intradayRepo,    // 4H intraday data for /intraday command (nil-safe)
	)

	// Wire alpha services (Factor + Strategy + Microstructure engines)
	if alphaServices != nil {
		handler.WithAlpha(alphaServices)
		log.Info().Msg("Alpha commands registered (/xfactors /playbook /heat /rankx /transition /cryptoalpha)")
	}

	// Wire CTA services (Classical Technical Analysis engine)
	{
		taEngine := ta.NewEngine()
		ctaServices := &tgbot.CTAServices{
			TAEngine:       taEngine,
			DailyPriceRepo: dailyPriceRepo,
			IntradayRepo:   intradayRepo,
			PriceMapping:   domain.DefaultPriceSymbolMappings,
		}
		handler.WithCTA(ctaServices)
		log.Info().Msg("CTA commands registered (/cta)")

		// Wire CTA Backtest services (reuses same TA engine + repos)
		ctabtServices := &tgbot.CTABTServices{
			TAEngine:       taEngine,
			DailyPriceRepo: dailyPriceRepo,
			IntradayRepo:   intradayRepo,
		}
		handler.WithCTABT(ctabtServices)
		log.Info().Msg("CTA Backtest commands registered (/ctabt)")

		// Wire Quant services (Econometric/Statistical Analysis engine)
		quantServices := &tgbot.QuantServices{
			DailyPriceRepo: dailyPriceRepo,
			IntradayRepo:   intradayRepo,
			PriceMapping:   domain.DefaultPriceSymbolMappings,
		}
		handler.WithQuant(quantServices)
		log.Info().Msg("Quant commands registered (/quant)")

		// Wire Volume Profile services
		vpServices := tgbot.VPServices{
			DailyPriceRepo: dailyPriceRepo,
			IntradayRepo:   intradayRepo,
		}
		handler.WithVP(vpServices)
		log.Info().Msg("Volume Profile commands registered (/vp)")

		// Wire ICT/SMC services (Smart Money Concepts analysis engine)
		ictServices := &tgbot.ICTServices{
			Engine:         ictsvc.NewEngine(),
			DailyPriceRepo: dailyPriceRepo,
			IntradayRepo:   intradayRepo,
		}
		handler.WithICT(ictServices)
		log.Info().Msg("ICT/SMC commands registered (/ict)")

		// Wire GEX services (Gamma Exposure engine via Deribit public API)
		gexServices := &tgbot.GEXServices{
			Engine: gexsvc.NewEngine(),
		}
		handler.WithGEX(gexServices)
		log.Info().Msg("GEX commands registered (/gex)")
	}

	// Register free-text handler for chatbot mode
	if chatService != nil {
		bot.SetFreeTextHandler(handler.HandleFreeText)
		log.Info().Msg("Free-text chatbot handler registered")
	}

	log.Info().Msg("Telegram handler registered")

	log.Info().Msg("Background schedulers started")

	// -----------------------------------------------------------------------
	// 9. Initial data load (BLOCKING — must complete before polling)
	// -----------------------------------------------------------------------
	{
		initCtx, initCancel := context.WithTimeout(ctx, 5*time.Minute)

		log.Info().Msg("Running initial data load...")

		// Fetch and sync COT history (this pulls 52 weeks for all contracts)
		log.Info().Msg("Syncing COT history (this may take a moment)...")
		if err := cotAnalyzer.SyncHistory(initCtx); err != nil {
			log.Error().Err(err).Msg("COT history sync failed")
			// Even if full history sync fails, attempt a fresh fetch of latest data
			log.Info().Msg("Attempting fallback: fetch latest COT only...")
			if _, err2 := cotAnalyzer.AnalyzeAll(initCtx); err2 != nil {
				log.Error().Err(err2).Msg("Fallback COT fetch also failed")
			} else {
				log.Info().Msg("Fallback COT fetch succeeded")
			}
		} else {
			log.Info().Msg("COT history sync complete")
		}

		// Gap B — Backfill RegimeAdjustedScore for any stored analyses that predate the feature.
		// Non-fatal: logs warning and continues if FRED data is unavailable.
		if err := cotAnalyzer.BackfillRegimeScores(initCtx); err != nil {
			log.Warn().Err(err).Msg("backfill regime scores (non-fatal)")
		}

		// Price history bootstrap (non-fatal — Yahoo fallback always available)
		log.Info().Msg("Bootstrapping price history...")
		priceRecords, err := priceFetcher.FetchAll(initCtx, cfg.PriceHistoryWeeks)
		if err != nil {
			log.Warn().Err(err).Msg("price history bootstrap failed (non-fatal)")
		} else if len(priceRecords) > 0 {
			if err := priceRepo.SavePrices(initCtx, priceRecords); err != nil {
				log.Warn().Err(err).Msg("save price history failed (non-fatal)")
			} else {
				log.Info().Int("records", len(priceRecords)).Msg("price history bootstrapped")
			}
		}

		// Log existing signal state before purge/bootstrap
		if allSigs, err := signalRepo.GetAllSignals(initCtx); err == nil && len(allSigs) > 0 {
			zeroEntry := 0
			for _, s := range allSigs {
				if s.EntryPrice == 0 {
					zeroEntry++
				}
			}
			log.Info().
				Int("total", len(allSigs)).
				Int("zero_entry_price", zeroEntry).
				Msg("existing signals before purge")
		}

		// Purge any signals with EntryPrice=0 (created by older bootstrap code).
		// This allows re-bootstrap to recreate them with valid entry prices.
		if purged, err := signalRepo.PurgeInvalidSignals(initCtx); err != nil {
			log.Warn().Err(err).Msg("signal purge failed (non-fatal)")
		} else if purged > 0 {
			log.Info().Int("purged", purged).Msg("purged invalid signals (EntryPrice=0)")
		}

		// Backtest bootstrap (replay historical COT signals against prices)
		log.Info().Msg("Running backtest bootstrap...")
		bootstrapper := backtestsvc.NewBootstrapper(cotRepo, priceRepo, signalRepo, signalRepo, dailyPriceRepo)
		if created, err := bootstrapper.Run(initCtx); err != nil {
			log.Warn().Err(err).Msg("backtest bootstrap failed (non-fatal)")
		} else if created > 0 {
			log.Info().Int("signals", created).Msg("backtest signals bootstrapped")
		}

		// Backfill FRED regime labels onto signals using historical FRED data.
		// For each unlabelled signal, fetches the regime that was active at its
		// DetectedAt date rather than stamping the current regime on everything.
		{
			backfilled, bfErr := backtestsvc.BackfillRegimeLabels(initCtx, signalRepo)
			if bfErr != nil {
				log.Warn().Err(bfErr).Msg("regime backfill failed (non-fatal)")
			} else if backfilled > 0 {
				log.Info().Int("backfilled", backfilled).Msg("FRED regime labels backfilled onto signals")
			}
		}

		// Always evaluate pending signals — covers both fresh bootstrap and restarts
		// where signals exist but haven't been evaluated yet.
		log.Info().Msg("Running signal evaluation...")
		evaluated, evalErr := signalEvaluator.EvaluatePending(initCtx)
		if evalErr != nil {
			log.Warn().Err(evalErr).Msg("initial signal evaluation failed (non-fatal)")
		} else {
			log.Info().Int("evaluated", evaluated).Msg("signal evaluation complete")
		}

		// Backfill calibrated confidence on bootstrap signals using Platt scaling.
		// Must run AFTER evaluation so we have outcome data to fit against.
		if calibrated, calErr := backtestsvc.BackfillCalibration(initCtx, signalRepo); calErr != nil {
			log.Warn().Err(calErr).Msg("confidence calibration backfill failed (non-fatal)")
		} else if calibrated > 0 {
			log.Info().Int("calibrated", calibrated).Msg("signal confidence backfill complete")
		}

		initCancel()
		logStorageSize(db)

		// Send startup notification (non-blocking — bot is about to start polling)
		go func() {
			startupMsg := fmt.Sprintf(
				"🦅 <b>ARK Intelligence Online</b>\n"+
					"<i>Systems synchronized</i>\n\n"+
					"<code>AI Engine :</code> %s\n"+
					"<code>Claude    :</code> %s\n"+
					"<code>Calendar  :</code> MQL5 Economic Calendar\n"+
					"<code>COT Data  :</code> CFTC Socrata\n\n"+
					"Type /help for commands • Send any message to chat",
				aiStatus(aiAnalyzer),
				claudeStatus(chatService),
			)
			if _, err := bot.SendHTML(ctx, cfg.ChatID, startupMsg); err != nil {
				log.Error().Err(err).Msg("Failed to send startup notification")
			}
		}()
	}

	// -----------------------------------------------------------------------
	// 10. Signal handling & graceful shutdown
	// -----------------------------------------------------------------------
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start polling in a goroutine
	pollDone := make(chan struct{})
	go func() {
		defer close(pollDone)
		log.Info().Msg("Starting Telegram long-polling...")
		if err := bot.StartPolling(ctx); err != nil {
			log.Error().Err(err).Msg("Polling exited with error")
		}
		log.Info().Msg("Polling stopped")
	}()

	// Block until signal
	sig := <-sigCh
	log.Info().Str("signal", sig.String()).Msg("Received signal — initiating graceful shutdown")

	// Phase 1: Cancel context (stops polling + schedulers)
	cancel()

	// Phase 2: Wait for polling to drain (max 10s)
	select {
	case <-pollDone:
		log.Info().Msg("Polling drained cleanly")
	case <-time.After(10 * time.Second):
		log.Warn().Msg("Polling drain timed out after 10s")
	}

	// Phase 3: Stop scheduler
	sched.Stop()
	log.Info().Msg("Scheduler stopped")

	// Phase 3b: Stop middleware cleanup goroutine
	authMiddleware.Stop()

	// Phase 3c: Stop legacy rate limiter cleanup goroutine
	bot.StopRateLimiter()

	// Phase 4: Close storage (handled by defer)
	log.Info().Msg("Shutdown complete. Goodbye.")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// aiStatus returns a human-readable AI status string.
func aiStatus(ai ports.AIAnalyzer) string {
	if ai != nil && ai.IsAvailable() {
		return "Active"
	}
	return "Offline"
}

// claudeStatus returns a human-readable Claude chatbot status string.
func claudeStatus(cs *aisvc.ChatService) string {
	if cs != nil {
		return "Active (chatbot enabled)"
	}
	return "Offline"
}

// logStorageSize logs the current database size.
func logStorageSize(db *storage.DB) {
	lsm, vlog := db.Size()
	total := lsm + vlog
	if total > 1<<20 {
		log.Info().
			Float64("total_mb", float64(total)/(1<<20)).
			Float64("lsm_mb", float64(lsm)/(1<<20)).
			Float64("vlog_mb", float64(vlog)/(1<<20)).
			Msg("Storage size")
	} else {
		log.Info().
			Int64("total_kb", total>>10).
			Int64("lsm_kb", lsm>>10).
			Int64("vlog_kb", vlog>>10).
			Msg("Storage size")
	}
}

// ownerChatIDForScheduler converts an owner user ID to a chat ID string.
// Returns "" if the owner ID is not set (disabling owner notifications).
func ownerChatIDForScheduler(ownerID int64) string {
	if ownerID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", ownerID)
}

// newImpactBootstrapper creates a configured ImpactBootstrapper with the
// specified number of months to backfill.
func newImpactBootstrapper(
	fetcher *newssvc.MQL5Fetcher,
	priceRepo ports.PriceRepository,
	impactRepo *storage.ImpactRepo,
	priceFetcher ports.PriceFetcher,
	months int,
) *newssvc.ImpactBootstrapper {
	ib := newssvc.NewImpactBootstrapper(fetcher, priceRepo, impactRepo, priceFetcher)
	ib.SetMonths(months)
	return ib
}

// fredPersistAdapter adapts storage.FREDRepo (ports.FREDRepository) to the
// fred.FREDPersister interface, bridging the ports ↔ fred type boundary.
type fredPersistAdapter struct {
	repo *storage.FREDRepo
}

func (a *fredPersistAdapter) SaveSnapshots(ctx context.Context, obs []fred.FREDObservation) error {
	portObs := make([]ports.FREDObservation, len(obs))
	for i, o := range obs {
		portObs[i] = ports.FREDObservation{SeriesID: o.SeriesID, Date: o.Date, Value: o.Value}
	}
	return a.repo.SaveSnapshots(ctx, portObs)
}