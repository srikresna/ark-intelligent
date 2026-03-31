# Research Report: Siklus 2 — Data & Integrasi Baru Gratis
**Tanggal:** 2026-04-01 | **Agent:** Research | **Siklus:** 2/5

---

## Ringkasan Temuan

Siklus ini fokus pada gap data source yang belum terimplementasi di ark-intelligent.
Analisis codebase menunjukkan bahwa proyek sudah punya fondasi solid (COT, FRED, 
CNN F&G, AAII, CBOE, EIA), namun beberapa sumber data gratis bernilai tinggi 
untuk konteks forex trading institusional belum tersedia.

---

## Data Sources Yang Sudah Ada ✅

| Sumber | Package | Status |
|---|---|---|
| CFTC Socrata (COT) | internal/service/cot | ✅ Aktif |
| FRED API | internal/service/fred | ✅ Aktif |
| MQL5 (Economic Calendar) | internal/service/news | ✅ Aktif |
| CBOE Put/Call via Firecrawl | internal/service/sentiment/cboe.go | ✅ Aktif |
| CNN Fear & Greed | internal/service/sentiment/sentiment.go | ✅ Aktif |
| AAII Survey via Firecrawl | internal/service/sentiment/sentiment.go | ✅ Aktif |
| EIA Energy Data | internal/service/price/eia.go | ✅ Aktif |
| TwelveData / Yahoo / AlphaVantage | internal/service/price/fetcher.go | ✅ Aktif |
| CoinGecko | internal/service/marketdata/coingecko | ✅ Aktif |
| Bybit Public | internal/service/marketdata/bybit | ✅ Aktif |
| Massive/Polygon.io | internal/service/marketdata/massive | ✅ Aktif |

---

## Gap Yang Ditemukan — Sumber Gratis Belum Terimplementasi

### 1. Myfxbook Retail Positioning (PRIORITAS TINGGI)
- **URL:** https://www.myfxbook.com/community/outlook
- **Data:** % long/short retail traders per pair (EURUSD, GBPUSD, dll)
- **Nilai:** Contrarian signal kuat — retail biasanya salah di ekstrem
- **Cara akses:** Public web page, scrape via Firecrawl
- **Gap:** Tidak ada di codebase sama sekali
- **Impact:** Langsung menambah dimensi baru ke /sentiment command
- **Contoh output:** "Retail 78% short EURUSD → contrarian bullish signal"

### 2. Fed Speeches & Minutes Scraper via Firecrawl (PRIORITAS TINGGI)
- **URL:** https://www.federalreserve.gov/newsevents/speeches.htm
- **Data:** Judul pidato Fed, pembicara, tanggal, link ke full text
- **Nilai:** Konteks kebijakan moneter real-time untuk AI prompt enrichment
- **Cara akses:** Firecrawl (key sudah ada di .env)
- **Gap:** Disebutkan di DATA_SOURCES_AUDIT.md sebagai task pending, belum ada kode
- **Impact:** Memperkaya /outlook dengan analisis Fed communication terkini
- **Note:** Beige Book juga tersedia di sini (8x/tahun)

### 3. World Bank API — Global Macro (PRIORITAS MEDIUM)
- **URL:** https://api.worldbank.org/v2/
- **Data:** GDP growth, current account balance, inflation, FX reserves
- **Nilai:** Konteks fundamental untuk EM currencies (IDR, TRY, BRL, dll)
- **Cara akses:** REST API gratis, no key needed
- **Gap:** Tidak ada implementasi sama sekali
- **Impact:** Menambah fundamental layer ke analisis pair EM
- **Contoh:** GDP_growth, CA_balance untuk AUD, NZD, CAD fundamentals

### 4. Stooq.com Historical Forex Data (PRIORITAS MEDIUM)
- **URL:** https://stooq.com/q/d/l/?s={pair}.fx&i=w
- **Data:** Historical weekly OHLC untuk semua major/minor pairs
- **Nilai:** Free fallback untuk historical data tanpa API key
- **Cara akses:** Direct CSV download, no key
- **Gap:** Price fetcher hanya punya TwelveData → AlphaVantage → Yahoo
- **Impact:** Menambah layer ke-4 fallback tanpa biaya tambahan
- **Note:** Format CSV sederhana, mudah diparse

### 5. BIS FX Statistics (PRIORITAS LOW-MEDIUM)
- **URL:** https://stats.bis.org/api/v1/data/WS_XRU/
- **Data:** BIS effective exchange rate indices (REER/NEER), FX turnover
- **Nilai:** Real Effective Exchange Rate lebih akurat dari nominal untuk fundamental
- **Cara akses:** BIS Statistics API v1 (gratis, no key)
- **Gap:** Tidak ada implementasi
- **Impact:** Menambah REER context ke /macro dan /bias commands

---

## Analisis Prioritas untuk Task Creation

Berdasarkan impact vs effort:

| # | Fitur | Impact | Effort | Priority |
|---|---|---|---|---|
| 1 | Myfxbook Retail Positioning | Tinggi | Rendah | HIGH |
| 2 | Fed Speeches Scraper | Tinggi | Rendah | HIGH |
| 3 | World Bank API | Medium | Medium | MEDIUM |
| 4 | Stooq Historical Fallback | Medium | Rendah | MEDIUM |
| 5 | BIS REER Data | Medium | Medium | LOW-MEDIUM |

---

## Temuan Teknis Tambahan

1. **Pattern implementasi konsisten:** Semua data source baru ikut pola
   `internal/service/{domain}/{source}.go` dengan Available flag dan circuit breaker

2. **Integrasi ke SentimentData:** Retail positioning (Myfxbook) paling natural
   masuk ke `SentimentData` struct di sentiment.go — sudah ada AAII, CNN, CBOE

3. **Firecrawl sudah terbayar:** Investasi terbaik adalah menggunakannya untuk
   scrape lebih banyak sumber (Myfxbook, Fed speeches, unusualwhales)

4. **UnifiedOutlookData struct:** Retail positioning dan Fed speeches akan
   memperkaya BuildUnifiedOutlookPrompt() secara langsung

5. **Stooq fallback:** Sangat simpel diimplementasi — fetch CSV, parse, return 
   []domain.PriceRecord — fit langsung ke FetcherInterface existing

---

## File Referensi
- `internal/service/sentiment/sentiment.go` — pola untuk data source baru
- `internal/service/sentiment/cboe.go` — pola Firecrawl integration
- `internal/service/price/eia.go` — pola REST API gratis
- `internal/service/price/fetcher.go` — pola fallback chain
- `.agents/DATA_SOURCES_AUDIT.md` — referensi audit lengkap
