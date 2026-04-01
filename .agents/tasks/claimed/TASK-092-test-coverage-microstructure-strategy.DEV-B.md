# TASK-092: Test Coverage — Microstructure & Strategy Engine (TECH-009)

**Priority:** MEDIUM
**Type:** Tech Refactor / Test
**Ref:** TECH-009 in TECH_REFACTOR_PLAN.md
**Branch target:** dev-b atau dev-c
**Estimated size:** Medium (M) — 200-350 LOC test
**Created by:** Research Agent
**Created at:** 2026-04-01 15:30 WIB
**Siklus:** 4 — Technical Refactor

---

## Problem

Dua service yang merupakan orchestration layer (multi-signal → trade decision) tidak punya test:
- `internal/service/microstructure/engine.go` (262 LOC) — ZERO tests
- `internal/service/strategy/engine.go` (283 LOC) + `types.go` (158 LOC) — ZERO tests

Strategy engine adalah "brain" dari trade signal generation. Bug di sini bisa menyebabkan
signal yang salah dikirim ke semua subscriber tanpa terdeteksi.

---

## Apa yang Perlu Di-test

### microstructure/engine.go
Berdasarkan analisis file (262 LOC), engine ini menghitung:
- Market microstructure indicators (bid-ask spread proxy, order flow imbalance)
- Fungsi utama: `Analyze()` atau equivalent

**Test cases:**
1. Input data normal → output dalam range yang valid
2. Input data kosong/nil → tidak crash, return zero struct
3. Edge case: single data point → graceful handling

### strategy/engine.go
Strategy engine mengambil:
- `COTBias` map → bullish/bearish per contract
- `VolRegime` map → expanding/contracting
- `CarryBps` map → carry differential

Dan menghasilkan trade signals per asset.

**Test cases:**
1. Semua bullish signals → output long recommendation
2. Mixed signals → neutral/no signal
3. Empty input → tidak crash, return empty slice
4. Single asset dengan full data → verifikasi field output

---

## Contoh Test Structure

```go
// strategy/engine_test.go
package strategy_test

import (
    "testing"
    "github.com/arkcode369/ark-intelligent/internal/service/strategy"
    "github.com/stretchr/testify/assert"
)

func TestEngine_EmptyInput(t *testing.T) {
    eng := strategy.NewEngine()
    result := eng.Generate(strategy.Input{})
    assert.NotNil(t, result)
    assert.Empty(t, result.Signals)
}

func TestEngine_AllBullish(t *testing.T) {
    eng := strategy.NewEngine()
    result := eng.Generate(strategy.Input{
        COTBias:   map[string]string{"099741": "BULLISH", "096742": "BULLISH"},
        VolRegime: map[string]string{"099741": "NORMAL", "096742": "NORMAL"},
        CarryBps:  map[string]float64{"099741": 50, "096742": 30},
    })
    // All bullish → expect some long signals
    hasBullish := false
    for _, s := range result.Signals {
        if s.Direction == "LONG" { hasBullish = true }
    }
    assert.True(t, hasBullish, "expected at least one LONG signal with all bullish COT bias")
}
```

---

## Acceptance Criteria

- [ ] Buat `internal/service/microstructure/engine_test.go`
- [ ] Buat `internal/service/strategy/engine_test.go`
- [ ] Minimal 4 test functions per file (empty input, normal input, edge case, output validation)
- [ ] Semua test pass: `go test ./internal/service/microstructure/... ./internal/service/strategy/...`
- [ ] `go build ./...` dan `go vet ./...` clean

---

## Catatan

- Baca source files dulu untuk memahami signature fungsi yang ada
- Kalau engine butuh dependencies (repository, dll) → gunakan nil/stub sederhana
- Jangan mock external I/O — unit test ini untuk business logic saja
