# PHI-DATA-001: Implement AAII Sentiment via Firecrawl

**ID:** PHI-DATA-001  
**Title:** Implement AAII Sentiment via Firecrawl  
**Priority:** MEDIUM  
**Type:** feature  
**Estimated:** M (2-4h)  
**Area:** internal/service  
**Assignee:** 

---

## Deskripsi

Tambah AAII (American Association of Individual Investors) Investor Sentiment sebagai data source baru. Firecrawl API sudah tersedia di environment — tinggal implement scraping dan parsing.

## Konteks

AAII Sentiment adalah indikator sentiment retail investor yang penting. Data tersedia gratis di aaii.com dan bisa di-scrape via Firecrawl yang sudah dibayar. Ini akan melengkapi existing sentiment sources (CBOE VIX).

## Acceptance Criteria

- [ ] Buat `internal/service/sentiment/aaii.go`
- [ ] Implement fetcher via Firecrawl ke `aaii.com/sentimentsurvey/sent_results`
- [ ] Parsing: Bullish, Neutral, Bearish percentages
- [ ] Caching: 24 jam TTL di BadgerDB via sentiment repo
- [ ] Tambah interface methods di sentiment service
- [ ] Tambah unit test: `aaii_test.go` dengan mock Firecrawl response
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

## Data Structure

```go
type AAIISentiment struct {
    Date       time.Time
    Bullish    float64 // percentage
    Neutral    float64 // percentage  
    Bearish    float64 // percentage
    Source     string
    FetchedAt  time.Time
}
```

## Files yang Akan Dibuat/Diubah

- `internal/service/sentiment/aaii.go` (baru)
- `internal/service/sentiment/aaii_test.go` (baru)
- `internal/service/sentiment/service.go` (modifikasi — tambah method)

## Referensi

- `.agents/DATA_SOURCES_AUDIT.md` — bagian "Peluang Manfaatkan Firecrawl"
- URL target: `https://www.aaii.com/sentimentsurvey/sent_results`

---

## Claim Instructions

1. Pastikan PHI-SETUP-001 sudah selesai
2. Copy file ini ke `.agents/tasks/in-progress/PHI-DATA-001.md`
3. Update field **Assignee** dengan `Dev-B`
4. Update `.agents/STATUS.md`
5. Buat branch: `git checkout -b feat/PHI-001-aaii-sentiment`
6. Implement dan test
7. Setelah selesai, move ke `done/` dan update STATUS.md
