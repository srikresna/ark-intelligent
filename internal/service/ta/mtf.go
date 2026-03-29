package ta

import (
	"fmt"
	"math"
	"sort"
)

// ---------------------------------------------------------------------------
// Multi-Timeframe (MTF) Result Types
// ---------------------------------------------------------------------------

// MTFResult aggregates confluence scores across multiple timeframes using
// weighted averaging per CTA_SPEC.md.
type MTFResult struct {
	Timeframes    map[string]*ConfluenceResult // "15m", "30m", "1h", "4h", "daily", "weekly"
	WeightedScore float64                       // -100 to +100
	WeightedGrade string
	Alignment     string   // "STRONG_BULLISH", "BULLISH", "MIXED", "BEARISH", "STRONG_BEARISH"
	Matrix        []MTFRow // for display
}

// MTFRow represents one row in the multi-timeframe alignment matrix.
type MTFRow struct {
	Timeframe string
	Score     float64
	Grade     string
	Direction string
	Weight    float64
}

// ---------------------------------------------------------------------------
// Timeframe weights (from CTA_SPEC.md)
// ---------------------------------------------------------------------------

// tfWeights maps canonical timeframe names to their multi-timeframe weight.
var tfWeights = map[string]float64{
	"daily":  0.35,
	"4h":     0.25,
	"1h":     0.20,
	"15m":    0.10,
	"weekly": 0.10,
}

// tfOrder defines the display order for the MTF matrix (longest → shortest).
var tfOrder = []string{"weekly", "daily", "4h", "1h", "30m", "15m"}

// ---------------------------------------------------------------------------
// CalcMTF — Multi-Timeframe Alignment
// ---------------------------------------------------------------------------

// CalcMTF computes multi-timeframe alignment from a set of IndicatorSnapshots
// keyed by timeframe name (e.g. "daily", "4h", "1h", "15m", "weekly").
// Each snapshot is first scored via CalcConfluence, then timeframe weights
// are applied to produce a single weighted score and alignment reading.
//
// Any timeframes not found in tfWeights (e.g. "30m") are included in the
// matrix but do not contribute to the weighted score.
func CalcMTF(snapshots map[string]*IndicatorSnapshot) *MTFResult {
	if len(snapshots) == 0 {
		return &MTFResult{
			Timeframes:    map[string]*ConfluenceResult{},
			WeightedGrade: "F",
			Alignment:     "MIXED",
		}
	}

	// Compute confluence for each timeframe
	confluences := make(map[string]*ConfluenceResult, len(snapshots))
	for tf, snap := range snapshots {
		confluences[tf] = CalcConfluence(snap)
	}

	// Build matrix rows and compute weighted score
	weightedSum := 0.0
	totalWeight := 0.0
	bullCount := 0
	bearCount := 0
	totalDirectional := 0

	// Collect all timeframes present, sorted by tfOrder
	present := make([]string, 0, len(snapshots))
	for tf := range snapshots {
		present = append(present, tf)
	}
	sort.Slice(present, func(i, j int) bool {
		return tfOrderIndex(present[i]) < tfOrderIndex(present[j])
	})

	var matrix []MTFRow
	for _, tf := range present {
		conf := confluences[tf]
		w := tfWeights[tf] // 0 if not in the canonical set

		row := MTFRow{
			Timeframe: tf,
			Score:     conf.Score,
			Grade:     conf.Grade,
			Direction: conf.Direction,
			Weight:    w,
		}
		matrix = append(matrix, row)

		if w > 0 {
			weightedSum += conf.Score * w
			totalWeight += w
		}

		// Count directions (all timeframes count for alignment)
		totalDirectional++
		if conf.Direction == "BULLISH" {
			bullCount++
		} else if conf.Direction == "BEARISH" {
			bearCount++
		}
	}

	// Normalise weighted score
	ws := 0.0
	if totalWeight > 0 {
		ws = weightedSum / totalWeight
	}
	ws = math.Max(-100, math.Min(100, ws))

	// Weighted grade
	abs := math.Abs(ws)
	wg := "F"
	switch {
	case abs >= 75:
		wg = "A"
	case abs >= 50:
		wg = "B"
	case abs >= 25:
		wg = "C"
	case abs >= 1:
		wg = "D"
	}

	// Alignment determination
	alignment := determineAlignment(bullCount, bearCount, totalDirectional)

	return &MTFResult{
		Timeframes:    confluences,
		WeightedScore: ws,
		WeightedGrade: wg,
		Alignment:     alignment,
		Matrix:        matrix,
	}
}

// determineAlignment returns the alignment string based on direction counts.
// All same → "STRONG_X"; >60% agree → "X"; else "MIXED".
func determineAlignment(bull, bear, total int) string {
	if total == 0 {
		return "MIXED"
	}

	bullPct := float64(bull) / float64(total)
	bearPct := float64(bear) / float64(total)

	switch {
	case bull == total:
		return "STRONG_BULLISH"
	case bear == total:
		return "STRONG_BEARISH"
	case bullPct > 0.6:
		return "BULLISH"
	case bearPct > 0.6:
		return "BEARISH"
	default:
		return "MIXED"
	}
}

// tfOrderIndex returns the display position for a timeframe name.
func tfOrderIndex(tf string) int {
	for i, t := range tfOrder {
		if t == tf {
			return i
		}
	}
	return len(tfOrder) // unknown → at end
}

// FormatMTFMatrix returns a human-readable string of the MTF alignment matrix.
func FormatMTFMatrix(m *MTFResult) string {
	if m == nil || len(m.Matrix) == 0 {
		return "No MTF data available"
	}
	var sb []string
	sb = append(sb, fmt.Sprintf("MTF Alignment: %s (Score: %.1f, Grade: %s)", m.Alignment, m.WeightedScore, m.WeightedGrade))
	sb = append(sb, "")
	sb = append(sb, fmt.Sprintf("%-10s %8s %6s %10s %6s", "Timeframe", "Score", "Grade", "Direction", "Weight"))
	sb = append(sb, "─────────────────────────────────────────────")
	for _, row := range m.Matrix {
		sb = append(sb, fmt.Sprintf("%-10s %8.1f %6s %10s %5.0f%%",
			row.Timeframe, row.Score, row.Grade, row.Direction, row.Weight*100))
	}
	return joinLines(sb)
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
