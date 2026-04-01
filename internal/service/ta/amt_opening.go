// Package ta — AMT Opening Type Analysis (Dalton's 4 types).
//
// Classifies the current session's opening relative to yesterday's Value Area.
// This is Module 2 of the AMT upgrade plan.
package ta

import (
	"math"
	"sort"
	"time"
)

// OpeningType enumerates Dalton's four opening types.
type OpeningType string

const (
	// OpenDrive — opens outside VA, aggressively moves away. Follow momentum.
	OpenDrive OpeningType = "Open Drive"

	// OpenTestDrive — opens outside VA, tests back to VA edge, then drives away.
	// Confirm direction after the test; high probability trend continuation.
	OpenTestDrive OpeningType = "Open Test Drive"

	// OpenRejectionReverse — opens outside VA, fails to extend, reverses into VA.
	// Fade the break; responsive activity wins.
	OpenRejectionReverse OpeningType = "Open Rejection Reverse"

	// OpenAuction — opens inside VA, auctions within the previous day's range.
	// Wait for a breakout; direction uncertain.
	OpenAuction OpeningType = "Open Auction"

	// OpenUnknown — insufficient data to classify.
	OpenUnknown OpeningType = "Unknown"
)

// ValueArea holds the key Market Profile levels for a single trading day.
type ValueArea struct {
	Date time.Time

	POC float64 // Point of Control (highest-volume price)
	VAH float64 // Value Area High (upper bound of 70% volume)
	VAL float64 // Value Area Low (lower bound of 70% volume)

	DayHigh float64
	DayLow  float64
}

// OpeningClassification holds the full opening type analysis.
type OpeningClassification struct {
	Date          time.Time   // date being analysed
	OpenPrice     float64     // first bar's open
	OpenLocation  string      // "ABOVE_VA" | "BELOW_VA" | "INSIDE_VA"
	YesterdayVA   ValueArea   // reference Value Area
	Type          OpeningType // classified opening type
	Implication   string      // trading implication text
	Confidence    string      // "HIGH" | "MEDIUM" | "LOW"

	// First-period direction tracking (for OTD/ORR detection)
	FirstPeriodHigh  float64
	FirstPeriodLow   float64
	FirstPeriodClose float64
}

// AMTOpeningResult contains today's opening analysis plus historical win rates.
type AMTOpeningResult struct {
	Today   OpeningClassification
	History []OpeningClassification // historical opening classifications (oldest-first)

	// Win rates per opening type (fraction of times the implied direction was correct)
	WinRates map[OpeningType]float64
}

// ClassifyOpening analyses the opening type for the most recent trading day.
// bars must be newest-first intraday bars (30m recommended).
// ibPeriods is the number of opening periods to use for first-session tracking.
// historyDays is the number of past days to include for win-rate calculation.
//
// Returns nil if there are insufficient bars to compute a Value Area.
func ClassifyOpening(bars []OHLCV, ibPeriods int, historyDays int) *AMTOpeningResult {
	if len(bars) == 0 || ibPeriods < 1 || historyDays < 1 {
		return nil
	}

	// Group bars into trading days (oldest-first).
	dayGroups := groupByDay(bars)
	if len(dayGroups) < 2 {
		// Need at least today + yesterday for VA computation.
		return nil
	}

	// Compute Value Areas for all days except the last (today).
	vaHistory := make([]ValueArea, 0, len(dayGroups)-1)
	for _, dg := range dayGroups[:len(dayGroups)-1] {
		va := computeValueArea(dg.date, dg.bars)
		vaHistory = append(vaHistory, va)
	}

	// Today's bars (oldest-first within the day).
	todayGroup := dayGroups[len(dayGroups)-1]
	yesterdayVA := vaHistory[len(vaHistory)-1]

	// Classify today.
	today := classifyOpening(todayGroup.date, todayGroup.bars, yesterdayVA, ibPeriods)

	// Build history of opening classifications (up to historyDays).
	start := len(dayGroups) - 1 - historyDays
	if start < 1 {
		start = 1
	}
	var history []OpeningClassification
	for i := start; i < len(dayGroups)-1; i++ {
		prevVA := vaHistory[i-1]
		oc := classifyOpening(dayGroups[i].date, dayGroups[i].bars, prevVA, ibPeriods)
		history = append(history, oc)
	}

	winRates := computeWinRates(history, dayGroups, start)

	return &AMTOpeningResult{
		Today:    today,
		History:  history,
		WinRates: winRates,
	}
}

// ---------------------------------------------------------------------------
// Value Area computation
// ---------------------------------------------------------------------------

const vaPercent = 0.70 // Standard 70% Value Area
const histogramBuckets = 50

// computeValueArea computes POC, VAH, VAL from a day's bars using a price histogram.
func computeValueArea(date time.Time, bars []OHLCV) ValueArea {
	va := ValueArea{Date: date}
	if len(bars) == 0 {
		return va
	}

	// Find day range.
	hi, lo := bars[0].High, bars[0].Low
	for _, b := range bars {
		if b.High > hi {
			hi = b.High
		}
		if b.Low < lo {
			lo = b.Low
		}
	}
	va.DayHigh = hi
	va.DayLow = lo

	rangeSize := hi - lo
	if rangeSize == 0 {
		va.POC = (hi + lo) / 2
		va.VAH = hi
		va.VAL = lo
		return va
	}

	// Build histogram: split range into N buckets, accumulate volume per bucket.
	bucketSize := rangeSize / float64(histogramBuckets)
	vol := make([]float64, histogramBuckets)
	for _, b := range bars {
		barMid := (b.High + b.Low) / 2.0
		idx := int((barMid - lo) / bucketSize)
		if idx >= histogramBuckets {
			idx = histogramBuckets - 1
		}
		if idx < 0 {
			idx = 0
		}
		vol[idx] += b.Volume
	}

	// Find POC bucket (highest volume).
	pocIdx := 0
	for i, v := range vol {
		if v > vol[pocIdx] {
			pocIdx = i
		}
	}
	va.POC = lo + (float64(pocIdx)+0.5)*bucketSize

	// Expand from POC outward until 70% of volume is captured.
	totalVol := 0.0
	for _, v := range vol {
		totalVol += v
	}
	target := totalVol * vaPercent

	accumulated := vol[pocIdx]
	low := pocIdx
	high := pocIdx

	for accumulated < target && (low > 0 || high < histogramBuckets-1) {
		addUp := 0.0
		if high+1 < histogramBuckets {
			addUp = vol[high+1]
		}
		addDown := 0.0
		if low-1 >= 0 {
			addDown = vol[low-1]
		}

		if addUp >= addDown && high+1 < histogramBuckets {
			high++
			accumulated += vol[high]
		} else if low-1 >= 0 {
			low--
			accumulated += vol[low]
		} else if high+1 < histogramBuckets {
			high++
			accumulated += vol[high]
		} else {
			break
		}
	}

	va.VAH = lo + float64(high+1)*bucketSize
	va.VAL = lo + float64(low)*bucketSize

	return va
}

// ---------------------------------------------------------------------------
// Opening type classification
// ---------------------------------------------------------------------------

func classifyOpening(date time.Time, bars []OHLCV, prevVA ValueArea, ibPeriods int) OpeningClassification {
	oc := OpeningClassification{
		Date:        date,
		YesterdayVA: prevVA,
	}

	if len(bars) == 0 {
		oc.Type = OpenUnknown
		oc.Implication = "No data available"
		return oc
	}

	oc.OpenPrice = bars[0].Open

	// Determine open location.
	switch {
	case oc.OpenPrice > prevVA.VAH:
		oc.OpenLocation = "ABOVE_VA"
	case oc.OpenPrice < prevVA.VAL:
		oc.OpenLocation = "BELOW_VA"
	default:
		oc.OpenLocation = "INSIDE_VA"
	}

	// If opening inside VA → Open Auction.
	if oc.OpenLocation == "INSIDE_VA" {
		oc.Type = OpenAuction
		oc.Implication = "Wait for VA breakout; no directional conviction at open"
		oc.Confidence = "MEDIUM"
		oc.FirstPeriodHigh = firstPeriodHigh(bars, ibPeriods)
		oc.FirstPeriodLow = firstPeriodLow(bars, ibPeriods)
		oc.FirstPeriodClose = firstPeriodClose(bars, ibPeriods)
		return oc
	}

	// Opening outside VA — need first-period tracking to distinguish OD/OTD/ORR.
	if len(bars) < ibPeriods {
		oc.Type = OpenUnknown
		oc.Implication = "Insufficient bars to classify opening"
		oc.Confidence = "LOW"
		return oc
	}

	ibHigh := firstPeriodHigh(bars, ibPeriods)
	ibLow := firstPeriodLow(bars, ibPeriods)
	ibClose := firstPeriodClose(bars, ibPeriods)
	oc.FirstPeriodHigh = ibHigh
	oc.FirstPeriodLow = ibLow
	oc.FirstPeriodClose = ibClose

	aboveVA := oc.OpenLocation == "ABOVE_VA"

	if aboveVA {
		// Price opened above VAH.
		testedVAH := ibLow <= prevVA.VAH // did price pull back to test VAH?
		closedAbove := ibClose > prevVA.VAH
		movedAway := ibHigh > oc.OpenPrice*1.0002 // extended further up (>2 pips for FX)

		switch {
		case !testedVAH && movedAway:
			// Never tested back, kept driving up.
			oc.Type = OpenDrive
			oc.Implication = "Strong bullish momentum; follow the drive upward"
			oc.Confidence = "HIGH"
		case testedVAH && closedAbove:
			// Tested VAH, held above, then drove higher.
			oc.Type = OpenTestDrive
			oc.Implication = "Bullish continuation after VAH test; buy dips to VAH"
			oc.Confidence = "HIGH"
		case testedVAH && !closedAbove:
			// Tested VAH and fell back inside VA → rejected.
			oc.Type = OpenRejectionReverse
			oc.Implication = "Bearish reversal; fade move back into VA and toward POC"
			oc.Confidence = "MEDIUM"
		default:
			oc.Type = OpenDrive
			oc.Implication = "Bullish open drive; follow momentum"
			oc.Confidence = "MEDIUM"
		}
	} else {
		// Price opened below VAL.
		testedVAL := ibHigh >= prevVA.VAL
		closedBelow := ibClose < prevVA.VAL
		movedAway := ibLow < oc.OpenPrice*0.9998 // extended further down

		switch {
		case !testedVAL && movedAway:
			oc.Type = OpenDrive
			oc.Implication = "Strong bearish momentum; follow the drive downward"
			oc.Confidence = "HIGH"
		case testedVAL && closedBelow:
			oc.Type = OpenTestDrive
			oc.Implication = "Bearish continuation after VAL test; sell rallies to VAL"
			oc.Confidence = "HIGH"
		case testedVAL && !closedBelow:
			oc.Type = OpenRejectionReverse
			oc.Implication = "Bullish reversal; fade move back into VA and toward POC"
			oc.Confidence = "MEDIUM"
		default:
			oc.Type = OpenDrive
			oc.Implication = "Bearish open drive; follow momentum"
			oc.Confidence = "MEDIUM"
		}
	}

	return oc
}

// ---------------------------------------------------------------------------
// Win rate calculation
// ---------------------------------------------------------------------------

// computeWinRates calculates the empirical win rate per opening type.
// A "win" is defined as: Open Drive → closed in drive direction; OTD → same;
// ORR → closed inside VA; OA → price broke out of VA in one direction.
func computeWinRates(history []OpeningClassification, dayGroups []dayGroup, start int) map[OpeningType]float64 {
	type counts struct{ wins, total int }
	statsMap := make(map[OpeningType]*counts)
	for _, ot := range []OpeningType{OpenDrive, OpenTestDrive, OpenRejectionReverse, OpenAuction} {
		statsMap[ot] = &counts{}
	}

	for i, oc := range history {
		if oc.Type == OpenUnknown {
			continue
		}
		c := statsMap[oc.Type]
		if c == nil {
			continue
		}
		c.total++

		// Get the day's closing price from dayGroups.
		// history[i] corresponds to dayGroups[start+i].
		dgIdx := start + i
		if dgIdx < 0 || dgIdx >= len(dayGroups) {
			continue
		}
		dg := dayGroups[dgIdx]
		if len(dg.bars) == 0 {
			continue
		}
		dayClose := dg.bars[len(dg.bars)-1].Close

		win := false
		switch oc.Type {
		case OpenDrive:
			if oc.OpenLocation == "ABOVE_VA" {
				win = dayClose > oc.OpenPrice
			} else {
				win = dayClose < oc.OpenPrice
			}
		case OpenTestDrive:
			if oc.OpenLocation == "ABOVE_VA" {
				win = dayClose > oc.YesterdayVA.VAH
			} else {
				win = dayClose < oc.YesterdayVA.VAL
			}
		case OpenRejectionReverse:
			win = dayClose >= oc.YesterdayVA.VAL && dayClose <= oc.YesterdayVA.VAH
		case OpenAuction:
			win = math.Abs(dayClose-oc.YesterdayVA.POC) < (oc.YesterdayVA.VAH-oc.YesterdayVA.VAL)*0.2
		}
		if win {
			c.wins++
		}
	}

	result := make(map[OpeningType]float64, 4)
	for ot, c := range statsMap {
		if c.total > 0 {
			result[ot] = float64(c.wins) / float64(c.total)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// IB helpers (reuse day's bars, oldest-first assumed)
// ---------------------------------------------------------------------------

func firstPeriodHigh(bars []OHLCV, n int) float64 {
	if n > len(bars) {
		n = len(bars)
	}
	h := bars[0].High
	for i := 1; i < n; i++ {
		if bars[i].High > h {
			h = bars[i].High
		}
	}
	return h
}

func firstPeriodLow(bars []OHLCV, n int) float64 {
	if n > len(bars) {
		n = len(bars)
	}
	l := bars[0].Low
	for i := 1; i < n; i++ {
		if bars[i].Low < l {
			l = bars[i].Low
		}
	}
	return l
}

func firstPeriodClose(bars []OHLCV, n int) float64 {
	if n > len(bars) {
		n = len(bars)
	}
	return bars[n-1].Close
}

// ---------------------------------------------------------------------------
// SortValueAreasByDate sorts a slice of ValueArea objects by date ascending.
// Exposed for callers that need sorted VA history.
// ---------------------------------------------------------------------------

func SortValueAreasByDate(vas []ValueArea) {
	sort.Slice(vas, func(i, j int) bool { return vas[i].Date.Before(vas[j].Date) })
}
