package wyckoff

import (
	"math"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// ─────────────────────────────────────────────────────────────────────────────
// Event detection helpers
// All functions receive bars in oldest-first order (index 0 = oldest bar).
// ─────────────────────────────────────────────────────────────────────────────

const minBarsForEvents = 20

// detectPS finds the Preliminary Support: a high-volume bar that temporarily
// halts a downtrend before the Selling Climax.
func detectPS(bars []ta.OHLCV, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if n < minBarsForEvents {
		return nil
	}
	// Look in first 40 % of the bars for a downtrend halt with above-avg volume.
	lookback := n * 40 / 100
	if lookback < 5 {
		lookback = 5
	}

	for i := 1; i < lookback; i++ {
		b := bars[i]
		// Requires bar to be a down bar reversing (close > open or large lower wick)
		lowerWick := b.Low
		bodyMid := (b.Open + b.Close) / 2
		if b.Volume > avgVol*1.3 && bodyMid > lowerWick+(b.High-b.Low)*0.3 {
			return &WyckoffEvent{
				Name:         EventPS,
				BarIndex:     i,
				Price:        b.Close,
				Volume:       b.Volume,
				Significance: "MEDIUM",
				Description:  "Preliminary Support — high volume deceleration in downtrend",
			}
		}
	}
	return nil
}

// detectSC finds the Selling Climax: the highest volume bar during a downtrend
// with a wide-range candle, representing capitulation.
func detectSC(bars []ta.OHLCV, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if n < minBarsForEvents {
		return nil
	}
	half := n / 2
	if half > 60 {
		half = 60
	}

	best := -1
	bestVol := 0.0
	avgRange := avgBarRange(bars, 14)
	for i := 0; i < half; i++ {
		b := bars[i]
		rangeSize := b.High - b.Low
		if b.Volume > avgVol*1.5 && rangeSize > avgRange*1.2 && b.Close < b.Open {
			if b.Volume > bestVol {
				bestVol = b.Volume
				best = i
			}
		}
	}
	if best < 0 {
		return nil
	}
	b := bars[best]
	return &WyckoffEvent{
		Name:         EventSC,
		BarIndex:     best,
		Price:        b.Close,
		Volume:       b.Volume,
		Significance: "HIGH",
		Description:  "Selling Climax — peak volume capitulation bar",
	}
}

// detectAR finds the Automatic Rally following the SC: a bounce with lower
// volume that covers short sellers.
func detectAR(bars []ta.OHLCV, scIndex int, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if scIndex < 0 || scIndex+2 >= n {
		return nil
	}
	scLow := bars[scIndex].Low

	// Look for the highest close after SC within next 20 bars.
	end := scIndex + 20
	if end > n-1 {
		end = n - 1
	}
	best := -1
	bestClose := 0.0
	for i := scIndex + 1; i <= end; i++ {
		b := bars[i]
		if b.Close > bestClose && b.Volume < avgVol*1.2 {
			bestClose = b.Close
			best = i
		}
	}
	if best < 0 || bestClose <= scLow {
		return nil
	}
	b := bars[best]
	return &WyckoffEvent{
		Name:         EventAR,
		BarIndex:     best,
		Price:        b.Close,
		Volume:       b.Volume,
		Significance: "HIGH",
		Description:  "Automatic Rally — bounce off SC on declining volume",
	}
}

// detectST finds the Secondary Test: a low-volume revisit of the SC lows.
func detectST(bars []ta.OHLCV, scIndex int, arIndex int, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if scIndex < 0 || arIndex <= scIndex {
		return nil
	}
	scLow := bars[scIndex].Low

	start := arIndex + 1
	end := arIndex + 30
	if end > n-1 {
		end = n - 1
	}

	for i := start; i <= end; i++ {
		b := bars[i]
		// Approaches SC low but doesn't break it, low volume = supply exhausted
		if math.Abs(b.Low-scLow)/scLow < 0.005 && b.Volume < avgVol*0.8 {
			sig := "HIGH"
			if b.Volume > avgVol*0.6 {
				sig = "MEDIUM"
			}
			return &WyckoffEvent{
				Name:         EventST,
				BarIndex:     i,
				Price:        b.Close,
				Volume:       b.Volume,
				Significance: sig,
				Description:  "Secondary Test — low volume revisit of SC lows (supply drying up)",
			}
		}
	}
	return nil
}

// detectSpring finds the Spring: a brief break below SC low that quickly
// reverses. Low volume = bullish (no real supply); high volume = bad spring.
func detectSpring(bars []ta.OHLCV, scIndex int, arIndex int, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if scIndex < 0 || arIndex <= scIndex {
		return nil
	}
	scLow := bars[scIndex].Low

	start := arIndex + 1
	end := start + 40
	if end > n-1 {
		end = n - 1
	}

	for i := start; i <= end-1; i++ {
		b := bars[i]
		if b.Low < scLow {
			// Check rapid recovery in next 1-3 bars
			for j := i + 1; j <= i+3 && j < n; j++ {
				recovery := bars[j]
				if recovery.Close > scLow {
					lowVol := b.Volume < avgVol*1.0
					sig := "HIGH"
					desc := "Spring — break below SC low with rapid recovery"
					if lowVol {
						desc += " (LOW volume ✅ bullish — no supply)"
					} else {
						sig = "MEDIUM"
						desc += " (HIGH volume ⚠️ — supply still present)"
					}
					return &WyckoffEvent{
						Name:         EventSpring,
						BarIndex:     i,
						Price:        b.Low,
						Volume:       b.Volume,
						Significance: sig,
						Description:  desc,
					}
				}
			}
		}
	}
	return nil
}

// detectSOS finds the Sign of Strength: a high-volume break above the AR high.
func detectSOS(bars []ta.OHLCV, arIndex int, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if arIndex < 0 || arIndex >= n {
		return nil
	}
	arHigh := bars[arIndex].High

	start := arIndex + 1
	end := start + 50
	if end > n-1 {
		end = n - 1
	}
	for i := start; i <= end; i++ {
		b := bars[i]
		if b.Close > arHigh && b.Volume > avgVol*1.2 {
			return &WyckoffEvent{
				Name:         EventSOS,
				BarIndex:     i,
				Price:        b.Close,
				Volume:       b.Volume,
				Significance: "HIGH",
				Description:  "Sign of Strength — high volume breakout above AR high",
			}
		}
	}
	return nil
}

// detectLPS finds the Last Point of Support: a low-volume pullback to prior
// AR-high after the SOS (former resistance becomes support).
func detectLPS(bars []ta.OHLCV, sosIndex int, arHigh float64, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if sosIndex < 0 || sosIndex >= n {
		return nil
	}
	start := sosIndex + 1
	end := start + 20
	if end > n-1 {
		end = n - 1
	}
	for i := start; i <= end; i++ {
		b := bars[i]
		if math.Abs(b.Low-arHigh)/arHigh < 0.006 && b.Volume < avgVol*0.9 {
			return &WyckoffEvent{
				Name:         EventLPS,
				BarIndex:     i,
				Price:        b.Close,
				Volume:       b.Volume,
				Significance: "HIGH",
				Description:  "Last Point of Support — low volume pullback to prior resistance (now support)",
			}
		}
	}
	return nil
}

// detectBC finds the Buying Climax (Distribution): peak volume blow-off top.
func detectBC(bars []ta.OHLCV, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if n < minBarsForEvents {
		return nil
	}
	half := n / 2
	if half > 60 {
		half = 60
	}
	best := -1
	bestVol := 0.0
	avgRange := avgBarRange(bars, 14)
	for i := 0; i < half; i++ {
		b := bars[i]
		rangeSize := b.High - b.Low
		if b.Volume > avgVol*1.5 && rangeSize > avgRange*1.1 && b.Close > b.Open {
			if b.Volume > bestVol {
				bestVol = b.Volume
				best = i
			}
		}
	}
	if best < 0 {
		return nil
	}
	b := bars[best]
	return &WyckoffEvent{
		Name:         EventBC,
		BarIndex:     best,
		Price:        b.Close,
		Volume:       b.Volume,
		Significance: "HIGH",
		Description:  "Buying Climax — peak volume exhaustion at top",
	}
}

// detectUTAD finds the Upthrust After Distribution: brief break above range
// high that quickly fails.
func detectUTAD(bars []ta.OHLCV, arDistIndex int, rangeHigh float64, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if arDistIndex < 0 || rangeHigh <= 0 {
		return nil
	}
	start := arDistIndex + 1
	end := start + 40
	if end > n-1 {
		end = n - 1
	}
	for i := start; i <= end-1; i++ {
		b := bars[i]
		if b.High > rangeHigh {
			for j := i + 1; j <= i+3 && j < n; j++ {
				rec := bars[j]
				if rec.Close < rangeHigh {
					return &WyckoffEvent{
						Name:         EventUTAD,
						BarIndex:     i,
						Price:        b.High,
						Volume:       b.Volume,
						Significance: "HIGH",
						Description:  "Upthrust After Distribution — failed break above range high",
					}
				}
			}
		}
	}
	return nil
}

// detectSOW finds the Sign of Weakness: a high-volume break below the
// distribution range support.
func detectSOW(bars []ta.OHLCV, bcIndex int, rangeLow float64, avgVol float64) *WyckoffEvent {
	n := len(bars)
	if bcIndex < 0 || rangeLow <= 0 {
		return nil
	}
	start := bcIndex + 1
	end := start + 60
	if end > n-1 {
		end = n - 1
	}
	for i := start; i <= end; i++ {
		b := bars[i]
		if b.Close < rangeLow && b.Volume > avgVol*1.1 {
			return &WyckoffEvent{
				Name:         EventSOW,
				BarIndex:     i,
				Price:        b.Close,
				Volume:       b.Volume,
				Significance: "HIGH",
				Description:  "Sign of Weakness — high volume break below distribution support",
			}
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Utilities
// ─────────────────────────────────────────────────────────────────────────────

// avgBarRange returns the mean (high-low) over the last n bars.
func avgBarRange(bars []ta.OHLCV, n int) float64 {
	if n > len(bars) {
		n = len(bars)
	}
	if n == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += bars[i].High - bars[i].Low
	}
	return sum / float64(n)
}

// avgVolume returns the mean volume over all bars.
func avgVolume(bars []ta.OHLCV) float64 {
	if len(bars) == 0 {
		return 0
	}
	sum := 0.0
	for _, b := range bars {
		sum += b.Volume
	}
	return sum / float64(len(bars))
}
