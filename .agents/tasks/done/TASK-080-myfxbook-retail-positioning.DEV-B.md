# TASK-080: Myfxbook Retail Positioning Integration

**Priority:** HIGH
**Siklus:** 2 (Data & Integrasi Gratis)
**Estimasi:** 4-6 jam
**Area:** internal/service/sentiment/

---

## Latar Belakang

Myfxbook Community Outlook menyediakan data % long/short retail traders 
untuk major forex pairs (EURUSD, GBPUSD, USDJPY, dll). Data ini FREE dan 
public — bisa di-scrape via Firecrawl yang sudah ada key-nya.

Retail positioning adalah contrarian indicator yang powerful:
- Retail 80%+ short → sinyal bullish potensial
- Retail 80%+ long → sinyal bearish potensial

---

## Tujuan

Tambah retail positioning data dari Myfxbook ke SentimentData struct,
tampilkan di /sentiment command dengan interpretasi contrarian.

---

## Implementasi

### 1. Buat `internal/service/sentiment/myfxbook.go`

```go
// Scrape Myfxbook Community Outlook via Firecrawl
// URL: https://www.myfxbook.com/community/outlook
// Fields per pair: symbol, long%, short%, longVolume, shortVolume

type MyfxbookPairSentiment struct {
    Symbol    string  // e.g. "EURUSD"
    LongPct   float64 // e.g. 32.5 (%)
    ShortPct  float64 // e.g. 67.5 (%)
    Signal    string  // "CONTRARIAN_BULLISH", "CONTRARIAN_BEARISH", "NEUTRAL"
}

type MyfxbookData struct {
    Pairs     []MyfxbookPairSentiment
    Available bool
    FetchedAt time.Time
}

func FetchMyfxbook(ctx context.Context) *MyfxbookData
```

Pola implementasi: sama persis dengan cboe.go (Firecrawl JSON extraction).

Schema Firecrawl:
```json
{
  "type": "object",
  "properties": {
    "pairs": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "symbol": {"type": "string"},
          "long_pct": {"type": "number"},
          "short_pct": {"type": "number"}
        }
      }
    }
  }
}
```

Prompt Firecrawl: "Extract retail trader positioning from Myfxbook Community 
Outlook: for each currency pair, extract the symbol, long percentage, and 
short percentage."

### 2. Extend SentimentData struct (sentiment.go)

Tambah field:
```go
// Myfxbook Retail Positioning
MyfxbookPairs     []MyfxbookPairSentiment
MyfxbookAvailable bool
```

### 3. Extend FetchSentiment() (sentiment.go)

```go
// Fetch Myfxbook retail positioning
mfx := FetchMyfxbook(ctx)
IntegrateMyfxbookIntoSentiment(data, mfx)
```

### 4. Signal Classification

```go
func ClassifyRetailSignal(longPct float64) string {
    switch {
    case longPct <= 20:
        return "CONTRARIAN_BULLISH" // retail 80%+ short → bull
    case longPct >= 80:
        return "CONTRARIAN_BEARISH" // retail 80%+ long → bear
    case longPct <= 30:
        return "LEAN_BULLISH"
    case longPct >= 70:
        return "LEAN_BEARISH"
    default:
        return "NEUTRAL"
    }
}
```

### 5. Update formatter.go

Tambah section di /sentiment output:
```
📊 RETAIL POSITIONING (Myfxbook)
EURUSD: Retail 32% Long / 68% Short → 🟢 Contrarian Bullish
GBPUSD: Retail 71% Long / 29% Short → 🔴 Contrarian Bearish
```

### 6. Integrasi ke UnifiedOutlookData & prompts.go

Tambah MyfxbookData ke UnifiedOutlookData struct dan sertakan
ke BuildUnifiedOutlookPrompt() sebagai context retail positioning.

---

## Testing

- Unit test: TestClassifyRetailSignal (edge cases: 0%, 50%, 100%)
- Integration test: verifikasi Firecrawl call return data valid
- Manual test: jalankan /sentiment, cek section Myfxbook muncul

---

## File yang Dimodifikasi

- `internal/service/sentiment/myfxbook.go` (NEW)
- `internal/service/sentiment/sentiment.go` (extend SentimentData)
- `internal/adapter/telegram/formatter.go` (tambah display section)
- `internal/service/ai/unified_outlook.go` (tambah ke prompt)

---

## Referensi

- Pola implementasi: `internal/service/sentiment/cboe.go`
- DATA_SOURCES_AUDIT.md — Myfxbook section
- Research: `.agents/research/2026-04-01-10-data-integrasi-gratis.md`
