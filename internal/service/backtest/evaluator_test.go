package backtest

import (
	"testing"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// TestBuildHistoryWindow tests the bootstrap's history windowing logic.
func TestBuildHistoryWindow(t *testing.T) {
	// Create 10 weeks of COT records in oldest-first order
	base := time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC) // Tuesday
	var records []domain.COTRecord
	for i := 0; i < 10; i++ {
		records = append(records, domain.COTRecord{
			ReportDate: base.AddDate(0, 0, 7*i),
		})
	}

	tests := []struct {
		name      string
		target    time.Time
		maxWeeks  int
		wantLen   int
		wantFirst time.Time // first record in window (oldest)
		wantLast  time.Time // last record in window (most recent)
	}{
		{
			"full window from middle",
			base.AddDate(0, 0, 7*5), // week 5
			4,
			4,
			base.AddDate(0, 0, 7*2), // weeks 2-5
			base.AddDate(0, 0, 7*5),
		},
		{
			"target before any records",
			base.AddDate(0, 0, -7),
			8,
			0,
			time.Time{},
			time.Time{},
		},
		{
			"target after all records",
			base.AddDate(0, 0, 7*20),
			8,
			8,
			base.AddDate(0, 0, 7*2), // 10 records, keep last 8
			base.AddDate(0, 0, 7*9),
		},
		{
			"small window",
			base.AddDate(0, 0, 7*2), // week 2
			8,
			3, // weeks 0, 1, 2 — only 3 available
			base,
			base.AddDate(0, 0, 7*2),
		},
		{
			"exact match on last record",
			base.AddDate(0, 0, 7*9), // last record
			4,
			4,
			base.AddDate(0, 0, 7*6),
			base.AddDate(0, 0, 7*9),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			window := buildHistoryWindow(records, tt.target, tt.maxWeeks)
			if len(window) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(window), tt.wantLen)
			}
			if tt.wantLen > 0 {
				if !window[0].ReportDate.Equal(tt.wantFirst) {
					t.Errorf("first = %v, want %v", window[0].ReportDate, tt.wantFirst)
				}
				if !window[len(window)-1].ReportDate.Equal(tt.wantLast) {
					t.Errorf("last = %v, want %v", window[len(window)-1].ReportDate, tt.wantLast)
				}
			}
		})
	}
}

// TestComputeStatsMixedEvaluation tests that stats correctly handle signals
// where some are evaluated and some are not.
func TestComputeStatsMixedEvaluation(t *testing.T) {
	now := time.Now()
	signals := []domain.PersistedSignal{
		// Fully evaluated
		{
			Direction: "BULLISH", Strength: 5, Confidence: 80,
			EntryPrice: 1.08, ReportDate: now.Add(-60 * 24 * time.Hour),
			Return1W: 0.5, Outcome1W: domain.OutcomeWin,
			Return2W: 1.0, Outcome2W: domain.OutcomeWin,
			Return4W: 2.0, Outcome4W: domain.OutcomeWin,
		},
		// Only 1W evaluated
		{
			Direction: "BEARISH", Strength: 3, Confidence: 60,
			EntryPrice: 1.08, ReportDate: now.Add(-10 * 24 * time.Hour),
			Return1W: -0.3, Outcome1W: domain.OutcomeWin,
			Outcome2W: "", Outcome4W: "",
		},
		// Not evaluated at all (too recent or no price data)
		{
			Direction: "BULLISH", Strength: 4, Confidence: 70,
			EntryPrice: 1.08, ReportDate: now.Add(-3 * 24 * time.Hour),
			Outcome1W: "", Outcome2W: "", Outcome4W: "",
		},
	}

	stats := computeStats(signals, "MIXED")

	if stats.TotalSignals != 3 {
		t.Errorf("TotalSignals = %d, want 3", stats.TotalSignals)
	}
	if stats.Evaluated1W != 2 {
		t.Errorf("Evaluated1W = %d, want 2", stats.Evaluated1W)
	}
	if stats.Evaluated2W != 1 {
		t.Errorf("Evaluated2W = %d, want 1", stats.Evaluated2W)
	}
	if stats.Evaluated4W != 1 {
		t.Errorf("Evaluated4W = %d, want 1", stats.Evaluated4W)
	}
	if stats.Evaluated != 2 {
		t.Errorf("Evaluated (primary) = %d, want 2", stats.Evaluated)
	}

	// 1W: 2 wins / 2 evaluated = 100%
	if stats.WinRate1W != 100 {
		t.Errorf("WinRate1W = %.2f, want 100", stats.WinRate1W)
	}

	// Strength breakdown should only count evaluated signals
	// Signal 1: strength 5, evaluated, win → highTotal=1, highWins=1
	// Signal 2: strength 3, evaluated, win → lowTotal=1, lowWins=1
	// Signal 3: strength 4, NOT evaluated → excluded from strength breakdown
	if stats.HighStrengthCount != 1 {
		t.Errorf("HighStrengthCount = %d, want 1 (only evaluated high-strength)", stats.HighStrengthCount)
	}
	if stats.LowStrengthCount != 1 {
		t.Errorf("LowStrengthCount = %d, want 1 (only evaluated low-strength)", stats.LowStrengthCount)
	}
	if stats.HighStrengthWinRate != 100 {
		t.Errorf("HighStrengthWinRate = %.2f, want 100", stats.HighStrengthWinRate)
	}
	if stats.LowStrengthWinRate != 100 {
		t.Errorf("LowStrengthWinRate = %.2f, want 100", stats.LowStrengthWinRate)
	}
}

// TestComputeStatsPerHorizonCounts verifies per-horizon evaluation counts
// differ when signals have different ages.
func TestComputeStatsPerHorizonCounts(t *testing.T) {
	signals := []domain.PersistedSignal{
		makeSignal("BULLISH", 4, 70, 0.5, 1.2, 2.1), // all 3 horizons
		makeSignal("BULLISH", 3, 60, -0.3, 0.8, 0),  // 1W+2W only (4W return=0 → no outcome)
		makeSignal("BEARISH", 5, 80, -0.7, 0, 0),    // 1W only
	}

	stats := computeStats(signals, "HORIZONS")

	if stats.Evaluated1W != 3 {
		t.Errorf("Evaluated1W = %d, want 3", stats.Evaluated1W)
	}
	if stats.Evaluated2W != 2 {
		t.Errorf("Evaluated2W = %d, want 2", stats.Evaluated2W)
	}
	if stats.Evaluated4W != 1 {
		t.Errorf("Evaluated4W = %d, want 1", stats.Evaluated4W)
	}
}
