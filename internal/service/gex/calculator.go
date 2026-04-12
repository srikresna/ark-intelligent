// Package gex implements Gamma Exposure (GEX) analysis using Deribit options data.
package gex

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// calculateGEX computes per-strike GEX from raw option data.
//
// Formula (per instrument):
//
//	GEX = Gamma × OpenInterest × ContractSize × SpotPrice²
//
// Calls contribute positive GEX (dealers are long gamma → damping).
// Puts contribute negative GEX (dealers are short gamma → amplifying).
//
// Parameters
//   - strikes: unique strike prices (sorted ascending)
//   - callGamma: map[strike] → gamma from call instruments
//   - callOI:    map[strike] → total call open interest
//   - putGamma:  map[strike] → gamma from put instruments
//   - putOI:     map[strike] → total put open interest
//   - contractSize: size of one contract in underlying units (usually 1.0)
//   - spot:       current spot price
func calculateGEX(
	strikes []float64,
	callGamma map[float64]float64,
	callOI map[float64]float64,
	putGamma map[float64]float64,
	putOI map[float64]float64,
	contractSize float64,
	spot float64,
) ([]GEXLevel, error) {
	if spot <= 0 {
		return nil, fmt.Errorf("invalid spot price: %f — all GEX values would be zero", spot)
	}
	if contractSize <= 0 {
		contractSize = 1.0
	}
	multiplier := contractSize * spot * spot

	levels := make([]GEXLevel, 0, len(strikes))
	for _, k := range strikes {
		cGEX := callGamma[k] * callOI[k] * multiplier
		pGEX := -(putGamma[k] * putOI[k] * multiplier) // puts are negative
		levels = append(levels, GEXLevel{
			Strike:  k,
			CallGEX: cGEX,
			PutGEX:  pGEX,
			NetGEX:  cGEX + pGEX,
		})
	}
	return levels, nil
}

// findFlipLevel returns the strike closest to the gamma-neutral price.
// This is the strike where the cumulative NetGEX (sorted ascending from spot)
// crosses zero.
func findFlipLevel(levels []GEXLevel, spot float64) float64 {
	if len(levels) == 0 {
		return 0
	}

	// Walk from spot downward summing cumulative GEX to find sign change.
	type kv struct {
		strike float64
		net    float64
	}
	var sorted []kv
	for _, l := range levels {
		sorted = append(sorted, kv{l.Strike, l.NetGEX})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].strike < sorted[j].strike })

	cumulative := 0.0
	prevSign := 0
	for _, s := range sorted {
		cumulative += s.net
		sign := 1
		if cumulative < 0 {
			sign = -1
		}
		if prevSign != 0 && sign != prevSign {
			return s.strike
		}
		prevSign = sign
	}
	return 0
}

// findMaxPain returns the strike at which total option holders lose the most value.
// Max pain = argmin over strikes of (sum of call intrinsic + sum of put intrinsic).
func findMaxPain(strikes []float64, callOI, putOI map[float64]float64) float64 {
	if len(strikes) == 0 {
		return 0
	}
	minPain := math.MaxFloat64
	maxPainStrike := strikes[0]
	for _, testStrike := range strikes {
		pain := 0.0
		for _, k := range strikes {
			// Call holder loses if test < k (OTM call)
			if testStrike < k {
				pain += callOI[k] * (k - testStrike)
			}
			// Put holder loses if test > k (OTM put)
			if testStrike > k {
				pain += putOI[k] * (testStrike - k)
			}
		}
		if pain < minPain {
			minPain = pain
			maxPainStrike = testStrike
		}
	}
	return maxPainStrike
}

// topKeyLevels returns up to n strikes with the largest absolute NetGEX.
func topKeyLevels(levels []GEXLevel, n int) []float64 {
	sorted := make([]GEXLevel, len(levels))
	copy(sorted, levels)
	sort.Slice(sorted, func(i, j int) bool {
		return math.Abs(sorted[i].NetGEX) > math.Abs(sorted[j].NetGEX)
	})
	keys := make([]float64, 0, n)
	for i := 0; i < n && i < len(sorted); i++ {
		keys = append(keys, sorted[i].Strike)
	}
	return keys
}

// gammaWall returns the strike with the highest call GEX (call-side resistance).
func gammaWall(levels []GEXLevel) float64 {
	best := 0.0
	wall := 0.0
	for _, l := range levels {
		if l.CallGEX > best {
			best = l.CallGEX
			wall = l.Strike
		}
	}
	return wall
}

// putWall returns the strike with the most negative put GEX (put-side support).
func putWall(levels []GEXLevel) float64 {
	best := 0.0
	wall := 0.0
	for _, l := range levels {
		if l.PutGEX < best {
			best = l.PutGEX
			wall = l.Strike
		}
	}
	return wall
}

// regimeAndImplication derives regime string and explanation from TotalGEX.
func regimeAndImplication(totalGEX float64, flipLevel float64) (regime, implication string) {
	if totalGEX >= 0 {
		regime = "POSITIVE_GEX"
		implication = fmt.Sprintf(
			"Dealer net long gamma → range-bound behaviour expected. "+
				"Price moves above/below will be absorbed. "+
				"Breakout requires significant volume to overcome dealer hedging. "+
				"GEX flip level at %s is the key threshold.",
			formatPrice(flipLevel),
		)
	} else {
		regime = "NEGATIVE_GEX"
		implication = fmt.Sprintf(
			"Dealer net short gamma → volatility amplifying environment. "+
				"Price moves tend to be extended by dealer hedging flows. "+
				"Breakouts above/below key levels can accelerate sharply. "+
				"GEX flip level at %s is critical — sustained break = momentum move.",
			formatPrice(flipLevel),
		)
	}
	return
}

// formatPrice formats a price to a clean human-readable string.
// Large prices (≥100) show no decimal places; smaller prices show 2.
func formatPrice(p float64) string {
	if p == 0 {
		return "N/A"
	}
	if p >= 100 {
		return fmt.Sprintf("$%s", commaSep(int64(math.Round(p))))
	}
	return fmt.Sprintf("$%.2f", p)
}

// commaSep formats an integer with comma separators.
func commaSep(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:]
	}
	var result strings.Builder
	for i, c := range s {
		pos := len(s) - i
		if i > 0 && pos%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(c)
	}
	if n < 0 {
		return "-" + result.String()
	}
	return result.String()
}
