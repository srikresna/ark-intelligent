package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/pkg/timeutil"
)

// briefingMu guards briefingLastSentDay to prevent double-sends on the same day.
var (
	briefingMu          sync.Mutex
	briefingLastSentDay string // format: "2006-01-02"
)

// jobDailyBriefing fires the morning briefing push at ~06:00 WIB.
// It runs on a 30-minute tick but only pushes once per calendar day (WIB).
// The push is skipped on weekends — no relevant market events on Sat/Sun.
func (s *Scheduler) jobDailyBriefing(ctx context.Context) error {
	if s.deps.DailyBriefing == nil {
		return nil
	}

	now := timeutil.NowWIB()

	// Weekends: skip — no meaningful market events
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return nil
	}

	// Only fire between 06:00–07:00 WIB
	hour := now.Hour()
	if hour < 6 || hour >= 7 {
		return nil
	}

	todayKey := now.Format("2006-01-02")

	briefingMu.Lock()
	if briefingLastSentDay == todayKey {
		briefingMu.Unlock()
		return nil // Already sent today
	}
	briefingLastSentDay = todayKey
	briefingMu.Unlock()

	// Get all active users with briefing alerts enabled
	activeUsers, err := s.deps.PrefsRepo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("daily briefing: failed to get active users")
		// Reset so we can retry on next tick
		briefingMu.Lock()
		briefingLastSentDay = ""
		briefingMu.Unlock()
		return err
	}

	count := 0
	for userID, prefs := range activeUsers {
		if !prefs.COTAlertsEnabled || prefs.ChatID == "" {
			continue
		}
		if s.deps.IsBanned != nil && s.deps.IsBanned(ctx, userID) {
			continue
		}
		if s.deps.DailyBriefing(ctx, prefs.ChatID) {
			count++
		}
		time.Sleep(config.TelegramFloodDelay)
	}

	log.Info().
		Int("users", count).
		Str("date", todayKey).
		Msg("daily briefing push sent")

	return nil
}
