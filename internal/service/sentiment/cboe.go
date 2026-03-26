package sentiment

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// CBOEPutCallData holds the latest CBOE put/call ratio data.
type CBOEPutCallData struct {
	TotalPC   float64   // Total Put/Call Ratio
	EquityPC  float64   // Equity-only Put/Call Ratio
	IndexPC   float64   // Index Put/Call Ratio
	Available bool
	FetchedAt time.Time
}

// FetchCBOEPutCall fetches the latest CBOE put/call ratios.
// Primary source: CBOE market statistics page.
// Fallback: returns unavailable (no hardcoded defaults).
func FetchCBOEPutCall(ctx context.Context) *CBOEPutCallData {
	result := &CBOEPutCallData{FetchedAt: time.Now()}

	client := &http.Client{Timeout: 15 * time.Second}

	// Try CBOE's JSON API endpoint for market statistics.
	// This endpoint provides daily volume data from which P/C can be derived.
	url := "https://www.cboe.com/us/options/market_statistics/daily/"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Debug().Err(err).Msg("CBOE P/C: failed to build request")
		return result
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ARKBot/1.0)")
	req.Header.Set("Accept", "text/html,application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Debug().Err(err).Msg("CBOE P/C: request failed")
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Debug().Int("status", resp.StatusCode).Msg("CBOE P/C: non-200 response")
		return result
	}

	// Try to parse as JSON (some endpoints return JSON).
	var data struct {
		Data []struct {
			TotalPC  string `json:"total_put_call_ratio"`
			EquityPC string `json:"equity_put_call_ratio"`
			IndexPC  string `json:"index_put_call_ratio"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && len(data.Data) > 0 {
		d := data.Data[0]
		if v, err := strconv.ParseFloat(d.TotalPC, 64); err == nil {
			result.TotalPC = v
		}
		if v, err := strconv.ParseFloat(d.EquityPC, 64); err == nil {
			result.EquityPC = v
		}
		if v, err := strconv.ParseFloat(d.IndexPC, 64); err == nil {
			result.IndexPC = v
		}
		if result.TotalPC > 0 {
			result.Available = true
		}
		return result
	}

	// If CBOE direct fetch fails, the data will be populated via
	// manual input or alternative sources in future iterations.
	log.Debug().Msg("CBOE P/C: could not parse response, data unavailable")
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

// IntegratePutCallIntoSentiment merges CBOE data into the main SentimentData.
// Call this after FetchSentiment to add P/C ratios.
// Note: The actual P/C fields live on MacroData (PutCallTotal/Equity/Index),
// so the caller (scheduler/handler) populates those directly from CBOEPutCallData.
func IntegratePutCallIntoSentiment(_ *SentimentData, _ *CBOEPutCallData) {
	// P/C data flows into MacroData.PutCallTotal/PutCallEquity/PutCallIndex,
	// which is populated by the caller. This function is a no-op placeholder
	// for future SentimentData integration if P/C fields are added there.
}
