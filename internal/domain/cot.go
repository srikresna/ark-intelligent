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
	// --- FX Majors (TFF) ---
	{Code: "099741", Name: "Euro FX", Symbol: "6E", Currency: "EUR", Inverse: false, ReportType: "TFF"},
	{Code: "096742", Name: "British Pound", Symbol: "6B", Currency: "GBP", Inverse: false, ReportType: "TFF"},
	{Code: "097741", Name: "Japanese Yen", Symbol: "6J", Currency: "JPY", Inverse: false, ReportType: "TFF"},
	{Code: "092741", Name: "Swiss Franc", Symbol: "6S", Currency: "CHF", Inverse: false, ReportType: "TFF"},
	{Code: "232741", Name: "Australian Dollar", Symbol: "6A", Currency: "AUD", Inverse: false, ReportType: "TFF"},
	{Code: "090741", Name: "Canadian Dollar", Symbol: "6C", Currency: "CAD", Inverse: false, ReportType: "TFF"},
	{Code: "112741", Name: "NZ Dollar", Symbol: "6N", Currency: "NZD", Inverse: false, ReportType: "TFF"},
	{Code: "098662", Name: "US Dollar Index", Symbol: "DX", Currency: "USD", Inverse: true, ReportType: "TFF"},

	// --- Metals (DISAGGREGATED) ---
	{Code: "088691", Name: "Gold", Symbol: "GC", Currency: "XAU", Inverse: false, ReportType: "DISAGGREGATED"},
	{Code: "084691", Name: "Silver", Symbol: "SI", Currency: "XAG", Inverse: false, ReportType: "DISAGGREGATED"},
	{Code: "085692", Name: "Copper", Symbol: "HG", Currency: "COPPER", Inverse: false, ReportType: "DISAGGREGATED"},

	// --- Energy (DISAGGREGATED) ---
	{Code: "067651", Name: "Crude Oil WTI", Symbol: "CL", Currency: "OIL", Inverse: false, ReportType: "DISAGGREGATED"},
	{Code: "022651", Name: "NY Harbor ULSD", Symbol: "HO", Currency: "ULSD", Inverse: false, ReportType: "DISAGGREGATED"},
	{Code: "111659", Name: "RBOB Gasoline", Symbol: "RB", Currency: "RBOB", Inverse: false, ReportType: "DISAGGREGATED"},

	// --- Bonds (TFF) ---
	{Code: "043602", Name: "10-Year T-Note", Symbol: "ZN", Currency: "BOND", Inverse: false, ReportType: "TFF"},
	{Code: "020601", Name: "30-Year T-Bond", Symbol: "ZB", Currency: "BOND30", Inverse: false, ReportType: "TFF"},
	{Code: "044601", Name: "5-Year T-Note", Symbol: "ZF", Currency: "BOND5", Inverse: false, ReportType: "TFF"},
	{Code: "042601", Name: "2-Year T-Note", Symbol: "TU", Currency: "BOND2", Inverse: false, ReportType: "TFF"},

	// --- Equity Indices (TFF) ---
	{Code: "13874A", Name: "S&P 500 E-mini", Symbol: "ES", Currency: "SPX500", Inverse: false, ReportType: "TFF"},
	{Code: "209742", Name: "Nasdaq 100 E-mini", Symbol: "NQ", Currency: "NDX", Inverse: false, ReportType: "TFF"},
	{Code: "124601", Name: "Dow Jones E-mini", Symbol: "YM", Currency: "DJI", Inverse: false, ReportType: "TFF"},
	{Code: "239742", Name: "Russell 2000 E-mini", Symbol: "RTY", Currency: "RUT", Inverse: false, ReportType: "TFF"},

	// --- Crypto (TFF) ---
	{Code: "133741", Name: "Bitcoin", Symbol: "BTC", Currency: "BTC", Inverse: false, ReportType: "TFF"},
	{Code: "146021", Name: "Ether", Symbol: "ETH", Currency: "ETH", Inverse: false, ReportType: "TFF"},
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

	// --- A. TFF (Financials: Currencies/Bonds) ---
	DealerLong    float64 `json:"dealer_long"`
	DealerShort   float64 `json:"dealer_short"`
	AssetMgrLong  float64 `json:"asset_mgr_long"`
	AssetMgrShort float64 `json:"asset_mgr_short"`
	LevFundLong   float64 `json:"lev_fund_long"`
	LevFundShort  float64 `json:"lev_fund_short"`

	// TFF Spread positions (offsetting long+short by same trader)
	DealerSpread   float64 `json:"dealer_spread"`
	AssetMgrSpread float64 `json:"asset_mgr_spread"`
	LevFundSpread  float64 `json:"lev_fund_spread"`
	OtherSpread    float64 `json:"other_spread"`

	// TFF WoW changes (direct from API — more accurate than manual diff)
	DealerLongChg    float64 `json:"dealer_long_chg"`
	DealerShortChg   float64 `json:"dealer_short_chg"`
	AssetMgrLongChg  float64 `json:"asset_mgr_long_chg"`
	AssetMgrShortChg float64 `json:"asset_mgr_short_chg"`
	LevFundLongChg   float64 `json:"lev_fund_long_chg"`
	LevFundShortChg  float64 `json:"lev_fund_short_chg"`
	OIChangeAPI      float64 `json:"oi_change_api"` // Official WoW OI change from CFTC

	// TFF Trader counts (# unique traders per category)
	DealerLongTraders    int `json:"dealer_long_traders"`
	DealerShortTraders   int `json:"dealer_short_traders"`
	AssetMgrLongTraders  int `json:"asset_mgr_long_traders"`
	AssetMgrShortTraders int `json:"asset_mgr_short_traders"`
	LevFundLongTraders   int `json:"lev_fund_long_traders"`
	LevFundShortTraders  int `json:"lev_fund_short_traders"`
	TotalTraders         int `json:"total_traders"`

	// --- B. Disaggregated (Physicals: Gold/Oil) ---
	ProdMercLong      float64 `json:"prod_merc_long"`
	ProdMercShort     float64 `json:"prod_merc_short"`
	SwapDealerLong    float64 `json:"swap_dealer_long"`
	SwapDealerShort   float64 `json:"swap_dealer_short"`
	ManagedMoneyLong  float64 `json:"managed_money_long"`
	ManagedMoneyShort float64 `json:"managed_money_short"`

	// DISAGG Spread positions
	ManagedMoneySpread float64 `json:"managed_money_spread"`
	ProdMercSpread    float64 `json:"prod_merc_spread"`
	SwapDealerSpread  float64 `json:"swap_dealer_spread"`

	// DISAGG WoW changes
	ProdMercLongChg      float64 `json:"prod_merc_long_chg"`
	ProdMercShortChg     float64 `json:"prod_merc_short_chg"`
	SwapLongChg          float64 `json:"swap_long_chg"`
	SwapShortChg         float64 `json:"swap_short_chg"`
	ManagedMoneyLongChg  float64 `json:"managed_money_long_chg"`
	ManagedMoneyShortChg float64 `json:"managed_money_short_chg"`

	// Shared WoW changes (both TFF and DISAGG) — from API
	SmallLongChgAPI  float64 `json:"small_long_chg_api"`
	SmallShortChgAPI float64 `json:"small_short_chg_api"`
	OtherLongChg     float64 `json:"other_long_chg"`
	OtherShortChg    float64 `json:"other_short_chg"`

	// DISAGG Trader counts
	ProdMercLongTraders  int `json:"prod_merc_long_traders"`
	ProdMercShortTraders int `json:"prod_merc_short_traders"`
	MMoneyLongTraders    int `json:"mmoney_long_traders"`
	MMoneyShortTraders   int `json:"mmoney_short_traders"`
	TotalTradersDisag    int `json:"total_traders_disag"`

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

	// NetChange — WoW smart money net change.
	// Populated from API change_in_* fields (preferred) or computed from history.
	NetChange float64 `json:"net_change"`

	// --- E. Options Positions (computed: Combined - FuturesOnly) ---
	HasOptions         bool    `json:"has_options,omitempty"`          // true if options data was computed
	OptionsOI          float64 `json:"opt_oi,omitempty"`              // Options-only open interest
	OptSmartMoneyLong  float64 `json:"opt_smart_money_long,omitempty"`
	OptSmartMoneyShort float64 `json:"opt_smart_money_short,omitempty"`
	OptCommercialLong  float64 `json:"opt_commercial_long,omitempty"`
	OptCommercialShort float64 `json:"opt_commercial_short,omitempty"`

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

// GetTotalSpread returns all spread positions (long+short offsets by same trader).
// Spread positions = true net exposure gap from OI.
func (r *COTRecord) GetTotalSpread(reportType string) float64 {
	if reportType == "TFF" {
		return r.DealerSpread + r.AssetMgrSpread + r.LevFundSpread + r.OtherSpread
	}
	return r.ManagedMoneySpread + r.ProdMercSpread + r.SwapDealerSpread
}

// GetSmartMoneyNetChangeAPI returns API-provided WoW change for smart money.
// Preferred over manual diff — uses official CFTC-computed values.
func (r *COTRecord) GetSmartMoneyNetChangeAPI(reportType string) float64 {
	if reportType == "TFF" {
		return (r.LevFundLongChg) - (r.LevFundShortChg)
	}
	return r.ManagedMoneyLongChg - r.ManagedMoneyShortChg
}

// GetSmallSpecNetChangeAPI returns API-provided WoW change for small speculators.
func (r *COTRecord) GetSmallSpecNetChangeAPI() float64 {
	return r.SmallLongChgAPI - r.SmallShortChgAPI
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

type MomentumDirection string

const (
	MomentumBuilding  MomentumDirection = "BUILDING"
	MomentumUnwinding MomentumDirection = "UNWINDING"
	MomentumStable    MomentumDirection = "STABLE"
	MomentumReversing MomentumDirection = "REVERSING"
)

// ---------------------------------------------------------------------------
// Signal Strength
// ---------------------------------------------------------------------------

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

type COTAnalysis struct {
	Contract   COTContract `json:"contract"`
	ReportDate time.Time   `json:"report_date"`

	// --- A. Core Positioning ---
	NetPosition float64 `json:"net_position"`
	NetChange   float64 `json:"net_change"`

	LevFundNet      float64 `json:"lev_fund_net"`
	ManagedMoneyNet float64 `json:"managed_money_net"`
	CommercialNet   float64 `json:"commercial_net"`
	SmallSpecNet    float64 `json:"small_spec_net"`

	LongShortRatio float64 `json:"long_short_ratio"`
	PctOfOI        float64 `json:"pct_of_oi"`

	NetCommercial float64 `json:"net_commercial"`
	CommPctOfOI   float64 `json:"comm_pct_of_oi"`
	CommLSRatio   float64 `json:"comm_ls_ratio"`
	CommNetChange float64 `json:"comm_net_change"`
	NetSmallSpec  float64 `json:"net_small_spec"`

	// --- B. COT Index & Extremes ---
	COTIndex        float64 `json:"cot_index"`
	COTIndexComm    float64 `json:"cot_index_comm"`
	IsExtremeBull   bool    `json:"is_extreme_bull"`
	IsExtremeBear   bool    `json:"is_extreme_bear"`
	CommExtremeBull bool    `json:"comm_extreme_bull"`
	CommExtremeBear bool    `json:"comm_extreme_bear"`
	WillcoIndex     float64 `json:"willco_index"`

	// --- C. Smart Money vs Dumb Money ---
	CommercialSignal    string `json:"commercial_signal"`
	SpeculatorSignal    string `json:"speculator_signal"`
	SmallSpecSignal     string `json:"small_spec_signal"`
	SmartDumbDivergence bool   `json:"smart_dumb_divergence"`

	// --- D. Open Interest Analysis ---
	OpenInterest      float64 `json:"open_interest"`
	OpenInterestChg   float64 `json:"open_interest_chg"`
	OIPctChange       float64 `json:"oi_pct_change"`
	OITrend           string  `json:"oi_trend"`
	Top4Concentration float64 `json:"top4_concentration"`
	Top4LongPct       float64 `json:"top4_long_pct"`  // Top-4 long % of OI
	Top4ShortPct      float64 `json:"top4_short_pct"` // Top-4 short % of OI
	Top8Concentration float64 `json:"top8_concentration"`
	SpreadPctOfOI     float64 `json:"spread_pct_of_oi"` // Now populated from API spread fields

	// --- E. Momentum & Trend ---
	SpecMomentum4W   float64           `json:"spec_momentum_4w"`
	SpecMomentum8W   float64           `json:"spec_momentum_8w"` // Now used in signal generation
	CommMomentum4W   float64           `json:"comm_momentum_4w"`
	MomentumDir      MomentumDirection `json:"momentum_dir"`
	ConsecutiveWeeks int               `json:"consecutive_weeks"` // Now used in display

	// --- F. Advanced Signals ---
	ShortTermBias  string        `json:"short_term_bias"`
	DivergenceFlag bool          `json:"divergence_flag"`
	CrowdingIndex  float64       `json:"crowding_index"`
	SentimentScore float64       `json:"sentiment_score"`
	SignalStrength SignalStrength `json:"signal_strength"`

	// --- G. Institutional Outlier Alerts (TFF) ---
	AssetMgrZScore float64 `json:"asset_mgr_z_score"`
	AssetMgrAlert  bool    `json:"asset_mgr_alert"`

	// Category Z-Scores (per category WoW change vs 52W mean/stddev)
	DealerZScore       float64 `json:"dealer_z_score"`
	DealerAlert        bool    `json:"dealer_alert"`
	LevFundZScore      float64 `json:"lev_fund_z_score"`
	LevFundAlert       bool    `json:"lev_fund_alert"`
	ManagedMoneyZScore float64 `json:"managed_money_z_score"`
	ManagedMoneyAlert  bool    `json:"managed_money_alert"`
	SwapDealerZScore   float64 `json:"swap_dealer_z_score"`
	SwapDealerAlert    bool    `json:"swap_dealer_alert"`

	// Cross-category divergence signal
	CategoryDivergence     bool   `json:"category_divergence"`      // true if significant divergence detected
	CategoryDivergenceDesc string `json:"category_divergence_desc"` // human-readable description

	// --- H. Trader Concentration (NEW: from traders_* API fields) ---
	// Number of unique traders per category — thin market / crowding detection.
	DealerShortTraders  int     `json:"dealer_short_traders"`  // Low = highly concentrated (risky)
	LevFundLongTraders  int     `json:"lev_fund_long_traders"` // Low = thin consensus
	LevFundShortTraders int     `json:"lev_fund_short_traders"`
	AssetMgrLongTraders int     `json:"asset_mgr_long_traders"`
	MMoneyLongTraders   int     `json:"mmoney_long_traders"`  // DISAGG
	MMoneyShortTraders  int     `json:"mmoney_short_traders"` // DISAGG
	TotalTraders        int     `json:"total_traders"`
	TraderConcentration string  `json:"trader_concentration"` // "THIN", "NORMAL", "DEEP"
	ThinMarketAlert     bool    `json:"thin_market_alert"`    // true if key category < threshold
	ThinMarketDesc      string  `json:"thin_market_desc"`     // e.g. "Only 9 dealers short EUR"

	// --- I. FRED-Adjusted Scores ---
	RegimeAdjustedScore float64 `json:"regime_adjusted_score"`

	// --- J. Options-Derived Metrics ---
	OptionsNetPosition  float64 `json:"opt_net_position,omitempty"`    // Smart money options net
	OptionsPctOfTotalOI float64 `json:"opt_pct_of_total_oi,omitempty"` // Options OI / Total OI * 100
	OptionsSmartBias    string  `json:"opt_smart_bias,omitempty"`      // "CALL-HEAVY", "PUT-HEAVY", "BALANCED"

	// AINarrative is reserved for per-contract AI narrative caching (populated by cache layer).
	AINarrative string `json:"ai_narrative,omitempty"`
}

// ---------------------------------------------------------------------------
// Socrata API Response Mapping
// ---------------------------------------------------------------------------

type SocrataRecord struct {
	ReportDate   string `json:"report_date_as_yyyy_mm_dd"`
	MarketName   string `json:"market_and_exchange_names"`
	ContractCode string `json:"cftc_contract_market_code"`
	OpenInterest string `json:"open_interest_all"`

	// --- A. TFF positions ---
	DealerPositionsLong    string `json:"dealer_positions_long_all"`
	DealerPositionsShort   string `json:"dealer_positions_short_all"`
	DealerPositionsSpread  string `json:"dealer_positions_spread_all"`
	AssetMgrPositionsLong  string `json:"asset_mgr_positions_long"`
	AssetMgrPositionsShort string `json:"asset_mgr_positions_short"`
	AssetMgrPositionsSpread string `json:"asset_mgr_positions_spread"`
	LevMoneyPositionsLong  string `json:"lev_money_positions_long"`
	LevMoneyPositionsShort string `json:"lev_money_positions_short"`
	LevMoneyPositionsSpread string `json:"lev_money_positions_spread"`
	OtherReptSpread        string `json:"other_rept_positions_spread"`

	// TFF WoW changes (CFTC-computed, more accurate than manual diff)
	ChangeDealerLong    string `json:"change_in_dealer_long_all"`
	ChangeDealerShort   string `json:"change_in_dealer_short_all"`
	ChangeAssetMgrLong  string `json:"change_in_asset_mgr_long"`
	ChangeAssetMgrShort string `json:"change_in_asset_mgr_short"`
	ChangeLevMoneyLong  string `json:"change_in_lev_money_long"`
	ChangeLevMoneyShort string `json:"change_in_lev_money_short"`
	ChangeOI            string `json:"change_in_open_interest_all"`

	// TFF Trader counts
	TradersAssetMgrLong  string `json:"traders_asset_mgr_long_all"`
	TradersAssetMgrShort string `json:"traders_asset_mgr_short_all"`
	TradersDealerLong    string `json:"traders_dealer_long_all"`
	TradersDealerShort   string `json:"traders_dealer_short_all"`
	TradersLevMoneyLong  string `json:"traders_lev_money_long_all"`
	TradersLevMoneyShort string `json:"traders_lev_money_short_all"`
	TradersTotAll        string `json:"traders_tot_all"`

	// --- B. Disaggregated positions ---
	ProdMercPositionsLong  string `json:"prod_merc_positions_long"`
	ProdMercPositionsShort string `json:"prod_merc_positions_short"`
	SwapPositionsLong      string `json:"swap_positions_long_all"`
	SwapPositionsShort     string `json:"swap__positions_short_all"` // double underscore API quirk
	MMoneyPositionsLong    string `json:"m_money_positions_long_all"`
	MMoneyPositionsShort   string `json:"m_money_positions_short_all"`
	MMoneyPositionsSpread  string `json:"m_money_positions_spread"`
	ProdMercPositionsSpread string `json:"prod_merc_positions_spread"`
	SwapPositionsSpread     string `json:"swap_positions_spread_all"`

	// DISAGG WoW changes
	ChangeProdMercLong  string `json:"change_in_prod_merc_long"`
	ChangeProdMercShort string `json:"change_in_prod_merc_short"`
	ChangeSwapLong      string `json:"change_in_swap_long_all"`
	ChangeSwapShort     string `json:"change_in_swap_short_all"`
	ChangeMMoneyLong    string `json:"change_in_m_money_long_all"`
	ChangeMMoneyShort   string `json:"change_in_m_money_short_all"`

	// Shared WoW changes (both TFF and DISAGG)
	ChangeNonReptLong  string `json:"change_in_nonrept_long_all"`
	ChangeNonReptShort string `json:"change_in_nonrept_short_all"`
	ChangeOtherReptLong  string `json:"change_in_other_rept_long"`
	ChangeOtherReptShort string `json:"change_in_other_rept_short"`

	// DISAGG Trader counts
	TradersMMoneyLong    string `json:"traders_m_money_long_all"`
	TradersMMoneyShort   string `json:"traders_m_money_short_all"`
	TradersProdMercLong  string `json:"traders_prod_merc_long_all"`
	TradersProdMercShort string `json:"traders_prod_merc_short_all"`
	TradersTotDisag      string `json:"-"` // Mapped manually from TradersTotAll for DISAGG

	// --- C. Shared ---
	OtherReptPositionsLong  string `json:"other_rept_positions_long"`
	OtherReptPositionsShort string `json:"other_rept_positions_short"`
	NonReptPositionsLong    string `json:"nonrept_positions_long_all"`
	NonReptPositionsShort   string `json:"nonrept_positions_short_all"`

	// Concentration
	Top4Long  string `json:"conc_gross_le_4_tdr_long"`
	Top4Short string `json:"conc_gross_le_4_tdr_short"`
	Top8Long  string `json:"conc_gross_le_8_tdr_long"`
	Top8Short string `json:"conc_gross_le_8_tdr_short"`
}
