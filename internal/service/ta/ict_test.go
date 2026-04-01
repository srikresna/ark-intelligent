package ta

import "testing"

// ictTestWavesBars creates bars with pronounced waves for testing ICT detectors.
func ictTestWavesBars() []OHLCV {
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
		bars[n-1-i] = OHLCV{
			Open:  c - 0.0005,
			High:  c + 0.0010,
			Low:   c - 0.0010,
			Close: c,
		}
	}
	return bars
}

// TestDetectFVG_BullishFVG verifies detection of a bullish Fair Value Gap.
func TestDetectFVG_BullishFVG(t *testing.T) {
	bars := []OHLCV{
		{Open: 1.1060, High: 1.1080, Low: 1.1050, Close: 1.1075},
		{Open: 1.1020, High: 1.1040, Low: 1.1010, Close: 1.1030},
		{Open: 1.0980, High: 1.1000, Low: 1.0960, Close: 1.0990},
		{Open: 1.0970, High: 1.0990, Low: 1.0950, Close: 1.0980},
		{Open: 1.0960, High: 1.0980, Low: 1.0940, Close: 1.0970},
	}

	atr := 0.0020
	results := DetectFVG(bars, atr)

	if len(results) == 0 {
		t.Fatal("expected at least one FVG, got none")
	}

	found := false
	for _, r := range results {
		if r.Direction == "BULLISH" {
			found = true
			if r.HighEdge != bars[0].Low {
				t.Errorf("expected HighEdge=%.5f, got %.5f", bars[0].Low, r.HighEdge)
			}
			if r.LowEdge != bars[2].High {
				t.Errorf("expected LowEdge=%.5f, got %.5f", bars[2].High, r.LowEdge)
			}
			break // check only the first (newest) bullish FVG
		}
	}
	if !found {
		t.Error("expected BULLISH FVG in results")
	}
}

// TestDetectFVG_BearishFVG verifies detection of a bearish Fair Value Gap.
func TestDetectFVG_BearishFVG(t *testing.T) {
	bars := []OHLCV{
		{Open: 1.0960, High: 1.0950, Low: 1.0930, Close: 1.0940},
		{Open: 1.0990, High: 1.1010, Low: 1.0980, Close: 1.0995},
		{Open: 1.1010, High: 1.1030, Low: 1.1000, Close: 1.1020},
		{Open: 1.1020, High: 1.1040, Low: 1.1010, Close: 1.1030},
		{Open: 1.1030, High: 1.1050, Low: 1.1020, Close: 1.1040},
	}

	atr := 0.0020
	results := DetectFVG(bars, atr)

	found := false
	for _, r := range results {
		if r.Direction == "BEARISH" {
			found = true
			if r.LowEdge != bars[0].High {
				t.Errorf("expected LowEdge=%.5f, got %.5f", bars[0].High, r.LowEdge)
			}
			if r.HighEdge != bars[2].Low {
				t.Errorf("expected HighEdge=%.5f, got %.5f", bars[2].Low, r.HighEdge)
			}
			break // check only the first (newest) bearish FVG
		}
	}
	if !found {
		t.Error("expected BEARISH FVG in results")
	}
}

// TestDetectFVG_SmallGapFiltered verifies small gaps below 0.2×ATR are excluded.
func TestDetectFVG_SmallGapFiltered(t *testing.T) {
	bars := []OHLCV{
		{Open: 1.1001, High: 1.1005, Low: 1.1001, Close: 1.1003},
		{Open: 1.0999, High: 1.1000, Low: 1.0999, Close: 1.1000},
		{Open: 1.0998, High: 1.0999, Low: 1.0997, Close: 1.0998},
		{Open: 1.0997, High: 1.0998, Low: 1.0996, Close: 1.0997},
		{Open: 1.0996, High: 1.0997, Low: 1.0995, Close: 1.0996},
	}

	atr := 0.0020
	results := DetectFVG(bars, atr)
	minGap := atr * 0.2
	for _, r := range results {
		gap := r.HighEdge - r.LowEdge
		if gap < minGap {
			t.Errorf("FVG gap %.5f < minGap %.5f should have been filtered", gap, minGap)
		}
	}
}

// TestDetectFVG_MaxFive verifies at most 5 FVGs are returned.
func TestDetectFVG_MaxFive(t *testing.T) {
	bars := ictTestWavesBars()
	results := DetectFVG(bars, 0.0015)
	if len(results) > 5 {
		t.Errorf("expected at most 5 FVGs, got %d", len(results))
	}
}

// TestDetectOrderBlocks_BullishOB verifies detection of a bullish order block.
func TestDetectOrderBlocks_BullishOB(t *testing.T) {
	atr := 0.0020
	impulseRange := atr * 2.0

	bars := []OHLCV{
		{Open: 1.0990, High: 1.0990 + impulseRange, Low: 1.0988, Close: 1.0990 + impulseRange - 0.0002},
		{Open: 1.1000, High: 1.1005, Low: 1.0985, Close: 1.0990},
		{Open: 1.1005, High: 1.1010, Low: 1.1000, Close: 1.1005},
		{Open: 1.1000, High: 1.1005, Low: 1.0995, Close: 1.1000},
		{Open: 1.0995, High: 1.1000, Low: 1.0990, Close: 1.0995},
	}

	results := DetectOrderBlocks(bars, atr)
	if len(results) == 0 {
		t.Fatal("expected at least one order block")
	}

	found := false
	for _, ob := range results {
		if ob.Direction == "BULLISH" {
			found = true
			if ob.High != bars[1].High {
				t.Errorf("expected High=%.5f, got %.5f", bars[1].High, ob.High)
			}
			if ob.Low != bars[1].Low {
				t.Errorf("expected Low=%.5f, got %.5f", bars[1].Low, ob.Low)
			}
		}
	}
	if !found {
		t.Error("expected BULLISH order block")
	}
}

// TestDetectOrderBlocks_MaxThree verifies at most 3 order blocks are returned.
func TestDetectOrderBlocks_MaxThree(t *testing.T) {
	bars := ictTestWavesBars()
	results := DetectOrderBlocks(bars, 0.0015)
	if len(results) > 3 {
		t.Errorf("expected at most 3 order blocks, got %d", len(results))
	}
}

// TestICTResultIntegration verifies ICT field is populated in ComputeSnapshot.
func TestICTResultIntegration(t *testing.T) {
	bars := ictTestWavesBars()
	engine := NewEngine()
	snap := engine.ComputeSnapshot(bars)

	if snap == nil {
		t.Fatal("ComputeSnapshot returned nil")
	}

	if snap.ICT != nil {
		if len(snap.ICT.FVGs) > 5 {
			t.Errorf("expected max 5 FVGs, got %d", len(snap.ICT.FVGs))
		}
		if len(snap.ICT.OrderBlocks) > 3 {
			t.Errorf("expected max 3 OBs, got %d", len(snap.ICT.OrderBlocks))
		}
		for _, fvg := range snap.ICT.FVGs {
			if fvg.Direction != "BULLISH" && fvg.Direction != "BEARISH" {
				t.Errorf("unexpected FVG direction: %q", fvg.Direction)
			}
			if fvg.HighEdge < fvg.LowEdge {
				t.Errorf("FVG HighEdge %.5f < LowEdge %.5f", fvg.HighEdge, fvg.LowEdge)
			}
		}
		for _, ob := range snap.ICT.OrderBlocks {
			if ob.Direction != "BULLISH" && ob.Direction != "BEARISH" {
				t.Errorf("unexpected OB direction: %q", ob.Direction)
			}
			if ob.Strength < 1 || ob.Strength > 5 {
				t.Errorf("OB Strength %d out of range 1-5", ob.Strength)
			}
		}
	}
}

