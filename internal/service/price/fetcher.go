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
	"github.com/arkcode369/ark-intelligent/pkg/errs"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("price")

// Fetcher implements ports.PriceFetcher with 3-layer resilience:
// Twelve Data (primary) → Alpha Vantage (secondary) → Yahoo Finance → Stooq.com (fallback).
// CoinGecko is used as a dedicated provider for crypto market cap data (TOTAL3).
type Fetcher struct {
	httpClient     *http.Client
	twelveDataKeys []string
	tdKeyIndex     uint64 // atomic counter for round-robin TD key selection
	avKeys         []string
	avKeyIndex     uint64 // atomic counter for round-robin AV key selection
	coinGeckoKey   string
	cbTwelveData   *circuitbreaker.Breaker
	cbAlphaVantage *circuitbreaker.Breaker
	cbYahoo        *circuitbreaker.Breaker
	cbCoinGecko    *circuitbreaker.Breaker
	cbStooq        *circuitbreaker.Breaker
}

// NewFetcher creates a new price fetcher with the given API keys.
// Both keys are optional — Yahoo Finance fallback requires no key.
func NewFetcher(twelveDataKeys []string, avKeys []string) *Fetcher {
	return &Fetcher{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		twelveDataKeys: twelveDataKeys,
		avKeys:         avKeys,
		cbTwelveData:   circuitbreaker.New("twelve-data", 3, 5*time.Minute),
		cbAlphaVantage: circuitbreaker.New("alpha-vantage", 3, 5*time.Minute),
		cbYahoo:        circuitbreaker.New("yahoo-finance", 5, 3*time.Minute),
		cbCoinGecko:    circuitbreaker.New("coingecko", 3, 5*time.Minute),
		cbStooq:        circuitbreaker.New("stooq", 3, 5*time.Minute),
	}
}

// SetCoinGeckoKey sets the CoinGecko API key for TOTAL3 market cap data.
func (f *Fetcher) SetCoinGeckoKey(key string) {
	f.coinGeckoKey = key
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

	// Fetch COT-contract price mappings (excludes risk-only and synthetic instruments).
	for _, mapping := range domain.COTPriceSymbolMappings() {
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

	// Fetch price-only instruments (cross pairs, CoinGecko TOTAL3).
	for _, mapping := range domain.PriceOnlyMappings() {
		records, err := f.FetchWeekly(ctx, mapping, weeks)
		if err != nil {
			log.Warn().Err(err).Str("instrument", mapping.Currency).Msg("Price-only instrument fetch failed — skipping")
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

	report.Duration = time.Since(start)

	if len(allRecords) == 0 && lastErr != nil {
		return nil, report, fmt.Errorf("all price fetches failed, last error: %w", lastErr)
	}

	log.Info().Int("records", len(allRecords)).Msg("Price fetch complete")
	return allRecords, report, nil
}

// FetchRiskInstruments fetches VIX and SPX weekly price data (risk-only instruments).
// These are fetched separately from COT contracts and stored with their synthetic
// contract codes ("risk_VIX", "risk_SPX") for use in the risk context builder.
// Returns nil error if fetching partially fails — callers should treat missing
// risk data as "no adjustment" rather than a hard failure.
func (f *Fetcher) FetchRiskInstruments(ctx context.Context, weeks int) ([]domain.PriceRecord, error) {
	var allRecords []domain.PriceRecord
	for _, mapping := range domain.RiskPriceSymbolMappings() {
		records, err := f.FetchWeekly(ctx, mapping, weeks)
		if err != nil {
			log.Warn().Err(err).Str("instrument", mapping.Currency).Msg("Risk instrument fetch failed — skipping")
			continue
		}
		allRecords = append(allRecords, records...)
	}
	return allRecords, nil
}

// FetchWeekly fetches weekly OHLC data for a single contract.
// Tries: CoinGecko (if applicable) → TwelveData → AlphaVantage → Yahoo Finance → Stooq → Synthetic cross.
func (f *Fetcher) FetchWeekly(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	// CoinGecko-sourced instruments (TOTAL3) — dedicated provider, no fallback chain
	if mapping.CoinGecko != "" && f.coinGeckoKey != "" {
		records, err := f.fetchCoinGecko(ctx, mapping, weeks)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("id", mapping.CoinGecko).Msg("CoinGecko failed")
		}
		// Fall through to Yahoo if available
	}

	// Try Twelve Data first (if key configured and symbol available)
	if len(f.twelveDataKeys) > 0 && mapping.TwelveData != "" {
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
			log.Debug().Err(err).Str("symbol", mapping.Yahoo).Msg("Yahoo failed")
		}
	}

	// Fallback to Stooq.com (free, no API key — forex historical OHLCV)
	if stooqSymbol(mapping.Currency) != "" {
		records, err := f.fetchStooq(ctx, mapping, weeks)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("currency", mapping.Currency).Msg("Stooq failed")
		}
	}

	// Final fallback: synthetic cross pair calculation (e.g. XAU/EUR = XAU/USD ÷ EUR/USD)
	if cross, ok := SyntheticCrossDef(mapping.Currency); ok {
		records, err := f.fetchSyntheticCross(ctx, mapping, cross, weeks)
		if err == nil && len(records) > 0 {
			return records, nil
		}
		if err != nil {
			log.Debug().Err(err).Str("cross", mapping.Currency).Msg("Synthetic cross failed")
		}
	}

	return nil, errs.Wrapf(errs.ErrNotFound, "no price source for %s", mapping.Currency)
}

// HealthCheck verifies that at least one price API is reachable.
// RISK-M3 fix: look up EUR by currency name instead of using a fragile index.
func (f *Fetcher) HealthCheck(ctx context.Context) error {
	// Use EUR/USD as the canary symbol (always present, liquid, all sources support it).
	mapping := domain.FindPriceMappingByCurrency("EUR")
	if mapping == nil {
		return fmt.Errorf("health check: EUR mapping not found in price symbol table")
	}
	_, err := f.FetchWeekly(ctx, *mapping, 1)
	return err
}

// --- Twelve Data ---

func (f *Fetcher) fetchTwelveData(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	var records []domain.PriceRecord

	err := f.cbTwelveData.Execute(func() error {
		url := fmt.Sprintf(
			"https://api.twelvedata.com/time_series?symbol=%s&interval=1week&outputsize=%d&apikey=%s",
			mapping.TwelveData, weeks, f.nextTDKey(),
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
				Volume:       parseFloat(v.Volume),
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
		case "WTI", "BRENT", "NATURAL_GAS", "COPPER":
			url = fmt.Sprintf(
				"https://www.alphavantage.co/query?function=%s&interval=weekly&apikey=%s",
				spec.Function, key,
			)
		case "TREASURY_YIELD":
			url = fmt.Sprintf(
				"https://www.alphavantage.co/query?function=TREASURY_YIELD&interval=weekly&maturity=10year&apikey=%s",
				key,
			)
		case "GOLD_SILVER_SPOT", "SILVER":
			// Gold/Silver spot is real-time only, not historical. For historical, use FX_WEEKLY XAU/USD
			// or fall through to Yahoo.
			return fmt.Errorf("gold/silver spot is real-time only, skipping AV for historical")
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
		return nil, errs.Wrapf(errs.ErrRateLimited, "alphavantage: %s%s", resp.Note, resp.Info)
	}
	if len(resp.TimeSeries) == 0 {
		return nil, errs.Wrap(errs.ErrNoData, "alphavantage FX")
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
		return nil, errs.Wrapf(errs.ErrRateLimited, "alphavantage: %s%s", resp.Note, resp.Info)
	}
	if len(resp.Data) == 0 {
		return nil, errs.Wrap(errs.ErrNoData, "alphavantage commodity")
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
			return errs.Wrapf(errs.ErrNoData, "yahoo empty response for %s", mapping.Yahoo)
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
			if i < len(quote.Volume) && quote.Volume[i] != nil {
				rec.Volume = *quote.Volume[i]
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

// --- Synthetic Cross Pairs ---

// CrossDef defines components for computing a synthetic cross pair.
// CrossDef defines components for computing a synthetic cross pair.
type CrossDef struct {
	NumeratorCurrency   string // e.g. "XAU" → fetched as XAU/USD (GC=F)
	DenominatorCurrency string // e.g. "EUR" → fetched as EUR/USD (EURUSD=X)
}

// SyntheticCrossDef returns the cross definition for a synthetic cross pair.
func SyntheticCrossDef(currency string) (CrossDef, bool) {
	defs := map[string]CrossDef{
		"XAUEUR": {NumeratorCurrency: "XAU", DenominatorCurrency: "EUR"},
		"XAUGBP": {NumeratorCurrency: "XAU", DenominatorCurrency: "GBP"},
		"XAGEUR": {NumeratorCurrency: "XAG", DenominatorCurrency: "EUR"},
		"XAGGBP": {NumeratorCurrency: "XAG", DenominatorCurrency: "GBP"},
	}
	d, ok := defs[currency]
	return d, ok
}

// fetchSyntheticCross computes a cross pair from two USD-based price series.
// E.g. XAU/EUR = XAU/USD ÷ EUR/USD
// Uses ISO-week matching since different Yahoo symbols have slightly different timestamps.
func (f *Fetcher) fetchSyntheticCross(ctx context.Context, mapping domain.PriceSymbolMapping, cross CrossDef, weeks int) ([]domain.PriceRecord, error) {
	numMapping := domain.FindPriceMappingByCurrency(cross.NumeratorCurrency)
	denMapping := domain.FindPriceMappingByCurrency(cross.DenominatorCurrency)
	if numMapping == nil || denMapping == nil {
		return nil, fmt.Errorf("synthetic cross: missing mapping for %s or %s", cross.NumeratorCurrency, cross.DenominatorCurrency)
	}

	numRecords, err := f.FetchWeekly(ctx, *numMapping, weeks)
	if err != nil {
		return nil, fmt.Errorf("synthetic cross numerator %s: %w", cross.NumeratorCurrency, err)
	}
	denRecords, err := f.FetchWeekly(ctx, *denMapping, weeks)
	if err != nil {
		return nil, fmt.Errorf("synthetic cross denominator %s: %w", cross.DenominatorCurrency, err)
	}

	// Build ISO-week-indexed map for denominator (handles different Yahoo timestamps)
	denMap := make(map[string]domain.PriceRecord)
	for _, r := range denRecords {
		y, w := r.Date.ISOWeek()
		key := fmt.Sprintf("%d-%02d", y, w)
		denMap[key] = r
	}

	var records []domain.PriceRecord
	for _, num := range numRecords {
		y, w := num.Date.ISOWeek()
		key := fmt.Sprintf("%d-%02d", y, w)
		den, ok := denMap[key]
		if !ok || den.Close == 0 || den.Open == 0 {
			continue
		}
		rec := domain.PriceRecord{
			ContractCode: mapping.ContractCode,
			Symbol:       mapping.Currency,
			Date:         num.Date,
			Open:         num.Open / den.Open,
			High:         num.High / den.Low, // max ratio when num is high and den is low
			Low:          num.Low / den.High,  // min ratio when num is low and den is high
			Close:        num.Close / den.Close,
			Source:       "synthetic",
		}
		if rec.Close > 0 {
			records = append(records, rec)
		}
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("synthetic cross: no matching dates for %s", mapping.Currency)
	}

	sortRecordsByDate(records)
	if len(records) > weeks {
		records = records[:weeks]
	}
	return records, nil
}

// --- Helpers ---

// --- CoinGecko ---

// coinGeckoDays maps weeks to the nearest valid CoinGecko days parameter.
// CoinGecko only accepts: 1, 7, 14, 30, 90, 180, 365, max.
func coinGeckoDays(weeks int) string {
	days := weeks * 7
	switch {
	case days <= 7:
		return "7"
	case days <= 14:
		return "14"
	case days <= 30:
		return "30"
	case days <= 90:
		return "90"
	case days <= 180:
		return "180"
	case days <= 365:
		return "365"
	default:
		return "max"
	}
}

// fetchCoinGecko fetches TOTAL3 (altcoin market cap excl BTC+ETH) from CoinGecko.
// Strategy: fetch BTC and ETH market cap charts, then use /global for total, compute TOTAL3 = total - BTC - ETH.
// Uses demo API (api.coingecko.com) since CG- keys are demo tier.
func (f *Fetcher) fetchCoinGecko(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	var records []domain.PriceRecord

	err := f.cbCoinGecko.Execute(func() error {
		daysParam := coinGeckoDays(weeks)
		headers := map[string]string{
			"x-cg-demo-api-key": f.coinGeckoKey,
		}

		// Fetch BTC, ETH, and total crypto market cap charts in sequence
		// (total uses /coins/list with category filter isn't available, so we approximate
		// by using a stablecoin-excluded total from individual large-cap queries)
		btcData, err := f.fetchCoinGeckoMarketChart(ctx, "bitcoin", daysParam, headers)
		if err != nil {
			return fmt.Errorf("coingecko btc: %w", err)
		}
		time.Sleep(300 * time.Millisecond) // Rate limit

		ethData, err := f.fetchCoinGeckoMarketChart(ctx, "ethereum", daysParam, headers)
		if err != nil {
			return fmt.Errorf("coingecko eth: %w", err)
		}
		time.Sleep(300 * time.Millisecond)

		// Get current total market cap from /global to establish the ratio
		globalBody, err := f.doGet(ctx, "https://api.coingecko.com/api/v3/global", headers)
		if err != nil {
			return fmt.Errorf("coingecko global: %w", err)
		}
		var globalResp struct {
			Data struct {
				TotalMarketCap      map[string]float64 `json:"total_market_cap"`
				MarketCapPercentage map[string]float64 `json:"market_cap_percentage"`
			} `json:"data"`
		}
		if err := json.Unmarshal(globalBody, &globalResp); err != nil {
			return fmt.Errorf("coingecko global parse: %w", err)
		}

		totalMcap := globalResp.Data.TotalMarketCap["usd"]
		btcDom := globalResp.Data.MarketCapPercentage["btc"] / 100.0
		if totalMcap == 0 {
			return fmt.Errorf("coingecko: total market cap is zero")
		}

		// Build time-indexed BTC and ETH market cap maps
		btcMap := make(map[string]float64)
		for _, dp := range btcData {
			if len(dp) >= 2 {
				t := time.Unix(int64(dp[0])/1000, 0).UTC()
				btcMap[t.Format("2006-01-02")] = dp[1]
			}
		}
		ethMap := make(map[string]float64)
		for _, dp := range ethData {
			if len(dp) >= 2 {
				t := time.Unix(int64(dp[0])/1000, 0).UTC()
				ethMap[t.Format("2006-01-02")] = dp[1]
			}
		}

		// Estimate historical total market cap from BTC dominance:
		// total_mcap_t ≈ btc_mcap_t / btc_dominance_current
		// Then TOTAL3_t = total_mcap_t - btc_mcap_t - eth_mcap_t
		// This approximation is reasonable for short periods (dominance changes slowly).
		type dailyPoint struct {
			date  time.Time
			value float64
		}

		var daily []dailyPoint
		for _, dp := range btcData {
			if len(dp) < 2 || btcDom == 0 {
				continue
			}
			t := time.Unix(int64(dp[0])/1000, 0).UTC()
			dateKey := t.Format("2006-01-02")
			btcMcap := dp[1]
			estTotal := btcMcap / btcDom
			ethMcap := ethMap[dateKey]
			total3 := estTotal - btcMcap - ethMcap
			if total3 > 0 {
				daily = append(daily, dailyPoint{date: t, value: total3})
			}
		}

		if len(daily) == 0 {
			return fmt.Errorf("coingecko: no TOTAL3 data computed")
		}

		// Aggregate into weekly records
		weekMap := make(map[string]*domain.PriceRecord)
		for _, d := range daily {
			year, week := d.date.ISOWeek()
			key := fmt.Sprintf("%d-%02d", year, week)

			if rec, ok := weekMap[key]; ok {
				if d.value > rec.High {
					rec.High = d.value
				}
				if d.value < rec.Low {
					rec.Low = d.value
				}
				if d.date.After(rec.Date) {
					rec.Close = d.value
					rec.Date = d.date
				}
			} else {
				weekMap[key] = &domain.PriceRecord{
					ContractCode: mapping.ContractCode,
					Symbol:       "TOTAL3",
					Date:         d.date,
					Open:         d.value,
					High:         d.value,
					Low:          d.value,
					Close:        d.value,
					Source:       "coingecko",
				}
			}
		}

		records = make([]domain.PriceRecord, 0, len(weekMap))
		for _, rec := range weekMap {
			records = append(records, *rec)
		}

		sortRecordsByDate(records)
		if len(records) > weeks {
			records = records[:weeks]
		}
		return nil
	})

	return records, err
}

// fetchCoinGeckoMarketChart fetches historical market cap for a specific coin (demo API).
func (f *Fetcher) fetchCoinGeckoMarketChart(ctx context.Context, coinID, days string, headers map[string]string) ([][]float64, error) {
	url := fmt.Sprintf(
		"https://api.coingecko.com/api/v3/coins/%s/market_chart?days=%s&vs_currency=usd",
		coinID, days,
	)
	body, err := f.doGet(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("coingecko %s chart: %w", coinID, err)
	}
	var resp struct {
		MarketCaps [][]float64 `json:"market_caps"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("coingecko %s parse: %w", coinID, err)
	}
	return resp.MarketCaps, nil
}

// --- Helpers (general) ---

func (f *Fetcher) nextAVKey() string {
	if len(f.avKeys) == 0 {
		return ""
	}
	idx := atomic.AddUint64(&f.avKeyIndex, 1)
	return f.avKeys[idx%uint64(len(f.avKeys))]
}

func (f *Fetcher) nextTDKey() string {
	if len(f.twelveDataKeys) == 0 {
		return ""
	}
	idx := atomic.AddUint64(&f.tdKeyIndex, 1)
	return f.twelveDataKeys[idx%uint64(len(f.twelveDataKeys))]
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

// FetchSpotPrice fetches the current real-time spot price for a contract.
func (f *Fetcher) FetchSpotPrice(ctx context.Context, contractCode string) (float64, error) {
	mapping := domain.FindPriceMapping(contractCode)
	if mapping == nil || mapping.Yahoo == "" {
		return 0, fmt.Errorf("no Yahoo symbol for contract %s", contractCode)
	}

	var spotPrice float64

	err := f.cbYahoo.Execute(func() error {
		url := fmt.Sprintf(
			"https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1d&interval=1d",
			mapping.Yahoo,
		)

		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		}

		body, err := f.doGet(ctx, url, headers)
		if err != nil {
			return fmt.Errorf("yahoo spot request: %w", err)
		}

		var resp yahooChartResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("yahoo spot parse: %w", err)
		}

		if resp.Chart.Error != nil {
			return fmt.Errorf("yahoo spot error: %s: %s", resp.Chart.Error.Code, resp.Chart.Error.Description)
		}

		if len(resp.Chart.Result) == 0 {
			return errs.Wrapf(errs.ErrNoData, "yahoo spot empty response for %s", mapping.Yahoo)
		}

		result := resp.Chart.Result[0]

		// Try regularMarketPrice first
		if result.Meta.RegularMarketPrice > 0 {
			spotPrice = result.Meta.RegularMarketPrice
			return nil
		}

		// Fallback to last close value from indicators
		if len(result.Indicators.Quote) > 0 {
			closes := result.Indicators.Quote[0].Close
			for i := len(closes) - 1; i >= 0; i-- {
				if closes[i] != nil && *closes[i] > 0 {
					spotPrice = *closes[i]
					return nil
				}
			}
		}

		return fmt.Errorf("yahoo spot: no price data for %s", mapping.Yahoo)
	})

	return spotPrice, err
}

// sortRecordsByDate sorts price records newest-first.
func sortRecordsByDate(records []domain.PriceRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date.After(records[j].Date)
	})
}
