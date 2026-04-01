# TASK-031: BIS Real Effective Exchange Rates (REER/NEER) Integration

**Priority:** high
**Type:** data
**Estimated:** M
**Area:** internal/service
**Created by:** Research Agent
**Created at:** 2026-04-01 15:xx WIB
**Siklus:** Data (Siklus 2 Putaran 2)

## Deskripsi
Integrasikan BIS (Bank for International Settlements) Real Effective Exchange Rate (REER)
dan Nominal Effective Exchange Rate (NEER) data untuk semua major currency pairs.
Data ini gratis, tanpa API key, dari stats.bis.org.

## Konteks
Saat ini bot punya price data (TwelveData, Yahoo), COT positioning, dan FRED macro.
Tapi tidak ada data REER/NEER — yang menunjukkan apakah suatu currency fundamentally
overvalued atau undervalued terhadap seluruh trading partners.

**Kenapa REER penting untuk forex:**
- REER > jangka panjang mean → currency OVERVALUED → tekanan bearish jangka menengah
- REER < jangka panjang mean → currency UNDERVALUED → tailwind bullish jangka menengah
- Bank sentral memantau REER untuk kebijakan FX intervention
- IMF dan BIS sendiri menggunakan REER sebagai benchmark valuation

**Contoh use case:**
```
USD REER: 118.3 (long-term avg 100) → USD historically overvalued
EUR REER: 92.4 → EUR undervalued
→ Structural case untuk EUR/USD reversal ke atas
```

BIS API: public REST API, SDMX-JSON format, no auth required.

## BIS API Details
```
Base URL: https://stats.bis.org/api/v2/data/BIS,WS_EER,1.0/M.B.{COUNTRY}.A.?startPeriod=2020-01&endPeriod=2026-01
```
- `M` = Monthly frequency
- `B` = Broad basket (preferred — 60+ partners)
- `{COUNTRY}` = US, XM (Eurozone), GB, JP, CH, AU, CA, NZ
- `A` = CPI deflated (REER); `N` = Nominal (NEER)

Response: SDMX-JSON format — parse `data.dataSets[0].series` untuk nilai series.

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Buat package `internal/service/bis/` dengan file `reer.go`
- [ ] Struct `REERData`:
  ```go
  type REERData struct {
      Currency    string  // "USD", "EUR", etc.
      REER        float64 // Latest REER value (index, base=2020)
      NEER        float64 // Latest NEER value
      LTAvg       float64 // Long-term average REER (computed from 5yr window)
      Deviation   float64 // REER vs LTAvg (%)  positive = overvalued
      Signal      string  // "OVERVALUED", "FAIR", "UNDERVALUED" (±5% threshold)
      AsOf        string  // "2026-01" (month string)
      Available   bool
  }
  ```
- [ ] Struct `BISData`:
  ```go
  type BISData struct {
      Currencies []REERData
      FetchedAt  time.Time
  }
  ```
- [ ] Fungsi `FetchBISData(ctx context.Context) (*BISData, error)` fetch concurrent untuk 8 currencies
- [ ] In-memory cache dengan TTL 24 jam (BIS data monthly, cukup re-fetch daily)
- [ ] `UnifiedOutlookData` ditambah field `BISData *bis.BISData`
- [ ] `BuildUnifiedOutlookPrompt()` menambahkan section baru "CURRENCY VALUATION (BIS REER)":
  ```
  BIS REER Valuation:
  USD: 118.3 (overvalued +18.3% vs LT avg) | EUR: 92.4 (undervalued -7.6%)
  GBP: 97.1 (fair) | JPY: 68.2 (deeply undervalued -31.8%)
  ```
- [ ] Graceful degradation: jika BIS API gagal, log warn + skip section

## File yang Kemungkinan Diubah
- `internal/service/bis/reer.go` (baru)
- `internal/service/ai/unified_outlook.go` (UnifiedOutlookData + section baru)
- `internal/adapter/telegram/handler.go` (inject BISData)
- Opsional: `/bis` Telegram command untuk menampilkan REER summary langsung

## Referensi
- `.agents/research/2026-04-01-15-data-integrasi-siklus2-putaran2.md` (GAP 2)
- BIS EER API docs: https://stats.bis.org/api/v2/data/BIS,WS_EER/
- `internal/service/worldbank/` (jika sudah ada dari TASK-009 — ikuti pola)
- `internal/service/fred/fetcher.go` (pola cache + concurrent fetch)
