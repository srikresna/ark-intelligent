package elliott

import "github.com/arkcode369/ark-intelligent/internal/service/ta"

// ---------------------------------------------------------------------------
// ZigZag swing-point detection
// ---------------------------------------------------------------------------

// detectZigZag identifies alternating swing-high / swing-low pivot points in
// a slice of OHLCV bars (newest-first ordering).
//
// The algorithm works on an oldest-first copy internally to make it easy to
// reason about left→right direction.  A new pivot is accepted only when price
// has reversed at least minRetracement (e.g. 0.05 = 5%) from the previous
// pivot, ensuring minor noise is filtered out.
//
// The returned slice is in chronological order (oldest first).  Each element
// is expressed as an index into the original newest-first bars slice.
func detectZigZag(bars []ta.OHLCV, minRetracement float64) []SwingPoint {
	if len(bars) < 3 {
		return nil
	}

	// Work oldest-first.
	asc := reverseOHLCV(bars)
	n := len(asc)

	// current tracks the running extreme while we are looking for a reversal.
	type candidate struct {
		idx    int
		price  float64
		isHigh bool
	}

	// Seed: determine the initial pivot direction.
	// If the first move is upward (next bar higher), the initial pivot is a LOW.
	// If the first move is downward, the initial pivot is a HIGH.
	firstMoveUp := isHigherHigh(asc, 0)
	// Seed is the OPPOSITE of the move direction: if price goes UP from here,
	// the starting point is a swing LOW; if price goes DOWN, it is a swing HIGH.
	initIsHigh := !firstMoveUp
	cur := candidate{
		idx:    0,
		price:  asc[0].Low,
		isHigh: false,
	}
	if initIsHigh {
		cur.price = asc[0].High
		cur.isHigh = true
	}

	var pivots []SwingPoint

	// Always record the very first pivot (the initial price extreme before the
	// first confirmed reversal).  This ensures the W1 start is captured even
	// if it occurs on the first bar.
	pivots = append(pivots, SwingPoint{
		Index:  newestFirstIdx(cur.idx, n),
		Price:  cur.price,
		IsHigh: cur.isHigh,
	})

	for i := 1; i < n; i++ {
		bar := asc[i]

		if cur.isHigh {
			// Looking for a new high or a reversal downward.
			if bar.High >= cur.price {
				// Extend the current swing high — update the already-recorded pivot.
				cur.price = bar.High
				cur.idx = i
				pivots[len(pivots)-1] = SwingPoint{
					Index:  newestFirstIdx(cur.idx, n),
					Price:  cur.price,
					IsHigh: true,
				}
			} else {
				// Check for reversal: did price drop enough?
				drop := (cur.price - bar.Low) / cur.price
				if drop >= minRetracement {
					// Start a new low candidate and record it tentatively.
					cur = candidate{idx: i, price: bar.Low, isHigh: false}
					pivots = append(pivots, SwingPoint{
						Index:  newestFirstIdx(cur.idx, n),
						Price:  cur.price,
						IsHigh: false,
					})
				}
			}
		} else {
			// Looking for a new low or a reversal upward.
			if bar.Low <= cur.price {
				// Extend the current swing low — update the already-recorded pivot.
				cur.price = bar.Low
				cur.idx = i
				pivots[len(pivots)-1] = SwingPoint{
					Index:  newestFirstIdx(cur.idx, n),
					Price:  cur.price,
					IsHigh: false,
				}
			} else {
				// Check for reversal.
				rise := (bar.High - cur.price) / cur.price
				if rise >= minRetracement {
					// Start a new high candidate and record it tentatively.
					cur = candidate{idx: i, price: bar.High, isHigh: true}
					pivots = append(pivots, SwingPoint{
						Index:  newestFirstIdx(cur.idx, n),
						Price:  cur.price,
						IsHigh: true,
					})
				}
			}
		}
	}

	return pivots
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// reverseOHLCV returns a new slice with order reversed (oldest-first).
func reverseOHLCV(bars []ta.OHLCV) []ta.OHLCV {
	n := len(bars)
	out := make([]ta.OHLCV, n)
	for i, b := range bars {
		out[n-1-i] = b
	}
	return out
}

// newestFirstIdx converts an oldest-first index to the corresponding
// newest-first index given the total number of bars n.
func newestFirstIdx(ascIdx, n int) int {
	return n - 1 - ascIdx
}

// isHigherHigh returns true when bar[1].High > bar[0].High (first two bars in
// ascending order), used to seed the initial ZigZag direction.
func isHigherHigh(asc []ta.OHLCV, start int) bool {
	if start+1 >= len(asc) {
		return true
	}
	return asc[start+1].High > asc[start].High
}
