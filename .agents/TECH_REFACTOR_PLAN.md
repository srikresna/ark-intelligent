# Technical Refactor Plan — ark-intelligent

> Dokumen ini adalah panduan teknis untuk Research Agent dan Dev-A
> dalam merencanakan refactor, penghapusan tech debt, dan peningkatan kualitas kode.

---

## 📊 Code Metrics (Current State — Updated Loop #128)

```
Total LOC: ~60,925 lines Go

main.go progression (DI Framework — TASK-094 series):
  Before TASK-094:     683 LOC
  After TASK-094-C3:   337 LOC  ✅ COMPLETE
  After TASK-094-D:    337 LOC  ✅ COMPLETE
  Target TASK-094-Cleanup: <200 LOC  🔄 IN PROGRESS (Dev-A assigned)

Largest files (god classes — perlu dipecah):
  formatter.go         4,489 LOC  ← CRITICAL
  handler.go           2,381 LOC  ← CRITICAL  
  handler_cta.go       1,618 LOC  ← HIGH
  bot.go               1,289 LOC  ← HIGH
  keyboard.go          1,170 LOC  ← MEDIUM
  scheduler.go         1,112 LOC  ← MEDIUM
  news/scheduler.go    1,099 LOC  ← MEDIUM
  indicators.go        1,007 LOC  ← MEDIUM
```

Build status: ✅ Clean (go build ./... sukses)

---

## ✅ COMPLETED — DI Framework (TASK-094 Series)

### TASK-094-C3: Extract wire_telegram.go and wire_schedulers.go ✅
**Status:** MERGED to main (commit f3ff0de)
**Changes:**
- Created `wire_telegram.go` (208 LOC) — Telegram bot + handler wiring
- Created `wire_schedulers.go` (151 LOC) — scheduler + news scheduler wiring
- Reduced main.go: 683 → 337 LOC (51% reduction)

### TASK-094-D: HandlerDeps Struct ✅
**Status:** MERGED to main (commit f3ff0de)
**Changes:**
- Added HandlerDeps struct (17 params → 1 struct)
- Clean handler initialization

### TASK-094-Cleanup: main.go <200 LOC 🔄
**Status:** ASSIGNED to Dev-A
**Target:** Reduce main.go from 337 → <200 LOC
**Approach:** Extract remaining initialization to wire_services.go

---

## 🔴 CRITICAL — Harus Refactor

### TECH-001: formatter.go (4,489 LOC) — God Class
**Problem:** Satu file menangani formatting untuk COT, Calendar, FRED, Price, Backtest, Sentiment, AI, dll.
**Risk:** Setiap feature baru makin memperbesar file ini. Merge conflict sangat sering.

**Solution: Split per domain**
```
internal/adapter/telegram/format/
├── cot.go          ← FormatCOT*, FormatBias*
├── calendar.go     ← FormatCalendar*
├── macro.go        ← FormatMacro*, FormatFRED*
├── price.go        ← FormatPrice*, FormatLevel*
├── backtest.go     ← FormatBacktest*
├── sentiment.go    ← FormatSentiment*
├── ai.go           ← FormatOutlook*, FormatChat*
├── signal.go       ← FormatSignal*, FormatAlert*
└── common.go       ← shared helpers (directionArrow, numberFormat, dll)
```

### TECH-002: handler.go (2,381 LOC) — God Handler
**Problem:** Satu file routing semua command + callback. Impossible to navigate.
**Risk:** Setiap dev agent yang mau tambah command harus buka file yang sama → conflict.

**Solution: Split per feature domain**
```
internal/adapter/telegram/handler/
├── cot.go          ← /cot, /bias, /rank, /rankx
├── calendar.go     ← /calendar
├── macro.go        ← /macro, /transition
├── price.go        ← /price, /levels, /seasonal
├── analysis.go     ← /cta, /ctabt, /quant, /vp
├── backtest.go     ← /backtest, /accuracy
├── alpha.go        ← /alpha, /xfactors, /heat, /playbook
├── ai.go           ← /outlook, chat handling
├── admin.go        ← /ban, /unban, /users, /setrole, /membership
├── settings.go     ← /settings, /prefs
└── core.go         ← /start, /help, /status, routing registry
```

---

## 🟠 HIGH — Perlu Refactor

### TECH-003: handler_cta.go (1,618 LOC) — Mixed Concerns
**Problem:** Handler CTA mengandung logic business (indicator parsing, formatting) seharusnya di service layer.
**Solution:**
- Pindahkan formatting ke `format/cta.go`
- Handler hanya orchestrate: ambil data → format → kirim
- Target: handler CTA < 300 LOC

### TECH-004: bot.go (1,289 LOC) — DI Container + Wiring
**Problem:** bot.go terlalu besar karena wiring semua dependency sekaligus.
**Solution:**
```go
// Pisah menjadi:
internal/adapter/telegram/
├── bot.go          ← hanya Telegram polling loop + dispatch
├── wiring.go       ← dependency injection & initialization
└── registry.go     ← command registration
```

### TECH-005: Dual Scheduler Problem
**Problem:** Ada dua scheduler:
- `internal/scheduler/scheduler.go` (1,112 LOC)
- `internal/service/news/scheduler.go` (1,099 LOC)

Ini redundan dan confusing. Background job logic tersebar di dua tempat.

**Solution:**
- Semua job masuk ke satu `internal/scheduler/`
- News scheduler dipecah jadi jobs: `news_fetch.go`, `impact_record.go`, `surprise.go`
- Gunakan interface `Job` yang sama

### TECH-006: Magic Numbers & Strings
**Problem:** Tersebar di seluruh codebase:
```go
// Examples yang perlu difix:
if sig.Strength >= 4 { ... }          // magic number
"149.154.166.110:443"                 // hardcoded Telegram IP
time.Sleep(50 * time.Millisecond)     // magic duration
```
**Solution:** Pindahkan ke `internal/config/constants.go`

---

## 🟡 MEDIUM — Tech Debt

### TECH-007: Error Handling Tidak Konsisten
**Problem:** Mix antara:
```go
log.Error().Err(err).Msg("failed")   // zerolog
fmt.Errorf("wrap: %w", err)          // stdlib
return nil, err                       // bare return
```
**Solution:** Buat error handling policy di `pkg/errs/`:
```go
// Sentinel errors
var ErrNoData = errors.New("no data available")
var ErrRateLimited = errors.New("rate limited")

// Wrap dengan context
return errs.Wrap(err, "cot fetch")
```

### TECH-008: Context Propagation Tidak Konsisten
**Problem:** Beberapa fungsi tidak terima `ctx context.Context` padahal melakukan I/O.
Beberapa fungsi terima ctx tapi tidak pass ke downstream.
**Solution:** Audit semua fungsi dengan I/O → pastikan ctx selalu propagated.

### TECH-009: Test Coverage Rendah
**Problem:** Hanya `internal/service/ta/` dan beberapa service yang punya test.
Critical paths seperti COT analysis, FRED regime detection, signal detection tidak punya test.
**Solution:** Target coverage per area:
- `service/cot/` → minimal 60% coverage
- `service/fred/` → minimal 60% coverage
- `service/price/` → minimal 40% coverage (data-dependent, harder to test)
- `adapter/telegram/format/` → minimal 80% (pure formatting, easy to test)

### TECH-010: Duplicate Code di Formatters
**Problem:** Pattern `strings.Builder`, header formatting, number formatting berulang di banyak tempat.
**Solution:** Buat `pkg/fmtutil/` yang lebih lengkap:
```go
// Sudah ada fmtutil, tapi perlu diperluas:
func FormatLargeNumber(n float64) string   // 1,234,567
func FormatPercent(f float64, dp int) string  // 67.3%
func FormatPips(f float64) string
func MessageHeader(title, emoji string) string
func Divider() string
```

### TECH-011: Hardcoded Contract Codes
**Problem:** Contract codes CFTC (seperti "099741" untuk EUR) hardcoded di multiple files.
**Solution:** Centralize di `internal/domain/contracts.go` sebagai constants.

---

## 🟢 LOW — Nice to Have

### TECH-012: Dependency Injection Framework ✅ COMPLETE (Manual)
**Status:** IMPLEMENTED — Manual restructuring (ADR-012 Option C)
**Decision:** Tidak menggunakan `google/wire` atau `uber-go/fx`
**Reasoning:** Overhead framework tidak justified untuk codebase ini. Manual wiring lebih predictable.

**Implementation:**
- `wire_telegram.go` — Telegram-specific wiring
- `wire_schedulers.go` — Scheduler wiring
- `wire_services.go` — Service layer DI
- `HandlerDeps` struct — Handler dependencies

**Result:** main.go reduced 683 → 337 LOC, further cleanup to <200 LOC in progress.

### TECH-013: Structured Logging Improvement
**Problem:** Log messages tidak konsistent. Beberapa tidak ada context (currency, user ID).
**Solution:** Standardize log fields:
```go
log.Info().
    Str("contract", code).
    Str("user", userID).
    Dur("latency", elapsed).
    Msg("cot analysis complete")
```

### TECH-014: Configuration Validation
**Problem:** App start dengan config invalid (missing keys, wrong values) tanpa error yang jelas.
**Solution:** Tambah `config.Validate()` di startup yang check semua required fields.

### TECH-015: Metrics & Observability
**Problem:** Tidak ada metrics untuk monitor performa production.
**Solution:** Tambah Prometheus metrics (atau minimal log-based metrics):
- Command latency per command
- Error rate per service
- API call count per provider
- Cache hit/miss rate

---

## ✅ COMPLETED — DI Framework Phase (TASK-094)

```
✅ TASK-094-C3: wire_telegram.go + wire_schedulers.go — main.go 683→337 LOC
✅ TASK-094-D: HandlerDeps struct — handler initialization cleaned
🔄 TASK-094-Cleanup: main.go <200 LOC — Dev-A assigned
⏳ TASK-094-Docs: Update TECH_REFACTOR_PLAN.md — TechLead assigned (this doc)
```

---

## 📋 Refactor Execution Order

Urutan yang direkomendasikan untuk Dev agents:

```
Phase 1 (Foundation — tidak breaking):
  TECH-006: Constants → tidak breaking, quick win
  TECH-010: fmtutil expansion → tidak breaking
  TECH-011: Contract code constants → tidak breaking
  TECH-014: Config validation → tidak breaking

Phase 2 (Split big files — high impact):
  TECH-001: Split formatter.go → HIGH PRIORITY
  TECH-002: Split handler.go → HIGH PRIORITY
  (Lakukan bersamaan di branch berbeda: formatter di dev-b, handler di dev-c)

Phase 3 (Architecture):
  TECH-003: handler_cta.go cleanup
  TECH-004: bot.go split
  TECH-005: Unify schedulers

Phase 4 (Quality):
  TECH-007: Error handling
  TECH-008: Context propagation
  TECH-009: Test coverage

Phase 5 (Enhancement):
  TECH-012: DI framework
  TECH-013: Logging
  TECH-015: Metrics
```

---

## ⚠️ Aturan Refactor untuk Dev Agents

1. **Satu PR = satu refactor item** — jangan gabung TECH-001 dan TECH-002 dalam satu PR
2. **Refactor = NO behavior change** — kalau ada behavior change, itu bukan refactor, itu feature
3. **Wajib build clean** sebelum PR: `go build ./... && go vet ./...`
4. **Wajib test existing** tidak ada yang break: `go test ./...`
5. **Kalau ragu** — buat task di `pending/` dengan tag `[NEEDS DISCUSSION]` dan notif Dev-A

---

## 📜 Update Log

### Loop #128 — TechLead-Intel Update
**Date:** 2026-04-03  
**Updated by:** TechLead-Intel

**Changes:**
1. ✅ Documented completed DI Framework work (TASK-094-C3, TASK-094-D)
2. ✅ Updated main.go metrics: 683→337→target <200 LOC
3. ✅ Marked TECH-012 (DI Framework) as complete — using manual restructuring
4. ✅ Added completed phase section for TASK-094 series
5. 🔄 Noted TASK-094-Cleanup in progress (Dev-A assigned)

**Context:**
All 5 PRs (#346-#350) successfully merged to main via cherry-pick strategy.
Sprint now in P2 phase (DI Framework Completion).
