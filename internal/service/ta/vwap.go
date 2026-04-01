package ta

import (
	"fmt"
	"math"
	"time"
)

// ---------------------------------------------------------------------------
// Anchored VWAP — Volume Weighted Average Price with Deviation Bands
// ---------------------------------------------------------------------------

// VWAPResult holds VWAP computation for a given anchor point.
type VWAPResult struct {
	VWAP        float64 // Volume Weighted Average Price at current bar
	Band1Upper  float64 // VWAP + 1σ
	Band1Lower  float64 // VWAP - 1σ
	Band2Upper  float64 // VWAP + 2σ
	Band2Lower  float64 // VWAP - 2σ
	Band3Upper  float64 // VWAP + 3σ
	Band3Lower  float64 // VWAP - 3σ
	Position    string  // "ABOVE", "BELOW", "AT" (within 0.1% of VWAP)
	Deviation   float64 // current deviation in σ units (signed)
	AnchorType  string  // "DAILY", "WEEKLY", "MONTHLY", "SWING_LOW", "SWING_HIGH"
	AnchorPrice float64 // price at anchor bar
	BarsUsed    int     // number of bars from anchor to current
}

// VWAPSet holds VWAP calculations for multiple anchor presets simultaneously.
type VWAPSet struct {
	Daily     *VWAPResult // Anchored to daily session start (00:00 UTC)
	Weekly    *VWAPResult // Anchored to Monday 00:00 UTC
	SwingLow  *VWAPResult // Anchored to most recent swing low (5-bar pivot)
	SwingHigh *VWAPResult // Anchored to most recent swing high (5-bar pivot)
}

// String returns a compact human-readable summary of the VWAPSet.
func (vs *VWAPSet) String() string {
	if vs == nil {
		return ""
	}
	out := ""
	if vs.Daily != nil {
		out += fmt.Sprintf("Daily VWAP: %.5f (%s, %.1fσ)", vs.Daily.VWAP, vs.Daily.Position, vs.Daily.Deviation)
	}
	if vs.Weekly != nil {
		if out != "" {
			out += " | "
		}
		out += fmt.Sprintf("Weekly VWAP: %.5f (%s, %.1fσ)", vs.Weekly.VWAP, vs.Weekly.Position, vs.Weekly.Deviation)
	}
	return out
}

// ---------------------------------------------------------------------------
// CalcVWAP — anchored VWAP from a specific bar index
// ---------------------------------------------------------------------------

// CalcVWAPAnchored computes VWAP starting from a specific bar index.
// bars: newest-first (index 0 = most recent bar).
// anchorIdx: index in bars slice (newest-first) where anchor begins.
// anchorType: label for the anchor kind.
//
// Returns nil if fewer than 2 bars or zero cumulative volume.
func CalcVWAPAnchored(bars []OHLCV, anchorIdx int, anchorType string) *VWAPResult {
	if anchorIdx < 1 || anchorIdx >= len(bars) {
		return nil
	}

	// Work oldest-first from anchor to current bar.
	// In newest-first slice: anchor is at bars[anchorIdx], current at bars[0].
	// We need bars from anchorIdx down to 0, reversed to oldest-first.
	n := anchorIdx + 1 // number of bars from anchor to current (inclusive)
	asc := make([]OHLCV, n)
	for i := 0; i < n; i++ {
		asc[i] = bars[anchorIdx-i]
	}

	var cumVolume, cumTPV float64
	var lastVWAP float64

	// Track variance for deviation bands (incremental Welford-like approach).
	var cumWeightedVariance float64

	for i := 0; i < n; i++ {
		tp := (asc[i].High + asc[i].Low + asc[i].Close) / 3.0
		vol := asc[i].Volume
		if vol <= 0 {
			// If volume is zero, use equal weight (1.0) to avoid losing the bar.
			vol = 1.0
		}

		cumVolume += vol
		cumTPV += tp * vol
		lastVWAP = cumTPV / cumVolume

		// Accumulate weighted squared deviations for band calculation.
		diff := tp - lastVWAP
		cumWeightedVariance += vol * diff * diff
	}

	if cumVolume == 0 {
		return nil
	}

	sigma := math.Sqrt(cumWeightedVariance / cumVolume)

	// Determine position relative to VWAP.
	currentPrice := bars[0].Close
	threshold := lastVWAP * 0.001 // 0.1% threshold for "AT"

	position := "ABOVE"
	if math.Abs(currentPrice-lastVWAP) <= threshold {
		position = "AT"
	} else if currentPrice < lastVWAP {
		position = "BELOW"
	}

	// Deviation in sigma units.
	deviation := 0.0
	if sigma > 0 {
		deviation = (currentPrice - lastVWAP) / sigma
	}

	return &VWAPResult{
		VWAP:        lastVWAP,
		Band1Upper:  lastVWAP + sigma,
		Band1Lower:  lastVWAP - sigma,
		Band2Upper:  lastVWAP + 2*sigma,
		Band2Lower:  lastVWAP - 2*sigma,
		Band3Upper:  lastVWAP + 3*sigma,
		Band3Lower:  lastVWAP - 3*sigma,
		Position:    position,
		Deviation:   deviation,
		AnchorType:  anchorType,
		AnchorPrice: asc[0].Close,
		BarsUsed:    n,
	}
}

// ---------------------------------------------------------------------------
// CalcVWAP — convenience wrapper using all bars from anchor index
// ---------------------------------------------------------------------------

// CalcVWAP computes anchored VWAP from the given bars.
// bars: newest-first (all bars from anchor point onward).
// anchorType: label for the anchor kind.
//
// This is equivalent to CalcVWAPAnchored(bars, len(bars)-1, anchorType),
// using all provided bars from oldest to newest.
// Returns nil if fewer than 2 bars or zero cumulative volume.
func CalcVWAP(bars []OHLCV, anchorType string) *VWAPResult {
	if len(bars) < 2 {
		return nil
	}
	return CalcVWAPAnchored(bars, len(bars)-1, anchorType)
}

// ---------------------------------------------------------------------------
// CalcVWAPSet — multiple anchor presets
// ---------------------------------------------------------------------------

// CalcVWAPSet computes VWAP for all standard anchor presets:
// Daily (midnight UTC), Weekly (Monday 00:00 UTC),
// SwingLow and SwingHigh (5-bar pivot detection).
//
// Bars must be newest-first and should contain intraday data for meaningful
// daily/weekly anchors. If volume is zero for all bars, equal-weighted VWAP
// is computed (degrades to a simple average).
func CalcVWAPSet(bars []OHLCV) *VWAPSet {
	if len(bars) < 2 {
		return nil
	}

	set := &VWAPSet{}

	// --- Daily anchor: first bar on or after midnight UTC of the current bar's date ---
	set.Daily = calcVWAPByTimeAnchor(bars, "DAILY", func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	})

	// --- Weekly anchor: Monday 00:00 UTC of the current bar's week ---
	set.Weekly = calcVWAPByTimeAnchor(bars, "WEEKLY", func(t time.Time) time.Time {
		weekday := t.Weekday()
		if weekday == time.Sunday {
			weekday = 7
		}
		daysBack := int(weekday) - int(time.Monday)
		monday := t.AddDate(0, 0, -daysBack)
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
	})

	// --- Swing-based anchors: use 5-bar pivot detection ---
	const pivotStrength = 5
	if len(bars) >= 2*pivotStrength+1 {
		swingLowIdx, swingHighIdx := findSwingAnchors(bars, pivotStrength)
		if swingLowIdx > 0 {
			set.SwingLow = CalcVWAPAnchored(bars, swingLowIdx, "SWING_LOW")
		}
		if swingHighIdx > 0 {
			set.SwingHigh = CalcVWAPAnchored(bars, swingHighIdx, "SWING_HIGH")
		}
	}

	return set
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// calcVWAPByTimeAnchor finds the oldest bar whose date is >= the anchor
// cutoff derived from the most recent bar's timestamp, then computes VWAP.
func calcVWAPByTimeAnchor(bars []OHLCV, label string, cutoff func(time.Time) time.Time) *VWAPResult {
	if len(bars) == 0 {
		return nil
	}
	anchor := cutoff(bars[0].Date)

	// Walk from newest to oldest (bars are newest-first) to find the last bar
	// that is on or after the anchor cutoff.
	anchorIdx := 0
	for i := 0; i < len(bars); i++ {
		if !bars[i].Date.Before(anchor) {
			anchorIdx = i
		} else {
			break
		}
	}
	if anchorIdx < 1 {
		return nil // not enough bars after anchor
	}
	return CalcVWAPAnchored(bars, anchorIdx, label)
}

// findSwingAnchors returns newest-first indices of the most recent swing low
// and swing high using a pivot strength of 'strength' bars on each side.
// Returns -1 for either if not found.
func findSwingAnchors(bars []OHLCV, strength int) (swingLowIdx, swingHighIdx int) {
	swingLowIdx = -1
	swingHighIdx = -1
	n := len(bars)

	// Scan from most recent eligible bar outward (newest-first).
	// A bar at index i needs 'strength' bars on each side in the newest-first ordering.
	// In newest-first: "before" = indices < i (more recent), "after" = indices > i (older).
	for i := strength; i < n-strength; i++ {
		// Swing Low: Low[i] < Low of 'strength' bars on each side
		if swingLowIdx < 0 {
			isLow := true
			for j := i - strength; j < i; j++ {
				if bars[j].Low <= bars[i].Low {
					isLow = false
					break
				}
			}
			if isLow {
				for j := i + 1; j <= i+strength; j++ {
					if bars[j].Low <= bars[i].Low {
						isLow = false
						break
					}
				}
			}
			if isLow {
				swingLowIdx = i
			}
		}

		// Swing High: High[i] > High of 'strength' bars on each side
		if swingHighIdx < 0 {
			isHigh := true
			for j := i - strength; j < i; j++ {
				if bars[j].High >= bars[i].High {
					isHigh = false
					break
				}
			}
			if isHigh {
				for j := i + 1; j <= i+strength; j++ {
					if bars[j].High >= bars[i].High {
						isHigh = false
						break
					}
				}
			}
			if isHigh {
				swingHighIdx = i
			}
		}

		if swingLowIdx >= 0 && swingHighIdx >= 0 {
			break
		}
	}
	return
}

// hasVolume returns true if at least some bars in the slice have non-zero volume.
// VWAP is only meaningful with volume data; without it, the result degrades to
// a simple average (still computed, but less useful).
func hasVolume(bars []OHLCV) bool {
	for i := range bars {
		if bars[i].Volume > 0 {
			return true
		}
	}
	return false
}
