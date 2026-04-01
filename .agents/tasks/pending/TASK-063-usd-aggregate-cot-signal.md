# TASK-063: USD Aggregate COT Signal (Cross-Currency Synthesis)

**Priority:** MEDIUM
**Cycle:** Siklus 3 — Fitur Baru
**Estimated Complexity:** LOW
**Research Ref:** `.agents/research/2026-04-01-02-fitur-baru-siklus3-lanjutan.md`

---

## Deskripsi

Buat USD Aggregate COT signal dengan mensintesis net positions dari semua major FX pairs (EUR, GBP, JPY, CHF, AUD, CAD, NZD) relative to USD. Hasilkan satu sinyal USD directional bias yang lebih komprehensif dari DX futures saja, termasuk detection divergence antara USD aggregate vs DX direct.

## Konteks Teknis

### Foundation Yang Ada
- `internal/service/cot/analyzer.go` — `AnalyzeAll()` returns `[]domain.COTAnalysis` untuk semua contracts
- `internal/domain/cot.go` — `DefaultCOTContracts` dengan currency info dan Inverse flag
- `domain.COTContract.Inverse` — DX (USD Index) sudah di-mark `Inverse: true`
- `domain.COTAnalysis.NetPosition` — net position sudah computed

### Logic Algoritma

**USD Aggregate Computation:**
```go
// For each major FX pair, normalize net position to "USD exposure direction"
// EUR (not inverse): net LONG EUR = net SHORT USD → multiply by -1
// JPY (not inverse): net LONG JPY = net SHORT USD → multiply by -1  
// GBP, AUD, CAD, NZD: same logic as EUR (all quoted as X/USD)
// CHF: same logic
// DX (inverse=true): net LONG DX = net LONG USD → multiply by +1

// Step 1: Normalize each pair's net to "USD direction"
func computeUSDDirection(a domain.COTAnalysis, contract domain.COTContract) float64 {
    if contract.Inverse {
        return a.NetPosition // DX: long = USD long
    }
    return -a.NetPosition // EUR/GBP/etc: long = USD short
}

// Step 2: Normalize by Open Interest to make comparable
// Step 3: Sum weighted contributions
// Step 4: Compare to DX direct positioning
```

**Divergence Detection:**
```
DX Direct Signal: DX net position direction (from DX COT)
USD Aggregate Signal: sum of cross-pair USD exposures

If DX says BULLISH but aggregate says BEARISH → divergence → unreliable DX signal
If both agree → high conviction USD directional call
```

### Files Yang Perlu Dibuat/Dimodifikasi

**`internal/service/cot/usd_aggregate.go`:**
```go
package cot

import "github.com/arkcode369/ark-intelligent/internal/domain"

// USDAggregate holds the synthesized USD positioning signal.
type USDAggregate struct {
    // Normalized score: +100 = extreme USD bullish, -100 = extreme USD bearish
    Score          float64
    Direction      string // "BULLISH", "BEARISH", "NEUTRAL"
    
    // Per-currency contributions
    Contributions  map[string]float64 // currency → normalized USD direction contribution
    
    // DX direct vs aggregate comparison
    DXDirectScore  float64 // from DX futures COT directly
    DXDirectDir    string  
    Divergence     bool   // DX disagrees with aggregate
    DivergenceDesc string
    
    // Conviction
    ConvictionPct  float64 // % of pairs agreeing with aggregate direction
    HighConviction bool    // >= 70% pairs aligned
}

// ComputeUSDAggregate synthesizes USD positioning from all FX pair analyses.
// analyses must include all major pairs + DX.
func ComputeUSDAggregate(analyses []domain.COTAnalysis, contracts []domain.COTContract) USDAggregate {
    // Implementation:
    // 1. Filter to FX pairs only (EUR, GBP, JPY, CHF, AUD, CAD, NZD, DX)
    // 2. Compute OI-normalized net for each
    // 3. Apply inverse flag
    // 4. Compute aggregate score and conviction
    // 5. Compare to DX direct
    // 6. Return USDAggregate
}
```

### Integration ke /bias Command

File `internal/adapter/telegram` handler untuk /bias — tambahkan section USD aggregate:

```
💵 USD Aggregate COT Signal
  
  Pair Contributions:
  • EUR: -0.32 (short USD via long EUR)
  • GBP: -0.18 (short USD via long GBP)
  • JPY: +0.41 (long USD via short JPY)
  • CHF: -0.22 (short USD via long CHF)
  • AUD: -0.15
  • CAD: -0.08
  
  Aggregate Score: -0.54 → MILDLY BEARISH USD
  DX Direct: +0.30 → MILDLY BULLISH USD
  ⚠️ DIVERGENCE: DX says bullish, cross-pairs say bearish
  → Low conviction USD call — await resolution
  
  Conviction: 57% pairs aligned (below 70% threshold)
```

## Acceptance Criteria
- [ ] `ComputeUSDAggregate()` fungsi yang menerima `[]COTAnalysis` dan return `USDAggregate`
- [ ] OI-normalization berjalan (agar EUR tidak dominasi hanya karena OI besar)
- [ ] Divergence detection antara DX direct vs aggregate signal
- [ ] ConvictionPct computed dari % pairs yang aligned dengan aggregate direction
- [ ] Output terintegrasi ke /bias command
- [ ] Unit test dengan mock analyses data

## Notes
- OI normalization: `normalizedNet = netPosition / openInterest` (jika OI tersedia)
- Fallback: normalisasi per-currency by historial max net position (percentile-based)
- Pairs yang tidak ada datanya (karena fetch gagal) harus dieskip, bukan error
- `domain.COTContract.Inverse` sudah ada — gunakan langsung
