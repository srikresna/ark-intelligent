// Package wyckoff provides the Wyckoff Method analysis engine.
// Input: []ta.OHLCV newest-first (index 0 = most recent bar).
// Internally reversed to oldest-first for sequential event detection.
package wyckoff

import (
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

const minBarsForAnalysis = 50

// Engine wraps Wyckoff analysis logic.
type Engine struct{}

// NewEngine creates a new Wyckoff Engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Analyze runs a full Wyckoff analysis on the provided bars.
// bars must be newest-first (index 0 = most recent). At least 50 bars are
// required; fewer returns a LOW-confidence UNKNOWN result.
func (e *Engine) Analyze(symbol, timeframe string, bars []ta.OHLCV) *WyckoffResult {
	now := time.Now()
	result := &WyckoffResult{
		Symbol:    symbol,
		Timeframe: timeframe,
		Schematic: "UNKNOWN",
		Confidence: "LOW",
		AnalyzedAt: now,
	}

	if len(bars) < minBarsForAnalysis {
		result.Summary = fmt.Sprintf("Insufficient data: %d bars (min %d required)", len(bars), minBarsForAnalysis)
		result.CurrentPhase = "UNDEFINED"
		return result
	}

	// Reverse to oldest-first for sequential detection.
	fwd := reverseOHLCV(bars)
	av := avgVolume(fwd)

	// ── Accumulation event detection ─────────────────────────────────────────
	var events []WyckoffEvent

	ps := detectPS(fwd, av)
	sc := detectSC(fwd, av)
	var ar, st, spring, sos, lps *WyckoffEvent
	if sc != nil {
		ar = detectAR(fwd, sc.BarIndex, av)
	}
	if sc != nil && ar != nil {
		st = detectST(fwd, sc.BarIndex, ar.BarIndex, av)
		spring = detectSpring(fwd, sc.BarIndex, ar.BarIndex, av)
		sos = detectSOS(fwd, ar.BarIndex, av)
	}
	if sos != nil && ar != nil {
		lps = detectLPS(fwd, sos.BarIndex, fwd[ar.BarIndex].High, av)
	}

	// ── Distribution event detection ─────────────────────────────────────────
	bc := detectBC(fwd, av)
	var arDist *WyckoffEvent
	if bc != nil {
		arDist = detectAR(fwd, bc.BarIndex, av)
		if arDist != nil {
			arDist.Name = EventARDist
			arDist.Description = "Automatic Reaction (Distribution) — sell-off from BC"
		}
	}
	var utad, sow *WyckoffEvent
	if bc != nil && arDist != nil {
		rHigh := fwd[bc.BarIndex].High
		utad = detectUTAD(fwd, arDist.BarIndex, rHigh, av)
		rLow := fwd[arDist.BarIndex].Low
		sow = detectSOW(fwd, bc.BarIndex, rLow, av)
	}

	// Collect non-nil events.
	for _, ev := range []*WyckoffEvent{ps, sc, ar, st, spring, sos, lps, bc, arDist, utad, sow} {
		if ev != nil {
			events = append(events, *ev)
		}
	}

	// Classify schematic, confidence, cause score.
	schematic, confidence, causeScore := classifySchematic(fwd, events, av)
	result.Schematic = schematic
	result.Confidence = confidence
	result.CauseBuilt = causeScore

	// Build phases.
	phases := buildPhases(events, len(fwd))
	result.Phases = phases
	result.CurrentPhase = currentPhase(phases)
	result.Events = events

	// Trading range.
	result.TradingRange = tradingRangeFor(fwd, events)

	// Projected move.
	result.ProjectedMove = projectedMove(result.TradingRange, causeScore)

	// Build summary.
	result.Summary = buildSummary(result)
	return result
}

// reverseOHLCV returns a new slice with bar order reversed.
func reverseOHLCV(bars []ta.OHLCV) []ta.OHLCV {
	n := len(bars)
	out := make([]ta.OHLCV, n)
	for i, b := range bars {
		out[n-1-i] = b
	}
	return out
}

// buildSummary composes a short narrative (≤ 300 chars) from the result.
func buildSummary(r *WyckoffResult) string {
	if r.Schematic == "UNKNOWN" {
		return "No clear Wyckoff structure detected. Need more price + volume data."
	}

	var lastEvent string
	if len(r.Events) > 0 {
		e := r.Events[len(r.Events)-1]
		lastEvent = string(e.Name)
	}

	s := fmt.Sprintf("%s schematic (%s confidence). Phase %s. Last event: %s. Range: %.4f–%.4f. Projected move: %.4f.",
		r.Schematic,
		r.Confidence,
		r.CurrentPhase,
		lastEvent,
		r.TradingRange[0],
		r.TradingRange[1],
		r.ProjectedMove,
	)
	if len(s) > 290 {
		s = s[:290] + "…"
	}
	return s
}
