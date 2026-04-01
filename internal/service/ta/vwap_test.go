package ta

import (
	"math"
	"testing"
	"time"
)

// helper: create bars with uniform price and volume (newest-first).
func uniformBars(n int, price, volume float64) []OHLCV {
	bars := make([]OHLCV, n)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		bars[i] = OHLCV{
			Date:   base.Add(-time.Duration(i) * time.Hour),
			Open:   price,
			High:   price,
			Low:    price,
			Close:  price,
			Volume: volume,
		}
	}
	return bars
}

// helper: approximately equal.
func approxEq(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestVWAPBasic_UniformPrice(t *testing.T) {
	// Uniform price + volume → VWAP = price, bands collapse to price.
	bars := uniformBars(20, 1.0800, 100)
	res := CalcVWAP(bars, "TEST")
	if res == nil {
		t.Fatal("expected non-nil VWAPResult")
	}
	if !approxEq(res.VWAP, 1.0800, 1e-6) {
		t.Errorf("VWAP = %.6f, want ~1.0800", res.VWAP)
	}
	// With uniform price, all deviations are zero → bands == VWAP.
	if !approxEq(res.Band1Upper, res.VWAP, 1e-6) {
		t.Errorf("Band1Upper = %.6f, want ~VWAP %.6f (uniform data)", res.Band1Upper, res.VWAP)
	}
	if res.Position != "AT" {
		t.Errorf("Position = %s, want AT (price == VWAP)", res.Position)
	}
	if res.BarsUsed != 20 {
		t.Errorf("BarsUsed = %d, want 20", res.BarsUsed)
	}
}

func TestVWAPBands_KnownData(t *testing.T) {
	// 5 bars with known prices and volumes, verify VWAP and that bands are sensible.
	// Bars: newest-first.
	base := time.Date(2026, 4, 1, 16, 0, 0, 0, time.UTC)
	bars := []OHLCV{
		{Date: base, Open: 1.10, High: 1.12, Low: 1.09, Close: 1.11, Volume: 200},
		{Date: base.Add(-1 * time.Hour), Open: 1.08, High: 1.10, Low: 1.07, Close: 1.09, Volume: 150},
		{Date: base.Add(-2 * time.Hour), Open: 1.06, High: 1.08, Low: 1.05, Close: 1.07, Volume: 100},
		{Date: base.Add(-3 * time.Hour), Open: 1.04, High: 1.06, Low: 1.03, Close: 1.05, Volume: 120},
		{Date: base.Add(-4 * time.Hour), Open: 1.03, High: 1.05, Low: 1.02, Close: 1.04, Volume: 130},
	}

	res := CalcVWAP(bars, "TEST")
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	// VWAP should be above simple-avg-TP because higher-priced bars carry more volume.
	simpleAvgTP := (1.0367 + 1.0467 + 1.0667 + 1.0867 + 1.1067) / 5.0
	if res.VWAP <= simpleAvgTP {
		t.Errorf("VWAP = %.6f, expected > simple-avg-TP %.4f (heavy volume at high prices)", res.VWAP, simpleAvgTP)
	}
	// Bands should be ordered: Band2Lower < Band1Lower < VWAP < Band1Upper < Band2Upper
	if !(res.Band2Lower < res.Band1Lower && res.Band1Lower < res.VWAP &&
		res.VWAP < res.Band1Upper && res.Band1Upper < res.Band2Upper) {
		t.Errorf("Bands not properly ordered: B2L=%.6f B1L=%.6f VWAP=%.6f B1U=%.6f B2U=%.6f",
			res.Band2Lower, res.Band1Lower, res.VWAP, res.Band1Upper, res.Band2Upper)
	}
	// BarsUsed should be 5.
	if res.BarsUsed != 5 {
		t.Errorf("BarsUsed = %d, want 5", res.BarsUsed)
	}
}

func TestVWAPPosition(t *testing.T) {
	// Build bars where current price is clearly above VWAP.
	// Start low, then jump high → VWAP will be somewhere in between.
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	bars := make([]OHLCV, 10)
	for i := 0; i < 10; i++ {
		p := 1.0500 // low base price
		if i < 3 {
			p = 1.1000 // latest 3 bars are high
		}
		bars[i] = OHLCV{
			Date:   base.Add(-time.Duration(i) * time.Hour),
			Open:   p,
			High:   p + 0.002,
			Low:    p - 0.002,
			Close:  p,
			Volume: 100,
		}
	}
	res := CalcVWAP(bars, "TEST")
	if res == nil {
		t.Fatal("expected non-nil")
	}
	if res.Position != "ABOVE" {
		t.Errorf("Position = %s, want ABOVE", res.Position)
	}
	if res.Deviation <= 0 {
		t.Errorf("Deviation = %.4f, want > 0", res.Deviation)
	}
}

func TestVWAPZeroVolume(t *testing.T) {
	// All bars have zero volume → should still compute (equal-weight fallback).
	bars := uniformBars(10, 1.0500, 0)
	res := CalcVWAP(bars, "TEST")
	if res == nil {
		t.Fatal("expected non-nil (zero-volume fallback to equal weights)")
	}
	if !approxEq(res.VWAP, 1.0500, 1e-6) {
		t.Errorf("VWAP = %.6f, want ~1.0500 (equal weight)", res.VWAP)
	}
}

func TestVWAPSet_MultiAnchor(t *testing.T) {
	// Build 30 hourly bars spanning multiple days for daily/weekly anchors.
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC) // Wednesday
	bars := make([]OHLCV, 100)
	for i := 0; i < 100; i++ {
		p := 1.0800 + float64(i%5)*0.001
		bars[i] = OHLCV{
			Date:   base.Add(-time.Duration(i) * time.Hour),
			Open:   p,
			High:   p + 0.003,
			Low:    p - 0.003,
			Close:  p,
			Volume: 100,
		}
	}
	set := CalcVWAPSet(bars)
	if set == nil {
		t.Fatal("expected non-nil VWAPSet")
	}
	if set.Daily == nil {
		t.Error("expected non-nil Daily VWAP")
	} else if set.Daily.AnchorType != "DAILY" {
		t.Errorf("Daily.AnchorType = %s, want DAILY", set.Daily.AnchorType)
	}
	if set.Weekly == nil {
		t.Error("expected non-nil Weekly VWAP")
	} else if set.Weekly.AnchorType != "WEEKLY" {
		t.Errorf("Weekly.AnchorType = %s, want WEEKLY", set.Weekly.AnchorType)
	}
}

func TestVWAPAnchored_InsufficientBars(t *testing.T) {
	bars := uniformBars(1, 1.05, 100)
	res := CalcVWAP(bars, "TEST")
	if res != nil {
		t.Error("expected nil for single bar")
	}

	res2 := CalcVWAPAnchored(nil, 0, "TEST")
	if res2 != nil {
		t.Error("expected nil for nil bars")
	}
}
