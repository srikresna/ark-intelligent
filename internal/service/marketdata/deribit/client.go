// Package deribit provides a public REST client for the Deribit options API.
// No API key is required — all endpoints used here are public.
package deribit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("deribit")

const (
	defaultBaseURL = "https://www.deribit.com/api/v2/public"
	defaultTimeout = 10 * time.Second
)

// Client is a thin HTTP client for Deribit's public API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Deribit public API client with the default base URL.
func NewClient() *Client {
	return &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// get performs a GET request to the Deribit API and unmarshals the result
// into dest. Returns an error on network failure or non-2xx status.
func (c *Client) get(ctx context.Context, endpoint string, params url.Values, dest any) error {
	rawURL := fmt.Sprintf("%s/%s", c.baseURL, endpoint)
	if len(params) > 0 {
		rawURL = rawURL + "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("deribit: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deribit: http get %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deribit: http %d from %s", resp.StatusCode, endpoint)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024)) // 4 MB limit
	if err != nil {
		return fmt.Errorf("deribit: read response: %w", err)
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("deribit: unmarshal %s: %w", endpoint, err)
	}
	return nil
}

// GetInstruments returns all active option instruments for the given currency
// (e.g. "BTC" or "ETH").
func (c *Client) GetInstruments(ctx context.Context, currency string) ([]Instrument, error) {
	params := url.Values{
		"currency": {currency},
		"kind":     {"option"},
		"expired":  {"false"},
	}
	var result instrumentsResult
	if err := c.get(ctx, "get_instruments", params, &result); err != nil {
		return nil, err
	}
	log.Debug().Str("currency", currency).Int("count", len(result.Result)).Msg("instruments fetched")
	return result.Result, nil
}

// GetBookSummary returns OI/volume data for all options of a given currency.
// Expired options are filtered out client-side by parsing the expiry date from
// the instrument name (e.g. BTC-28MAR25-80000-C). This ensures consistency
// with GetInstruments which passes expired=false server-side.
func (c *Client) GetBookSummary(ctx context.Context, currency string) ([]BookSummary, error) {
	params := url.Values{
		"currency": {currency},
		"kind":     {"option"},
	}
	var result bookSummaryResult
	if err := c.get(ctx, "get_book_summary_by_currency", params, &result); err != nil {
		return nil, err
	}

	// Filter out expired options to match GetInstruments behavior
	now := time.Now()
	active := make([]BookSummary, 0, len(result.Result))
	expired := 0
	for _, bs := range result.Result {
		if expiry, ok := parseInstrumentExpiry(bs.InstrumentName); ok {
			if expiry.Before(now) {
				expired++
				continue
			}
		}
		active = append(active, bs)
	}
	if expired > 0 {
		log.Debug().Str("currency", currency).Int("expired_filtered", expired).
			Int("active", len(active)).Msg("filtered expired options from book summary")
	}
	log.Debug().Str("currency", currency).Int("count", len(active)).Msg("book summary fetched")
	return active, nil
}

// parseInstrumentExpiry extracts the expiration date from a Deribit instrument name.
// Format: BTC-28MAR25-80000-C → expiry is 28MAR25 (parsed as 2025-03-28 08:00 UTC).
// Deribit options expire at 08:00 UTC on expiry day.
func parseInstrumentExpiry(name string) (time.Time, bool) {
	parts := strings.SplitN(name, "-", 4)
	if len(parts) < 3 {
		return time.Time{}, false
	}
	datePart := parts[1] // e.g. 28MAR25
	t, err := time.Parse("2Jan06", datePart)
	if err != nil {
		return time.Time{}, false
	}
	// Deribit options expire at 08:00 UTC
	t = t.Add(8 * time.Hour)
	return t, true
}

// GetTicker returns per-instrument Greeks for a single option contract.
func (c *Client) GetTicker(ctx context.Context, instrument string) (*Ticker, error) {
	params := url.Values{
		"instrument_name": {instrument},
	}
	var result tickerResult
	if err := c.get(ctx, "get_ticker", params, &result); err != nil {
		return nil, err
	}
	return &result.Result, nil
}

// GetIndexPrice returns the current Deribit index price for a currency (e.g. "btc_usd").
// This is the canonical spot price source when UnderlyingPrice is unavailable in book summaries.
// Deribit index name format: lowercase currency + "_usd" (e.g. "btc_usd", "eth_usd").
func (c *Client) GetIndexPrice(ctx context.Context, currency string) (float64, error) {
	// Deribit index names are lowercase: btc_usd, eth_usd, etc.
	indexName := strings.ToLower(currency) + "_usd"
	params := url.Values{
		"index_name": {indexName},
	}
	var result indexPriceResult
	if err := c.get(ctx, "get_index_price", params, &result); err != nil {
		return 0, fmt.Errorf("deribit: get_index_price %s: %w", indexName, err)
	}
	if result.Result.IndexPrice <= 0 {
		return 0, fmt.Errorf("deribit: invalid index price (%.4f) for %s", result.Result.IndexPrice, indexName)
	}
	log.Debug().Str("index", indexName).Float64("price", result.Result.IndexPrice).Msg("index price fetched")
	return result.Result.IndexPrice, nil
}
