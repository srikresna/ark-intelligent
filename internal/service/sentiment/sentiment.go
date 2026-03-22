// Package sentiment fetches investor sentiment survey data from public sources.
//
// Currently supported:
//   - CNN Fear & Greed Index (daily, 0-100 scale)
//   - AAII Investor Sentiment Survey (weekly, bull/bear/neutral %)
//
// Neither source offers a stable, documented API, so this package uses
// lightweight HTTP scraping with graceful degradation: if a source is
// unreachable or changes format, the corresponding Available flag is false
// and the rest of the system continues to work.
package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	AAIIAvailable bool

	// CNN Fear & Greed Index
	CNNFearGreed      float64 // 0-100 (0=Extreme Fear, 100=Extreme Greed)
	CNNFearGreedLabel string  // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
	CNNAvailable      bool

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

	// Fetch AAII Sentiment
	fetchAAIISentiment(ctx, client, data)

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
		Score       float64 `json:"score"`
		Rating      string  `json:"rating"`
		Timestamp   string  `json:"timestamp"`
		PreviousClose float64 `json:"previous_close"`
		Previous1Week float64 `json:"previous_1_week"`
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

	data.CNNFearGreed = result.FearAndGreed.Score
	data.CNNFearGreedLabel = normalizeFearGreedLabel(result.FearAndGreed.Rating)
	data.CNNAvailable = true

	log.Debug().
		Float64("score", data.CNNFearGreed).
		Str("label", data.CNNFearGreedLabel).
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
// AAII Investor Sentiment Survey
// ---------------------------------------------------------------------------

// aaiiSentimentURL is the public data endpoint for the AAII weekly survey.
const aaiiSentimentURL = "https://www.aaii.com/sentimentsurvey/sent_results"

func fetchAAIISentiment(ctx context.Context, client *http.Client, data *SentimentData) {
	req, err := http.NewRequestWithContext(ctx, "GET", aaiiSentimentURL, nil)
	if err != nil {
		log.Warn().Err(err).Msg("AAII: failed to build request")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ArkIntelligent/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("AAII: request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Msg("AAII: non-2xx response")
		return
	}

	// AAII does not expose a stable JSON API. The results page returns HTML
	// that we attempt to parse for the three sentiment percentages.
	// If the page structure changes, we gracefully degrade.
	bullish, bearish, neutral, ok := parseAAIIHTML(resp)
	if !ok {
		log.Warn().Msg("AAII: failed to parse sentiment percentages from HTML — endpoint may have changed")
		return
	}

	data.AAIIBullish = bullish
	data.AAIIBearish = bearish
	data.AAIINeutral = neutral
	if bearish > 0 {
		data.AAIIBullBear = bullish / bearish
	}
	data.AAIIAvailable = true

	log.Debug().
		Float64("bullish", bullish).
		Float64("bearish", bearish).
		Float64("neutral", neutral).
		Float64("bull_bear", data.AAIIBullBear).
		Msg("AAII sentiment fetched")
}

// parseAAIIHTML attempts to extract bullish/bearish/neutral percentages
// from the AAII sentiment survey results page.
// Returns (bullish, bearish, neutral, ok).
func parseAAIIHTML(resp *http.Response) (float64, float64, float64, bool) {
	// Read a limited amount of the body to avoid huge allocations.
	buf := make([]byte, 256*1024) // 256 KB should be plenty
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// The AAII page typically contains percentage values near keywords
	// like "Bullish", "Neutral", "Bearish". We look for patterns like:
	//   Bullish  38.0%
	//   Neutral  30.5%
	//   Bearish  31.5%
	bullish := extractPercentNear(body, "Bullish")
	bearish := extractPercentNear(body, "Bearish")
	neutral := extractPercentNear(body, "Neutral")

	if bullish < 0 || bearish < 0 || neutral < 0 {
		return 0, 0, 0, false
	}

	// Sanity check: percentages should roughly sum to ~100
	sum := bullish + bearish + neutral
	if sum < 90 || sum > 110 {
		return 0, 0, 0, false
	}

	return bullish, bearish, neutral, true
}

// extractPercentNear finds a keyword in the body and extracts the nearest
// percentage value (e.g., "38.0" from "Bullish  38.0%").
// Returns -1 if not found.
func extractPercentNear(body, keyword string) float64 {
	idx := strings.Index(body, keyword)
	if idx < 0 {
		// Try case-insensitive
		idx = strings.Index(strings.ToLower(body), strings.ToLower(keyword))
	}
	if idx < 0 {
		return -1
	}

	// Search in a window after the keyword for a number
	window := body[idx:]
	if len(window) > 200 {
		window = window[:200]
	}

	var val float64
	// Try to find a decimal number pattern
	for i := 0; i < len(window); i++ {
		if window[i] >= '0' && window[i] <= '9' {
			n, err := fmt.Sscanf(window[i:], "%f", &val)
			if err == nil && n == 1 && val >= 0 && val <= 100 {
				return val
			}
		}
	}

	return -1
}
