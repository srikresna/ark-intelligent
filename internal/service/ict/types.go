// Package ict implements Smart Money Concepts (SMC) / Inner Circle Trader (ICT)
// analysis engine for forex pairs. It detects Fair Value Gaps, Order Blocks,
// Breaker Blocks, Change of Character (CHoCH), Break of Structure (BOS),
// and Liquidity Sweeps from OHLCV data.
//
// FVG and Order Block detection is delegated to the canonical ta.CalcICT()
// implementation in internal/service/ta/ict.go. This package adds structure
// detection (BOS/CHoCH) and liquidity sweep analysis on top.
//
// Type field names are aligned with the ta package for consistency:
//   - Type (not Kind) for directional classification
//   - High/Low (not Top/Bottom) for price boundaries
package ict

import "time"

// ---------------------------------------------------------------------------
// Core Structs — field names aligned with ta.FVG / ta.OrderBlock
// ---------------------------------------------------------------------------

// FVGZone represents a Fair Value Gap — a 3-candle imbalance zone.
// Field names (High, Low, Type) match ta.FVG for consistency.
type FVGZone struct {
	Type      string    // "BULLISH" | "BEARISH"  (aligned with ta.FVG.Type)
	High      float64   // upper bound of the gap (aligned with ta.FVG.High)
	Low       float64   // lower bound of the gap (aligned with ta.FVG.Low)
	CreatedAt time.Time // timestamp of the middle candle
	BarIndex  int       // index of the middle candle (in the input slice)
	Filled    bool      // true if price has entered this zone
	FillPct   float64   // how far the gap has been filled (0–100%)
}

// OrderBlock represents an institutional demand/supply zone.
// A Bearish OB is the last bullish candle before a bearish impulse move.
// A Bullish OB is the last bearish candle before a bullish impulse move.
// Field names (High, Low, Type) match ta.OrderBlock for consistency.
type OrderBlock struct {
	Type     string  // "BULLISH" | "BEARISH"  (aligned with ta.OrderBlock.Type)
	High     float64 // high of the order block candle (aligned with ta.OrderBlock.High)
	Low      float64 // low of the order block candle  (aligned with ta.OrderBlock.Low)
	Volume   float64 // volume of the order block candle
	BarIndex int     // index in the input slice
	Broken   bool    // true = price has broken through → becomes a Breaker Block
}

// StructureEvent represents a BOS (Break of Structure) or CHoCH (Change of Character).
type StructureEvent struct {
	Type      string  // "CHOCH" | "BOS"
	Direction string  // "BULLISH" | "BEARISH" (direction of the break)
	Level     float64 // swing high/low that was broken
	BarIndex  int     // index of the candle that broke the level
}

// LiquiditySweep represents a candle that wicks through a prior swing high/low
// (grabbing stop-loss liquidity) before reversing.
type LiquiditySweep struct {
	Type      string  // "SWEEP_HIGH" | "SWEEP_LOW"
	Level     float64 // previous swing high/low that was swept
	SweepHigh float64 // high of the sweeping candle
	SweepLow  float64 // low of the sweeping candle
	BarIndex  int     // index of the sweeping candle
	Reversed  bool    // true if confirmed reversal after sweep (close opposite side)
}

// swingPoint is an internal struct for swing highs/lows (not exported to callers).
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
