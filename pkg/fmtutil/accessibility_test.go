package fmtutil

import "testing"

func TestSentimentLabel(t *testing.T) {
	got := SentimentLabel(true, "Bullish", "Bearish")
	want := "🟢 Bullish"
	if got != want {
		t.Errorf("SentimentLabel(true) = %q, want %q", got, want)
	}
	got = SentimentLabel(false, "Bullish", "Bearish")
	want = "🔴 Bearish"
	if got != want {
		t.Errorf("SentimentLabel(false) = %q, want %q", got, want)
	}
}

func TestSignalDot(t *testing.T) {
	tests := []struct {
		score     float64
		threshold float64
		want      string
	}{
		{20, 15, "🟢 Bullish"},
		{-20, 15, "🔴 Bearish"},
		{5, 15, "⚪ Neutral"},
		{0, 15, "⚪ Neutral"},
	}
	for _, tt := range tests {
		got := SignalDot(tt.score, tt.threshold, "Bullish", "Bearish", "Neutral")
		if got != tt.want {
			t.Errorf("SignalDot(%v, %v) = %q, want %q", tt.score, tt.threshold, got, tt.want)
		}
	}
}

func TestBullBearNeutral(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"BULLISH", "🟢 Bullish"},
		{"BEARISH", "🔴 Bearish"},
		{"", "⚪ Neutral"},
		{"MIXED", "⚪ Neutral"},
	}
	for _, tt := range tests {
		if got := BullBearNeutral(tt.input); got != tt.want {
			t.Errorf("BullBearNeutral(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChangeLabel(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{1.5, "🟢 Up"},
		{-0.5, "🔴 Down"},
		{0, "⚪ Flat"},
	}
	for _, tt := range tests {
		if got := ChangeLabel(tt.v); got != tt.want {
			t.Errorf("ChangeLabel(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestConfidenceLabel(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"HIGH", "🟢 High"},
		{"MEDIUM", "🟡 Medium"},
		{"LOW", "🔴 Low"},
		{"UNKNOWN", ""},
	}
	for _, tt := range tests {
		if got := ConfidenceLabel(tt.level); got != tt.want {
			t.Errorf("ConfidenceLabel(%q) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestStabilityLabel(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{80, "🟢 Stable"},
		{60, "🟡 Moderate"},
		{30, "🔴 Unstable"},
	}
	for _, tt := range tests {
		if got := StabilityLabel(tt.pct); got != tt.want {
			t.Errorf("StabilityLabel(%v) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}

func TestAnomalyLabel(t *testing.T) {
	want1 := "🔴 Anomaly"
	if got := AnomalyLabel(true); got != want1 {
		t.Errorf("AnomalyLabel(true) = %q, want %q", got, want1)
	}
	want2 := "🟢 Normal"
	if got := AnomalyLabel(false); got != want2 {
		t.Errorf("AnomalyLabel(false) = %q, want %q", got, want2)
	}
}
