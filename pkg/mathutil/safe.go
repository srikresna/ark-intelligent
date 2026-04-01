// Package mathutil — safe arithmetic helpers to prevent NaN/Inf propagation
// in financial calculations.
package mathutil

import "math"

// IsFinite returns true if f is neither NaN nor ±Inf.
func IsFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// SafeDiv divides a by b, returning fallback if b is zero or the result
// is NaN/Inf. This prevents division-by-zero artifacts from propagating
// through downstream calculations and into user-facing output.
func SafeDiv(a, b, fallback float64) float64 {
	if b == 0 {
		return fallback
	}
	result := a / b
	if !IsFinite(result) {
		return fallback
	}
	return result
}

// ClampFloat restricts f to [min, max]. If f is NaN or Inf, returns the
// midpoint of [min, max] as a safe default.
func ClampFloat(f, min, max float64) float64 {
	if !IsFinite(f) {
		return (min + max) / 2
	}
	if f < min {
		return min
	}
	if f > max {
		return max
	}
	return f
}

// SanitizeFloat returns fallback if f is NaN or ±Inf, otherwise returns f.
// Useful as a final guard before formatting or storing a value.
func SanitizeFloat(f, fallback float64) float64 {
	if !IsFinite(f) {
		return fallback
	}
	return f
}
