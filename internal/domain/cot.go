package domain

import "time"

// ---------------------------------------------------------------------------
// COT Contract Definitions
// ---------------------------------------------------------------------------

// COTContract defines a tracked CFTC futures contract.
type COTContract struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	Currency   string `json:"currency"`
	Inverse    bool   `json:"inverse"`
	ReportType string `json:"report_type"` // "TFF" or "DISAGGREGATED"
}

var DefaultCOTContracts = []COTContract{
	{Code: "099741", Name: "Euro FX", Symbol: "6E", Currency: "EUR", Inverse: false, ReportType: "TFF"},
	{Code: "096742", Name: "British Pound", Symbol: "6B", Currency: "GBP", Inverse: false, ReportType: "TFF"},
	{Code: "097741", Name: "Japanese Yen", Symbol: "6J", Currency: "JPY", Inverse: false, ReportType: "TFF"},
	{Code: "092741", Name: "Swiss Franc", Symbol: "6S", Currency: "CHF", Inverse: false, ReportType: "TFF"},
	{Code: "232741", Name: "Australian Dollar", Symbol: "6A", Currency: "AUD", Inverse: false, ReportType: "TFF"},
	{Code: "090741", Name: "Canadian Dollar", Symbol: "6C", Currency: "CAD", Inverse: false, ReportType: "TFF"},
	{Code: "112741", Name: "NZ Dollar", Symbol: "6N", Currency: "NZD", Inverse: false, ReportType: "TFF"},
	{Code: "098662", Name: "US Dollar Index", Symbol: "DX", Currency: "USD", Inverse: true, ReportType: "TFF"},
	{Code: "088691", Name: "Gold", Symbol: "GC", Currency: "XAU", Inverse: false, ReportType: "DISAGGREGATED"},
	{Code: "067651", Name: "Crude Oil WTI", Symbol: "CL", Currency: "OIL", Inverse: false, ReportType: "DISAGGREGATED"},
	{Code: "043602", Name: "10-Year T-Note", Symbol: "ZN", Currency: "BOND", Inverse: false, ReportType: "TFF"},
}

// ---------------------------------------------------------------------------
// COT Record — Raw CFTC Data
// ---------------------------------------------------------------------------

// COTRecord represents raw COT data from a single CFTC weekly report.
type COTRecord struct {
	// Identification
	ContractCode string    `json:"contract_code"` // CFTC code
	ContractName string    `json:"contract_name"` // Market name
	ReportDate   time.Time `json:"report_date"`   // Report as-of date (Tuesday)

	// Open Interest
	OpenInterest    float64 `json:"open_interest"`
	OpenInterestOld float64 `json:"open_interest_old"` // Previous week for change calc

	// --- A. TFF (Financials: Currencies/Bonds) ---
	DealerLong    float64 `json:"dealer_long"`
	DealerShort   float64 `json:"dealer_short"`
	AssetMgrLong  float64 `json:"asset_mgr_long"`
	AssetMgrShort float64 `json:"asset_mgr_short"`
	LevFundLong   float64 `json:"lev_fund_long"`
	LevFundShort  float64 `json:"lev_fund_short"`

	// --- B. Disaggregated (Physicals: Gold/Oil) ---
	ProdMercLong      float64 `json:"prod_merc_long"`
	ProdMercShort     float64 `json:"prod_merc_short"`
	SwapDealerLong    float64 `json:"swap_dealer_long"`
	SwapDealerShort   float64 `json:"swap_dealer_short"`
	ManagedMoneyLong  float64 `json:"managed_money_long"`
	ManagedMoneyShort float64 `json:"managed_money_short"`

	// --- C. Non-Reportable (Small Specs - common across all) ---
	SmallLong  float64 `json:"small_long"`
	SmallShort float64 `json:"small_short"`

	// Other Reportables (common)
	OtherLong  float64 `json:"other_long"`
	OtherShort float64 `json:"other_short"`

	// Concentration data (Top traders)
	Top4Long  float64 `json:"top4_long"`
	Top4Short float64 `json:"top4_short"`
	Top8Long  float64 `json:"top8_long"`
	Top8Short float64 `json:"top8_short"`

	// Changes from previous week (for the primary Speculator/Managed Money/Lev Funds category)
	NetChange float64 `json:"net_change"`

	// --- D. Legacy/Fallback Fields (for CSV/Migration) ---
	CommLong         float64 `json:"comm_long,omitempty"`
	CommShort        float64 `json:"comm_short,omitempty"`
	SpecLong         float64 `json:"spec_long,omitempty"`
	SpecShort        float64 `json:"spec_short,omitempty"`
	CommLongChange   float64 `json:"comm_long_change,omitempty"`
	CommShortChange  float64 `json:"comm_short_change,omitempty"`
	SpecLongChange   float64 `json:"spec_long_change,omitempty"`
	SpecShortChange  float64 `json:"spec_short_change,omitempty"`
	SmallLongChange  float64 `json:"small_long_change,omitempty"`
	SmallShortChange float64 `json:"small_short_change,omitempty"`
}

// GetSmartMoneyNet returns the primary speculative position (Lev Funds for TFF, Managed Money for Disaggregated).
func (r *COTRecord) GetSmartMoneyNet(reportType string) float64 {
	if reportType == "TFF" {
		return r.LevFundLong - r.LevFundShort
	}
	// DISAGGREGATED or default
	return r.ManagedMoneyLong - r.ManagedMoneyShort
}

// GetCommercialNet returns the primary commercial/hedging position.
func (r *COTRecord) GetCommercialNet(reportType string) float64 {
	if reportType == "TFF" {
		return r.DealerLong - r.DealerShort
	}
	return r.ProdMercLong - r.ProdMercShort + r.SwapDealerLong - r.SwapDealerShort
}

// GetSmallSpecNet returns the non-reportable position.
func (r *COTRecord) GetSmallSpecNet() float64 {
	return r.SmallLong - r.SmallShort
}

// CurrencyToContract maps a currency code to the CFTC contract code used in COT data.
func CurrencyToContract(currency string) string {
	for _, c := range DefaultCOTContracts {
		if c.Currency == currency {
			return c.Code
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Momentum Direction
// ---------------------------------------------------------------------------

// MomentumDirection indicates the direction and intensity of positioning changes.
type MomentumDirection string

const (
	MomentumBuilding  MomentumDirection = "BUILDING"  // Accelerating in same direction
	MomentumUnwinding MomentumDirection = "UNWINDING" // Reducing positions
	MomentumStable    MomentumDirection = "STABLE"    // Little change
	MomentumReversing MomentumDirection = "REVERSING" // Changing direction
)

// ---------------------------------------------------------------------------
// Signal Strength
// ---------------------------------------------------------------------------

// SignalStrength rates the conviction level of a COT signal.
type SignalStrength string

const (
	SignalStrong   SignalStrength = "STRONG"
	SignalModerate SignalStrength = "MODERATE"
	SignalWeak     SignalStrength = "WEAK"
	SignalNeutral  SignalStrength = "NEUTRAL"
)

// ---------------------------------------------------------------------------
// COT Analysis — Computed Metrics
// ---------------------------------------------------------------------------

// COTAnalysis contains all computed metrics for a single contract.
type COTAnalysis struct {
	// Reference
	Contract   COTContract `json:"contract"`
	ReportDate time.Time   `json:"report_date"`

	// --- A. Core Positioning ---
	// --- A. Core Positioning (Focused on "Smart Money") ---
	// Large Speculator equivalent: Lev Funds (TFF) or Managed Money (Disaggregated)
	NetPosition float64 `json:"net_position"` // Smart Money net
	NetChange   float64 `json:"net_change"`   // WoW change in smart net

	// Breakdown
	LevFundNet      float64 `json:"lev_fund_net"`      // TFF only
	ManagedMoneyNet float64 `json:"managed_money_net"` // Disaggregated only
	CommercialNet   float64 `json:"commercial_net"`    // Dealers (TFF) or Prod/Swap (Disaggregated)
	SmallSpecNet    float64 `json:"small_spec_net"`    // Non-reportable

	LongShortRatio float64 `json:"long_short_ratio"` // Smart Money Ratio
	PctOfOI        float64 `json:"pct_of_oi"`        // Net as % of Open Interest

	// Legacy Field Support (Sync)
	NetCommercial float64 `json:"net_commercial"`  // Same as CommercialNet
	CommPctOfOI   float64 `json:"comm_pct_of_oi"`  // Commercial net as % of OI
	CommLSRatio   float64 `json:"comm_ls_ratio"`   // Commercial Long/Short ratio
	CommNetChange float64 `json:"comm_net_change"` // WoW change in commercial net
	NetSmallSpec  float64 `json:"net_small_spec"`  // Same as SmallSpecNet

	// --- B. COT Index & Extremes ---
	COTIndex        float64 `json:"cot_index"`         // Williams COT Index (0-100) for specs
	COTIndexComm    float64 `json:"cot_index_comm"`    // COT Index for commercials
	IsExtremeBull   bool    `json:"is_extreme_bull"`   // COT Index > 90
	IsExtremeBear   bool    `json:"is_extreme_bear"`   // COT Index < 10
	CommExtremeBull bool    `json:"comm_extreme_bull"` // Commercial COT Index > 90
	CommExtremeBear bool    `json:"comm_extreme_bear"` // Commercial COT Index < 10
	WillcoIndex     float64 `json:"willco_index"`      // EMA-weighted COT Index variant

	// --- C. Smart Money vs Dumb Money ---
	CommercialSignal    string `json:"commercial_signal"`     // Contrarian signal from commercials
	SpeculatorSignal    string `json:"speculator_signal"`     // Trend-following signal from large specs
	SmallSpecSignal     string `json:"small_spec_signal"`     // Contrarian signal from small specs
	SmartDumbDivergence bool   `json:"smart_dumb_divergence"` // Commercial vs Speculator divergence

	// --- D. Open Interest Analysis ---
	OpenInterestChg   float64 `json:"open_interest_chg"`  // Absolute change in OI WoW
	OIPctChange       float64 `json:"oi_pct_change"`      // OI % change week-over-week
	OITrend           string  `json:"oi_trend"`           // OI Trend: RISING, FALLING, FLAT
	Top4Concentration float64 `json:"top4_concentration"` // Top 4 trader dominance %
	Top8Concentration float64 `json:"top8_concentration"` // Top 8 trader dominance %
	SpreadPctOfOI     float64 `json:"spread_pct_of_oi"`   // Spread positions as % of OI

	// --- E. Momentum & Trend ---
	SpecMomentum4W   float64           `json:"spec_momentum_4w"`  // 4-week rate of change of net spec
	SpecMomentum8W   float64           `json:"spec_momentum_8w"`  // 8-week rate of change
	CommMomentum4W   float64           `json:"comm_momentum_4w"`  // 4-week commercial momentum
	MomentumDir      MomentumDirection `json:"momentum_dir"`      // Overall momentum direction
	ConsecutiveWeeks int               `json:"consecutive_weeks"` // Weeks in same direction

	// --- F. Advanced Signals ---
	ShortTermBias  string         `json:"short_term_bias"` // Intra/Swing bias (e.g., BUY DIPS)
	DivergenceFlag bool           `json:"divergence_flag"` // Price vs positioning divergence
	CrowdingIndex  float64        `json:"crowding_index"`  // How one-sided (0-100, >80 = extreme)
	SentimentScore float64        `json:"sentiment_score"` // Weighted composite (-100 to +100)
	SignalStrength SignalStrength `json:"signal_strength"` // Overall signal conviction

	// --- G. Institutional Outlier Alerts (TFF) ---
	AssetMgrZScore float64 `json:"asset_mgr_z_score"` // Z-Score of Asset Mgr Net Position change
	AssetMgrAlert  bool    `json:"asset_mgr_alert"`   // |Z-Score| > 2.0 or threshold

	// --- H. FRED-Adjusted Scores (Gap B) ---
	// RegimeAdjustedScore is SentimentScore adjusted by FRED macro regime multiplier per currency.
	// Range: -100 to +100. Populated after FRED data is available.
	RegimeAdjustedScore float64 `json:"regime_adjusted_score"`

	// AI interpretation (filled by Gemini)
	AINarrative string `json:"ai_narrative,omitempty"`
}

// ---------------------------------------------------------------------------
// Socrata API Response Mapping
// ---------------------------------------------------------------------------

// SocrataRecord maps the CFTC Socrata JSON response fields.
// Used to parse the raw API response before converting to COTRecord.
type SocrataRecord struct {
	ReportDate   string `json:"report_date_as_yyyy_mm_dd"`
	MarketName   string `json:"market_and_exchange_names"`
	ContractCode string `json:"cftc_contract_market_code"`
	OpenInterest string `json:"open_interest_all"`

	// --- A. TFF (Financials) ---
	DealerPositionsLong    string `json:"dealer_positions_long_all"`
	DealerPositionsShort   string `json:"dealer_positions_short_all"`
	AssetMgrPositionsLong  string `json:"asset_mgr_positions_long"`  // TFF format
	AssetMgrPositionsShort string `json:"asset_mgr_positions_short"` // TFF format
	LevMoneyPositionsLong  string `json:"lev_money_positions_long"`  // TFF format
	LevMoneyPositionsShort string `json:"lev_money_positions_short"` // TFF format

	// --- B. Disaggregated (Physicals) ---
	ProdMercPositionsLong  string `json:"prod_merc_positions_long_all"`
	ProdMercPositionsShort string `json:"prod_merc_positions_short_all"`
	SwapPositionsLong      string `json:"swap_positions_long_all"`
	SwapPositionsShort     string `json:"swap_positions_short_all"`
	MMoneyPositionsLong    string `json:"m_money_positions_long_all"`
	MMoneyPositionsShort   string `json:"m_money_positions_short_all"`

	// --- C. Shared ---
	OtherReptPositionsLong  string `json:"other_rept_positions_long_all"`
	OtherReptPositionsShort string `json:"other_rept_positions_short_all"`
	NonReptPositionsLong    string `json:"nonrept_positions_long_all"`
	NonReptPositionsShort   string `json:"nonrept_positions_short_all"`

	// Concentration
	Top4Long  string `json:"pct_of_oi_4_or_less_long_all"`
	Top4Short string `json:"pct_of_oi_4_or_less_short_all"`
	Top8Long  string `json:"pct_of_oi_8_or_less_long_all"`
	Top8Short string `json:"pct_of_oi_8_or_less_short_all"`
}
