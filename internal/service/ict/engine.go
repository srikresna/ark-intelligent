package ict

import (
	"fmt"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// Engine is the top-level ICT/SMC analysis engine.
type Engine struct{}

// NewEngine creates a new ICT Engine.
func NewEngine() *Engine { return &Engine{} }

// Analyze runs the full ICT/SMC analysis on a slice of OHLCV bars (newest-first).
// symbol and timeframe are for display purposes only.
//
// FVG and Order Block detection is delegated to the canonical ta.CalcICT
// implementation to avoid duplicated algorithms. Structure detection
// (BOS/CHoCH) and Liquidity Sweeps remain in this package as they are
// features unique to the service/ict engine.
func (e *Engine) Analyze(bars []ta.OHLCV, symbol, timeframe string) *ICTResult {
	result := &ICTResult{
		Symbol:     symbol,
		Timeframe:  timeframe,
		AnalyzedAt: time.Now().UTC(),
	}

	if len(bars) < 15 {
		result.Bias = "NEUTRAL"
		result.Summary = "Insufficient data for ICT/SMC analysis (need at least 15 bars)."
		return result
	}

	// Step 1: Compute ATR for ta.CalcICT.
	atr := ta.CalcATR(bars, 14)

	// Step 2: Delegate FVG + Order Block + Liquidity Level detection to ta.CalcICT.
	// This is the single authoritative source for these computations.
	if taICT := ta.CalcICT(bars, atr); taICT != nil {
		result.FVGZones = convertFVGs(taICT.FairValueGaps)
		result.OrderBlocks = convertOrderBlocks(taICT.OrderBlocks, taICT.BreakerBlocks)
	}

	// Step 3: Detect swing points (prerequisite for structure + sweeps).
	swings := detectSwings(bars)

	// Step 4: Market structure — CHoCH & BOS (unique to this engine).
	result.Structure = DetectStructure(swings)

	// Step 5: Liquidity Sweeps (unique to this engine).
	result.Sweeps = DetectLiquiditySweeps(bars, swings)

	// Step 6: Derive current bias from structure events.
	result.Bias = currentBias(result.Structure)

	// Step 7: Killzone detection from the most recent bar.
	if len(bars) > 0 {
		result.Killzone = detectKillzone(bars[0].Date)
	}

	// Step 8: Build narrative summary.
	result.Summary = buildSummary(result)

	return result
}

// ---------------------------------------------------------------------------
// Type conversion: ta.ICTResult → ict package types
// ---------------------------------------------------------------------------

// convertFVGs converts ta.FVG slices to ict.FVGZone slices.
// Field mapping is straightforward since names are now aligned.
func convertFVGs(taFVGs []ta.FVG) []FVGZone {
	if len(taFVGs) == 0 {
		return nil
	}
	zones := make([]FVGZone, len(taFVGs))
	for i, f := range taFVGs {
		zones[i] = FVGZone{
			Type:     f.Type,
			High:     f.High,
			Low:      f.Low,
			BarIndex: f.BarIndex,
			Filled:   f.Filled,
			FillPct:  f.FillPct,
		}
	}
	return zones
}

// convertOrderBlocks converts ta.OrderBlock slices to ict.OrderBlock slices.
// Breaker blocks (Broken=true in ta) are merged into the same output slice
// with Broken=true so callers can distinguish them.
func convertOrderBlocks(taOBs, taBreakers []ta.OrderBlock) []OrderBlock {
	total := len(taOBs) + len(taBreakers)
	if total == 0 {
		return nil
	}
	out := make([]OrderBlock, 0, total)
	for _, ob := range taOBs {
		out = append(out, OrderBlock{
			Type:     ob.Type,
			High:     ob.High,
			Low:      ob.Low,
			BarIndex: ob.BarIndex,
			Broken:   ob.Broken,
		})
	}
	for _, ob := range taBreakers {
		out = append(out, OrderBlock{
			Type:     ob.Type,
			High:     ob.High,
			Low:      ob.Low,
			BarIndex: ob.BarIndex,
			Broken:   true, // breakers are always broken by definition
		})
	}
	return out
}

// detectKillzone returns the ICT session name if the given UTC time falls
// within a known killzone window.
func detectKillzone(t time.Time) string {
	h := t.UTC().Hour()
	switch {
	case h >= 2 && h < 5:
		return "🌏 Asian Killzone (02:00–05:00 UTC)"
	case h >= 7 && h < 10:
		return "🇬🇧 London Open Killzone (07:00–10:00 UTC)"
	case h >= 12 && h < 15:
		return "🇺🇸 New York AM Killzone (12:00–15:00 UTC)"
	case h >= 19 && h < 21:
		return "🌙 London Close Killzone (19:00–21:00 UTC)"
	default:
		return ""
	}
}

// buildSummary generates a brief human-readable summary of the ICT result.
func buildSummary(r *ICTResult) string {
	activeFVG := 0
	for _, z := range r.FVGZones {
		if !z.Filled {
			activeFVG++
		}
	}
	activeOB := 0
	for _, ob := range r.OrderBlocks {
		if !ob.Broken {
			activeOB++
		}
	}
	reversedSweeps := 0
	for _, s := range r.Sweeps {
		if s.Reversed {
			reversedSweeps++
		}
	}

	lastStruct := ""
	if len(r.Structure) > 0 {
		ev := r.Structure[len(r.Structure)-1]
		lastStruct = fmt.Sprintf("%s %s", ev.Type, ev.Direction)
	}

	summary := fmt.Sprintf("Bias: %s.", r.Bias)
	if lastStruct != "" {
		summary += fmt.Sprintf(" Last structure event: %s.", lastStruct)
	}
	if activeOB > 0 {
		summary += fmt.Sprintf(" %d active Order Block(s).", activeOB)
	}
	if activeFVG > 0 {
		summary += fmt.Sprintf(" %d unfilled FVG(s).", activeFVG)
	}
	if reversedSweeps > 0 {
		summary += fmt.Sprintf(" %d confirmed liquidity sweep reversal(s).", reversedSweeps)
	}
	return summary
}
