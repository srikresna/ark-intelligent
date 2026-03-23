package domain

import "time"

// ---------------------------------------------------------------------------
// Daily Price Record — Daily OHLCV from external APIs
// ---------------------------------------------------------------------------

// DailyPrice represents a single daily OHLCV price bar.
type DailyPrice struct {
	ContractCode string    `json:"contract_code"` // CFTC code (e.g. "099741")
	Symbol       string    `json:"symbol"`        // Display symbol (e.g. "EUR/USD")
	Date         time.Time `json:"date"`          // Trading date
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	Close        float64   `json:"close"`
	Volume       float64   `json:"volume,omitempty"` // Daily volume (may be 0 for FX)
	Source       string    `json:"source"`           // "twelvedata", "alphavantage", "yahoo"
}

// DailyChange returns the percentage change from open to close.
func (d *DailyPrice) DailyChange() float64 {
	if d.Open == 0 {
		return 0
	}
	return (d.Close - d.Open) / d.Open * 100
}

// DailyRange returns the normalized daily range (High-Low)/Close as percentage.
func (d *DailyPrice) DailyRange() float64 {
	if d.Close == 0 {
		return 0
	}
	return (d.High - d.Low) / d.Close * 100
}

// ---------------------------------------------------------------------------
// Daily Price Context — Computed from recent DailyPrice records
// ---------------------------------------------------------------------------

// DailyPriceContext holds daily-granularity price context for a contract.
type DailyPriceContext struct {
	ContractCode string  `json:"contract_code"`
	Currency     string  `json:"currency"`
	CurrentPrice float64 `json:"current_price"`

	// Daily changes
	DailyChgPct  float64 `json:"daily_chg_pct"`  // 1-day % change
	WeeklyChgPct float64 `json:"weekly_chg_pct"`  // 5-day % change
	MonthlyChgPct float64 `json:"monthly_chg_pct"` // 20-day % change

	// Daily Moving Averages
	DMA20     float64 `json:"dma_20"`       // 20-day SMA
	DMA50     float64 `json:"dma_50"`       // 50-day SMA
	DMA200    float64 `json:"dma_200"`      // 200-day SMA
	AboveDMA20  bool  `json:"above_dma_20"`
	AboveDMA50  bool  `json:"above_dma_50"`
	AboveDMA200 bool  `json:"above_dma_200"`

	// Daily ATR
	DailyATR       float64 `json:"daily_atr"`        // 14-day Average True Range
	NormalizedATR  float64 `json:"normalized_atr"`    // ATR / Close * 100

	// Trend
	DailyTrend     string `json:"daily_trend"`      // "UP", "DOWN", "FLAT" (5-day)
	ConsecDays     int    `json:"consec_days"`       // Consecutive up/down days
	ConsecDir      string `json:"consec_dir"`        // "UP" or "DOWN"

	// Momentum
	Momentum5D  float64 `json:"momentum_5d"`   // Rate of change 5-day
	Momentum10D float64 `json:"momentum_10d"`  // Rate of change 10-day
	Momentum20D float64 `json:"momentum_20d"`  // Rate of change 20-day
}

// MATrendDaily returns a summary of daily MA alignment.
// "BULLISH" if price > DMA20 > DMA50 > DMA200, "BEARISH" if reversed, else "MIXED".
func (dc *DailyPriceContext) MATrendDaily() string {
	if dc.CurrentPrice > dc.DMA20 && dc.DMA20 > dc.DMA50 && dc.DMA50 > dc.DMA200 && dc.DMA200 > 0 {
		return "BULLISH"
	}
	if dc.CurrentPrice < dc.DMA20 && dc.DMA20 < dc.DMA50 && dc.DMA50 < dc.DMA200 && dc.DMA200 > 0 {
		return "BEARISH"
	}
	return "MIXED"
}
