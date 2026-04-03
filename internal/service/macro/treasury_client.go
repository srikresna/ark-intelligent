// Package macro integrates free public macro data sources (Treasury.gov, etc.)
// that are independent from FRED — providing redundancy and institutional authority.
package macro

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/errs"
	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("macro")

// cacheTTL for Treasury data (updated daily, cache 12h).
const cacheTTL = 12 * time.Hour

// httpTimeout for Treasury.gov HTTP requests.
const httpTimeout = 20 * time.Second

// tenors defines the yield curve tenors we fetch (years).
var tenors = []string{"5", "7", "10", "20", "30"}

// YieldRow holds yield data for a single date.
type YieldRow struct {
	Date        time.Time
	Y5, Y7, Y10, Y20, Y30 float64 // yield in percent
}

// Breakevens holds computed breakeven inflation per tenor for a single date.
type Breakevens struct {
	Date                   time.Time
	BE5, BE7, BE10, BE20, BE30             float64 // nominal - real, in percent
	Nominal5, Nominal10, Nominal30         float64
	Real5, Real10, Real30                  float64
}

// IsZero returns true if no breakeven data is available.
func (b *Breakevens) IsZero() bool {
	return b == nil || b.Date.IsZero()
}

// TreasuryClient fetches TIPS real yields and nominal yields from Treasury.gov.
// All data is free, no API key required.
type TreasuryClient struct {
	mu       sync.Mutex
	cached   *Breakevens
	cachedAt time.Time
	hc       *http.Client
}

// NewTreasuryClient creates a new TreasuryClient with a default HTTP client.
func NewTreasuryClient() *TreasuryClient {
	return &TreasuryClient{
		hc: httpclient.New(httpclient.WithTimeout(httpTimeout)),
	}
}

// GetBreakevens returns the most recent breakeven inflation rates.
// Returns cached data if fresh (< cacheTTL). Thread-safe.
func (c *TreasuryClient) GetBreakevens(ctx context.Context) (*Breakevens, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.cachedAt) < cacheTTL {
		return c.cached, nil
	}

	result, err := c.fetchBreakevens(ctx)
	if err != nil {
		// Return stale cache on error if available.
		if c.cached != nil {
			log.Warn().Err(err).Msg("Treasury fetch failed, returning stale cache")
			return c.cached, nil
		}
		return nil, err
	}

	c.cached = result
	c.cachedAt = time.Now()
	return result, nil
}

// fetchBreakevens fetches both TIPS and nominal yields then computes breakevens.
func (c *TreasuryClient) fetchBreakevens(ctx context.Context) (*Breakevens, error) {
	year := time.Now().Year()

	// Fetch TIPS real yields.
	realRows, err := c.fetchYields(ctx, year, "daily_treasury_real_yield_curve")
	if err != nil || len(realRows) == 0 {
		// Try previous year if current year has no data yet (early Jan).
		if err == nil && len(realRows) == 0 {
			realRows, err = c.fetchYields(ctx, year-1, "daily_treasury_real_yield_curve")
		}
		if err != nil {
			return nil, fmt.Errorf("fetch TIPS yields: %w", err)
		}
		if len(realRows) == 0 {
			return nil, errs.Wrap(errs.ErrNoData, "TIPS yield")
		}
	}

	// Fetch nominal yields.
	nomRows, err := c.fetchYields(ctx, year, "daily_treasury_yield_curve")
	if err != nil || len(nomRows) == 0 {
		if err == nil && len(nomRows) == 0 {
			nomRows, err = c.fetchYields(ctx, year-1, "daily_treasury_yield_curve")
		}
		if err != nil {
			return nil, fmt.Errorf("fetch nominal yields: %w", err)
		}
		if len(nomRows) == 0 {
			return nil, errs.Wrap(errs.ErrNoData, "nominal yield")
		}
	}

	// Use most recent row from each.
	real := realRows[len(realRows)-1]
	nom := nomRows[len(nomRows)-1]

	be := &Breakevens{
		Date:      nom.Date,
		BE5:       safeBreakeven(nom.Y5, real.Y5),
		BE7:       safeBreakeven(nom.Y7, real.Y7),
		BE10:      safeBreakeven(nom.Y10, real.Y10),
		BE20:      safeBreakeven(nom.Y20, real.Y20),
		BE30:      safeBreakeven(nom.Y30, real.Y30),
		Nominal5:  nom.Y5,
		Nominal10: nom.Y10,
		Nominal30: nom.Y30,
		Real5:     real.Y5,
		Real10:    real.Y10,
		Real30:    real.Y30,
	}

	log.Info().
		Str("date", be.Date.Format("2006-01-02")).
		Float64("be5", be.BE5).
		Float64("be10", be.BE10).
		Float64("be30", be.BE30).
		Msg("Treasury breakevens fetched")

	return be, nil
}

// fetchYields downloads and parses a Treasury.gov yield curve CSV.
// rateType is "daily_treasury_real_yield_curve" or "daily_treasury_yield_curve".
func (c *TreasuryClient) fetchYields(ctx context.Context, year int, rateType string) ([]YieldRow, error) {
	url := fmt.Sprintf(
		"https://home.treasury.gov/resource-center/data-chart-center/interest-rates/daily-treasury-rates.csv/%d/all?type=%s&field_tdr_date_value=%d&page&_format=csv",
		year, rateType, year,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "ark-intelligent-bot/1.0 (+https://github.com/arkcode369/ark-intelligent)")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errs.Wrapf(errs.ErrUpstream, "Treasury API HTTP %d for %s", resp.StatusCode, rateType)
	}

	return parseYieldCSV(resp.Body)
}

// parseYieldCSV parses the Treasury.gov yield curve CSV format.
// The CSV has a "Date" column and tenor columns (e.g. "5 Yr", "10 Yr", etc.).
func parseYieldCSV(r io.Reader) ([]YieldRow, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Find column indices for date and tenors we care about.
	dateIdx := -1
	tenorIdx := map[string]int{} // "5" → column index

	for i, h := range headers {
		h = strings.TrimSpace(h)
		lower := strings.ToLower(h)
		if lower == "date" {
			dateIdx = i
			continue
		}
		// Match patterns: "5 Yr", "7 Yr", "10 Yr", "20 Yr", "30 Yr"
		for _, t := range tenors {
			if strings.HasPrefix(lower, t+" yr") || lower == t+"yr" {
				tenorIdx[t] = i
			}
		}
	}

	if dateIdx < 0 {
		return nil, errs.Wrapf(errs.ErrInvalidFormat, "Treasury CSV: no 'Date' column (headers: %v)", headers)
	}

	var rows []YieldRow
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}
		if len(record) <= dateIdx {
			continue
		}

		dateStr := strings.TrimSpace(record[dateIdx])
		if dateStr == "" {
			continue
		}

		// Treasury uses M/D/YYYY format.
		t, parseErr := time.Parse("01/02/2006", dateStr)
		if parseErr != nil {
			t, parseErr = time.Parse("2006-01-02", dateStr)
			if parseErr != nil {
				continue
			}
		}

		row := YieldRow{Date: t}
		for tenor, idx := range tenorIdx {
			if idx >= len(record) {
				continue
			}
			v := parseYieldFloat(record[idx])
			switch tenor {
			case "5":
				row.Y5 = v
			case "7":
				row.Y7 = v
			case "10":
				row.Y10 = v
			case "20":
				row.Y20 = v
			case "30":
				row.Y30 = v
			}
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// safeBreakeven computes nominal - real.
// Returns 0 if either input is 0 (data unavailable).
func safeBreakeven(nominal, real float64) float64 {
	if nominal == 0 || real == 0 {
		return 0
	}
	return nominal - real
}

// parseYieldFloat parses a yield value string, returning 0 for empty/invalid.
func parseYieldFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" || s == "NA" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// FormatBreakevens formats breakeven inflation data for Telegram HTML display.
func FormatBreakevens(be *Breakevens) string {
	if be == nil || be.IsZero() {
		return "<i>Treasury TIPS data unavailable</i>"
	}

	signal := func(v float64) string {
		switch {
		case v > 2.8:
			return "🔴" // well above Fed target → hawkish / USD bullish
		case v > 2.0:
			return "🟡" // above or near target
		default:
			return "🟢" // below target → dovish
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 <b>Treasury Breakeven Inflation</b> (%s)\n", be.Date.Format("Jan 2, 2006")))
	sb.WriteString("<code>")
	sb.WriteString(fmt.Sprintf("%-5s  %-8s %-7s %s\n", "Tenor", "Nominal", "TIPS", "Breakeven"))
	sb.WriteString(fmt.Sprintf("%-5s  %-8s %-7s %s\n", "─────", "───────", "─────", "─────────"))

	type row struct {
		tenor   string
		nominal float64
		real    float64
		be      float64
	}
	rows := []row{
		{"5Y", be.Nominal5, be.Real5, be.BE5},
		{"10Y", be.Nominal10, be.Real10, be.BE10},
		{"30Y", be.Nominal30, be.Real30, be.BE30},
	}
	for _, r := range rows {
		if r.nominal == 0 && r.real == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("%-5s  %6.2f%%  %5.2f%%  %s %.2f%%\n",
			r.tenor, r.nominal, r.real, signal(r.be), r.be))
	}
	sb.WriteString("</code>\n")

	// Market signal interpretation.
	if be.BE10 > 0 {
		switch {
		case be.BE10 > 2.8:
			sb.WriteString("⚠️ <b>Elevated breakevens</b> — inflation risk priced above Fed target. USD <b>bullish</b> (rate hikes repriced higher)\n")
		case be.BE10 > 2.3:
			sb.WriteString("📈 <b>Breakevens above Fed 2% target</b> — mild hawkish USD pressure\n")
		case be.BE10 > 1.7:
			sb.WriteString("✅ <b>Breakevens near target</b> — inflation expectations anchored, neutral\n")
		default:
			sb.WriteString("📉 <b>Low breakevens</b> — deflationary risk. Dovish USD pressure\n")
		}
	}

	return sb.String()
}
