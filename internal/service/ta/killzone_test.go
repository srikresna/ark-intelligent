package ta

import (
	"testing"
	"time"
)

// makeUTC creates a UTC time with the given hour and minute for testing.
func makeUTC(hour, minute int) time.Time {
	return time.Date(2026, 4, 1, hour, minute, 0, 0, time.UTC)
}

// TestClassifyKillzone_Asia verifies Asia session detection.
func TestClassifyKillzone_Asia(t *testing.T) {
	result := ClassifyKillzone(makeUTC(2, 0))
	if result.ActiveKillzone != "ASIA" {
		t.Errorf("expected ASIA at 02:00 UTC, got %q", result.ActiveKillzone)
	}
	if !result.IsActive {
		t.Error("expected IsActive=true at 02:00 UTC")
	}
}

// TestClassifyKillzone_LondonOpen verifies London Open killzone detection.
func TestClassifyKillzone_LondonOpen(t *testing.T) {
	result := ClassifyKillzone(makeUTC(8, 30))
	if result.ActiveKillzone != "LONDON_OPEN" {
		t.Errorf("expected LONDON_OPEN at 08:30 UTC, got %q", result.ActiveKillzone)
	}
	if !result.IsActive {
		t.Error("expected IsActive=true at 08:30 UTC")
	}
}

// TestClassifyKillzone_NYOpen verifies NY Open killzone detection.
func TestClassifyKillzone_NYOpen(t *testing.T) {
	result := ClassifyKillzone(makeUTC(13, 0))
	if result.ActiveKillzone != "NY_OPEN" {
		t.Errorf("expected NY_OPEN at 13:00 UTC, got %q", result.ActiveKillzone)
	}
	if !result.IsActive {
		t.Error("expected IsActive=true at 13:00 UTC")
	}
}

// TestClassifyKillzone_LondonClose verifies London Close killzone detection.
func TestClassifyKillzone_LondonClose(t *testing.T) {
	result := ClassifyKillzone(makeUTC(15, 30))
	if result.ActiveKillzone != "LONDON_CLOSE" {
		t.Errorf("expected LONDON_CLOSE at 15:30 UTC, got %q", result.ActiveKillzone)
	}
}

// TestClassifyKillzone_NYClose verifies NY Close killzone detection.
func TestClassifyKillzone_NYClose(t *testing.T) {
	result := ClassifyKillzone(makeUTC(21, 0))
	if result.ActiveKillzone != "NY_CLOSE" {
		t.Errorf("expected NY_CLOSE at 21:00 UTC, got %q", result.ActiveKillzone)
	}
}

// TestClassifyKillzone_OffHours verifies off-hours classification.
func TestClassifyKillzone_OffHours(t *testing.T) {
	for _, h := range []int{5, 6, 11, 18, 23} {
		result := ClassifyKillzone(makeUTC(h, 0))
		if result.ActiveKillzone != "OFF_HOURS" {
			t.Errorf("expected OFF_HOURS at %02d:00 UTC, got %q", h, result.ActiveKillzone)
		}
		if result.IsActive {
			t.Errorf("expected IsActive=false at %02d:00 UTC", h)
		}
	}
}

// TestClassifyKillzone_IntersessionOverlap verifies London-NY overlap detection.
func TestClassifyKillzone_IntersessionOverlap(t *testing.T) {
	// 13:00-16:00 UTC is the overlap
	for _, h := range []int{13, 14, 15} {
		result := ClassifyKillzone(makeUTC(h, 30))
		if !result.IntersessionOverlap {
			t.Errorf("expected IntersessionOverlap=true at %02d:30 UTC", h)
		}
	}
	// Outside overlap
	result := ClassifyKillzone(makeUTC(12, 0))
	if result.IntersessionOverlap {
		t.Error("expected IntersessionOverlap=false at 12:00 UTC")
	}
}

// TestClassifyKillzone_BoundaryExact verifies exact boundary handling.
func TestClassifyKillzone_BoundaryExact(t *testing.T) {
	// Start boundary: 07:00 UTC = London Open
	r := ClassifyKillzone(makeUTC(7, 0))
	if r.ActiveKillzone != "LONDON_OPEN" {
		t.Errorf("expected LONDON_OPEN at 07:00 UTC, got %q", r.ActiveKillzone)
	}

	// End boundary: 10:00 UTC = Off hours (exclusive end)
	r = ClassifyKillzone(makeUTC(10, 0))
	if r.ActiveKillzone != "OFF_HOURS" {
		t.Errorf("expected OFF_HOURS at 10:00 UTC, got %q", r.ActiveKillzone)
	}

	// Midnight: 00:00 UTC = Asia
	r = ClassifyKillzone(makeUTC(0, 0))
	if r.ActiveKillzone != "ASIA" {
		t.Errorf("expected ASIA at 00:00 UTC, got %q", r.ActiveKillzone)
	}
}

// TestNextKillzoneInfo verifies reasonable values for next KZ timing.
func TestNextKillzoneInfo(t *testing.T) {
	// At 06:00 UTC (between Asia end and London Open)
	name, minutes := NextKillzoneInfo(makeUTC(6, 0))
	if name == "" {
		t.Error("expected non-empty next killzone name")
	}
	if minutes <= 0 || minutes > 1440 {
		t.Errorf("expected minutes in (0, 1440], got %d", minutes)
	}
	// Should be LONDON_OPEN starting at 07:00, so 60 minutes away
	if name != "LONDON_OPEN" {
		t.Errorf("expected next KZ to be LONDON_OPEN at 06:00 UTC, got %q", name)
	}
	if minutes != 60 {
		t.Errorf("expected 60 minutes to LONDON_OPEN, got %d", minutes)
	}
}

// TestKillzoneIntegration verifies Killzone is populated in ComputeSnapshot.
func TestKillzoneIntegration(t *testing.T) {
	// Build a minimal 55-bar dataset
	closes := []float64{
		1.1000, 1.1020, 1.1045, 1.1070, 1.1100, 1.1130, 1.1150,
		1.1130, 1.1105, 1.1080, 1.1060, 1.1040,
		1.1065, 1.1095, 1.1125, 1.1160, 1.1200, 1.1240, 1.1270,
		1.1250, 1.1220, 1.1195, 1.1170, 1.1150,
		1.1175, 1.1210, 1.1250, 1.1295, 1.1340, 1.1380, 1.1420,
		1.1390, 1.1360, 1.1330, 1.1305, 1.1285,
		1.1310, 1.1355, 1.1400, 1.1450, 1.1500, 1.1540, 1.1580,
		1.1550, 1.1515, 1.1480, 1.1455, 1.1435,
		1.1460, 1.1500, 1.1545, 1.1590, 1.1640, 1.1685, 1.1720,
	}
	n := len(closes)
	bars := make([]OHLCV, n)
	for i, c := range closes {
		bars[n-1-i] = OHLCV{Open: c - 0.0005, High: c + 0.0015, Low: c - 0.0015, Close: c}
	}
	engine := NewEngine()
	snap := engine.ComputeSnapshot(bars)

	if snap == nil {
		t.Fatal("ComputeSnapshot returned nil")
	}
	if snap.Killzone == nil {
		t.Fatal("expected Killzone to be populated in IndicatorSnapshot")
	}

	// Validate fields
	valid := map[string]bool{
		"ASIA": true, "LONDON_OPEN": true, "NY_OPEN": true,
		"LONDON_CLOSE": true, "NY_CLOSE": true, "OFF_HOURS": true,
	}
	if !valid[snap.Killzone.ActiveKillzone] {
		t.Errorf("unexpected ActiveKillzone: %q", snap.Killzone.ActiveKillzone)
	}
	if snap.Killzone.MinutesUntilNext < 0 || snap.Killzone.MinutesUntilNext > 1440 {
		t.Errorf("MinutesUntilNext %d out of [0, 1440]", snap.Killzone.MinutesUntilNext)
	}
}
