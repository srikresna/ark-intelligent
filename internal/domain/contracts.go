package domain

// ---------------------------------------------------------------------------
// CFTC Contract Code Constants
// ---------------------------------------------------------------------------
// Centralized contract codes for COT (Commitment of Traders) reporting.
// Source: CFTC Socrata API (https://publicreporting.cftc.gov)

// Forex contracts
const (
	ContractEUR = "099741" // Euro FX (CME 6E)
	ContractGBP = "096742" // British Pound (CME 6B)
	ContractJPY = "097741" // Japanese Yen (CME 6J)
	ContractAUD = "232741" // Australian Dollar (CME 6A)
	ContractCAD = "090741" // Canadian Dollar (CME 6C)
	ContractCHF = "092741" // Swiss Franc (CME 6S)
	ContractNZD = "112741" // New Zealand Dollar (CME 6N)
	ContractDXY = "098662" // US Dollar Index (ICE DX)
)

// Commodity contracts
const (
	ContractGOLD   = "088691" // Gold (COMEX GC)
	ContractSILVER = "084691" // Silver (COMEX SI)
	ContractCOPPER = "085692" // Copper (COMEX HG)
	ContractOIL    = "067651" // Crude Oil WTI (NYMEX CL)
	ContractULSD   = "022651" // Ultra Low Sulfur Diesel (NYMEX HO)
	ContractRBOB   = "111659" // RBOB Gasoline (NYMEX RB)
)

// Bond contracts
const (
	ContractBOND2  = "042601" // 2-Year Treasury Note
	ContractBOND5  = "044601" // 5-Year Treasury Note
	ContractBOND10 = "043602" // 10-Year Treasury Note
	ContractBOND30 = "020601" // 30-Year Treasury Bond
)

// Index contracts
const (
	ContractSPX = "13874A" // S&P 500 (CME ES)
	ContractNDX = "209742" // Nasdaq 100 (CME NQ)
	ContractDJI = "124601" // Dow Jones (CBOT YM)
	ContractRUT = "239742" // Russell 2000 (CME RTY)
)

// Crypto contracts
const (
	ContractBTC = "133741" // Bitcoin (CME BTC)
	ContractETH = "146021" // Ethereum (CME ETH)
)

// ContractToFriendly maps CFTC contract codes to human-readable labels.
var ContractToFriendly = map[string]string{
	ContractEUR:    "EUR",
	ContractGBP:    "GBP",
	ContractJPY:    "JPY",
	ContractCHF:    "CHF",
	ContractAUD:    "AUD",
	ContractCAD:    "CAD",
	ContractNZD:    "NZD",
	ContractDXY:    "USD",
	ContractGOLD:   "GOLD",
	ContractSILVER: "SILVER",
	ContractCOPPER: "COPPER",
	ContractOIL:    "OIL",
	ContractULSD:   "ULSD",
	ContractRBOB:   "RBOB",
	ContractBOND10: "BOND10",
	ContractBOND30: "BOND30",
	ContractBOND5:  "BOND5",
	ContractBOND2:  "BOND2",
	ContractSPX:    "SPX",
	ContractNDX:    "NDX",
	ContractDJI:    "DJI",
	ContractRUT:    "RUT",
	ContractBTC:    "BTC",
	ContractETH:    "ETH",
}

// CurrencyToContractMap maps currency short names to CFTC contract codes.
// Note: domain.CurrencyToContract(currency) function also exists for DefaultCOTContracts lookup.
var CurrencyToContractMap = map[string]string{
	"EUR":  ContractEUR,
	"GBP":  ContractGBP,
	"JPY":  ContractJPY,
	"AUD":  ContractAUD,
	"CAD":  ContractCAD,
	"CHF":  ContractCHF,
	"NZD":  ContractNZD,
	"USD":  ContractDXY,
	"DXY":  ContractDXY,
	"GOLD": ContractGOLD,
	"XAU":  ContractGOLD,
	"OIL":  ContractOIL,
	"BTC":  ContractBTC,
	"ETH":  ContractETH,
}

// CoreForexContracts is the ordered list of primary forex contracts for COT overview.
var CoreForexContracts = []string{
	ContractDXY, ContractEUR, ContractGBP, ContractJPY,
	ContractAUD, ContractNZD, ContractCAD, ContractCHF,
}
