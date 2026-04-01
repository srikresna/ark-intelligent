package elliott

import "math"

// ---------------------------------------------------------------------------
// Elliott Wave rule validator
// ---------------------------------------------------------------------------

// validateImpulse checks the three mandatory Elliott Wave rules for a
// five-wave impulse sequence (w1…w5) and annotates each wave in-place.
//
// Rule 1: Wave 2 must NOT retrace more than 100% of Wave 1.
// Rule 2: Wave 3 must NOT be the shortest impulse wave (vs W1 and W5).
// Rule 3: Wave 4 must NOT overlap with Wave 1's price territory
//
//	(the top of Wave 1 for a bullish count).
//
// Returns true when all three rules pass (count is fully valid).
func validateImpulse(waves []Wave) bool {
	if len(waves) < 5 {
		return false
	}

	w1 := &waves[0]
	w2 := &waves[1]
	w3 := &waves[2]
	w4 := &waves[3]
	w5 := &waves[4]

	// Mark all valid first; rules below may flip individual waves.
	for i := range waves {
		waves[i].Valid = true
		waves[i].Violation = ""
	}

	allValid := true

	// -----------------------------------------------------------------------
	// Rule 1: Wave 2 retracement ≤ 100% of Wave 1
	// -----------------------------------------------------------------------
	if w1.Length() > 0 {
		w2.Retracement = w2.Length() / w1.Length()
		if w2.Retracement > 1.0 {
			w2.Valid = false
			w2.Violation = "Rule 1: Wave 2 retraced >100% of Wave 1"
			allValid = false
		}
	}

	// -----------------------------------------------------------------------
	// Rule 2: Wave 3 must NOT be the shortest impulse wave
	// -----------------------------------------------------------------------
	l1 := w1.Length()
	l3 := w3.Length()
	l5 := w5.Length()

	if l3 < l1 && l3 < l5 {
		w3.Valid = false
		w3.Violation = "Rule 2: Wave 3 is the shortest impulse wave"
		allValid = false
	}

	// -----------------------------------------------------------------------
	// Rule 3: Wave 4 must NOT overlap Wave 1's territory
	// -----------------------------------------------------------------------
	// For a bullish impulse: w1 goes up, w2 goes down, etc.
	// The top of Wave 1 = w1.End.  Wave 4 must not drop below that level.
	bullish := w1.Direction == "UP"
	if bullish {
		// Wave 4 end price must not go below Wave 1 end (the top of Wave 1)
		w1Top := w1.End
		w4Bottom := math.Min(w4.Start, w4.End)
		if w4Bottom < w1Top {
			w4.Valid = false
			w4.Violation = "Rule 3: Wave 4 overlaps Wave 1 territory"
			allValid = false
		}
	} else {
		// Bearish impulse: W1 goes down, W1 end is a low, W4 must not exceed W1 end
		w1Bottom := w1.End
		w4Top := math.Max(w4.Start, w4.End)
		if w4Top > w1Bottom {
			w4.Valid = false
			w4.Violation = "Rule 3: Wave 4 overlaps Wave 1 territory"
			allValid = false
		}
	}

	// -----------------------------------------------------------------------
	// Compute Fibonacci ratios (guideline, informational only)
	// -----------------------------------------------------------------------
	if w1.Length() > 0 {
		w3.FibRatio = w3.Length() / w1.Length()
		w5.FibRatio = w5.Length() / w1.Length()
	}
	if w3.Length() > 0 {
		w4.Retracement = w4.Length() / w3.Length()
	}

	return allValid
}

// ---------------------------------------------------------------------------
// Confidence scoring helper
// ---------------------------------------------------------------------------

// scoreConfidence rates the wave count based on how closely guidelines are met.
// Returns "HIGH", "MEDIUM", or "LOW".
func scoreConfidence(waves []Wave, barCount int) string {
	if barCount < 50 {
		return "LOW"
	}

	// Check how many guideline metrics are close to ideal Fibonacci ratios.
	score := 0

	if len(waves) >= 5 {
		w2 := waves[1]
		w3 := waves[2]
		w4 := waves[3]

		// W2 retracement near 50% or 61.8%
		if isNear(w2.Retracement, 0.50, 0.10) || isNear(w2.Retracement, 0.618, 0.10) {
			score++
		}

		// W3 near 1.618x W1
		if isNear(w3.FibRatio, 1.618, 0.25) {
			score++
		}

		// W4 retracement near 38.2%
		if isNear(w4.Retracement, 0.382, 0.10) {
			score++
		}

		// All waves individually valid
		allValid := true
		for _, w := range waves {
			if !w.Valid {
				allValid = false
				break
			}
		}
		if allValid {
			score++
		}
	}

	switch {
	case score >= 3:
		return "HIGH"
	case score >= 2:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// isNear returns true when |value - target| ≤ tolerance.
func isNear(value, target, tolerance float64) bool {
	return math.Abs(value-target) <= tolerance
}
