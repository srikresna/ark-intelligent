# TASK-281: Fed Speeches RSS Parser — Gratis, No Key, No Firecrawl

**Priority:** high
**Type:** feature
**Estimated:** S
**Area:** internal/service/fedspeech/ (baru), internal/service/ai/unified_outlook.go
**Created by:** Research Agent
**Created at:** 2026-04-02 26:00 WIB

## Deskripsi

Federal Reserve menyediakan RSS feed gratis untuk speech gubernur Fed di:
`https://www.federalreserve.gov/feeds/speeches.xml`

Feed ini:
- **Gratis sepenuhnya** — no API key, no Firecrawl, no biaya
- **Plain XML** — bisa di-parse dengan Go `encoding/xml` standard library
- **Update real-time** — setiap kali ada speech baru
- **Berisi:** Speaker, judul speech, tanggal, URL full text

Implementasi parser ini memungkinkan AI di `/outlook` mendapat konteks Fed communication terbaru (hawkish/dovish tone) tanpa bergantung pada web scraping.

**Verifikasi:** Feed tested, aktif berisi speech terbaru:
- "Waller, Labor Market Data: Signal or Noise?"
- "Barr, Brief Remarks on Monetary Policy"
- "Powell, Acceptance Remarks"
- "Bowman, Capital Rules for the Real Economy"

## Perubahan yang Diperlukan

### 1. Buat package baru: `internal/service/fedspeech/`

#### `internal/service/fedspeech/parser.go`

```go
// Package fedspeech parses the Federal Reserve's public speeches RSS feed.
// No API key required. Feed URL: https://www.federalreserve.gov/feeds/speeches.xml
package fedspeech

import (
    "context"
    "encoding/xml"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/arkcode369/ark-intelligent/pkg/logger"
)

var log = logger.Component("fedspeech")

const (
    feedURL    = "https://www.federalreserve.gov/feeds/speeches.xml"
    httpTimeout = 15 * time.Second
    cacheTTL   = 6 * time.Hour
    maxSpeeches = 5 // latest N speeches to include in prompt
)

// Speech represents a single Fed speech from the RSS feed.
type Speech struct {
    Speaker   string    // "Powell", "Waller", "Bowman", etc.
    Title     string    // Speech title
    Date      time.Time // Publication date
    URL       string    // Link to full text
    Tone      string    // "HAWKISH", "DOVISH", "NEUTRAL" — inferred from title
}

// rssChannel represents the RSS XML structure.
type rssFeed struct {
    XMLName xml.Name `xml:"rss"`
    Channel struct {
        Items []rssItem `xml:"channel>item"`
    }
}

type rssItem struct {
    Title   string `xml:"title"`
    Link    string `xml:"link"`
    PubDate string `xml:"pubDate"`
}

// FetchLatestSpeeches fetches the latest Fed speeches from the public RSS feed.
func FetchLatestSpeeches(ctx context.Context) ([]Speech, error) {
    client := &http.Client{Timeout: httpTimeout}
    req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
    if err != nil {
        return nil, fmt.Errorf("fedspeech: build request: %w", err)
    }
    req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ArkIntelligent/1.0)")

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fedspeech: request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("fedspeech: non-200 status: %d", resp.StatusCode)
    }

    var feed rssFeed
    if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
        return nil, fmt.Errorf("fedspeech: xml decode: %w", err)
    }

    var speeches []Speech
    for i, item := range feed.Channel.Items {
        if i >= maxSpeeches {
            break
        }
        s := parseItem(item)
        speeches = append(speeches, s)
    }

    log.Debug().Int("count", len(speeches)).Msg("Fed speeches fetched")
    return speeches, nil
}

// parseItem extracts structured data from an RSS item.
func parseItem(item rssItem) Speech {
    // Title format: "LastName, Speech Title" e.g. "Waller, Labor Market Data"
    speaker, title := splitTitle(item.Title)
    pubDate, _ := time.Parse(time.RFC1123, item.PubDate)
    return Speech{
        Speaker: speaker,
        Title:   title,
        Date:    pubDate,
        URL:     item.Link,
        Tone:    inferTone(title),
    }
}

// splitTitle splits "LastName, Title" into (speaker, title).
func splitTitle(raw string) (string, string) {
    parts := strings.SplitN(raw, ",", 2)
    if len(parts) == 2 {
        return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
    }
    return "Fed Official", raw
}

// inferTone infers hawkish/dovish tone from speech title keywords.
func inferTone(title string) string {
    lower := strings.ToLower(title)
    hawkishKW := []string{"inflation", "price stability", "rate", "restrictive", "tight", "overheating"}
    dovishKW  := []string{"employment", "labor market", "jobs", "cut", "easing", "growth", "recession", "soft landing"}
    for _, kw := range hawkishKW {
        if strings.Contains(lower, kw) {
            return "HAWKISH"
        }
    }
    for _, kw := range dovishKW {
        if strings.Contains(lower, kw) {
            return "DOVISH"
        }
    }
    return "NEUTRAL"
}
```

### 2. Tambah field ke `UnifiedOutlookData`

```go
import "github.com/arkcode369/ark-intelligent/internal/service/fedspeech"

type UnifiedOutlookData struct {
    // ... existing ...
    FedSpeeches []fedspeech.Speech // latest Fed speeches for tone analysis
}
```

### 3. Fetch di handler.go dan tambah ke prompt

Di handler.go (sekitar line 1004):
```go
fedSpeeches, _ := fedspeech.FetchLatestSpeeches(ctx) // best-effort, ignore error
```

Di `BuildUnifiedOutlookPrompt`:
```go
if len(data.FedSpeeches) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. FED COMMUNICATION (Recent Speeches) ===\n", section))
    section++
    for _, s := range data.FedSpeeches {
        b.WriteString(fmt.Sprintf("  [%s] %s — \"%s\" → %s\n",
            s.Date.Format("Jan 2"), s.Speaker, s.Title, s.Tone))
    }
    b.WriteString("\n")
}
```

## File yang Harus Diubah

1. `internal/service/fedspeech/parser.go` — **BARU**, buat dari scratch
2. `internal/service/ai/unified_outlook.go` — tambah field FedSpeeches + prompt section
3. `internal/adapter/telegram/handler.go` — fetch dan inject ke unifiedData

## Verifikasi

```bash
go build ./...
go test ./internal/service/fedspeech/...
# Manual: /outlook → cek apakah section Fed Communication muncul
```

## Acceptance Criteria

- [ ] Package `internal/service/fedspeech/` berisi `FetchLatestSpeeches(ctx)`
- [ ] Parsing XML berhasil — speaker, title, date, URL ter-extract
- [ ] `inferTone()` mengklasifikasikan HAWKISH/DOVISH/NEUTRAL dari keywords judul
- [ ] `UnifiedOutlookData` memiliki field `FedSpeeches []fedspeech.Speech`
- [ ] Handler fetch speeches dan inject ke unifiedData (best-effort)
- [ ] `BuildUnifiedOutlookPrompt` include section Fed Communication
- [ ] `go build ./...` clean, `go test ./...` pass

## Referensi

- `.agents/research/2026-04-02-26-data-sources-audit-putaran18.md` — SOURCE-NEW-1
- Feed URL: `https://www.federalreserve.gov/feeds/speeches.xml` (verified working)
- `internal/service/ai/unified_outlook.go` — UnifiedOutlookData struct
- `internal/adapter/telegram/handler.go:1004` — unified data assembly
