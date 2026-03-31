# Data Sources & API Audit — ark-intelligent

## ✅ GRATIS SEPENUHNYA (No API Key)

| Sumber | Data | URL | Notes |
|---|---|---|---|
| **CFTC Socrata** | COT positioning data | publicreporting.cftc.gov | Public government data, unlimited |
| **FRED (St. Louis Fed)** | Macro indicators (yield curve, labor, inflation) | api.stlouisfed.org | Free API key, sangat generous (120 req/min) |
| **MQL5 Economic Calendar** | Economic events, actuals, forecasts | mql5.com | Scraping public page, no key needed |
| **CBOE** | VIX data | cboe.com | Public statistics page |

---

## ⚠️ FREEMIUM (Ada Free Tier, Perlu Key)

| Sumber | Data | Free Tier | Batasan Free |
|---|---|---|---|
| **TwelveData** | Price OHLCV (forex, crypto, stocks) | ✅ Free tier ada | 800 req/day, 8 req/min — cukup untuk beberapa pair |
| **CoinGecko** | BTC dominance, TOTAL3 | ✅ Demo plan gratis | 30 req/min — cukup |
| **EIA** | Energy data (oil, gas inventory) | ✅ Free | Unlimited, perlu registrasi |
| **Bybit** | Crypto orderbook, trades | ✅ Public endpoints gratis | Market data tidak perlu API key |
| **Gemini AI** | AI narrative/outlook | ✅ Free tier ada | `gemini-2.0-flash` gratis via API |

---

## ❌ BERBAYAR / PERLU LANGGANAN

| Sumber | Data | Biaya | Alternatif Gratis |
|---|---|---|---|
| **Massive (Polygon.io)** | Historical price data, flat files | Berbayar | Yahoo Finance, Alpha Vantage free |
| **Firecrawl** | Web scraping (AAII sentiment) | Berbayar ($16/mo+) | Scrape langsung AAII.com |
| **Claude (Anthropic)** | AI analysis | Berbayar | Gemini free tier |
| **TwelveData (paid)** | Multi-key rotation untuk volume tinggi | Berbayar | Lihat alternatif di bawah |

---

## 🔄 REKOMENDASI PENGGANTIAN

### 1. Massive → Yahoo Finance (gratis, no key)
```go
// Yahoo Finance OHLCV — tidak perlu API key
// https://query1.finance.yahoo.com/v8/finance/chart/EURUSD=X?interval=1d&range=2y
```
Data: forex (suffix =X), stocks, ETFs, futures — gratis unlimited

### 2. Firecrawl → Direct scrape AAII (gratis)
```go
// AAII sentiment page bisa di-scrape langsung
// https://www.aaii.com/sentimentsurvey/sent_results
```

### 3. Massive S3 flat files → CFTC bulk download (gratis)
- CFTC sudah provide bulk historical COT data gratis

### 4. Claude (mahal) → Gemini Flash (gratis)
- `gemini-2.0-flash-exp` gratis via Google AI Studio key
- Sudah ada integrasi Gemini di codebase

### 5. TwelveData (kalau limit) → Alpha Vantage (gratis)
- 25 req/day free tier (terbatas), tapi ada `premium` community key
- Alternatif: Stooq.com (no key, Polish data provider, semua forex gratis)

---

## 📋 ACTION ITEMS untuk Research Agent

Task yang perlu dibuat untuk memastikan semua gratis:

1. **TASK: Ganti Massive → Yahoo Finance** untuk price fetching
2. **TASK: Ganti Firecrawl → direct AAII scraper** untuk sentiment
3. **TASK: Tambah fallback chain** — TwelveData → Yahoo Finance → Stooq → Alpha Vantage
4. **TASK: FRED API key** — daftar gratis di fred.stlouisfed.org (5 menit)
5. **TASK: EIA API key** — daftar gratis di eia.gov/opendata (5 menit)
6. **TASK: Gemini API key** — gratis di aistudio.google.com
7. **TASK: CoinGecko demo key** — gratis di coingecko.com/api

---

## 💯 Target Arsitektur Data (Semua Gratis)

```
COT Data          → CFTC Socrata (gratis, unlimited) ✅ sudah
Macro/FRED        → FRED API (gratis, 120 req/min) ✅ sudah
Economic Calendar → MQL5 scraping (gratis) ✅ sudah
Price Data        → Yahoo Finance (gratis) ← GANTI dari Massive
Intraday          → TwelveData free tier (800/day) ← sudah ada
AI Analysis       → Gemini Flash (gratis) ← sudah ada, jadikan primary
Crypto data       → Bybit public + CoinGecko demo (gratis) ✅ sudah
VIX Sentiment     → CBOE public (gratis) ✅ sudah
AAII Sentiment    → Direct scrape (gratis) ← GANTI dari Firecrawl
Energy Data       → EIA (gratis) ← sudah ada, daftar key
```
