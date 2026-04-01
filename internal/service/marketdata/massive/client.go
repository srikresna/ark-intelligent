// Package massive provides a REST client for the Massive (formerly Polygon.io) API.
// Supports multiple API keys with round-robin rotation (good for free tier).
//
// Auth: Bearer token in Authorization header OR ?apiKey= query param.
// Base URL: https://api.massive.com
// Rate limit: depends on plan — free tier is limited, rotating keys helps.
package massive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/service/marketdata/keyring"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
	"github.com/arkcode369/ark-intelligent/pkg/retry"
)

var log = logger.Component("massive")

const (
	defaultRestBase = "https://api.massive.com"
	defaultTimeout  = 30 * time.Second
)

// Client is a rate-limit-aware HTTP client for the Massive REST API.
type Client struct {
	httpClient *http.Client
	keys       *keyring.Keyring
	restBase   string
}

// NewClient creates a new Massive REST client.
// keys: slice of API keys for round-robin rotation.
// restBase: optional override (default: https://api.massive.com).
func NewClient(keys []string, restBase string) *Client {
	base := restBase
	if base == "" {
		base = defaultRestBase
	}
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		keys:       keyring.New(keys),
		restBase:   base,
	}
}

// IsConfigured returns true if at least one API key is set.
func (c *Client) IsConfigured() bool { return !c.keys.IsEmpty() }

// get performs an authenticated GET request to the given path with optional query params.
// It injects the API key as a Bearer token in the Authorization header.
func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	apiKey, err := c.keys.Next()
	if err != nil {
		return nil, fmt.Errorf("massive: %w", err)
	}

	fullURL := c.restBase + path
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	return retry.Do(ctx, func() ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("massive: build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("massive: http: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("massive: read body: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("massive: rate limited (429) on key rotation — add more keys or slow down")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("massive: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
		}
		return body, nil
	})
}

// ---------------------------------------------------------------------------
// Ticker Snapshot — get current price snapshot for a forex pair
// ---------------------------------------------------------------------------

// ForexSnapshot holds the current market snapshot for a forex pair.
type ForexSnapshot struct {
	Ticker          string  `json:"ticker"`
	TodaysChange    float64 `json:"todaysChange"`
	TodaysChangePct float64 `json:"todaysChangePerc"`
	Day             struct {
		Open   float64 `json:"o"`
		High   float64 `json:"h"`
		Low    float64 `json:"l"`
		Close  float64 `json:"c"`
		Volume float64 `json:"v"`
		VWAP   float64 `json:"vw"`
	} `json:"day"`
	LastQuote struct {
		Ask       float64 `json:"a"`
		Bid       float64 `json:"b"`
		Timestamp int64   `json:"t"`
	} `json:"lastQuote"`
}

// GetForexSnapshot retrieves the current snapshot for a forex pair (e.g. "C:EURUSD").
func (c *Client) GetForexSnapshot(ctx context.Context, ticker string) (*ForexSnapshot, error) {
	path := fmt.Sprintf("/v2/snapshot/locale/global/markets/forex/tickers/%s", ticker)
	body, err := c.get(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status string        `json:"status"`
		Ticker ForexSnapshot `json:"ticker"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("massive: parse forex snapshot: %w", err)
	}
	return &resp.Ticker, nil
}

// ---------------------------------------------------------------------------
// Aggregate Bars — OHLCV historical
// ---------------------------------------------------------------------------

// Bar holds one OHLCV aggregate bar.
type Bar struct {
	Open         float64 `json:"o"`
	High         float64 `json:"h"`
	Low          float64 `json:"l"`
	Close        float64 `json:"c"`
	Volume       float64 `json:"v"`
	VWAP         float64 `json:"vw"`
	Timestamp    int64   `json:"t"` // Unix ms
	Transactions int     `json:"n"`
}

// GetForexBars retrieves historical aggregate bars for a forex pair.
// ticker: e.g. "C:EURUSD"
// multiplier: bar size multiplier (e.g. 1)
// timespan: "minute", "hour", "day", "week"
// from/to: date strings "YYYY-MM-DD"
func (c *Client) GetForexBars(ctx context.Context, ticker string, multiplier int, timespan, from, to string) ([]Bar, error) {
	path := fmt.Sprintf("/v2/aggs/ticker/%s/range/%d/%s/%s/%s", ticker, multiplier, timespan, from, to)
	params := url.Values{"adjusted": {"true"}, "sort": {"asc"}, "limit": {"50000"}}
	body, err := c.get(ctx, path, params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []Bar `json:"results"`
		Count   int   `json:"resultsCount"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("massive: parse bars: %w", err)
	}
	return resp.Results, nil
}

// ---------------------------------------------------------------------------
// Economy endpoints — Treasury yields, inflation, labor market
// ---------------------------------------------------------------------------

// TreasuryYield holds a single Treasury yield observation.
type TreasuryYield struct {
	Date   string  `json:"date"`
	Value  float64 `json:"value"`
	Series string  `json:"series"` // e.g. "DGS10"
}

// GetTreasuryYields retrieves Treasury yield series data.
// series: e.g. "DGS10", "DGS2", "DGS30", "T10Y2Y"
func (c *Client) GetTreasuryYields(ctx context.Context, series string, limit int) ([]TreasuryYield, error) {
	params := url.Values{"series": {series}, "limit": {fmt.Sprintf("%d", limit)}}
	body, err := c.get(ctx, "/v1/economy/treasury-yields", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []TreasuryYield `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("massive: parse treasury yields: %w", err)
	}
	return resp.Results, nil
}

// Inflation holds a single inflation data point.
type Inflation struct {
	Date   string  `json:"date"`
	Value  float64 `json:"value"`
	Series string  `json:"series"`
}

// GetInflation retrieves inflation indicators (CPI, PCE, etc.).
func (c *Client) GetInflation(ctx context.Context, limit int) ([]Inflation, error) {
	params := url.Values{"limit": {fmt.Sprintf("%d", limit)}}
	body, err := c.get(ctx, "/v1/economy/inflation", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []Inflation `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("massive: parse inflation: %w", err)
	}
	return resp.Results, nil
}

// ---------------------------------------------------------------------------
// Indices snapshot — for SPX500, NDX, etc. as regime proxies
// ---------------------------------------------------------------------------

// IndexSnapshot holds current snapshot for an index.
type IndexSnapshot struct {
	Ticker string  `json:"ticker"`
	Value  float64 `json:"value"`
	Day    struct {
		Open  float64 `json:"o"`
		High  float64 `json:"h"`
		Low   float64 `json:"l"`
		Close float64 `json:"c"`
	} `json:"session"`
}

// GetIndexSnapshot retrieves a snapshot for an index ticker.
// ticker: e.g. "I:SPX", "I:NDX", "I:VIX"
func (c *Client) GetIndexSnapshot(ctx context.Context, ticker string) (*IndexSnapshot, error) {
	body, err := c.get(ctx, "/v3/snapshot", url.Values{"ticker": {ticker}})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []IndexSnapshot `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("massive: parse index snapshot: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("massive: no snapshot for %s", ticker)
	}
	return &resp.Results[0], nil
}

// ---------------------------------------------------------------------------
// Options snapshot — for vol proxy (e.g. SPY options for vol regime)
// ---------------------------------------------------------------------------

// OptionsChainItem holds a single option contract from the chain.
type OptionsChainItem struct {
	Ticker            string  `json:"ticker"`
	StrikePrice       float64 `json:"strike_price"`
	ContractType      string  `json:"contract_type"` // "call" or "put"
	ExpirationDate    string  `json:"expiration_date"`
	ImpliedVolatility float64 `json:"implied_volatility"`
	Delta             float64 `json:"delta"`
	Gamma             float64 `json:"gamma"`
	Vega              float64 `json:"vega"`
	OpenInterest      float64 `json:"open_interest"`
	Volume            float64 `json:"volume"`
	UnderlyingTicker  string  `json:"underlying_ticker"`
}

// GetOptionsChain retrieves option chain snapshot for an underlying.
// underlying: e.g. "SPY", "QQQ"
// expDate: optional filter e.g. "2025-04-18"
func (c *Client) GetOptionsChain(ctx context.Context, underlying, expDate string, limit int) ([]OptionsChainItem, error) {
	params := url.Values{
		"underlying_asset": {underlying},
		"limit":            {fmt.Sprintf("%d", limit)},
	}
	if expDate != "" {
		params.Set("expiration_date", expDate)
	}
	body, err := c.get(ctx, "/v3/snapshot/options/"+underlying, params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []struct {
			Details OptionsChainItem `json:"details"`
			Greeks  struct {
				Delta float64 `json:"delta"`
				Gamma float64 `json:"gamma"`
				Vega  float64 `json:"vega"`
			} `json:"greeks"`
			IV  float64 `json:"implied_volatility"`
			OI  float64 `json:"open_interest"`
			Vol float64 `json:"day"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("massive: parse options chain: %w", err)
	}

	items := make([]OptionsChainItem, 0, len(resp.Results))
	for _, r := range resp.Results {
		item := r.Details
		item.ImpliedVolatility = r.IV
		item.Delta = r.Greeks.Delta
		item.Gamma = r.Greeks.Gamma
		item.Vega = r.Greeks.Vega
		item.OpenInterest = r.OI
		items = append(items, item)
	}
	return items, nil
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

// ensure log variable is used (suppress unused import warning)
var _ = log
