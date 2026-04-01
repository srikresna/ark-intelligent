package price

import (
	"bytes"
	"context"
	"math"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// stooqSymbols maps ark-intelligent currency codes to Stooq pair symbols.
// Stooq uses lowercase pair notation (e.g. "eurusd").
var stooqSymbols = map[string]string{
	"EUR":    "eurusd",
	"GBP":    "gbpusd",
	"JPY":    "usdjpy",
	"AUD":    "audusd",
	"NZD":    "nzdusd",
	"CAD":    "usdcad",
	"CHF":    "usdchf",
	"XAUUSD": "xauusd",
	"XAGUSD": "xagusd",
}

// stooqSymbol returns the Stooq pair symbol for the given currency code, or empty string.
func stooqSymbol(currency string) string {
	return stooqSymbols[currency]
}

// fetchStooq fetches weekly OHLCV data from Stooq.com CSV endpoint.
// Stooq provides free historical data without an API key.
// URL pattern: https://stooq.com/q/d/l/?s={pair}.fx&i=w
func (f *Fetcher) fetchStooq(ctx context.Context, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	sym := stooqSymbol(mapping.Currency)
	if sym == "" {
		return nil, fmt.Errorf("stooq: no symbol mapping for %s", mapping.Currency)
	}

	var records []domain.PriceRecord

	err := f.cbStooq.Execute(func() error {
		url := fmt.Sprintf("https://stooq.com/q/d/l/?s=%s.fx&i=w", sym)

		headers := map[string]string{
			"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		}

		body, err := f.doGet(ctx, url, headers)
		if err != nil {
			return fmt.Errorf("stooq request: %w", err)
		}

		parsed, err := parseStooqCSV(body, mapping, weeks)
		if err != nil {
			return err
		}
		if len(parsed) == 0 {
			return fmt.Errorf("stooq: no data rows for %s", sym)
		}
		records = parsed
		return nil
	})

	return records, err
}

// parseStooqCSV parses the Stooq CSV response into PriceRecords.
// Stooq CSV format:
//
//	Date,Open,High,Low,Close,Volume
//	2026-03-28,1.08234,1.08901,1.07123,1.08456,123456
//
// Records are returned sorted newest-first and trimmed to the requested week count.
func parseStooqCSV(body []byte, mapping domain.PriceSymbolMapping, weeks int) ([]domain.PriceRecord, error) {
	reader := csv.NewReader(bytes.NewReader(body))
	reader.TrimLeadingSpace = true

	allRows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("stooq csv parse: %w", err)
	}

	if len(allRows) < 2 {
		return nil, fmt.Errorf("stooq csv: insufficient rows (%d)", len(allRows))
	}

	// Find column indices from header
	header := allRows[0]
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	dateCol, okD := colIdx["date"]
	openCol, okO := colIdx["open"]
	highCol, okH := colIdx["high"]
	lowCol, okL := colIdx["low"]
	closeCol, okC := colIdx["close"]
	if !okD || !okO || !okH || !okL || !okC {
		return nil, fmt.Errorf("stooq csv: missing required columns (have: %v)", header)
	}
	volCol, hasVol := colIdx["volume"]

	records := make([]domain.PriceRecord, 0, len(allRows)-1)
	for _, row := range allRows[1:] {
		if len(row) <= closeCol {
			continue
		}

		date, parseErr := time.Parse("2006-01-02", strings.TrimSpace(row[dateCol]))
		if parseErr != nil {
			continue // skip unparseable rows
		}

		open, parseErr := strconv.ParseFloat(strings.TrimSpace(row[openCol]), 64)
		if parseErr != nil || open <= 0 || math.IsNaN(open) {
			continue
		}
		high, parseErr := strconv.ParseFloat(strings.TrimSpace(row[highCol]), 64)
		if parseErr != nil {
			continue
		}
		low, parseErr := strconv.ParseFloat(strings.TrimSpace(row[lowCol]), 64)
		if parseErr != nil {
			continue
		}
		closeVal, parseErr := strconv.ParseFloat(strings.TrimSpace(row[closeCol]), 64)
		if parseErr != nil || closeVal <= 0 || math.IsNaN(closeVal) {
			continue
		}

		var vol float64
		if hasVol && len(row) > volCol {
			vol, _ = strconv.ParseFloat(strings.TrimSpace(row[volCol]), 64)
		}

		rec := domain.PriceRecord{
			ContractCode: mapping.ContractCode,
			Symbol:       stooqSymbol(mapping.Currency) + ".fx",
			Date:         date,
			Open:         open,
			High:         high,
			Low:          low,
			Close:        closeVal,
			Volume:       vol,
			Source:       "stooq",
		}
		records = append(records, rec)
	}

	// Sort newest first
	sortRecordsByDate(records)

	// Trim to requested weeks
	if len(records) > weeks {
		records = records[:weeks]
	}

	return records, nil
}
