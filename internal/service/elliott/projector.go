package elliott

// ---------------------------------------------------------------------------
// Wave projection calculator — Fibonacci-based targets
// ---------------------------------------------------------------------------

// projectTargets calculates Wave 5 price targets based on Fibonacci extensions
// of Wave 1.  Returns (conservative, aggressive) price levels.
//
// Conservative:  Wave5Target = Wave1Start + Wave1Length × 1.0
// Aggressive:    Wave5Target = Wave1Start + Wave1Length × 1.618
//
// For a downtrend the formulas are mirrored.
func projectTargets(waves []Wave) (target1, target2 float64) {
	if len(waves) < 1 {
		return 0, 0
	}

	w1 := waves[0]
	w1Len := w1.Length()
	if w1Len == 0 {
		return 0, 0
	}

	bullish := w1.Direction == "UP"
	w1Start := w1.Start

	if bullish {
		// Upward impulse: targets are above Wave 1 start
		target1 = w1Start + w1Len*1.000
		target2 = w1Start + w1Len*1.618
	} else {
		// Downward impulse: targets are below Wave 1 start
		target1 = w1Start - w1Len*1.000
		target2 = w1Start - w1Len*1.618
	}

	return target1, target2
}

// projectW3Target estimates a Wave 3 extension target from Wave 1 data.
// Wave 3 target = Wave1Start + Wave1Length × 1.618
func projectW3Target(w1 Wave) float64 {
	if w1.Direction == "UP" {
		return w1.Start + w1.Length()*1.618
	}
	return w1.Start - w1.Length()*1.618
}

// projectW4EndZone returns the ideal Wave 4 end range (38.2% retracement of W3).
// Returns (low, high) of the expected zone.
func projectW4EndZone(w3 Wave) (low, high float64) {
	retrace382 := w3.Length() * 0.382
	retrace500 := w3.Length() * 0.500
	if w3.Direction == "UP" {
		// W4 retraces down from W3 end
		low = w3.End - retrace500
		high = w3.End - retrace382
	} else {
		// W4 retraces up from W3 end
		low = w3.End + retrace382
		high = w3.End + retrace500
	}
	return low, high
}

// invalidationLevel returns the price that would invalidate this wave count.
// For a bullish count: the start of Wave 1 (price must not close below it).
// For a bearish count: the start of Wave 1 (price must not close above it).
func invalidationLevel(waves []Wave) float64 {
	if len(waves) == 0 {
		return 0
	}
	return waves[0].Start
}

// waveProgress estimates how far the current wave has progressed (0-100).
// It uses the average length of completed waves of the same impulse/corrective
// character as a proxy for the expected total length of the current wave.
func waveProgress(waves []Wave, currentWaveLabel string) float64 {
	if len(waves) == 0 {
		return 0
	}

	current := waves[len(waves)-1]
	currentLen := current.Length()

	// Average length of prior impulse waves (1, 3, 5) or corrective (2, 4)
	var refLengths []float64
	isImpulse := currentWaveLabel == "1" || currentWaveLabel == "3" || currentWaveLabel == "5"
	for _, w := range waves[:len(waves)-1] {
		wIsImpulse := w.Number == "1" || w.Number == "3" || w.Number == "5"
		if wIsImpulse == isImpulse && w.Length() > 0 {
			refLengths = append(refLengths, w.Length())
		}
	}

	if len(refLengths) == 0 {
		return 50 // default midpoint
	}

	sum := 0.0
	for _, l := range refLengths {
		sum += l
	}
	avgLen := sum / float64(len(refLengths))
	if avgLen == 0 {
		return 50
	}

	pct := (currentLen / avgLen) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}
