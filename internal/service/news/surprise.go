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
