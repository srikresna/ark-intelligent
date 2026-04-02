package ta

import "fmt"

// ---------------------------------------------------------------------------
// Wyckoff Phase Detection
// Simplified implementation of Richard Wyckoff's method for identifying
// accumulation/distribution phases and key structural events.
// ---------------------------------------------------------------------------

// WyckoffPhase identifies the current market structure phase.
type WyckoffPhase string

const (
	PhaseAccumulation WyckoffPhase = "ACCUMULATION" // Phase A-E classic accumulation
	PhaseMarkup       WyckoffPhase = "MARKUP"        // trending up after accumulation
	PhaseDistribution WyckoffPhase = "DISTRIBUTION"  // Phase A-E classic distribution
	PhaseMarkdown     WyckoffPhase = "MARKDOWN"       // trending down after distribution
	PhaseTransition   WyckoffPhase = "TRANSITION"     // unclear / changing
)

// TradingRange defines the price range in which price has been consolidating.
type TradingRange struct {
	High        float64 // resistance (top of range)
	Low         float64 // support (bottom of range)
	MidPoint    float64 // (High+Low)/2
	Width       float64 // ATR-normalized width
	BarsInRange int     // how many bars price has stayed in range
	VolumeDecl  bool    // is volume declining in range (accumulation signal)
}

// WyckoffEvent represents a single detected Wyckoff structural event.
type WyckoffEvent struct {
	Name     string  // "SC", "AR", "ST", "SPRING", "SOS", "SOW", "UT", "UTAD"
	BarIndex int     // newest-first index
	Price    float64 // close price at event bar
	Volume   float64 // relative to 20-bar avg
	Signal   string  // "BULLISH" or "BEARISH"
}

// WyckoffResult holds the full output of a Wyckoff analysis run.
type WyckoffResult struct {
	Phase        WyckoffPhase   // current detected phase
	PhaseConf    float64        // confidence 0-1
	TradingRange *TradingRange  // nil if not in a range
	KeyEvents    []WyckoffEvent // recent key events (newest first)
	EffortResult []EffortResult // effort vs result analysis
	Bias         string         // "BULLISH", "BEARISH", "NEUTRAL"
	Description  string         // human-readable phase explanation
}

// EffortResult measures whether volume effort matches price result.
// High volume + small price move = absorption (bearish for dist, bullish for acc).
// Low volume + large price move = ease of movement (trend continuation).
type EffortResult struct {
	BarIndex   int
	Effort     float64 // volume relative to 20-bar avg
	Result     float64 // price move relative to ATR
	Divergence bool    // true if high effort + low result (absorption)
	Type       string  // "ABSORPTION", "EASE_OF_MOVEMENT", "NORMAL"
}

// CalcWyckoff analyzes Wyckoff phase and key events.
// bars: newest-first, minimum 50 bars recommended for reliable phase detection.
// atr: 14-period ATR for range normalization. Pass 0 to auto-compute.
// Returns nil if fewer than 20 bars.
func CalcWyckoff(bars []OHLCV, atr float64) *WyckoffResult {
	if len(bars) < 20 {
		return nil
	}
	if atr <= 0 {
		atr = wyckoffCalcATR(bars, 14)
	}
	if atr <= 0 {
		return nil
	}

	n := len(bars)

	// Step 1: Identify trading range using 20-bar swing window.
	tr := wyckoffIdentifyTradingRange(bars, atr)

	// Step 2: Compute volume averages.
	avgVol20 := wyckoffAvgVol(bars, 0, n, 20)
	avgVol5 := wyckoffAvgVol(bars, 0, 5, 5)
	volumeDecl := avgVol5 < 0.7*avgVol20
	volumeExpand := avgVol5 > 1.3*avgVol20
	if tr != nil {
		tr.VolumeDecl = volumeDecl
	}

	// Step 3: Detect key events.
	events := wyckoffDetectEvents(bars, atr, avgVol20, tr)

	// Step 4: Classify phase.
	phase, conf, bias, desc := wyckoffClassifyPhase(bars, atr, tr, events, volumeDecl, volumeExpand)

	// Step 5: Effort vs result for recent 10 bars.
	effortResults := wyckoffEffortResult(bars, atr, avgVol20, 10)

	return &WyckoffResult{
		Phase:        phase,
		PhaseConf:    conf,
		TradingRange: tr,
		KeyEvents:    events,
		EffortResult: effortResults,
		Bias:         bias,
		Description:  desc,
	}
}

// ---------------------------------------------------------------------------
// Trading Range Detection
// ---------------------------------------------------------------------------

func wyckoffIdentifyTradingRange(bars []OHLCV, atr float64) *TradingRange {
	window := 20
	if len(bars) < window {
		window = len(bars)
	}

	// Use Close-based range so UT/Spring wicks do not contaminate the range boundaries.
	// A UT is defined as a bar whose High exceeds the range high (close-based) but closes back inside.
	high := bars[0].Close
	low := bars[0].Close
	for i := 1; i < window; i++ {
		if bars[i].Close > high {
			high = bars[i].Close
		}
		if bars[i].Close < low {
			low = bars[i].Close
		}
	}

	rangeSize := high - low
	width := rangeSize / atr

	// Range only valid if at least 2 ATR wide.
	if width < 2.0 {
		return nil
	}

	// Count how many bars closed within range (no breakout > ATR*0.5 on close).
	barsInRange := 0
	threshold := atr * 0.5
	for i := 0; i < window; i++ {
		if bars[i].Close <= high+threshold && bars[i].Close >= low-threshold {
			barsInRange++
		}
	}

	// Need at least 10 bars in range to be valid.
	if barsInRange < 10 {
		return nil
	}

	return &TradingRange{
		High:        high,
		Low:         low,
		MidPoint:    (high + low) / 2,
		Width:       width,
		BarsInRange: barsInRange,
	}
}

// ---------------------------------------------------------------------------
// Key Event Detection
// ---------------------------------------------------------------------------

func wyckoffDetectEvents(bars []OHLCV, atr, avgVol20 float64, tr *TradingRange) []WyckoffEvent {
	var events []WyckoffEvent
	n := len(bars)
	if n < 5 {
		return events
	}

	// Selling Climax (SC): large bearish body > 2*ATR, volume > 2x avg, at/below prior low.
	scIdx := -1
	for i := 5; i < n-2; i++ {
		bar := bars[i]
		body := bar.Open - bar.Close
		if body < 2*atr {
			continue
		}
		if bar.Volume < 2*avgVol20 {
			continue
		}
		priorLow := bar.Low
		for j := i + 1; j < i+6 && j < n; j++ {
			if bars[j].Low < priorLow {
				priorLow = bars[j].Low
			}
		}
		if bar.Close > priorLow*1.002 {
			continue
		}
		events = append(events, WyckoffEvent{
			Name:     "SC",
			BarIndex: i,
			Price:    bar.Close,
			Volume:   bar.Volume / avgVol20,
			Signal:   "BEARISH",
		})
		scIdx = i
		break
	}

	// Automatic Rally (AR): after SC, sharp bounce > 1 ATR upward within 10 bars.
	arIdx := -1
	if scIdx >= 0 {
		for i := scIdx - 1; i >= 0 && i >= scIdx-10; i-- {
			bar := bars[i]
			if bar.Close-bars[scIdx].Close > atr {
				events = append(events, WyckoffEvent{
					Name:     "AR",
					BarIndex: i,
					Price:    bar.Close,
					Volume:   bar.Volume / avgVol20,
					Signal:   "BULLISH",
				})
				arIdx = i
				break
			}
		}
	}

	// Spring: price wicks below range low then closes back inside.
	if tr != nil && scIdx >= 0 {
		rangeLow := tr.Low
		for i := 1; i < n-1; i++ {
			bar := bars[i]
			if bar.Low < rangeLow && bar.Close > rangeLow {
				events = append(events, WyckoffEvent{
					Name:     "SPRING",
					BarIndex: i,
					Price:    bar.Close,
					Volume:   bar.Volume / avgVol20,
					Signal:   "BULLISH",
				})
				break
			}
		}
	}

	// Sign of Strength (SOS): bullish bar breaking above AR high, volume > 1.5x avg.
	arHigh := 0.0
	if arIdx >= 0 {
		arHigh = bars[arIdx].High
	} else if tr != nil {
		arHigh = tr.High
	}
	if arHigh > 0 {
		for i := 0; i < 10 && i < n; i++ {
			bar := bars[i]
			if bar.Close > arHigh && bar.Volume > 1.5*avgVol20 && bar.Close > bar.Open {
				events = append(events, WyckoffEvent{
					Name:     "SOS",
					BarIndex: i,
					Price:    bar.Close,
					Volume:   bar.Volume / avgVol20,
					Signal:   "BULLISH",
				})
				break
			}
		}
	}

	// Upthrust (UT): price briefly pierces above range high, closes back inside.
	if tr != nil {
		rangeHigh := tr.High
		for i := 1; i < n-1; i++ {
			bar := bars[i]
			if bar.High > rangeHigh && bar.Close < rangeHigh {
				events = append(events, WyckoffEvent{
					Name:     "UT",
					BarIndex: i,
					Price:    bar.Close,
					Volume:   bar.Volume / avgVol20,
					Signal:   "BEARISH",
				})
				break
			}
		}
	}

	// Sign of Weakness (SOW): bearish bar closing below range low, volume > avg.
	if tr != nil {
		rangeLow := tr.Low
		for i := 0; i < 10 && i < n; i++ {
			bar := bars[i]
			if bar.Close < rangeLow && bar.Volume > avgVol20 && bar.Open > bar.Close {
				events = append(events, WyckoffEvent{
					Name:     "SOW",
					BarIndex: i,
					Price:    bar.Close,
					Volume:   bar.Volume / avgVol20,
					Signal:   "BEARISH",
				})
				break
			}
		}
	}

	return events
}

// ---------------------------------------------------------------------------
// Phase Classification
// ---------------------------------------------------------------------------

func wyckoffClassifyPhase(
	bars []OHLCV,
	atr float64,
	tr *TradingRange,
	events []WyckoffEvent,
	volumeDecl, volumeExpand bool,
) (WyckoffPhase, float64, string, string) {

	hasSC := wyckoffHasEvent(events, "SC")
	hasAR := wyckoffHasEvent(events, "AR")
	hasSpring := wyckoffHasEvent(events, "SPRING")
	hasSOS := wyckoffHasEvent(events, "SOS")
	hasUT := wyckoffHasEvent(events, "UT")
	hasSOW := wyckoffHasEvent(events, "SOW")

	currentPrice := bars[0].Close

	// MARKUP: SOS confirmed breakout.
	if hasSOS {
		conf := 0.70
		if tr != nil && currentPrice > tr.High {
			conf = 0.85
		}
		return PhaseMarkup, conf, "BULLISH",
			"Markup phase: SOS confirmed breakout above range. Bias BULLISH — look for pullbacks to support."
	}

	// MARKDOWN: SOW confirmed breakdown.
	if hasSOW {
		conf := 0.70
		if tr != nil && currentPrice < tr.Low {
			conf = 0.85
		}
		return PhaseMarkdown, conf, "BEARISH",
			"Markdown phase: SOW confirmed breakdown below range. Bias BEARISH — rallies are selling opportunities."
	}

	// DISTRIBUTION: price in range after markup, UT detected, volume declining.
	if tr != nil && hasUT && volumeDecl {
		conf := 0.55
		if hasSC {
			conf = 0.65
		}
		return PhaseDistribution, conf, "BEARISH",
			fmt.Sprintf("Distribution phase: Upthrust detected. Range %.4f–%.4f. Bias BEARISH — watch for SOW below %.4f.",
				tr.Low, tr.High, tr.Low)
	}

	// ACCUMULATION: price in range, SC + AR defined, declining volume.
	if tr != nil && hasSC && hasAR && volumeDecl {
		conf := 0.55
		if hasSpring {
			conf = 0.72 // Spring confirms Phase C
		}
		watchFor := fmt.Sprintf("%.4f", tr.High)
		return PhaseAccumulation, conf, "BULLISH",
			fmt.Sprintf("Accumulation phase: SC ✓ AR ✓ Spring %s. Range %.4f–%.4f. Bias BULLISH — watch for SOS above %s.",
				wyckoffCheck(hasSpring), tr.Low, tr.High, watchFor)
	}

	// Possible MARKUP — trending up, expanding volume, no range.
	if tr == nil && volumeExpand && len(bars) > 20 && currentPrice > bars[20].Close*1.005 {
		return PhaseMarkup, 0.45, "BULLISH",
			"Possible markup: price trending up with expanding volume. No clear range structure."
	}

	// Possible MARKDOWN — trending down, expanding volume, no range.
	if tr == nil && volumeExpand && len(bars) > 20 && currentPrice < bars[20].Close*0.995 {
		return PhaseMarkdown, 0.45, "BEARISH",
			"Possible markdown: price trending down with expanding volume. No clear range structure."
	}

	return PhaseTransition, 0.30, "NEUTRAL",
		"Transition phase: insufficient evidence to classify. Monitor for SC/BC events to define structure."
}

// ---------------------------------------------------------------------------
// Effort vs Result
// ---------------------------------------------------------------------------

func wyckoffEffortResult(bars []OHLCV, atr, avgVol20 float64, lookback int) []EffortResult {
	if len(bars) < lookback {
		lookback = len(bars)
	}
	results := make([]EffortResult, 0, lookback)
	for i := 0; i < lookback; i++ {
		bar := bars[i]
		effort := bar.Volume / avgVol20
		priceMove := (bar.High - bar.Low) / atr
		divergence := effort > 1.5 && priceMove < 0.5
		erType := "NORMAL"
		if divergence {
			erType = "ABSORPTION"
		} else if effort < 0.7 && priceMove > 1.0 {
			erType = "EASE_OF_MOVEMENT"
		}
		results = append(results, EffortResult{
			BarIndex:   i,
			Effort:     effort,
			Result:     priceMove,
			Divergence: divergence,
			Type:       erType,
		})
	}
	return results
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func wyckoffAvgVol(bars []OHLCV, start, end, window int) float64 {
	if end > len(bars) {
		end = len(bars)
	}
	if start >= end {
		return 0
	}
	count := end - start
	if count > window {
		count = window
	}
	sum := 0.0
	for i := start; i < start+count; i++ {
		sum += bars[i].Volume
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func wyckoffHasEvent(events []WyckoffEvent, name string) bool {
	for _, e := range events {
		if e.Name == name {
			return true
		}
	}
	return false
}

func wyckoffCheck(b bool) string {
	if b {
		return "✓"
	}
	return "pending"
}

// wyckoffCalcATR computes a simple ATR for use within CalcWyckoff when caller passes atr=0.
func wyckoffCalcATR(bars []OHLCV, period int) float64 {
	if len(bars) < period+1 {
		return 0
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		hl := bars[i].High - bars[i].Low
		hpc := bars[i].High - bars[i+1].Close
		if hpc < 0 {
			hpc = -hpc
		}
		lpc := bars[i].Low - bars[i+1].Close
		if lpc < 0 {
			lpc = -lpc
		}
		tr := hl
		if hpc > tr {
			tr = hpc
		}
		if lpc > tr {
			tr = lpc
		}
		sum += tr
	}
	return sum / float64(period)
}
