package ta

import (
	"fmt"
	"math"
	"strings"
)

// ---------------------------------------------------------------------------
// Confluence Result
// ---------------------------------------------------------------------------

// ConfluenceResult aggregates normalised signals from all available indicators
// into a single directional score with a letter grade.
type ConfluenceResult struct {
	Score           float64    // -100 to +100
	Grade           string     // "A", "B", "C", "D", "F"
	Direction       string     // "BULLISH", "BEARISH", "NEUTRAL"
	BullishCount    int        // number of bullish indicators
	BearishCount    int        // number of bearish indicators
	NeutralCount    int        // number of neutral indicators
	TotalIndicators int
	Signals         []TASignal // individual indicator signals
	Summary         string     // human-readable summary
}

// ---------------------------------------------------------------------------
// Weight categories (from CTA_SPEC.md)
// ---------------------------------------------------------------------------

// indicatorWeight defines the default weight for an indicator and its category.
type indicatorWeight struct {
	category string
	weight   float64
}

// defaultWeights maps indicator name → category + weight.
// Categories: Trend (40%), Momentum (35%), Volume (15%), Volatility (10%).
var defaultWeights = map[string]indicatorWeight{
	// Trend (40%)
	"EMA_RIBBON":  {category: "trend", weight: 0.15},
	"SUPERTREND":  {category: "trend", weight: 0.10},
	"ICHIMOKU":    {category: "trend", weight: 0.10},
	"ADX":         {category: "trend", weight: 0.05},
	// Momentum (35%)
	"RSI":         {category: "momentum", weight: 0.10},
	"MACD":        {category: "momentum", weight: 0.12},
	"STOCHASTIC":  {category: "momentum", weight: 0.08},
	"CCI":         {category: "momentum", weight: 0.05},
	// Volume (15%)
	"OBV":         {category: "volume", weight: 0.08},
	"MFI":         {category: "volume", weight: 0.07},
	// Volatility (10%)
	"BOLLINGER":   {category: "volatility", weight: 0.06},
	"WILLIAMS_R":  {category: "volatility", weight: 0.04},
	// Structure (SMC) — additional signal, does not replace existing categories
	"SMC":          {category: "trend", weight: 0.15},
}

// ---------------------------------------------------------------------------
// Per-indicator signal extraction helpers (-1 to +1)
// ---------------------------------------------------------------------------

func signalRSI(r *RSIResult) (float64, string) {
	if r == nil {
		return 0, ""
	}
	v := r.Value
	switch {
	case v < 30:
		return 0.8, fmt.Sprintf("RSI oversold at %.1f", v)
	case v < 45:
		return 0.3, fmt.Sprintf("RSI mildly bullish at %.1f", v)
	case v <= 55:
		return 0, fmt.Sprintf("RSI neutral at %.1f", v)
	case v <= 70:
		return -0.3, fmt.Sprintf("RSI mildly bearish at %.1f", v)
	default:
		return -0.8, fmt.Sprintf("RSI overbought at %.1f", v)
	}
}

func signalMACD(m *MACDResult) (float64, string) {
	if m == nil {
		return 0, ""
	}
	switch {
	case m.BullishCross:
		return 1.0, "MACD bullish crossover"
	case m.BearishCross:
		return -1.0, "MACD bearish crossover"
	case m.Histogram > 0:
		return 0.4, fmt.Sprintf("MACD histogram positive (%.4f)", m.Histogram)
	case m.Histogram < 0:
		return -0.4, fmt.Sprintf("MACD histogram negative (%.4f)", m.Histogram)
	default:
		return 0, "MACD neutral"
	}
}

func signalStochastic(s *StochasticResult) (float64, string) {
	if s == nil {
		return 0, ""
	}
	k, d := s.K, s.D
	switch {
	case k < d && k < 20:
		return 0.8, fmt.Sprintf("Stoch oversold %%K=%.1f %%D=%.1f", k, d)
	case k > d && k > 80:
		return -0.8, fmt.Sprintf("Stoch overbought %%K=%.1f %%D=%.1f", k, d)
	case s.Cross == "BULLISH_CROSS":
		return 0.5, "Stochastic bullish cross"
	case s.Cross == "BEARISH_CROSS":
		return -0.5, "Stochastic bearish cross"
	default:
		return 0, fmt.Sprintf("Stoch neutral %%K=%.1f", k)
	}
}

func signalBollinger(b *BollingerResult) (float64, string) {
	if b == nil {
		return 0, ""
	}
	if b.Squeeze {
		return 0, "Bollinger squeeze (neutral)"
	}
	switch {
	case b.PercentB < 0.05:
		return 0.7, fmt.Sprintf("Price near lower BB (%%B=%.2f), bounce potential", b.PercentB)
	case b.PercentB > 0.95:
		return -0.7, fmt.Sprintf("Price near upper BB (%%B=%.2f), reversal potential", b.PercentB)
	default:
		return 0, fmt.Sprintf("Bollinger neutral (%%B=%.2f)", b.PercentB)
	}
}

func signalEMARibbon(e *EMAResult) (float64, string) {
	if e == nil {
		return 0, ""
	}
	return e.AlignmentScore, fmt.Sprintf("EMA ribbon %s (score=%.2f)", e.RibbonAlignment, e.AlignmentScore)
}

func signalADX(a *ADXResult) (float64, string) {
	if a == nil {
		return 0, ""
	}
	// Direction from +DI vs -DI
	dir := 0.0
	if a.PlusDI > a.MinusDI {
		dir = 1.0
	} else if a.MinusDI > a.PlusDI {
		dir = -1.0
	}
	// Scale by ADX strength: ADX>25 → full weight, ADX<20 → half, interpolate between
	scale := 0.5
	if a.ADX >= 25 {
		scale = 1.0
	} else if a.ADX > 20 {
		scale = 0.5 + (a.ADX-20)/(25-20)*0.5
	}
	sig := dir * scale
	return sig, fmt.Sprintf("ADX=%.1f +DI=%.1f -DI=%.1f", a.ADX, a.PlusDI, a.MinusDI)
}

func signalOBV(o *OBVResult) (float64, string) {
	if o == nil {
		return 0, ""
	}
	switch o.Trend {
	case "RISING":
		return 0.6, "OBV trending up"
	case "FALLING":
		return -0.6, "OBV trending down"
	default:
		return 0, "OBV flat"
	}
}

func signalWilliamsR(w *WilliamsRResult) (float64, string) {
	if w == nil {
		return 0, ""
	}
	switch {
	case w.Value < -80:
		return 0.7, fmt.Sprintf("Williams %%R oversold at %.1f", w.Value)
	case w.Value > -20:
		return -0.7, fmt.Sprintf("Williams %%R overbought at %.1f", w.Value)
	default:
		return 0, fmt.Sprintf("Williams %%R neutral at %.1f", w.Value)
	}
}

func signalCCI(c *CCIResult) (float64, string) {
	if c == nil {
		return 0, ""
	}
	switch {
	case c.Value < -100:
		return 0.7, fmt.Sprintf("CCI oversold at %.1f", c.Value)
	case c.Value > 100:
		return -0.7, fmt.Sprintf("CCI overbought at %.1f", c.Value)
	default:
		return 0, fmt.Sprintf("CCI neutral at %.1f", c.Value)
	}
}

func signalMFI(m *MFIResult) (float64, string) {
	if m == nil {
		return 0, ""
	}
	switch {
	case m.Value < 20:
		return 0.7, fmt.Sprintf("MFI oversold at %.1f", m.Value)
	case m.Value > 80:
		return -0.7, fmt.Sprintf("MFI overbought at %.1f", m.Value)
	default:
		return 0, fmt.Sprintf("MFI neutral at %.1f", m.Value)
	}
}

// ---------------------------------------------------------------------------
// CalcConfluence — main multi-indicator confluence scoring
// ---------------------------------------------------------------------------

// CalcConfluence computes a composite directional score from all available
// indicators in the snapshot. Missing (nil) indicators have their weight
// redistributed proportionally among other indicators in the same category.
func CalcConfluence(snap *IndicatorSnapshot) *ConfluenceResult {
	if snap == nil {
		return &ConfluenceResult{Grade: "F", Direction: "NEUTRAL"}
	}

	// Collect raw signals: name → (signal, note, available)
	type rawSig struct {
		signal float64
		note   string
		avail  bool
	}

	raw := map[string]rawSig{}

	// Extract signals for each indicator
	if snap.RSI != nil {
		s, n := signalRSI(snap.RSI)
		raw["RSI"] = rawSig{s, n, true}
	}
	if snap.MACD != nil {
		s, n := signalMACD(snap.MACD)
		raw["MACD"] = rawSig{s, n, true}
	}
	if snap.Stochastic != nil {
		s, n := signalStochastic(snap.Stochastic)
		raw["STOCHASTIC"] = rawSig{s, n, true}
	}
	if snap.Bollinger != nil {
		s, n := signalBollinger(snap.Bollinger)
		raw["BOLLINGER"] = rawSig{s, n, true}
	}
	if snap.EMA != nil {
		s, n := signalEMARibbon(snap.EMA)
		raw["EMA_RIBBON"] = rawSig{s, n, true}
	}
	if snap.ADX != nil {
		s, n := signalADX(snap.ADX)
		raw["ADX"] = rawSig{s, n, true}
	}
	if snap.OBV != nil {
		s, n := signalOBV(snap.OBV)
		raw["OBV"] = rawSig{s, n, true}
	}
	if snap.WilliamsR != nil {
		s, n := signalWilliamsR(snap.WilliamsR)
		raw["WILLIAMS_R"] = rawSig{s, n, true}
	}
	if snap.CCI != nil {
		s, n := signalCCI(snap.CCI)
		raw["CCI"] = rawSig{s, n, true}
	}
	if snap.MFI != nil {
		s, n := signalMFI(snap.MFI)
		raw["MFI"] = rawSig{s, n, true}
	}

	// Ichimoku (nil-safe — may not exist yet)
	ichSig, ichNote := signalIchimokuFromSnap(snap)
	if ichNote != "" {
		raw["ICHIMOKU"] = rawSig{ichSig, ichNote, true}
	}

	// SuperTrend (nil-safe — may not exist yet)
	stSig, stNote := signalSuperTrendFromSnap(snap)
	if stNote != "" {
		raw["SUPERTREND"] = rawSig{stSig, stNote, true}
	}

	// SMC: Smart Money Concepts (BOS, CHOCH, structure)
	smcSig, smcNote := signalSMCFromSnap(snap)
	if smcNote != "" {
		raw["SMC"] = rawSig{smcSig, smcNote, true}
	}

	// Compute category totals for redistribution
	catTotal := map[string]float64{}   // sum of default weights per category
	catAvail := map[string]float64{}   // sum of available weights per category
	for name, iw := range defaultWeights {
		catTotal[iw.category] += iw.weight
		if _, ok := raw[name]; ok {
			catAvail[iw.category] += iw.weight
		}
	}

	// Compute effective weight: if some indicators in a category are missing,
	// redistribute their weight proportionally.
	effectiveWeight := func(name string) float64 {
		iw, ok := defaultWeights[name]
		if !ok {
			return 0
		}
		avail := catAvail[iw.category]
		if avail == 0 {
			return 0 // entire category missing
		}
		total := catTotal[iw.category]
		return iw.weight * (total / avail)
	}

	// Compute weighted score
	weightedSum := 0.0
	totalWeight := 0.0
	bullish, bearish, neutral := 0, 0, 0
	var signals []TASignal

	for name, rs := range raw {
		w := effectiveWeight(name)
		totalWeight += w
		weightedSum += rs.signal * w

		signals = append(signals, TASignal{
			Indicator: name,
			Value:     rs.signal,
			Weight:    w,
			Note:      rs.note,
		})

		if rs.signal > 0.05 {
			bullish++
		} else if rs.signal < -0.05 {
			bearish++
		} else {
			neutral++
		}
	}

	// Normalise score to -100..+100
	score := 0.0
	if totalWeight > 0 {
		score = (weightedSum / totalWeight) * 100
	}
	score = math.Max(-100, math.Min(100, score))

	// Grade
	abs := math.Abs(score)
	grade := "F"
	switch {
	case abs >= 75:
		grade = "A"
	case abs >= 50:
		grade = "B"
	case abs >= 25:
		grade = "C"
	case abs >= 1:
		grade = "D"
	}

	// Direction
	direction := "NEUTRAL"
	if score > 0 {
		direction = "BULLISH"
	} else if score < 0 {
		direction = "BEARISH"
	}

	// Summary
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s (Grade %s, Score %.1f) — %d bullish, %d bearish, %d neutral out of %d indicators",
		direction, grade, score, bullish, bearish, neutral, bullish+bearish+neutral)

	return &ConfluenceResult{
		Score:           score,
		Grade:           grade,
		Direction:       direction,
		BullishCount:    bullish,
		BearishCount:    bearish,
		NeutralCount:    neutral,
		TotalIndicators: bullish + bearish + neutral,
		Signals:         signals,
		Summary:         sb.String(),
	}
}

// ---------------------------------------------------------------------------
// Nil-safe signal helpers for advanced indicators
// ---------------------------------------------------------------------------

// signalIchimokuFromSnap extracts an Ichimoku signal from the snapshot.
// Returns (0, "") if Ichimoku data is not available.
func signalIchimokuFromSnap(snap *IndicatorSnapshot) (float64, string) {
	if snap.Ichimoku == nil {
		return 0, ""
	}
	ich := snap.Ichimoku

	// Composite signal from TK cross + cloud position + chikou
	sig := 0.0
	parts := 0

	// TK Cross: +1 bullish, -1 bearish
	switch ich.TKCross {
	case "BULLISH_CROSS":
		sig += 1.0
		parts++
	case "BEARISH_CROSS":
		sig -= 1.0
		parts++
	default:
		parts++
	}

	// Kumo breakout / cloud position
	switch ich.KumoBreakout {
	case "BULLISH_BREAKOUT":
		sig += 1.0
	case "BEARISH_BREAKOUT":
		sig -= 1.0
	case "INSIDE_CLOUD":
		// neutral, no change
	}
	parts++

	// Chikou span
	switch ich.ChikouSignal {
	case "BULLISH":
		sig += 0.5
	case "BEARISH":
		sig -= 0.5
	}
	parts++

	// Cloud color
	switch ich.CloudColor {
	case "BULLISH":
		sig += 0.5
	case "BEARISH":
		sig -= 0.5
	}
	parts++

	// Normalise to -1..+1
	if parts > 0 {
		sig = sig / float64(parts)
	}
	if sig > 1 {
		sig = 1
	} else if sig < -1 {
		sig = -1
	}

	return sig, fmt.Sprintf("Ichimoku %s (TK=%s, Kumo=%s, Chikou=%s)",
		ich.Overall, ich.TKCross, ich.KumoBreakout, ich.ChikouSignal)
}

// signalSuperTrendFromSnap extracts a SuperTrend signal from the snapshot.
// Returns (0, "") if SuperTrend data is not available.
func signalSuperTrendFromSnap(snap *IndicatorSnapshot) (float64, string) {
	if snap.SuperTrend == nil {
		return 0, ""
	}
	st := snap.SuperTrend
	switch st.Direction {
	case "UP":
		return 1.0, fmt.Sprintf("SuperTrend bullish (%.4f)", st.Value)
	case "DOWN":
		return -1.0, fmt.Sprintf("SuperTrend bearish (%.4f)", st.Value)
	default:
		return 0, "SuperTrend neutral"
	}
}

// signalSMCFromSnap extracts a market structure signal from the SMC analysis.
// BOS confirms trend continuation; CHOCH signals reversal.
// Returns (0, "") if SMC data is not available.
func signalSMCFromSnap(snap *IndicatorSnapshot) (float64, string) {
	if snap.SMC == nil {
		return 0, ""
	}
	smc := snap.SMC

	// Base signal from overall structure
	sig := 0.0
	switch smc.Structure {
	case StructureBullish:
		sig = 0.5
	case StructureBearish:
		sig = -0.5
	}

	// Boost if there is a recent CHOCH (reversal signal -- stronger weight)
	if len(smc.RecentCHOCH) > 0 {
		choch := smc.RecentCHOCH[0]
		if choch.Dir == "BULLISH" {
			sig = 0.8
		} else if choch.Dir == "BEARISH" {
			sig = -0.8
		}
	}

	// Boost if there is a recent BOS (continuation signal)
	if len(smc.RecentBOS) > 0 {
		bos := smc.RecentBOS[0]
		if bos.Dir == "BULLISH" && sig >= 0 {
			sig = 0.6
		} else if bos.Dir == "BEARISH" && sig <= 0 {
			sig = -0.6
		}
	}

	// Build note
	zone := smc.CurrentZone
	note := fmt.Sprintf("SMC %s (structure=%s, zone=%s", smc.Trend, smc.Structure, zone)
	if len(smc.RecentCHOCH) > 0 {
		note += fmt.Sprintf(", CHOCH %s", smc.RecentCHOCH[0].Dir)
	}
	if len(smc.RecentBOS) > 0 {
		note += fmt.Sprintf(", BOS %s", smc.RecentBOS[0].Dir)
	}
	note += ")"

	return sig, note
}
