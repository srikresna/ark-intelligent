// Package main provides Telegram layer wiring for dependency injection.
// This file extracts Telegram bot and handler initialization from main.go per TECH-012 ADR.
// It centralizes bot creation, middleware setup, and handler registration with all services.
package main

import (
	"context"
	"fmt"

	tgbot "github.com/arkcode369/ark-intelligent/internal/adapter/telegram"
	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	ictsvc "github.com/arkcode369/ark-intelligent/internal/service/ict"
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
	"github.com/arkcode369/ark-intelligent/internal/service/gex"
	"github.com/arkcode369/ark-intelligent/internal/service/elliott"
	"github.com/arkcode369/ark-intelligent/internal/service/wyckoff"
)

// TelegramDeps holds all Telegram layer dependencies initialized by InitializeTelegram.
// This struct centralizes bot, middleware, and handler access for clean DI.
type TelegramDeps struct {
	Bot        *tgbot.Bot
	Middleware *tgbot.Middleware
	Handler    *tgbot.Handler
}

// TelegramConfig holds configuration parameters for Telegram initialization.
type TelegramConfig struct {
	BotToken   string
	ChatID     string
	Changelog  string // Embedded CHANGELOG.md content
}

// InitializeTelegram sets up the entire Telegram layer: bot creation, middleware,
// handler with all service registrations, and command wiring.
//
// This is Step 4 of TECH-012 DI refactor — extracted from main.go sections 4 and 8.
func InitializeTelegram(
	cfg TelegramConfig,
	storageDeps *StorageDeps,
	serviceDeps *ServiceDeps,
	newsSched tgbot.SurpriseProvider, // May be nil
) (*TelegramDeps, error) {
	// -----------------------------------------------------------------------
	// 1. Telegram bot
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
	authMiddleware := tgbot.NewMiddleware(storageDeps.UserRepo, bot.OwnerID())
	bot.SetMiddleware(authMiddleware)

	log.Info().Msg("Telegram bot created (with user management middleware)")

	// -----------------------------------------------------------------------
	// 2. Telegram handler (registers commands on bot)
	// -----------------------------------------------------------------------
	// Handler is wired after newsSched so it can receive the surprise accumulator.
	// newsSched implements SurpriseProvider via GetSurpriseSigma — enables full
	// 3-source conviction scoring (COT + FRED + Calendar) in /rank and /cot detail.
	handler := tgbot.NewHandler(tgbot.HandlerDeps{
		Bot:            bot,
		EventRepo:      storageDeps.EventRepo,
		COTRepo:        storageDeps.COTRepo,
		PrefsRepo:      storageDeps.PrefsRepo,
		NewsRepo:       storageDeps.NewsRepo,
		NewsFetcher:    serviceDeps.NewsFetcher,
		AIAnalyzer:     serviceDeps.AIAnalyzer,
		Changelog:      cfg.Changelog,
		NewsScheduler:  newsSched,
		Middleware:     authMiddleware,
		PriceRepo:      storageDeps.PriceRepo,
		SignalRepo:     storageDeps.SignalRepo,
		ChatService:    serviceDeps.ChatService,
		ClaudeAnalyzer: serviceDeps.ClaudeAnalyzer,
		ImpactProvider: storageDeps.ImpactRepo,
		DailyPriceRepo: storageDeps.DailyPriceRepo,
		IntradayRepo:   storageDeps.IntradayRepo,
	})

	// Wire feedback repo for 👍/👎 reaction buttons on analysis messages (TASK-051)
	handler.WithFeedback(storageDeps.FeedbackRepo)

	// Wire alpha services (Factor + Strategy + Microstructure engines)
	if serviceDeps.AlphaServices != nil {
		handler.WithAlpha(serviceDeps.AlphaServices)
		log.Info().Msg("Alpha commands registered (/xfactors /playbook /heat /rankx /transition /cryptoalpha)")
	}

	// Wire CTA services (Classical Technical Analysis engine)
	{
		taEngine := ta.NewEngine()
		ctaServices := &tgbot.CTAServices{
			TAEngine:       taEngine,
			DailyPriceRepo: storageDeps.DailyPriceRepo,
			IntradayRepo:   storageDeps.IntradayRepo,
			PriceMapping:   domain.DefaultPriceSymbolMappings,
		}
		handler.WithCTA(ctaServices)
		log.Info().Msg("CTA commands registered (/cta)")

		// Wire CTA Backtest services (reuses same TA engine + repos)
		ctabtServices := &tgbot.CTABTServices{
			TAEngine:       taEngine,
			DailyPriceRepo: storageDeps.DailyPriceRepo,
			IntradayRepo:   storageDeps.IntradayRepo,
		}
		handler.WithCTABT(ctabtServices)
		log.Info().Msg("CTA Backtest commands registered (/ctabt)")

		// Wire Quant services (Econometric/Statistical Analysis engine)
		quantServices := &tgbot.QuantServices{
			DailyPriceRepo: storageDeps.DailyPriceRepo,
			IntradayRepo:   storageDeps.IntradayRepo,
			PriceMapping:   domain.DefaultPriceSymbolMappings,
		}
		handler.WithQuant(quantServices)
		log.Info().Msg("Quant commands registered (/quant)")

		// Wire Volume Profile services
		vpServices := tgbot.VPServices{
			DailyPriceRepo: storageDeps.DailyPriceRepo,
			IntradayRepo:   storageDeps.IntradayRepo,
		}
		handler.WithVP(vpServices)
		log.Info().Msg("Volume Profile commands registered (/vp)")

		// Wire ICT/SMC services (Smart Money Concepts analysis engine)
		ictServices := &tgbot.ICTServices{
			Engine:         ictsvc.NewEngine(),
			DailyPriceRepo: storageDeps.DailyPriceRepo,
			IntradayRepo:   storageDeps.IntradayRepo,
		}
		handler.WithICT(ictServices)
		log.Info().Msg("ICT/SMC commands registered (/ict)")

		// Wire SMC services (Smart Money Concepts: BOS/CHOCH + ICT overlay)
		smcServices := &tgbot.SMCServices{
			ICTEngine:      ictsvc.NewEngine(),
			TAEngine:       taEngine,
			DailyPriceRepo: storageDeps.DailyPriceRepo,
			IntradayRepo:   storageDeps.IntradayRepo,
		}
		handler.WithSMC(smcServices)
		log.Info().Msg("SMC commands registered (/smc)")

		// Wire GEX services (Gamma Exposure engine via Deribit public API)
		gexServices := &tgbot.GEXServices{
			Engine: gexsvc.NewEngine(),
		}
		handler.WithGEX(gexServices)
		log.Info().Msg("GEX commands registered (/gex)")

		// Wire Elliott Wave services (automated wave counting and projection)
		elliottServices := tgbot.ElliottServices{
			DailyPriceRepo: storageDeps.DailyPriceRepo,
			IntradayRepo:   storageDeps.IntradayRepo,
			Engine:         elliottsvc.NewEngine(),
		}
		handler.WithElliott(elliottServices)
		log.Info().Msg("Elliott Wave commands registered (/elliott)")

		// Wire Wyckoff services (Wyckoff Method structure detection)
		wyckoffServices := tgbot.WyckoffServices{
			DailyPriceRepo: storageDeps.DailyPriceRepo,
			IntradayRepo:   storageDeps.IntradayRepo,
			WyckoffEngine:  wyckoffsvc.NewEngine(),
		}
		handler.WithWyckoff(wyckoffServices)
		log.Info().Msg("Wyckoff commands registered (/wyckoff)")
	}

	// Register free-text handler for chatbot mode
	if serviceDeps.ChatService != nil {
		bot.SetFreeTextHandler(handler.HandleFreeText)
		log.Info().Msg("Free-text chatbot handler registered")
	}

	log.Info().Msg("Telegram handler registered")

	return &TelegramDeps{
		Bot:        bot,
		Middleware: authMiddleware,
		Handler:    handler,
	}, nil
}

// GetOwnerChatID returns the owner chat ID for scheduler notifications.
// Returns empty string if owner ID is not set.
func (tg *TelegramDeps) GetOwnerChatID() string {
	if tg.Bot == nil {
		return ""
	}
	ownerID := tg.Bot.OwnerID()
	if ownerID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", ownerID)
}
