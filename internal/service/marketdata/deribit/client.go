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

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
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
		baseURL:    defaultBaseURL,
		httpClient: httpclient.NewClient(defaultTimeout),
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
// Expired options are filtered out client-side, consistent with GetInstruments
// which uses expired=false. This prevents stale data on expiry days.
func (c *Client) GetBookSummary(ctx context.Context, currency string) ([]BookSummary, error) {
	params := url.Values{
		"currency": {currency},
		"kind":     {"option"},
	}
	var result bookSummaryResult
	if err := c.get(ctx, "get_book_summary_by_currency", params, &result); err != nil {
		return nil, err
	}

	// Filter out expired options.
	now := time.Now()
	nowMs := now.UnixMilli()
	active := make([]BookSummary, 0, len(result.Result))
	filtered := 0
	for _, bs := range result.Result {
		expMs := bs.ExpirationTS
		// Fallback: parse expiry from instrument name if timestamp is missing.
		if expMs == 0 {
			if t, err := parseExpiryFromInstrument(bs.InstrumentName); err == nil {
				expMs = t.UnixMilli()
			}
		}
		if expMs > 0 && expMs <= nowMs {
			filtered++
			continue
		}
		active = append(active, bs)
	}

	log.Debug().
		Str("currency", currency).
		Int("total", len(result.Result)).
		Int("filtered", filtered).
		Int("active", len(active)).
		Msg("book summary fetched (expired filtered)")
	return active, nil
}

// parseExpiryFromInstrument extracts the expiration date from a Deribit
// instrument name like "BTC-28MAR25-80000-C". The expiry is at 08:00 UTC
// on the given date per Deribit convention.
func parseExpiryFromInstrument(name string) (time.Time, error) {
	return ParseExpiryFromInstrument(name)
}

// ParseExpiryFromInstrument is the exported version of parseExpiryFromInstrument,
// used by other packages (e.g. gex) to parse expiry from instrument names.
func ParseExpiryFromInstrument(name string) (time.Time, error) {
	parts := strings.SplitN(name, "-", 4)
	if len(parts) < 4 {
		return time.Time{}, fmt.Errorf("invalid instrument name: %s", name)
	}
	dateStr := parts[1] // e.g. "28MAR25"
	t, err := time.Parse("2Jan06", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse expiry from %s: %w", name, err)
	}
	// Deribit options expire at 08:00 UTC.
	t = t.Add(8 * time.Hour)
	return t, nil
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

// GetIndexPriceByName returns the current Deribit index price for an explicit
// index name (e.g. "btc_usd", "sol_usdc"). This is useful for USDC-settled
// altcoin options where the index name differs from the simple currency_usd pattern.
func (c *Client) GetIndexPriceByName(ctx context.Context, indexName string) (float64, error) {
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
