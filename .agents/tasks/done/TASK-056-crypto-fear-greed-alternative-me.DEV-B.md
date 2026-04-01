# TASK-056: Crypto Fear & Greed Index via alternative.me API

**Priority:** 🟢 HIGH  
**Cycle:** Siklus 2 — Data & Integrasi  
**Effort:** ~2 jam  
**Assignee:** Dev-B atau Dev-C

---

## Context

Bot sudah punya CNN Fear & Greed (equities/macro), AAII (retail investor survey), dan CBOE Put/Call ratio. Namun tidak ada **crypto-specific sentiment** dari sisi fear & greed.

alternative.me menyediakan Crypto Fear & Greed Index — free, no API key, JSON endpoint sederhana.

**Verified live:**
```
GET https://api.alternative.me/fng/?limit=3
Response: [{"value":"11","value_classification":"Extreme Fear","timestamp":"1774915200"}]
```
Current reading: **11/100 — Extreme Fear** (sangat relevan untuk trading context).

---

## Acceptance Criteria

1. **Tambah field ke `SentimentData`** di `internal/service/sentiment/sentiment.go`:
   ```go
   // Crypto Fear & Greed Index (alternative.me)
   CryptoFearGreed      float64 // 0-100
   CryptoFearGreedLabel string  // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
   CryptoFearGreedAvailable bool
   ```

2. **Tambah fetch function** `fetchCryptoFearGreed()` di `sentiment.go`:
   - URL: `https://api.alternative.me/fng/?limit=2`
   - Parse JSON: `data[0].value` dan `data[0].value_classification`
   - Timeout: 10 detik
   - Graceful fail: set `CryptoFearGreedAvailable = false` jika gagal

3. **Call di `FetchSentiment()`** bersama sumber lainnya.

4. **Tampilkan di formatter** (`internal/adapter/telegram/formatter.go`, fungsi `FormatSentiment()`):
   - Tambah baris: `🎰 Crypto F&G: 11/100 — Extreme Fear`
   - Posisi: setelah CNN F&G, sebelum AAII

5. **Inject ke AI context** di `internal/service/ai/unified_outlook.go` bagian SentimentData:
   ```go
   if sd.CryptoFearGreedAvailable {
       b.WriteString(fmt.Sprintf("Crypto Fear & Greed: %.0f/100 (%s)\n", sd.CryptoFearGreed, sd.CryptoFearGreedLabel))
   }
   ```

---

## API Spec

```
GET https://api.alternative.me/fng/?limit=2
No auth header needed.

Response:
{
  "name": "Fear and Greed Index",
  "data": [
    {
      "value": "11",
      "value_classification": "Extreme Fear",
      "timestamp": "1774915200",
      "time_until_update": "16496"
    }
  ]
}
```

Scale:
- 0-24: Extreme Fear
- 25-44: Fear
- 45-55: Neutral
- 56-74: Greed
- 75-100: Extreme Greed

---

## Files to Edit

- `internal/service/sentiment/sentiment.go` — tambah field + fetch function
- `internal/adapter/telegram/formatter.go` — tambah ke FormatSentiment()
- `internal/service/ai/unified_outlook.go` — tambah ke sentiment section

---

## Notes

- Tidak perlu Firecrawl — endpoint JSON publik
- Tidak perlu API key
- Update harian (sekali per hari dari alternative.me)
- Cache bisa 6-12 jam (sama seperti CNN F&G)
