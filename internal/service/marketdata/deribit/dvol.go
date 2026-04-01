// dvol.go adds Deribit Volatility Index (DVOL) and Historical Volatility endpoints.
// DVOL is the crypto-native equivalent of CBOE VIX — a 30-day implied volatility index.
// No API key required; all endpoints are public.
package deribit

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// DVOLCandle represents a single DVOL OHLC candle.
type DVOLCandle struct {
	Open      float64 `json:"-"` // parsed from Data array
	High      float64 `json:"-"`
	Low       float64 `json:"-"`
	Close     float64 `json:"-"`
	Timestamp int64   `json:"-"` // milliseconds
}

// dvolResult is the raw API envelope for get_volatility_index_data.
// Data is: [[open, high, low, close, timestamp_ms], ...]
type dvolResult struct {
	Result struct {
		Continuation int64       `json:"continuation"`
		Data         [][]float64 `json:"data"`
	} `json:"result"`
}

// HistoricalVolatility represents a single historical volatility reading.
type HistoricalVolatility struct {
	Timestamp int64   `json:"-"` // milliseconds
	Value     float64 `json:"-"` // annualized HV (e.g. 0.65 = 65%)
}

// hvResult is the raw API envelope for get_historical_volatility.
// Result is: [[timestamp_ms, value], ...]
type hvResult struct {
	Result [][]float64 `json:"result"`
}

// GetDVOL fetches DVOL (Deribit Volatility Index) candle data for a currency.
// Currency: "BTC" or "ETH". Resolution: "1" (1s), "60" (1m), "3600" (1h),
// "43200" (12h), "86400" (1D). Returns candles for approximately the last 24h
// when using resolution "3600".
func (c *Client) GetDVOL(ctx context.Context, currency string, resolution string) ([]DVOLCandle, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if resolution == "" {
		resolution = "3600" // default: 1h candles
	}

	// Fetch last 24h of data
	endTS := time.Now().UnixMilli()
	startTS := endTS - 24*60*60*1000 // 24 hours ago

	params := url.Values{
		"currency":        {cur},
		"resolution":      {resolution},
		"start_timestamp": {fmt.Sprintf("%d", startTS)},
		"end_timestamp":   {fmt.Sprintf("%d", endTS)},
	}

	var result dvolResult
	if err := c.get(ctx, "get_volatility_index_data", params, &result); err != nil {
		return nil, fmt.Errorf("deribit: get DVOL %s: %w", cur, err)
	}

	candles := make([]DVOLCandle, 0, len(result.Result.Data))
	for _, row := range result.Result.Data {
		if len(row) < 5 {
			continue
		}
		candles = append(candles, DVOLCandle{
			Open:      row[0],
			High:      row[1],
			Low:       row[2],
			Close:     row[3],
			Timestamp: int64(row[4]),
		})
	}

	log.Debug().
		Str("currency", cur).
		Str("resolution", resolution).
		Int("candles", len(candles)).
		Msg("DVOL data fetched")

	return candles, nil
}

// GetHistoricalVolatility fetches realized (historical) volatility for a currency.
// Returns pairs of [timestamp_ms, annualized_hv]. The returned values are
// typically the last N days of HV readings. Deribit computes this from recent
// underlying price changes.
func (c *Client) GetHistoricalVolatility(ctx context.Context, currency string) ([]HistoricalVolatility, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))

	params := url.Values{
		"currency": {cur},
	}

	var result hvResult
	if err := c.get(ctx, "get_historical_volatility", params, &result); err != nil {
		return nil, fmt.Errorf("deribit: get historical volatility %s: %w", cur, err)
	}

	hvs := make([]HistoricalVolatility, 0, len(result.Result))
	for _, row := range result.Result {
		if len(row) < 2 {
			continue
		}
		hvs = append(hvs, HistoricalVolatility{
			Timestamp: int64(row[0]),
			Value:     row[1],
		})
	}

	log.Debug().
		Str("currency", cur).
		Int("readings", len(hvs)).
		Msg("historical volatility fetched")

	return hvs, nil
}
