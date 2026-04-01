# TASK-090: Migrate fmt.Printf ke Zerolog di Service Layer (TECH-007/013)

**Priority:** MEDIUM
**Type:** Tech Refactor / Logging
**Ref:** TECH-007 + TECH-013 in TECH_REFACTOR_PLAN.md
**Branch target:** dev-b atau dev-c
**Estimated size:** Small (S) — ~50 LOC change, tapi butuh test run
**Created by:** Research Agent
**Created at:** 2026-04-01 15:30 WIB
**Siklus:** 4 — Technical Refactor

---

## Problem

13 `fmt.Printf` di service layer yang seharusnya menggunakan structured zerolog.
Ini menyebabkan log production tidak bisa di-filter, tidak punya level (Debug/Info/Warn),
dan tidak ada correlation ID/component context.

```go
// SEKARANG — tidak structured, tidak bisa di-filter:
fmt.Printf("[fred-cache] returning cached data (age: %s)\n", age.Round(time.Second))
fmt.Printf("[correlation] SKIP %s: no price mapping found\n", cur)
fmt.Printf("WARNING: simpleOLS produced negative R²...\n", r2)

// SEHARUSNYA:
log.Debug().Dur("age", age).Msg("returning cached data")
log.Warn().Str("currency", cur).Msg("skip: no price mapping found")
log.Warn().Float64("r2", r2).Msg("simpleOLS produced negative R², clamping to 0")
```

---

## File yang Perlu Diubah

### 1. `internal/service/fred/cache.go`
- Line 60: cache hit log (Debug level)
- Line 75: cache miss/fresh fetch log (Debug level)
- Line 91: cache hit log (Debug level)
- Line 105: cache miss/fresh fetch log (Debug level)

**Action:** Tambahkan `var log = logger.Component("fred-cache")` di package level,
replace 4 fmt.Printf dengan zerolog Debug calls.

### 2. `internal/service/price/correlation.go`
- Line 38: skip no price mapping (Warn level)
- Line 51: skip fetch error (Warn level + Err field)
- Line 56: skip insufficient records (Warn level + Int fields)
- Line 79: skip insufficient returns (Warn level + Int fields)
- Line 85: summary log (Info level)
- Line 129: fallback log (Warn level)
- Line 136: fallback success (Info level)
- Line 219: diagnosis msg (Info level)

**Action:** Tambahkan `var log = logger.Component("price-correlation")`,
replace semua fmt.Printf dengan zerolog calls yang tepat level-nya.

### 3. `internal/service/backtest/factor_decomposition.go`
- Line 500: WARNING negative R² (Warn level)

**Action:** Replace fmt.Printf dengan `log.Warn().Float64("r2", r2).Msg("...")`.

---

## Acceptance Criteria

- [ ] `go build ./...` clean setelah perubahan
- [ ] `go vet ./...` clean
- [ ] `go test ./...` semua test lama tetap pass
- [ ] Tidak ada `fmt.Printf` yang tersisa di ketiga file tersebut
- [ ] Setiap zerolog call memakai level yang tepat (Debug/Info/Warn/Error)
- [ ] Field names konsisten dengan existing zerolog calls di codebase (snake_case)

---

## Catatan

- Jangan ubah behavior: ini pure logging replacement, tidak ada logic change
- Gunakan `pkg/logger` `Component()` helper yang sudah ada
- Cek `internal/service/fred/regime_history.go` sebagai contoh best practice zerolog usage
