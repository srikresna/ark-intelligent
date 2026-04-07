# TASK-298: Fix GARCH `Converged` Flag — Inverted Logic untuk Fine Grid Improvement

**Priority:** low
**Type:** bug-fix
**Estimated:** XS
**Area:** internal/service/price/garch.go
**Created by:** Research Agent
**Created at:** 2026-04-02 10:00 WIB

## Deskripsi

Di `estimateGARCHFromReturns()`, terdapat logika konvergensi yang **terbalik**:

```go
// internal/service/price/garch.go:156-165
converged := true
if math.IsInf(fineLL, -1) || math.IsNaN(fineLL) {
    converged = false
} else if fineLL-bestLL < 0.1 {
    // Fine grid didn't meaningfully improve over coarse grid
    converged = false  // ← LOGIKA TERBALIK!
}
if alpha+beta > 0.999 {
    converged = false
}
```

**Masalah:** `fineLL - bestLL < 0.1` berarti fine grid tidak terlalu berbeda dari coarse grid → ini justru tanda **estimasi sudah stabil dan converge**. Tapi kode menandainya sebagai `converged = false`.

Sebaliknya, jika `fineLL - bestLL` besar, artinya fine grid menemukan solusi yang sangat berbeda dari coarse grid → ada instabilitas, mungkin non-converged.

**Dampak nyata:**
`GARCHConfidenceMultiplier()` return `1.0` (no adjustment) saat `g.Converged == false`:
```go
func GARCHConfidenceMultiplier(g *GARCHResult) float64 {
    if g == nil || !g.Converged {
        return 1.0  // ← No GARCH adjustment
    }
    // ...
}
```

Artinya GARCH yang sebenarnya sudah konverge dengan baik (fine == coarse) tidak memberikan volatility-adjusted confidence multiplier. **Signals kehilangan GARCH adjustment untuk kasus yang sebenarnya paling reliable.**

## Perubahan yang Diperlukan

### Fix: Hapus atau Invert Kondisi Fine Grid

**Opsi 1 (Recommended): Hapus kondisi yang misleading**
```go
converged := true
if math.IsInf(fineLL, -1) || math.IsNaN(fineLL) {
    converged = false
}
// HAPUS kondisi fineLL-bestLL < 0.1 — tidak relevan untuk convergence
if alpha+beta > 0.999 {
    converged = false
}
```

**Opsi 2: Invert logic menjadi benar**
```go
} else if fineLL-bestLL > 5.0 {
    // Fine grid improved DRAMATICALLY over coarse → optimization landscape unstable
    converged = false
}
```

Opsi 1 lebih aman karena menghindari threshold yang arbitrary.

## File yang Harus Diubah

1. `internal/service/price/garch.go` — baris sekitar 156-165, fungsi `estimateGARCHFromReturns()`

## Verifikasi

```bash
go test ./internal/service/price/... -run "TestGARCH\|GARCH"
go build ./...
```

Cek secara manual: `GARCHResult.Converged` bernilai `true` untuk dataset dengan volatility stabil.

## Acceptance Criteria

- [ ] Kondisi `fineLL-bestLL < 0.1` dihapus atau diinvert
- [ ] `Converged = true` untuk estimasi yang stabil (fine grid ≈ coarse grid)
- [ ] `Converged = false` masih tepat untuk: LL = Inf/NaN, atau alpha+beta ≥ 1 (unit root)
- [ ] `go build ./...` clean
- [ ] `GARCHConfidenceMultiplier()` memberikan adjustment yang benar untuk data stabil

## Referensi

- `.agents/research/2026-04-02-10-codebase-bug-analysis-putaran21.md` — BUG-4
- `internal/service/price/garch.go:156-165` — convergence check
- `internal/service/price/garch.go:197-214` — GARCHConfidenceMultiplier()
