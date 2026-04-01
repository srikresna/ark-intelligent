# TASK-223: Fix HMM baumWelchStep — Log-Likelihood Off-by-One Iteration

**Priority:** low
**Type:** fix
**Estimated:** S
**Area:** internal/service/price/
**Created by:** Research Agent
**Created at:** 2026-04-02 09:00 WIB

## Deskripsi

`hmm_regime.go` fungsi `baumWelchStep` mengembalikan `logLikOld` — yang dihitung dari **model input** sebelum re-estimation, bukan dari model baru yang sudah diestimasi. Akibatnya convergence check di Baum-Welch loop selalu membandingkan LL yang tertinggal satu iterasi.

## Bug Detail

```go
// hmm_regime.go — baumWelchStep
// ...setelah forward pass...
logLikOld := 0.0
for t := 0; t < T; t++ {
    if scale[t] > 0 { logLikOld += math.Log(scale[t]) }
}
// ... estimate newModel ...
return newModel, logLikOld  // BUG: ini LL dari model lama, bukan newModel
```

Outer loop:
```go
newModel, logLik := baumWelchStep(&model, obs)
model = newModel
if iter > 0 && math.Abs(logLik-prevLogLik) < 1e-4 {
    converged = true  // Dibandingkan LL model sebelumnya, bukan model sekarang
}
```

Dampak: HMM bisa berhenti terlalu dini atau terlambat, estimasi regime sedikit tidak optimal.

## Fix

Hitung LL dari `newModel` di akhir `baumWelchStep`:
```go
// Setelah B normalization selesai, hitung LL dari newModel
newLogLik := computeScaledLogLik(&newModel, obs)
return newModel, newLogLik

// Fungsi helper baru:
func computeScaledLogLik(m *HMMModel, obs []int) float64 {
    T := len(obs)
    N := hmmNumStates
    scale := make([]float64, T)
    alpha := make([]float64, N)
    for i := 0; i < N; i++ { alpha[i] = m.Pi[i] * m.B[i][obs[0]] }
    // ... (forward pass) ...
    ll := 0.0
    for _, s := range scale { if s > 0 { ll += math.Log(s) } }
    return ll
}
```

## File yang Kemungkinan Diubah

- `internal/service/price/hmm_regime.go` — baumWelchStep return value + helper function

## Acceptance Criteria

- [ ] `baumWelchStep` mengembalikan LL dari model yang baru diestimasi
- [ ] Convergence check membandingkan LL(current_model) vs LL(previous_model)
- [ ] Unit test: 3-state HMM pada synthetic data converges dalam < 50 iterasi

## Referensi

- `.agents/research/2026-04-02-09-bug-hunt-excursion-garch-hmm.md`
- TASK-171: HMM minimum returns boundary (related)
