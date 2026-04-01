package orderflow

import (
	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// pointOfControl returns the Close price of the bar that carries the most volume
// from the provided OHLCV slice.
// Returns 0.0 if bars is empty.
func pointOfControl(bars []ta.OHLCV) float64 {
	if len(bars) == 0 {
		return 0
	}
	bestIdx := 0
	for i := 1; i < len(bars); i++ {
		if bars[i].Volume > bars[bestIdx].Volume {
			bestIdx = i
		}
	}
	return bars[bestIdx].Close
}
