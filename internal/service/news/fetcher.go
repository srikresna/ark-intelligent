package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/pkg/circuitbreaker"
	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("mql5")

// MQL5Fetcher fetches economic calendar data from MQL5's hidden POST endpoint.
// No API key required. Returns all impact levels (low, medium, high, holiday).
type MQL5Fetcher struct {
	httpClient *http.Client
	cb         *circuitbreaker.Breaker
}

// NewMQL5Fetcher creates a new MQL5Fetcher.
func NewMQL5Fetcher() *MQL5Fetcher {
	return &MQL5Fetcher{
		httpClient: httpclient.NewClient(30 * time.Second),
		cb:         circuitbreaker.New("mql5", 5, 3*time.Minute),
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

// mql5Endpoint is the hidden POST API used by the MQL5 Economic Calendar page.
const mql5Endpoint = "https://www.mql5.com/en/economic-calendar/content"

// mql5ImportanceAll — bitmask 15 = low(1) + medium(2) + high(4) + holiday(8)
const mql5ImportanceAll = "15"

// mql5CurrenciesMask — 131071 = all major currencies supported by MQL5
const mql5CurrenciesMask = "131071"

// wibLocation is WIB (UTC+7).
var wibLocation *time.Location

func init() {
	var err error
	// MQL5 FullDate is in UTC — confirmed via ReleaseDate (Unix ms) cross-check.
	// e.g. RBA 04:30 UTC = 11:30 WIB ✓, Fed Manufacturing 13:15 UTC = 20:15 WIB ✓
	wibLocation, err = time.LoadLocation("Asia/Jakarta")
	if err != nil {
		wibLocation = time.FixedZone("WIB", 7*60*60)
	}
}

// ---------------------------------------------------------------------------
// Date range helpers
// ---------------------------------------------------------------------------

// getWeekRange returns the from/to ISO timestamps for a given week label.
// week: "this" | "next" | "prev"
func getWeekRange(week string) (from, to string) {
	now := time.Now().In(wibLocation)
	// Walk back to Monday
	for now.Weekday() != time.Monday {
		now = now.AddDate(0, 0, -1)
	}
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, wibLocation)

	switch week {
	case "next":
		monday = monday.AddDate(0, 0, 7)
	case "prev":
		monday = monday.AddDate(0, 0, -7)
	}

	sunday := monday.AddDate(0, 0, 6)
	end := time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 0, wibLocation)

	from = monday.UTC().Format("2006-01-02T15:04:05")
	to = end.UTC().Format("2006-01-02T15:04:05")
	return
}

// getMonthRange returns the dateMode and from/to for a given month type.
// monthType: "current" | "prev" | "next"
func getMonthRange(monthType string) (dateMode, from, to string) {
	now := time.Now().In(wibLocation)

	var year int
	var month time.Month

	switch monthType {
	case "prev":
		dateMode = "5"
		prev := now.AddDate(0, -1, 0)
		year, month = prev.Year(), prev.Month()
	case "next":
		dateMode = "6"
		next := now.AddDate(0, 1, 0)
		year, month = next.Year(), next.Month()
	default: // "current"
		dateMode = "4"
		year, month = now.Year(), now.Month()
	}

	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, wibLocation)
	lastDay := firstDay.AddDate(0, 1, -1)
	endOfLastDay := time.Date(lastDay.Year(), lastDay.Month(), lastDay.Day(), 23, 59, 59, 0, wibLocation)

	from = firstDay.UTC().Format("2006-01-02T15:04:05")
	to = endOfLastDay.UTC().Format("2006-01-02T15:04:05")
	return
}

// ---------------------------------------------------------------------------
// Public fetch methods
// ---------------------------------------------------------------------------

// ScrapeCalendar fetches all events (all impact levels) for the given week.
// week: "this" | "next" | "prev"
func (f *MQL5Fetcher) ScrapeCalendar(ctx context.Context, week string) ([]domain.NewsEvent, error) {
	fromStr, toStr := getWeekRange(week)
	log.Info().Str("week", week).Str("from", fromStr).Str("to", toStr).Msg("ScrapeCalendar")

	raw, err := f.fetchMQL5(ctx, "1", fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("mql5 fetch failed: %w", err)
	}

	events := convertEvents(raw, "low", "medium", "high", "holiday", "none")
	log.Info().Int("events", len(events)).Msg("ScrapeCalendar returned events (all impact)")
	return events, nil
}

// ScrapeActuals fetches events for a specific day using a precise from/to range.
// date format: "20060102" (WIB). Falls back to "this" week on parse error.
func (f *MQL5Fetcher) ScrapeActuals(ctx context.Context, date string) ([]domain.NewsEvent, error) {
	t, err := time.ParseInLocation("20060102", date, wibLocation)
	if err != nil {
		log.Warn().Str("date", date).Err(err).Msg("ScrapeActuals: bad date, falling back to this week")
		return f.ScrapeCalendar(ctx, "this")
	}

	// Build a [00:00, 23:59:59] WIB window for that specific day, converted to UTC for MQL5.
	from := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, wibLocation).UTC().Format("2006-01-02T15:04:05")
	to := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, wibLocation).UTC().Format("2006-01-02T15:04:05")

	log.Info().Str("date", date).Str("from", from).Str("to", to).Msg("ScrapeActuals")
	raw, err := f.fetchMQL5(ctx, "1", from, to)
	if err != nil {
		return nil, fmt.Errorf("mql5 fetch actuals failed: %w", err)
	}
	events := convertEvents(raw, "low", "medium", "high", "holiday", "none")
	log.Info().Int("events", len(events)).Msg("ScrapeActuals returned events")
	return events, nil
}

// ScrapeMonth fetches all events for a given month.
// monthType: "current" | "prev" | "next"
func (f *MQL5Fetcher) ScrapeMonth(ctx context.Context, monthType string) ([]domain.NewsEvent, error) {
	dateMode, fromStr, toStr := getMonthRange(monthType)
	log.Info().Str("type", monthType).Str("date_mode", dateMode).Str("from", fromStr).Str("to", toStr).Msg("ScrapeMonth")

	raw, err := f.fetchMQL5(ctx, dateMode, fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("mql5 fetch month failed: %w", err)
	}

	events := convertEvents(raw, "low", "medium", "high", "holiday", "none")
	log.Info().Int("events", len(events)).Msg("ScrapeMonth returned events")
	return events, nil
}

// ScrapeRange fetches events with an explicit date_mode, from, and to.
func (f *MQL5Fetcher) ScrapeRange(ctx context.Context, dateMode, from, to string) ([]domain.NewsEvent, error) {
	log.Info().Str("date_mode", dateMode).Str("from", from).Str("to", to).Msg("ScrapeRange")

	raw, err := f.fetchMQL5(ctx, dateMode, from, to)
	if err != nil {
		return nil, fmt.Errorf("mql5 fetch range failed: %w", err)
	}

	events := convertEvents(raw, "low", "medium", "high", "holiday", "none")
	log.Info().Int("events", len(events)).Msg("ScrapeRange returned events")
	return events, nil
}

// ---------------------------------------------------------------------------
// Internal HTTP fetch
// ---------------------------------------------------------------------------

// fetchMQL5 performs the POST request to MQL5 with circuit breaker and retry.
func (f *MQL5Fetcher) fetchMQL5(ctx context.Context, dateMode, from, to string) ([]mql5Event, error) {
	var result []mql5Event
	err := f.cb.Execute(func() error {
		var fetchErr error
		result, fetchErr = f.doFetchMQL5(ctx, dateMode, from, to)
		if fetchErr != nil {
			// Retry once on transient failure; respect context cancellation during wait
			log.Warn().Err(fetchErr).Msg("MQL5 fetch failed, retrying in 3s")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
			result, fetchErr = f.doFetchMQL5(ctx, dateMode, from, to)
		}
		return fetchErr
	})
	return result, err
}

// doFetchMQL5 performs the raw POST request to MQL5.
func (f *MQL5Fetcher) doFetchMQL5(ctx context.Context, dateMode, from, to string) ([]mql5Event, error) {
	form := url.Values{}
	form.Set("date_mode", dateMode)
	form.Set("from", from)
	form.Set("to", to)
	form.Set("importance", mql5ImportanceAll)
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

	var events []mql5Event
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("failed to parse mql5 response: %w (body: %.200s)", err, string(body))
	}

	return events, nil
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

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

		// Parse FullDate as UTC (confirmed: MQL5 FullDate is UTC, not New York time)
		t, err := time.ParseInLocation("2006-01-02T15:04:05", e.FullDate, time.UTC)
		if err != nil {
			log.Warn().Int("id", e.ID).Str("event", e.EventName).Str("date", e.FullDate).Err(err).Msg("skip event: bad date")
			continue
		}

		wibTime := t.In(wibLocation)

		// Determine currency: prefer CurrencyCode from API, fall back to country code map
		currency := e.CurrencyCode
		if currency == "" {
			currency = countryIDToCurrency(e.Country)
		}

		// Skip events with unknown/unsupported currencies.
		// Exception: "none" impact holidays may carry a country currency we still want to show.
		// If currency is truly empty even after fallback, skip.
		if currency == "" {
			continue
		}

		id := fmt.Sprintf("mql5-%d", e.ID)

		results = append(results, domain.NewsEvent{
			ID:              id,
			Date:            wibTime.Format("Mon Jan 2"),
			Time:            wibTime.Format("15:04"),
			TimeWIB:         wibTime,
			Currency:        currency,
			Event:           e.EventName,
			Impact:          impact,
			Forecast:        e.ForecastValue,
			Previous:        e.PreviousValue,
			OldPrevious:     e.OldPreviousValue,
			Actual:          e.ActualValue,
			Status:          statusFromProcessed(e.Processed),
			ImpactDirection: e.ImpactDirection,
			ImpactValue:     e.ImpactValue,
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
func countryIDToCurrency(countryID int) string {
	switch countryID {
	case 840:
		return "USD"
	case 999, 276, 250, 380, 724, 528, 56, 40, 246, 372, 620, 300, 705, 233, 428, 440, 196, 442, 470:
		return "EUR"
	case 392:
		return "JPY"
	case 826:
		return "GBP"
	case 36:
		return "AUD"
	case 124:
		return "CAD"
	case 756:
		return "CHF"
	case 554:
		return "NZD"
	case 156:
		return "CNY"
	default:
		return ""
	}
}
