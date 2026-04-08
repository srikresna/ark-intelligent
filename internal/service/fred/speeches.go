// Package fred — Fed speeches scraper.
//
// Scrapes the Federal Reserve speeches listing page
// (https://www.federalreserve.gov/newsevents/speeches.htm) via Firecrawl
// JSON extraction. Returns the 5 most recent speeches with keyword-based
// tone classification (HAWKISH / DOVISH / NEUTRAL).
//
// Cache TTL: 6 hours. If FIRECRAWL_API_KEY is not set, returns Available=false
// with no error (graceful degradation).
package fred

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// speechLog is the logger for the speeches scraper.
var speechLog = log.With().Str("component", "fed-speeches").Logger()

// --------------------------------------------------------------------------
// Types
// --------------------------------------------------------------------------

// FedSpeech represents a single Federal Reserve speech or statement.
type FedSpeech struct {
	Title   string    // Speech title
	Speaker string    // Speaker name (e.g., "Jerome H. Powell")
	Date    time.Time // Speech date (UTC)
	Topics  []string  // Extracted keyword topics: "inflation", "rates", "employment"
	URL     string    // Original URL on federalreserve.gov
	Tone    string    // "HAWKISH", "DOVISH", "NEUTRAL"
}

// FedSpeechData is the output of FetchRecentSpeeches.
type FedSpeechData struct {
	Speeches  []FedSpeech // Last 5 speeches, most recent first
	Available bool
	FetchedAt time.Time
}

// --------------------------------------------------------------------------
// Cache
// --------------------------------------------------------------------------

const speechCacheTTL = 6 * time.Hour

var (
	speechCacheMu    sync.RWMutex
	speechCacheData  *FedSpeechData
	speechCacheUntil time.Time
)

// --------------------------------------------------------------------------
// Keyword-based tone classification
// --------------------------------------------------------------------------

var hawkishPhrases = []string{
	"inflation remains elevated",
	"further tightening",
	"higher for longer",
	"restrictive policy",
	"rate hike",
	"above target",
	"upside inflation risks",
	"tighten further",
	"remain vigilant",
	"additional firming",
}

var dovishPhrases = []string{
	"labor market softening",
	"price stability achieved",
	"rate cuts appropriate",
	"easing policy",
	"downside risks",
	"inflation near target",
	"disinflation",
	"rate reduction",
	"accommodation",
	"moderating inflation",
}

// classifyTone returns HAWKISH, DOVISH, or NEUTRAL based on keyword presence
// in the combined title and any available content.
func classifyTone(title string) string {
	lower := strings.ToLower(title)
	hawkScore := 0
	dovishScore := 0
	for _, p := range hawkishPhrases {
		if strings.Contains(lower, p) {
			hawkScore++
		}
	}
	for _, p := range dovishPhrases {
		if strings.Contains(lower, p) {
			dovishScore++
		}
	}
	switch {
	case hawkScore > dovishScore:
		return "HAWKISH"
	case dovishScore > hawkScore:
		return "DOVISH"
	default:
		return "NEUTRAL"
	}
}

// extractTopics returns topic keywords found in the speech title.
func extractTopics(title string) []string {
	lower := strings.ToLower(title)
	topicMap := map[string]string{
		"inflation":     "inflation",
		"employment":    "employment",
		"rate":          "rates",
		"interest":      "rates",
		"economic":      "economy",
		"gdp":           "growth",
		"growth":        "growth",
		"financial":     "financial stability",
		"stability":     "financial stability",
		"banking":       "banking",
		"digital":       "digital assets",
		"crypto":        "digital assets",
		"payment":       "payments",
		"climate":       "climate risk",
		"balance sheet": "balance sheet",
		"quantitative":  "QE/QT",
	}
	seen := map[string]bool{}
	var topics []string
	for keyword, topic := range topicMap {
		if strings.Contains(lower, keyword) && !seen[topic] {
			seen[topic] = true
			topics = append(topics, topic)
		}
	}
	return topics
}

// --------------------------------------------------------------------------
// Firecrawl scraper
// --------------------------------------------------------------------------

const fedSpeechesURL = "https://www.federalreserve.gov/newsevents/speeches.htm"

// FetchRecentSpeeches scrapes the Fed speeches listing page via Firecrawl and
// returns the 5 most recent speeches with metadata. Results are cached for 6h.
// Returns Available=false (no error) when FIRECRAWL_API_KEY is not set.
func FetchRecentSpeeches(ctx context.Context) *FedSpeechData {
	// Cache read
	speechCacheMu.RLock()
	if speechCacheData != nil && time.Now().Before(speechCacheUntil) {
		cached := speechCacheData
		speechCacheMu.RUnlock()
		speechLog.Debug().Msg("returning cached speeches")
		return cached
	}
	speechCacheMu.RUnlock()

	result := &FedSpeechData{FetchedAt: time.Now()}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		speechLog.Debug().Msg("FIRECRAWL_API_KEY not set — skipping Fed speeches fetch")
		return result
	}

	// Firecrawl JSON extraction schema
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"speeches": {
				"type": "array",
				"maxItems": 5,
				"items": {
					"type": "object",
					"properties": {
						"title":   {"type": "string"},
						"speaker": {"type": "string"},
						"date":    {"type": "string"},
						"url":     {"type": "string"}
					},
					"required": ["title"]
				}
			}
		}
	}`)

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

	reqBody := fcReq{
		URL:     fedSpeechesURL,
		Formats: []string{"json"},
		WaitFor: 4000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract the 5 most recent Federal Reserve speeches from the listing. For each speech include: title, speaker name, date (as written), and the relative or absolute URL to the speech page.",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		speechLog.Debug().Err(err).Msg("failed to marshal Firecrawl request")
		return result
	}

	client := httpclient.New(httpclient.WithTimeout(30 * time.Second))
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(bodyBytes))
	if err != nil {
		speechLog.Debug().Err(err).Msg("failed to build Firecrawl request")
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := client.Do(req)
	if err != nil {
		speechLog.Debug().Err(err).Msg("Firecrawl request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		speechLog.Debug().Int("status", resp.StatusCode).Msg("Firecrawl non-2xx response")
		return result
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				Speeches []struct {
					Title   string `json:"title"`
					Speaker string `json:"speaker"`
					Date    string `json:"date"`
					URL     string `json:"url"`
				} `json:"speeches"`
			} `json:"json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		speechLog.Debug().Err(err).Msg("Firecrawl decode failed")
		return result
	}
	if !fcResp.Success {
		speechLog.Debug().Msg("Firecrawl returned unsuccessful")
		return result
	}

	for _, raw := range fcResp.Data.JSON.Speeches {
		if raw.Title == "" {
			continue
		}
		// Normalize URL
		speechURL := raw.URL
		if speechURL != "" && !strings.HasPrefix(speechURL, "http") {
			speechURL = "https://www.federalreserve.gov" + speechURL
		}
		// Parse date (best-effort; Fed uses formats like "April 1, 2026")
		var speechDate time.Time
		if raw.Date != "" {
			for _, layout := range []string{"January 2, 2006", "Jan 2, 2006", "2006-01-02", "01/02/2006"} {
				if t, parseErr := time.Parse(layout, raw.Date); parseErr == nil {
					speechDate = t
					break
				}
			}
		}
		speech := FedSpeech{
			Title:   raw.Title,
			Speaker: raw.Speaker,
			Date:    speechDate,
			Topics:  extractTopics(raw.Title),
			URL:     speechURL,
			Tone:    classifyTone(raw.Title),
		}
		result.Speeches = append(result.Speeches, speech)
	}

	if len(result.Speeches) > 0 {
		result.Available = true
		speechLog.Debug().Int("count", len(result.Speeches)).Msg("Fed speeches fetched via Firecrawl")
	}

	// Cache write
	speechCacheMu.Lock()
	speechCacheData = result
	speechCacheUntil = time.Now().Add(speechCacheTTL)
	speechCacheMu.Unlock()

	return result
}
