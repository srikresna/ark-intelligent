package price

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// FetchDaily fetches daily OHLCV data for a single contract.
// Uses Yahoo Finance as the primary source (no API key, reliable daily data).
// Falls back to TwelveData if available.
func (f *Fetcher) FetchDaily(ctx context.Context, mapping domain.PriceSymbolMapping, days int) ([]domain.DailyPrice, error) {
	// Try Yahoo Finance first (best daily data, no key needed)
	if mapping.Yahoo != "" {
		records, err := f.fetchYahooDaily(ctx, mapping, days)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("symbol", mapping.Yahoo).Msg("Yahoo daily failed, trying TwelveData")
		}
	}

	// Fallback to TwelveData
	if f.twelveDataKey != "" && mapping.TwelveData != "" {
		records, err := f.fetchTwelveDataDaily(ctx, mapping, days)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("symbol", mapping.TwelveData).Msg("TwelveData daily failed")
		}
	}

	return nil, fmt.Errorf("no daily price source available for %s", mapping.Currency)
}

// FetchAllDaily fetches daily prices for all COT contracts + price-only instruments.
// Returns a detailed report of which sources were used.
func (f *Fetcher) FetchAllDaily(ctx context.Context, days int) ([]domain.DailyPrice, *FetchReport, error) {
	start := time.Now()
	var allRecords []domain.DailyPrice
	var lastErr error
	report := &FetchReport{}

	// Fetch COT-contract mappings
	for _, mapping := range domain.COTPriceSymbolMappings() {
		records, err := f.FetchDaily(ctx, mapping, days)
		if err != nil {
			log.Warn().Err(err).Str("contract", mapping.Currency).Msg("Failed to fetch daily price")
			lastErr = err
			report.Results = append(report.Results, ContractFetchResult{
				Currency: mapping.Currency,
				Error:    err.Error(),
			})
			report.Failed++
			continue
		}
		allRecords = append(allRecords, records...)
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
			return allRecords, report, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	// Fetch risk instruments daily (VIX, SPX)
	for _, mapping := range domain.RiskPriceSymbolMappings() {
		records, err := f.FetchDaily(ctx, mapping, days)
		if err != nil {
			log.Warn().Err(err).Str("instrument", mapping.Currency).Msg("Risk daily fetch failed — skipping")
			continue
		}
		allRecords = append(allRecords, records...)
	}

	report.Duration = time.Since(start)

	if len(allRecords) == 0 && lastErr != nil {
		return nil, report, fmt.Errorf("all daily price fetches failed, last error: %w", lastErr)
	}

	log.Info().Int("records", len(allRecords)).Msg("Daily price fetch complete")
	return allRecords, report, nil
}

// --- Yahoo Finance Daily ---

func (f *Fetcher) fetchYahooDaily(ctx context.Context, mapping domain.PriceSymbolMapping, days int) ([]domain.DailyPrice, error) {
	var records []domain.DailyPrice

	err := f.cbYahoo.Execute(func() error {
		rangeStr := yahooRangeForDays(days)

		url := fmt.Sprintf(
			"https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=%s",
			mapping.Yahoo, rangeStr,
		)

		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		}

		body, err := f.doGet(ctx, url, headers)
		if err != nil {
			return fmt.Errorf("yahoo daily request: %w", err)
		}

		var resp yahooChartResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("yahoo daily parse: %w", err)
		}

		if resp.Chart.Error != nil {
			return fmt.Errorf("yahoo daily error: %s: %s", resp.Chart.Error.Code, resp.Chart.Error.Description)
		}

		if len(resp.Chart.Result) == 0 {
			return fmt.Errorf("yahoo daily empty response for %s", mapping.Yahoo)
		}

		result := resp.Chart.Result[0]
		if len(result.Indicators.Quote) == 0 {
			return fmt.Errorf("yahoo daily no quote data for %s", mapping.Yahoo)
		}

		quote := result.Indicators.Quote[0]
		records = make([]domain.DailyPrice, 0, len(result.Timestamp))

		for i, ts := range result.Timestamp {
			if i >= len(quote.Close) {
				break
			}
			if quote.Close[i] == nil || quote.Open[i] == nil {
				continue
			}

			t := time.Unix(ts, 0).UTC()
			rec := domain.DailyPrice{
				ContractCode: mapping.ContractCode,
				Symbol:       mapping.Yahoo,
				Date:         t,
				Open:         derefFloat(quote.Open[i]),
				High:         derefFloat(safeIndex(quote.High, i)),
				Low:          derefFloat(safeIndex(quote.Low, i)),
				Close:        derefFloat(quote.Close[i]),
				Source:       "yahoo",
			}
			// Parse volume if available
			if i < len(quote.Volume) && quote.Volume[i] != nil {
				rec.Volume = *quote.Volume[i]
			}
			if rec.Close > 0 {
				records = append(records, rec)
			}
		}

		sortDailyByDate(records)
		if len(records) > days {
			records = records[:days]
		}
		return nil
	})

	return records, err
}

// --- TwelveData Daily ---

func (f *Fetcher) fetchTwelveDataDaily(ctx context.Context, mapping domain.PriceSymbolMapping, days int) ([]domain.DailyPrice, error) {
	var records []domain.DailyPrice

	err := f.cbTwelveData.Execute(func() error {
		url := fmt.Sprintf(
			"https://api.twelvedata.com/time_series?symbol=%s&interval=1day&outputsize=%d&apikey=%s",
			mapping.TwelveData, days, f.twelveDataKey,
		)

		body, err := f.doGet(ctx, url, nil)
		if err != nil {
			return fmt.Errorf("twelvedata daily request: %w", err)
		}

		var resp twelveDataResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("twelvedata daily parse: %w", err)
		}

		if resp.Status != "ok" {
			return fmt.Errorf("twelvedata daily error: code=%d msg=%s", resp.Code, resp.Msg)
		}

		records = make([]domain.DailyPrice, 0, len(resp.Values))
		for _, v := range resp.Values {
			t, err := time.Parse("2006-01-02", v.Datetime)
			if err != nil {
				continue
			}
			rec := domain.DailyPrice{
				ContractCode: mapping.ContractCode,
				Symbol:       mapping.TwelveData,
				Date:         t,
				Open:         parseFloat(v.Open),
				High:         parseFloat(v.High),
				Low:          parseFloat(v.Low),
				Close:        parseFloat(v.Close),
				Source:       "twelvedata",
			}
			if rec.Close > 0 {
				records = append(records, rec)
			}
		}

		sortDailyByDate(records)
		if len(records) > days {
			records = records[:days]
		}
		return nil
	})

	return records, err
}

// --- Helpers ---

// yahooRangeForDays converts a day count to Yahoo Finance range parameter.
func yahooRangeForDays(days int) string {
	switch {
	case days <= 5:
		return "5d"
	case days <= 30:
		return "1mo"
	case days <= 90:
		return "3mo"
	case days <= 180:
		return "6mo"
	case days <= 365:
		return "1y"
	default:
		return "2y"
	}
}

// sortDailyByDate sorts daily price records newest-first.
func sortDailyByDate(records []domain.DailyPrice) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date.After(records[j].Date)
	})
}
