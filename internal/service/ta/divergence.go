package ta

import "math"

// ---------------------------------------------------------------------------
// RSI and MACD Divergence Detection
// ---------------------------------------------------------------------------

// Divergence describes a detected divergence between price and an indicator.
//
// Types:
// - REGULAR_BULLISH:  price lower low  + indicator higher low  → reversal up
// - REGULAR_BEARISH:  price higher high + indicator lower high  → reversal down
// - HIDDEN_BULLISH:   price higher low  + indicator lower low   → continuation up
// - HIDDEN_BEARISH:   price lower high  + indicator higher high → continuation down
type Divergence struct {
	Type        string  // "REGULAR_BULLISH", "REGULAR_BEARISH", "HIDDEN_BULLISH", "HIDDEN_BEARISH"
	Indicator   string  // "RSI" or "MACD"
	Strength    float64 // 0.0–1.0, how pronounced the divergence is
	Description string  // Human-readable description
}

// pivotPoint records a swing high or low found in a series.
type pivotPoint struct {
	idx   int     // index in the series (oldest-first)
	value float64 // value at the pivot
}

// findSwingHighs finds swing highs in a series using 5-bar lookback/forward.
// Input is oldest-first. Returns pivots in oldest-first order.
func findSwingHighs(series []float64, strength int) []pivotPoint {
	n := len(series)
	var pivots []pivotPoint
	for i := strength; i < n-strength; i++ {
		isHigh := true
		for j := i - strength; j < i; j++ {
			if series[j] >= series[i] {
				isHigh = false
				break
			}
		}
		if isHigh {
			for j := i + 1; j <= i+strength; j++ {
				if series[j] >= series[i] {
					isHigh = false
					break
				}
			}
		}
		if isHigh {
			pivots = append(pivots, pivotPoint{idx: i, value: series[i]})
		}
	}
	return pivots
}

// findSwingLows finds swing lows in a series using 5-bar lookback/forward.
// Input is oldest-first. Returns pivots in oldest-first order.
func findSwingLows(series []float64, strength int) []pivotPoint {
	n := len(series)
	var pivots []pivotPoint
	for i := strength; i < n-strength; i++ {
		isLow := true
		for j := i - strength; j < i; j++ {
			if series[j] <= series[i] {
				isLow = false
				break
			}
		}
		if isLow {
			for j := i + 1; j <= i+strength; j++ {
				if series[j] <= series[i] {
					isLow = false
					break
				}
			}
		}
		if isLow {
			pivots = append(pivots, pivotPoint{idx: i, value: series[i]})
		}
	}
	return pivots
}

// DetectDivergences detects RSI and MACD divergences between price and indicator.
// Bars, rsiSeries, and macdSeries are all newest-first and must have the same length
// (or rsiSeries/macdSeries may be shorter — aligned to the newest end of bars).
//
// The function uses close prices for the price series and looks for swing pivots
// using a 5-bar window. Minimum 5 bars between pivot pairs.
//
// Pass nil for rsiSeries or macdSeries to skip that indicator's divergence check.
func DetectDivergences(bars []OHLCV, rsiSeries []float64, macdSeries []float64) []Divergence {
	if len(bars) < 15 { // Need enough bars for meaningful pivots
		return nil
	}

	const pivotStrength = 5
	const minPivotDistance = 5

	// Build price series (closes, oldest-first)
	priceAsc := make([]float64, len(bars))
	for i, b := range bars {
		priceAsc[len(bars)-1-i] = b.Close
	}

	var divergences []Divergence

	// Helper to detect divergences between price and an indicator series
	detectForIndicator := func(indicatorName string, indSeriesNewest []float64) []Divergence {
		if len(indSeriesNewest) < 15 {
			return nil
		}

		// Convert indicator to oldest-first
		indAsc := reverseFloat64(indSeriesNewest)

		// Align: use the most recent min(len(priceAsc), len(indAsc)) bars
		pLen := len(priceAsc)
		iLen := len(indAsc)
		useLen := pLen
		if iLen < useLen {
			useLen = iLen
		}

		// Slice from the end (most recent)
		pSeries := priceAsc[pLen-useLen:]
		iSeries := indAsc[iLen-useLen:]

		// Find pivots
		priceHighs := findSwingHighs(pSeries, pivotStrength)
		priceLows := findSwingLows(pSeries, pivotStrength)
		indHighs := findSwingHighs(iSeries, pivotStrength)
		indLows := findSwingLows(iSeries, pivotStrength)

		var divs []Divergence

		// Regular Bearish: price higher high + indicator lower high
		if len(priceHighs) >= 2 && len(indHighs) >= 2 {
			for i := len(priceHighs) - 1; i >= 1; i-- {
				ph1 := priceHighs[i-1] // earlier
				ph2 := priceHighs[i]   // later (more recent)

				if ph2.idx-ph1.idx < minPivotDistance {
					continue
				}

				// Find indicator highs closest to these price pivot indices
				ih1 := findClosestPivot(indHighs, ph1.idx)
				ih2 := findClosestPivot(indHighs, ph2.idx)

				if ih1 == nil || ih2 == nil || ih1.idx == ih2.idx {
					continue
				}
				if ih2.idx-ih1.idx < minPivotDistance {
					continue
				}

				// Price higher high + indicator lower high
				if ph2.value > ph1.value && ih2.value < ih1.value {
					strength := calcDivStrength(ph1.value, ph2.value, ih1.value, ih2.value)
					divs = append(divs, Divergence{
						Type:        "REGULAR_BEARISH",
						Indicator:   indicatorName,
						Strength:    strength,
						Description: indicatorName + " regular bearish divergence: price making higher highs while " + indicatorName + " making lower highs",
					})
					break // use the most recent divergence
				}
			}
		}

		// Regular Bullish: price lower low + indicator higher low
		if len(priceLows) >= 2 && len(indLows) >= 2 {
			for i := len(priceLows) - 1; i >= 1; i-- {
				pl1 := priceLows[i-1] // earlier
				pl2 := priceLows[i]   // later (more recent)

				if pl2.idx-pl1.idx < minPivotDistance {
					continue
				}

				il1 := findClosestPivot(indLows, pl1.idx)
				il2 := findClosestPivot(indLows, pl2.idx)

				if il1 == nil || il2 == nil || il1.idx == il2.idx {
					continue
				}
				if il2.idx-il1.idx < minPivotDistance {
					continue
				}

				// Price lower low + indicator higher low
				if pl2.value < pl1.value && il2.value > il1.value {
					strength := calcDivStrength(pl1.value, pl2.value, il1.value, il2.value)
					divs = append(divs, Divergence{
						Type:        "REGULAR_BULLISH",
						Indicator:   indicatorName,
						Strength:    strength,
						Description: indicatorName + " regular bullish divergence: price making lower lows while " + indicatorName + " making higher lows",
					})
					break
				}
			}
		}

		// Hidden Bullish: price higher low + indicator lower low
		if len(priceLows) >= 2 && len(indLows) >= 2 {
			for i := len(priceLows) - 1; i >= 1; i-- {
				pl1 := priceLows[i-1]
				pl2 := priceLows[i]

				if pl2.idx-pl1.idx < minPivotDistance {
					continue
				}

				il1 := findClosestPivot(indLows, pl1.idx)
				il2 := findClosestPivot(indLows, pl2.idx)

				if il1 == nil || il2 == nil || il1.idx == il2.idx {
					continue
				}
				if il2.idx-il1.idx < minPivotDistance {
					continue
				}

				// Price higher low + indicator lower low
				if pl2.value > pl1.value && il2.value < il1.value {
					strength := calcDivStrength(pl1.value, pl2.value, il1.value, il2.value)
					divs = append(divs, Divergence{
						Type:        "HIDDEN_BULLISH",
						Indicator:   indicatorName,
						Strength:    strength,
						Description: indicatorName + " hidden bullish divergence: price making higher lows while " + indicatorName + " making lower lows — continuation signal",
					})
					break
				}
			}
		}

		// Hidden Bearish: price lower high + indicator higher high
		if len(priceHighs) >= 2 && len(indHighs) >= 2 {
			for i := len(priceHighs) - 1; i >= 1; i-- {
				ph1 := priceHighs[i-1]
				ph2 := priceHighs[i]

				if ph2.idx-ph1.idx < minPivotDistance {
					continue
				}

				ih1 := findClosestPivot(indHighs, ph1.idx)
				ih2 := findClosestPivot(indHighs, ph2.idx)

				if ih1 == nil || ih2 == nil || ih1.idx == ih2.idx {
					continue
				}
				if ih2.idx-ih1.idx < minPivotDistance {
					continue
				}

				// Price lower high + indicator higher high
				if ph2.value < ph1.value && ih2.value > ih1.value {
					strength := calcDivStrength(ph1.value, ph2.value, ih1.value, ih2.value)
					divs = append(divs, Divergence{
						Type:        "HIDDEN_BEARISH",
						Indicator:   indicatorName,
						Strength:    strength,
						Description: indicatorName + " hidden bearish divergence: price making lower highs while " + indicatorName + " making higher highs — continuation signal",
					})
					break
				}
			}
		}

		return divs
	}

	// Detect RSI divergences
	if rsiSeries != nil {
		divergences = append(divergences, detectForIndicator("RSI", rsiSeries)...)
	}

	// Detect MACD divergences
	if macdSeries != nil {
		divergences = append(divergences, detectForIndicator("MACD", macdSeries)...)
	}

	return divergences
}

// findClosestPivot finds the pivot in the list closest to the target index.
func findClosestPivot(pivots []pivotPoint, targetIdx int) *pivotPoint {
	if len(pivots) == 0 {
		return nil
	}
	best := &pivots[0]
	bestDist := abs(pivots[0].idx - targetIdx)
	for i := 1; i < len(pivots); i++ {
		d := abs(pivots[i].idx - targetIdx)
		if d < bestDist {
			bestDist = d
			best = &pivots[i]
		}
	}
	return best
}

// abs returns the absolute value of an int.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// calcDivStrength calculates divergence strength (0-1) based on how pronounced
// the divergence is. Uses the ratio of indicator divergence to price divergence.
func calcDivStrength(priceA, priceB, indA, indB float64) float64 {
	// Price change magnitude
	priceDiff := math.Abs(priceB - priceA)
	priceBase := math.Max(math.Abs(priceA), math.Abs(priceB))
	if priceBase == 0 {
		return 0.5
	}
	priceChangePct := priceDiff / priceBase

	// Indicator change magnitude
	indDiff := math.Abs(indB - indA)
	indBase := math.Max(math.Abs(indA), math.Abs(indB))
	if indBase == 0 {
		return 0.5
	}
	indChangePct := indDiff / indBase

	// Strength is based on both magnitudes
	totalChange := priceChangePct + indChangePct
	if totalChange == 0 {
		return 0
	}

	// Normalize to 0-1 range. A combined 10% change is moderate, 20%+ is strong.
	strength := math.Min(totalChange/0.2, 1.0)

	return strength
}
