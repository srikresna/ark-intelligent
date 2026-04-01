package macro

import (
	"strings"
	"testing"
	"time"
)

func TestParseYieldCSV_Basic(t *testing.T) {
	csv := `Date,1 Mo,2 Mo,3 Mo,4 Mo,6 Mo,1 Yr,2 Yr,3 Yr,5 Yr,7 Yr,10 Yr,20 Yr,30 Yr
03/31/2026,5.33,5.32,5.31,5.29,5.18,5.02,4.59,4.46,4.30,4.37,4.43,4.75,4.65
03/28/2026,5.33,5.32,5.30,5.28,5.17,5.00,4.58,4.45,4.28,4.35,4.41,4.73,4.63
`
	rows, err := parseYieldCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Y10 != 4.43 {
		t.Errorf("expected Y10=4.43, got %v", rows[0].Y10)
	}
	if rows[0].Y5 != 4.30 {
		t.Errorf("expected Y5=4.30, got %v", rows[0].Y5)
	}
	if rows[0].Date.Day() != 31 {
		t.Errorf("expected date day=31, got %d", rows[0].Date.Day())
	}
}

func TestParseYieldCSV_TIPSFormat(t *testing.T) {
	// TIPS CSV has same format with same tenors (real yields are typically negative or low)
	csv := `Date,5 Yr,7 Yr,10 Yr,20 Yr,30 Yr
03/31/2026,1.98,2.10,2.22,2.45,2.41
03/28/2026,1.95,2.07,2.19,2.42,2.38
`
	rows, err := parseYieldCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Y10 != 2.22 {
		t.Errorf("expected Y10=2.22, got %v", rows[0].Y10)
	}
}

func TestParseYieldCSV_EmptyInput(t *testing.T) {
	rows, err := parseYieldCSV(strings.NewReader("Date,5 Yr,10 Yr\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for header-only CSV, got %d", len(rows))
	}
}

func TestParseYieldCSV_NAValues(t *testing.T) {
	csv := `Date,5 Yr,7 Yr,10 Yr,20 Yr,30 Yr
03/31/2026,N/A,N/A,2.20,N/A,2.40
`
	rows, err := parseYieldCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Y5 != 0 {
		t.Errorf("N/A should parse as 0, got %v", rows[0].Y5)
	}
	if rows[0].Y10 != 2.20 {
		t.Errorf("expected Y10=2.20, got %v", rows[0].Y10)
	}
}

func TestSafeBreakeven(t *testing.T) {
	cases := []struct {
		nom, real, want float64
	}{
		{4.43, 2.21, 2.22},
		{0, 2.21, 0},   // missing nominal → 0
		{4.43, 0, 0},   // missing real → 0
		{0, 0, 0},      // both missing → 0
		{4.0, 4.5, -0.5}, // negative (unusual but valid)
	}
	for _, c := range cases {
		got := safeBreakeven(c.nom, c.real)
		if abs(got-c.want) > 0.001 {
			t.Errorf("safeBreakeven(%.2f, %.2f) = %.4f, want %.4f", c.nom, c.real, got, c.want)
		}
	}
}

func TestFormatBreakevens_Nil(t *testing.T) {
	got := FormatBreakevens(nil)
	if !strings.Contains(got, "unavailable") {
		t.Errorf("expected 'unavailable' for nil input, got: %q", got)
	}
}

func TestFormatBreakevens_HighInflation(t *testing.T) {
	be := &Breakevens{
		Date:      time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		BE10:      3.1, // above 2.8 → USD bullish signal
		Nominal10: 5.31,
		Real10:    2.21,
	}
	got := FormatBreakevens(be)
	if !strings.Contains(got, "Elevated breakevens") {
		t.Errorf("expected 'Elevated breakevens' signal for BE10=3.1, got: %q", got)
	}
	if !strings.Contains(got, "bullish") {
		t.Errorf("expected 'bullish' for elevated breakevens, got: %q", got)
	}
}

func TestFormatBreakevens_LowInflation(t *testing.T) {
	be := &Breakevens{
		Date:      time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		BE10:      1.5, // below 1.7
		Nominal10: 4.0,
		Real10:    2.5,
	}
	got := FormatBreakevens(be)
	if !strings.Contains(got, "deflationary") {
		t.Errorf("expected 'deflationary' for BE10=1.5, got: %q", got)
	}
}

func TestNewTreasuryClient(t *testing.T) {
	c := NewTreasuryClient()
	if c == nil {
		t.Fatal("expected non-nil TreasuryClient")
	}
	if c.hc == nil {
		t.Error("expected non-nil http.Client")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
