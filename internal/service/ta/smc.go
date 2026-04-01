package ta

// smc.go — Smart Money Concepts (SMC) market structure analysis.
//
// Implements:
//   - BOS (Break of Structure): confirmation of trend continuation
//   - CHOCH (Change of Character): reversal signal
//   - Market Structure state machine: BULLISH / BEARISH / RANGING
//   - Premium / Discount / Equilibrium zones
//   - Internal liquidity identification
//
// Bars are always newest-first (index 0 = most recent bar).

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// MarketStructure represents the current market structure classification.
type MarketStructure string

const (
	// StructureBullish indicates a bullish market structure (HH + HL pattern).
	StructureBullish MarketStructure = "BULLISH"
	// StructureBearish indicates a bearish market structure (LH + LL pattern).
	StructureBearish MarketStructure = "BEARISH"
	// StructureRanging indicates no clear directional structure.
	StructureRanging MarketStructure = "RANGING"
)

// StructureEvent records a BOS or CHOCH occurrence.
type StructureEvent struct {
	Type     string  // "BOS" or "CHOCH"
	Dir      string  // "BULLISH" or "BEARISH"
	Price    float64 // price level that was broken
	BarIndex int     // index in bars slice (newest-first)
	Impulse  float64 // size of move after break in ATR multiples
}

// LiqRange describes an internal liquidity pool.
type LiqRange struct {
	High  float64
	Low   float64
	Type  string // "INTERNAL" or "EXTERNAL"
	Swept bool
}

// SMCResult holds the complete Smart Money Concepts analysis.
type SMCResult struct {
	Structure    MarketStructure  // current overall structure
	RecentBOS    []StructureEvent // last 5 BOS events, newest first
	RecentCHOCH  []StructureEvent // last 3 CHOCH events, newest first
	PremiumZone  float64          // level above which is premium
	DiscountZone float64          // level below which is discount
	Equilibrium  float64          // 50% of last significant swing
	CurrentZone  string           // "PREMIUM", "DISCOUNT", "EQUILIBRIUM"
	InternalLiq  []LiqRange       // internal liquidity pools
	Trend        string           // "BULLISH", "BEARISH", "RANGING"
	LastSwingHigh float64
	LastSwingLow  float64
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

type swingPoint struct {
	idx   int     // index in oldest-first working array
	price float64 // swing high or low price
	isHigh bool
}

// detectSwingPoints finds swing highs and lows in oldest-first bars.
// A swing high at index i: bars[i].High > all bars in [i-n, i-1] and [i+1, i+n].
// A swing low at index i: bars[i].Low  < all bars in [i-n, i-1] and [i+1, i+n].
func detectSwingPoints(bars []OHLCV, n int) []swingPoint {
	count := len(bars)
	if count < 2*n+1 {
		return nil
	}

	var swings []swingPoint
	for i := n; i < count-n; i++ {
		highCandidate := bars[i].High
		lowCandidate := bars[i].Low
		isSwingHigh := true
		isSwingLow := true

		for j := 1; j <= n; j++ {
			if bars[i-j].High >= highCandidate {
				isSwingHigh = false
			}
			if bars[i+j].High >= highCandidate {
				isSwingHigh = false
			}
			if bars[i-j].Low <= lowCandidate {
				isSwingLow = false
			}
			if bars[i+j].Low <= lowCandidate {
				isSwingLow = false
			}
		}

		if isSwingHigh {
			swings = append(swings, swingPoint{idx: i, price: highCandidate, isHigh: true})
		}
		if isSwingLow {
			swings = append(swings, swingPoint{idx: i, price: lowCandidate, isHigh: false})
		}
	}
	return swings
}

// ---------------------------------------------------------------------------
// CalcSMC
// ---------------------------------------------------------------------------

// CalcSMC computes SMC market structure for the given bars.
// bars: newest-first. Minimum 30 bars recommended.
// atr: ATR(14) value for impulse sizing (pass 0 if unknown — impulse will be 0).
//
// Returns nil if insufficient data.
func CalcSMC(bars []OHLCV, atr float64) *SMCResult {
	if len(bars) < 15 {
		return nil
	}

	// Work oldest-first for sequential analysis
	asc := reverseOHLCV(bars)
	n := len(asc)

	const pivotN = 3 // bars on each side for swing detection
	swings := detectSwingPoints(asc, pivotN)
	if len(swings) < 2 {
		return nil
	}

	// Separate highs and lows (in oldest-first order)
	var highs, lows []swingPoint
	for _, s := range swings {
		if s.isHigh {
			highs = append(highs, s)
		} else {
			lows = append(lows, s)
		}
	}

	if len(highs) == 0 || len(lows) == 0 {
		return nil
	}

	// -----------------------------------------------------------------------
	// BOS / CHOCH detection via state machine
	// -----------------------------------------------------------------------
	// State: track last swing high and low, current structure
	type structureState struct {
		structure  MarketStructure
		lastHigh   float64
		lastHighIdx int
		lastLow    float64
		lastLowIdx  int
	}

	state := structureState{
		structure:   StructureRanging,
		lastHigh:    highs[0].price,
		lastHighIdx: highs[0].idx,
		lastLow:     lows[0].price,
		lastLowIdx:  lows[0].idx,
	}

	// Track recent swing highs and lows for HH/HL/LH/LL detection
	type swing struct{ price float64; idx int }
	var recentHighs []swing
	var recentLows []swing

	// Accumulate all detected swings
	for _, h := range highs {
		recentHighs = append(recentHighs, swing{h.price, h.idx})
	}
	for _, l := range lows {
		recentLows = append(recentLows, swing{l.price, l.idx})
	}

	// Determine initial structure from first few swings
	if len(recentHighs) >= 2 && len(recentLows) >= 2 {
		// Sort by index
		lastH := recentHighs[len(recentHighs)-1]
		prevH := recentHighs[len(recentHighs)-2]
		lastL := recentLows[len(recentLows)-1]
		prevL := recentLows[len(recentLows)-2]

		hhhl := lastH.price > prevH.price && lastL.price > prevL.price
		lhll := lastH.price < prevH.price && lastL.price < prevL.price

		if hhhl {
			state.structure = StructureBullish
		} else if lhll {
			state.structure = StructureBearish
		}
	}

	var bosList []StructureEvent
	var chochList []StructureEvent

	currentPrice := asc[n-1].Close

	// Scan bars in oldest-first order for BOS/CHOCH
	for i := pivotN; i < n; i++ {
		bar := asc[i]
		impulse := 0.0
		if atr > 0 {
			impulse = (bar.Close - asc[max0(0, i-1)].Close) / atr
			if impulse < 0 {
				impulse = -impulse
			}
		}

		// Check for bullish BOS: close breaks above last swing high
		if bar.Close > state.lastHigh && state.lastHighIdx < i {
			evt := StructureEvent{
				Type:     "BOS",
				Dir:      "BULLISH",
				Price:    state.lastHigh,
				BarIndex: n - 1 - i, // convert to newest-first
				Impulse:  impulse,
			}
			// Is this a CHOCH? (structure was bearish)
			if state.structure == StructureBearish {
				evt.Type = "CHOCH"
				chochList = append(chochList, evt)
				state.structure = StructureBullish
			} else {
				bosList = append(bosList, evt)
				state.structure = StructureBullish
			}
			// Update last high reference
			for _, h := range highs {
				if h.idx < i && h.price > state.lastHigh {
					state.lastHigh = h.price
					state.lastHighIdx = h.idx
				}
			}
		}

		// Check for bearish BOS: close breaks below last swing low
		if bar.Close < state.lastLow && state.lastLowIdx < i {
			evt := StructureEvent{
				Type:     "BOS",
				Dir:      "BEARISH",
				Price:    state.lastLow,
				BarIndex: n - 1 - i, // convert to newest-first
				Impulse:  impulse,
			}
			// Is this a CHOCH? (structure was bullish)
			if state.structure == StructureBullish {
				evt.Type = "CHOCH"
				chochList = append(chochList, evt)
				state.structure = StructureBearish
			} else {
				bosList = append(bosList, evt)
				state.structure = StructureBearish
			}
			// Update last low reference
			for _, l := range lows {
				if l.idx < i && l.price < state.lastLow {
					state.lastLow = l.price
					state.lastLowIdx = l.idx
				}
			}
		}

		// Update running swing references
		for _, h := range highs {
			if h.idx == i && h.price > state.lastHigh {
				state.lastHigh = h.price
				state.lastHighIdx = h.idx
			}
		}
		for _, l := range lows {
			if l.idx == i && l.price < state.lastLow {
				state.lastLow = l.price
				state.lastLowIdx = l.idx
			}
		}
	}

	// -----------------------------------------------------------------------
	// Premium / Discount zones from last significant swing
	// -----------------------------------------------------------------------
	// Find most recent significant swing high and low
	lastSigHigh := highs[len(highs)-1].price
	lastSigLow := lows[len(lows)-1].price

	// Ensure high > low
	if lastSigHigh < lastSigLow {
		lastSigHigh, lastSigLow = lastSigLow, lastSigHigh
	}

	swingRange := lastSigHigh - lastSigLow
	equilibrium := lastSigLow + swingRange*0.5
	premiumZone := lastSigLow + swingRange*0.618 // golden pocket upper
	discountZone := lastSigLow + swingRange*0.382 // golden pocket lower

	// Classify current price zone
	currentZone := "EQUILIBRIUM"
	if currentPrice > equilibrium {
		currentZone = "PREMIUM"
	} else if currentPrice < equilibrium {
		currentZone = "DISCOUNT"
	}

	// -----------------------------------------------------------------------
	// Internal liquidity pools (equal highs / equal lows within 0.1% range)
	// -----------------------------------------------------------------------
	var liqRanges []LiqRange
	const eqThreshold = 0.001 // 0.1% tolerance for "equal" levels

	for i, h := range highs {
		for j := i + 1; j < len(highs); j++ {
			diff := (highs[j].price - h.price)
			if diff < 0 {
				diff = -diff
			}
			avg := (highs[j].price + h.price) / 2
			if avg > 0 && diff/avg < eqThreshold {
				swept := currentPrice > avg
				liqRanges = append(liqRanges, LiqRange{
					High:  max64(h.price, highs[j].price) * 1.0002,
					Low:   min64(h.price, highs[j].price) * 0.9998,
					Type:  "INTERNAL",
					Swept: swept,
				})
				break
			}
		}
	}

	for i, l := range lows {
		for j := i + 1; j < len(lows); j++ {
			diff := (lows[j].price - l.price)
			if diff < 0 {
				diff = -diff
			}
			avg := (lows[j].price + l.price) / 2
			if avg > 0 && diff/avg < eqThreshold {
				swept := currentPrice < avg
				liqRanges = append(liqRanges, LiqRange{
					High:  max64(l.price, lows[j].price) * 1.0002,
					Low:   min64(l.price, lows[j].price) * 0.9998,
					Type:  "INTERNAL",
					Swept: swept,
				})
				break
			}
		}
	}

	// -----------------------------------------------------------------------
	// Trim result lists (newest-first)
	// -----------------------------------------------------------------------
	reverseEvents(bosList)
	reverseEvents(chochList)

	maxBOS := 5
	if len(bosList) > maxBOS {
		bosList = bosList[:maxBOS]
	}
	maxCHOCH := 3
	if len(chochList) > maxCHOCH {
		chochList = chochList[:maxCHOCH]
	}
	if len(liqRanges) > 6 {
		liqRanges = liqRanges[:6]
	}

	// -----------------------------------------------------------------------
	// Build result
	// -----------------------------------------------------------------------
	trend := string(state.structure)

	return &SMCResult{
		Structure:     state.structure,
		RecentBOS:     bosList,
		RecentCHOCH:   chochList,
		PremiumZone:   premiumZone,
		DiscountZone:  discountZone,
		Equilibrium:   equilibrium,
		CurrentZone:   currentZone,
		InternalLiq:   liqRanges,
		Trend:         trend,
		LastSwingHigh: lastSigHigh,
		LastSwingLow:  lastSigLow,
	}
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

func reverseEvents(s []StructureEvent) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func max0(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
