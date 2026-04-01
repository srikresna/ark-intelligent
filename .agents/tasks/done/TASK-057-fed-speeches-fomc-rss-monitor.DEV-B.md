# TASK-057: Fed Speeches & FOMC Press RSS Monitor

**Priority:** 🟢 HIGH  
**Cycle:** Siklus 2 — Data & Integrasi  
**Effort:** ~3-4 jam  
**Assignee:** Dev-A

---

## Context

Federal Reserve menyediakan RSS feed XML gratis, no API key:

1. **Speeches:** `https://www.federalreserve.gov/feeds/speeches.xml`
   - Pidato semua Fed officials (Powell, Vice Chairs, Governors)
   - 15 speeches dalam 2 minggu terakhir
   - Sangat market-moving — Powell speech bisa gerakkan DXY/EURUSD 100+ pips

2. **FOMC Monetary Policy:** `https://www.federalreserve.gov/feeds/press_monetary.xml`
   - FOMC statement releases
   - FOMC minutes releases
   - SEP (dot plot) projections
   - Latest: Mar 18 2026 — FOMC statement

**Verified live:** Keduanya return HTTP 200 dengan XML content.

Saat ini bot tidak memiliki cara programmatic untuk detect/alert Fed speech baru atau FOMC statement release.

---

## Acceptance Criteria

### 1. RSS Parser (`internal/service/news/fed_rss.go`)

```go
type FedSpeech struct {
    Title       string
    Description string    // venue/context
    URL         string
    Category    string    // "Speech" | "Press Release" | "Minutes"
    Speaker     string    // parsed dari Title (Powell, Barr, dll)
    IsVoting    bool      // true jika voting member (Powell, Vice Chair, dll)
    PublishedAt time.Time
}

func FetchFedSpeeches(ctx context.Context) ([]FedSpeech, error)
func FetchFOMCPress(ctx context.Context) ([]FedSpeech, error)
```

Parser harus:
- Parse RSS XML dari kedua URL
- Extract `<title>`, `<description>`, `<link>`, `<pubDate>`
- Parse speaker name dari title (e.g. "Powell, Remarks on..." → Speaker: "Powell")
- Flag `IsVoting` untuk: Powell, Jefferson, Waller, Cook, Kugler, Barr + 4 rotating presidents

### 2. Scheduler Integration

Tambah ke `internal/scheduler/scheduler.go` atau `internal/service/news/scheduler.go`:
- Check setiap 30 menit
- Track `lastSeenGUID` per feed untuk detect speech baru
- Skip jika speech lebih dari 24 jam lalu (avoid flood on restart)

### 3. Alert ke User

Trigger alert jika ada speech baru:
- **Voting member:** Alert HIGH — "🏛️ Fed Alert: Powell berbicara tentang [topik] — [URL]"
- **Non-voting:** Alert MEDIUM — "🏛️ Fed Speech: [Speaker] — [topik]"
- **FOMC Statement:** Alert CRITICAL — "🚨 FOMC Statement Released — [URL]"
- **FOMC Minutes:** Alert HIGH — "📋 FOMC Minutes Released — [URL]"

Format alert (Telegram):
```
🏛️ Fed Speech Alert
Speaker: Jerome Powell
Topic: Economic Outlook and Monetary Policy
Venue: The Brookings Institution
🔗 [Link]
```

### 4. AI Context Enrichment

Inject latest speech titles ke AI context builder (`internal/service/ai/context_builder.go`):
```go
// Latest Fed speeches (last 3, max 5 days old)
if len(recentSpeeches) > 0 {
    b.WriteString("Recent Fed speeches:\n")
    for _, s := range recentSpeeches[:3] {
        b.WriteString(fmt.Sprintf("- %s (%s): %s\n", s.Speaker, s.PublishedAt.Format("Jan 2"), s.Title))
    }
}
```

---

## Files to Create/Edit

- `internal/service/news/fed_rss.go` — NEW: RSS parser + types
- `internal/service/news/scheduler.go` — tambah Fed RSS polling loop
- `internal/service/ai/context_builder.go` — inject Fed speech context
- `internal/adapter/telegram/formatter.go` — FormatFedAlert()

---

## RSS Spec

```xml
<!-- https://www.federalreserve.gov/feeds/speeches.xml -->
<item>
  <title>Barr, Brief Remarks on the Economic Outlook and Monetary Policy</title>
  <link>https://www.federalreserve.gov/newsevents/speech/barr20260326a.htm</link>
  <description>Speech At the Brookings Institution, Washington, D.C.</description>
  <category>Speech</category>
  <pubDate>Thu, 26 Mar 2026 23:10:00 GMT</pubDate>
</item>
```

---

## Notes

- No API key needed — pure XML fetch
- Throttle: max 1 request per feed per 30 menit (cukup untuk real-time)
- Gunakan `encoding/xml` stdlib — no external dependency
- Store last-seen GUIDs di memory (atau BadgerDB jika perlu persist across restart)
- Speech URL bisa di-scrape via Firecrawl untuk konten lengkap (opsional, step 2)
