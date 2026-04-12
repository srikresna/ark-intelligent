package fmtutil

import (
	"strings"
	"testing"
)

func TestFmtNum(t *testing.T) {
	tests := []struct {
		name     string
		v        float64
		decimals int
		want     string
	}{
		{"large with decimals", 1234567.89, 2, "1,234,567.89"},
		{"zero no decimals", 0, 0, "0"},
		{"negative", -1234, 0, "-1,234"},
		{"below thousand", 999, 0, "999"},
		{"exactly thousand", 1000, 0, "1,000"},
		{"one decimal", 42.1, 1, "42.1"},
		{"millions", 9999999, 0, "9,999,999"},
		{"small negative", -5, 0, "-5"},
		{"negative with decimals", -1234567.89, 2, "-1,234,567.89"},
		{"zero with decimals", 0, 2, "0.00"},
		{"hundred", 100, 0, "100"},
		{"ten thousand", 10000, 0, "10,000"},
		{"hundred thousand", 100000, 0, "100,000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FmtNum(tt.v, tt.decimals)
			if got != tt.want {
				t.Errorf("FmtNum(%v, %d) = %q, want %q", tt.v, tt.decimals, got, tt.want)
			}
		})
	}
}

func TestFmtNumSigned(t *testing.T) {
	tests := []struct {
		name     string
		v        float64
		decimals int
		want     string
	}{
		{"positive", 1234.5, 1, "+1,234.5"},
		{"negative", -500, 0, "-500"},
		{"zero", 0, 0, "0"},
		{"small positive", 1, 0, "+1"},
		{"large positive", 999999, 0, "+999,999"},
		{"negative with decimals", -42.75, 2, "-42.75"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FmtNumSigned(tt.v, tt.decimals)
			if got != tt.want {
				t.Errorf("FmtNumSigned(%v, %d) = %q, want %q", tt.v, tt.decimals, got, tt.want)
			}
		})
	}
}

func TestFmtPct(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		want string
	}{
		{"positive", 12.5, "+12.5%"},
		{"negative", -3.2, "-3.2%"},
		{"zero", 0, "0.0%"},
		{"large", 100.0, "+100.0%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FmtPct(tt.v)
			if got != tt.want {
				t.Errorf("FmtPct(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestFmtRatio(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		want string
	}{
		{"positive", 1.5, "1.50"},
		{"zero", 0, "0.00"},
		{"negative", -0.75, "-0.75"},
		{"whole number", 3.0, "3.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FmtRatio(tt.v)
			if got != tt.want {
				t.Errorf("FmtRatio(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestCOTIndexBar(t *testing.T) {
	tests := []struct {
		name         string
		index        float64
		width        int
		wantContains []string
		wantFilled   int
		wantDash     int
	}{
		{"75 pct", 75, 10, []string{"75"}, -1, -1},
		{"zero", 0, 10, []string{"[", "] 0"}, 0, 10},
		{"full", 100, 10, []string{"[", "] 100"}, 10, 0},
		{"half", 50, 10, []string{"50"}, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := COTIndexBar(tt.index, tt.width)
			for _, sub := range tt.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("COTIndexBar(%v, %d) = %q, want to contain %q", tt.index, tt.width, got, sub)
				}
			}
			if tt.wantFilled >= 0 {
				eqs := strings.Count(got, "=")
				if eqs != tt.wantFilled {
					t.Errorf("COTIndexBar(%v, %d) has %d '=' chars, want %d", tt.index, tt.width, eqs, tt.wantFilled)
				}
			}
			if tt.wantDash >= 0 {
				dashes := strings.Count(got, "-")
				if dashes != tt.wantDash {
					t.Errorf("COTIndexBar(%v, %d) has %d '-' chars, want %d", tt.index, tt.width, dashes, tt.wantDash)
				}
			}
		})
	}

	// Clamping tests
	t.Run("clamp above 100", func(t *testing.T) {
		got := COTIndexBar(150, 10)
		if !strings.Contains(got, "] 100") {
			t.Errorf("COTIndexBar(150, 10) should clamp to 100, got %q", got)
		}
	})
	t.Run("clamp below 0", func(t *testing.T) {
		got := COTIndexBar(-10, 10)
		if !strings.Contains(got, "] 0") {
			t.Errorf("COTIndexBar(-10, 10) should clamp to 0, got %q", got)
		}
	})
	t.Run("zero width defaults", func(t *testing.T) {
		got := COTIndexBar(50, 0)
		if got == "" {
			t.Error("COTIndexBar(50, 0) should not return empty string")
		}
	})
}

func TestConfluenceBar(t *testing.T) {
	tests := []struct {
		name      string
		score     float64
		wantLabel string
	}{
		{"bullish", 80, "BULLISH"},
		{"bearish", 20, "BEARISH"},
		{"lean bull", 60, "LEAN BULL"},
		{"lean bear", 40, "LEAN BEAR"},
		{"neutral", 50, "NEUTRAL"},
		{"edge bullish 70", 70, "BULLISH"},
		{"edge bearish 30", 30, "BEARISH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConfluenceBar(tt.score)
			if !strings.Contains(got, tt.wantLabel) {
				t.Errorf("ConfluenceBar(%v) = %q, want to contain %q", tt.score, got, tt.wantLabel)
			}
		})
	}
}

func TestImpactEmoji(t *testing.T) {
	tests := []struct {
		impact string
		want   string
	}{
		{"high", "[!!!]"},
		{"High", "[!!!]"},
		{"HIGH", "[!!!]"},
		{"medium", "[!!]"},
		{"Medium", "[!!]"},
		{"low", "[!]"},
		{"Low", "[!]"},
		{"other", "[-]"},
		{"", "[-]"},
		{"unknown", "[-]"},
	}

	for _, tt := range tests {
		t.Run(tt.impact, func(t *testing.T) {
			got := ImpactEmoji(tt.impact)
			if got != tt.want {
				t.Errorf("ImpactEmoji(%q) = %q, want %q", tt.impact, got, tt.want)
			}
		})
	}
}

func TestDirectionArrow(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  string
	}{
		{"positive", 1.5, ">>"},
		{"negative", -2.0, "<<"},
		{"zero", 0, "--"},
		{"small positive", 0.001, ">>"},
		{"small negative", -0.001, "<<"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DirectionArrow(tt.value)
			if got != tt.want {
				t.Errorf("DirectionArrow(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestSignalLabel(t *testing.T) {
	tests := []struct {
		signal string
		want   string
	}{
		{"BULLISH", "[BULL]"},
		{"bullish", "[BULL]"},
		{"BEARISH", "[BEAR]"},
		{"bearish", "[BEAR]"},
		{"NEUTRAL", "[NEUT]"},
		{"neutral", "[NEUT]"},
		{"EXTREME_BULL", "[!!BULL!!]"},
		{"EXTREME_BEAR", "[!!BEAR!!]"},
		{"custom", "[custom]"},
		{"", "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.signal, func(t *testing.T) {
			got := SignalLabel(tt.signal)
			if got != tt.want {
				t.Errorf("SignalLabel(%q) = %q, want %q", tt.signal, got, tt.want)
			}
		})
	}
}

func TestRankMedal(t *testing.T) {
	tests := []struct {
		pos  int
		want string
	}{
		{1, "#1"},
		{2, "#2"},
		{3, "#3"},
		{10, "#10"},
		{0, "#0"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := RankMedal(tt.pos)
			if got != tt.want {
				t.Errorf("RankMedal(%d) = %q, want %q", tt.pos, got, tt.want)
			}
		})
	}
}

func TestRankBar(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		maxWidth int
		wantLen  int
	}{
		{"full", 100, 20, 20},
		{"empty", 0, 20, 20},
		{"half", 50, 20, 20},
		{"zero width defaults", 50, 0, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RankBar(tt.score, tt.maxWidth)
			if len(got) != tt.wantLen {
				t.Errorf("RankBar(%v, %d) length = %d, want %d", tt.score, tt.maxWidth, len(got), tt.wantLen)
			}
		})
	}

	t.Run("full bar all pipes", func(t *testing.T) {
		got := RankBar(100, 10)
		if got != "||||||||||" {
			t.Errorf("RankBar(100, 10) = %q, want all pipes", got)
		}
	})
	t.Run("empty bar all dots", func(t *testing.T) {
		got := RankBar(0, 10)
		if got != ".........." {
			t.Errorf("RankBar(0, 10) = %q, want all dots", got)
		}
	})
	t.Run("clamp above 100", func(t *testing.T) {
		got := RankBar(150, 10)
		if got != "||||||||||" {
			t.Errorf("RankBar(150, 10) = %q, want all pipes (clamped)", got)
		}
	})
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"no truncation", "hello", 10, "hello"},
		{"truncated", "hello world", 8, "hello..."},
		{"exact length", "ab", 2, "ab"},
		{"maxLen 3 exact", "abcdef", 3, "abc"},
		{"maxLen 1", "abcdef", 1, "a"},
		{"empty string", "", 5, ""},
		{"maxLen equals len", "hello", 5, "hello"},
		{"truncate by one", "hello!", 5, "he..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"pad needed", "hi", 5, "hi   "},
		{"no pad needed", "hello", 3, "hello"},
		{"exact width", "abc", 3, "abc"},
		{"empty string", "", 4, "    "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadRight(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"pad needed", "hi", 5, "   hi"},
		{"no pad needed", "hello", 3, "hello"},
		{"exact width", "abc", 3, "abc"},
		{"empty string", "", 4, "    "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadLeft(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestSectionHeader(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"COT Analysis", "=== COT ANALYSIS ==="},
		{"hello", "=== HELLO ==="},
		{"", "===  ==="},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := SectionHeader(tt.title)
			if got != tt.want {
				t.Errorf("SectionHeader(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestSubHeader(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Positioning", "--- Positioning ---"},
		{"test", "--- test ---"},
		{"", "---  ---"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := SubHeader(tt.title)
			if got != tt.want {
				t.Errorf("SubHeader(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestBulletList(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  string
	}{
		{"multiple items", []string{"one", "two", "three"}, "  * one\n  * two\n  * three\n"},
		{"single item", []string{"only"}, "  * only\n"},
		{"empty list", []string{}, ""},
		{"nil list", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BulletList(tt.items)
			if got != tt.want {
				t.Errorf("BulletList(%v) = %q, want %q", tt.items, got, tt.want)
			}
		})
	}
}
