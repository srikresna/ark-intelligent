package ta

// ict.go — ICT (Inner Circle Trader) Fair Value Gap and Order Block detection.
//
// Implements:
//   - FVG (Fair Value Gap): 3-candle imbalance zones
//   - Order Blocks: last opposing candle before impulse moves
//
// Bars are always newest-first (index 0 = most recent bar).

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// FVGResult represents a Fair Value Gap — a 3-candle imbalance zone.
type FVGResult struct {
	Direction string  // "BULLISH" or "BEARISH"
	HighEdge  float64 // upper boundary of the gap
	LowEdge   float64 // lower boundary of the gap
	Filled    bool    // true if price has traded through the gap
	BarIndex  int     // index of the middle candle (in newest-first input)
}

// OrderBlockResult represents an ICT Order Block zone.
type OrderBlockResult struct {
	Direction   string  // "BULLISH" or "BEARISH"
	High        float64 // top of the order block candle
	Low         float64 // bottom of the order block candle
	Strength    int     // 1–5 rating based on impulse size relative to ATR
	Mitigated   bool    // true if price returned into the OB zone
}

// ICTResult bundles Fair Value Gaps and Order Blocks.
type ICTResult struct {
	FVGs        []FVGResult        // up to 5 most recent unfilled FVGs
	OrderBlocks []OrderBlockResult // up to 3 most recent order blocks
}

// ---------------------------------------------------------------------------
// DetectFVG
// ---------------------------------------------------------------------------

// DetectFVG scans bars (newest-first) for 3-candle Fair Value Gap patterns.
//
// Bullish FVG: bars[i+2].High < bars[i].Low
//   (gap between bar[i]'s low and bar[i+2]'s high — bar[i] is newest of the three)
//
// Bearish FVG: bars[i+2].Low > bars[i].High
//   (gap between bar[i]'s high and bar[i+2]'s low)
//
// Minimum gap size = 0.2 × ATR.
// Returns up to 5 most recent unfilled (or partially filled) FVGs, newest-first.
func DetectFVG(bars []OHLCV, atr float64) []FVGResult {
	if len(bars) < 3 {
		return nil
	}

	minGap := atr * 0.2

	var results []FVGResult
	currentPrice := bars[0].Close

	// Scan from newest (index 0) going older
	// bars[i] = newest of the triplet, bars[i+2] = oldest of the triplet
	for i := 0; i+2 < len(bars); i++ {
		newest := bars[i]
		oldest := bars[i+2]

		// Bullish FVG: gap between oldest.High and newest.Low
		if newest.Low > oldest.High {
			gap := newest.Low - oldest.High
			if gap >= minGap {
				filled := currentPrice <= newest.Low && currentPrice >= oldest.High
				results = append(results, FVGResult{
					Direction: "BULLISH",
					HighEdge:  newest.Low,
					LowEdge:   oldest.High,
					Filled:    filled,
					BarIndex:  i,
				})
			}
		}

		// Bearish FVG: gap between newest.High and oldest.Low
		if newest.High < oldest.Low {
			gap := oldest.Low - newest.High
			if gap >= minGap {
				filled := currentPrice >= newest.High && currentPrice <= oldest.Low
				results = append(results, FVGResult{
					Direction: "BEARISH",
					HighEdge:  oldest.Low,
					LowEdge:   newest.High,
					Filled:    filled,
					BarIndex:  i,
				})
			}
		}

		if len(results) >= 5 {
			break
		}
	}

	return results
}

// ---------------------------------------------------------------------------
// DetectOrderBlocks
// ---------------------------------------------------------------------------

// DetectOrderBlocks scans bars (newest-first) for ICT Order Block candles.
//
// Bullish OB: the last bearish candle (close < open) immediately before a bullish
// impulse move (next candle's range ≥ 1.5 × ATR).
//
// Bearish OB: the last bullish candle (close > open) immediately before a bearish
// impulse move.
//
// A block is Mitigated when subsequent price trades back into the OB zone (High–Low range).
// Strength is rated 1–5 based on impulse size in ATR multiples.
//
// Returns up to 3 most recent order blocks, newest-first.
func DetectOrderBlocks(bars []OHLCV, atr float64) []OrderBlockResult {
	if len(bars) < 3 || atr <= 0 {
		return nil
	}

	impulseThreshold := atr * 1.5
	currentPrice := bars[0].Close

	var results []OrderBlockResult

	// Scan from newest to oldest
	// bars[i] = the potential OB candle (newest of the pair)
	// bars[i+1] = the impulse candle (one bar before OB in time = older bar in newest-first)
	// Wait — newest-first: bars[0]=today, bars[1]=yesterday
	// So OB candle is bars[i+1] and impulse is bars[i] (the newer one follows the OB)
	for i := 0; i+1 < len(bars); i++ {
		impulse := bars[i]        // the impulse candle (newer)
		obCandle := bars[i+1]    // the potential order block (older, just before impulse)

		impulseRange := impulse.High - impulse.Low
		if impulseRange < impulseThreshold {
			continue
		}

		isBullishImpulse := impulse.Close > impulse.Open
		isBearishOBCandle := obCandle.Close < obCandle.Open
		isBearishImpulse := impulse.Close < impulse.Open
		isBullishOBCandle := obCandle.Close > obCandle.Open

		strength := impulseStrength(impulseRange, atr)

		if isBullishImpulse && isBearishOBCandle {
			// Bullish OB: bearish candle before bullish impulse
			mitigated := currentPrice >= obCandle.Low && currentPrice <= obCandle.High
			results = append(results, OrderBlockResult{
				Direction: "BULLISH",
				High:      obCandle.High,
				Low:       obCandle.Low,
				Strength:  strength,
				Mitigated: mitigated,
			})
		} else if isBearishImpulse && isBullishOBCandle {
			// Bearish OB: bullish candle before bearish impulse
			mitigated := currentPrice >= obCandle.Low && currentPrice <= obCandle.High
			results = append(results, OrderBlockResult{
				Direction: "BEARISH",
				High:      obCandle.High,
				Low:       obCandle.Low,
				Strength:  strength,
				Mitigated: mitigated,
			})
		}

		if len(results) >= 3 {
			break
		}
	}

	return results
}

// impulseStrength rates impulse size on a 1–5 scale relative to ATR.
func impulseStrength(impulseRange, atr float64) int {
	if atr <= 0 {
		return 1
	}
	multiple := impulseRange / atr
	switch {
	case multiple >= 4.0:
		return 5
	case multiple >= 3.0:
		return 4
	case multiple >= 2.0:
		return 3
	case multiple >= 1.5:
		return 2
	default:
		return 1
	}
}
