# TASK-294: Unit Tests — FRED Macro Alerts & Threshold Detection (TECH-009)

**Priority:** high
**Type:** tech-refactor / test
**Estimated:** M
**Area:** internal/service/fred/alerts.go
**Created by:** Research Agent
**Created at:** 2026-04-02 03:00 WIB

## Deskripsi

`internal/service/fred/alerts.go` berisi fungsi `CheckAlerts(current, previous *MacroData) []MacroAlert` yang **tidak memiliki unit tests sama sekali**. Ini adalah **critical business logic** karena:

1. Directly triggers user notifications (broadcast alert ke Telegram) ketika threshold terlewati
2. Mendeteksi macro regime shifts: yield curve inversion/uninversion, CPI breakthrough, employment shock, dll
3. False positive atau missed alert langsung berdampak ke user experience

`CheckAlerts()` adalah **pure function** — menerima 2 struct dan return slice, tidak ada side effects, tidak ada I/O. Sangat mudah di-unit test.

**Jumlah alert types** (dari kode): yield curve (2Y-10Y inversion/uninversion), 3M-10Y spread, plus berbagai threshold lainnya. Total banyak cabang yang perlu di-cover.

## Perubahan yang Diperlukan

### Buat `internal/service/fred/alerts_test.go` (baru)

```go
package fred

import (
    "testing"
)

// Helper untuk membuat MacroData minimal
func newMacroData() *MacroData {
    return &MacroData{}
}

// --- CheckAlerts: yield curve events ---

func TestCheckAlerts_YieldCurveInversion(t *testing.T) {
    // 2Y-10Y spread goes from positive to negative = inversion
    prev := &MacroData{YieldSpread: 0.5}  // sebelumnya positif
    curr := &MacroData{YieldSpread: -0.2} // sekarang negatif

    alerts := CheckAlerts(curr, prev)

    found := false
    for _, a := range alerts {
        if a.Type == AlertYieldInvert {
            found = true
            if a.Severity != "HIGH" {
                t.Errorf("yield inversion should be HIGH severity, got %s", a.Severity)
            }
            break
        }
    }
    if !found {
        t.Error("expected AlertYieldInvert to be triggered")
    }
}

func TestCheckAlerts_YieldCurveUninversion(t *testing.T) {
    // 2Y-10Y spread goes from negative to positive = uninversion
    prev := &MacroData{YieldSpread: -0.5}
    curr := &MacroData{YieldSpread: 0.3}

    alerts := CheckAlerts(curr, prev)

    found := false
    for _, a := range alerts {
        if a.Type == AlertYieldUninvert {
            found = true
            break
        }
    }
    if !found {
        t.Error("expected AlertYieldUninvert to be triggered")
    }
}

func TestCheckAlerts_NoYieldChange(t *testing.T) {
    // 2Y-10Y stays positive = no inversion/uninversion alert
    prev := &MacroData{YieldSpread: 0.5}
    curr := &MacroData{YieldSpread: 0.6}

    alerts := CheckAlerts(curr, prev)

    for _, a := range alerts {
        if a.Type == AlertYieldInvert || a.Type == AlertYieldUninvert {
            t.Errorf("unexpected yield alert when spread stays positive: %s", a.Type)
        }
    }
}

func TestCheckAlerts_3MYieldUninversion(t *testing.T) {
    // 3M-10Y uninversion (stronger recession predictor)
    prev := &MacroData{
        YieldSpread:  0.1,  // 2Y-10Y neutral
        Spread3M10Y: -0.8,  // 3M-10Y negative
    }
    curr := &MacroData{
        YieldSpread:  0.1,  // 2Y-10Y unchanged
        Spread3M10Y:  0.2,  // 3M-10Y turns positive
    }

    alerts := CheckAlerts(curr, prev)

    found := false
    for _, a := range alerts {
        if a.Type == Alert3MUninvert {
            found = true
            break
        }
    }
    if !found {
        t.Error("expected Alert3MUninvert to be triggered")
    }
}

// --- CheckAlerts: nil safety ---

func TestCheckAlerts_NilCurrent(t *testing.T) {
    prev := &MacroData{YieldSpread: 0.5}
    alerts := CheckAlerts(nil, prev)
    if alerts != nil && len(alerts) > 0 {
        t.Error("expected nil/empty alerts when current is nil")
    }
}

func TestCheckAlerts_NilPrevious(t *testing.T) {
    curr := &MacroData{YieldSpread: -0.5}
    alerts := CheckAlerts(curr, nil)
    if alerts != nil && len(alerts) > 0 {
        t.Error("expected nil/empty alerts when previous is nil")
    }
}

func TestCheckAlerts_BothNil(t *testing.T) {
    alerts := CheckAlerts(nil, nil)
    if alerts != nil && len(alerts) > 0 {
        t.Error("expected nil/empty alerts for nil input")
    }
}

// --- CheckAlerts: no spurious alerts ---

func TestCheckAlerts_NoChangeNoAlerts(t *testing.T) {
    // Identical data = no alerts (nothing changed)
    data := &MacroData{
        YieldSpread:  0.5,
        Spread3M10Y:  0.3,
    }
    alerts := CheckAlerts(data, data)

    // Count high-severity alerts — should be none for identical data
    highSeverity := 0
    for _, a := range alerts {
        if a.Severity == "HIGH" {
            highSeverity++
        }
    }
    if highSeverity > 0 {
        t.Errorf("expected no HIGH severity alerts for identical data, got %d", highSeverity)
    }
}

// --- MacroAlert struct validation ---

func TestMacroAlert_FieldsPopulated(t *testing.T) {
    prev := &MacroData{YieldSpread: 0.1}
    curr := &MacroData{YieldSpread: -0.3}

    alerts := CheckAlerts(curr, prev)

    for _, a := range alerts {
        if a.Type == AlertYieldInvert {
            if a.Title == "" {
                t.Error("alert Title should not be empty")
            }
            if a.Description == "" {
                t.Error("alert Description should not be empty")
            }
            if a.Severity == "" {
                t.Error("alert Severity should not be empty")
            }
            // Value and Previous should be populated
            if a.Value != curr.YieldSpread {
                t.Errorf("alert Value should be current YieldSpread %.2f, got %.2f", curr.YieldSpread, a.Value)
            }
        }
    }
}
```

**Catatan:** Sebelum mulai, baca `alerts.go` untuk temukan semua `AlertType` constants yang ada. Jika ada threshold alerts lain (CPI, employment, dll), tambahkan test cases untuk mereka juga.

## File yang Harus Diubah

1. `internal/service/fred/alerts_test.go` — **buat baru** (gunakan `package fred`)

## Verifikasi

```bash
go test ./internal/service/fred/... -v -run "TestCheckAlerts"
# Semua test harus pass
go test ./internal/service/fred/... -cover
# alerts.go coverage naik ke >= 50%
```

## Acceptance Criteria

- [ ] `alerts_test.go` dibuat dengan minimal 8 test cases
- [ ] Yield curve inversion/uninversion alert dicovered
- [ ] 3M-10Y spread alert dicovered
- [ ] Nil input safety dicovered (tidak panic)
- [ ] No-change no-alerts case dicovered
- [ ] Alert fields (Title, Description, Severity, Value, Previous) divalidasi
- [ ] Semua test pass: `go test ./internal/service/fred/...`
- [ ] `go build ./...` tetap clean

## Referensi

- `.agents/TECH_REFACTOR_PLAN.md` — TECH-009 test coverage, TECH-007 error handling
- `.agents/research/2026-04-02-03-tech-refactor-gaps-test-coverage-ctx-storage-putaran20.md` — Temuan 2
- `internal/service/fred/alerts.go:44` — CheckAlerts() signature
- `internal/service/fred/alerts.go:32-43` — MacroAlert struct dan AlertType constants
- `internal/service/fred/alerts.go:50-100` — yield curve detection logic (template untuk test data)
