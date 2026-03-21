package price

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/circuitbreaker"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("price")

// Fetcher implements ports.PriceFetcher with 3-layer resilience:
// Twelve Data (primary) → Alpha Vantage (secondary) → Yahoo Finance (fallback).
type Fetcher struct {
	httpClient     *http.Client
	twelveDataKey  string
	avKeys         []string
	avKeyIndex     uint64 // atomic counter for round-robin AV key selection
	cbTwelveData   *circuitbreaker.Breaker
	cbAlphaVantage *circuitbreaker.Breaker
	cbYahoo        *circuitbreaker.Breaker
}

// NewFetcher creates a new price fetcher with the given API keys.
// Both keys are optional — Yahoo Finance fallback requires no key.
func NewFetcher(twelveDataKey string, avKeys []string) *Fetcher {
	return &Fetcher{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		twelveDataKey:  twelveDataKey,
		avKeys:         avKeys,
		cbTwelveData:   circuitbreaker.New("twelve-data", 3, 5*time.Minute),
		cbAlphaVantage: circuitbreaker.New("alpha-vantage", 3, 5*time.Minute),
		cbYahoo:        circuitbreaker.New("yahoo-finance", 5, 3*time.Minute),
	}
}

// ContractFetchResult holds the result of fetching price data for a single contract.
type ContractFetchResult struct {
	Currency string // e.g. "EUR"
	Source   string // "twelvedata", "alphavantage", "yahoo", or "" if failed
	Records  int    // number of records fetched
	Error    string // non-empty if all sources failed
}

// FetchReport summarises a FetchAll run — which provider served each contract.
type FetchReport struct {
	Results  []ContractFetchResult
	Success  int
	Failed   int
	Duration time.Duration
}

// FetchAll fetches weekly prices for all 11 default contracts,
// routing each to the optimal API source.
func (f *Fetcher) FetchAll(ctx context.Context, weeks int) ([]domain.PriceRecord, error) {
	records, _, err := f.FetchAllDetailed(ctx, weeks)
	return records, err
}

// FetchAllDetailed is like FetchAll but also returns a FetchReport
// with per-contract source and error information.
func (f *Fetcher) FetchAllDetailed(ctx context.Context, weeks int) ([]domain.PriceRecord, *FetchReport, error) {
	start := time.Now()
	var allRecords []domain.PriceRecord
	var lastErr error
	report := &FetchReport{}

	for _, mapping := range domain.DefaultPriceSymbolMappings {
		records, err := f.FetchWeekly(ctx, mapping, weeks)
		if err != nil {
			log.Warn().Err(err).Str("contract", mapping.Currency).Msg("Failed to fetch price")
			lastErr = err
			report.Results = append(report.Results, ContractFetchResult{
				Currency: mapping.Currency,
				Error:    err.Error(),
			})
			report.Failed++
			continue
		}
		allRecords = append(allRecords, records...)

		// Determine source from first record
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

		// Rate limit between calls to respect API limits
		select {
		case <-ctx.Done():
			report.Duration = time.Since(start)
			return allRecords, report, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	report.Duration = time.Since(start)

	if len(allRecords) == 0 && lastErr != nil {
		return nil, report, fmt.Errorf("all price fetches failed, last error: %w", lastErr)
	}

	log.Info().Int("records", len(allRecords)).Msg("Price fetch complete")
	return allRecords, report, nil
}

// FetchWeekly fetches weekly OHLC data for a single contract.
// Tries: TwelveData → AlphaVantage → Yahoo Finance.
func (f *Fetcher) FetchWeekly(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	// Try Twelve Data first (if key configured and symbol available)
	if f.twelveDataKey != "" && mapping.TwelveData != "" {
		records, err := f.fetchTwelveData(ctx, mapping, weeks)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("symbol", mapping.TwelveData).Msg("TwelveData failed, trying AlphaVantage")
		}
	}

	// Try Alpha Vantage (if keys configured and spec available)
	if len(f.avKeys) > 0 && mapping.AlphaVantage.Function != "" {
		records, err := f.fetchAlphaVantage(ctx, mapping, weeks)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("function", mapping.AlphaVantage.Function).Msg("AlphaVantage failed, trying Yahoo")
		}
	}

	// Fallback to Yahoo Finance (no key needed)
	if mapping.Yahoo != "" {
		records, err := f.fetchYahoo(ctx, mapping, weeks)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			return nil, fmt.Errorf("all sources failed for %s: yahoo: %w", mapping.Currency, err)
		}
	}

	return nil, fmt.Errorf("no price source available for %s", mapping.Currency)
}

// HealthCheck verifies that at least one price API is reachable.
func (f *Fetcher) HealthCheck(ctx context.Context) error {
	// Try to fetch 1 week of EUR/USD from any source
	mapping := domain.DefaultPriceSymbolMappings[0] // EUR
	_, err := f.FetchWeekly(ctx, mapping, 1)
	return err
}

// --- Twelve Data ---

func (f *Fetcher) fetchTwelveData(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	var records []domain.PriceRecord

	err := f.cbTwelveData.Execute(func() error {
		url := fmt.Sprintf(
			"https://api.twelvedata.com/time_series?symbol=%s&interval=1week&outputsize=%d&apikey=%s",
			mapping.TwelveData, weeks, f.twelveDataKey,
		)

		body, err := f.doGet(ctx, url, nil)
		if err != nil {
			return fmt.Errorf("twelvedata request: %w", err)
		}

		var resp twelveDataResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("twelvedata parse: %w", err)
		}

		if resp.Status != "ok" {
			return fmt.Errorf("twelvedata error: code=%d msg=%s", resp.Code, resp.Msg)
		}

		records = make([]domain.PriceRecord, 0, len(resp.Values))
		for _, v := range resp.Values {
			t, err := time.Parse("2006-01-02", v.Datetime)
			if err != nil {
				continue
			}
			rec := domain.PriceRecord{
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

		sortRecordsByDate(records)
		if len(records) > weeks {
			records = records[:weeks]
		}
		return nil
	})

	return records, err
}

// --- Alpha Vantage ---

func (f *Fetcher) fetchAlphaVantage(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	var records []domain.PriceRecord

	err := f.cbAlphaVantage.Execute(func() error {
		spec := mapping.AlphaVantage
		key := f.nextAVKey()

		var url string
		switch spec.Function {
		case "FX_WEEKLY":
			url = fmt.Sprintf(
				"https://www.alphavantage.co/query?function=FX_WEEKLY&from_symbol=%s&to_symbol=%s&apikey=%s",
				spec.FromSymbol, spec.ToSymbol, key,
			)
		case "WTI", "BRENT", "NATURAL_GAS":
			url = fmt.Sprintf(
				"https://www.alphavantage.co/query?function=%s&interval=weekly&apikey=%s",
				spec.Function, key,
			)
		case "TREASURY_YIELD":
			url = fmt.Sprintf(
				"https://www.alphavantage.co/query?function=TREASURY_YIELD&interval=weekly&maturity=10year&apikey=%s",
				key,
			)
		case "GOLD_SILVER_SPOT":
			// Gold spot is real-time only, not historical. For historical gold, use FX_WEEKLY XAU/USD
			// or fall through to Yahoo.
			return fmt.Errorf("gold spot is real-time only, skipping AV for historical")
		default:
			return fmt.Errorf("unsupported AV function: %s", spec.Function)
		}

		body, err := f.doGet(ctx, url, nil)
		if err != nil {
			return fmt.Errorf("alphavantage request: %w", err)
		}

		var parseErr error
		switch spec.Function {
		case "FX_WEEKLY":
			records, parseErr = f.parseAVFXWeekly(body, mapping, weeks)
		default:
			records, parseErr = f.parseAVCommodity(body, mapping, weeks)
		}
		return parseErr
	})

	return records, err
}

func (f *Fetcher) parseAVFXWeekly(body []byte, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	var resp avFXWeeklyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse AV FX response: %w", err)
	}
	if resp.Note != "" || resp.Info != "" {
		return nil, fmt.Errorf("AV rate limited: %s%s", resp.Note, resp.Info)
	}
	if len(resp.TimeSeries) == 0 {
		return nil, fmt.Errorf("AV FX empty response")
	}

	records := make([]domain.PriceRecord, 0, len(resp.TimeSeries))
	for dateStr, ohlc := range resp.TimeSeries {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		rec := domain.PriceRecord{
			ContractCode: mapping.ContractCode,
			Symbol:       fmt.Sprintf("%s/%s", mapping.AlphaVantage.FromSymbol, mapping.AlphaVantage.ToSymbol),
			Date:         t,
			Open:         parseFloat(ohlc.Open),
			High:         parseFloat(ohlc.High),
			Low:          parseFloat(ohlc.Low),
			Close:        parseFloat(ohlc.Close),
			Source:       "alphavantage",
		}
		if rec.Close > 0 {
			records = append(records, rec)
		}
	}

	sortRecordsByDate(records)
	if len(records) > weeks {
		records = records[:weeks]
	}
	return records, nil
}

func (f *Fetcher) parseAVCommodity(body []byte, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	var resp avCommodityResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse AV commodity response: %w", err)
	}
	if resp.Note != "" || resp.Info != "" {
		return nil, fmt.Errorf("AV rate limited: %s%s", resp.Note, resp.Info)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("AV commodity empty response")
	}

	records := make([]domain.PriceRecord, 0, weeks)
	for _, d := range resp.Data {
		if d.Value == "." || d.Value == "" {
			continue // AV uses "." for missing data
		}
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			continue
		}
		price := parseFloat(d.Value)
		if price <= 0 {
			continue
		}
		rec := domain.PriceRecord{
			ContractCode: mapping.ContractCode,
			Symbol:       mapping.AlphaVantage.Function,
			Date:         t,
			Open:         price,
			High:         price,
			Low:          price,
			Close:        price, // Commodity/Treasury only has single value
			Source:       "alphavantage",
		}
		records = append(records, rec)
	}

	sortRecordsByDate(records)
	if len(records) > weeks {
		records = records[:weeks]
	}
	return records, nil
}

// --- Yahoo Finance ---

func (f *Fetcher) fetchYahoo(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	var records []domain.PriceRecord

	err := f.cbYahoo.Execute(func() error {
		// Yahoo range parameter: convert weeks to range string
		rangeStr := "1y"
		if weeks > 52 {
			rangeStr = fmt.Sprintf("%dy", (weeks/52)+1)
		} else if weeks <= 13 {
			rangeStr = "3mo"
		} else if weeks <= 26 {
			rangeStr = "6mo"
		}

		url := fmt.Sprintf(
			"https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=1wk&range=%s",
			mapping.Yahoo, rangeStr,
		)

		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		}

		body, err := f.doGet(ctx, url, headers)
		if err != nil {
			return fmt.Errorf("yahoo request: %w", err)
		}

		var resp yahooChartResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("yahoo parse: %w", err)
		}

		if resp.Chart.Error != nil {
			return fmt.Errorf("yahoo error: %s: %s", resp.Chart.Error.Code, resp.Chart.Error.Description)
		}

		if len(resp.Chart.Result) == 0 {
			return fmt.Errorf("yahoo empty response for %s", mapping.Yahoo)
		}

		result := resp.Chart.Result[0]
		if len(result.Indicators.Quote) == 0 {
			return fmt.Errorf("yahoo no quote data for %s", mapping.Yahoo)
		}

		quote := result.Indicators.Quote[0]
		records = make([]domain.PriceRecord, 0, len(result.Timestamp))

		for i, ts := range result.Timestamp {
			if i >= len(quote.Close) {
				break
			}
			// Yahoo uses nullable floats
			if quote.Close[i] == nil || quote.Open[i] == nil {
				continue
			}

			t := time.Unix(ts, 0).UTC()
			rec := domain.PriceRecord{
				ContractCode: mapping.ContractCode,
				Symbol:       mapping.Yahoo,
				Date:         t,
				Open:         derefFloat(quote.Open[i]),
				High:         derefFloat(safeIndex(quote.High, i)),
				Low:          derefFloat(safeIndex(quote.Low, i)),
				Close:        derefFloat(quote.Close[i]),
				Source:       "yahoo",
			}
			if rec.Close > 0 {
				records = append(records, rec)
			}
		}

		sortRecordsByDate(records)
		if len(records) > weeks {
			records = records[:weeks]
		}
		return nil
	})

	return records, err
}

// --- Helpers ---

func (f *Fetcher) nextAVKey() string {
	if len(f.avKeys) == 0 {
		return ""
	}
	idx := atomic.AddUint64(&f.avKeyIndex, 1)
	return f.avKeys[idx%uint64(len(f.avKeys))]
}

func (f *Fetcher) doGet(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func derefFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func safeIndex(s []*float64, i int) *float64 {
	if i >= len(s) {
		return nil
	}
	return s[i]
}

// sortRecordsByDate sorts price records newest-first.
func sortRecordsByDate(records []domain.PriceRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date.After(records[j].Date)
	})
}
