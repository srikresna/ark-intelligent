# Research: Data & Integrasi Baru — Siklus 2 Putaran 2
**Tanggal:** 2026-04-01 15:xx WIB
**Fokus:** Siklus 2 — Data Sources & Integrasi Baru (lanjutan)

---

## Konteks
Siklus 2 putaran pertama (TASK-006 s/d TASK-009) sudah membuat task untuk:
- CBOE P/C ke Unified Outlook
- Fed Speeches scraper
- Market Breadth via Firecrawl
- World Bank Cross-Country Macro

Putaran 2 ini menggali lebih dalam gap yang BELUM dicakup putaran pertama,
dengan analisis lebih rinci ke codebase aktual.

---

## Analisis Codebase Aktual

### Service Sentiment (`internal/service/sentiment/`)
```
sentiment.go  ← CNN F&G + AAII (via Firecrawl) — ✅ ADA
cboe.go       ← CBOE Put/Call (via Firecrawl)  — ✅ ADA
cache.go      ← in-memory cache, TTL 6 jam     — ⚠️ HANYA MEMORY
```

**Cache Masalah:** `cache.go` menggunakan pure in-memory cache (`var cachedSentiment *SentimentData`).
Setiap kali bot restart, cache hilang → 3 Firecrawl API calls sekaligus.
BadgerDB sudah ada di project (`internal/adapter/storage/cache_repo.go` pakai `badger/v4`).
Sentiment fetch melibatkan 3 Firecrawl calls yang berbayar — sangat worth untuk dipersist.

### Service FRED (`internal/service/fred/`)
```
fetcher.go         ← SOFR, FedFundsRate, IORB, yield curve — ✅ ADA
rate_differential.go ← Carry ranking via CentralBankRateMapping — ✅ ADA
composites.go      ← SOFR-IORB spread, financial conditions — ✅ ADA
```

**GAP Besar: TIDAK ADA market-implied rate path.**
FRED punya data actual rates (FEDFUNDS, SOFR, yield curve), tapi TIDAK ADA:
- Forward OIS rates (implied rate expectations)
- Fed Funds Futures probabilities
- "Priced-in" rate cuts/hikes count

CME FedWatch menunjukkan probabilitas berdasarkan Fed Funds Futures. Ini sangat
penting: jika market price 80% rate cut, USD akan tertekan bahkan sebelum Fed action.
Saat ini bot tidak punya data ini sama sekali.

### Service AI Unified Outlook
```go
// Section 6 current:
if sd.CNNAvailable {
    b.WriteString("CNN Fear & Greed: ...")
    if sd.AAIIAvailable {
        b.WriteString("AAII: ...")
    }
}
// CBOE Put/Call ← MASIH HILANG (TASK-006 pending)
// FedSpeeches   ← BELUM ADA (TASK-007 pending)
// WorldBank     ← BELUM ADA (TASK-009 pending)
// Rate Path     ← BELUM ADA task
```

### Domain Rate Differential
```go
// rate_differential.go menggunakan CentralBankRateMapping
// Hanya actual policy rates — tidak ada "expected next rate"
// TIDAK ADA: next meeting implied probability, dots vs market divergence
```

---

## GAP Baru yang Ditemukan

### GAP 1: CME FedWatch / Market-Implied Rate Cut Probabilities
**Belum ada task. HIGH impact.**

FRED sudah punya SOFR data, tapi tidak ada implied forward rates.
CME FedWatch (cmegroup.com/markets/interest-rates/cme-fedwatch-tool.html)
adalah public page — bisa di-scrape via Firecrawl.

Alternatif lebih robust: FRED menyediakan SOFR term structure dan FF futures.
- `SFRZ4` (SOFR Dec futures via FRED) — approximation
- Atau scrape CME FedWatch langsung via Firecrawl

Data yang diinginkan:
```
Next FOMC: 2026-05-07 | Probability: Hold=35% Cut25=55% Cut50=10%
3-meeting outlook: Cut 1x=45% Cut 2x=30% Hold=25%
Implied year-end rate: 3.75-4.00%
```

**Value untuk bot:**
- AI context sangat terbantu: "market pricing 55% rate cut next FOMC"
- Langsung relevan untuk USD directional bias
- CBOE P/C + AAII + FedWatch = complete sentiment picture

**Source:** CME FedWatch public page (via Firecrawl) atau FRED fed futures

---

### GAP 2: BIS Real Effective Exchange Rates (REER/NEER)
**Belum ada task. HIGH impact untuk forex fundamental.**

BIS (Bank for International Settlements) menyediakan REER dan NEER gratis:
- URL: `https://stats.bis.org/api/v2/data/eer/{country}/{freq}?startPeriod=...`
- Format: JSON (SDMX-JSON)
- No API key required
- Update: bulanan

REER = Real Effective Exchange Rate (inflation-adjusted trade-weighted basket)
NEER = Nominal Effective Exchange Rate

**Why this matters untuk forex:**
- REER > long-term avg → currency OVERVALUED → mean reversion bias bearish
- REER < long-term avg → currency UNDERVALUED → mean reversion bias bullish
- EUR REER vs USD REER → structural directional bias multi-month
- Dipakai IMF, World Bank, dan bank-bank besar sebagai reference valuation

Countries covered: USD, EUR, GBP, JPY, CHF, AUD, CAD, NZD

---

### GAP 3: Sentiment Service Cache Persistence (BadgerDB)
**Belum ada task. MEDIUM impact, HIGH reliability improvement.**

Saat ini `internal/service/sentiment/cache.go`:
```go
var cachedSentiment *SentimentData  // pure memory
var cacheExpiry     time.Time
```

Setiap restart: 3 Firecrawl API calls (AAII + CBOE P/C + CNN).
Firecrawl berbayar — setiap restart membuang quota.

BadgerDB sudah tersedia dan digunakan oleh:
- `internal/adapter/storage/cache_repo.go` (AI cache)
- `internal/service/fred/persistence.go` (FRED snapshots)

Solution: persist SentimentData ke BadgerDB dengan key `sentiment:latest` dan TTL 6 jam.

---

### GAP 4: IMF Data API — Growth & Policy Rate Forecasts
**Belum ada task. MEDIUM impact.**

IMF menyediakan `api.imf.org` datamapper — gratis, no API key.

```
GET https://www.imf.org/external/datamapper/api/v1/NGDP_RPCH/USA/GBR/JPN/DEU/AUS/CAN/NZD/CHE
```

Ini memberikan GDP growth forecast dari IMF untuk semua major currency countries.
Untuk trading, IMF forecasts lebih valuable dari World Bank historical:
"IMF expects Eurozone GDP 1.2% vs US 2.8% → structural USD strength."

---

### GAP 5: Fed Dot Plot via FRED (FEDTARMD series)
**Belum ada task. HIGH impact, effort SANGAT KECIL.**

FRED punya series `FEDTARMD` (Fed Target Rate Median dari dot plot).
Ini bisa langsung di-fetch tanpa Firecrawl — gratis via FRED API yang sudah ada.

Data structure yang ada (`MacroData` di `fetcher.go`) bisa ditambah:
```go
FedDotMedian float64  // FEDTARMD — Fed's own rate target median
FedDotHigh   float64  // FEDTARH
FedDotLow    float64  // FEDTARL
```

Lalu di Unified Outlook prompt:
```
Fed Dot Plot Median: 3.875% | Market SOFR: 4.33%
Market vs Dots: Market pricing 2 more cuts than Fed expects
```

Divergence dots vs market = major driver volatility dan trend.

---

## Rangkuman Gap dan Prioritas

| # | Gap | Priority | Effort | Source | Key |
|---|-----|----------|--------|--------|-----|
| 1 | CME FedWatch implied probabilities | HIGH | M | Firecrawl scrape | Sudah ada |
| 2 | BIS REER/NEER effective exchange rates | HIGH | M | BIS API JSON | Tidak perlu |
| 3 | Fed Dot Plot via FRED FEDTARMD | HIGH | S | FRED API | Sudah ada |
| 4 | Sentiment Cache Persistence ke BadgerDB | MEDIUM | S | Internal refactor | Tidak perlu |
| 5 | IMF WEO Growth + Inflation Forecasts | MEDIUM | M | IMF datamapper API | Tidak perlu |

---

## Referensi
- `.agents/DATA_SOURCES_AUDIT.md`
- `internal/service/sentiment/cache.go` (in-memory cache yang perlu dipersist)
- `internal/service/fred/fetcher.go` (fetch pattern + struct MacroData)
- `internal/service/ai/unified_outlook.go` (section 6, MARKET SENTIMENT)
- `internal/adapter/storage/cache_repo.go` (BadgerDB cache pattern)
- `internal/service/fred/persistence.go` (BadgerDB persistence pattern)
- CME FedWatch: cmegroup.com/markets/interest-rates/cme-fedwatch-tool.html
- BIS EER API: stats.bis.org/api/v2/data/BIS,WS_EER
- IMF DataMapper API: imf.org/external/datamapper/api/v1/
- FRED FEDTARMD: fred.stlouisfed.org/series/FEDTARMD
