# TASK-068: Structured Log — Component Fields Standardization (TECH-013)

**Priority:** LOW  
**Type:** Tech Refactor / Observability  
**Ref:** TECH-013 in TECH_REFACTOR_PLAN.md  
**Branch target:** dev-b atau dev-c  
**Estimated size:** Small (30-50 LOC change)

---

## Problem

`pkg/logger` sudah ada `Component()` helper untuk per-service logger, tapi banyak service di `internal/service/` tidak menggunakannya.

Contoh inconsistency:
```go
// sentiment/sentiment.go — tidak pakai component logger
log.Error().Err(err).Msg("failed to fetch CNN fear greed")

// fred/regime_history.go — pakai component logger dengan context ✅
log.Error().Str("series", seriesID).Err(err).Msg("failed to build request")
```

Akibatnya: log dari sentiment tidak bisa di-filter per component di production.

---

## Scope (small, targeted)

Hanya 3 file yang paling sering diakses dan paling minim context:

1. `internal/service/sentiment/sentiment.go`
   - Tambah `var sentimentLog = logger.Component("sentiment")` di package level
   - Ganti semua `log.Error()` dan `log.Warn()` dengan `sentimentLog.Error()` dll

2. `internal/service/cot/analyzer.go`
   - Tambah `var analyzerLog = logger.Component("cot-analyzer")`
   - Untuk setiap log call yang tidak punya context field, tambah minimal satu identifier (currency/contract)

3. `internal/adapter/telegram/middleware.go`
   - Sudah ada logger, tapi cek apakah userID disertakan di error logs

---

## Pattern Target

```go
// package-level component logger
var log = logger.Component("sentiment")  // reuse existing var name

// log call dengan context
log.Error().
    Str("source", "cnn").
    Err(err).
    Msg("fear-greed fetch failed")
```

---

## Acceptance Criteria

- [ ] `sentiment.go` menggunakan component logger
- [ ] Minimal 1 context field (source, currency, series, dll) di setiap Error/Warn call di 3 file target
- [ ] `go build ./...` clean

---

## Notes

- Lihat `internal/service/fred/regime_history.go` sebagai contoh pattern logging yang baik
- Ini adalah perubahan kecil — JANGAN refactor structure, hanya update log calls
- Jika `var log = ...` sudah ada di file, update isinya saja
