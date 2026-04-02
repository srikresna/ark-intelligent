// Package ta — AMT Module 3: Rotation Factor.
//
// Counts half-rotations between Value Area extremes (VAH ↔ VAL).
// RF > 4 = balanced market (fade extremes); RF < 2 = directional (follow momentum).
package ta

import "time"

// RotationResult holds the analysis for a single trading day's rotation factor.
type RotationResult struct {
	Date time.Time

	// Core rotation metric
	RotationFactor int    // count of half-rotations between VA extremes
	Interpretation string // "BALANCED" | "DIRECTIONAL" | "TRANSITIONAL"
	Description    string // human-readable explanation

	// Time-based metrics
	TimeInVA      float64 // fraction of bars with midprice inside VA
	TimeOutsideVA float64 // fraction of bars with midprice outside VA
	TimeRatio     float64 // TimeInVA / TimeOutsideVA (>1 = balanced)

	// Value Area reference
	VA ValueArea
}

// AMTRotationResult holds rotation analysis for multiple days.
type AMTRotationResult struct {
	Days []RotationResult

	// Cross-day patterns
	AvgRotation float64 // average RF over analysed days
	Trend       string  // "INCREASING" | "DECREASING" | "STABLE" — RF direction over days
}

// ClassifyRotation analyses the rotation factor for the last N trading days.
// bars must be intraday (30m recommended), newest-first.
// ibPeriods is the number of bar periods constituting the Initial Balance.
// maxDays limits the number of days analysed.
func ClassifyRotation(bars []OHLCV, ibPeriods int, maxDays int) *AMTRotationResult {
	if len(bars) == 0 || ibPeriods < 1 || maxDays < 1 {
		return nil
	}

	dayGroups := groupByDay(bars)
	if len(dayGroups) == 0 {
		return nil
	}

	if len(dayGroups) > maxDays {
		dayGroups = dayGroups[len(dayGroups)-maxDays:]
	}

	result := &AMTRotationResult{}
	for _, dg := range dayGroups {
		if len(dg.bars) < ibPeriods+1 {
			continue
		}
		va := computeValueArea(dg.date, dg.bars)
		rr := analyseRotation(dg.date, dg.bars, va)
		result.Days = append(result.Days, rr)
	}

	if len(result.Days) == 0 {
		return nil
	}

	// Compute cross-day stats.
	sum := 0.0
	for _, d := range result.Days {
		sum += float64(d.RotationFactor)
	}
	result.AvgRotation = sum / float64(len(result.Days))
	result.Trend = rotationTrend(result.Days)

	return result
}

// analyseRotation computes the rotation factor for a single day.
func analyseRotation(date time.Time, bars []OHLCV, va ValueArea) RotationResult {
	rr := RotationResult{
		Date: date,
		VA:   va,
	}

	if len(bars) == 0 || va.VAH == va.VAL {
		rr.RotationFactor = 0
		rr.Interpretation = "DIRECTIONAL"
		rr.Description = "Zero-range VA; cannot compute rotation"
		return rr
	}

	// Track half-rotations: each crossing of VAH or VAL boundary counts as one.
	// A half-rotation occurs when price moves from near one VA extreme to the other.
	rf := 0
	inVA := 0
	outVA := 0

	// State: last extreme touched ("VAH", "VAL", or "")
	lastExtreme := ""
	// Threshold: how close to VAH/VAL counts as "touching"
	threshold := (va.VAH - va.VAL) * 0.10

	for _, b := range bars {
		mid := (b.High + b.Low) / 2.0

		// Count time in VA
		if mid >= va.VAL && mid <= va.VAH {
			inVA++
		} else {
			outVA++
		}

		// Check proximity to VA extremes
		nearVAH := b.High >= (va.VAH - threshold)
		nearVAL := b.Low <= (va.VAL + threshold)

		if nearVAH && lastExtreme == "VAL" {
			rf++
			lastExtreme = "VAH"
		} else if nearVAL && lastExtreme == "VAH" {
			rf++
			lastExtreme = "VAL"
		} else if nearVAH && lastExtreme == "" {
			lastExtreme = "VAH"
		} else if nearVAL && lastExtreme == "" {
			lastExtreme = "VAL"
		}
	}

	rr.RotationFactor = rf

	total := inVA + outVA
	if total > 0 {
		rr.TimeInVA = float64(inVA) / float64(total)
		rr.TimeOutsideVA = float64(outVA) / float64(total)
	}
	if outVA > 0 {
		rr.TimeRatio = float64(inVA) / float64(outVA)
	} else {
		rr.TimeRatio = float64(inVA)
	}

	// Interpret
	switch {
	case rf >= 4:
		rr.Interpretation = "BALANCED"
		rr.Description = "High rotation (≥4): market is balanced; fade VA extremes"
	case rf >= 2:
		rr.Interpretation = "TRANSITIONAL"
		rr.Description = "Moderate rotation (2-3): potential breakout building; watch for directional move"
	default:
		rr.Interpretation = "DIRECTIONAL"
		rr.Description = "Low rotation (<2): directional conviction; follow momentum"
	}

	return rr
}

// rotationTrend checks if the rotation factor is increasing, decreasing, or stable across days.
func rotationTrend(days []RotationResult) string {
	if len(days) < 2 {
		return "STABLE"
	}

	inc, dec := 0, 0
	for i := 1; i < len(days); i++ {
		diff := days[i].RotationFactor - days[i-1].RotationFactor
		if diff > 0 {
			inc++
		} else if diff < 0 {
			dec++
		}
	}

	total := inc + dec
	if total == 0 {
		return "STABLE"
	}
	if float64(inc)/float64(total) >= 0.65 {
		return "INCREASING"
	}
	if float64(dec)/float64(total) >= 0.65 {
		return "DECREASING"
	}
	return "STABLE"
}
