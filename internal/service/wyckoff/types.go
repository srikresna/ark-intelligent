// Package wyckoff implements the Wyckoff Method for detecting Accumulation and
// Distribution schematics on OHLCV price data.
//
// References:
//   - Richard D. Wyckoff, "The Richard D. Wyckoff Method of Trading and
//     Investing in Stocks" (1931)
//   - Jack Hutson, Stock Market Institute Wyckoff course
//   - Ruben Villahermosa, "Wyckoff 2.0" (2020)
package wyckoff

import "time"

// ─────────────────────────────────────────────────────────────────────────────
// Event types
// ─────────────────────────────────────────────────────────────────────────────

// EventName identifies a specific Wyckoff event.
type EventName string

const (
	EventPS     EventName = "PS"     // Preliminary Support
	EventSC     EventName = "SC"     // Selling Climax
	EventAR     EventName = "AR"     // Automatic Rally
	EventST     EventName = "ST"     // Secondary Test
	EventSpring EventName = "SPRING" // Spring (Accumulation shakeout)
	EventSOS    EventName = "SOS"    // Sign of Strength
	EventLPS    EventName = "LPS"    // Last Point of Support
	EventUTAD   EventName = "UTAD"   // Upthrust After Distribution
	EventSOW    EventName = "SOW"    // Sign of Weakness
	EventBC     EventName = "BC"     // Buying Climax (Distribution)
	EventARDist EventName = "AR_D"   // Automatic Reaction (Distribution)
	EventUP     EventName = "UP"     // Upthrust Point
)

// ─────────────────────────────────────────────────────────────────────────────
// Structs
// ─────────────────────────────────────────────────────────────────────────────

// WyckoffEvent represents a single detected Wyckoff event on the price chart.
type WyckoffEvent struct {
	Name         EventName // e.g. "SC", "AR", "SPRING"
	BarIndex     int       // 0-based index into the bars slice (oldest-first internally)
	Price        float64   // closing price of the bar
	Volume       float64   // volume at the bar
	Significance string    // "HIGH", "MEDIUM", "LOW"
	Description  string    // human-readable explanation
}

// WyckoffPhase represents one Wyckoff phase (A–E).
type WyckoffPhase struct {
	Phase  string         // "A", "B", "C", "D", "E"
	Start  int            // bar index where phase begins
	End    int            // bar index where phase ends; -1 = ongoing
	Events []WyckoffEvent // events that occurred within this phase
}

// WyckoffResult is the full output of an analysis run.
type WyckoffResult struct {
	Symbol        string          // e.g. "EURUSD"
	Timeframe     string          // e.g. "H4"
	Schematic     string          // "ACCUMULATION" | "DISTRIBUTION" | "UNKNOWN"
	CurrentPhase  string          // "A", "B", "C", "D", "E", "UNDEFINED"
	Phases        []WyckoffPhase  // ordered list of identified phases
	Events        []WyckoffEvent  // all events (across all phases)
	TradingRange  [2]float64      // [support, resistance] of identified range
	CauseBuilt    float64         // composite "cause" energy score (0–100)
	ProjectedMove float64         // estimated breakout magnitude in price units
	Confidence    string          // "HIGH", "MEDIUM", "LOW"
	Summary       string          // narrative summary ≤ 300 chars
	AnalyzedAt    time.Time
}
