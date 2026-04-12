package ta

import (
	"testing"
	"time"
)

func TestClassifyClose_Basic(t *testing.T) {
	// Create two days of bars so we can compute VA for day 1 and close for day 2.
	now := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)

	var bars []OHLCV

	// Yesterday: range from 1.1000 to 1.1100, closing at 1.1050.
	for i := 0; i < 16; i++ {
		t := yesterday.Add(time.Duration(i) * 30 * time.Minute)
		bars = append(bars, OHLCV{
			Date: t, Open: 1.1000 + float64(i)*0.00005,
			High: 1.1100, Low: 1.1000,
			Close: 1.1050, Volume: 100,
		})
	}

	// Today: range from 1.1050 to 1.1150, closing above yesterday's VA.
	for i := 0; i < 16; i++ {
		t := now.Add(time.Duration(i) * 30 * time.Minute)
		bars = append(bars, OHLCV{
			Date: t, Open: 1.1100 + float64(i)*0.00005,
			High: 1.1150, Low: 1.1050,
			Close: 1.1140, Volume: 100,
		})
	}

	r := ClassifyClose(bars, 5)
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if len(r.Days) == 0 {
		t.Fatal("expected at least one day")
	}
	if r.TodayImplication == "" {
		t.Error("expected non-empty implication")
	}
}

func TestClassifyClose_NilOnEmpty(t *testing.T) {
	r := ClassifyClose(nil, 5)
	if r != nil {
		t.Error("expected nil for empty bars")
	}
}

func TestClassifyClose_NilOnOneDay(t *testing.T) {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	bars := makeTrendingBars(now, 1.1000, 1.1100, 16)
	r := ClassifyClose(bars, 5)
	if r != nil {
		t.Error("expected nil for single day (need at least 2)")
	}
}

func TestClassifyCloseLocation(t *testing.T) {
	va := ValueArea{
		POC: 1.1050,
		VAH: 1.1080,
		VAL: 1.1020,
	}

	tests := []struct {
		close    float64
		expected CloseLocation
	}{
		{1.1100, CloseAboveVAH},
		{1.1000, CloseBelowVAL},
		{1.1050, CloseAtPOC},    // exactly at POC
		{1.1060, CloseInsideVA}, // inside VA but not near POC
	}

	for _, tt := range tests {
		cc := classifyCloseLocation(time.Now(), tt.close, va)
		if cc.Location != tt.expected {
			t.Errorf("close=%.4f: expected %s, got %s", tt.close, tt.expected, cc.Location)
		}
	}
}
