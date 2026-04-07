# Research: Tech Refactor Plan — Test Coverage & Context Propagation Gaps
**Siklus:** 4/5 — Tech Refactor Plan
**Putaran:** 20
**Date:** 2026-04-02 03:00 WIB
**Agent:** Research Agent

---

## Ringkasan Eksekutif

Audit mendalam terhadap `TECH_REFACTOR_PLAN.md` untuk menemukan gap implementasi yang belum di-cover oleh task pending yang ada (289 task). Fokus pada TECH-008 (context propagation), TECH-009 (test coverage), dan TECH-010 (fmtutil).

---

## Temuan 1: TECH-009 — COT Service Test Coverage Gap

`internal/service/cot/` memiliki 10 Go files, 5 test files. File yang **belum punya test**:

| File | Fungsi Kritis | Kenapa Penting |
|------|--------------|----------------|
| `analyzer.go` | `Analyze()`, index percentile, bias scoring | Core business logic — hasil analisis COT |
| `regime.go` | `ClassifyRegime()`, threshold mapping | Menentukan "EXTREME_LONG" vs "NEUTRAL" dll |
| `index.go` | Index computation | Dipanggil oleh semua /cot output |
| `fetcher.go` | HTTP fetch + parse | Integration test sudah ada, tapi unit test tidak |
| `confluence.go` | Confluence scoring coordination | Sudah ada confluence_score_test.go tapi bukan untuk confluence.go |

**Gap terbesar**: `analyzer.go` + `regime.go` adalah critical path — setiap output /cot bergantung pada ini, tapi ZERO unit tests.

---

## Temuan 2: TECH-009 — FRED Service Test Coverage Gap

`internal/service/fred/` memiliki 10 Go files, hanya 3 test files:

| File | Test Ada? | Keterangan |
|------|-----------|------------|
| `composites.go` | ✅ composites_test.go | Covered |
| `regime.go` | ✅ regime_test.go | Covered |
| `composites_test.go` | ✅ audit_test.go | Covered |
| `rate_differential.go` | ❌ | FetchCarryRanking, ComputeCarryScores — untested |
| `regime_asset.go` | ❌ | PerformanceStats calculation — pure math, untested |
| `regime_history.go` | ❌ | Regime snapshot persistence — untested |
| `persistence.go` | ❌ | FRED data persistence — untested |
| `cache.go` | ❌ | TTL cache logic — untested |
| `regime_performance.go` | ❌ | Performance matrix computation — untested |
| `alerts.go` | ❌ | FRED alert threshold detection — CRITICAL, triggers user notif |

**Gap terbesar**: `alerts.go` triggers notifikasi ke user ketika FRED data berubah signifikan. Zero test coverage untuk logic critical ini.

`rate_differential.go` mengandung carry ranking logic (pure math) yang sangat mudah di-unit test tanpa external calls.

---

## Temuan 3: TECH-008 — Storage Repos Ignoring Context

Semua `internal/adapter/storage/` repos accept `context.Context` tapi menggunakan blank identifier `_`:

```go
// intraday_repo.go
func (r *IntradayRepo) GetHistory(_ context.Context, ...) { ... }
func (r *IntradayRepo) SaveBars(_ context.Context, ...) { ... }

// event_repo.go  
func (r *EventRepo) GetEventsByDateRange(_ context.Context, ...) { ... }
// + 8 fungsi lainnya

// price_repo.go
func (r *PriceRepo) GetHistory(_ context.Context, ...) { ... }
// + 4 fungsi lainnya

// signal_repo.go
func (r *SignalRepo) GetSignalsByContract(_ context.Context, ...) { ... }
```

**Total: 20+ fungsi** di 4 storage repos yang ignore context. Fix: gunakan `QueryContext`, `ExecContext`, `PrepareContext` dari `database/sql` agar DB queries bisa di-cancel.

Ini berbeda dari TASK-098 (impact-recorder) dan TASK-217 (handlers) — ini khusus storage repos.

---

## Temuan 4: Formatter.go Masih Tumbuh

Formatter.go sekarang **4,539 LOC** (dari 4,489 LOC di plan → +50 LOC). Sementara TASK-015 sudah di-pending tapi belum dikerjakan. Setiap feature baru masih ditambah ke sana.

TECH-001 split masih sangat diperlukan dan makin urgent. Estimasi jika tidak di-split: akan mencapai 5,000 LOC dalam 2-3 bulan.

---

## Temuan 5: TECH-009 — FRED rate_differential Pure Math Mudah di-Test

`rate_differential.go` mengandung:
- `ComputeCarryScores()` — mengambil map[string]float64 rates, return ranked list
- Differential calculation: semua pure math
- Sorting logic

Ini adalah kandidat **quick win** test coverage — tidak perlu mock HTTP, cukup inject rates langsung.

---

## Rekomendasi Task Baru (TASK-290 s/d TASK-294)

| Task | Area | TECH Ref | Priority |
|------|------|---------|----------|
| TASK-290 | COT analyzer.go + regime.go unit tests | TECH-009 | HIGH |
| TASK-291 | FRED rate_differential.go unit tests | TECH-009 | MEDIUM |
| TASK-292 | FRED regime_asset + regime_performance tests | TECH-009 | MEDIUM |
| TASK-293 | Storage repos QueryContext/ExecContext | TECH-008 | MEDIUM |
| TASK-294 | FRED alerts.go unit tests | TECH-009 | HIGH |
