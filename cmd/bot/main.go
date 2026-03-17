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
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/adapter/storage"
	tgbot "github.com/arkcode369/ff-calendar-bot/internal/adapter/telegram"
	"github.com/arkcode369/ff-calendar-bot/internal/config"
	"github.com/arkcode369/ff-calendar-bot/internal/scheduler"
	aisvc "github.com/arkcode369/ff-calendar-bot/internal/service/ai"
	cotsvc "github.com/arkcode369/ff-calendar-bot/internal/service/cot"
	newssvc "github.com/arkcode369/ff-calendar-bot/internal/service/news"
)

const banner = `
╔══════════════════════════════════════════════════╗
║     Institutional Positioning (COT) • Macro Intel ║
║     Built for institutional-grade macro intel     ║
╚══════════════════════════════════════════════════╝`

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	fmt.Println(banner)
	log.Printf("[MAIN] Starting ARK Community Intelligent v1.0 (Go %s, %s/%s)",
		runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// -----------------------------------------------------------------------
	// 1. Configuration
	// -----------------------------------------------------------------------
	cfg := config.MustLoad()
	log.Printf("[MAIN] Config: %s", cfg)

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
		log.Fatalf("[MAIN] Failed to open storage: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("[MAIN] Storage close error: %v", err)
		}
	}()

	eventRepo := storage.NewEventRepo(db)
	cotRepo := storage.NewCOTRepo(db)
	prefsRepo := storage.NewPrefsRepo(db)
	newsRepo := storage.NewNewsRepo(db)

	log.Println("[MAIN] Storage layer initialized")
	logStorageSize(db)

	// -----------------------------------------------------------------------
	// 4. Telegram bot
	// -----------------------------------------------------------------------
	bot := tgbot.NewBot(cfg.BotToken, cfg.ChatID)

	log.Println("[MAIN] Telegram bot created")

	// -----------------------------------------------------------------------
	// 5. AI layer (optional — graceful degradation)
	// -----------------------------------------------------------------------
	var aiAnalyzer *aisvc.Interpreter

	if cfg.HasGemini() {
		gemini, err := aisvc.NewGeminiClient(ctx, cfg.GeminiAPIKey)
		if err != nil {
			log.Printf("[MAIN] WARNING: Gemini init failed, AI features disabled: %v", err)
		} else {
			aiAnalyzer = aisvc.NewInterpreter(gemini, eventRepo, cotRepo)
			log.Println("[MAIN] Gemini AI initialized")
		}
	} else {
		log.Println("[MAIN] No GEMINI_API_KEY — AI features disabled (template fallback active)")
	}

	// -----------------------------------------------------------------------
	// 6. Service layer
	// -----------------------------------------------------------------------

	// COT services
	cotFetcher := cotsvc.NewFetcher()
	cotAnalyzer := cotsvc.NewAnalyzer(cotRepo, cotFetcher)

	// News services (uses MQL5 Economic Calendar API — no API key required)
	newsFetcher := newssvc.NewMQL5Fetcher()
	log.Println("[MAIN] Service layer initialized")

	// -----------------------------------------------------------------------
	// 7. Telegram handler (registers commands on bot)
	// -----------------------------------------------------------------------
	_ = tgbot.NewHandler(
		bot,
		eventRepo,
		cotRepo,
		prefsRepo,
		newsRepo,
		newsFetcher,
		aiAnalyzer, // nil-safe: handler checks IsAvailable()
	)

	log.Println("[MAIN] Telegram handler registered")

	// -----------------------------------------------------------------------
	// 8. Background scheduler
	// -----------------------------------------------------------------------
	sched := scheduler.New(&scheduler.Deps{
		COTAnalyzer: cotAnalyzer,
		AIAnalyzer:  aiAnalyzer,
		Bot:         bot,
		COTRepo:     cotRepo,
		PrefsRepo:   prefsRepo,
		ChatID:      cfg.ChatID,
	})

	sched.Start(ctx, &scheduler.Intervals{
		COTFetch: cfg.COTFetchInterval,
	})

	// News Background Scheduler (always starts — uses MQL5 Economic Calendar)
	newsSched := newssvc.NewScheduler(newsRepo, newsFetcher, aiAnalyzer, bot, prefsRepo)
	newsSched.Start(ctx)
	log.Println("[MAIN] News Background scheduler started")

	log.Println("[MAIN] Background schedulers started")

	// -----------------------------------------------------------------------
	// 9. Initial data load (non-blocking)
	// -----------------------------------------------------------------------
	go func() {
		initCtx, initCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer initCancel()

		log.Println("[MAIN] Running initial data load...")

		// Fetch and sync COT history (this pulls 52 weeks for all contracts)
		log.Println("[MAIN] Syncing COT history (this may take a moment)...")
		if err := cotAnalyzer.SyncHistory(initCtx); err != nil {
			log.Printf("[MAIN] COT history sync failed: %v", err)
		} else {
			log.Println("[MAIN] COT history sync complete")
		}

		// Send startup notification
		startupMsg := fmt.Sprintf(
			"🦅 <b>ARK Intelligence Online</b>\n"+
				"<i>Systems synchronized</i>\n\n"+
				"<code>AI Engine :</code> %s\n"+
				"<code>Calendar  :</code> MQL5 Economic Calendar\n"+
				"<code>COT Data  :</code> CFTC Socrata\n\n"+
				"Type /help for commands",
			aiStatus(aiAnalyzer),
		)
		if _, err := bot.SendHTML(initCtx, cfg.ChatID, startupMsg); err != nil {
			log.Printf("[MAIN] Failed to send startup notification: %v", err)
		}
	}()

	// -----------------------------------------------------------------------
	// 10. Signal handling & graceful shutdown
	// -----------------------------------------------------------------------
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start polling in a goroutine
	pollDone := make(chan struct{})
	go func() {
		defer close(pollDone)
		log.Println("[MAIN] Starting Telegram long-polling...")
		bot.StartPolling(ctx)
		log.Println("[MAIN] Polling stopped")
	}()

	// Block until signal
	sig := <-sigCh
	log.Printf("[MAIN] Received signal: %v — initiating graceful shutdown", sig)

	// Phase 1: Cancel context (stops polling + schedulers)
	cancel()

	// Phase 2: Wait for polling to drain (max 10s)
	select {
	case <-pollDone:
		log.Println("[MAIN] Polling drained cleanly")
	case <-time.After(10 * time.Second):
		log.Println("[MAIN] WARNING: Polling drain timed out after 10s")
	}

	// Phase 3: Stop scheduler
	sched.Stop()
	log.Println("[MAIN] Scheduler stopped")

	// Phase 4: Close storage (handled by defer)
	log.Println("[MAIN] Shutdown complete. Goodbye.")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// aiStatus returns a human-readable AI status string.
func aiStatus(ai *aisvc.Interpreter) string {
	if ai != nil && ai.IsAvailable() {
		return "Active"
	}
	return "Offline"
}

// logStorageSize logs the current database size.
func logStorageSize(db *storage.DB) {
	lsm, vlog := db.Size()
	total := lsm + vlog
	if total > 1<<20 {
		log.Printf("[MAIN] Storage size: %.1f MB (LSM=%.1f MB, VLog=%.1f MB)",
			float64(total)/(1<<20), float64(lsm)/(1<<20), float64(vlog)/(1<<20))
	} else {
		log.Printf("[MAIN] Storage size: %d KB (LSM=%d KB, VLog=%d KB)",
			total>>10, lsm>>10, vlog>>10)
	}
}
