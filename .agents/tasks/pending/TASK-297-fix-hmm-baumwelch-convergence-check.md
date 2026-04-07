# TASK-297: Fix HMM Baum-Welch Convergence Check (Off-by-One LL Comparison)

**Priority:** medium
**Type:** bug-fix
**Estimated:** S
**Area:** internal/service/price/hmm_regime.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

Implementasi Baum-Welch (EM) di `EstimateHMMRegime()` memiliki masalah pada convergence check:

```go
// internal/service/price/hmm_regime.go:74-90
prevLogLik := math.Inf(-1)
for iter = 0; iter < maxIter; iter++ {
    newModel, logLik := baumWelchStep(&model, obs)
    model = newModel
    if iter > 0 && math.Abs(logLik-prevLogLik) < 1e-4 {
        converged = true
        break
    }
    prevLogLik = logLik
}
```

**Masalah utama:**
`baumWelchStep()` mengembalikan **log-likelihood dari model LAMA** (sebelum parameter update). Karena LL dihitung menggunakan forward algorithm dengan parameter lama, kemudian parameter di-update, iterasi N+1 membandingkan LL dari parameter lama iterasi N dengan LL dari parameter lama iterasi N-1.

Ini **tidak konsisten** — kita seharusnya membandingkan kualitas parameter sebelum dan sesudah update, bukan membandingkan backward-looking LL.

**Masalah kedua:** guard `iter > 0` tidak mencegah konvergensi prematur di iterasi 1 jika LL kebetulan sangat kecil (praktis tidak terjadi tapi logic-nya tidak elegant).

**Dampak praktis:**
- HMM bisa melakukan lebih banyak atau lebih sedikit iterasi dari yang optimal
- `Converged = false` dilaporkan untuk model yang sebenarnya sudah converge
- `Converged = true` bisa dilaporkan prematur jika data sangat stabil

## Perubahan yang Diperlukan

### Fix: Konsistenkan LL Tracking

```go
// SOLUSI: Track LL dari iterasi sebelumnya secara konsisten
// Hitung LL awal sebelum loop (untuk iterasi 0 vs 1 comparison valid)

// Hitung LL dari model awal sebelum update pertama
initialLL := computeLogLikelihood(&model, obs) // helper baru
prevLogLik := initialLL

converged := false
var iter int
for iter = 0; iter < maxIter; iter++ {
    newModel, logLik := baumWelchStep(&model, obs)
    model = newModel
    if math.Abs(logLik-prevLogLik) < 1e-4 {
        converged = true
        break
    }
    prevLogLik = logLik
}
```

Alternatif sederhana: pertahankan kode existing tapi **hapus guard `iter > 0`** — karena `prevLogLik` diinisiasi `math.Inf(-1)`, iterasi pertama tidak akan pernah converge (Inf - finite = Inf > 1e-4). Guard tidak diperlukan.

```go
// MINIMAL FIX: hapus guard iter > 0 (behavior tidak berubah, logic lebih bersih)
if math.Abs(logLik-prevLogLik) < 1e-4 {  // tanpa && iter > 0
    converged = true
    break
}
```

**Catatan:** Karena `prevLogLik` dimulai dari `math.Inf(-1)`, `math.Abs(logLik - math.Inf(-1)) = math.Inf(1)` yang selalu > 1e-4. Minimal fix tidak mengubah behavior tapi lebih correct secara semantic.

### Optional: Tambah Helper `computeLogLikelihood`

```go
// Hitung log-likelihood dari model dan sequence observasi
func computeLogLikelihood(m *HMMModel, obs []int) float64 {
    T := len(obs)
    N := hmmNumStates
    
    scale := 0.0
    alpha := make([]float64, N)
    for i := 0; i < N; i++ {
        alpha[i] = m.Pi[i] * m.B[i][obs[0]]
        scale += alpha[i]
    }
    
    ll := 0.0
    if scale > 0 {
        ll += math.Log(scale)
    }
    // ... (forward algorithm loop)
    return ll
}
```

## File yang Harus Diubah

1. `internal/service/price/hmm_regime.go` — perbaiki loop convergence check di `EstimateHMMRegime()`

## Verifikasi

```bash
go test ./internal/service/price/... -run "TestHMM\|hmm"
go build ./...
```

Verifikasi manual: output `HMMResult.Converged` dan `HMMResult.Iterations` masuk akal untuk data harga yang cukup.

## Acceptance Criteria

- [ ] Guard `iter > 0` dihapus atau convergence logic diperbaiki secara semantik
- [ ] `Converged` field di `HMMResult` mencerminkan konvergensi yang benar
- [ ] `go build ./...` clean
- [ ] Tidak ada perubahan di return type atau field lain di `HMMResult`

## Referensi

- `.agents/research/2026-04-02-10-codebase-bug-analysis-putaran21.md` — BUG-3
- `internal/service/price/hmm_regime.go:74-90` — Baum-Welch loop
- `internal/service/price/hmm_regime.go:193-250` — baumWelchStep() implementation
