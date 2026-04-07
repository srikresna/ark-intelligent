# Research Report — Siklus 2: Data & Integrasi Baru Gratis
**Tanggal:** 2026-04-06
**Agent:** Research
**Siklus:** 2 (Data & Integrasi Baru Gratis)
**Referensi:** .agents/DATA_SOURCES_AUDIT.md

---

## Ringkasan Eksekutif

Audit menyeluruh terhadap data layer codebase menunjukkan bahwa **sebagian besar
rekomendasi DATA_SOURCES_AUDIT.md sudah diimplementasikan** — AAII Sentiment, CNN F&G,
Fed Speeches, CBOE P/C, OpenInsider, DTCC, OECD CLI, ECB, SNB, BIS (3 dataset), IMF WEO,
World Bank, Eurostat, TradingEconomics, EIA, Stooq, Yahoo Finance (fallback) semuanya
sudah ada. Codebase jauh lebih mature dari yang tertulis di audit.

Riset ini mengidentifikasi celah data yang BENAR-BENAR belum ada dan bernilai tinggi.

---

## Inventarisasi Lengkap Data Sources (Status Aktual)

| Sumber | Status | Package |
|--------|--------|---------|
| CFTC COT (TFF + Disaggregated) | ✅ Aktif | service/cot |
| FRED (75+ series incl. ISM, VIX term) | ✅ Aktif | service/fred |
| OECD CLI | ✅ Aktif | service/macro |
| ECB SDW (MRR, M3, EURUSD) | ✅ Aktif | service/macro |
| Eurostat (GDP, CPI, unemployment) | ✅ Aktif | service/macro |
| SNB Balance Sheet | ✅ Aktif | service/macro |
| DTCC FX Swaps PPD | ✅ Aktif | service/macro |
| TradingEconomics global scraping | ✅ Aktif | service/macro |
| BIS Policy Rates (WS_CBPOL) | ✅ Aktif | service/bis |
| BIS Credit Gap (WS_CREDIT_GAP) | ✅ Aktif | service/bis |
| BIS REER (WS_EER) | ✅ Aktif | service/bis |
| IMF WEO | ✅ Aktif | service/imf |
| World Bank Macro | ✅ Aktif | service/fred + service/worldbank |
| TwelveData (intraday OHLCV) | ✅ Aktif | service/price |
| Stooq (historical FX) | ✅ Aktif | service/price/stooq.go |
| Yahoo Finance (FX fallback) | ✅ Aktif | service/price/fetcher.go |
| Alpha Vantage (secondary) | ✅ Aktif | service/price/fetcher.go |
| Massive/Polygon (historical) | ✅ Aktif | service/marketdata/massive |
| CoinGecko (BTC dom, TOTAL3) | ✅ Aktif | service/marketdata/coingecko |
| CryptoCompare (exchange vol) | ✅ Aktif | service/marketdata/cryptocompare |
| Bybit (crypto microstructure) | ✅ Aktif | service/marketdata/bybit |
| Deribit (options GEX/DVOL/skew) | ✅ Aktif | service/gex + service/marketdata/deribit |
| DefiLlama (TVL, DEX, stablecoins) | ✅ Aktif | service/marketdata/defillama |
| EIA (energy inventories) | ✅ Aktif | service/price/eia.go |
| CBOE VIX + Term Structure | ✅ Aktif | service/vix |
| CBOE P/C Ratio (via Firecrawl) | ✅ Aktif | service/sentiment/cboe.go |
| CNN Fear & Greed | ✅ Aktif | service/sentiment/sentiment.go |
| Crypto Fear & Greed | ✅ Aktif | service/sentiment/sentiment.go |
| AAII Sentiment (via Firecrawl) | ✅ Aktif | service/sentiment/sentiment.go |
| OpenInsider Cluster Buys (Firecrawl) | ✅ Aktif | service/sentiment/openinsider.go |
| Myfxbook Retail Positioning (Firecrawl) | ✅ Aktif | service/sentiment/myfxbook.go |
| Finviz Futures + Sectors (Firecrawl) | ✅ Aktif | service/marketdata/finviz |
| Fed Speeches tone classifier (Firecrawl) | ✅ Aktif | service/fred/speeches.go |
| US Treasury Auction results | ✅ Aktif | service/macro/treasury_client.go |
| SEC EDGAR 13F holdings | ✅ Aktif | service/sec |
| ISM New Orders (FRED NAPMNOI) | ✅ Aktif | service/fred/fetcher.go |

---

## Celah Data yang Ditemukan (Genuine Gaps)

### Gap 1: Atlanta Fed GDPNow [KRITIS - HIGH]

Real-time US GDP tracker dari Atlanta Fed, diupdate setiap 2-3 hari setelah
data ekonomi baru rilis. Ini lebih cepat dari FRED GDP series (quarterly, lagging).
GDPNow langsung mempengaruhi Fed rate expectations dan USD direction.

- URL: https://www.atlantafed.org/cqer/research/gdpnow
- Metode: Firecrawl JSON extraction (key sudah ada)
- Integrasi: /macro dashboard + /outlook context builder
- Status: BELUM ADA di seluruh codebase
- Output contoh: "GDPNow Q2 2026: +2.1% annualized (prev: +1.8%) ↑ USD-bullish"

### Gap 2: Market Breadth via Barchart [MEDIUM]

% saham di atas 50MA/200MA, new 52W highs vs lows, A/D ratio.
Saat ini /sentiment punya: VIX, AAII, CNN F&G, Crypto F&G, myfxbook, OpenInsider,
CBOE P/C — tapi TIDAK ADA equity market breadth indicator. Market breadth adalah
filter kritis untuk membedakan "healthy bull" vs "narrow rally" (topping risk).

- URL: https://www.barchart.com/stocks/market-pulse
- Metode: Firecrawl JSON extraction
- Integrasi: /sentiment command — tambah "Market Breadth Health Score"
- Status: BELUM ADA (disebut di DATA_SOURCES_AUDIT.md tapi belum diimplementasikan)

### Gap 3: COT Open Interest Trend Analysis [MEDIUM]

Data OI sudah ada di domain model (domain.COTRecord.OpenInterest, OpenInterestChg,
OIPctChange) dan difetch dari CFTC sejak awal. Tapi tidak ada:
1. OI trend visualization di output /cot
2. "OI expansion + net long increase = institutional accumulation" signal
3. "OI contraction from extreme + price stagnant = unwind risk" alert

Ini adalah "data free win" — tidak perlu sumber baru, hanya analysis layer baru
di service/cot/signals.go dan service/cot/confluence_score.go.

- Signals yang bisa ditambah:
  * OI_EXPANSION_BULL: 3 weeks consecutive OI up + net longs up → accumulation
  * OI_CONTRACTION_EXTREME: OI down from 90th percentile + price flat → unwind
  * OI_DIVERGENCE: OI up + net position flipping → dealers vs speculators clash

### Gap 4: OECD Consumer Confidence / Business Climate [MEDIUM]

ISM New Orders (FRED NAPMNOI) sudah ada, tapi ini hanya US manufacturing.
Missing: Consumer Confidence — leading indicator untuk consumer spending (70% GDP).

- OECD menyediakan CCI dan BCI gratis via SDMX API (sama dengan CLI yang sudah ada)
- URL: https://sdmx.oecd.org/public/rest/data/OECD.SDD.STES,DSD_STES@DF_MEI_CLI
- Flow: Mirip dengan oecd_client.go yang sudah ada — extend dengan dataset CCI/BCI
- Countries: US, EU, UK, JP, AU, CA, CH, CN
- Integrasi: /leading command (sudah ada) — tambah CCI/BCI section

### Gap 5: BofA Global Fund Manager Survey [LOW]

Survey bulanan ~300 fund manager global (AUM >$600B). Key metrics: cash levels
(>5% = risk-off extreme), equity overweight/underweight, most crowded trades.
Data tidak ada public API — hanya via media coverage saat rilis (minggu ke-3 bulanan).

- Metode: Firecrawl scrape dari BofA securities site atau Reuters saat rilis
- Kompleksitas: TINGGI (irregular release, no structured endpoint)
- Rekomendasi: LOW priority, deferkan ke siklus berikutnya

---

## Prioritas Task Siklus 2

| Task | Topik | Priority | Effort |
|------|-------|----------|--------|
| TASK-006 | Atlanta Fed GDPNow via Firecrawl | HIGH | S |
| TASK-007 | Market Breadth via Barchart Firecrawl | MEDIUM | M |
| TASK-008 | COT OI Trend Analysis (no new data) | MEDIUM | M |
| TASK-009 | OECD Consumer Confidence (extend CLI client) | MEDIUM | S |
| TASK-010 | BofA FMS monthly scraper | LOW | L |

---

## Catatan Arsitektur

Semua gap 1-4 dapat diimplementasikan dengan pola yang SUDAH ADA di codebase:
- Gap 1 (GDPNow): ikuti pola fred/speeches.go (Firecrawl + cache + graceful degradation)
- Gap 2 (Breadth): ikuti pola sentiment/cboe.go (Firecrawl JSON extraction)
- Gap 3 (OI Trend): extend signals.go, tidak perlu fetcher baru
- Gap 4 (CCI): extend oecd_client.go (fungsi fetchOECDSeries sudah general purpose)

Tidak ada dependensi eksternal baru diperlukan. Semua pakai Firecrawl atau SDMX API gratis.
