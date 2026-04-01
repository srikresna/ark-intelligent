package scheduler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/config"
	"github.com/arkcode369/ark-intelligent/internal/domain"
	pricesvc "github.com/arkcode369/ark-intelligent/internal/service/price"
)

// regimeEngine is the proactive regime alert engine. Initialized lazily on first check.
// Protected by regimeMu.
var (
	regimeEngine *pricesvc.RegimeAlertEngine
)

// regimeMonitoredAssets returns the key assets to track for proactive regime alerts.
// We monitor major FX pairs, metals, crypto, and equity indices.
func regimeMonitoredAssets() []domain.PriceSymbolMapping {
	codes := []string{
		"099741", // EUR
		"096742", // GBP
		"097741", // JPY
		"088691", // XAU (Gold)
		"067651", // OIL
		"133741", // BTC
		"146021", // ETH
		"13874A", // SPX500
	}
	var out []domain.PriceSymbolMapping
	for _, code := range codes {
		if m := domain.FindPriceMapping(code); m != nil {
			out = append(out, *m)
		}
	}
	return out
}

// jobRegimeAlert checks HMM regime state for monitored assets and broadcasts
// proactive alerts when regime transitions are detected.
func (s *Scheduler) jobRegimeAlert(ctx context.Context) error {
	if s.deps.DailyPriceRepo == nil {
		return nil // Daily price data required
	}

	// Lazy init
	s.regimeMu.Lock()
	if regimeEngine == nil {
		regimeEngine = pricesvc.NewRegimeAlertEngine()
	}
	s.regimeMu.Unlock()

	assets := regimeMonitoredAssets()
	var alerts []*pricesvc.RegimeAlert

	for _, asset := range assets {
		dailyPrices, err := s.deps.DailyPriceRepo.GetDailyHistory(ctx, asset.ContractCode, 120)
		if err != nil || len(dailyPrices) < 60 {
			continue
		}

		// Convert DailyPrice → PriceRecord (newest-first)
		records := make([]domain.PriceRecord, len(dailyPrices))
		for i, d := range dailyPrices {
			records[i] = domain.PriceRecord{
				ContractCode: d.ContractCode,
				Symbol:       d.Symbol,
				Date:         d.Date,
				Open:         d.Open,
				High:         d.High,
				Low:          d.Low,
				Close:        d.Close,
				Volume:       d.Volume,
				Source:       d.Source,
			}
		}

		alert, err := regimeEngine.Update(asset.ContractCode, asset.Currency, records)
		if err != nil {
			log.Warn().Err(err).Str("symbol", asset.Currency).Msg("regime alert check failed")
			continue
		}
		if alert != nil {
			alerts = append(alerts, alert)
		}
	}

	if len(alerts) == 0 {
		return nil
	}

	// Check for multi-asset divergence
	divergence := regimeEngine.DetectDivergence()

	// Format and broadcast
	msg := formatRegimeAlerts(alerts, divergence)
	s.broadcastRegimeAlert(ctx, msg)

	for _, a := range alerts {
		log.Info().
			Str("symbol", a.Symbol).
			Str("tier", string(a.Tier)).
			Str("prev", a.PrevState).
			Str("new", a.NewState).
			Float64("crisis_prob", a.CrisisProb).
			Msg("regime alert broadcast")
	}

	return nil
}

// formatRegimeAlerts formats multiple regime alerts into a single HTML message.
func formatRegimeAlerts(alerts []*pricesvc.RegimeAlert, divergence string) string {
	var sb strings.Builder
	sb.WriteString("📊 <b>REGIME CHANGE ALERT</b>\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")

	for _, a := range alerts {
		sb.WriteString(a.Message)
		sb.WriteString("\n")
		if a.DaysInRegime > 0 {
			sb.WriteString(fmt.Sprintf("   Duration in previous regime: %dd\n", a.DaysInRegime))
		}
		sb.WriteString("\n")
	}

	if divergence != "" {
		sb.WriteString(divergence)
		sb.WriteString("\n\n")
	}

	sb.WriteString("<i>Use /regime for full regime dashboard</i>")
	return sb.String()
}

// broadcastRegimeAlert sends a regime alert to all active subscribed users.
func (s *Scheduler) broadcastRegimeAlert(ctx context.Context, msg string) {
	activeUsers, err := s.deps.PrefsRepo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("regime alert: failed to get active users")
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
		if _, sendErr := s.deps.Bot.SendHTML(ctx, prefs.ChatID, msg); sendErr == nil {
			count++
		}
		time.Sleep(config.TelegramFloodDelay)
	}
	log.Info().Int("users", count).Msg("regime alert broadcast sent")
}

// GetRegimeStates returns the current regime states for the /regime command.
// Returns nil if the engine hasn't been initialized yet.
func (s *Scheduler) GetRegimeStates() []*pricesvc.RegimeState {
	s.regimeMu.RLock()
	defer s.regimeMu.RUnlock()
	if regimeEngine == nil {
		return nil
	}
	return regimeEngine.GetAllStates()
}

// GetRegimeDivergence returns any multi-asset divergence detected.
func (s *Scheduler) GetRegimeDivergence() string {
	s.regimeMu.RLock()
	defer s.regimeMu.RUnlock()
	if regimeEngine == nil {
		return ""
	}
	return regimeEngine.DetectDivergence()
}

// FormatRegimeDashboard formats the full regime dashboard for the /regime command.
func FormatRegimeDashboard(states []*pricesvc.RegimeState, divergence string) string {
	if len(states) == 0 {
		return "📊 <b>REGIME MONITOR</b>\n\n<i>No regime data available yet. Data will appear after the next scheduled check.</i>"
	}

	var sb strings.Builder
	sb.WriteString("📊 <b>REGIME MONITOR</b>\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")

	// Sort by alert tier (RED first, then AMBER, then NONE), then by symbol
	sort.Slice(states, func(i, j int) bool {
		ti := tierOrder(states[i].AlertTier)
		tj := tierOrder(states[j].AlertTier)
		if ti != tj {
			return ti < tj
		}
		return states[i].Symbol < states[j].Symbol
	})

	for _, s := range states {
		icon := stateIcon(s.CurrentState)
		tierBadge := tierBadge(s.AlertTier)

		sb.WriteString(fmt.Sprintf("<b>%s %s</b>", icon, s.Symbol))
		if tierBadge != "" {
			sb.WriteString(" " + tierBadge)
		}
		sb.WriteString("\n")

		sb.WriteString(fmt.Sprintf("  State: <code>%s</code> (%dd)\n", s.CurrentState, s.DaysInRegime))
		sb.WriteString(fmt.Sprintf("  P: RiskOn=%.0f%% RiskOff=%.0f%% Crisis=%.0f%%\n",
			s.StateProbabilities[0]*100,
			s.StateProbabilities[1]*100,
			s.StateProbabilities[2]*100))

		if s.MeanDuration > 0 {
			sb.WriteString(fmt.Sprintf("  Avg regime duration: %.0fd\n", s.MeanDuration))
		}
		sb.WriteString("\n")
	}

	if divergence != "" {
		sb.WriteString(divergence)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("<i>Updated: %s</i>", time.Now().Format("2006-01-02 15:04 MST")))
	return sb.String()
}

func tierOrder(t pricesvc.AlertTier) int {
	switch t {
	case pricesvc.AlertTierRed:
		return 0
	case pricesvc.AlertTierAmber:
		return 1
	default:
		return 2
	}
}

func stateIcon(state string) string {
	switch state {
	case pricesvc.HMMRiskOn:
		return "🟢"
	case pricesvc.HMMRiskOff:
		return "🟡"
	case pricesvc.HMMCrisis:
		return "🔴"
	default:
		return "⚪"
	}
}

func tierBadge(t pricesvc.AlertTier) string {
	switch t {
	case pricesvc.AlertTierRed:
		return "🔴 <b>RED</b>"
	case pricesvc.AlertTierAmber:
		return "🟡 <b>AMBER</b>"
	default:
		return ""
	}
}
