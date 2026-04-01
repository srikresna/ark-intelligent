package bis

import (
	"testing"
)

func TestParseBISCSV_PolicyRates(t *testing.T) {
	// Simulated BIS WS_CBPOL CSV response.
	csv := `KEY,FREQ,REF_AREA,INSTR_ASSET,TIME_PERIOD,OBS_VALUE
Q.US,Q,US,P,2025-Q3,5.25
Q.XM,Q,XM,P,2025-Q3,4.00
Q.GB,Q,GB,P,2025-Q3,5.00
Q.JP,Q,JP,P,2025-Q3,-0.10
Q.AU,Q,AU,P,2025-Q3,4.35
`
	rows, err := ParseBISCSV([]byte(csv))
	if err != nil {
		t.Fatalf("ParseBISCSV error: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 rows, got %d", len(rows))
	}

	latest := LatestByRefArea(rows)
	if r, ok := latest["US"]; !ok {
		t.Error("missing US entry")
	} else if r.OBSValue != 5.25 {
		t.Errorf("US rate: want 5.25, got %v", r.OBSValue)
	}
	if r, ok := latest["JP"]; !ok {
		t.Error("missing JP entry")
	} else if r.OBSValue != -0.10 {
		t.Errorf("JP rate: want -0.10, got %v", r.OBSValue)
	}
}

func TestParseBISCSV_MultipleObservations(t *testing.T) {
	// When multiple observations exist for same area, LatestByRefArea picks the latest period.
	csv := `KEY,FREQ,REF_AREA,TIME_PERIOD,OBS_VALUE
Q.US,Q,US,2025-Q2,5.50
Q.US,Q,US,2025-Q3,5.25
Q.US,Q,US,2025-Q1,5.50
`
	rows, err := ParseBISCSV([]byte(csv))
	if err != nil {
		t.Fatalf("ParseBISCSV error: %v", err)
	}
	latest := LatestByRefArea(rows)
	r := latest["US"]
	if r.TimePeriod != "2025-Q3" {
		t.Errorf("expected latest period 2025-Q3, got %s", r.TimePeriod)
	}
	if r.OBSValue != 5.25 {
		t.Errorf("expected value 5.25, got %v", r.OBSValue)
	}
}

func TestParseBISCSV_SkipsNaNAndEmpty(t *testing.T) {
	csv := `KEY,FREQ,REF_AREA,TIME_PERIOD,OBS_VALUE
Q.US,Q,US,2025-Q3,5.25
Q.XM,Q,XM,2025-Q3,NaN
Q.GB,Q,GB,2025-Q3,
Q.CH,Q,CH,2025-Q3,.
Q.AU,Q,AU,2025-Q3,4.35
`
	rows, err := ParseBISCSV([]byte(csv))
	if err != nil {
		t.Fatalf("ParseBISCSV error: %v", err)
	}
	// Only US and AU have valid values.
	if len(rows) != 2 {
		t.Errorf("expected 2 valid rows, got %d", len(rows))
	}
}

func TestParseBISCSV_KeyFallback(t *testing.T) {
	// REF_AREA column missing — should extract from KEY column.
	csv := `KEY,FREQ,TIME_PERIOD,OBS_VALUE
Q.US,Q,2025-Q3,5.25
Q.JP,Q,2025-Q3,0.10
`
	rows, err := ParseBISCSV([]byte(csv))
	if err != nil {
		t.Fatalf("ParseBISCSV error: %v", err)
	}
	latest := LatestByRefArea(rows)
	if _, ok := latest["US"]; !ok {
		t.Error("missing US entry from KEY fallback")
	}
	if _, ok := latest["JP"]; !ok {
		t.Error("missing JP entry from KEY fallback")
	}
}

func TestParseBISCSV_EmptyBody(t *testing.T) {
	_, err := ParseBISCSV([]byte(""))
	if err == nil {
		t.Error("expected error for empty body, got nil")
	}
}

func TestParseBISCSV_MissingRequiredColumns(t *testing.T) {
	csv := `KEY,FREQ,REF_AREA
Q.US,Q,US
`
	_, err := ParseBISCSV([]byte(csv))
	if err == nil {
		t.Error("expected error for missing OBS_VALUE column, got nil")
	}
}

func TestClassifyCreditGap(t *testing.T) {
	tests := []struct {
		gap  float64
		want string
	}{
		{3.5, "WARNING"},
		{1.0, "ELEVATED"},
		{0.5, "ELEVATED"},
		{0.0, "NEUTRAL"},
		{-2.0, "NEUTRAL"},
		{-10.0, "NEUTRAL"},
	}
	for _, tt := range tests {
		got := classifyCreditGap(tt.gap)
		if got != tt.want {
			t.Errorf("classifyCreditGap(%v) = %q, want %q", tt.gap, got, tt.want)
		}
	}
}

func TestJoinPlus(t *testing.T) {
	got := joinPlus([]string{"Q.US", "Q.XM", "Q.GB"})
	want := "Q.US+Q.XM+Q.GB"
	if got != want {
		t.Errorf("joinPlus = %q, want %q", got, want)
	}
}
