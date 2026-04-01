// Package elliott provides Elliott Wave counting and projection for OHLCV data.
package elliott

import (
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ---------------------------------------------------------------------------
// Engine — public entry point
// ---------------------------------------------------------------------------

// Engine performs Elliott Wave analysis on OHLCV bars.
type Engine struct {
	// MinRetracement is the minimum price reversal (as a fraction, e.g. 0.05
	// for 5%) required to confirm a new ZigZag pivot.
	MinRetracement float64
}

// NewEngine returns an Engine with sensible defaults.
func NewEngine() *Engine {
	return &Engine{MinRetracement: 0.05}
}

// ---------------------------------------------------------------------------
// Analyze — main public method
// ---------------------------------------------------------------------------

// Analyze runs a full Elliott Wave count on the provided bars and returns a
// WaveCountResult.  Bars must be in newest-first order (index 0 = most recent).
//
// symbol and timeframe are informational labels for the output.
//
// Returns nil when bars is too short (minimum 20 bars required).
func (e *Engine) Analyze(bars []ta.OHLCV, symbol, timeframe string) *WaveCountResult {
	if len(bars) < 20 {
		return nil
	}

	pivots := detectZigZag(bars, e.MinRetracement)
	if len(pivots) < 6 {
		// Not enough pivots for a 5-wave count; return a low-confidence result.
		return e.insufficientDataResult(symbol, timeframe, len(bars))
	}

	// Attempt to fit a 5-wave impulse into the most recent swing points.
	result := e.fitImpulse(pivots, bars, symbol, timeframe)
	if result != nil {
		return result
	}

	// Fallback: incomplete count detected (< 5 pivots confirmed).
	return e.incompleteCountResult(pivots, bars, symbol, timeframe)
}

// ---------------------------------------------------------------------------
// fitImpulse — try to identify a 5-wave impulse in the pivot sequence
// ---------------------------------------------------------------------------

// fitImpulse scans the pivot sequence (oldest→newest) looking for the most
// recent 5-pivot segment that respects Elliott Wave rules.
//
// It returns nil when no valid (or partially-valid) 5-wave sequence can be
// identified.
func (e *Engine) fitImpulse(pivots []SwingPoint, bars []ta.OHLCV, symbol, timeframe string) *WaveCountResult {
	// We need at least 6 pivots to form W1-W2-W3-W4-W5:
	//   pivot[0] = W1 start
	//   pivot[1] = W1 end / W2 start
	//   pivot[2] = W2 end / W3 start
	//   pivot[3] = W3 end / W4 start
	//   pivot[4] = W4 end / W5 start
	//   pivot[5] = W5 end  (may be the ongoing bar)
	if len(pivots) < 6 {
		return nil
	}

	// Try from the end, using the most recent 6 pivots.
	n := len(pivots)
	seg := pivots[n-6:]

	// Determine impulse direction from W1 pivot pair.
	bullish := !seg[0].IsHigh // W1 starts at a swing low → bullish impulse
	if seg[0].IsHigh == seg[1].IsHigh {
		// Malformed pivot sequence — two highs or two lows in a row.
		return nil
	}

	w1Dir := "UP"
	if !bullish {
		w1Dir = "DOWN"
	}

	waves := buildWaveLegs(seg, w1Dir)
	if len(waves) < 5 {
		return nil
	}

	// Validate all three Elliott Wave rules (annotates waves in-place).
	validateImpulse(waves)

	// Determine current wave (the last leg — may be in-progress).
	currentWave := waves[len(waves)-1].Number
	progress := waveProgress(waves, currentWave)
	inv := invalidationLevel(waves)
	t1, t2 := projectTargets(waves)

	confidence := scoreConfidence(waves, len(bars))
	summary := buildSummary(waves, currentWave, bullish, confidence)

	// Build an alternate count when confidence < HIGH.
	var altCount *WaveCountResult
	if confidence != "HIGH" && n >= 7 {
		altSeg := pivots[n-7 : n-1]
		altWaves := buildWaveLegs(altSeg, w1Dir)
		if len(altWaves) >= 5 {
			validateImpulse(altWaves)
			altT1, altT2 := projectTargets(altWaves)
			altCount = &WaveCountResult{
				Symbol:            symbol,
				Timeframe:         timeframe,
				Degree:            "MINOR",
				Waves:             altWaves,
				CurrentWave:       altWaves[len(altWaves)-1].Number,
				InvalidationLevel: invalidationLevel(altWaves),
				Target1:           altT1,
				Target2:           altT2,
				Confidence:        "LOW",
				Summary:           "Alternate count: shifted pivot set",
				AnalyzedAt:        time.Now(),
			}
		}
	}

	return &WaveCountResult{
		Symbol:            symbol,
		Timeframe:         timeframe,
		Degree:            "PRIMARY",
		Waves:             waves,
		CurrentWave:       currentWave,
		WaveProgress:      progress,
		InvalidationLevel: inv,
		Target1:           t1,
		Target2:           t2,
		AlternateCount:    altCount,
		Confidence:        confidence,
		Summary:           summary,
		AnalyzedAt:        time.Now(),
	}
}

// ---------------------------------------------------------------------------
// buildWaveLegs — convert pivot pairs into Wave structs
// ---------------------------------------------------------------------------

// buildWaveLegs converts a 6-pivot segment into Wave structs for waves 1–5.
// The w1Dir parameter is "UP" or "DOWN" indicating the direction of Wave 1.
func buildWaveLegs(seg []SwingPoint, w1Dir string) []Wave {
	labels := []string{"1", "2", "3", "4", "5"}
	waves := make([]Wave, 0, 5)

	for i := 0; i < 5 && i+1 < len(seg); i++ {
		start := seg[i]
		end := seg[i+1]

		dir := "UP"
		if end.Price < start.Price {
			dir = "DOWN"
		}

		kind := Impulse
		if i == 1 || i == 3 { // waves 2 and 4 are corrective
			kind = Corrective
		}

		endBar := end.Index
		if i == 4 && end.Index == seg[len(seg)-1].Index {
			endBar = -1 // last pivot = current/ongoing
		}

		waves = append(waves, Wave{
			Number:   labels[i],
			Kind:     kind,
			Start:    start.Price,
			End:      end.Price,
			StartBar: start.Index,
			EndBar:   endBar,
			Direction: dir,
		})
	}

	_ = w1Dir // kept for future directional checks
	return waves
}

// ---------------------------------------------------------------------------
// fallback result builders
// ---------------------------------------------------------------------------

// insufficientDataResult returns a LOW-confidence placeholder when there are
// too few bars / pivots to complete a wave count.
func (e *Engine) insufficientDataResult(symbol, timeframe string, barCount int) *WaveCountResult {
	return &WaveCountResult{
		Symbol:      symbol,
		Timeframe:   timeframe,
		Degree:      "UNKNOWN",
		CurrentWave: "?",
		Confidence:  "LOW",
		Summary:     fmt.Sprintf("Insufficient data (%d bars) — need ≥50 for reliable count", barCount),
		AnalyzedAt:  time.Now(),
	}
}

// incompleteCountResult returns an in-progress result when fewer than 5 waves
// have been identified (e.g. early in a new trend).
func (e *Engine) incompleteCountResult(pivots []SwingPoint, bars []ta.OHLCV, symbol, timeframe string) *WaveCountResult {
	// Build as many wave legs as we have pivots for.
	n := len(pivots)
	if n < 2 {
		return e.insufficientDataResult(symbol, timeframe, len(bars))
	}

	seg := pivots
	if n > 6 {
		seg = pivots[n-6:]
	}

	bullish := !seg[0].IsHigh
	w1Dir := "UP"
	if !bullish {
		w1Dir = "DOWN"
	}

	waves := buildWaveLegs(seg, w1Dir)
	labels := []string{"1", "2", "3", "4", "5"}
	currentWave := "?"
	if len(waves) > 0 {
		idx := len(waves) - 1
		if idx < len(labels) {
			currentWave = labels[idx]
		}
	}

	return &WaveCountResult{
		Symbol:      symbol,
		Timeframe:   timeframe,
		Degree:      "PRIMARY",
		Waves:       waves,
		CurrentWave: currentWave,
		Confidence:  "LOW",
		Summary:     fmt.Sprintf("Wave count in progress — currently in Wave %s (incomplete)", currentWave),
		AnalyzedAt:  time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Summary builder
// ---------------------------------------------------------------------------

func buildSummary(waves []Wave, currentWave string, bullish bool, confidence string) string {
	dir := "bullish"
	if !bullish {
		dir = "bearish"
	}

	// Count valid waves.
	valid := 0
	for _, w := range waves {
		if w.Valid {
			valid++
		}
	}

	return fmt.Sprintf("%s impulse — currently Wave %s (%d/5 waves valid, confidence: %s)",
		dir, currentWave, valid, confidence)
}
