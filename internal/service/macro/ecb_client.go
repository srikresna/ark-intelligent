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
)

// ecbHTTPTimeout for ECB SDW API requests.
const ecbHTTPTimeout = 20 * time.Second

// ecbCacheTTL for ECB data (monthly updates, cache 24h).
const ecbCacheTTL = 24 * time.Hour

// ecbAPIBase is the ECB Statistical Data Warehouse REST API base URL.
const ecbAPIBase = "https://data-api.ecb.europa.eu/service/data"

// ECBData holds the latest ECB monetary policy indicators.
type ECBData struct {
	FetchedAt time.Time

	// Key interest rate (Main Refinancing Rate)
	MRRDate  time.Time
	MRRValue float64 // percent

	// M3 money supply growth (year-over-year, percent)
	M3Date  time.Time
	M3YoY   float64

	// EUR/USD official exchange rate (monthly average)
	EURUSDDate  time.Time
	EURUSDValue float64
}

// IsZero returns true if no ECB data has been fetched.
func (d *ECBData) IsZero() bool {
	return d == nil || d.FetchedAt.IsZero()
}

// ECBClient fetches data from the ECB Statistical Data Warehouse (SDW).
// No API key required — free public data.
type ECBClient struct {
	mu       sync.Mutex
	cached   *ECBData
	cachedAt time.Time
	hc       *http.Client
}

// NewECBClient creates a new ECBClient.
func NewECBClient() *ECBClient {
	return &ECBClient{
		hc: &http.Client{Timeout: ecbHTTPTimeout},
	}
}

// GetData returns the latest ECB data, using cache if fresh.
func (c *ECBClient) GetData(ctx context.Context) (*ECBData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.cachedAt) < ecbCacheTTL {
		return c.cached, nil
	}

	data, err := c.fetchAll(ctx)
	if err != nil {
		if c.cached != nil {
			log.Warn().Err(err).Msg("ECB fetch failed, returning stale cache")
			return c.cached, nil
		}
		return nil, err
	}

	c.cached = data
	c.cachedAt = time.Now()
	return data, nil
}

// fetchAll fetches all ECB data series concurrently.
func (c *ECBClient) fetchAll(ctx context.Context) (*ECBData, error) {
	type result struct {
		name string
		date time.Time
		val  float64
		err  error
	}

	series := []struct {
		name   string
		flowID string
		key    string
	}{
		{
			name:   "mrr",
			flowID: "FM",
			key:    "B.U2.EUR.4F.KR.MRR_FR.LEV",
		},
		{
			name:   "m3",
			flowID: "BSI",
			key:    "M.U2.Y.V.M30.X.I.U2.2300.Z01.E",
		},
		{
			name:   "eurusd",
			flowID: "EXR",
			key:    "M.USD.EUR.SP00.A",
		},
	}

	results := make(chan result, len(series))
	for _, s := range series {
		s := s
		go func() {
			date, val, err := c.fetchLatestObservation(ctx, s.flowID, s.key)
			results <- result{name: s.name, date: date, val: val, err: err}
		}()
	}

	data := &ECBData{FetchedAt: time.Now()}
	var fetchErrs []string
	for range series {
		r := <-results
		if r.err != nil {
			fetchErrs = append(fetchErrs, fmt.Sprintf("%s: %v", r.name, r.err))
			continue
		}
		switch r.name {
		case "mrr":
			data.MRRDate = r.date
			data.MRRValue = r.val
		case "m3":
			data.M3Date = r.date
			data.M3YoY = r.val
		case "eurusd":
			data.EURUSDDate = r.date
			data.EURUSDValue = r.val
		}
	}

	// Return partial data with a warning if some series failed.
	if len(fetchErrs) > 0 && data.MRRValue == 0 && data.M3YoY == 0 && data.EURUSDValue == 0 {
		return nil, errs.Wrapf(errs.ErrUpstream, "all ECB series failed: %s", strings.Join(fetchErrs, "; "))
	}
	if len(fetchErrs) > 0 {
		log.Warn().Strs("errors", fetchErrs).Msg("some ECB series failed, returning partial data")
	}

	return data, nil
}

// fetchLatestObservation fetches the most recent observation for a given ECB series.
// Uses the ECB SDW REST API CSV format.
func (c *ECBClient) fetchLatestObservation(ctx context.Context, flowID, key string) (time.Time, float64, error) {
	url := fmt.Sprintf("%s/%s/%s?lastNObservations=1&format=csvdata", ecbAPIBase, flowID, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/csv")

	resp, err := c.hc.Do(req)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, 0, errs.Wrapf(errs.ErrUpstream, "ECB API status %d for %s/%s", resp.StatusCode, flowID, key)
	}

	return parseECBCSV(resp.Body)
}

// parseECBCSV parses the ECB SDW CSV response and returns the latest observation.
// ECB CSV format: header row + data rows with columns including TIME_PERIOD and OBS_VALUE.
func parseECBCSV(r io.Reader) (time.Time, float64, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("read CSV header: %w", err)
	}

	timeIdx := -1
	valueIdx := -1
	for i, h := range headers {
		switch strings.TrimSpace(strings.ToUpper(h)) {
		case "TIME_PERIOD":
			timeIdx = i
		case "OBS_VALUE":
			valueIdx = i
		}
	}
	if timeIdx == -1 || valueIdx == -1 {
		return time.Time{}, 0, errs.Wrapf(errs.ErrInvalidFormat, "ECB CSV missing TIME_PERIOD or OBS_VALUE column (headers: %v)", headers)
	}

	// Read all rows, use the last one (most recent with lastNObservations=1 there's only 1).
	var lastDate time.Time
	var lastVal float64
	found := false

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}
		if timeIdx >= len(row) || valueIdx >= len(row) {
			continue
		}

		rawDate := strings.TrimSpace(row[timeIdx])
		rawVal := strings.TrimSpace(row[valueIdx])
		if rawVal == "" || rawVal == "NaN" || rawVal == "-" {
			continue
		}

		val, parseErr := strconv.ParseFloat(rawVal, 64)
		if parseErr != nil {
			continue
		}

		date := parseECBDate(rawDate)
		if date.IsZero() {
			continue
		}

		lastDate = date
		lastVal = val
		found = true
	}

	if !found {
		return time.Time{}, 0, errs.Wrap(errs.ErrNoData, "ECB CSV: no valid observations")
	}

	return lastDate, lastVal, nil
}

// parseECBDate parses ECB date strings: "YYYY-MM" (monthly) or "YYYY-MM-DD" (daily).
func parseECBDate(s string) time.Time {
	if t, err := time.Parse("2006-01", s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return time.Time{}
}

// ---------------------------------------------------------------------------
// Package-level singleton
// ---------------------------------------------------------------------------

var defaultECBClient = NewECBClient()

// GetECBData returns the latest ECB data using the package-level client.
func GetECBData(ctx context.Context) (*ECBData, error) {
	return defaultECBClient.GetData(ctx)
}

// FormatECBData formats ECB data for Telegram HTML display.
func FormatECBData(d *ECBData) string {
	if d == nil || d.IsZero() {
		return "❌ ECB data tidak tersedia."
	}

	var sb strings.Builder
	sb.WriteString("🏦 <b>ECB Monetary Policy Dashboard</b>\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Key Rate
	if d.MRRValue != 0 {
		sb.WriteString("📊 <b>Key Interest Rate (MRR)</b>\n")
		sb.WriteString(fmt.Sprintf("   Rate: <b>%.2f%%</b>", d.MRRValue))
		if !d.MRRDate.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", d.MRRDate.Format("Jan 2006")))
		}
		sb.WriteString("\n\n")
	}

	// M3 Money Supply
	if d.M3YoY != 0 {
		m3Arrow := "→"
		if d.M3YoY > 5 {
			m3Arrow = "📈"
		} else if d.M3YoY < 2 {
			m3Arrow = "📉"
		}
		sb.WriteString("💰 <b>M3 Money Supply (YoY)</b>\n")
		sb.WriteString(fmt.Sprintf("   Growth: <b>%+.1f%%</b> %s", d.M3YoY, m3Arrow))
		if !d.M3Date.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", d.M3Date.Format("Jan 2006")))
		}
		sb.WriteString("\n\n")
	}

	// EUR/USD Rate
	if d.EURUSDValue != 0 {
		sb.WriteString("💱 <b>EUR/USD Official Rate</b>\n")
		sb.WriteString(fmt.Sprintf("   Rate: <b>%.4f</b>", d.EURUSDValue))
		if !d.EURUSDDate.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", d.EURUSDDate.Format("Jan 2006")))
		}
		sb.WriteString("\n\n")
	}

	// Interpretation
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📝 <i>Sumber: ECB Statistical Data Warehouse (sdw.ecb.europa.eu)</i>\n")
	sb.WriteString("<i>Data bulanan, diperbarui setiap bulan oleh ECB</i>")

	return sb.String()
}
