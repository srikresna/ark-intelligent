package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/arkcode369/ark-intelligent/internal/domain"
)

// breadthFCSchema is the JSON schema for Firecrawl structured extraction from barchart.com/stocks/market-pulse.
var breadthFCSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"pct_above_50ma":        {"type": "number"},
		"pct_above_200ma":       {"type": "number"},
		"advance_decline_ratio": {"type": "number"},
		"new_52wk_highs":        {"type": "integer"},
		"new_52wk_lows":         {"type": "integer"}
	}
}`)

// FetchMarketBreadth fetches market breadth data from barchart.com/stocks/market-pulse via Firecrawl.
// If FIRECRAWL_API_KEY is not set, returns Available=false with no error.
// Individual parse failures are logged and also return Available=false.
func FetchMarketBreadth(ctx context.Context) (*domain.MarketBreadthData, error) {
	result := &domain.MarketBreadthData{FetchedAt: time.Now()}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("Market Breadth: skipping — FIRECRAWL_API_KEY not set")
		return result, nil
	}

	type fcJSONOptsLocal struct {
		Prompt string          `json:"prompt"`
		Schema json.RawMessage `json:"schema"`
	}
	type fcReqLocal struct {
		URL         string           `json:"url"`
		Formats     []string         `json:"formats"`
		WaitFor     int              `json:"waitFor"`
		JSONOptions *fcJSONOptsLocal `json:"jsonOptions,omitempty"`
	}

	reqBody := fcReqLocal{
		URL:     "https://www.barchart.com/stocks/market-pulse",
		Formats: []string{"json"},
		WaitFor: 5000,
		JSONOptions: &fcJSONOptsLocal{
			Prompt: "Extract the following market breadth statistics for S&P 500 stocks: " +
				"(1) percentage of stocks above 50-day moving average, " +
				"(2) percentage of stocks above 200-day moving average, " +
				"(3) advance/decline ratio (advancing stocks divided by declining stocks), " +
				"(4) number of new 52-week highs, " +
				"(5) number of new 52-week lows. " +
				"Return all values as numbers.",
			Schema: breadthFCSchema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Debug().Err(err).Msg("Market Breadth: failed to marshal Firecrawl request")
		return result, nil
	}

	fcClient := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlScrapeURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Debug().Err(err).Msg("Market Breadth: failed to build Firecrawl request")
		return result, nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("Market Breadth: Firecrawl request failed")
		return result, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Debug().Int("status", resp.StatusCode).Msg("Market Breadth: Firecrawl non-2xx response")
		return result, nil
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				PctAbove50MA        float64 `json:"pct_above_50ma"`
				PctAbove200MA       float64 `json:"pct_above_200ma"`
				AdvanceDeclineRatio float64 `json:"advance_decline_ratio"`
				New52WkHighs        int     `json:"new_52wk_highs"`
				New52WkLows         int     `json:"new_52wk_lows"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		log.Debug().Err(err).Msg("Market Breadth: Firecrawl decode failed")
		return result, nil
	}

	if !fcResp.Success {
		log.Debug().Msg("Market Breadth: Firecrawl returned unsuccessful")
		return result, nil
	}

	d := fcResp.Data.JSON
	// Require at least one MA percentage to consider data valid
	if d.PctAbove50MA == 0 && d.PctAbove200MA == 0 {
		log.Debug().Msg("Market Breadth: Firecrawl returned empty data")
		return result, nil
	}

	result.PctAbove50MA = d.PctAbove50MA
	result.PctAbove200MA = d.PctAbove200MA
	result.AdvanceDeclineRatio = d.AdvanceDeclineRatio
	result.New52WkHighs = d.New52WkHighs
	result.New52WkLows = d.New52WkLows
	result.Available = true

	log.Debug().
		Float64("pct_above_50ma", result.PctAbove50MA).
		Float64("pct_above_200ma", result.PctAbove200MA).
		Float64("adv_dec_ratio", result.AdvanceDeclineRatio).
		Int("new_highs", result.New52WkHighs).
		Int("new_lows", result.New52WkLows).
		Msg("Market Breadth fetched via Firecrawl")

	return result, nil
}
