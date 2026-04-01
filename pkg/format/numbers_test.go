package format

import (
	"testing"
)

// ---------------------------------------------------------------------------
// FormatInt
// ---------------------------------------------------------------------------

func TestFormatInt(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{"zero", 0, "0"},
		{"positive small", 999, "999"},
		{"exactly 1000", 1000, "1,000"},
		{"positive 5 digits", 12345, "12,345"},
		{"positive 6 digits", 123456, "123,456"},
		{"positive 7 digits", 1234567, "1,234,567"},
		{"negative 4 digits", -1234, "-1,234"},
		{"negative large", -1234567, "-1,234,567"},
		{"negative 3 digits", -999, "-999"},
		{"ten thousand", 10000, "10,000"},
		{"hundred thousand", 100000, "100,000"},
		{"million", 1000000, "1,000,000"},
		{"billion", 1000000000, "1,000,000,000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatInt(tt.n)
			if got != tt.want {
				t.Errorf("FormatInt(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FormatFloat
// ---------------------------------------------------------------------------

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		f        float64
		decimals int
		want     string
	}{
		{"integer no decimals", 12345.0, 0, "12,345"},
		{"with 2 decimals", 12345.678, 2, "12,345.68"},
		{"negative with decimals", -1234567.89, 2, "-1,234,567.89"},
		{"zero no decimals", 0, 0, "0"},
		{"zero with decimals", 0, 2, "0.00"},
		{"small positive", 42.5, 1, "42.5"},
		{"large number", 9999999.0, 0, "9,999,999"},
		{"negative small", -5.0, 0, "-5"},
		{"below thousand", 999.99, 2, "999.99"},
		{"negative decimals treated as zero", 100.5, -1, "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFloat(tt.f, tt.decimals)
			if got != tt.want {
				t.Errorf("FormatFloat(%v, %d) = %q, want %q", tt.f, tt.decimals, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FormatPct
// ---------------------------------------------------------------------------

func TestFormatPct(t *testing.T) {
	tests := []struct {
		name string
		f    float64
		want string
	}{
		{"decimal fraction 0.673", 0.673, "67.3%"},
		{"decimal fraction 0.5", 0.5, "50.0%"},
		{"already pct 67.3", 67.3, "67.3%"},
		{"already pct 100", 100.0, "100.0%"},
		{"already pct 0", 0.0, "0.0%"},
		{"negative decimal", -0.25, "-25.0%"},
		{"negative pct", -25.0, "-25.0%"},
		{"small decimal 0.001", 0.001, "0.1%"},
		{"rounding", 0.6789, "67.9%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPct(tt.f)
			if got != tt.want {
				t.Errorf("FormatPct(%v) = %q, want %q", tt.f, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FormatForex
// ---------------------------------------------------------------------------

func TestFormatForex(t *testing.T) {
	tests := []struct {
		name  string
		price float64
		isJPY bool
		want  string
	}{
		{"EURUSD 5 decimals", 1.08432, false, "1.08432"},
		{"GBPUSD 5 decimals", 1.27345, false, "1.27345"},
		{"USDJPY 3 decimals", 149.215, true, "149.215"},
		{"EURJPY 3 decimals", 161.50, true, "161.500"},
		{"zero non-JPY", 0.0, false, "0.00000"},
		{"zero JPY", 0.0, true, "0.000"},
		{"small price", 0.00001, false, "0.00001"},
		{"rounding non-JPY", 1.123456789, false, "1.12346"},
		{"rounding JPY", 149.9999, true, "150.000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatForex(tt.price, tt.isJPY)
			if got != tt.want {
				t.Errorf("FormatForex(%v, isJPY=%v) = %q, want %q", tt.price, tt.isJPY, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FormatNetPosition
// ---------------------------------------------------------------------------

func TestFormatNetPosition(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{"zero", 0, "0"},
		{"positive", 123456, "+123,456"},
		{"negative", -50000, "-50,000"},
		{"small positive", 1, "+1"},
		{"small negative", -1, "-1"},
		{"large positive", 999999, "+999,999"},
		{"large negative", -1234567, "-1,234,567"},
		{"exactly 1000 positive", 1000, "+1,000"},
		{"exactly 1000 negative", -1000, "-1,000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatNetPosition(tt.n)
			if got != tt.want {
				t.Errorf("FormatNetPosition(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// addThousandSeparators (internal, tested via public API)
// ---------------------------------------------------------------------------

func TestAddThousandSeparatorsViaFormatInt(t *testing.T) {
	// Edge cases: 1, 2, 3 digit numbers should have no commas
	cases := []struct {
		n    int64
		want string
	}{
		{1, "1"},
		{12, "12"},
		{123, "123"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{12345678, "12,345,678"},
	}

	for _, c := range cases {
		got := FormatInt(c.n)
		if got != c.want {
			t.Errorf("FormatInt(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
