package ta

import (
	"fmt"
	"math"
)

// ---------------------------------------------------------------------------
// Zone Result
// ---------------------------------------------------------------------------

// ZoneResult represents computed entry/exit zones with stop-loss and take-profit
// levels, plus risk:reward ratios.
type ZoneResult struct {
	Direction   string  // "LONG" or "SHORT"
	EntryLow    float64 // entry zone bottom
	EntryHigh   float64 // entry zone top
	StopLoss    float64
	TakeProfit1 float64 // conservative TP
	TakeProfit2 float64 // aggressive TP
	RiskReward1 float64 // R:R for TP1
	RiskReward2 float64 // R:R for TP2
	Confidence  string  // "HIGH", "MEDIUM", "LOW"
	Reasoning   string  // why these levels
	Valid       bool    // false if no good setup found
}

// ---------------------------------------------------------------------------
// ATR helper (needed for zone calculation)
// ---------------------------------------------------------------------------

// CalcATR computes the Average True Range for the most recent bar over `period` bars.
// True Range = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
// ATR = SMA of True Range over period (simplified; Wilder's smoothing is used for ADX).
// Returns 0 if insufficient data.
func CalcATR(bars []OHLCV, period int) float64 {
	if len(bars) < period+1 || period <= 0 {
		return 0
	}
	// Work newest-first: bars[0] is most recent
	// We need period TR values starting from bars[0]
	sum := 0.0
	count := 0
	for i := 0; i < period; i++ {
		if i+1 >= len(bars) {
			break
		}
		hl := bars[i].High - bars[i].Low
		hpc := math.Abs(bars[i].High - bars[i+1].Close)
		lpc := math.Abs(bars[i].Low - bars[i+1].Close)
		tr := math.Max(hl, math.Max(hpc, lpc))
		sum += tr
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// ---------------------------------------------------------------------------
// CalcZones — Entry/Exit Zone Calculator
// ---------------------------------------------------------------------------

// CalcZones computes entry, stop-loss, and take-profit levels from the
// indicator snapshot and confluence result.
//
// Logic:
//  1. Only valid if confluence Grade >= "C" (abs score >= 25)
//  2. Direction from confluence
//  3. Entry zone: current price ± ATR*0.3
//  4. Stop Loss with Fibonacci/Bollinger/SuperTrend levels or ATR fallback
//  5. Take Profit at Fibonacci levels or Bollinger opposite band
//  6. Valid only if R:R >= 1.5
func CalcZones(snap *IndicatorSnapshot, confluence *ConfluenceResult) *ZoneResult {
	if snap == nil || confluence == nil {
		return &ZoneResult{Valid: false, Reasoning: "Insufficient data"}
	}

	// Only valid if grade >= C
	absScore := math.Abs(confluence.Score)
	if absScore < 25 {
		return &ZoneResult{
			Valid:     false,
			Reasoning: fmt.Sprintf("Score too weak (%.1f, Grade %s) — need at least Grade C (±25)", confluence.Score, confluence.Grade),
		}
	}

	// Direction
	direction := "LONG"
	if confluence.Direction == "BEARISH" {
		direction = "SHORT"
	}

	// Use actual current price from snapshot
	currentPrice := snap.CurrentPrice
	if currentPrice == 0 {
		// Fallback: use Bollinger Middle if available
		if snap.Bollinger != nil {
			currentPrice = snap.Bollinger.Middle
		}
		if currentPrice == 0 {
			// Try shortest EMA as proxy
			if snap.EMA != nil && len(snap.EMA.Values) > 0 {
				minPeriod := 0
				for p := range snap.EMA.Values {
					if minPeriod == 0 || p < minPeriod {
						minPeriod = p
					}
				}
				if minPeriod > 0 {
					currentPrice = snap.EMA.Values[minPeriod]
				}
			}
		}
	}
	if currentPrice == 0 {
		return &ZoneResult{Valid: false, Reasoning: "Cannot determine current price level"}
	}

	// Use ATR from snapshot, fallback to estimation
	atr := snap.ATR
	if atr <= 0 {
		atr = estimateATR(snap, currentPrice)
	}
	if atr <= 0 {
		// Fallback: use 1% of price
		atr = currentPrice * 0.01
	}

	// Entry zone: current price ± ATR*0.3
	entryLow := currentPrice - atr*0.3
	entryHigh := currentPrice + atr*0.3
	entryMid := currentPrice

	// Collect support/resistance levels from available indicators
	var supportLevels []float64
	var resistanceLevels []float64

	// Bollinger bands as levels
	if snap.Bollinger != nil {
		supportLevels = append(supportLevels, snap.Bollinger.Lower)
		resistanceLevels = append(resistanceLevels, snap.Bollinger.Upper)
	}

	// Fibonacci levels
	if snap.Fibonacci != nil {
		for _, lvl := range snap.Fibonacci.Levels {
			if lvl < currentPrice {
				supportLevels = append(supportLevels, lvl)
			} else if lvl > currentPrice {
				resistanceLevels = append(resistanceLevels, lvl)
			}
		}
	}

	// SuperTrend as support/resistance
	if snap.SuperTrend != nil {
		if snap.SuperTrend.Direction == "UP" {
			// SuperTrend below price = support
			supportLevels = append(supportLevels, snap.SuperTrend.Value)
		} else {
			// SuperTrend above price = resistance
			resistanceLevels = append(resistanceLevels, snap.SuperTrend.Value)
		}
	}

	// EMA levels as support/resistance
	if snap.EMA != nil {
		for _, v := range snap.EMA.Values {
			if v < currentPrice {
				supportLevels = append(supportLevels, v)
			} else if v > currentPrice {
				resistanceLevels = append(resistanceLevels, v)
			}
		}
	}

	// Compute stop loss and take profits
	var sl, tp1, tp2 float64
	var reasoning string

	if direction == "LONG" {
		sl = computeLongStopLoss(entryMid, atr, supportLevels)
		tp1, tp2 = computeLongTakeProfits(entryMid, atr, resistanceLevels, snap)
		reasoning = fmt.Sprintf("LONG setup: entry %.4f, SL %.4f (%.1f%% risk), TP1 %.4f, TP2 %.4f",
			entryMid, sl, (entryMid-sl)/entryMid*100, tp1, tp2)
	} else {
		sl = computeShortStopLoss(entryMid, atr, resistanceLevels)
		tp1, tp2 = computeShortTakeProfits(entryMid, atr, supportLevels, snap)
		reasoning = fmt.Sprintf("SHORT setup: entry %.4f, SL %.4f (%.1f%% risk), TP1 %.4f, TP2 %.4f",
			entryMid, sl, (sl-entryMid)/entryMid*100, tp1, tp2)
	}

	// Risk:Reward calculation
	risk := math.Abs(entryMid - sl)
	if risk == 0 {
		return &ZoneResult{Valid: false, Reasoning: "Zero risk distance — cannot compute R:R"}
	}

	rr1 := math.Abs(tp1-entryMid) / risk
	rr2 := math.Abs(tp2-entryMid) / risk

	// Valid only if R:R >= 1.5
	valid := rr1 >= 1.5
	if !valid {
		reasoning += fmt.Sprintf(" | R:R too low (%.2f < 1.5)", rr1)
	}

	// Confidence
	confidence := "LOW"
	if confluence.Grade == "A" && rr1 > 2.0 {
		confidence = "HIGH"
	} else if confluence.Grade == "B" || (confluence.Grade == "A" && rr1 <= 2.0) {
		confidence = "MEDIUM"
	}

	return &ZoneResult{
		Direction:   direction,
		EntryLow:    entryLow,
		EntryHigh:   entryHigh,
		StopLoss:    sl,
		TakeProfit1: tp1,
		TakeProfit2: tp2,
		RiskReward1: rr1,
		RiskReward2: rr2,
		Confidence:  confidence,
		Reasoning:   reasoning,
		Valid:       valid,
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// estimateATR estimates ATR from Bollinger bandwidth or other available data.
func estimateATR(snap *IndicatorSnapshot, price float64) float64 {
	if snap.Bollinger != nil && snap.Bollinger.Upper > snap.Bollinger.Lower {
		// BB width ≈ 4 * stddev. ATR ≈ 1.5 * stddev (rough approximation)
		stddev := (snap.Bollinger.Upper - snap.Bollinger.Lower) / 4.0
		return stddev * 1.5
	}
	return 0
}

// computeLongStopLoss finds stop loss for a long position.
// Prefers: nearest support level below entry - ATR*0.5
// Fallback: entry - ATR*1.5
func computeLongStopLoss(entry, atr float64, supportLevels []float64) float64 {
	bestSupport := 0.0
	for _, lvl := range supportLevels {
		if lvl < entry && (bestSupport == 0 || lvl > bestSupport) {
			bestSupport = lvl
		}
	}
	if bestSupport > 0 {
		return bestSupport - atr*0.5
	}
	return entry - atr*1.5
}

// computeShortStopLoss finds stop loss for a short position.
// Prefers: nearest resistance level above entry + ATR*0.5
// Fallback: entry + ATR*1.5
func computeShortStopLoss(entry, atr float64, resistanceLevels []float64) float64 {
	bestResistance := 0.0
	for _, lvl := range resistanceLevels {
		if lvl > entry && (bestResistance == 0 || lvl < bestResistance) {
			bestResistance = lvl
		}
	}
	if bestResistance > 0 {
		return bestResistance + atr*0.5
	}
	return entry + atr*1.5
}

// computeLongTakeProfits finds TP1 (conservative) and TP2 (aggressive) for longs.
func computeLongTakeProfits(entry, atr float64, resistanceLevels []float64, snap *IndicatorSnapshot) (tp1, tp2 float64) {
	// Sort resistance levels above entry (ascending)
	var above []float64
	for _, lvl := range resistanceLevels {
		if lvl > entry {
			above = append(above, lvl)
		}
	}
	sortFloat64Asc(above)

	// TP1: nearest resistance above entry
	if len(above) >= 1 {
		tp1 = above[0]
	} else {
		tp1 = entry + atr*2.0
	}

	// TP2: second resistance or Bollinger upper
	if len(above) >= 2 {
		tp2 = above[1]
	} else if snap.Bollinger != nil && snap.Bollinger.Upper > entry {
		tp2 = snap.Bollinger.Upper
	} else {
		tp2 = entry + atr*3.0
	}

	// Ensure TP2 > TP1
	if tp2 <= tp1 {
		tp2 = tp1 + atr
	}
	return
}

// computeShortTakeProfits finds TP1 (conservative) and TP2 (aggressive) for shorts.
func computeShortTakeProfits(entry, atr float64, supportLevels []float64, snap *IndicatorSnapshot) (tp1, tp2 float64) {
	// Sort support levels below entry (descending = nearest first)
	var below []float64
	for _, lvl := range supportLevels {
		if lvl < entry {
			below = append(below, lvl)
		}
	}
	sortFloat64Desc(below)

	// TP1: nearest support below entry
	if len(below) >= 1 {
		tp1 = below[0]
	} else {
		tp1 = entry - atr*2.0
	}

	// TP2: second support or Bollinger lower
	if len(below) >= 2 {
		tp2 = below[1]
	} else if snap.Bollinger != nil && snap.Bollinger.Lower < entry {
		tp2 = snap.Bollinger.Lower
	} else {
		tp2 = entry - atr*3.0
	}

	// Ensure TP2 < TP1 (further from entry)
	if tp2 >= tp1 {
		tp2 = tp1 - atr
	}
	return
}

// sortFloat64Asc sorts a float64 slice in ascending order.
func sortFloat64Asc(s []float64) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// sortFloat64Desc sorts a float64 slice in descending order.
func sortFloat64Desc(s []float64) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] > s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
