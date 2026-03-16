package news

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
	"github.com/arkcode369/ff-calendar-bot/pkg/timeutil"
)

// FirecrawlFetcher implements ports.NewsFetcher using Firecrawl's REST API.
type FirecrawlFetcher struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewFirecrawlFetcher creates a new fetcher instance.
func NewFirecrawlFetcher(apiKey string) *FirecrawlFetcher {
	return &FirecrawlFetcher{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 45 * time.Second, // Scrape can take a bit
		},
		baseURL: "https://api.firecrawl.dev/v1/scrape",
	}
}

// ScrapeCalendar syncs the entire week's upcoming data.
// 'week' parameter should be "this" or "next".
func (f *FirecrawlFetcher) ScrapeCalendar(ctx context.Context, week string) ([]domain.NewsEvent, error) {
	urlTarget := "https://www.forexfactory.com/calendar?week=this" // Enforce full week anonymously
	if week == "next" {
		urlTarget = "https://www.forexfactory.com/calendar?week=next"
	}
	
	prompt := `Extract ALL scheduled economic calendar events from the table on the page for the ENTIRE week.
Include events for Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday. Do NOT skip items.
Return ONLY an array of events named "calendar_events".
For each event I need:
- id: a unique hash string generated based on name and date
- date: e.g., "Mon Mar 16", "Tue Mar 17"
- time: "7:30am" or "Tentative"
- currency: "USD", "EUR" etc.
- event: full title like "CPI m/m"
- impact: "high", "medium", "low", "non"
- forecast: string
- previous: string
- actual: string (or empty if blank)

Ensure null values are converted to empty strings.`

	return f.doExtractCall(ctx, urlTarget, prompt)
}

// ScrapeActuals performs a micro-pull just for a specific date's actuals.
// Target example: https://www.forexfactory.com/calendar?day=mar17.2026
func (f *FirecrawlFetcher) ScrapeActuals(ctx context.Context, date string) ([]domain.NewsEvent, error) {
	urlTarget := fmt.Sprintf("https://www.forexfactory.com/calendar?day=%s", date)
	
	prompt := `Extract the upcoming economic calendar events from ForexFactory. 
Return ONLY an array of events named "calendar_events". Focus heavily on extracting the 'actual' figures if they exist.
For each event I need exactly:
- id: a unique hash string generated based on name and date
- date: "Mon Mar 17"
- time: "7:30am" or "Tentative"
- currency: "USD", "EUR" etc.
- event: full title like "CPI m/m"
- impact: "high", "medium", "low", "non"
- forecast: string
- previous: string
- actual: string (or empty string if blank)

Ensure null values are converted to empty strings.`

	return f.doExtractCall(ctx, urlTarget, prompt)
}

func (f *FirecrawlFetcher) doExtractCall(ctx context.Context, urlTarget, prompt string) ([]domain.NewsEvent, error) {
	// Firecrawl Scrape API format with Extract (LLM) configuration
	payload := map[string]interface{}{
		"url": urlTarget,
		"formats": []string{"extract"},
		"extract": map[string]interface{}{
			"prompt": prompt,
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"calendar_events": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"id": map[string]interface{}{"type": "string"},
								"date": map[string]interface{}{"type": "string"},
								"time": map[string]interface{}{"type": "string"},
								"currency": map[string]interface{}{"type": "string"},
								"event": map[string]interface{}{"type": "string"},
								"impact": map[string]interface{}{"type": "string"},
								"forecast": map[string]interface{}{"type": "string"},
								"previous": map[string]interface{}{"type": "string"},
								"actual": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
				"required": []string{"calendar_events"},
			},
		},
		"waitFor": 5000,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", f.baseURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create req: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+f.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do req: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status %d: %s", resp.StatusCode, string(respBytes))
	}

	// Parse Firecrawl root response structure
	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			Extract struct {
				CalendarEvents []domain.NewsEvent `json:"calendar_events"`
			} `json:"extract"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		return nil, fmt.Errorf("decode resp: %w", err)
	}

	if !fcResp.Success {
		return nil, fmt.Errorf("firecrawl API returned success=false")
	}

	events := fcResp.Data.Extract.CalendarEvents

	// Hydrate parse time (TimeWIB)
	year := timeutil.NowWIB().Year()
	for i := range events {
		events[i].Status = "upcoming"
		if events[i].Actual != "" {
			events[i].Status = "released"
		}
		
		// Map "High" to "high"
		events[i].Impact = parseImpact(events[i].Impact)

		// Create TimeWIB from Date & Time fields if possible
		if events[i].Time != "Tentative" && events[i].Time != "All Day" && events[i].Time != "" {
			// Format: "Mon Mar 17 7:30am 2026"  (Merging text)
			timeStr := fmt.Sprintf("%s %s %d", events[i].Date, events[i].Time, year)
			// Timezone tricky handling: target is usually America/New_York relative if you don't login.
			// However let's assume we request it relative to GMT or we just parse string. 
			// In production, we assume user is passing a specific string format.
			loc, _ := time.LoadLocation("America/New_York") // Default ForexFactory anonymous timezone
			if t, err := time.ParseInLocation("Mon Jan 2 3:04pm 2006", timeStr, loc); err == nil {
				// Translate to WIB (+7)
				wibLoc, _ := time.LoadLocation("Asia/Jakarta")
				events[i].TimeWIB = t.In(wibLoc)
			} else {
				log.Printf("[FETCHER] Error parsing time string %q: %v", timeStr, err)
			}
		} else {
			// fallback mapping to midnight
			timeStr := fmt.Sprintf("%s 12:00am %d", events[i].Date, year)
			loc, _ := time.LoadLocation("America/New_York")
			if t, err := time.ParseInLocation("Mon Jan 2 3:04pm 2006", timeStr, loc); err == nil {
				wibLoc, _ := time.LoadLocation("Asia/Jakarta")
				events[i].TimeWIB = t.In(wibLoc)
			}
		}
	}

	return events, nil
}

func parseImpact(raw string) string {
	lower := string(bytes.ToLower([]byte(raw)))
	if lower == "high" || lower == "medium" || lower == "low" || lower == "non" {
		return lower
	}
	return "non" // assume lowest if broken
}
