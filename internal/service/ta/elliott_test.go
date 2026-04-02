package ta

import (
	"testing"
	"time"
)

// buildBar creates an OHLCV bar with identical OHLC values for simplicity.
func buildBar(price float64) OHLCV {
	return OHLCV{
		Date:   time.Now(),
		Open:   price,
		High:   price * 1.001,
		Low:    price * 0.999,
		Close:  price,
		Volume: 1000,
	}
}

// buildBarHL creates an OHLCV bar with explicit High and Low.
func buildBarHL(close, high, low float64) OHLCV {
	return OHLCV{
		Date:   time.Now(),
		Open:   close,
		High:   high,
		Low:    low,
		Close:  close,
		Volume: 1000,
	}
}

// ---------------------------------------------------------------------------
// TestAnalyzeElliott_ValidImpulse constructs a synthetic 5-wave bullish
// impulse that satisfies all three core Elliott rules.
//
// Wave structure (price):
//   W1: 100 → 120  (+20)
//   W2: 120 → 110  (-10, retrace 50% of W1 ✓)
//   W3: 110 → 145  (+35, > W1 ✓)
//   W4: 145 → 135  (-10, no overlap with W1 ✓)
//   W5: 135 → 155  (+20, ongoing)
//
// We build bars that produce these swing points.
// ---------------------------------------------------------------------------

func TestAnalyzeElliott_ValidImpulse(t *testing.T) {
	// Build a synthetic series (newest-first, reversed at end).
	// We'll construct bars in chronological order then reverse.

	// Each "wave leg" is 5 bars: 3 sensitivity + 2 confirming bars on each side.
	// We need at least sensitivity=3 bars on each side of a swing to detect it.
	// Pattern: ascend to swing high, descend to swing low, repeat.

	var bars []OHLCV

	// Flat lead-in (5 bars) so first bars aren't on the boundary.
	for i := 0; i < 5; i++ {
		bars = append(bars, buildBarHL(100, 102, 98))
	}

	// W0 = swing LOW at 100 (start)
	// Build ascending bars toward W1 high 120.
	// 3 bars rising to 120, then 3 bars declining (to confirm swing high).
	bars = append(bars, buildBarHL(105, 107, 99))
	bars = append(bars, buildBarHL(112, 114, 104))
	bars = append(bars, buildBarHL(119, 121, 111)) // swing high ~120
	bars = append(bars, buildBarHL(115, 117, 113))
	bars = append(bars, buildBarHL(112, 114, 110))
	bars = append(bars, buildBarHL(110, 112, 108)) // W1 high confirmed

	// W2 = swing LOW at 110 (retrace ~50% of W1 = 10 pts)
	bars = append(bars, buildBarHL(109, 111, 107))
	bars = append(bars, buildBarHL(108, 110, 106))
	bars = append(bars, buildBarHL(110, 112, 108)) // swing low ~110 (confirmed)
	bars = append(bars, buildBarHL(112, 114, 109))
	bars = append(bars, buildBarHL(115, 117, 111))
	bars = append(bars, buildBarHL(118, 120, 114)) // W2 low confirmed

	// W3 high at 145 (+35 from 110, > W1 +20 ✓)
	bars = append(bars, buildBarHL(125, 127, 117))
	bars = append(bars, buildBarHL(135, 137, 123))
	bars = append(bars, buildBarHL(143, 146, 133)) // swing high ~145
	bars = append(bars, buildBarHL(140, 142, 138))
	bars = append(bars, buildBarHL(138, 140, 136))
	bars = append(bars, buildBarHL(136, 138, 134)) // W3 high confirmed

	// W4 low at 135 (no overlap with W1 top @120 ✓)
	bars = append(bars, buildBarHL(139, 141, 136))
	bars = append(bars, buildBarHL(137, 139, 134))
	bars = append(bars, buildBarHL(135, 137, 133)) // swing low ~135
	bars = append(bars, buildBarHL(136, 138, 134))
	bars = append(bars, buildBarHL(138, 140, 135))
	bars = append(bars, buildBarHL(139, 141, 137)) // W4 low confirmed

	// W5 (ongoing) — just flat bars showing we're past W4
	for i := 0; i < 5; i++ {
		bars = append(bars, buildBarHL(142, 144, 140))
	}

	// Reverse to newest-first.
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}

	result := AnalyzeElliott(bars)
	if result == nil {
		t.Fatal("AnalyzeElliott returned nil for a valid impulse series")
	}

	if len(result.RulesViolated) > 0 {
		t.Errorf("expected no rule violations, got: %v", result.RulesViolated)
	}

	if result.Confidence < 50 {
		t.Errorf("expected confidence >= 50, got %d", result.Confidence)
	}

	if result.Bias != "BULLISH" {
		t.Errorf("expected BULLISH bias, got %s", result.Bias)
	}

	if result.PossibleWavePosition == "UNCLEAR" {
		t.Errorf("expected a defined wave position, got UNCLEAR")
	}
}

// ---------------------------------------------------------------------------
// TestAnalyzeElliott_Rule1Violation constructs a series where Wave 2
// retraces more than 100% of Wave 1 (Rule 1 violation).
// We need 5 alternating swings: W0(LOW) → W1(HIGH) → W2(LOW) → W3(HIGH) → W4(LOW)
// W2 drops below W0 → wave2 > wave1 → Rule 1 triggered.
// ---------------------------------------------------------------------------

func TestAnalyzeElliott_Rule1Violation(t *testing.T) {
	var bars []OHLCV

	// Flat lead-in (5 bars at ~120)
	for i := 0; i < 5; i++ {
		bars = append(bars, buildBarHL(120, 122, 118))
	}

	// Descend to W0 swing LOW at 100
	bars = append(bars, buildBarHL(115, 117, 113))
	bars = append(bars, buildBarHL(108, 110, 106))
	bars = append(bars, buildBarHL(101, 103, 99))  // swing low ~100
	bars = append(bars, buildBarHL(103, 105, 101))
	bars = append(bars, buildBarHL(106, 108, 104))
	bars = append(bars, buildBarHL(109, 111, 107)) // W0 low confirmed

	// W1 HIGH at 120 (+20 from 100)
	bars = append(bars, buildBarHL(113, 115, 109))
	bars = append(bars, buildBarHL(117, 119, 111))
	bars = append(bars, buildBarHL(119, 121, 115)) // swing high ~120
	bars = append(bars, buildBarHL(117, 119, 115))
	bars = append(bars, buildBarHL(115, 117, 113))
	bars = append(bars, buildBarHL(113, 115, 111)) // W1 high confirmed

	// W2 LOW at 92 (−28 from 120, retraces 140% of W1 → VIOLATION)
	bars = append(bars, buildBarHL(108, 110, 106))
	bars = append(bars, buildBarHL(100, 102, 98))
	bars = append(bars, buildBarHL(93, 95, 91))   // swing low ~92 (below W0=100 → >100% retrace)
	bars = append(bars, buildBarHL(95, 97, 93))
	bars = append(bars, buildBarHL(97, 99, 95))
	bars = append(bars, buildBarHL(99, 101, 97))  // W2 low confirmed

	// W3 HIGH at 130
	bars = append(bars, buildBarHL(107, 109, 97))
	bars = append(bars, buildBarHL(118, 120, 105))
	bars = append(bars, buildBarHL(128, 131, 116)) // swing high ~130
	bars = append(bars, buildBarHL(126, 128, 124))
	bars = append(bars, buildBarHL(124, 126, 122))
	bars = append(bars, buildBarHL(122, 124, 120)) // W3 high confirmed

	// W4 LOW at 122 (above W1 top @120 ✓)
	bars = append(bars, buildBarHL(124, 126, 122))
	bars = append(bars, buildBarHL(122, 124, 120))
	bars = append(bars, buildBarHL(122, 124, 120)) // swing low ~122
	bars = append(bars, buildBarHL(123, 125, 121))
	bars = append(bars, buildBarHL(124, 126, 122))
	bars = append(bars, buildBarHL(125, 127, 123)) // W4 low confirmed

	// Ongoing W5
	for i := 0; i < 5; i++ {
		bars = append(bars, buildBarHL(128, 130, 126))
	}

	// Reverse to newest-first.
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}

	result := AnalyzeElliott(bars)
	if result == nil {
		t.Fatal("AnalyzeElliott returned nil")
	}

	found := false
	for _, v := range result.RulesViolated {
		if len(v) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least one rule violation for W2 > 100%% retrace, got pos=%s conf=%d violations=%v",
			result.PossibleWavePosition, result.Confidence, result.RulesViolated)
	}
}
