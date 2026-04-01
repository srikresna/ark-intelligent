// Package scheduler orchestrates all background periodic jobs.
// Each job runs on its own ticker, respects context cancellation,
// and logs errors without crashing the process.
//
// Jobs:
//   - Economic Calendar scrape + alert scheduling
//   - FF Revision detection
//   - COT data fetch + analysis
//   - Surprise index recalculation
//   - Confluence score computation
//   - Volatility forecast
//   - Currency ranking
//   - Weekly outlook (Sunday 18:00 WIB)
package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/adapter/storage"
	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	aisvc "github.com/arkcode369/ark-intelligent/internal/service/ai"
	backtestsvc "github.com/arkcode369/ark-intelligent/internal/service/backtest"
	cotsvc "github.com/arkcode369/ark-intelligent/internal/service/cot"
	newssvc "github.com/arkcode369/ark-intelligent/internal/service/news"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/mathutil"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

var log = logger.Component("scheduler")

// ---------------------------------------------------------------------------
// Dependencies & Configuration
// ---------------------------------------------------------------------------

// Deps holds all service dependencies the scheduler needs.
type Deps struct {
	COTAnalyzer *cotsvc.Analyzer
	AIAnalyzer  ports.AIAnalyzer
	Bot         ports.Messenger
	COTRepo     ports.COTRepository
	PrefsRepo   ports.PrefsRepository
	ChatID      string
	CachedAI    *aisvc.CachedInterpreter
	DB          *storage.DB

	// Price & Backtest (optional — nil-safe)
	PriceRepo      ports.PriceRepository
	SignalRepo     ports.SignalRepository
	PriceFetcher   ports.PriceFetcher
	Evaluator      *backtestsvc.Evaluator
	DailyPriceRepo *storage.DailyPriceRepo
	IntradayRepo   *storage.IntradayRepo // 4H intraday data — may be nil

	// ImpactBootstrapper backfills historical event impact data on startup.
	// May be nil (bootstrap is skipped).
	ImpactBootstrapper *newssvc.ImpactBootstrapper

	// FREDAlertCheck is a callback that returns whether a user should receive FRED alerts.
	// Free-tier users are excluded. May be nil (all users receive FRED alerts).
	FREDAlertCheck func(ctx context.Context, userID int64) bool

	// IsBanned checks if a user is banned. May be nil (no ban check).
	IsBanned func(ctx context.Context, userID int64) bool

	// SurpriseProvider retrieves per-currency economic surprise sigma.
	// Used for ConvictionScoreV3 in COT broadcast. May be nil (sigma=0).
	SurpriseProvider interface {
		GetSurpriseSigma(currency string) float64
	}

	// OwnerChatID is the owner's chat ID for debug notifications.
	// If empty, debug notifications are skipped.
	OwnerChatID string
}

// Intervals configures how often each job runs.
type Intervals struct {
	COTFetch      time.Duration // Default: 6h
	PriceFetch    time.Duration // Default: 6h
	IntradayFetch time.Duration // Default: 15m
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

// Scheduler manages all background periodic jobs.
type Scheduler struct {
	deps         *Deps
	stopCh       chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex      // lifecycle mutex (Start/Stop)
	fredMu          sync.Mutex      // protects lastFREDData + lastFREDBroadcast
	lastFREDData    *fred.MacroData // previous FRED snapshot for alert diffing
	lastFREDBroadcast time.Time    // last time FRED alerts were broadcast (dedup guard)

	lastTailRisk     string     // previous VolSuite TailRisk state for SKEW/VIX alert diffing
		cotBroadcastMu   sync.Mutex // protects lastCOTBroadcast
	lastCOTBroadcast time.Time  // last date successfully broadcast to prevent duplicates
}

// New creates a new Scheduler.
func New(deps *Deps) *Scheduler {
	return &Scheduler{
		deps:   deps,
		stopCh: make(chan struct{}),
	}
}

// Start launches all background jobs. Non-blocking.
// SetSurpriseProvider injects the economic surprise accumulator into the scheduler.
// This is called after the news scheduler is initialized (due to init order dependency).
func (s *Scheduler) SetSurpriseProvider(sp interface{ GetSurpriseSigma(string) float64 }) {
	s.deps.SurpriseProvider = sp
}

func (s *Scheduler) Start(ctx context.Context, intervals *Intervals) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		log.Info().Msg("already running")
		return
	}
	s.running = true

	// COT fetch + analysis
	s.startJob(ctx, "cot-fetch", intervals.COTFetch, s.jobCOTFetch)

	// Weekly outlook (check every hour, fires on Sunday 18:00 WIB)
	s.startJob(ctx, "weekly-outlook", 1*time.Hour, s.jobWeeklyOutlook)

	// FRED alert monitor (checks every hour for regime changes)
	s.startJob(ctx, "fred-alerts", 1*time.Hour, s.jobFREDAlerts)

	// Data retention cleanup (runs daily at 03:00 WIB)
	s.startJob(ctx, "retention-cleanup", 1*time.Hour, s.jobRetentionCleanup)

	jobCount := 4

	// Price fetch (if price fetcher is configured)
	if s.deps.PriceFetcher != nil && s.deps.PriceRepo != nil {
		priceFetchInterval := intervals.PriceFetch
		if priceFetchInterval == 0 {
			priceFetchInterval = 6 * time.Hour
		}
		s.startJobWithDelay(ctx, "price-fetch", priceFetchInterval, 30*time.Second, s.jobPriceFetch)
		jobCount++
	}

	// Daily price fetch (if daily price repo and price fetcher are configured)
	if s.deps.PriceFetcher != nil && s.deps.DailyPriceRepo != nil {
		priceFetchInterval := intervals.PriceFetch
		if priceFetchInterval == 0 {
			priceFetchInterval = 6 * time.Hour
		}
		s.startJobWithDelay(ctx, "daily-price-fetch", priceFetchInterval, 45*time.Second, s.jobDailyPriceFetch)
		jobCount++
	}

	// Multi-timeframe intraday fetch (15m base -> aggregate to 30m/1h/4h/6h/12h)
	if s.deps.PriceFetcher != nil && s.deps.IntradayRepo != nil {
		intradayInterval := intervals.IntradayFetch
		if intradayInterval == 0 {
			intradayInterval = 15 * time.Minute
		}
		s.startJobWithDelay(ctx, "intraday-price-fetch", intradayInterval, 90*time.Second, s.jobIntradayPriceFetch)
		jobCount++
	}

	// Signal evaluation (if evaluator is configured, runs every 2 hours)
	if s.deps.Evaluator != nil {
		s.startJobWithDelay(ctx, "signal-eval", 2*time.Hour, 1*time.Minute, s.jobSignalEval)
		jobCount++
	}

	// One-time impact bootstrap (backfills historical event impacts on startup)
	if s.deps.ImpactBootstrapper != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			// Delay to let price data load first.
			select {
			case <-time.After(2 * time.Minute):
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			}
			bootstrapCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
			defer cancel()
			created, err := s.deps.ImpactBootstrapper.Bootstrap(bootstrapCtx)
			if err != nil {
				log.Error().Err(err).Msg("impact bootstrap failed")
			} else if created > 0 {
				log.Info().Int("created", created).Msg("impact bootstrap completed")
			}
		}()
	}

	log.Info().Int("jobs", jobCount).Msg("started background jobs")
}

// Stop signals all jobs to stop and waits for them to finish. Safe to call multiple times.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	log.Info().Msg("stopping all jobs")
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
	s.running = false
	log.Info().Msg("all jobs stopped")
}

// ---------------------------------------------------------------------------
// Job Launcher Helpers
// ---------------------------------------------------------------------------

type jobFunc func(ctx context.Context) error

// startJob launches a goroutine that runs fn immediately on start, then on every tick.
func (s *Scheduler) startJob(ctx context.Context, name string, interval time.Duration, fn jobFunc) {
	s.startJobWithDelay(ctx, name, interval, 0, fn)
}

// startJobWithDelay launches a goroutine with an initial delay before first tick.
func (s *Scheduler) startJobWithDelay(ctx context.Context, name string, interval, delay time.Duration, fn jobFunc) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Initial delay to stagger jobs
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			}
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Info().Str("job", name).Dur("interval", interval).Dur("delay", delay).Msg("job started")

		// Run immediately on first start (don't wait for first tick)
		s.runJob(ctx, name, fn)

		for {
			select {
			case <-ticker.C:
				s.runJob(ctx, name, fn)
			case <-ctx.Done():
				log.Info().Str("job", name).Msg("context cancelled")
				return
			case <-s.stopCh:
				log.Info().Str("job", name).Msg("stop signal received")
				return
			}
		}
	}()
}

// runJob executes a job with timeout, panic recovery, and logging.
func (s *Scheduler) runJob(ctx context.Context, name string, fn jobFunc) {
	start := time.Now()

	// Per-job timeout: 5 minutes max
	jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			log.Error().Str("job", name).Interface("panic", r).Msg("PANIC in job")
		}
	}()

	if err := fn(jobCtx); err != nil {
		log.Error().Str("job", name).Dur("took", time.Since(start)).Err(err).Msg("job failed")
	} else {
		log.Info().Str("job", name).Dur("took", time.Since(start)).Msg("job completed")
	}
}

// ---------------------------------------------------------------------------
// Job Implementations
// ---------------------------------------------------------------------------

// jobCOTFetch fetches latest COT data from CFTC and runs analysis.
func (s *Scheduler) jobCOTFetch(ctx context.Context) error {
	// 1. Get current latest date before fetch
	oldLatest, _ := s.deps.COTRepo.GetLatestReportDate(ctx)

	// 2. Fetch and analyze
	analyses, err := s.deps.COTAnalyzer.AnalyzeAll(ctx)
	if err != nil {
		return fmt.Errorf("cot fetch+analyze: %w", err)
	}

	// 3. Check for new release
	newLatest, _ := s.deps.COTRepo.GetLatestReportDate(ctx)
	if !newLatest.IsZero() && newLatest.After(oldLatest) {
		log.Info().Str("date", newLatest.Format("2006-01-02")).Msg("new COT data detected")
		s.broadcastCOTRelease(ctx, newLatest, analyses)
	}

	log.Info().Msg("COT data fetched and analyzed")
	// Invalidate AI caches that depend on COT data
	if s.deps.CachedAI != nil {
		s.deps.CachedAI.InvalidateOnCOTUpdate(ctx)
	}
	return nil
}

// broadcastCOTRelease sends a notification to all active users when new data is available.
func (s *Scheduler) broadcastCOTRelease(ctx context.Context, date time.Time, analyses []domain.COTAnalysis) {
	// Dedup guard: prevent duplicate broadcasts for the same report date.
	s.cotBroadcastMu.Lock()
	if !date.After(s.lastCOTBroadcast) {
		s.cotBroadcastMu.Unlock()
		log.Debug().Str("date", date.Format("2006-01-02")).Msg("COT broadcast skipped — already sent for this date")
		return
	}
	s.lastCOTBroadcast = date
	s.cotBroadcastMu.Unlock()

	activeUsers, err := s.deps.PrefsRepo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get active users for broadcast")
		return
	}

	msg := fmt.Sprintf("\xF0\x9F\x94\x94 <b>NEW COT DATA RELEASED</b>\xF0\x9F\x94\x94\n\nReport Date: <b>%s</b>\n\nData positioning terbaru sudah di-fetch dan dianalisis.\n\nð Tap <b>Lihat Detail</b> untuk melihat insight terbaru.",
		date.Format("Monday, 02 Jan 2006"))

	cotAlertKB := ports.InlineKeyboard{Rows: [][]ports.InlineButton{
		{
			{Text: "ð Lihat Detail", CallbackData: "cmd:cot"},
			{Text: "ð Matikan Alert", CallbackData: "alert:off:cot"},
		},
		{
			{Text: "âï¸ Pengaturan Alert", CallbackData: "set:alerts"},
		},
	}}


	count := 0
	for userID, prefs := range activeUsers {
		if !prefs.COTAlertsEnabled || prefs.ChatID == "" {
			continue
		}
		// Skip banned users
		if s.deps.IsBanned != nil && s.deps.IsBanned(ctx, userID) {
			continue
		}
		if _, err := s.deps.Bot.SendWithKeyboard(ctx, prefs.ChatID, msg, cotAlertKB); err == nil {
			count++
		}
		// Avoid flooding Telegram API
		time.Sleep(config.TelegramFloodDelay)
	}

	log.Info().Int("users", count).Msg("sent COT release alert")

	// Signal detection — alert on strong signals (Strength >= 4)
	historyMap := make(map[string][]domain.COTRecord)
	for _, a := range analyses {
		records, hErr := s.deps.COTRepo.GetHistory(ctx, a.Contract.Code, 8)
		if hErr == nil && len(records) > 0 {
			historyMap[a.Contract.Code] = records
		}
	}
	// Build recalibrated detector with historical win rates + VIX filter
	recalDetector := cotsvc.NewRecalibratedDetector(s.deps.SignalRepo)
	if s.deps.SignalRepo != nil {
		if loadErr := recalDetector.LoadTypeStats(ctx); loadErr != nil {
			log.Warn().Err(loadErr).Msg("Failed to load signal type stats for recalibration — using raw confidence")
		}
	}

	// Set FRED regime for regime-conditional signal filtering
	if md, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && md != nil {
		comp := fred.ComputeComposites(md)
		regime := fred.ClassifyMacroRegime(md, comp)
		if regime.Name != "" {
			recalDetector.SetCurrentRegime(regime.Name)
		}
	}

	// Build VIX/SPX risk context (nil-safe — no adjustment if unavailable)
	var riskCtx *domain.RiskContext
	var priceCtxsSched map[string]*domain.PriceContext
	if s.deps.PriceRepo != nil {
		rcBuilder := pricesvc.NewRiskContextBuilder(s.deps.PriceRepo)
		riskCtx, _ = rcBuilder.Build(ctx) // ignore error — nil means no adjustment
		if riskCtx != nil {
			if mdEnrich, _ := fred.GetCachedOrFetch(ctx); mdEnrich != nil {
				pricesvc.EnrichWithTermStructure(riskCtx, mdEnrich.VIX3M)
			}
		}
		// Build price contexts for ATR volatility multiplier
		ctxBuilder := pricesvc.NewContextBuilder(s.deps.PriceRepo)
		if pcs, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil {
			priceCtxsSched = pcs
		}
	}

	signals := recalDetector.DetectAll(analyses, historyMap, riskCtx, priceCtxsSched)

	var strongSignals []cotsvc.Signal
	for _, sig := range signals {
		if sig.Strength >= config.SignalStrengthAlert {
			strongSignals = append(strongSignals, sig)
		}
	}

	if len(strongSignals) > 0 {
		signalHTML := formatStrongSignalAlert(strongSignals)
		signalAlertKB := ports.InlineKeyboard{Rows: [][]ports.InlineButton{
			{
				{Text: "ð Lihat COT", CallbackData: "cmd:cot"},
				{Text: "ð¯ Lihat Bias", CallbackData: "cmd:bias"},
			},
			{
				{Text: "ð Matikan Signal Alert", CallbackData: "alert:off:signal"},
				{Text: "âï¸ Pengaturan", CallbackData: "set:alerts"},
			},
		}}
		for userID, prefs := range activeUsers {
			if prefs.COTAlertsEnabled && prefs.ChatID != "" {
				if s.deps.IsBanned != nil && s.deps.IsBanned(ctx, userID) {
					continue
				}
				_, _ = s.deps.Bot.SendWithKeyboard(ctx, prefs.ChatID, signalHTML, signalAlertKB)
			}
		}
		log.Info().Int("signals", len(strongSignals)).Msg("sent strong signal alert to active users")
	}

	// Persist signals for backtesting (if repos are configured)
	if s.deps.SignalRepo != nil && s.deps.PriceRepo != nil && len(signals) > 0 {
		// Build quant enrichment: price contexts + carry data for signal enrichment.
		qe := quantEnrichment{priceCtxs: priceCtxsSched}

		// Fetch carry ranking for carry trade adjustment (best-effort).
		carryEngine := fred.NewRateDifferentialEngine()
		if cr, crErr := carryEngine.FetchCarryRanking(ctx); crErr == nil {
			qe.carryRanking = cr
		}

		s.persistSignals(ctx, signals, analyses, qe)
	}

	// Thin market and concentration alerts
	for _, a := range analyses {
		var alerts []string
		if a.ThinMarketAlert {
			alerts = append(alerts, fmt.Sprintf("\xF0\x9F\x9A\xA8 <b>THIN MARKET:</b> %s \xe2\x80\x94 %s", a.Contract.Currency, a.ThinMarketDesc))
		}
		if a.Top4Concentration > 50 {
			direction := "long unwind"
			if a.NetPosition < 0 {
				direction = "short squeeze"
			}
			alerts = append(alerts, fmt.Sprintf("\xe2\x9a\xa0\xef\xb8\x8f <b>CONCENTRATION:</b> %s \xe2\x80\x94 Top 4 traders hold %.0f%% of OI (%s risk)",
				a.Contract.Currency, a.Top4Concentration, direction))
		}

		if len(alerts) > 0 {
			html := "\xF0\x9F\x93\xA1 <b>COT POSITION ALERT</b>\n\n"
			html += strings.Join(alerts, "\n")
			html += "\n\n<i>Use /cot " + a.Contract.Currency + " for details</i>"

			for userID, prefs := range activeUsers {
				if prefs.COTAlertsEnabled && prefs.ChatID != "" {
					if s.deps.IsBanned != nil && s.deps.IsBanned(ctx, userID) {
						continue
					}
					_, _ = s.deps.Bot.SendHTML(ctx, prefs.ChatID, html)
					time.Sleep(config.TelegramFloodDelay)
				}
			}
		}
	}
}

// jobWeeklyOutlook generates and sends the weekly outlook on Sunday evening.
// Fires every hour but only executes on Sunday between 18:00-18:59 WIB.
func (s *Scheduler) jobWeeklyOutlook(ctx context.Context) error {
	now := timeutil.NowWIB()

	// Only fire on Sunday 18:xx WIB
	if now.Weekday() != time.Sunday || now.Hour() != 18 {
		return nil
	}

	// Check if AI is available
	if s.deps.AIAnalyzer == nil || !s.deps.AIAnalyzer.IsAvailable() {
		log.Info().Msg("AI not available, skipping weekly outlook")
		return nil
	}

	// Gather all data for the outlook
	data, err := s.gatherWeeklyData(ctx)
	if err != nil {
		return fmt.Errorf("gather weekly data: %w", err)
	}

	// Generate outlook via AI
	outlook, err := s.deps.AIAnalyzer.GenerateWeeklyOutlook(ctx, data)
	if err != nil {
		return fmt.Errorf("generate outlook: %w", err)
	}

	// Send to chat
	msg := fmt.Sprintf("<b>\xF0\x9F\x93\x8B Weekly Macro Outlook</b>\n\n%s", outlook)
	if _, err := s.deps.Bot.SendHTML(ctx, s.deps.ChatID, msg); err != nil {
		return fmt.Errorf("send outlook: %w", err)
	}

	log.Info().Msg("weekly outlook sent")
	return nil
}

// jobFREDAlerts checks for FRED macro regime changes and broadcasts alerts to subscribed users.
// Runs every hour. Compares the freshly fetched MacroData against the previous snapshot.
func (s *Scheduler) jobFREDAlerts(ctx context.Context) error {
	current, err := fred.GetCachedOrFetch(ctx)
	if err != nil {
		return fmt.Errorf("fred fetch for alerts: %w", err)
	}

	s.fredMu.Lock()
	previous := s.lastFREDData
	s.lastFREDData = current
	s.fredMu.Unlock()

	alerts := fred.CheckAlerts(current, previous)
	if len(alerts) == 0 {
		return nil
	}

	// Dedup guard: prevent duplicate FRED broadcasts within a 10-minute window.
	// (Handles edge case where concurrent job triggers fire before snapshot swap commits.)
	s.fredMu.Lock()
	if time.Since(s.lastFREDBroadcast) < 10*time.Minute {
		s.fredMu.Unlock()
		log.Debug().Msg("FRED broadcast skipped — already sent within dedup window")
		return nil
	}
	s.lastFREDBroadcast = time.Now()
	s.fredMu.Unlock()

	log.Info().Int("alerts", len(alerts)).Msg("FRED alerts detected")

	activeUsers, err := s.deps.PrefsRepo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("get active users for fred alerts: %w", err)
	}

	fredAlertKB := ports.InlineKeyboard{Rows: [][]ports.InlineButton{
		{
			{Text: "ð Lihat Macro", CallbackData: "cmd:macro"},
			{Text: "ð Matikan Alert", CallbackData: "alert:off:fred"},
		},
		{
			{Text: "âï¸ Pengaturan Alert", CallbackData: "set:alerts"},
		},
	}}

	for _, alert := range alerts {
		msg := fred.FormatMacroAlert(alert)
		count := 0
		for userID, prefs := range activeUsers {
			if !prefs.COTAlertsEnabled || prefs.ChatID == "" {
				continue
			}
			// Ban check (defensive — FREDAlertCheck also excludes banned, but this is explicit)
			if s.deps.IsBanned != nil && s.deps.IsBanned(ctx, userID) {
				continue
			}
			// Tier check: Free users don't receive FRED alerts
			if s.deps.FREDAlertCheck != nil && !s.deps.FREDAlertCheck(ctx, userID) {
				continue
			}
			if _, sendErr := s.deps.Bot.SendWithKeyboard(ctx, prefs.ChatID, msg, fredAlertKB); sendErr == nil {
				count++
			}
			time.Sleep(config.TelegramFloodDelay)
		}
		log.Info().Str("alert_type", string(alert.Type)).Int("users", count).Msg("FRED alert sent")
	}

	// Invalidate AI caches that depend on FRED data
	if len(alerts) > 0 && s.deps.CachedAI != nil {
		s.deps.CachedAI.InvalidateOnFREDUpdate(ctx)
	}

	// --- SKEW/VIX tail risk alert (TASK-208) ---
	// Fetch VIX term structure (includes VolSuite with SKEW data) and check
	// for tail risk state transitions. Runs in goroutine to not block FRED job.
	go s.checkSKEWVIXAlert(ctx)

	return nil
}

// formatStrongSignalAlert formats a Telegram alert for high-strength COT signals.
func formatStrongSignalAlert(signals []cotsvc.Signal) string {
	var b strings.Builder
	b.WriteString("\xF0\x9F\x8E\xAF <b>STRONG COT SIGNAL ALERT</b>\n\n")
	for _, s := range signals {
		dirIcon := "\xF0\x9F\x9F\xA2"
		if s.Direction == "BEARISH" {
			dirIcon = "\xF0\x9F\x94\xB4"
		}
		b.WriteString(fmt.Sprintf("%s <b>%s %s</b> \xE2\x80\x94 Strength: %d/5 (%.0f%%)\n",
			dirIcon, s.Currency, string(s.Type), s.Strength, s.Confidence))
		b.WriteString(fmt.Sprintf("<i>%s</i>\n\n", s.Description))
	}
	b.WriteString("<i>Use /bias for full bias list</i>")
	return b.String()
}

// jobRetentionCleanup deletes expired data once per day at 03:00 WIB.
func (s *Scheduler) jobRetentionCleanup(ctx context.Context) error {
	now := timeutil.NowWIB()
	// Only run at 03:xx WIB to minimize impact
	if now.Hour() != 3 {
		return nil
	}

	if s.deps.DB == nil {
		return nil
	}

	policy := storage.DefaultRetentionPolicy()
	deleted, err := s.deps.DB.RunRetentionCleanup(ctx, policy)
	if err != nil {
		return fmt.Errorf("retention cleanup: %w", err)
	}
	if deleted > 0 {
		log.Info().Int("deleted", deleted).Msg("retention cleanup completed")
	}
	return nil
}

// jobPriceFetch fetches weekly price data for all contracts and stores it.
func (s *Scheduler) jobPriceFetch(ctx context.Context) error {
	if s.deps.PriceFetcher == nil || s.deps.PriceRepo == nil {
		return nil
	}

	// Use detailed fetch if the concrete Fetcher type is available.
	var records []domain.PriceRecord
	var report *pricesvc.FetchReport

	if fetcher, ok := s.deps.PriceFetcher.(*pricesvc.Fetcher); ok {
		var err error
		records, report, err = fetcher.FetchAllDetailed(ctx, 260)
		if err != nil {
			s.notifyOwnerPriceFetch(ctx, report, err)
			return fmt.Errorf("price fetch: %w", err)
		}

		// BUG-C1 fix: also fetch VIX + SPX for the risk filter.
		// Stored under "risk_VIX" / "risk_SPX" keys. Best-effort — non-fatal.
		riskRecords, riskErr := fetcher.FetchRiskInstruments(ctx, 8)
		if riskErr != nil {
			log.Warn().Err(riskErr).Msg("risk instrument fetch failed — VIX filter inactive")
		} else if len(riskRecords) > 0 {
			records = append(records, riskRecords...)
			log.Info().Int("risk_records", len(riskRecords)).Msg("VIX/SPX risk data fetched")
		}
	} else {
		var err error
		records, err = s.deps.PriceFetcher.FetchAll(ctx, 260)
		if err != nil {
			return fmt.Errorf("price fetch: %w", err)
		}
	}

	if len(records) > 0 {
		if err := s.deps.PriceRepo.SavePrices(ctx, records); err != nil {
			return fmt.Errorf("save prices: %w", err)
		}
		log.Info().Int("records", len(records)).Msg("price data saved")
	}

	// Send debug report to owner (only if detailed report available)
	if report != nil {
		s.notifyOwnerPriceFetch(ctx, report, nil)
	}

	return nil
}

// jobDailyPriceFetch fetches daily OHLCV data for all contracts and stores it.
func (s *Scheduler) jobDailyPriceFetch(ctx context.Context) error {
	if s.deps.PriceFetcher == nil || s.deps.DailyPriceRepo == nil {
		return nil
	}

	fetcher, ok := s.deps.PriceFetcher.(*pricesvc.Fetcher)
	if !ok {
		return fmt.Errorf("daily price fetch requires concrete Fetcher type")
	}

	// Fetch 730 days of daily data (2 years for comprehensive backtesting)
	records, report, err := fetcher.FetchAllDaily(ctx, 730)
	if err != nil {
		log.Warn().Err(err).Msg("daily price fetch failed")
		return fmt.Errorf("daily price fetch: %w", err)
	}

	if len(records) > 0 {
		if err := s.deps.DailyPriceRepo.SaveDailyPrices(ctx, records); err != nil {
			return fmt.Errorf("save daily prices: %w", err)
		}
		log.Info().Int("records", len(records)).Msg("daily price data saved")
	}

	if report != nil {
		log.Info().
			Int("success", report.Success).
			Int("failed", report.Failed).
			Dur("duration", report.Duration).
			Msg("daily price fetch report")
	}

	return nil
}

// jobIntradayPriceFetch fetches 15m OHLCV data and aggregates to all higher timeframes.
func (s *Scheduler) jobIntradayPriceFetch(ctx context.Context) error {
	if s.deps.PriceFetcher == nil || s.deps.IntradayRepo == nil {
		return nil
	}

	fetcher, ok := s.deps.PriceFetcher.(*pricesvc.Fetcher)
	if !ok {
		return fmt.Errorf("intraday price fetch requires concrete Fetcher type")
	}

	// Fetch 5000 bars of 15m data (~52 days, max TwelveData outputsize)
	baseBars, report, err := fetcher.FetchAllIntraday(ctx, "15m", 5000)
	if err != nil {
		log.Warn().Err(err).Msg("intraday price fetch failed")
		return fmt.Errorf("intraday price fetch: %w", err)
	}

	if len(baseBars) == 0 {
		return nil
	}

	// Group base bars by contract for aggregation
	byContract := make(map[string][]domain.IntradayBar)
	for _, bar := range baseBars {
		byContract[bar.ContractCode] = append(byContract[bar.ContractCode], bar)
	}

	// Aggregate and save all timeframes
	totalSaved := 0
	savedByInterval := make(map[string]int)
	for code, contractBars := range byContract {
		aggregated := pricesvc.AggregateFromBase(contractBars)
		for interval, bars := range aggregated {
			if len(bars) > 0 {
				if err := s.deps.IntradayRepo.SaveBars(ctx, bars); err != nil {
					log.Warn().Err(err).Str("interval", interval).Str("contract", code).Int("bars", len(bars)).Msg("save aggregated bars failed")
					continue
				}
				totalSaved += len(bars)
				savedByInterval[interval] += len(bars)
			}
		}
	}

	// Compute synthetic cross pairs for intraday (XAUEUR, etc.)
	for _, mapping := range domain.PriceOnlyMappings() {
		cross, ok := pricesvc.SyntheticCrossDef(mapping.Currency)
		if !ok {
			continue
		}
		numMapping := domain.FindPriceMappingByCurrency(cross.NumeratorCurrency)
		denMapping := domain.FindPriceMappingByCurrency(cross.DenominatorCurrency)
		if numMapping == nil || denMapping == nil {
			continue
		}
		numBars := byContract[numMapping.ContractCode]
		denBars := byContract[denMapping.ContractCode]
		if len(numBars) == 0 || len(denBars) == 0 {
			continue
		}

		// Build lookup by timestamp for denominator
		denByTS := make(map[time.Time]domain.IntradayBar)
		for _, b := range denBars {
			denByTS[b.Timestamp] = b
		}

		var crossBars []domain.IntradayBar
		for _, numBar := range numBars {
			denBar, ok := denByTS[numBar.Timestamp]
			if !ok || denBar.Close == 0 {
				continue
			}
			crossBars = append(crossBars, domain.IntradayBar{
				ContractCode: mapping.ContractCode,
				Interval:     numBar.Interval,
				Timestamp:    numBar.Timestamp,
				Open:         numBar.Open / denBar.Open,
				High:         numBar.High / denBar.High,
				Low:          numBar.Low / denBar.Low,
				Close:        numBar.Close / denBar.Close,
				Volume:       0,
			})
		}

		if len(crossBars) > 0 {
			// Aggregate cross bars to all timeframes
			aggregatedCross := pricesvc.AggregateFromBase(crossBars)
			for interval, bars := range aggregatedCross {
				if len(bars) > 0 {
					if err := s.deps.IntradayRepo.SaveBars(ctx, bars); err != nil {
						log.Warn().Err(err).Str("cross", mapping.Currency).Str("interval", interval).Msg("save cross intraday failed")
						continue
					}
					totalSaved += len(bars)
					savedByInterval[interval] += len(bars)
				}
			}
			log.Info().Str("cross", mapping.Currency).Int("base_bars", len(crossBars)).Msg("synthetic cross intraday computed")
		}
	}

	// Log per-interval breakdown
	for _, iv := range []string{"15m", "30m", "1h", "4h", "6h", "12h"} {
		if n := savedByInterval[iv]; n > 0 {
			log.Info().Str("interval", iv).Int("bars", n).Msg("intraday bars saved")
		}
	}

	log.Info().
		Int("base_bars", len(baseBars)).
		Int("total_saved", totalSaved).
		Msg("multi-timeframe intraday data saved")

	// Verify: read back counts for first contract
	if s.deps.IntradayRepo != nil {
		var firstCode string
		for c := range byContract {
			firstCode = c
			break
		}
		if firstCode != "" {
			for _, iv := range []string{"15m", "30m", "1h", "4h", "6h", "12h"} {
				readBack, _ := s.deps.IntradayRepo.GetHistory(ctx, firstCode, iv, 1)
				log.Info().Str("contract", firstCode).Str("interval", iv).Int("readback", len(readBack)).Msg("verify readback")
			}
		}
	}

	if report != nil {
		log.Info().
			Int("success", report.Success).
			Int("failed", report.Failed).
			Dur("duration", report.Duration).
			Msg("intraday price fetch report")
	}

	return nil
}

// notifyOwnerPriceFetch sends a debug-level price fetch report to the owner.
func (s *Scheduler) notifyOwnerPriceFetch(ctx context.Context, report *pricesvc.FetchReport, fetchErr error) {
	if s.deps.OwnerChatID == "" || s.deps.Bot == nil {
		return
	}
	if report == nil {
		return
	}

	var b strings.Builder
	b.WriteString("🔧 <b>Price Fetch Report</b>\n\n")

	// Per-contract results
	for _, r := range report.Results {
		if r.Error != "" {
			b.WriteString(fmt.Sprintf("❌ <b>%s</b>: %s\n", r.Currency, r.Error))
		} else {
			b.WriteString(fmt.Sprintf("✅ %s: <code>%s</code> (%d rec)\n", r.Currency, r.Source, r.Records))
		}
	}

	// Summary
	b.WriteString(fmt.Sprintf("\n<b>%d</b>/%d OK", report.Success, report.Success+report.Failed))
	if report.Duration > 0 {
		b.WriteString(fmt.Sprintf(" | took %s", report.Duration.Round(time.Millisecond)))
	}

	// Source breakdown
	srcCount := make(map[string]int)
	for _, r := range report.Results {
		if r.Source != "" {
			srcCount[r.Source]++
		}
	}
	if len(srcCount) > 0 {
		var srcParts []string
		for src, n := range srcCount {
			srcParts = append(srcParts, fmt.Sprintf("%s(%d)", src, n))
		}
		b.WriteString(fmt.Sprintf(" | %s", strings.Join(srcParts, ", ")))
	}

	if fetchErr != nil {
		b.WriteString(fmt.Sprintf("\n\n⚠️ <b>Error:</b> %s", fetchErr.Error()))
	}

	if _, err := s.deps.Bot.SendHTML(ctx, s.deps.OwnerChatID, b.String()); err != nil {
		log.Warn().Err(err).Msg("failed to send price fetch report to owner")
	}
}

// jobSignalEval evaluates pending signals by checking future price outcomes.
func (s *Scheduler) jobSignalEval(ctx context.Context) error {
	if s.deps.Evaluator == nil {
		return nil
	}

	evaluated, err := s.deps.Evaluator.EvaluatePending(ctx)
	if err != nil {
		return fmt.Errorf("signal eval: %w", err)
	}

	if evaluated > 0 {
		log.Info().Int("evaluated", evaluated).Msg("signal outcomes evaluated")
	}
	return nil
}

// gatherWeeklyData collects all data needed for the weekly outlook.
func (s *Scheduler) gatherWeeklyData(ctx context.Context) (ports.WeeklyData, error) {
	var data ports.WeeklyData

	// COT analyses
	analyses, err := s.deps.COTRepo.GetAllLatestAnalyses(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("COT analyses unavailable for weekly outlook")
	} else {
		data.COTAnalyses = analyses
	}

	// FRED macro data (best-effort)
	if md, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && md != nil {
		data.MacroData = md
	}

	// Backtest stats (best-effort)
	if s.deps.SignalRepo != nil {
		calc := backtestsvc.NewStatsCalculator(s.deps.SignalRepo)
		if stats, statsErr := calc.ComputeAll(ctx); statsErr == nil && stats.Evaluated > 0 {
			data.BacktestStats = stats
		}
	}

	// Price contexts (best-effort — nil if price data unavailable)
	if s.deps.PriceRepo != nil {
		ctxBuilder := pricesvc.NewContextBuilder(s.deps.PriceRepo)
		if priceCtxs, pcErr := ctxBuilder.BuildAll(ctx); pcErr == nil && len(priceCtxs) > 0 {
			data.PriceContexts = priceCtxs
		} else if pcErr != nil {
			log.Debug().Err(pcErr).Msg("price contexts unavailable for weekly outlook")
		}
	}

	return data, nil
}

// quantEnrichment holds pre-computed data for signal enrichment during persistence.
// All fields are optional — nil means the data was unavailable.
type quantEnrichment struct {
	priceCtxs    map[string]*domain.PriceContext // For ConvictionScoreV3 price component
	carryRanking *domain.CarryRanking            // For carry trade confidence adjustment (±5)
}

// persistSignals saves detected signals with their entry prices for backtesting.
func (s *Scheduler) persistSignals(ctx context.Context, signals []cotsvc.Signal, analyses []domain.COTAnalysis, enrich ...quantEnrichment) {
	// Build a lookup for analyses by contract code
	analysisMap := make(map[string]*domain.COTAnalysis, len(analyses))
	for i := range analyses {
		analysisMap[analyses[i].Contract.Code] = &analyses[i]
	}

	now := time.Now()
	var toSave []domain.PersistedSignal

	// Best-effort: fetch current FRED regime once for all signals.
	var fredRegime string
	var macroRegime fred.MacroRegime
	var macroData *fred.MacroData
	if md, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && md != nil {
		macroData = md
		comp := fred.ComputeComposites(md)
		macroRegime = fred.ClassifyMacroRegime(md, comp)
		fredRegime = macroRegime.Name
	}

	// Build daily trend filter once (reused across all signals to avoid redundant DB reads)
	var trendFilter *backtestsvc.DailyTrendFilter
	if s.deps.DailyPriceRepo != nil {
		dailyBuilder := pricesvc.NewDailyContextBuilder(s.deps.DailyPriceRepo)
		trendFilter = backtestsvc.NewDailyTrendFilter(dailyBuilder)
	}

	for _, sig := range signals {
		analysis := analysisMap[sig.ContractCode]
		if analysis == nil {
			continue
		}

		// Deduplicate: skip if signal already exists in the repository
		if s.deps.SignalRepo != nil {
			exists, err := s.deps.SignalRepo.SignalExists(ctx, sig.ContractCode, analysis.ReportDate, string(sig.Type))
			if err == nil && exists {
				log.Debug().
					Str("contract", sig.ContractCode).
					Str("type", string(sig.Type)).
					Msg("signal already exists — skipping duplicate")
				continue
			}
		}

		// Look up entry price at COT release date (Friday), not report date (Tuesday).
		// COT data reflects positioning as of Tuesday but is published Friday after market close.
		// Using Tuesday's price would be look-ahead bias — traders cannot act until Friday.
		releaseDate := cotReleaseDate(analysis.ReportDate)
		priceRec, err := s.deps.PriceRepo.GetPriceAt(ctx, sig.ContractCode, releaseDate)
		if err != nil || priceRec == nil || priceRec.Close <= 0 {
			log.Debug().
				Str("contract", sig.ContractCode).
				Msg("No entry price at report date — skipping signal persistence")
			continue
		}
		entryClose := priceRec.Close

		// Look up inverse flag
		var inverse bool
		mapping := domain.FindPriceMapping(sig.ContractCode)
		if mapping != nil {
			inverse = mapping.Inverse
		}

		ps := domain.PersistedSignal{
			ContractCode:   sig.ContractCode,
			Currency:       sig.Currency,
			SignalType:     string(sig.Type),
			Direction:      sig.Direction,
			Strength:       sig.Strength,
			Confidence:     sig.Confidence,
			Description:    sig.Description,
			ReportDate:     analysis.ReportDate,
			DetectedAt:     now,
			EntryPrice:     entryClose,
			Inverse:        inverse,
			COTIndex:       analysis.COTIndex,
			SentimentScore: analysis.SentimentScore,
		}

		ps.FREDRegime = fredRegime

		// Daily trend filter: adjust confidence based on daily price trend alignment.
		// This is additive — COT signal is primary, daily trend is secondary confirmation.
		if trendFilter != nil {
			adj := trendFilter.Adjust(ctx, sig.ContractCode, sig.Currency, sig.Direction, ps.Confidence)

			ps.RawConfidence = adj.RawConfidence
			ps.Confidence = adj.AdjustedConfidence
			ps.DailyTrend = adj.DailyTrend
			ps.DailyMATrend = adj.MATrend
			ps.DailyTrendAdj = adj.Adjustment

			if adj.Adjustment != 0 {
				log.Debug().
					Str("contract", sig.ContractCode).
					Str("direction", sig.Direction).
					Float64("raw", adj.RawConfidence).
					Float64("adj", adj.Adjustment).
					Float64("final", adj.AdjustedConfidence).
					Str("daily_trend", adj.DailyTrend).
					Str("ma_trend", adj.MATrend).
					Str("reason", adj.Reason).
					Msg("daily trend filter applied")
			}
		}

		// Compute ConvictionScore V3 (5-component: COT + Calendar + Stress + FRED + Price)
		var surpriseSigma float64
		if s.deps.SurpriseProvider != nil {
			surpriseSigma = s.deps.SurpriseProvider.GetSurpriseSigma(analysis.Contract.Currency)
		}
		var priceCtx *domain.PriceContext
		if len(enrich) > 0 && enrich[0].priceCtxs != nil {
			priceCtx = enrich[0].priceCtxs[sig.ContractCode]
		}
		cs := cotsvc.ComputeConvictionScoreV3(*analysis, macroRegime, surpriseSigma, "", macroData, priceCtx)
		ps.ConvictionScore = cs.Score

		// --- Quant Model Enrichment ---
		// NOTE: GARCH and HMM confidence multipliers are NOT applied here.
		// RecalibratedDetector already applies VIX (market regime) + ATR (per-contract vol)
		// multipliers to signal confidence. GARCH (forward vol) overlaps with ATR, and
		// HMM (crisis/risk-on/risk-off) overlaps with VIX regime classification.
		// Applying them here would double-count volatility/regime adjustments.
		//
		// GARCH/HMM results ARE computed and logged for observability (see broadcastCOTRelease),
		// and their information feeds into ConvictionScoreV3 via PriceContext indirectly.
		//
		// To properly integrate GARCH/HMM into confidence: replace the VIX+ATR pipeline
		// in RecalibratedDetector with a unified volatility multiplier using
		// CombineVolatilityWithGARCH() — tracked as a future refactor.

		// Apply carry trade adjustment (±5 additive based on rate differential alignment).
		// This is genuinely new information — interest rate differentials are independent
		// of volatility/regime adjustments already applied.
		if len(enrich) > 0 && enrich[0].carryRanking != nil {
			for _, pair := range enrich[0].carryRanking.Pairs {
				if pair.Currency == sig.Currency {
					carryAdj := fred.CarryAdjustment(pair, sig.Direction)
					if carryAdj != 0 {
						preCarryConf := ps.Confidence
						ps.Confidence = mathutil.Clamp(ps.Confidence+carryAdj, 0, 100)
						log.Debug().
							Str("contract", sig.ContractCode).
							Str("currency", sig.Currency).
							Float64("diff", pair.Differential).
							Float64("carry_adj", carryAdj).
							Float64("pre", preCarryConf).
							Float64("post", ps.Confidence).
							Msg("carry trade confidence adjustment")
					}
					break
				}
			}
		}

		toSave = append(toSave, ps)
	}

	if len(toSave) > 0 {
		if err := s.deps.SignalRepo.SaveSignals(ctx, toSave); err != nil {
			log.Warn().Err(err).Int("count", len(toSave)).Msg("failed to persist signals")
		} else {
			log.Info().Int("persisted", len(toSave)).Msg("signals persisted for backtesting")
		}
	}
}

// cotReleaseDate returns the Friday following a COT report date (Tuesday).
// CFTC reports are as-of Tuesday but published Friday at 3:30 PM ET.
func cotReleaseDate(reportDate time.Time) time.Time {
	wd := reportDate.Weekday()
	daysToFriday := (5 - int(wd) + 7) % 7
	if daysToFriday == 0 {
		daysToFriday = 7
	}
	return reportDate.AddDate(0, 0, daysToFriday)
}
