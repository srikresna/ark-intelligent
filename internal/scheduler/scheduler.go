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
	"log"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	cotsvc "github.com/arkcode369/ff-calendar-bot/internal/service/cot"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

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
}

// Intervals configures how often each job runs.
type Intervals struct {
	COTFetch time.Duration // Default: 6h
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

// Scheduler manages all background periodic jobs.
type Scheduler struct {
	deps         *Deps
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex
	lastFREDData *fred.MacroData // previous FRED snapshot for alert diffing
}

// New creates a new Scheduler.
func New(deps *Deps) *Scheduler {
	return &Scheduler{
		deps:   deps,
		stopCh: make(chan struct{}),
	}
}

// Start launches all background jobs. Non-blocking.
func (s *Scheduler) Start(ctx context.Context, intervals *Intervals) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		log.Println("[SCHED] Already running")
		return
	}
	s.running = true

	// COT fetch + analysis
	s.startJob(ctx, "cot-fetch", intervals.COTFetch, s.jobCOTFetch)

	// Weekly outlook (check every hour, fires on Sunday 18:00 WIB)
	s.startJob(ctx, "weekly-outlook", 1*time.Hour, s.jobWeeklyOutlook)

	// FRED alert monitor (checks every hour for regime changes)
	s.startJob(ctx, "fred-alerts", 1*time.Hour, s.jobFREDAlerts)

	log.Printf("[SCHED] Started 3 background jobs")
}

// Stop signals all jobs to stop and waits for them to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	log.Println("[SCHED] Stopping all jobs...")
	close(s.stopCh)
	s.wg.Wait()
	s.running = false
	log.Println("[SCHED] All jobs stopped")
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

		log.Printf("[SCHED] Job %q started (interval=%v, delay=%v)", name, interval, delay)

		// Run immediately on first start (don't wait for first tick)
		s.runJob(ctx, name, fn)

		for {
			select {
			case <-ticker.C:
				s.runJob(ctx, name, fn)
			case <-ctx.Done():
				log.Printf("[SCHED] Job %q: context cancelled", name)
				return
			case <-s.stopCh:
				log.Printf("[SCHED] Job %q: stop signal received", name)
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
			log.Printf("[SCHED] PANIC in job %q: %v", name, r)
		}
	}()

	if err := fn(jobCtx); err != nil {
		log.Printf("[SCHED] Job %q failed (took %v): %v", name, time.Since(start), err)
	} else {
		log.Printf("[SCHED] Job %q completed (took %v)", name, time.Since(start))
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
		log.Printf("[SCHED:cot-fetch] NEW DATA DETECTED: %s", newLatest.Format("2006-01-02"))
		s.broadcastCOTRelease(ctx, newLatest, analyses)
	}

	log.Println("[SCHED:cot-fetch] COT data fetched and analyzed")
	return nil
}

// broadcastCOTRelease sends a notification to all active users when new data is available.
func (s *Scheduler) broadcastCOTRelease(ctx context.Context, date time.Time, analyses []domain.COTAnalysis) {
	activeUsers, err := s.deps.PrefsRepo.GetAllActive(ctx)
	if err != nil {
		log.Printf("[SCHED:broadcast] Failed to get active users: %v", err)
		return
	}

	msg := fmt.Sprintf("\xF0\x9F\x94\x94 <b>NEW COT DATA RELEASED</b>\xF0\x9F\x94\x94\n\nReport Date: <b>%s</b>\n\nLatest positioning data has been fetched and analyzed. Use /cot to view the new insights.",
		date.Format("Monday, 02 Jan 2006"))

	count := 0
	for userID, prefs := range activeUsers {
		if !prefs.COTAlertsEnabled {
			continue
		}
		chatID := fmt.Sprintf("%d", userID)
		if _, err := s.deps.Bot.SendHTML(ctx, chatID, msg); err == nil {
			count++
		}
		// Avoid flooding Telegram API
		time.Sleep(50 * time.Millisecond)
	}

	log.Printf("[SCHED:broadcast] Sent COT release alert to %d users", count)

	// Signal detection — alert on strong signals (Strength >= 4)
	historyMap := make(map[string][]domain.COTRecord)
	for _, a := range analyses {
		records, hErr := s.deps.COTRepo.GetHistory(ctx, a.Contract.Code, 8)
		if hErr == nil && len(records) > 0 {
			historyMap[a.Contract.Code] = records
		}
	}
	detector := cotsvc.NewSignalDetector()
	signals := detector.DetectAll(analyses, historyMap)

	var strongSignals []cotsvc.Signal
	for _, sig := range signals {
		if sig.Strength >= 4 {
			strongSignals = append(strongSignals, sig)
		}
	}

	if len(strongSignals) > 0 {
		signalHTML := formatStrongSignalAlert(strongSignals)
		for _, prefs := range activeUsers {
			if prefs.COTAlertsEnabled && prefs.ChatID != "" {
				_, _ = s.deps.Bot.SendHTML(ctx, prefs.ChatID, signalHTML)
			}
		}
		log.Printf("[SCHED:broadcast] Sent strong signal alert (%d signals) to active users", len(strongSignals))
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

			for _, prefs := range activeUsers {
				if prefs.COTAlertsEnabled && prefs.ChatID != "" {
					_, _ = s.deps.Bot.SendHTML(ctx, prefs.ChatID, html)
					time.Sleep(50 * time.Millisecond)
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
		log.Println("[SCHED:weekly-outlook] AI not available, skipping")
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

	log.Println("[SCHED:weekly-outlook] Weekly outlook sent")
	return nil
}

// jobFREDAlerts checks for FRED macro regime changes and broadcasts alerts to subscribed users.
// Runs every hour. Compares the freshly fetched MacroData against the previous snapshot.
func (s *Scheduler) jobFREDAlerts(ctx context.Context) error {
	current, err := fred.GetCachedOrFetch(ctx)
	if err != nil {
		return fmt.Errorf("fred fetch for alerts: %w", err)
	}

	s.mu.Lock()
	previous := s.lastFREDData
	s.lastFREDData = current
	s.mu.Unlock()

	alerts := fred.CheckAlerts(current, previous)
	if len(alerts) == 0 {
		return nil
	}

	log.Printf("[SCHED:fred-alerts] %d alert(s) detected", len(alerts))

	activeUsers, err := s.deps.PrefsRepo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("get active users for fred alerts: %w", err)
	}

	for _, alert := range alerts {
		msg := fred.FormatMacroAlert(alert)
		count := 0
		for userID, prefs := range activeUsers {
			if !prefs.COTAlertsEnabled {
				continue
			}
			chatID := fmt.Sprintf("%d", userID)
			if _, sendErr := s.deps.Bot.SendHTML(ctx, chatID, msg); sendErr == nil {
				count++
			}
			time.Sleep(50 * time.Millisecond)
		}
		log.Printf("[SCHED:fred-alerts] Alert %q sent to %d users", alert.Type, count)
	}

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
	b.WriteString("<i>Use /signals for full signal list</i>")
	return b.String()
}

// gatherWeeklyData collects all data needed for the weekly outlook.
func (s *Scheduler) gatherWeeklyData(ctx context.Context) (ports.WeeklyData, error) {
	var data ports.WeeklyData

	// COT analyses
	analyses, err := s.deps.COTRepo.GetAllLatestAnalyses(ctx)
	if err != nil {
		log.Printf("[SCHED:weekly-outlook] COT analyses unavailable: %v", err)
	} else {
		data.COTAnalyses = analyses
	}

	return data, nil
}
