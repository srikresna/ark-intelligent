package domain

import "time"

// MarketBreadthData holds the latest market breadth indicators from barchart.com.
// These are equity-market health metrics used as a risk-sentiment proxy for forex.
//
// Interpretation:
//   - PctAbove200MA < 30%  → pasar dalam kondisi breadth lemah — risk-off bias
//   - PctAbove200MA > 70%  → pasar sehat secara breadth — risk-on
//   - Breadth divergence    → harga indeks naik tapi breadth melemah → distribusi, bearish warning
type MarketBreadthData struct {
	// PctAbove50MA is the percentage of S&P 500 stocks trading above their 50-day moving average.
	PctAbove50MA float64

	// PctAbove200MA is the percentage of S&P 500 stocks trading above their 200-day moving average.
	PctAbove200MA float64

	// AdvanceDeclineRatio is the ratio of advancing to declining stocks (advance / decline).
	// Values > 1 are bullish; values < 1 are bearish.
	AdvanceDeclineRatio float64

	// New52WkHighs is the number of stocks making new 52-week highs.
	New52WkHighs int

	// New52WkLows is the number of stocks making new 52-week lows.
	New52WkLows int

	// Available indicates whether valid data was successfully fetched.
	Available bool

	// FetchedAt is the time at which this data was retrieved.
	FetchedAt time.Time
}

// BreadthRegime classifies the market breadth condition for forex risk analysis.
func (m *MarketBreadthData) BreadthRegime() string {
	if !m.Available {
		return "UNKNOWN"
	}
	switch {
	case m.PctAbove200MA >= 70:
		return "RISK-ON"
	case m.PctAbove200MA >= 50:
		return "NEUTRAL"
	case m.PctAbove200MA >= 30:
		return "CAUTION"
	default:
		return "RISK-OFF"
	}
}

// HighLowRatio returns the ratio of new 52-week highs to lows.
// A high ratio (>2) indicates broad participation; a low ratio (<0.5) signals weakness.
func (m *MarketBreadthData) HighLowRatio() float64 {
	if m.New52WkLows == 0 {
		return 0
	}
	return float64(m.New52WkHighs) / float64(m.New52WkLows)
}
