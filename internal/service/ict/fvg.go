package ict

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DetectFVG detects Fair Value Gaps from a newest-first bar slice.
//
// Detection is fully delegated to the canonical ta.CalcICT implementation
// (single source of truth). Returns nil if insufficient data (<20 bars or
// ATR cannot be computed).
func DetectFVG(bars []ta.OHLCV) []FVGZone {
	n := len(bars)
	if n < 20 {
		return nil
	}

	atr := ta.CalcATR(bars, 14)
	if atr <= 0 {
		atr = estimateATR(bars)
		if atr <= 0 {
			return nil
		}
	}

	taResult := ta.CalcICT(bars, atr)
	if taResult == nil {
		return nil
	}
	return convertFVGs(taResult.FairValueGaps)
}

// estimateATR provides a simple average-range fallback when CalcATR returns 0.
func estimateATR(bars []ta.OHLCV) float64 {
	if len(bars) == 0 {
		return 0
	}
	sum := 0.0
	for _, b := range bars {
		sum += b.High - b.Low
	}
	return sum / float64(len(bars))
}
