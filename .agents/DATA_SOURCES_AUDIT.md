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

## ⚠️ BERBAYAR — SUDAH ADA, BIARKAN

| Sumber | Data | Status |
|---|---|---|
| **Massive (Polygon.io)** | Historical price data, flat files | Sudah di .env → pakai |
| **Firecrawl** | Web scraping | Sudah di .env → **manfaatkan lebih** |
| **Claude (Anthropic)** | AI analysis | Sudah di .env → pakai |
| **TwelveData** | Intraday price data | Sudah di .env → pakai |

**Prinsip: jangan rombak yang sudah ada. Kalau butuh sumber baru → cari gratis.**

---

## 🔄 PELUANG MANFAATKAN FIRECRAWL LEBIH LANJUT

Firecrawl sudah ada dan dibayar — maksimalkan penggunaannya:

### Sumber yang bisa di-scrape via Firecrawl (gratis setelah punya key):
- **AAII Sentiment Survey** — `aaii.com/sentimentsurvey/sent_results`
- **Fear & Greed Index** — `money.cnn.com/data/fear-and-greed`
- **BofA Fund Manager Survey** (saat dirilis) — media coverage
- **COT positioning narrative** dari berbagai broker/analyst
- **Commitments of Traders commentary** — tastytrade, barchart
- **Market breadth data** — barchart.com/stocks/market-pulse
- **Insider trading flow** — openinsider.com (public)
- **Options unusual activity** — unusualwhales.com (public pages)
- **Macro commentary** — federalreserve.gov speeches/minutes

### TASK untuk Research Agent:
1. **TASK: Tambah AAII sentiment via Firecrawl** (sudah ada key, tinggal implement)
2. **TASK: Tambah Fear & Greed Index scraper** via Firecrawl
3. **TASK: Fed speeches scraper** via Firecrawl → input ke AI analysis

---

## 📋 KALAU BUTUH SUMBER BARU — GRATIS DULU

Prioritas pencarian sumber baru:

1. **Yahoo Finance** — forex, stocks, ETFs (no key, unlimited)
2. **Stooq.com** — historical forex data (no key)
3. **Alpha Vantage free** — 25 req/day (minimal, backup only)
4. **Quandl/NASDAQ Data Link free** — beberapa dataset gratis
5. **World Bank API** — macro global data (gratis, no key)
6. **IMF Data API** — macro global (gratis, no key)
7. **BIS Data** — FX turnover, cross-border banking (gratis)

---

## 💯 Arsitektur Data Saat Ini (Jangan Dirombak)

```
COT Data          → CFTC Socrata ✅ gratis
Macro/FRED        → FRED API ✅ gratis (perlu key)
Economic Calendar → MQL5 scraping ✅ gratis
Price (intraday)  → TwelveData ✅ sudah di .env
Price (historical)→ Massive/Polygon ✅ sudah di .env
AI Analysis       → Claude + Gemini ✅ sudah di .env
Web Scraping      → Firecrawl ✅ sudah di .env — manfaatkan lebih!
Crypto data       → Bybit public + CoinGecko ✅ sudah di .env
VIX Sentiment     → CBOE public ✅ gratis
Energy Data       → EIA ✅ gratis (perlu key)

Baru (gratis):
AAII Sentiment    → via Firecrawl (implement)
Fear & Greed      → via Firecrawl (implement)
Fed Speeches      → via Firecrawl (implement)
```
