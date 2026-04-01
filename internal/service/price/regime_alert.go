package price

import (
	"fmt"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Proactive Regime Change Alert System
// ---------------------------------------------------------------------------
//
// Leverages the existing HMM TransitionWarning to detect regime shifts.
// Tracks regime duration, implements tiered alerts (AMBER/RED), and
// supports multi-asset divergence detection.
//
// Alert tiers:
//   🟡 AMBER: P(CRISIS) > 20% — early warning
//   🔴 RED:   P(CRISIS) > 50% or regime flip confirmed via Viterbi
//
// Cooldown: max 1 alert per regime transition event per asset.

// AlertTier represents the severity of a regime change alert.
type AlertTier string

const (
	AlertTierNone  AlertTier = "NONE"
	AlertTierAmber AlertTier = "AMBER"
	AlertTierRed   AlertTier = "RED"
)

// RegimeState tracks the current HMM regime and its history for one asset.
type RegimeState struct {
	Symbol           string    `json:"symbol"`
	ContractCode     string    `json:"contract_code"`
	CurrentState     string    `json:"current_state"`       // RISK_ON, RISK_OFF, CRISIS
	RegimeStartDate  time.Time `json:"regime_start_date"`   // When this regime started
	DaysInRegime     int       `json:"days_in_regime"`      // Days in current regime
	MeanDuration     float64   `json:"mean_duration"`       // Historical mean regime duration
	PreviousState    string    `json:"previous_state"`      // State before current
	CrisisProb       float64   `json:"crisis_prob"`         // Current P(CRISIS)
	AlertTier        AlertTier `json:"alert_tier"`          // Current alert tier
	LastAlertTime    time.Time `json:"last_alert_time"`     // For cooldown
	LastAlertTier    AlertTier `json:"last_alert_tier"`     // Last tier we alerted on
	StateProbabilities [3]float64 `json:"state_probabilities"` // [RISK_ON, RISK_OFF, CRISIS]
	UpdatedAt        time.Time `json:"updated_at"`
}

// RegimeAlert is emitted when a regime transition is detected.
type RegimeAlert struct {
	Symbol       string    `json:"symbol"`
	ContractCode string    `json:"contract_code"`
	Tier         AlertTier `json:"tier"`
	PrevState    string    `json:"prev_state"`
	NewState     string    `json:"new_state"`
	CrisisProb   float64   `json:"crisis_prob"`
	DaysInRegime int       `json:"days_in_regime"`
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message"`
}

// RegimeAlertEngine tracks regime states across multiple assets and generates alerts.
type RegimeAlertEngine struct {
	mu     sync.RWMutex
	states map[string]*RegimeState // contractCode → state
}

// NewRegimeAlertEngine creates a new regime alert tracker.
func NewRegimeAlertEngine() *RegimeAlertEngine {
	return &RegimeAlertEngine{
		states: make(map[string]*RegimeState),
	}
}

// AlertCooldown is the minimum time between alerts for the same asset.
const AlertCooldown = 4 * time.Hour

// Update runs HMM on the given prices and returns an alert if a tier change occurred.
// Prices must be newest-first (as returned by DailyPriceStore).
// Returns nil alert if no notification is needed (no change or cooldown active).
func (e *RegimeAlertEngine) Update(contractCode, symbol string, prices []domain.PriceRecord) (*RegimeAlert, error) {
	result, err := EstimateHMMRegime(prices)
	if err != nil {
		return nil, fmt.Errorf("HMM for %s: %w", symbol, err)
	}

	now := time.Now()
	crisisProb := result.StateProbabilities[2] // Index 2 = CRISIS

	// Determine alert tier
	tier := classifyTier(result)

	e.mu.Lock()
	defer e.mu.Unlock()

	prev, exists := e.states[contractCode]
	if !exists {
		// First observation — initialize without alerting
		e.states[contractCode] = &RegimeState{
			Symbol:             symbol,
			ContractCode:       contractCode,
			CurrentState:       result.CurrentState,
			RegimeStartDate:    now,
			DaysInRegime:       0,
			PreviousState:      "",
			CrisisProb:         crisisProb,
			AlertTier:          tier,
			LastAlertTime:      time.Time{},
			LastAlertTier:      AlertTierNone,
			StateProbabilities: result.StateProbabilities,
			UpdatedAt:          now,
		}
		return nil, nil
	}

	// Update days in regime
	daysInRegime := int(now.Sub(prev.RegimeStartDate).Hours() / 24)

	// Detect regime flip
	regimeFlipped := result.CurrentState != prev.CurrentState

	if regimeFlipped {
		// Mean duration tracking (simple exponential moving average)
		if prev.MeanDuration == 0 {
			prev.MeanDuration = float64(daysInRegime)
		} else {
			prev.MeanDuration = prev.MeanDuration*0.7 + float64(daysInRegime)*0.3
		}
	}

	// Update state
	state := &RegimeState{
		Symbol:             symbol,
		ContractCode:       contractCode,
		CurrentState:       result.CurrentState,
		PreviousState:      prev.CurrentState,
		CrisisProb:         crisisProb,
		AlertTier:          tier,
		StateProbabilities: result.StateProbabilities,
		MeanDuration:       prev.MeanDuration,
		UpdatedAt:          now,
		// Carry forward alert tracking
		LastAlertTime: prev.LastAlertTime,
		LastAlertTier: prev.LastAlertTier,
	}

	if regimeFlipped {
		state.RegimeStartDate = now
		state.DaysInRegime = 0
	} else {
		state.RegimeStartDate = prev.RegimeStartDate
		state.DaysInRegime = daysInRegime
	}

	e.states[contractCode] = state

	// Determine if we should fire an alert
	alert := e.shouldAlert(state, prev, regimeFlipped, now)
	if alert != nil {
		state.LastAlertTime = now
		state.LastAlertTier = alert.Tier
	}

	return alert, nil
}

// classifyTier determines alert tier from HMM result.
func classifyTier(result *HMMResult) AlertTier {
	crisisProb := result.StateProbabilities[2]

	// RED: confirmed crisis (Viterbi shows CRISIS) or P(CRISIS) > 50%
	if result.CurrentState == HMMCrisis || crisisProb > 0.50 {
		return AlertTierRed
	}

	// AMBER: elevated crisis probability
	if crisisProb > 0.20 {
		return AlertTierAmber
	}

	return AlertTierNone
}

// shouldAlert checks if an alert should fire based on tier changes and cooldown.
func (e *RegimeAlertEngine) shouldAlert(current, prev *RegimeState, flipped bool, now time.Time) *RegimeAlert {
	// Cooldown check: skip if we recently alerted at the same or higher tier
	if !prev.LastAlertTime.IsZero() && now.Sub(prev.LastAlertTime) < AlertCooldown {
		// Allow escalation (AMBER → RED) even during cooldown
		if current.AlertTier != AlertTierRed || prev.LastAlertTier == AlertTierRed {
			return nil
		}
	}

	// Case 1: Regime flipped — always alert if entering CRISIS or leaving CRISIS
	if flipped {
		if current.CurrentState == HMMCrisis {
			return &RegimeAlert{
				Symbol:       current.Symbol,
				ContractCode: current.ContractCode,
				Tier:         AlertTierRed,
				PrevState:    prev.CurrentState,
				NewState:     current.CurrentState,
				CrisisProb:   current.CrisisProb,
				DaysInRegime: prev.DaysInRegime,
				Timestamp:    now,
				Message:      fmt.Sprintf("🔴 REGIME CHANGE: %s shifted from %s → CRISIS (P=%.0f%%)", current.Symbol, prev.CurrentState, current.CrisisProb*100),
			}
		}
		if prev.CurrentState == HMMCrisis {
			// Recovery from crisis — all-clear
			return &RegimeAlert{
				Symbol:       current.Symbol,
				ContractCode: current.ContractCode,
				Tier:         AlertTierNone,
				PrevState:    prev.CurrentState,
				NewState:     current.CurrentState,
				CrisisProb:   current.CrisisProb,
				DaysInRegime: 0,
				Timestamp:    now,
				Message:      fmt.Sprintf("🟢 REGIME RECOVERY: %s shifted from CRISIS → %s (P(crisis)=%.0f%%)", current.Symbol, current.CurrentState, current.CrisisProb*100),
			}
		}
		// Non-crisis flip (e.g. RISK_ON → RISK_OFF)
		if current.AlertTier != AlertTierNone {
			return &RegimeAlert{
				Symbol:       current.Symbol,
				ContractCode: current.ContractCode,
				Tier:         current.AlertTier,
				PrevState:    prev.CurrentState,
				NewState:     current.CurrentState,
				CrisisProb:   current.CrisisProb,
				DaysInRegime: prev.DaysInRegime,
				Timestamp:    now,
				Message:      fmt.Sprintf("🟡 REGIME SHIFT: %s moved from %s → %s (P(crisis)=%.0f%%)", current.Symbol, prev.CurrentState, current.CurrentState, current.CrisisProb*100),
			}
		}
	}

	// Case 2: Tier escalation without flip (e.g. AMBER → RED)
	if current.AlertTier == AlertTierRed && prev.AlertTier != AlertTierRed {
		return &RegimeAlert{
			Symbol:       current.Symbol,
			ContractCode: current.ContractCode,
			Tier:         AlertTierRed,
			PrevState:    current.CurrentState,
			NewState:     current.CurrentState,
			CrisisProb:   current.CrisisProb,
			DaysInRegime: current.DaysInRegime,
			Timestamp:    now,
			Message:      fmt.Sprintf("🔴 CRISIS WARNING ELEVATED: %s P(crisis)=%.0f%% — regime: %s", current.Symbol, current.CrisisProb*100, current.CurrentState),
		}
	}

	// Case 3: New AMBER alert (was NONE before)
	if current.AlertTier == AlertTierAmber && prev.AlertTier == AlertTierNone {
		return &RegimeAlert{
			Symbol:       current.Symbol,
			ContractCode: current.ContractCode,
			Tier:         AlertTierAmber,
			PrevState:    current.CurrentState,
			NewState:     current.CurrentState,
			CrisisProb:   current.CrisisProb,
			DaysInRegime: current.DaysInRegime,
			Timestamp:    now,
			Message:      fmt.Sprintf("🟡 REGIME WARNING: %s P(crisis)=%.0f%% — elevated risk detected", current.Symbol, current.CrisisProb*100),
		}
	}

	return nil
}

// GetState returns the current regime state for an asset. Thread-safe.
func (e *RegimeAlertEngine) GetState(contractCode string) *RegimeState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if s, ok := e.states[contractCode]; ok {
		cp := *s
		return &cp
	}
	return nil
}

// GetAllStates returns a snapshot of all tracked regime states. Thread-safe.
func (e *RegimeAlertEngine) GetAllStates() []*RegimeState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*RegimeState, 0, len(e.states))
	for _, s := range e.states {
		cp := *s
		out = append(out, &cp)
	}
	return out
}

// DetectDivergence checks if BTC and ETH are in different regimes (multi-asset divergence).
// Returns a description of the divergence, or empty string if aligned.
func (e *RegimeAlertEngine) DetectDivergence() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	btc := e.states["133741"] // BTC contract code
	eth := e.states["146021"] // ETH contract code

	if btc == nil || eth == nil {
		return ""
	}

	if btc.CurrentState != eth.CurrentState {
		return fmt.Sprintf("⚠️ Crypto divergence: BTC=%s vs ETH=%s", btc.CurrentState, eth.CurrentState)
	}
	return ""
}
