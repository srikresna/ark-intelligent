// Package ict implements Smart Money Concepts (SMC) / Inner Circle Trader (ICT)
// analysis engine for forex pairs. It detects Fair Value Gaps, Order Blocks,
// Breaker Blocks, Change of Character (CHoCH), Break of Structure (BOS),
// and Liquidity Sweeps from OHLCV data.
package ict

import "time"

// ---------------------------------------------------------------------------
// Core Structs
// ---------------------------------------------------------------------------

// FVGZone represents a Fair Value Gap — a 3-candle imbalance zone.
type FVGZone struct {
	Kind      string    // "BULLISH" | "BEARISH"
	Top       float64   // upper bound of the gap
	Bottom    float64   // lower bound of the gap
	CreatedAt time.Time // timestamp of the middle candle
	BarIndex  int       // index of the middle candle (in the input slice)
	Filled    bool      // true if price has entered this zone
	FillPct   float64   // how far the gap has been filled (0–100%)
}

// OrderBlock represents an institutional demand/supply zone.
// A Bearish OB is the last bullish candle before a bearish impulse move.
// A Bullish OB is the last bearish candle before a bullish impulse move.
type OrderBlock struct {
	Kind     string  // "BULLISH" | "BEARISH"
	Top      float64 // high of the order block candle
	Bottom   float64 // low of the order block candle
	Volume   float64 // volume of the order block candle
	BarIndex int     // index in the input slice
	Broken   bool    // true = price has broken through → becomes a Breaker Block
}

// StructureEvent represents a BOS (Break of Structure) or CHoCH (Change of Character).
type StructureEvent struct {
	Kind      string  // "CHOCH" | "BOS"
	Direction string  // "BULLISH" | "BEARISH" (direction of the break)
	Level     float64 // swing high/low that was broken
	BarIndex  int     // index of the candle that broke the level
}

// LiquiditySweep represents a candle that wicks through a prior swing high/low
// (grabbing stop-loss liquidity) before reversing.
type LiquiditySweep struct {
	Kind      string  // "SWEEP_HIGH" | "SWEEP_LOW"
	Level     float64 // previous swing high/low that was swept
	SweepHigh float64 // high of the sweeping candle
	SweepLow  float64 // low of the sweeping candle
	BarIndex  int     // index of the sweeping candle
	Reversed  bool    // true if confirmed reversal after sweep (close opposite side)
}

// SwingPoint is an internal struct for swing highs/lows (not exported to callers).
type swingPoint struct {
	isHigh   bool
	level    float64
	barIndex int
}

// ICTResult is the main output of the ICT engine.
type ICTResult struct {
	Symbol      string
	Timeframe   string
	FVGZones    []FVGZone
	OrderBlocks []OrderBlock
	Structure   []StructureEvent
	Sweeps      []LiquiditySweep
	Bias        string    // "BULLISH" | "BEARISH" | "NEUTRAL"
	Killzone    string    // current killzone if applicable (e.g. "London", "New York")
	Summary     string    // human-readable narrative
	AnalyzedAt  time.Time
}
