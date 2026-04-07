# Research Report: Data Siklus 2 Putaran 2 — ECB, SNB, Treasury TIPS, OECD CLI, DTCC

**Tanggal:** 2026-04-01 21:00 WIB
**Fokus:** Data & Integrasi Baru Gratis (Siklus 2, Putaran 2)
**Siklus:** 2/5

---

## Ringkasan

Riset 15 data source potensial. 8 confirmed viable, 5 dipilih sebagai task baru berdasarkan edge value tertinggi dan kemudahan implementasi. Semua GRATIS, tanpa API key, dan sudah diverifikasi endpoint-nya berjalan.

---

## Sumber Data Terverifikasi (Top 5 untuk Task)

### 1. ECB Statistical Data Warehouse API [VERY HIGH EDGE]
- **Endpoint:** `https://data-api.ecb.europa.eu/service/data/{flowRef}?lastNObservations=N&format=csvdata`
- **Gratis, tanpa API key, CSV/JSON output**
- **Data:** EUR/USD exchange rate, ECB key interest rate (MRR), M3 money supply, bank lending, government debt yields, balance of payments
- **Edge:** M3 money supply growth divergence ECB vs Fed = powerful EUR/USD signal. Institutional-grade data yang retail trader hampir tidak pernah akses.
- **Contoh verified:** ECB MRR rate cuts dari 3.15% ke 2.15% (2025) — clean data.

### 2. SNB (Swiss National Bank) Data API [VERY HIGH EDGE for CHF]
- **Endpoint:** `https://data.snb.ch/api/cube/{cubeId}/data/csv/en`
- **Gratis, tanpa API key, CSV output**
- **Data:** Full SNB balance sheet — gold holdings, foreign currency investments, repo transactions, sight deposits
- **Edge:** Foreign currency investments = direct proxy for FX intervention. SNB FX reserves (~$850B+) moves CHF significantly. Dimension API juga tersedia untuk structured queries.
- **Implementasi:** Monthly update, direct CSV download.

### 3. Treasury.gov TIPS Real Yields / Breakeven Inflation [HIGH EDGE]
- **Endpoint:** `https://home.treasury.gov/resource-center/data-chart-center/interest-rates/daily-treasury-rates.csv/2025/all?type=daily_treasury_real_yield_curve&...`
- **Gratis, tanpa API key, CSV output**
- **Data:** Daily TIPS real yields (5Y, 7Y, 10Y, 20Y, 30Y), nominal yields, bill rates
- **Edge:** Breakeven inflation = Nominal - TIPS real yield. Core institutional signal. Rising breakevens = hawkish = USD bullish. Independent dari FRED sebagai fallback + direct Treasury source authority.
- **Implementasi:** Simple CSV, yearly files.

### 4. OECD Composite Leading Indicators (CLI) [HIGH EDGE]
- **Endpoint:** `https://sdmx.oecd.org/public/rest/data/OECD.SDD.STES,DSD_STES@DF_CLI/...`
- **Gratis, tanpa API key, CSV/JSON/XML**
- **Data:** CLI for all OECD countries (monthly), amplitude-adjusted index. Consumer Confidence, Business Confidence, Industrial Production.
- **Edge:** CLI = 6-9 bulan forward-looking. Cross-country CLI divergence bisa predict FX trends. US CLI naik vs EU CLI turun = bullish USD.
- **Implementasi:** SDMX REST, parse CSV. Monthly cadence.

### 5. DTCC Swap Data Repository [HIGH EDGE for institutional]
- **Endpoint:** `https://pddata.dtcc.com/ppd/api/report/cumulative/CFTC/FOREX?asof=YYYY-MM-DD`
- **Gratis, web-based, REST API**
- **Data:** Public post-trade FX swap/derivative data. Individual trade records dengan notional amounts, currencies, maturities.
- **Edge:** Actual trade-level FX derivative data. Volume patterns di FX forwards/swaps signal hedging flows dan positioning shifts. Sangat sedikit retail trader yang akses ini.
- **Implementasi:** REST API, JSON responses. Mungkin perlu pagination.

---

## Sumber Lain yang Viable (tidak dijadikan task sekarang)

| Source | Status | Edge | Alasan Ditunda |
|--------|--------|------|----------------|
| Bank of England API | FREE, verified | HIGH (GBP-specific) | Niche — hanya relevan untuk GBP pairs |
| Investing.com Calendar | FREE via Firecrawl | MEDIUM-HIGH | MQL5 sudah cukup, overlap |
| IG Client Sentiment | FREE via scraping | HIGH (contrarian) | Perlu reverse-engineer IG pages, brittle |
| BOJ Intervention | Exists, URL changed | HIGH for JPY | URL restructured, perlu investigation |
| ForexFactory Calendar | JS-heavy, sulit | MEDIUM | Firecrawl gagal render, butuh agent mode |

---

## Yang TIDAK Viable

| Source | Alasan |
|--------|--------|
| CME Group Open Data | No free API, Cloudflare protected |
| ICE/Eurex FX Options | Paid subscription only |
| CLS Group Settlement | Commercial product (~$50K+/year) |
| ISDA Statistics | PDF reports only, no API |
| TradingView Widgets | ToS prohibits scraping |
| OANDA Order Book | Discontinued free tier |

---

## Arsitektur Rekomendasi

Untuk kelima sumber baru, rekomendasi arsitektur:
```
internal/service/macro/
├── ecb_client.go       ← ECB SDW API client
├── snb_client.go       ← SNB balance sheet client
├── treasury_client.go  ← Treasury.gov TIPS/yields CSV
├── oecd_client.go      ← OECD CLI SDMX client
├── dtcc_client.go      ← DTCC swap data client
```

Semua bisa masuk di `service/fred/` yang sudah handle macro data, atau buat package terpisah `service/macro/` untuk central bank + institutional data.

---

## Task Recommendations

1. **TASK-105**: ECB SDW API integration (M3, rates, bank lending) [HIGH]
2. **TASK-106**: SNB balance sheet / FX intervention proxy [HIGH]
3. **TASK-107**: Treasury.gov TIPS yields & breakeven inflation [HIGH]
4. **TASK-108**: OECD CLI composite leading indicators [MEDIUM]
5. **TASK-109**: DTCC FX swap data repository [MEDIUM]
