// Package sentiment fetches investor sentiment survey data from public sources.
//
// Currently supported:
//   - CNN Fear & Greed Index (daily, 0-100 scale)
//   - AAII Investor Sentiment Survey (weekly, bull/bear/neutral % via Firecrawl)
//
// CNN uses a public JSON endpoint. AAII is behind Imperva bot protection and
// requires Firecrawl API to scrape. If FIRECRAWL_API_KEY is not set, AAII is
// skipped gracefully. Each source has an Available flag for callers to check.
package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("sentiment")

// SentimentData holds the latest readings from all sentiment sources.
type SentimentData struct {
	// AAII Investor Sentiment Survey
	AAIIBullish   float64 // % bullish
	AAIIBearish   float64 // % bearish
	AAIINeutral   float64 // % neutral
	AAIIBullBear  float64 // Bull/Bear ratio (>1 = bullish sentiment)
	AAIIWeekDate  string  // Survey week ending date (e.g. "3/18/2026")
	AAIIAvailable bool

	// CNN Fear & Greed Index
	CNNFearGreed      float64 // 0-100 (0=Extreme Fear, 100=Extreme Greed)
	CNNFearGreedLabel string  // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
	CNNPrevClose      float64 // Previous trading day close score
	CNNPrev1Week      float64 // Score 1 week ago
	CNNPrev1Month     float64 // Score 1 month ago
	CNNPrev1Year      float64 // Score 1 year ago
	CNNAvailable      bool

	// CBOE Put/Call Ratios
	PutCallTotal   float64 // Total Put/Call Ratio
	PutCallEquity  float64 // Equity Put/Call Ratio
	PutCallIndex   float64 // Index Put/Call Ratio
	PutCallSignal  string  // "EXTREME FEAR", "FEAR", "NEUTRAL", "COMPLACENCY", "EXTREME COMPLACENCY"
	PutCallAvailable bool

	// Crypto Fear & Greed Index (alternative.me)
	CryptoFearGreed          float64 // 0-100 (0=Extreme Fear, 100=Extreme Greed)
	CryptoFearGreedLabel     string  // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
	CryptoFearGreedAvailable bool

	FetchedAt time.Time
}

// FetchSentiment fetches sentiment data from all supported sources.
// Individual source failures are logged but do not cause an overall error;
// callers should check the Available flags on the returned data.
func FetchSentiment(ctx context.Context) (*SentimentData, error) {
	data := &SentimentData{FetchedAt: time.Now()}
	client := &http.Client{Timeout: 15 * time.Second}

	// Fetch CNN Fear & Greed
	fetchCNNFearGreed(ctx, client, data)

	// Fetch AAII Sentiment (via Firecrawl if API key available)
	fetchAAIISentiment(ctx, client, data)

	// Fetch CBOE Put/Call Ratios (via Firecrawl if API key available)
	pcData := FetchCBOEPutCall(ctx)
	IntegratePutCallIntoSentiment(data, pcData)

	// Fetch Crypto Fear & Greed Index (alternative.me — no API key needed)
	fetchCryptoFearGreed(ctx, client, data)

	return data, nil
}

// ---------------------------------------------------------------------------
// CNN Fear & Greed Index
// ---------------------------------------------------------------------------

// cnnFearGreedURL is the public JSON endpoint for the CNN Fear & Greed data.
const cnnFearGreedURL = "https://production.dataviz.cnn.io/index/fearandgreed/graphdata"

// cnnResponse models the relevant portion of the CNN Fear & Greed JSON response.
type cnnResponse struct {
	FearAndGreed struct {
		Score          float64 `json:"score"`
		Rating         string  `json:"rating"`
		Timestamp      string  `json:"timestamp"`
		PreviousClose  float64 `json:"previous_close"`
		Previous1Week  float64 `json:"previous_1_week"`
		Previous1Month float64 `json:"previous_1_month"`
		Previous1Year  float64 `json:"previous_1_year"`
	} `json:"fear_and_greed"`
}

func fetchCNNFearGreed(ctx context.Context, client *http.Client, data *SentimentData) {
	req, err := http.NewRequestWithContext(ctx, "GET", cnnFearGreedURL, nil)
	if err != nil {
		log.Warn().Err(err).Msg("CNN F&G: failed to build request")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ArkIntelligent/1.0)")
	req.Header.Set("Referer", "https://www.cnn.com/markets/fear-and-greed")

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("CNN F&G: request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Msg("CNN F&G: non-2xx response")
		return
	}

	var result cnnResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Warn().Err(err).Msg("CNN F&G: decode failed")
		return
	}

	fg := result.FearAndGreed
	data.CNNFearGreed = fg.Score
	data.CNNFearGreedLabel = normalizeFearGreedLabel(fg.Rating)
	data.CNNPrevClose = fg.PreviousClose
	data.CNNPrev1Week = fg.Previous1Week
	data.CNNPrev1Month = fg.Previous1Month
	data.CNNPrev1Year = fg.Previous1Year
	data.CNNAvailable = true

	log.Debug().
		Float64("score", data.CNNFearGreed).
		Str("label", data.CNNFearGreedLabel).
		Float64("prev_week", data.CNNPrev1Week).
		Float64("prev_month", data.CNNPrev1Month).
		Msg("CNN F&G fetched")
}

// normalizeFearGreedLabel normalizes the CNN rating string to a display label.
func normalizeFearGreedLabel(rating string) string {
	switch strings.ToLower(strings.TrimSpace(rating)) {
	case "extreme fear":
		return "Extreme Fear"
	case "fear":
		return "Fear"
	case "neutral":
		return "Neutral"
	case "greed":
		return "Greed"
	case "extreme greed":
		return "Extreme Greed"
	default:
		if rating != "" {
			return rating
		}
		return "Unknown"
	}
}

// ---------------------------------------------------------------------------
// AAII Investor Sentiment Survey (via Firecrawl)
// ---------------------------------------------------------------------------

// firecrawlScrapeURL is the Firecrawl v1 scrape endpoint.
const firecrawlScrapeURL = "https://api.firecrawl.dev/v1/scrape"

// aaiiFCRequest is the Firecrawl scrape request body for AAII.
type aaiiFCRequest struct {
	URL         string       `json:"url"`
	Formats     []string     `json:"formats"`
	WaitFor     int          `json:"waitFor"`
	JSONOptions *fcJSONOpts  `json:"jsonOptions,omitempty"`
}

type fcJSONOpts struct {
	Prompt string          `json:"prompt"`
	Schema json.RawMessage `json:"schema"`
}

// aaiiFCResponse models the Firecrawl scrape response for AAII data.
type aaiiFCResponse struct {
	Success bool `json:"success"`
	Data    struct {
		JSON struct {
			LatestWeek  string  `json:"latest_week"`
			BullishPct  float64 `json:"bullish_pct"`
			NeutralPct  float64 `json:"neutral_pct"`
			BearishPct  float64 `json:"bearish_pct"`
		} `json:"json"`
	} `json:"data"`
}

// aaiiFCSchema is the JSON schema for Firecrawl structured extraction.
var aaiiFCSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"latest_week":  {"type": "string"},
		"bullish_pct":  {"type": "number"},
		"neutral_pct":  {"type": "number"},
		"bearish_pct":  {"type": "number"}
	}
}`)

func fetchAAIISentiment(ctx context.Context, client *http.Client, data *SentimentData) {
	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("AAII: skipping — FIRECRAWL_API_KEY not set")
		data.AAIIAvailable = false
		return
	}

	reqBody := aaiiFCRequest{
		URL:     "https://www.aaii.com/sentimentsurvey",
		Formats: []string{"json"},
		WaitFor: 5000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract the latest AAII sentiment survey data: latest week ending date, bullish %, neutral %, and bearish %.",
			Schema: aaiiFCSchema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Warn().Err(err).Msg("AAII: failed to marshal Firecrawl request")
		data.AAIIAvailable = false
		return
	}

	// Use a longer timeout for Firecrawl (it needs to render the page)
	fcClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlScrapeURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Warn().Err(err).Msg("AAII: failed to build Firecrawl request")
		data.AAIIAvailable = false
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("AAII: Firecrawl request failed")
		data.AAIIAvailable = false
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Msg("AAII: Firecrawl non-2xx response")
		data.AAIIAvailable = false
		return
	}

	var result aaiiFCResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Warn().Err(err).Msg("AAII: Firecrawl decode failed")
		data.AAIIAvailable = false
		return
	}

	if !result.Success || result.Data.JSON.BullishPct == 0 {
		log.Warn().Msg("AAII: Firecrawl returned empty or failed result")
		data.AAIIAvailable = false
		return
	}

	j := result.Data.JSON
	data.AAIIBullish = j.BullishPct
	data.AAIINeutral = j.NeutralPct
	data.AAIIBearish = j.BearishPct
	data.AAIIWeekDate = j.LatestWeek
	if j.BearishPct > 0 {
		data.AAIIBullBear = j.BullishPct / j.BearishPct
	}
	data.AAIIAvailable = true

	log.Debug().
		Float64("bullish", data.AAIIBullish).
		Float64("bearish", data.AAIIBearish).
		Float64("neutral", data.AAIINeutral).
		Str("week", data.AAIIWeekDate).
		Msg("AAII fetched via Firecrawl")
}

// ---------------------------------------------------------------------------
// Crypto Fear & Greed Index (alternative.me — no API key required)
// ---------------------------------------------------------------------------

// cryptoFGURL is the alternative.me public JSON endpoint.
const cryptoFGURL = "https://api.alternative.me/fng/?limit=2"

// cryptoFGResponse models the alternative.me Fear & Greed API response.
type cryptoFGResponse struct {
	Name string `json:"name"`
	Data []struct {
		Value               string `json:"value"`
		ValueClassification string `json:"value_classification"`
		Timestamp           string `json:"timestamp"`
	} `json:"data"`
}

// fetchCryptoFearGreed fetches the Crypto Fear & Greed Index from alternative.me.
// Scale: 0-24 Extreme Fear, 25-44 Fear, 45-55 Neutral, 56-74 Greed, 75-100 Extreme Greed.
func fetchCryptoFearGreed(ctx context.Context, client *http.Client, data *SentimentData) {
	req, err := http.NewRequestWithContext(ctx, "GET", cryptoFGURL, nil)
	if err != nil {
		log.Warn().Err(err).Msg("Crypto F&G: failed to build request")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ArkIntelligent/1.0)")

	fgClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := fgClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("Crypto F&G: request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Msg("Crypto F&G: non-2xx response")
		return
	}

	var result cryptoFGResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Warn().Err(err).Msg("Crypto F&G: decode failed")
		return
	}

	if len(result.Data) == 0 || result.Data[0].Value == "" {
		log.Warn().Msg("Crypto F&G: empty data from alternative.me")
		return
	}

	var score float64
	if _, err := fmt.Sscanf(result.Data[0].Value, "%f", &score); err != nil {
		log.Warn().Err(err).Str("raw", result.Data[0].Value).Msg("Crypto F&G: failed to parse score")
		return
	}

	data.CryptoFearGreed = score
	data.CryptoFearGreedLabel = normalizeFearGreedLabel(result.Data[0].ValueClassification)
	data.CryptoFearGreedAvailable = true

	log.Debug().
		Float64("score", data.CryptoFearGreed).
		Str("label", data.CryptoFearGreedLabel).
		Msg("Crypto F&G fetched from alternative.me")
}
