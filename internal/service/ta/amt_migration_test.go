package ta

import (
	"strings"
	"testing"
	"time"
)

func TestClassifyMigration_Basic(t *testing.T) {
	now := time.Now().UTC().Truncate(24 * time.Hour)

	var bars []OHLCV
	// Create 5 days of bars with upward-drifting POC.
	for d := 0; d < 5; d++ {
		dayStart := now.Add(time.Duration(-4+d) * 24 * time.Hour)
		basePrice := 1.1000 + float64(d)*0.0020 // POC drifting up
		for i := 0; i < 16; i++ {
			t := dayStart.Add(time.Duration(i) * 30 * time.Minute)
			bars = append(bars, OHLCV{
				Date:   t,
				Open:   basePrice + float64(i)*0.00003,
				High:   basePrice + 0.0010,
				Low:    basePrice - 0.0010,
				Close:  basePrice + float64(i)*0.00003,
				Volume: 100,
			})
		}
	}

	r := ClassifyMigration(bars, 5)
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if len(r.Days) < 2 {
		t.Fatal("expected at least 2 days")
	}
	if r.MigrationScore <= 0 {
		t.Errorf("expected positive migration score for upward drift, got %.1f", r.MigrationScore)
	}
	if r.NetDirection != MigrationUp {
		t.Errorf("expected UP migration, got %s", r.NetDirection)
	}
	if r.MigrationChart == "" {
		t.Error("expected non-empty migration chart")
	}
	if r.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestClassifyMigration_NilOnEmpty(t *testing.T) {
	r := ClassifyMigration(nil, 5)
	if r != nil {
		t.Error("expected nil for empty bars")
	}
}

func TestClassifyMigration_NilOnOneDay(t *testing.T) {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	bars := makeTrendingBars(now, 1.1000, 1.1100, 16)
	r := ClassifyMigration(bars, 5)
	if r != nil {
		t.Error("expected nil for single day")
	}
}

func TestComputeVAOverlap(t *testing.T) {
	a := ValueArea{VAH: 1.1100, VAL: 1.1000}
	b := ValueArea{VAH: 1.1080, VAL: 1.0980}

	overlap := computeVAOverlap(a, b)
	if overlap < 0.5 || overlap > 1.0 {
		t.Errorf("expected significant overlap, got %.2f", overlap)
	}

	// No overlap
	c := ValueArea{VAH: 1.1200, VAL: 1.1110}
	overlap2 := computeVAOverlap(a, c)
	if overlap2 != 0 {
		t.Errorf("expected zero overlap, got %.2f", overlap2)
	}
}

func TestBuildMigrationChart(t *testing.T) {
	now := time.Now().UTC()
	days := []DayMigration{
		{Date: now.Add(-2 * 24 * time.Hour), VA: ValueArea{POC: 1.1000}},
		{Date: now.Add(-1 * 24 * time.Hour), VA: ValueArea{POC: 1.1020}},
		{Date: now, VA: ValueArea{POC: 1.1050}},
	}

	chart := buildMigrationChart(days)
	if chart == "" {
		t.Error("expected non-empty chart")
	}
	if !strings.Contains(chart, "◆") {
		t.Error("expected diamond marker in chart")
	}
}

func TestMGIAnalysis(t *testing.T) {
	prevVA := ValueArea{
		POC: 1.1050,
		VAH: 1.1080,
		VAL: 1.1020,
	}

	now := time.Now().UTC().Truncate(24 * time.Hour)
	// Create bars that spend time near POC.
	var bars []OHLCV
	for i := 0; i < 20; i++ {
		bars = append(bars, OHLCV{
			Date:   now.Add(time.Duration(i) * 30 * time.Minute),
			Open:   1.1048,
			High:   1.1052,
			Low:    1.1048,
			Close:  1.1050,
			Volume: 100,
		})
	}

	levels := analyseMGI(bars, prevVA)
	if len(levels) != 3 {
		t.Fatalf("expected 3 MGI levels, got %d", len(levels))
	}

	// POC should be accepted (all bars near POC).
	pocLevel := levels[0]
	if !pocLevel.Accepted {
		t.Error("expected POC to be accepted when bars cluster near it")
	}
}
