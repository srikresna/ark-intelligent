package domain

// ---------------------------------------------------------------------------
// Carry Trade Monitor & Unwind Detector
// ---------------------------------------------------------------------------

// CarryPairSnapshot holds point-in-time carry data for a single FX pair.
type CarryPairSnapshot struct {
	Currency     string  `json:"currency"`       // e.g. "AUD"
	Spread       float64 `json:"spread"`         // annualized rate differential (bps)
	DailyAccrual float64 `json:"daily_accrual"`  // daily carry earned (bps)
	Direction    string  `json:"direction"`       // "LONG" or "SHORT"
}

// UnwindRisk classifies the current carry unwind danger level.
type UnwindRisk string

const (
	UnwindNormal   UnwindRisk = "NORMAL"
	UnwindNarrow   UnwindRisk = "NARROWING"
	UnwindAlert    UnwindRisk = "UNWIND"
)

// CarryMonitorResult holds the full carry trade dashboard output.
type CarryMonitorResult struct {
	Pairs       []CarryPairSnapshot `json:"pairs"`        // ranked by attractiveness
	SpreadRange float64             `json:"spread_range"` // max spread - min spread (bps)
	PrevRange   float64             `json:"prev_range"`   // previous week's range for comparison
	RangeChange float64             `json:"range_change"` // percentage change in range
	Risk        UnwindRisk          `json:"risk"`         // unwind risk classification
	BestCarry   string              `json:"best_carry"`
	WorstCarry  string              `json:"worst_carry"`
	AsOf        string              `json:"as_of"`
}
