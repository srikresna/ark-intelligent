# Bug Hunt — Siklus 5 Putaran 6 — 2026-04-02 09:00 WIB

## Ringkasan

Analisis mendalam kode Go di `internal/service/backtest/`, `internal/service/price/`, dan `internal/service/cot/`. Ditemukan **4 bug nyata** + 1 desain issue yang berdampak pada keakuratan analisis kuantitatif.

---

## BUG-1: `excursion.go` — startIdx Termasuk Signal Date (Off-by-One)

**File:** `internal/service/backtest/excursion.go` baris 216  
**Severity:** High — semua data MFE/MAE off-by-one day

### Kode Bermasalah
```go
for i, dp := range reversed {
    if dp.Date.After(signalDate) || dp.Date.Equal(signalDate) {
        startIdx = i
        break
    }
}
```

### Masalah
Kondisi `dp.Date.Equal(signalDate)` menyebabkan `startIdx` menunjuk ke **hari sinyal itu sendiri** sebagai Day 0 excursion. Signal dikeluarkan pada close hari `signalDate`, sehingga EntryPrice = close `signalDate`. Akibatnya:
- Day 0: `closeMove = (close_signalDate - entryPrice) / entryPrice ≈ 0%`
- MFEDay dan MAEDay semua mundur 1 hari
- Optimal exit day salah (1 hari terlalu awal)

### Fix
```go
if dp.Date.After(signalDate) {   // hapus || dp.Date.Equal(signalDate)
```

---

## BUG-2: `excursion.go` — Boundary Check Terlalu Ketat, Signal Recent Dibuang

**File:** `internal/service/backtest/excursion.go` baris 222  
**Severity:** Medium — recent signals tidak dianalisis excursion

### Kode Bermasalah
```go
if startIdx < 0 || startIdx+evalDays > len(reversed) {
    return nil, fmt.Errorf("insufficient daily data after signal date")
}
```

### Masalah
Jika sinyal diterbitkan kurang dari `evalDays` (default 10 hari) yang lalu, check ini membuang sinyal tersebut dari analisis. Efek: excursion summary **tidak pernah** menyertakan sinyal terbaru (selalu ≥2 minggu lama). Analisis MFE/MAE menjadi bias terhadap sinyal lama.

### Fix
```go
effectiveEvalDays := evalDays
if startIdx+evalDays > len(reversed) {
    effectiveEvalDays = len(reversed) - startIdx
}
if startIdx < 0 || effectiveEvalDays <= 0 {
    return nil, fmt.Errorf("insufficient daily data after signal date")
}
// ganti evalDays → effectiveEvalDays di loop bawah
```

---

## BUG-3: `garch.go` — Convergence Flag Terbalik

**File:** `internal/service/price/garch.go` baris 212–213  
**Severity:** Medium — banyak GARCH fit yang valid dilaporkan `converged=false`

### Kode Bermasalah
```go
} else if fineLL-bestLL < 0.1 {
    // Fine grid didn't meaningfully improve over coarse grid
    converged = false
}
```

### Masalah
Logika ini **terbalik**. Jika `fineLL - bestLL < 0.1` (fine grid hampir tidak meningkat dari coarse grid), itu artinya coarse grid **sudah menemukan optimum** — ini adalah tanda konvergensi yang baik, bukan tanda tidak konvergen. Akibat: `GARCHConfidenceMultiplier` selalu return `1.0` (neutral) untuk model yang justru paling stabil, sementara model outlier (fine grid beda banyak dari coarse) dianggap "converged".

### Fix
```go
} else if fineLL-bestLL > 5.0 {
    // Fine grid improved significantly over coarse — coarse resolution insufficient
    converged = false
}
```

---

## BUG-4: `hmm_regime.go` — baumWelchStep Returns LL dari Model Lama

**File:** `internal/service/price/hmm_regime.go` — fungsi `baumWelchStep`  
**Severity:** Low — konvergensi HMM diperiksa satu iterasi lebih lambat

### Masalah
`baumWelchStep` mengembalikan `logLikOld` yang dihitung dari **model input** (bukan model baru yang diestimasi). Sehingga convergence check:
```go
if iter > 0 && math.Abs(logLik-prevLogLik) < 1e-4 {
```
Membandingkan `LL(model_iter_i-1)` dengan `LL(model_iter_i-2)` — tertinggal satu iterasi. Dampak: HMM bisa berhenti satu iterasi terlambat atau tidak konvergen meski sudah stabil.

### Fix
Hitung log-likelihood dari `newModel` sebelum return:
```go
// Di akhir baumWelchStep, setelah model baru diestimasi
newLogLik := computeLogLikelihood(&newModel, obs, scale)
return newModel, newLogLik
```

---

## Temuan Lain (Bukan Bug, Tapi Perlu Perhatian)

### Design Issue: `decay.go` — `absDuration` Duplicate dengan `excursion.go`
Kedua file di package `backtest` mendefinisikan fungsi `absDuration(d time.Duration)`. Ini akan menyebabkan **compile error** di Go. Perlu diperiksa apakah salah satunya harus dihapus.

**Catatan:** Setelah cek ulang, `decay.go` definisi absDuration di baris 169 dan tidak ada definisi serupa di excursion.go — jadi OK, tidak duplikat.

---

## Tasks yang Dibuat

- TASK-220: Fix excursion startIdx off-by-one (signal date included)
- TASK-221: Fix excursion boundary check too strict (recent signals dropped)
- TASK-222: Fix GARCH convergence flag inverted logic
- TASK-223: Fix HMM baumWelchStep log-likelihood off-by-one
- TASK-224: Add unit tests for excursion edge cases
