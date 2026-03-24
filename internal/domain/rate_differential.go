package domain

// ---------------------------------------------------------------------------
// Interest Rate Differential & Carry Trade Model
// ---------------------------------------------------------------------------

// RateDifferential holds the interest rate differential for an FX pair.
type RateDifferential struct {
	Currency     string  `json:"currency"`      // e.g. "EUR"
	BaseCurrency string  `json:"base_currency"` // Always "USD" for our pairs
	BaseRate     float64 `json:"base_rate"`     // USD policy rate proxy
	QuoteRate    float64 `json:"quote_rate"`    // Counter-currency rate proxy
	Differential float64 `json:"differential"`  // QuoteRate - BaseRate (positive = carry in favor)
	CarryScore   float64 `json:"carry_score"`   // Normalized -100 to +100
	Direction    string  `json:"direction"`     // "LONG" (positive carry) or "SHORT" (negative carry)
}

// CarryRanking holds the full carry trade attractiveness ranking.
type CarryRanking struct {
	Pairs      []RateDifferential `json:"pairs"`
	USRate     float64            `json:"us_rate"`     // Fed Funds / SOFR proxy
	AsOf       string             `json:"as_of"`       // Date string
	BestCarry  string             `json:"best_carry"`  // Currency with highest positive carry
	WorstCarry string             `json:"worst_carry"` // Currency with most negative carry
}

// CentralBankRateMapping maps currencies to their policy rate proxies.
// These are approximations using available FRED data and market rates.
var CentralBankRateMapping = map[string]CentralBankRateInfo{
	"USD": {Currency: "USD", Name: "Federal Reserve", FREDSeries: "FEDFUNDS", FallbackSeries: "SOFR"},
	"EUR": {Currency: "EUR", Name: "ECB", FREDSeries: "ECBDFR", FallbackSeries: ""},
	"GBP": {Currency: "GBP", Name: "Bank of England", FREDSeries: "BOERUKM", FallbackSeries: ""},
	"JPY": {Currency: "JPY", Name: "Bank of Japan", FREDSeries: "IRSTCI01JPM156N", FallbackSeries: ""},
	"CHF": {Currency: "CHF", Name: "Swiss National Bank", FREDSeries: "IRSTCI01CHM156N", FallbackSeries: ""},
	"AUD": {Currency: "AUD", Name: "Reserve Bank of Australia", FREDSeries: "IRSTCI01AUM156N", FallbackSeries: ""},
	"CAD": {Currency: "CAD", Name: "Bank of Canada", FREDSeries: "IRSTCI01CAM156N", FallbackSeries: ""},
	"NZD": {Currency: "NZD", Name: "Reserve Bank of NZ", FREDSeries: "IRSTCI01NZM156N", FallbackSeries: ""},
}

// CentralBankRateInfo describes how to look up a currency's policy rate.
type CentralBankRateInfo struct {
	Currency       string `json:"currency"`
	Name           string `json:"name"`            // Central bank name
	FREDSeries     string `json:"fred_series"`     // Primary FRED series ID
	FallbackSeries string `json:"fallback_series"` // Backup FRED series ID
}
