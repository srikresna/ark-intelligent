package macro

// eurostat_client.go — Eurostat API integration for EU macro indicators.
// Fetches EU HICP inflation, unemployment rate, and GDP growth from the
// Eurostat JSON-stat API. Free, no API key required.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/errs"
	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// eurostatHTTPTimeout for Eurostat API requests.
const eurostatHTTPTimeout = 20 * time.Second

// eurostatCacheTTL — monthly/quarterly data, cache 24h.
const eurostatCacheTTL = 24 * time.Hour

// eurostatAPIBase is the Eurostat dissemination API base URL.
const eurostatAPIBase = "https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// EurostatData holds the latest EU macro indicators from Eurostat.
type EurostatData struct {
	FetchedAt time.Time

	// HICP Inflation — annual rate of change (%)
	HICPDate     time.Time
	HICPHeadline float64 // All-items HICP (CP00)
	HICPCore     float64 // All-items excluding energy and unprocessed food (TOT_X_NRG_FOOD)

	// Unemployment rate — seasonally adjusted (%)
	UnempDate time.Time
	UnempRate float64

	// GDP growth — quarter-on-quarter, seasonally adjusted (%)
	GDPDate   time.Time
	GDPGrowth float64

	// Comparison context (latest US values for differential)
	USCPIHeadline float64 // from FRED, populated externally if available
	USUnempRate   float64 // from FRED, populated externally if available
}

// IsZero returns true if no Eurostat data has been fetched.
func (d *EurostatData) IsZero() bool {
	return d == nil || d.FetchedAt.IsZero()
}

// ---------------------------------------------------------------------------
// JSON-stat response parsing
// ---------------------------------------------------------------------------

// eurostatResponse represents the Eurostat JSON-stat 2.0 response.
type eurostatResponse struct {
	Value     map[string]float64       `json:"value"`
	Dimension map[string]eurostatDim   `json:"dimension"`
	ID        []string                 `json:"id"`
	Size      []int                    `json:"size"`
}

type eurostatDim struct {
	Category eurostatCategory `json:"category"`
}

type eurostatCategory struct {
	Index map[string]int    `json:"index"`
	Label map[string]string `json:"label"`
}

// eurostatObservation is a parsed (period, value) pair.
type eurostatObservation struct {
	Period string
	Value  float64
}

// parseEurostatJSON parses a Eurostat JSON-stat response and returns
// observations sorted by time period (oldest first).
func parseEurostatJSON(body io.Reader) ([]eurostatObservation, error) {
	var resp eurostatResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode JSON-stat: %w", err)
	}

	// Find the time dimension
	timeDim, ok := resp.Dimension["time"]
	if !ok {
		return nil, errs.Wrap(errs.ErrInvalidFormat, "Eurostat: no 'time' dimension")
	}

	// Build index → period mapping
	periodByIndex := make(map[int]string, len(timeDim.Category.Index))
	for period, idx := range timeDim.Category.Index {
		periodByIndex[idx] = period
	}

	// Extract observations: the value map key is the flat index.
	// Since we filter to single geo/single indicator, the flat index
	// corresponds to the time dimension index.
	var obs []eurostatObservation
	for idxStr, val := range resp.Value {
		var idx int
		if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil {
			continue
		}
		period, exists := periodByIndex[idx]
		if !exists {
			continue
		}
		obs = append(obs, eurostatObservation{Period: period, Value: val})
	}

	// Sort by period ascending
	sort.Slice(obs, func(i, j int) bool {
		return obs[i].Period < obs[j].Period
	})

	return obs, nil
}

// parsePeriodDate converts Eurostat period strings to time.Time.
// Supports "YYYY-MM" (monthly) and "YYYY-QN" (quarterly).
func parsePeriodDate(period string) time.Time {
	if t, err := time.Parse("2006-01", period); err == nil {
		return t
	}
	// Quarterly: "2025-Q1" → 2025-01-01
	if len(period) == 7 && period[4] == '-' && period[5] == 'Q' {
		year := period[:4]
		q := period[6]
		month := "01"
		switch q {
		case '1':
			month = "01"
		case '2':
			month = "04"
		case '3':
			month = "07"
		case '4':
			month = "10"
		}
		if t, err := time.Parse("2006-01", year+"-"+month); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// EurostatClient fetches EU macro data from the Eurostat API.
type EurostatClient struct {
	mu       sync.Mutex
	cached   *EurostatData
	cachedAt time.Time
	hc       *http.Client
}

// NewEurostatClient creates a new EurostatClient.
func NewEurostatClient() *EurostatClient {
	return &EurostatClient{
		hc: httpclient.New(httpclient.WithTimeout(eurostatHTTPTimeout)),
	}
}

// GetData returns the latest Eurostat data, using cache if fresh.
func (c *EurostatClient) GetData(ctx context.Context) (*EurostatData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.cachedAt) < eurostatCacheTTL {
		return c.cached, nil
	}

	data, err := c.fetchAll(ctx)
	if err != nil {
		if c.cached != nil {
			log.Warn().Err(err).Msg("Eurostat fetch failed, returning stale cache")
			return c.cached, nil
		}
		return nil, err
	}

	c.cached = data
	c.cachedAt = time.Now()
	return data, nil
}

// fetchAll fetches all Eurostat series concurrently.
func (c *EurostatClient) fetchAll(ctx context.Context) (*EurostatData, error) {
	type result struct {
		name string
		obs  []eurostatObservation
		err  error
	}

	// Calculate since parameter: 12 months ago for monthly, 8 quarters for quarterly.
	now := time.Now()
	sinceMonthly := now.AddDate(0, -12, 0).Format("2006-01")
	sinceQuarterly := fmt.Sprintf("%d-Q1", now.Year()-2)

	queries := []struct {
		name string
		url  string
	}{
		{
			name: "hicp_headline",
			url: fmt.Sprintf("%s/prc_hicp_manr?geo=EA20&coicop=CP00&sinceTimePeriod=%s&format=JSON",
				eurostatAPIBase, sinceMonthly),
		},
		{
			name: "hicp_core",
			url: fmt.Sprintf("%s/prc_hicp_manr?geo=EA20&coicop=TOT_X_NRG_FOOD&sinceTimePeriod=%s&format=JSON",
				eurostatAPIBase, sinceMonthly),
		},
		{
			name: "unemployment",
			url: fmt.Sprintf("%s/une_rt_m?geo=EA20&s_adj=SA&age=TOTAL&sex=T&unit=PC_ACT&sinceTimePeriod=%s&format=JSON",
				eurostatAPIBase, sinceMonthly),
		},
		{
			name: "gdp",
			url: fmt.Sprintf("%s/namq_10_gdp?geo=EA20&unit=CLV_PCH_PRE&s_adj=SCA&na_item=B1GQ&sinceTimePeriod=%s&format=JSON",
				eurostatAPIBase, sinceQuarterly),
		},
	}

	results := make(chan result, len(queries))
	for _, q := range queries {
		q := q
		go func() {
			obs, err := c.fetchSeries(ctx, q.url)
			results <- result{name: q.name, obs: obs, err: err}
		}()
	}

	data := &EurostatData{FetchedAt: time.Now()}
	var fetchErrs []string

	for range queries {
		r := <-results
		if r.err != nil {
			fetchErrs = append(fetchErrs, fmt.Sprintf("%s: %v", r.name, r.err))
			continue
		}
		if len(r.obs) == 0 {
			fetchErrs = append(fetchErrs, fmt.Sprintf("%s: no observations", r.name))
			continue
		}

		// Use the most recent observation
		latest := r.obs[len(r.obs)-1]
		date := parsePeriodDate(latest.Period)

		switch r.name {
		case "hicp_headline":
			data.HICPDate = date
			data.HICPHeadline = latest.Value
		case "hicp_core":
			data.HICPCore = latest.Value
		case "unemployment":
			data.UnempDate = date
			data.UnempRate = latest.Value
		case "gdp":
			data.GDPDate = date
			data.GDPGrowth = latest.Value
		}
	}

	// Return partial data if at least one series succeeded
	if data.HICPHeadline == 0 && data.UnempRate == 0 && data.GDPGrowth == 0 {
		return nil, errs.Wrapf(errs.ErrUpstream, "all Eurostat series failed: %s", strings.Join(fetchErrs, "; "))
	}
	if len(fetchErrs) > 0 {
		log.Warn().Strs("errors", fetchErrs).Msg("some Eurostat series failed, returning partial data")
	}

	log.Info().
		Float64("hicp", data.HICPHeadline).
		Float64("core", data.HICPCore).
		Float64("unemp", data.UnempRate).
		Float64("gdp", data.GDPGrowth).
		Msg("Eurostat EU macro data fetched")

	return data, nil
}

// fetchSeries fetches a single Eurostat JSON-stat series.
func (c *EurostatClient) fetchSeries(ctx context.Context, url string) ([]eurostatObservation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "ark-intelligent-bot/1.0 (+https://github.com/arkcode369/ark-intelligent)")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errs.Wrapf(errs.ErrUpstream, "Eurostat HTTP %d for %s", resp.StatusCode, url)
	}

	return parseEurostatJSON(resp.Body)
}

// ---------------------------------------------------------------------------
// Package-level singleton
// ---------------------------------------------------------------------------

var defaultEurostatClient = NewEurostatClient()

// GetEurostatData returns the latest Eurostat data using the package-level client.
func GetEurostatData(ctx context.Context) (*EurostatData, error) {
	return defaultEurostatClient.GetData(ctx)
}

// ---------------------------------------------------------------------------
// Formatter
// ---------------------------------------------------------------------------

// FormatEurostatData formats Eurostat data for Telegram HTML display.
func FormatEurostatData(d *EurostatData) string {
	if d == nil || d.IsZero() {
		return "❌ EU macro data (Eurostat) tidak tersedia."
	}

	var sb strings.Builder
	sb.WriteString("🇪🇺 <b>EU Economy Dashboard (Eurostat)</b>\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// HICP Inflation
	if d.HICPHeadline != 0 {
		inflArrow := inflationArrow(d.HICPHeadline)
		sb.WriteString("📊 <b>HICP Inflation (YoY)</b>\n")
		sb.WriteString(fmt.Sprintf("   Headline: <b>%.1f%%</b> %s", d.HICPHeadline, inflArrow))
		if !d.HICPDate.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", d.HICPDate.Format("Jan 2006")))
		}
		sb.WriteString("\n")
		if d.HICPCore != 0 {
			coreArrow := inflationArrow(d.HICPCore)
			sb.WriteString(fmt.Sprintf("   Core:     <b>%.1f%%</b> %s\n", d.HICPCore, coreArrow))
		}
		// ECB target interpretation
		if d.HICPHeadline > 2.5 {
			sb.WriteString("   ⚠️ <i>Above ECB 2% target — hawkish pressure</i>\n")
		} else if d.HICPHeadline >= 1.5 && d.HICPHeadline <= 2.5 {
			sb.WriteString("   ✅ <i>Near ECB 2% target — balanced</i>\n")
		} else {
			sb.WriteString("   📉 <i>Below ECB target — dovish pressure</i>\n")
		}
		sb.WriteString("\n")
	}

	// Unemployment
	if d.UnempRate != 0 {
		unempEmoji := "📈"
		if d.UnempRate < 7.0 {
			unempEmoji = "✅"
		} else if d.UnempRate > 8.0 {
			unempEmoji = "⚠️"
		}
		sb.WriteString("👷 <b>Unemployment Rate (SA)</b>\n")
		sb.WriteString(fmt.Sprintf("   Rate: <b>%.1f%%</b> %s", d.UnempRate, unempEmoji))
		if !d.UnempDate.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", d.UnempDate.Format("Jan 2006")))
		}
		sb.WriteString("\n\n")
	}

	// GDP Growth
	if d.GDPGrowth != 0 {
		gdpEmoji := "📊"
		if d.GDPGrowth > 0.5 {
			gdpEmoji = "📈"
		} else if d.GDPGrowth < 0 {
			gdpEmoji = "📉"
		}
		sb.WriteString("🏭 <b>GDP Growth (QoQ, SA)</b>\n")
		sb.WriteString(fmt.Sprintf("   Growth: <b>%+.1f%%</b> %s", d.GDPGrowth, gdpEmoji))
		if !d.GDPDate.IsZero() {
			sb.WriteString(fmt.Sprintf(" (%s)", formatQuarter(d.GDPDate)))
		}
		sb.WriteString("\n")
		if d.GDPGrowth < 0 {
			sb.WriteString("   ⚠️ <i>Contraction — recession risk for EUR</i>\n")
		} else if d.GDPGrowth < 0.2 {
			sb.WriteString("   🟡 <i>Sluggish growth — EUR neutral to weak</i>\n")
		} else {
			sb.WriteString("   🟢 <i>Positive growth — supports EUR</i>\n")
		}
		sb.WriteString("\n")
	}

	// EUR Trading Context
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("💱 <b>EUR Trading Context</b>\n")
	writeEURContext(&sb, d)

	sb.WriteString("\n<i>📝 Source: Eurostat (ec.europa.eu/eurostat)</i>")
	return sb.String()
}

// writeEURContext adds EUR-specific trading insights.
func writeEURContext(sb *strings.Builder, d *EurostatData) {
	// Overall assessment
	hawkish := 0
	dovish := 0

	if d.HICPHeadline > 2.3 {
		hawkish++
	} else if d.HICPHeadline < 1.8 && d.HICPHeadline != 0 {
		dovish++
	}

	if d.UnempRate < 6.5 && d.UnempRate != 0 {
		hawkish++ // tight labor → hawkish
	} else if d.UnempRate > 7.5 {
		dovish++ // weak labor → dovish
	}

	if d.GDPGrowth > 0.3 {
		hawkish++
	} else if d.GDPGrowth < 0 {
		dovish++
	}

	switch {
	case hawkish >= 2 && dovish == 0:
		sb.WriteString("   🦅 <b>Hawkish lean</b> — data supports ECB rate hold/hike → EUR bullish\n")
	case dovish >= 2 && hawkish == 0:
		sb.WriteString("   🕊️ <b>Dovish lean</b> — data supports ECB rate cut path → EUR bearish\n")
	case hawkish > dovish:
		sb.WriteString("   📊 <b>Mildly hawkish</b> — EUR slightly positive bias\n")
	case dovish > hawkish:
		sb.WriteString("   📊 <b>Mildly dovish</b> — EUR slightly negative bias\n")
	default:
		sb.WriteString("   📊 <b>Mixed signals</b> — EUR neutral, watch for momentum shift\n")
	}

	// US vs EU differential context
	if d.USCPIHeadline > 0 && d.HICPHeadline > 0 {
		diff := d.HICPHeadline - d.USCPIHeadline
		if diff > 0.5 {
			sb.WriteString(fmt.Sprintf("   🔄 EU-US inflation gap: <b>%+.1f%%</b> (EU higher → ECB relatively hawkish)\n", diff))
		} else if diff < -0.5 {
			sb.WriteString(fmt.Sprintf("   🔄 EU-US inflation gap: <b>%+.1f%%</b> (US higher → Fed relatively hawkish → USD)\n", diff))
		}
	}
}

// inflationArrow returns an emoji for inflation level relative to ECB 2% target.
func inflationArrow(v float64) string {
	switch {
	case v > 3.0:
		return "🔴" // well above target
	case v > 2.3:
		return "🟡" // above target
	case v >= 1.5:
		return "🟢" // near target
	default:
		return "📉" // below target
	}
}

// formatQuarter formats a date as "Q1 2025".
func formatQuarter(t time.Time) string {
	q := (t.Month()-1)/3 + 1
	return fmt.Sprintf("Q%d %d", q, t.Year())
}
