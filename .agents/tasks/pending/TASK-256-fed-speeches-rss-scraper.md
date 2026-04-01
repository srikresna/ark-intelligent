# TASK-256: Fed Speeches RSS Scraper — Tambah Retorika Fed ke Unified Outlook

**Priority:** medium
**Type:** data-source
**Estimated:** S
**Area:** internal/service/fed/ (new package), internal/service/ai/unified_outlook.go, internal/adapter/telegram/handler.go
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

Federal Reserve mempublikasikan RSS feed gratis untuk speeches pejabat Fed:
`https://www.federalreserve.gov/feeds/speeches.xml`

Feed ini berisi judul + tanggal speech terbaru dari Powell, Barr, Bowman, Jefferson,
Cook, Waller, dll. Diverifikasi aksesibel (HTTP 200, XML) pada 2026-04-01.

Nilai untuk outlook: judul speech sangat informatif untuk menilai hawkish/dovish tone:
- "Economic Outlook and Monetary Policy" → neutral/dovish direction
- "Prospects for Shrinking the Fed's Balance Sheet" → hawkish/QT
- "Labor Market Data: Signal or Noise?" → data-dependent signaling

AI bisa assess monetary policy tone dari 5-10 judul terbaru tanpa perlu baca full speech.
Tidak butuh Firecrawl — cukup native HTTP GET + `encoding/xml` parsing.

## File yang Harus Dibuat/Diubah

1. `internal/service/fed/speeches.go` — new file: HTTP fetch + XML parse RSS
2. `internal/service/ai/unified_outlook.go` — tambah `FedSpeeches []FedSpeech` field + section
3. `internal/adapter/telegram/handler.go` — fetch speeches saat `/outlook`, inject ke unifiedData

## Implementasi

### 1. internal/service/fed/speeches.go (new file)

```go
package fed

import (
    "context"
    "encoding/xml"
    "fmt"
    "net/http"
    "time"
)

const speechesRSSURL = "https://www.federalreserve.gov/feeds/speeches.xml"

// Speech holds a single Fed speech from the RSS feed.
type Speech struct {
    Title   string    // e.g. "Powell, Economic Outlook and Monetary Policy"
    Date    time.Time // publication date
    Link    string    // URL to full speech
}

// rssRoot models the RSS 2.0 feed structure.
type rssRoot struct {
    XMLName xml.Name `xml:"rss"`
    Channel struct {
        Items []struct {
            Title   string `xml:"title"`
            Link    string `xml:"link"`
            PubDate string `xml:"pubDate"`
        } `xml:"item"`
    } `xml:"channel"`
}

// FetchRecentSpeeches fetches the most recent Fed speeches (up to `limit`).
// Returns empty slice (not error) if fetch fails — caller checks len().
func FetchRecentSpeeches(ctx context.Context, limit int) []Speech {
    client := &http.Client{Timeout: 10 * time.Second}
    req, err := http.NewRequestWithContext(ctx, "GET", speechesRSSURL, nil)
    if err != nil {
        return nil
    }
    req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ArkIntelligent/1.0)")

    resp, err := client.Do(req)
    if err != nil || resp.StatusCode != 200 {
        return nil
    }
    defer resp.Body.Close()

    var feed rssRoot
    if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
        return nil
    }

    speeches := make([]Speech, 0, limit)
    for i, item := range feed.Channel.Items {
        if i >= limit {
            break
        }
        t, _ := time.Parse(time.RFC1123, item.PubDate)
        speeches = append(speeches, Speech{
            Title: item.Title,
            Date:  t,
            Link:  item.Link,
        })
    }
    return speeches
}
```

### 2. unified_outlook.go — tambah field dan section

Di `UnifiedOutlookData` struct:
```go
FedSpeeches []fed.Speech // recent Fed official speeches (from RSS)
```

Di `BuildUnifiedOutlookPrompt()`, tambah section setelah COT atau Macro section:
```go
if len(data.FedSpeeches) > 0 {
    b.WriteString(fmt.Sprintf("=== %d. RECENT FED RHETORIC (Speeches RSS) ===\n", section))
    section++
    for _, s := range data.FedSpeeches {
        dateStr := ""
        if !s.Date.IsZero() {
            dateStr = s.Date.Format("Jan 02")
        }
        b.WriteString(fmt.Sprintf("- [%s] %s\n", dateStr, s.Title))
    }
    b.WriteString("NOTE: Assess hawkish/dovish tone from speech titles above.\n\n")
}
```

### 3. handler.go — fetch saat /outlook

Di fungsi `cmdOutlook` (atau equivalent), sebelum construct `unifiedData`:
```go
fedSpeeches := fed.FetchRecentSpeeches(ctx, 8) // 8 terbaru
```

Inject ke unifiedData:
```go
unifiedData := aisvc.UnifiedOutlookData{
    // ... existing fields ...
    FedSpeeches: fedSpeeches,
}
```

## Acceptance Criteria

- [ ] `internal/service/fed/speeches.go` compile clean
- [ ] `FetchRecentSpeeches(ctx, 8)` returns list speech terbaru dari Fed RSS
- [ ] Jika fetch gagal → return `[]Speech{}`, tidak crash handler
- [ ] `/outlook` prompt includes "RECENT FED RHETORIC" section dengan 5-8 judul
- [ ] Format: `[Mar 15] Powell, Economic Outlook and Monetary Policy`
- [ ] `go build ./...` clean

## Referensi

- `.agents/research/2026-04-02-09-data-sources-audit-gaps-putaran13.md` — Temuan #4
- Verified URL: `https://www.federalreserve.gov/feeds/speeches.xml` (HTTP 200, XML)
- `internal/service/ai/unified_outlook.go:22` — UnifiedOutlookData struct
- `internal/adapter/telegram/handler.go:1004` — unifiedData construction point
- Related: Fed press releases RSS: `https://www.federalreserve.gov/feeds/press_all.xml`
