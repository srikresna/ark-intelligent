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
	if len(f.twelveDataKeys) > 0 && mapping.TwelveData != "" {
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

	// Convert internal interval names to TwelveData API format.
	// TwelveData uses "15min", "30min", "45min" (not "15m", "30m").
	// Hourly+ intervals like "1h", "4h" are the same in both.
	tdInterval := interval
	switch interval {
	case "15m":
		tdInterval = "15min"
	case "30m":
		tdInterval = "30min"
	case "45m":
		tdInterval = "45min"
	}

	err := f.cbTwelveData.Execute(func() error {
		url := fmt.Sprintf(
			"https://api.twelvedata.com/time_series?symbol=%s&interval=%s&outputsize=%d&apikey=%s",
			mapping.TwelveData, tdInterval, bars, f.nextTDKey(),
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
				Volume:       parseFloat(v.Volume),
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

	// Yahoo supports: 15m, 30m, 1h. For larger intervals, fetch 1h and aggregate.
	yahooInterval := interval
	fetchBars := bars
	aggregate := false

	switch interval {
	case "15m":
		yahooInterval = "15m"
	case "30m":
		yahooInterval = "30m"
	case "1h":
		yahooInterval = "1h"
	default:
		// 4h, 6h, 12h — fetch 1h and aggregate
		yahooInterval = "1h"
		fetchBars = bars * 4 // fetch enough 1h bars
		aggregate = true
	}

	// Calculate days needed based on interval granularity.
	// Yahoo limits: 15m/30m max range=1mo (30 days works reliably; 3mo gets rejected).
	var daysNeeded int
	switch yahooInterval {
	case "15m":
		daysNeeded = (fetchBars * 15 / 1440) + 2 // 1440 min/day, with margin
		if daysNeeded > 30 {
			daysNeeded = 30 // Yahoo 15m: range=1mo is the safe max
		}
	case "30m":
		daysNeeded = (fetchBars * 30 / 1440) + 2
		if daysNeeded > 30 {
			daysNeeded = 30
		}
	default: // 1h
		daysNeeded = (fetchBars / 6) + 2 // ~6 bars per day for 1h
	}
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
			records = aggregateToInterval(hourBars, mapping.ContractCode, interval)
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
// aggregateToInterval aggregates hourly bars into a target interval (e.g. 4h, 6h, 12h).
func aggregateToInterval(hourBars []domain.IntradayBar, contractCode string, targetInterval string) []domain.IntradayBar {
	if len(hourBars) == 0 {
		return nil
	}

	// Determine bucket size in hours
	bucketHours := 4 // default
	switch targetInterval {
	case "4h":
		bucketHours = 4
	case "6h":
		bucketHours = 6
	case "12h":
		bucketHours = 12
	}

	// Sort chronologically (oldest first) to ensure correct Open/Close assignment.
	sorted := make([]domain.IntradayBar, len(hourBars))
	copy(sorted, hourBars)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

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

	for _, bar := range sorted {
		h := bar.Timestamp.Hour()
		bucketHour := (h / bucketHours) * bucketHours
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
		if bar.Low < b.low {
			b.low = bar.Low
		}
		b.close = bar.Close
		b.volume += bar.Volume
		b.count++
	}

	result := make([]domain.IntradayBar, 0, len(buckets))
	for _, b := range buckets {
		if b.count < 2 { // Skip incomplete buckets
			continue
		}
		result = append(result, domain.IntradayBar{
			ContractCode: contractCode,
			Symbol:       hourBars[0].Symbol,
			Interval:     targetInterval,
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
