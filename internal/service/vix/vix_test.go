package vix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// makeVXCSV builds a minimal VX_EOD.csv for testing.
func makeVXCSV(date string, contracts []struct{ symbol, settle string }) string {
	var sb strings.Builder
	sb.WriteString("Trade Date,Futures,Open,High,Low,Close,Settle,Change,%Change,Volume,EFP,Open Interest\n")
	for _, c := range contracts {
		sb.WriteString(date + "," + c.symbol + ",0,0,0,0," + c.settle + ",0,0,0,0,0\n")
	}
	return sb.String()
}

// makeVIXCSV builds a minimal VIX_EOD.csv for testing.
func makeVIXCSV(close string) string {
	return "Date,Open,High,Low,Close\n2026-03-31,18.0,19.0,17.5," + close + "\n"
}

// serverWith creates a test HTTP server that serves different responses based on URL path.
func serverWith(vixClose, vvixClose, vxCSV string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "VVIX"):
			w.Header().Set("Content-Type", "text/csv")
			w.Write([]byte("Date,Open,High,Low,Close\n2026-03-31,85,90,83," + vvixClose + "\n"))
		case strings.Contains(r.URL.Path, "VX_EOD"):
			w.Header().Set("Content-Type", "text/csv")
			w.Write([]byte(vxCSV))
		case strings.Contains(r.URL.Path, "VIX_EOD"):
			w.Header().Set("Content-Type", "text/csv")
			w.Write([]byte(makeVIXCSV(vixClose)))
		default:
			http.NotFound(w, r)
		}
	}))
}

// ---------------------------------------------------------------------------
// parseVXSymbolExpiry tests
// ---------------------------------------------------------------------------

// TestParseVXSymbolExpiry_ValidSymbol verifies standard contract symbol parsing.
func TestParseVXSymbolExpiry_ValidSymbol(t *testing.T) {
	// /VXK26 = May 2026
	result := parseVXSymbolExpiry("/VXK26")
	if result.IsZero() {
		t.Fatal("expected non-zero expiry for /VXK26")
	}
	if result.Month() != time.May {
		t.Errorf("expected May, got %v", result.Month())
	}
	if result.Year() != 2026 {
		t.Errorf("expected 2026, got %d", result.Year())
	}
}

// TestParseVXSymbolExpiry_InvalidSymbol verifies graceful zero return for bad input.
func TestParseVXSymbolExpiry_InvalidSymbol(t *testing.T) {
	result := parseVXSymbolExpiry("")
	if !result.IsZero() {
		t.Error("expected zero expiry for empty symbol")
	}
	result = parseVXSymbolExpiry("/VXQ") // too short
	if !result.IsZero() {
		t.Error("expected zero expiry for too-short symbol")
	}
	result = parseVXSymbolExpiry("/VXZZ") // bad month code
	if !result.IsZero() {
		t.Error("expected zero expiry for invalid month code ZZ")
	}
}

// ---------------------------------------------------------------------------
// classifyRegime tests
// ---------------------------------------------------------------------------

// TestClassifyRegime_ExtremeFear verifies EXTREME_FEAR classification.
func TestClassifyRegime_ExtremeFear(t *testing.T) {
	ts := &VIXTermStructure{Spot: 25.0, M1: 22.0, Backwardation: true}
	classifyRegime(ts)
	if ts.Regime != "EXTREME_FEAR" {
		t.Errorf("expected EXTREME_FEAR (Spot>M1*1.10), got %q", ts.Regime)
	}
}

// TestClassifyRegime_Fear verifies FEAR classification (moderate backwardation).
func TestClassifyRegime_Fear(t *testing.T) {
	ts := &VIXTermStructure{Spot: 22.0, M1: 21.0, Backwardation: true}
	classifyRegime(ts)
	if ts.Regime != "FEAR" {
		t.Errorf("expected FEAR, got %q", ts.Regime)
	}
}

// TestClassifyRegime_Contango verifies contango regime classification.
func TestClassifyRegime_Contango(t *testing.T) {
	ts := &VIXTermStructure{Spot: 17.0, M1: 18.0, M2: 20.5, SlopePct: 13.9}
	classifyRegime(ts)
	if ts.Regime != "RISK_ON_COMPLACENT" {
		t.Errorf("expected RISK_ON_COMPLACENT (slope=13.9), got %q", ts.Regime)
	}
}

// TestClassifyRegime_Normal verifies RISK_ON_NORMAL classification.
func TestClassifyRegime_Normal(t *testing.T) {
	ts := &VIXTermStructure{Spot: 17.0, M1: 18.0, M2: 19.0, SlopePct: 5.6}
	classifyRegime(ts)
	if ts.Regime != "RISK_ON_NORMAL" {
		t.Errorf("expected RISK_ON_NORMAL (slope=5.6), got %q", ts.Regime)
	}
}

// TestClassifyRegime_Elevated verifies ELEVATED flat structure.
func TestClassifyRegime_Elevated(t *testing.T) {
	ts := &VIXTermStructure{Spot: 17.0, M1: 18.0, M2: 18.5, SlopePct: 2.8}
	classifyRegime(ts)
	if ts.Regime != "ELEVATED" {
		t.Errorf("expected ELEVATED (slope=2.8), got %q", ts.Regime)
	}
}

// ---------------------------------------------------------------------------
// computeDerivedFields tests
// ---------------------------------------------------------------------------

// TestComputeDerivedFields_ContangoDetection verifies contango flag.
func TestComputeDerivedFields_ContangoDetection(t *testing.T) {
	ts := &VIXTermStructure{Spot: 17.0, M1: 18.5, M2: 20.0}
	computeDerivedFields(ts)
	if !ts.Contango {
		t.Error("expected Contango=true when M1 > Spot")
	}
	if ts.Backwardation {
		t.Error("expected Backwardation=false when M1 > Spot")
	}
	expectedSlope := (20.0 - 18.5) / 18.5 * 100
	if ts.SlopePct < expectedSlope-0.01 || ts.SlopePct > expectedSlope+0.01 {
		t.Errorf("expected SlopePct=%.2f, got %.2f", expectedSlope, ts.SlopePct)
	}
}

// TestComputeDerivedFields_BackwardationDetection verifies backwardation flag.
func TestComputeDerivedFields_BackwardationDetection(t *testing.T) {
	ts := &VIXTermStructure{Spot: 25.0, M1: 22.0, M2: 21.0}
	computeDerivedFields(ts)
	if !ts.Backwardation {
		t.Error("expected Backwardation=true when M1 < Spot")
	}
	if ts.Contango {
		t.Error("expected Contango=false when M1 < Spot")
	}
}

// ---------------------------------------------------------------------------
// Cache tests
// ---------------------------------------------------------------------------

// TestCacheInvalidate verifies that Invalidate clears the cached data.
func TestCacheInvalidate(t *testing.T) {
	c := NewCache()
	// Manually inject fake data
	c.mu.Lock()
	c.data = &VIXTermStructure{Available: true, Spot: 18.0}
	c.fetchedAt = time.Now()
	c.mu.Unlock()

	c.Invalidate()

	c.mu.RLock()
	empty := c.data == nil
	c.mu.RUnlock()
	if !empty {
		t.Error("expected cache to be empty after Invalidate")
	}
}
