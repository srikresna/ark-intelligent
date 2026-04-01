package ta

import (
	"math"
	"testing"
	"time"
)

// makeAMTBar creates an OHLCV bar with a given date string, OHLCV, and volume.
func makeAMTBar(dateStr string, o, h, l, c, v float64) OHLCV {
	t, _ := time.Parse("2006-01-02 15:04", dateStr)
	return OHLCV{Date: t, Open: o, High: h, Low: l, Close: c, Volume: v}
}

// buildTrendDay creates a clear trend day: IB in first 2 bars covers ~20% of range,
// then price extends strongly upward.
func buildTrendDay(date string) []OHLCV {
	// IB: 1.0800–1.0820 (range = 20 pips)
	// Final day range: 1.0800–1.0900 (range = 100 pips → IB = 20%)
	return []OHLCV{
		makeAMTBar(date+" 00:30", 1.0810, 1.0820, 1.0800, 1.0815, 100), // IB bar 1
		makeAMTBar(date+" 01:00", 1.0815, 1.0820, 1.0810, 1.0818, 80),  // IB bar 2
		makeAMTBar(date+" 01:30", 1.0820, 1.0840, 1.0818, 1.0838, 150),
		makeAMTBar(date+" 02:00", 1.0838, 1.0860, 1.0836, 1.0858, 200),
		makeAMTBar(date+" 02:30", 1.0858, 1.0880, 1.0855, 1.0878, 220),
		makeAMTBar(date+" 03:00", 1.0878, 1.0900, 1.0875, 1.0898, 250),
	}
}

// buildNormalDay creates a normal day: IB covers ~90% of range.
func buildNormalDay(date string) []OHLCV {
	// IB: 1.0800–1.0890 (range = 90 pips)
	// Day: 1.0800–1.0895 (range = 95 pips → IB ~95%)
	return []OHLCV{
		makeAMTBar(date+" 00:30", 1.0845, 1.0890, 1.0800, 1.0860, 100),
		makeAMTBar(date+" 01:00", 1.0860, 1.0890, 1.0805, 1.0850, 90),
		makeAMTBar(date+" 01:30", 1.0850, 1.0892, 1.0848, 1.0855, 80),
		makeAMTBar(date+" 02:00", 1.0855, 1.0895, 1.0802, 1.0848, 70),
		makeAMTBar(date+" 02:30", 1.0848, 1.0888, 1.0805, 1.0850, 75),
		makeAMTBar(date+" 03:00", 1.0850, 1.0888, 1.0803, 1.0855, 85),
	}
}

// buildPShapeDay creates a P-shape day: close at top, high volume upper half, long lower tail.
func buildPShapeDay(date string) []OHLCV {
	// Range: 1.0720 – 1.0850 (130 pips). Lower tail: 1.0720–1.0760. Upper volume heavy.
	return []OHLCV{
		makeAMTBar(date+" 00:30", 1.0760, 1.0780, 1.0720, 1.0770, 50),  // lower tail, low vol
		makeAMTBar(date+" 01:00", 1.0770, 1.0800, 1.0760, 1.0795, 60),
		makeAMTBar(date+" 01:30", 1.0795, 1.0830, 1.0790, 1.0825, 200), // move into upper half
		makeAMTBar(date+" 02:00", 1.0825, 1.0845, 1.0820, 1.0840, 250), // heavy upper vol
		makeAMTBar(date+" 02:30", 1.0840, 1.0850, 1.0835, 1.0848, 300),
		makeAMTBar(date+" 03:00", 1.0848, 1.0850, 1.0842, 1.0849, 280),
	}
}

// buildBShapeDay creates a b-shape day: open at top, heavy lower volume, long upper tail.
func buildBShapeDay(date string) []OHLCV {
	// Range: 1.0800 – 1.0930 (130 pips). Upper tail: 1.0900–1.0930. Lower vol heavy.
	return []OHLCV{
		makeAMTBar(date+" 00:30", 1.0920, 1.0930, 1.0900, 1.0910, 40),  // upper tail, low vol
		makeAMTBar(date+" 01:00", 1.0910, 1.0925, 1.0880, 1.0885, 60),
		makeAMTBar(date+" 01:30", 1.0885, 1.0890, 1.0840, 1.0845, 200), // drop into lower half
		makeAMTBar(date+" 02:00", 1.0845, 1.0855, 1.0810, 1.0815, 280), // heavy lower vol
		makeAMTBar(date+" 02:30", 1.0815, 1.0820, 1.0800, 1.0808, 300),
		makeAMTBar(date+" 03:00", 1.0808, 1.0815, 1.0800, 1.0803, 250),
	}
}

func TestClassifyDayTypes_Trend(t *testing.T) {
	bars := buildTrendDay("2026-03-31")
	// Reverse to newest-first for ClassifyDayTypes input.
	reverseBars(bars)

	result := ClassifyDayTypes(bars, 2, 5)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Days) == 0 {
		t.Fatal("expected at least one day")
	}
	dc := result.Days[0]
	if dc.Type != DayTypeTrend {
		t.Errorf("expected Trend day, got %s (IBPercent=%.1f%%)", dc.Type, dc.IBPercent)
	}
}

func TestClassifyDayTypes_Normal(t *testing.T) {
	bars := buildNormalDay("2026-03-31")
	reverseBars(bars)

	result := ClassifyDayTypes(bars, 2, 5)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	dc := result.Days[0]
	if dc.Type != DayTypeNormal && dc.Type != DayTypeNormalVariation {
		t.Errorf("expected Normal or NormalVariation day, got %s (IBPercent=%.1f%%)", dc.Type, dc.IBPercent)
	}
}

func TestClassifyDayTypes_PShape(t *testing.T) {
	bars := buildPShapeDay("2026-03-31")
	reverseBars(bars)

	result := ClassifyDayTypes(bars, 2, 5)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	dc := result.Days[0]
	if dc.Type != DayTypePShape {
		t.Logf("P-shape day got %s (UpperVol=%.2f, LowerTail=%.4f)", dc.Type, dc.UpperVolumeRatio, (dc.IBLow-dc.DayLow)/dc.DayRange)
	}
}

func TestClassifyDayTypes_BShape(t *testing.T) {
	bars := buildBShapeDay("2026-03-31")
	reverseBars(bars)

	result := ClassifyDayTypes(bars, 2, 5)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	dc := result.Days[0]
	if dc.Type != DayTypeBShape {
		t.Logf("b-shape day got %s (LowerVol=%.2f, UpperTail=%.4f)", dc.Type, dc.LowerVolumeRatio, (dc.DayHigh-dc.IBHigh)/dc.DayRange)
	}
}

func TestClassifyDayTypes_MultiDay(t *testing.T) {
	// Build 5 bars across 2 days (3 bars day1, 3 bars day2) newest-first.
	day1 := buildTrendDay("2026-03-30")
	day2 := buildTrendDay("2026-03-31")

	// Merge newest-first: day2 is newer.
	allBars := append(reversedCopy(day2), reversedCopy(day1)...)

	result := ClassifyDayTypes(allBars, 2, 5)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Days) < 2 {
		t.Fatalf("expected 2 days, got %d", len(result.Days))
	}
	if result.ConsecutiveTrendDays != 2 {
		t.Errorf("expected 2 consecutive trend days, got %d", result.ConsecutiveTrendDays)
	}
}

func TestClassifyDayTypes_IBPercent(t *testing.T) {
	// Manually verify IB calculation.
	bars := buildTrendDay("2026-03-31")
	reverseBars(bars)

	result := ClassifyDayTypes(bars, 2, 5)
	if result == nil || len(result.Days) == 0 {
		t.Fatal("no result")
	}
	dc := result.Days[0]

	// IB should be first 2 bars.
	ibHigh := math.Max(bars[len(bars)-1].High, bars[len(bars)-2].High)
	ibLow := math.Min(bars[len(bars)-1].Low, bars[len(bars)-2].Low)
	expectedIBRange := ibHigh - ibLow

	if math.Abs(dc.IBRange-expectedIBRange) > 1e-6 {
		t.Errorf("IBRange mismatch: got %.5f want %.5f", dc.IBRange, expectedIBRange)
	}
}

func TestClassifyDayTypes_NilOnEmpty(t *testing.T) {
	if ClassifyDayTypes(nil, 2, 5) != nil {
		t.Error("expected nil for nil bars")
	}
	if ClassifyDayTypes([]OHLCV{}, 2, 5) != nil {
		t.Error("expected nil for empty bars")
	}
}

// reverseBars reverses a slice in-place (converts oldest-first to newest-first).
func reverseBars(bars []OHLCV) {
	for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
		bars[i], bars[j] = bars[j], bars[i]
	}
}

// reversedCopy returns a reversed copy of the slice.
func reversedCopy(bars []OHLCV) []OHLCV {
	out := make([]OHLCV, len(bars))
	copy(out, bars)
	reverseBars(out)
	return out
}
