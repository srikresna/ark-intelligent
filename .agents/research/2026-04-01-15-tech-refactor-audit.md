# Research Report — Technical Refactor Audit (Siklus 4)

**Tanggal:** 2026-04-01 15:30 WIB
**Siklus:** 4/5 — Technical Refactor & Tech Debt
**Referensi:** .agents/TECH_REFACTOR_PLAN.md

---

## Ringkasan Eksekutif

Audit mendalam terhadap 15 TECH item di TECH_REFACTOR_PLAN.md. 10 item sudah memiliki task coverage di pending/. Ditemukan **5 gap nyata** yang belum di-cover oleh task manapun — ini yang menjadi fokus task creation siklus ini.

---

## Status Coverage TECH Items

| Item | Judul | Task Ada? |
|------|-------|-----------|
| TECH-001 | Split formatter.go | TASK-015 ✅ |
| TECH-002 | Split handler.go | TASK-016 ✅ |
| TECH-003 | handler_cta.go cleanup | TASK-040 ✅ |
| TECH-004 | bot.go wiring split | TASK-041 ✅ |
| TECH-005 | Unified scheduler | TASK-042 ✅ |
| TECH-006 | Magic numbers/constants | TASK-018 ✅ |
| TECH-007 | Error handling consistency | TASK-043 ✅ |
| TECH-008 | Context propagation | TASK-020 (partial) ⚠️ |
| TECH-009 | Test coverage | TASK-065 (partial) ⚠️ |
| TECH-010 | fmtutil expansion | TASK-019 ✅ |
| TECH-011 | CFTC contract constants | TASK-017 ✅ |
| TECH-012 | DI Framework (wire/fx) | ❌ **TIDAK ADA** |
| TECH-013 | Structured logging | TASK-068 (partial) ⚠️ |
| TECH-014 | Config validation | Sudah ada `validate()` ✅ |
| TECH-015 | Metrics & Observability | ❌ **TIDAK ADA** |

---

## Gap Analisis

### Gap 1: fmt.Printf masih tersebar di service layer (TECH-007/013 overlap)
Meski TASK-043 cover error handling dan TASK-068 cover component logger,
tidak ada task yang spesifik migrate `fmt.Printf` ke zerolog di:
- `internal/service/fred/cache.go` — 4 fmt.Printf untuk cache hit/miss
- `internal/service/price/correlation.go` — 8 fmt.Printf untuk correlation diagnostics
- `internal/service/backtest/factor_decomposition.go` — 1 fmt.Printf WARNING

Total: 13 `fmt.Printf` di service layer yang seharusnya pakai zerolog.

### Gap 2: formatter.go ZERO test coverage (TECH-009 gap)
TASK-065 cover sentiment/ai service tests.
Tetapi `internal/adapter/telegram/formatter.go` (4,489 LOC, 50+ fungsi)
sama sekali tidak punya test file. TECH_REFACTOR_PLAN bilang format layer harus
80% coverage (pure formatting, easy to test).

Fungsi prioritas untuk di-test:
- `FormatCOTOverview`, `FormatCOTDetail` — output kompleks, rawan regresi
- `FormatMacroRegime`, `FormatFREDContext` — paling sering diubah
- `FormatSentiment` — 185 LOC, banyak edge case

### Gap 3: Test coverage microstructure & strategy service (TECH-009 gap)
`internal/service/microstructure/engine.go` (262 LOC) — ZERO tests
`internal/service/strategy/engine.go` (283 LOC) + `types.go` (158 LOC) — ZERO tests
Kedua service ini merupakan orchestration layer (multi-signal → trade decision)
yang paling butuh test coverage.

### Gap 4: TECH-012 — DI Framework belum ada task
bot.go memiliki 1,289 LOC wiring manual. TECH plan menyebut `google/wire` atau
`uber-go/fx` sebagai solusi. Ini LOW priority tapi tidak ada task sama sekali.
Perlu research lebih lanjut untuk evaluasi trade-off sebelum commitment.

### Gap 5: TECH-015 — Metrics/Observability sama sekali tidak ada task
Tidak ada visibility sama sekali ke performa production:
- Tidak tahu command mana yang slow (latency per command)
- Tidak tahu error rate per service
- Tidak tahu API call count (rate limit risk tidak terdeteksi)
- Tidak tahu cache hit rate (FRED, COT, price)

Implementasi ringan: log-based metrics dulu sebelum full Prometheus.

---

## Temuan Tambahan

### Code Health Snapshot
```
formatter.go:   4,489 LOC, 50+ exported funcs, ZERO tests — HIGH RISK
handler.go:     2,381 LOC — split sudah di-task tapi belum dikerjakan
scheduler.go:   1,112 LOC + news/scheduler.go 1,099 LOC — duplikasi aktif
fmt.Printf:     13 instances di service layer (harus zerolog)
```

### Prioritas Eksekusi Phase 1 (tidak breaking)
Dari TECH_REFACTOR_PLAN fase 1, yang belum ada tasknya:
- TECH-015 metrics log-based → quick win, tidak breaking

### Prioritas Eksekusi Phase 4 (quality)
- formatter.go tests → HIGH impact, pure functions mudah di-test
- microstructure + strategy tests → critical path coverage

---

## Task Dibuat

- TASK-090: fmt.Printf Migration ke Zerolog di Service Layer
- TASK-091: Test Coverage — formatter.go Core Functions  
- TASK-092: Test Coverage — Microstructure & Strategy Engine
- TASK-093: Log-Based Metrics untuk Command Latency & Error Rate (TECH-015 minimal)
- TASK-094: DI Framework Evaluation — Wire vs Fx vs Manual (TECH-012 research)

