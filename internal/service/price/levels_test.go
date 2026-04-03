package price

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// stubDailyPriceStore implements DailyPriceStore for tests.
type stubDailyPriceStore struct {
	records []domain.DailyPrice
	err     error
}

func (s *stubDailyPriceStore) GetDailyHistory(_ context.Context, _ string, _ int) ([]domain.DailyPrice, error) {
	return s.records, s.err
}

// makeDailyRecords generates n newest-first DailyPrice records around a base price.
func makeDailyRecords(n int, basePrice float64) []domain.DailyPrice {
	records := make([]domain.DailyPrice, n)
	for i := 0; i < n; i++ {
		p := basePrice - float64(i)*0.001*basePrice // slight downward drift going back in time
		records[i] = domain.DailyPrice{
			Date:  time.Now().AddDate(0, 0, -i),
			Open:  p * 0.999,
			High:  p * 1.005,
			Low:   p * 0.995,
			Close: p,
		}
	}
	return records
}

// TestClassifyLevel covers all branches of classifyLevel.
func TestClassifyLevel(t *testing.T) {
	tests := []struct {
		current float64
		level   float64
		want    string
	}{
		{current: 0, level: 1.0, want: "SUPPORT"},          // zero current → SUPPORT
		{current: 1.0, level: 1.0, want: "SUPPORT"},        // exactly equal (within 0.01%) → SUPPORT
		{current: 1.0, level: 1.00005, want: "SUPPORT"},    // within 0.01% above → SUPPORT
		{current: 1.0, level: 0.99, want: "SUPPORT"},       // below current → SUPPORT
		{current: 1.0, level: 1.005, want: "RESISTANCE"},   // above current → RESISTANCE
		{current: 1.2, level: 1.5, want: "RESISTANCE"},     // clearly above → RESISTANCE
		{current: 1.5, level: 1.2, want: "SUPPORT"},        // clearly below → SUPPORT
	}
	for _, tt := range tests {
		got := classifyLevel(tt.current, tt.level)
		if got != tt.want {
			t.Errorf("classifyLevel(%v, %v) = %q, want %q", tt.current, tt.level, got, tt.want)
		}
	}
}

// TestPctDistance covers normal and zero-denominator cases.
func TestPctDistance(t *testing.T) {
	if got := pctDistance(0, 1.0); got != 0 {
		t.Errorf("pctDistance(0, 1.0) = %v, want 0", got)
	}

	got := pctDistance(1.0, 1.1)
	// (1.1-1.0)/1.0*100 = 10.0
	if math.Abs(got-10.0) > 0.001 {
		t.Errorf("pctDistance(1.0, 1.1) = %v, want ~10.0", got)
	}

	got = pctDistance(1.0, 0.9)
	// (0.9-1.0)/1.0*100 = -10.0
	if math.Abs(got-(-10.0)) > 0.001 {
		t.Errorf("pctDistance(1.0, 0.9) = %v, want ~-10.0", got)
	}
}

// TestRangeHighLow verifies the function returns the correct high and low.
func TestRangeHighLow(t *testing.T) {
	records := []domain.DailyPrice{
		{High: 1.10, Low: 1.00},
		{High: 1.15, Low: 1.02},
		{High: 1.05, Low: 0.98},
		{High: 1.08, Low: 1.01},
	}
	high, low := rangeHighLow(records)
	if high != 1.15 {
		t.Errorf("rangeHighLow high = %v, want 1.15", high)
	}
	if low != 0.98 {
		t.Errorf("rangeHighLow low = %v, want 0.98", low)
	}
}

// TestRangeHighLow_SingleRecord handles the edge case of one record.
func TestRangeHighLow_SingleRecord(t *testing.T) {
	records := []domain.DailyPrice{
		{High: 2.0, Low: 1.5},
	}
	high, low := rangeHighLow(records)
	if high != 2.0 {
		t.Errorf("got high %v, want 2.0", high)
	}
	if low != 1.5 {
		t.Errorf("got low %v, want 1.5", low)
	}
}

// TestFindSwingLevels_InsufficientData returns empty when not enough records.
func TestFindSwingLevels_InsufficientData(t *testing.T) {
	records := makeDailyRecords(9, 1.0) // need at least 2*window+1 = 11
	levels := findSwingLevels(records, 1.0)
	if len(levels) != 0 {
		t.Errorf("expected 0 levels for insufficient data, got %d", len(levels))
	}
}

// TestFindSwingLevels_DetectsSwings verifies swing detection on a known pattern.
func TestFindSwingLevels_DetectsSwings(t *testing.T) {
	// Build 30 records (newest-first). Insert a swing high at index 10 and swing low at index 20.
	n := 30
	records := make([]domain.DailyPrice, n)
	base := 1.2000
	for i := 0; i < n; i++ {
		h := base + 0.001
		l := base - 0.001
		// swing high: index 10 has a prominent high
		if i == 10 {
			h = base + 0.05
		}
		// swing low: index 20 has a prominent low
		if i == 20 {
			l = base - 0.05
		}
		records[i] = domain.DailyPrice{
			Date:  time.Now().AddDate(0, 0, -i),
			High:  roundN(h, 6),
			Low:   roundN(l, 6),
			Close: base,
			Open:  base,
		}
	}

	levels := findSwingLevels(records, base)
	// We should detect something — at minimum the swing high or low
	if len(levels) == 0 {
		t.Error("expected at least one swing level, got none")
	}
}

// TestDeduplicateLevels_Empty returns empty slice without panicking.
func TestDeduplicateLevels_Empty(t *testing.T) {
	result := deduplicateLevels(nil, 0.1)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// TestDeduplicateLevels_SingleLevel returns the level unchanged.
func TestDeduplicateLevels_SingleLevel(t *testing.T) {
	in := []KeyLevel{{Price: 1.0, Strength: 2, Type: "SUPPORT"}}
	result := deduplicateLevels(in, 0.1)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Price != 1.0 {
		t.Errorf("price changed: got %v", result[0].Price)
	}
}

// TestDeduplicateLevels_MergesDuplicates merges levels within threshold.
func TestDeduplicateLevels_MergesDuplicates(t *testing.T) {
	in := []KeyLevel{
		{Price: 1.200, Strength: 2, Type: "RESISTANCE"},
		{Price: 1.201, Strength: 3, Type: "RESISTANCE"}, // within 0.1% of 1.200
		{Price: 1.500, Strength: 1, Type: "RESISTANCE"}, // far away
	}
	result := deduplicateLevels(in, 0.1)
	if len(result) != 2 {
		t.Fatalf("expected 2 merged levels, got %d", len(result))
	}
	// The merged level should keep strength=3 (higher)
	if result[0].Strength != 3 {
		t.Errorf("expected merged strength 3, got %d", result[0].Strength)
	}
}

// TestDeduplicateLevels_BumpsStrengthOnTie bumps strength when equal.
func TestDeduplicateLevels_BumpsStrengthOnTie(t *testing.T) {
	in := []KeyLevel{
		{Price: 1.200, Strength: 2, Type: "SUPPORT"},
		{Price: 1.2005, Strength: 2, Type: "SUPPORT"}, // within 0.1%
	}
	result := deduplicateLevels(in, 0.1)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged level, got %d", len(result))
	}
	// Strength should be bumped from 2 → 3
	if result[0].Strength != 3 {
		t.Errorf("expected bumped strength 3, got %d", result[0].Strength)
	}
}

// TestNewLevelsBuilder constructs without panic.
func TestNewLevelsBuilder(t *testing.T) {
	repo := &stubDailyPriceStore{}
	lb := NewLevelsBuilder(repo)
	if lb == nil {
		t.Fatal("NewLevelsBuilder returned nil")
	}
}

// TestBuild_RepoError propagates repository errors.
func TestBuild_RepoError(t *testing.T) {
	repo := &stubDailyPriceStore{err: errors.New("db down")}
	lb := NewLevelsBuilder(repo)
	_, err := lb.Build(context.Background(), "TEST", "USD")
	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
}

// TestBuild_InsufficientData returns error when < 10 records.
func TestBuild_InsufficientData(t *testing.T) {
	repo := &stubDailyPriceStore{records: makeDailyRecords(5, 1.2)}
	lb := NewLevelsBuilder(repo)
	_, err := lb.Build(context.Background(), "TEST", "USD")
	if err == nil {
		t.Fatal("expected insufficient-data error, got nil")
	}
}

// TestBuild_Success verifies full Build with enough data returns a valid LevelsContext.
func TestBuild_Success(t *testing.T) {
	records := makeDailyRecords(220, 1.2000)
	repo := &stubDailyPriceStore{records: records}
	lb := NewLevelsBuilder(repo)

	lc, err := lb.Build(context.Background(), "EURUSD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc == nil {
		t.Fatal("LevelsContext is nil")
	}
	if lc.ContractCode != "EURUSD" {
		t.Errorf("ContractCode = %q, want EURUSD", lc.ContractCode)
	}
	if lc.CurrentPrice <= 0 {
		t.Errorf("CurrentPrice = %v, want > 0", lc.CurrentPrice)
	}
	if lc.DailyATR < 0 {
		t.Errorf("DailyATR = %v, want >= 0", lc.DailyATR)
	}
	if lc.DailyPivot <= 0 {
		t.Errorf("DailyPivot = %v, want > 0", lc.DailyPivot)
	}
	if len(lc.Levels) == 0 {
		t.Error("expected at least one level, got none")
	}

	// Verify pivot math: P = (H+L+C)/3
	latest := records[0]
	expectedPivot := roundN((latest.High+latest.Low+latest.Close)/3, 6)
	if math.Abs(lc.DailyPivot-expectedPivot) > 1e-9 {
		t.Errorf("DailyPivot = %v, want %v", lc.DailyPivot, expectedPivot)
	}
}

// TestBuild_NearestSupportResistance verifies nearest levels are populated.
func TestBuild_NearestSupportResistance(t *testing.T) {
	records := makeDailyRecords(220, 1.1000)
	repo := &stubDailyPriceStore{records: records}
	lb := NewLevelsBuilder(repo)

	lc, err := lb.Build(context.Background(), "GBPUSD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With enough levels we should find at least one nearest support or resistance
	// (not guaranteed both, but at minimum the levels slice is populated)
	if len(lc.Levels) == 0 {
		t.Error("expected levels, got none")
	}
}
