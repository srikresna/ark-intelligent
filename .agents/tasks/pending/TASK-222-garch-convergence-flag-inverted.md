# TASK-222: Fix GARCH Convergence Flag — Logic Terbalik

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/service/price/
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

`garch.go` baris 212–213 menandai model sebagai `converged=false` ketika fine grid tidak banyak berbeda dari coarse grid. Ini logika yang terbalik: perbedaan kecil antara fine dan coarse grid justru menandakan coarse grid sudah menemukan optimum yang stabil (konvergen), bukan sebaliknya.

## Bug Detail

```go
// garch.go:207-214 — BUG: logika terbalik
converged := true
if math.IsInf(fineLL, -1) || math.IsNaN(fineLL) {
    converged = false
} else if fineLL-bestLL < 0.1 {
    // Fine grid didn't meaningfully improve over coarse grid
    converged = false  // BUG: ini seharusnya = true
}
```

Dampak:
- `GARCHConfidenceMultiplier` selalu return 1.0 (neutral) untuk model paling stabil
- `/quant` menampilkan "GARCH not converged" untuk estimasi yang justru reliable
- Confidence multiplier tidak berpengaruh pada banyak sinyal yang seharusnya terpengaruh

## Fix

```go
converged := true
if math.IsInf(fineLL, -1) || math.IsNaN(fineLL) {
    converged = false
} else if fineLL-bestLL > 5.0 {
    // Fine grid improved significantly → coarse grid resolution insufficient
    converged = false
}
if alpha+beta > 0.999 {
    converged = false
}
```

Threshold 5.0 LL units = perubahan substantif dalam log-likelihood yang mengindikasikan grid search belum menemukan optimum stabil.

## File yang Kemungkinan Diubah

- `internal/service/price/garch.go` — ubah kondisi convergence check baris ~210-215

## Acceptance Criteria

- [ ] Model dengan `fineLL ≈ bestLL` (perbedaan < 5.0) ditandai `converged=true`
- [ ] Model dengan `fineLL >> bestLL` (perbedaan > 5.0) ditandai `converged=false`
- [ ] Model dengan `alpha+beta > 0.999` tetap `converged=false`
- [ ] Unit test: verifikasi model dengan parameter stabil (low alpha, high beta typical) menghasilkan `Converged=true`

## Referensi

- `.agents/research/2026-04-02-09-bug-hunt-excursion-garch-hmm.md`
