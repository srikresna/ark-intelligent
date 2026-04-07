# Research Siklus 3 / Putaran 9: Feature Index Gap Analysis

**Tanggal:** 2026-04-02 21:00 WIB  
**Siklus:** 3/5 (FEATURE_INDEX.md)  
**Putaran:** 9  
**Fokus:** Identifikasi gap di feature yang sudah ada + fitur potensial bernilai tinggi yang belum diimplementasi

---

## Metodologi

1. Baca FEATURE_INDEX.md secara menyeluruh
2. Analisis kode di service-service utama: `vix/`, `sentiment/`, `fred/`, `ict/`, `gex/`, `wyckoff/`, `ta/`
3. Cross-reference command list di `handler.go` dan formatter
4. Identifikasi gap antara "apa yang ada di FEATURE_INDEX" vs "apa yang sebenarnya diimplementasi"
5. Verifikasi ketersediaan data source (GRATIS) untuk gap yang ditemukan

---

## Temuan 1: CBOE SKEW Index belum ada di service/vix/

**Status:** GAP KRITIS

### Apa yang sudah ada:
- `service/vix/fetcher.go` mengambil: VIX spot (`VIX_EOD.csv`), VX Futures M1/M2/M3 (`VX_EOD.csv`), VVIX (`VVIX_EOD.csv`)
- `VIXTermStructure` struct sudah punya Contango/Backwardation/SlopePct/RollYield/Regime
- `/sentiment` command menampilkan VIX term structure lewat composites

### Apa yang HILANG:
**CBOE SKEW Index** — mengukur tail risk / cost of OTM puts vs ATM. Nilai >145 = tail risk elevated (crash hedging aktif), nilai <115 = low tail risk / complacency.

### Data Source (GRATIS, sudah dipakai pola sama):
```
URL: https://cdn.cboe.com/api/global/us_indices/daily_prices/SKEW_EOD.csv
Format: Date,Open,High,Low,Close (sama persis dengan VIX_EOD.csv dan VVIX_EOD.csv)
```

**Pola fetching sudah ada** di `fetchSingleIndexCSV()` — cukup tambah 1 URL constant dan 1 field `SKEW float64` di `VIXTermStructure`.

### SKEW Signal Classification:
| SKEW | Interpretasi |
|------|-------------|
| >150 | EXTREME TAIL RISK — heavy crash hedging, pasar takut |
| 140-150 | ELEVATED — tail hedging aktif |
| 120-140 | NORMAL — typical range |
| 110-120 | LOW — complacency, put-buyers tenang |
| <110 | VERY LOW — extreme complacency |

### Cross-signal VIX + SKEW:
- VIX rendah + SKEW tinggi = institutions hedge diam-diam → WARNING SETUP
- VIX tinggi + SKEW rendah = panic selling tanpa hedging → potential capitulation
- VIX rendah + SKEW rendah = genuine risk-on environment

---

## Temuan 2: FRED IG Credit Spread belum ditrack (BAMLC0A0CM)

**Status:** GAP PENTING

### Apa yang sudah ada:
- `fred/fetcher.go` mengambil **HY OAS** (`BAMLH0A0HYM2`) dan menyimpannya di field `TedSpread` (field name misleading)
- Digunakan di `regime.go` sebagai credit stress proxy

### Apa yang HILANG:
**Investment Grade OAS** (`BAMLC0A0CM`) — ICE BofA US Corporate AAA-A Option-Adjusted Spread.

**Mengapa penting:**
1. **HY-IG Ratio** = `BAMLH0A0HYM2 / BAMLC0A0CM` → credit quality ratio. Semakin tinggi = pasar semakin mendiskon junk vs investment grade → risk-off signal
2. IG spread sendiri adalah early warning indicator — naik sebelum HY spread naik
3. Lebih sensitif terhadap liquidity stress early-stage

### Verifikasi (confirmed working):
```
curl "https://fred.stlouisfed.org/graph/fredgraph.csv?id=BAMLC0A0CM"
→ observation_date,BAMLC0A0CM
→ 1996-12-31,0.60   ✅ Data valid
```

### Implementasi:
- Tambah field `IGSpread float64` di `MacroData` struct
- Tambah `IGSpreadTrend SeriesTrend`
- Tambah ke series fetch list di `fetchAll()`
- Tambah computed field `HYIGRatio float64 = TedSpread / IGSpread`
- Tampilkan di `/macro` → Credit section

---

## Temuan 3: ICT PD Array / Premium-Discount Zone belum ada

**Status:** GAP MODERAT (high leverage feature)

### Apa yang sudah ada di `service/ict/`:
- `fvg.go`: Fair Value Gap detection ✅
- `orderblock.go`: Order Block detection ✅
- `structure.go`: CHoCH/BOS detection ✅
- `liquidity.go`: Liquidity Sweep detection ✅
- `swing.go`: Swing point detection ✅

### Apa yang HILANG:
**PD Array dengan Premium/Discount Zone classification**

#### Konsep ICT PD Array (urutan prioritas dari paling bullish ke bearish):
1. **Bullish PD Array** (beli di sini): FVG Bullish, Bullish Order Block, Breaker Block (dari bearish OB yang broke)
2. **Bearish PD Array** (jual di sini): FVG Bearish, Bearish Order Block, Breaker Block (dari bullish OB yang broke)

#### Premium vs Discount Zone:
- Tentukan **swing range** (high to low dari swing terakhir yang terdeteksi)  
- **Equilibrium** = 50% dari range
- **Discount Zone** = 0%-50% (price below equilibrium) → look for buys
- **Premium Zone** = 50%-100% (price above equilibrium) → look for sells
- **OTE Zone** = Fibonacci 62%-79% → Optimal Trade Entry (confluence dengan PD Array)

#### Implementasi yang diusulkan:
```go
// PDArrayResult ranks current market relative to PD arrays
type PDArrayResult struct {
    CurrentZone    string  // "PREMIUM" | "DISCOUNT" | "EQUILIBRIUM"
    ZonePct        float64 // 0-100, where 50 = equilibrium
    OTEActive      bool    // true if price in 62-79% retracement zone
    TopPDArray     string  // highest-priority bullish/bearish PD array near price
    OTELow         float64 // 62% Fib level
    OTEHigh        float64 // 79% Fib level
    Equilibrium    float64 // 50% of range
}
```

---

## Temuan 4: ICT Multi-Timeframe (MTF) Bias overlay belum ada di /ict

**Status:** GAP MODERAT

### Apa yang sudah ada:
- `/ict EURUSD H4` → analisis single timeframe
- Data intraday sudah tersedia di `IntradayStore` (15m ke 12h)
- Data daily sudah tersedia di `DailyPriceStore`

### Apa yang HILANG:
**HTF Bias Context** — untuk `/ict EURUSD H4`, seharusnya:
1. Ambil data Daily → ICT analysis → ekstrak bias (BULLISH/BEARISH/NEUTRAL)
2. Tampilkan HTF bias sebagai filter: "Daily: BEARISH → only take H4 shorts in this environment"
3. Cross-TF alignment scoring

### Contoh output yang diinginkan:
```
📐 ICT/SMC Analysis — EURUSD H4
⏱️ Multi-TF Alignment:
  Daily : 🔴 BEARISH (CHoCH bearish, OB overhead)
  H4    : 🟡 NEUTRAL (no clear structure break)
  → Caution: HTF bearish — favor sell setups only
```

### Implementasi:
- Tambah `HTFBias string` dan `HTFSummary string` ke `ICTResult` 
- Di `cmdICT`, fetch daily bars dan run secondary Analyze()
- Hanya tampilkan HTF bias (bukan full detail), sebagai filter context

---

## Temuan 5: COT Seasonality belum ada

**Status:** RESEARCH ITEM (complex)

### Apa yang sudah ada:
- `service/price/seasonal.go` — price seasonality per month/quarter
- `handler_seasonal.go` — `/seasonal` command
- COT data tersedia di `service/cot/` dengan historical positions

### Apa yang HILANG:
**COT Positioning Seasonality** — historical patterns dalam net positioning COT per bulan.

**Concept:**
- Selama April, EUR commercial traders historically net long X% di atas rata-rata?
- Selama Q4, JPY non-commercial positioning biasanya reverting?

### Data flow yang diusulkan:
1. Gunakan COT historical data (sudah disimpan di BadgerDB)
2. Group by month untuk 5-10 tahun terakhir
3. Compute avg net position per month → "seasonal band"
4. Compare current vs seasonal baseline → z-score deviation
5. Alert jika positioning significantly out of seasonal range

### Sumber data:
- Data COT yang sudah ada di BadgerDB (gratis, sudah diambil setiap Jumat)
- Tidak perlu API baru

---

## Temuan 6: VIX Service tidak terintegrasi ke /sentiment display secara langsung

**Status:** GAP MINOR (UX/display)

### Situasi saat ini:
- `service/vix/` → full VIX term structure analysis ✅
- `service/ai/prompts.go` → CompositeIndicators pakai VIX term data ✅
- `/sentiment` formatter menggunakan `composites.VIXTermRegime` dan `composites.VIXTermRatio` ✅
- **TAPI**: SKEW tidak ada di SentimentData struct → tidak ada cross-signal VIX+SKEW

### Yang kurang:
- SentimentData belum punya SKEW field
- FormatSentiment tidak menampilkan SKEW alongside P/C ratios

---

## Summary: 5 Task Priorities untuk TASK-235 sampai TASK-239

| # | Task | Priority | Complexity |
|---|------|----------|-----------|
| TASK-235 | CBOE SKEW Index → VIX service + /sentiment | HIGH | S |
| TASK-236 | FRED IG Credit Spread (BAMLC0A0CM) → /macro | HIGH | S |
| TASK-237 | ICT PD Array + Premium/Discount Zone + OTE | MEDIUM | M |
| TASK-238 | ICT Multi-Timeframe (MTF) HTF Bias overlay | MEDIUM | M |
| TASK-239 | COT Seasonality engine + command integration | MEDIUM | L |

---

## Verified Free Data Sources

| Data | Source | URL/Method | Status |
|------|--------|-----------|--------|
| CBOE SKEW | CBOE CDN | `https://cdn.cboe.com/api/global/us_indices/daily_prices/SKEW_EOD.csv` | Pattern same as VIX |
| IG OAS | FRED API | `BAMLC0A0CM` series | ✅ Confirmed |
| ICT PD Array | Internal (price data) | Already in DailyPriceStore | ✅ No new API |
| ICT MTF | Internal (price data) | DailyPriceStore + IntradayStore | ✅ No new API |
| COT Seasonality | Internal (BadgerDB) | Historical COT already stored | ✅ No new API |

Semua 5 task menggunakan data GRATIS atau data yang sudah ada.
