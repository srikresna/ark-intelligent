package ta

import (
	"testing"
	"time"
)

func TestClassifyRotation_Balanced(t *testing.T) {
	// Create bars that oscillate between VA extremes → high rotation factor.
	bars := makeRotationBars(time.Now().UTC(), 4.0, 5.0, true) // oscillating inside VA
	r := ClassifyRotation(bars, 2, 5)
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if len(r.Days) == 0 {
		t.Fatal("expected at least one day")
	}
}

func TestClassifyRotation_Directional(t *testing.T) {
	// Create bars that trend in one direction → low rotation factor.
	bars := makeTrendingBars(time.Now().UTC(), 1.1000, 1.1200, 24)
	r := ClassifyRotation(bars, 2, 5)
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if len(r.Days) == 0 {
		t.Fatal("expected at least one day")
	}
	for _, d := range r.Days {
		if d.RotationFactor > 3 {
			t.Errorf("expected low rotation for trending bars, got RF=%d", d.RotationFactor)
		}
	}
}

func TestClassifyRotation_NilOnEmpty(t *testing.T) {
	r := ClassifyRotation(nil, 2, 5)
	if r != nil {
		t.Error("expected nil for empty bars")
	}
}

func TestRotationTrend(t *testing.T) {
	days := []RotationResult{
		{RotationFactor: 1},
		{RotationFactor: 2},
		{RotationFactor: 3},
		{RotationFactor: 5},
	}
	trend := rotationTrend(days)
	if trend != "INCREASING" {
		t.Errorf("expected INCREASING, got %s", trend)
	}
}

// makeRotationBars creates bars for one day that oscillate within a range.
func makeRotationBars(baseDate time.Time, low, high float64, _ bool) []OHLCV {
	date := baseDate.Truncate(24 * time.Hour)
	mid := (low + high) / 2.0
	var bars []OHLCV

	// Create 24 half-hour bars oscillating between low and high.
	for i := 0; i < 24; i++ {
		t := date.Add(time.Duration(i) * 30 * time.Minute)
		var o, h, l, c float64
		if i%2 == 0 {
			// Up bar: from low to high
			o, h, l, c = low, high, low, high
		} else {
			// Down bar: from high to low
			o, h, l, c = high, high, low, low
		}
		_ = mid
		bars = append(bars, OHLCV{
			Date: t, Open: o, High: h, Low: l, Close: c, Volume: 100,
		})
	}
	return bars
}

// makeTrendingBars creates bars for one day that trend from start to end.
func makeTrendingBars(baseDate time.Time, start, end float64, count int) []OHLCV {
	date := baseDate.Truncate(24 * time.Hour)
	step := (end - start) / float64(count)
	var bars []OHLCV

	for i := 0; i < count; i++ {
		t := date.Add(time.Duration(i) * 30 * time.Minute)
		o := start + step*float64(i)
		c := start + step*float64(i+1)
		h := c + 0.0005
		l := o - 0.0005
		bars = append(bars, OHLCV{
			Date: t, Open: o, High: h, Low: l, Close: c, Volume: 100,
		})
	}
	return bars
}
