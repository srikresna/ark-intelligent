# TASK-032: Fed Dot Plot via FRED (FEDTARMD / FEDTARH / FEDTARL series)

**Priority:** high
**Type:** data
**Estimated:** S
**Area:** internal/service/fred
**Created by:** Research Agent
**Created at:** 2026-04-01 15:xx WIB
**Siklus:** Data (Siklus 2 Putaran 2)

## Deskripsi
Tambahkan 3 FRED series ke MacroData: Fed Dot Plot median, high, dan low target rate
(`FEDTARMD`, `FEDTARH`, `FEDTARL`). Ini menunjukkan apa yang Fed officials sendiri
ekspektasikan untuk policy rate — berbeda dari actual rate dan market expectations.

## Konteks
Bot sudah punya `FedFundsRate` dan `SOFR` (actual/overnight rates). Tapi tidak ada:
- **Fed Dot Plot**: di mana Fed officials sendiri expect rates di masa depan
- **Market vs Dots divergence**: ini driver utama currency volatility

Divergence dots vs market pricing = source utama forex volatility:
- Market pricing 3 cuts, dots 1 cut → divergence → saat data kuat, USD bisa spike tajam
- Market pricing 0 cuts, dots 2 cuts → divergence lain → USD vulnerability jika dovish surprise

FRED series `FEDTARMD` adalah quarterly update (dari Summary of Economic Projections / dot plot).
Effort sangat kecil karena tinggal menambah 3 series ke `fetchSeriesParallel()` yang sudah ada.

## Acceptance Criteria
- [ ] `go build ./...` sukses
- [ ] `go vet ./...` sukses
- [ ] Tambah field ke `MacroData` struct di `fetcher.go`:
  ```go
  FedDotMedian float64 // FEDTARMD — Fed's median projected policy rate (%)
  FedDotHigh   float64 // FEDTARH  — Fed's high projection
  FedDotLow    float64 // FEDTARL  — Fed's low projection
  ```
- [ ] Tambah 3 series ke `fetchSeriesParallel()`: `FEDTARMD`, `FEDTARH`, `FEDTARL`
  dengan `lookback=6` (series quarterly, ambil 6 obs = 1.5 tahun)
- [ ] Populate data di `populateMacroData()`:
  ```go
  data.FedDotMedian = single("FEDTARMD")
  data.FedDotHigh   = single("FEDTARH")
  data.FedDotLow    = single("FEDTARL")
  ```
- [ ] Sanitize di `sanitizeMacroData()`:
  ```go
  sanitizeFloat(&data.FedDotMedian)
  sanitizeFloat(&data.FedDotHigh)
  sanitizeFloat(&data.FedDotLow)
  ```
- [ ] Di `BuildUnifiedOutlookPrompt()`, tambahkan ke section MACRO ENVIRONMENT:
  ```
  Fed Dot Plot: Median=X.XX% | High=X.XX% | Low=X.XX%
  Dots vs SOFR Gap: +/-X.XXbps (market {dovish|hawkish} vs Fed)
  ```
- [ ] Di `persistence.go`, persist 3 series baru ke BadgerDB (ikuti pola yang ada)
- [ ] Graceful: jika series 0 (quarterly, bisa antara updates) → skip display

## File yang Kemungkinan Diubah
- `internal/service/fred/fetcher.go` (MacroData struct + fetchSeriesParallel + populate + sanitize)
- `internal/service/fred/persistence.go` (tambah 3 obs baru ke PersistSnapshot)
- `internal/service/ai/unified_outlook.go` (tampilkan dots di section MACRO)

## Referensi
- `.agents/research/2026-04-01-15-data-integrasi-siklus2-putaran2.md` (GAP 5)
- FRED FEDTARMD: https://fred.stlouisfed.org/series/FEDTARMD
- FRED FEDTARH: https://fred.stlouisfed.org/series/FEDTARH
- FRED FEDTARL: https://fred.stlouisfed.org/series/FEDTARL
- `internal/service/fred/fetcher.go` (pola fetchSeriesParallel + populateMacroData)
- `internal/service/fred/persistence.go` (pola addObs)
