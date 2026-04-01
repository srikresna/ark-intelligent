package ict

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DetectFVG scans a bar slice (newest-first) and returns all Fair Value Gap zones.
// Bullish FVG: bars[i+2].High < bars[i].Low  → gap between candle i-2 and candle i
// Bearish FVG: bars[i+2].Low  > bars[i].High → gap between candle i-2 and candle i
//
// We work on a chronological copy internally. The returned FVGZone.BarIndex
// refers to the middle candle in the chronological order.
//
// After detecting gaps we check which ones have been (partially) filled by
// subsequent price action. A gap is filled when a later candle's range
// overlaps with the gap zone.
func DetectFVG(bars []ta.OHLCV) []FVGZone {
	n := len(bars)
	if n < 3 {
		return nil
	}

	// Build chronological slice.
	chron := make([]ta.OHLCV, n)
	for i, b := range bars {
		chron[n-1-i] = b
	}

	var zones []FVGZone

	// i is the index of the MIDDLE candle (index 0 = oldest).
	// The "left" candle is i-1, the "right" candle is i+1.
	// Classic 3-candle FVG: gap between candle[i-1] and candle[i+1].
	for i := 1; i < n-1; i++ {
		left := chron[i-1]
		right := chron[i+1]
		mid := chron[i]

		// Bullish FVG: right candle's low > left candle's high
		if right.Low > left.High {
			zone := FVGZone{
				Kind:      "BULLISH",
				Top:       right.Low,
				Bottom:    left.High,
				CreatedAt: mid.Date,
				BarIndex:  i,
			}
			zones = append(zones, zone)
		}

		// Bearish FVG: right candle's high < left candle's low
		if right.High < left.Low {
			zone := FVGZone{
				Kind:      "BEARISH",
				Top:       left.Low,
				Bottom:    right.High,
				CreatedAt: mid.Date,
				BarIndex:  i,
			}
			zones = append(zones, zone)
		}
	}

	// Mark filled / compute fill percentage using subsequent bars.
	for z := range zones {
		fvg := &zones[z]
		gapSize := fvg.Top - fvg.Bottom
		if gapSize <= 0 {
			fvg.Filled = true
			fvg.FillPct = 100
			continue
		}

		maxPenetration := 0.0
		// Bars after the FVG middle candle (chronologically later).
		for i := fvg.BarIndex + 2; i < n; i++ {
			bar := chron[i]
			if fvg.Kind == "BULLISH" {
				// Price fills from the top down.
				if bar.Low < fvg.Top {
					pen := fvg.Top - bar.Low
					if pen > maxPenetration {
						maxPenetration = pen
					}
				}
			} else {
				// Bearish: price fills from the bottom up.
				if bar.High > fvg.Bottom {
					pen := bar.High - fvg.Bottom
					if pen > maxPenetration {
						maxPenetration = pen
					}
				}
			}
		}

		pct := (maxPenetration / gapSize) * 100
		if pct > 100 {
			pct = 100
		}
		fvg.FillPct = pct
		fvg.Filled = pct >= 100
	}

	// Return only the most recent N zones to keep output manageable.
	if len(zones) > 10 {
		zones = zones[len(zones)-10:]
	}
	return zones
}
