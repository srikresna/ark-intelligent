package cot

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// makeRecord creates a minimal COTRecord for testing seasonal analysis.
func makeRecord(date time.Time, assetLong, assetShort float64) domain.COTRecord {
	return domain.COTRecord{
		ContractCode: "099741",
		ContractName: "Euro FX",
		ReportDate:   date,
		AssetMgrLong: assetLong,
		AssetMgrShort: assetShort,
	}
}

func TestAnalyzeRecords_Basic(t *testing.T) {
	eng := &SeasonalEngine{}
	contract := domain.COTContract{
		Code:       "099741",
		Name:       "Euro FX",
		Currency:   "EUR",
		ReportType: "TFF",
	}

	// Create 12 weekly records spanning weeks 1-12 of 2026.
	base := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC) // ISO week 2
	var history []domain.COTRecord
	for i := 0; i < 12; i++ {
		d := base.AddDate(0, 0, 7*i)
		// Simulate a rising net position: 10000 + 1000*i
		history = append(history, makeRecord(d, float64(20000+1000*i), float64(10000)))
	}

	// Analyze as-of week 8 (mid-February)
	now := time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC)
	result, err := eng.analyzeRecords(contract, history, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Basic sanity checks
	if result.Currency != "EUR" {
		t.Errorf("expected currency EUR, got %s", result.Currency)
	}
	if result.TotalRecords != 12 {
		t.Errorf("expected 12 records, got %d", result.TotalRecords)
	}
	if len(result.Curve) == 0 {
		t.Fatal("expected non-empty seasonal curve")
	}
	if result.Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestAnalyzeRecords_InsufficientData(t *testing.T) {
	eng := &SeasonalEngine{}
	contract := domain.COTContract{
		Code:       "099741",
		Currency:   "EUR",
		ReportType: "TFF",
	}

	// Only 3 records — should fail (needs ≥8).
	history := []domain.COTRecord{
		makeRecord(time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), 20000, 10000),
		makeRecord(time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC), 21000, 10000),
		makeRecord(time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), 22000, 10000),
	}

	_, err := eng.analyzeRecords(contract, history, time.Now())
	if err == nil {
		t.Error("expected error for insufficient data, got nil")
	}
}

func TestClassifyTrend(t *testing.T) {
	tests := []struct {
		z    float64
		want string
	}{
		{2.0, "ABOVE_SEASONAL"},
		{-2.0, "BELOW_SEASONAL"},
		{0.5, "IN_LINE"},
		{1.5, "IN_LINE"}, // boundary: ≤1.5 is IN_LINE
		{1.51, "ABOVE_SEASONAL"},
		{-1.51, "BELOW_SEASONAL"},
	}
	for _, tt := range tests {
		got := classifyTrend(tt.z)
		if got != tt.want {
			t.Errorf("classifyTrend(%.2f) = %s, want %s", tt.z, got, tt.want)
		}
	}
}

func TestComputeSmartNet(t *testing.T) {
	rec := domain.COTRecord{
		AssetMgrLong:     50000,
		AssetMgrShort:    20000,
		ManagedMoneyLong: 80000,
		ManagedMoneyShort: 30000,
	}

	tffNet := computeSmartNet(rec, "TFF")
	if tffNet != 30000 {
		t.Errorf("TFF net: expected 30000, got %.0f", tffNet)
	}

	disaggNet := computeSmartNet(rec, "DISAGGREGATED")
	if disaggNet != 50000 {
		t.Errorf("DISAGGREGATED net: expected 50000, got %.0f", disaggNet)
	}
}

func TestLookupWeek_Exact(t *testing.T) {
	curve := []COTSeasonalPoint{
		{WeekOfYear: 10, AvgNet: 5000, StdDev: 1000, SampleSize: 3},
		{WeekOfYear: 11, AvgNet: 6000, StdDev: 1200, SampleSize: 3},
	}
	avg, std := lookupWeek(curve, 10)
	if avg != 5000 || std != 1000 {
		t.Errorf("expected (5000, 1000), got (%.0f, %.0f)", avg, std)
	}
}

func TestLookupWeek_Nearest(t *testing.T) {
	curve := []COTSeasonalPoint{
		{WeekOfYear: 5, AvgNet: 3000, StdDev: 500, SampleSize: 2},
		{WeekOfYear: 15, AvgNet: 7000, StdDev: 900, SampleSize: 2},
	}
	// Week 7 is closer to 5 than to 15
	avg, _ := lookupWeek(curve, 7)
	if avg != 3000 {
		t.Errorf("expected 3000 (nearest to week 5), got %.0f", avg)
	}
}

func TestAnalyzeRecords_DeviationZ(t *testing.T) {
	eng := &SeasonalEngine{}
	contract := domain.COTContract{
		Code:       "099741",
		Currency:   "EUR",
		ReportType: "TFF",
	}

	// All records in the same week (week 10) with identical net=10000.
	// Then the latest has net=15000 → deviation = 5000.
	base := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC) // Week 10
	var history []domain.COTRecord
	for i := 0; i < 9; i++ {
		d := base.AddDate(0, 0, -7*(9-i))
		history = append(history, makeRecord(d, 20000, 10000)) // net 10000
	}
	// Latest record: net = 15000 (in week 10)
	history = append(history, makeRecord(base, 25000, 10000))

	_, wk := base.ISOWeek()
	result, err := eng.analyzeRecords(contract, history, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CurrentWeek != wk {
		t.Errorf("expected week %d, got %d", wk, result.CurrentWeek)
	}
	if result.CurrentNet != 15000 {
		t.Errorf("expected current net 15000, got %.0f", result.CurrentNet)
	}
	if result.Deviation != result.CurrentNet-result.SeasonalAvg {
		t.Errorf("deviation mismatch: %.0f ≠ %.0f − %.0f", result.Deviation, result.CurrentNet, result.SeasonalAvg)
	}
}
