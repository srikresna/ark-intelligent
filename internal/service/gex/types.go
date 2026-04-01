// Package gex implements Gamma Exposure (GEX) analysis using Deribit options data.
package gex

import "time"

// GEXLevel represents the net gamma exposure at a single strike price.
type GEXLevel struct {
	Strike  float64 // option strike price
	CallGEX float64 // gamma exposure from call options
	PutGEX  float64 // gamma exposure from put options (negative convention)
	NetGEX  float64 // CallGEX + PutGEX
}

// GEXResult is the complete gamma exposure profile for a crypto asset.
type GEXResult struct {
	Symbol     string // e.g. "BTC"
	SpotPrice  float64

	// Aggregate GEX across all strikes
	TotalGEX float64 // positive = damping, negative = amplifying

	// Price level at which cumulative GEX changes sign (gamma neutral)
	GEXFlipLevel float64

	// Full profile, sorted by strike ascending
	Levels []GEXLevel

	// Strikes with the largest absolute NetGEX (support/resistance magnets)
	KeyLevels []float64 // up to 5 strikes

	// Put/Call walls
	GammaWall float64 // strike with highest call GEX (call resistance)
	PutWall   float64 // strike with lowest (most negative) put GEX (put support)
	MaxPain   float64 // strike that minimises total option holder value

	// Market regime derived from TotalGEX sign
	Regime     string // "POSITIVE_GEX" | "NEGATIVE_GEX"
	Implication string // human-readable interpretation

	// LowLiquidity is true when the number of instruments with meaningful
	// open interest is below a usable threshold (e.g. <20 strikes with OI).
	LowLiquidity bool

	AnalyzedAt time.Time
}
