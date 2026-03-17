package news

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
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

	// sentReminders prevents duplicate pre-event alerts.
	// Key: "{eventID}:{minsUntil}", reset at midnight.
	sentMu        sync.Mutex
	sentReminders map[string]bool
	lastResetDay  string
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
		repo:          repo,
		fetcher:       fetcher,
		aiAnalyzer:    aiAnalyzer,
		messenger:     messenger,
		prefsRepo:     prefsRepo,
		sentReminders: make(map[string]bool),
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

	// 3. Micro-Scrape Trigger (Evaluated every minute — picks up actuals after release)
	go s.runMicroScrapeLoop(ctx)

	// 4. Pre-Event Reminder (Evaluated every minute — sends alerts X mins before event)
	go s.runPreEventReminderLoop(ctx)
}

// ---------------------------------------------------------------------------
// Weekly Sync
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Daily Morning Reminder
// ---------------------------------------------------------------------------

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

// broadcastDailyReminder sends a per-user morning summary filtered by their preferences.
func (s *Scheduler) broadcastDailyReminder(ctx context.Context, now time.Time) {
	dateStr := now.Format("20060102")
	events, err := s.repo.GetByDate(ctx, dateStr)
	if err != nil || len(events) == 0 {
		return
	}

	activeUsers, err := s.prefsRepo.GetAllActive(ctx)
	if err != nil {
		log.Printf("[NEWS SCHEDULER] broadcastDailyReminder: get users failed: %v", err)
		return
	}

	for userID, prefs := range activeUsers {
		if !prefs.AlertsEnabled || prefs.ChatID == "" {
			continue
		}

		alertImpactsLower := toLowerSlice(prefs.AlertImpacts)
		impactSet := toSet(alertImpactsLower)

		highCount, medCount, lowCount := 0, 0, 0
		var firstMatch *domain.NewsEvent

		for i := range events {
			e := &events[i]
			if len(prefs.CurrencyFilter) > 0 && !containsStr(prefs.CurrencyFilter, e.Currency) {
				continue
			}
			if !impactSet[strings.ToLower(e.Impact)] {
				continue
			}
			switch strings.ToLower(e.Impact) {
			case "high":
				highCount++
			case "medium":
				medCount++
			case "low":
				lowCount++
			}
			if firstMatch == nil {
				ev := *e
				firstMatch = &ev
			}
		}

		if highCount == 0 && medCount == 0 && lowCount == 0 {
			continue // Nothing matching this user's preferences today
		}

		html := fmt.Sprintf("🦅 <b>NEWS RADAR</b>: %s\n", now.Format("Mon Jan 02"))
		if highCount > 0 {
			html += fmt.Sprintf("🔴 High Impact: %d events\n", highCount)
		}
		if medCount > 0 {
			html += fmt.Sprintf("🟠 Medium Impact: %d events\n", medCount)
		}
		if lowCount > 0 {
			html += fmt.Sprintf("🟡 Low Impact: %d events\n", lowCount)
		}
		if firstMatch != nil {
			html += fmt.Sprintf("\nPertama: %s WIB — %s %s",
				firstMatch.TimeWIB.Format("15:04"), firstMatch.Currency, firstMatch.Event)
		}

		if _, sendErr := s.messenger.SendHTML(ctx, prefs.ChatID, html); sendErr != nil {
			log.Printf("[NEWS SCHEDULER] Failed to send daily reminder to user %d: %v", userID, sendErr)
		}
		time.Sleep(50 * time.Millisecond) // Avoid Telegram flood
	}
}

// ---------------------------------------------------------------------------
// Pre-Event Reminder (X minutes before event)
// ---------------------------------------------------------------------------

func (s *Scheduler) runPreEventReminderLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.evaluatePreEventReminders(ctx)
		}
	}
}

func (s *Scheduler) evaluatePreEventReminders(ctx context.Context) {
	now := timeutil.NowWIB()
	dateStr := now.Format("20060102")

	// Reset sent-reminders map at midnight
	s.sentMu.Lock()
	if s.lastResetDay != dateStr {
		s.sentReminders = make(map[string]bool)
		s.lastResetDay = dateStr
	}
	s.sentMu.Unlock()

	events, err := s.repo.GetByDate(ctx, dateStr)
	if err != nil || len(events) == 0 {
		return
	}

	activeUsers, err := s.prefsRepo.GetAllActive(ctx)
	if err != nil {
		return
	}

	for _, e := range events {
		if e.Actual != "" {
			continue // Already released
		}

		minsUntil := int(e.TimeWIB.Sub(now).Minutes())
		if minsUntil < 0 || minsUntil > 120 {
			continue // Not in relevant window
		}

		for userID, prefs := range activeUsers {
			if !prefs.AlertsEnabled || prefs.ChatID == "" {
				continue
			}

			// Check if this reminder minute matches user's alert minutes
			if !containsInt(prefs.AlertMinutes, minsUntil) {
				continue
			}

			// Check impact filter
			alertImpactsLower := toLowerSlice(prefs.AlertImpacts)
			if !toSet(alertImpactsLower)[strings.ToLower(e.Impact)] {
				continue
			}

			// Check currency filter
			if len(prefs.CurrencyFilter) > 0 && !containsStr(prefs.CurrencyFilter, e.Currency) {
				continue
			}

			// Anti-duplicate: skip if already sent this reminder
			reminderKey := fmt.Sprintf("%s:%d:%d", e.ID, minsUntil, userID)
			s.sentMu.Lock()
			alreadySent := s.sentReminders[reminderKey]
			if !alreadySent {
				s.sentReminders[reminderKey] = true
			}
			s.sentMu.Unlock()

			if alreadySent {
				continue
			}

			html := fmt.Sprintf("⏰ <b>EVENT INCOMING</b> — %d menit lagi\n\n", minsUntil)
			html += fmt.Sprintf("%s <b>%s</b> — %s\n", e.FormatImpactColor(), e.Currency, e.Event)
			html += fmt.Sprintf("🕐 %s WIB\n", e.TimeWIB.Format("15:04"))
			if e.Forecast != "" || e.Previous != "" {
				html += fmt.Sprintf("📊 Forecast: %s | Prev: %s\n", e.Forecast, e.Previous)
			}

			if _, sendErr := s.messenger.SendHTML(ctx, prefs.ChatID, html); sendErr != nil {
				log.Printf("[NEWS SCHEDULER] Failed to send pre-event alert to user %d: %v", userID, sendErr)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// ---------------------------------------------------------------------------
// Micro-Scrape (picks up actual values after release)
// ---------------------------------------------------------------------------

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

	// Hourly Sweep logic
	if now.Minute() == 0 {
		log.Println("[NEWS SCHEDULER] Running Hourly Slow-Poll Sweep")
		s.triggerMicroScrape(ctx, dateStr, "hourly")
		return
	}

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

		// Micro-Scrape targets: within 30 minutes after scheduled release.
		// Check at specific intervals to avoid hammering MQL5, but broad enough
		// to survive bot restarts or timing jitter that miss exact minutes.
		if minsSinceRelease >= 0 && minsSinceRelease <= 30 {
			if minsSinceRelease == 1 || minsSinceRelease == 3 || minsSinceRelease == 5 ||
				minsSinceRelease == 10 || minsSinceRelease == 15 || minsSinceRelease == 20 ||
				minsSinceRelease == 30 {
				log.Printf("[NEWS SCHEDULER] Micro-scrape triggered by %s %s (+%dm)", e.Currency, e.Event, minsSinceRelease)
				triggerScrape = true
				break
			}
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

	for _, ev := range newEvents {
		if ev.Actual != "" {
			originalEvt, _ := s.getEventByID(ctx, dateStr, ev.ID)
			if originalEvt != nil && originalEvt.Actual == "" {
				s.onNewRelease(ctx, ev)
			}
			_ = s.repo.UpdateActual(ctx, ev.ID, ev.Actual)
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

// onNewRelease broadcasts an actual-release alert to all eligible users.
func (s *Scheduler) onNewRelease(ctx context.Context, ev domain.NewsEvent) {
	log.Printf("[NEWS SCHEDULER] New Release Detected: %s %s -> %s", ev.Currency, ev.Event, ev.Actual)

	activeUsers, err := s.prefsRepo.GetAllActive(ctx)
	if err != nil {
		log.Printf("[NEWS SCHEDULER] onNewRelease: get users failed: %v", err)
		return
	}

	for userID, prefs := range activeUsers {
		if !prefs.AlertsEnabled || prefs.ChatID == "" {
			continue
		}

		alertImpactsLower := toLowerSlice(prefs.AlertImpacts)
		if !toSet(alertImpactsLower)[strings.ToLower(ev.Impact)] {
			continue
		}

		if len(prefs.CurrencyFilter) > 0 && !containsStr(prefs.CurrencyFilter, ev.Currency) {
			continue
		}

		// Optionally run AI flash analysis (language from prefs)
		analysisStr := ""
		if s.aiAnalyzer != nil && s.aiAnalyzer.IsAvailable() {
			analysisStr, _ = s.aiAnalyzer.AnalyzeActualRelease(ctx, ev, prefs.Language)
		}

		// Compute direction arrow (simple string-compare fallback)
		direction := "⚪"
		if ev.Actual != "" && ev.Forecast != "" && ev.Actual != ev.Forecast {
			if ev.Actual > ev.Forecast {
				direction = "🟢"
			} else {
				direction = "🔴"
			}
		}

		html := fmt.Sprintf("📈 <b>News Actual Release!</b>\n\n%s <b>%s</b>\n", ev.FormatImpactColor(), ev.Event)
		html += fmt.Sprintf("Currency: <b>%s</b>\n", ev.Currency)
		html += fmt.Sprintf("Actual: <b>%s %s</b> (Forecast: %s / Prev: %s)\n", ev.Actual, direction, ev.Forecast, ev.Previous)

		if analysisStr != "" {
			html += fmt.Sprintf("\n💡 <b>AI Analysis:</b>\n%s", analysisStr)
		}

		if _, sendErr := s.messenger.SendHTML(ctx, prefs.ChatID, html); sendErr != nil {
			log.Printf("[NEWS SCHEDULER] Failed to send release alert to user %d: %v", userID, sendErr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ---------------------------------------------------------------------------
// Initial Sync
// ---------------------------------------------------------------------------

func (s *Scheduler) runInitialSync(ctx context.Context) {
	now := timeutil.NowWIB()
	dateStr := now.Format("20060102")

	events, _ := s.repo.GetByDate(ctx, dateStr)
	if len(events) > 0 {
		log.Println("[NEWS SCHEDULER] Initial sync skipped: data already exists for today.")
		// Still run a startup micro-scrape to catch any actuals missed during downtime.
		log.Println("[NEWS SCHEDULER] Running startup missed-actuals check...")
		s.triggerMicroScrape(ctx, dateStr, "startup-check")
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

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

func toLowerSlice(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToLower(s)
	}
	return out
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func containsStr(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
