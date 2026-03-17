package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arkcode369/ff-calendar-bot/internal/domain"
)

// MQL5Fetcher fetches economic calendar data from MQL5's hidden POST endpoint.
// No API key required. Returns Medium + High impact events with actual values.
type MQL5Fetcher struct {
	httpClient *http.Client
}

// NewMQL5Fetcher creates a new MQL5Fetcher.
func NewMQL5Fetcher() *MQL5Fetcher {
	return &MQL5Fetcher{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// mql5Event is the raw JSON structure returned by the MQL5 API.
type mql5Event struct {
	ID               int    `json:"Id"`
	EventName        string `json:"EventName"`
	Importance       string `json:"Importance"`   // "low", "medium", "high"
	CurrencyCode     string `json:"CurrencyCode"` // Already the currency code e.g. "USD"
	Country          int    `json:"Country"`      // int country code
	ActualValue      string `json:"ActualValue"`
	ForecastValue    string `json:"ForecastValue"`
	PreviousValue    string `json:"PreviousValue"`
	OldPreviousValue string `json:"OldPreviousValue"`
	FullDate         string `json:"FullDate"`        // "2026-03-17T14:00:00" — New York (ET)
	ReleaseDate      int64  `json:"ReleaseDate"`     // Unix milliseconds (unused; use FullDate)
	ImpactDirection  int    `json:"ImpactDirection"` // 0=neutral, 1=positive, 2=negative
	ImpactValue      string `json:"ImpactValue"`
	Processed        int    `json:"Processed"` // 1=released, 0=upcoming
}

// mql5DateMode — MQL5 supports several date modes; we use mode 1 (custom date range).
const mql5DateMode = "1"

// mql5Endpoint is the hidden POST API used by the MQL5 Economic Calendar page.
const mql5Endpoint = "https://www.mql5.com/en/economic-calendar/content"

// importance values:
//
//	4 = Medium + High (we filter ourselves from the response)
//	We always fetch with importance=4 and filter in Go.
const mql5ImportanceFilter = "4"

// currencies bitmask — 262143 = all major currencies
const mql5CurrenciesMask = "262143"

// nyLocation is New York (ET) — the timezone MQL5 FullDate uses.
var nyLocation *time.Location

// wibLocation is WIB (UTC+7).
var wibLocation *time.Location

func init() {
	var err error
	nyLocation, err = time.LoadLocation("America/New_York")
	if err != nil {
		nyLocation = time.FixedZone("EST", -5*60*60)
	}
	wibLocation, err = time.LoadLocation("Asia/Jakarta")
	if err != nil {
		wibLocation = time.FixedZone("WIB", 7*60*60)
	}
}

// getWeekRange returns the from/to ISO timestamps for a given week label.
// week: "this" | "next" | "prev"
// Returns times in UTC (MQL5 accepts UTC-formatted ISO strings).
func getWeekRange(week string) (string, string) {
	// Anchor to Monday of the current WIB week
	now := time.Now().In(wibLocation)
	// Walk back to Monday
	for now.Weekday() != time.Monday {
		now = now.AddDate(0, 0, -1)
	}
	// Zero out to midnight WIB Monday
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, wibLocation)

	switch week {
	case "next":
		monday = monday.AddDate(0, 0, 7)
	case "prev":
		monday = monday.AddDate(0, 0, -7)
	}

	// End = Sunday 23:59:59 of that week
	sunday := monday.AddDate(0, 0, 6)
	end := time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 0, wibLocation)

	// MQL5 wants format: 2006-01-02T15:04:05 (no timezone, interpreted as NY time by server)
	// But sending UTC works as the server adjusts. We'll send in UTC format.
	fromStr := monday.UTC().Format("2006-01-02T15:04:05")
	toStr := end.UTC().Format("2006-01-02T15:04:05")
	return fromStr, toStr
}

// ScrapeCalendar fetches all Medium+High impact events for the given week.
// week: "this" | "next" | "prev"
func (f *MQL5Fetcher) ScrapeCalendar(ctx context.Context, week string) ([]domain.NewsEvent, error) {
	fromStr, toStr := getWeekRange(week)
	log.Printf("[MQL5] ScrapeCalendar week=%s from=%s to=%s", week, fromStr, toStr)

	raw, err := f.fetchMQL5(ctx, fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("mql5 fetch failed: %w", err)
	}

	events := convertEvents(raw, "medium", "high")
	log.Printf("[MQL5] ScrapeCalendar returned %d events (med+high)", len(events))
	return events, nil
}

// ScrapeActuals fetches today's events (for micro-scrape to pick up actuals).
// date parameter is ignored; we always fetch "this" week's data.
func (f *MQL5Fetcher) ScrapeActuals(ctx context.Context, date string) ([]domain.NewsEvent, error) {
	// For actuals we fetch the current week — same as ScrapeCalendar("this")
	return f.ScrapeCalendar(ctx, "this")
}

// fetchMQL5 performs the POST request to MQL5 and returns the raw event list.
func (f *MQL5Fetcher) fetchMQL5(ctx context.Context, from, to string) ([]mql5Event, error) {
	form := url.Values{}
	form.Set("date_mode", mql5DateMode)
	form.Set("from", from)
	form.Set("to", to)
	form.Set("importance", mql5ImportanceFilter)
	form.Set("currencies", mql5CurrenciesMask)

	req, err := http.NewRequestWithContext(ctx, "POST", mql5Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	// Required headers to avoid 403/empty response
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.mql5.com/en/economic-calendar")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Origin", "https://www.mql5.com")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mql5 API returned status %d: %s", resp.StatusCode, string(body))
	}

	// MQL5 returns a JSON array directly
	var events []mql5Event
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("failed to parse mql5 response: %w (body: %.200s)", err, string(body))
	}

	return events, nil
}

// convertEvents converts raw MQL5 events to domain.NewsEvent, filtering by impact level.
func convertEvents(raw []mql5Event, allowedImpacts ...string) []domain.NewsEvent {
	allowMap := make(map[string]bool, len(allowedImpacts))
	for _, imp := range allowedImpacts {
		allowMap[strings.ToLower(imp)] = true
	}

	var results []domain.NewsEvent
	for _, e := range raw {
		impact := strings.ToLower(e.Importance)
		if !allowMap[impact] {
			continue
		}

		// Parse FullDate as New York time
		t, err := time.ParseInLocation("2006-01-02T15:04:05", e.FullDate, nyLocation)
		if err != nil {
			log.Printf("[MQL5] skip event %d %q: bad date %q: %v", e.ID, e.EventName, e.FullDate, err)
			continue
		}

		wibTime := t.In(wibLocation)

		// Determine currency: prefer CurrencyCode from API, fall back to country code map
		currency := e.CurrencyCode
		if currency == "" {
			currency = countryIDToCurrency(e.Country)
		}

		// Skip events with unknown/unsupported currencies
		if currency == "" {
			continue
		}

		id := fmt.Sprintf("mql5-%d", e.ID)

		results = append(results, domain.NewsEvent{
			ID:       id,
			Date:     wibTime.Format("Mon Jan 2"),
			Time:     wibTime.Format("15:04"),
			TimeWIB:  wibTime,
			Currency: currency,
			Event:    e.EventName,
			Impact:   impact,
			Forecast: e.ForecastValue,
			Previous: e.PreviousValue,
			Actual:   e.ActualValue,
			Status:   statusFromProcessed(e.Processed),
		})
	}
	return results
}

// statusFromProcessed maps MQL5 Processed field to domain status string.
func statusFromProcessed(processed int) string {
	if processed == 1 {
		return "released"
	}
	return "upcoming"
}

// countryIDToCurrency maps MQL5 integer country codes to currency codes.
// Used as fallback if CurrencyCode is empty.
func countryIDToCurrency(countryID int) string {
	switch countryID {
	case 840:
		return "USD" // United States
	case 999, 276, 250, 380, 724, 528, 56, 40, 246, 372, 620, 300, 705, 233, 428, 440, 196, 442, 470:
		return "EUR" // Eurozone countries
	case 392:
		return "JPY" // Japan
	case 826:
		return "GBP" // United Kingdom
	case 36:
		return "AUD" // Australia
	case 124:
		return "CAD" // Canada
	case 756:
		return "CHF" // Switzerland
	case 554:
		return "NZD" // New Zealand
	case 156:
		return "CNY" // China
	default:
		return ""
	}
}
