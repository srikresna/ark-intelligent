# Research: Data Sources Audit — Gap Analysis & New Integrations
**Siklus:** 2/5 — Data Sources Audit | Putaran 13
**Date:** 2026-04-02 09:00 WIB
**Agent:** Research Agent

---

## Ringkasan Eksekutif

Audit mendalam terhadap data sources yang sudah ada vs. yang belum diintegrasikan.
Ditemukan 5 gap signifikan: 2 service internal yang sudah dibuat tapi belum masuk
unified_outlook, dan 3 sumber data gratis baru yang belum diimplementasikan.

---

## Status Data Sources Saat Ini

### ✅ Fully Implemented & Integrated

| Sumber | Service | Unified Outlook |
|---|---|---|
| CFTC COT | ✅ | ✅ |
| FRED Macro | ✅ | ✅ |
| MQL5 Economic Calendar | ✅ | ✅ |
| TwelveData price | ✅ | ✅ |
| Polygon.io historical | ✅ | ✅ |
| CNN Fear & Greed | ✅ sentiment.go | ✅ |
| AAII Sentiment | ✅ sentiment.go via Firecrawl | ✅ |
| CBOE Put/Call | ✅ sentiment/cboe.go via Firecrawl | ✅ |
| Crypto F&G (alternative.me) | ✅ sentiment.go | ✅ |
| CoinGecko | ✅ | ✅ |
| Bybit public | ✅ microstructure/gex | ✅ /alpha cmd only |
| World Bank | ✅ worldbank/ | ✅ |
| BIS REER | ✅ bis/ | ✅ |
| Deribit GEX | ✅ gex/ | ✅ /gex cmd only |

### ⚠️ Service Ada, Belum Masuk Unified Outlook

#### 1. VIX Term Structure (vix/ package)
- **Path:** `internal/service/vix/`
- **Data:** VIX spot, VX M1/M2 futures dari CBOE CDN CSV (gratis, public)
- **Gaps:** `UnifiedOutlookData` tidak punya field `VIXTermData`
- **Nilai:** VVIX, VX contango/backwardation dengan angka presisi dari futures — lebih akurat
  dari FRED VIXCLS/VXVCLS untuk term structure signaling
- **Referensi:** `vix/fetcher.go:25`, `vix/types.go:6`, `vix/cache.go`
- **Note:** FRED sudah fetch VIXCLS/VXVCLS untuk regime basic. Dedicated VIX service
  memberikan VVIX + full M1/M2 futures settle prices untuk analisis lebih dalam.

#### 2. Microstructure Bybit (microstructure/ package)
- **Path:** `internal/service/microstructure/`
- **Data:** BTC/ETH orderbook imbalance, taker flow, OI change, funding rate
- **Gaps:** Hanya dipakai di `/alpha` command (handler_alpha.go), tidak di unified_outlook
- **Nilai:** Crypto microstructure = leading indicator untuk short-term direction,
  sangat relevan untuk unified outlook crypto pair (BTCUSD, ETHUSD)
- **Referensi:** `microstructure/engine.go:1`, `handler_alpha.go:36`

---

### 🆕 Sumber Baru Gratis — Belum Diimplementasikan

#### 3. FRED NYSE Market Breadth (ADVN, DECN, NHNL)
- **URL:** Sudah pakai FRED API — tinggal tambah series ID
- **Series yang tersedia (gratis, existing key):**
  - `ADVN` — NYSE Advancing Issues (daily)
  - `DECN` — NYSE Declining Issues (daily)
  - `NHNL` — NYSE New Highs minus New Lows (weekly)
- **Cara implement:** Tambah ke `fetcher.go` jobs list (baris ~272), tambah field ke
  `MacroData` struct, compute `AdvDecRatio` di composites, include di unified_outlook
- **Nilai:** Equity market breadth = health indicator pasar saham, berguna untuk
  risk sentiment section di unified_outlook. Jika breadth negatif + VIX tinggi → risk-off
- **Cost:** GRATIS — pakai existing FRED API key, 0 tambahan

#### 4. Fed Speeches RSS Scraper (federalreserve.gov)
- **URL:** `https://www.federalreserve.gov/feeds/speeches.xml`
- **Verified:** ✅ Accessible, returns XML dengan title + link per speech
- **Sample data:** Powell, Barr, Bowman, Jefferson, Cook, Waller, Miran speeches
- **Cara implement:** Native HTTP GET + XML parsing (encoding/xml). No Firecrawl needed.
  Fetch top 5 speech titles/dates → feed ke unified_outlook sebagai "Recent Fed Rhetoric" block
- **Nilai:** AI bisa assess hawkish/dovish tone dari judul speech terbaru, tanpa perlu baca
  full content (judul sudah informatif: "Economic Outlook and Monetary Policy",
  "Prospects for Shrinking the Fed's Balance Sheet", dll.)
- **Cost:** GRATIS — no API key, no Firecrawl credit

#### 5. OpenInsider Scraper (Firecrawl)
- **URL:** `https://openinsider.com`
- **Verified:** ✅ Scrapeable via Firecrawl — live data ter-extract (tested 2026-04-01)
- **Sample data:** Ticker, company, insider name, buy/sell type, value, date
- **Cara implement:** Firecrawl JSON extraction (existing key). Add `InsiderFlowData`
  struct to `SentimentData`, scrape weekly (cache 24h).
- **Nilai:** Cluster insider buying pada sektor tertentu = bullish leading signal.
  Berguna sebagai context tambahan untuk equity/risk sentiment analysis.
- **Frekuensi:** SEC Form 4 update real-time; cukup daily fetch, cache 24h

---

## Temuan Lain: Fed FOMC Minutes RSS

- **URL:** `https://www.federalreserve.gov/feeds/press_all.xml`
- RSS ini include press releases termasuk FOMC minutes, policy statements
- Lebih komprehensif dari speeches-only RSS
- Bisa filter by title keyword ("Minutes of the Federal Open Market Committee")

---

## Prioritas Implementasi

| # | Task | Effort | Value | Free? |
|---|---|---|---|---|
| 1 | FRED breadth (ADVN/DECN/NHNL) | XS | High | ✅ |
| 2 | VIX term struct → unified_outlook | S | High | ✅ |
| 3 | Microstructure → unified_outlook | S | Medium | ✅ |
| 4 | Fed speeches RSS scraper | S | Medium | ✅ |
| 5 | OpenInsider scraper via Firecrawl | M | Medium | Pakai existing FC key |

---

## Catatan Arsitektur

- Semua tambahan menggunakan sumber gratis atau key yang sudah ada
- Tidak ada perombakan arsitektur existing — hanya extending `UnifiedOutlookData`
  dan `MacroData`
- FRED breadth data paling mudah: tinggal tambah 3 series ID ke jobs list yang sudah ada
- VIX/microstructure integration memerlukan field baru di `UnifiedOutlookData` struct
  dan section baru di `BuildUnifiedOutlookPrompt()`
