// Package coingecko provides a REST client for the CoinGecko API (Demo plan).
// Demo plan: rate limited to ~30 req/min.
// Auth: x-cg-demo-api-key header (or ?x_cg_demo_api_key= query param).
package coingecko

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("coingecko")

const (
	defaultBase    = "https://api.coingecko.com/api/v3"
	demoBase       = "https://pro-api.coingecko.com/api/v3"
	defaultTimeout = 20 * time.Second
)

// Client wraps CoinGecko API calls.
type Client struct {
	httpClient *http.Client
	apiKey     string
	base       string
}

// NewClient creates a CoinGecko client.
// apiKey: your demo or pro API key (empty = public tier with stricter limits).
func NewClient(apiKey string) *Client {
	base := defaultBase
	if apiKey != "" {
		base = demoBase // demo + pro keys use pro subdomain
	}
	return &Client{
		httpClient: httpclient.NewClient(defaultTimeout),
		apiKey:     apiKey,
		base:       base,
	}
}

// IsConfigured returns true if an API key is set.
func (c *Client) IsConfigured() bool { return c.apiKey != "" }

func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	fullURL := c.base + path
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("coingecko: build request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("x-cg-demo-api-key", c.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("coingecko: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("coingecko: read body: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("coingecko: rate limited (429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

// ---------------------------------------------------------------------------
// Global Market Stats — for TOTAL3-style crypto market cap
// ---------------------------------------------------------------------------

// GlobalData holds aggregate crypto market data.
type GlobalData struct {
	TotalMarketCap         map[string]float64 `json:"total_market_cap"`
	TotalVolume            map[string]float64 `json:"total_volume"`
	MarketCapPercentage    map[string]float64 `json:"market_cap_percentage"` // BTC dominance etc.
	MarketCapChangePercent float64            `json:"market_cap_change_percentage_24h_usd"`
	ActiveCryptocurrencies int                `json:"active_cryptocurrencies"`
	UpdatedAt              int64              `json:"updated_at"`
}

// GetGlobalData retrieves aggregate crypto market stats.
func (c *Client) GetGlobalData(ctx context.Context) (*GlobalData, error) {
	body, err := c.get(ctx, "/global", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data GlobalData `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("coingecko: parse global: %w", err)
	}
	return &resp.Data, nil
}

// ---------------------------------------------------------------------------
// Coin Market Data — current price, volume, market cap, 24h change
// ---------------------------------------------------------------------------

// CoinMarket holds market data for a single coin.
type CoinMarket struct {
	ID                string  `json:"id"`
	Symbol            string  `json:"symbol"`
	Name              string  `json:"name"`
	CurrentPrice      float64 `json:"current_price"`
	MarketCap         float64 `json:"market_cap"`
	MarketCapRank     int     `json:"market_cap_rank"`
	TotalVolume       float64 `json:"total_volume"`
	High24h           float64 `json:"high_24h"`
	Low24h            float64 `json:"low_24h"`
	PriceChange24h    float64 `json:"price_change_24h"`
	PriceChangePct24h float64 `json:"price_change_percentage_24h"`
	CirculatingSupply float64 `json:"circulating_supply"`
	TotalSupply       float64 `json:"total_supply"`
	ATH               float64 `json:"ath"`
	ATHChangePct      float64 `json:"ath_change_percentage"`
	LastUpdated       string  `json:"last_updated"`
}

// GetCoinMarkets retrieves market data for given coin IDs.
// ids: comma-separated, e.g. "bitcoin,ethereum"
// vsCurrency: e.g. "usd"
func (c *Client) GetCoinMarkets(ctx context.Context, ids, vsCurrency string) ([]CoinMarket, error) {
	params := url.Values{
		"vs_currency": {vsCurrency},
		"ids":         {ids},
		"order":       {"market_cap_desc"},
	}
	body, err := c.get(ctx, "/coins/markets", params)
	if err != nil {
		return nil, err
	}

	var coins []CoinMarket
	if err := json.Unmarshal(body, &coins); err != nil {
		return nil, fmt.Errorf("coingecko: parse markets: %w", err)
	}
	return coins, nil
}

// ---------------------------------------------------------------------------
// OHLC Historical
// ---------------------------------------------------------------------------

// OHLCBar holds a single OHLC candlestick.
type OHLCBar struct {
	Timestamp int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
}

// GetOHLC retrieves OHLC historical data for a coin.
// coinID: e.g. "bitcoin", "ethereum"
// days: number of days of history ("1", "7", "14", "30", "90", "180", "365", "max")
func (c *Client) GetOHLC(ctx context.Context, coinID, vsCurrency, days string) ([]OHLCBar, error) {
	params := url.Values{
		"vs_currency": {vsCurrency},
		"days":        {days},
	}
	body, err := c.get(ctx, "/coins/"+coinID+"/ohlc", params)
	if err != nil {
		return nil, err
	}

	var raw [][]float64
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("coingecko: parse ohlc: %w", err)
	}

	bars := make([]OHLCBar, 0, len(raw))
	for _, row := range raw {
		if len(row) < 5 {
			continue
		}
		bars = append(bars, OHLCBar{
			Timestamp: int64(row[0]),
			Open:      row[1],
			High:      row[2],
			Low:       row[3],
			Close:     row[4],
		})
	}
	return bars, nil
}

// ---------------------------------------------------------------------------
// Fear & Greed proxy via global market cap change
// ---------------------------------------------------------------------------

// MarketSentiment is a simple derived sentiment from market cap change.
type MarketSentiment struct {
	Label     string  // "EXTREME_FEAR", "FEAR", "NEUTRAL", "GREED", "EXTREME_GREED"
	Score     float64 // 0-100
	Change24h float64 // market cap change % 24h
}

// GetMarketSentiment derives a rough sentiment reading from global market data.
func (c *Client) GetMarketSentiment(ctx context.Context) (*MarketSentiment, error) {
	global, err := c.GetGlobalData(ctx)
	if err != nil {
		return nil, err
	}

	chg := global.MarketCapChangePercent
	score := 50.0 + chg*5
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	label := "NEUTRAL"
	switch {
	case score < 20:
		label = "EXTREME_FEAR"
	case score < 40:
		label = "FEAR"
	case score > 80:
		label = "EXTREME_GREED"
	case score > 60:
		label = "GREED"
	}

	return &MarketSentiment{
		Label:     label,
		Score:     score,
		Change24h: chg,
	}, nil
}

// ---------------------------------------------------------------------------
// BTC Dominance
// ---------------------------------------------------------------------------

// GetBTCDominance returns the BTC market cap dominance percentage.
func (c *Client) GetBTCDominance(ctx context.Context) (float64, error) {
	global, err := c.GetGlobalData(ctx)
	if err != nil {
		return 0, err
	}
	return global.MarketCapPercentage["btc"], nil
}

// ---------------------------------------------------------------------------
// TOTAL3 equivalent — altcoin market cap (total minus BTC and ETH)
// ---------------------------------------------------------------------------

// GetAltcoinMarketCap returns the approximate altcoin market cap in USD
// (total market cap minus BTC and ETH market caps).
func (c *Client) GetAltcoinMarketCap(ctx context.Context) (float64, error) {
	global, err := c.GetGlobalData(ctx)
	if err != nil {
		return 0, err
	}
	total := global.TotalMarketCap["usd"]

	// Fetch BTC + ETH market cap directly
	coins, err := c.GetCoinMarkets(ctx, "bitcoin,ethereum", "usd")
	if err != nil {
		// Fallback: use dominance percentages
		btcDom := global.MarketCapPercentage["btc"] / 100
		ethDom := global.MarketCapPercentage["eth"] / 100
		return total * (1 - btcDom - ethDom), nil
	}

	var btcMcap, ethMcap float64
	for _, coin := range coins {
		switch strings.ToLower(coin.ID) {
		case "bitcoin":
			btcMcap = coin.MarketCap
		case "ethereum":
			ethMcap = coin.MarketCap
		}
	}
	return total - btcMcap - ethMcap, nil
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

// ensure log is used
var _ = log
