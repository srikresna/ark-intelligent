# TASK-066: Add Circuit Breaker ke Sentiment Service

**Priority:** HIGH  
**Type:** Tech Refactor / Reliability  
**Ref:** NEW FINDING — tidak ada di TECH_REFACTOR_PLAN.md  
**Branch target:** dev-b  
**Estimated size:** Small (50-80 LOC)

---

## Problem

`internal/service/sentiment/sentiment.go` memanggil 3 external endpoints tanpa circuit breaker:
- CNN Fear & Greed API
- AAII Sentiment Survey (via Firecrawl)
- CBOE Put/Call data

Jika salah satu endpoint down atau lambat, setiap user yang request `/sentiment` akan menunggu full timeout (default HTTP 10-30 detik) sebelum error. Tidak ada fast-fail.

Semua service lain sudah punya circuit breaker:
- COT: `cbSocrata`, `cbCSV`
- Price: `cbTwelveData`, `cbAlphaVantage`, `cbYahoo`, `cbCoinGecko`
- News: `cb` (mql5)

---

## Solution

Buat `SentimentFetcher` struct dengan 3 circuit breakers (pattern sama dengan `internal/service/news/fetcher.go`):

```go
type SentimentFetcher struct {
    httpClient *http.Client
    cbCNN      *circuitbreaker.Breaker
    cbAAII     *circuitbreaker.Breaker
    cbCBOE     *circuitbreaker.Breaker
}

func NewSentimentFetcher() *SentimentFetcher {
    return &SentimentFetcher{
        httpClient: &http.Client{Timeout: 15 * time.Second},
        cbCNN:  circuitbreaker.New("sentiment-cnn", 3, 5*time.Minute),
        cbAAII: circuitbreaker.New("sentiment-aaii", 3, 5*time.Minute),
        cbCBOE: circuitbreaker.New("sentiment-cboe", 3, 5*time.Minute),
    }
}
```

Wrap setiap fetch call:
```go
err := f.cbCNN.Execute(func() error {
    fetchCNNFearGreed(ctx, f.httpClient, data)
    return nil
})
```

---

## Implementation Steps

1. Ubah `FetchSentiment(ctx)` (package-level function) menjadi method `(f *SentimentFetcher) Fetch(ctx) (*SentimentData, error)`
2. Buat constructor `NewSentimentFetcher()` seperti contoh di atas
3. Update wiring di `internal/adapter/telegram/bot.go` — ganti `sentiment.FetchSentiment(ctx)` dengan `sentimentFetcher.Fetch(ctx)`
4. Update semua caller di `handler.go` atau `scheduler.go` yang memanggil `sentiment.FetchSentiment`

---

## Acceptance Criteria

- [ ] `SentimentFetcher` struct dengan 3 circuit breakers dibuat
- [ ] Semua 3 fetch operations wrapped dalam `cb.Execute()`
- [ ] Wiring di bot.go diupdate
- [ ] `go build ./...` clean
- [ ] Behavior test: jika cbCNN open (terlalu banyak error), `Fetch()` tetap return data dari AAII + CBOE (partial data), tidak panic

---

## Notes

- JANGAN ubah logic fetching, hanya tambah circuit breaker wrapper
- Lihat `internal/service/news/fetcher.go` sebagai referensi pattern
- Import: `"github.com/arkcode369/ark-intelligent/pkg/circuitbreaker"`
