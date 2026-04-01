package deribit

import (
	"testing"
	"time"
)

func TestParseInstrumentExpiry(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantDay string // YYYY-MM-DD format
	}{
		{"BTC call", "BTC-28MAR25-80000-C", true, "2025-03-28"},
		{"ETH put", "ETH-27JUN25-3000-P", true, "2025-06-27"},
		{"BTC perp", "BTC-PERPETUAL", false, ""},
		{"empty", "", false, ""},
		{"no date", "BTC-SPOT", false, ""},
		{"single digit day", "BTC-4APR25-70000-C", true, "2025-04-04"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseInstrumentExpiry(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseInstrumentExpiry(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			gotDay := got.Format("2006-01-02")
			if gotDay != tt.wantDay {
				t.Errorf("parseInstrumentExpiry(%q) = %s, want %s", tt.input, gotDay, tt.wantDay)
			}
			// Verify 08:00 UTC expiry time
			if got.Hour() != 8 || got.Minute() != 0 {
				t.Errorf("expected 08:00 UTC expiry, got %s", got.Format(time.RFC3339))
			}
		})
	}
}

func TestGetBookSummary_FiltersExpired(t *testing.T) {
	// Verify that the filtering logic correctly identifies expired instruments.
	now := time.Now()

	// Build a past expiry date string
	pastDate := now.AddDate(0, 0, -2)
	pastInstr := "BTC-" + pastDate.Format("2Jan06") + "-80000-C"

	// Build a future expiry date string
	futureDate := now.AddDate(0, 0, 30)
	futureInstr := "BTC-" + futureDate.Format("2Jan06") + "-80000-C"

	pastExpiry, pastOK := parseInstrumentExpiry(pastInstr)
	futureExpiry, futureOK := parseInstrumentExpiry(futureInstr)

	if !pastOK {
		t.Fatalf("failed to parse past instrument: %s", pastInstr)
	}
	if !futureOK {
		t.Fatalf("failed to parse future instrument: %s", futureInstr)
	}

	if !pastExpiry.Before(now) {
		t.Errorf("past instrument %s should be before now", pastInstr)
	}
	if !futureExpiry.After(now) {
		t.Errorf("future instrument %s should be after now", futureInstr)
	}
}
