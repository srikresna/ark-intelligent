package macro

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"math"

	"github.com/arkcode369/ark-intelligent/pkg/errs"
	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// ---------------------------------------------------------------------------
// OECD Composite Leading Indicators (CLI)
// ---------------------------------------------------------------------------
//
// The OECD CLI is a 6-9 month forward-looking macro indicator.
// Cross-country CLI divergence predicts FX trends: e.g., US CLI rising
// vs EU CLI falling = bullish USD / bearish EUR.
//
// API: https://sdmx.oecd.org (free, no key required)
// Data: monthly, amplitude-adjusted CLI index for OECD countries.
// ---------------------------------------------------------------------------

const (
	oecdHTTPTimeout = 25 * time.Second
	oecdCacheTTL    = 24 * time.Hour
	oecdAPIBase     = "https://sdmx.oecd.org/public/rest/data"
	oecdCLIFlow     = "OECD.SDD.STES,DSD_STES@DF_CLI"
)

// oecdFXCountries maps OECD country codes to forex-relevant names.
var oecdFXCountries = map[string]string{
	"USA": "🇺🇸 US",
	"G7":  "🌍 G7",
	"GBR": "🇬🇧 UK",
	"JPN": "🇯🇵 JP",
	"CAN": "🇨🇦 CA",
	"AUS": "🇦🇺 AU",
	"DEU": "🇩🇪 DE",
	"FRA": "🇫🇷 FR",
	"CHN": "🇨🇳 CN",
	"KOR": "🇰🇷 KR",
	"IND": "🇮🇳 IN",
	"BRA": "🇧🇷 BR",
}

// CLIDataPoint holds a single CLI reading for a country+month.
type CLIDataPoint struct {
	Country   string  // ISO3 code e.g. "USA"
	Name      string  // Display name e.g. "🇺🇸 US"
	Period    string  // YYYY-MM
	Value     float64 // CLI index (100 = long-term trend)
	Momentum  float64 // month-over-month change
}

// OECDCLIData holds the full CLI dataset for forex-relevant countries.
type OECDCLIData struct {
	FetchedAt time.Time
	// Latest holds the most recent CLI value per country.
	Latest map[string]*CLIDataPoint
	// History holds the last N months per country (most recent first).
	History map[string][]*CLIDataPoint
}

// IsZero returns true if no OECD data has been fetched.
func (d *OECDCLIData) IsZero() bool {
	return d == nil || d.FetchedAt.IsZero()
}

// OECDClient fetches OECD Composite Leading Indicators via SDMX REST API.
type OECDClient struct {
	mu       sync.Mutex
	cached   *OECDCLIData
	cachedAt time.Time
	hc       *http.Client
}

// NewOECDClient creates a new OECDClient.
func NewOECDClient() *OECDClient {
	return &OECDClient{
		hc: httpclient.New(httpclient.WithTimeout(oecdHTTPTimeout)),
	}
}

// GetCLIData returns the latest OECD CLI data, using cache if fresh.
func (c *OECDClient) GetCLIData(ctx context.Context) (*OECDCLIData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.cachedAt) < oecdCacheTTL {
		return c.cached, nil
	}

	data, err := c.fetchCLI(ctx)
	if err != nil {
		if c.cached != nil {
			log.Warn().Err(err).Msg("OECD CLI fetch failed, returning stale cache")
			return c.cached, nil
		}
		return nil, err
	}

	c.cached = data
	c.cachedAt = time.Now()
	return data, nil
}

// fetchCLI fetches CLI data from the OECD SDMX API as CSV.
func (c *OECDClient) fetchCLI(ctx context.Context) (*OECDCLIData, error) {
	// Fetch last 18 months of data to compute momentum
	startPeriod := time.Now().AddDate(0, -18, 0).Format("2006-01")
	url := fmt.Sprintf("%s/%s/.M.LI...AA...H?startPeriod=%s&format=csvfilewithlabels",
		oecdAPIBase, oecdCLIFlow, startPeriod)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errs.Wrapf(errs.ErrUpstream, "OECD request build: %v", err)
	}
	req.Header.Set("Accept", "text/csv")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, errs.Wrapf(errs.ErrUpstream, "OECD fetch: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errs.Wrapf(errs.ErrUpstream, "OECD API status %d", resp.StatusCode)
	}

	return c.parseCLICSV(resp.Body)
}

// parseCLICSV parses the OECD SDMX CSV response into structured data.
func (c *OECDClient) parseCLICSV(body io.Reader) (*OECDCLIData, error) {
	reader := csv.NewReader(body)
	reader.LazyQuotes = true

	headers, err := reader.Read()
	if err != nil {
		return nil, errs.Wrapf(errs.ErrInvalidFormat, "OECD CSV header read: %v", err)
	}

	// Find column indices
	colIdx := make(map[string]int)
	for i, h := range headers {
		colIdx[h] = i
	}

	refAreaIdx, ok1 := colIdx["REF_AREA"]
	periodIdx, ok2 := colIdx["TIME_PERIOD"]
	valueIdx, ok3 := colIdx["OBS_VALUE"]
	if !ok1 || !ok2 || !ok3 {
		return nil, errs.Wrapf(errs.ErrInvalidFormat, "OECD CSV missing required columns (have: %v)", headers)
	}

	// Parse all rows, grouped by country
	type rawPoint struct {
		period string
		value  float64
	}
	byCountry := make(map[string][]rawPoint)

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}
		if len(row) <= valueIdx || len(row) <= refAreaIdx || len(row) <= periodIdx {
			continue
		}

		country := row[refAreaIdx]
		// Only keep forex-relevant countries
		if _, ok := oecdFXCountries[country]; !ok {
			continue
		}

		val, err := strconv.ParseFloat(strings.TrimSpace(row[valueIdx]), 64)
		if err != nil {
			continue
		}

		period := strings.TrimSpace(row[periodIdx])
		byCountry[country] = append(byCountry[country], rawPoint{period: period, value: val})
	}

	if len(byCountry) == 0 {
		return nil, errs.Wrap(errs.ErrNoData, "OECD CLI: no valid observations for target countries")
	}

	data := &OECDCLIData{
		FetchedAt: time.Now(),
		Latest:    make(map[string]*CLIDataPoint),
		History:   make(map[string][]*CLIDataPoint),
	}

	for country, points := range byCountry {
		// Sort by period descending (most recent first)
		sort.Slice(points, func(i, j int) bool {
			return points[i].period > points[j].period
		})

		name := oecdFXCountries[country]
		var history []*CLIDataPoint
		for i, pt := range points {
			dp := &CLIDataPoint{
				Country: country,
				Name:    name,
				Period:  pt.period,
				Value:   pt.value,
			}
			// Compute month-over-month momentum
			if i+1 < len(points) {
				dp.Momentum = pt.value - points[i+1].value
			}
			history = append(history, dp)
		}

		data.History[country] = history
		if len(history) > 0 {
			data.Latest[country] = history[0]
		}
	}

	return data, nil
}

// ---------------------------------------------------------------------------
// Package-level singleton
// ---------------------------------------------------------------------------

var defaultOECDClient = NewOECDClient()

// GetOECDCLIData returns the latest OECD CLI data using the package-level client.
func GetOECDCLIData(ctx context.Context) (*OECDCLIData, error) {
	return defaultOECDClient.GetCLIData(ctx)
}

// FormatOECDCLIData formats OECD CLI data for Telegram HTML display.
func FormatOECDCLIData(d *OECDCLIData) string {
	if d == nil || d.IsZero() || len(d.Latest) == 0 {
		return "❌ OECD CLI data tidak tersedia."
	}

	var sb strings.Builder
	sb.WriteString("📊 <b>OECD Composite Leading Indicators</b>\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("<i>CLI > 100 = expansion, &lt; 100 = contraction</i>\n\n")

	// Sort countries by latest CLI value (descending — strongest first)
	type entry struct {
		code string
		dp   *CLIDataPoint
	}
	var entries []entry
	for code, dp := range d.Latest {
		entries = append(entries, entry{code: code, dp: dp})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].dp.Value > entries[j].dp.Value
	})

	// Render each country
	for _, e := range entries {
		dp := e.dp
		arrow := "→"
		if dp.Momentum > 0.05 {
			arrow = "📈"
		} else if dp.Momentum < -0.05 {
			arrow = "📉"
		}

		zone := "⚪"
		if dp.Value > 100.5 {
			zone = "🟢" // strong expansion
		} else if dp.Value > 100 {
			zone = "🔵" // mild expansion
		} else if dp.Value > 99.5 {
			zone = "🟡" // near trend
		} else {
			zone = "🔴" // contraction
		}

		sb.WriteString(fmt.Sprintf("%s %s  <b>%.2f</b> %s %+.2f\n",
			zone, dp.Name, dp.Value, arrow, dp.Momentum))
	}

	// Key divergences
	sb.WriteString("\n<b>🔑 Key Divergences (FX Signals)</b>\n")
	divergences := computeCLIDivergences(d)
	if len(divergences) == 0 {
		sb.WriteString("   Tidak ada divergensi signifikan saat ini\n")
	} else {
		for _, div := range divergences {
			sb.WriteString(fmt.Sprintf("   %s\n", div))
		}
	}

	// Period info
	if len(entries) > 0 {
		sb.WriteString(fmt.Sprintf("\n<i>Period: %s</i>\n", entries[0].dp.Period))
	}
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📝 <i>Source: OECD SDMX API (sdmx.oecd.org)</i>\n")
	sb.WriteString("<i>Updated monthly · 6-9 month forward-looking indicator</i>")

	return sb.String()
}

// computeCLIDivergences finds notable CLI divergences between forex-paired countries.
func computeCLIDivergences(d *OECDCLIData) []string {
	type pair struct {
		a, b   string
		fxPair string
	}
	pairs := []pair{
		{"USA", "DEU", "EUR/USD"},
		{"USA", "JPN", "USD/JPY"},
		{"USA", "GBR", "GBP/USD"},
		{"USA", "CAN", "USD/CAD"},
		{"AUS", "USA", "AUD/USD"},
	}

	var divs []string
	for _, p := range pairs {
		a, okA := d.Latest[p.a]
		b, okB := d.Latest[p.b]
		if !okA || !okB {
			continue
		}

		spread := a.Value - b.Value
		momDiff := a.Momentum - b.Momentum

		// Only report if spread > 0.5 or momentum diverges > 0.15
		if math.Abs(spread) < 0.5 && math.Abs(momDiff) < 0.15 {
			continue
		}

		var signal string
		if spread > 0.5 {
			signal = fmt.Sprintf("%s: %s CLI leads by %.1fpt → %s bullish bias",
				p.fxPair, oecdFXCountries[p.a], spread, p.a)
		} else if spread < -0.5 {
			signal = fmt.Sprintf("%s: %s CLI leads by %.1fpt → %s bullish bias",
				p.fxPair, oecdFXCountries[p.b], -spread, p.b)
		} else if momDiff > 0.15 {
			signal = fmt.Sprintf("%s: %s momentum diverging (%+.2f vs %+.2f)",
				p.fxPair, oecdFXCountries[p.a], a.Momentum, b.Momentum)
		} else if momDiff < -0.15 {
			signal = fmt.Sprintf("%s: %s momentum diverging (%+.2f vs %+.2f)",
				p.fxPair, oecdFXCountries[p.b], b.Momentum, a.Momentum)
		}

		if signal != "" {
			divs = append(divs, signal)
		}
	}
	return divs
}

// abs returns the absolute value of a float64.
