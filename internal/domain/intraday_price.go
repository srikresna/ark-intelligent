package domain

import "time"

// ---------------------------------------------------------------------------
// Intraday Price Record — 4H/1H OHLCV from external APIs
// ---------------------------------------------------------------------------

// IntradayBar represents a single intraday OHLCV bar (4H or 1H).
type IntradayBar struct {
	ContractCode string    `json:"contract_code"` // CFTC code (e.g. "099741")
	Symbol       string    `json:"symbol"`        // Display symbol (e.g. "EUR/USD")
	Interval     string    `json:"interval"`      // "4h" or "1h"
	Timestamp    time.Time `json:"timestamp"`     // Bar open time (UTC)
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	Close        float64   `json:"close"`
	Volume       float64   `json:"volume,omitempty"`
	Source       string    `json:"source"` // "twelvedata", "yahoo"
}

// ---------------------------------------------------------------------------
// Intraday Context — Computed from recent IntradayBar records
// ---------------------------------------------------------------------------

// IntradayContext holds 4H-granularity price context for a contract.
type IntradayContext struct {
	ContractCode string    `json:"contract_code"`
	Currency     string    `json:"currency"`
	Interval     string    `json:"interval"` // "4h"
	CurrentPrice float64   `json:"current_price"`
	AsOf         time.Time `json:"as_of"` // Timestamp of latest bar

	// Short-term changes
	Chg4H  float64 `json:"chg_4h"`  // Last bar % change
	Chg12H float64 `json:"chg_12h"` // 3-bar % change (12h)
	Chg24H float64 `json:"chg_24h"` // 6-bar % change (24h)

	// Intraday Moving Averages (period = number of 4H bars)
	IMA8  float64 `json:"ima_8"`  // 8-bar SMA  (~32h / 1.3 days)
	IMA21 float64 `json:"ima_21"` // 21-bar SMA (~3.5 days)
	IMA55 float64 `json:"ima_55"` // 55-bar SMA (~9 days)

	AboveIMA8  bool `json:"above_ima_8"`
	AboveIMA21 bool `json:"above_ima_21"`
	AboveIMA55 bool `json:"above_ima_55"`

	// Intraday ATR (14-bar)
	IntradayATR    float64 `json:"intraday_atr"`
	NormalizedIATR float64 `json:"normalized_iatr"` // ATR / Price * 100

	// Short-term trend (last 6 bars = 24h)
	IntradayTrend string `json:"intraday_trend"` // "UP", "DOWN", "FLAT"

	// Momentum
	Momentum6  float64 `json:"momentum_6"`  // ROC 6-bar (24h)
	Momentum12 float64 `json:"momentum_12"` // ROC 12-bar (48h)

	// Session context
	SessionHigh float64 `json:"session_high"` // Current day high
	SessionLow  float64 `json:"session_low"`  // Current day low
}

// IntradayMATrend returns intraday MA alignment.
// "BULLISH" if price > IMA8 > IMA21 > IMA55, "BEARISH" if reversed, else "MIXED".
func (ic *IntradayContext) IntradayMATrend() string {
	if ic.CurrentPrice > ic.IMA8 && ic.IMA8 > ic.IMA21 && ic.IMA21 > ic.IMA55 && ic.IMA55 > 0 {
		return "BULLISH"
	}
	if ic.CurrentPrice < ic.IMA8 && ic.IMA8 < ic.IMA21 && ic.IMA21 < ic.IMA55 && ic.IMA55 > 0 {
		return "BEARISH"
	}
	return "MIXED"
}
