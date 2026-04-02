// Package ta — AMT Module 5: Multi-Day Migration + MGI (Market-Generated Information).
//
// Tracks POC and Value Area migration across multiple days to identify
// value acceptance/rejection and composite context (weekly + monthly).
package ta

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// MigrationDirection describes the direction of value migration.
type MigrationDirection string

const (
	MigrationUp       MigrationDirection = "UP"       // POC shifting higher
	MigrationDown     MigrationDirection = "DOWN"     // POC shifting lower
	MigrationOverlap  MigrationDirection = "OVERLAP"  // VA overlap but no clear drift
	MigrationBalanced MigrationDirection = "BALANCED" // stable VA across days
)

// MGILevel holds acceptance/rejection info at a key level.
type MGILevel struct {
	Price       float64 // the key level (prior POC or VA edge)
	LevelType   string  // "POC" | "VAH" | "VAL"
	Accepted    bool    // true if price spent time at this level (accepted)
	Description string  // human-readable explanation
}

// DayMigration holds the migration data for a single day.
type DayMigration struct {
	Date time.Time
	VA   ValueArea

	// Relative to previous day
	POCShift       float64            // POC today - POC yesterday (positive = up)
	VAOverlap      float64            // fraction of today's VA that overlaps with yesterday (0–1)
	Direction      MigrationDirection // direction of value migration
	MGILevels      []MGILevel         // acceptance/rejection at key levels
}

// CompositeVA holds a composite Value Area computed over multiple days.
type CompositeVA struct {
	Period string  // "WEEKLY" | "MONTHLY"
	POC    float64
	VAH    float64
	VAL    float64
	High   float64
	Low    float64
}

// AMTMigrationResult holds the full multi-day migration analysis.
type AMTMigrationResult struct {
	Days []DayMigration

	// Composite Value Areas
	WeeklyVA  *CompositeVA
	MonthlyVA *CompositeVA

	// Overall migration score
	MigrationScore float64            // -100 to +100 (positive = value migrating up)
	NetDirection   MigrationDirection // overall net direction
	Summary        string             // human-readable summary

	// Text-based POC migration chart
	MigrationChart string // ASCII visualization of POC migration
}

// ClassifyMigration analyses the value migration over the last N trading days.
// bars must be intraday (30m recommended), newest-first.
// maxDays limits the number of days analysed.
func ClassifyMigration(bars []OHLCV, maxDays int) *AMTMigrationResult {
	if len(bars) == 0 || maxDays < 2 {
		return nil
	}

	dayGroups := groupByDay(bars)
	if len(dayGroups) < 2 {
		return nil
	}

	if len(dayGroups) > maxDays {
		dayGroups = dayGroups[len(dayGroups)-maxDays:]
	}

	// Compute Value Areas for all days.
	vas := make([]ValueArea, len(dayGroups))
	for i, dg := range dayGroups {
		vas[i] = computeValueArea(dg.date, dg.bars)
	}

	result := &AMTMigrationResult{}

	// Build migration data for each day (starting from day 2).
	for i := 0; i < len(dayGroups); i++ {
		dm := DayMigration{
			Date: dayGroups[i].date,
			VA:   vas[i],
		}

		if i > 0 {
			prevVA := vas[i-1]
			dm.POCShift = vas[i].POC - prevVA.POC
			dm.VAOverlap = computeVAOverlap(prevVA, vas[i])
			dm.Direction = classifyMigrationDir(dm.POCShift, dm.VAOverlap, vas[i], prevVA)
			dm.MGILevels = analyseMGI(dayGroups[i].bars, prevVA)
		}

		result.Days = append(result.Days, dm)
	}

	if len(result.Days) == 0 {
		return nil
	}

	// Composite Value Areas.
	result.WeeklyVA = computeCompositeVA(dayGroups, vas, "WEEKLY", 5)
	result.MonthlyVA = computeCompositeVA(dayGroups, vas, "MONTHLY", 20)

	// Migration score and summary.
	result.MigrationScore = computeMigrationScore(result.Days)
	result.NetDirection = netMigrationDirection(result.MigrationScore)
	result.MigrationChart = buildMigrationChart(result.Days)
	result.Summary = buildMigrationSummary(result)

	return result
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// computeVAOverlap returns the fraction of overlap between two Value Areas (0–1).
func computeVAOverlap(a, b ValueArea) float64 {
	overlapHigh := math.Min(a.VAH, b.VAH)
	overlapLow := math.Max(a.VAL, b.VAL)

	if overlapHigh <= overlapLow {
		return 0 // no overlap
	}

	overlapWidth := overlapHigh - overlapLow
	// Use the larger VA as denominator.
	widthA := a.VAH - a.VAL
	widthB := b.VAH - b.VAL
	maxWidth := math.Max(widthA, widthB)

	if maxWidth == 0 {
		return 0
	}
	ratio := overlapWidth / maxWidth
	if ratio > 1 {
		ratio = 1
	}
	return ratio
}

// classifyMigrationDir determines the direction of value migration.
func classifyMigrationDir(pocShift, vaOverlap float64, today, prev ValueArea) MigrationDirection {
	vaWidth := math.Max(today.VAH-today.VAL, prev.VAH-prev.VAL)
	if vaWidth == 0 {
		return MigrationBalanced
	}

	// Normalise POC shift as fraction of VA width.
	normShift := pocShift / vaWidth

	switch {
	case vaOverlap > 0.80 && math.Abs(normShift) < 0.15:
		return MigrationBalanced
	case normShift > 0.15:
		return MigrationUp
	case normShift < -0.15:
		return MigrationDown
	default:
		return MigrationOverlap
	}
}

// analyseMGI checks acceptance/rejection at yesterday's key levels.
func analyseMGI(todayBars []OHLCV, prevVA ValueArea) []MGILevel {
	if len(todayBars) == 0 {
		return nil
	}

	levels := []MGILevel{
		{Price: prevVA.POC, LevelType: "POC"},
		{Price: prevVA.VAH, LevelType: "VAH"},
		{Price: prevVA.VAL, LevelType: "VAL"},
	}

	vaWidth := prevVA.VAH - prevVA.VAL
	threshold := vaWidth * 0.05 // 5% of VA width
	if threshold < 0.0001 {
		threshold = 0.0001
	}

	for i, lvl := range levels {
		touchBars := 0
		totalBars := len(todayBars)

		for _, b := range todayBars {
			barMid := (b.High + b.Low) / 2.0
			if math.Abs(barMid-lvl.Price) <= threshold {
				touchBars++
			}
		}

		fraction := float64(touchBars) / float64(totalBars)

		// Acceptance = price spent significant time near the level (>15% of bars).
		// Rejection = price touched but moved away quickly (<5% of bars touched, but did approach).
		if fraction >= 0.15 {
			levels[i].Accepted = true
			levels[i].Description = fmt.Sprintf("Accepted at %s (%.5f) — %.0f%% of bars near level",
				lvl.LevelType, lvl.Price, fraction*100)
		} else if touchBars > 0 {
			levels[i].Accepted = false
			levels[i].Description = fmt.Sprintf("Rejected at %s (%.5f) — only %.0f%% contact",
				lvl.LevelType, lvl.Price, fraction*100)
		} else {
			// Did price gap over this level entirely?
			levels[i].Accepted = false
			levels[i].Description = fmt.Sprintf("No contact with %s (%.5f) — level not tested",
				lvl.LevelType, lvl.Price)
		}
	}

	return levels
}

// computeCompositeVA builds a composite Value Area from the last N days of bars.
func computeCompositeVA(dayGroups []dayGroup, vas []ValueArea, period string, maxDays int) *CompositeVA {
	n := len(dayGroups)
	if n == 0 {
		return nil
	}

	start := n - maxDays
	if start < 0 {
		start = 0
	}

	// Merge all bars into one pool.
	var allBars []OHLCV
	for i := start; i < n; i++ {
		allBars = append(allBars, dayGroups[i].bars...)
	}

	if len(allBars) == 0 {
		return nil
	}

	// Find overall range.
	hi, lo := allBars[0].High, allBars[0].Low
	for _, b := range allBars {
		if b.High > hi {
			hi = b.High
		}
		if b.Low < lo {
			lo = b.Low
		}
	}

	// Use computeValueArea on a dummy date to get POC/VAH/VAL.
	va := computeValueArea(time.Time{}, allBars)

	return &CompositeVA{
		Period: period,
		POC:    va.POC,
		VAH:    va.VAH,
		VAL:    va.VAL,
		High:   hi,
		Low:    lo,
	}
}

// computeMigrationScore returns a score from -100 to +100 based on POC drift.
func computeMigrationScore(days []DayMigration) float64 {
	if len(days) < 2 {
		return 0
	}

	// Use the sum of normalised POC shifts.
	upCount, downCount := 0, 0
	for i := 1; i < len(days); i++ {
		if days[i].POCShift > 0 {
			upCount++
		} else if days[i].POCShift < 0 {
			downCount++
		}
	}

	total := upCount + downCount
	if total == 0 {
		return 0
	}

	// Score: (up - down) / total × 100
	return float64(upCount-downCount) / float64(total) * 100
}

func netMigrationDirection(score float64) MigrationDirection {
	switch {
	case score > 30:
		return MigrationUp
	case score < -30:
		return MigrationDown
	case math.Abs(score) <= 15:
		return MigrationBalanced
	default:
		return MigrationOverlap
	}
}

// buildMigrationChart creates a simple ASCII chart showing POC positions across days.
func buildMigrationChart(days []DayMigration) string {
	if len(days) < 2 {
		return ""
	}

	// Find POC range.
	minPOC, maxPOC := days[0].VA.POC, days[0].VA.POC
	for _, d := range days {
		if d.VA.POC < minPOC {
			minPOC = d.VA.POC
		}
		if d.VA.POC > maxPOC {
			maxPOC = d.VA.POC
		}
	}

	pocRange := maxPOC - minPOC
	if pocRange == 0 {
		return "POC stable across all days"
	}

	const chartWidth = 20
	var sb strings.Builder
	sb.WriteString("<code>")

	for _, d := range days {
		dateStr := d.Date.Format("Jan02")
		// Normalise POC position to 0–chartWidth.
		pos := int((d.VA.POC - minPOC) / pocRange * float64(chartWidth))
		if pos > chartWidth {
			pos = chartWidth
		}

		line := strings.Repeat("·", pos) + "◆" + strings.Repeat("·", chartWidth-pos)
		sb.WriteString(fmt.Sprintf("%s │%s│\n", dateStr, line))
	}

	sb.WriteString("</code>")
	sb.WriteString(fmt.Sprintf("     <i>%.5f → %.5f</i>", minPOC, maxPOC))

	return sb.String()
}

// buildMigrationSummary creates a human-readable summary.
func buildMigrationSummary(r *AMTMigrationResult) string {
	if len(r.Days) < 2 {
		return "Insufficient data for migration analysis."
	}

	var parts []string

	// Migration direction.
	switch r.NetDirection {
	case MigrationUp:
		parts = append(parts, "Value migrating HIGHER — institutional buying shifting price upward")
	case MigrationDown:
		parts = append(parts, "Value migrating LOWER — institutional selling pushing price down")
	case MigrationBalanced:
		parts = append(parts, "Value STABLE — market in balance; fade extremes")
	case MigrationOverlap:
		parts = append(parts, "Value areas overlapping — consolidation; watch for breakout direction")
	}

	// MGI from most recent day.
	if len(r.Days) > 0 {
		latest := r.Days[len(r.Days)-1]
		for _, mgi := range latest.MGILevels {
			if mgi.Accepted {
				parts = append(parts, fmt.Sprintf("Price accepted at %s — level validated", mgi.LevelType))
			}
		}
	}

	// Composite VA context.
	if r.WeeklyVA != nil {
		today := r.Days[len(r.Days)-1]
		if today.VA.POC > r.WeeklyVA.VAH {
			parts = append(parts, "Current POC above weekly VA — bullish premium zone")
		} else if today.VA.POC < r.WeeklyVA.VAL {
			parts = append(parts, "Current POC below weekly VA — bearish discount zone")
		}
	}

	return strings.Join(parts, ". ") + "."
}
