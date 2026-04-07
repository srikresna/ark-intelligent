# Tech Refactor Audit — Siklus 4 Putaran 6
**Date:** 2026-04-02 08:00 WIB
**Cycle:** 4 — Technical Refactor (TECH_REFACTOR_PLAN.md)

---

## Temuan Utama

### ✅ Yang Sudah Selesai

| Item | Status | Bukti |
|------|--------|-------|
| TECH-004: bot.go split | ✅ DONE | bot.go turun dari 1,289 → 433 LOC; wiring.go ada |
| TECH-011: contract codes | ✅ DONE | internal/domain/contracts.go lengkap |
| TECH-014: config validation | ✅ DONE | config.go punya validate() |
| TECH-006: signal strength constant | ✅ DONE | constants.go: SignalStrengthAlert, ZScoreExtreme, cache TTLs, dll |

---

### 🔴 God Classes Masih Tumbuh (Masalah Serius)

**formatter.go: 4,489 → 4,539 LOC (+50 baris sejak plan ditulis)**
- `format/` subdirectory belum ada
- New formatter files (formatter_compact.go, formatter_gex.go, formatter_ict.go, formatter_quant.go, formatter_wyckoff.go) ditambahkan *di samping* formatter.go, bukan split dari dalamnya
- TASK-015 masih pending — perlu diprioritaskan

**handler.go: 2,381 → 2,909 LOC (+528 baris)**
- 15 handler_*.go files sudah ada (handler_alpha, handler_backtest, handler_cta, handler_gex, handler_ict, handler_levels, handler_price, handler_quant, handler_seasonal, handler_smc, handler_vp, handler_wyckoff, dll)
- Core handler.go masih berisi: admin commands, COT, calendar, macro, settings, membership, chat, history, bias, rank, impact — semua masih di satu file
- Khusus admin block: cmdBan + cmdUnban + cmdUsers + cmdSetRole + cmdMembership = ~280 LOC yang bisa diextract ke handler_admin.go
- TASK-016 masih pending

---

### 🟠 Gap yang Belum Ada Task

**TECH-007: pkg/errs — BELUM ADA**
- 718 pola error handling campuran di codebase
- Tidak ada `pkg/errs/` package sama sekali
- Masih mix antara `log.Error().Err()`, `fmt.Errorf()`, bare `return nil, err`

**TECH-008: Context Propagation — BELUM LENGKAP**
- 10 titik `context.Background()`/`context.TODO()` di production code
- Kritis: handler_cta.go:581, handler_quant.go:484, handler_vp.go:422
  → Ketiga handler ini create new context.Background() di tengah request handling padahal parent ctx sudah ada
- TASK-020 hanya cover kasus MQL5, bukan handler utama

**TECH-009: COT Analyzer Unit Tests — BELUM ADA**
- `internal/service/cot/analyzer.go` = 822 LOC, 17 functions
- Fungsi pure (tidak butuh DB/network): `computeCOTIndex`, `computeSentiment`, `classifySignal`, `classifySignalStrength`, `classifySmallSpec`, `detectDivergence`, `classifyMomentumDir`
- Tidak ada satu pun test file untuk analyzer.go
- Existing tests: category_zscore_test.go, confluence_score_test.go, recalibrated_detector_test.go, signals_test.go — tapi bukan untuk analyzer.go

**TECH-013: Structured Logging Handler Layer — BELUM ADA TASK**
- COT analyzer logging: 11 Str()/context fields ✅
- Handler layer: banyak log tanpa userID, currency, command context
- Request latency tidak di-log di handler level sama sekali

---

### 🟡 Yang Ada Tapi Belum Diprioritaskan

**TECH-006 (magic sleep durations) — constants.go ada tapi sleep masih hardcoded:**
```
internal/scheduler/scheduler.go:353,463,551   → time.Sleep(50 * time.Millisecond)
internal/service/news/scheduler.go:290,491,725 → time.Sleep(50 * time.Millisecond)
internal/service/price/fetcher.go:669,675      → time.Sleep(300 * time.Millisecond)
internal/service/cot/fetcher.go:174            → time.Sleep(200 * time.Millisecond)
```
TASK-018 masih pending.

**TECH-010 (fmtutil expansion) — fmtutil/format.go ada tapi masih kurang:**
- Ada: FmtNum, FmtNumSigned, FmtPct, FmtRatio, COTIndexBar, ConfluenceBar
- Belum ada: FormatPips, MessageHeader/Divider, FormatLargeNumber untuk big numbers
TASK-019 masih pending.

---

## Rekomendasi Prioritas Task Baru

Berdasarkan execution order dari TECH_REFACTOR_PLAN (Phase 3 → Phase 4):

1. **handler_admin.go extraction** — quick win, unblock TASK-016 progress, <200 LOC change
2. **pkg/errs foundation** — TECH-007 Phase 4 start, tidak breaking
3. **Context propagation fix in 3 handlers** — TECH-008 targeted, low risk
4. **COT analyzer unit tests** — TECH-009 highest value, pure functions = easy to test
5. **Handler command latency logging** — TECH-013 observability value

---

## File Reference

```
internal/adapter/telegram/formatter.go           4,539 LOC (growing)
internal/adapter/telegram/handler.go             2,909 LOC (growing)
internal/adapter/telegram/handler_cta.go         1,624 LOC
internal/adapter/telegram/keyboard.go            1,346 LOC
internal/scheduler/scheduler.go                  1,125 LOC
internal/service/cot/analyzer.go                   822 LOC
internal/adapter/telegram/wiring.go                 49 LOC
```
