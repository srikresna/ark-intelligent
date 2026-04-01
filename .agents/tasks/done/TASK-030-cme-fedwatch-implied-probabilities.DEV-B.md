# TASK-030: CME FedWatch Implied Rate Probabilities via Firecrawl

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service/fed
**Created by:** Research Agent
**Created at:** 2026-04-01 15:xx WIB
**Siklus:** Data (Siklus 2 Putaran 2)

## Deskripsi
Scrape CME FedWatch tool untuk mendapatkan market-implied Fed rate cut/hike probabilities.
Data ini menunjukkan apa yang market pricing: "55% probability rate cut pada FOMC berikutnya".
Saat ini bot tidak punya data ini sama sekali, padahal ini adalah driver USD terbesar.

## Konteks
Bot sudah punya actual policy rates via FRED (FEDFUNDS, SOFR, dll), tapi TIDAK ADA data
market-implied forward rate expectations. Perbedaan antara "actual rate" dan "expected rate"
adalah yang menggerakkan currency: USD naik bukan karena rate tinggi, tapi karena market
mengexpectasi rate lebih tinggi dari yang sekarang dipricing.

Firecrawl key sudah tersedia di .env. CME FedWatch adalah public page.

Alternatif yang lebih simple: FRED series `FEDTARMD` (dot plot median), tapi kurang real-time.
Untuk data benar-benar market-implied: scrape CME FedWatch JSON endpoint.

CME FedWatch juga punya internal JSON API (tidak perlu Firecrawl untuk ini):
`https://www.cmegroup.com/CmeWS/mvc/MboFutEOD/V1/942/EOSB`

Tapi karena endpoint CME bisa berubah dan memerlukan parsing kompleks, pendekatan via
Firecrawl JSON extraction lebih maintainable.

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat `internal/service/fed/fedwatch.go` (atau tambahkan ke `speeches.go` jika sudah ada)
- [ ] Struct `FedWatchData`:
  ```go
  type FedWatchData struct {
      NextMeetingDate    string  // "2026-05-07"
      HoldProbability    float64 // %
      Cut25Probability   float64 // %
      Cut50Probability   float64 // %
      Hike25Probability  float64 // %
      ImpliedYearEndRate float64 // bps implied from Dec futures
      MeetingCount       int     // berapa FOMC meetings sampai Dec
      Available          bool
      FetchedAt          time.Time
  }
  ```
- [ ] Fungsi `FetchFedWatch(ctx context.Context) (*FedWatchData, error)` via Firecrawl JSON extraction
- [ ] Jika FIRECRAWL_API_KEY tidak di-set → return Available=false, no error
- [ ] In-memory cache dengan TTL 4 jam (FOMC probabilities berubah saat ada data/speech baru)
- [ ] `UnifiedOutlookData` di `unified_outlook.go` ditambah field `FedWatchData *fed.FedWatchData`
- [ ] `BuildUnifiedOutlookPrompt()` menampilkan FedWatch di section MACRO (setelah rate differential):
  ```
  CME FedWatch (Next FOMC 2026-05-07): Hold=35% Cut25=55% Cut50=10%
  Market implies 1.5 cuts by year-end
  ```
- [ ] Graceful degradation: jika fetch gagal, log warn + skip section

## File yang Kemungkinan Diubah
- `internal/service/fed/fedwatch.go` (baru)
- `internal/service/ai/unified_outlook.go` (UnifiedOutlookData + prompt section)
- `internal/adapter/telegram/handler.go` (inject ke handler yang build outlook)

## Referensi
- `.agents/research/2026-04-01-15-data-integrasi-siklus2-putaran2.md` (GAP 1)
- `internal/service/sentiment/cboe.go` (contoh pattern Firecrawl JSON extract)
- `internal/service/fed/speeches.go` (jika sudah ada dari TASK-007)
- CME FedWatch: https://www.cmegroup.com/markets/interest-rates/cme-fedwatch-tool.html
