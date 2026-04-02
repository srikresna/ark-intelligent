package ta

import (
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// ---------------------------------------------------------------------------
// OHLCV — Unified bar for any timeframe
// ---------------------------------------------------------------------------

// OHLCV represents a single price bar (Open-High-Low-Close-Volume).
// Slices of OHLCV are always ordered newest-first (index 0 = most recent bar).
type OHLCV struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// ---------------------------------------------------------------------------
// Indicator Result Types
// ---------------------------------------------------------------------------

// RSIResult holds the output of a Relative Strength Index calculation.
// Ref: Wilder, "New Concepts in Technical Trading Systems" (1978).
type RSIResult struct {
	Value float64 // RSI value (0–100)
	Zone  string  // "OVERBOUGHT" (>70), "OVERSOLD" (<30), "NEUTRAL"
	Trend string  // "RISING", "FALLING", "FLAT" based on last few RSI values
}

// MACDResult holds the output of a MACD calculation.
// Ref: Gerald Appel's MACD.
type MACDResult struct {
	MACD         float64 // MACD line value
	Signal       float64 // Signal line value
	Histogram    float64 // MACD - Signal
	Cross        string  // "BULLISH_CROSS", "BEARISH_CROSS", "NONE"
	BullishCross bool    // true if MACD just crossed above signal
	BearishCross bool    // true if MACD just crossed below signal
}

// StochasticResult holds the output of a Stochastic Oscillator calculation.
// Ref: George Lane's Stochastic.
type StochasticResult struct {
	K     float64 // %K (slow)
	D     float64 // %D (signal line)
	Zone  string  // "OVERBOUGHT" (>80), "OVERSOLD" (<20), "NEUTRAL"
	Cross string  // "BULLISH_CROSS", "BEARISH_CROSS", "NONE"
}

// BollingerResult holds the output of a Bollinger Bands calculation.
// Ref: John Bollinger's "Bollinger on Bollinger Bands".
type BollingerResult struct {
	Upper     float64 // Upper band
	Middle    float64 // Middle band (SMA)
	Lower     float64 // Lower band
	Bandwidth float64 // (Upper - Lower) / Middle * 100
	PercentB  float64 // (Close - Lower) / (Upper - Lower)
	Squeeze   bool    // true if bandwidth < 75% of its own 20-period average
}

// EMAResult holds a snapshot of an EMA ribbon.
type EMAResult struct {
	Values         map[int]float64 // period → current EMA value
	RibbonAlignment string         // "BULLISH", "BEARISH", "MIXED"
	AlignmentScore float64         // -1 to +1 (1 = perfectly bullish aligned)
}

// ADXResult holds the output of the Average Directional Index.
// Ref: Wilder, "New Concepts in Technical Trading Systems" (1978).
type ADXResult struct {
	ADX           float64 // ADX value
	PlusDI        float64 // +DI value
	MinusDI       float64 // -DI value
	Trending      bool    // true if ADX > 25
	TrendStrength string  // "STRONG" (>50), "MODERATE" (25-50), "WEAK" (<25)
}

// OBVResult holds the output of On-Balance Volume.
// Ref: Joseph Granville's OBV.
type OBVResult struct {
	Value  float64   // Current OBV value
	Trend  string    // "RISING", "FALLING", "FLAT"
	Series []float64 // Full OBV series (newest-first)
}

// WilliamsRResult holds the output of Williams %R.
// Ref: Larry Williams' %R.
type WilliamsRResult struct {
	Value float64 // %R value (-100 to 0)
	Zone  string  // "OVERBOUGHT" (> -20), "OVERSOLD" (< -80), "NEUTRAL"
}

// CCIResult holds the output of the Commodity Channel Index.
// Ref: Donald Lambert's CCI.
type CCIResult struct {
	Value float64 // CCI value
	Zone  string  // "OVERBOUGHT" (>+100), "OVERSOLD" (<-100), "NEUTRAL"
}

// MFIResult holds the output of the Money Flow Index.
// Ref: Gene Quong & Avrum Soudack's MFI.
type MFIResult struct {
	Value float64 // MFI value (0–100)
	Zone  string  // "OVERBOUGHT" (>80), "OVERSOLD" (<20), "NEUTRAL"
}

// ---------------------------------------------------------------------------
// Composite Types
// ---------------------------------------------------------------------------

// IndicatorSnapshot bundles all indicator results for a single timeframe.
type IndicatorSnapshot struct {
	Timeframe    string             // e.g. "daily", "4h", "1h", "15m", "weekly"
	CurrentPrice float64            // Close price of the most recent bar
	ATR          float64            // ATR(14) from raw bars — used by zones
	RSI          *RSIResult         // nil if insufficient data
	MACD         *MACDResult        // nil if insufficient data
	Stochastic   *StochasticResult  // nil if insufficient data
	Bollinger    *BollingerResult   // nil if insufficient data
	EMA          *EMAResult         // nil if insufficient data
	ADX          *ADXResult         // nil if insufficient data
	OBV          *OBVResult         // nil if insufficient data
	WilliamsR    *WilliamsRResult   // nil if insufficient data
	CCI          *CCIResult         // nil if insufficient data
	MFI          *MFIResult         // nil if insufficient data

	// Advanced indicators (nil if insufficient data or file not yet available)
	Ichimoku   *IchimokuResult    // nil if insufficient data
	SuperTrend *SuperTrendResult  // nil if insufficient data
	Fibonacci  *FibResult         // nil if insufficient data
	Killzone   *KillzoneResult    // current ICT killzone classification (always populated)
	VWAP       *VWAPSet           // anchored VWAP (daily, weekly, swing anchors) — nil if insufficient volume data
	Delta      *DeltaResult       // tick-rule estimated delta (cumulative buy/sell pressure) — nil if insufficient data
	SMC        *SMCResult         // Smart Money Concepts: BOS, CHOCH, premium/discount — nil if insufficient data
	Wyckoff    *WyckoffResult     // Wyckoff phase detection — nil if insufficient data (< 20 bars)
}

// TASignal represents a normalized signal from one indicator.
type TASignal struct {
	Indicator string  // e.g. "RSI", "MACD", "EMA_RIBBON"
	Value     float64 // -1.0 (strong bearish) to +1.0 (strong bullish)
	Weight    float64 // configured weight (0 to 1)
	Note      string  // human-readable note (e.g. "RSI oversold at 28")
}

// ---------------------------------------------------------------------------
// Converter Functions
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// WyckoffSummary — lightweight Wyckoff result for embedding in FullResult.
// Avoids circular import with the wyckoff package.
// The caller (handler) converts wyckoff.WyckoffResult → WyckoffSummary.
// ---------------------------------------------------------------------------

// WyckoffSummary captures key Wyckoff analysis data without depending on the
// wyckoff package (which imports ta).
type WyckoffSummary struct {
	Schematic     string     // "ACCUMULATION" | "DISTRIBUTION" | "UNKNOWN"
	CurrentPhase  string     // "A", "B", "C", "D", "E", "UNDEFINED"
	Confidence    string     // "HIGH", "MEDIUM", "LOW"
	TradingRange  [2]float64 // [support, resistance]
	CauseBuilt    float64    // composite cause energy score (0–100)
	ProjectedMove float64    // estimated breakout magnitude in price units
	Summary       string     // narrative summary ≤ 300 chars
}

// DailyPricesToOHLCV converts a slice of domain.DailyPrice to []OHLCV.
// Both input and output are newest-first (index 0 = most recent).
func DailyPricesToOHLCV(prices []domain.DailyPrice) []OHLCV {
	out := make([]OHLCV, len(prices))
	for i, p := range prices {
		out[i] = OHLCV{
			Date:   p.Date,
			Open:   p.Open,
			High:   p.High,
			Low:    p.Low,
			Close:  p.Close,
			Volume: p.Volume,
		}
	}
	return out
}

// IntradayBarsToOHLCV converts a slice of domain.IntradayBar to []OHLCV.
// Both input and output are newest-first (index 0 = most recent).
func IntradayBarsToOHLCV(bars []domain.IntradayBar) []OHLCV {
	out := make([]OHLCV, len(bars))
	for i, b := range bars {
		out[i] = OHLCV{
			Date:   b.Timestamp,
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
			Volume: b.Volume,
		}
	}
	return out
}
