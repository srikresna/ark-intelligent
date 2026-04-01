package ta

import (
	"testing"
	"time"
)

// buildDayBars creates a set of 30m bars for a trading day.
// dayDate is "2006-01-02", bars are times and OHLCV values.
func buildDayBars(dayDate string, priceSeqs [][5]float64) []OHLCV {
	var out []OHLCV
	baseTime, _ := time.Parse("2006-01-02", dayDate)
	for i, p := range priceSeqs {
		t := baseTime.Add(time.Duration(i+1) * 30 * time.Minute)
		out = append(out, OHLCV{
			Date:   t,
			Open:   p[0],
			High:   p[1],
			Low:    p[2],
			Close:  p[3],
			Volume: p[4],
		})
	}
	return out
}

// reverseOHLCVSlice reverses a slice in-place.
func reverseOHLCVSlice(bars []OHLCV) {
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}
}

// TestComputeValueArea verifies basic VA computation.
func TestComputeValueArea(t *testing.T) {
	// Build 6 bars with known volume distribution.
	// High volume in the 1.0840–1.0860 range → POC should be there.
	priceSeqs := [][5]float64{
		{1.0810, 1.0830, 1.0800, 1.0820, 50},
		{1.0820, 1.0845, 1.0818, 1.0840, 80},
		{1.0840, 1.0860, 1.0838, 1.0855, 300}, // heavy vol
		{1.0855, 1.0865, 1.0850, 1.0860, 280}, // heavy vol
		{1.0860, 1.0870, 1.0855, 1.0862, 100},
		{1.0862, 1.0868, 1.0855, 1.0860, 90},
	}
	bars := buildDayBars("2026-03-30", priceSeqs)
	date, _ := time.Parse("2006-01-02", "2026-03-30")
	va := computeValueArea(date, bars)

	if va.POC == 0 {
		t.Error("POC should not be zero")
	}
	if va.VAH <= va.VAL {
		t.Errorf("VAH %.5f should be > VAL %.5f", va.VAH, va.VAL)
	}
	if va.POC < va.VAL || va.POC > va.VAH {
		t.Errorf("POC %.5f should be within VAL %.5f – VAH %.5f", va.POC, va.VAL, va.VAH)
	}
	// VA should be within day range.
	if va.VAH > va.DayHigh || va.VAL < va.DayLow {
		t.Errorf("VA [%.5f,%.5f] outside day range [%.5f,%.5f]", va.VAL, va.VAH, va.DayLow, va.DayHigh)
	}
}

// TestClassifyOpening_OpenAuction verifies Open Auction classification.
func TestClassifyOpening_OpenAuction(t *testing.T) {
	// Yesterday VA: VAL=1.0820, VAH=1.0860, POC=1.0840
	// Today opens inside VA at 1.0838.
	prevVA := ValueArea{
		VAL: 1.0820,
		VAH: 1.0860,
		POC: 1.0840,
	}
	priceSeqs := [][5]float64{
		{1.0838, 1.0850, 1.0830, 1.0845, 100},
		{1.0845, 1.0855, 1.0840, 1.0848, 80},
	}
	todayBars := buildDayBars("2026-03-31", priceSeqs)
	date, _ := time.Parse("2006-01-02", "2026-03-31")

	oc := classifyOpening(date, todayBars, prevVA, 2)
	if oc.Type != OpenAuction {
		t.Errorf("expected OpenAuction, got %s (OpenLocation=%s)", oc.Type, oc.OpenLocation)
	}
}

// TestClassifyOpening_OpenDriveUp verifies Open Drive (upward) classification.
func TestClassifyOpening_OpenDriveUp(t *testing.T) {
	// Yesterday VA: VAL=1.0820, VAH=1.0860.
	// Today opens above VAH at 1.0870, never tests back, keeps driving up.
	prevVA := ValueArea{
		VAL: 1.0820,
		VAH: 1.0860,
		POC: 1.0840,
	}
	priceSeqs := [][5]float64{
		{1.0870, 1.0890, 1.0868, 1.0888, 200}, // opens above VAH, drives up
		{1.0888, 1.0910, 1.0885, 1.0905, 250},
	}
	todayBars := buildDayBars("2026-03-31", priceSeqs)
	date, _ := time.Parse("2006-01-02", "2026-03-31")

	oc := classifyOpening(date, todayBars, prevVA, 2)
	if oc.Type != OpenDrive {
		t.Errorf("expected OpenDrive, got %s", oc.Type)
	}
	if oc.OpenLocation != "ABOVE_VA" {
		t.Errorf("expected ABOVE_VA, got %s", oc.OpenLocation)
	}
}

// TestClassifyOpening_ORR verifies Open Rejection Reverse classification.
func TestClassifyOpening_ORR(t *testing.T) {
	// Yesterday VA: VAL=1.0820, VAH=1.0860.
	// Today opens above VAH, tests back to VAH, then closes inside VA.
	prevVA := ValueArea{
		VAL: 1.0820,
		VAH: 1.0860,
		POC: 1.0840,
	}
	priceSeqs := [][5]float64{
		{1.0875, 1.0880, 1.0858, 1.0862, 150}, // opens above VAH, dips to VAH level
		{1.0862, 1.0863, 1.0840, 1.0845, 180}, // closes inside VA
	}
	todayBars := buildDayBars("2026-03-31", priceSeqs)
	date, _ := time.Parse("2006-01-02", "2026-03-31")

	oc := classifyOpening(date, todayBars, prevVA, 2)
	if oc.Type != OpenRejectionReverse {
		t.Errorf("expected OpenRejectionReverse, got %s (close=%.5f, VAH=%.5f)", oc.Type, oc.FirstPeriodClose, prevVA.VAH)
	}
}

// TestClassifyOpening_OTD verifies Open Test Drive classification.
func TestClassifyOpening_OTD(t *testing.T) {
	// Yesterday VA: VAL=1.0820, VAH=1.0860.
	// Today opens above VAH, dips back to VAH level but closes above VAH → OTD.
	prevVA := ValueArea{
		VAL: 1.0820,
		VAH: 1.0860,
		POC: 1.0840,
	}
	priceSeqs := [][5]float64{
		{1.0875, 1.0878, 1.0860, 1.0872, 150}, // opens above VAH, tests to VAH, holds
		{1.0872, 1.0895, 1.0870, 1.0892, 200}, // drives away
	}
	todayBars := buildDayBars("2026-03-31", priceSeqs)
	date, _ := time.Parse("2006-01-02", "2026-03-31")

	oc := classifyOpening(date, todayBars, prevVA, 2)
	if oc.Type != OpenTestDrive {
		t.Errorf("expected OpenTestDrive, got %s (testedVAH=%v, closedAbove=%v)",
			oc.Type,
			oc.FirstPeriodLow <= prevVA.VAH,
			oc.FirstPeriodClose > prevVA.VAH,
		)
	}
}

// TestClassifyOpening_NilOnSingleDay verifies nil return when only one day available.
func TestClassifyOpening_NilOnSingleDay(t *testing.T) {
	priceSeqs := [][5]float64{
		{1.0810, 1.0830, 1.0800, 1.0820, 50},
		{1.0820, 1.0840, 1.0818, 1.0835, 60},
	}
	bars := buildDayBars("2026-03-31", priceSeqs)
	reverseOHLCVSlice(bars) // newest-first

	result := ClassifyOpening(bars, 2, 5)
	if result != nil {
		t.Error("expected nil when only one day of data available")
	}
}
