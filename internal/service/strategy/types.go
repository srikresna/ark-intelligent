// Package strategy implements the regime-aware strategy playbook engine.
// It combines factor rankings, FRED macro regime, COT signals, and price regime
// into actionable trade playbooks with conviction scores.
package strategy

import "time"

// ---------------------------------------------------------------------------
// Playbook Entry
// ---------------------------------------------------------------------------

// PlaybookEntry describes a single trade idea from the strategy engine.
type PlaybookEntry struct {
	ContractCode string
	Currency     string
	Name         string

	Direction   Direction // LONG, SHORT, FLAT
	Conviction  float64   // 0.0 to 1.0 (1.0 = highest confidence)
	ConvLevel   ConvictionLevel

	// Supporting evidence
	FactorScore   float64 // composite factor score [-1, +1]
	COTBias       string  // "BULLISH", "BEARISH", "NEUTRAL"
	RegimeFit     string  // how well trade fits macro regime
	RateDiffBps   float64 // carry in bps

	// Risk info
	VolatilityRegime string // "EXPANDING", "CONTRACTING", "NORMAL"
	IsTransition     bool   // regime transition in progress — reduce size
	TransitionNote   string // e.g. "Regime shifting from RISK_ON to RISK_OFF"

	UpdatedAt time.Time
}

// Direction is the directional bias of a trade.
type Direction string

const (
	DirectionLong  Direction = "LONG"
	DirectionShort Direction = "SHORT"
	DirectionFlat  Direction = "FLAT"
)

// ConvictionLevel maps conviction score to a human label.
type ConvictionLevel string

const (
	ConvictionHigh     ConvictionLevel = "HIGH"
	ConvictionMedium   ConvictionLevel = "MEDIUM"
	ConvictionLow      ConvictionLevel = "LOW"
	ConvictionAvoid    ConvictionLevel = "AVOID"
)

// ConvictionToLevel maps float conviction to a label.
func ConvictionToLevel(c float64) ConvictionLevel {
	switch {
	case c >= 0.70:
		return ConvictionHigh
	case c >= 0.45:
		return ConvictionMedium
	case c >= 0.25:
		return ConvictionLow
	default:
		return ConvictionAvoid
	}
}

// ---------------------------------------------------------------------------
// Portfolio Heat
// ---------------------------------------------------------------------------

// PortfolioHeat measures current aggregate risk exposure.
type PortfolioHeat struct {
	TotalExposure float64 // sum of abs(conviction * direction_sign) across all positions
	LongExposure  float64 // sum of long conviction scores
	ShortExposure float64 // sum of short conviction scores
	NetExposure   float64 // LongExposure - ShortExposure
	HeatLevel     HeatLevel
	ActiveTrades  int
	UpdatedAt     time.Time
}

// HeatLevel describes aggregate risk level.
type HeatLevel string

const (
	HeatCold     HeatLevel = "COLD"     // <30% of max exposure
	HeatWarm     HeatLevel = "WARM"     // 30-60%
	HeatHot      HeatLevel = "HOT"      // 60-80%
	HeatOverheat HeatLevel = "OVERHEAT" // >80% — reduce/flatten
)

// ComputeHeatLevel maps total exposure (0-1 scale) to HeatLevel.
func ComputeHeatLevel(totalExposure float64) HeatLevel {
	switch {
	case totalExposure < 0.30:
		return HeatCold
	case totalExposure < 0.60:
		return HeatWarm
	case totalExposure < 0.80:
		return HeatHot
	default:
		return HeatOverheat
	}
}

// ---------------------------------------------------------------------------
// Transition Warning
// ---------------------------------------------------------------------------

// TransitionWarning signals that the macro regime is shifting.
type TransitionWarning struct {
	IsActive       bool
	FromRegime     string
	ToRegime       string
	Probability    float64 // 0-1 probability of transition
	AffectedAssets []string
	Note           string
	DetectedAt     time.Time
}

// ---------------------------------------------------------------------------
// Strategy Result
// ---------------------------------------------------------------------------

// PlaybookResult is the full output from the Strategy Engine.
type PlaybookResult struct {
	Playbook   []PlaybookEntry
	Heat       PortfolioHeat
	Transition TransitionWarning
	MacroRegime string // current FRED macro regime
	ComputedAt time.Time
}

// TopLong returns the top n long ideas sorted by conviction.
func (r *PlaybookResult) TopLong(n int) []PlaybookEntry {
	return filterTop(r.Playbook, DirectionLong, n)
}

// TopShort returns the top n short ideas sorted by conviction.
func (r *PlaybookResult) TopShort(n int) []PlaybookEntry {
	return filterTop(r.Playbook, DirectionShort, n)
}

func filterTop(entries []PlaybookEntry, dir Direction, n int) []PlaybookEntry {
	var out []PlaybookEntry
	for _, e := range entries {
		if e.Direction == dir {
			out = append(out, e)
		}
	}
	// already sorted by conviction from engine
	if n < len(out) {
		out = out[:n]
	}
	return out
}
