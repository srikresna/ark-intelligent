package domain

// ---------------------------------------------------------------------------
// CFTC Contract Definitions
// ---------------------------------------------------------------------------
// Single source of truth for all CFTC Commitment of Traders contract codes.
// If CFTC changes a code or we add a new instrument, edit here only.

// ContractCode is a CFTC contract code string.
type ContractCode string

// Known CFTC contract codes.
const (
	ContractEUR   ContractCode = "099741"
	ContractGBP   ContractCode = "096742"
	ContractJPY   ContractCode = "097741"
	ContractAUD   ContractCode = "232741"
	ContractNZD   ContractCode = "112741"
	ContractCAD   ContractCode = "090741"
	ContractCHF   ContractCode = "092741"
	ContractDXY   ContractCode = "098662"
	ContractGold  ContractCode = "088691"
	ContractOil   ContractCode = "067651"
	ContractSilver ContractCode = "084691"
	ContractCopper ContractCode = "085692"
	ContractSP500 ContractCode = "13874+"
	ContractNasdaq ContractCode = "209742"
	ContractBTC   ContractCode = "133741"
)

// ContractInfo holds display metadata for a CFTC contract.
type ContractInfo struct {
	Code     ContractCode
	Currency string // Short ticker / symbol (e.g. "EUR", "GOLD")
	Name     string // Full name (e.g. "EURO FX")
	Emoji    string
}

// AllContracts is the canonical ordered list of all tracked contracts.
var AllContracts = []ContractInfo{
	{ContractEUR, "EUR", "EURO FX", "🇪🇺"},
	{ContractGBP, "GBP", "BRITISH POUND", "🇬🇧"},
	{ContractJPY, "JPY", "JAPANESE YEN", "🇯🇵"},
	{ContractAUD, "AUD", "AUSTRALIAN DOLLAR", "🇦🇺"},
	{ContractNZD, "NZD", "NEW ZEALAND DOLLAR", "🇳🇿"},
	{ContractCAD, "CAD", "CANADIAN DOLLAR", "🇨🇦"},
	{ContractCHF, "CHF", "SWISS FRANC", "🇨🇭"},
	{ContractDXY, "DXY", "US DOLLAR INDEX", "🇺🇸"},
	{ContractGold, "GOLD", "GOLD", "🥇"},
	{ContractOil, "OIL", "CRUDE OIL, LIGHT SWEET", "🛢"},
	{ContractSilver, "SILVER", "SILVER", "🥈"},
	{ContractCopper, "COPPER", "COPPER", "🔶"},
}

// FXContracts is the subset of forex contracts (excluding commodities/indices).
var FXContracts = []ContractInfo{
	{ContractEUR, "EUR", "EURO FX", "🇪🇺"},
	{ContractGBP, "GBP", "BRITISH POUND", "🇬🇧"},
	{ContractJPY, "JPY", "JAPANESE YEN", "🇯🇵"},
	{ContractAUD, "AUD", "AUSTRALIAN DOLLAR", "🇦🇺"},
	{ContractNZD, "NZD", "NEW ZEALAND DOLLAR", "🇳🇿"},
	{ContractCAD, "CAD", "CANADIAN DOLLAR", "🇨🇦"},
	{ContractCHF, "CHF", "SWISS FRANC", "🇨🇭"},
	{ContractDXY, "DXY", "US DOLLAR INDEX", "🇺🇸"},
}

// ContractByCurrency maps currency ticker → ContractInfo for fast lookup.
var ContractByCurrency = func() map[string]ContractInfo {
	m := make(map[string]ContractInfo, len(AllContracts))
	for _, c := range AllContracts {
		m[c.Currency] = c
	}
	return m
}()

// ContractByCode maps CFTC code → ContractInfo for fast lookup.
var ContractByCode = func() map[ContractCode]ContractInfo {
	m := make(map[ContractCode]ContractInfo, len(AllContracts))
	for _, c := range AllContracts {
		m[c.Code] = c
	}
	return m
}()

// CurrencyForCode returns the short ticker for a CFTC code, or the code itself as fallback.
func CurrencyForCode(code string) string {
	if info, ok := ContractByCode[ContractCode(code)]; ok {
		return info.Currency
	}
	return code
}

// CodeForCurrency returns the CFTC code for a currency ticker, or empty string if not found.
func CodeForCurrency(currency string) string {
	if info, ok := ContractByCurrency[currency]; ok {
		return string(info.Code)
	}
	return ""
}
