package domain

import "time"

// ---------------------------------------------------------------------------
// Price Record — Weekly OHLC from external APIs
// ---------------------------------------------------------------------------

// PriceRecord represents a single weekly OHLC price bar.
type PriceRecord struct {
	ContractCode string    `json:"contract_code"` // CFTC code (e.g. "099741")
	Symbol       string    `json:"symbol"`        // Display symbol (e.g. "EUR/USD")
	Date         time.Time `json:"date"`          // Week ending date
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	Close        float64   `json:"close"`
	Source       string    `json:"source"` // "twelvedata", "alphavantage", "yahoo"
}

// WeeklyChange returns the percentage change from open to close.
func (p *PriceRecord) WeeklyChange() float64 {
	if p.Open == 0 {
		return 0
	}
	return (p.Close - p.Open) / p.Open * 100
}

// WeeklyRange returns the normalized weekly range (High-Low)/Close as percentage.
func (p *PriceRecord) WeeklyRange() float64 {
	if p.Close == 0 {
		return 0
	}
	return (p.High - p.Low) / p.Close * 100
}

// ---------------------------------------------------------------------------
// Price Context — Computed from recent PriceRecords
// ---------------------------------------------------------------------------

// PriceContext holds current price context for a contract,
// computed from recent PriceRecords for display and analysis.
type PriceContext struct {
	ContractCode  string  `json:"contract_code"`
	Currency      string  `json:"currency"`
	CurrentPrice  float64 `json:"current_price"`
	WeeklyChgPct  float64 `json:"weekly_chg_pct"`  // 1-week % change
	MonthlyChgPct float64 `json:"monthly_chg_pct"` // 4-week % change
	Trend4W       string  `json:"trend_4w"`         // "UP", "DOWN", "FLAT"
	Trend13W      string  `json:"trend_13w"`        // "UP", "DOWN", "FLAT"
	PriceMA4W     float64 `json:"price_ma_4w"`      // 4-week simple moving average
	PriceMA13W    float64 `json:"price_ma_13w"`     // 13-week simple moving average
	AboveMA4W     bool    `json:"above_ma_4w"`      // Price above 4W MA
	AboveMA13W    bool    `json:"above_ma_13w"`     // Price above 13W MA

	// Price regime classification
	PriceRegime string  `json:"price_regime,omitempty"` // TRENDING, RANGING, CRISIS
	ADX         float64 `json:"adx,omitempty"`          // Approximated directional index

	// ATR-based volatility context (nil if insufficient price data).
	VolatilityRegime     string  `json:"volatility_regime,omitempty"`      // EXPANDING, CONTRACTING, NORMAL
	ATR                  float64 `json:"atr,omitempty"`                    // 20-week Average True Range
	NormalizedATR        float64 `json:"normalized_atr,omitempty"`         // ATR / Close * 100
	VolatilityMultiplier float64 `json:"volatility_multiplier,omitempty"`  // Confidence multiplier from ATR regime
}

// MATrend returns a summary of MA alignment.
// "BULLISH" if price > MA4W > MA13W, "BEARISH" if price < MA4W < MA13W, else "MIXED".
func (pc *PriceContext) MATrend() string {
	if pc.CurrentPrice > pc.PriceMA4W && pc.PriceMA4W > pc.PriceMA13W {
		return "BULLISH"
	}
	if pc.CurrentPrice < pc.PriceMA4W && pc.PriceMA4W < pc.PriceMA13W {
		return "BEARISH"
	}
	return "MIXED"
}

// ---------------------------------------------------------------------------
// Price Symbol Mapping — Maps COT contracts to API symbols
// ---------------------------------------------------------------------------

// AlphaVantageSpec holds API-specific function and parameters for Alpha Vantage.
type AlphaVantageSpec struct {
	Function   string // "FX_WEEKLY", "WTI", "TREASURY_YIELD", "GOLD_SILVER_SPOT"
	FromSymbol string // e.g. "EUR" (for FX_WEEKLY only)
	ToSymbol   string // e.g. "USD" (for FX_WEEKLY only)
}

// PriceSymbolMapping maps a COT contract to its price API symbols across providers.
type PriceSymbolMapping struct {
	ContractCode string
	Currency     string
	TwelveData   string           // Empty if not available on free tier
	AlphaVantage AlphaVantageSpec // Empty Function if not available
	Yahoo        string           // Fallback — always available
	Inverse      bool             // true for USD/JPY, USD/CHF, USD/CAD, DXY
	RiskOnly     bool             // true for VIX, SPX — not COT contracts, used for risk filter only
}

// DefaultPriceSymbolMappings maps all 11 tracked COT contracts to price API symbols.
var DefaultPriceSymbolMappings = []PriceSymbolMapping{
	{ContractCode: "099741", Currency: "EUR", TwelveData: "EUR/USD", AlphaVantage: AlphaVantageSpec{"FX_WEEKLY", "EUR", "USD"}, Yahoo: "EURUSD=X"},
	{ContractCode: "096742", Currency: "GBP", TwelveData: "GBP/USD", AlphaVantage: AlphaVantageSpec{"FX_WEEKLY", "GBP", "USD"}, Yahoo: "GBPUSD=X"},
	{ContractCode: "097741", Currency: "JPY", TwelveData: "USD/JPY", AlphaVantage: AlphaVantageSpec{"FX_WEEKLY", "USD", "JPY"}, Yahoo: "JPY=X", Inverse: true},
	{ContractCode: "092741", Currency: "CHF", TwelveData: "USD/CHF", AlphaVantage: AlphaVantageSpec{"FX_WEEKLY", "USD", "CHF"}, Yahoo: "USDCHF=X", Inverse: true},
	{ContractCode: "232741", Currency: "AUD", TwelveData: "AUD/USD", AlphaVantage: AlphaVantageSpec{"FX_WEEKLY", "AUD", "USD"}, Yahoo: "AUDUSD=X"},
	{ContractCode: "090741", Currency: "CAD", TwelveData: "USD/CAD", AlphaVantage: AlphaVantageSpec{"FX_WEEKLY", "USD", "CAD"}, Yahoo: "USDCAD=X", Inverse: true},
	{ContractCode: "112741", Currency: "NZD", TwelveData: "NZD/USD", AlphaVantage: AlphaVantageSpec{"FX_WEEKLY", "NZD", "USD"}, Yahoo: "NZDUSD=X"},
	{ContractCode: "098662", Currency: "USD", TwelveData: "", AlphaVantage: AlphaVantageSpec{}, Yahoo: "DX-Y.NYB", Inverse: true},
	{ContractCode: "088691", Currency: "XAU", TwelveData: "XAU/USD", AlphaVantage: AlphaVantageSpec{"GOLD_SILVER_SPOT", "", ""}, Yahoo: "GC=F"},
	{ContractCode: "067651", Currency: "OIL", TwelveData: "", AlphaVantage: AlphaVantageSpec{"WTI", "", ""}, Yahoo: "CL=F"},
	{ContractCode: "043602", Currency: "BOND", TwelveData: "", AlphaVantage: AlphaVantageSpec{"TREASURY_YIELD", "", ""}, Yahoo: "ZN=F"},
	// Risk sentiment instruments — not COT contracts, fetched for VIX/SPX risk filter only.
	// ContractCode uses synthetic prefix "risk_" to avoid collision with CFTC codes.
	{ContractCode: "risk_VIX", Currency: "VIX", TwelveData: "", AlphaVantage: AlphaVantageSpec{}, Yahoo: "^VIX", RiskOnly: true},
	{ContractCode: "risk_SPX", Currency: "SPX", TwelveData: "", AlphaVantage: AlphaVantageSpec{}, Yahoo: "^GSPC", RiskOnly: true},
}

// COTPriceSymbolMappings returns only the COT-contract mappings (excludes risk-only instruments).
func COTPriceSymbolMappings() []PriceSymbolMapping {
	var out []PriceSymbolMapping
	for _, m := range DefaultPriceSymbolMappings {
		if !m.RiskOnly {
			out = append(out, m)
		}
	}
	return out
}

// RiskPriceSymbolMappings returns only the risk-sentiment mappings (VIX, SPX).
func RiskPriceSymbolMappings() []PriceSymbolMapping {
	var out []PriceSymbolMapping
	for _, m := range DefaultPriceSymbolMappings {
		if m.RiskOnly {
			out = append(out, m)
		}
	}
	return out
}

// FindPriceMapping returns the PriceSymbolMapping for a COT contract code.
// Returns nil if not found.
func FindPriceMapping(contractCode string) *PriceSymbolMapping {
	for i := range DefaultPriceSymbolMappings {
		if DefaultPriceSymbolMappings[i].ContractCode == contractCode {
			return &DefaultPriceSymbolMappings[i]
		}
	}
	return nil
}

// FindPriceMappingByCurrency returns the PriceSymbolMapping for a currency code.
// Returns nil if not found.
func FindPriceMappingByCurrency(currency string) *PriceSymbolMapping {
	for i := range DefaultPriceSymbolMappings {
		if DefaultPriceSymbolMappings[i].Currency == currency {
			return &DefaultPriceSymbolMappings[i]
		}
	}
	return nil
}
