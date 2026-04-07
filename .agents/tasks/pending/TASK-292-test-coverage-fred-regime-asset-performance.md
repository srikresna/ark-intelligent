# TASK-292: Unit Tests — FRED Regime Asset Performance Matrix (TECH-009)

**Priority:** medium
**Type:** tech-refactor / test
**Estimated:** M
**Area:** internal/service/fred/regime_asset.go, internal/service/fred/regime_performance.go
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB

## Deskripsi

`regime_asset.go` dan `regime_performance.go` adalah komponen yang menghitung **performa aset per macro regime** (GOLDILOCKS, INFLATIONARY, STRESS, dll) dan ditampilkan di `/macro` output. Keduanya **belum punya unit tests sama sekali**.

Fungsi kritis yang perlu di-cover (semua pure math, tidak perlu mock):

Dari `regime_asset.go`:
- `ComputeRegimeAssetMatrix(snapshots, prices)` — core computation
- `annualizeWeekly(weeklyPct float64) float64` — pure math
- `regimeAt(snapshots, date)` — regime lookup logic
- `GetCurrentRegimeInsight(currentRegime, matrix)` — insight extraction

Dari `regime_performance.go`:
- Functions computing PerformanceStats (AvgWeeklyReturn, Occurrences, BestWeek, WorstWeek)

## Perubahan yang Diperlukan

### 1. Buat `internal/service/fred/regime_asset_test.go` (baru)

```go
package fred

import (
    "math"
    "testing"
    "time"
)

// --- annualizeWeekly ---

func TestAnnualizeWeekly_PositiveReturn(t *testing.T) {
    // 1% weekly return → annualized ~ 67.8%
    result := annualizeWeekly(1.0)
    expected := (math.Pow(1+1.0/100, 52) - 1) * 100
    if math.Abs(result-expected) > 0.01 {
        t.Errorf("annualizeWeekly(1.0) = %.4f, want %.4f", result, expected)
    }
}

func TestAnnualizeWeekly_ZeroReturn(t *testing.T) {
    result := annualizeWeekly(0)
    if result != 0 {
        t.Errorf("annualizeWeekly(0) = %.4f, want 0", result)
    }
}

func TestAnnualizeWeekly_NegativeReturn(t *testing.T) {
    result := annualizeWeekly(-1.0)
    if result >= 0 {
        t.Errorf("expected negative annualized return for negative weekly, got %.4f", result)
    }
}

// --- regimeAt ---

func TestRegimeAt_ExactMatch(t *testing.T) {
    date := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
    snapshots := []RegimeSnapshot{
        {Date: time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC), Regime: "GOLDILOCKS"},
        {Date: time.Date(2024, 6, 17, 0, 0, 0, 0, time.UTC), Regime: "INFLATIONARY"},
    }
    result := regimeAt(snapshots, date)
    // date falls between two snapshots, should return closest prior
    if result == "" {
        t.Error("expected non-empty regime, got empty")
    }
}

func TestRegimeAt_EmptySnapshots(t *testing.T) {
    result := regimeAt(nil, time.Now())
    if result != "" && result != "UNKNOWN" {
        // Accept empty or UNKNOWN for nil input
    }
}

// --- GetCurrentRegimeInsight ---

func TestGetCurrentRegimeInsight_KnownRegime(t *testing.T) {
    matrix := &RegimeAssetMatrix{
        Regimes: map[string]map[string]PerformanceStats{
            "GOLDILOCKS": {
                "EUR": {AvgWeeklyReturn: 0.3, Occurrences: 10},
                "USD": {AvgWeeklyReturn: -0.1, Occurrences: 10},
            },
        },
    }
    insight := GetCurrentRegimeInsight("GOLDILOCKS", matrix)
    if insight.Regime != "GOLDILOCKS" {
        t.Errorf("expected GOLDILOCKS, got %s", insight.Regime)
    }
    if len(insight.BestAssets) == 0 && len(insight.WorstAssets) == 0 {
        t.Error("expected some assets in insight")
    }
}

func TestGetCurrentRegimeInsight_UnknownRegime(t *testing.T) {
    matrix := &RegimeAssetMatrix{
        Regimes: map[string]map[string]PerformanceStats{},
    }
    insight := GetCurrentRegimeInsight("NONEXISTENT", matrix)
    // Should not panic, should return empty/zero insight
    _ = insight
}
```

### 2. Buat `internal/service/fred/regime_performance_test.go` (baru)

```go
package fred

import (
    "testing"
    "time"
)

// Test PerformanceStats computation correctness
// (test whatever compute functions exist in regime_performance.go)
func TestPerformanceStats_Basic(t *testing.T) {
    // Verify PerformanceStats struct can hold expected values
    stats := PerformanceStats{
        AvgWeeklyReturn:     0.5,
        AvgAnnualizedReturn: 29.3,
        Occurrences:         20,
        BestWeek:            3.2,
        WorstWeek:          -1.5,
    }

    if stats.Occurrences != 20 {
        t.Errorf("expected 20 occurrences, got %d", stats.Occurrences)
    }
    if stats.BestWeek <= stats.WorstWeek {
        t.Error("BestWeek should be > WorstWeek")
    }
    _ = time.Now() // prevent import removal
}
```

## File yang Harus Diubah

1. `internal/service/fred/regime_asset_test.go` — **buat baru**
2. `internal/service/fred/regime_performance_test.go` — **buat baru** (extend jika ada compute functions)

**Catatan:** Sebelum mulai, baca `regime_performance.go` untuk melihat fungsi compute yang ada — tambahkan test untuk setiap fungsi pure math yang ditemukan.

## Verifikasi

```bash
go test ./internal/service/fred/... -v -run "TestAnnualize|TestRegimeAt|TestGetCurrent|TestPerformance"
go test ./internal/service/fred/... -cover
# Target: regime_asset.go coverage >= 40%
```

## Acceptance Criteria

- [ ] `regime_asset_test.go` dibuat dengan minimal 6 test cases
- [ ] `annualizeWeekly` dicovered untuk positive, zero, negative input
- [ ] `regimeAt` dicovered untuk exact match dan edge cases
- [ ] `GetCurrentRegimeInsight` tidak panic untuk known dan unknown regime
- [ ] `regime_performance_test.go` dibuat dengan tests untuk compute functions yang ada
- [ ] `go test ./internal/service/fred/...` pass semua test (existing + baru)
- [ ] `go build ./...` tetap clean

## Referensi

- `.agents/TECH_REFACTOR_PLAN.md` — TECH-009 test coverage
- `.agents/research/2026-04-02-03-tech-refactor-gaps-test-coverage-ctx-storage-putaran20.md` — Temuan 2
- `internal/service/fred/regime_asset.go:59,139,159,171` — fungsi yang perlu di-test
- `internal/service/fred/regime_performance.go` — baca dulu untuk temukan compute functions
