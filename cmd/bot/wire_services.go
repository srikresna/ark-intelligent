// Package main provides service layer wiring for dependency injection.
// This file extracts service initialization from main.go per TECH-012 ADR.
// It centralizes AI services, core business services, and alpha layer initialization.
package main

import (
	"context"
	"fmt"

	tgbot "github.com/arkcode369/ark-intelligent/internal/adapter/telegram"
	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	cotsvc "github.com/arkcode369/ark-intelligent/internal/service/cot"
	factorsvc "github.com/arkcode369/ark-intelligent/internal/service/factors"
	bybitpkg "github.com/arkcode369/ark-intelligent/internal/service/marketdata/bybit"
	microsvc "github.com/arkcode369/ark-intelligent/internal/service/microstructure"
	newssvc "github.com/arkcode369/ark-intelligent/internal/service/news"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	strategysvc "github.com/arkcode369/ark-intelligent/internal/service/strategy"
)

// ServiceDeps holds all service layer dependencies initialized by InitializeServices.
// This struct centralizes service access for clean DI.
type ServiceDeps struct {
	// AI Services (may be nil if not configured)
	AIAnalyzer     ports.AIAnalyzer
	CachedAI       *aisvc.CachedInterpreter
	ClaudeAnalyzer *aisvc.ClaudeAnalyzer
	ChatService    *aisvc.ChatService
	ContextBuilder *aisvc.ContextBuilder

	// Core Services
	COTFetcher      *cotsvc.Fetcher
	COTAnalyzer     *cotsvc.Analyzer
	NewsFetcher     *newssvc.MQL5Fetcher
	PriceFetcher    *pricesvc.Fetcher
	SignalEvaluator *backtestsvc.Evaluator

	// Alpha Services (may be nil if not configured)
	AlphaServices *tgbot.AlphaServices
}

// AIServiceConfig holds configuration for AI service initialization.
type AIServiceConfig struct {
	GeminiAPIKey         string
	GeminiModel          string
	AIMaxRPM             int
	AIMaxDaily           int
	ClaudeEndpoint       string
	ClaudeTimeout        int
	ClaudeModel          string
	ClaudeMaxTokens      int
	ClaudeThinkingBudget int
}

// PriceServiceConfig holds configuration for price service initialization.
type PriceServiceConfig struct {
	TwelveDataAPIKeys   []string
	AlphaVantageAPIKeys []string
	CoinGeckoAPIKey     string
}

// AlphaServiceConfig holds configuration for alpha layer initialization.
type AlphaServiceConfig struct {
	EnableFactorEngine        bool
	EnableBybitMicrostructure bool
	BybitAPIKey               string
	BybitAPISecret            string
	BybitRestBase             string
}

// ServiceConfig holds all configuration parameters for service initialization.
type ServiceConfig struct {
	AI    AIServiceConfig
	Price PriceServiceConfig
	Alpha AlphaServiceConfig
}

// ServiceInitOptions holds optional parameters for service initialization.
type ServiceInitOptions struct {
	OwnerNotifyFunc func(ctx context.Context, html string)
	OwnerID         int64
}

// InitializeServices sets up the entire service layer: AI services, core business
// services (COT, News, Price), and backtest evaluator. Returns ServiceDeps with
// initialized handles. Some services may be nil if configuration is missing
// (graceful degradation).
//
// This is Step 3 of TECH-012 DI refactor — extracted from main.go sections 5-6.
func InitializeServices(
	ctx context.Context,
	cfg ServiceConfig,
	storageDeps *StorageDeps,
	opts *ServiceInitOptions,
) (*ServiceDeps, error) {
	deps := &ServiceDeps{}

	// -----------------------------------------------------------------------
	// AI Layer (optional — graceful degradation)
	// -----------------------------------------------------------------------
	if cfg.AI.GeminiAPIKey != "" {
		gemini, err := aisvc.NewGeminiClient(ctx, cfg.AI.GeminiAPIKey, cfg.AI.GeminiModel, cfg.AI.AIMaxRPM, cfg.AI.AIMaxDaily)
		if err != nil {
			log.Warn().Err(err).Msg("Gemini init failed, AI features disabled")
		} else {
			rawAI := aisvc.NewInterpreter(gemini, storageDeps.EventRepo, storageDeps.COTRepo)
			deps.CachedAI = aisvc.NewCachedInterpreter(rawAI, storageDeps.CacheRepo)
			deps.AIAnalyzer = deps.CachedAI
			log.Info().Msg("Gemini AI initialized (with cache layer)")
		}
	} else {
		log.Info().Msg("No GEMINI_API_KEY — AI features disabled (template fallback active)")
	}

	// -----------------------------------------------------------------------
	// Claude chatbot layer (optional — graceful degradation)
	// -----------------------------------------------------------------------
	var geminiForFallback *aisvc.GeminiClient
	var chatService *aisvc.ChatService
	var claudeAnalyzer *aisvc.ClaudeAnalyzer
	var contextBuilder *aisvc.ContextBuilder

	if cfg.AI.ClaudeEndpoint != "" {
		claudeClient := aisvc.NewClaudeClient(cfg.AI.ClaudeEndpoint, cfg.AI.ClaudeTimeout, cfg.AI.ClaudeMaxTokens)
		if cfg.AI.ClaudeModel != "" {
			claudeClient.SetModel(cfg.AI.ClaudeModel)
		}
		if cfg.AI.ClaudeThinkingBudget > 0 {
			claudeClient.SetThinkingBudget(cfg.AI.ClaudeThinkingBudget)
		} else {
			claudeClient.SetThinkingBudget(0) // explicitly disable
		}

		// ClaudeAnalyzer: AIAnalyzer implementation for /outlook when user prefers Claude.
		claudeAnalyzer = aisvc.NewClaudeAnalyzer(claudeClient, storageDeps.EventRepo, storageDeps.COTRepo)
		log.Info().Str("endpoint", cfg.AI.ClaudeEndpoint).Msg("ClaudeAnalyzer initialized for /outlook")

		// Memory tool: per-user file-based memory persisted in BadgerDB
		memoryStore := aisvc.NewMemoryStore(storageDeps.MemoryRepo)
		toolExecutor := aisvc.NewMemoryToolExecutor(memoryStore)
		claudeClient.SetToolExecutor(toolExecutor)

		toolConfig := aisvc.NewToolConfig()
		contextBuilder = aisvc.NewContextBuilder(storageDeps.COTRepo, storageDeps.NewsRepo, storageDeps.PriceRepo)

		// Reuse existing Gemini client as fallback (if available)
		if cfg.AI.GeminiAPIKey != "" {
			var err error
			geminiForFallback, err = aisvc.NewGeminiClient(ctx, cfg.AI.GeminiAPIKey, cfg.AI.GeminiModel, cfg.AI.AIMaxRPM, cfg.AI.AIMaxDaily)
			if err != nil {
				log.Warn().Err(err).Msg("Gemini fallback init failed — Claude-only mode")
				geminiForFallback = nil
			}
		}

		chatService = aisvc.NewChatService(claudeClient, geminiForFallback, storageDeps.ConversationRepo, contextBuilder, toolConfig)

		// Wire owner notification for AI failure alerts
		if opts != nil && opts.OwnerNotifyFunc != nil {
			chatService.SetOwnerNotify(opts.OwnerNotifyFunc)
		}

		log.Info().Str("endpoint", cfg.AI.ClaudeEndpoint).Msg("Claude chatbot initialized (with memory tool)")
	} else {
		log.Info().Msg("No CLAUDE_ENDPOINT — chatbot mode disabled")
	}

	deps.ClaudeAnalyzer = claudeAnalyzer
	deps.ChatService = chatService
	deps.ContextBuilder = contextBuilder

	// -----------------------------------------------------------------------
	// Core Service Layer
	// -----------------------------------------------------------------------

	// COT services
	deps.COTFetcher = cotsvc.NewFetcher()
	deps.COTAnalyzer = cotsvc.NewAnalyzer(storageDeps.COTRepo, deps.COTFetcher)

	// News services (uses MQL5 Economic Calendar API — no API key required)
	deps.NewsFetcher = newssvc.NewMQL5Fetcher()

	// Price fetcher (3-layer resilience: TwelveData → AlphaVantage → Yahoo + CoinGecko)
	deps.PriceFetcher = pricesvc.NewFetcher(cfg.Price.TwelveDataAPIKeys, cfg.Price.AlphaVantageAPIKeys)
	if cfg.Price.CoinGeckoAPIKey != "" {
		deps.PriceFetcher.SetCoinGeckoKey(cfg.Price.CoinGeckoAPIKey)
		log.Info().Msg("CoinGecko API key configured for TOTAL3 market cap data")
	}

	// Backtest evaluator
	deps.SignalEvaluator = backtestsvc.NewEvaluator(storageDeps.SignalRepo, storageDeps.PriceRepo, storageDeps.DailyPriceRepo)

	log.Info().Msg("Service layer initialized")

	// -----------------------------------------------------------------------
	// Alpha Layer (optional — gracefully disabled if not configured)
	// -----------------------------------------------------------------------
	if cfg.Alpha.EnableFactorEngine {
		profileSvc := factorsvc.NewProfileService(storageDeps.DailyPriceRepo, storageDeps.COTRepo)
		factorEng := factorsvc.NewEngine(factorsvc.DefaultWeights())
		stratEng := strategysvc.NewEngine()

		as := &tgbot.AlphaServices{
			FactorEngine:   factorEng,
			StrategyEngine: stratEng,
			ProfileBuilder: profileSvc,
		}
		// Microstructure: enable only if Bybit is configured
		if cfg.Alpha.EnableBybitMicrostructure {
			bybitCli := bybitpkg.NewClient(cfg.Alpha.BybitAPIKey, cfg.Alpha.BybitAPISecret, cfg.Alpha.BybitRestBase)
			as.MicroEngine = microsvc.NewEngine(bybitCli)
			log.Info().Msg("Bybit microstructure engine initialized")
		}
		deps.AlphaServices = as
		log.Info().Msg("Factor + Strategy engines initialized")
	} else {
		log.Info().Msg("Factor Engine disabled (ENABLE_FACTOR_ENGINE=false)")
	}

	return deps, nil
}

// BuildServiceConfig creates a ServiceConfig from the main config object.
// This helper bridges the gap between the config package and service initialization.
func BuildServiceConfig(cfg *config.Config) ServiceConfig {
	return ServiceConfig{
		AI: AIServiceConfig{
			GeminiAPIKey:         cfg.GeminiAPIKey,
			GeminiModel:          cfg.GeminiModel,
			AIMaxRPM:             cfg.AIMaxRPM,
			AIMaxDaily:           cfg.AIMaxDaily,
			ClaudeEndpoint:       cfg.ClaudeEndpoint,
			ClaudeTimeout:        cfg.ClaudeTimeout,
			ClaudeModel:          cfg.ClaudeModel,
			ClaudeMaxTokens:      cfg.ClaudeMaxTokens,
			ClaudeThinkingBudget: cfg.ClaudeThinkingBudget,
		},
		Price: PriceServiceConfig{
			TwelveDataAPIKeys:   cfg.TwelveDataAPIKeys,
			AlphaVantageAPIKeys: cfg.AlphaVantageAPIKeys,
			CoinGeckoAPIKey:     cfg.CoinGeckoAPIKey,
		},
		Alpha: AlphaServiceConfig{
			EnableFactorEngine:        cfg.EnableFactorEngine,
			EnableBybitMicrostructure: cfg.EnableBybitMicrostructure,
			BybitAPIKey:               cfg.BybitAPIKey,
			BybitAPISecret:            cfg.BybitAPISecret,
			BybitRestBase:             cfg.BybitRestBase,
		},
	}
}
