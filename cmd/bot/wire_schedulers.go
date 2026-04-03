// Package main provides scheduler layer wiring for dependency injection.
// This file extracts scheduler initialization from main.go per TECH-012 ADR.
// It centralizes main scheduler and news scheduler setup with all wiring.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/scheduler"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	newssvc "github.com/arkcode369/ark-intelligent/internal/service/news"
)

// SchedulerDeps holds all scheduler dependencies initialized by InitializeSchedulers.
// This struct centralizes scheduler access and their interconnections.
type SchedulerDeps struct {
	MainScheduler *scheduler.Scheduler
	NewsScheduler *newssvc.Scheduler
}

// SchedulerConfig holds configuration parameters for scheduler initialization.
type SchedulerConfig struct {
	ChatID             string
	OwnerChatID        string
	COTFetchInterval   time.Duration
	PriceFetchInterval time.Duration
	IntradayInterval   time.Duration
	ImpactBootstrapMonths int
}

// InitializeSchedulers sets up all background schedulers: main scheduler and news scheduler.
// It handles all the complex wiring between schedulers, services, and other components.
//
// This is Step 5 of TECH-012 DI refactor — extracted from main.go section 7.
func InitializeSchedulers(
	ctx context.Context,
	cfg SchedulerConfig,
	storageDeps *StorageDeps,
	serviceDeps *ServiceDeps,
	tgDeps TelegramDeps,
) (*SchedulerDeps, error) {
	// -----------------------------------------------------------------------
	// 1. Main scheduler
	// -----------------------------------------------------------------------
	sched := scheduler.New(&scheduler.Deps{
		COTAnalyzer:        serviceDeps.COTAnalyzer,
		AIAnalyzer:         serviceDeps.AIAnalyzer,
		Bot:                tgDeps.Bot,
		COTRepo:            storageDeps.COTRepo,
		PrefsRepo:          storageDeps.PrefsRepo,
		ChatID:             cfg.ChatID,
		CachedAI:           serviceDeps.CachedAI,
		DB:                 storageDeps.DB,
		PriceRepo:          storageDeps.PriceRepo,
		SignalRepo:         storageDeps.SignalRepo,
		PriceFetcher:       serviceDeps.PriceFetcher,
		Evaluator:          serviceDeps.SignalEvaluator,
		DailyPriceRepo:     storageDeps.DailyPriceRepo,
		IntradayRepo:       storageDeps.IntradayRepo,
		ImpactBootstrapper: newImpactBootstrapper(serviceDeps.NewsFetcher, storageDeps.PriceRepo, storageDeps.ImpactRepo, serviceDeps.PriceFetcher, cfg.ImpactBootstrapMonths),
		FREDAlertCheck:     tgDeps.Middleware.ShouldReceiveFREDAlerts,
		IsBanned:           tgDeps.Middleware.IsUserBanned,
		OwnerChatID:        cfg.OwnerChatID,
	})

	sched.Start(ctx, &scheduler.Intervals{
		COTFetch:      cfg.COTFetchInterval,
		PriceFetch:    cfg.PriceFetchInterval,
		IntradayFetch: cfg.IntradayInterval,
	})

	// -----------------------------------------------------------------------
	// 2. News Background Scheduler (always starts — uses MQL5 Economic Calendar)
	// -----------------------------------------------------------------------
	// P1.1: cotRepo injected for Confluence Alert cross-check on actual releases
	// newsSched is created before NewHandler so the surprise accumulator can be injected.
	newsSched := newssvc.NewScheduler(storageDeps.NewsRepo, serviceDeps.NewsFetcher, serviceDeps.AIAnalyzer, tgDeps.Bot, storageDeps.PrefsRepo, storageDeps.COTRepo)

	// Wire AI cache invalidation on significant news releases
	if serviceDeps.CachedAI != nil {
		newsSched.SetNewsInvalidateFunc(serviceDeps.CachedAI.InvalidateOnNewsUpdate)
	}

	// Wire tier-based alert filtering (Free → USD + High only)
	newsSched.SetAlertFilterFunc(tgDeps.Middleware.EffectiveAlertFilters)

	// Wire ban check for all news broadcast loops
	newsSched.SetIsBannedFunc(tgDeps.Middleware.IsUserBanned)

	// Wire impact recorder for Event Impact Database
	impactRecorder := newssvc.NewImpactRecorder(storageDeps.ImpactRepo, storageDeps.PriceRepo, serviceDeps.PriceFetcher)
	newsSched.SetImpactRecorder(impactRecorder)

	// TASK-202: Wire alert gate into news scheduler (quiet hours, per-type toggle, daily cap).
	newsSched.SetAlertGateFunc(sched.ShouldDeliverAlert)
	newsSched.SetRecordDeliveryFunc(sched.RecordAlertDelivery)

	newsSched.Start(ctx)

	// Wire Fed speech provider into AI context builder for enriched chatbot prompts.
	// newsSched caches the latest Fed speeches in-memory after each poll; contextBuilder
	// reads them on demand during prompt construction (nil-safe: skipped if not set).
	if serviceDeps.ContextBuilder != nil {
		serviceDeps.ContextBuilder.SetFedSpeechProvider(func(n int) []aisvc.FedSpeechSummary {
			speeches := newsSched.LatestFedSpeeches(n)
			summaries := make([]aisvc.FedSpeechSummary, 0, len(speeches))
			for _, s := range speeches {
				summaries = append(summaries, aisvc.FedSpeechSummary{
					Speaker:     s.Speaker,
					Title:       s.Title,
					PublishedAt: s.PublishedAt.Format("Jan 2"),
				})
			}
			return summaries
		})
		log.Info().Msg("Fed RSS speech provider wired into AI context builder")
	}

	// Wire surprise accumulator to main scheduler for ConvictionScoreV3 (fixes BUG-5)
	sched.SetSurpriseProvider(newsSched)
	log.Info().Msg("News Background scheduler started")

	return &SchedulerDeps{
		MainScheduler: sched,
		NewsScheduler: newsSched,
	}, nil
}

// newImpactBootstrapper creates a configured ImpactBootstrapper with the
// specified number of months to backfill.
func newImpactBootstrapper(
	fetcher *newssvc.MQL5Fetcher,
	priceRepo ports.PriceRepository,
	impactRepo ports.ImpactRepository,
	priceFetcher ports.PriceFetcher,
	months int,
) *newssvc.ImpactBootstrapper {
	ib := newssvc.NewImpactBootstrapper(fetcher, priceRepo, impactRepo, priceFetcher)
	ib.SetMonths(months)
	return ib
}

// Stop gracefully stops all schedulers.
func (sd *SchedulerDeps) Stop() {
	if sd.MainScheduler != nil {
		sd.MainScheduler.Stop()
	}
}
