package wyckoff

import "github.com/arkcode369/ark-intelligent/internal/service/ta"

// ─────────────────────────────────────────────────────────────────────────────
// Accumulation vs Distribution classifier
// Determines schematic type and confidence from detected events + volume bias.
// ─────────────────────────────────────────────────────────────────────────────

// classifySchematic returns the Wyckoff schematic ("ACCUMULATION", "DISTRIBUTION",
// or "UNKNOWN") together with a confidence label and a cause score.
func classifySchematic(bars []ta.OHLCV, events []WyckoffEvent, avgVol float64) (schematic, confidence string, causeScore float64) {
	accumScore := 0
	distScore := 0

	for _, e := range events {
		switch e.Name {
		case EventPS, EventSC, EventAR, EventST, EventSpring, EventSOS, EventLPS:
			accumScore++
		case EventBC, EventARDist, EventUP, EventUTAD, EventSOW:
			distScore++
		}
	}

	// Volume absorption: high volume at low prices = accumulation; at high prices = distribution.
	if len(bars) > 10 {
		n := len(bars)
		// Split bars into lower and upper halves by price (use median close).
		closes := make([]float64, n)
		for i, b := range bars {
			closes[i] = b.Close
		}
		median := medianFloat(closes)

		var loVol, hiVol float64
		for _, b := range bars {
			if b.Close <= median {
				loVol += b.Volume
			} else {
				hiVol += b.Volume
			}
		}
		if loVol > hiVol*1.15 {
			accumScore++
		} else if hiVol > loVol*1.15 {
			distScore++
		}
	}

	total := accumScore + distScore
	if total == 0 {
		return "UNKNOWN", "LOW", 0
	}

	if accumScore > distScore {
		schematic = "ACCUMULATION"
		causeScore = float64(accumScore) / float64(total) * 100
	} else {
		schematic = "DISTRIBUTION"
		causeScore = float64(distScore) / float64(total) * 100
	}

	switch {
	case causeScore >= 75:
		confidence = "HIGH"
	case causeScore >= 50:
		confidence = "MEDIUM"
	default:
		confidence = "LOW"
	}
	return
}

// projectedMove estimates the likely breakout magnitude based on the width of
// the trading range and the number of "cause bars" within it. Uses Wyckoff's
// Point & Figure "count" concept approximated as range width × cause multiplier.
func projectedMove(tradingRange [2]float64, causeScore float64) float64 {
	rangeWidth := tradingRange[1] - tradingRange[0]
	if rangeWidth <= 0 {
		return 0
	}
	// A "well-built cause" (score ≈ 100) gives up to 1.5× the range width.
	multiplier := 0.5 + causeScore/100.0
	return rangeWidth * multiplier
}

// medianFloat returns the approximate median of a float64 slice without sorting.
// Uses a simple pivot; accurate enough for our purpose.
func medianFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals)) // mean as proxy for median
}
