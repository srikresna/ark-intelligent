package ta

import "math"

// ---------------------------------------------------------------------------
// Elliott Wave Swing Labeler — Phase 1
// Automated swing detection + basic Elliott Wave rule validation.
// Output: possible wave position with confidence score.
// ---------------------------------------------------------------------------

// SwingPoint represents a detected swing high or low in a price series.
type SwingPoint struct {
	Type     string  // "HIGH" or "LOW"
	Price    float64 // High (for swing high) or Low (for swing low)
	BarIndex int     // index in the original newest-first bars slice
}

// WaveCandidate represents a candidate Elliott Wave leg between two swing points.
type WaveCandidate struct {
	WaveNumber int     // 1–5 for impulse, or wave label (1=A, 2=B, 3=C for corrective)
	WaveLabel  string  // "1","2","3","4","5","A","B","C"
	StartPrice float64 // price at wave start (swing point)
	EndPrice   float64 // price at wave end (swing point)
	StartBar   int     // bar index of start swing point (newest-first)
	EndBar     int     // bar index of end swing point (newest-first)
	FibRatio   float64 // ratio of this wave relative to the prior wave (0 if first)
}

// ElliottResult holds the output of an Elliott Wave analysis run.
type ElliottResult struct {
	Swings             []SwingPoint    // detected swing points (max 20 most recent)
	PossibleWavePosition string        // "W1","W2","W3","W4","W5","WA","WB","WC","UNCLEAR"
	WaveCandidates     []WaveCandidate // candidate wave segments from most recent swings
	RulesViolated      []string        // Elliott rules that are violated
	Confidence         int             // 0-100: likelihood the wave count is correct
	Bias               string          // "BULLISH", "BEARISH", "NEUTRAL"
}

// ---------------------------------------------------------------------------
// DetectSwings identifies swing highs and lows in a bar series.
// bars must be newest-first. sensitivity is the number of confirming bars on
// each side (default 3 if <= 0).
// Returns swings in newest-first order (most recent swing at index 0).
// ---------------------------------------------------------------------------

func DetectSwings(bars []OHLCV, sensitivity int) []SwingPoint {
	if sensitivity <= 0 {
		sensitivity = 3
	}
	n := len(bars)
	if n < 2*sensitivity+1 {
		return nil
	}

	// Work oldest-first for the pivot scan, then reverse results.
	asc := reverseOHLCV(bars)
	na := len(asc)

	var swings []SwingPoint

	for i := sensitivity; i < na-sensitivity; i++ {
		// Swing High: asc[i].High > all bars in [i-sensitivity, i+sensitivity]
		isHigh := true
		for j := i - sensitivity; j <= i+sensitivity; j++ {
			if j == i {
				continue
			}
			if asc[j].High >= asc[i].High {
				isHigh = false
				break
			}
		}
		if isHigh {
			// Convert index from oldest-first to newest-first
			swings = append(swings, SwingPoint{
				Type:     "HIGH",
				Price:    asc[i].High,
				BarIndex: na - 1 - i,
			})
			continue // skip low check for the same bar
		}

		// Swing Low: asc[i].Low < all bars in [i-sensitivity, i+sensitivity]
		isLow := true
		for j := i - sensitivity; j <= i+sensitivity; j++ {
			if j == i {
				continue
			}
			if asc[j].Low <= asc[i].Low {
				isLow = false
				break
			}
		}
		if isLow {
			swings = append(swings, SwingPoint{
				Type:     "LOW",
				Price:    asc[i].Low,
				BarIndex: na - 1 - i,
			})
		}
	}

	// Swings are currently in oldest-first order; reverse to newest-first.
	for i, j := 0, len(swings)-1; i < j; i, j = i+1, j-1 {
		swings[i], swings[j] = swings[j], swings[i]
	}

	// Cap at 20 most recent swings.
	if len(swings) > 20 {
		swings = swings[:20]
	}
	return swings
}

// ---------------------------------------------------------------------------
// AnalyzeElliott attempts to fit the most recent swings to an impulse or
// corrective pattern and validates basic Elliott rules.
// bars must be newest-first, len >= 30.
// Returns nil if there are insufficient swings to analyse.
// ---------------------------------------------------------------------------

func AnalyzeElliott(bars []OHLCV) *ElliottResult {
	if len(bars) < 30 {
		return nil
	}

	swings := DetectSwings(bars, 3)
	if len(swings) < 3 {
		return nil
	}

	result := &ElliottResult{
		Swings: swings,
	}

	// We need at least 5 swings (alternating H/L) to attempt a full 5-wave
	// impulse count. With fewer we try a corrective (A-B-C) count.
	if len(swings) >= 5 {
		analyseImpulse(result, swings)
	} else {
		analyseCorrective(result, swings)
	}

	if result.PossibleWavePosition == "" {
		result.PossibleWavePosition = "UNCLEAR"
	}
	if result.Bias == "" {
		result.Bias = "NEUTRAL"
	}

	return result
}

// ---------------------------------------------------------------------------
// analyseImpulse tries to fit the 5 most recent alternating swings to a
// classic 5-wave impulse pattern. It validates the three core Elliott rules
// and computes a confidence score.
// ---------------------------------------------------------------------------

func analyseImpulse(result *ElliottResult, swings []SwingPoint) {
	// Extract the last 5 alternating swings (oldest at index 4, newest at 0).
	last5 := extractAlternating(swings, 5)
	if len(last5) < 5 {
		// Fall back to corrective analysis.
		analyseCorrective(result, swings)
		return
	}

	// In a 5-wave impulse the alternation is: L0→H1→L2→H3→L4 (bullish)
	// or H0→L1→H2→L3→H4 (bearish).
	// last5[4] = oldest swing (Wave 1 start), last5[0] = newest swing.

	// Determine direction from the first leg.
	bullish := last5[4].Type == "LOW" // starts from a low → bullish impulse

	// Name the wave points w0..w4 in chronological (oldest-first) order.
	w := [5]SwingPoint{last5[4], last5[3], last5[2], last5[1], last5[0]}

	// Wave lengths (absolute price moves).
	wave1 := math.Abs(w[1].Price - w[0].Price)
	wave2 := math.Abs(w[2].Price - w[1].Price)
	wave3 := math.Abs(w[3].Price - w[2].Price)
	wave4 := math.Abs(w[4].Price - w[3].Price)
	// wave5 is from w4 to current price (not yet complete) — skip for now.

	// Build WaveCandidates.
	mkCandidate := func(label string, num int, start, end SwingPoint, prevLen float64) WaveCandidate {
		wLen := math.Abs(end.Price - start.Price)
		ratio := 0.0
		if prevLen > 0 {
			ratio = wLen / prevLen
		}
		return WaveCandidate{
			WaveNumber: num,
			WaveLabel:  label,
			StartPrice: start.Price,
			EndPrice:   end.Price,
			StartBar:   start.BarIndex,
			EndBar:     end.BarIndex,
			FibRatio:   ratio,
		}
	}

	result.WaveCandidates = []WaveCandidate{
		mkCandidate("1", 1, w[0], w[1], 0),
		mkCandidate("2", 2, w[1], w[2], wave1),
		mkCandidate("3", 3, w[2], w[3], wave1),
		mkCandidate("4", 4, w[3], w[4], wave3),
	}

	// -----------------------------------------------------------------------
	// Validate three core Elliott rules.
	// -----------------------------------------------------------------------
	var violated []string

	// Rule 1: Wave 2 must not retrace more than 100% of Wave 1.
	if wave2 > wave1 {
		violated = append(violated, "Rule1: Wave2 retraces >100% of Wave1")
	}

	// Rule 2: Wave 3 must not be the shortest of W1, W3, W5.
	// Since W5 is incomplete we only check W3 vs W1 and a rough W5 estimate.
	if wave3 < wave1 && wave3 < wave4 {
		// Wave 4 is not W5 but using it as a proxy for "is W3 clearly shortest"
		violated = append(violated, "Rule2: Wave3 is shortest impulse wave")
	}

	// Rule 3: Wave 4 must not overlap Wave 1 territory.
	if bullish {
		if w[4].Price < w[1].Price {
			violated = append(violated, "Rule3: Wave4 overlaps Wave1 territory")
		}
	} else {
		if w[4].Price > w[1].Price {
			violated = append(violated, "Rule3: Wave4 overlaps Wave1 territory")
		}
	}

	result.RulesViolated = violated

	// -----------------------------------------------------------------------
	// Fibonacci guideline checks.
	// -----------------------------------------------------------------------
	fibPenalty := 0

	// W3 guideline: W3 ≥ 1.618 × W1
	if wave3 < 1.618*wave1 {
		fibPenalty += 15 // not a rule violation, just reduces confidence
	}

	// W2 retrace guideline: 50–61.8% of W1
	w2Retrace := wave2 / wave1
	if w2Retrace < 0.382 || w2Retrace > 0.786 {
		fibPenalty += 10
	}

	// -----------------------------------------------------------------------
	// Determine current wave position: we are entering W5 (last leg).
	// -----------------------------------------------------------------------
	result.PossibleWavePosition = "W5"
	if bullish {
		result.Bias = "BULLISH"
	} else {
		result.Bias = "BEARISH"
	}

	// -----------------------------------------------------------------------
	// Compute confidence score.
	// -----------------------------------------------------------------------
	penalty := len(violated)*25 + fibPenalty
	conf := 100 - penalty
	if conf < 0 {
		conf = 0
	}
	result.Confidence = conf

	// If rules are badly violated, label as unclear.
	if len(violated) >= 2 {
		result.PossibleWavePosition = "UNCLEAR"
		result.Bias = "NEUTRAL"
	}
}

// ---------------------------------------------------------------------------
// analyseCorrective fits 3 alternating swings to an A-B-C corrective pattern.
// ---------------------------------------------------------------------------

func analyseCorrective(result *ElliottResult, swings []SwingPoint) {
	last3 := extractAlternating(swings, 3)
	if len(last3) < 3 {
		result.PossibleWavePosition = "UNCLEAR"
		result.Confidence = 20
		return
	}

	w := [3]SwingPoint{last3[2], last3[1], last3[0]} // oldest → newest

	waveA := math.Abs(w[1].Price - w[0].Price)
	waveB := math.Abs(w[2].Price - w[1].Price)

	mkC := func(start, end SwingPoint, prevLen float64) WaveCandidate {
		wLen := math.Abs(end.Price - start.Price)
		ratio := 0.0
		if prevLen > 0 {
			ratio = wLen / prevLen
		}
		return WaveCandidate{
			WaveNumber: 0,
			WaveLabel:  "C",
			StartPrice: start.Price,
			EndPrice:   end.Price,
			StartBar:   start.BarIndex,
			EndBar:     end.BarIndex,
			FibRatio:   ratio,
		}
	}

	result.WaveCandidates = []WaveCandidate{
		{WaveLabel: "A", StartPrice: w[0].Price, EndPrice: w[1].Price, StartBar: w[0].BarIndex, EndBar: w[1].BarIndex},
		{WaveLabel: "B", StartPrice: w[1].Price, EndPrice: w[2].Price, StartBar: w[1].BarIndex, EndBar: w[2].BarIndex, FibRatio: waveB / waveA},
		mkC(w[2], w[2], waveA), // Wave C not yet complete; placeholder end = last swing
	}

	// B retracement check: B should not exceed 100% of A.
	var violated []string
	if waveB > waveA {
		violated = append(violated, "Rule: Wave B exceeds 100% of Wave A")
	}
	result.RulesViolated = violated

	// Currently in corrective C wave.
	result.PossibleWavePosition = "WC"

	// Determine bias from direction of wave A.
	if w[0].Type == "HIGH" {
		// Corrective started from a high → downward A → correction = bearish pressure
		result.Bias = "BEARISH"
	} else {
		result.Bias = "BULLISH"
	}

	penalty := len(violated) * 25
	conf := 70 - penalty // corrective patterns inherently less certain
	if conf < 0 {
		conf = 0
	}
	result.Confidence = conf
}

// ---------------------------------------------------------------------------
// extractAlternating picks the last n swing points that strictly alternate
// between HIGH and LOW from the newest-first swings slice.
// Returns them in newest-first order (same as input).
// ---------------------------------------------------------------------------

func extractAlternating(swings []SwingPoint, n int) []SwingPoint {
	if len(swings) == 0 {
		return nil
	}
	result := []SwingPoint{swings[0]}
	for i := 1; i < len(swings) && len(result) < n; i++ {
		if swings[i].Type != result[len(result)-1].Type {
			result = append(result, swings[i])
		}
	}
	return result
}
