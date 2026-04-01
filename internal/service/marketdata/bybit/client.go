// Package bybit provides a REST client for the Bybit V5 API.
// Market data endpoints (orderbook, trades, tickers) are PUBLIC — no auth needed.
// Account endpoints require HMAC-SHA256 signed requests.
//
// Auth headers:
//
//	X-BAPI-API-KEY: your API key
//	X-BAPI-TIMESTAMP: UTC ms timestamp
//	X-BAPI-SIGN: HMAC-SHA256(timestamp+apiKey+recvWindow+queryString)
//	X-BAPI-RECV-WINDOW: 5000 (default)
package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/retry"
)

var log = logger.Component("bybit")

const (
	defaultRestBase   = "https://api.bybit.com"
	defaultRecvWindow = "5000"
	defaultTimeout    = 15 * time.Second
)

// Client is a Bybit V5 REST client.
type Client struct {
	httpClient *http.Client
	apiKey     string
	apiSecret  string
	restBase   string
}

// NewClient creates a Bybit V5 client.
// apiKey and apiSecret are optional — only needed for private endpoints.
// restBase: override the base URL (e.g. testnet URL).
func NewClient(apiKey, apiSecret, restBase string) *Client {
	base := restBase
	if base == "" {
		base = defaultRestBase
	}
	return &Client{
		httpClient: httpclient.NewClient(defaultTimeout),
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		restBase:   base,
	}
}

// IsConfigured returns true if API key and secret are set.
func (c *Client) IsConfigured() bool { return c.apiKey != "" && c.apiSecret != "" }

// ---------------------------------------------------------------------------
// Public REST helpers (no auth needed)
// ---------------------------------------------------------------------------

// getPublic performs a public GET request (no auth header).
func (c *Client) getPublic(ctx context.Context, path string, params url.Values) ([]byte, error) {
	fullURL := c.restBase + path
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	return retry.Do(ctx, func() ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("bybit: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("bybit: http: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("bybit: read body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("bybit: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		}

		// Check Bybit retCode
		var check struct {
			RetCode int    `json:"retCode"`
			RetMsg  string `json:"retMsg"`
		}
		if err := json.Unmarshal(body, &check); err == nil && check.RetCode != 0 {
			return nil, fmt.Errorf("bybit: retCode=%d msg=%s", check.RetCode, check.RetMsg)
		}

		return body, nil
	})
}

// ---------------------------------------------------------------------------
// Orderbook
// ---------------------------------------------------------------------------

// OrderbookLevel represents one price level in the orderbook.
type OrderbookLevel struct {
	Price    float64
	Quantity float64
}

// Orderbook holds a snapshot of the order book.
type Orderbook struct {
	Symbol   string
	Bids     []OrderbookLevel // sorted best (highest) bid first
	Asks     []OrderbookLevel // sorted best (lowest) ask first
	Ts       int64            // server timestamp ms
	UpdateID int64
}

// GetOrderbook fetches a snapshot of the orderbook for a symbol.
// category: "spot", "linear", "inverse"
// symbol: e.g. "BTCUSDT"
// limit: depth per side (1, 25, 50, 100, 200)
func (c *Client) GetOrderbook(ctx context.Context, category, symbol string, limit int) (*Orderbook, error) {
	params := url.Values{
		"category": {category},
		"symbol":   {symbol},
		"limit":    {strconv.Itoa(limit)},
	}
	body, err := c.getPublic(ctx, "/v5/market/orderbook", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			S  string     `json:"s"`
			B  [][]string `json:"b"` // bids: [price, qty]
			A  [][]string `json:"a"` // asks: [price, qty]
			Ts int64      `json:"ts"`
			U  int64      `json:"u"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bybit: parse orderbook: %w", err)
	}

	ob := &Orderbook{
		Symbol:   resp.Result.S,
		Ts:       resp.Result.Ts,
		UpdateID: resp.Result.U,
	}
	for _, b := range resp.Result.B {
		if len(b) < 2 {
			continue
		}
		p, errP := strconv.ParseFloat(b[0], 64)
		q, errQ := strconv.ParseFloat(b[1], 64)
		if errP != nil {
			log.Warn().Str("raw_price", b[0]).Msg("orderbook bid: parse price failed, skipping level")
			continue
		}
		if errQ != nil {
			log.Warn().Str("raw_qty", b[1]).Msg("orderbook bid: parse qty failed, skipping level")
			continue
		}
		if p == 0 || q == 0 {
			log.Warn().Float64("price", p).Float64("qty", q).Msg("orderbook bid: zero price or qty, skipping level")
			continue
		}
		ob.Bids = append(ob.Bids, OrderbookLevel{Price: p, Quantity: q})
	}
	for _, a := range resp.Result.A {
		if len(a) < 2 {
			continue
		}
		p, errP := strconv.ParseFloat(a[0], 64)
		q, errQ := strconv.ParseFloat(a[1], 64)
		if errP != nil {
			log.Warn().Str("raw_price", a[0]).Msg("orderbook ask: parse price failed, skipping level")
			continue
		}
		if errQ != nil {
			log.Warn().Str("raw_qty", a[1]).Msg("orderbook ask: parse qty failed, skipping level")
			continue
		}
		if p == 0 || q == 0 {
			log.Warn().Float64("price", p).Float64("qty", q).Msg("orderbook ask: zero price or qty, skipping level")
			continue
		}
		ob.Asks = append(ob.Asks, OrderbookLevel{Price: p, Quantity: q})
	}
	return ob, nil
}

// ---------------------------------------------------------------------------
// Recent Trades
// ---------------------------------------------------------------------------

// Trade represents a single executed trade.
type Trade struct {
	Symbol     string
	Price      float64
	Qty        float64
	Side       string // "Buy" or "Sell"
	Timestamp  int64  // Unix ms
	IsBuyTaker bool   // true = buyer was taker (aggressive buy)
}

// GetRecentTrades fetches recent public trades.
// limit: max 1000, default 500
func (c *Client) GetRecentTrades(ctx context.Context, category, symbol string, limit int) ([]Trade, error) {
	params := url.Values{
		"category": {category},
		"symbol":   {symbol},
		"limit":    {strconv.Itoa(limit)},
	}
	body, err := c.getPublic(ctx, "/v5/market/recent-trade", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				Symbol string `json:"symbol"`
				Price  string `json:"price"`
				Size   string `json:"size"`
				Side   string `json:"side"` // "Buy" or "Sell"
				Time   string `json:"time"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bybit: parse trades: %w", err)
	}

	trades := make([]Trade, 0, len(resp.Result.List))
	for _, t := range resp.Result.List {
		p, errP := strconv.ParseFloat(t.Price, 64)
		if errP != nil {
			log.Warn().Str("raw_price", t.Price).Str("symbol", t.Symbol).Msg("trade: parse price failed, skipping entry")
			continue
		}
		if p == 0 {
			log.Warn().Str("symbol", t.Symbol).Msg("trade: zero price, skipping entry")
			continue
		}
		q, errQ := strconv.ParseFloat(t.Size, 64)
		if errQ != nil {
			log.Warn().Str("raw_qty", t.Size).Str("symbol", t.Symbol).Msg("trade: parse qty failed, skipping entry")
			continue
		}
		ts, errTs := strconv.ParseInt(t.Time, 10, 64)
		if errTs != nil {
			log.Warn().Str("raw_time", t.Time).Str("symbol", t.Symbol).Msg("trade: parse timestamp failed, skipping entry")
			continue
		}
		trades = append(trades, Trade{
			Symbol:     t.Symbol,
			Price:      p,
			Qty:        q,
			Side:       t.Side,
			Timestamp:  ts,
			IsBuyTaker: t.Side == "Buy",
		})
	}
	return trades, nil
}

// ---------------------------------------------------------------------------
// Tickers
// ---------------------------------------------------------------------------

// Ticker holds market ticker data.
type Ticker struct {
	Symbol            string
	LastPrice         float64
	IndexPrice        float64
	MarkPrice         float64
	PrevPrice24h      float64
	Price24hPcnt      float64
	HighPrice24h      float64
	LowPrice24h       float64
	Volume24h         float64
	Turnover24h       float64
	OpenInterest      float64
	OpenInterestValue float64
	FundingRate       float64
	NextFundingTime   int64
	Ask1Price         float64
	Bid1Price         float64
}

// GetTicker fetches ticker data for a symbol.
func (c *Client) GetTicker(ctx context.Context, category, symbol string) (*Ticker, error) {
	params := url.Values{
		"category": {category},
		"symbol":   {symbol},
	}
	body, err := c.getPublic(ctx, "/v5/market/tickers", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				Symbol            string `json:"symbol"`
				LastPrice         string `json:"lastPrice"`
				IndexPrice        string `json:"indexPrice"`
				MarkPrice         string `json:"markPrice"`
				PrevPrice24h      string `json:"prevPrice24h"`
				Price24hPcnt      string `json:"price24hPcnt"`
				HighPrice24h      string `json:"highPrice24h"`
				LowPrice24h       string `json:"lowPrice24h"`
				Volume24h         string `json:"volume24h"`
				Turnover24h       string `json:"turnover24h"`
				OpenInterest      string `json:"openInterest"`
				OpenInterestValue string `json:"openInterestValue"`
				FundingRate       string `json:"fundingRate"`
				NextFundingTime   string `json:"nextFundingTime"`
				Ask1Price         string `json:"ask1Price"`
				Bid1Price         string `json:"bid1Price"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bybit: parse ticker: %w", err)
	}
	if len(resp.Result.List) == 0 {
		return nil, fmt.Errorf("bybit: no ticker for %s", symbol)
	}
	t := resp.Result.List[0]

	toF := func(s string) float64 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil && s != "" {
			log.Warn().Str("value", s).Err(err).Msg("bybit ticker: float parse failed")
		}
		return v
	}
	toI := func(s string) int64 {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil && s != "" {
			log.Warn().Str("value", s).Err(err).Msg("bybit ticker: int parse failed")
		}
		return v
	}

	return &Ticker{
		Symbol:            t.Symbol,
		LastPrice:         toF(t.LastPrice),
		IndexPrice:        toF(t.IndexPrice),
		MarkPrice:         toF(t.MarkPrice),
		PrevPrice24h:      toF(t.PrevPrice24h),
		Price24hPcnt:      toF(t.Price24hPcnt),
		HighPrice24h:      toF(t.HighPrice24h),
		LowPrice24h:       toF(t.LowPrice24h),
		Volume24h:         toF(t.Volume24h),
		Turnover24h:       toF(t.Turnover24h),
		OpenInterest:      toF(t.OpenInterest),
		OpenInterestValue: toF(t.OpenInterestValue),
		FundingRate:       toF(t.FundingRate),
		NextFundingTime:   toI(t.NextFundingTime),
		Ask1Price:         toF(t.Ask1Price),
		Bid1Price:         toF(t.Bid1Price),
	}, nil
}

// ---------------------------------------------------------------------------
// Kline (OHLCV)
// ---------------------------------------------------------------------------

// Kline holds one OHLCV candlestick.
type Kline struct {
	StartTime int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Turnover  float64
}

// GetKline fetches historical klines/candlesticks.
// interval: "1", "5", "15", "30", "60", "D", "W", "M"
// limit: max 1000
func (c *Client) GetKline(ctx context.Context, category, symbol, interval string, limit int) ([]Kline, error) {
	params := url.Values{
		"category": {category},
		"symbol":   {symbol},
		"interval": {interval},
		"limit":    {strconv.Itoa(limit)},
	}
	body, err := c.getPublic(ctx, "/v5/market/kline", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List [][]string `json:"list"` // [startTime, open, high, low, close, volume, turnover]
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bybit: parse kline: %w", err)
	}

	klines := make([]Kline, 0, len(resp.Result.List))
	for _, row := range resp.Result.List {
		if len(row) < 7 {
			continue
		}
		toF := func(s string) float64 {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil && s != "" {
				log.Warn().Str("value", s).Err(err).Msg("bybit kline: float parse failed")
			}
			return v
		}
		toI := func(s string) int64 {
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil && s != "" {
				log.Warn().Str("value", s).Err(err).Msg("bybit kline: int parse failed")
			}
			return v
		}
		klines = append(klines, Kline{
			StartTime: toI(row[0]),
			Open:      toF(row[1]),
			High:      toF(row[2]),
			Low:       toF(row[3]),
			Close:     toF(row[4]),
			Volume:    toF(row[5]),
			Turnover:  toF(row[6]),
		})
	}
	return klines, nil
}

// ---------------------------------------------------------------------------
// Long-Short Ratio (sentiment)
// ---------------------------------------------------------------------------

// LongShortRatio holds the long/short position ratio at a point in time.
type LongShortRatio struct {
	Symbol    string
	BuyRatio  float64
	SellRatio float64
	Timestamp int64
}

// GetLongShortRatio fetches long-short ratio history for a symbol.
// period: "5min", "15min", "30min", "1h", "4h", "1d"
func (c *Client) GetLongShortRatio(ctx context.Context, category, symbol, period string, limit int) ([]LongShortRatio, error) {
	params := url.Values{
		"category": {category},
		"symbol":   {symbol},
		"period":   {period},
		"limit":    {strconv.Itoa(limit)},
	}
	body, err := c.getPublic(ctx, "/v5/market/account-ratio", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				Symbol    string `json:"symbol"`
				BuyRatio  string `json:"buyRatio"`
				SellRatio string `json:"sellRatio"`
				Timestamp string `json:"timestamp"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bybit: parse long-short: %w", err)
	}

	ratios := make([]LongShortRatio, 0, len(resp.Result.List))
	for _, r := range resp.Result.List {
		buy, errB := strconv.ParseFloat(r.BuyRatio, 64)
		if errB != nil {
			log.Warn().Str("raw_buy_ratio", r.BuyRatio).Str("symbol", r.Symbol).Msg("long-short: parse buyRatio failed, skipping entry")
			continue
		}
		sell, errS := strconv.ParseFloat(r.SellRatio, 64)
		if errS != nil {
			log.Warn().Str("raw_sell_ratio", r.SellRatio).Str("symbol", r.Symbol).Msg("long-short: parse sellRatio failed, skipping entry")
			continue
		}
		ts, errTs := strconv.ParseInt(r.Timestamp, 10, 64)
		if errTs != nil {
			log.Warn().Str("raw_timestamp", r.Timestamp).Str("symbol", r.Symbol).Msg("long-short: parse timestamp failed, skipping entry")
			continue
		}
		ratios = append(ratios, LongShortRatio{
			Symbol:    r.Symbol,
			BuyRatio:  buy,
			SellRatio: sell,
			Timestamp: ts,
		})
	}
	return ratios, nil
}

// ---------------------------------------------------------------------------
// Open Interest History
// ---------------------------------------------------------------------------

// OIData holds open interest at a point in time.
type OIData struct {
	Symbol       string
	OpenInterest float64
	Timestamp    int64
}

// GetOpenInterestHistory fetches open interest history.
// intervalTime: "5min","15min","30min","1h","4h","1d"
func (c *Client) GetOpenInterestHistory(ctx context.Context, category, symbol, intervalTime string, limit int) ([]OIData, error) {
	params := url.Values{
		"category":     {category},
		"symbol":       {symbol},
		"intervalTime": {intervalTime},
		"limit":        {strconv.Itoa(limit)},
	}
	body, err := c.getPublic(ctx, "/v5/market/open-interest", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				OpenInterest string `json:"openInterest"`
				Timestamp    string `json:"timestamp"`
			} `json:"list"`
			Symbol string `json:"symbol"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bybit: parse OI: %w", err)
	}

	data := make([]OIData, 0, len(resp.Result.List))
	for _, r := range resp.Result.List {
		oi, errOI := strconv.ParseFloat(r.OpenInterest, 64)
		if errOI != nil {
			log.Warn().Str("raw_oi", r.OpenInterest).Str("symbol", resp.Result.Symbol).Msg("open-interest: parse openInterest failed, skipping entry")
			continue
		}
		ts, errTs := strconv.ParseInt(r.Timestamp, 10, 64)
		if errTs != nil {
			log.Warn().Str("raw_timestamp", r.Timestamp).Str("symbol", resp.Result.Symbol).Msg("open-interest: parse timestamp failed, skipping entry")
			continue
		}
		data = append(data, OIData{
			Symbol:       resp.Result.Symbol,
			OpenInterest: oi,
			Timestamp:    ts,
		})
	}
	return data, nil
}


// ---------------------------------------------------------------------------
// Funding Rate History
// ---------------------------------------------------------------------------

// FundingRate holds one funding rate settlement record.
type FundingRate struct {
	Symbol               string
	FundingRate          float64
	FundingRateTimestamp int64 // Unix ms
}

// FundingRateStats holds computed funding rate statistics.
type FundingRateStats struct {
	Symbol     string
	Current    float64
	Avg7D      float64
	Avg30D     float64
	Max30D     float64
	Min30D     float64
	Regime     string  // "POSITIVE_BIAS", "NEGATIVE_BIAS", "NEUTRAL"
	Percentile float64 // percentile of current rate in 30d history (0-100)
	History    []FundingRate
}

// GetFundingHistory fetches historical funding rate settlements.
// category: "linear" for USDT perpetuals
// symbol: e.g. "BTCUSDT"
// limit: max 200
func (c *Client) GetFundingHistory(ctx context.Context, category, symbol string, limit int) ([]FundingRate, error) {
	params := url.Values{
		"category": {category},
		"symbol":   {symbol},
		"limit":    {strconv.Itoa(limit)},
	}
	body, err := c.getPublic(ctx, "/v5/market/funding/history", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				Symbol               string `json:"symbol"`
				FundingRate          string `json:"fundingRate"`
				FundingRateTimestamp string `json:"fundingRateTimestamp"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bybit: parse funding history: %w", err)
	}

	rates := make([]FundingRate, 0, len(resp.Result.List))
	for _, r := range resp.Result.List {
		rate, errR := strconv.ParseFloat(r.FundingRate, 64)
		if errR != nil {
			log.Warn().Str("raw_rate", r.FundingRate).Str("symbol", r.Symbol).
				Msg("funding-history: parse rate failed, skipping entry")
			continue
		}
		ts, errTs := strconv.ParseInt(r.FundingRateTimestamp, 10, 64)
		if errTs != nil {
			log.Warn().Str("raw_ts", r.FundingRateTimestamp).Str("symbol", r.Symbol).
				Msg("funding-history: parse timestamp failed, skipping entry")
			continue
		}
		rates = append(rates, FundingRate{
			Symbol:               r.Symbol,
			FundingRate:          rate,
			FundingRateTimestamp: ts,
		})
	}
	return rates, nil
}

// ComputeFundingStats computes funding rate statistics from historical data.
// It calculates current rate, 7d/30d averages, min/max, regime, and percentile.
func ComputeFundingStats(symbol string, rates []FundingRate) *FundingRateStats {
	if len(rates) == 0 {
		return &FundingRateStats{Symbol: symbol, Regime: "NEUTRAL"}
	}

	stats := &FundingRateStats{
		Symbol:  symbol,
		Current: rates[0].FundingRate, // newest first from Bybit
		History: rates,
	}

	// Bybit settles funding 3x/day (every 8h)
	// 7d = 21 settlements, 30d = 90 settlements
	n7d := 21
	n30d := 90
	if n7d > len(rates) {
		n7d = len(rates)
	}
	if n30d > len(rates) {
		n30d = len(rates)
	}

	// 7-day average
	sum7 := 0.0
	for _, r := range rates[:n7d] {
		sum7 += r.FundingRate
	}
	stats.Avg7D = sum7 / float64(n7d)

	// 30-day average, min, max
	sum30 := 0.0
	stats.Max30D = rates[0].FundingRate
	stats.Min30D = rates[0].FundingRate
	for _, r := range rates[:n30d] {
		sum30 += r.FundingRate
		if r.FundingRate > stats.Max30D {
			stats.Max30D = r.FundingRate
		}
		if r.FundingRate < stats.Min30D {
			stats.Min30D = r.FundingRate
		}
	}
	stats.Avg30D = sum30 / float64(n30d)

	// Percentile: what % of 30d rates are below current
	below := 0
	for _, r := range rates[:n30d] {
		if r.FundingRate < stats.Current {
			below++
		}
	}
	stats.Percentile = float64(below) / float64(n30d) * 100

	// Regime classification
	switch {
	case stats.Avg7D > 0.0003: // > 3bps avg = strong positive bias
		stats.Regime = "POSITIVE_BIAS"
	case stats.Avg7D < -0.0001: // < -1bps avg = negative bias
		stats.Regime = "NEGATIVE_BIAS"
	default:
		stats.Regime = "NEUTRAL"
	}

	return stats
}

// ---------------------------------------------------------------------------
// HMAC signing (for private endpoints — future use)
// ---------------------------------------------------------------------------

func (c *Client) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func nowMs() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Ensure unexported helpers are referenced to avoid "declared and not used" errors.
// sign and nowMs are reserved for future private endpoint use.
var (
	_ = defaultRecvWindow
)

func init() {
	// Reference sign/nowMs indirectly so the compiler doesn't complain
	// when private endpoints are not yet wired up.
	_ = func() {
		c := &Client{}
		_ = c.sign("")
		_ = nowMs()
	}
}
