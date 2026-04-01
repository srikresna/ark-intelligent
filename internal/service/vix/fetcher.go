package vix

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	stdlog "log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	vxEODURL   = "https://cdn.cboe.com/api/global/us_indices/daily_prices/VX_EOD.csv"
	vixEODURL  = "https://cdn.cboe.com/api/global/us_indices/daily_prices/VIX_EOD.csv"
	vvixEODURL = "https://cdn.cboe.com/api/global/us_indices/daily_prices/VVIX_EOD.csv"
)

// FetchTermStructure fetches VIX spot, VIX futures, and VVIX from CBOE,
// then computes the term structure regime.
//
// Returns a result with Available=false and Error set on failure.
// Never returns a nil pointer.
func FetchTermStructure(ctx context.Context) (*VIXTermStructure, error) {
	ts := &VIXTermStructure{AsOf: time.Now().UTC()}
	client := &http.Client{Timeout: 15 * time.Second}

	// 1. VIX spot
	spot, err := fetchSingleIndexCSV(ctx, client, vixEODURL)
	if err != nil {
		ts.Error = fmt.Sprintf("VIX spot fetch failed: %v", err)
		return ts, nil // graceful fallback
	}
	ts.Spot = spot

	// 2. VVIX
	vvix, err := fetchSingleIndexCSV(ctx, client, vvixEODURL)
	if err == nil {
		ts.VVIX = vvix
	}
	// VVIX failure is non-fatal

	// 3. VIX futures (VX_EOD.csv)
	if err := fetchVXFutures(ctx, client, ts); err != nil {
		ts.Error = fmt.Sprintf("VX futures fetch failed: %v", err)
		// Can still use spot-only result
		ts.Available = ts.Spot > 0
		if ts.Available {
			classifyRegime(ts)
		}
		return ts, nil
	}

	computeDerivedFields(ts)
	ts.Available = true

	// Fetch MOVE index for cross-asset vol comparison (non-fatal)
	moveData, moveErr := FetchMOVE(ctx, ts.Spot)
	if moveErr != nil {
		stdlog.Printf("MOVE fetch failed (non-fatal): %v", moveErr)
	} else if moveData.Available {
		ts.MOVE = moveData
	}

	return ts, nil
}

// ---------------------------------------------------------------------------
// CSV parsing helpers
// ---------------------------------------------------------------------------

// fetchSingleIndexCSV fetches an EOD index CSV (VIX or VVIX) and returns the
// last-row close price. Column layout: Date, Open, High, Low, Close, ...
func fetchSingleIndexCSV(ctx context.Context, client *http.Client, url string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	r := csv.NewReader(resp.Body)
	r.FieldsPerRecord = -1 // allow variable columns

	var lastClose float64
	header := true
	rowNum := 0
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			stdlog.Printf("vix: fetchSingleIndexCSV: skipping row %d: %v", rowNum, err)
			continue
		}
		if header {
			header = false
			continue
		}
		// Date, Open, High, Low, Close
		if len(row) < 5 {
			continue
		}
		v, parseErr := strconv.ParseFloat(strings.TrimSpace(row[4]), 64)
		if parseErr == nil && v > 0 {
			lastClose = v
		}
	}

	if lastClose == 0 {
		return 0, fmt.Errorf("no valid close price found in %s", url)
	}
	return lastClose, nil
}

// vxRow holds parsed data for a single VX futures contract row.
type vxRow struct {
	symbol  string
	settle  float64
	expiry  time.Time // parsed from symbol
}

// fetchVXFutures parses VX_EOD.csv and populates M1/M2/M3 settle prices.
// CSV header: Trade Date,Futures,Open,High,Low,Close,Settle,Change,%Change,Volume,EFP,Open Interest
func fetchVXFutures(ctx context.Context, client *http.Client, ts *VIXTermStructure) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, vxEODURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from VX_EOD", resp.StatusCode)
	}

	r := csv.NewReader(resp.Body)
	r.FieldsPerRecord = -1

	// Read all rows, find the latest trade date
	type rawRow struct {
		date   string
		symbol string
		settle string
	}
	var rows []rawRow

	header := true
	rowNum := 0
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			stdlog.Printf("vix: fetchVXFutures: skipping row %d: %v", rowNum, err)
			continue
		}
		if header {
			header = false
			continue
		}
		if len(row) < 7 {
			continue
		}
		rows = append(rows, rawRow{
			date:   strings.TrimSpace(row[0]),
			symbol: strings.TrimSpace(row[1]),
			settle: strings.TrimSpace(row[6]),
		})
	}

	if len(rows) == 0 {
		return fmt.Errorf("no rows in VX_EOD.csv")
	}

	// Find latest trade date
	latestDate := rows[len(rows)-1].date
	for _, rw := range rows {
		if rw.date > latestDate {
			latestDate = rw.date
		}
	}

	// Collect contracts from the latest date
	var contracts []vxRow
	for _, rw := range rows {
		if rw.date != latestDate {
			continue
		}
		settle, parseErr := strconv.ParseFloat(rw.settle, 64)
		if parseErr != nil || settle <= 0 {
			continue
		}
		expiry := parseVXSymbolExpiry(rw.symbol)
		if expiry.IsZero() {
			continue
		}
		contracts = append(contracts, vxRow{
			symbol: rw.symbol,
			settle: settle,
			expiry: expiry,
		})
	}

	if len(contracts) == 0 {
		return fmt.Errorf("no valid contracts on latest date %s", latestDate)
	}

	// Sort by expiry ascending (M1=nearest)
	sort.Slice(contracts, func(i, j int) bool {
		return contracts[i].expiry.Before(contracts[j].expiry)
	})

	// Populate M1, M2, M3
	if len(contracts) >= 1 {
		ts.M1 = contracts[0].settle
		ts.M1Symbol = contracts[0].symbol
	}
	if len(contracts) >= 2 {
		ts.M2 = contracts[1].settle
		ts.M2Symbol = contracts[1].symbol
	}
	if len(contracts) >= 3 {
		ts.M3 = contracts[2].settle
		ts.M3Symbol = contracts[2].symbol
	}

	return nil
}

// vxMonthCodes maps CBOE VX futures month codes to month numbers.
var vxMonthCodes = map[byte]int{
	'F': 1, 'G': 2, 'H': 3, 'J': 4, 'K': 5, 'M': 6,
	'N': 7, 'Q': 8, 'U': 9, 'V': 10, 'X': 11, 'Z': 12,
}

// parseVXSymbolExpiry extracts the expiry month/year from a VX symbol.
// Format: /VX{Month}{Year2} e.g. "/VXK26" = May 2026
func parseVXSymbolExpiry(symbol string) time.Time {
	// Strip leading "/" and "VX" prefix
	clean := strings.TrimPrefix(symbol, "/")
	clean = strings.TrimPrefix(clean, "VX")
	if len(clean) < 3 {
		return time.Time{}
	}

	monthCode := clean[0]
	monthNum, ok := vxMonthCodes[monthCode]
	if !ok {
		return time.Time{}
	}

	yearStr := clean[1:]
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return time.Time{}
	}
	if year < 100 {
		year += 2000 // two-digit year
	}

	// Expiry is the Wednesday before the third Friday of the month (approximate)
	// For sorting purposes, use the 1st of the month
	return time.Date(year, time.Month(monthNum), 1, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// Derived field computation
// ---------------------------------------------------------------------------

// computeDerivedFields calculates SlopePct, Contango, Backwardation, RollYield,
// and Regime from the raw M1/M2/Spot values.
func computeDerivedFields(ts *VIXTermStructure) {
	if ts.M1 > 0 && ts.M2 > 0 {
		ts.SlopePct = (ts.M2 - ts.M1) / ts.M1 * 100
		// Approximate monthly roll yield: negative for long VIX in contango
		ts.RollYield = -ts.SlopePct
	} else if ts.M1 == 0 {
		// Defensive: M1 zero from malformed CSV → set safe defaults
		ts.SlopePct = 0
		ts.RollYield = 0
	}

	// Guard against NaN/Inf from unexpected calculations
	if math.IsNaN(ts.SlopePct) || math.IsInf(ts.SlopePct, 0) {
		ts.SlopePct = 0
		ts.RollYield = 0
	}

	if ts.Spot > 0 && ts.M1 > 0 {
		ts.Backwardation = ts.M1 < ts.Spot
		ts.Contango = ts.M1 > ts.Spot && (ts.M2 == 0 || ts.M2 >= ts.M1)
	}

	classifyRegime(ts)
}

// classifyRegime assigns the Regime string based on term structure shape.
func classifyRegime(ts *VIXTermStructure) {
	if ts.Spot == 0 {
		ts.Regime = "UNKNOWN"
		return
	}

	if ts.Backwardation {
		if ts.Spot > 0 && ts.M1 > 0 && ts.Spot > ts.M1*1.10 {
			ts.Regime = "EXTREME_FEAR"
		} else {
			ts.Regime = "FEAR"
		}
		return
	}

	slope := ts.SlopePct
	switch {
	case slope < 3:
		ts.Regime = "ELEVATED" // flat term structure, vol still elevated
	case slope < 7:
		ts.Regime = "RISK_ON_NORMAL"
	default:
		ts.Regime = "RISK_ON_COMPLACENT" // steep contango = max complacency
	}
}
