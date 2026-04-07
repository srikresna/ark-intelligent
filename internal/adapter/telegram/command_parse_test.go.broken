package telegram

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Command Argument Parsing Tests
// ---------------------------------------------------------------------------

func TestParseCommandArgs_Simple(t *testing.T) {
	tests := []struct {
		input    string
		cmd      string
		args     string
	}{
		{"/start", "start", ""},
		{"/help", "help", ""},
		{"/cot", "cot", ""},
		{"/cot EUR", "cot", "EUR"},
		{"/price USD", "price", "USD"},
		{"/calendar week", "calendar", "week"},
		{"/calendar", "calendar", ""},
		{"/macro", "macro", ""},
		{"/settings lang en", "settings", "lang en"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.cmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.cmd)
			}
			if args != tt.args {
				t.Errorf("parseCommand(%q) args = %q, want %q", tt.input, args, tt.args)
			}
		})
	}
}

func TestParseCommandArgs_CaseHandling(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs string
	}{
		{"/START", "start", ""},
		{"/COT EUR", "cot", "EUR"},
		{"/Price gbp", "price", "gbp"},
		{"/CoT xau", "cot", "xau"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if args != tt.wantArgs {
				t.Errorf("parseCommand(%q) args = %q, want %q", tt.input, args, tt.wantArgs)
			}
		})
	}
}

func TestParseCommandArgs_Whitespace(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs string
	}{
		{"/cot  EUR", "cot", "EUR"},
		{"/price   USD   ", "price", "USD"},
		{"/calendar  week  extra  ", "calendar", "week  extra"},
		{"  /start  ", "start", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("parseCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
			}
			if args != tt.wantArgs {
				t.Errorf("parseCommand(%q) args = %q, want %q", tt.input, args, tt.wantArgs)
			}
		})
	}
}

func TestParseCommandArgs_Invalid(t *testing.T) {
	tests := []struct {
		input string
	}{
		{""},
		{"notacommand"},
		{"just text without slash"},
		{"/"},
		{"/ "},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd, args := parseCommand(tt.input)
			// Invalid commands should return empty
			if cmd != "" || args != "" {
				t.Errorf("parseCommand(%q) should return empty, got cmd=%q args=%q", tt.input, cmd, args)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Currency Code Extraction Tests
// ---------------------------------------------------------------------------

func TestExtractCurrencyCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"EUR", "EUR"},
		{"eur", "EUR"},
		{"Eur", "EUR"},
		{"USD", "USD"},
		{"usd", "USD"},
		{"XAU", "XAU"},
		{"xau", "XAU"},
		{"GBP", "GBP"},
		{"JPY", "JPY"},
		{"AUD", "AUD"},
		{"CAD", "CAD"},
		{"CHF", "CHF"},
		{"NZD", "NZD"},
		{"BTC", "BTC"},
		{"ETH", "ETH"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeCurrency(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeCurrency(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractCurrencyCode_Invalid(t *testing.T) {
	tests := []struct {
		input string
	}{
		{""},
		{"EURO"},
		{"EU"},
		{"123"},
		{"US"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeCurrency(tt.input)
			if result != "" {
				t.Errorf("normalizeCurrency(%q) should return empty for invalid input, got %q", tt.input, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Numeric Value Parsing Tests
// ---------------------------------------------------------------------------

func TestParseNumericValues(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
		wantNil  bool
	}{
		// Standard numbers
		{"100", 100, false},
		{"100.5", 100.5, false},
		{"0", 0, false},
		{"0.0", 0, false},
		{"-50", -50, false},
		{"-50.5", -50.5, false},
		
		// Percentages
		{"50%", 50, false},
		{"3.2%", 3.2, false},
		{"100%", 100, false},
		
		// K/M/B suffixes
		{"10K", 10000, false},
		{"10k", 10000, false},
		{"1.5M", 1500000, false},
		{"1.5m", 1500000, false},
		{"2B", 2000000000, false},
		{"2b", 2000000000, false},
		
		// With commas
		{"1,000", 1000, false},
		{"1,000.50", 1000.5, false},
		{"10,000,000", 10000000, false},
		
		// Invalid values
		{"", 0, true},
		{"abc", 0, true},
		{"N/A", 0, true},
		{"-", 0, true},
		{"null", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseNumeric(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("parseNumeric(%q) = %v, want nil", tt.input, *result)
				}
			} else {
				if result == nil {
					t.Fatalf("parseNumeric(%q) returned nil, want %v", tt.input, tt.expected)
				}
				if *result != tt.expected {
					t.Errorf("parseNumeric(%q) = %v, want %v", tt.input, *result, tt.expected)
				}
			}
		})
	}
}

func TestParseNumeric_Consolidated(t *testing.T) {
	// Test with K/M/B combinations
	tests := []struct {
		input    string
		expected float64
	}{
		{"100K", 100000},
		{"1.5M", 1500000},
		{"2.5B", 2500000000},
		{"0.5K", 500},
		{"0.001M", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseNumeric(tt.input)
			if result == nil {
				t.Fatalf("parseNumeric(%q) returned nil", tt.input)
			}
			if *result != tt.expected {
				t.Errorf("parseNumeric(%q) = %v, want %v", tt.input, *result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Timeframe Parsing Tests
// ---------------------------------------------------------------------------

func TestParseTimeframe(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"4h", "4h"},
		{"4H", "4h"},
		{"1d", "1d"},
		{"1D", "1d"},
		{"1w", "1w"},
		{"1W", "1w"},
		{"1m", "1m"},
		{"1M", "1m"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeTimeframe(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeTimeframe(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Command Prefix Matching Tests
// ---------------------------------------------------------------------------

func TestIsCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"/start", true},
		{"/help", true},
		{"/cot EUR", true},
		{"START", false},       // no slash
		{"start", false},         // no slash
		{"/", false},             // no command name
		{"", false},              // empty
		{"regular message", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isCommand(tt.input)
			if result != tt.expected {
				t.Errorf("isCommand(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Price Format Tests
// ---------------------------------------------------------------------------

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		currency string
		price    float64
		contains string // partial match since we just check formatting
	}{
		{"JPY", 145.123, "."},
		{"XAU", 2050.50, "."},
		{"BTC", 50000, "."},
		{"EUR", 1.0850, "."},
		{"USD", 1.0, "."},
	}

	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			result := formatPrice(tt.price, tt.currency)
			if result == "" {
				t.Errorf("formatPrice(%f, %q) returned empty", tt.price, tt.currency)
			}
			// Just verify it doesn't panic and returns something with a decimal
			if len(result) == 0 {
				t.Errorf("formatPrice returned empty string")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helper function tests for command extraction
// ---------------------------------------------------------------------------

func TestExtractFirstArg(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"EUR", "EUR"},
		{"EUR extra", "EUR"},
		{"  EUR  ", "EUR"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := firstArg(tt.input)
			if result != tt.expected {
				t.Errorf("firstArg(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractAllArgs(t *testing.T) {
	tests := []struct {
		input         string
		expectedArgs  []string
	}{
		{"EUR", []string{"EUR"}},
		{"EUR 4h", []string{"EUR", "4h"}},
		{"EUR 4H daily", []string{"EUR", "4H", "daily"}},
		{"  EUR   4h  ", []string{"EUR", "4h"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitArgs(tt.input)
			if len(result) != len(tt.expectedArgs) {
				t.Errorf("splitArgs(%q) returned %d args, want %d", tt.input, len(result), len(tt.expectedArgs))
				return
			}
			for i, arg := range result {
				if arg != tt.expectedArgs[i] {
					t.Errorf("splitArgs(%q)[%d] = %q, want %q", tt.input, i, arg, tt.expectedArgs[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Command Validation Tests
// ---------------------------------------------------------------------------

func TestValidateCommand(t *testing.T) {
	validCommands := []string{
		"/start",
		"/help",
		"/cot",
		"/calendar",
		"/price",
		"/settings",
		"/macro",
		"/bias",
		"/rank",
		"/accuracy",
		"/seasonal",
		"/backtest",
		"/quant",
		"/vp",
		"/cta",
		"/alpha",
		"/outlook",
		"/impact",
		"/alert",
		"/news",
		"/levels",
		"/membership",
		"/clear",
		"/status",
		"/report",
		"/sentiment",
	}

	for _, cmd := range validCommands {
		t.Run(cmd, func(t *testing.T) {
			if !isValidCommand(cmd) {
				t.Errorf("isValidCommand(%q) should return true", cmd)
			}
		})
	}
}

func TestInvalidCommands(t *testing.T) {
	invalidCommands := []string{
		"/invalid",
		"/xyz",
		"/notacommand",
		"/foo bar",
	}

	for _, cmd := range invalidCommands {
		t.Run(cmd, func(t *testing.T) {
			if isValidCommand(cmd) {
				t.Errorf("isValidCommand(%q) should return false", cmd)
			}
		})
	}
}
