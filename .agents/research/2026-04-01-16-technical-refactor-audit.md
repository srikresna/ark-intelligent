# Research: Technical Refactor Audit — Siklus 4
**Date:** 2026-04-01 16:xx WIB  
**Focus:** Tech debt deep-dive — TECH-003 s/d TECH-009 + temuan baru

---

## Ringkasan Kondisi Codebase

```
Total LOC: ~72,754 (naik dari 60,925 — ada penambahan fitur siklus 3)
Build status: belum diverifikasi (go not found di agent sandbox)
```

## Temuan Per Area

### 1. handler_cta.go — 1,618 LOC (TECH-003, belum ditask)
- File mencampur: command dispatch, business logic (indicator parsing), formatting
- Ada `ctaStateCache` struct (60 LOC) yang bisa jadi independent package
- Target: pisah menjadi `handler/cta.go` (handler murni <300 LOC) + logic ke service layer

### 2. bot.go — 1,289 LOC (TECH-004, belum ditask)
- Menggunakan `map[string]interface{}` untuk **semua** Telegram API params (15 lokasi)
- Dependency injection + polling loop + API wrapper semua dalam satu file
- `apiCall(ctx, method, map[string]interface{}, interface{})` — type-unsafe

### 3. Dual Scheduler (TECH-005, belum ditask)
- `internal/scheduler/scheduler.go` — 1,112 LOC (general jobs)
- `internal/service/news/scheduler.go` — 1,099 LOC (news-specific jobs)
- Pattern `recover()` diulang 5x di news/scheduler.go — boilerplate perlu di-DRY
- `time.Sleep(50ms)` tersebar di 8 lokasi berbeda untuk Telegram flood control

### 4. Error Handling (TECH-007, belum ditask)
- Mix `log.Error().Err(err).Msg()`, `fmt.Errorf()`, dan bare `return nil, err`
- Tidak ada sentinel error package
- `pkg/errs/` belum ada — perlu dibuat

### 5. interface{} modernization — TEMUAN BARU
- Go 1.22 digunakan, tapi 32 lokasi masih pakai `interface{}` bukan `any`
- bot.go: `map[string]interface{}` untuk Telegram params — bisa diganti struct typed
- Modernisasi ke `any` adalah quick win, zero behavior change

### 6. handler_alpha.go — 1,112 LOC — TEMUAN BARU
- Format functions tersembunyi di handler file (`formatAlphaSummary`, `buildReasonIndonesian`, `formatPlaybook`, dll)
- Seharusnya pindah ke `formatter.go` domain format layer
- Ini tidak tercatat di TECH_REFACTOR_PLAN.md — gap!

### 7. Context Propagation (TECH-008, partial)
- 390 dari 1540 fungsi menerima `ctx` = 25.3% coverage
- HTTP calls via `http.Get`/`http.Post` sudah pakai `NewRequestWithContext` — bagus
- `time.Sleep` tanpa `ctx.Done()` check = tidak cancellable (15 lokasi)

### 8. Test Coverage Gap (TECH-009)
- `internal/service/cot/` — ada test file tapi coverage belum diukur
- `internal/service/fred/` — belum ada test
- `internal/adapter/telegram/format/` — belum ada test (formatter functions pure)

---

## Tasks Yang Sudah Ada (Hindari Duplikasi)
- TASK-015: split-formatter-per-domain ← TECH-001 ✅
- TASK-016: split-handler-per-domain ← TECH-002 ✅
- TASK-017: cftc-contract-constants ← TECH-011 ✅
- TASK-018: magic-numbers-constants ← TECH-006 ✅
- TASK-019: expand-fmtutil-helpers ← TECH-010 ✅

## Tasks Baru Yang Perlu Dibuat
- TASK-040: handler-cta-refactor ← TECH-003
- TASK-041: bot-wiring-split ← TECH-004
- TASK-042: unified-scheduler-refactor ← TECH-005
- TASK-043: error-sentinel-package ← TECH-007
- TASK-044: interface-any-modernization ← NEW FINDING (Go 1.22)

