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

	"github.com/arkcode369/ark-intelligent/pkg/errs"
)

// ---------------------------------------------------------------------------
// SNB (Swiss National Bank) Balance Sheet / FX Intervention Proxy
// ---------------------------------------------------------------------------
//
// Data source: SNB Data API — free, no API key required.
// Cube: snbbipo — Full SNB balance sheet (monthly, CHF millions).
//
// Key metrics tracked:
//   D   = Foreign currency investments (FX reserves) — intervention proxy
//   GFG = Gold holdings
//   GB  = Sight deposits of domestic banks
//   N   = Banknotes in circulation
//   T0  = Total assets
//
// A large month-over-month change in "D" (>5B CHF) is a strong indicator
// of SNB FX market intervention.

const (
	snbHTTPTimeout = 20 * time.Second
	snbCacheTTL    = 24 * time.Hour

	snbAPIBase = "https://data.snb.ch/api/cube/snbbipo/data/csv/en"

	// Alert threshold: CHF millions. > 5000 = likely FX intervention.
	snbInterventionThresholdMillion = 5_000.0

	// Lookback: fetch 24 months to build MoM history.
	snbLookbackMonths = 24
)

// snbDimensionID maps SNB cube dimension codes to human-readable names.
var snbDimensionID = map[string]string{
	"D":   "Foreign Currency Investments",
	"GFG": "Gold Holdings",
	"GB":  "Sight Deposits (Domestic Banks)",
	"N":   "Banknotes in Circulation",
	"T0":  "Total Assets",
}

// SNBData holds the latest SNB balance sheet snapshot.
type SNBData struct {
	FetchedAt     time.Time
	PublishedAt   time.Time // when SNB published this data
	LatestDate    time.Time // most recent monthly observation
	PreviousDate  time.Time // prior month

	// Values in CHF millions (latest month)
	FXReserves      float64 // "D" — foreign currency investments
	GoldHoldings    float64 // "GFG"
	SightDeposits   float64 // "GB" — domestic bank deposits (SNB policy tool)
	Banknotes       float64 // "N"
	TotalAssets     float64 // "T0"

	// Month-over-month changes (CHF millions)
	FXReservesMoM    float64
	GoldHoldingsMoM  float64
	SightDepositsMoM float64
	TotalAssetsMoM   float64

	// Derived signals
	IsLikelyIntervention bool   // |FXReservesMoM| > threshold
	InterventionDir      string // "BUYING_CHF" | "SELLING_CHF" | "NONE"
	AlertMessage         string // human-readable alert if intervention detected
}

// IsZero returns true if no SNB data has been fetched.
func (d *SNBData) IsZero() bool {
	return d == nil || d.FetchedAt.IsZero()
}

// SNBClient fetches data from the SNB Data API.
// No API key required — free public data.
type SNBClient struct {
	mu       sync.Mutex
	cached   *SNBData
	cachedAt time.Time
	hc       *http.Client
}

// NewSNBClient creates a new SNBClient.
func NewSNBClient() *SNBClient {
	return &SNBClient{
		hc: &http.Client{Timeout: snbHTTPTimeout},
	}
}

// GetData returns the latest SNB balance sheet data, using cache if fresh.
func (c *SNBClient) GetData(ctx context.Context) (*SNBData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.cachedAt) < snbCacheTTL {
		return c.cached, nil
	}

	data, err := c.fetchAll(ctx)
	if err != nil {
		if c.cached != nil {
			return c.cached, nil
		}
		return nil, err
	}

	c.cached = data
	c.cachedAt = time.Now()
	return data, nil
}

// fetchAll fetches SNB balance sheet data for the past snbLookbackMonths months.
func (c *SNBClient) fetchAll(ctx context.Context) (*SNBData, error) {
	now := time.Now().UTC()
	fromDate := now.AddDate(0, -snbLookbackMonths, 0).Format("2006-01")
	toDate := now.Format("2006-01")

	url := fmt.Sprintf("%s?fromDate=%s&toDate=%s", snbAPIBase, fromDate, toDate)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errs.Wrapf(errs.ErrUpstream, "SNB build request: %v", err)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, errs.Wrapf(errs.ErrUpstream, "SNB HTTP: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errs.Wrapf(errs.ErrUpstream, "SNB HTTP status %d", resp.StatusCode)
	}

	return parseSNBCSV(resp.Body)
}

// ---------------------------------------------------------------------------
// CSV parser
// ---------------------------------------------------------------------------

// snbRecord is a single row from the SNB CSV response.
type snbRecord struct {
	date      time.Time
	dimension string
	value     float64
}

// parseSNBCSV parses the SNB semicolon-delimited CSV response.
//
// Format:
//
//	"CubeId";"snbbipo"
//	"PublishingDate";"2026-03-31 09:00"
//	(blank line)
//	"Date";"D0";"Value"
//	"2024-01";"D";"683843.03"
//	...
func parseSNBCSV(r io.Reader) (*SNBData, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, errs.Wrapf(errs.ErrUpstream, "SNB read body: %v", err)
	}

	lines := strings.Split(string(body), "\n")

	var publishedAt time.Time
	var records []snbRecord

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		// Remove surrounding quotes from each field.
		// SNB uses semicolons + quoted fields.
		fields := splitSNBLine(line)
		if len(fields) < 2 {
			continue
		}

		// Parse PublishingDate metadata line.
		if fields[0] == "PublishingDate" && publishedAt.IsZero() {
			if t, parseErr := time.Parse("2006-01-02 15:04", fields[1]); parseErr == nil {
				publishedAt = t
			}
			continue
		}

		// Skip header row and metadata rows.
		if fields[0] == "Date" || fields[0] == "CubeId" || fields[0] == "PublishingDate" {
			continue
		}

		if len(fields) < 3 {
			continue
		}

		// Only process dimensions we care about.
		dim := fields[1]
		if _, ok := snbDimensionID[dim]; !ok {
			continue
		}

		rawVal := strings.TrimSpace(fields[2])
		if rawVal == "" || rawVal == "NaN" || rawVal == "-" {
			continue
		}

		val, parseErr := strconv.ParseFloat(rawVal, 64)
		if parseErr != nil {
			continue
		}

		date := parseSNBDate(fields[0])
		if date.IsZero() {
			continue
		}

		records = append(records, snbRecord{
			date:      date,
			dimension: dim,
			value:     val,
		})
	}

	if len(records) == 0 {
		return nil, errs.Wrap(errs.ErrNoData, "SNB CSV: no valid observations")
	}

	return buildSNBData(records, publishedAt)
}

// splitSNBLine splits a semicolon-delimited SNB line and strips surrounding quotes.
func splitSNBLine(line string) []string {
	reader := csv.NewReader(strings.NewReader(line))
	reader.Comma = ';'
	reader.LazyQuotes = true
	record, err := reader.Read()
	if err != nil {
		// Fallback: manual split
		parts := strings.Split(line, ";")
		for i, p := range parts {
			parts[i] = strings.Trim(strings.TrimSpace(p), `"`)
		}
		return parts
	}
	return record
}

// parseSNBDate parses SNB date strings: "YYYY-MM".
func parseSNBDate(s string) time.Time {
	s = strings.Trim(s, `"`)
	if t, err := time.Parse("2006-01", s); err == nil {
		return t
	}
	return time.Time{}
}

// buildSNBData aggregates raw records into an SNBData struct, computing MoM changes.
func buildSNBData(records []snbRecord, publishedAt time.Time) (*SNBData, error) {
	// Group by dimension → sorted list of (date, value).
	type obs struct {
		date  time.Time
		value float64
	}
	byDim := make(map[string][]obs)
	for _, r := range records {
		byDim[r.dimension] = append(byDim[r.dimension], obs{r.date, r.value})
	}

	// Sort each dimension's observations by date (ascending).
	for dim := range byDim {
		sort.Slice(byDim[dim], func(i, j int) bool {
			return byDim[dim][i].date.Before(byDim[dim][j].date)
		})
	}

	// Helper: get latest + previous value for a dimension.
	latestVal := func(dim string) (latest, previous float64, latestDate, prevDate time.Time) {
		obs := byDim[dim]
		if len(obs) == 0 {
			return
		}
		latest = obs[len(obs)-1].value
		latestDate = obs[len(obs)-1].date
		if len(obs) >= 2 {
			previous = obs[len(obs)-2].value
			prevDate = obs[len(obs)-2].date
		}
		return
	}

	fxLatest, fxPrev, latestDate, prevDate := latestVal("D")
	goldLatest, goldPrev, _, _ := latestVal("GFG")
	gbLatest, gbPrev, _, _ := latestVal("GB")
	bkLatest, _, _, _ := latestVal("N")
	t0Latest, t0Prev, _, _ := latestVal("T0")

	if latestDate.IsZero() {
		return nil, errs.Wrap(errs.ErrNoData, "SNB: no date in parsed data")
	}

	fxMoM := fxLatest - fxPrev
	goldMoM := goldLatest - goldPrev
	gbMoM := gbLatest - gbPrev
	t0MoM := t0Latest - t0Prev

	// Determine intervention signal.
	interventionDir := "NONE"
	isIntervention := false
	var alertMsg string

	absMoM := fxMoM
	if absMoM < 0 {
		absMoM = -absMoM
	}
	if absMoM > snbInterventionThresholdMillion {
		isIntervention = true
		if fxMoM < 0 {
			// FX reserves decreased → SNB sold foreign currency, bought CHF → CHF strengthening
			interventionDir = "BUYING_CHF"
			alertMsg = fmt.Sprintf("⚠️ SNB kemungkinan intervensi: FX reserves turun %.1fB CHF → SNB membeli CHF (menguatkan CHF)",
				-fxMoM/1000)
		} else {
			// FX reserves increased → SNB bought foreign currency, sold CHF → CHF weakening
			interventionDir = "SELLING_CHF"
			alertMsg = fmt.Sprintf("⚠️ SNB kemungkinan intervensi: FX reserves naik +%.1fB CHF → SNB menjual CHF (melemahkan CHF)",
				fxMoM/1000)
		}
	}

	return &SNBData{
		FetchedAt:           time.Now(),
		PublishedAt:         publishedAt,
		LatestDate:          latestDate,
		PreviousDate:        prevDate,
		FXReserves:          fxLatest,
		GoldHoldings:        goldLatest,
		SightDeposits:       gbLatest,
		Banknotes:           bkLatest,
		TotalAssets:         t0Latest,
		FXReservesMoM:       fxMoM,
		GoldHoldingsMoM:     goldMoM,
		SightDepositsMoM:    gbMoM,
		TotalAssetsMoM:      t0MoM,
		IsLikelyIntervention: isIntervention,
		InterventionDir:     interventionDir,
		AlertMessage:        alertMsg,
	}, nil
}

// ---------------------------------------------------------------------------
// Package-level singleton
// ---------------------------------------------------------------------------

var defaultSNBClient = NewSNBClient()

// GetSNBData returns the latest SNB balance sheet data using the package-level client.
func GetSNBData(ctx context.Context) (*SNBData, error) {
	return defaultSNBClient.GetData(ctx)
}

// FormatSNBData formats SNB balance sheet data for Telegram HTML display.
func FormatSNBData(d *SNBData) string {
	if d == nil || d.IsZero() {
		return "❌ SNB data tidak tersedia."
	}

	var sb strings.Builder

	sb.WriteString("🏦 <b>SNB Balance Sheet — FX Intervention Monitor</b>\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Period
	sb.WriteString(fmt.Sprintf("📅 <b>Periode:</b> %s", d.LatestDate.Format("Jan 2006")))
	if !d.PreviousDate.IsZero() {
		sb.WriteString(fmt.Sprintf(" vs %s", d.PreviousDate.Format("Jan 2006")))
	}
	sb.WriteString("\n\n")

	// Intervention alert (top of output if triggered)
	if d.IsLikelyIntervention {
		sb.WriteString(d.AlertMessage + "\n\n")
	}

	// FX Reserves — the key metric
	fxBillions := d.FXReserves / 1000
	fxMoMBillions := d.FXReservesMoM / 1000
	momArrow := snbMoMArrow(d.FXReservesMoM)
	sb.WriteString("💱 <b>Foreign Currency Investments (FX Reserves)</b>\n")
	sb.WriteString(fmt.Sprintf("   Latest: <b>CHF %.1fB</b>\n", fxBillions))
	if d.FXReservesMoM != 0 {
		sb.WriteString(fmt.Sprintf("   MoM: <b>%+.1fB CHF</b> %s\n", fxMoMBillions, momArrow))
	}
	sb.WriteString("\n")

	// Gold Holdings
	goldBillions := d.GoldHoldings / 1000
	goldMoMBillions := d.GoldHoldingsMoM / 1000
	sb.WriteString("🥇 <b>Gold Holdings</b>\n")
	sb.WriteString(fmt.Sprintf("   Latest: <b>CHF %.1fB</b>", goldBillions))
	if d.GoldHoldingsMoM != 0 {
		sb.WriteString(fmt.Sprintf("  MoM: <b>%+.1fB</b> %s", goldMoMBillions, snbMoMArrow(d.GoldHoldingsMoM)))
	}
	sb.WriteString("\n\n")

	// Sight Deposits — domestic banks (policy tool)
	gbBillions := d.SightDeposits / 1000
	gbMoMBillions := d.SightDepositsMoM / 1000
	sb.WriteString("🏛️ <b>Sight Deposits (Domestic Banks)</b>\n")
	sb.WriteString(fmt.Sprintf("   Latest: <b>CHF %.1fB</b>", gbBillions))
	if d.SightDepositsMoM != 0 {
		sb.WriteString(fmt.Sprintf("  MoM: <b>%+.1fB</b> %s", gbMoMBillions, snbMoMArrow(d.SightDepositsMoM)))
	}
	sb.WriteString("\n\n")

	// Total Assets
	t0Billions := d.TotalAssets / 1000
	t0MoMBillions := d.TotalAssetsMoM / 1000
	sb.WriteString("📊 <b>Total Assets</b>\n")
	sb.WriteString(fmt.Sprintf("   Latest: <b>CHF %.1fB</b>", t0Billions))
	if d.TotalAssetsMoM != 0 {
		sb.WriteString(fmt.Sprintf("  MoM: <b>%+.1fB</b> %s", t0MoMBillions, snbMoMArrow(d.TotalAssetsMoM)))
	}
	sb.WriteString("\n\n")

	// CHF interpretation
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("💡 <b>Interpretasi FX Intervention:</b>\n")
	switch d.InterventionDir {
	case "BUYING_CHF":
		sb.WriteString("   🔴 SNB <b>membeli CHF</b> — menjual FX reserves\n")
		sb.WriteString("   → Sinyal: SNB ingin <b>menguatkan CHF</b>\n")
	case "SELLING_CHF":
		sb.WriteString("   🟢 SNB <b>menjual CHF</b> — menambah FX reserves\n")
		sb.WriteString("   → Sinyal: SNB ingin <b>melemahkan CHF</b>\n")
	default:
		sb.WriteString("   ✅ Tidak ada sinyal intervensi signifikan bulan ini\n")
		sb.WriteString("   → CHF movement kemungkinan bukan karena SNB\n")
	}

	sb.WriteString("\n")
	sb.WriteString("📝 <i>Sumber: SNB Data API (data.snb.ch) — Cube snbbipo</i>\n")
	if !d.PublishedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("<i>SNB published: %s</i>", d.PublishedAt.Format("2 Jan 2006 15:04")))
	}

	return sb.String()
}

// snbMoMArrow returns an emoji arrow based on the direction of change.
func snbMoMArrow(delta float64) string {
	if delta > 0 {
		return "⬆️"
	}
	if delta < 0 {
		return "⬇️"
	}
	return "→"
}
