// Package news provides surprise score computation for economic events.
// A surprise score measures how far an actual release deviates from the forecast,
// normalized by the historical standard deviation of that event's surprises.
package news

import (
	"math"
	"strconv"
	"strings"

	"github.com/arkcode369/ff-calendar-bot/pkg/mathutil"
)

// ComputeSurprise returns the normalized surprise in sigma units.
// Formula: (Actual - Forecast) / StdDev(historical surprises)
// history should be a slice of (actual - forecast) values for the same event, oldest first.
// Returns raw (actual - forecast) when insufficient history.
func ComputeSurprise(actual, forecast float64, history []float64) float64 {
	raw := actual - forecast
	if len(history) < 3 {
		return raw // not enough history for normalization
	}
	stddev := mathutil.StdDevSample(history)
	if stddev == 0 {
		return 0
	}
	return raw / stddev
}

// ClassifySurprise returns a human-readable label for a sigma surprise value.
func ClassifySurprise(sigma float64) string {
	abs := math.Abs(sigma)
	switch {
	case abs >= 2.0:
		if sigma > 0 {
			return "MAJOR HAWKISH SURPRISE"
		}
		return "MAJOR DOVISH SURPRISE"
	case abs >= 1.0:
		if sigma > 0 {
			return "HAWKISH SURPRISE"
		}
		return "DOVISH SURPRISE"
	case abs >= 0.5:
		if sigma > 0 {
			return "SLIGHT HAWKISH"
		}
		return "SLIGHT DOVISH"
	default:
		return "IN LINE"
	}
}

// ParseNumericValue parses a string value that may contain %, K, M suffixes.
// Returns (value, true) on success, (0, false) on failure.
func ParseNumericValue(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" || s == "-" {
		return 0, false
	}

	multiplier := 1.0
	// Handle common suffixes
	if strings.HasSuffix(s, "K") || strings.HasSuffix(s, "k") {
		multiplier = 1_000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "M") || strings.HasSuffix(s, "m") {
		multiplier = 1_000_000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "B") || strings.HasSuffix(s, "b") {
		multiplier = 1_000_000_000
		s = s[:len(s)-1]
	}

	// Strip trailing % and commas
	s = strings.TrimRight(s, "%")
	s = strings.ReplaceAll(s, ",", "")

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v * multiplier, true
}

// SurpriseDirection classifies the raw direction of a surprise (positive = hawkish).
func SurpriseDirection(sigma float64) string {
	if sigma > 0.5 {
		return "HAWKISH"
	} else if sigma < -0.5 {
		return "DOVISH"
	}
	return "NEUTRAL"
}

// ComputeSurpriseWithDirection returns the normalized surprise adjusted by MQL5's ImpactDirection.
// ImpactDirection: 0=neutral (use raw diff), 1=bullish for currency, 2=bearish for currency.
// When ImpactDirection=2 and actual > forecast, the surprise is negative (bearish),
// e.g., unemployment claims rising = bad for currency even though actual > forecast.
func ComputeSurpriseWithDirection(actual, forecast float64, history []float64, impactDirection int) float64 {
	raw := actual - forecast

	// If MQL5 says higher actual is bearish (e.g., unemployment, CPI for non-USD),
	// flip the sign so positive sigma always means "good for currency".
	if impactDirection == 2 && raw > 0 {
		raw = -raw
	} else if impactDirection == 1 && raw < 0 {
		// MQL5 says bullish but raw diff is negative — this means
		// it's an inverted indicator (lower = better, e.g., unemployment rate).
		raw = -raw
	}

	if len(history) < 3 {
		return raw
	}
	stddev := mathutil.StdDevSample(history)
	if stddev == 0 {
		return 0
	}
	return raw / stddev
}

// ClassifySurpriseWithDirection returns a label that respects ImpactDirection.
// When impactDirection is known (1 or 2), the label reflects the actual market impact
// rather than just the numeric direction of the surprise.
func ClassifySurpriseWithDirection(sigma float64, impactDirection int) string {
	abs := math.Abs(sigma)

	// After ComputeSurpriseWithDirection, positive sigma = good for currency,
	// negative = bad for currency. Labels follow this convention.
	bullish := sigma > 0
	_ = impactDirection // direction already baked into sigma by ComputeSurpriseWithDirection

	switch {
	case abs >= 2.0:
		if bullish {
			return "MAJOR BULLISH SURPRISE"
		}
		return "MAJOR BEARISH SURPRISE"
	case abs >= 1.0:
		if bullish {
			return "BULLISH SURPRISE"
		}
		return "BEARISH SURPRISE"
	case abs >= 0.5:
		if bullish {
			return "SLIGHT BULLISH"
		}
		return "SLIGHT BEARISH"
	default:
		return "IN LINE"
	}
}

// SurpriseDirectionWithImpact classifies the direction of a surprise
// after ImpactDirection adjustment. Positive sigma = BULLISH for currency.
func SurpriseDirectionWithImpact(sigma float64) string {
	if sigma > 0.5 {
		return "BULLISH"
	} else if sigma < -0.5 {
		return "BEARISH"
	}
	return "NEUTRAL"
}
