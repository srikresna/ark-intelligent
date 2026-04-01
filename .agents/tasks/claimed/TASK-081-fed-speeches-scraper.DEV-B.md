# TASK-081: Fed Speeches & Minutes Scraper via Firecrawl

**Priority:** HIGH
**Siklus:** 2 (Data & Integrasi Gratis)
**Estimasi:** 3-5 jam
**Area:** internal/service/fred/ atau internal/service/news/

---

## Latar Belakang

Federal Reserve mempublikasikan semua pidato, press releases, dan minutes
di federalreserve.gov secara gratis. Data ini sangat relevan untuk:
- Mendeteksi hawkish/dovish shift dalam komunikasi Fed
- Mengidentifikasi topik kebijakan yang disorot (inflation, employment, rates)
- Memperkaya AI prompt untuk /outlook dengan context Fed communication terkini

Firecrawl sudah ada dan berbayar — penggunaan sumber ini tidak menambah biaya.

---

## Tujuan

Buat scraper untuk fed speeches terbaru (30 hari terakhir) dan integrasikan
ke AI prompt builder untuk memperkaya /outlook dan /macro commands.

---

## Implementasi

### 1. Buat `internal/service/fred/speeches.go`

```go
package fred

// FedSpeech represents a single Federal Reserve speech/statement.
type FedSpeech struct {
    Title     string    // Speech title
    Speaker   string    // Speaker name (e.g., "Jerome H. Powell")
    Date      time.Time // Speech date
    Topics    []string  // Extracted keywords: "inflation", "rates", "employment"
    URL       string    // Original URL on federalreserve.gov
    Tone      string    // "HAWKISH", "DOVISH", "NEUTRAL" (AI-classified)
}

type FedSpeechData struct {
    Speeches  []FedSpeech // Last 5 speeches (most recent first)
    Available bool
    FetchedAt time.Time
}

// FetchRecentSpeeches scrapes the Fed speeches listing page via Firecrawl
// and returns the 5 most recent speeches with metadata.
func FetchRecentSpeeches(ctx context.Context) *FedSpeechData
```

Target URL: https://www.federalreserve.gov/newsevents/speeches.htm

Firecrawl schema:
```json
{
  "type": "object",
  "properties": {
    "speeches": {
      "type": "array",
      "maxItems": 5,
      "items": {
        "type": "object",
        "properties": {
          "title": {"type": "string"},
          "speaker": {"type": "string"},
          "date": {"type": "string"},
          "url": {"type": "string"}
        }
      }
    }
  }
}
```

### 2. Tone Classification

Gunakan keyword-based classification (no AI needed):
- HAWKISH: "inflation remains elevated", "further tightening", "higher for longer"
- DOVISH: "labor market softening", "price stability", "rate cuts appropriate"
- Fallback ke AI via Gemini jika konten speech tersedia

### 3. Caching (fred/cache.go pattern)

Cache speeches selama 6 jam (update max 4x/hari).
Hanya re-fetch jika ada speech baru sejak last check.

### 4. Integrasi ke UnifiedOutlookData (ai/unified_outlook.go)

```go
type UnifiedOutlookData struct {
    // ... existing fields ...
    FedSpeeches *fred.FedSpeechData  // NEW
}
```

Tambah section di BuildUnifiedOutlookPrompt():
```
=== N. FED COMMUNICATION (Last 5 speeches) ===
[Date] [Speaker]: [Title] — Tone: HAWKISH
...
→ Overall Fed stance: moderately hawkish, focus on inflation persistence
```

### 5. FOMC Minutes Juga

URL: https://www.federalreserve.gov/monetarypolicy/fomccalendars.htm
Scrape judul + tanggal minutes terbaru, link ke full text.

---

## Testing

- Unit test: TestToneClassification (hawkish/dovish keywords)
- Integration test: FetchRecentSpeeches mengembalikan >0 speeches
- Manual: jalankan /outlook, cek apakah Fed communication section muncul

---

## File yang Dimodifikasi

- `internal/service/fred/speeches.go` (NEW)
- `internal/service/ai/unified_outlook.go` (extend UnifiedOutlookData + prompt)
- `internal/adapter/telegram/handler_macro.go` (tambah Fed speeches ke /macro output)

---

## Referensi

- Pola: `internal/service/sentiment/cboe.go` (Firecrawl JSON extraction)
- DATA_SOURCES_AUDIT.md — "Fed speeches scraper" section
- Research: `.agents/research/2026-04-01-10-data-integrasi-gratis.md`
