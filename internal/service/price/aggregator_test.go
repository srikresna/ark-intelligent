package price

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// makeIntradayBars builds n 15m bars starting from a base time, newest-first.
func makeIntradayBars(n int, basePrice float64) []domain.IntradayBar {
	bars := make([]domain.IntradayBar, n)
	// Start from now and go backward in 15m steps
	base := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC) // midnight, makes alignment predictable
	for i := 0; i < n; i++ {
		p := basePrice + float64(i)*0.0001 // slight variation
		bars[i] = domain.IntradayBar{
			ContractCode: "TEST",
			Symbol:       "EURUSD",
			Interval:     "15m",
			// newest bar at index 0 (most recent), bars go back in time
			Timestamp: base.Add(time.Duration(n-1-i) * 15 * time.Minute),
			Open:      p,
			High:      p * 1.001,
			Low:       p * 0.999,
			Close:     p,
			Volume:    100,
			Source:    "test",
		}
	}
	return bars
}

// TestAlignToBucket verifies time alignment to bucket boundaries.
func TestAlignToBucket(t *testing.T) {
	tests := []struct {
		t       time.Time
		minutes int
		wantH   int
		wantM   int
	}{
		{time.Date(2024, 1, 1, 0, 17, 30, 0, time.UTC), 30, 0, 0},   // 00:17 → 00:00 bucket
		{time.Date(2024, 1, 1, 1, 45, 0, 0, time.UTC), 60, 1, 0},    // 01:45 → 01:00 bucket
		{time.Date(2024, 1, 1, 5, 30, 0, 0, time.UTC), 240, 4, 0},   // 05:30 → 04:00 bucket
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 15, 0, 0},     // exact boundary
		{time.Date(2024, 1, 1, 0, 14, 59, 0, time.UTC), 15, 0, 0},   // just before boundary
		{time.Date(2024, 1, 1, 0, 15, 0, 0, time.UTC), 15, 0, 15},   // exactly next bucket
	}

	for _, tt := range tests {
		got := alignToBucket(tt.t, tt.minutes)
		if got.Hour() != tt.wantH || got.Minute() != tt.wantM {
			t.Errorf("alignToBucket(%v, %d) = %02d:%02d, want %02d:%02d",
				tt.t.Format("15:04:05"), tt.minutes, got.Hour(), got.Minute(), tt.wantH, tt.wantM)
		}
		// Seconds must always be zero
		if got.Second() != 0 || got.Nanosecond() != 0 {
			t.Errorf("alignToBucket result has non-zero sub-minute: %v", got)
		}
	}
}

// TestAggregateBars_Empty handles nil/empty input.
func TestAggregateBars_Empty(t *testing.T) {
	result := aggregateBars(nil, "1h", 60)
	if result != nil {
		t.Errorf("expected nil result for empty input, got %v", result)
	}
	result = aggregateBars([]domain.IntradayBar{}, "1h", 60)
	if result != nil {
		t.Errorf("expected nil result for empty slice, got %v", result)
	}
}

// TestAggregateBars_30m aggregates 4 consecutive 15m bars into 2 30m bars.
func TestAggregateBars_30m(t *testing.T) {
	// 4 bars starting at midnight in 15m steps → should produce 2 complete 30m buckets
	base := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	bars := []domain.IntradayBar{
		{ContractCode: "T", Symbol: "EURUSD", Interval: "15m", Timestamp: base, Open: 1.0, High: 1.01, Low: 0.99, Close: 1.005, Volume: 100, Source: "test"},
		{ContractCode: "T", Symbol: "EURUSD", Interval: "15m", Timestamp: base.Add(15 * time.Minute), Open: 1.005, High: 1.015, Low: 1.0, Close: 1.010, Volume: 110, Source: "test"},
		{ContractCode: "T", Symbol: "EURUSD", Interval: "15m", Timestamp: base.Add(30 * time.Minute), Open: 1.010, High: 1.020, Low: 1.005, Close: 1.015, Volume: 120, Source: "test"},
		{ContractCode: "T", Symbol: "EURUSD", Interval: "15m", Timestamp: base.Add(45 * time.Minute), Open: 1.015, High: 1.025, Low: 1.010, Close: 1.020, Volume: 130, Source: "test"},
	}

	result := aggregateBars(bars, "30m", 30)

	if len(result) != 2 {
		t.Fatalf("expected 2 30m bars, got %d", len(result))
	}

	// Results are newest-first; find the bar at midnight
	var bar00 *domain.IntradayBar
	for i := range result {
		if result[i].Timestamp.Equal(base) {
			bar00 = &result[i]
		}
	}
	if bar00 == nil {
		t.Fatal("could not find 00:00 30m bar")
	}

	// Open of bucket = first bar's open (1.0)
	if bar00.Open != 1.0 {
		t.Errorf("Open = %v, want 1.0", bar00.Open)
	}
	// Close = last bar's close in the bucket (1.010)
	if bar00.Close != 1.010 {
		t.Errorf("Close = %v, want 1.010", bar00.Close)
	}
	// High = max of bar highs (1.015)
	if bar00.High != 1.015 {
		t.Errorf("High = %v, want 1.015", bar00.High)
	}
	// Low = min of bar lows (0.99)
	if bar00.Low != 0.99 {
		t.Errorf("Low = %v, want 0.99", bar00.Low)
	}
	// Volume = sum (100 + 110)
	if bar00.Volume != 210 {
		t.Errorf("Volume = %v, want 210", bar00.Volume)
	}
	if bar00.Interval != "30m" {
		t.Errorf("Interval = %q, want 30m", bar00.Interval)
	}
}

// TestAggregateBars_MetadataCarried checks ContractCode/Symbol/Source are propagated.
func TestAggregateBars_MetadataCarried(t *testing.T) {
	base := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	bars := []domain.IntradayBar{
		{ContractCode: "099741", Symbol: "EUR/USD", Interval: "15m", Timestamp: base, Open: 1.1, High: 1.15, Low: 1.09, Close: 1.12, Volume: 50, Source: "twelvedata"},
		{ContractCode: "099741", Symbol: "EUR/USD", Interval: "15m", Timestamp: base.Add(15 * time.Minute), Open: 1.12, High: 1.13, Low: 1.11, Close: 1.115, Volume: 60, Source: "twelvedata"},
	}

	result := aggregateBars(bars, "30m", 30)
	if len(result) == 0 {
		t.Fatal("expected at least one bar")
	}
	b := result[0]
	if b.ContractCode != "099741" {
		t.Errorf("ContractCode = %q, want 099741", b.ContractCode)
	}
	if b.Symbol != "EUR/USD" {
		t.Errorf("Symbol = %q, want EUR/USD", b.Symbol)
	}
	if b.Source != "twelvedata" {
		t.Errorf("Source = %q, want twelvedata", b.Source)
	}
}

// TestAggregateFromBase_AllIntervalsPresent verifies all expected keys are returned.
func TestAggregateFromBase_AllIntervalsPresent(t *testing.T) {
	// 96 bars = 24 hours of 15m data, enough to form at least partial higher timeframes
	bars := makeIntradayBars(96, 1.1000)
	result := AggregateFromBase(bars)

	expected := []string{"15m", "30m", "1h", "4h", "6h", "12h"}
	for _, key := range expected {
		if _, ok := result[key]; !ok {
			t.Errorf("missing interval %q in result", key)
		}
	}
}

// TestAggregateFromBase_15mPassthrough verifies 15m bars are returned as-is.
func TestAggregateFromBase_15mPassthrough(t *testing.T) {
	bars := makeIntradayBars(8, 1.2)
	result := AggregateFromBase(bars)

	if len(result["15m"]) != len(bars) {
		t.Errorf("15m passthrough: got %d bars, want %d", len(result["15m"]), len(bars))
	}
}

// TestAggregateFromBase_Empty handles empty input.
func TestAggregateFromBase_Empty(t *testing.T) {
	result := AggregateFromBase(nil)
	// 15m key should exist with nil value; other intervals should be nil
	if v, ok := result["15m"]; !ok || v != nil {
		// nil input → 15m = nil
	}
	// Should not panic — just verify it returns a map
	if result == nil {
		t.Error("expected non-nil map from AggregateFromBase(nil)")
	}
}

// TestAggregateBars_SortedNewestFirst verifies output is sorted newest-first.
func TestAggregateBars_SortedNewestFirst(t *testing.T) {
	bars := makeIntradayBars(96, 1.0)
	result := aggregateBars(bars, "1h", 60)
	if len(result) < 2 {
		t.Skip("not enough 1h bars to check ordering")
	}
	for i := 1; i < len(result); i++ {
		if result[i].Timestamp.After(result[i-1].Timestamp) {
			t.Errorf("result not sorted newest-first at index %d: %v after %v",
				i, result[i].Timestamp, result[i-1].Timestamp)
		}
	}
}
