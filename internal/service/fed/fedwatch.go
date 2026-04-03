// Package fed provides Federal Reserve data integrations.
//
// Currently supported:
//   - CME FedWatch implied Fed-funds rate probabilities (via Firecrawl).
//
// If FIRECRAWL_API_KEY is not set, FetchFedWatch returns Available=false
// with no error. An in-memory cache with a 4-hour TTL prevents excessive
// Firecrawl calls.
package fed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("fed")

// FedWatchData holds CME FedWatch market-implied rate probabilities.
type FedWatchData struct {
	NextMeetingDate    string  `json:"next_meeting_date"`    // e.g. "2026-05-07"
	HoldProbability    float64 `json:"hold_probability"`     // %
	Cut25Probability   float64 `json:"cut_25_probability"`   // %
	Cut50Probability   float64 `json:"cut_50_probability"`   // %
	Hike25Probability  float64 `json:"hike_25_probability"`  // %
	ImpliedYearEndRate float64 `json:"implied_year_end_rate"` // bps implied from Dec futures
	MeetingCount       int     `json:"meeting_count"`        // FOMC meetings remaining until Dec
	Available          bool    `json:"available"`
	FetchedAt          time.Time `json:"fetched_at"`
}

// cache holds the last successful FedWatch fetch with a 4-hour TTL.
var (
	cacheMu    sync.RWMutex
	cacheData  *FedWatchData
	cacheUntil time.Time
	cacheTTL   = 4 * time.Hour
)

// FetchFedWatch fetches CME FedWatch implied rate probabilities via Firecrawl
// JSON extraction. Returns a non-nil FedWatchData with Available=false if the
// API key is missing or the fetch fails.
func FetchFedWatch(ctx context.Context) *FedWatchData {
	// Check cache first.
	cacheMu.RLock()
	if cacheData != nil && time.Now().Before(cacheUntil) {
		cached := *cacheData // shallow copy
		cacheMu.RUnlock()
		log.Debug().Msg("fedwatch: returning cached data")
		return &cached
	}
	cacheMu.RUnlock()

	result := &FedWatchData{FetchedAt: time.Now()}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("fedwatch: skipping — FIRECRAWL_API_KEY not set")
		return result
	}

	type fcJSONOpts struct {
		Prompt string          `json:"prompt"`
		Schema json.RawMessage `json:"schema"`
	}
	type fcReq struct {
		URL         string      `json:"url"`
		Formats     []string    `json:"formats"`
		WaitFor     int         `json:"waitFor"`
		JSONOptions *fcJSONOpts `json:"jsonOptions,omitempty"`
	}

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"next_meeting_date":    {"type": "string", "description": "Next FOMC meeting date in YYYY-MM-DD format"},
			"hold_probability":     {"type": "number", "description": "Probability of no rate change in percent"},
			"cut_25_probability":   {"type": "number", "description": "Probability of a 25bp rate cut in percent"},
			"cut_50_probability":   {"type": "number", "description": "Probability of a 50bp or more rate cut in percent"},
			"hike_25_probability":  {"type": "number", "description": "Probability of a 25bp rate hike in percent"},
			"implied_year_end_rate":{"type": "number", "description": "Market-implied fed funds rate at year end in percent"},
			"meeting_count":        {"type": "integer", "description": "Number of FOMC meetings remaining until December"}
		}
	}`)

	reqBody := fcReq{
		URL:     "https://www.cmegroup.com/markets/interest-rates/cme-fedwatch-tool.html",
		Formats: []string{"json"},
		WaitFor: 8000, // FedWatch page is JS-heavy
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract the CME FedWatch implied probabilities for the NEXT FOMC meeting: " +
				"the next meeting date, probability of holding rates unchanged (no change), " +
				"probability of a 25 basis point rate cut, probability of a 50bp or more cut, " +
				"probability of a 25bp hike, the market-implied fed funds rate at year-end, " +
				"and the number of FOMC meetings remaining until December. " +
				"Return all probabilities as percentages (e.g. 55.2 for 55.2%).",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Debug().Err(err).Msg("fedwatch: failed to marshal Firecrawl request")
		return result
	}

	fcClient := httpclient.New(httpclient.WithTimeout(45 * time.Second))
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(bodyBytes))
	if err != nil {
		log.Debug().Err(err).Msg("fedwatch: failed to build Firecrawl request")
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("fedwatch: Firecrawl request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Msg("fedwatch: Firecrawl non-2xx response")
		return result
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				NextMeetingDate    string  `json:"next_meeting_date"`
				HoldProbability    float64 `json:"hold_probability"`
				Cut25Probability   float64 `json:"cut_25_probability"`
				Cut50Probability   float64 `json:"cut_50_probability"`
				Hike25Probability  float64 `json:"hike_25_probability"`
				ImpliedYearEndRate float64 `json:"implied_year_end_rate"`
				MeetingCount       int     `json:"meeting_count"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		log.Warn().Err(err).Msg("fedwatch: Firecrawl decode failed")
		return result
	}

	if !fcResp.Success {
		log.Warn().Msg("fedwatch: Firecrawl returned unsuccessful")
		return result
	}

	d := fcResp.Data.JSON
	result.NextMeetingDate = d.NextMeetingDate
	result.HoldProbability = d.HoldProbability
	result.Cut25Probability = d.Cut25Probability
	result.Cut50Probability = d.Cut50Probability
	result.Hike25Probability = d.Hike25Probability
	result.ImpliedYearEndRate = d.ImpliedYearEndRate
	result.MeetingCount = d.MeetingCount

	// Mark available if we got any meaningful probability data.
	if result.HoldProbability > 0 || result.Cut25Probability > 0 || result.Hike25Probability > 0 {
		result.Available = true
		log.Info().
			Str("next_meeting", result.NextMeetingDate).
			Float64("hold", result.HoldProbability).
			Float64("cut25", result.Cut25Probability).
			Float64("cut50", result.Cut50Probability).
			Float64("hike25", result.Hike25Probability).
			Float64("year_end_rate", result.ImpliedYearEndRate).
			Int("meetings_left", result.MeetingCount).
			Msg("fedwatch: fetched via Firecrawl")
	} else {
		log.Warn().Msg("fedwatch: Firecrawl returned data but no probabilities found")
	}

	// Update cache on success.
	if result.Available {
		cacheMu.Lock()
		cacheData = result
		cacheUntil = time.Now().Add(cacheTTL)
		cacheMu.Unlock()
	}

	return result
}

// DominantOutcome returns a human-readable summary of the most likely outcome.
func DominantOutcome(d *FedWatchData) string {
	if d == nil || !d.Available {
		return "N/A"
	}
	type outcome struct {
		label string
		prob  float64
	}
	outcomes := []outcome{
		{"Hold", d.HoldProbability},
		{"Cut 25bp", d.Cut25Probability},
		{"Cut 50bp+", d.Cut50Probability},
		{"Hike 25bp", d.Hike25Probability},
	}
	best := outcomes[0]
	for _, o := range outcomes[1:] {
		if o.prob > best.prob {
			best = o
		}
	}
	return fmt.Sprintf("%s (%.1f%%)", best.label, best.prob)
}

// ImpliedCutsToYearEnd estimates the number of 25bp cuts implied by the
// difference between the current fed-funds rate and year-end implied rate.
// currentRate should be the actual FFR (e.g. 4.50).
func ImpliedCutsToYearEnd(d *FedWatchData, currentRate float64) float64 {
	if d == nil || !d.Available || d.ImpliedYearEndRate <= 0 || currentRate <= 0 {
		return 0
	}
	diff := currentRate - d.ImpliedYearEndRate
	if diff <= 0 {
		return 0 // implies hikes or no change
	}
	return diff / 0.25 // each cut is 25bp
}
