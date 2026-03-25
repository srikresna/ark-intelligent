package price

// ---------------------------------------------------------------------------
// API Response Types — Private structs for JSON deserialization
// ---------------------------------------------------------------------------

// --- Twelve Data ---

type twelveDataResponse struct {
	Meta   twelveDataMeta    `json:"meta"`
	Values []twelveDataValue `json:"values"`
	Status string            `json:"status"`
	Code   int               `json:"code,omitempty"`
	Msg    string            `json:"message,omitempty"`
}

type twelveDataMeta struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Type     string `json:"type"`
}

type twelveDataValue struct {
	Datetime string `json:"datetime"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
}

// --- Alpha Vantage FX Weekly ---

type avFXWeeklyResponse struct {
	MetaData   map[string]string                `json:"Meta Data"`
	TimeSeries map[string]avFXWeeklyOHLC        `json:"Time Series FX (Weekly)"`
	Note       string                           `json:"Note,omitempty"` // Rate limit message
	Info       string                           `json:"Information,omitempty"`
}

type avFXWeeklyOHLC struct {
	Open  string `json:"1. open"`
	High  string `json:"2. high"`
	Low   string `json:"3. low"`
	Close string `json:"4. close"`
}

// --- Alpha Vantage Commodity/Treasury ---

type avCommodityResponse struct {
	Name     string            `json:"name"`
	Interval string            `json:"interval"`
	Unit     string            `json:"unit"`
	Data     []avCommodityData `json:"data"`
	Note     string            `json:"Note,omitempty"`
	Info     string            `json:"Information,omitempty"`
}

type avCommodityData struct {
	Date  string `json:"date"`
	Value string `json:"value"`
}

// --- Yahoo Finance ---

type yahooChartResponse struct {
	Chart struct {
		Result []yahooChartResult `json:"result"`
		Error  *yahooError        `json:"error"`
	} `json:"chart"`
}

type yahooChartResult struct {
	Meta struct {
		Symbol             string  `json:"symbol"`
		Currency           string  `json:"currency"`
		InstrumentType     string  `json:"instrumentType"`
		RegularMarketPrice float64 `json:"regularMarketPrice"`
	} `json:"meta"`
	Timestamp  []int64 `json:"timestamp"`
	Indicators struct {
		Quote []yahooQuote `json:"quote"`
	} `json:"indicators"`
}

type yahooQuote struct {
	Open   []*float64 `json:"open"`
	High   []*float64 `json:"high"`
	Low    []*float64 `json:"low"`
	Close  []*float64 `json:"close"`
	Volume []*float64 `json:"volume"`
}

type yahooError struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

