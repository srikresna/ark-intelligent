# Research Report — Siklus 2: Data & Integrasi Baru Gratis
**Date:** 2026-04-01 14:00 WIB  
**Focus:** DATA_SOURCES_AUDIT.md — gap analysis, free APIs, peluang Firecrawl

---

## Ringkasan Eksekutif

Dari audit mendalam codebase + verifikasi endpoint langsung, ditemukan **5 peluang integrasi data gratis** yang belum diimplementasi dan langsung bisa dieksekusi.

---

## Status Data Sources Saat Ini

### ✅ SUDAH ADA & BERJALAN
| Source | Service | Notes |
|---|---|---|
| CFTC Socrata | service/cot | Gratis, unlimited |
| FRED API | service/fred | Gratis dengan key — **PROBLEM: FRED_API_KEY tidak ada di .env/.env.example** |
| MQL5 Calendar | service/news | Scraping gratis |
| TwelveData | service/price | Freemium, sudah di .env |
| CoinGecko | service/marketdata/coingecko | Freemium, sudah di .env |
| Yahoo Finance | service/price | Gratis, no key, jadi fallback harian |
| Bybit Public | service/microstructure | Gratis, no key |
| Firecrawl | service/sentiment (CBOE, AAII) | Berbayar, sudah di .env |
| CNN Fear & Greed | service/sentiment | Gratis endpoint JSON |
| AAII Sentiment | service/sentiment | via Firecrawl |
| CBOE Put/Call | service/sentiment | via Firecrawl |
| Massive/Polygon | service/marketdata/massive | Berbayar, sudah di .env |
| EIA | service/price | Freemium, sudah ada key |

---

## Gap & Peluang Yang Ditemukan

### 🔴 CRITICAL GAP: FRED_API_KEY Tidak Dikonfigurasi

**Problem:** FRED fetcher memerlukan `FRED_API_KEY` untuk beroperasi. FRED tanpa key mengembalikan `400 Bad Request`. Namun `FRED_API_KEY` tidak ada di `.env` dan tidak ada di `.env.example`.

**Impact:** `/macro` command dan semua macro overlay mungkin mengembalikan data kosong secara silent. Regime classification, yield curve, labor indicators — semua bergantung FRED.

**Fix:** 
1. Tambah `FRED_API_KEY=` ke `.env.example` dengan instruksi registrasi gratis
2. Tambah validasi/warning di startup jika FRED_API_KEY tidak di-set

---

### 🟢 PELUANG 1: Crypto Fear & Greed Index (alternative.me)

**Endpoint:** `GET https://api.alternative.me/fng/?limit=N`  
**Status:** ✅ Verified live, gratis, no API key  
**Data saat ini:** 11/100 — "Extreme Fear" (signifikan!)  
**Update:** Daily

**Relevansi:** Bot sudah punya CNN F&G (saham/equities) dan AAII (retail sentiment). Crypto-specific F&G adalah pelengkap sempurna — terutama untuk `/cryptoalpha` dan `/sentiment`.

**Integration Point:**  
- Tambah `CryptoFearGreed` struct ke `SentimentData` di `internal/service/sentiment/sentiment.go`
- Endpoint JSON langsung, tidak perlu Firecrawl
- Integrate ke `/sentiment` command output

---

### 🟢 PELUANG 2: Fed Speeches RSS Feed

**Endpoint:** `GET https://www.federalreserve.gov/feeds/speeches.xml`  
**Status:** ✅ Verified live, gratis, no API key, XML RSS  
**Data:** 15 speeches tersedia, termasuk Powell (20 Mar), Barr (31 Mar, 26 Mar), Jefferson  
**Update:** Real-time (langsung setelah speech published)

**Relevansi:** Sangat tinggi untuk trader forex institusional. Powell speech = market-moving event. Saat ini bot tidak memiliki monitoring untuk Fed speeches. AI context builder (`context_builder.go`) mencantumkan "fed" sebagai keyword relevan tapi tidak ada data feed.

**Integration Point:**
- Buat `internal/service/news/fed_speeches.go` — RSS parser
- Tambah periodic check (setiap 30 menit) untuk speeches baru
- Alert ke user jika speech baru dari voting members (Powell, vice chair)
- Feed ke AI context builder untuk enrichment `/outlook`

---

### 🟢 PELUANG 3: Fed FOMC Monetary Policy RSS

**Endpoint:** `GET https://www.federalreserve.gov/feeds/press_monetary.xml`  
**Status:** ✅ Verified live, gratis, no API key, XML RSS  
**Data:** FOMC statements, minutes releases, SEP projections  
**Latest:** Mar 18 2026 — FOMC statement + projections release

**Relevansi:** FOMC statement adalah single most important monetary policy event. Bot sekarang tidak bisa detect FOMC statement release secara programmatic — hanya bisa lewat MQL5 calendar (tapi tidak ada konten statementnya).

**Integration Point:**
- Bisa digabung dengan TASK Fed Speeches (service yang sama)
- Alert khusus untuk FOMC statement release
- Snippet ringkasan bisa diambil via Firecrawl dari URL yang ada di RSS

---

### 🟢 PELUANG 4: DeFiLlama Total TVL API

**Endpoint:** `GET https://api.llama.fi/v2/historicalChainTvl`  
**Status:** ✅ Verified live, gratis, no API key  
**Data:** Time series TVL semua DeFi (saat ini ~$94B)  
**Update:** Daily  

**Relevansi:** DeFi TVL adalah indikator risk appetite crypto institusional. TVL naik = DeFi bullish, TVL turun = risk-off. Relevan untuk `/cryptoalpha` dan overlay market context.

**Endpoint tambahan yang bekerja:**
- `GET https://api.llama.fi/v2/chains` — TVL per chain (Ethereum, BSC, dll)
- `GET https://api.llama.fi/v2/protocols` — top protocols by TVL

**Integration Point:**
- Tambah ke `internal/service/marketdata/` → `defillama/client.go`
- Integrate ke `/cryptoalpha` output sebagai crypto market health indicator
- TVL trend (naik/turun 7d, 30d) lebih berguna dari absolute level

---

### 🟡 PELUANG 5: FRED TGA Balance (WDTGAL)

**Endpoint:** FRED API — series `WDTGAL` (Treasury General Account balance)  
**Status:** ✅ Available via FRED (requires free API key — tapi FRED_API_KEY tidak ada!)  
**Data:** Weekly TGA balance (billions USD) — critical liquidity metric  

**Relevansi:** TGA balance adalah leading indicator untuk US dollar liquidity. TGA drawdown = tambahan likuiditas ke pasar = USD lemah + risk assets naik. Saat ini FRED service tidak track TGA.

**Integration Point:**
- Tambah `TGABalance float64` ke `MacroData` struct di `fred/fetcher.go`
- Tambah series `WDTGAL` ke batch fetch list
- Include di `/macro` output dan liquidity section
- **Dependency:** Butuh FRED_API_KEY dulu (lihat CRITICAL GAP di atas)

---

## Analisis Codebase — Temuan Tambahan

### Struktur Sentimen Saat Ini (sentiment.go)
```
SentimentData = {
  AAII: bull/bear/neutral %  ← via Firecrawl ✅
  CNN F&G: 0-100             ← JSON endpoint ✅
  CBOE Put/Call: ratio       ← via Firecrawl ✅
  CryptoF&G: MISSING         ← Gap ditemukan ❌
}
```

### Fed Data Gap di Context Builder
`context_builder.go` line 41 mencantumkan `"fed"` sebagai keyword relevan untuk enrichment, tapi tidak ada service yang fetch Fed speeches/statements untuk diinject ke context. Saat ini AI hanya bisa rely pada pengetahuan training-nya.

### DeFiLlama Integration Path
`internal/service/marketdata/` sudah punya struktur untuk external data (bybit/, coingecko/, massive/). DeFiLlama client bisa dibuat mengikuti pola yang sama.

---

## Prioritas Eksekusi

| Task | Impact | Effort | Priority |
|---|---|---|---|
| FRED_API_KEY di .env.example | CRITICAL | Low | 🔴 P0 |
| Crypto F&G (alternative.me) | High | Low | 🟢 P1 |
| Fed Speeches + FOMC RSS | High | Medium | 🟢 P1 |
| DeFiLlama TVL | Medium | Low | 🟡 P2 |
| TGA Balance via FRED | Medium | Low (post-FRED fix) | 🟡 P2 |

