package macro

import (
	"strings"
	"testing"
	"time"
)

func TestParseEurostatJSON_HICP(t *testing.T) {
	// Minimal JSON-stat 2.0 response for HICP
	body := `{
		"value": {"0": 2.0, "1": 2.2, "2": 2.1},
		"dimension": {
			"time": {
				"category": {
					"index": {"2025-10": 0, "2025-11": 1, "2025-12": 2}
				}
			}
		}
	}`

	obs, err := parseEurostatJSON(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parseEurostatJSON failed: %v", err)
	}
	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}
	// Should be sorted ascending
	if obs[0].Period != "2025-10" {
		t.Errorf("first period should be 2025-10, got %s", obs[0].Period)
	}
	if obs[2].Period != "2025-12" {
		t.Errorf("last period should be 2025-12, got %s", obs[2].Period)
	}
	if obs[2].Value != 2.1 {
		t.Errorf("last value should be 2.1, got %f", obs[2].Value)
	}
}

func TestParseEurostatJSON_GDP_Quarterly(t *testing.T) {
	body := `{
		"value": {"0": 0.3, "1": 0.2, "2": 0.4},
		"dimension": {
			"time": {
				"category": {
					"index": {"2025-Q1": 0, "2025-Q2": 1, "2025-Q3": 2}
				}
			}
		}
	}`

	obs, err := parseEurostatJSON(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parseEurostatJSON failed: %v", err)
	}
	if len(obs) != 3 {
		t.Fatalf("expected 3, got %d", len(obs))
	}
	if obs[2].Value != 0.4 {
		t.Errorf("expected 0.4, got %f", obs[2].Value)
	}
}

func TestParseEurostatJSON_Empty(t *testing.T) {
	body := `{
		"value": {},
		"dimension": {
			"time": {
				"category": {
					"index": {}
				}
			}
		}
	}`

	obs, err := parseEurostatJSON(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("expected 0 observations, got %d", len(obs))
	}
}

func TestParseEurostatJSON_NoTimeDimension(t *testing.T) {
	body := `{
		"value": {"0": 1.0},
		"dimension": {
			"geo": {"category": {"index": {"EA20": 0}}}
		}
	}`

	_, err := parseEurostatJSON(strings.NewReader(body))
	if err == nil {
		t.Fatal("expected error for missing time dimension")
	}
}

func TestParsePeriodDate_Monthly(t *testing.T) {
	d := parsePeriodDate("2025-12")
	if d.IsZero() {
		t.Fatal("expected non-zero date")
	}
	if d.Year() != 2025 || d.Month() != time.December {
		t.Errorf("expected 2025-12, got %v", d)
	}
}

func TestParsePeriodDate_Quarterly(t *testing.T) {
	tests := []struct {
		period    string
		wantYear  int
		wantMonth time.Month
	}{
		{"2025-Q1", 2025, time.January},
		{"2025-Q2", 2025, time.April},
		{"2025-Q3", 2025, time.July},
		{"2025-Q4", 2025, time.October},
	}

	for _, tc := range tests {
		d := parsePeriodDate(tc.period)
		if d.IsZero() {
			t.Errorf("parsePeriodDate(%q) returned zero", tc.period)
			continue
		}
		if d.Year() != tc.wantYear || d.Month() != tc.wantMonth {
			t.Errorf("parsePeriodDate(%q) = %v, want %d-%s", tc.period, d, tc.wantYear, tc.wantMonth)
		}
	}
}

func TestParsePeriodDate_Invalid(t *testing.T) {
	d := parsePeriodDate("invalid")
	if !d.IsZero() {
		t.Errorf("expected zero for invalid period, got %v", d)
	}
}

func TestFormatQuarter(t *testing.T) {
	tests := []struct {
		month time.Month
		want  string
	}{
		{time.January, "Q1 2025"},
		{time.March, "Q1 2025"},
		{time.April, "Q2 2025"},
		{time.July, "Q3 2025"},
		{time.October, "Q4 2025"},
		{time.December, "Q4 2025"},
	}

	for _, tc := range tests {
		d := time.Date(2025, tc.month, 1, 0, 0, 0, 0, time.UTC)
		got := formatQuarter(d)
		if got != tc.want {
			t.Errorf("formatQuarter(%v) = %q, want %q", tc.month, got, tc.want)
		}
	}
}

func TestFormatEurostatData_Nil(t *testing.T) {
	result := FormatEurostatData(nil)
	if !strings.Contains(result, "tidak tersedia") {
		t.Error("nil data should produce unavailable message")
	}
}

func TestFormatEurostatData_Full(t *testing.T) {
	d := &EurostatData{
		FetchedAt:    time.Now(),
		HICPDate:     time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		HICPHeadline: 2.0,
		HICPCore:     2.7,
		UnempDate:    time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		UnempRate:    6.2,
		GDPDate:      time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC),
		GDPGrowth:    0.3,
	}

	result := FormatEurostatData(d)
	if !strings.Contains(result, "EU Economy Dashboard") {
		t.Error("should contain header")
	}
	if !strings.Contains(result, "2.0%") {
		t.Error("should contain HICP headline")
	}
	if !strings.Contains(result, "6.2%") {
		t.Error("should contain unemployment rate")
	}
	if !strings.Contains(result, "0.3%") {
		t.Error("should contain GDP growth")
	}
	if !strings.Contains(result, "Eurostat") {
		t.Error("should contain source attribution")
	}
}

func TestInflationArrow(t *testing.T) {
	tests := []struct {
		val  float64
		want string
	}{
		{4.0, "🔴"},
		{2.5, "🟡"},
		{2.0, "🟢"},
		{1.0, "📉"},
	}
	for _, tc := range tests {
		got := inflationArrow(tc.val)
		if got != tc.want {
			t.Errorf("inflationArrow(%.1f) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

func TestEurostatData_IsZero(t *testing.T) {
	var d *EurostatData
	if !d.IsZero() {
		t.Error("nil should be zero")
	}

	d2 := &EurostatData{}
	if !d2.IsZero() {
		t.Error("empty struct should be zero")
	}

	d3 := &EurostatData{FetchedAt: time.Now()}
	if d3.IsZero() {
		t.Error("populated struct should not be zero")
	}
}
