# TASK-284: OpenInsider Cluster Buys Scraper via Firecrawl — Risk Sentiment Signal

**Priority:** medium
**Type:** feature
**Estimated:** M
**Area:** internal/service/sentiment/ (tambah file baru), internal/service/sentiment/sentiment.go
**Created by:** Research Agent
**Created at:** 2026-04-02 26:00 WIB

## Deskripsi

OpenInsider (openinsider.com) menyediakan data cluster insider buying dari SEC Form 4 secara gratis dan publik. "Cluster buy" = multiple insiders dari perusahaan yang sama membeli saham dalam periode singkat — signal smart money bullish.

**Data ini berguna sebagai risk sentiment indicator:**
- Banyak cluster buys dalam seminggu → insiders optimistis → risk-on signal → bullish AUD, NZD, CAD
- Sedikit cluster buys → wait-and-see → risk-off tilting

**Verifikasi:** `https://openinsider.com/latest-cluster-buys` berhasil di-scrape via Firecrawl. Data aktual (2026-04-01): 100 cluster buys, KKR insiders beli $42M, total nilai signifikan.

**Implementasi:** Gunakan Firecrawl (sudah ada key di .env) dengan JSON schema extraction.

## Perubahan yang Diperlukan

### 1. Buat `internal/service/sentiment/openinsider.go`

```go
// Package sentiment — OpenInsider cluster buys scraper via Firecrawl.
// openinsider.com/latest-cluster-buys — public data, no API key required.
// Uses FIRECRAWL_API_KEY for scraping.

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
    ClusterBuyCount   int     // number of cluster buys in the list
    TotalValueUSD     float64 // total $ value of all cluster purchases
    TopBuyers         []InsiderClusterEntry
    Available         bool
    FetchedAt         time.Time
}

type InsiderClusterEntry struct {
    Ticker    string
    Company   string
    Insiders  int
    ValueUSD  float64
}

// openInsiderSchema for Firecrawl JSON extraction.
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
                    "ticker": {"type": "string"},
                    "company": {"type": "string"},
                    "insiders": {"type": "integer"},
                    "value_usd": {"type": "number"}
                }
            }
        }
    }
}`)

// FetchInsiderClusterBuys scrapes OpenInsider cluster buys via Firecrawl.
func FetchInsiderClusterBuys(ctx context.Context) *InsiderClusterData {
    apiKey := os.Getenv("FIRECRAWL_API_KEY")
    if apiKey == "" {
        log.Debug().Msg("OpenInsider: skipping — FIRECRAWL_API_KEY not set")
        return &InsiderClusterData{Available: false}
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

    bodyBytes, _ := json.Marshal(reqBody)
    req, err := http.NewRequestWithContext(ctx, "POST", firecrawlScrapeURL, bytes.NewReader(bodyBytes))
    if err != nil {
        log.Warn().Err(err).Msg("OpenInsider: failed to build request")
        return &InsiderClusterData{Available: false}
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

    fcClient := &http.Client{Timeout: 30 * time.Second}
    resp, err := fcClient.Do(req)
    if err != nil {
        log.Warn().Err(err).Msg("OpenInsider: Firecrawl request failed")
        return &InsiderClusterData{Available: false}
    }
    defer resp.Body.Close()

    // Parse response
    var result struct {
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

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || !result.Success {
        log.Warn().Msg("OpenInsider: decode failed or Firecrawl unsuccessful")
        return &InsiderClusterData{Available: false}
    }

    j := result.Data.JSON
    data := &InsiderClusterData{
        ClusterBuyCount: j.ClusterBuyCount,
        TotalValueUSD:   j.TotalValueUSD,
        Available:       j.ClusterBuyCount > 0,
        FetchedAt:       time.Now(),
    }
    for _, e := range j.TopEntries {
        data.TopBuyers = append(data.TopBuyers, InsiderClusterEntry{
            Ticker:   e.Ticker,
            Company:  e.Company,
            Insiders: e.Insiders,
            ValueUSD: e.ValueUSD,
        })
    }

    log.Debug().
        Int("count", data.ClusterBuyCount).
        Float64("total_usd", data.TotalValueUSD).
        Msg("OpenInsider cluster buys fetched")
    return data
}
```

### 2. Tambah ke `SentimentData` atau `UnifiedOutlookData`

Opsi: tambah ke `SentimentData` struct di `sentiment.go`:
```go
InsiderClusters *InsiderClusterData // OpenInsider cluster buys
```

Dan fetch di `Fetch()` method dengan circuit breaker `cbInsider`.

### 3. Tambah ke prompt di `BuildUnifiedOutlookPrompt`

```go
if sd := data.SentimentData; sd != nil && sd.InsiderClusters != nil && sd.InsiderClusters.Available {
    ic := sd.InsiderClusters
    b.WriteString("Insider Clusters: ")
    b.WriteString(fmt.Sprintf("%d cluster buys (total $%.0fM) — ", ic.ClusterBuyCount, ic.TotalValueUSD/1e6))
    if ic.ClusterBuyCount > 50 {
        b.WriteString("ELEVATED insider buying → risk-on signal\n")
    } else if ic.ClusterBuyCount < 20 {
        b.WriteString("LOW insider buying → caution / risk-off lean\n")
    } else {
        b.WriteString("NORMAL insider activity\n")
    }
}
```

## File yang Harus Diubah

1. `internal/service/sentiment/openinsider.go` — **BARU**, buat dari scratch
2. `internal/service/sentiment/sentiment.go` — tambah `InsiderClusters` ke `SentimentData` + fetch di `Fetch()`
3. `internal/service/ai/unified_outlook.go` — tambah InsiderClusters ke prompt output

## Verifikasi

```bash
go build ./...
go test ./internal/service/sentiment/...
# Manual: /sentiment → cek InsiderClusters muncul
# Manual: /outlook → cek insider signal masuk ke prompt
```

## Acceptance Criteria

- [ ] `internal/service/sentiment/openinsider.go` baru berisi `FetchInsiderClusterBuys()`
- [ ] `SentimentData` memiliki field `InsiderClusters *InsiderClusterData`
- [ ] InsiderClusters di-fetch di `SentimentFetcher.Fetch()` dengan circuit breaker
- [ ] Gracefully skip jika FIRECRAWL_API_KEY tidak ada
- [ ] `BuildUnifiedOutlookPrompt` include InsiderClusters sebagai risk sentiment context
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-26-data-sources-audit-putaran18.md` — SOURCE-NEW-2
- `https://openinsider.com/latest-cluster-buys` — verified via Firecrawl
- `internal/service/sentiment/sentiment.go` — SentimentFetcher, SentimentData, Firecrawl pattern
- `internal/service/sentiment/cboe.go` — contoh Firecrawl scraping pattern di sentiment package
