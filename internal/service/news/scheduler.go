package news

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/internal/service/cot"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/saferun"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

var schedLog = logger.Component("news-scheduler")

// AlertFilterFunc is a callback that returns the effective currency filter and impact filter
// for a user, applying tier-based overrides (e.g., Free → USD+High only).
// Parameters: (ctx, userID, prefsCurrencies, prefsImpacts) → (currencies, impacts)
type AlertFilterFunc func(ctx context.Context, userID int64, prefsCurrencies, prefsImpacts []string) ([]string, []string)

// FREDAlertCheckFunc checks if a user should receive FRED alerts (Free tier excluded).
type FREDAlertCheckFunc func(ctx context.Context, userID int64) bool

// Scheduler manages background pulling of economic data and dispatching alerts.
type Scheduler struct {
	repo       ports.NewsRepository
	fetcher    ports.NewsFetcher
	aiAnalyzer ports.AIAnalyzer
	messenger  ports.Messenger
	prefsRepo  ports.PrefsRepository
	cotRepo    ports.COTRepository // P1.1 — for Confluence Alert cross-check

	// sentReminders prevents duplicate pre-event alerts.
	// Key: "{eventID}:{minsUntil}", reset at midnight.
	sentMu        sync.Mutex
	sentReminders map[string]bool
	lastResetDay  string

	// surpriseAccMu guards surpriseAccum.
	surpriseAccMu sync.RWMutex
	// surpriseAccum holds cumulative sigma per currency for the current ISO week.
	// Key: "YYYYWW:CURRENCY" (e.g. "202612:EUR"), Value: cumulative sigma sum.
	surpriseAccum map[string]float64
	// surpriseWeek tracks the current ISO week global key (e.g. "202612") for auto-reset.
	surpriseWeek string

	// onNewsInvalidate is called when significant news data changes (new releases with high surprise).
	// Used by AI cache layer to invalidate news-dependent caches.
	onNewsInvalidate func(ctx context.Context)

	// alertFilter applies tier-based overrides to alert filtering. May be nil (no tier filtering).
	alertFilter AlertFilterFunc

	// isBanned checks if a user is banned. May be nil (no ban check).
	// When set, all broadcast loops explicitly skip banned users.
	isBanned func(ctx context.Context, userID int64) bool

	// impactRecorder captures price impact after event releases.
	// May be nil — impact recording disabled if price data unavailable.
	impactRecorder *ImpactRecorder

	// latestFedSpeeches caches the most recent Fed speeches for AI context.
	latestFedMu      sync.RWMutex
	latestFedSpeeches []FedSpeech
}

// NewScheduler creates a new background scheduler.
func NewScheduler(
	repo ports.NewsRepository,
	fetcher ports.NewsFetcher,
	aiAnalyzer ports.AIAnalyzer,
	messenger ports.Messenger,
	prefsRepo ports.PrefsRepository,
	cotRepo ports.COTRepository, // P1.1 — injected for confluence alerts
) *Scheduler {
	return &Scheduler{
		repo:          repo,
		fetcher:       fetcher,
		aiAnalyzer:    aiAnalyzer,
		messenger:     messenger,
		prefsRepo:     prefsRepo,
		cotRepo:       cotRepo,
		sentReminders: make(map[string]bool),
		surpriseAccum: make(map[string]float64),
	}
}

// SetNewsInvalidateFunc sets the callback for news cache invalidation.
func (s *Scheduler) SetNewsInvalidateFunc(fn func(ctx context.Context)) {
	s.onNewsInvalidate = fn
}

// SetAlertFilterFunc sets the tier-based alert filter callback.
func (s *Scheduler) SetAlertFilterFunc(fn AlertFilterFunc) {
	s.alertFilter = fn
}

// SetIsBannedFunc sets the ban-check callback for broadcast filtering.
func (s *Scheduler) SetIsBannedFunc(fn func(ctx context.Context, userID int64) bool) {
	s.isBanned = fn
}

// SetImpactRecorder sets the impact recorder for capturing price impact after releases.
func (s *Scheduler) SetImpactRecorder(recorder *ImpactRecorder) {
	s.impactRecorder = recorder
}

// Start begins the background monitoring loop.
func (s *Scheduler) Start(ctx context.Context) {
	schedLog.Info().Msg("starting background monitors")

	// 0. Initial Sync (Run once on startup if empty)
	saferun.Go(ctx, "news-initial-sync", schedLog, func() { s.runInitialSync(ctx) })

	// 1. Weekly Sync Monitor (Runs every Sunday at 23:00 WIB)
	saferun.Go(ctx, "news-weekly-sync", schedLog, func() { s.runWeeklySyncLoop(ctx) })

	// 2. Daily Morning Reminder Monitor (Runs every day at 06:00 WIB)
	saferun.Go(ctx, "news-daily-reminder", schedLog, func() { s.runDailyReminderLoop(ctx) })

	// 3. Micro-Scrape Trigger (Evaluated every minute — picks up actuals after release)
	saferun.Go(ctx, "news-micro-scrape", schedLog, func() { s.runMicroScrapeLoop(ctx) })

	// 4. Pre-Event Reminder (Evaluated every minute — sends alerts X mins before event)
	saferun.Go(ctx, "news-pre-event-reminder", schedLog, func() { s.runPreEventReminderLoop(ctx) })

	// 5. Fed Speeches & FOMC Press RSS Monitor (every 30 minutes)
	saferun.Go(ctx, "fed-rss-monitor", schedLog, func() { s.runFedRSSLoop(ctx) })
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
				schedLog.Info().Msg("triggering weekly sync scrape")
				events, err := s.fetcher.ScrapeCalendar(ctx, "next")
				if err != nil {
					schedLog.Error().Err(err).Msg("weekly sync failed")
					continue
				}
				if err := s.repo.SaveEvents(ctx, events); err != nil {
					schedLog.Error().Err(err).Msg("failed to save weekly events")
				}
				schedLog.Info().Int("events", len(events)).Msg("weekly sync successful")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Daily Morning Reminder — P1.2 Storm Day Detection
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

			// Condition: 06:xx AM WIB and not already sent today
			// Removed now.Minute() == 0 check — ticker phase depends on
			// process start time, so the minute-0 window can be missed entirely.
			// lastSentDate guard already prevents duplicate sends.
			if now.Hour() == 6 && lastSentDate != dateStr {
				schedLog.Info().Msg("triggering daily morning reminder")
				s.broadcastDailyReminder(ctx, now)
				lastSentDate = dateStr
			}
		}
	}
}

// broadcastDailyReminder sends a per-user morning summary filtered by their preferences.
// P1.2: Includes Storm Day warning if 3+ high-impact events detected.
func (s *Scheduler) broadcastDailyReminder(ctx context.Context, now time.Time) {
	dateStr := now.Format("20060102")
	events, err := s.repo.GetByDate(ctx, dateStr)
	if err != nil || len(events) == 0 {
		return
	}

	activeUsers, err := s.prefsRepo.GetAllActive(ctx)
	if err != nil {
		schedLog.Error().Err(err).Msg("broadcastDailyReminder: get users failed")
		return
	}

	for userID, prefs := range activeUsers {
		if !prefs.AlertsEnabled || prefs.ChatID == "" {
			continue
		}
		// Explicit ban check
		if s.isBanned != nil && s.isBanned(ctx, userID) {
			continue
		}

		// Apply tier-based filter overrides (Free → USD + High only)
		effectiveCurrencies := prefs.CurrencyFilter
		effectiveImpacts := prefs.AlertImpacts
		if s.alertFilter != nil {
			effectiveCurrencies, effectiveImpacts = s.alertFilter(ctx, userID, prefs.CurrencyFilter, prefs.AlertImpacts)
		}

		alertImpactsLower := toLowerSlice(effectiveImpacts)
		impactSet := toSet(alertImpactsLower)

		highCount, medCount, lowCount := 0, 0, 0
		var firstMatch *domain.NewsEvent

		for i := range events {
			e := &events[i]
			if len(effectiveCurrencies) > 0 && !containsStr(effectiveCurrencies, e.Currency) {
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

		// P1.2 — Append Storm Day warning if applicable (filtered by user's effective currencies)
		stormWarning := s.buildStormDayWarning(events, now, effectiveCurrencies)
		if stormWarning != "" {
			html += "\n\n" + stormWarning
		}

		if _, sendErr := s.messenger.SendHTML(ctx, prefs.ChatID, html); sendErr != nil {
			schedLog.Error().Int64("user_id", userID).Err(sendErr).Msg("failed to send daily reminder")
		}
		time.Sleep(config.TelegramFloodDelay) // Avoid Telegram flood
	}
}

// buildStormDayWarning detects if today is a Storm Day (3+ high-impact events).
// Returns the formatted HTML warning string, or "" if not a storm day.
// If currencyFilter is non-empty, only events matching those currencies are considered.
//
// P1.2 — Storm Day Detection
func (s *Scheduler) buildStormDayWarning(events []domain.NewsEvent, now time.Time, currencyFilter []string) string {
	var highEvents []domain.NewsEvent
	currencySet := make(map[string]bool)

	for _, e := range events {
		if strings.ToLower(e.Impact) == "high" {
			// Apply currency filter if present
			if len(currencyFilter) > 0 && !containsStr(currencyFilter, e.Currency) {
				continue
			}
			highEvents = append(highEvents, e)
			if e.Currency != "" {
				currencySet[strings.ToUpper(e.Currency)] = true
			}
		}
	}

	if len(highEvents) < 3 {
		return "" // Not a storm day
	}

	// Build event name list (max 4 shown)
	eventNames := make([]string, 0, len(highEvents))
	for _, e := range highEvents {
		eventNames = append(eventNames, e.Currency+" "+e.Event)
	}
	shownEvents := eventNames
	if len(shownEvents) > 4 {
		shownEvents = make([]string, 5)
		copy(shownEvents, eventNames[:4])
		shownEvents[4] = fmt.Sprintf("+%d more", len(eventNames)-4)
	}

	// Build volatile pairs from involved currencies
	pairs := buildVolatilePairs(currencySet)

	html := fmt.Sprintf("⚡ <b>STORM DAY</b> — %s\n", now.Format("Mon 02 Jan"))
	html += fmt.Sprintf("%d High-Impact Events: <i>%s</i>\n", len(highEvents), strings.Join(shownEvents, ", "))
	if len(pairs) > 0 {
		html += fmt.Sprintf("Pairs volatile: <b>%s</b>\n", strings.Join(pairs, ", "))
	}
	html += "→ Kurangi size, wider stops ⚠️"

	return html
}

// buildVolatilePairs generates pair names from a set of currencies involved in high-impact events.
func buildVolatilePairs(currencySet map[string]bool) []string {
	var pairs []string
	seen := make(map[string]bool)

	priorityPairs := [][2]string{
		{"USD", "GBP"}, {"USD", "EUR"}, {"USD", "JPY"}, {"USD", "AUD"},
		{"USD", "CAD"}, {"USD", "NZD"}, {"USD", "CHF"},
		{"GBP", "JPY"}, {"EUR", "JPY"}, {"GBP", "EUR"},
	}

	for _, pair := range priorityPairs {
		a, b := pair[0], pair[1]
		if currencySet[a] && currencySet[b] {
			var pairName string
			if a == "USD" {
				pairName = b + "USD"
			} else if b == "USD" {
				pairName = a + "USD"
			} else {
				pairName = a + b
			}
			if !seen[pairName] {
				pairs = append(pairs, pairName)
				seen[pairName] = true
			}
			if len(pairs) >= 4 {
				break
			}
		}
	}

	if currencySet["USD"] && !seen["DXY"] {
		pairs = append(pairs, "DXY")
	}

	return pairs
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

		if e.TimeWIB.IsZero() {
			schedLog.Warn().Str("event", e.Event).Str("currency", e.Currency).Msg("skipping event with zero TimeWIB")
			continue
		}

		minsUntil := int(e.TimeWIB.Sub(now).Minutes())
		if minsUntil < 0 || minsUntil > 120 {
			continue // Not in relevant window
		}

		for userID, prefs := range activeUsers {
			if !prefs.AlertsEnabled || prefs.ChatID == "" {
				continue
			}
			// Explicit ban check
			if s.isBanned != nil && s.isBanned(ctx, userID) {
				continue
			}

			if !containsInt(prefs.AlertMinutes, minsUntil) {
				continue
			}

			// Apply tier-based filter overrides
			effectiveCurrencies := prefs.CurrencyFilter
			effectiveImpacts := prefs.AlertImpacts
			if s.alertFilter != nil {
				effectiveCurrencies, effectiveImpacts = s.alertFilter(ctx, userID, prefs.CurrencyFilter, prefs.AlertImpacts)
			}

			alertImpactsLower := toLowerSlice(effectiveImpacts)
			if !toSet(alertImpactsLower)[strings.ToLower(e.Impact)] {
				continue
			}

			if len(effectiveCurrencies) > 0 && !containsStr(effectiveCurrencies, e.Currency) {
				continue
			}

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
				schedLog.Error().Int64("user_id", userID).Err(sendErr).Msg("failed to send pre-event alert")
			}
			time.Sleep(config.TelegramFloodDelay)
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

	if now.Minute() == 0 {
		schedLog.Info().Msg("running hourly slow-poll sweep")
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
			continue
		}

		if e.TimeWIB.IsZero() {
			continue
		}

		minsSinceRelease := int(now.Sub(e.TimeWIB).Minutes())

		if minsSinceRelease >= 0 && minsSinceRelease <= 30 {
			if minsSinceRelease == 1 || minsSinceRelease == 3 || minsSinceRelease == 5 ||
				minsSinceRelease == 10 || minsSinceRelease == 15 || minsSinceRelease == 20 ||
				minsSinceRelease == 30 {
				schedLog.Info().Str("currency", e.Currency).Str("event", e.Event).Int("mins_since", minsSinceRelease).Msg("micro-scrape triggered")
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
		schedLog.Error().Str("reason", reason).Err(err).Msg("micro-scrape failed")
		return
	}

	for _, ev := range newEvents {
		if ev.Actual != "" {
			originalEvt, _ := s.getEventByID(ctx, dateStr, ev.ID)
			if originalEvt != nil && originalEvt.Actual == "" {
				s.onNewRelease(ctx, ev)
			}
			if err := s.repo.UpdateActual(ctx, ev.ID, ev.Actual); err != nil {
				schedLog.Error().Str("id", ev.ID).Err(err).Msg("failed to persist actual value")
			}
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

// ---------------------------------------------------------------------------
// onNewRelease — P1.1 Confluence Alert
// ---------------------------------------------------------------------------

// onNewRelease broadcasts an actual-release alert to all eligible users.
// P1.1: Cross-checks the release against COT positioning for the same currency.
func (s *Scheduler) onNewRelease(ctx context.Context, ev domain.NewsEvent) {
	schedLog.Info().Str("currency", ev.Currency).Str("event", ev.Event).Str("actual", ev.Actual).Msg("new release detected")

	activeUsers, err := s.prefsRepo.GetAllActive(ctx)
	if err != nil {
		schedLog.Error().Err(err).Msg("onNewRelease: get users failed")
		return
	}

	// P1.1 — Fetch COT analysis for this currency (best-effort)
	var cotAnalysis *domain.COTAnalysis
	if s.cotRepo != nil {
		contractCode := domain.CurrencyToContract(ev.Currency)
		if contractCode != "" {
			cotAnalysis, _ = s.cotRepo.GetLatestAnalysis(ctx, contractCode)
		}
	}

	// Compute and store surprise metrics on the event
	actualVal, hasActual := ParseNumericValue(ev.Actual)
	forecastVal, hasForecast := ParseNumericValue(ev.Forecast)
	previousVal, hasPrevious := ParseNumericValue(ev.Previous)
	oldPreviousVal, hasOldPrevious := ParseNumericValue(ev.OldPrevious)

	if hasActual && hasForecast {
		// BUG #5 FIX: Fetch historical (actual - forecast) diffs for the same event/currency
		// so ComputeSurpriseWithDirection can normalize by stddev instead of using raw diff.
		// Without this, a raw diff of 0.05 could incorrectly label as "MAJOR BULLISH SURPRISE"
		// because the threshold (abs >= 2.0) was designed for sigma units, not raw values.
		// GetHistoricalSurprises returns nil (not an error) when < 3 data points exist,
		// in which case ComputeSurpriseWithDirection gracefully falls back to raw diff.
		history, histErr := s.repo.GetHistoricalSurprises(ctx, ev.Event, ev.Currency, 6)
		if histErr != nil {
			schedLog.Warn().Str("event", ev.Event).Err(histErr).Msg("failed to fetch historical surprises; falling back to raw diff")
			history = nil
		}
		ev.SurpriseScore = ComputeSurpriseWithDirection(actualVal, forecastVal, history, ev.ImpactDirection)
		ev.SurpriseLabel = ClassifySurpriseWithDirection(ev.SurpriseScore, ev.ImpactDirection)
	}

	// Revision tracking: OldPrevious → Previous
	if hasOldPrevious && hasPrevious && math.Abs(previousVal-oldPreviousVal) > 0.001 {
		revDiff := previousVal - oldPreviousVal
		if revDiff > 0 {
			ev.RevisionLabel = "UPWARD REVISION"
		} else {
			ev.RevisionLabel = "DOWNWARD REVISION"
		}
		if oldPreviousVal != 0 {
			ev.RevisionSurprise = revDiff / math.Abs(oldPreviousVal)
		} else {
			ev.RevisionSurprise = revDiff
		}
	}

	// Persist revision to storage for historical tracking
	if ev.RevisionLabel != "" && s.repo != nil {
		dir := domain.RevisionUp
		if ev.RevisionSurprise < 0 {
			dir = domain.RevisionDown
		}
		revRecord := domain.EventRevision{
			EventID:       ev.ID,
			EventName:     ev.Event,
			Currency:      ev.Currency,
			RevisionDate:  time.Now(),
			OriginalValue: ev.OldPrevious,
			RevisedValue:  ev.Previous,
			Direction:     dir,
			Magnitude:     math.Abs(ev.RevisionSurprise),
		}
		if saveErr := s.repo.SaveRevision(ctx, revRecord); saveErr != nil {
			schedLog.Error().Str("currency", ev.Currency).Str("event", ev.Event).Err(saveErr).Msg("failed to save revision")
		}
	}

	// Record price impact for the Event Impact Database (non-blocking).
	// Use context.Background() so a scheduler ctx cancellation (restart/shutdown)
	// does not prevent past-horizon impact records from being persisted.
	if s.impactRecorder != nil && hasActual {
		saferun.Go(ctx, "record-impact-"+ev.Event, schedLog, func() {
			s.impactRecorder.RecordImpact(context.Background(), ev, ev.SurpriseScore, []string{"15m", "30m", "1h", "4h"})
		})
	}

	for userID, prefs := range activeUsers {
		if !prefs.AlertsEnabled || prefs.ChatID == "" {
			continue
		}
		// Explicit ban check
		if s.isBanned != nil && s.isBanned(ctx, userID) {
			continue
		}

		// Apply tier-based filter overrides
		effectiveCurrencies := prefs.CurrencyFilter
		effectiveImpacts := prefs.AlertImpacts
		if s.alertFilter != nil {
			effectiveCurrencies, effectiveImpacts = s.alertFilter(ctx, userID, prefs.CurrencyFilter, prefs.AlertImpacts)
		}

		alertImpactsLower := toLowerSlice(effectiveImpacts)
		if !toSet(alertImpactsLower)[strings.ToLower(ev.Impact)] {
			continue
		}

		if len(effectiveCurrencies) > 0 && !containsStr(effectiveCurrencies, ev.Currency) {
			continue
		}

		var html string

		// P1.1 — Build confluence alert if COT data is available
		if cotAnalysis != nil {
			html = s.buildConfluenceAlert(ctx, ev, cotAnalysis)
		} else {
			html = s.buildStandardReleaseAlert(ctx, ev, prefs.Language)
		}

		if _, sendErr := s.messenger.SendHTML(ctx, prefs.ChatID, html); sendErr != nil {
			schedLog.Error().Int64("user_id", userID).Err(sendErr).Msg("failed to send release alert")
		}
		time.Sleep(config.TelegramFloodDelay)
	}

	// Real-time conviction update — recompute and broadcast when surprise is significant
	if math.Abs(ev.SurpriseScore) > 0.5 && s.cotRepo != nil {
		contractCode := domain.CurrencyToContract(ev.Currency)
		if contractCode != "" {
			cotAn, cotErr := s.cotRepo.GetLatestAnalysis(ctx, contractCode)
			if cotErr == nil && cotAn != nil {
				macroD, _ := fred.GetCachedOrFetch(ctx)
				var regime fred.MacroRegime
				if macroD != nil {
					regime = fred.ClassifyMacroRegime(macroD)
				}
				sigmaAcc := s.GetSurpriseSigma(ev.Currency)
				cs := cot.ComputeConvictionScoreV3(*cotAn, regime, sigmaAcc, ev.Event, macroD, nil)

				convHTML := fmt.Sprintf("📊 <b>CONVICTION UPDATE: %s</b>\n", ev.Currency)
				convHTML += fmt.Sprintf("After: <i>%s</i> (%s)\n", ev.Event, ev.SurpriseLabel)
				dirIcon := "⚪"
				if cs.Direction == "LONG" {
					dirIcon = "🟢"
				} else if cs.Direction == "SHORT" {
					dirIcon = "🔴"
				}
				convHTML += fmt.Sprintf("%s Conv: <b>%.0f/100 %s</b>\n", dirIcon, cs.Score, cs.Label)
				convHTML += fmt.Sprintf("<code>COT Bias: %s | FRED: %s</code>\n", cs.COTBias, cs.FREDRegime)
				convHTML += "<i>Real-time update — /rank for full ranking</i>"

				for userID, prefs := range activeUsers {
					if prefs.AlertsEnabled && prefs.ChatID != "" {
						// Explicit ban check
						if s.isBanned != nil && s.isBanned(ctx, userID) {
							continue
						}
						// Apply tier filter for conviction updates too
						effCur := prefs.CurrencyFilter
						if s.alertFilter != nil {
							effCur, _ = s.alertFilter(ctx, userID, prefs.CurrencyFilter, prefs.AlertImpacts)
						}
						if len(effCur) == 0 || containsStr(effCur, ev.Currency) {
							_, _ = s.messenger.SendHTML(ctx, prefs.ChatID, convHTML)
							time.Sleep(config.TelegramFloodDelay)
						}
					}
				}

				// Invalidate news-dependent AI caches
				if s.onNewsInvalidate != nil {
					s.onNewsInvalidate(ctx)
				}
			}
		}
	}
}

// buildConfluenceAlert builds the P1.1 Confluence Alert message.
// Gap A/C: Now accepts ctx to fetch FRED regime and apply regime-adjusted surprise scoring.
func (s *Scheduler) buildConfluenceAlert(ctx context.Context, ev domain.NewsEvent, analysis *domain.COTAnalysis) string {
	// Parse numeric values
	actualVal, hasActual := ParseNumericValue(ev.Actual)
	forecastVal, hasForecast := ParseNumericValue(ev.Forecast)
	previousVal, hasPrevious := ParseNumericValue(ev.Previous)
	oldPreviousVal, hasOldPrevious := ParseNumericValue(ev.OldPrevious)

	var dataDirection string
	var beatMiss string

	// BUG #8 FIX: Use ev.SurpriseScore set by onNewRelease() (stddev-normalized via
	// GetHistoricalSurprises) instead of re-computing a raw pct diff locally.
	// The old local formula (diff / abs(forecast)) produces a different scale than
	// the normalized sigma in ev.SurpriseScore, causing inconsistent labels and
	// wrong values going into recordSurprise() and the COT confluence classifier.
	//
	// We still need to compute `diff` for direction labels (HAWKISH/DOVISH/Beat/Miss),
	// but the sigma value for all scoring uses ev.SurpriseScore.
	surpriseSigma := ev.SurpriseScore // already normalized & ImpactDirection-aware

	if hasActual && hasForecast {
		diff := actualVal - forecastVal

		// Use MQL5 ImpactDirection to validate surprise sign.
		// ImpactDirection: 1 = bullish for currency, 2 = bearish, 0 = neutral.
		// This corrects cases where "higher number = bearish" (e.g., unemployment, CPI miss).
		if ev.ImpactDirection == 1 {
			// MQL5 says bullish — ensure diff is treated as positive signal
			dataDirection = "HAWKISH"
			beatMiss = "Beat"
			if diff < 0 {
				// Inverted indicator (e.g., unemployment rate down = good)
				diff = -diff
			}
		} else if ev.ImpactDirection == 2 {
			// MQL5 says bearish
			dataDirection = "DOVISH"
			beatMiss = "Miss"
			if diff > 0 {
				// Inverted indicator
				diff = -diff
			}
		} else {
			// ImpactDirection = 0 (neutral/unknown): fall back to raw diff
			if diff > 0 {
				dataDirection = "HAWKISH"
				beatMiss = "Beat"
			} else if diff < 0 {
				dataDirection = "DOVISH"
				beatMiss = "Miss"
			} else {
				dataDirection = "NEUTRAL"
				beatMiss = "In Line"
			}
		}

		// If ev.SurpriseScore was not set (e.g. onNewRelease not called, or history empty
		// and raw diff was zero), fall back to raw pct diff for this function only.
		if surpriseSigma == 0 && diff != 0 {
			if forecastVal != 0 {
				surpriseSigma = diff / math.Abs(forecastVal)
			} else {
				surpriseSigma = diff
			}
		}
	} else {
		dataDirection = "NEUTRAL"
		beatMiss = "Actual"
	}

	// Revision tracking: if OldPrevious exists and differs from Previous, add revision component.
	// Note: revision sigma is additive on top of the normalized surprise score.
	if hasOldPrevious && hasPrevious {
		revDiff := previousVal - oldPreviousVal
		if math.Abs(revDiff) > 0.001 {
			var revSigma float64
			if oldPreviousVal != 0 {
				revSigma = revDiff / math.Abs(oldPreviousVal)
			} else {
				revSigma = revDiff
			}
			// Apply revision as additive component (20% weight)
			surpriseSigma += revSigma * 0.2
		}
	}

	// Record surprise sigma in the weekly per-currency accumulator.
	// Now uses the stddev-normalized value from ev.SurpriseScore (BUG #8 fix).
	if surpriseSigma != 0 {
		s.recordSurprise(ev.Currency, surpriseSigma)
	}

	// Gap A/C — Fetch FRED macro data for regime-adjusted scoring (best-effort, non-fatal)
	var macroData *fred.MacroData
	var adjustedSigma = surpriseSigma
	var regimeAdjLabel string

	if md, fredErr := fred.GetCachedOrFetch(ctx); fredErr == nil && md != nil {
		macroData = md
		regime := fred.ClassifyMacroRegime(md)

		// Gap C: filter surprise sigma through FRED regime context
		adjustedSigma = cot.AdjustSurpriseByFREDContext(ev.Currency, surpriseSigma, regime)

		// Gap A: compute regime-adjusted sentiment label
		surpriseRecords := []domain.SurpriseRecord{{
			Currency:   ev.Currency,
			EventName:  ev.Event,
			SigmaValue: adjustedSigma,
		}}
		regimeAdjLabel = cot.AdjustSentimentBySurprise(*analysis, surpriseRecords, macroData)
	}

	// Suppress unused variable warning
	_ = hasOldPrevious

	cotBullish := analysis.SentimentScore > 0
	cotBias := "BULLISH"
	if !cotBullish {
		cotBias = "BEARISH"
	}

	// Use adjusted sigma for confluence classification
	effectiveSigma := adjustedSigma

	var confluenceType string
	var confluenceIcon string
	var insight string

	if effectiveSigma > 0.1 && cotBullish {
		confluenceType = "CONFLUENCE"
		confluenceIcon = "🟢"
		insight = "Smart money dan data sepakat — " + cotBias + ". Trend continuation likely"
	} else if effectiveSigma < -0.1 && !cotBullish {
		confluenceType = "CONFLUENCE"
		confluenceIcon = "🟢"
		insight = "Smart money dan data sepakat — " + cotBias + ". Trend continuation likely"
	} else if effectiveSigma > 0.1 && !cotBullish {
		confluenceType = "DIVERGENCE"
		confluenceIcon = "🔴"
		insight = "Smart money SHORT tapi data HAWKISH → Watch: short squeeze potensial, hati-hati counter-trend"
	} else if effectiveSigma < -0.1 && cotBullish {
		confluenceType = "DIVERGENCE"
		confluenceIcon = "🔴"
		insight = "Smart money LONG tapi data DOVISH → Watch: long liquidation potensial, hati-hati counter-trend"
	} else {
		confluenceType = "NEUTRAL"
		confluenceIcon = "⚪"
		insight = "Sinyal mixed — tunggu konfirmasi lebih lanjut"
	}

	netK := analysis.NetPosition / 1000

	html := fmt.Sprintf("⚡ <b>%s</b> — %s\n", ev.Event, ev.Currency)
	html += fmt.Sprintf("✅ Actual: <b>%s</b> | %s Forecast %s → <b>%s</b>\n",
		ev.Actual, beatMiss, ev.Forecast, dataDirection)
	html += fmt.Sprintf("%s <b>%s</b>: COT %s <b>%s</b> (Spec Net %.0fK, idx %.0f%%)\n",
		confluenceIcon, confluenceType, ev.Currency, cotBias, netK, analysis.COTIndex)
	html += fmt.Sprintf("→ %s\n", insight)

	// Gap A — show regime-adjusted bias if available
	if regimeAdjLabel != "" && macroData != nil {
		html += fmt.Sprintf("📐 Regime-Adj Bias: <b>%s</b>\n", regimeAdjLabel)
	}

	return html
}

// buildStandardReleaseAlert builds the standard release alert (fallback without COT data).
func (s *Scheduler) buildStandardReleaseAlert(ctx context.Context, ev domain.NewsEvent, language string) string {
	analysisStr := ""
	if s.aiAnalyzer != nil && s.aiAnalyzer.IsAvailable() {
		analysisStr, _ = s.aiAnalyzer.AnalyzeActualRelease(ctx, ev, language)
	}

	// BUG #7 FIX: Respect ImpactDirection when determining arrow color.
	// Without this, inverted indicators (unemployment, CPI, trade deficit) show
	// the wrong color: actual > forecast looks "green" even when it's bearish.
	direction := "⚪"
	if ev.Actual != "" && ev.Forecast != "" && ev.Actual != ev.Forecast {
		actualVal, aOk := ParseNumericValue(ev.Actual)
		forecastVal, fOk := ParseNumericValue(ev.Forecast)
		if aOk && fOk {
			diff := actualVal - forecastVal
			// Apply ImpactDirection: dir=2 (bearish when higher) → flip sign.
			// dir=1 (bullish when higher) → also flip if negative (inverted indicator).
			// dir=0 (neutral) → use raw diff as-is.
			if ev.ImpactDirection == 2 && diff > 0 {
				diff = -diff
			} else if ev.ImpactDirection == 1 && diff < 0 {
				diff = -diff
			}
			if diff > 0 {
				direction = "🟢"
			} else {
				direction = "🔴"
			}
		}
	}

	html := fmt.Sprintf("📈 <b>News Actual Release!</b>\n\n%s <b>%s</b>\n", ev.FormatImpactColor(), ev.Event)
	html += fmt.Sprintf("Currency: <b>%s</b>\n", ev.Currency)
	html += fmt.Sprintf("Actual: <b>%s %s</b> (Forecast: %s / Prev: %s)\n", ev.Actual, direction, ev.Forecast, ev.Previous)

	if analysisStr != "" {
		html += fmt.Sprintf("\n💡 <b>AI Analysis:</b>\n%s", analysisStr)
	}

	return html
}

// ---------------------------------------------------------------------------
// Per-Currency Surprise Accumulator
// ---------------------------------------------------------------------------

// recordSurprise adds a sigma value to the current-week accumulator for a currency.
// The accumulator auto-resets on each new ISO week so stale surprises never carry over.
func (s *Scheduler) recordSurprise(currency string, sigma float64) {
	now := timeutil.NowWIB()
	year, week := now.ISOWeek()
	weekKey := fmt.Sprintf("%d%02d:%s", year, week, currency)
	globalKey := fmt.Sprintf("%d%02d", year, week)

	s.surpriseAccMu.Lock()
	defer s.surpriseAccMu.Unlock()

	// Auto-reset on new ISO week
	if s.surpriseWeek != globalKey {
		s.surpriseAccum = make(map[string]float64)
		s.surpriseWeek = globalKey
	}
	s.surpriseAccum[weekKey] += sigma
}

// GetSurpriseSigma returns the accumulated surprise sigma for a currency this week.
// Returns 0.0 if no surprises have been recorded yet for that currency.
func (s *Scheduler) GetSurpriseSigma(currency string) float64 {
	now := timeutil.NowWIB()
	year, week := now.ISOWeek()
	weekKey := fmt.Sprintf("%d%02d:%s", year, week, currency)

	s.surpriseAccMu.RLock()
	defer s.surpriseAccMu.RUnlock()
	return s.surpriseAccum[weekKey]
}

// ---------------------------------------------------------------------------
// Initial Sync
// ---------------------------------------------------------------------------

func (s *Scheduler) runInitialSync(ctx context.Context) {
	now := timeutil.NowWIB()
	dateStr := now.Format("20060102")

	events, _ := s.repo.GetByDate(ctx, dateStr)
	if len(events) > 0 {
		schedLog.Info().Msg("initial sync skipped: data already exists for today")
		schedLog.Info().Msg("running startup missed-actuals check")
		s.triggerMicroScrape(ctx, dateStr, "startup-check")
		return
	}

	schedLog.Info().Msg("triggering initial sync scrape for current week")
	newEvents, err := s.fetcher.ScrapeCalendar(ctx, "this")
	if err != nil {
		schedLog.Error().Err(err).Msg("initial sync failed")
		return
	}

	if err := s.repo.SaveEvents(ctx, newEvents); err != nil {
		schedLog.Error().Err(err).Msg("failed to save initial events")
	} else {
		schedLog.Info().Int("events", len(newEvents)).Msg("initial sync successful")
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
