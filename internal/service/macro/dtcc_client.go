package macro

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/errs"
)

// ---------------------------------------------------------------------------
// DTCC Public Price Dissemination (PPD) — FX Swap Data
// ---------------------------------------------------------------------------
//
// DTCC PPD provides institutional post-trade FX derivative records (CFTC-reporting).
// Cumulative daily data: FX forwards, swaps, options — notional amounts per currency pair.
// Volume patterns signal institutional hedging flows and positioning shifts.
//
// API: https://pddata.dtcc.com/ppd/api/report/cumulative/CFTC/FOREX?asof=YYYY-MM-DD
// Free public data — no API key required.
// ---------------------------------------------------------------------------

const (
	dtccHTTPTimeout   = 30 * time.Second
	dtccCacheTTL      = 12 * time.Hour
	dtccAPIBase       = "https://pddata.dtcc.com/ppd/api/report/cumulative/CFTC/FOREX"
	dtccHistoryDays   = 10
	dtccAnomalyStdDev = 2.0
)

// dtccFXPairs are the major/minor pairs we aggregate and display.
var dtccFXPairs = []string{
	"EURUSD", "USDJPY", "GBPUSD", "AUDUSD", "USDCAD",
	"USDCHF", "NZDUSD", "USDMXN", "USDBRL", "USDHKD",
}

// DTCCRecord is a single DTCC PPD trade record from the JSON response.
type DTCCRecord struct {
	NotionalCurrency1 string `json:"NOTIONAL_CURRENCY_1"`
	NotionalCurrency2 string `json:"NOTIONAL_CURRENCY_2"`
	RoundedNotional1  string `json:"ROUNDED_NOTIONAL_AMOUNT_1"`
	RoundedNotional2  string `json:"ROUNDED_NOTIONAL_AMOUNT_2"`
	ContractType      string `json:"CONTRACT_TYPE"`
	SubAssetClass     string `json:"SUB_ASSET_CLASS"`
}

// DTCCPairData holds aggregated data for a single currency pair.
type DTCCPairData struct {
	Pair             string
	TotalNotional    float64 // total notional in millions USD-equivalent
	TradeCount       int
	Date             time.Time
	IsAnomaly        bool
	AnomalyFactor    float64 // std devs above mean
	HistoricalMean   float64 // millions
	HistoricalStdDev float64 // millions
}

// DTCCData holds the full DTCC FX swap dataset.
type DTCCData struct {
	FetchedAt   time.Time
	AsOf        time.Time
	Pairs       map[string]*DTCCPairData
	TotalVolume float64 // millions, all pairs combined
	History     map[string][]float64
}

// IsZero returns true if no DTCC data has been fetched.
func (d *DTCCData) IsZero() bool {
	return d == nil || d.FetchedAt.IsZero()
}

// DTCCClient fetches and aggregates DTCC PPD FX swap data.
type DTCCClient struct {
	mu       sync.Mutex
	cached   *DTCCData
	cachedAt time.Time
	hc       *http.Client
}

// NewDTCCClient creates a new DTCCClient.
func NewDTCCClient() *DTCCClient {
	return &DTCCClient{
		hc: &http.Client{Timeout: dtccHTTPTimeout},
	}
}

// GetData returns the latest DTCC FX swap data, using cache if fresh.
func (c *DTCCClient) GetData(ctx context.Context) (*DTCCData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.cachedAt) < dtccCacheTTL {
		return c.cached, nil
	}

	data, err := c.fetchAll(ctx)
	if err != nil {
		if c.cached != nil {
			log.Warn().Err(err).Msg("DTCC fetch failed, returning stale cache")
			return c.cached, nil
		}
		return nil, err
	}

	c.cached = data
	c.cachedAt = time.Now()
	return data, nil
}

// fetchAll fetches today's data plus historical days for anomaly detection.
func (c *DTCCClient) fetchAll(ctx context.Context) (*DTCCData, error) {
	asOf := dtccLatestBusinessDay(time.Now().UTC())

	records, err := c.fetchRecords(ctx, asOf)
	if err != nil {
		return nil, errs.Wrapf(errs.ErrUpstream, "DTCC fetch for %s: %v", asOf.Format("2006-01-02"), err)
	}

	todayByPair := aggregateDTCCByPair(records)

	// Fetch history for anomaly detection — best-effort (ignore per-day errors).
	history := make(map[string][]float64)
	for i := 1; i <= dtccHistoryDays; i++ {
		day := dtccLatestBusinessDay(asOf.AddDate(0, 0, -i))
		hrecs, herr := c.fetchRecords(ctx, day)
		if herr != nil {
			continue
		}
		for pair, notional := range aggregateDTCCByPair(hrecs) {
			history[pair] = append(history[pair], notional)
		}
	}

	pairs := make(map[string]*DTCCPairData)
	var totalVol float64
	for _, pair := range dtccFXPairs {
		notional := todayByPair[pair]
		totalVol += notional

		pd := &DTCCPairData{
			Pair:          pair,
			TotalNotional: notional,
			Date:          asOf,
		}

		for _, r := range records {
			if normalizeDTCCPair(r.NotionalCurrency1, r.NotionalCurrency2) == pair {
				pd.TradeCount++
			}
		}

		if hist := history[pair]; len(hist) >= 3 {
			mean, stddev := dtccMeanStdDev(hist)
			pd.HistoricalMean = mean
			pd.HistoricalStdDev = stddev
			if stddev > 0 && notional > 0 {
				factor := (notional - mean) / stddev
				if factor >= dtccAnomalyStdDev {
					pd.IsAnomaly = true
					pd.AnomalyFactor = factor
				}
			}
		}

		pairs[pair] = pd
	}

	return &DTCCData{
		FetchedAt:   time.Now(),
		AsOf:        asOf,
		Pairs:       pairs,
		TotalVolume: totalVol,
		History:     history,
	}, nil
}

// fetchRecords fetches raw DTCC trade records for a specific date, handling pagination.
func (c *DTCCClient) fetchRecords(ctx context.Context, asOf time.Time) ([]DTCCRecord, error) {
	dateStr := asOf.Format("2006-01-02")
	var all []DTCCRecord

	for page := 0; page < 20; page++ {
		url := fmt.Sprintf("%s?asof=%s", dtccAPIBase, dateStr)
		if page > 0 {
			url = fmt.Sprintf("%s&page=%d", url, page)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http get page %d: %w", page, err)
		}

		body, readErr := dtccReadBody(resp)
		if readErr != nil {
			return nil, readErr
		}

		if resp.StatusCode == http.StatusNotFound {
			return nil, errs.Wrap(errs.ErrNoData, "no DTCC data for date "+dateStr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, errs.Wrapf(errs.ErrUpstream, "DTCC API status %d for %s", resp.StatusCode, dateStr)
		}

		isLast, pageRecs, parseErr := parseDTCCResponse(body)
		if parseErr != nil {
			return nil, parseErr
		}

		all = append(all, pageRecs...)
		if isLast || len(pageRecs) == 0 {
			break
		}
	}

	return all, nil
}

// parseDTCCResponse parses DTCC JSON — bare array or paged wrapper.
// Returns (isLastPage, records, error).
func parseDTCCResponse(body []byte) (bool, []DTCCRecord, error) {
	// Try bare array.
	var records []DTCCRecord
	if err := json.Unmarshal(body, &records); err == nil {
		return true, records, nil
	}

	// Try paged wrapper: {"data": [...], "totalRecords": N, "page": N, "pageSize": N}
	var wrapper struct {
		Data         []DTCCRecord `json:"data"`
		TotalRecords int          `json:"totalRecords"`
		Page         int          `json:"page"`
		PageSize     int          `json:"pageSize"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Data != nil {
		isLast := len(wrapper.Data) == 0 ||
			(wrapper.PageSize > 0 && (wrapper.Page+1)*wrapper.PageSize >= wrapper.TotalRecords)
		return isLast, wrapper.Data, nil
	}

	// Try {"records": [...]}
	var altWrapper struct {
		Records []DTCCRecord `json:"records"`
	}
	if err := json.Unmarshal(body, &altWrapper); err == nil {
		return true, altWrapper.Records, nil
	}

	return true, nil, errs.Wrapf(errs.ErrInvalidFormat,
		"DTCC: unrecognized response format (len=%d)", len(body))
}

// dtccReadBody reads an HTTP response body and closes it.
func dtccReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	var buf []byte
	tmp := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

// aggregateDTCCByPair sums notional amounts (in millions) per normalized currency pair.
func aggregateDTCCByPair(records []DTCCRecord) map[string]float64 {
	result := make(map[string]float64)
	for _, r := range records {
		pair := normalizeDTCCPair(r.NotionalCurrency1, r.NotionalCurrency2)
		if pair == "" {
			continue
		}
		n1 := parseDTCCNotional(r.RoundedNotional1)
		n2 := parseDTCCNotional(r.RoundedNotional2)
		notional := n1
		if n2 > n1 {
			notional = n2
		}
		result[pair] += notional
	}
	return result
}

// normalizeDTCCPair returns the canonical pair name from two currency codes.
func normalizeDTCCPair(ccy1, ccy2 string) string {
	if ccy1 == "" || ccy2 == "" {
		return ""
	}
	c1 := strings.ToUpper(strings.TrimSpace(ccy1))
	c2 := strings.ToUpper(strings.TrimSpace(ccy2))
	for _, p := range dtccFXPairs {
		if p == c1+c2 || p == c2+c1 {
			return p
		}
	}
	return ""
}

// parseDTCCNotional parses a DTCC notional amount string to float64 in millions.
// DTCC rounds values and may use "+" suffix for values above a cap.
func parseDTCCNotional(s string) float64 {
	if s == "" {
		return 0
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "+")
	s = strings.ReplaceAll(s, ",", "")
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v / 1_000_000
}

// dtccMeanStdDev computes mean and population std dev of a float64 slice.
func dtccMeanStdDev(vals []float64) (mean, stddev float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean = sum / float64(len(vals))
	variance := 0.0
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	stddev = math.Sqrt(variance / float64(len(vals)))
	return mean, stddev
}

// dtccLatestBusinessDay walks backward from t to find the last Mon-Fri.
func dtccLatestBusinessDay(t time.Time) time.Time {
	for {
		wd := t.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			return t
		}
		t = t.AddDate(0, 0, -1)
	}
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

// FormatDTCCData formats DTCC FX swap data for Telegram HTML display.
func FormatDTCCData(d *DTCCData) string {
	if d == nil || d.IsZero() {
		return "❌ DTCC FX swap data tidak tersedia."
	}

	var sb strings.Builder
	sb.WriteString("🏛 <b>DTCC FX Swap Institutional Flows</b>\n")
	sb.WriteString(fmt.Sprintf("📅 As of: <b>%s</b>\n", d.AsOf.Format("Mon, 02 Jan 2006")))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	type pairEntry struct {
		pair string
		data *DTCCPairData
	}
	var entries []pairEntry
	for _, pair := range dtccFXPairs {
		if pd, ok := d.Pairs[pair]; ok && pd.TotalNotional > 0 {
			entries = append(entries, pairEntry{pair, pd})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].data.TotalNotional > entries[j].data.TotalNotional
	})

	var anomalies []pairEntry
	for _, e := range entries {
		pd := e.data
		anomalyTag := ""
		if pd.IsAnomaly {
			anomalyTag = fmt.Sprintf(" 🚨 +%.1fσ", pd.AnomalyFactor)
			anomalies = append(anomalies, e)
		}

		sb.WriteString(fmt.Sprintf("💱 <b>%s</b>%s\n", pd.Pair, anomalyTag))
		sb.WriteString(fmt.Sprintf("   Volume: <b>$%.0fM</b>", pd.TotalNotional))
		if pd.TradeCount > 0 {
			sb.WriteString(fmt.Sprintf(" (%d trades)", pd.TradeCount))
		}
		if pd.HistoricalMean > 0 {
			pctDiff := ((pd.TotalNotional - pd.HistoricalMean) / pd.HistoricalMean) * 100
			arrow := "→"
			if pctDiff > 10 {
				arrow = "📈"
			} else if pctDiff < -10 {
				arrow = "📉"
			}
			sb.WriteString(fmt.Sprintf(" %s avg $%.0fM", arrow, pd.HistoricalMean))
		}
		sb.WriteString("\n\n")
	}

	if len(entries) == 0 {
		sb.WriteString("<i>Tidak ada data FX swap tersedia untuk tanggal ini.</i>\n\n")
	}

	if d.TotalVolume > 0 {
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString(fmt.Sprintf("📊 <b>Total Volume: $%.0fM</b>\n\n", d.TotalVolume))
	}

	if len(anomalies) > 0 {
		sb.WriteString("⚠️ <b>Volume Anomalies (&gt;2σ):</b>\n")
		for _, e := range anomalies {
			pd := e.data
			mult := pd.TotalNotional / math.Max(pd.HistoricalMean, 1)
			sb.WriteString(fmt.Sprintf("  • %s: $%.0fM (%.1fx avg, +%.1fσ)\n",
				pd.Pair, pd.TotalNotional, mult, pd.AnomalyFactor))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📝 <i>Sumber: DTCC Public Price Dissemination (CFTC/FOREX)</i>\n")
	sb.WriteString("<i>Post-trade swap records — institutional hedging &amp; positioning flows</i>")

	return sb.String()
}

// ---------------------------------------------------------------------------
// Package-level singleton
// ---------------------------------------------------------------------------

var defaultDTCCClient = NewDTCCClient()

// GetDTCCData returns the latest DTCC FX swap data using the package-level client.
func GetDTCCData(ctx context.Context) (*DTCCData, error) {
	return defaultDTCCClient.GetData(ctx)
}
