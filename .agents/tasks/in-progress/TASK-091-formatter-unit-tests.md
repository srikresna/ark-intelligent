# TASK-091: Test Coverage — formatter.go Core Functions (TECH-009)

**Priority:** HIGH
**Type:** Tech Refactor / Test
**Ref:** TECH-009 in TECH_REFACTOR_PLAN.md
**Branch target:** dev-b atau dev-c
**Estimated size:** Large (L) — 400-600 LOC test
**Created by:** Research Agent
**Created at:** 2026-04-01 15:30 WIB
**Siklus:** 4 — Technical Refactor

---

## Problem

`internal/adapter/telegram/formatter.go` memiliki 4,489 LOC dan 50+ exported functions
tapi **ZERO test file**. TECH plan menargetkan 80% coverage untuk format layer
karena ini pure formatting (tidak ada I/O, mudah di-test).

Saat ini setiap perubahan ke formatter bisa break output format tanpa terdeteksi
sampai user complain di production.

---

## Scope: Fungsi Prioritas Tinggi

Fokuskan pada fungsi yang paling sering berubah dan paling critical:

### Group A — COT (highest usage)
- `FormatCOTOverview(analyses, convictions)` — output kompleks 30+ lines
- `FormatCOTDetail(analysis)` / `FormatCOTDetailWithCode(analysis, code)` — 140 LOC function
- `FormatRanking(analyses, date)` — table output yang bisa regresi

### Group B — Macro/FRED
- `FormatMacroRegime(regime, data)` — sering berubah saat ada FRED feature
- `FormatFREDContext(data, regime)` — critical untuk macro command
- `FormatMacroSummary(regime, data, implications)` — composite output

### Group C — Sentiment (sudah ada TASK-065 untuk service, tapi bukan formatter)
- `FormatSentiment(data, macroRegime)` — 185 LOC, banyak conditional path
- `sentimentGauge(score, width)` — pure math → easy to test
- `fearGreedEmoji(score)` — boundary testing penting

### Group D — Helper functions (quick wins)
- `directionArrow(actual, forecast, impactDirection)` — boundary cases
- `cotIdxLabel(idx)` — exact label mapping
- `convictionMiniBar(score, dir)` — bar rendering

---

## Cara Membuat Test

```go
// Contoh test structure di formatter_test.go:
package telegram_test

import (
    "testing"
    "github.com/arkcode369/ark-intelligent/internal/adapter/telegram"
    "github.com/arkcode369/ark-intelligent/internal/domain"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFormatCOTOverview_EmptyInput(t *testing.T) {
    f := telegram.NewFormatter()
    out := f.FormatCOTOverview(nil, nil)
    assert.Contains(t, out, "COT") // minimal: tidak crash dan ada header
}

func TestFormatCOTOverview_SingleCurrency(t *testing.T) {
    f := telegram.NewFormatter()
    analyses := []domain.COTAnalysis{
        {Currency: "EUR", Index: 72.5, Trend: "BULLISH"},
    }
    out := f.FormatCOTOverview(analyses, nil)
    assert.Contains(t, out, "EUR")
    assert.Contains(t, out, "72.5")
}

func TestSentimentGauge_Boundaries(t *testing.T) {
    // Test boundary values
    tests := []struct{score float64; wantContains string}{
        {0,   "▱"}, // extreme fear
        {50,  "▰"}, // neutral
        {100, "▰"}, // extreme greed
    }
    // ...
}
```

---

## Acceptance Criteria

- [ ] Buat `internal/adapter/telegram/formatter_test.go` (file baru)
- [ ] Minimal 15 test functions covering Group A, B, C, D
- [ ] Semua test pass: `go test ./internal/adapter/telegram/...`
- [ ] Test untuk nil/empty input (jangan crash)
- [ ] Test untuk boundary values (score 0, 50, 100; index 0, 50, 100)
- [ ] Test untuk expected string output (snapshot-style: assert.Contains)
- [ ] `go build ./...` dan `go vet ./...` clean

---

## Catatan

- Gunakan testify (sudah ada di go.mod)
- Untuk fungsi yang butuh mock data, buat test fixtures sederhana inline
- Jangan test private helper functions secara langsung (hanya exported)
- Ini prioritas HIGH karena formatter split (TASK-015) akan lebih aman jika ada tests dulu
