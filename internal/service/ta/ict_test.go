package ta

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeBar creates a simple OHLCV bar.
func makeBar(t time.Time, o, h, l, c, v float64) OHLCV {
	return OHLCV{Date: t, Open: o, High: h, Low: l, Close: c, Volume: v}
}

func baseTime() time.Time {
	// 2026-01-01 14:00 UTC — NY killzone
	return time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// TestKillzoneDetection
// ---------------------------------------------------------------------------

func TestKillzoneDetection(t *testing.T) {
	cases := []struct {
		hour int
		want string
	}{
		{0, "ASIAN"},
		{1, "ASIAN"},
		{2, "ASIAN"},
		{3, "OFF"},
		{8, "LONDON"},
		{9, "LONDON"},
		{10, "OFF"},
		{13, "NY"},
		{14, "NY"},
		{15, "OFF"},
		{20, "OFF"},
	}
	for _, tc := range cases {
		ts := time.Date(2026, 1, 1, tc.hour, 0, 0, 0, time.UTC)
		got := detectKillzone(ts)
		if got != tc.want {
			t.Errorf("hour=%d: want %q got %q", tc.hour, tc.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// TestFVGBullish — three bars with a clear upward gap
// ---------------------------------------------------------------------------

func TestFVGBullish(t *testing.T) {
	// Newest-first.  bars[2] = oldest, bars[1] = middle, bars[0] = newest.
	// Bullish FVG: bars[2].High < bars[0].Low
	now := baseTime()
	bars := []OHLCV{
		makeBar(now, 110, 115, 108, 112, 1000),    // bars[0] newest — Low=108 > bars[2].High=105
		makeBar(now.Add(-1*time.Hour), 104, 107, 102, 106, 1000), // bars[1] middle
		makeBar(now.Add(-2*time.Hour), 98, 105, 96, 103, 1000),   // bars[2] oldest — High=105
	}
	// Manually: prev=bars[2] High=105, next=bars[0] Low=108 → gap Low=105 High=108

	atr := 5.0 // minSize = 0.5, gap=3 > 0.5 → detected

	// Pad to 20 bars minimum required by CalcICT
	padded := padBars(bars, 20, now)
	result := CalcICT(padded, atr)
	if result == nil {
		t.Fatal("CalcICT returned nil")
	}

	var found *FVG
	for i := range result.FairValueGaps {
		if result.FairValueGaps[i].Type == "BULLISH" {
			found = &result.FairValueGaps[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected BULLISH FVG, got none")
	}
	if found.Low != 105 {
		t.Errorf("FVG.Low: want 105 got %v", found.Low)
	}
	if found.High != 108 {
		t.Errorf("FVG.High: want 108 got %v", found.High)
	}
	if found.Filled {
		t.Error("FVG should not be filled (no bar returned into gap)")
	}
}

// ---------------------------------------------------------------------------
// TestFVGFilled — checkFVGFill returns Filled=true when price returns into gap
// ---------------------------------------------------------------------------

func TestFVGFilled(t *testing.T) {
	// Test the fill detection function directly for deterministic results.
	// Bullish FVG: Low=105, High=108. A bar with Low=103 fills it 100%.
	fvg := FVG{Low: 105, High: 108, Type: "BULLISH"}

	// Bar that dips to 103 — fully fills the gap (103 < 105)
	fillerBars := []OHLCV{
		makeBar(baseTime(), 107, 109, 103, 108, 1000),
	}
	filled, fillPct := checkFVGFill(fillerBars, fvg, "BULLISH")
	if !filled {
		t.Errorf("expected Filled=true, fillPct=%.1f", fillPct)
	}
	if fillPct != 100 {
		t.Errorf("expected fillPct=100, got %.1f", fillPct)
	}

	// Bar that only dips to 106.5 — partial fill (108-106.5)/(108-105) = 50%
	partialBars := []OHLCV{
		makeBar(baseTime(), 107, 109, 106.5, 108, 1000),
	}
	filled2, fillPct2 := checkFVGFill(partialBars, fvg, "BULLISH")
	if filled2 {
		t.Error("expected Filled=false for partial fill")
	}
	wantPct := (108.0 - 106.5) / (108.0 - 105.0) * 100 // 50%
	if fillPct2 < wantPct-1 || fillPct2 > wantPct+1 {
		t.Errorf("expected fillPct≈%.1f, got %.1f", wantPct, fillPct2)
	}
}

// ---------------------------------------------------------------------------
// TestOrderBlockBullish
// ---------------------------------------------------------------------------

func TestOrderBlockBullish(t *testing.T) {
	// Build a sequence: old bearish candle followed by a strong bullish impulse.
	// newest-first: bars[0] is the most recent.
	// We want: somewhere in bars there's a bearish candle at bars[i],
	// then bars[i-1..i-3] are 3 consecutive bullish bars.
	now := baseTime()
	bars := make([]OHLCV, 25)
	for i := range bars {
		// Fill with neutral bars
		ts := now.Add(time.Duration(-i) * time.Hour)
		bars[i] = makeBar(ts, 100, 102, 98, 100, 500)
	}

	// Place bearish OB candle at index 20 (old)
	bars[20] = makeBar(now.Add(-20*time.Hour), 105, 108, 98, 99, 1000) // bearish: Close<Open

	// Place 3 consecutive bullish bars at index 19, 18, 17 (newer)
	bars[19] = makeBar(now.Add(-19*time.Hour), 99, 104, 99, 103, 1000)
	bars[18] = makeBar(now.Add(-18*time.Hour), 103, 108, 103, 107, 1000)
	bars[17] = makeBar(now.Add(-17*time.Hour), 107, 112, 107, 111, 1000)

	atr := 3.0
	result := CalcICT(bars, atr)
	if result == nil {
		t.Fatal("CalcICT returned nil")
	}

	var found *OrderBlock
	for i := range result.OrderBlocks {
		if result.OrderBlocks[i].Type == "BULLISH" {
			found = &result.OrderBlocks[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected BULLISH OrderBlock, got none")
	}
	// OB should correspond to the bearish candle at index 20
	if found.High != 108 {
		t.Errorf("OB.High: want 108 got %v", found.High)
	}
	if found.Low != 98 {
		t.Errorf("OB.Low: want 98 got %v", found.Low)
	}
}

// ---------------------------------------------------------------------------
// TestOrderBlockMitigated
// ---------------------------------------------------------------------------

func TestOrderBlockMitigated(t *testing.T) {
	now := baseTime()
	bars := make([]OHLCV, 25)
	for i := range bars {
		ts := now.Add(time.Duration(-i) * time.Hour)
		bars[i] = makeBar(ts, 100, 102, 98, 100, 500)
	}

	// Bearish OB at index 22
	bars[22] = makeBar(now.Add(-22*time.Hour), 105, 108, 98, 99, 1000)

	// Bullish impulse at 21, 20, 19
	bars[21] = makeBar(now.Add(-21*time.Hour), 99, 104, 99, 103, 1000)
	bars[20] = makeBar(now.Add(-20*time.Hour), 103, 108, 103, 107, 1000)
	bars[19] = makeBar(now.Add(-19*time.Hour), 107, 112, 107, 111, 1000)

	// Price returns to OB zone at index 10 (newer bar)
	// OB zone: Low=98, High=108. Bar touches the zone.
	bars[10] = makeBar(now.Add(-10*time.Hour), 102, 106, 99, 105, 1000) // Low=99 touches OB

	atr := 3.0
	result := CalcICT(bars, atr)
	if result == nil {
		t.Fatal("CalcICT returned nil")
	}

	var found *OrderBlock
	for i := range result.OrderBlocks {
		if result.OrderBlocks[i].Type == "BULLISH" {
			found = &result.OrderBlocks[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected BULLISH OrderBlock")
	}
	if !found.Mitigated {
		t.Error("expected OB to be Mitigated (price returned to zone)")
	}
}

// ---------------------------------------------------------------------------
// TestLiquiditySweep
// ---------------------------------------------------------------------------

func TestLiquiditySweep(t *testing.T) {
	now := baseTime()
	bars := make([]OHLCV, 30)
	basePrice := 100.0
	for i := range bars {
		ts := now.Add(time.Duration(-i) * time.Hour)
		bars[i] = makeBar(ts, basePrice, basePrice+2, basePrice-2, basePrice, 500)
	}

	// Place 3 swing highs near the same level (e.g., ~110) at bars 25, 22, 19
	// (swing high: higher than both neighbours)
	bars[26] = makeBar(now.Add(-26*time.Hour), 108, 109, 107, 108.5, 500)
	bars[25] = makeBar(now.Add(-25*time.Hour), 109, 110.2, 108, 109, 500) // swing high ≈110
	bars[24] = makeBar(now.Add(-24*time.Hour), 108, 109, 107, 108.5, 500)

	bars[23] = makeBar(now.Add(-23*time.Hour), 109, 109.5, 108, 109, 500)
	bars[22] = makeBar(now.Add(-22*time.Hour), 109, 110.4, 109, 109.5, 500) // swing high ≈110
	bars[21] = makeBar(now.Add(-21*time.Hour), 109, 109.5, 108, 109, 500)

	bars[20] = makeBar(now.Add(-20*time.Hour), 109, 109.3, 108, 109, 500)
	bars[19] = makeBar(now.Add(-19*time.Hour), 109, 110.1, 109, 109.5, 500) // swing high ≈110
	bars[18] = makeBar(now.Add(-18*time.Hour), 109, 109.3, 108, 109, 500)

	// Sweep bar: wick above 110 but closes below
	bars[5] = makeBar(now.Add(-5*time.Hour), 110, 111.5, 109, 109.5, 2000)

	atr := 2.5 // tolerance = 2.5*0.15 = 0.375 — covers cluster spread ~0.3
	result := CalcICT(bars, atr)
	if result == nil {
		t.Fatal("CalcICT returned nil")
	}

	var swept *LiquidityLevel
	for i := range result.LiquidityLevels {
		if result.LiquidityLevels[i].Type == "BUY_SIDE" && result.LiquidityLevels[i].Swept {
			swept = &result.LiquidityLevels[i]
			break
		}
	}
	if swept == nil {
		t.Fatal("expected swept BUY_SIDE liquidity level")
	}
	if swept.Count < 3 {
		t.Errorf("expected cluster count >= 3, got %d", swept.Count)
	}
}

// ---------------------------------------------------------------------------
// TestCalcICTNilOnInsufficientData
// ---------------------------------------------------------------------------

func TestCalcICTNilOnInsufficientData(t *testing.T) {
	// Fewer than 20 bars → nil
	bars := make([]OHLCV, 10)
	for i := range bars {
		bars[i] = makeBar(baseTime().Add(time.Duration(-i)*time.Hour), 100, 102, 98, 100, 500)
	}
	if CalcICT(bars, 2.0) != nil {
		t.Error("expected nil for < 20 bars")
	}

	// Zero ATR → nil
	padded := padBars(bars, 20, baseTime())
	if CalcICT(padded, 0) != nil {
		t.Error("expected nil for atr=0")
	}
}

// ---------------------------------------------------------------------------
// TestEquilibrium
// ---------------------------------------------------------------------------

func TestEquilibrium(t *testing.T) {
	now := baseTime()
	bars := make([]OHLCV, 20)
	// Range: Low=90, High=110 → Eq=100
	for i := range bars {
		ts := now.Add(time.Duration(-i) * time.Hour)
		bars[i] = makeBar(ts, 100, 110, 90, 105, 500) // Close=105 → premium
	}
	eq, premium, discount := calcEquilibrium(bars)
	if eq != 100 {
		t.Errorf("equilibrium: want 100, got %v", eq)
	}
	if !premium {
		t.Error("expected premium zone (close=105 > eq=100)")
	}
	if discount {
		t.Error("expected discount=false")
	}
}

// ---------------------------------------------------------------------------
// padBars — helper to pad a slice to minLen using flat bars
// ---------------------------------------------------------------------------

func padBars(bars []OHLCV, minLen int, now time.Time) []OHLCV {
	result := make([]OHLCV, len(bars))
	copy(result, bars)
	for len(result) < minLen {
		idx := len(result)
		ts := now.Add(time.Duration(-idx) * time.Hour)
		result = append(result, makeBar(ts, 100, 102, 98, 100, 500))
	}
	return result
}
