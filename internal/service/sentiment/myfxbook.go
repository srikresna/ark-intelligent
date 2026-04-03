package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/arkcode369/ark-intelligent/pkg/httpclient"
)

// MyfxbookPairSentiment holds retail positioning for a single forex pair.
type MyfxbookPairSentiment struct {
	Symbol   string  // e.g. "EURUSD"
	LongPct  float64 // e.g. 32.5 (%)
	ShortPct float64 // e.g. 67.5 (%)
	Signal   string  // "CONTRARIAN_BULLISH", "CONTRARIAN_BEARISH", "LEAN_BULLISH", "LEAN_BEARISH", "NEUTRAL"
}

// MyfxbookData holds retail positioning data from Myfxbook Community Outlook.
type MyfxbookData struct {
	Pairs     []MyfxbookPairSentiment
	Available bool
	FetchedAt time.Time
}

// ClassifyRetailSignal returns a contrarian interpretation of the retail long %.
func ClassifyRetailSignal(longPct float64) string {
	switch {
	case longPct <= 20:
		return "CONTRARIAN_BULLISH" // retail 80%+ short → bull
	case longPct <= 30:
		return "LEAN_BULLISH"
	case longPct >= 80:
		return "CONTRARIAN_BEARISH" // retail 80%+ long → bear
	case longPct >= 70:
		return "LEAN_BEARISH"
	default:
		return "NEUTRAL"
	}
}

// FetchMyfxbook scrapes Myfxbook Community Outlook via Firecrawl JSON extraction.
// If FIRECRAWL_API_KEY is not set, returns unavailable (Available=false).
func FetchMyfxbook(ctx context.Context) *MyfxbookData {
	result := &MyfxbookData{FetchedAt: time.Now()}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("Myfxbook: skipping — FIRECRAWL_API_KEY not set")
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
			"pairs": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"symbol":    {"type": "string"},
						"long_pct":  {"type": "number"},
						"short_pct": {"type": "number"}
					}
				}
			}
		}
	}`)

	reqBody := fcReq{
		URL:     "https://www.myfxbook.com/community/outlook",
		Formats: []string{"json"},
		WaitFor: 5000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract retail trader positioning from Myfxbook Community Outlook. For each currency pair shown, extract the symbol (e.g. EURUSD), long percentage, and short percentage. Return all pairs visible on the page.",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Debug().Err(err).Msg("Myfxbook: failed to marshal Firecrawl request")
		return result
	}

	fcClient := httpclient.New(httpclient.WithTimeout(45 * time.Second))
	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlScrapeURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Debug().Err(err).Msg("Myfxbook: failed to build Firecrawl request")
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("Myfxbook: Firecrawl request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Debug().Int("status", resp.StatusCode).Msg("Myfxbook: Firecrawl non-2xx response")
		return result
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				Pairs []struct {
					Symbol   string  `json:"symbol"`
					LongPct  float64 `json:"long_pct"`
					ShortPct float64 `json:"short_pct"`
				} `json:"pairs"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		log.Debug().Err(err).Msg("Myfxbook: Firecrawl decode failed")
		return result
	}

	if !fcResp.Success {
		log.Debug().Msg("Myfxbook: Firecrawl returned unsuccessful")
		return result
	}

	for _, p := range fcResp.Data.JSON.Pairs {
		if p.Symbol == "" || (p.LongPct == 0 && p.ShortPct == 0) {
			continue
		}
		pair := MyfxbookPairSentiment{
			Symbol:   p.Symbol,
			LongPct:  p.LongPct,
			ShortPct: p.ShortPct,
			Signal:   ClassifyRetailSignal(p.LongPct),
		}
		result.Pairs = append(result.Pairs, pair)
	}

	if len(result.Pairs) > 0 {
		result.Available = true
		log.Debug().Int("pairs", len(result.Pairs)).Msg("Myfxbook retail positioning fetched via Firecrawl")
	}

	return result
}

// IntegrateMyfxbookIntoSentiment merges Myfxbook data into SentimentData.
func IntegrateMyfxbookIntoSentiment(sd *SentimentData, mfx *MyfxbookData) {
	if sd == nil || mfx == nil || !mfx.Available {
		return
	}
	sd.MyfxbookPairs = mfx.Pairs
	sd.MyfxbookAvailable = true
}
