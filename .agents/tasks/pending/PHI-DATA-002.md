# PHI-DATA-002: Implement Fear & Greed Index

**ID:** PHI-DATA-002  
**Title:** Implement Fear & Greed Index  
**Priority:** MEDIUM  
**Type:** feature  
**Estimated:** M (2-4h)  
**Area:** internal/service  
**Assignee:** 

---

## Deskripsi

Tambah CNN Fear & Greed Index sebagai sentiment indicator. Firecrawl sudah tersedia untuk scraping.

## Konteks

Fear & Greed Index adalah indikator sentiment pasar yang populer (0 = Extreme Fear, 100 = Extreme Greed). Data tersedia gratis di CNN Money dan bisa di-scrape via Firecrawl.

## Acceptance Criteria

- [ ] Buat `internal/service/sentiment/fear_greed.go`
- [ ] Implement fetcher via Firecrawl ke `money.cnn.com/data/fear-and-greed`
- [ ] Parsing: Index value (0-100) + classification (Extreme Fear, Fear, Neutral, Greed, Extreme Greed)
- [ ] Caching: 4 jam TTL di BadgerDB (update intraday)
- [ ] Tambah interface methods di sentiment service
- [ ] Tambah unit test: `fear_greed_test.go` dengan mock response
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean

## Data Structure

```go
type FearGreedIndex struct {
    Value       int       // 0-100
    Classification string // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
    PrevClose   int       // previous day close value
    WeekAgo     int       // value 1 week ago
    MonthAgo    int       // value 1 month ago
    YearAgo     int       // value 1 year ago
    FetchedAt   time.Time
}
```

## Files yang Akan Dibuat/Diubah

- `internal/service/sentiment/fear_greed.go` (baru)
- `internal/service/sentiment/fear_greed_test.go` (baru)
- `internal/service/sentiment/service.go` (modifikasi — tambah method)

## Referensi

- `.agents/DATA_SOURCES_AUDIT.md` — bagian "Peluang Manfaatkan Firecrawl"
- URL target: `https://money.cnn.com/data/fear-and-greed/`

---

## Claim Instructions

1. Pastikan PHI-SETUP-001 sudah selesai
2. Copy file ini ke `.agents/tasks/in-progress/PHI-DATA-002.md`
3. Update field **Assignee** dengan `Dev-C`
4. Update `.agents/STATUS.md`
5. Buat branch: `git checkout -b feat/PHI-002-fear-greed`
6. Implement dan test
7. Setelah selesai, move ke `done/` dan update STATUS.md
