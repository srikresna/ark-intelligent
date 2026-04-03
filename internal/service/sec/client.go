package sec

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("sec") //nolint:gochecknoglobals

const (
	cacheTTL    = 7 * 24 * time.Hour // quarterly data — cache 7 days
	httpTimeout = 25 * time.Second
	userAgent   = "Ark-Intelligent support@ark-intelligent.ai"
	ratePause   = 120 * time.Millisecond // ~8 req/s (under 10 limit)

	submissionsBase = "https://data.sec.gov/submissions/CIK"
	archivesBase    = "https://www.sec.gov/Archives/edgar/data"
)

// targetInstitutions is the list of major institutions we track.
var targetInstitutions = []Institution{ //nolint:gochecknoglobals
	{Name: "Berkshire Hathaway", CIK: "0001067983"},
	{Name: "Bridgewater Associates", CIK: "0001350694"},
	{Name: "Renaissance Technologies", CIK: "0001037389"},
	{Name: "Citadel Advisors", CIK: "0001423053"},
	{Name: "Soros Fund Management", CIK: "0001061768"},
}

// package-level cache.
var (
	globalCache *EdgarData  //nolint:gochecknoglobals
	cacheMu     sync.RWMutex
	httpClient  = httpclient.New(httpclient.WithTimeout(httpTimeout)) //nolint:gochecknoglobals
)

// GetCachedOrFetch returns cached SEC 13F data if within TTL, otherwise fetches
// fresh data from EDGAR.
func GetCachedOrFetch(ctx context.Context) (*EdgarData, error) {
	cacheMu.RLock()
	if globalCache != nil && time.Since(globalCache.FetchedAt) < cacheTTL {
		data := globalCache
		cacheMu.RUnlock()
		return data, nil
	}
	cacheMu.RUnlock()

	data, err := FetchAll(ctx)
	if err != nil {
		cacheMu.RLock()
		stale := globalCache
		cacheMu.RUnlock()
		if stale != nil {
			log.Warn().Err(err).Msg("SEC EDGAR fetch failed; using stale cache")
			return stale, nil
		}
		return nil, fmt.Errorf("sec edgar fetch failed: %w", err)
	}

	cacheMu.Lock()
	globalCache = data
	cacheMu.Unlock()

	return data, nil
}

// CacheAge returns seconds since last fetch, or -1 if no cache.
func CacheAge() float64 {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if globalCache == nil {
		return -1
	}
	return time.Since(globalCache.FetchedAt).Seconds()
}

// FetchAll fetches 13F data for all target institutions.
func FetchAll(ctx context.Context) (*EdgarData, error) {
	var reports []InstitutionReport
	var fetchErr error

	for _, inst := range targetInstitutions {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		report, err := fetchInstitution(ctx, inst)
		if err != nil {
			log.Warn().Err(err).Str("institution", inst.Name).Msg("Failed to fetch 13F")
			fetchErr = err
			continue
		}
		if report != nil {
			reports = append(reports, *report)
		}

		// Rate limit pause.
		time.Sleep(ratePause)
	}

	if len(reports) == 0 && fetchErr != nil {
		return nil, fetchErr
	}

	return &EdgarData{
		Reports:   reports,
		FetchedAt: time.Now(),
	}, nil
}

// submissionsResponse is the JSON structure from the submissions endpoint.
type submissionsResponse struct {
	CIK           string         `json:"cik"`
	Name          string         `json:"name"`
	RecentFilings recentFilings  `json:"filings"`
}

type recentFilings struct {
	Recent filingList `json:"recent"`
}

type filingList struct {
	AccessionNumber []string `json:"accessionNumber"`
	FilingDate      []string `json:"filingDate"`
	ReportDate      []string `json:"reportDate"`
	Form            []string `json:"form"`
	PrimaryDocument []string `json:"primaryDocument"`
}

// fetchInstitution fetches the latest 13F filings for one institution.
func fetchInstitution(ctx context.Context, inst Institution) (*InstitutionReport, error) {
	// Step 1: Get submissions index.
	url := fmt.Sprintf("%s%s.json", submissionsBase, inst.CIK)

	body, err := doGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("submissions fetch for %s: %w", inst.Name, err)
	}

	var sub submissionsResponse
	if err := json.Unmarshal(body, &sub); err != nil {
		return nil, fmt.Errorf("submissions parse for %s: %w", inst.Name, err)
	}

	// Step 2: Find latest two 13F-HR filings.
	var filingIndices []int
	for i, form := range sub.RecentFilings.Recent.Form {
		if form == "13F-HR" && len(filingIndices) < 2 {
			filingIndices = append(filingIndices, i)
		}
	}

	if len(filingIndices) == 0 {
		log.Info().Str("institution", inst.Name).Msg("No 13F-HR filings found")
		return nil, nil
	}

	report := &InstitutionReport{Institution: inst}

	for idx, fi := range filingIndices {
		accession := sub.RecentFilings.Recent.AccessionNumber[fi]
		filingDateStr := sub.RecentFilings.Recent.FilingDate[fi]
		reportDateStr := sub.RecentFilings.Recent.ReportDate[fi]

		filingDate, _ := time.Parse("2006-01-02", filingDateStr)
		reportDate, _ := time.Parse("2006-01-02", reportDateStr)

		time.Sleep(ratePause)

		holdings, err := fetch13FHoldings(ctx, inst.CIK, accession)
		if err != nil {
			log.Warn().Err(err).Str("accession", accession).Msg("Failed to fetch 13F holdings")
			continue
		}

		var totalVal float64
		for _, h := range holdings {
			totalVal += h.Value
		}

		filing := &Filing{
			AccessionNumber: accession,
			FilingDate:      filingDate,
			ReportDate:      reportDate,
			Holdings:        holdings,
			TotalValue:      totalVal,
		}

		if idx == 0 {
			report.LatestFiling = filing
		} else {
			report.PreviousFiling = filing
		}
	}

	// Step 3: Compute top holdings and changes.
	if report.LatestFiling != nil {
		report.TopHoldings = topHoldings(report.LatestFiling.Holdings, 15)
	}

	if report.LatestFiling != nil && report.PreviousFiling != nil {
		report.Changes, report.NewPositions, report.Exits = computeChanges(
			report.LatestFiling.Holdings, report.PreviousFiling.Holdings)
	}

	return report, nil
}

// fetch13FHoldings fetches and parses the 13F holdings XML for a filing.
func fetch13FHoldings(ctx context.Context, cik, accession string) ([]Holding, error) {
	// Normalize accession: "0001067983-26-000015" → "000106798326000015"
	accessionClean := strings.ReplaceAll(accession, "-", "")

	// The 13F XML file is typically named infotable.xml inside the filing directory.
	// Try common naming patterns.
	cikClean := strings.TrimLeft(cik, "0")
	patterns := []string{
		fmt.Sprintf("%s/%s/%s/infotable.xml", archivesBase, cikClean, accessionClean),
		fmt.Sprintf("%s/%s/%s/InfoTable.xml", archivesBase, cikClean, accessionClean),
	}

	var body []byte
	var err error
	for _, url := range patterns {
		body, err = doGet(ctx, url)
		if err == nil && len(body) > 0 {
			break
		}
		time.Sleep(ratePause)
	}
	if err != nil {
		// Try index page to find the actual XML filename.
		return fetchHoldingsViaIndex(ctx, cikClean, accessionClean)
	}

	return parseHoldingsXML(body)
}

// fetchHoldingsViaIndex fetches the filing index page and finds the infotable XML.
func fetchHoldingsViaIndex(ctx context.Context, cik, accessionClean string) ([]Holding, error) {
	indexURL := fmt.Sprintf("%s/%s/%s/index.json", archivesBase, cik, accessionClean)
	body, err := doGet(ctx, indexURL)
	if err != nil {
		return nil, fmt.Errorf("filing index fetch: %w", err)
	}

	var idx struct {
		Directory struct {
			Item []struct {
				Name string `json:"name"`
			} `json:"item"`
		} `json:"directory"`
	}
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("filing index parse: %w", err)
	}

	// Look for XML file containing "infotable" (case-insensitive).
	var xmlFile string
	for _, item := range idx.Directory.Item {
		lower := strings.ToLower(item.Name)
		if strings.Contains(lower, "infotable") && strings.HasSuffix(lower, ".xml") {
			xmlFile = item.Name
			break
		}
	}
	if xmlFile == "" {
		return nil, fmt.Errorf("no infotable XML found in filing index")
	}

	time.Sleep(ratePause)

	xmlURL := fmt.Sprintf("%s/%s/%s/%s", archivesBase, cik, accessionClean, xmlFile)
	xmlBody, err := doGet(ctx, xmlURL)
	if err != nil {
		return nil, fmt.Errorf("infotable XML fetch: %w", err)
	}

	return parseHoldingsXML(xmlBody)
}

// doGet performs an HTTP GET with proper User-Agent header and context.
func doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, application/xml, text/xml")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return body, nil
}
