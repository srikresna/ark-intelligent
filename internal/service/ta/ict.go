package ta

import "time"

// ---------------------------------------------------------------------------
// ICT — Inner Circle Trader Structural Analysis
// ---------------------------------------------------------------------------
// Implements Fair Value Gap (FVG), Order Block (OB), Breaker Block,
// Liquidity Sweep, and Killzone detection per the TASK-035 specification.

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ICTResult holds all ICT-derived analysis for a set of OHLCV bars.
type ICTResult struct {
	FairValueGaps   []FVG            // all detected FVGs, sorted newest first
	OrderBlocks     []OrderBlock     // all detected OBs, sorted newest first
	BreakerBlocks   []OrderBlock     // mitigated OBs that flipped polarity
	LiquidityLevels []LiquidityLevel // equal highs/lows (sweep targets)
	Killzone        string           // "ASIAN", "LONDON", "NY", "OFF"
	PremiumZone     bool             // price in premium (above 50% of last range)
	DiscountZone    bool             // price in discount (below 50% of last range)
	Equilibrium     float64          // 50% level of last significant swing
}

// FVG represents a Fair Value Gap — a three-candle imbalance.
type FVG struct {
	High     float64 // top of the gap
	Low      float64 // bottom of the gap
	Type     string  // "BULLISH" or "BEARISH"
	BarIndex int     // index of the middle candle (newest-first)
	Filled   bool    // true if price has returned to close the gap
	FillPct  float64 // 0-100 how much of the gap has been filled
	Midpoint float64 // (High+Low)/2
}

// OrderBlock represents a supply/demand candle before an impulse.
type OrderBlock struct {
	High      float64
	Low       float64
	Type      string // "BULLISH" or "BEARISH"
	BarIndex  int    // index of the OB candle (newest-first)
	Mitigated bool   // price returned to OB zone
	Broken    bool   // price closed through OB (becomes a breaker)
	Strength  int    // 1–3 based on impulse size relative to ATR
}

// LiquidityLevel represents a cluster of equal highs or equal lows.
type LiquidityLevel struct {
	Price  float64
	Type   string // "BUY_SIDE" (equal highs) or "SELL_SIDE" (equal lows)
	Swept  bool   // price broke briefly then closed back inside
	Count  int    // number of pivots clustered at this level
}

// ---------------------------------------------------------------------------
// CalcICT — main entry point
// ---------------------------------------------------------------------------

// CalcICT computes all ICT structural elements for the given bars.
// bars: newest-first, minimum 20 bars required.
// atr: pre-computed ATR(14) for filtering noise.
// Returns nil if insufficient data.
func CalcICT(bars []OHLCV, atr float64) *ICTResult {
	if len(bars) < 20 || atr <= 0 {
		return nil
	}

	result := &ICTResult{}

	// Killzone based on most recent bar time
	result.Killzone = detectKillzone(bars[0].Date)

	// Equilibrium / Premium / Discount zones
	result.Equilibrium, result.PremiumZone, result.DiscountZone = calcEquilibrium(bars)

	// FVGs
	result.FairValueGaps = detectFVGs(bars, atr)

	// Order Blocks + Breaker Blocks
	obs, breakers := detectOrderBlocks(bars, atr)
	result.OrderBlocks = obs
	result.BreakerBlocks = breakers

	// Liquidity levels
	result.LiquidityLevels = detectLiquidityLevels(bars, atr)

	return result
}

// ---------------------------------------------------------------------------
// Killzone
// ---------------------------------------------------------------------------

// detectKillzone maps a bar timestamp (UTC) to an ICT session name.
func detectKillzone(t time.Time) string {
	h := t.UTC().Hour()
	switch {
	case h >= 0 && h < 3:
		return "ASIAN"
	case h >= 8 && h < 10:
		return "LONDON"
	case h >= 13 && h < 15:
		return "NY"
	default:
		return "OFF"
	}
}

// ---------------------------------------------------------------------------
// Equilibrium / Premium / Discount
// ---------------------------------------------------------------------------

// calcEquilibrium computes the 50% midpoint of the last significant swing
// and determines whether the current price is in premium or discount.
func calcEquilibrium(bars []OHLCV) (eq float64, premium, discount bool) {
	// Use the high/low range over the last 50 bars (or all if fewer)
	n := len(bars)
	if n > 50 {
		n = 50
	}
	high := bars[0].High
	low := bars[0].Low
	for i := 1; i < n; i++ {
		if bars[i].High > high {
			high = bars[i].High
		}
		if bars[i].Low < low {
			low = bars[i].Low
		}
	}
	eq = (high + low) / 2.0
	price := bars[0].Close
	premium = price > eq
	discount = price < eq
	return
}

// ---------------------------------------------------------------------------
// Fair Value Gap Detection
// ---------------------------------------------------------------------------

// detectFVGs scans bars for three-candle imbalance patterns.
// Returns up to 10 FVGs (5 bullish + 5 bearish), newest first.
func detectFVGs(bars []OHLCV, atr float64) []FVG {
	var bullish, bearish []FVG
	minSize := atr * 0.1

	// Need at least 3 bars; iterate middle candle indices
	for i := 1; i < len(bars)-1; i++ {
		prev := bars[i+1] // older
		next := bars[i-1] // newer

		// Bullish FVG: gap between prev.High and next.Low
		if prev.High < next.Low {
			gap := next.Low - prev.High
			if gap >= minSize && len(bullish) < 5 {
				fvg := FVG{
					Low:      prev.High,
					High:     next.Low,
					Type:     "BULLISH",
					BarIndex: i,
					Midpoint: (prev.High + next.Low) / 2,
				}
				// Check fill: has any bar[0..i-1] closed into this gap?
				fvg.Filled, fvg.FillPct = checkFVGFill(bars[:i], fvg, "BULLISH")
				bullish = append(bullish, fvg)
			}
		}

		// Bearish FVG: gap between next.High and prev.Low
		if next.High < prev.Low {
			gap := prev.Low - next.High
			if gap >= minSize && len(bearish) < 5 {
				fvg := FVG{
					Low:      next.High,
					High:     prev.Low,
					Type:     "BEARISH",
					BarIndex: i,
					Midpoint: (next.High + prev.Low) / 2,
				}
				fvg.Filled, fvg.FillPct = checkFVGFill(bars[:i], fvg, "BEARISH")
				bearish = append(bearish, fvg)
			}
		}
	}

	// Merge newest-first (bullish and bearish interleaved by index)
	return mergeFVGs(bullish, bearish)
}

// checkFVGFill determines how much of a FVG has been filled by subsequent bars.
// recentBars: the bars newer than the middle candle (bars[0..i-1]).
func checkFVGFill(recentBars []OHLCV, fvg FVG, fvgType string) (filled bool, fillPct float64) {
	if len(recentBars) == 0 {
		return false, 0
	}
	gapSize := fvg.High - fvg.Low
	if gapSize <= 0 {
		return false, 0
	}

	var deepest float64
	switch fvgType {
	case "BULLISH":
		// Price returning into bullish FVG means a bar's Low goes below fvg.High
		deepest = fvg.High // start from top of gap
		for _, b := range recentBars {
			if b.Low <= fvg.High {
				penetration := fvg.High - b.Low
				if penetration > (fvg.High - deepest) {
					deepest = b.Low
				}
				if deepest < fvg.Low {
					deepest = fvg.Low
				}
			}
		}
		fillPct = (fvg.High - deepest) / gapSize * 100
		if fillPct < 0 {
			fillPct = 0
		}
		if fillPct > 100 {
			fillPct = 100
		}
		filled = fillPct >= 100
	case "BEARISH":
		// Price returning into bearish FVG means a bar's High goes above fvg.Low
		deepest = fvg.Low
		for _, b := range recentBars {
			if b.High >= fvg.Low {
				if b.High > deepest {
					deepest = b.High
				}
				if deepest > fvg.High {
					deepest = fvg.High
				}
			}
		}
		fillPct = (deepest - fvg.Low) / gapSize * 100
		if fillPct < 0 {
			fillPct = 0
		}
		if fillPct > 100 {
			fillPct = 100
		}
		filled = fillPct >= 100
	}
	return
}

// mergeFVGs merges two FVG slices and returns them sorted by BarIndex ascending
// (lowest BarIndex = most recent candle position).
func mergeFVGs(bullish, bearish []FVG) []FVG {
	all := append(bullish, bearish...)
	// Sort by BarIndex ascending (lower index = newer)
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].BarIndex < all[j-1].BarIndex; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	return all
}

// ---------------------------------------------------------------------------
// Order Block Detection
// ---------------------------------------------------------------------------

// detectOrderBlocks identifies supply (bearish OB) and demand (bullish OB) zones.
// Returns (order blocks, breaker blocks) — each sorted newest first.
func detectOrderBlocks(bars []OHLCV, atr float64) (obs, breakers []OrderBlock) {
	bullishOBs := detectBullishOBs(bars, atr)
	bearishOBs := detectBearishOBs(bars, atr)

	for _, ob := range bullishOBs {
		if ob.Broken {
			breakers = append(breakers, ob)
		} else {
			obs = append(obs, ob)
		}
	}
	for _, ob := range bearishOBs {
		if ob.Broken {
			breakers = append(breakers, ob)
		} else {
			obs = append(obs, ob)
		}
	}
	return
}

// detectBullishOBs finds bearish candles (Close < Open) just before a bullish impulse.
func detectBullishOBs(bars []OHLCV, atr float64) []OrderBlock {
	var result []OrderBlock
	n := len(bars)

	for i := n - 2; i >= 2; i-- {
		// bars is newest-first: bars[i] is older, bars[i-1] is newer
		// Find a bearish candle
		if bars[i].Close >= bars[i].Open {
			continue
		}

		// Check for bullish impulse in bars[i-1] going backwards (i.e., newer bars)
		strength := bullishImpulseStrength(bars, i-1, atr)
		if strength == 0 {
			continue
		}

		ob := OrderBlock{
			High:     bars[i].High,
			Low:      bars[i].Low,
			Type:     "BULLISH",
			BarIndex: i,
			Strength: strength,
		}

		// Check mitigation and broken status using newer bars (bars[0..i-1])
		ob.Mitigated, ob.Broken = checkOBStatus(bars[:i], ob)

		result = append(result, ob)
		if len(result) >= 5 {
			break
		}
	}
	return result
}

// detectBearishOBs finds bullish candles (Close > Open) just before a bearish impulse.
func detectBearishOBs(bars []OHLCV, atr float64) []OrderBlock {
	var result []OrderBlock
	n := len(bars)

	for i := n - 2; i >= 2; i-- {
		// Find a bullish candle
		if bars[i].Close <= bars[i].Open {
			continue
		}

		// Check for bearish impulse in newer bars
		strength := bearishImpulseStrength(bars, i-1, atr)
		if strength == 0 {
			continue
		}

		ob := OrderBlock{
			High:     bars[i].High,
			Low:      bars[i].Low,
			Type:     "BEARISH",
			BarIndex: i,
			Strength: strength,
		}

		ob.Mitigated, ob.Broken = checkOBStatus(bars[:i], ob)

		result = append(result, ob)
		if len(result) >= 5 {
			break
		}
	}
	return result
}

// bullishImpulseStrength returns the impulse strength (1-3) if bars starting
// at startIdx represent a bullish impulse; returns 0 if not an impulse.
// startIdx is newest-first index (lower = newer).
func bullishImpulseStrength(bars []OHLCV, startIdx int, atr float64) int {
	if startIdx < 0 || startIdx >= len(bars) {
		return 0
	}

	// Check for single large bullish bar
	singleBar := bars[startIdx]
	barSize := singleBar.High - singleBar.Low
	if singleBar.Close > singleBar.Open && barSize > 1.5*atr {
		return impulseStrengthFromSize(barSize, atr)
	}

	// Check for 3+ consecutive bullish bars
	count := 0
	totalMove := 0.0
	for i := startIdx; i >= 0 && count < 5; i-- {
		if bars[i].Close > bars[i].Open {
			count++
			totalMove += bars[i].Close - bars[i].Open
		} else {
			break
		}
	}
	if count >= 3 {
		return impulseStrengthFromSize(totalMove, atr)
	}

	return 0
}

// bearishImpulseStrength mirrors bullishImpulseStrength for bearish impulses.
func bearishImpulseStrength(bars []OHLCV, startIdx int, atr float64) int {
	if startIdx < 0 || startIdx >= len(bars) {
		return 0
	}

	singleBar := bars[startIdx]
	barSize := singleBar.High - singleBar.Low
	if singleBar.Close < singleBar.Open && barSize > 1.5*atr {
		return impulseStrengthFromSize(barSize, atr)
	}

	count := 0
	totalMove := 0.0
	for i := startIdx; i >= 0 && count < 5; i-- {
		if bars[i].Close < bars[i].Open {
			count++
			totalMove += bars[i].Open - bars[i].Close
		} else {
			break
		}
	}
	if count >= 3 {
		return impulseStrengthFromSize(totalMove, atr)
	}

	return 0
}

// impulseStrengthFromSize converts a move size relative to ATR into strength 1-3.
func impulseStrengthFromSize(size, atr float64) int {
	switch {
	case size > 2*atr:
		return 3
	case size > atr:
		return 2
	default:
		return 1
	}
}

// checkOBStatus determines whether an OB has been mitigated or broken
// by the more-recent bars (recentBars = bars[0..obIndex-1]).
func checkOBStatus(recentBars []OHLCV, ob OrderBlock) (mitigated, broken bool) {
	for _, b := range recentBars {
		switch ob.Type {
		case "BULLISH":
			// Mitigated: price returns to touch the OB zone
			if b.Low <= ob.High && b.High >= ob.Low {
				mitigated = true
			}
			// Broken: price closes BELOW the OB low
			if b.Close < ob.Low {
				broken = true
			}
		case "BEARISH":
			// Mitigated: price returns to touch the OB zone
			if b.High >= ob.Low && b.Low <= ob.High {
				mitigated = true
			}
			// Broken: price closes ABOVE the OB high
			if b.Close > ob.High {
				broken = true
			}
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Liquidity Level Detection
// ---------------------------------------------------------------------------

// detectLiquidityLevels finds clusters of equal highs (buy-side liquidity)
// and equal lows (sell-side liquidity), and checks if they've been swept.
func detectLiquidityLevels(bars []OHLCV, atr float64) []LiquidityLevel {
	tolerance := atr * 0.15
	var result []LiquidityLevel

	highs := swingHighs(bars)
	lows := swingLows(bars)

	// Cluster equal highs → BUY_SIDE liquidity
	result = append(result, clusterLevels(bars, highs, "BUY_SIDE", tolerance)...)

	// Cluster equal lows → SELL_SIDE liquidity
	result = append(result, clusterLevels(bars, lows, "SELL_SIDE", tolerance)...)

	return result
}

// swingHighs returns the High values of swing-high candles (higher than neighbours).
func swingHighs(bars []OHLCV) []float64 {
	var highs []float64
	for i := 1; i < len(bars)-1; i++ {
		// bars is newest-first; bars[i] is between bars[i-1] (newer) and bars[i+1] (older)
		if bars[i].High > bars[i-1].High && bars[i].High > bars[i+1].High {
			highs = append(highs, bars[i].High)
		}
	}
	return highs
}

// swingLows returns the Low values of swing-low candles.
func swingLows(bars []OHLCV) []float64 {
	var lows []float64
	for i := 1; i < len(bars)-1; i++ {
		if bars[i].Low < bars[i-1].Low && bars[i].Low < bars[i+1].Low {
			lows = append(lows, bars[i].Low)
		}
	}
	return lows
}

// clusterLevels groups pivot prices within tolerance and creates LiquidityLevel entries.
func clusterLevels(bars []OHLCV, pivots []float64, lvlType string, tolerance float64) []LiquidityLevel {
	var result []LiquidityLevel
	used := make([]bool, len(pivots))

	for i, p := range pivots {
		if used[i] {
			continue
		}
		cluster := []float64{p}
		used[i] = true
		for j := i + 1; j < len(pivots); j++ {
			if !used[j] && abs64(pivots[j]-p) <= tolerance {
				cluster = append(cluster, pivots[j])
				used[j] = true
			}
		}
		if len(cluster) < 3 {
			continue
		}
		// Average price of cluster
		avg := mean64(cluster)
		ll := LiquidityLevel{
			Price: avg,
			Type:  lvlType,
			Count: len(cluster),
		}
		// Check if swept: any bar briefly exceeded the level but closed back
		ll.Swept = isSept(bars, avg, lvlType)
		result = append(result, ll)
	}
	return result
}

// isSept checks whether price swept through the liquidity level but closed back.
func isSept(bars []OHLCV, price float64, lvlType string) bool {
	for _, b := range bars {
		switch lvlType {
		case "BUY_SIDE":
			// Wick above level but closed below
			if b.High > price && b.Close < price {
				return true
			}
		case "SELL_SIDE":
			// Wick below level but closed above
			if b.Low < price && b.Close > price {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func mean64(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
