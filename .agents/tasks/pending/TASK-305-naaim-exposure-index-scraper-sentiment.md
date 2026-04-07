# TASK-305: NAAIM Exposure Index — Scraper + /sentiment Integration

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service/sentiment/sentiment.go, internal/service/sentiment/naaim.go (baru), internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-04 01:00 WIB
**Siklus:** Data-2 (Siklus 2 Putaran 23)
**Ref:** research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md

## Deskripsi

NAAIM (National Association of Active Investment Managers) Exposure Index adalah survey mingguan yang mengukur rata-rata eksposur ke pasar saham AS dari fund managers aktif. Range: -200% (max short) hingga +200% (max long). Nilai > 80% = bullish berlebihan, < 30% = risk-off.

**Sumber data:** gratis, tidak perlu API key.
**URL page:** `https://www.naaim.org/programs/naaim-exposure-index/`
**Format:** Excel (.xlsx) yang diupdate setiap Kamis, URL berubah tiap minggu.

**Verified data (2026-03-25):**
- NAAIM Mean: 68.52%
- Median: 82%
- Q1 (25th pctile): 31.75%
- Q3 (75th pctile): 100%
- Prev week (2026-03-18): 60.24% → trend naik

**Excel URL pattern:**
`https://naaim.org/wp-content/uploads/YYYY/MM/USE_Data-since-Inception_YYYY-MM-DD.xlsx`
URL berubah tiap minggu — perlu scrape halaman untuk dapat URL terbaru.

## Implementation Plan

### Pendekatan: Firecrawl → extract Excel URL → HTTP GET xlsx → Python/CSV parse

Karena `excelize` **tidak ada di go.mod**, gunakan pendekatan:
1. Firecrawl JSON extraction dari NAAIM page → ambil link Excel terbaru
2. HTTP GET xlsx (direct download, no auth, ~85KB)
3. Parse dengan Python subprocess (`openpyxl` atau `xlrd`) ATAU `encoding/csv` jika NAAIM punya CSV export

**Alternative approach (simpler, recommended):** Firecrawl JSON extraction langsung dari NAAIM page untuk current week data tanpa perlu download Excel:
```
GET https://naaim.org/programs/naaim-exposure-index/
Firecrawl JSON schema: { date, mean_exposure, median_exposure }
```

### Step 1: Cek apakah excelize tersedia

```bash
grep "excelize" go.mod
```

Jika tidak ada, gunakan Firecrawl JSON extraction (tanpa Excel).

### Step 2: Buat `internal/service/sentiment/naaim.go`

```go
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

// NAAIMData holds the latest NAAIM Exposure Index reading.
type NAAIMData struct {
    MeanExposure float64   // % equity exposure (mean of all managers), -200 to +200
    Median       float64   // Median response
    Q1           float64   // 25th percentile
    Q3           float64   // 75th percentile
    WeekDate     string    // Survey week date (e.g. "2026-03-25")
    Available    bool
    FetchedAt    time.Time
}

// fetchNAAIMSentiment fetches NAAIM Exposure Index via Firecrawl JSON extraction.
// Requires FIRECRAWL_API_KEY. Returns struct with Available=false on failure.
func fetchNAAIMSentiment(ctx context.Context, httpClient *http.Client, data *SentimentData) {
    apiKey := os.Getenv("FIRECRAWL_API_KEY")
    if apiKey == "" {
        log.Debug().Msg("NAAIM: skipping — FIRECRAWL_API_KEY not set")
        return
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
            "date":          {"type": "string"},
            "mean_exposure": {"type": "number"},
            "median":        {"type": "number"},
            "q1":            {"type": "number"},
            "q3":            {"type": "number"}
        }
    }`)

    reqBody := fcReq{
        URL:     "https://naaim.org/programs/naaim-exposure-index/",
        Formats: []string{"json"},
        WaitFor: 3000,
        JSONOptions: &fcJSONOpts{
            Prompt: "Extract the most recent NAAIM Exposure Index data: survey date, mean/average exposure percentage, median exposure, Q1 (25th percentile), Q3 (75th percentile). Return numbers as decimals (e.g. 68.52 not 68.52%).",
            Schema: schema,
        },
    }

    bodyBytes, err := json.Marshal(reqBody)
    if err != nil {
        log.Debug().Err(err).Msg("NAAIM: marshal failed")
        return
    }

    req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/scrape", bytes.NewReader(bodyBytes))
    if err != nil {
        log.Debug().Err(err).Msg("NAAIM: request build failed")
        return
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

    resp, err := httpClient.Do(req)
    if err != nil {
        log.Debug().Err(err).Msg("NAAIM: request failed")
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        log.Debug().Int("status", resp.StatusCode).Msg("NAAIM: non-2xx response")
        return
    }

    var fcResp struct {
        Success bool `json:"success"`
        Data    struct {
            JSON struct {
                Date         string  `json:"date"`
                MeanExposure float64 `json:"mean_exposure"`
                Median       float64 `json:"median"`
                Q1           float64 `json:"q1"`
                Q3           float64 `json:"q3"`
            } `json:"json"`
        } `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&fcResp); err != nil {
        log.Debug().Err(err).Msg("NAAIM: decode failed")
        return
    }

    if !fcResp.Success || fcResp.Data.JSON.MeanExposure == 0 {
        log.Debug().Msg("NAAIM: unsuccessful or empty data")
        return
    }

    d := fcResp.Data.JSON
    data.NAAIMMeanExposure = d.MeanExposure
    data.NAAIMMedian       = d.Median
    data.NAAIMG1           = d.Q1
    data.NAAIMG3           = d.Q3
    data.NAAIMWeekDate     = d.Date
    data.NAAIMAvailable    = true

    log.Debug().
        Float64("mean", d.MeanExposure).
        Str("date", d.Date).
        Msg("NAAIM Exposure Index fetched")
}
```

### Step 3: Tambah fields ke SentimentData struct (`sentiment.go`)

```go
// NAAIM Exposure Index (weekly, active investment managers)
NAAIMMeanExposure float64 // Average equity exposure % (-200 to +200)
NAAIMMedian       float64 // Median response
NAAIMG1           float64 // Q1 (25th percentile)
NAAIMG3           float64 // Q3 (75th percentile)
NAAIMWeekDate     string  // Survey week date
NAAIMAvailable    bool
```

### Step 4: Tambah circuit breaker di SentimentFetcher

```go
// Di SentimentFetcher struct:
cbNAAIM *circuitbreaker.Breaker

// Di NewSentimentFetcher():
cbNAAIM: circuitbreaker.New("sentiment-naaim", 3, 5*time.Minute),
```

### Step 5: Panggil di Fetch() setelah AAII

```go
// NAAIM Exposure Index (weekly, professionals)
if err := f.cbNAAIM.Execute(func() error {
    fetchNAAIMSentiment(ctx, f.httpClient, data)
    if !data.NAAIMAvailable && os.Getenv("FIRECRAWL_API_KEY") != "" {
        return fmt.Errorf("NAAIM unavailable")
    }
    return nil
}); err != nil {
    log.Debug().Err(err).Msg("sentiment: NAAIM circuit breaker rejected")
}
```

### Step 6: Tampilkan di formatter.go (`FormatSentiment`)

```go
if data.NAAIMAvailable {
    var signal string
    switch {
    case data.NAAIMMeanExposure > 80:
        signal = "⚠️ Bullish berlebihan (potential top)"
    case data.NAAIMMeanExposure > 60:
        signal = "🟢 Moderat bullish"
    case data.NAAIMMeanExposure > 40:
        signal = "🟡 Netral"
    case data.NAAIMMeanExposure > 20:
        signal = "🟠 Moderat bearish"
    default:
        signal = "🔴 Risk-off (potential bottom)"
    }

    b.WriteString("\n<b>NAAIM Exposure Index</b>\n")
    b.WriteString(fmt.Sprintf("<code>Mean Exp  : %.1f%%</code>\n", data.NAAIMMeanExposure))
    b.WriteString(fmt.Sprintf("<code>Median    : %.1f%%</code>\n", data.NAAIMMedian))
    b.WriteString(fmt.Sprintf("<code>Range     : Q1=%.0f%% Q3=%.0f%%</code>\n", data.NAAIMG1, data.NAAIMG3))
    b.WriteString(fmt.Sprintf("<code>Signal    : %s</code>\n", signal))
    if data.NAAIMWeekDate != "" {
        b.WriteString(fmt.Sprintf("<code>Survei    : minggu %s</code>\n", data.NAAIMWeekDate))
    }
}
```

## Acceptance Criteria

- [ ] `NAAIMData` struct (atau fields di SentimentData) terdefinisi
- [ ] `fetchNAAIMSentiment()` di `naaim.go` berhasil call Firecrawl dan parse response
- [ ] Fields `NAAIMMeanExposure`, `NAAIMWeekDate`, `NAAIMAvailable` diisi di SentimentData
- [ ] Circuit breaker `cbNAAIM` terpasang di `SentimentFetcher`
- [ ] `/sentiment` command menampilkan section NAAIM dengan mean, median, Q1/Q3
- [ ] Ketika FIRECRAWL_API_KEY tidak set → skip gracefully (tidak error)
- [ ] Ketika Firecrawl gagal → Available=false, section tidak muncul (tanpa crash)
- [ ] `go build ./...` clean

## Catatan Implementasi

- NAAIM weekly → cache TTL bisa 24 jam (tidak perlu refetch tiap 6 jam)
- Data sering tersedia mulai Kamis sore ET
- Mean >80: historically predicts 3-6% equity drawdown dalam 4-6 minggu (contrarian signal)
- Cache TTL terpisah atau inherit dari sentiment cache (6h) — either OK karena data weekly

## Referensi

- `internal/service/sentiment/cboe.go` — pattern Firecrawl JSON extraction untuk diikuti
- `internal/service/sentiment/sentiment.go:115` — SentimentData struct
- `internal/service/sentiment/sentiment.go:50` — NewSentimentFetcher (cbNAAIM ditambah di sini)
- NAAIM page: https://naaim.org/programs/naaim-exposure-index/
- research/2026-04-04-01-data-sources-audit-naaim-coingecko-global-sentiment-ux-putaran23.md
