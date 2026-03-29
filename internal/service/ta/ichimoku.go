package ta

import "math"

// ---------------------------------------------------------------------------
// Ichimoku Cloud (9,26,52)
// ---------------------------------------------------------------------------

// IchimokuResult holds the latest Ichimoku Cloud values and derived signals.
//
// Ref: Goichi Hosoda's Ichimoku Kinko Hyo.
// - Tenkan-sen = (Highest High(9) + Lowest Low(9)) / 2
// - Kijun-sen  = (Highest High(26) + Lowest Low(26)) / 2
// - Senkou A   = (Tenkan + Kijun) / 2, plotted 26 periods ahead
// - Senkou B   = (Highest High(52) + Lowest Low(52)) / 2, plotted 26 periods ahead
// - Chikou     = Close, plotted 26 periods back
type IchimokuResult struct {
	Tenkan       float64 // Tenkan-sen (Conversion Line)
	Kijun        float64 // Kijun-sen (Base Line)
	SenkouA      float64 // Senkou Span A (current bar's projected value)
	SenkouB      float64 // Senkou Span B (current bar's projected value)
	Chikou       float64 // Chikou Span (current close)
	CloudColor   string  // "BULLISH" (SenkouA > SenkouB), "BEARISH", "NEUTRAL"
	TKCross      string  // "BULLISH_CROSS", "BEARISH_CROSS", "NONE"
	KumoBreakout string  // "BULLISH_BREAKOUT", "BEARISH_BREAKOUT", "INSIDE_CLOUD", "NONE"
	ChikouSignal string  // "BULLISH", "BEARISH", "NEUTRAL"
	Overall      string  // "STRONG_BULLISH", "BULLISH", "NEUTRAL", "BEARISH", "STRONG_BEARISH"
}

// IchimokuSeries holds the full Ichimoku series for charting.
// All slices are newest-first and same length as input bars.
// SenkouA/B are the "current" calculation values (not shifted forward);
// the caller should shift them 26 periods forward for chart plotting.
type IchimokuSeries struct {
	Tenkan  []float64 // Tenkan-sen series
	Kijun   []float64 // Kijun-sen series
	SenkouA []float64 // Senkou Span A series (at calculation point, shift +26 for plotting)
	SenkouB []float64 // Senkou Span B series (at calculation point, shift +26 for plotting)
	Chikou  []float64 // Chikou Span series (current close values, shift -26 for plotting)
}

// highestHigh returns the highest High in bars[start..end] (inclusive, oldest-first).
func highestHigh(asc []OHLCV, start, end int) float64 {
	hh := asc[start].High
	for i := start + 1; i <= end; i++ {
		if asc[i].High > hh {
			hh = asc[i].High
		}
	}
	return hh
}

// lowestLow returns the lowest Low in bars[start..end] (inclusive, oldest-first).
func lowestLow(asc []OHLCV, start, end int) float64 {
	ll := asc[start].Low
	for i := start + 1; i <= end; i++ {
		if asc[i].Low < ll {
			ll = asc[i].Low
		}
	}
	return ll
}

// CalcIchimoku computes the Ichimoku Cloud for the most recent bar.
// Requires at least 52 bars. Bars are newest-first.
//
// The "current" Senkou A/B here represent the values calculated at the current bar
// (which would be plotted 26 bars into the future on a chart). For the cloud at
// the current bar's position on the chart, we use the Senkou values calculated
// 26 bars ago.
func CalcIchimoku(bars []OHLCV) *IchimokuResult {
	const (
		tenkanPeriod = 9
		kijunPeriod  = 26
		senkouPeriod = 52
		shift        = 26
	)

	// We need at least senkouPeriod + shift bars to have cloud data at current bar position
	// But at minimum we need senkouPeriod bars for the basic calculation
	if len(bars) < senkouPeriod {
		return nil
	}

	// Work oldest-first
	asc := reverseOHLCV(bars)
	n := len(asc)
	last := n - 1

	// Tenkan-sen at latest bar
	tenkan := (highestHigh(asc, last-tenkanPeriod+1, last) + lowestLow(asc, last-tenkanPeriod+1, last)) / 2

	// Kijun-sen at latest bar
	kijun := (highestHigh(asc, last-kijunPeriod+1, last) + lowestLow(asc, last-kijunPeriod+1, last)) / 2

	// Senkou A = (Tenkan + Kijun) / 2 — this is the value that would be plotted 26 bars ahead
	senkouA := (tenkan + kijun) / 2

	// Senkou B = (Highest High(52) + Lowest Low(52)) / 2 — plotted 26 bars ahead
	senkouB := (highestHigh(asc, last-senkouPeriod+1, last) + lowestLow(asc, last-senkouPeriod+1, last)) / 2

	// Chikou = current close (plotted 26 bars back)
	chikou := asc[last].Close

	// For cloud color at current price position, use Senkou values from 26 bars ago
	// (because those were projected 26 bars forward to "now")
	cloudColor := "NEUTRAL"
	if n >= senkouPeriod+shift {
		pastIdx := last - shift
		pastTenkan := (highestHigh(asc, pastIdx-tenkanPeriod+1, pastIdx) + lowestLow(asc, pastIdx-tenkanPeriod+1, pastIdx)) / 2
		pastKijun := (highestHigh(asc, pastIdx-kijunPeriod+1, pastIdx) + lowestLow(asc, pastIdx-kijunPeriod+1, pastIdx)) / 2
		pastSA := (pastTenkan + pastKijun) / 2
		pastSB := (highestHigh(asc, pastIdx-senkouPeriod+1, pastIdx) + lowestLow(asc, pastIdx-senkouPeriod+1, pastIdx)) / 2

		if pastSA > pastSB {
			cloudColor = "BULLISH"
		} else if pastSA < pastSB {
			cloudColor = "BEARISH"
		}
	} else {
		// Fallback: use current Senkou values
		if senkouA > senkouB {
			cloudColor = "BULLISH"
		} else if senkouA < senkouB {
			cloudColor = "BEARISH"
		}
	}

	// TK Cross: compare current and previous Tenkan vs Kijun
	tkCross := "NONE"
	if n >= senkouPeriod+1 {
		prev := last - 1
		if prev >= kijunPeriod-1 && prev >= tenkanPeriod-1 {
			prevTenkan := (highestHigh(asc, prev-tenkanPeriod+1, prev) + lowestLow(asc, prev-tenkanPeriod+1, prev)) / 2
			prevKijun := (highestHigh(asc, prev-kijunPeriod+1, prev) + lowestLow(asc, prev-kijunPeriod+1, prev)) / 2

			if prevTenkan <= prevKijun && tenkan > kijun {
				tkCross = "BULLISH_CROSS"
			} else if prevTenkan >= prevKijun && tenkan < kijun {
				tkCross = "BEARISH_CROSS"
			}
		}
	}

	// Kumo Breakout: check if price has broken above/below the cloud
	kumoBreakout := "NONE"
	if n >= senkouPeriod+shift {
		pastIdx := last - shift
		pastTenkan := (highestHigh(asc, pastIdx-tenkanPeriod+1, pastIdx) + lowestLow(asc, pastIdx-tenkanPeriod+1, pastIdx)) / 2
		pastKijun := (highestHigh(asc, pastIdx-kijunPeriod+1, pastIdx) + lowestLow(asc, pastIdx-kijunPeriod+1, pastIdx)) / 2
		curSA := (pastTenkan + pastKijun) / 2
		curSB := (highestHigh(asc, pastIdx-senkouPeriod+1, pastIdx) + lowestLow(asc, pastIdx-senkouPeriod+1, pastIdx)) / 2

		cloudTop := math.Max(curSA, curSB)
		cloudBottom := math.Min(curSA, curSB)
		price := asc[last].Close

		if price > cloudTop {
			kumoBreakout = "BULLISH_BREAKOUT"
		} else if price < cloudBottom {
			kumoBreakout = "BEARISH_BREAKOUT"
		} else {
			kumoBreakout = "INSIDE_CLOUD"
		}
	}

	// Chikou Signal: current close vs price 26 bars ago
	chikouSignal := "NEUTRAL"
	if n > shift {
		priceBack := asc[last-shift].Close
		if chikou > priceBack {
			chikouSignal = "BULLISH"
		} else if chikou < priceBack {
			chikouSignal = "BEARISH"
		}
	}

	// Overall signal
	score := 0
	if cloudColor == "BULLISH" {
		score++
	} else if cloudColor == "BEARISH" {
		score--
	}
	if tkCross == "BULLISH_CROSS" {
		score++
	} else if tkCross == "BEARISH_CROSS" {
		score--
	}
	if kumoBreakout == "BULLISH_BREAKOUT" {
		score++
	} else if kumoBreakout == "BEARISH_BREAKOUT" {
		score--
	}
	if chikouSignal == "BULLISH" {
		score++
	} else if chikouSignal == "BEARISH" {
		score--
	}

	// Also factor in Tenkan vs Kijun relationship
	if tenkan > kijun {
		score++
	} else if tenkan < kijun {
		score--
	}

	overall := "NEUTRAL"
	switch {
	case score >= 4:
		overall = "STRONG_BULLISH"
	case score >= 2:
		overall = "BULLISH"
	case score <= -4:
		overall = "STRONG_BEARISH"
	case score <= -2:
		overall = "BEARISH"
	}

	return &IchimokuResult{
		Tenkan:       tenkan,
		Kijun:        kijun,
		SenkouA:      senkouA,
		SenkouB:      senkouB,
		Chikou:       chikou,
		CloudColor:   cloudColor,
		TKCross:      tkCross,
		KumoBreakout: kumoBreakout,
		ChikouSignal: chikouSignal,
		Overall:      overall,
	}
}

// CalcIchimokuSeries computes the full Ichimoku series for charting.
// All output slices have the same length as input bars (newest-first).
// Values where insufficient lookback exists are NaN.
//
// Note on plotting:
// - SenkouA[i] and SenkouB[i] are the values calculated at bar i. To plot them
//   on the chart, shift 26 periods into the future.
// - Chikou[i] is the close at bar i. To plot on chart, shift 26 periods into the past.
func CalcIchimokuSeries(bars []OHLCV) *IchimokuSeries {
	const (
		tenkanPeriod = 9
		kijunPeriod  = 26
		senkouPeriod = 52
	)

	if len(bars) < senkouPeriod {
		return nil
	}

	asc := reverseOHLCV(bars)
	n := len(asc)

	tenkan := make([]float64, n)
	kijunS := make([]float64, n)
	senkouA := make([]float64, n)
	senkouB := make([]float64, n)
	chikouS := make([]float64, n)

	for i := 0; i < n; i++ {
		// Tenkan-sen: need tenkanPeriod bars
		if i >= tenkanPeriod-1 {
			tenkan[i] = (highestHigh(asc, i-tenkanPeriod+1, i) + lowestLow(asc, i-tenkanPeriod+1, i)) / 2
		} else {
			tenkan[i] = math.NaN()
		}

		// Kijun-sen: need kijunPeriod bars
		if i >= kijunPeriod-1 {
			kijunS[i] = (highestHigh(asc, i-kijunPeriod+1, i) + lowestLow(asc, i-kijunPeriod+1, i)) / 2
		} else {
			kijunS[i] = math.NaN()
		}

		// Senkou A: need both Tenkan and Kijun
		if i >= kijunPeriod-1 {
			senkouA[i] = (tenkan[i] + kijunS[i]) / 2
		} else {
			senkouA[i] = math.NaN()
		}

		// Senkou B: need senkouPeriod bars
		if i >= senkouPeriod-1 {
			senkouB[i] = (highestHigh(asc, i-senkouPeriod+1, i) + lowestLow(asc, i-senkouPeriod+1, i)) / 2
		} else {
			senkouB[i] = math.NaN()
		}

		// Chikou: just the close
		chikouS[i] = asc[i].Close
	}

	return &IchimokuSeries{
		Tenkan:  reverseFloat64(tenkan),
		Kijun:   reverseFloat64(kijunS),
		SenkouA: reverseFloat64(senkouA),
		SenkouB: reverseFloat64(senkouB),
		Chikou:  reverseFloat64(chikouS),
	}
}
