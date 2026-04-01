package vix

import "time"

// VIXTermStructure holds VIX spot, futures term structure, and derived signals.
type VIXTermStructure struct {
	// Raw data
	Spot  float64 // VIX spot index
	M1    float64 // Front-month VIX futures settle price
	M2    float64 // Second-month VIX futures settle price
	M3    float64 // Third-month VIX futures settle price
	VVIX  float64 // VIX of VIX (vol-of-vol)

	// Contract symbols
	M1Symbol string // e.g. "/VXK26"
	M2Symbol string
	M3Symbol string

	// Derived signals
	Contango      bool    // true if M1 > Spot and M2 > M1 (normal/risk-on)
	Backwardation bool    // true if M1 < Spot (fear/risk-off)
	SlopePct      float64 // (M2-M1)/M1 * 100 — % slope of term structure
	RollYield     float64 // approximate monthly roll cost/benefit (% per month)

	// Regime classification
	Regime string // "EXTREME_FEAR", "FEAR", "ELEVATED", "RISK_ON_NORMAL", "RISK_ON_COMPLACENT"

	// Metadata
	Available bool      // false if data could not be fetched/parsed
	AsOf      time.Time // UTC timestamp of data
	Error     string    // non-empty if fetching failed

	// MOVE Index (bond volatility) — cross-asset vol comparison
	MOVE *MOVEData // nil if MOVE data unavailable
}
