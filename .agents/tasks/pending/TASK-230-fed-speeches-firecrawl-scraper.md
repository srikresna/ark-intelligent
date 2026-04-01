# TASK-230: Fed Speeches Scraper via Firecrawl → AI Outlook Context

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service/fedspeech/ (new), internal/service/ai/unified_outlook.go
**Created by:** Research Agent
**Created at:** 2026-04-02 19:00 WIB

## Deskripsi

Federal Reserve speeches di `federalreserve.gov/newsevents/speeches-testimony.htm` **dapat di-scrape via Firecrawl** (confirmed live, 2026-04-02). Mengembalikan speaker, role, date, speech title, dan URL per speech.

Saat ini AI unified_outlook.go ada komentar `"Look up recent central bank speeches"` di prompt tapi tidak ada implementasi actual. Task ini mengimplementasikan scraper dan feed hasilnya ke `/outlook` sebagai context "Central Bank Tone".

## Verified Live Data (2026-04-02):

Firecrawl berhasil extract:
- Jerome H. Powell (Chair) — 3/21/2026 — "Acceptance Remarks"
- Philip N. Jefferson (Vice Chair) — 3/26/2026 — "Economic Outlook and Energy Effects"  
- Christopher J. Waller (Governor) — 2/24/2026 — "Labor Market Data: Signal or Noise?"

## File yang Harus Dibuat/Diubah

- `internal/service/fedspeech/fetcher.go` — NEW: Firecrawl scraper untuk speech list
- `internal/service/fedspeech/models.go` — NEW: FedSpeech struct (Speaker, Role, Date, Title, URL, IsPriorityVoter)
- `internal/service/fedspeech/cache.go` — NEW: 1-jam cache (speeches tidak update sering)
- `internal/service/ai/unified_outlook.go` — Tambah FedSpeechData ke UnifiedOutlookInput
- `internal/adapter/telegram/handler.go` — Pass FedSpeechData ke cmdOutlook

## Implementasi

### fetcher.go — Firecrawl request

```go
const fedSpeechListURL = "https://www.federalreserve.gov/newsevents/speeches-testimony.htm"
const firecrawlScrapeURL = "https://api.firecrawl.dev/v1/scrape"

// FetchSpeechList fetches latest Fed speeches via Firecrawl structured extraction.
// Returns up to 10 most recent speeches. Requires FIRECRAWL_API_KEY.
func FetchSpeechList(ctx context.Context) ([]FedSpeech, error)
```

Schema untuk Firecrawl:
```json
{
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "speaker_name": {"type": "string"},
      "title_role": {"type": "string"},
      "date": {"type": "string"},
      "speech_title": {"type": "string"},
      "url": {"type": "string"}
    }
  }
}
```

### models.go — Priority voter flag

```go
// priorityVoters = FOMC voting members whose speeches are market-moving
var priorityVoters = map[string]bool{
    "Jerome H. Powell":     true, // Chair
    "Philip N. Jefferson":  true, // Vice Chair
    "Michelle W. Bowman":   true, // Vice Chair for Supervision
    "Christopher J. Waller": true,
    "Lisa D. Cook":         true,
}
```

### unified_outlook.go — Context injection

Tambah ke AI prompt:
```
## Recent Fed Communications
[Untuk setiap speech dalam 14 hari: speaker (role) — date — title]
→ Assess tone: hawkish / dovish / neutral. Notable themes.
```

## Acceptance Criteria

- [ ] `FetchSpeechList` berhasil scrape `speeches-testimony.htm` via Firecrawl
- [ ] Mengembalikan minimal 5 speeches terbaru dengan speaker, role, date, title
- [ ] `IsPriorityVoter` flag benar untuk Chair dan voting members
- [ ] Cache 1 jam berjalan (tidak call Firecrawl setiap `/outlook`)
- [ ] /outlook AI prompt berisi context Fed speeches terbaru
- [ ] Jika FIRECRAWL_API_KEY tidak ada → skip gracefully tanpa error
- [ ] Unit test: `TestFedSpeechParseResponse` memverifikasi parsing

## Referensi

- `.agents/research/2026-04-02-19-data-fed-speeches-cg-trending-edgar-form4-putaran8.md` — Temuan 1
- `internal/service/sentiment/sentiment.go:fetchAAIISentiment` — Pola Firecrawl yang sudah ada
- `internal/service/ai/unified_outlook.go` — Komentar "Look up recent central bank speeches" (baris ~perlu dicari)
- `internal/service/ai/prompts.go` — "Central Bank Watch" section sudah ada tapi data tidak diisi
