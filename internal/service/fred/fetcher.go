// Package fred provides integration with the FRED (Federal Reserve Economic Data) API.
// FRED is operated by the St. Louis Fed and provides free access to thousands of
// macroeconomic data series via a public REST API.
//
// Free API key available at: https://fred.stlouisfed.org/docs/api/api_key.html
// Set FRED_API_KEY environment variable. Without a key, the API still works for
// basic requests but may be rate-limited.
package fred

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// MacroData holds the latest values for all tracked FRED series.
type MacroData struct {
	// Yield curve
	Yield2Y     float64 // DGS2  — 2-Year Treasury Constant Maturity Rate
	Yield10Y    float64 // DGS10 — 10-Year Treasury Constant Maturity Rate
	YieldSpread float64 // DGS10 - DGS2 (positive = normal, negative = inverted)

	// Inflation
	Breakeven5Y float64 // T10YIE — 5-Year 5-Year Forward Inflation Expectation Rate
	CorePCE     float64 // PCEPILFE — Core PCE Price Index (YoY %)

	// Financial stress
	NFCI float64 // NFCI — National Financial Conditions Index (negative = loose)

	// Labor market
	InitialClaims float64 // ICSA — Initial Jobless Claims (4-week MA in units)

	FetchedAt time.Time
}

// fredResponse is the JSON structure returned by the FRED observations endpoint.
type fredResponse struct {
	Observations []struct {
		Date  string `json:"date"`
		Value string `json:"value"`
	} `json:"observations"`
}

// FetchMacroData fetches the latest values for all tracked series from FRED.
// It makes one HTTP request per series and is resilient to individual failures.
// If FRED_API_KEY is not set, it uses a placeholder (some endpoints work without a key).
func FetchMacroData(ctx context.Context) (*MacroData, error) {
	apiKey := os.Getenv("FRED_API_KEY")
	if apiKey == "" {
		// FRED allows limited access without a key — use empty string
		apiKey = ""
	}

	data := &MacroData{FetchedAt: time.Now()}

	type seriesTarget struct {
		id     string
		target *float64
	}

	targets := []seriesTarget{
		{"DGS2", &data.Yield2Y},
		{"DGS10", &data.Yield10Y},
		{"T10YIE", &data.Breakeven5Y},
		{"PCEPILFE", &data.CorePCE},
		{"NFCI", &data.NFCI},
		{"ICSA", &data.InitialClaims},
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, t := range targets {
		url := buildFREDURL(t.id, apiKey)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			log.Printf("[FRED] failed to build request for %s: %v", t.id, err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[FRED] request failed for %s: %v", t.id, err)
			continue
		}

		var result fredResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if decodeErr != nil {
			log.Printf("[FRED] decode failed for %s: %v", t.id, decodeErr)
			continue
		}

		// Find the latest non-missing observation
		for _, obs := range result.Observations {
			if obs.Value == "." || obs.Value == "" {
				continue // FRED uses "." for missing values
			}
			v, parseErr := strconv.ParseFloat(obs.Value, 64)
			if parseErr != nil {
				continue
			}
			*t.target = v
			break
		}
	}

	// Derived metrics
	data.YieldSpread = data.Yield10Y - data.Yield2Y

	return data, nil
}

// buildFREDURL constructs the FRED API observations URL for a series.
func buildFREDURL(seriesID, apiKey string) string {
	base := fmt.Sprintf(
		"https://api.stlouisfed.org/fred/series/observations?series_id=%s&file_type=json&limit=10&sort_order=desc",
		seriesID,
	)
	if apiKey != "" {
		base += "&api_key=" + apiKey
	}
	return base
}
