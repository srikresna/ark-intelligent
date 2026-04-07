# TASK-236: FRED IG Credit Spread (BAMLC0A0CM) → /macro Credit Section

**Priority:** high
**Type:** data
**Estimated:** S
**Area:** internal/service/fred/, internal/adapter/telegram/formatter.go
**Created by:** Research Agent
**Created at:** 2026-04-02 21:00 WIB

## Deskripsi

Saat ini `service/fred/` hanya mengambil **HY OAS** (`BAMLH0A0HYM2`) sebagai credit stress proxy — disimpan di field `TedSpread` (nama field yang misleading). Field ini digunakan di `regime.go` untuk credit stress detection.

**Investment Grade OAS** (`BAMLC0A0CM`) belum diambil. Menambahkan IG spread memungkinkan:
1. **HY-IG Credit Ratio** = `HYSpread / IGSpread` → semakin tinggi = pasar mendiskon junk lebih besar = risk-off
2. IG spread adalah **early warning** — naik sebelum HY spread karena lebih liquid
3. Complete credit curve view: investment grade → high yield → credit quality premium

## Verified Data Source (GRATIS):
```
FRED series: BAMLC0A0CM
Nama: ICE BofA AAA-A US Corporate Index Option-Adjusted Spread (%)
curl "https://fred.stlouisfed.org/graph/fredgraph.csv?id=BAMLC0A0CM" → ✅ Working
Data tersedia dari 1996 hingga present, update harian (weekdays)
```

## File yang Harus Diubah

- `internal/service/fred/fetcher.go` — tambah field `IGSpread`, `IGSpreadTrend`, `HYIGRatio` ke `MacroData`; tambah ke `fetchAll()` dan `buildFromCache()`
- `internal/service/fred/regime.go` — gunakan IG spread sebagai additional credit stress indicator
- `internal/service/fred/persistence.go` — tambah `BAMLC0A0CM` ke persistence
- `internal/adapter/telegram/formatter.go` — tampilkan IG spread + HY/IG ratio di credit section `/macro`

## Implementasi

### MacroData fields baru (fred/fetcher.go)
```go
// Credit spreads — full curve
IGSpread      float64     // BAMLC0A0CM — ICE BofA IG OAS (%, ~0.5-3.0 normal range)
IGSpreadTrend SeriesTrend
HYIGRatio     float64     // TedSpread / IGSpread — credit quality ratio (>5x = stress)
```

### Fetch (fetcher.go buildFromCache)
```go
data.IGSpread, data.IGSpreadTrend = trend("BAMLC0A0CM", 0.05)
if data.IGSpread > 0 {
    data.HYIGRatio = data.TedSpread / data.IGSpread
}
```

### Series list di fetchAll()
Tambah ke batch setelah `BAMLH0A0HYM2`:
```go
{"BAMLC0A0CM", 5},
```

### Persistence (persistence.go)
```go
addObs("BAMLC0A0CM", data.IGSpread)
```

### HY-IG Ratio Classification
```go
func ClassifyHYIGRatio(ratio float64) (label, description string) {
    switch {
    case ratio >= 7.0:
        return "SEVERE STRESS", "HY premium 7× IG — extreme credit discrimination"
    case ratio >= 5.0:
        return "ELEVATED STRESS", "HY premium 5× IG — significant risk aversion"
    case ratio >= 4.0:
        return "CAUTIOUS", "HY premium 4× IG — normal-to-elevated credit stress"
    default:
        return "BENIGN", "HY premium below 4× IG — credit markets calm"
    }
}
```

### Formatter display (/macro credit section)
```
💳 Credit Markets
HY OAS  : 3.85%  ↑  (BAMLH0A0HYM2)
IG OAS  : 0.92%  →  (BAMLC0A0CM)
HY/IG   : 4.18×  — CAUTIOUS
Signal  : 🟡 IG stable but HY widening — watch for contagion
```

## Acceptance Criteria

- [ ] `data.IGSpread` terisi dari FRED `BAMLC0A0CM` di setiap fetch cycle
- [ ] `data.HYIGRatio` dihitung = `TedSpread / IGSpread` jika keduanya > 0
- [ ] `BAMLC0A0CM` tersimpan di BadgerDB persistence
- [ ] `/macro` menampilkan IG OAS + HY/IG ratio dengan classification
- [ ] `regime.go` menggunakan IG spread sebagai tambahan credit stress signal (ringan, non-breaking)
- [ ] Rename field `TedSpread` ke `HYSpread` di `MacroData` dengan update semua referensi
- [ ] Unit test: `TestHYIGRatioClassification` memverifikasi 4 level

## Referensi

- `.agents/research/2026-04-02-21-feature-gaps-skew-credit-ict-pdarray-cot-seasonal-putaran9.md` — Temuan 2
- `internal/service/fred/fetcher.go:MacroData.TedSpread` — field yang menyimpan BAMLH0A0HYM2 (rename ke HYSpread)
- `internal/service/fred/regime.go:203` — logika credit stress yang sudah ada
- `internal/service/fred/persistence.go:84` — pola addObs untuk series baru
