// Package orderflow provides estimated delta and order flow analysis from OHLCV bars.
//
// Because tick data is unavailable for forex, estimation uses the "tick rule":
//   - Bullish bar: EstBuyVol = Volume × (Close - Low) / (High - Low)
//   - Bearish bar: EstSellVol = Volume × (High - Close) / (High - Low)
//
// For a zero-range bar (High == Low), volume is split 50/50.
package orderflow

import (
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/ta"
)

// DeltaBar is one OHLCV bar annotated with estimated buy/sell volume.
type DeltaBar struct {
	OHLCV    ta.OHLCV
	BuyVol   float64 // estimated buy volume
	SellVol  float64 // estimated sell volume
	Delta    float64 // BuyVol - SellVol (positive = buyer dominated)
	CumDelta float64 // cumulative delta from start of the lookback window
}

// OrderFlowResult is the complete output of the order-flow engine for one
// symbol+timeframe analysis.
type OrderFlowResult struct {
	Symbol    string
	Timeframe string

	// DeltaBars contains the annotated bars (newest-first, up to MaxBars).
	DeltaBars []DeltaBar

	// PriceDeltaDivergence detects when price direction and cumulative delta
	// direction disagree over the analysis window.
	//   "BULLISH_DIV"  – price lower low but delta higher low (hidden buying)
	//   "BEARISH_DIV"  – price higher high but delta lower high (hidden selling)
	//   "NONE"
	PriceDeltaDivergence string

	// PointOfControl is the price level (bar Close) with the highest volume.
	PointOfControl float64

	// BullishAbsorption contains indices (into DeltaBars) where heavy selling
	// was absorbed by buyers (limited downward movement despite large sell delta).
	BullishAbsorption []int

	// BearishAbsorption contains indices (into DeltaBars) where heavy buying
	// was absorbed by sellers (limited upward movement despite large buy delta).
	BearishAbsorption []int

	// DeltaTrend is the overall trend of the cumulative delta series.
	DeltaTrend string // "RISING" | "FALLING" | "FLAT"

	// CumDelta is the total cumulative delta across all analysed bars.
	CumDelta float64

	// Bias is the synthesised directional bias.
	Bias string // "BULLISH" | "BEARISH" | "NEUTRAL"

	// Summary is a short human-readable explanation (≤ 200 chars).
	Summary string

	AnalyzedAt time.Time
}
