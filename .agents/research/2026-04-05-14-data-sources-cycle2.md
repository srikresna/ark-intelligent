# Research: Data Sources & New Integrations — Cycle 2
**Date:** 2026-04-05  
**Agent:** Research (Cycle 2)  
**Focus:** DATA_SOURCES_AUDIT.md — identifikasi gap integrasi, sumber gratis baru, peluang Firecrawl

---

## Ringkasan Temuan

### 1. SUDAH TERIMPLEMENTASI (tidak perlu task)

| Sumber | Status | Catatan |
|---|---|---|
| CFTC COT (Legacy + TFF + Disaggregated) | ✅ Done | Socrata + CSV fallback + circuit breaker |
| FRED (WALCL, TGA, RRP, yield curve, M2, dll) | ✅ Done | 50+ series, sangat komprehensif |
| MQL5 Economic Calendar | ✅ Done | Hidden POST endpoint, WIB timezone |
| CBOE VIX + VIX term structure | ✅ Done | CSV endpoint, free |
| CBOE Put/Call Ratios | ✅ Done | Via Firecrawl, circuit breaker |
| AAII Sentiment Survey | ✅ Done | Via Firecrawl, weekly |
| CNN Fear & Greed Index | ✅ Done | Public JSON endpoint |
| Crypto Fear & Greed (alternative.me) | ✅ Done | Free JSON API |
| CoinGecko (BTC dom, TOTAL3, global market) | ✅ Done | Demo plan |
| Bybit (microstructure, OI, funding, L/S) | ✅ Done | Public endpoints |
| OpenInsider Cluster Buys | ✅ Done | Via Firecrawl |
| Myfxbook Retail Positioning | ✅ Done | Via Firecrawl |
| Finviz (cross-asset futures + sectors) | ✅ Done | Via Firecrawl, BadgerDB cache |
| DTCC PPD FX Swap flows | ✅ Done | Free public, no key |
| ECB SDW, Eurostat, OECD CLI, SNB | ✅ Done | Free APIs |
| World Bank, IMF WEO | ✅ Done | Free APIs |
| BIS (CB rates, credit gap, REER) | ✅ Done | Free CSV endpoints |
| US Treasury auction results | ✅ Done | Free JSON API |
| SEC EDGAR 13F (5 institutions) | ✅ Done | Free, rate-limited |
| Fed speeches + FOMC press (RSS) | ✅ Done | Public RSS feed |
| CME FedWatch probabilities | ✅ Done | Via Firecrawl |
| CoinMetrics (exchange flows, active addr) | ✅ Done | Free community plan |
| DefiLlama (TVL, protocol health) | ✅ Done | Free API |
| Deribit DVOL (BTC/ETH implied vol) | ✅ Done | Free API |
| Stooq (weekly FX + metals) | ✅ Done | Free CSV, fallback source |
| Yahoo Finance (daily prices) | ✅ Done | Primary daily price source |
| TwelveData (intraday) | ✅ Done | Paid key |
| Polygon.io/Massive (historical) | ✅ Done | Paid key |

---

### 2. GAP UTAMA — BELUM TERIMPLEMENTASI

#### 🔴 HIGH VALUE: NAAIM Exposure Index
**Sumber:** `https://www.naaim.org/programs/naaim-exposure-index/`  
**Status:** BELUM ada di codebase sama sekali.  
**Data:** Rata-rata equity exposure profesional aktif fund manager AS (0–200%, sering 0–100%).  
**Update:** Setiap Rabu (~16:00 ET).  
**Nilai saat ini (2026-04-01):** 68.36  
**Akses:** Via Firecrawl JSON extraction (sudah terbukti bekerja).  
**Historical pattern dari halaman:** Data lengkap tahun 2025–2026 embedded di halaman.  
**XLSX download:** URL pattern `https://naaim.org/wp-content/uploads/{YYYY}/{MM}/USE_Data-since-Inception_{YYYY}-{MM}-{DD}.xlsx`

**Mengapa penting:**
- AAII mengukur retail investor — NAAIM mengukur PROFESIONAL aktif managers
- Cross-validation: jika AAII bullish tapi NAAIM masih bearish → sinyal campuran
- Contrarian: NAAIM >90% historically precedes correction; <30% precedes rally
- Fits naturally ke `/sentiment` command & AI context untuk outlook
- Complement to CBOE P/C (derivatives positioning) + Myfxbook (retail)

**Implementasi:** Tambah `fetchNAAIMExposure` ke `internal/service/sentiment/` — pola sama dengan `cboe.go` dan `openinsider.go`. Firecrawl JSON extraction sudah confirmed bekerja.

---

#### 🟡 MEDIUM VALUE: Binance Futures Free APIs (Cross-Exchange Validation)
**Status:** BELUM — saat ini hanya Bybit.  
**Free endpoints (no key required):**
- `https://fapi.binance.com/futures/data/globalLongShortAccountRatio?symbol=BTCUSDT&period=1h&limit=24`
- `https://fapi.binance.com/futures/data/topLongShortPositionRatio?symbol=BTCUSDT&period=1h&limit=24`
- `https://fapi.binance.com/futures/data/takerlongshortRatio?symbol=BTCUSDT&period=1h&limit=24`
- `https://fapi.binance.com/futures/data/openInterestHist?symbol=BTCUSDT&period=1h&limit=24`
- `https://fapi.binance.com/fapi/v1/premiumIndex?symbol=BTCUSDT` (mark/index premium)
- `https://fapi.binance.com/fapi/v1/fundingRate?symbol=BTCUSDT&limit=8`

**Data verified (2026-04-05):**
- Global L/S ratio: 1.56 (60.9% long / 39.1% short)
- Top trader L/S ratio: 0.85 (45.9% long / 54.0% short) — divergent!
- Taker buy/sell: 1.07 (slightly buy-dominant)
- Funding rate: 0.00001829 (neutral/slightly bullish)

**Nilai:** Binance = bigger retail base. Divergence antara Bybit (whales) dan Binance (retail) adalah signal penting. Juga validates atau contradicts microstructure signals dari Bybit.

**Target:** Buat `internal/service/sentiment/binance_futures.go` atau tambahkan ke microstructure package sebagai secondary data source.

---

#### 🟡 MEDIUM VALUE: CFTC Bank Participation Report (BPR)
**Status:** BELUM ada di codebase.  
**Sumber:** `https://www.cftc.gov/MarketReports/BankParticipation/dea{mon}{yy}f` (HTML)  
**Update:** Monthly (sekitar awal bulan).  
**Data:** Posisi derivatives FX, interest rates, commodities dari US banks dan non-US banks.  
**Akses:** Via Firecrawl HTML extraction.  
**Nilai:** Menunjukkan seberapa besar bank-bank besar (JPMorgan, Goldman, dll.) memegang posisi net. Complement ke DTCC PPD yang sudah ada.  
**Kompleksitas:** HTML table, perlu Firecrawl JSON extraction schema yang tepat.  
**Catatan:** Monthly update saja, cukup cache 7 hari.

---

#### 🟢 LOW VALUE: Stooq Daily Prices (Upgrade from Weekly)
**Status:** Stooq diimplementasi sebagai weekly fallback (`&i=w`).  
**Test result:** Stooq **memblokir** programmatic access pada April 2026 dengan pesan: "Write to www@stooq.com if you want to use our data". Tidak reliable sebagai sumber. Tidak perlu di-upgrade.  
**Rekomendasi:** Tetap pakai Yahoo Finance (daily) + TwelveData (intraday) sebagai primary.

---

#### 🟢 LOW VALUE: CoinGecko DeFi Metrics (Leverage existing)
**Status:** CoinGecko sudah terintegrasi (BTC dom, total mcap, active currencies, tickers).  
**Gap minor:** `defi_market_cap` dan `defi_to_eth_ratio` dari `api.coingecko.com/api/v3/global` belum diambil.  
**Nilai:** Minor enhancement — DeFi/ETH ratio berguna sebagai proxy DeFi sentiment.  
**Implementasi:** 2 baris tambahan di `fetchCryptoGlobal()`.

---

#### 🟢 LOW VALUE: DVOL dalam AI Context
**Status:** DVOL (Deribit BTC/ETH implied vol) sudah diambil di sentiment service dan dimunculkan di `/sentiment` formatter, tapi BELUM masuk ke AI unified outlook context builder (`unified_outlook.go`).  
**Gap:** `unified_outlook.go` tidak menggunakan `SentimentData.DVOLData` dalam prompt.  
**Nilai:** Crypto IV signal penting untuk bias analysis.  
**Implementasi:** Tambah ~5 baris di `unified_outlook.go` section market sentiment.

---

### 3. SUMBER BARU YANG DI-EVALUATE TAPI TIDAK DIREKOMENDASIKAN

| Sumber | Alasan tidak direkomendasikan |
|---|---|
| CoinGlass | Butuh API key bahkan untuk basic data |
| CryptoQuant | Free tier sangat terbatas, data penting di-gate |
| Glassnode | Free tier hampir tidak ada data berarti |
| Alpha Vantage free | 25 req/day — tidak cukup untuk production |
| BofA FMS | Hanya tersedia via media coverage, tidak scrapeable secara andal |
| CFTC BPR CSV | Format tidak tersedia, hanya HTML table di cftc.gov |

---

## Prioritas Task Berdasarkan Value/Effort

| Rank | Task | Value | Effort | Sumber |
|---|---|---|---|---|
| 1 | NAAIM Exposure Index via Firecrawl | HIGH | S | Firecrawl (sudah berbayar) |
| 2 | Binance Futures L/S + OI + Funding | MEDIUM | M | Free, no key |
| 3 | DVOL masuk AI context (unified_outlook) | MEDIUM | XS | Existing data, wiring only |
| 4 | CoinGecko DeFi metrics (defi_market_cap) | LOW | XS | Existing API, 2 lines |
| 5 | CFTC Bank Participation Report | MEDIUM | L | Firecrawl HTML extraction |

