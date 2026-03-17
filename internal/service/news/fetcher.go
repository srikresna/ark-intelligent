package news

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

type FirecrawlFetcher struct {
	apiKey     string
	httpClient *http.Client
}

func NewFirecrawlFetcher(apiKey string) *FirecrawlFetcher {
	return &FirecrawlFetcher{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Long timeout for 3 sequential scrapes if needed
		},
	}
}

type firecrawlReq struct {
	URL         string                 `json:"url"`
	Formats     []string               `json:"formats"`
	JSONOptions map[string]interface{} `json:"jsonOptions"`
}

type scrapedEvent struct {
	Date      string `json:"date"`
	Time      string `json:"time"`
	Country   string `json:"country"`
	Event     string `json:"event"`
	Actual    string `json:"actual"`
	Previous  string `json:"previous"`
	Consensus string `json:"consensus"`
	Forecast  string `json:"forecast"`
}

type scrapeResp struct {
	Success bool `json:"success"`
	Data    struct {
		JSON struct {
			Events []scrapedEvent `json:"events"`
		} `json:"json"`
	} `json:"data"`
	Error string `json:"error"`
}

func getWeekRange(week string) (string, string) {
	now := time.Now().UTC()
	for now.Weekday() != time.Monday {
		now = now.AddDate(0, 0, -1)
	}
	start := now
	if week == "next" {
		start = start.AddDate(0, 0, 7)
	} else if week == "prev" {
		start = start.AddDate(0, 0, -7)
	}
	end := start.AddDate(0, 0, 6)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func (f *FirecrawlFetcher) ScrapeCalendar(ctx context.Context, week string) ([]domain.NewsEvent, error) {
	if f.apiKey == "" {
		return nil, fmt.Errorf("firecrawl api key is empty")
	}

	d1, d2 := getWeekRange(week)

	// Since we need impacts, and Firecrawl can't easily extract TradingEconomics CSS classes,
	// we scrape 3 times with different filters. (1=All, 2=Med+High, 3=High)
	// We do this sequentially to avoid 429 Too Many Requests on Firecrawl APIs.

	log.Printf("[FETCHER] Fetching High impact events...")
	highEvents, err := f.doScrape(ctx, fmt.Sprintf("https://tradingeconomics.com/calendar?importance=3&d1=%s&d2=%s", d1, d2))
	if err != nil {
		return nil, fmt.Errorf("failed fetching high impact: %w", err)
	}

	log.Printf("[FETCHER] Fetching Med+High impact events...")
	medHighEvents, err := f.doScrape(ctx, fmt.Sprintf("https://tradingeconomics.com/calendar?importance=2&d1=%s&d2=%s", d1, d2))
	if err != nil {
		return nil, fmt.Errorf("failed fetching med impact: %w", err)
	}

	return mergeEvents(medHighEvents, highEvents), nil
}

func (f *FirecrawlFetcher) ScrapeActuals(ctx context.Context, date string) ([]domain.NewsEvent, error) {
	return f.ScrapeCalendar(ctx, "this")
}

func (f *FirecrawlFetcher) doScrape(ctx context.Context, targetURL string) ([]scrapedEvent, error) {
	reqBody := firecrawlReq{
		URL:     targetURL,
		Formats: []string{"json"},
		JSONOptions: map[string]interface{}{
			"prompt": `Extract ALL economic calendar events from the layout table associated with each date header. DO NOT SKIP any row.
VERY IMPORTANT FOR COLUMN ALIGNMENT: The table columns are strictly ordered left to right: [Time], [Country], [Event Name], [Actual], [Previous], [Consensus], [Forecast].
If a cell value is EMPTY (like no Actual for an upcoming event), DO NOT shift the next number left! Place it strictly in its correct struct key based on the column index.

NUMERCIAL VALUE EXTRACTION: For Actual, Previous, Consensus, and Forecast, strictly extract the VISIBLE VISUAL numerical values (e.g. '0.5%', '15.5K', '58.3', '-12.0'). 
DO NOT extract underlying link identifiers, anchor tags, historical code IDs, or text tracking codes (like 'USAAECW' or 'UNITEDSTAPENHOMSAL').`,
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"events": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"date":      map[string]interface{}{"type": "string"},
								"time":      map[string]interface{}{"type": "string"},
								"country":   map[string]interface{}{"type": "string"},
								"event":     map[string]interface{}{"type": "string"},
								"actual":    map[string]interface{}{"type": "string"},
								"previous":  map[string]interface{}{"type": "string"},
								"consensus": map[string]interface{}{"type": "string"},
								"forecast":  map[string]interface{}{"type": "string"},
							},
							"required": []string{"date", "time", "country", "event"},
						},
					},
				},
				"required": []string{"events"},
			},
		},
	}

	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+f.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firecrawl API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var sResp scrapeResp
	if err := json.Unmarshal(bodyBytes, &sResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal firecrawl response: %w", err)
	}

	if !sResp.Success {
		return nil, fmt.Errorf("firecrawl job failed: %s", sResp.Error)
	}

	return sResp.Data.JSON.Events, nil
}

func genKey(e scrapedEvent) string {
	return e.Date + "|" + e.Time + "|" + strings.ToUpper(e.Country) + "|" + e.Event
}

func mergeEvents(medHigh, high []scrapedEvent) []domain.NewsEvent {
	highMap := make(map[string]bool)
	for _, e := range high {
		highMap[genKey(e)] = true
	}

	wibLoc, _ := time.LoadLocation("Asia/Jakarta")
	nyLoc, _ := time.LoadLocation("America/New_York")

	var results []domain.NewsEvent
	for _, raw := range medHigh {
		k := genKey(raw)
		impact := "medium"
		if highMap[k] {
			impact = "high"
		}

		// Firecrawl scrape returns timezone in NY (EST/EDT) usually from TradingEconomics
		// We'll parse the date and time strings.
		// date usually looks like "Tuesday March 17 2026"
		// time usually looks like "08:30 AM"
		var t time.Time
		if raw.Time != "" {
			tStr := raw.Date + " " + raw.Time
			parsed, err := time.ParseInLocation("Monday January 2 2006 03:04 PM", tStr, nyLoc)
			if err != nil {
				log.Printf("[FETCHER] skip bad date/time format: %s", tStr)
				continue
			}
			t = parsed
		} else {
			// tentative times
			parsed, err := time.ParseInLocation("Monday January 2 2006", raw.Date, nyLoc)
			if err != nil {
				continue
			}
			t = parsed
		}

		wibTime := t.In(wibLoc)

		id := fmt.Sprintf("%x", md5.Sum([]byte(k)))

		results = append(results, domain.NewsEvent{
			ID:       id,
			Date:     wibTime.Format("Mon Jan 2"),
			Time:     wibTime.Format("3:04pm"),
			TimeWIB:  wibTime,
			Currency: countryToCurrency(raw.Country),
			Event:    raw.Event,
			Impact:   impact,
			Forecast: func() string {
				if raw.Forecast != "" { return raw.Forecast }
				return raw.Consensus
			}(),
			Previous: raw.Previous,
			Actual:   raw.Actual,
			Status:   statusFromActual(raw.Actual),
		})
	}
	return results
}

func statusFromActual(actual string) string {
	if actual != "" {
		return "released"
	}
	return "upcoming"
}

func countryToCurrency(c string) string {
	switch strings.ToUpper(c) {
	case "US": return "USD"
	case "EU", "DE", "FR", "IT", "ES", "NL": return "EUR"
	case "JP": return "JPY"
	case "GB": return "GBP"
	case "AU": return "AUD"
	case "NZ": return "NZD"
	case "CA": return "CAD"
	case "CH": return "CHF"
	default: return strings.ToUpper(c)
	}
}
