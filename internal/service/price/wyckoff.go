package price

import (
	"fmt"
	"math"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// Wyckoff Phase Detector
// ---------------------------------------------------------------------------
//
// Classifies market phase using Wyckoff methodology:
//   ACCUMULATION — smart money absorbing supply; expect markup
//   MARKUP        — trending up after accumulation
//   DISTRIBUTION  — smart money distributing; expect markdown
//   MARKDOWN      — trending down after distribution
//
// Key events detected within the last 60 bars:
//   PS/SC/AR/ST — Phase A setup events
//   SPRING/UPTHRUST — Phase C tests
//   SOS/SOW/LPS/LPSY — Phase D confirmation events
//
// Input: []domain.DailyPrice, newest-first. Requires >= 60 bars.

// WyckoffEvent represents a detected Wyckoff key event.
type WyckoffEvent struct {
	Type        string  `json:"type"`        // PS, SC, AR, ST, SPRING, UPTHRUST, SOS, SOW, LPS, LPSY
	Price       float64 `json:"price"`       // Close price at event bar
	BarIndex    int     `json:"bar_index"`   // Index in input slice (0 = newest)
	Description string  `json:"description"` // Human-readable description
}

// WyckoffResult holds the output of Wyckoff phase analysis.
type WyckoffResult struct {
	Phase          string         `json:"phase"`           // ACCUMULATION / MARKUP / DISTRIBUTION / MARKDOWN / UNCERTAIN
	SubPhase       string         `json:"sub_phase"`       // PHASE_A / PHASE_B / PHASE_C / PHASE_D / PHASE_E
	Confidence     int            `json:"confidence"`      // 0-100
	KeyEvents      []WyckoffEvent `json:"key_events"`      // Events in last 60 bars
	SupportZone    float64        `json:"support_zone"`    // Approximate range low
	ResistanceZone float64        `json:"resistance_zone"` // Approximate range high
	Interpretation string         `json:"interpretation"`  // Human-readable summary
}

// AnalyzeWyckoff runs Wyckoff phase detection on daily bars (newest-first).
// Returns nil if fewer than 60 bars are provided.
func AnalyzeWyckoff(bars []domain.DailyPrice) *WyckoffResult {
	if len(bars) < 60 {
		return nil
	}

	// Work on the 120 most recent bars (newest-first); invert to oldest-first for chronological logic.
	lookback := 120
	if len(bars) < lookback {
		lookback = len(bars)
	}
	slice := bars[:lookback]

	// Reverse to oldest-first for processing.
	n := len(slice)
	chron := make([]domain.DailyPrice, n)
	for i, b := range slice {
		chron[n-1-i] = b
	}

	// Compute ATR (14-day) across the window.
	atr := computeWyckoffATR(chron, 14)

	// Trading range: support/resistance from the last 60 bars (last 60 in chron slice).
	start60 := n - 60
	if start60 < 0 {
		start60 = 0
	}
	recent60 := chron[start60:]
	support, resistance := tradingRange(recent60)
	rangeSize := resistance - support
	_ = rangeSize

	// --- Feature detection ---

	// 1. Range-bound: low ATR slope (volatility contracting over last 30 bars).
	atrTrend := atrSlope(chron, atr, 30)
	isConsolidating := atrTrend < 0

	// 2. Volume characteristics at swing lows vs swing highs.
	swingLowVol, swingHighVol := swingVolumeRatio(recent60, support, resistance, atr)
	highVolAtLow := swingLowVol > swingHighVol*1.15  // volume higher at lows -> Accumulation
	highVolAtHigh := swingHighVol > swingLowVol*1.15 // volume higher at highs -> Distribution

	// 3. Spring detection: brief undercut of support + recovery.
	spring := detectSpring(recent60, support, atr)

	// 4. Upthrust detection: brief overshot of resistance + rejection.
	upthrust := detectUpthrust(recent60, resistance, atr)

	// 5. Sign of Strength: strong rally with expanding volume after potential spring.
	sos := detectSOS(recent60, resistance, atr)

	// 6. Sign of Weakness: strong decline with expanding volume.
	sow := detectSOW(recent60, support, atr)

	// 7. Preliminary Support / Selling Climax / Automatic Rally / Secondary Test (Phase A).
	ps, sc, ar, st := detectPhaseA(recent60, support, resistance, atr)

	// 8. LPS / LPSY (Phase D)
	lps, lpsy := detectPhaseDEvents(recent60, support, resistance, atr)

	// 9. Markup / Markdown trend: price clearly above/below range midpoint with momentum.
	mid := (support + resistance) / 2
	latestClose := chron[n-1].Close
	prevClose := chron[n-10].Close
	trendUp := latestClose > resistance*0.97 && latestClose > prevClose*1.01
	trendDown := latestClose < support*1.03 && latestClose < prevClose*0.99

	// --- Collect events ---
	events := []WyckoffEvent{}
	if ps != nil {
		events = append(events, *ps)
	}
	if sc != nil {
		events = append(events, *sc)
	}
	if ar != nil {
		events = append(events, *ar)
	}
	if st != nil {
		events = append(events, *st)
	}
	if spring != nil {
		events = append(events, *spring)
	}
	if upthrust != nil {
		events = append(events, *upthrust)
	}
	if sos != nil {
		events = append(events, *sos)
	}
	if sow != nil {
		events = append(events, *sow)
	}
	if lps != nil {
		events = append(events, *lps)
	}
	if lpsy != nil {
		events = append(events, *lpsy)
	}

	// --- Phase classification ---
	phase, subPhase, confidence := classifyWyckoffPhase(
		isConsolidating, highVolAtLow, highVolAtHigh,
		spring, upthrust, sos, sow, lps, lpsy, sc, ar, st,
		trendUp, trendDown, latestClose, mid,
	)

	interpretation := buildWyckoffInterpretation(phase, subPhase, confidence, events, support, resistance)

	return &WyckoffResult{
		Phase:          phase,
		SubPhase:       subPhase,
		Confidence:     confidence,
		KeyEvents:      events,
		SupportZone:    support,
		ResistanceZone: resistance,
		Interpretation: interpretation,
	}
}

// ---------------------------------------------------------------------------
// Phase classification
// ---------------------------------------------------------------------------

func classifyWyckoffPhase(
	isConsolidating, highVolAtLow, highVolAtHigh bool,
	spring, upthrust, sos, sow, lps, lpsy, sc, ar, st *WyckoffEvent,
	trendUp, trendDown bool,
	latestClose, mid float64,
) (phase, subPhase string, confidence int) {
	accChars := 0
	distChars := 0
	total := 0

	if isConsolidating {
		accChars++
		distChars++
		total++
	}
	if highVolAtLow {
		accChars += 2
		total += 2
	}
	if highVolAtHigh {
		distChars += 2
		total += 2
	}
	if spring != nil {
		accChars += 3
		total += 3
	}
	if sos != nil {
		accChars += 2
		total += 2
	}
	if lps != nil {
		accChars++
		total++
	}
	if sc != nil {
		accChars++
		total++
	}
	if ar != nil {
		accChars++
		total++
	}
	if st != nil {
		accChars++
		total++
	}
	if upthrust != nil {
		distChars += 3
		total += 3
	}
	if sow != nil {
		distChars += 2
		total += 2
	}
	if lpsy != nil {
		distChars++
		total++
	}
	if latestClose > mid {
		accChars++
		total++
	} else {
		distChars++
		total++
	}

	if total == 0 {
		return "UNCERTAIN", "PHASE_B", 0
	}

	accScore := float64(accChars) / float64(total)
	distScore := float64(distChars) / float64(total)

	if trendUp && sos != nil && accScore > 0.4 {
		sub := wyckoffSubPhase(true, spring, sos, lps, ar, st)
		return "MARKUP", sub, wyckoffClamp(int(accScore*100)+10, 0, 95)
	}
	if trendDown && sow != nil && distScore > 0.4 {
		sub := wyckoffSubPhase(false, upthrust, sow, lpsy, ar, st)
		return "MARKDOWN", sub, wyckoffClamp(int(distScore*100)+10, 0, 95)
	}
	if accScore >= distScore && accScore > 0.3 {
		sub := wyckoffSubPhase(true, spring, sos, lps, ar, st)
		return "ACCUMULATION", sub, wyckoffClamp(int(accScore*100), 0, 90)
	}
	if distScore > accScore && distScore > 0.3 {
		sub := wyckoffSubPhase(false, upthrust, sow, lpsy, ar, st)
		return "DISTRIBUTION", sub, wyckoffClamp(int(distScore*100), 0, 90)
	}
	return "UNCERTAIN", "PHASE_B", wyckoffClamp(int(math.Max(accScore, distScore)*60), 0, 40)
}

func wyckoffSubPhase(isAcc bool, test, confirm, late, ar, st *WyckoffEvent) string {
	_ = isAcc
	if confirm != nil && late != nil {
		return "PHASE_D"
	}
	if test != nil {
		return "PHASE_C"
	}
	if st != nil {
		return "PHASE_B"
	}
	if ar != nil {
		return "PHASE_A"
	}
	return "PHASE_B"
}

// ---------------------------------------------------------------------------
// Event detectors
// ---------------------------------------------------------------------------

func detectSpring(bars []domain.DailyPrice, support, atr float64) *WyckoffEvent {
	threshold := support - atr*0.3
	for i := 2; i < len(bars)-1; i++ {
		b := bars[i]
		if b.Low < threshold && b.Close > support*0.995 {
			recovered := false
			for j := i + 1; j < i+3 && j < len(bars); j++ {
				if bars[j].Close > support {
					recovered = true
					break
				}
			}
			if recovered {
				return &WyckoffEvent{
					Type:        "SPRING",
					Price:       b.Close,
					BarIndex:    len(bars) - 1 - i,
					Description: fmt.Sprintf("Spring — brief dip below support %.4f, recovered quickly", support),
				}
			}
		}
	}
	return nil
}

func detectUpthrust(bars []domain.DailyPrice, resistance, atr float64) *WyckoffEvent {
	threshold := resistance + atr*0.3
	for i := 2; i < len(bars)-1; i++ {
		b := bars[i]
		if b.High > threshold && b.Close < resistance*1.005 {
			rejected := false
			for j := i + 1; j < i+3 && j < len(bars); j++ {
				if bars[j].Close < resistance {
					rejected = true
					break
				}
			}
			if rejected {
				return &WyckoffEvent{
					Type:        "UPTHRUST",
					Price:       b.Close,
					BarIndex:    len(bars) - 1 - i,
					Description: fmt.Sprintf("Upthrust — push above resistance %.4f rejected", resistance),
				}
			}
		}
	}
	return nil
}

func detectSOS(bars []domain.DailyPrice, resistance, atr float64) *WyckoffEvent {
	avg := avgVolume(bars)
	for i := 5; i < len(bars); i++ {
		b := bars[i]
		rally := b.Close - b.Open
		if rally > atr*0.6 && b.Volume > avg*1.3 && b.Close > resistance*0.98 {
			return &WyckoffEvent{
				Type:        "SOS",
				Price:       b.Close,
				BarIndex:    len(bars) - 1 - i,
				Description: "Sign of Strength — strong rally bar with expanding volume",
			}
		}
	}
	return nil
}

func detectSOW(bars []domain.DailyPrice, support, atr float64) *WyckoffEvent {
	avg := avgVolume(bars)
	for i := 5; i < len(bars); i++ {
		b := bars[i]
		drop := b.Open - b.Close
		if drop > atr*0.6 && b.Volume > avg*1.3 && b.Close < support*1.02 {
			return &WyckoffEvent{
				Type:        "SOW",
				Price:       b.Close,
				BarIndex:    len(bars) - 1 - i,
				Description: "Sign of Weakness — strong decline bar with expanding volume",
			}
		}
	}
	return nil
}

func detectPhaseA(bars []domain.DailyPrice, support, resistance, atr float64) (ps, sc, ar, st *WyckoffEvent) {
	avg := avgVolume(bars)
	n := len(bars)
	_ = resistance

	for i := 0; i < n; i++ {
		b := bars[i]
		relIdx := n - 1 - i

		if sc == nil && i < n/2 {
			drop := b.Open - b.Close
			if drop > atr && b.Volume > avg*1.8 && b.Low <= support*1.02 {
				sc = &WyckoffEvent{
					Type:        "SC",
					Price:       b.Close,
					BarIndex:    relIdx,
					Description: "Selling Climax — large down bar near support on high volume",
				}
			}
		}

		if ps == nil && sc == nil && i < n/3 {
			if b.Volume > avg*1.4 && b.Close > b.Open && b.Low <= support*1.03 {
				ps = &WyckoffEvent{
					Type:        "PS",
					Price:       b.Close,
					BarIndex:    relIdx,
					Description: "Preliminary Support — first buying on heavy volume",
				}
			}
		}

		if ar == nil && sc != nil {
			rally := b.Close - b.Open
			if rally > atr*0.7 && b.Close > sc.Price*1.01 {
				ar = &WyckoffEvent{
					Type:        "AR",
					Price:       b.Close,
					BarIndex:    relIdx,
					Description: "Automatic Rally — bounce from Selling Climax",
				}
			}
		}

		if st == nil && sc != nil && ar != nil {
			if b.Low <= sc.Price*1.03 && b.Volume < avg*0.9 {
				st = &WyckoffEvent{
					Type:        "ST",
					Price:       b.Close,
					BarIndex:    relIdx,
					Description: "Secondary Test — retest SC lows on lower volume",
				}
			}
		}
	}
	return
}

func detectPhaseDEvents(bars []domain.DailyPrice, support, resistance, atr float64) (lps, lpsy *WyckoffEvent) {
	n := len(bars)
	avg := avgVolume(bars)
	_ = atr

	for i := n / 2; i < n; i++ {
		b := bars[i]
		relIdx := n - 1 - i

		if lps == nil && b.Low <= support*1.02 && b.Close > support && b.Volume < avg*0.85 {
			lps = &WyckoffEvent{
				Type:        "LPS",
				Price:       b.Close,
				BarIndex:    relIdx,
				Description: "Last Point of Support — low-volume pullback to support",
			}
		}

		if lpsy == nil && b.High >= resistance*0.98 && b.Close < resistance && b.Volume < avg*0.85 {
			lpsy = &WyckoffEvent{
				Type:        "LPSY",
				Price:       b.Close,
				BarIndex:    relIdx,
				Description: "Last Point of Supply — low-volume rally failing at resistance",
			}
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func computeWyckoffATR(bars []domain.DailyPrice, period int) float64 {
	if len(bars) == 0 {
		return 0
	}
	if len(bars) < period+1 {
		sum := 0.0
		for _, b := range bars {
			sum += b.High - b.Low
		}
		return sum / float64(len(bars))
	}
	sum := 0.0
	for i := 1; i <= period; i++ {
		tr := math.Max(bars[i].High-bars[i].Low,
			math.Max(math.Abs(bars[i].High-bars[i-1].Close),
				math.Abs(bars[i].Low-bars[i-1].Close)))
		sum += tr
	}
	return sum / float64(period)
}

func tradingRange(bars []domain.DailyPrice) (support, resistance float64) {
	if len(bars) == 0 {
		return 0, 0
	}
	support = bars[0].Low
	resistance = bars[0].High
	for _, b := range bars {
		if b.Low < support {
			support = b.Low
		}
		if b.High > resistance {
			resistance = b.High
		}
	}
	return
}

func atrSlope(bars []domain.DailyPrice, globalATR float64, window int) float64 {
	n := len(bars)
	if n < window+1 || globalATR == 0 {
		return 0
	}
	half := window / 2
	start := n - window
	sumEarly, sumLate := 0.0, 0.0
	for i := start; i < start+half; i++ {
		sumEarly += bars[i].High - bars[i].Low
	}
	for i := start + half; i < start+window; i++ {
		sumLate += bars[i].High - bars[i].Low
	}
	return (sumLate - sumEarly) / float64(half) / globalATR
}

func swingVolumeRatio(bars []domain.DailyPrice, support, resistance, atr float64) (lowVol, highVol float64) {
	var lowSum, highSum float64
	var lowCount, highCount int
	threshold := atr * 0.5
	for _, b := range bars {
		if b.Low <= support+threshold {
			lowSum += b.Volume
			lowCount++
		}
		if b.High >= resistance-threshold {
			highSum += b.Volume
			highCount++
		}
	}
	if lowCount > 0 {
		lowVol = lowSum / float64(lowCount)
	}
	if highCount > 0 {
		highVol = highSum / float64(highCount)
	}
	return
}

func avgVolume(bars []domain.DailyPrice) float64 {
	if len(bars) == 0 {
		return 1
	}
	sum := 0.0
	count := 0
	for _, b := range bars {
		if b.Volume > 0 {
			sum += b.Volume
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return sum / float64(count)
}

func wyckoffClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func buildWyckoffInterpretation(phase, subPhase string, confidence int, events []WyckoffEvent, support, resistance float64) string {
	eventNames := []string{}
	for _, e := range events {
		eventNames = append(eventNames, e.Type)
	}
	detail := ""
	if len(eventNames) > 0 {
		detail = " ("
		for i, name := range eventNames {
			if i > 0 {
				detail += ", "
			}
			detail += name
		}
		detail += " terdeteksi)"
	}
	return fmt.Sprintf("Wyckoff: %s %s%s — Confidence: %d%% | Support: %.4f | Resistance: %.4f",
		phase, subPhase, detail, confidence, support, resistance)
}
