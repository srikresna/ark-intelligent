// Package domain defines core business types for the ARK Intelligent trading bot.
package domain

import "time"

// MacroComposites holds aggregated scores derived from raw FRED macro data.
// These composites sit between raw series and regime classification,
// making the scoring pipeline modular and testable.
type MacroComposites struct {
	// US domestic composites
	LaborHealth       float64 // 0-100 (100 = robust labor market)
	LaborLabel        string  // ROBUST, HEALTHY, SOFTENING, WEAKENING, DETERIORATING
	InflationMomentum float64 // -1.0 to +1.0 (positive = accelerating)
	InflationLabel    string  // ACCELERATING, WARMING, STABLE, COOLING, DEFLATING
	YieldCurveSignal  string  // DEEP_INVERSION, INVERTED, STEEPENING, FLAT, NORMAL, STEEP
	CreditStress      float64 // 0-100 (100 = crisis)
	CreditLabel       string  // LOOSE, NORMAL, TIGHTENING, STRESSED, CRISIS
	HousingPulse      string  // EXPANDING, STABLE, CONTRACTING, COLLAPSING
	FinConditions     float64 // -1.0 to +1.0 (positive = loose)

	// Per-country macro scores (-100 to +100)
	USScore float64
	EZScore float64
	UKScore float64
	JPScore float64
	AUScore float64
	CAScore float64
	NZScore float64

	// Sentiment composite (contrarian-adjusted, -100 to +100)
	SentimentComposite float64 // -100 = extreme greed (bearish), +100 = extreme fear (bullish)
	SentimentLabel     string  // EXTREME_GREED, GREED, NEUTRAL, FEAR, EXTREME_FEAR

	// VIX term structure
	VIXTermRatio  float64 // VIX / VIX3M (>1 = backwardation)
	VIXTermRegime string  // BACKWARDATION, FLAT, CONTANGO

	// NYSE Market Breadth
	AdvDecRatio float64 // NYSE Adv/Dec ratio (>1 = net positive breadth)
	NetNewHighs float64 // NYSE net new highs (NHs - NLs)

	ComputedAt time.Time
}

// LaborHealthLabel returns a label for a given labor health score.
func LaborHealthLabel(score float64) string {
	switch {
	case score >= 80:
		return "ROBUST"
	case score >= 60:
		return "HEALTHY"
	case score >= 40:
		return "SOFTENING"
	case score >= 20:
		return "WEAKENING"
	default:
		return "DETERIORATING"
	}
}

// InflationMomentumLabel returns a label for a given inflation momentum value.
func InflationMomentumLabel(momentum float64) string {
	switch {
	case momentum >= 0.5:
		return "ACCELERATING"
	case momentum >= 0.2:
		return "WARMING"
	case momentum >= -0.2:
		return "STABLE"
	case momentum >= -0.5:
		return "COOLING"
	default:
		return "DEFLATING"
	}
}

// CreditStressLabel returns a label for a given credit stress score.
func CreditStressLabel(score float64) string {
	switch {
	case score >= 80:
		return "CRISIS"
	case score >= 60:
		return "STRESSED"
	case score >= 40:
		return "TIGHTENING"
	case score >= 20:
		return "NORMAL"
	default:
		return "LOOSE"
	}
}

// SentimentCompositeLabel returns a contrarian-adjusted sentiment label.
func SentimentCompositeLabel(score float64) string {
	switch {
	case score >= 50:
		return "EXTREME_FEAR"
	case score >= 20:
		return "FEAR"
	case score >= -20:
		return "NEUTRAL"
	case score >= -50:
		return "GREED"
	default:
		return "EXTREME_GREED"
	}
}
