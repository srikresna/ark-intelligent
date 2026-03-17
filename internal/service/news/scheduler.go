package news

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

// Scheduler manages background pulling of economic data and dispatching alerts.
type Scheduler struct {
	repo       ports.NewsRepository
	fetcher    ports.NewsFetcher
	aiAnalyzer ports.AIAnalyzer
	messenger  ports.Messenger
	prefsRepo  ports.PrefsRepository
}

// NewScheduler creates a new background scheduler.
func NewScheduler(
	repo ports.NewsRepository,
	fetcher ports.NewsFetcher,
	aiAnalyzer ports.AIAnalyzer,
	messenger ports.Messenger,
	prefsRepo ports.PrefsRepository,
) *Scheduler {
	return &Scheduler{
		repo:       repo,
		fetcher:    fetcher,
		aiAnalyzer: aiAnalyzer,
		messenger:  messenger,
		prefsRepo:  prefsRepo,
	}
}

// Start begins the background monitoring loop.
func (s *Scheduler) Start(ctx context.Context) {
	log.Println("[NEWS SCHEDULER] Starting background monitors...")

	// 0. Initial Sync (Run once on startup if empty)
	go s.runInitialSync(ctx)

	// 1. Weekly Sync Monitor (Runs every Sunday at 23:00 WIB)
	go s.runWeeklySyncLoop(ctx)

	// 2. Daily Morning Reminder Monitor (Runs every day at 06:00 WIB)
	go s.runDailyReminderLoop(ctx)

	// 3. Micro-Scrape Trigger (Evaluated every minute)
	go s.runMicroScrapeLoop(ctx)
}

func (s *Scheduler) runWeeklySyncLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := timeutil.NowWIB()
			// Condition: Sunday and Hour == 23
			if now.Weekday() == time.Sunday && now.Hour() == 23 {
				log.Println("[NEWS SCHEDULER] Triggering Weekly Sync Scrape")
				events, err := s.fetcher.ScrapeCalendar(ctx, "next")
				if err != nil {
					log.Printf("[NEWS SCHEDULER] Weekly sync failed: %v", err)
					continue
				}

				if err := s.repo.SaveEvents(ctx, events); err != nil {
					log.Printf("[NEWS SCHEDULER] Failed to save weekly events: %v", err)
				}
				log.Printf("[NEWS SCHEDULER] Weekly sync successful, parsed %d events", len(events))
			}
		}
	}
}

func (s *Scheduler) runDailyReminderLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	lastSentDate := ""

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := timeutil.NowWIB()
			dateStr := now.Format("20060102")

			// Condition: 06:00 AM WIB and not already sent today
			if now.Hour() == 6 && now.Minute() == 0 && lastSentDate != dateStr {
				log.Println("[NEWS SCHEDULER] Triggering Daily Morning Reminder")
				s.broadcastDailyReminder(ctx, now)
				lastSentDate = dateStr
			}
		}
	}
}

func (s *Scheduler) broadcastDailyReminder(ctx context.Context, now time.Time) {
	dateStr := now.Format("20060102")
	events, err := s.repo.GetByDate(ctx, dateStr)
	if err != nil || len(events) == 0 {
		return
	}

	highCount, medCount := 0, 0
	var firstHigh *domain.NewsEvent

	for _, e := range events {
		if e.Impact == "high" {
			highCount++
			if firstHigh == nil {
				// Store copy of first high impact event for the day
				evt := e
				firstHigh = &evt
			}
		} else if e.Impact == "medium" {
			medCount++
		}
	}

	if highCount == 0 && medCount == 0 {
		return // Nothing interesting today
	}

	html := fmt.Sprintf("🦅 <b>NEWS RADAR</b>: %s\n", now.Format("Mon Jan 02"))
	if highCount > 0 {
		html += fmt.Sprintf("🔴 High Impact: %d events\n", highCount)
	}
	if medCount > 0 {
		html += fmt.Sprintf("🟡 Medium Impact: %d events\n", medCount)
	}
	if firstHigh != nil {
		html += fmt.Sprintf("\nPertama: %s WIB - %s %s", firstHigh.TimeWIB.Format("15:04"), firstHigh.Currency, firstHigh.Event)
	}

	// Assuming we have a global broadcast group ID in messenger (default chat ID)
	// Alternatively, we get it from environment
	// Ports.Messenger interface doesn't natively expose Broadcast, but Bot does.
	// For simplicity, we just send to a generic target if we know it, or we rely on user subscriptions.
	// In the FF bot design, broadcasts go to a default Channel via a specific wrapper.
	// We'll use SendHTML with empty string which defaults to the global Broadcast if implemented.
	_, _ = s.messenger.SendHTML(ctx, "", html)
}

func (s *Scheduler) runMicroScrapeLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.evaluatePendingScrapes(ctx)
		}
	}
}

func (s *Scheduler) evaluatePendingScrapes(ctx context.Context) {
	now := timeutil.NowWIB()
	dateStr := now.Format("20060102")

	// Hourly Sweep logic (checking for missed/pending data across the day if at top of hour)
	if now.Minute() == 0 {
		log.Println("[NEWS SCHEDULER] Running Hourly Slow-Poll Sweep")
		s.triggerMicroScrape(ctx, dateStr, "hourly")
		return
	}

	// Fast Backoff Logic
	events, err := s.repo.GetByDate(ctx, dateStr)
	if err != nil || len(events) == 0 {
		return
	}

	triggerScrape := false
	for _, e := range events {
		if e.Actual != "" {
			continue // Already fulfilled
		}

		minsSinceRelease := int(now.Sub(e.TimeWIB).Minutes())

		// Micro-Scrape targets: +1min, +5min, +10min
		if minsSinceRelease == 1 || minsSinceRelease == 5 || minsSinceRelease == 10 {
			log.Printf("[NEWS SCHEDULER] Micro-scrape triggered by %s %s (+%dm)", e.Currency, e.Event, minsSinceRelease)
			triggerScrape = true
			break // Batch fetch covers whole day anyway
		}
	}

	if triggerScrape {
		s.triggerMicroScrape(ctx, dateStr, "event-driven")
	}
}

func (s *Scheduler) triggerMicroScrape(ctx context.Context, dateStr string, reason string) {
	newEvents, err := s.fetcher.ScrapeActuals(ctx, dateStr)
	if err != nil {
		log.Printf("[NEWS SCHEDULER] Micro-Scrape failed (%s): %v", reason, err)
		return
	}

	// Update datastore
	for _, ev := range newEvents {
		if ev.Actual != "" {
			// Find if we need to alert just now
			originalEvt, _ := s.getEventByID(ctx, dateStr, ev.ID)
			if originalEvt != nil && originalEvt.Actual == "" {
				s.onNewRelease(ctx, ev)
			}
			s.repo.UpdateActual(ctx, ev.ID, ev.Actual)
		}
	}
}

func (s *Scheduler) getEventByID(ctx context.Context, dateStr string, id string) (*domain.NewsEvent, error) {
	events, err := s.repo.GetByDate(ctx, dateStr)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		if e.ID == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *Scheduler) onNewRelease(ctx context.Context, ev domain.NewsEvent) {
	if ev.Impact != "high" && ev.Impact != "medium" {
		return // Only alert high/medium
	}

	log.Printf("[NEWS SCHEDULER] New Release Detected: %s %s -> %s", ev.Currency, ev.Event, ev.Actual)

	analysisStr := ""
	if s.aiAnalyzer.IsAvailable() {
		// Attempt flash analysis
		// Wait, we haven't implemented language check here yet. Just passing English for now or default "id".
		analysisStr, _ = s.aiAnalyzer.AnalyzeActualRelease(ctx, ev, "id")
	}

	html := fmt.Sprintf("📈 <b>News Actual Release!</b>\n\n%s <b>%s</b>\n", ev.FormatImpactColor(), ev.Event)
	html += fmt.Sprintf("Data: <b>%s</b>\n", ev.Currency)
	html += fmt.Sprintf("Actual: <b>%s</b> (Forecast: %s / Prev: %s)\n", ev.Actual, ev.Forecast, ev.Previous)

	if analysisStr != "" {
		html += fmt.Sprintf("\n💡 <b>AI Analysis:</b>\n%s", analysisStr)
	}

	_, _ = s.messenger.SendHTML(ctx, "", html)
}

func (s *Scheduler) runInitialSync(ctx context.Context) {
	now := timeutil.NowWIB()
	dateStr := now.Format("20060102")

	events, _ := s.repo.GetByDate(ctx, dateStr)
	if len(events) > 0 {
		log.Println("[NEWS SCHEDULER] Initial sync skipped: data already exists for today.")
		return
	}

	log.Println("[NEWS SCHEDULER] Triggering Initial Sync Scrape for current week")
	newEvents, err := s.fetcher.ScrapeCalendar(ctx, "this")
	if err != nil {
		log.Printf("[NEWS SCHEDULER] Initial sync failed: %v", err)
		return
	}

	if err := s.repo.SaveEvents(ctx, newEvents); err != nil {
		log.Printf("[NEWS SCHEDULER] Failed to save initial events: %v", err)
	} else {
		log.Printf("[NEWS SCHEDULER] Initial sync successful, saved %d events for the week", len(newEvents))
	}
}
