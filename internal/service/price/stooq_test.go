package price

import (
	"testing"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

func TestStooqSymbol(t *testing.T) {
	tests := []struct {
		currency string
		want     string
	}{
		{"EUR", "eurusd"},
		{"GBP", "gbpusd"},
		{"JPY", "usdjpy"},
		{"AUD", "audusd"},
		{"NZD", "nzdusd"},
		{"CAD", "usdcad"},
		{"CHF", "usdchf"},
		{"XAUUSD", "xauusd"},
		{"XAGUSD", "xagusd"},
		{"UNKNOWN", ""},
	}
	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			got := stooqSymbol(tt.currency)
			if got != tt.want {
				t.Errorf("stooqSymbol(%q) = %q, want %q", tt.currency, got, tt.want)
			}
		})
	}
}

func TestParseStooqCSV_Valid(t *testing.T) {
	csv := []byte(`Date,Open,High,Low,Close,Volume
2026-03-28,1.08234,1.08901,1.07123,1.08456,123456
2026-03-21,1.07500,1.08300,1.06800,1.08100,98765
2026-03-14,1.07100,1.07900,1.06500,1.07600,87654
`)

	mapping := domain.PriceSymbolMapping{
		ContractCode: "099741",
		Currency:     "EUR",
	}

	records, err := parseStooqCSV(csv, mapping, 52)
	if err != nil {
		t.Fatalf("parseStooqCSV: unexpected error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}

	// Should be sorted newest first
	if records[0].Date.After(records[1].Date) == false {
		t.Error("records not sorted newest-first")
	}
	if records[0].Source != "stooq" {
		t.Errorf("source = %q, want %q", records[0].Source, "stooq")
	}
	if records[0].ContractCode != "099741" {
		t.Errorf("contract code = %q, want %q", records[0].ContractCode, "099741")
	}
	if records[0].Close != 1.08456 {
		t.Errorf("close = %f, want 1.08456", records[0].Close)
	}
}

func TestParseStooqCSV_LimitWeeks(t *testing.T) {
	csv := []byte(`Date,Open,High,Low,Close,Volume
2026-03-28,1.08234,1.08901,1.07123,1.08456,123456
2026-03-21,1.07500,1.08300,1.06800,1.08100,98765
2026-03-14,1.07100,1.07900,1.06500,1.07600,87654
2026-03-07,1.06800,1.07500,1.06200,1.07200,76543
`)

	mapping := domain.PriceSymbolMapping{
		ContractCode: "099741",
		Currency:     "EUR",
	}

	records, err := parseStooqCSV(csv, mapping, 2)
	if err != nil {
		t.Fatalf("parseStooqCSV: unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2 (limited)", len(records))
	}
}

func TestParseStooqCSV_EmptyBody(t *testing.T) {
	csv := []byte(`Date,Open,High,Low,Close,Volume
`)

	mapping := domain.PriceSymbolMapping{
		ContractCode: "099741",
		Currency:     "EUR",
	}

	_, err := parseStooqCSV(csv, mapping, 52)
	if err == nil {
		t.Error("expected error for empty CSV, got nil")
	}
}

func TestParseStooqCSV_MissingColumns(t *testing.T) {
	csv := []byte(`Date,Price
2026-03-28,1.08456
`)

	mapping := domain.PriceSymbolMapping{
		ContractCode: "099741",
		Currency:     "EUR",
	}

	_, err := parseStooqCSV(csv, mapping, 52)
	if err == nil {
		t.Error("expected error for missing columns, got nil")
	}
}

func TestParseStooqCSV_BadRows(t *testing.T) {
	csv := []byte(`Date,Open,High,Low,Close,Volume
bad-date,1.08,1.09,1.07,1.08,100
2026-03-28,NaN,1.09,1.07,1.08,100
2026-03-21,1.07,1.08,1.06,1.07,200
`)

	mapping := domain.PriceSymbolMapping{
		ContractCode: "099741",
		Currency:     "EUR",
	}

	records, err := parseStooqCSV(csv, mapping, 52)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only the last valid row should be parsed
	if len(records) != 1 {
		t.Errorf("got %d records, want 1 (skipping bad rows)", len(records))
	}
}
