# TASK-239: COT Seasonality Engine — Pola Posisi Historis per Bulan

**Priority:** medium
**Type:** feature
**Estimated:** L
**Area:** internal/service/cot/ (new: seasonality.go), internal/adapter/telegram/handler_seasonal.go
**Created by:** Research Agent
**Created at:** 2026-04-02 21:00 WIB

## Deskripsi

`service/price/seasonal.go` sudah menganalisis seasonality **harga** per bulan. Yang belum ada adalah seasonality **posisi COT** — apakah commercial dan non-commercial traders biasanya long/short pada bulan tertentu?

**Use case:**
- "Ini April — historically, EUR commercials biasanya membangun long di April?"
- "JPY non-commercial shorts di Q4 biasanya peak sebelum reversal?"
- "Current COT positioning: z-score berapa deviasi dari seasonal norm?"

### Manfaat:
1. Konteks seasonal membantu membedakan positioning yang "normal" vs "outlier"
2. Kombinasi dengan COT signal → higher conviction setups
3. Tidak butuh API baru — gunakan data COT yang sudah tersimpan di BadgerDB

## Data yang sudah tersedia:
- COT historical positions tersimpan di BadgerDB per week (sudah ada sejak service start)
- `service/cot/` punya struct `COTData` dengan `NetLong`, `NonCommNetLong`, `CommercialNetLong`
- Data CFTC biasanya tersedia 3+ tahun ke belakang via API (CFTC menyediakan historical backfill)

## File yang Harus Dibuat/Diubah

- `internal/service/cot/seasonality.go` — NEW: engine seasonal analysis COT
- `internal/service/cot/types.go` — tambah `COTSeasonalResult` struct
- `internal/adapter/telegram/handler_seasonal.go` — tambah opsi `/seasonal EURUSD cot` atau command baru `/cotseasonal`
- `internal/adapter/telegram/formatter.go` atau formatter baru — format COT seasonal output

## Implementasi

### COTSeasonalResult struct (types.go)
```go
// COTSeasonalResult holds seasonal analysis of COT positioning.
type COTSeasonalResult struct {
    Symbol       string
    ContractCode string
    Month        int       // 1-12 (current month analyzed)
    Year         int       // current year

    // Non-commercial (speculators) seasonal baseline
    AvgNonCommNet    float64   // average net position this month (historical)
    StdNonCommNet    float64   // standard deviation
    CurrentNonCommNet float64  // current week's non-comm net
    NonCommZScore    float64   // (current - avg) / std
    NonCommSignal    string    // "ABOVE_NORMAL" | "BELOW_NORMAL" | "NORMAL"

    // Commercial (hedgers) seasonal baseline
    AvgCommNet    float64
    StdCommNet    float64
    CurrentCommNet float64
    CommZScore    float64
    CommSignal    string

    // Historical context
    SampleYears  int       // how many years of data used
    DataAvailable bool
    Message      string    // if data insufficient
}
```

### seasonality.go — ComputeCOTSeasonality()
```go
// ComputeCOTSeasonality calculates seasonal norms for COT positions.
// Reads stored historical COT data from BadgerDB and computes monthly stats.
func ComputeCOTSeasonality(
    ctx context.Context,
    store COTHistoricalStore,
    contractCode string,
    targetMonth int,
) *COTSeasonalResult
```

**Algoritma:**
1. Load semua COT records untuk `contractCode` dari BadgerDB
2. Filter records di mana `week.Month() == targetMonth`
3. Compute mean dan std dev dari `NonCommNetLong` dan `CommercialNetLong`
4. Get current week's positioning
5. Compute z-score: `(current - mean) / std`
6. Classify: |z| < 1 = NORMAL, z > 1 = ABOVE_NORMAL, z < -1 = BELOW_NORMAL

### Signal Classification Z-Score
```go
func classifyCOTZScore(z float64) string {
    switch {
    case z >= 2.0:
        return "EXTREME LONG" // rare, potential exhaustion
    case z >= 1.0:
        return "ABOVE NORMAL"
    case z <= -2.0:
        return "EXTREME SHORT" // rare, potential reversal
    case z <= -1.0:
        return "BELOW NORMAL"
    default:
        return "SEASONAL NORMAL"
    }
}
```

### Command: `/seasonal EURUSD cot` atau `/cotseasonal EURUSD`
Output:
```
📅 COT SEASONALITY — EURUSD (April)
Based on 4 years of April data

Non-Commercial (Speculators):
  Seasonal avg : +85,432 net long
  Current      : +102,847 (+20%)
  Z-Score      : +1.4 → 🟡 ABOVE NORMAL
  
Commercial (Hedgers):
  Seasonal avg : -78,210 net short
  Current      : -95,112 (-21%)
  Z-Score      : -1.6 → 🟡 BELOW NORMAL (more hedged than usual)

Signal: Speculators positioned 1.4σ more long than typical April →
        Watch for potential mean reversion if price stalls.
```

## Acceptance Criteria

- [ ] `ComputeCOTSeasonality()` menghitung avg + std dari BadgerDB historical COT per month
- [ ] Minimum 2 tahun data untuk hasil yang bermakna; jika <2 tahun → `DataAvailable = false`
- [ ] Z-score dihitung untuk NonComm dan Commercial positions
- [ ] Signal classification benar untuk 5 level z-score
- [ ] Command `/cotseasonal EURUSD` atau `/seasonal EURUSD cot` berfungsi
- [ ] Output menampilkan sample years dan data date range
- [ ] Jika BadgerDB tidak punya data → tampilkan graceful "insufficient historical data"
- [ ] Unit test: `TestCOTSeasonalZScore` dengan mock data 3 tahun

## Referensi

- `.agents/research/2026-04-02-21-feature-gaps-skew-credit-ict-pdarray-cot-seasonal-putaran9.md` — Temuan 5
- `internal/service/price/seasonal.go` — pola seasonal engine yang bisa dijadikan referensi
- `internal/service/cot/` — COT data structures dan BadgerDB storage
- `internal/adapter/telegram/handler_seasonal.go` — handler seasonal yang bisa diextend
