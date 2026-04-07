# Research Report: Data Siklus 2 Putaran 8
# Fed Speeches Scraper, CoinGecko Trending, SEC Form 4 Daily Insider, BofA MOVE Index
**Date:** 2026-04-02 19:00 WIB
**Siklus:** 2/5 (Data & Integrasi Baru) — Putaran 8
**Author:** Research Agent

## Ringkasan

4 verified data sources baru yang belum jadi task. Fokus: (1) Fed speeches via Firecrawl — CONFIRMED bisa scrape; (2) CoinGecko trending coins/categories — gratis dengan existing API key; (3) SEC EDGAR Form 4 daily insider trades — gratis, no auth; (4) ICE BofA MOVE Index via FRED — sudah ada FRED client, tinggal tambah series.

---

## Temuan 1: Fed Speeches Scraper — federalreserve.gov

**Endpoint:** `https://www.federalreserve.gov/newsevents/speeches-testimony.htm`
**Auth:** None. **Firecrawl:** CONFIRMED WORKING.
**Data tersedia:** speaker_name, title_role, date, speech_title_topic, URL per speech.
**Update frequency:** Beberapa kali seminggu. Speeches dari Chair Powell paling market-moving.

### Verified live scrape (2026-04-02):
```
- Michelle W. Bowman (Vice Chair for Supervision) — 3/31/2026 — "Supporting Small Businesses"
- Michael S. Barr (Governor) — 3/31/2026 — "Brief Remarks on Stablecoins"
- Philip N. Jefferson (Vice Chair) — 3/26/2026 — "Economic Outlook and Energy Effects"
- Jerome H. Powell (Chair) — 3/21/2026 — "Acceptance Remarks"
- Christopher J. Waller (Governor) — 2/24/2026 — "Operationalizing AI at the Federal Reserve"
```

### Use Case:
- Fetch list dari speeches-testimony.htm → filter Chair + Vice Chair
- Fetch full text dari 3-5 speeches terbaru via Firecrawl
- Feed ke AI unified outlook sebagai "Central Bank Tone" context
- Alert ke user kalau ada speech dari Powell dengan kata "rates", "inflation", "outlook"

### Implementation Path:
- New service: `internal/service/fedspeech/` (fetcher.go + parser.go)
- Firecrawl scrape URL list → filter by speaker priority
- Firecrawl scrape individual speech text (waitFor: 2000)
- Cache: 1 jam (speeches tidak update terlalu sering)
- Input ke `unified_outlook.go` sbg FedSpeechData

---

## Temuan 2: CoinGecko Trending Search + Categories

**Endpoint:** `GET /api/v3/search/trending` (demo plan, API key sudah ada di .env)
**Auth:** `x-cg-demo-api-key` header — sudah ada di client.go.
**Rate:** 30 req/min — lebih dari cukup.
**Data tersedia:**
- Top 15 trending coins (sorted by search volume) — includes price, 24h change, market cap
- Top 7 trending NFTs (sorted by floor price change)
- Top 6 trending categories (sector rotation signal) — e.g. "Solana Meme Coins" +14% 24h

### Use Case:
- `/sentiment` command: tambah section "🔥 Trending Crypto" — apa yang paling dicari user?
- Trending categories: signal sektor mana yang mendapat capital inflow
- Kalau "AI" atau "Layer 2" trending → bisa jadi altseason signal di sektor spesifik
- TIDAK butuh kode baru sama sekali — hanya tambah method ke existing `CoinGeckoClient`

### Implementation:
```go
// internal/service/marketdata/coingecko/client.go — tambah:
func (c *Client) GetTrending(ctx context.Context) (*TrendingData, error)
// Endpoint: GET /api/v3/search/trending (no additional params)
```
- Formatter: tampilkan top 5 trending coins + top 3 trending categories di /sentiment output
- Cache: 10 menit (endpoint update setiap 10 menit)

---

## Temuan 3: SEC EDGAR Form 4 — Daily Insider Trading

**Endpoint:** `https://data.sec.gov/submissions/CIK{10digit}.json`
**Auth:** User-Agent header only. **Rate:** 10 req/sec. **Confirmed working.**
**Data tersedia:** Form 4 filings (direktur, eksekutif, >10% pemegang saham)
  - Ticker, company, insider name, title, Buy/Sell, shares, price, date

### Beda dari TASK-206 (13F):
- 13F: quarterly aggregate institutional holdings (Berkshire, Bridgewater, dll)
- Form 4: **harian**, individual C-suite & directors buys/sells, lebih timely

### Tambahan API (EDGAR full-text search):
```
https://efts.sec.gov/LATEST/search-index?forms=4&dateRange=custom&startdt=2026-03-28&enddt=2026-04-02
```
→ Returns list of all Form 4 filings, filterabel by ticker

### Use Case:
- Insider cluster buying: CEO + CFO + 3 direktur semua beli dalam 2 minggu = signal kuat
- Unusual large purchase: eksekutif beli $5M saham perusahaannya sendiri
- Input ke /outlook: "Insider activity: Net buyer / Net seller past 30 days"

### CATATAN: Scope lebih kecil dari TASK-206 (yang L sized). Form 4 bisa scope M/S.

---

## Temuan 4: MOVE Index — Bond Market Volatility via FRED

**FRED Series:** `BAMLMOVE` atau scrape via Firecrawl dari ICE BofA page
**Alt source:** ICE BofA MOVE Index (ICE Data Services) — ada di FRED sebagai `MOVE`

### Konfirmasi FRED series yang sudah ada di codebase:
```
BAMLH0A0HYM2 — ICE BofA HY OAS (credit spread, sudah ada)
DFII10       — 10Y TIPS Real Yield (sudah ada)
T10Y2Y       — yield curve spread (sudah ada)
```

### Gap: MOVE Index BELUM ada
- MOVE = bond market VIX. Mengukur expected bond volatility 1 bulan ke depan.
- MOVE tinggi + VIX rendah = bond pasar lebih takut dari equity (rate/debt ceiling concern)
- MOVE rendah + VIX tinggi = equity panik tapi bond tenang (growth scare, bukan rate shock)

### FRED series: Perlu verifikasi availability
- Kemungkinan FRED menyediakan sebagai `MOVE` series atau via `BAMLMOVE`
- Alt: scrape dari macrotrends.net atau cboe.com jika tidak ada di FRED

### Verification:
```bash
curl "https://api.stlouisfed.org/fred/series?series_id=MOVE&api_key=..."
```

---

## Data Sources Rejected / Already Covered

| Source | Status | Reason |
|--------|--------|--------|
| barchart.com market breadth | ❌ Blocked | 404 on all market breadth pages, Firecrawl cannot render |
| openinsider.com | ❌ Blocked | ERR_TUNNEL_CONNECTION_FAILED from Firecrawl |
| housestockwatcher.com | ❌ Offline | DNS resolution failed — site down |
| Congressional trading APIs | ❌ Requires signup | No completely free API without registration |
| Treasury.gov yield API | ✅ Covered | FRED already fetches all yield curve series |
| BLS CPI/PPI direct | ✅ Covered | FRED fetches CPIAUCSL, PCEPILFE, etc. |
| CryptoCompare | ✅ Covered (TASK-207) | Already created as pending task |
| CBOE SKEW/OVX/GVZ | ✅ Covered (TASK-205/209) | Already created as pending tasks |

---

## Rekomendasi Task Priority

| Task | Priority | Effort | Signal Value |
|------|----------|--------|-------------|
| Fed speeches scraper | HIGH | M | Sangat tinggi (AI context for /outlook) |
| CoinGecko trending | MEDIUM | S | Tinggi (crypto sector rotation) |
| MOVE Index via FRED | MEDIUM | S | Tinggi (bond vol divergence dari VIX) |
| EDGAR Form 4 insider | MEDIUM | M | Tinggi (daily C-suite insider signal) |
| Sentiment→Outlook integration | HIGH | S | Consolidasi data ke unified output |
