# TASK-291: Unit Tests — FRED Rate Differential & Carry Adjustment (TECH-009)

**Priority:** medium
**Type:** tech-refactor / test
**Estimated:** S
**Area:** internal/service/fred/rate_differential.go
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB

## Deskripsi

`internal/service/fred/rate_differential.go` berisi `CarryAdjustment()` dan helper functions yang **tidak memiliki unit tests sama sekali**. Fungsi-fungsi ini adalah pure math — ideal untuk unit test tanpa mock apapun.

**Fungsi yang perlu di-cover:**

- `CarryAdjustment(diff domain.RateDifferential, signalDirection string) float64`
  — menentukan seberapa besar carry rate mempengaruhi COT signal conviction
  — digunakan di signal scoring untuk carry trade confirmation/contradiction
- `clampFloat(v, lo, hi float64) float64` — generic clamp helper
- `roundN(v float64, n int) float64` — rounding helper

Fungsi `FetchCarryRanking()` memerlukan HTTP mock sehingga tidak perlu di-cover sekarang (sudah ada TASK untuk HTTP integration tests di masa depan).

## Perubahan yang Diperlukan

### Buat `internal/service/fred/rate_differential_test.go` (baru)

```go
package fred

import (
    "math"
    "testing"

    "github.com/arkcode369/ark-intelligent/internal/domain"
)

// --- CarryAdjustment ---

func TestCarryAdjustment_BullishWithPositiveDiff(t *testing.T) {
    // Signal BULLISH + positive rate diff = carry confirmation = positive boost
    diff := domain.RateDifferential{Differential: 2.5} // 2.5% higher than USD
    result := CarryAdjustment(diff, "BULLISH")
    if result <= 0 {
        t.Errorf("expected positive boost when carry aligns with bullish signal, got %.2f", result)
    }
    if result > 5 {
        t.Errorf("expected max 5 boost, got %.2f", result)
    }
}

func TestCarryAdjustment_BullishWithNegativeDiff(t *testing.T) {
    // Signal BULLISH + rate diff < -1% = carry headwind = negative penalty
    diff := domain.RateDifferential{Differential: -2.0}
    result := CarryAdjustment(diff, "BULLISH")
    if result >= 0 {
        t.Errorf("expected negative penalty when carry contradicts bullish signal, got %.2f", result)
    }
    if result < -5 {
        t.Errorf("expected max -5 penalty, got %.2f", result)
    }
}

func TestCarryAdjustment_BearishWithNegativeDiff(t *testing.T) {
    // Signal BEARISH + negative rate diff = carry confirmation = positive boost
    diff := domain.RateDifferential{Differential: -3.0}
    result := CarryAdjustment(diff, "BEARISH")
    if result <= 0 {
        t.Errorf("expected positive boost when carry aligns with bearish signal, got %.2f", result)
    }
    if result > 5 {
        t.Errorf("expected max 5 boost, got %.2f", result)
    }
}

func TestCarryAdjustment_BearishWithPositiveDiff(t *testing.T) {
    // Signal BEARISH + rate diff > 1% = carry headwind for short = negative
    diff := domain.RateDifferential{Differential: 2.5}
    result := CarryAdjustment(diff, "BEARISH")
    if result >= 0 {
        t.Errorf("expected negative penalty when carry contradicts bearish, got %.2f", result)
    }
}

func TestCarryAdjustment_Neutral(t *testing.T) {
    // Near-zero differential = neutral carry
    diff := domain.RateDifferential{Differential: 0.5}
    result := CarryAdjustment(diff, "BULLISH")
    if result != 0 {
        t.Errorf("expected 0 for small positive diff with BULLISH (threshold > 0), got %.2f", result)
    }
}

func TestCarryAdjustment_MaxCap(t *testing.T) {
    // Very large differential should be capped at ±5
    diff := domain.RateDifferential{Differential: 100.0}
    result := CarryAdjustment(diff, "BULLISH")
    if result > 5.0+0.01 {
        t.Errorf("expected max 5.0 cap, got %.2f", result)
    }
}

// --- clampFloat ---

func TestClampFloat_WithinRange(t *testing.T) {
    result := clampFloat(3.5, 0, 5)
    if result != 3.5 {
        t.Errorf("want 3.5, got %.2f", result)
    }
}

func TestClampFloat_BelowLow(t *testing.T) {
    result := clampFloat(-1, 0, 5)
    if result != 0 {
        t.Errorf("want 0, got %.2f", result)
    }
}

func TestClampFloat_AboveHigh(t *testing.T) {
    result := clampFloat(10, 0, 5)
    if result != 5 {
        t.Errorf("want 5, got %.2f", result)
    }
}

// --- roundN ---

func TestRoundN_TwoDecimals(t *testing.T) {
    result := roundN(3.14159, 2)
    if math.Abs(result-3.14) > 0.001 {
        t.Errorf("want 3.14, got %.4f", result)
    }
}

func TestRoundN_ZeroDecimals(t *testing.T) {
    result := roundN(3.7, 0)
    if math.Abs(result-4.0) > 0.001 {
        t.Errorf("want 4.0, got %.4f", result)
    }
}
```

## File yang Harus Diubah

1. `internal/service/fred/rate_differential_test.go` — **buat baru** (gunakan `package fred` agar akses internal functions)

## Verifikasi

```bash
go test ./internal/service/fred/... -v -run "TestCarry|TestClamp|TestRound"
# Semua test harus pass
go test ./internal/service/fred/... -cover
# rate_differential.go coverage naik ke >= 50%
```

## Acceptance Criteria

- [ ] `rate_differential_test.go` dibuat dengan minimal 9 test cases
- [ ] `CarryAdjustment` dicovered untuk semua 4 cabang (bullish+pos, bullish+neg, bearish+neg, bearish+pos)
- [ ] Max cap (+/-5) diverifikasi
- [ ] `clampFloat` dan `roundN` di-cover
- [ ] Semua test pass: `go test ./internal/service/fred/...`
- [ ] `go build ./...` tetap clean

## Referensi

- `.agents/TECH_REFACTOR_PLAN.md` — TECH-009 test coverage
- `.agents/research/2026-04-02-03-tech-refactor-gaps-test-coverage-ctx-storage-putaran20.md` — Temuan 5
- `internal/service/fred/rate_differential.go:137-170` — CarryAdjustment, clampFloat, roundN
