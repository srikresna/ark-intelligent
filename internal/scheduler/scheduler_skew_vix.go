package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/service/fred"
	vixsvc "github.com/arkcode369/ark-intelligent/internal/service/vix"
)

// checkSKEWVIXAlert fetches VIX term structure (which includes the VolSuite
// with SKEW data) and broadcasts an alert when the tail risk state transitions
// between NORMAL, ELEVATED, and EXTREME.
//
// Called asynchronously from jobFREDAlerts to piggyback on the hourly cadence
// without blocking FRED alert processing.
func (s *Scheduler) checkSKEWVIXAlert(parentCtx context.Context) {
	vixCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts, err := vixsvc.FetchTermStructure(vixCtx)
	if err != nil || ts == nil || ts.VolSuite == nil || !ts.VolSuite.Available {
		return
	}

	vs := ts.VolSuite

	s.fredMu.Lock()
	prevTailRisk := s.lastTailRisk
	s.lastTailRisk = vs.TailRisk
	s.fredMu.Unlock()

	// Only alert on state transitions (not every hour).
	// Skip the first run (prevTailRisk == "") to establish baseline.
	if prevTailRisk == vs.TailRisk || prevTailRisk == "" {
		return
	}

	// Transition back to NORMAL — send all-clear if coming from alert state
	if !vs.ShouldAlert() {
		if prevTailRisk == "EXTREME" || prevTailRisk == "ELEVATED" {
			allClear := fred.MacroAlert{
				Type:  fred.AlertSKEWVIXNormal,
				Title: "\U0001f7e2 TAIL RISK NORMALIZED — SKEW/VIX ratio returned to normal range",
				Description: fmt.Sprintf(
					"SKEW: %.1f | SKEW/VIX ratio: %.1f\n"+
						"Previous state: %s → NORMAL\n\n"+
						"Tail risk has subsided. Options market no longer pricing extreme downside.",
					vs.SKEW, vs.SKEWVIXRatio, prevTailRisk),
				Severity: "LOW",
				Value:    vs.SKEWVIXRatio,
			}
			msg := fred.FormatMacroAlert(allClear)
			s.broadcastToActiveUsers(context.Background(), msg)
		}
		return
	}

	// Transition to ELEVATED or EXTREME — send alert
	alertType := fred.AlertSKEWVIXElevated
	if vs.TailRisk == "EXTREME" {
		alertType = fred.AlertSKEWVIXExtreme
	}
	skewAlert := fred.MacroAlert{
		Type:        alertType,
		Title:       vs.AlertSummary(),
		Description: vs.FormatAlertDetail(ts.Spot),
		Severity:    vs.AlertLevel(),
		Value:       vs.SKEWVIXRatio,
	}
	msg := fred.FormatMacroAlert(skewAlert)
	s.broadcastToActiveUsers(context.Background(), msg)
	log.Info().
		Str("tail_risk", vs.TailRisk).
		Float64("skew_vix_ratio", vs.SKEWVIXRatio).
		Float64("skew", vs.SKEW).
		Float64("vix_spot", ts.Spot).
		Msg("SKEW/VIX tail risk alert broadcast")
}

// broadcastToActiveUsers sends a message to all active premium users.
// Reuses the same broadcast pattern as jobFREDAlerts.
func (s *Scheduler) broadcastToActiveUsers(ctx context.Context, msg string) {
	activeUsers, err := s.deps.PrefsRepo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("broadcastToActiveUsers: failed to get active users")
		return
	}

	count := 0
	for userID, prefs := range activeUsers {
		if !prefs.COTAlertsEnabled || prefs.ChatID == "" {
			continue
		}
		if s.deps.IsBanned != nil && s.deps.IsBanned(ctx, userID) {
			continue
		}
		if s.deps.FREDAlertCheck != nil && !s.deps.FREDAlertCheck(ctx, userID) {
			continue
		}
		if _, sendErr := s.deps.Bot.SendHTML(ctx, prefs.ChatID, msg); sendErr == nil {
			count++
		}
		time.Sleep(config.TelegramFloodDelay)
	}
	log.Info().Int("users", count).Msg("broadcast sent to active users")
}
