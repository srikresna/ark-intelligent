# Research Report: Bug Hunting Siklus 5 Putaran 2 — Bounds Checks, Div-by-Zero, Goroutine Safety

**Tanggal:** 2026-04-02 00:00 WIB
**Fokus:** Bug Hunting & Edge Cases (Siklus 5, Putaran 2)
**Siklus:** 5/5

---

## Ringkasan

Audit mendalam terhadap financial calculations, slice bounds, division safety, dan goroutine lifecycle. Fokus pada area yang belum di-cover oleh existing bug tasks (TASK-020–024, 045–049, 071–074, 095–099, 116–117).

---

## Temuan Bug Baru

### 1. HIGH: OBV Panic — series[0] Access Without Final Bounds Check
- **File:** `internal/service/ta/indicators.go:818`
- **Issue:** `CalcOBV()` returns `series[0]` at line 818 tanpa final bounds check. Meskipun `CalcOBVSeries()` SEHARUSNYA return non-empty slice, edge case saat `len(bars) < 2` bisa menyebabkan empty series yang lolos ke sini.
- **Impact:** Panic: index out of range
- **Berbeda dari TASK-024:** TASK-024 focus pada panic guard di OBV series computation, ini focus pada return value access.

### 2. MEDIUM: Volatility avgATR Division by Zero
- **File:** `internal/service/price/volatility.go:70`
- **Issue:** `ratio := currentATR / avgATR` tanpa explicit guard untuk `avgATR == 0`. Guard di line 66 check `avgATR <= 0` tapi logic flow memungkinkan zero avgATR lolos jika semua bar punya identical Close (rare tapi possible di low-liquidity pairs).
- **Impact:** Produces `Inf` atau `NaN` yang propagate ke downstream calculations

### 3. MEDIUM: Walk-Forward Backtest Empty Slice Risk
- **File:** `internal/service/backtest/walkforward.go:90-91`
- **Issue:** Code accesses `evaluated[0]` dan `evaluated[len(evaluated)-1]` setelah early return di line 74. Meskipun safe saat ini, filter operations antara line 74 dan 90 bisa reduce slice ke empty di edge cases.
- **Also:** `walkforward_multi.go` dan `walkforward_optimizer.go` punya pattern serupa.
- **Impact:** Panic: index out of range

### 4. MEDIUM: Formatter Quant Fragile Slice Boundary
- **File:** `internal/adapter/telegram/formatter_quant.go:146`
- **Issue:** `len(fxCurrencies) >= 4` lalu `topFX := fxCurrencies[:4]` — safe sekarang tapi fragile. Refactoring atau concurrent mutation bisa break. Defensive coding seharusnya pakai `min(4, len(fxCurrencies))`.
- **Impact:** Low risk sekarang, tapi time bomb untuk future changes.

### 5. LOW: Timezone Handling Mixed in impact_bootstrap.go
- **File:** `internal/service/news/impact_bootstrap.go:117, 422-423`
- **Issue:** Mix `time.Now().In(wibLocation)` dengan `.UTC().Format()` — correct tapi confusing. Jika server timezone berubah, behavior bisa unexpected.
- **Impact:** Edge case saat server tidak di UTC.

---

## Filtering: Overlap dengan Existing Tasks

| Temuan | Status | Note |
|--------|--------|------|
| Impact recorder detached ctx | SUDAH ADA: TASK-098 | Skip |
| OBV panic guard | PARTIALLY: TASK-024 | Buat task baru untuk RETURN value bounds |
| Division by zero | BARU | Buat task |
| Walk-forward empty slice | BARU | Buat task |
| Fragile slice boundary | BARU | Buat task |
| Timezone handling | BARU tapi LOW | Gabung ke general hardening task |

---

## Task Recommendations (5 Non-Duplicate)

1. **TASK-120**: OBV return value bounds guard di indicators.go [HIGH]
2. **TASK-121**: Volatility avgATR zero-division guard [MEDIUM]
3. **TASK-122**: Walk-forward backtest empty slice safety [MEDIUM]
4. **TASK-123**: Defensive slice bounds di formatter_quant.go [LOW]
5. **TASK-124**: Financial calculation NaN/Inf propagation guard [MEDIUM]
