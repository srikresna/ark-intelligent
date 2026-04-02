// Package ta — AMT Module 4: Close Location Tracker.
//
// Tracks where the session closes relative to Value Profile levels
// (POC, VAH, VAL) and computes historical follow-through rates.
package ta

import (
	"math"
	"time"
)

// CloseLocation enumerates close positions relative to the Value Area.
type CloseLocation string

const (
	CloseAboveVAH CloseLocation = "ABOVE_VAH" // Bullish: close above Value Area High
	CloseAtPOC    CloseLocation = "AT_POC"     // Neutral: close near Point of Control
	CloseInsideVA CloseLocation = "INSIDE_VA"  // Balanced: close within VA but not at POC
	CloseBelowVAL CloseLocation = "BELOW_VAL"  // Bearish: close below Value Area Low
)

// CloseClassification holds the close location analysis for a single day.
type CloseClassification struct {
	Date time.Time

	ClosePrice float64
	Location   CloseLocation
	VA         ValueArea

	// Distance metrics
	DistFromPOC float64 // close distance from POC as fraction of VA width
	DistFromVAH float64 // close distance from VAH (positive = above)
	DistFromVAL float64 // close distance from VAL (positive = above)

	// Follow-through (populated only when next day data is available)
	NextDayDirection string  // "BULLISH" | "BEARISH" | "FLAT" | "" (no data yet)
	FollowedThrough  bool    // did price continue in the direction implied by close location?
}

// AMTCloseResult holds close location analysis for multiple days.
type AMTCloseResult struct {
	Days []CloseClassification

	// Historical follow-through rates per close location
	FollowThroughRates map[CloseLocation]float64 // fraction of days that followed through

	// Today's context
	TodayImplication string // trading implication based on today's close + history
}

// ClassifyClose analyses the close location for the last N trading days.
// bars must be intraday (30m recommended), newest-first.
// maxDays limits the number of days analysed.
func ClassifyClose(bars []OHLCV, maxDays int) *AMTCloseResult {
	if len(bars) == 0 || maxDays < 1 {
		return nil
	}

	dayGroups := groupByDay(bars)
	if len(dayGroups) < 2 {
		return nil
	}

	// Limit to maxDays (keep one extra before for follow-through calc).
	start := 0
	if len(dayGroups) > maxDays+1 {
		start = len(dayGroups) - maxDays - 1
	}

	// Compute Value Areas for each day.
	vas := make([]ValueArea, len(dayGroups))
	for i, dg := range dayGroups {
		vas[i] = computeValueArea(dg.date, dg.bars)
	}

	result := &AMTCloseResult{
		FollowThroughRates: make(map[CloseLocation]float64),
	}

	// Classify close locations from start to end.
	for i := start; i < len(dayGroups); i++ {
		dg := dayGroups[i]
		if len(dg.bars) == 0 {
			continue
		}

		va := vas[i]
		closePrice := dg.bars[len(dg.bars)-1].Close

		cc := classifyCloseLocation(dg.date, closePrice, va)

		// Check follow-through from next day if available.
		if i+1 < len(dayGroups) {
			nextDg := dayGroups[i+1]
			if len(nextDg.bars) > 0 {
				nextOpen := nextDg.bars[0].Open
				nextClose := nextDg.bars[len(nextDg.bars)-1].Close
				cc.NextDayDirection = nextDayDir(nextOpen, nextClose)
				cc.FollowedThrough = checkFollowThrough(cc.Location, cc.NextDayDirection)
			}
		}

		result.Days = append(result.Days, cc)
	}

	if len(result.Days) == 0 {
		return nil
	}

	// Compute follow-through rates.
	type counts struct{ wins, total int }
	statsMap := map[CloseLocation]*counts{
		CloseAboveVAH: {},
		CloseAtPOC:    {},
		CloseInsideVA: {},
		CloseBelowVAL: {},
	}

	for _, cc := range result.Days {
		if cc.NextDayDirection == "" {
			continue // no next-day data (today)
		}
		c := statsMap[cc.Location]
		if c == nil {
			continue
		}
		c.total++
		if cc.FollowedThrough {
			c.wins++
		}
	}

	for loc, c := range statsMap {
		if c.total > 0 {
			result.FollowThroughRates[loc] = float64(c.wins) / float64(c.total)
		}
	}

	// Today's implication.
	today := result.Days[len(result.Days)-1]
	ftRate := result.FollowThroughRates[today.Location]
	result.TodayImplication = closeImplication(today.Location, ftRate)

	return result
}

// classifyCloseLocation determines where the close sits relative to VA.
func classifyCloseLocation(date time.Time, closePrice float64, va ValueArea) CloseClassification {
	cc := CloseClassification{
		Date:       date,
		ClosePrice: closePrice,
		VA:         va,
	}

	vaWidth := va.VAH - va.VAL
	if vaWidth == 0 {
		cc.Location = CloseInsideVA
		return cc
	}

	cc.DistFromPOC = (closePrice - va.POC) / vaWidth
	cc.DistFromVAH = closePrice - va.VAH
	cc.DistFromVAL = closePrice - va.VAL

	// POC proximity threshold: within 10% of VA width
	pocThreshold := vaWidth * 0.10

	switch {
	case closePrice > va.VAH:
		cc.Location = CloseAboveVAH
	case closePrice < va.VAL:
		cc.Location = CloseBelowVAL
	case math.Abs(closePrice-va.POC) <= pocThreshold:
		cc.Location = CloseAtPOC
	default:
		cc.Location = CloseInsideVA
	}

	return cc
}

// nextDayDir returns the direction of the next day.
func nextDayDir(open, close float64) string {
	diff := close - open
	threshold := open * 0.0001 // 1 pip for FX
	switch {
	case diff > threshold:
		return "BULLISH"
	case diff < -threshold:
		return "BEARISH"
	default:
		return "FLAT"
	}
}

// checkFollowThrough returns true if the next day's direction is consistent
// with the implication of the close location.
func checkFollowThrough(loc CloseLocation, nextDir string) bool {
	switch loc {
	case CloseAboveVAH:
		return nextDir == "BULLISH" // close above VAH → expect bullish continuation
	case CloseBelowVAL:
		return nextDir == "BEARISH" // close below VAL → expect bearish continuation
	case CloseAtPOC:
		return nextDir == "FLAT" || nextDir == "BEARISH" || nextDir == "BULLISH"
		// POC close is neutral — any outcome is "follow-through" for balanced
	case CloseInsideVA:
		return nextDir == "FLAT" || nextDir != "" // balanced continuation
	default:
		return false
	}
}

// closeImplication returns a trading implication string.
func closeImplication(loc CloseLocation, ftRate float64) string {
	ftPct := ftRate * 100
	switch loc {
	case CloseAboveVAH:
		if ftPct >= 60 {
			return "Close above VAH — bullish continuation expected (%.0f%% follow-through rate)"
		}
		return "Close above VAH — bullish signal, but follow-through historically mixed"
	case CloseBelowVAL:
		if ftPct >= 60 {
			return "Close below VAL — bearish continuation expected (%.0f%% follow-through rate)"
		}
		return "Close below VAL — bearish signal, but follow-through historically mixed"
	case CloseAtPOC:
		return "Close near POC — balanced market; expect range-bound action tomorrow"
	case CloseInsideVA:
		return "Close inside VA — no directional edge; watch for breakout signals"
	default:
		return "Insufficient data for implication"
	}
}
