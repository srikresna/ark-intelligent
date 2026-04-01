// Package ta — AMT Day Type Classification (Dalton's 6 Types).
//
// Classifies each trading day into one of six Market Profile day types
// based on the relationship between the Initial Balance (IB) and the
// full day range. Requires intraday bars at 30m or finer resolution.
package ta

import (
	"math"
	"sort"
	"time"
)

// DayType enumerates the six Dalton Market Profile day types.
type DayType string

const (
	// DayTypeNormal — IB constitutes ≥85% of the day range.
	// Tight value area; market is balanced. Fade extreme moves.
	DayTypeNormal DayType = "Normal"

	// DayTypeNormalVariation — IB is 70–85% of the day range.
	// Modest single-sided extension; moderate directional lean.
	DayTypeNormalVariation DayType = "Normal Variation"

	// DayTypeTrend — IB is <50% of range; one-directional extension.
	// Open and close at opposite ends. Strong conviction day.
	DayTypeTrend DayType = "Trend"

	// DayTypeDoubleDistribution — two separate volume clusters visible.
	// Starts balanced, then an event causes value migration to a new level.
	DayTypeDoubleDistribution DayType = "Double Distribution"

	// DayTypePShape — volume concentrated in the upper half with a long lower tail.
	// Indicates buying / short-covering rally from below.
	DayTypePShape DayType = "P-shape"

	// DayTypeBShape — volume concentrated in the lower half with a long upper tail.
	// Indicates selling / long liquidation from above.
	DayTypeBShape DayType = "b-shape"
)

// DayClassification holds the full analysis for a single trading day.
type DayClassification struct {
	Date time.Time // UTC date of the trading day

	// Range metrics
	DayHigh   float64
	DayLow    float64
	DayRange  float64 // DayHigh - DayLow
	IBHigh    float64 // Initial Balance high (first-hour bars)
	IBLow     float64 // Initial Balance low
	IBRange   float64 // IBHigh - IBLow
	IBPercent float64 // IBRange / DayRange × 100

	// Extension (how much range was added beyond IB)
	ExtensionUp   float64 // bars above IBHigh
	ExtensionDown float64 // bars below IBLow
	NetExtension  float64 // positive = up bias, negative = down bias
	ExtensionRatio float64 // (ExtensionUp+ExtensionDown) / IBRange

	// Profile shape
	UpperVolumeRatio float64 // fraction of volume in upper half of range
	LowerVolumeRatio float64 // fraction of volume in lower half of range

	// Open / Close
	Open  float64
	Close float64

	// Classification
	Type        DayType
	Description string // human-readable explanation
}

// AMTDayTypeResult holds classifications for all analysed days.
type AMTDayTypeResult struct {
	Days []DayClassification

	// Pattern detection across days
	ConsecutiveTrendDays int    // number of consecutive trend days at head
	ValueMigration       string // "HIGHER" | "LOWER" | "STABLE" | "MIXED"
	Bias                 string // "BULLISH" | "BEARISH" | "NEUTRAL"
}

// ClassifyDayTypes analyses the last N trading days from a slice of intraday bars
// and returns an AMTDayTypeResult. bars must be ordered newest-first (standard ta package
// convention). ibPeriods is the number of bar periods that constitute the Initial
// Balance (e.g. 2 for the first hour when using 30-minute bars).
//
// Returns nil if there are fewer than ibPeriods+1 bars.
func ClassifyDayTypes(bars []OHLCV, ibPeriods int, maxDays int) *AMTDayTypeResult {
	if len(bars) == 0 || ibPeriods < 1 || maxDays < 1 {
		return nil
	}

	// Group bars into trading days.
	dayGroups := groupByDay(bars)
	if len(dayGroups) == 0 {
		return nil
	}

	// Take the last maxDays (newest) days from the sorted list.
	// dayGroups is sorted oldest-first after groupByDay.
	if len(dayGroups) > maxDays {
		dayGroups = dayGroups[len(dayGroups)-maxDays:]
	}

	result := &AMTDayTypeResult{}
	for _, dg := range dayGroups {
		if len(dg.bars) < ibPeriods+1 {
			continue
		}
		dc := classifyDay(dg.date, dg.bars, ibPeriods)
		result.Days = append(result.Days, dc)
	}

	if len(result.Days) == 0 {
		return nil
	}

	result.ConsecutiveTrendDays = countConsecutiveTrend(result.Days)
	result.ValueMigration = detectValueMigration(result.Days)
	result.Bias = detectBias(result.Days)

	return result
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

type dayGroup struct {
	date time.Time
	bars []OHLCV // oldest-first within day
}

// groupByDay splits newest-first bars into per-day groups (oldest-first within group).
// Returned slice is sorted oldest-day-first.
func groupByDay(bars []OHLCV) []dayGroup {
	dayMap := make(map[string][]OHLCV)
	for _, b := range bars {
		key := b.Date.UTC().Format("2006-01-02")
		dayMap[key] = append(dayMap[key], b)
	}

	var groups []dayGroup
	for key, bs := range dayMap {
		date, _ := time.Parse("2006-01-02", key)
		// Sort bars within day oldest-first (ascending).
		sort.Slice(bs, func(i, j int) bool { return bs[i].Date.Before(bs[j].Date) })
		groups = append(groups, dayGroup{date: date, bars: bs})
	}

	// Sort groups oldest-first.
	sort.Slice(groups, func(i, j int) bool { return groups[i].date.Before(groups[j].date) })
	return groups
}

func classifyDay(date time.Time, bars []OHLCV, ibPeriods int) DayClassification {
	dc := DayClassification{Date: date}

	// Compute day H/L/O/C.
	dc.Open = bars[0].Open
	dc.Close = bars[len(bars)-1].Close
	dc.DayHigh = bars[0].High
	dc.DayLow = bars[0].Low
	for _, b := range bars {
		if b.High > dc.DayHigh {
			dc.DayHigh = b.High
		}
		if b.Low < dc.DayLow {
			dc.DayLow = b.Low
		}
	}
	dc.DayRange = dc.DayHigh - dc.DayLow

	// Initial Balance from first ibPeriods bars.
	ibBars := bars[:ibPeriods]
	dc.IBHigh = ibBars[0].High
	dc.IBLow = ibBars[0].Low
	for _, b := range ibBars {
		if b.High > dc.IBHigh {
			dc.IBHigh = b.High
		}
		if b.Low < dc.IBLow {
			dc.IBLow = b.Low
		}
	}
	dc.IBRange = dc.IBHigh - dc.IBLow

	if dc.DayRange == 0 {
		dc.Type = DayTypeNormal
		dc.Description = "Zero-range day (holiday / no data)"
		return dc
	}

	dc.IBPercent = (dc.IBRange / dc.DayRange) * 100.0

	// Extension beyond IB.
	extBars := bars[ibPeriods:]
	for _, b := range extBars {
		if b.High > dc.IBHigh {
			dc.ExtensionUp += b.High - dc.IBHigh
		}
		if b.Low < dc.IBLow {
			dc.ExtensionDown += dc.IBLow - b.Low
		}
	}
	dc.NetExtension = dc.ExtensionUp - dc.ExtensionDown
	if dc.IBRange > 0 {
		dc.ExtensionRatio = (dc.ExtensionUp + dc.ExtensionDown) / dc.IBRange
	}

	// Volume shape: split day into upper and lower half by range midpoint.
	mid := (dc.DayHigh + dc.DayLow) / 2.0
	var totalVol, upperVol, lowerVol float64
	for _, b := range bars {
		v := b.Volume
		barMid := (b.High + b.Low) / 2.0
		totalVol += v
		if barMid >= mid {
			upperVol += v
		} else {
			lowerVol += v
		}
	}
	if totalVol > 0 {
		dc.UpperVolumeRatio = upperVol / totalVol
		dc.LowerVolumeRatio = lowerVol / totalVol
	}

	// Check double distribution (two separate clusters separated by a gap/thin area).
	isDD := detectDoubleDistribution(bars, dc.DayHigh, dc.DayLow)

	// Classification logic.
	dc.Type, dc.Description = classify(dc, isDD)
	return dc
}

func classify(dc DayClassification, isDD bool) (DayType, string) {
	// Double Distribution check first (overrides shape-based classification).
	if isDD {
		return DayTypeDoubleDistribution, "Two separate value clusters detected — event-driven migration"
	}

	// P-shape / b-shape based on volume distribution with long tail.
	tailThreshold := 0.30 // tail must be > 30% of range
	if dc.DayRange > 0 {
		upperTailRatio := (dc.DayHigh - dc.IBHigh) / dc.DayRange
		lowerTailRatio := (dc.IBLow - dc.DayLow) / dc.DayRange

		// P-shape: high volume upper, long lower tail (buying from below).
		if dc.UpperVolumeRatio >= 0.60 && lowerTailRatio >= tailThreshold {
			return DayTypePShape, "Heavy volume in upper half; long lower tail — rally/buying"
		}
		// b-shape: high volume lower, long upper tail (selling from above).
		if dc.LowerVolumeRatio >= 0.60 && upperTailRatio >= tailThreshold {
			return DayTypeBShape, "Heavy volume in lower half; long upper tail — sell-off/liquidation"
		}
	}

	// IB-based classification.
	switch {
	case dc.IBPercent >= 85:
		return DayTypeNormal, "IB ≥85% of range — balanced; fade extremes"
	case dc.IBPercent >= 70:
		return DayTypeNormalVariation, "IB 70–85% of range — modest extension; moderate directional lean"
	case dc.IBPercent < 50:
		return DayTypeTrend, "IB <50% of range — strong one-directional extension; follow momentum"
	default:
		// 50–70% IB range is transitional.
		if math.Abs(dc.NetExtension) > dc.IBRange*0.3 {
			return DayTypeNormalVariation, "IB 50–70% with clear directional extension — normal variation"
		}
		return DayTypeNormal, "IB 50–70% with balanced extension — near-normal day"
	}
}

// detectDoubleDistribution looks for a thin/sparse volume zone between two clusters.
// Uses a simplified approach: checks if there is a price band in the middle 20% of range
// with significantly less volume than both the upper and lower thirds.
func detectDoubleDistribution(bars []OHLCV, high, low float64) bool {
	rangeSize := high - low
	if rangeSize == 0 || len(bars) < 6 {
		return false
	}

	thirdSize := rangeSize / 3.0
	upper := high - thirdSize
	lower := low + thirdSize

	var upperVol, midVol, lowerVol float64
	for _, b := range bars {
		mid := (b.High + b.Low) / 2.0
		if mid >= upper {
			upperVol += b.Volume
		} else if mid <= lower {
			lowerVol += b.Volume
		} else {
			midVol += b.Volume
		}
	}

	minCluster := math.Min(upperVol, lowerVol)
	if minCluster == 0 {
		return false
	}

	// Both clusters must be significant and the middle must be thin.
	return upperVol > 0 && lowerVol > 0 &&
		midVol/minCluster < 0.25 && // middle is < 25% of the smaller cluster
		upperVol > midVol*2 &&
		lowerVol > midVol*2
}

// countConsecutiveTrend counts consecutive Trend days at the end (most recent) of the slice.
func countConsecutiveTrend(days []DayClassification) int {
	count := 0
	for i := len(days) - 1; i >= 0; i-- {
		if days[i].Type == DayTypeTrend {
			count++
		} else {
			break
		}
	}
	return count
}

// detectValueMigration compares close prices across days to determine drift direction.
func detectValueMigration(days []DayClassification) string {
	if len(days) < 2 {
		return "STABLE"
	}
	up, down := 0, 0
	for i := 1; i < len(days); i++ {
		diff := days[i].Close - days[i-1].Close
		if diff > 0 {
			up++
		} else if diff < 0 {
			down++
		}
	}
	total := up + down
	if total == 0 {
		return "STABLE"
	}
	switch {
	case float64(up)/float64(total) >= 0.70:
		return "HIGHER"
	case float64(down)/float64(total) >= 0.70:
		return "LOWER"
	case up > down:
		return "HIGHER"
	case down > up:
		return "LOWER"
	default:
		return "MIXED"
	}
}

// detectBias computes an overall directional bias from day types and migration.
func detectBias(days []DayClassification) string {
	if len(days) == 0 {
		return "NEUTRAL"
	}
	bullScore, bearScore := 0, 0
	for _, d := range days {
		switch d.Type {
		case DayTypeTrend:
			if d.NetExtension > 0 {
				bullScore++
			} else {
				bearScore++
			}
		case DayTypePShape:
			bullScore++
		case DayTypeBShape:
			bearScore++
		case DayTypeNormalVariation:
			if d.NetExtension > 0 {
				bullScore++
			} else {
				bearScore++
			}
		}
	}
	switch {
	case bullScore > bearScore*2:
		return "BULLISH"
	case bearScore > bullScore*2:
		return "BEARISH"
	default:
		return "NEUTRAL"
	}
}
