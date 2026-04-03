// Package sentiment — OpenInsider cluster buys scraper via Firecrawl.
// openinsider.com/latest-cluster-buys — public data, no registration required.
// Uses FIRECRAWL_API_KEY for extraction. Returns Available=false when key absent.
package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// InsiderClusterData holds aggregated OpenInsider cluster buy statistics.
type InsiderClusterData struct {
	ClusterBuyCount int                   // number of cluster buys in the list
	TotalValueUSD   float64               // total $ value of all cluster purchases
	TopBuyers       []InsiderClusterEntry // top entries (up to 5)
	Available       bool
	FetchedAt       time.Time
}

// InsiderClusterEntry is a single cluster-buy entry from OpenInsider.
type InsiderClusterEntry struct {
	Ticker   string  // ticker symbol
	Company  string  // company name
	Insiders int     // number of insiders buying
	ValueUSD float64 // total purchase value in USD
}

// openInsiderSchema is the Firecrawl JSON extraction schema for OpenInsider.
var openInsiderSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"cluster_buy_count": {"type": "integer"},
		"total_value_usd": {"type": "number"},
		"top_entries": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"ticker":   {"type": "string"},
					"company":  {"type": "string"},
					"insiders": {"type": "integer"},
					"value_usd": {"type": "number"}
				}
			}
		}
	}
}`)

// openInsiderFCResponse models the Firecrawl scrape response for OpenInsider.
type openInsiderFCResponse struct {
	Success bool `json:"success"`
	Data    struct {
		JSON struct {
			ClusterBuyCount int     `json:"cluster_buy_count"`
			TotalValueUSD   float64 `json:"total_value_usd"`
			TopEntries      []struct {
				Ticker   string  `json:"ticker"`
				Company  string  `json:"company"`
				Insiders int     `json:"insiders"`
				ValueUSD float64 `json:"value_usd"`
			} `json:"top_entries"`
		} `json:"json"`
	} `json:"data"`
}

// fetchInsiderClusterBuys scrapes OpenInsider cluster buys via Firecrawl
// and populates InsiderClusters on the SentimentData.
func fetchInsiderClusterBuys(ctx context.Context, client *http.Client, data *SentimentData) {
	apiKey := os.Getenv("FIRECRAWL_API_KEY")
	if apiKey == "" {
		log.Debug().Msg("OpenInsider: skipping — FIRECRAWL_API_KEY not set")
		return
	}

	reqBody := aaiiFCRequest{
		URL:     "https://openinsider.com/latest-cluster-buys",
		Formats: []string{"json"},
		WaitFor: 3000,
		JSONOptions: &fcJSONOpts{
			Prompt: "Extract: total number of cluster buy entries in the table, total dollar value of all purchases, and the top 5 entries with ticker symbol, company name, number of insiders buying, and value in USD.",
			Schema: openInsiderSchema,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Warn().Err(err).Msg("OpenInsider: marshal request failed")
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", firecrawlScrapeURL, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Warn().Err(err).Msg("OpenInsider: build request failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("OpenInsider: HTTP request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Msg("OpenInsider: non-200 response")
		return
	}

	var result openInsiderFCResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || !result.Success {
		log.Warn().Err(err).Bool("success", result.Success).Msg("OpenInsider: decode failed or Firecrawl unsuccessful")
		return
	}

	j := result.Data.JSON
	ic := &InsiderClusterData{
		ClusterBuyCount: j.ClusterBuyCount,
		TotalValueUSD:   j.TotalValueUSD,
		Available:       j.ClusterBuyCount > 0,
		FetchedAt:       time.Now(),
	}
	for _, e := range j.TopEntries {
		ic.TopBuyers = append(ic.TopBuyers, InsiderClusterEntry{
			Ticker:   e.Ticker,
			Company:  e.Company,
			Insiders: e.Insiders,
			ValueUSD: e.ValueUSD,
		})
	}

	data.InsiderClusters = ic
	log.Debug().
		Int("count", ic.ClusterBuyCount).
		Float64("total_usd", ic.TotalValueUSD).
		Msg("OpenInsider cluster buys fetched")
}
