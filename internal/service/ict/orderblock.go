package ict

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DetectOrderBlocks identifies Order Blocks and marks Breaker Blocks.
//
// Algorithm:
//   Bullish OB: The last BEARISH candle immediately before a swing low that
//               leads to a strong bullish impulse (subsequent bars make a new
//               swing high). If price later breaks below the OB, it becomes a
//               Breaker Block (Broken=true).
//   Bearish OB: The last BULLISH candle immediately before a swing high that
//               leads to a strong bearish impulse. Becomes a Breaker if price
//               later breaks above it.
//
// We only expose the most recent 5 order blocks to keep output readable.
func DetectOrderBlocks(bars []ta.OHLCV, swings []swingPoint) []OrderBlock {
	n := len(bars)
	if n < swingLookback*2+2 || len(swings) == 0 {
		return nil
	}

	// Work on a chronological copy.
	chron := make([]ta.OHLCV, n)
	for i, b := range bars {
		chron[n-1-i] = b
	}

	var obs []OrderBlock

	for _, sp := range swings {
		i := sp.barIndex
		if i < 2 || i >= n-1 {
			continue
		}

		if !sp.isHigh {
			// Swing Low → look for last BEARISH candle just before it.
			// A bearish candle: close < open.
			obIdx := -1
			for k := i - 1; k >= max0(i-5, 0); k-- {
				if chron[k].Close < chron[k].Open {
					obIdx = k
					break
				}
			}
			if obIdx < 0 {
				continue
			}
			ob := OrderBlock{
				Kind:     "BULLISH",
				Top:      chron[obIdx].High,
				Bottom:   chron[obIdx].Low,
				Volume:   chron[obIdx].Volume,
				BarIndex: obIdx,
			}
			// Mark as Breaker if any later bar closes below the OB bottom.
			for k := i + 1; k < n; k++ {
				if chron[k].Close < ob.Bottom {
					ob.Broken = true
					break
				}
			}
			obs = append(obs, ob)
		} else {
			// Swing High → look for last BULLISH candle just before it.
			obIdx := -1
			for k := i - 1; k >= max0(i-5, 0); k-- {
				if chron[k].Close > chron[k].Open {
					obIdx = k
					break
				}
			}
			if obIdx < 0 {
				continue
			}
			ob := OrderBlock{
				Kind:     "BEARISH",
				Top:      chron[obIdx].High,
				Bottom:   chron[obIdx].Low,
				Volume:   chron[obIdx].Volume,
				BarIndex: obIdx,
			}
			// Mark as Breaker if any later bar closes above the OB top.
			for k := i + 1; k < n; k++ {
				if chron[k].Close > ob.Top {
					ob.Broken = true
					break
				}
			}
			obs = append(obs, ob)
		}
	}

	// De-duplicate overlapping OBs (same kind within 0.05% of each other).
	obs = deduplicateOBs(obs)

	// Return only the most recent OBs.
	if len(obs) > 5 {
		obs = obs[len(obs)-5:]
	}
	return obs
}

// deduplicateOBs removes order blocks whose midpoint is within 0.05% of another.
func deduplicateOBs(obs []OrderBlock) []OrderBlock {
	if len(obs) <= 1 {
		return obs
	}
	result := []OrderBlock{obs[0]}
	for i := 1; i < len(obs); i++ {
		dup := false
		midI := (obs[i].Top + obs[i].Bottom) / 2
		for _, r := range result {
			midR := (r.Top + r.Bottom) / 2
			if r.Kind == obs[i].Kind && midR != 0 && abs64(midI-midR)/abs64(midR) < 0.0005 {
				dup = true
				break
			}
		}
		if !dup {
			result = append(result, obs[i])
		}
	}
	return result
}

func max0(a, b int) int {
	if a > b {
		return a
	}
	return b
}
