# TASK-288: HMM 4-State Regime — Tambah TRENDING State

**Priority:** medium
**Type:** enhancement
**Estimated:** M
**Area:** internal/service/price/hmm_regime.go
**Created by:** Research Agent
**Created at:** 2026-04-02 27:00 WIB

## Deskripsi

HMM saat ini 3-state (RISK_ON, RISK_OFF, CRISIS) di `price/hmm_regime.go`. FEATURE_INDEX menyebut "lebih banyak state" sebagai area riset. Masalah utama: RISK_ON menggabungkan "calm trending" dengan "volatile risk-on" — ini menciptakan ambiguity pada signal generation.

**Solusi: Tambah TRENDING state (State 3)**

```
State 0: RISK_ON    — moderate positive drift, moderate vol, choppy
State 1: RISK_OFF   — near-zero drift, moderate vol, ranging
State 2: CRISIS     — negative drift, high vol, dislocations
State 3: TRENDING   — strong positive drift (>0.3%/wk), LOW vol, Hurst > 0.55
```

**Implikasi Trading:**
- RISK_ON + COT signal → normal conviction
- TRENDING + COT signal → boost conviction +3 (trending = directional signals work better)  
- RISK_OFF → reduce conviction (mean-reversion, not trend)
- CRISIS → minimum conviction (all signals unreliable)

**Dependencies:** TASK-171 (HMM minimum boundary fix) harus selesai dulu.

## Perubahan yang Diperlukan

### 1. Update constants di `internal/service/price/hmm_regime.go`

```go
const (
    HMMRiskOn   = "RISK_ON"
    HMMRiskOff  = "RISK_OFF"
    HMMCrisis   = "CRISIS"
    HMMTrending = "TRENDING"  // NEW

    hmmNumStates    = 4  // was 3
    hmmNumEmissions = 5  // unchanged
)
```

### 2. Update `HMMResult` struct

```go
type HMMResult struct {
    CurrentState       string       `json:"current_state"`        // RISK_ON, RISK_OFF, CRISIS, TRENDING
    StateProbabilities [4]float64   `json:"state_probabilities"`  // was [3]
    TransitionMatrix   [4][4]float64 `json:"transition_matrix"`   // was [3][3]
    ViterbiPath        []string     `json:"viterbi_path,omitempty"`
    TransitionWarning  string       `json:"transition_warning,omitempty"`
    SampleSize         int          `json:"sample_size"`
    Converged          bool         `json:"converged"`
    Iterations         int          `json:"iterations"`
}
```

### 3. Update `HMMModel` struct

```go
type HMMModel struct {
    Pi [4]float64              // was [3]
    A  [4][4]float64           // was [3][3]
    B  [4][5]float64           // was [3][5]
}
```

### 4. Update `initHMMPriors()` — Prior untuk 4 states

```go
func initHMMPriors() HMMModel {
    return HMMModel{
        Pi: [4]float64{0.35, 0.35, 0.15, 0.15}, // initial distribution
        A: [4][4]float64{
            {0.70, 0.15, 0.05, 0.10}, // RISK_ON → can go to TRENDING
            {0.15, 0.70, 0.10, 0.05}, // RISK_OFF → rarely TRENDING
            {0.20, 0.40, 0.35, 0.05}, // CRISIS → mostly to RISK_OFF
            {0.20, 0.05, 0.05, 0.70}, // TRENDING → persistent
        },
        B: [4][5]float64{
            // RISK_ON: moderate positive drift, moderate vol
            {0.10, 0.25, 0.30, 0.25, 0.10},
            // RISK_OFF: near-zero drift, moderate vol
            {0.10, 0.20, 0.40, 0.20, 0.10},
            // CRISIS: negative drift, high vol
            {0.40, 0.25, 0.15, 0.12, 0.08},
            // TRENDING: strong positive drift, LOW vol → tight distribution right-skewed
            {0.05, 0.10, 0.20, 0.35, 0.30},
        },
    }
}
```

### 5. Update `baumWelchStep()` for 4 states

Ubah semua loop yang menggunakan `hmmNumStates` — karena menggunakan constant, seharusnya auto-update. Verifikasi loop indices.

### 6. Update state label function

```go
func stateLabel(s int) string {
    switch s {
    case 0: return HMMRiskOn
    case 1: return HMMRiskOff
    case 2: return HMMCrisis
    case 3: return HMMTrending
    default: return "UNKNOWN"
    }
}
```

### 7. Update transition warning logic

```go
// Warning jika prob CRISIS > 0.25 atau TRENDING → RISK_OFF probability naik
if probs[2] > 0.25 {
    result.TransitionWarning = fmt.Sprintf("CRISIS risk elevated (%.0f%%)", probs[2]*100)
} else if result.CurrentState == HMMTrending && transMatrix[3][0]+transMatrix[3][1] > 0.35 {
    result.TransitionWarning = "TRENDING → may shift to RISK_ON/RISK_OFF"
}
```

### 8. Update semua downstream consumers

Cari `HMMRiskOn\|HMMRiskOff\|HMMCrisis` di codebase, pastikan tidak ada hardcoded `[3]float64`:
```bash
grep -rn "HMMRiskOn\|HMMRiskOff\|HMMCrisis\|\[3\]float64.*hmm\|hmm.*\[3\]" internal/
```

Surface di formatter: tambah warna/icon untuk TRENDING:
```
🔵 TRENDING (85%) — Strong directional regime. Directional signals +reliable.
```

## File yang Harus Diubah

1. `internal/service/price/hmm_regime.go` — update constants, structs, priors, transitions
2. `internal/adapter/telegram/formatter_quant.go` — update HMM state display untuk TRENDING
3. Cek downstream: `context.go`, `aggregator.go`, `regime.go` untuk hardcoded [3]

## Verifikasi

```bash
go build ./...
go test ./internal/service/price/...
# Manual: /quant EURUSD → lihat "TRENDING" muncul jika pair sedang trending
# Test: StateProbabilities harus sum to 1.0 untuk semua 4 states
```

## Acceptance Criteria

- [ ] `hmmNumStates = 4` — tidak ada hardcoded `3` yang tersisa
- [ ] `StateProbabilities[4]float64` sum to 1.0
- [ ] TRENDING state dengan emission prior yang benar (low vol, positive drift)
- [ ] TransitionWarning diupdate untuk 4-state system
- [ ] `/quant` output menampilkan TRENDING dengan icon yang benar
- [ ] `go build ./...` + `go test ./internal/service/price/...` clean

## Dependencies

- TASK-171 (HMM minimum boundary fix) — sebaiknya selesai dulu sebelum ini

## Referensi

- `.agents/research/2026-04-02-27-feature-index-gaps-carry-gjrgarch-oi4w-hmm4-vix-putaran19.md` — GAP 4
- `internal/service/price/hmm_regime.go:54` — EstimateHMMRegime (template)
- `internal/service/price/audit_test.go:301` — HMM audit tests yang perlu diupdate
- FEATURE_INDEX.md → "HMM → lebih banyak state, online learning"
