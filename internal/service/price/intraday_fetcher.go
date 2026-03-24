package price

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// FetchIntraday fetches 4H OHLCV data for a single contract.
// Uses TwelveData as primary (supports 4h interval), Yahoo as fallback (1h only).
func (f *Fetcher) FetchIntraday(ctx context.Context, mapping domain.PriceSymbolMapping, interval string, bars int) ([]domain.IntradayBar, error) {
	// TwelveData is the primary source for 4H data
	if f.twelveDataKey != "" && mapping.TwelveData != "" {
		records, err := f.fetchTwelveDataIntraday(ctx, mapping, interval, bars)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("symbol", mapping.TwelveData).Msg("TwelveData intraday failed, trying Yahoo")
		}
	}

	// Yahoo Finance fallback — supports 1h and approximate 4h via 1h aggregation
	if mapping.Yahoo != "" {
		records, err := f.fetchYahooIntraday(ctx, mapping, interval, bars)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("symbol", mapping.Yahoo).Msg("Yahoo intraday failed")
		}
	}

	return nil, fmt.Errorf("no intraday source available for %s (%s)", mapping.Currency, interval)
}

// FetchAllIntraday fetches intraday bars for all COT contracts.
func (f *Fetcher) FetchAllIntraday(ctx context.Context, interval string, bars int) ([]domain.IntradayBar, *FetchReport, error) {
	start := time.Now()
	var allBars []domain.IntradayBar
	var lastErr error
	report := &FetchReport{}

	for _, mapping := range domain.COTPriceSymbolMappings() {
		records, err := f.FetchIntraday(ctx, mapping, interval, bars)
		if err != nil {
			log.Warn().Err(err).Str("contract", mapping.Currency).Msg("Failed to fetch intraday")
			lastErr = err
			report.Results = append(report.Results, ContractFetchResult{
				Currency: mapping.Currency,
				Error:    err.Error(),
			})
			report.Failed++
			continue
		}
		allBars = append(allBars, records...)
		src := ""
		if len(records) > 0 {
			src = records[0].Source
		}
		report.Results = append(report.Results, ContractFetchResult{
			Currency: mapping.Currency,
			Source:   src,
			Records:  len(records),
		})
		report.Success++

		select {
		case <-ctx.Done():
			report.Duration = time.Since(start)
			return allBars, report, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	report.Duration = time.Since(start)

	if len(allBars) == 0 && lastErr != nil {
		return nil, report, fmt.Errorf("all intraday fetches failed, last error: %w", lastErr)
	}

	log.Info().Int("bars", len(allBars)).Str("interval", interval).Msg("Intraday fetch complete")
	return allBars, report, nil
}

// --- TwelveData Intraday ---

func (f *Fetcher) fetchTwelveDataIntraday(ctx context.Context, mapping domain.PriceSymbolMapping, interval string, bars int) ([]domain.IntradayBar, error) {
	var records []domain.IntradayBar

	err := f.cbTwelveData.Execute(func() error {
		url := fmt.Sprintf(
			"https://api.twelvedata.com/time_series?symbol=%s&interval=%s&outputsize=%d&apikey=%s",
			mapping.TwelveData, interval, bars, f.twelveDataKey,
		)

		body, err := f.doGet(ctx, url, nil)
		if err != nil {
			return fmt.Errorf("twelvedata intraday request: %w", err)
		}

		var resp twelveDataResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("twelvedata intraday parse: %w", err)
		}

		if resp.Status != "ok" {
			return fmt.Errorf("twelvedata intraday error: code=%d msg=%s", resp.Code, resp.Msg)
		}

		records = make([]domain.IntradayBar, 0, len(resp.Values))
		for _, v := range resp.Values {
			t, err := time.Parse("2006-01-02 15:04:05", v.Datetime)
			if err != nil {
				// Try date-only format
				t, err = time.Parse("2006-01-02", v.Datetime)
				if err != nil {
					continue
				}
			}
			bar := domain.IntradayBar{
				ContractCode: mapping.ContractCode,
				Symbol:       mapping.TwelveData,
				Interval:     interval,
				Timestamp:    t.UTC(),
				Open:         parseFloat(v.Open),
				High:         parseFloat(v.High),
				Low:          parseFloat(v.Low),
				Close:        parseFloat(v.Close),
				Source:       "twelvedata",
			}
			if bar.Close > 0 {
				records = append(records, bar)
			}
		}

		sortIntradayByTime(records)
		if len(records) > bars {
			records = records[:bars]
		}
		return nil
	})

	return records, err
}

// --- Yahoo Finance Intraday ---

func (f *Fetcher) fetchYahooIntraday(ctx context.Context, mapping domain.PriceSymbolMapping, interval string, bars int) ([]domain.IntradayBar, error) {
	var records []domain.IntradayBar

	// Yahoo supports: 1h, but not 4h directly.
	// For 4h, we fetch 1h bars and aggregate.
	yahooInterval := "1h"
	fetchBars := bars
	aggregate := false
	if interval == "4h" {
		yahooInterval = "1h"
		fetchBars = bars * 4
		aggregate = true
	}

	// Yahoo 1h data available for ~730 days (use 60d for our needs)
	daysNeeded := (fetchBars / 6) + 2 // ~6 bars per day for 1h
	rangeStr := yahooRangeForDays(daysNeeded)

	err := f.cbYahoo.Execute(func() error {
		url := fmt.Sprintf(
			"https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=%s&range=%s",
			mapping.Yahoo, yahooInterval, rangeStr,
		)

		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		}

		body, err := f.doGet(ctx, url, headers)
		if err != nil {
			return fmt.Errorf("yahoo intraday request: %w", err)
		}

		var resp yahooChartResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("yahoo intraday parse: %w", err)
		}

		if resp.Chart.Error != nil {
			return fmt.Errorf("yahoo intraday error: %s: %s", resp.Chart.Error.Code, resp.Chart.Error.Description)
		}

		if len(resp.Chart.Result) == 0 {
			return fmt.Errorf("yahoo intraday empty response for %s", mapping.Yahoo)
		}

		result := resp.Chart.Result[0]
		if len(result.Indicators.Quote) == 0 {
			return fmt.Errorf("yahoo intraday no quote data for %s", mapping.Yahoo)
		}

		quote := result.Indicators.Quote[0]
		hourBars := make([]domain.IntradayBar, 0, len(result.Timestamp))

		for i, ts := range result.Timestamp {
			if i >= len(quote.Close) {
				break
			}
			if quote.Close[i] == nil || quote.Open[i] == nil {
				continue
			}

			t := time.Unix(ts, 0).UTC()
			bar := domain.IntradayBar{
				ContractCode: mapping.ContractCode,
				Symbol:       mapping.Yahoo,
				Interval:     yahooInterval,
				Timestamp:    t,
				Open:         derefFloat(quote.Open[i]),
				High:         derefFloat(safeIndex(quote.High, i)),
				Low:          derefFloat(safeIndex(quote.Low, i)),
				Close:        derefFloat(quote.Close[i]),
				Source:       "yahoo",
			}
			if i < len(quote.Volume) && quote.Volume[i] != nil {
				bar.Volume = *quote.Volume[i]
			}
			if bar.Close > 0 {
				hourBars = append(hourBars, bar)
			}
		}

		if aggregate && len(hourBars) > 0 {
			records = aggregateTo4H(hourBars, mapping.ContractCode)
		} else {
			records = hourBars
		}

		sortIntradayByTime(records)
		if len(records) > bars {
			records = records[:bars]
		}
		return nil
	})

	return records, err
}

// aggregateTo4H converts 1H bars into 4H bars.
// Groups by 4-hour buckets: 00-03, 04-07, 08-11, 12-15, 16-19, 20-23.
func aggregateTo4H(hourBars []domain.IntradayBar, contractCode string) []domain.IntradayBar {
	type bucket struct {
		open    float64
		high    float64
		low     float64
		close   float64
		volume  float64
		ts      time.Time
		count   int
	}

	buckets := make(map[string]*bucket)

	for _, bar := range hourBars {
		h := bar.Timestamp.Hour()
		bucketHour := (h / 4) * 4
		bucketTime := time.Date(
			bar.Timestamp.Year(), bar.Timestamp.Month(), bar.Timestamp.Day(),
			bucketHour, 0, 0, 0, time.UTC,
		)
		key := bucketTime.Format("200601021504")

		b, ok := buckets[key]
		if !ok {
			b = &bucket{
				open: bar.Open,
				high: bar.High,
				low:  bar.Low,
				ts:   bucketTime,
			}
			buckets[key] = b
		}
		if bar.High > b.high {
			b.high = bar.High
		}
		if bar.Low < b.low || b.low == 0 {
			b.low = bar.Low
		}
		b.close = bar.Close
		b.volume += bar.Volume
		b.count++
	}

	result := make([]domain.IntradayBar, 0, len(buckets))
	for _, b := range buckets {
		if b.count < 2 { // Skip incomplete buckets (need at least 2 of 4 hours)
			continue
		}
		result = append(result, domain.IntradayBar{
			ContractCode: contractCode,
			Symbol:       hourBars[0].Symbol,
			Interval:     "4h",
			Timestamp:    b.ts,
			Open:         b.open,
			High:         b.high,
			Low:          b.low,
			Close:        b.close,
			Volume:       b.volume,
			Source:       hourBars[0].Source,
		})
	}

	return result
}

// --- Helpers ---

func sortIntradayByTime(bars []domain.IntradayBar) {
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].Timestamp.After(bars[j].Timestamp)
	})
}
