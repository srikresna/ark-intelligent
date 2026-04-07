# Research: Data Sources Audit — Putaran 23
**Siklus:** 2/5 (Data Sources Audit)
**Date:** 2026-04-04
**Researcher:** Research Agent

---

## Status Data Sources Saat Ini (Update dari Putaran 18)

### ✅ Sudah Lengkap (tidak perlu task baru)
| Service | Status |
|---|---|
| FRED Macro (60+ series termasuk DXY, yield spreads, CPI, dll) | ✅ Aktif |
| AAII Sentiment via Firecrawl | ✅ Aktif |
| CNN Fear & Greed (public JSON) | ✅ Aktif |
| Crypto Fear & Greed (alternative.me) | ✅ Aktif |
| CBOE Put/Call via Firecrawl | ✅ Aktif |
| VIX Spot + M1 + M2 + VVIX (CBOE) | ✅ Aktif |
| MQL5 Economic Calendar | ✅ Aktif |
| BIS REER (free API, no key) | ✅ Aktif |
| WorldBank Global Macro | ✅ Aktif |
| COT CFTC Socrata | ✅ Aktif |
| CentralBankRateMapping (8 currencies via FRED OECD series) | ✅ Aktif di carry engine |
| Bybit Microstructure (OI history, long/short, taker flow) | ✅ Aktif (via /quant) |
| EIA Energy Data | ✅ Aktif (via /seasonal) |
| Deribit GEX | ✅ Aktif (via /gex) |
| Yahoo Finance (price fallback) | ✅ Aktif |
| TwelveData Intraday | ✅ Aktif |
| CoinGecko TOTAL3 (price-only) | ✅ Aktif |

---

## Temuan Baru — Gaps yang Belum Ditask

### GAP-DS-23-1: NAAIM Exposure Index — Data Source Baru, Belum Pernah Ditask

**Verified:** ✅ Fully verified live
**URL:** `https://naaim.org/programs/naaim-exposure-index/`
**Data format:** Excel download (direct URL), tidak perlu auth
**Frequency:** Weekly (setiap Rabu), update Kamis

**Data yang tersedia (2026-03-25):**
- NAAIM Number (Mean exposure): 68.52%
- Median: 82%
- Q1 (25th pctile): 31.75%
- Q3 (75th pctile): 100%
- Most Bearish: -0.14% (leverage short)
- Most Bullish: 200% (2x leveraged long)

**Excel URL pattern:** `https://naaim.org/wp-content/uploads/YYYY/MM/USE_Data-since-Inception_YYYY-MM-DD.xlsx`
- URL berubah tiap minggu (tanggal survei)
- Perlu scrape halaman NAAIM untuk dapat URL terbaru → lalu HTTP GET xlsx langsung
- Verified: HTTP GET xlsx returns 200 OK, Content-Type: xlsx, ukuran ~85KB

**Why important:**
- NAAIM diisi oleh fund managers, bukan retail → institutional positioning signal
- Mean > 80: manager bullish berlebihan → potential top
- Mean < 30: manager risk-off besar → potential bottom
- Divergence NAAIM bullish + VIX tinggi = mixed signal (caution)

**Implementation path:**
1. Firecrawl scrape `naaim.org/programs/naaim-exposure-index/` → extract current Excel URL
2. `http.Get(excelURL)` → download xlsx (~85KB)
3. Parse xlsx dengan `github.com/xuri/excelize/v2` — cek apakah sudah di go.mod
4. Ambil 2 baris terbaru (latest + prev week for trend)
5. Tambah ke `SentimentData`: `NAAIMExposure float64`, `NAAIMWeekDate string`, `NAAIMAvailable bool`
6. Tampilkan di `/sentiment` output

**Cek go.mod excelize:**
```bash
grep "excelize" ~/ark-intelligent/go.mod
```

---

### GAP-DS-23-2: CoinGecko Global Data — Dead Code Belum Diwire

**Verified:** ✅ Methods ada di codebase, tapi 0 caller di luar package
**Files affected:** `internal/service/marketdata/coingecko/client.go`

Dead methods yang **belum pernah dipanggil** di luar package:
- `GetGlobalData()` → total market cap, market cap 24h change%, active cryptos
- `GetBTCDominance()` → % BTC of total market cap (saat ini 2026-04: ~55%)
- `GetAltcoinMarketCap()` → altcoin market cap approx (total minus BTC minus ETH)
- `GetMarketSentiment()` → derived score 0-100 dari market cap 24h change

Verified via grep: `grep -rn "GetGlobalData\|GetBTCDominance\|GetAltcoinMarketCap\|GetMarketSentiment" internal/ | grep -v "client.go" | wc -l` → **0**

**CoinGecko API key sudah di .env** (COINGECKO_API_KEY). Rate: 30 req/min gratis.

**What to display in /sentiment:**
```
🌐 Crypto Market (CoinGecko)
Total MCap : $2.45T (24h: +1.2%)
BTC Dom    : 55.3%
Altcoin Cap: ~$740B
Signal     : NEUTRAL (score 56/100)
```

**Also valuable as fallback:** Ketika alternative.me Crypto F&G gagal, bisa gunakan `GetMarketSentiment()` sebagai fallback score.

---

### GAP-DS-23-3: Sentiment Data Age — Tidak Ditampilkan ke User

**Issue:** `SentimentData.FetchedAt` ada di struct dan diisi dengan `time.Now()`, tapi **tidak pernah ditampilkan** di formatter.

AAII survey adalah **mingguan** (update tiap Rabu). Jika user request /sentiment hari Selasa, AAII data sudah 6 hari lama. User tidak tahu ini.

CNN F&G updates lebih sering. Cache TTL = 6 jam. Bisa sudah stale beberapa jam.

**Fix:** Tambah "as of" di setiap section di `FormatSentiment()`:
- CNN: `Fear & Greed : 62 (Greed) • 3h ago`
- AAII: `Survei minggu 2026-03-25 (Wed)`
- CBOE: `Data: Kemarin (market close)`

**Also:** Tampilkan berapa sumber yang unavailable: `⚠️ 1/5 sumber tidak tersedia (CBOE)`

---

### GAP-DS-23-4: AAII Historical Weekly — Tidak Tersimpan, Tidak Ada Trend

**Issue:** AAII hanya fetch current week. Tidak ada comparison dengan 4-week average.

Informasi penting yang hilang:
- Apakah sentiment sedang *meningkat* atau *menurun*?
- AAII Bull-Bear Spread vs 4-week avg = trend signal
- Contoh: Bull-Bear = -15%, tapi 4 minggu lalu -30% → sentiment sedang pulih (bullish)

**Implementation:**
1. Buat tabel SQLite `aaii_history` (date, bullish, bearish, neutral, bull_bear_spread)
2. Setelah tiap fetch berhasil, INSERT IF NOT EXISTS berdasarkan weekdate
3. Hitung 4-week moving average dari historical
4. Tampilkan di /sentiment: trend arrow ↑↓ dan spread vs 4wk avg

---

### GAP-DS-23-5: /sentiment Source Availability Footer

**Issue:** Ketika source gagal (CBOE, AAII, dll), section kosong. User bingung apakah itu normal atau error.

**Current behavior:** Jika CBOE Firecrawl gagal → CBOE section tidak muncul sama sekali.
**Better:** Tampilkan footer diagnostics:
```
📡 Sumber Data: CNN ✅ AAII ✅ CBOE ❌ CryptoFG ✅ VIX ✅
❌ CBOE: Firecrawl timeout (coba lagi nanti)
```

Ini memberi user context mengapa data tidak lengkap.

---

## Check: Apakah excelize sudah di go.mod?

Perlu dicek sebelum implement TASK-305:
```bash
grep "excelize" ~/ark-intelligent/go.mod
```

Jika tidak ada: gunakan alternatif — Firecrawl JSON extraction dari NAAIM HTML untuk data terbaru (tanpa perlu xlsx parsing). Atau: `encoding/csv` kalau NAAIM punya CSV export.

---

## Prioritas Tasks Putaran 23

| # | Task | Impact | Effort | Source |
|---|---|---|---|---|
| 1 | NAAIM Exposure Index | HIGH | M | Free (naaim.org) |
| 2 | CoinGecko Global Data wiring | HIGH | S | Existing key |
| 3 | Sentiment Source Age Display | MEDIUM | S | UX fix |
| 4 | AAII 4-Week Historical SQLite | MEDIUM | M | UX + data quality |
| 5 | /sentiment Source Diagnostic Footer | LOW-MEDIUM | S | UX fix |

---

## Referensi

- `internal/service/sentiment/sentiment.go` — SentimentData struct (line 115)
- `internal/service/sentiment/cache.go` — cacheTTL = 6h (line 11)
- `internal/service/marketdata/coingecko/client.go` — dead methods (line 100, 219, 258, 272)
- NAAIM verified Excel: `https://naaim.org/wp-content/uploads/2026/03/USE_Data-since-Inception_2026-03-25.xlsx`
- NAAIM page: `https://www.naaim.org/programs/naaim-exposure-index/`
