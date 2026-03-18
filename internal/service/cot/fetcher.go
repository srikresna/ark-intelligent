package cot

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// Fetcher retrieves COT data from CFTC Socrata API with CSV fallback.
// Primary: CFTC Socrata Open Data API (JSON)
// Fallback: CFTC bulk CSV download from cftc.gov
type Fetcher struct {
	httpClient *http.Client
	endpoints  map[string]string // reportType -> url
	defaultCSV string
}

// NewFetcher creates a COT fetcher with modern CFTC endpoints.
func NewFetcher() *Fetcher {
	return &Fetcher{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		endpoints: map[string]string{
			"TFF":           "https://publicreporting.cftc.gov/resource/yw9f-hn96.json", // TFF Combined
			"DISAGGREGATED": "https://publicreporting.cftc.gov/resource/kh3c-gbw2.json", // Disaggregated Combined
		},
		defaultCSV: "https://www.cftc.gov/dea/newcot/deafut.txt", // Legacy fallback (still useful)
	}
}

// FetchLatest retrieves the most recent COT records for all tracked contracts.
// It compares Socrata API and CSV fallback and picks the one with the more recent data.
func (f *Fetcher) FetchLatest(ctx context.Context, contracts []domain.COTContract) ([]domain.COTRecord, error) {
	socrataRecords, sErr := f.fetchFromSocrata(ctx, contracts)
	csvRecords, cErr := f.fetchFromCSV(ctx, contracts)

	if sErr != nil && cErr != nil {
		return nil, fmt.Errorf("both Socrata (%v) and CSV (%v) failed", sErr, cErr)
	}

	if sErr != nil {
		log.Printf("[cot] Socrata failed, using CSV: %v", sErr)
		return csvRecords, nil
	}
	if cErr != nil {
		log.Printf("[cot] CSV failed, using Socrata: %v", cErr)
		return socrataRecords, nil
	}

	// Compare dates to pick the freshest data
	sDate := getLatestDate(socrataRecords)
	cDate := getLatestDate(csvRecords)

	if cDate.After(sDate) {
		log.Printf("[cot] CSV data (%s) is newer than Socrata (%s), using CSV",
			cDate.Format("2006-01-02"), sDate.Format("2006-01-02"))
		return csvRecords, nil
	}

	return socrataRecords, nil
}

func getLatestDate(records []domain.COTRecord) time.Time {
	var latest time.Time
	for _, r := range records {
		if r.ReportDate.After(latest) {
			latest = r.ReportDate
		}
	}
	return latest
}

// FetchHistory retrieves historical COT data for a specific contract.
func (f *Fetcher) FetchHistory(ctx context.Context, contract domain.COTContract, weeks int) ([]domain.COTRecord, error) {
	url, ok := f.endpoints[contract.ReportType]
	if !ok {
		return nil, fmt.Errorf("no endpoint for report type %s", contract.ReportType)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("$where", fmt.Sprintf("cftc_contract_market_code='%s'", contract.Code))
	q.Add("$order", "report_date_as_yyyy_mm_dd DESC")
	q.Add("$limit", fmt.Sprintf("%d", weeks))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch history: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("socrata history: status %d", resp.StatusCode)
	}

	var raw []domain.SocrataRecord
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}

	records := make([]domain.COTRecord, 0, len(raw))
	for _, sr := range raw {
		records = append(records, socrataToRecord(sr, contract))
	}

	return records, nil
}

// FetchAllHistory retrieves a full year of historical data for all contracts.
func (f *Fetcher) FetchAllHistory(ctx context.Context, contracts []domain.COTContract) ([]domain.COTRecord, error) {
	var allRecords []domain.COTRecord
	for _, c := range contracts {
		log.Printf("[cot] Syncing history for %s...", c.Name)
		history, err := f.FetchHistory(ctx, c, 52)
		if err != nil {
			log.Printf("[cot] warn: failed to fetch history for %s: %v", c.Name, err)
			continue
		}
		allRecords = append(allRecords, history...)
		// Stagger requests to avoid Socrata rate limits
		time.Sleep(200 * time.Millisecond)
	}
	return allRecords, nil
}

// fetchFromSocrata queries the CFTC Socrata API for latest data from multiple reports.
func (f *Fetcher) fetchFromSocrata(ctx context.Context, contracts []domain.COTContract) ([]domain.COTRecord, error) {
	// Group contracts by report type
	byReport := make(map[string][]domain.COTContract)
	for _, c := range contracts {
		byReport[c.ReportType] = append(byReport[c.ReportType], c)
	}

	var allRecords []domain.COTRecord
	var errs []error

	for reportType, reportContracts := range byReport {
		url, ok := f.endpoints[reportType]
		if !ok {
			log.Printf("[cot] warn: no endpoint for report type %s", reportType)
			continue
		}

		records, err := f.fetchReport(ctx, url, reportContracts)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", reportType, err))
			continue
		}
		allRecords = append(allRecords, records...)
	}

	if len(allRecords) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all socrata reports failed: %v", errs)
	}

	return allRecords, nil
}

func (f *Fetcher) fetchReport(ctx context.Context, url string, contracts []domain.COTContract) ([]domain.COTRecord, error) {
	codes := make([]string, len(contracts))
	for i, c := range contracts {
		codes[i] = fmt.Sprintf("'%s'", c.Code)
	}
	where := fmt.Sprintf("cftc_contract_market_code in(%s)", strings.Join(codes, ","))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("$where", where)
	q.Add("$order", "report_date_as_yyyy_mm_dd DESC")
	q.Add("$limit", fmt.Sprintf("%d", len(contracts)*2))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var raw []domain.SocrataRecord
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	contractMap := make(map[string]domain.COTContract)
	for _, c := range contracts {
		contractMap[c.Code] = c
	}

	seen := make(map[string]bool)
	var records []domain.COTRecord
	for _, sr := range raw {
		contract, ok := contractMap[sr.ContractCode]
		if !ok || seen[sr.ContractCode] {
			continue
		}
		seen[sr.ContractCode] = true
		records = append(records, socrataToRecord(sr, contract))
	}

	return records, nil
}

// fetchFromCSV downloads and parses the CFTC bulk CSV as fallback.
func (f *Fetcher) fetchFromCSV(ctx context.Context, contracts []domain.COTContract) ([]domain.COTRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.defaultCSV, nil)
	if err != nil {
		return nil, fmt.Errorf("create csv request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("csv request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("csv status %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	reader.LazyQuotes = true

	// Read header row
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("csv header: %w", err)
	}

	colIdx := buildColumnIndex(header)
	contractMap := buildContractMap(contracts)
	var records []domain.COTRecord

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		cftcCode := getCSVField(row, colIdx, "CFTC_Contract_Market_Code")
		contract, ok := contractMap[cftcCode]
		if !ok {
			continue
		}

		record := csvRowToRecord(row, colIdx, contract)
		records = append(records, record)
	}

	return records, nil
}

// --- conversion helpers ---

// socrataFloat parses a string field from Socrata JSON to float64.
func socrataFloat(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func socrataInt(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func socrataToRecord(sr domain.SocrataRecord, contract domain.COTContract) domain.COTRecord {
	reportDate, _ := time.Parse("2006-01-02T15:04:05.000", sr.ReportDate)
	if reportDate.IsZero() && len(sr.ReportDate) >= 10 {
		reportDate, _ = time.Parse("2006-01-02", sr.ReportDate[:10])
	}

	rec := domain.COTRecord{
		ContractCode: contract.Code,
		ContractName: contract.Name,
		ReportDate:   reportDate,
		OpenInterest: socrataFloat(sr.OpenInterest),

		// TFF positions
		DealerLong:    socrataFloat(sr.DealerPositionsLong),
		DealerShort:   socrataFloat(sr.DealerPositionsShort),
		AssetMgrLong:  socrataFloat(sr.AssetMgrPositionsLong),
		AssetMgrShort: socrataFloat(sr.AssetMgrPositionsShort),
		LevFundLong:   socrataFloat(sr.LevMoneyPositionsLong),
		LevFundShort:  socrataFloat(sr.LevMoneyPositionsShort),

		// TFF spread positions
		DealerSpread:   socrataFloat(sr.DealerPositionsSpread),
		AssetMgrSpread: socrataFloat(sr.AssetMgrPositionsSpread),
		LevFundSpread:  socrataFloat(sr.LevMoneyPositionsSpread),
		OtherSpread:    socrataFloat(sr.OtherReptSpread),

		// TFF WoW changes (API-computed — preferred over manual history diff)
		DealerLongChg:    socrataFloat(sr.ChangeDealerLong),
		DealerShortChg:   socrataFloat(sr.ChangeDealerShort),
		AssetMgrLongChg:  socrataFloat(sr.ChangeAssetMgrLong),
		AssetMgrShortChg: socrataFloat(sr.ChangeAssetMgrShort),
		LevFundLongChg:   socrataFloat(sr.ChangeLevMoneyLong),
		LevFundShortChg:  socrataFloat(sr.ChangeLevMoneyShort),
		OIChangeAPI:      socrataFloat(sr.ChangeOI),

		// TFF trader counts
		AssetMgrLongTraders:  socrataInt(sr.TradersAssetMgrLong),
		AssetMgrShortTraders: socrataInt(sr.TradersAssetMgrShort),
		DealerLongTraders:    socrataInt(sr.TradersDealerLong),
		DealerShortTraders:   socrataInt(sr.TradersDealerShort),
		LevFundLongTraders:   socrataInt(sr.TradersLevMoneyLong),
		LevFundShortTraders:  socrataInt(sr.TradersLevMoneyShort),
		TotalTraders:         socrataInt(sr.TradersTotAll),

		// DISAGG positions
		ProdMercLong:      socrataFloat(sr.ProdMercPositionsLong),
		ProdMercShort:     socrataFloat(sr.ProdMercPositionsShort),
		SwapDealerLong:    socrataFloat(sr.SwapPositionsLong),
		SwapDealerShort:   socrataFloat(sr.SwapPositionsShort),
		ManagedMoneyLong:  socrataFloat(sr.MMoneyPositionsLong),
		ManagedMoneyShort: socrataFloat(sr.MMoneyPositionsShort),

		// DISAGG spread
		ManagedMoneySpread: socrataFloat(sr.MMoneyPositionsSpread),
		ProdMercSpread:    socrataFloat(sr.ProdMercPositionsSpread),
		SwapDealerSpread:  socrataFloat(sr.SwapPositionsSpread),

		// DISAGG WoW changes
		ProdMercLongChg:      socrataFloat(sr.ChangeProdMercLong),
		ProdMercShortChg:     socrataFloat(sr.ChangeProdMercShort),
		SwapLongChg:          socrataFloat(sr.ChangeSwapLong),
		SwapShortChg:         socrataFloat(sr.ChangeSwapShort),
		ManagedMoneyLongChg:  socrataFloat(sr.ChangeMMoneyLong),
		ManagedMoneyShortChg: socrataFloat(sr.ChangeMMoneyShort),

		// Shared WoW changes
		SmallLongChgAPI:  socrataFloat(sr.ChangeNonReptLong),
		SmallShortChgAPI: socrataFloat(sr.ChangeNonReptShort),
		OtherLongChg:     socrataFloat(sr.ChangeOtherReptLong),
		OtherShortChg:    socrataFloat(sr.ChangeOtherReptShort),

		// DISAGG trader counts
		MMoneyLongTraders:    socrataInt(sr.TradersMMoneyLong),
		MMoneyShortTraders:   socrataInt(sr.TradersMMoneyShort),
		ProdMercLongTraders:  socrataInt(sr.TradersProdMercLong),
		ProdMercShortTraders: socrataInt(sr.TradersProdMercShort),
		TotalTradersDisag:    socrataInt(sr.TradersTotDisag),

		// Shared
		SmallLong:  socrataFloat(sr.NonReptPositionsLong),
		SmallShort: socrataFloat(sr.NonReptPositionsShort),
		OtherLong:  socrataFloat(sr.OtherReptPositionsLong),
		OtherShort: socrataFloat(sr.OtherReptPositionsShort),

		// Concentration
		Top4Long:  socrataFloat(sr.Top4Long),
		Top4Short: socrataFloat(sr.Top4Short),
		Top8Long:  socrataFloat(sr.Top8Long),
		Top8Short: socrataFloat(sr.Top8Short),
	}

	// Populate NetChange from API-provided change fields (more accurate than history diff).
	// Falls back to zero — analyzer will compute from history if needed.
	rec.NetChange = rec.GetSmartMoneyNetChangeAPI(contract.ReportType)

	// Manually map TotalTradersDisag from shared TradersTotAll field
	// (TradersTotDisag has json:"-" to avoid duplicate tag conflict)
	if contract.ReportType == "DISAGGREGATED" {
		rec.TotalTradersDisag = socrataInt(sr.TradersTotAll)
	}

	return rec
}

// csvRowToRecord converts a CSV row to a COTRecord.
func csvRowToRecord(row []string, colIdx map[string]int, contract domain.COTContract) domain.COTRecord {
	reportDate, _ := time.Parse("2006-01-02", getCSVField(row, colIdx, "As_of_Date_In_Form_YYMMDD"))
	if reportDate.IsZero() {
		// Try alternate format
		reportDate, _ = time.Parse("060102", getCSVField(row, colIdx, "As_of_Date_In_Form_YYMMDD"))
	}

	return domain.COTRecord{
		ContractCode: contract.Code,
		ContractName: contract.Name,
		ReportDate:   reportDate,
		OpenInterest: csvFloat(row, colIdx, "Open_Interest_All"),

		CommLong:   csvFloat(row, colIdx, "Comm_Positions_Long_All"),
		CommShort:  csvFloat(row, colIdx, "Comm_Positions_Short_All"),
		SpecLong:   csvFloat(row, colIdx, "NonComm_Positions_Long_All"),
		SpecShort:  csvFloat(row, colIdx, "NonComm_Positions_Short_All"),
		SmallLong:  csvFloat(row, colIdx, "NonRept_Positions_Long_All"),
		SmallShort: csvFloat(row, colIdx, "NonRept_Positions_Short_All"),

		CommLongChange:   csvFloat(row, colIdx, "Change_in_Comm_Long_All"),
		CommShortChange:  csvFloat(row, colIdx, "Change_in_Comm_Short_All"),
		SpecLongChange:   csvFloat(row, colIdx, "Change_in_NonComm_Long_All"),
		SpecShortChange:  csvFloat(row, colIdx, "Change_in_NonComm_Short_All"),
		SmallLongChange:  csvFloat(row, colIdx, "Change_in_NonRept_Long_All"),
		SmallShortChange: csvFloat(row, colIdx, "Change_in_NonRept_Short_All"),

		Top4Long:  csvFloat(row, colIdx, "Pct_of_OI_4_or_Less_Long_All"),
		Top4Short: csvFloat(row, colIdx, "Pct_of_OI_4_or_Less_Short_All"),
		Top8Long:  csvFloat(row, colIdx, "Pct_of_OI_8_or_Less_Long_All"),
		Top8Short: csvFloat(row, colIdx, "Pct_of_OI_8_or_Less_Short_All"),
	}
}

// --- CSV helpers ---

func buildContractMap(contracts []domain.COTContract) map[string]domain.COTContract {
	m := make(map[string]domain.COTContract, len(contracts))
	for _, c := range contracts {
		m[c.Code] = c
	}
	return m
}

func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

func getCSVField(row []string, colIdx map[string]int, col string) string {
	if i, ok := colIdx[col]; ok && i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}

func csvFloat(row []string, colIdx map[string]int, col string) float64 {
	s := getCSVField(row, colIdx, col)
	s = strings.ReplaceAll(s, ",", "")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
