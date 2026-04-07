# Research: Data Sources Audit — Putaran 18
**Siklus:** 2/5 (Data Sources Audit)
**Date:** 2026-04-02
**Researcher:** Research Agent

---

## Temuan Utama

### 1. Status Implementasi Data Sources (Audit Aktual)

#### ✅ SUDAH IMPLEMENT & AKTIF
| Service | Package | Command/Usage | Status |
|---|---|---|---|
| FRED Macro (60+ series) | `internal/service/fred/` | `/macro`, `/outlook`, `/bias` | ✅ Aktif |
| AAII Sentiment via Firecrawl | `internal/service/sentiment/sentiment.go` | `/sentiment` | ✅ Aktif |
| CNN Fear & Greed (public JSON) | `internal/service/sentiment/sentiment.go` | `/sentiment` | ✅ Aktif |
| Crypto Fear & Greed (alternative.me) | `internal/service/sentiment/sentiment.go` | `/sentiment` | ✅ Aktif |
| CBOE Put/Call via Firecrawl | `internal/service/sentiment/cboe.go` | `/sentiment` | ✅ Aktif |
| VIX Term Structure (CBOE public) | `internal/service/vix/` | `/macro`, `/outlook` | ✅ Aktif (via MergeSentiment) |
| MQL5 Economic Calendar | `internal/service/news/fetcher.go` | `/calendar`, `/alerts` | ✅ Aktif |
| BIS REER (free API, no key) | `internal/service/bis/reer.go` | `/outlook` | ✅ Aktif |
| WorldBank Global Macro | `internal/service/worldbank/client.go` | `/outlook` | ✅ Aktif |
| COT CFTC Socrata | `internal/service/cot/` | `/cot`, `/outlook` | ✅ Aktif |
| Bybit Microstructure | `internal/service/microstructure/engine.go` | `/quant` hanya | ⚠️ Tidak di /outlook |
| Deribit GEX | `internal/service/gex/engine.go` | `/gex` hanya | ⚠️ Tidak di /outlook |
| EIA Energy Data | `internal/service/price/eia.go` | `/seasonal` hanya | ⚠️ Tidak di /macro atau /outlook |
| Yahoo Finance Price | `internal/service/price/fetcher.go` | fallback untuk harga | ✅ Aktif sebagai fallback |
| TwelveData Intraday | `internal/service/price/intraday_fetcher.go` | `/intraday` | ✅ Aktif |
| Massive/Polygon Historical | `internal/service/marketdata/massive/` | price history | ✅ Aktif |
| Bybit Public API | `internal/service/marketdata/bybit/` | crypto data | ✅ Aktif |
| CoinGecko | `internal/service/marketdata/coingecko/` | BTC dominance dll | ✅ Aktif |

---

### 2. GAP KRITIS: Data Tersedia Tapi Tidak Di-Inject ke /outlook

#### GAP-DS1: EIA Energy Data hanya di /seasonal, tidak di /macro atau /outlook
- **File:** `internal/service/price/eia.go` — implementasi lengkap ✅
- **Problem:** EIAClient hanya dipanggil di `handler_seasonal.go` (line 96-101)
- **Dampak:** Analisis USDCAD, AUDUSD, USDNOK tidak mendapat crude inventory context di /outlook
- **Fix:** Inject EIASeasonalData ke UnifiedOutlookData → BuildUnifiedOutlookPrompt
- **Cost:** $0 (EIA_API_KEY sudah ada di .env atau gratis daftar)

#### GAP-DS2: GEX tidak di /outlook
- **File:** `internal/service/gex/engine.go` — implementasi lengkap ✅
- **Problem:** GEX Engine hanya dipakai di `/gex` command (handler_gex.go)
- **Dampak:** UnifiedOutlookData tidak punya gamma exposure — AI tidak bisa analisis key gamma levels BTC/ETH
- **Fix:** Tambah `GEXData map[string]*gex.GEXResult` ke `UnifiedOutlookData`, fetch di handler.go
- **Cost:** $0 (Deribit public API, no key required)

#### GAP-DS3: Microstructure signals tidak di /outlook
- **File:** `internal/service/microstructure/engine.go` — implementasi lengkap ✅
- **Problem:** Microstructure Engine hanya dipakai via quant command
- **Dampak:** AI tidak dapat orderbook imbalance, taker flow, OI momentum saat generate unified outlook
- **Fix:** Tambah `MicrostructureData []microstructure.Signal` ke `UnifiedOutlookData`
- **Cost:** $0 (Bybit public API)

---

### 3. SUMBER BARU GRATIS — BELUM DIIMPLEMENTASI

#### SOURCE-NEW-1: Fed Speeches RSS Feed (GRATIS, no key, no Firecrawl)
- **URL:** `https://www.federalreserve.gov/feeds/speeches.xml`
- **Format:** XML RSS, bisa di-parse langsung dengan Go `encoding/xml`
- **Data:** Speaker (Powell, Waller, Bowman, dll), judul speech, tanggal, URL
- **Verifikasi:** ✅ Tested — feed aktif, berisi 15+ speech terbaru
- **Contoh:** "Waller, Labor Market Data: Signal or Noise?", "Barr, Brief Remarks on Monetary Policy"
- **Use case:** Kategorikan hawkish/dovish berdasarkan judul → inject ke AI context sebagai Fed Tone Signal
- **Implementasi:** ~150 baris Go, package baru `internal/service/fedspeech/`
- **Catatan:** Tidak butuh Firecrawl, tidak butuh key. Pure Go HTTP + XML parsing.

#### SOURCE-NEW-2: OpenInsider Cluster Buys (GRATIS via Firecrawl)
- **URL:** `https://openinsider.com/latest-cluster-buys`
- **Format:** HTML table, parseable via Firecrawl JSON extraction
- **Data:** Filing date, ticker, company, industry, # insiders buying, value ($), ΔOwn%
- **Verifikasi:** ✅ Tested via Firecrawl — data lengkap tersedia
- **Contoh data aktual (2026-04-01):** 100 cluster buys, KKR insiders beli $42M, AHCO insiders beli $24M
- **Use case:** Insider cluster buying = smart money risk-on signal. Counter-signal untuk equity/FX risk sentiment
- **Implementasi:** Firecrawl scrape + JSON schema extraction, tambah ke SentimentData
- **Note:** Relevan untuk context: "Are insiders buying the dip?" → bullish risk-on signal untuk AUD, NZD, CAD

---

### 4. IMF DATA API (GRATIS, no key)

- **URL:** `https://dataservices.imf.org/REST/SDMX_JSON.svc/`
- **Data tersedia gratis:**
  - Global current account balances
  - External debt statistics
  - World Economic Outlook projections (GDP growth, inflation per negara)
- **Use case:** Context tambahan untuk FX fundamental: current account deficits, external debt
- **Contoh request:** `https://dataservices.imf.org/REST/SDMX_JSON.svc/CompactData/IFS/...`
- **Assessment:** Lower priority — WorldBank API sudah cover sebagian. Berguna untuk current account data per negara yang belum ada di FRED.
- **Task priority:** Low (nice to have)

---

### 5. DATA SOURCE YANG SUDAH OPTIMAL (Jangan Dirombak)

- **Yahoo Finance** — sudah jadi fallback di price fetcher ✅
- **Stooq.com** — tidak perlu, Yahoo Finance sudah cover sebagai fallback
- **Alpha Vantage** — tidak perlu, TwelveData sudah ada
- **CBOE VIX data** — sudah implement via vix service ✅
- **Quandl/NASDAQ Data Link** — tidak perlu, FRED lebih comprehensive

---

## Prioritas Tasks

| # | Task | Impact | Effort | Free? |
|---|---|---|---|---|
| 1 | GEX di /outlook | HIGH | S | ✅ |
| 2 | Fed Speeches RSS Parser | HIGH | S | ✅ gratis (no key, no Firecrawl) |
| 3 | Microstructure di /outlook | MEDIUM | S | ✅ |
| 4 | EIA di /outlook + /macro | MEDIUM | M | ✅ |
| 5 | OpenInsider via Firecrawl | MEDIUM | M | Firecrawl (sudah punya key) |

---

## Referensi
- `internal/service/price/eia.go` — EIAClient
- `internal/service/gex/engine.go` — GEX Engine
- `internal/service/microstructure/engine.go` — Microstructure Engine
- `internal/service/ai/unified_outlook.go` — UnifiedOutlookData struct
- `internal/adapter/telegram/handler.go:1004` — unified data assembly
- Fed RSS feed: https://www.federalreserve.gov/feeds/speeches.xml (verified working)
- OpenInsider: https://openinsider.com/latest-cluster-buys (verified via Firecrawl)
