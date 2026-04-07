# TASK-290: Unit Tests — COT Analyzer & Regime Detection (TECH-009)

**Priority:** high
**Type:** tech-refactor / test
**Estimated:** M
**Area:** internal/service/cot/analyzer.go, internal/service/cot/regime.go
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB

## Deskripsi

`internal/service/cot/analyzer.go` (core COT analysis engine) dan `internal/service/cot/regime.go` (risk regime detector) **tidak memiliki unit tests sama sekali**, padahal keduanya adalah critical business logic yang menentukan semua output `/cot`.

**Fungsi kritis yang perlu di-cover:**

Dari `analyzer.go`:
- `computeCOTIndex(nets []float64) float64` — percentile ranking (0–100), pure math
- `computeSentiment(a COTAnalysis) float64` — weighted composite score, pure math
- `classifySignal(cotIndex, momentum float64, isCommercial bool) string` — rule-based
- `classifySignalStrength(a COTAnalysis) domain.SignalStrength` — pure rule-based
- `detectDivergence(specNetChange, commNetChange float64) bool` — pure math
- `computeCrowding(r domain.COTRecord, reportType string) float64` — pure math

Dari `regime.go`:
- `DetectRegime(analyses []COTAnalysis) RegimeResult` — rule-based classifier
- Confidence scoring logic

**Semua fungsi di atas tidak melakukan I/O** — sangat mudah di-unit test tanpa mock apapun.

## Perubahan yang Diperlukan

### 1. Buat `internal/service/cot/analyzer_test.go` (baru)

```go
package cot

import (
    "math"
    "testing"

    "github.com/arkcode369/ark-intelligent/internal/domain"
)

// --- computeCOTIndex ---

func TestComputeCOTIndex_Neutral(t *testing.T) {
    // Insufficient data → 50.0 neutral
    result := computeCOTIndex([]float64{100})
    if result != 50.0 {
        t.Errorf("want 50.0, got %.2f", result)
    }
}

func TestComputeCOTIndex_AtMax(t *testing.T) {
    // Current at max of range → 100
    nets := []float64{100, 50, 0, -50, -100} // current = 100
    result := computeCOTIndex(nets)
    if math.Abs(result-100.0) > 0.01 {
        t.Errorf("want 100.0, got %.2f", result)
    }
}

func TestComputeCOTIndex_AtMin(t *testing.T) {
    // Current at min → 0
    nets := []float64{-100, 50, 100, 0, -50}
    result := computeCOTIndex(nets)
    if math.Abs(result-0.0) > 0.01 {
        t.Errorf("want 0.0, got %.2f", result)
    }
}

func TestComputeCOTIndex_ZeroSpan(t *testing.T) {
    // All same values → 50.0 neutral
    result := computeCOTIndex([]float64{42, 42, 42})
    if result != 50.0 {
        t.Errorf("want 50.0, got %.2f", result)
    }
}

// --- classifySignal ---

func TestClassifySignal_ExtremeLong(t *testing.T) {
    result := classifySignal(95, 0.5, false)
    if result != "BULLISH" && result != "EXTREME_LONG" {
        t.Errorf("expected bullish signal for high COT index, got %s", result)
    }
}

func TestClassifySignal_ExtremeShort(t *testing.T) {
    result := classifySignal(5, -0.5, false)
    if result != "BEARISH" && result != "EXTREME_SHORT" {
        t.Errorf("expected bearish signal for low COT index, got %s", result)
    }
}

// --- detectDivergence ---

func TestDetectDivergence_True(t *testing.T) {
    // Spec bullish, commercial bearish = divergence
    if !detectDivergence(1000, -1000) {
        t.Error("expected divergence when spec and commercial move opposite")
    }
}

func TestDetectDivergence_False(t *testing.T) {
    // Both moving same direction = no divergence
    if detectDivergence(1000, 500) {
        t.Error("expected no divergence when both move in same direction")
    }
}

// --- computeCrowding ---

func TestComputeCrowding_Legacy(t *testing.T) {
    r := domain.COTRecord{
        NonCommLong:  100000,
        NonCommShort: 10000,
    }
    score := computeCrowding(r, "LEGACY")
    // Extreme long crowding — score should be elevated
    if score <= 50 {
        t.Errorf("expected high crowding score for extreme net long, got %.2f", score)
    }
}
```

### 2. Buat `internal/service/cot/regime_test.go` (baru)

```go
package cot

import (
    "testing"

    "github.com/arkcode369/ark-intelligent/internal/domain"
)

func makeAnalysis(currency string, sentiment float64) domain.COTAnalysis {
    return domain.COTAnalysis{
        Contract:       domain.COTContract{Currency: currency},
        SentimentScore: sentiment,
    }
}

func TestDetectRegime_RiskOn(t *testing.T) {
    // Safe havens (JPY, CHF) bearish = risk-on
    analyses := []domain.COTAnalysis{
        makeAnalysis("JPY", -60), // specs selling JPY = risk-on
        makeAnalysis("CHF", -50), // specs selling CHF = risk-on
        makeAnalysis("AUD", 40),  // specs buying AUD = risk-on
    }
    result := DetectRegime(analyses)
    if result.Regime != RegimeRiskOn {
        t.Errorf("expected RISK-ON, got %s (confidence: %.1f)", result.Regime, result.Confidence)
    }
    if result.Confidence <= 0 {
        t.Error("expected positive confidence")
    }
}

func TestDetectRegime_RiskOff(t *testing.T) {
    // Safe havens bullish = risk-off
    analyses := []domain.COTAnalysis{
        makeAnalysis("JPY", 60),  // specs buying JPY = risk-off
        makeAnalysis("CHF", 50),  // specs buying CHF = risk-off
        makeAnalysis("AUD", -40), // specs selling AUD = risk-off
    }
    result := DetectRegime(analyses)
    if result.Regime != RegimeRiskOff {
        t.Errorf("expected RISK-OFF, got %s", result.Regime)
    }
}

func TestDetectRegime_Uncertainty(t *testing.T) {
    // Mixed signals = uncertainty
    analyses := []domain.COTAnalysis{
        makeAnalysis("JPY", 5),  // near neutral
        makeAnalysis("AUD", -5), // near neutral
    }
    result := DetectRegime(analyses)
    // Low confidence or uncertainty expected
    if result.Confidence > 75 {
        t.Errorf("expected low confidence for mixed signals, got %.1f", result.Confidence)
    }
}

func TestDetectRegime_EmptyInput(t *testing.T) {
    result := DetectRegime(nil)
    if result.Regime == "" {
        t.Error("expected non-empty regime even for empty input")
    }
}
```

## File yang Harus Diubah

1. `internal/service/cot/analyzer_test.go` — **buat baru** (unit tests untuk package-level functions)
2. `internal/service/cot/regime_test.go` — **buat baru** (unit tests untuk DetectRegime)

**Catatan:** Fungsi yang ditest adalah package-level functions (lowercase), bukan method. Test file harus menggunakan `package cot` (bukan `package cot_test`) agar bisa akses internal functions.

## Verifikasi

```bash
go test ./internal/service/cot/... -v -run "TestComputeCOT|TestClassify|TestDetect|TestCompute"
# Semua test harus pass
go test ./internal/service/cot/... -cover
# Target: coverage analyzer.go + regime.go naik ke minimal 40%
```

## Acceptance Criteria

- [ ] `analyzer_test.go` dibuat dengan minimal 6 test cases untuk computeCOTIndex, classifySignal, detectDivergence, computeCrowding
- [ ] `regime_test.go` dibuat dengan minimal 4 test cases: RiskOn, RiskOff, Uncertainty, EmptyInput
- [ ] Semua test pass: `go test ./internal/service/cot/...`
- [ ] `go build ./...` tetap clean
- [ ] Coverage untuk `analyzer.go` dan `regime.go` meningkat ke >= 35%

## Referensi

- `.agents/TECH_REFACTOR_PLAN.md` — TECH-009 test coverage
- `.agents/research/2026-04-02-03-tech-refactor-gaps-test-coverage-ctx-storage-putaran20.md` — Temuan 1
- `internal/service/cot/analyzer.go:575-800` — fungsi pure math yang perlu di-test
- `internal/service/cot/regime.go:40-100` — DetectRegime logic
