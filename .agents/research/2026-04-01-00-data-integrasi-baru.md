# Research: Data & Integrasi Baru — Siklus 2
**Tanggal:** 2026-04-01 00:xx WIB
**Fokus:** Siklus 2 — Data Sources & Integrasi Baru

---

## Metodologi
Audit dilakukan dengan:
1. Review DATA_SOURCES_AUDIT.md sebagai referensi utama
2. Analisis seluruh file di `internal/service/`, `internal/domain/`, `internal/adapter/telegram/`
3. Cross-check apa yang sudah ada vs apa yang tercantum sebagai peluang di audit dokumen

---

## Status Data Sources Saat Ini

### ✅ Sudah Diimplementasi
| Source | Implementasi | File |
|--------|-------------|------|
| CNN Fear & Greed | Direct JSON API | `internal/service/sentiment/sentiment.go` |
| AAII Sentiment | Via Firecrawl JSON extract | `internal/service/sentiment/sentiment.go` |
| CBOE Put/Call Ratios | Via Firecrawl JSON extract | `internal/service/sentiment/cboe.go` |
| FRED Macro (yield curve, inflation, dll) | REST API | `internal/service/fred/fetcher.go` |
| MQL5 Economic Calendar | POST scraping | `internal/service/news/fetcher.go` |
| EIA Energy Data | REST API v2 | `internal/service/price/eia.go` |
| CoinGecko (BTC dominance, dll) | REST API | `internal/service/marketdata/coingecko/client.go` |
| Bybit (orderbook, trades, OI, L/S ratio) | REST API V5 | `internal/service/marketdata/bybit/client.go` |

---

## Temuan: Gap yang Signifikan

### GAP 1: CBOE Put/Call TIDAK masuk ke AI Prompt (Unified Outlook)
- **Problem:** Data CBOE Put/Call SUDAH di-fetch dan disimpan di `SentimentData.PutCallTotal/Equity/Index/Signal`
- **Problem:** Data TERSEBUT ditampilkan di `/sentiment` formatter (`formatter.go:3682`)
- **TAPI:** Di `unified_outlook.go` Section 6 (MARKET SENTIMENT), hanya CNN F&G dan AAII yang dimasukkan ke prompt AI. CBOE Put/Call sama sekali tidak ada!
- **Impact:** AI tidak punya data options sentiment saat generate unified outlook
- **Fix:** Cukup tambahkan 3-4 baris di `unified_outlook.go` section 6

### GAP 2: Fed Speeches/Minutes Scraper Belum Ada
- **Status:** DATA_SOURCES_AUDIT.md secara eksplisit mencantumkan sebagai TASK: "TASK: Fed speeches scraper via Firecrawl"
- **Source:** `federalreserve.gov/newsevents/speech/` — public, free, no auth
- **Value:** Fed communication adalah leading indicator untuk USD dan seluruh forex market
  - Hawkish speech → USD bullish bias, EM currencies bearish
  - Dovish speech → sebaliknya
- **Implementasi yang ada:** Tidak ada. Context builder tidak punya info Fed communication terbaru
- **Firecrawl key:** Sudah ada di .env — tinggal implementasi

### GAP 3: Market Breadth Data Belum Ada
- **Status:** Tidak ada data advance-decline, % stocks above MA, new highs/lows di sistem
- **Source:** barchart.com/stocks/market-pulse — public, free
- **Via:** Firecrawl JSON extraction (sudah ada key)
- **Value:** Market breadth adalah leading indicator risk sentiment:
  - Breadth divergence (price naik tapi breadth melemah) = distribusi, bearish jangka menengah
  - Strong breadth = genuine bull market, risk-on
  - Complement CBOE P/C dan CNN F&G untuk sentiment dashboard yang lebih lengkap
- **Data yang bisa diambil:** % NYSE/S&P above 50MA, % above 200MA, AD line, new 52wk highs/lows

### GAP 4: World Bank API Belum Dieksplor
- **Status:** Gratis, no API key, unlimited
- **Source:** `api.worldbank.org/v2/`
- **Value untuk forex:** GDP growth differential antara negara adalah driver jangka panjang exchange rate
  - EUR/USD: Eurozone GDP vs US GDP → bias arah fundamental
  - GBP, AUD, CAD: masing-masing negara punya GDP momentum berbeda
  - Current Account balance → structural demand/supply untuk currency
- **Series yang relevan:**
  - `NY.GDP.MKTP.KD.ZG` — GDP growth (annual)
  - `BN.CAB.XOKA.CD` — Current Account Balance
  - `FP.CPI.TOTL.ZG` — CPI Inflation

---

## Temuan Teknis

### Arsitektur Sentiment Service
```
FetchSentiment(ctx)
  ├── fetchCNNFearGreed()     ← direct HTTP
  ├── fetchAAIISentiment()    ← Firecrawl JSON
  └── FetchCBOEPutCall()      ← Firecrawl JSON

GetCachedOrFetch() ← in-memory cache, 6h TTL
```

**Masalah tambahan:** Cache hanya di memory — restart process membuang cache dan memaksa re-fetch semua Firecrawl calls sekaligus (3 API calls). Untuk produksi yang lebih stabil, cache sentiment ke BadgerDB dengan TTL.

### Unified Outlook Section 6 (saat ini)
```go
if sd.CNNAvailable {
    b.WriteString(fmt.Sprintf("CNN Fear & Greed: %.0f/100 (%s)\n", ...))
    if sd.AAIIAvailable {
        b.WriteString(fmt.Sprintf("AAII: Bull=%.1f%% Bear=%.1f%%...\n", ...))
    }
}
// CBOE Put/Call — HILANG dari sini!
```

---

## Rekomendasi Task (Prioritas)

| # | Task | Priority | Effort | Type |
|---|------|----------|--------|------|
| TASK-006 | CBOE Put/Call masuk ke Unified Outlook AI Prompt | HIGH | S | fix |
| TASK-007 | Fed Speeches Scraper via Firecrawl | HIGH | M | data |
| TASK-008 | Market Breadth via Firecrawl (barchart) | MEDIUM | M | data |
| TASK-009 | World Bank API Cross-Country Macro | MEDIUM | L | data |

---

## Referensi
- `.agents/DATA_SOURCES_AUDIT.md`
- `internal/service/sentiment/sentiment.go`
- `internal/service/sentiment/cboe.go`
- `internal/service/ai/unified_outlook.go` (section 6)
- `internal/adapter/telegram/formatter.go` (FormatSentiment)
