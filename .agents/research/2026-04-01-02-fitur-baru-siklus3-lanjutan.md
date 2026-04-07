# Research Report: Fitur Baru Siklus 3 — Lanjutan (COT Disaggregated, VIX Term Structure, Intermarket)

**Date:** 2026-04-01 02:40 WIB
**Fokus:** Siklus 3 — Fitur Baru (putaran kedua)
**Agent:** Research

---

## Ringkasan Eksekutif

Siklus 3 putaran pertama (2026-04-01 15:45) sudah mencakup ICT/SMC (TASK-010, 035-037), Wyckoff (TASK-011, 039), GEX/Deribit (TASK-012), Elliott Wave (TASK-013), Estimated Delta (TASK-014), dan VWAP (TASK-038). Riset ini menggali fitur-fitur yang **belum tercakup** di task manapun: COT disaggregated category analysis, VIX term structure, COT seasonality, cross-currency COT divergence, dan intermarket correlation signals.

---

## Analisis Codebase

### Data COT Yang Ada Tapi Belum Dieksploitasi
File `internal/domain/cot.go` dan `internal/service/cot/analyzer.go` memiliki data lengkap:

**TFF (Financials/FX/Bonds) — 4 kategori:**
- `DealerLong/Short` — Dealer/Intermediary (bank besar, liquidity providers)
- `AssetMgrLong/Short` — Asset Manager (mutual funds, pension, long-only)
- `LevFundLong/Short` — Leveraged Funds (hedge funds, CTA, momentum)
- `OtherReportable` — Other Reportables

**DISAGGREGATED (Metals/Energy) — 4 kategori:**
- `ProdMercLong/Short` — Producer/Merchant/Processor/User
- `SwapDealerLong/Short` — Swap Dealers (financial intermediaries)
- `ManagedMoneyLong/Short` — Managed Money (hedge funds/CTA)
- `OtherReportable`

**Yang sudah dihitung:**
- `LevFundNet` — net position Lev Funds
- `ManagedMoneyNet` — net position Managed Money
- `AssetMgrZScore` — ZScore untuk asset managers (WoW change)
- `CommLSRatio` — combined commercial long/short ratio

**Yang BELUM ada:**
- ZScore per-kategori untuk Dealer, LevFund, ManagedMoney, SwapDealer
- Divergence signal: LevFund vs AssetMgr (momentum vs institutional)
- Dealer positioning (contrarian signal — dealers often fade retail)
- Cross-currency aggregation / USD aggregate signal
- COT week-of-year seasonality (multi-year historical patterns)

### Sentiment: VIX Yang Ada Tapi Belum Optimal
File `internal/service/sentiment/sentiment.go` sudah fetch CNN Fear & Greed (includes VIX data), tapi:
- Tidak ada VIX futures term structure
- Tidak ada spot VIX raw value (hanya embedded dalam CNN F&G)
- CBOE menyediakan VIX futures settlement data via free CSV harian
- VIX9D (9-day) dan VIX3M (3-month) tersedia dari CBOE (gratis)

### Price: Correlation Matrix Ada Tapi Terbatas
`internal/service/price/correlation.go` ada correlation matrix, tapi:
- Hanya price-to-price correlation, bukan intermarket signal
- Tidak ada structured intermarket relationship rules
- Tidak ada cross-asset regime signal (risk-on/risk-off detector via corr)

---

## Temuan Riset

### 1. COT Category Deep Analysis — Gap Terbesar

**Konsep:** Trader institusional memandang COT bukan hanya "net position" tapi breakdown per kategori:
- **Dealer/Intermediary (TFF)** = smart money, sering contrarian → fade mereka
- **Asset Manager (TFF)** = institutional trend follower, slow-moving → confirm dengan mereka
- **Leveraged Fund (TFF)** = CTA/hedge fund, momentum → crowding risk signal
- **Swap Dealer (DISAGG)** = commercial intermediary → commercial hedge signal
- **Managed Money (DISAGG)** = speculative, trend-follower untuk metals/energy

**Signal yang bisa dihasilkan:**
1. LevFund vs AssetMgr divergence: LevFund extreme long + AssetMgr neutral → overextended, reversal risk
2. Dealer vs LevFund spread: Dealers adding shorts + LevFunds adding longs = squeeze setup
3. AssetMgr momentum: WoW change ZScore >2 = institutional accumulation signal
4. "Follow the commercials" for DISAGG: ProdMerc + SwapDealer net direction = smart money directional bias

**Implementation:**
- Fungsi `computeCategoryZScores()` baru di `internal/service/cot/analyzer.go`
- Fields baru di `domain.COTAnalysis`: `DealerZScore`, `LevFundZScore`, `ManagedMoneyZScore`, `SwapDealerZScore`
- Signal: `CategoryDivergence` bool + `CategoryDivergenceType` string
- Update formatter untuk tampilkan breakdown per kategori di /cot

### 2. VIX Term Structure — Gratis via CBOE

**Konsep:** VIX futures term structure menunjukkan apakah pasar dalam contango (normal/risk-on) atau backwardation (fear/risk-off):
- Contango: VX1 < VX2 < VX3 → market complacent, bullish jangka pendek
- Backwardation: VX1 > VX2 → elevated fear, risk-off positioning
- Roll yield: contango steep → VIX ETPs lose value (bullish for equities)

**Data Source GRATIS:**
```
# CBOE VIX Futures Daily Settlement (CSV, no API key)
https://cdn.cboe.com/api/global/us_indices/daily_prices/VX_EOD.csv
# Format: Trade Date, Futures, Open, High, Low, Close, Settle, Change, %Change, Volume, EFP, Open Interest
# Contains all active VIX future contracts

# VIX Index raw data
https://cdn.cboe.com/api/global/us_indices/daily_prices/VIX_EOD.csv

# VVIX (VIX of VIX) — volatility of volatility
https://cdn.cboe.com/api/global/us_indices/daily_prices/VVIX_EOD.csv
```

**Implementation:**
- Service baru: `internal/service/vix/` — fetch + parse CBOE CSV
- Types: `VIXTermStructure{Spot, M1, M2, M3, VVIX, Contango bool, RollYield float64}`
- Integrate ke `SentimentData` atau standalone
- /sentiment atau /vix command: tampilkan term structure

### 3. COT Seasonality — Multi-Year Historical

**Konsep:** COT positioning memiliki pola musiman yang bisa diprediksi:
- EUR net positioning historically peaks bullish di Q4 (year-end hedging)
- JPY net short biasanya dalam di Q1 (carry trade season)
- Gold Managed Money historically bullish di Sep-Nov (physical demand)
- Seasonal divergence: current vs historical average = timing signal

**Implementation:**
- Extend `internal/service/price/seasonal.go` pattern ke COT
- Buat `internal/service/cot/seasonal.go`
- Gunakan data history yang sudah disimpan di BadgerDB (52 weeks minimal)
- Bisa extend ke multi-year dengan fetch CFTC historical CSV (gratis):
  ```
  https://www.cftc.gov/files/dea/history/fut_disagg_xls_2020_2024.zip
  https://www.cftc.gov/dea/newcot/c_disagg.txt (current year, all contracts)
  ```
- Hitung: per-week-of-year average net positions over 5Y history
- Signal: "Current EUR positioning is 1.8σ above seasonal average → hedge against seasonal mean reversion"

### 4. Multi-Currency COT USD Aggregate Signal

**Konsep:** USD direction tidak hanya dari DX futures, tapi dari AGGREGATE position across all major pairs:
- Sum up: (-1 × EUR_net) + (-1 × GBP_net) + (-1 × AUD_net) + JPY_net + (-1 × CHF_net)
  (all relative to USD)
- Hasilkan "USD Aggregate COT Signal" = synthetic dollar positioning index
- Compare to DX futures COT → convergence/divergence signal

**Implementation:**
- Fungsi baru di `internal/service/cot/`: `ComputeUSDAggregate(analyses []domain.COTAnalysis) USDAggregate`
- Tampilkan di /bias command sebagai "USD Aggregate Position"
- Alert jika USD aggregate diverges from DX direct positioning

### 5. Intermarket Correlation Signal Engine

**Konsep:** Forex intermarket memiliki hubungan yang well-established:
- AUD/USD → positif dengan Gold, positif dengan S&P500 (risk-on currency)
- CAD/USD → positif dengan Oil WTI
- JPY/USD → negatif dengan S&P500, negatif dengan yields (safe haven)
- CHF/USD → negatif dengan risk, positif dengan Gold
- USD Index → negatif dengan Gold, mixed dengan equities

**Implementation:**
- Gunakan data yang sudah ada: price data (TwelveData/Polygon) + FRED (yields) + EIA (oil)
- Buat `internal/service/intermarket/` — rules-based intermarket signal engine
- RuleSet: define expected correlation direction untuk tiap pair
- Signal: jika actual rolling 20-day correlation DIVERGES dari expected → signal
  - e.g. AUDUSD rising while Gold falling = divergence → fade AUD rally
- /intermarket command: tampilkan 8-10 key intermarket relationships + current status

---

## Prioritas Implementasi

| Task | Judul | Priority | Kompleksitas |
|---|---|---|---|
| TASK-060 | COT Category ZScore + Divergence Signal | HIGH | MEDIUM |
| TASK-061 | VIX Term Structure Engine (CBOE CSV) | HIGH | LOW-MEDIUM |
| TASK-062 | COT Seasonality Analysis | MEDIUM | MEDIUM |
| TASK-063 | USD Aggregate COT Signal | MEDIUM | LOW |
| TASK-064 | Intermarket Correlation Signal Engine | MEDIUM | MEDIUM-HIGH |

---

## File Structure

```
internal/service/cot/
├── category_zscore.go   # NEW: per-category ZScore + divergence (TASK-060)
└── seasonal.go          # NEW: COT seasonality (TASK-062)

internal/service/vix/
├── types.go             # NEW: VIXTermStructure struct
├── engine.go            # NEW: CBOE CSV fetch + parse + term structure calc
└── cache.go             # NEW: 1-hour cache (CBOE updates end-of-day)

internal/service/intermarket/
├── types.go             # NEW: IntermarketSignal, CorrelationRule
└── engine.go            # NEW: rules-based correlation signal engine
```

---

## Kesimpulan

5 task dibuat untuk melanjutkan Siklus 3 (Fitur Baru) dengan fokus pada:
1. Eksploitasi data COT yang sudah ada tapi belum dianalisis per-kategori
2. VIX term structure gratis dari CBOE
3. COT seasonality multi-year
4. USD aggregate dari cross-pair COT
5. Intermarket correlation signals berbasis rules

Semua menggunakan data sumber GRATIS dan tidak memerlukan dependency eksternal baru (hanya HTTP fetch CSV/JSON standar).
