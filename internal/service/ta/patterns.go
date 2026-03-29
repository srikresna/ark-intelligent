package ta

import "math"

// ---------------------------------------------------------------------------
// Candlestick Pattern Detection
// ---------------------------------------------------------------------------

// CandlePattern describes a detected candlestick pattern.
type CandlePattern struct {
	Name        string // Pattern name, e.g. "Doji", "Bullish Engulfing"
	Direction   string // "BULLISH", "BEARISH", "NEUTRAL"
	Reliability int    // 1 (low) to 3 (high)
	BarIndex    int    // Index in original bars slice (newest-first, 0 = most recent)
	Description string // Human-readable description
}

// ---------------------------------------------------------------------------
// Candle helpers
// ---------------------------------------------------------------------------

// bodySize returns the absolute body size of a candle.
func bodySize(b OHLCV) float64 {
	return math.Abs(b.Close - b.Open)
}

// upperShadow returns the upper shadow (wick) length.
func upperShadow(b OHLCV) float64 {
	return b.High - math.Max(b.Open, b.Close)
}

// lowerShadow returns the lower shadow (tail) length.
func lowerShadow(b OHLCV) float64 {
	return math.Min(b.Open, b.Close) - b.Low
}

// candleRange returns the full range (High - Low) of a candle.
func candleRange(b OHLCV) float64 {
	return b.High - b.Low
}

// isBullish returns true if the candle closed higher than it opened.
func isBullish(b OHLCV) bool {
	return b.Close > b.Open
}

// isBearish returns true if the candle closed lower than it opened.
func isBearish(b OHLCV) bool {
	return b.Close < b.Open
}

// bodyTop returns the higher of Open/Close.
func bodyTop(b OHLCV) float64 {
	return math.Max(b.Open, b.Close)
}

// bodyBottom returns the lower of Open/Close.
func bodyBottom(b OHLCV) float64 {
	return math.Min(b.Open, b.Close)
}

// bodyMidpoint returns the midpoint of the candle body.
func bodyMidpoint(b OHLCV) float64 {
	return (b.Open + b.Close) / 2
}

// isLargeBody returns true if the body is significantly larger than average
// (heuristic: body > 60% of the range).
func isLargeBody(b OHLCV) bool {
	r := candleRange(b)
	if r == 0 {
		return false
	}
	return bodySize(b) > 0.6*r
}

// isSmallBody returns true if the body is small (< 30% of range).
func isSmallBody(b OHLCV) bool {
	r := candleRange(b)
	if r == 0 {
		return true // zero range = no body
	}
	return bodySize(b) < 0.3*r
}

// isDowntrend checks if the context (last few bars) shows a downtrend.
// Uses bars[1..4] (the 4 bars before the current one).
func isDowntrend(bars []OHLCV) bool {
	if len(bars) < 5 {
		return false
	}
	// Check if the 4 preceding bars are generally declining
	declines := 0
	for i := 1; i < 5; i++ {
		if bars[i].Close > bars[i-1].Close { // older bar closed higher than newer = declining
			declines++
		}
	}
	return declines >= 3
}

// isUptrend checks if the context (last few bars) shows an uptrend.
func isUptrend(bars []OHLCV) bool {
	if len(bars) < 5 {
		return false
	}
	advances := 0
	for i := 1; i < 5; i++ {
		if bars[i].Close < bars[i-1].Close { // older bar closed lower than newer = advancing
			advances++
		}
	}
	return advances >= 3
}

// ---------------------------------------------------------------------------
// DetectPatterns scans the most recent bars for candlestick patterns.
// Bars are newest-first (index 0 = most recent bar).
// Returns all detected patterns, sorted roughly by significance.
// ---------------------------------------------------------------------------

func DetectPatterns(bars []OHLCV) []CandlePattern {
	if len(bars) < 1 {
		return nil
	}

	var patterns []CandlePattern

	// Single-bar patterns (bar[0])
	patterns = append(patterns, detectSingleBar(bars)...)

	// Two-bar patterns (bar[0] and bar[1])
	if len(bars) >= 2 {
		patterns = append(patterns, detectTwoBar(bars)...)
	}

	// Three-bar patterns (bar[0], bar[1], bar[2])
	if len(bars) >= 3 {
		patterns = append(patterns, detectThreeBar(bars)...)
	}

	return patterns
}

// ---------------------------------------------------------------------------
// Single-bar patterns
// ---------------------------------------------------------------------------

func detectSingleBar(bars []OHLCV) []CandlePattern {
	var patterns []CandlePattern
	b := bars[0]
	r := candleRange(b)

	if r == 0 {
		return nil // skip zero-range bars
	}

	body := bodySize(b)
	us := upperShadow(b)
	ls := lowerShadow(b)

	// Doji: |Open-Close| < 0.1 * (High-Low)
	if body < 0.1*r {
		patterns = append(patterns, CandlePattern{
			Name:        "Doji",
			Direction:   "NEUTRAL",
			Reliability: 1,
			BarIndex:    0,
			Description: "Doji — indecision, body is tiny relative to range",
		})
	}

	// Hammer: lower shadow >= 2x body, small upper shadow (< 0.3 * range), in downtrend
	if body > 0 && ls >= 2*body && us < 0.3*r && isDowntrend(bars) {
		patterns = append(patterns, CandlePattern{
			Name:        "Hammer",
			Direction:   "BULLISH",
			Reliability: 2,
			BarIndex:    0,
			Description: "Hammer — long lower shadow in downtrend, potential reversal up",
		})
	}

	// Inverted Hammer: upper shadow >= 2x body, small lower shadow, in downtrend
	if body > 0 && us >= 2*body && ls < 0.3*r && isDowntrend(bars) {
		patterns = append(patterns, CandlePattern{
			Name:        "Inverted Hammer",
			Direction:   "BULLISH",
			Reliability: 1,
			BarIndex:    0,
			Description: "Inverted Hammer — long upper shadow in downtrend, potential reversal up",
		})
	}

	// Shooting Star: upper shadow >= 2x body, small lower shadow, in uptrend
	if body > 0 && us >= 2*body && ls < 0.3*r && isUptrend(bars) {
		patterns = append(patterns, CandlePattern{
			Name:        "Shooting Star",
			Direction:   "BEARISH",
			Reliability: 2,
			BarIndex:    0,
			Description: "Shooting Star — long upper shadow in uptrend, potential reversal down",
		})
	}

	// Spinning Top: small body, both shadows > body
	if isSmallBody(b) && us > body && ls > body && body > 0.01*r {
		// Avoid overlapping with Doji (which has even smaller body)
		if body >= 0.1*r {
			patterns = append(patterns, CandlePattern{
				Name:        "Spinning Top",
				Direction:   "NEUTRAL",
				Reliability: 1,
				BarIndex:    0,
				Description: "Spinning Top — small body with long shadows, indecision",
			})
		}
	}

	// Bullish Marubozu: close > open, no/tiny shadows (within 5% of range)
	if isBullish(b) && us <= 0.05*r && ls <= 0.05*r {
		patterns = append(patterns, CandlePattern{
			Name:        "Bullish Marubozu",
			Direction:   "BULLISH",
			Reliability: 2,
			BarIndex:    0,
			Description: "Bullish Marubozu — strong buying, no shadows",
		})
	}

	// Bearish Marubozu: close < open, no/tiny shadows
	if isBearish(b) && us <= 0.05*r && ls <= 0.05*r {
		patterns = append(patterns, CandlePattern{
			Name:        "Bearish Marubozu",
			Direction:   "BEARISH",
			Reliability: 2,
			BarIndex:    0,
			Description: "Bearish Marubozu — strong selling, no shadows",
		})
	}

	return patterns
}

// ---------------------------------------------------------------------------
// Two-bar patterns
// ---------------------------------------------------------------------------

func detectTwoBar(bars []OHLCV) []CandlePattern {
	var patterns []CandlePattern
	b0 := bars[0] // newest
	b1 := bars[1] // previous

	// Bullish Engulfing: bar[1] bearish, bar[0] bullish, bar[0] body engulfs bar[1] body
	if isBearish(b1) && isBullish(b0) &&
		bodyBottom(b0) < bodyBottom(b1) && bodyTop(b0) > bodyTop(b1) {
		patterns = append(patterns, CandlePattern{
			Name:        "Bullish Engulfing",
			Direction:   "BULLISH",
			Reliability: 3,
			BarIndex:    0,
			Description: "Bullish Engulfing — current bullish candle engulfs prior bearish candle",
		})
	}

	// Bearish Engulfing: bar[1] bullish, bar[0] bearish, bar[0] body engulfs bar[1] body
	if isBullish(b1) && isBearish(b0) &&
		bodyBottom(b0) < bodyBottom(b1) && bodyTop(b0) > bodyTop(b1) {
		patterns = append(patterns, CandlePattern{
			Name:        "Bearish Engulfing",
			Direction:   "BEARISH",
			Reliability: 3,
			BarIndex:    0,
			Description: "Bearish Engulfing — current bearish candle engulfs prior bullish candle",
		})
	}

	// Piercing: bar[1] bearish, bar[0] opens below bar[1] low, closes above midpoint of bar[1] body
	if isBearish(b1) && isBullish(b0) &&
		b0.Open < b1.Low &&
		b0.Close > bodyMidpoint(b1) &&
		b0.Close < bodyTop(b1) { // doesn't fully engulf
		patterns = append(patterns, CandlePattern{
			Name:        "Piercing",
			Direction:   "BULLISH",
			Reliability: 2,
			BarIndex:    0,
			Description: "Piercing — opens below prior low, closes above midpoint of prior body",
		})
	}

	// Dark Cloud Cover: bar[1] bullish, bar[0] opens above bar[1] high, closes below midpoint of bar[1] body
	if isBullish(b1) && isBearish(b0) &&
		b0.Open > b1.High &&
		b0.Close < bodyMidpoint(b1) &&
		b0.Close > bodyBottom(b1) { // doesn't fully engulf
		patterns = append(patterns, CandlePattern{
			Name:        "Dark Cloud Cover",
			Direction:   "BEARISH",
			Reliability: 2,
			BarIndex:    0,
			Description: "Dark Cloud Cover — opens above prior high, closes below midpoint of prior body",
		})
	}

	// Tweezer Top: two bars at resistance, highs within 0.1% of each other
	if b0.High > 0 && b1.High > 0 {
		highDiffPct := math.Abs(b0.High-b1.High) / b1.High * 100
		if highDiffPct <= 0.1 && isBullish(b1) && isBearish(b0) {
			patterns = append(patterns, CandlePattern{
				Name:        "Tweezer Top",
				Direction:   "BEARISH",
				Reliability: 2,
				BarIndex:    0,
				Description: "Tweezer Top — two bars with nearly identical highs, potential reversal down",
			})
		}
	}

	// Tweezer Bottom: two bars at support, lows within 0.1% of each other
	if b0.Low > 0 && b1.Low > 0 {
		lowDiffPct := math.Abs(b0.Low-b1.Low) / b1.Low * 100
		if lowDiffPct <= 0.1 && isBearish(b1) && isBullish(b0) {
			patterns = append(patterns, CandlePattern{
				Name:        "Tweezer Bottom",
				Direction:   "BULLISH",
				Reliability: 2,
				BarIndex:    0,
				Description: "Tweezer Bottom — two bars with nearly identical lows, potential reversal up",
			})
		}
	}

	return patterns
}

// ---------------------------------------------------------------------------
// Three-bar patterns
// ---------------------------------------------------------------------------

func detectThreeBar(bars []OHLCV) []CandlePattern {
	var patterns []CandlePattern
	b0 := bars[0] // newest
	b1 := bars[1] // middle
	b2 := bars[2] // oldest of the three

	// Morning Star: bar[2] bearish large body, bar[1] small body (gap down),
	// bar[0] bullish closes above midpoint of bar[2]
	if isBearish(b2) && isLargeBody(b2) &&
		isSmallBody(b1) &&
		bodyTop(b1) < bodyBottom(b2) && // gap down between bar[2] and bar[1]
		isBullish(b0) &&
		b0.Close > bodyMidpoint(b2) {
		patterns = append(patterns, CandlePattern{
			Name:        "Morning Star",
			Direction:   "BULLISH",
			Reliability: 3,
			BarIndex:    0,
			Description: "Morning Star — bearish large candle, small gap-down candle, then bullish close above midpoint",
		})
	}

	// Evening Star: bar[2] bullish large body, bar[1] small body (gap up),
	// bar[0] bearish closes below midpoint of bar[2]
	if isBullish(b2) && isLargeBody(b2) &&
		isSmallBody(b1) &&
		bodyBottom(b1) > bodyTop(b2) && // gap up between bar[2] and bar[1]
		isBearish(b0) &&
		b0.Close < bodyMidpoint(b2) {
		patterns = append(patterns, CandlePattern{
			Name:        "Evening Star",
			Direction:   "BEARISH",
			Reliability: 3,
			BarIndex:    0,
			Description: "Evening Star — bullish large candle, small gap-up candle, then bearish close below midpoint",
		})
	}

	// Three White Soldiers: 3 consecutive bullish bars, each closing higher,
	// each opening within prior body
	if isBullish(b2) && isBullish(b1) && isBullish(b0) &&
		b1.Close > b2.Close && b0.Close > b1.Close &&
		b1.Open >= bodyBottom(b2) && b1.Open <= bodyTop(b2) &&
		b0.Open >= bodyBottom(b1) && b0.Open <= bodyTop(b1) {
		patterns = append(patterns, CandlePattern{
			Name:        "Three White Soldiers",
			Direction:   "BULLISH",
			Reliability: 3,
			BarIndex:    0,
			Description: "Three White Soldiers — three consecutive bullish candles, each closing higher",
		})
	}

	// Three Black Crows: 3 consecutive bearish bars, each closing lower,
	// each opening within prior body
	if isBearish(b2) && isBearish(b1) && isBearish(b0) &&
		b1.Close < b2.Close && b0.Close < b1.Close &&
		b1.Open >= bodyBottom(b2) && b1.Open <= bodyTop(b2) &&
		b0.Open >= bodyBottom(b1) && b0.Open <= bodyTop(b1) {
		patterns = append(patterns, CandlePattern{
			Name:        "Three Black Crows",
			Direction:   "BEARISH",
			Reliability: 3,
			BarIndex:    0,
			Description: "Three Black Crows — three consecutive bearish candles, each closing lower",
		})
	}

	return patterns
}
