package domain

import "time"

// ---------------------------------------------------------------------------
// Weekly Report — Signal performance summary
// ---------------------------------------------------------------------------

// SignalResult holds the performance outcome of a single signal.
type SignalResult struct {
	Contract      string // e.g. "EUR"
	SignalType    string // e.g. "SMART_MONEY"
	Direction     string // "BULLISH" or "BEARISH"
	PriceAtSignal float64
	CurrentPrice  float64 // Price1W if evaluated, else EntryPrice
	PipsChange    float64 // Percentage change from entry
	Result        string  // "WIN", "LOSS", "PENDING"
	DetectedAt    time.Time
}

// WeeklyReport aggregates signal performance for a reporting period.
type WeeklyReport struct {
	WeekStart         time.Time
	WeekEnd           time.Time
	Signals           []SignalResult
	Wins              int
	Losses            int
	Pending           int
	WeeklyScore       string  // e.g. "3/5 (60%)"
	RunningAverage52W float64 // 52-week rolling win rate (%)
	BestStreak        int
	CurrentStreak     int
}
