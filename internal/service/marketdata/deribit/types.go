// Package deribit provides a public REST client for the Deribit options API.
// No API key is required — all endpoints used here are public.
package deribit

// Instrument represents a single option contract listed on Deribit.
type Instrument struct {
	InstrumentName  string  `json:"instrument_name"`   // e.g. "BTC-28MAR25-80000-C"
	Strike          float64 `json:"strike"`
	OptionType      string  `json:"option_type"`       // "call" | "put"
	ExpirationTS    int64   `json:"expiration_timestamp"` // milliseconds
	ContractSize    float64 `json:"contract_size"`
	IsActive        bool    `json:"is_active"`
}

// BookSummary holds open-interest and volume data for a single instrument.
type BookSummary struct {
	InstrumentName  string  `json:"instrument_name"`
	OpenInterest    float64 `json:"open_interest"`
	Volume          float64 `json:"volume"`
	MarkPrice       float64 `json:"mark_price"`
	MarkIV          float64 `json:"mark_iv"`              // implied volatility of mark price (annualised %)
	UnderlyingPrice float64 `json:"underlying_price"`
	ExpirationTS    int64   `json:"expiration_timestamp"` // milliseconds, 0 if absent
}

// Ticker holds per-instrument Greeks and mark data.
type Ticker struct {
	InstrumentName string  `json:"instrument_name"`
	Delta          float64 `json:"delta"`
	Gamma          float64 `json:"gamma"`
	Vega           float64 `json:"vega"`
	MarkPrice      float64 `json:"mark_price"`
	UnderlyingPrice float64 `json:"underlying_price"`
}

// instrumentsResult is the raw API envelope for get_instruments.
type instrumentsResult struct {
	Result []Instrument `json:"result"`
}

// bookSummaryResult is the raw API envelope for get_book_summary_by_currency.
type bookSummaryResult struct {
	Result []BookSummary `json:"result"`
}

// tickerResult is the raw API envelope for get_ticker.
type tickerResult struct {
	Result Ticker `json:"result"`
}

// IndexPrice holds the Deribit index price for a currency.
type IndexPrice struct {
	IndexPrice float64 `json:"index_price"`
}

// indexPriceResult is the raw API envelope for get_index_price.
type indexPriceResult struct {
	Result IndexPrice `json:"result"`
}
