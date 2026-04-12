// Package format provides number formatting utilities for consistent
// display of financial figures across the ark-intelligent bot.
package format

import (
	"fmt"
	"math"
	"strings"
)

// FormatInt formats an integer with thousands separator.
//
// Examples:
//
//	FormatInt(123456)   → "123,456"
//	FormatInt(-1234567) → "-1,234,567"
//	FormatInt(0)        → "0"
func FormatInt(n int64) string {
	if n == 0 {
		return "0"
	}

	neg := n < 0
	abs := n
	if neg {
		abs = -n
	}

	s := fmt.Sprintf("%d", abs)
	s = addThousandSeparators(s)

	if neg {
		return "-" + s
	}
	return s
}

// FormatFloat formats a float64 with thousands separator and given decimal places.
//
// Examples:
//
//	FormatFloat(12345.678, 2) → "12,345.68"
//	FormatFloat(-1234567, 0) → "-1,234,567"
func FormatFloat(f float64, decimals int) string {
	if decimals < 0 {
		decimals = 0
	}

	format := fmt.Sprintf("%%.%df", decimals)
	s := fmt.Sprintf(format, f)

	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]

	neg := ""
	if strings.HasPrefix(intPart, "-") {
		neg = "-"
		intPart = intPart[1:]
	}

	intPart = addThousandSeparators(intPart)
	out := neg + intPart
	if len(parts) == 2 {
		out += "." + parts[1]
	}
	return out
}

// FormatPct formats a percentage to 1 decimal place (no sign).
// Input can be in decimal form (0.673) or percentage form (67.3) —
// values >= 1 are treated as already in percentage form.
//
// Examples:
//
//	FormatPct(0.673) → "67.3%"
//	FormatPct(67.3)  → "67.3%"
//	FormatPct(1.0)   → "1.0%"  (ambiguous: treated as percentage form)
func FormatPct(f float64) string {
	// Heuristic: if absolute value < 1 treat as decimal fraction
	if math.Abs(f) < 1.0 {
		f *= 100
	}
	return fmt.Sprintf("%.1f%%", f)
}

// FormatForex formats a forex price with appropriate decimal places.
//
// JPY pairs (isJPY=true): 3 decimal places.
// Other pairs: 5 decimal places.
//
// Examples:
//
//	FormatForex(1.08432, false) → "1.08432"
//	FormatForex(149.215, true)  → "149.215"
func FormatForex(price float64, isJPY bool) string {
	if isJPY {
		return fmt.Sprintf("%.3f", price)
	}
	return fmt.Sprintf("%.5f", price)
}

// FormatNetPosition formats a COT net position with sign and thousands separator.
//
// Examples:
//
//	FormatNetPosition(123456)  → "+123,456"
//	FormatNetPosition(-50000)  → "-50,000"
//	FormatNetPosition(0)       → "0"
func FormatNetPosition(n int64) string {
	if n == 0 {
		return "0"
	}
	s := FormatInt(n)
	if n > 0 {
		return "+" + s
	}
	return s
}

// addThousandSeparators inserts commas into a pure digit string.
// The string must contain only ASCII digits (no sign, no decimal point).
func addThousandSeparators(digits string) string {
	if len(digits) <= 3 {
		return digits
	}

	var result []byte
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
