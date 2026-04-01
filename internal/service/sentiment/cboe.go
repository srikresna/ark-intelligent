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

// CBOEPutCallData holds the latest CBOE put/call ratio data.
type CBOEPutCallData struct {
	TotalPC   float64   // Total Put/Call Ratio
	EquityPC  float64   // Equity-only Put/Call Ratio
	IndexPC   float64   // Index Put/Call Ratio
	Available bool
	FetchedAt time.Time
}

// FetchCBOEPutCall fetches the latest CBOE put/call ratios via Firecrawl.
// The CBOE market statistics page is HTML-only; Firecrawl renders and extracts
// the data using structured JSON extraction.
// If FIRECRAWL_API_KEY is not set, returns unavailable.
func FetchCBOEPutCall(ctx context.Context) *CBOEPutCallData {
	result := &CBOEPutCallData{FetchedAt: time.Now()}

	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("CBOE P/C: skipping — FIRECRAWL_API_KEY not set")
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
			"total_put_call_ratio":  {"type": "number"},
			"equity_put_call_ratio": {"type": "number"},
			"index_put_call_ratio":  {"type": "number"}
		}
	}`)

	reqBody := fcReq{
		URL:     "https://www.cboe.com/us/options/market_statistics/daily/",
		Formats: []string{"json"},
		WaitFor: 5000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract the latest CBOE daily put/call ratios: total put/call ratio, equity put/call ratio, and index put/call ratio. Return as numbers.",
			Schema: schema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Debug().Err(err).Msg("CBOE P/C: failed to marshal Firecrawl request")
		return result
	}

	fcClient := httpclient.New(httpclient.WithTimeout(30 * time.Second))
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(bodyBytes))
	if err != nil {
		log.Debug().Err(err).Msg("CBOE P/C: failed to build Firecrawl request")
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := fcClient.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("CBOE P/C: Firecrawl request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Debug().Int("status", resp.StatusCode).Msg("CBOE P/C: Firecrawl non-2xx response")
		return result
	}

	var fcResp struct {
		Success bool `json:"success"`
		Data    struct {
			JSON struct {
				TotalPC  float64 `json:"total_put_call_ratio"`
				EquityPC float64 `json:"equity_put_call_ratio"`
				IndexPC  float64 `json:"index_put_call_ratio"`
			} `json:"json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
		log.Debug().Err(err).Msg("CBOE P/C: Firecrawl decode failed")
		return result
	}

	if !fcResp.Success {
		log.Debug().Msg("CBOE P/C: Firecrawl returned unsuccessful")
		return result
	}

	d := fcResp.Data.JSON
	result.TotalPC = d.TotalPC
	result.EquityPC = d.EquityPC
	result.IndexPC = d.IndexPC
	if result.TotalPC > 0 {
		result.Available = true
		log.Debug().
			Float64("total", result.TotalPC).
			Float64("equity", result.EquityPC).
			Float64("index", result.IndexPC).
			Msg("CBOE P/C fetched via Firecrawl")
	}

	return result
}

// ClassifyPutCallSignal returns a contrarian interpretation of the put/call ratio.
func ClassifyPutCallSignal(totalPC float64) (signal, description string) {
	switch {
	case totalPC >= 1.2:
		return "EXTREME FEAR", "Very high put buying — strong contrarian bullish signal"
	case totalPC >= 1.0:
		return "FEAR", "Elevated put buying — contrarian bullish"
	case totalPC >= 0.8:
		return "NEUTRAL", "Normal put/call balance"
	case totalPC >= 0.7:
		return "COMPLACENCY", "Low put buying — mild contrarian bearish"
	default:
		return "EXTREME COMPLACENCY", "Very low protection buying — contrarian bearish warning"
	}
}

// IntegratePutCallIntoSentiment merges CBOE put/call data into SentimentData.
func IntegratePutCallIntoSentiment(sd *SentimentData, pc *CBOEPutCallData) {
	if sd == nil || pc == nil || !pc.Available {
		return
	}
	sd.PutCallTotal = pc.TotalPC
	sd.PutCallEquity = pc.EquityPC
	sd.PutCallIndex = pc.IndexPC
	sd.PutCallAvailable = true
	if pc.TotalPC > 0 {
		sd.PutCallSignal, _ = ClassifyPutCallSignal(pc.TotalPC)
	}
}
