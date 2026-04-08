package domain

import "time"

// ---------------------------------------------------------------------------
// Persisted Signal — Signal snapshot for backtesting
// ---------------------------------------------------------------------------

// PersistedSignal stores a signal snapshot at generation time,
// including the price at the moment of detection and eventual outcomes.
type PersistedSignal struct {
	// Signal identity
	ContractCode string  `json:"contract_code"`
	Currency     string  `json:"currency"`
	SignalType   string  `json:"signal_type"` // e.g. "SMART_MONEY"
	Direction    string  `json:"direction"`   // "BULLISH" or "BEARISH"
	Strength     int     `json:"strength"`    // 1-5
	Confidence   float64 `json:"confidence"`  // 0-100
	Description  string  `json:"description"`

	// Timing
	ReportDate time.Time `json:"report_date"` // COT report date
	DetectedAt time.Time `json:"detected_at"` // When signal was generated

	// Price at detection
	EntryPrice float64 `json:"entry_price"` // Close price on detection week
	Inverse    bool    `json:"inverse"`     // Whether pair is inverse (USD/JPY etc.)

	// Context at detection (for later analysis)
	SentimentScore  float64 `json:"sentiment_score,omitempty"`
	COTIndex        float64 `json:"cot_index,omitempty"`
	ConvictionScore float64 `json:"conviction_score,omitempty"`
	FREDRegime      string  `json:"fred_regime,omitempty"`

	// Daily trend context at detection (for trend filter analysis)
	DailyTrend    string  `json:"daily_trend,omitempty"`     // "UP", "DOWN", "FLAT" at detection
	DailyMATrend  string  `json:"daily_ma_trend,omitempty"`  // "BULLISH", "BEARISH", "MIXED" (MA alignment)
	DailyTrendAdj float64 `json:"daily_trend_adj,omitempty"` // Confidence adjustment applied (+/- %)
	RawConfidence float64 `json:"raw_confidence,omitempty"`  // Confidence before daily trend adjustment

	// Outcome (populated later by evaluator)
	Price1W float64 `json:"price_1w,omitempty"` // Close price +1 week
	Price2W float64 `json:"price_2w,omitempty"` // Close price +2 weeks
	Price4W float64 `json:"price_4w,omitempty"` // Close price +4 weeks

	Return1W float64 `json:"return_1w,omitempty"` // % change from entry
	Return2W float64 `json:"return_2w,omitempty"`
	Return4W float64 `json:"return_4w,omitempty"`

	Outcome1W string `json:"outcome_1w,omitempty"` // "WIN", "LOSS", "PENDING"
	Outcome2W string `json:"outcome_2w,omitempty"`
	Outcome4W string `json:"outcome_4w,omitempty"`

	// Flexible exit outcomes (WIN if target return hit at any point within window)
	MaxFavorableReturn float64 `json:"max_favorable_return,omitempty"` // Best favorable return within 4W
	MaxFavorableDay    int     `json:"max_favorable_day,omitempty"`    // Trading day of best return (1-20)
	FlexOutcome1W      string  `json:"flex_outcome_1w,omitempty"`      // WIN if target hit within 1W
	FlexOutcome2W      string  `json:"flex_outcome_2w,omitempty"`
	FlexOutcome4W      string  `json:"flex_outcome_4w,omitempty"`

	EvaluatedAt time.Time `json:"evaluated_at,omitempty"`
}

// Signal outcome constants.
const (
	OutcomeWin     = "WIN"
	OutcomeLoss    = "LOSS"
	OutcomePending = "PENDING"
	OutcomeExpired = "EXPIRED"
)

// IsFullyEvaluated returns true if all three time horizons have outcomes.
func (s *PersistedSignal) IsFullyEvaluated() bool {
	return s.Outcome1W != "" && s.Outcome1W != OutcomePending &&
		s.Outcome2W != "" && s.Outcome2W != OutcomePending &&
		s.Outcome4W != "" && s.Outcome4W != OutcomePending
}

// NeedsEvaluation returns true if any outcome is still pending or empty
// and enough time has passed for at least one horizon to be evaluable.
func (s *PersistedSignal) NeedsEvaluation(now time.Time) bool {
	if s.EntryPrice == 0 {
		return false // No price data at detection — cannot evaluate
	}
	age := now.Sub(s.ReportDate)
	if age < 7*24*time.Hour {
		return false // Too early for any evaluation
	}
	// Check if any horizon still needs evaluation
	if s.Outcome1W == "" || s.Outcome1W == OutcomePending {
		return true
	}
	if age >= 14*24*time.Hour && (s.Outcome2W == "" || s.Outcome2W == OutcomePending) {
		return true
	}
	if age >= 28*24*time.Hour && (s.Outcome4W == "" || s.Outcome4W == OutcomePending) {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Backtest Statistics — Aggregate metrics
// ---------------------------------------------------------------------------

// BacktestStats holds aggregate statistics for a group of signals.
type BacktestStats struct {
	GroupLabel   string `json:"group_label"`   // e.g. "EUR", "SMART_MONEY", "ALL"
	TotalSignals int    `json:"total_signals"` // Total persisted signals
	Evaluated    int    `json:"evaluated"`     // Signals with at least 1W outcome

	// Per-horizon evaluation counts (may differ if signals are too recent for longer horizons)
	Evaluated1W int `json:"evaluated_1w"`
	Evaluated2W int `json:"evaluated_2w"`
	Evaluated4W int `json:"evaluated_4w"`

	// Win rates by holding period (0-100%)
	WinRate1W float64 `json:"win_rate_1w"`
	WinRate2W float64 `json:"win_rate_2w"`
	WinRate4W float64 `json:"win_rate_4w"`

	// Average returns by holding period (%)
	AvgReturn1W float64 `json:"avg_return_1w"`
	AvgReturn2W float64 `json:"avg_return_2w"`
	AvgReturn4W float64 `json:"avg_return_4w"`

	// Risk metrics
	AvgWinReturn1W  float64 `json:"avg_win_return_1w,omitempty"`  // Avg return on winning trades
	AvgLossReturn1W float64 `json:"avg_loss_return_1w,omitempty"` // Avg return on losing trades (negative)

	// Risk-adjusted performance metrics
	SharpeRatio   float64   `json:"sharpe_ratio,omitempty"`   // Annualized Sharpe ratio (weekly data)
	MaxDrawdown   float64   `json:"max_drawdown,omitempty"`   // Maximum peak-to-trough drawdown (%)
	CalmarRatio   float64   `json:"calmar_ratio,omitempty"`   // Avg annual return / max drawdown
	ProfitFactor  float64   `json:"profit_factor,omitempty"`  // Sum of wins / sum of losses
	ExpectedValue float64   `json:"expected_value,omitempty"` // Expected return per trade (%)
	KellyFraction float64   `json:"kelly_fraction,omitempty"` // Kelly criterion position sizing fraction
	WeeklyReturns []float64 `json:"weekly_returns,omitempty"` // Individual 1W returns for computation

	// Optimal holding period
	BestPeriod  string  `json:"best_period"` // "1W", "2W", "4W"
	BestWinRate float64 `json:"best_win_rate"`

	// Confidence calibration
	AvgConfidence     float64 `json:"avg_confidence"`     // Average stated confidence (0-100)
	ActualAccuracy    float64 `json:"actual_accuracy"`    // Actual win rate at best period
	CalibrationError  float64 `json:"calibration_error"`  // |confidence - accuracy|
	BrierScore        float64 `json:"brier_score"`        // Mean squared error of calibrated predictions (0=perfect, <0.25=good)
	CalibrationMethod string  `json:"calibration_method"` // "Platt" or "WinRate"

	// Strength breakdown
	HighStrengthWinRate float64 `json:"high_strength_win_rate"` // Strength 4-5
	LowStrengthWinRate  float64 `json:"low_strength_win_rate"`  // Strength 1-3
	HighStrengthCount   int     `json:"high_strength_count"`
	LowStrengthCount    int     `json:"low_strength_count"`

	// Statistical significance
	WinRatePValue              float64    `json:"win_rate_p_value"`             // Binomial test p-value (win rate > 50%)
	ReturnTStat                float64    `json:"return_t_stat"`                // One-sample t-test statistic (returns > 0)
	ReturnPValue               float64    `json:"return_p_value"`               // Two-tailed p-value for return t-test
	WinRateCI                  [2]float64 `json:"win_rate_ci"`                  // 95% CI for win rate (percentage points)
	IsStatisticallySignificant bool       `json:"is_statistically_significant"` // WinRatePValue < 0.05
	MinSamplesNeeded           int        `json:"min_samples_needed"`           // Min samples for ±5% precision at 95% CI
}
