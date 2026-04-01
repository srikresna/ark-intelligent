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
func (c *Client) GetBookSummary(ctx context.Context, currency string) ([]BookSummary, error) {
	params := url.Values{
		"currency": {currency},
		"kind":     {"option"},
	}
	var result bookSummaryResult
	if err := c.get(ctx, "get_book_summary_by_currency", params, &result); err != nil {
		return nil, err
	}
	log.Debug().Str("currency", currency).Int("count", len(result.Result)).Msg("book summary fetched")
	return result.Result, nil
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
