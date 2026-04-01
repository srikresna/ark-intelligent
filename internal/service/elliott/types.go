// Package elliott implements Elliott Wave counting and projection for OHLCV data.
// Only Phase 1 (MVP) is implemented:
//   - ZigZag swing detection
//   - Basic 5-wave impulse identification
//   - Rule validation (3 rules)
//   - Simple Fibonacci targets
package elliott

import "time"

// ---------------------------------------------------------------------------
// WaveType — canonical Elliott Wave pattern labels
// ---------------------------------------------------------------------------

// WaveType represents the structural type of an Elliott Wave pattern.
type WaveType string

const (
	Impulse    WaveType = "IMPULSE"
	Corrective WaveType = "CORRECTIVE"
	Diagonal   WaveType = "DIAGONAL"
	ZigZag     WaveType = "ZIGZAG"
	Flat       WaveType = "FLAT"
	Triangle   WaveType = "TRIANGLE"
)

// ---------------------------------------------------------------------------
// Wave — single wave leg
// ---------------------------------------------------------------------------

// Wave represents one labelled wave leg in an Elliott Wave count.
type Wave struct {
	// Number is the wave label: "1","2","3","4","5" for impulse legs,
	// "A","B","C" for corrective legs.
	Number string

	// Kind is the wave pattern type.
	Kind WaveType

	// Start is the price level at the beginning of this wave.
	Start float64

	// End is the price level at the end of this wave.
	// 0 = ongoing / not yet confirmed (current wave).
	End float64

	// StartBar is the bar index (in the input OHLCV slice, newest-first)
	// where this wave begins.
	StartBar int

	// EndBar is the bar index where this wave ends.
	// -1 = current / ongoing wave.
	EndBar int

	// Direction is "UP" or "DOWN".
	Direction string

	// Retracement is the percentage retracement relative to the previous wave.
	// For example 0.618 means the wave retraced 61.8% of the prior move.
	Retracement float64

	// FibRatio is the ratio of this wave's length relative to Wave 1 (for
	// waves 3 and 5) or Wave A (for wave C).
	FibRatio float64

	// Valid is false when an Elliott Wave rule is violated.
	Valid bool

	// Violation describes the rule violation when Valid==false.
	Violation string
}

// Length returns the absolute price distance of the wave.
func (w Wave) Length() float64 {
	d := w.End - w.Start
	if d < 0 {
		d = -d
	}
	return d
}

// ---------------------------------------------------------------------------
// WaveCountResult — full count output
// ---------------------------------------------------------------------------

// WaveCountResult is the output of Engine.Analyze.
type WaveCountResult struct {
	// Symbol and Timeframe are informational labels passed by the caller.
	Symbol    string
	Timeframe string

	// Degree is the Elliott Wave degree label
	// (e.g. "PRIMARY", "INTERMEDIATE", "MINOR").
	Degree string

	// Waves holds the identified wave legs in sequence.
	Waves []Wave

	// CurrentWave is the label of the wave currently in progress.
	CurrentWave string

	// WaveProgress is a 0-100 estimate of how far through the current wave we are,
	// based on the average length of prior waves of the same type.
	WaveProgress float64

	// InvalidationLevel is the price that would invalidate this wave count.
	// For a bullish count this is typically the start of Wave 1.
	InvalidationLevel float64

	// Target1 is the conservative price projection.
	Target1 float64

	// Target2 is the aggressive price projection.
	Target2 float64

	// AlternateCount holds an alternative wave scenario.  May be nil.
	AlternateCount *WaveCountResult

	// Confidence is "HIGH", "MEDIUM", or "LOW".
	Confidence string

	// Summary is a one-line human-readable assessment.
	Summary string

	// AnalyzedAt is the time the analysis was computed.
	AnalyzedAt time.Time
}

// ---------------------------------------------------------------------------
// SwingPoint — internal ZigZag node
// ---------------------------------------------------------------------------

// SwingPoint is an intermediate ZigZag pivot used by the engine.
type SwingPoint struct {
	// Index is the bar index in the original (newest-first) OHLCV slice.
	Index int

	// Price is the relevant price: High for swing highs, Low for swing lows.
	Price float64

	// IsHigh is true for swing highs, false for swing lows.
	IsHigh bool
}
