package timeutil

import (
	"testing"
	"time"
)

func TestToWIB(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		wantHour int
	}{
		{"UTC midnight becomes 7am WIB", time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), 7},
		{"UTC noon becomes 7pm WIB", time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC), 19},
		{"UTC 17:00 becomes midnight next day WIB", time.Date(2025, 6, 15, 17, 0, 0, 0, time.UTC), 0},
		{"already WIB stays same", time.Date(2025, 6, 15, 10, 0, 0, 0, WIB), 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToWIB(tt.input)
			if got.Hour() != tt.wantHour {
				t.Errorf("ToWIB(%v).Hour() = %d, want %d", tt.input, got.Hour(), tt.wantHour)
			}
			if got.Location().String() != WIB.String() {
				t.Errorf("ToWIB location = %s, want %s", got.Location(), WIB)
			}
		})
	}
}

func TestStartOfWeekWIB(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		wantDay  int
		wantWday time.Weekday
	}{
		{
			"Wednesday returns Monday",
			time.Date(2025, 6, 18, 15, 30, 0, 0, WIB), // Wednesday
			16, time.Monday,
		},
		{
			"Monday returns same Monday",
			time.Date(2025, 6, 16, 10, 0, 0, 0, WIB), // Monday
			16, time.Monday,
		},
		{
			"Sunday returns previous Monday",
			time.Date(2025, 6, 22, 10, 0, 0, 0, WIB), // Sunday
			16, time.Monday,
		},
		{
			"Saturday returns Monday",
			time.Date(2025, 6, 21, 23, 59, 0, 0, WIB), // Saturday
			16, time.Monday,
		},
		{
			"Friday returns Monday",
			time.Date(2025, 6, 20, 8, 0, 0, 0, WIB), // Friday
			16, time.Monday,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StartOfWeekWIB(tt.input)
			if got.Day() != tt.wantDay {
				t.Errorf("StartOfWeekWIB(%v).Day() = %d, want %d", tt.input, got.Day(), tt.wantDay)
			}
			if got.Weekday() != tt.wantWday {
				t.Errorf("StartOfWeekWIB(%v).Weekday() = %v, want %v", tt.input, got.Weekday(), tt.wantWday)
			}
			if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
				t.Errorf("StartOfWeekWIB should return midnight, got %02d:%02d:%02d", got.Hour(), got.Minute(), got.Second())
			}
		})
	}
}

func TestStartOfDayWIB(t *testing.T) {
	tests := []struct {
		name  string
		input time.Time
	}{
		{"afternoon", time.Date(2025, 3, 15, 14, 30, 45, 0, WIB)},
		{"midnight", time.Date(2025, 3, 15, 0, 0, 0, 0, WIB)},
		{"utc input", time.Date(2025, 3, 15, 20, 0, 0, 0, time.UTC)}, // 03:00 WIB on Mar 16
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StartOfDayWIB(tt.input)
			if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 {
				t.Errorf("StartOfDayWIB should return midnight, got %02d:%02d:%02d", got.Hour(), got.Minute(), got.Second())
			}
			// Verify the day matches the WIB interpretation
			wibInput := tt.input.In(WIB)
			if got.Day() != wibInput.Day() {
				t.Errorf("StartOfDayWIB day = %d, want %d", got.Day(), wibInput.Day())
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	input := time.Date(2025, 6, 15, 10, 30, 0, 0, WIB) // Sunday
	got := FormatDate(input)
	want := "Sun 15 Jun"
	if got != want {
		t.Errorf("FormatDate = %q, want %q", got, want)
	}
}

func TestFormatTime(t *testing.T) {
	input := time.Date(2025, 6, 15, 14, 30, 0, 0, WIB)
	got := FormatTime(input)
	want := "14:30 WIB"
	if got != want {
		t.Errorf("FormatTime = %q, want %q", got, want)
	}
}

func TestFormatDateISO(t *testing.T) {
	tests := []struct {
		name  string
		input time.Time
		want  string
	}{
		{"wib time", time.Date(2025, 1, 5, 10, 0, 0, 0, WIB), "2025-01-05"},
		{"utc late night rolls to next day", time.Date(2025, 1, 5, 20, 0, 0, 0, time.UTC), "2025-01-06"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDateISO(tt.input)
			if got != tt.want {
				t.Errorf("FormatDateISO = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDateISO(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantDay int
	}{
		{"valid", "2025-06-15", false, 15},
		{"invalid", "not-a-date", true, 0},
		{"wrong format", "15/06/2025", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateISO(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDateISO(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Day() != tt.wantDay {
				t.Errorf("ParseDateISO(%q).Day() = %d, want %d", tt.input, got.Day(), tt.wantDay)
			}
			if !tt.wantErr && got.Location().String() != WIB.String() {
				t.Errorf("ParseDateISO(%q) location = %s, want WIB", tt.input, got.Location())
			}
		})
	}
}

func TestParseDateTimeISO(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantHour int
	}{
		{"valid", "2025-06-15T14:30:00", false, 14},
		{"invalid", "2025-06-15", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateTimeISO(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDateTimeISO(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Hour() != tt.wantHour {
				t.Errorf("ParseDateTimeISO(%q).Hour() = %d, want %d", tt.input, got.Hour(), tt.wantHour)
			}
		})
	}
}

func TestParseFFDate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantMonth time.Month
		wantDay   int
	}{
		{"ISO format", "2025-03-11", false, time.March, 11},
		{"Mon Jan 2 format", "Tue Mar 11", false, time.March, 11},
		{"Jan 2 format", "Mar 11", false, time.March, 11},
		{"invalid", "gibberish", true, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFFDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFFDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Month() != tt.wantMonth {
					t.Errorf("ParseFFDate(%q).Month() = %v, want %v", tt.input, got.Month(), tt.wantMonth)
				}
				if got.Day() != tt.wantDay {
					t.Errorf("ParseFFDate(%q).Day() = %d, want %d", tt.input, got.Day(), tt.wantDay)
				}
			}
		})
	}
}

func TestParseFFTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHour  int
		wantMin   int
		wantValid bool
	}{
		{"morning am", "8:30am", 8, 30, true},
		{"afternoon pm", "2:00pm", 14, 0, true},
		{"noon", "12:00pm", 12, 0, true},
		{"midnight", "12:00am", 0, 0, true},
		{"late pm", "11:45pm", 23, 45, true},
		{"early am", "1:15am", 1, 15, true},
		{"all day", "All Day", 0, 0, false},
		{"tentative", "Tentative", 0, 0, false},
		{"day 1", "Day 1", 0, 0, false},
		{"day 2", "Day 2", 0, 0, false},
		{"empty", "", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, m, valid := ParseFFTime(tt.input)
			if h != tt.wantHour || m != tt.wantMin || valid != tt.wantValid {
				t.Errorf("ParseFFTime(%q) = (%d, %d, %v), want (%d, %d, %v)",
					tt.input, h, m, valid, tt.wantHour, tt.wantMin, tt.wantValid)
			}
		})
	}
}

func TestIsSameDay(t *testing.T) {
	tests := []struct {
		name string
		a    time.Time
		b    time.Time
		want bool
	}{
		{
			"same day same time",
			time.Date(2025, 6, 15, 10, 0, 0, 0, WIB),
			time.Date(2025, 6, 15, 22, 0, 0, 0, WIB),
			true,
		},
		{
			"different days",
			time.Date(2025, 6, 15, 10, 0, 0, 0, WIB),
			time.Date(2025, 6, 16, 10, 0, 0, 0, WIB),
			false,
		},
		{
			"UTC vs WIB same WIB day",
			time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), // 07:00 WIB Jun 15
			time.Date(2025, 6, 15, 10, 0, 0, 0, WIB),     // 10:00 WIB Jun 15
			true,
		},
		{
			"UTC late becomes next day in WIB",
			time.Date(2025, 6, 15, 20, 0, 0, 0, time.UTC), // 03:00 WIB Jun 16
			time.Date(2025, 6, 15, 10, 0, 0, 0, WIB),      // Jun 15 WIB
			false,
		},
		{
			"different years",
			time.Date(2025, 1, 1, 0, 0, 0, 0, WIB),
			time.Date(2024, 1, 1, 0, 0, 0, 0, WIB),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSameDay(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("IsSameDay(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsWeekend(t *testing.T) {
	tests := []struct {
		name  string
		input time.Time
		want  bool
	}{
		{"saturday", time.Date(2025, 6, 14, 12, 0, 0, 0, WIB), true},   // Saturday
		{"sunday", time.Date(2025, 6, 15, 12, 0, 0, 0, WIB), true},     // Sunday
		{"monday", time.Date(2025, 6, 16, 12, 0, 0, 0, WIB), false},    // Monday
		{"wednesday", time.Date(2025, 6, 18, 12, 0, 0, 0, WIB), false}, // Wednesday
		{"friday", time.Date(2025, 6, 20, 12, 0, 0, 0, WIB), false},    // Friday
		{
			"UTC friday night becomes saturday WIB",
			time.Date(2025, 6, 20, 18, 0, 0, 0, time.UTC), // Sat 01:00 WIB
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWeekend(tt.input)
			if got != tt.want {
				t.Errorf("IsWeekend(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	input := time.Date(2025, 6, 15, 14, 30, 0, 0, WIB)
	got := FormatDateTime(input)
	want := "Sun 15 Jun 14:30 WIB"
	if got != want {
		t.Errorf("FormatDateTime = %q, want %q", got, want)
	}
}

func TestStartOfWeekAlias(t *testing.T) {
	input := time.Date(2025, 6, 18, 15, 0, 0, 0, WIB)
	a := StartOfWeek(input)
	b := StartOfWeekWIB(input)
	if !a.Equal(b) {
		t.Errorf("StartOfWeek and StartOfWeekWIB should return same value")
	}
}

func TestStartOfDayAlias(t *testing.T) {
	input := time.Date(2025, 6, 18, 15, 0, 0, 0, WIB)
	a := StartOfDay(input)
	b := StartOfDayWIB(input)
	if !a.Equal(b) {
		t.Errorf("StartOfDay and StartOfDayWIB should return same value")
	}
}

func TestEndOfDay(t *testing.T) {
	input := time.Date(2025, 6, 15, 10, 30, 0, 0, WIB)
	got := EndOfDay(input)
	if got.Hour() != 23 || got.Minute() != 59 || got.Second() != 59 {
		t.Errorf("EndOfDay time = %02d:%02d:%02d, want 23:59:59", got.Hour(), got.Minute(), got.Second())
	}
	if got.Day() != 15 {
		t.Errorf("EndOfDay day = %d, want 15", got.Day())
	}
}
