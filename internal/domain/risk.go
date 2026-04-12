package domain

// ---------------------------------------------------------------------------
// Risk Sentiment Context — VIX + S&P 500 data for signal confidence filter
// ---------------------------------------------------------------------------

// RiskRegime classifies the current market risk environment.
type RiskRegime string

const (
	RiskRegimePanic    RiskRegime = "PANIC"    // VIX > 30 — all signals unreliable
	RiskRegimeElevated RiskRegime = "ELEVATED" // VIX 20-30 — reduce confidence
	RiskRegimeNormal   RiskRegime = "NORMAL"   // VIX 15-20 — neutral
	RiskRegimeLow      RiskRegime = "LOW"      // VIX < 15 — signals more directional
)

// RiskContext holds current VIX and S&P 500 snapshot for confidence adjustment.
type RiskContext struct {
	VIXLevel           float64    `json:"vix_level"`            // Latest VIX close
	VIX4WAvg           float64    `json:"vix_4w_avg"`           // 4-week VIX average (baseline)
	VIXTrend           string     `json:"vix_trend"`            // "RISING", "FALLING", "STABLE"
	SPXWeeklyChg       float64    `json:"spx_weekly_chg"`       // S&P 500 weekly % change
	SPXMonthlyChg      float64    `json:"spx_monthly_chg"`      // S&P 500 4-week % change
	SPXAboveMA4W       bool       `json:"spx_above_ma4w"`       // Risk-on if true
	Regime             RiskRegime `json:"regime"`               // Classified risk regime
	TermStructureSlope float64    `json:"term_structure_slope"` // VIX / VIX3M ratio (>1 = backwardation)
	IsBackwardation    bool       `json:"is_backwardation"`     // true when VIX > VIX3M
}

// ClassifyRiskRegime determines the risk regime from VIX level.
func ClassifyRiskRegime(vix float64) RiskRegime {
	switch {
	case vix > 30:
		return RiskRegimePanic
	case vix > 20:
		return RiskRegimeElevated
	case vix > 15:
		return RiskRegimeNormal
	default:
		return RiskRegimeLow
	}
}

// IsRiskOn returns true when market is in risk-on mode (SPX rising + VIX low).
func (rc *RiskContext) IsRiskOn() bool {
	if rc == nil {
		return false
	}
	return rc.SPXAboveMA4W && rc.VIXLevel < 20
}

// IsRiskOff returns true when market is in risk-off mode (VIX panic or SPX falling).
func (rc *RiskContext) IsRiskOff() bool {
	if rc == nil {
		return false
	}
	return rc.VIXLevel > 25 || rc.SPXMonthlyChg < -5
}

// ConfidenceAdjustment returns a multiplier applied to signal confidence
// based on VIX level and SPX trend. Final multiplier is clamped to [0.50, 1.25].
//
// Base multiplier by regime:
//   - PANIC    (VIX > 30): 0.70 — correlations break, models unreliable
//   - ELEVATED (VIX 20-30): 0.85 — noise elevated, reduce conviction
//   - NORMAL   (VIX 15-20): 1.00 — neutral
//   - LOW      (VIX < 15):  1.15 — clean trending environment, signals cleaner
//
// SPX modifiers applied on top of base:
//   - SPX monthly change < -5%: −0.10 (sharp equity selloff → additional dampening)
//   - SPX above MA4W and monthly change > 2%: +0.05 (risk-on momentum → slight boost)
//
// Effective range after clamp: [0.50, 1.25].
// (Worst case: PANIC 0.70 − 0.10 = 0.60, but can be 0.50 via clamp if future logic extends.)
func (rc *RiskContext) ConfidenceAdjustment() float64 {
	if rc == nil {
		return 1.0 // no adjustment when context unavailable
	}
	adj := 1.0

	switch rc.Regime {
	case RiskRegimePanic:
		adj = 0.70
	case RiskRegimeElevated:
		adj = 0.85
	case RiskRegimeNormal:
		adj = 1.00
	case RiskRegimeLow:
		adj = 1.15
	}

	// SPX trend modifier
	if rc.SPXMonthlyChg < -5 {
		adj -= 0.10 // Sharp equity selloff → additional dampening
	} else if rc.SPXAboveMA4W && rc.SPXMonthlyChg > 2 {
		adj += 0.05 // Risk-on momentum → slight boost
	}

	// Clamp: never below 50% or above 125%
	if adj < 0.50 {
		adj = 0.50
	}
	if adj > 1.25 {
		adj = 1.25
	}
	return adj
}

// AdjustConfidence applies the VIX/SPX risk filter to a signal confidence value.
// Returns the adjusted confidence clamped to [0, 100].
func (rc *RiskContext) AdjustConfidence(confidence float64) float64 {
	if rc == nil {
		return confidence
	}
	adjusted := confidence * rc.ConfidenceAdjustment()
	if adjusted > 100 {
		adjusted = 100
	}
	if adjusted < 0 {
		adjusted = 0
	}
	return adjusted
}

// RegimeLabel returns a human-readable label for the risk regime.
func (rc *RiskContext) RegimeLabel() string {
	if rc == nil {
		return "UNKNOWN"
	}
	switch rc.Regime {
	case RiskRegimePanic:
		return "⛔ PANIC (VIX>" + fmtFloat(rc.VIXLevel) + ")"
	case RiskRegimeElevated:
		return "⚠️ ELEVATED (VIX>" + fmtFloat(rc.VIXLevel) + ")"
	case RiskRegimeNormal:
		return "🟡 NORMAL (VIX=" + fmtFloat(rc.VIXLevel) + ")"
	case RiskRegimeLow:
		return "🟢 LOW (VIX=" + fmtFloat(rc.VIXLevel) + ")"
	default:
		return "UNKNOWN"
	}
}

func fmtFloat(v float64) string {
	// Simple one-decimal formatter without importing fmt
	// to avoid circular import issues in domain package.
	// Format: "25.3"
	// Guard: VIX values above ~200 are nonsensical but don't crash.
	if v > 9999 || v < -9999 {
		return "?.?"
	}
	i := int(v * 10)
	neg := ""
	if i < 0 {
		neg = "-"
		i = -i
	}
	return neg + itoa(i/10) + "." + itoa(i%10)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}
