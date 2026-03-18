package news

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/internal/ports"
	"github.com/arkcode369/ff-calendar-bot/internal/service/cot"
	"github.com/arkcode369/ff-calendar-bot/internal/service/fred"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

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
// P1.2: Includes Storm Day warning if 3+ high-impact events detected.
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

	// P1.2 — Build Storm Day info once (same for all users)
	stormWarning := s.buildStormDayWarning(events, now)

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

		// P1.2 — Append Storm Day warning if applicable
		if stormWarning != "" {
			html += "\n\n" + stormWarning
		}

		if _, sendErr := s.messenger.SendHTML(ctx, prefs.ChatID, html); sendErr != nil {
			log.Printf("[NEWS SCHEDULER] Failed to send daily reminder to user %d: %v", userID, sendErr)
		}
		time.Sleep(50 * time.Millisecond) // Avoid Telegram flood
	}
}

// buildStormDayWarning detects if today is a Storm Day (3+ high-impact events).
// Returns the formatted HTML warning string, or "" if not a storm day.
//
// P1.2 — Storm Day Detection
func (s *Scheduler) buildStormDayWarning(events []domain.NewsEvent, now time.Time) string {
	var highEvents []domain.NewsEvent
	currencySet := make(map[string]bool)

	for _, e := range events {
		if strings.ToLower(e.Impact) == "high" {
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
		shownEvents = append(shownEvents[:4], fmt.Sprintf("+%d more", len(eventNames)-4))
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

		minsUntil := int(e.TimeWIB.Sub(now).Minutes())
		if minsUntil < 0 || minsUntil > 120 {
			continue // Not in relevant window
		}

		for userID, prefs := range activeUsers {
			if !prefs.AlertsEnabled || prefs.ChatID == "" {
				continue
			}

			if !containsInt(prefs.AlertMinutes, minsUntil) {
				continue
			}

			alertImpactsLower := toLowerSlice(prefs.AlertImpacts)
			if !toSet(alertImpactsLower)[strings.ToLower(e.Impact)] {
				continue
			}

			if len(prefs.CurrencyFilter) > 0 && !containsStr(prefs.CurrencyFilter, e.Currency) {
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
			continue
		}

		minsSinceRelease := int(now.Sub(e.TimeWIB).Minutes())

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

// ---------------------------------------------------------------------------
// onNewRelease — P1.1 Confluence Alert
// ---------------------------------------------------------------------------

// onNewRelease broadcasts an actual-release alert to all eligible users.
// P1.1: Cross-checks the release against COT positioning for the same currency.
func (s *Scheduler) onNewRelease(ctx context.Context, ev domain.NewsEvent) {
	log.Printf("[NEWS SCHEDULER] New Release Detected: %s %s -> %s", ev.Currency, ev.Event, ev.Actual)

	activeUsers, err := s.prefsRepo.GetAllActive(ctx)
	if err != nil {
		log.Printf("[NEWS SCHEDULER] onNewRelease: get users failed: %v", err)
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
		// Use ImpactDirection-aware surprise computation
		ev.SurpriseScore = ComputeSurpriseWithDirection(actualVal, forecastVal, nil, ev.ImpactDirection)
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
			log.Printf("[NEWS SCHEDULER] Failed to save revision for %s %s: %v", ev.Currency, ev.Event, saveErr)
		}
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

		var html string

		// P1.1 — Build confluence alert if COT data is available
		if cotAnalysis != nil {
			html = s.buildConfluenceAlert(ctx, ev, cotAnalysis)
		} else {
			html = s.buildStandardReleaseAlert(ctx, ev, prefs.Language)
		}

		if _, sendErr := s.messenger.SendHTML(ctx, prefs.ChatID, html); sendErr != nil {
			log.Printf("[NEWS SCHEDULER] Failed to send release alert to user %d: %v", userID, sendErr)
		}
		time.Sleep(50 * time.Millisecond)
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
				cs := cot.ComputeConvictionScore(*cotAn, regime, sigmaAcc, ev.Event, macroD)

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

				for _, prefs := range activeUsers {
					if prefs.AlertsEnabled && prefs.ChatID != "" {
						if len(prefs.CurrencyFilter) == 0 || containsStr(prefs.CurrencyFilter, ev.Currency) {
							_, _ = s.messenger.SendHTML(ctx, prefs.ChatID, convHTML)
							time.Sleep(50 * time.Millisecond)
						}
					}
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
	surpriseSigma := 0.0

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

		if forecastVal != 0 {
			surpriseSigma = diff / math.Abs(forecastVal)
		} else {
			surpriseSigma = diff
		}
	} else {
		dataDirection = "NEUTRAL"
		beatMiss = "Actual"
	}

	// Revision tracking: if OldPrevious exists and differs from Previous, compute revision surprise
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

	// Record raw surprise sigma in the weekly per-currency accumulator (non-fatal)
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
		log.Println("[NEWS SCHEDULER] Initial sync skipped: data already exists for today.")
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
