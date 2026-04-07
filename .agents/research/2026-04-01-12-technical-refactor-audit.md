# Research Report — Siklus 4: Technical Refactor & Tech Debt
**Date:** 2026-04-01 12:00 WIB
**Focus:** TECH_REFACTOR_PLAN analysis, prioritas execution order

---

## Executive Summary

Codebase saat ini memiliki 5 "god files" yang masing-masing >1,000 LOC dan berisiko tinggi menyebabkan merge conflict antar dev agents. Selain itu ditemukan 33+ hardcoded CFTC contract codes di non-test files, magic numbers untuk signal strength thresholds, dan magic duration values untuk rate limiting. Prioritas tertinggi adalah Phase 1 (foundation fixes yang non-breaking) sebelum Phase 2 (file splitting).

---

## Temuan Utama

### 1. God Files Status (Konfirmasi)

| File | LOC | Status | Priority |
|------|-----|--------|----------|
| formatter.go | 4,489 | CRITICAL — semua domain dalam satu file | P1 |
| handler.go | 2,381 | CRITICAL — semua command routes di satu file | P1 |
| handler_cta.go | 1,618 | HIGH — business logic bercampur handler | P2 |
| bot.go | 1,289 | HIGH — wiring + dispatch bercampur | P2 |
| scheduler.go | 1,112 | MEDIUM | P3 |
| news/scheduler.go | 1,099 | MEDIUM | P3 |

Dev agents yang simultaneous menyentuh handler.go atau formatter.go **pasti conflict**.
Split harus dilakukan sebelum Dev-B dan Dev-C mulai banyak PR.

### 2. Hardcoded CFTC Contract Codes — 33 occurrences di non-test files

Ditemukan di:
- `internal/adapter/telegram/handler.go:1103` — map EUR→"099741" hardcoded langsung
- `internal/adapter/telegram/formatter.go:266` — array dari 8 contract codes
- `internal/adapter/telegram/formatter.go:1008-1015` — reverse mapping
- `internal/adapter/telegram/keyboard.go:228-235` — duplikasi mapping

Ini berarti jika CFTC ganti kode kontrak, perlu edit 4+ lokasi berbeda.
**Fix:** Centralize ke `internal/domain/contracts.go` sebagai constants.

### 3. Magic Numbers — Signal Strength Threshold

```go
// Di 2 tempat berbeda, nilai sama "4" tidak punya nama:
internal/scheduler/scheduler.go:392  → if sig.Strength >= 4
internal/service/backtest/stats.go:152 → if s.Strength >= 4
```

Jika threshold perlu diubah (misalnya jadi 3 atau 5), harus cari manual di codebase.
**Fix:** `const MinAlertStrength = 4` di `internal/config/constants.go`

### 4. Magic Duration Values — Rate Limiter Sleeps

```go
// Tersebar di 15+ lokasi, contoh:
time.Sleep(50 * time.Millisecond)   // Telegram flood control — muncul 6x
time.Sleep(200 * time.Millisecond)  // COT fetcher rate limit
time.Sleep(300 * time.Millisecond)  // Price fetcher rate limit
```

**Fix:** Named constants di `internal/config/constants.go`:
```go
const (
    TelegramFloodDelay    = 50 * time.Millisecond
    COTFetcherDelay       = 200 * time.Millisecond
    PriceFetcherDelay     = 300 * time.Millisecond
)
```

### 5. pkg/fmtutil — Sudah Ada, Tapi Perlu Ekspansi

File `pkg/fmtutil/format.go` (225 LOC) sudah punya:
- `FmtNum`, `FmtNumSigned`, `FmtPct`

**Yang masih missing** (ditemukan pattern inline di formatter.go):
- `FormatPips(f float64) string` — dibutuhkan di price formatting
- `MessageHeader(title, emoji string) string` — pattern berulang di setiap section
- `Divider() string` — `strings.Repeat("─", 28)` muncul >20x di formatter.go
- `FormatLargeNumber(n float64) string` — untuk COT net positioning numbers

### 6. Config Validation — Sudah Ada, Cukup Baik

`internal/config/config.go` sudah punya `validate()` dengan pengecekan:
- COT_HISTORY_WEEKS >= 4
- COT_FETCH_INTERVAL >= 1m

Tidak perlu task baru untuk ini. **TECH-014 bisa di-skip.**

### 7. Test Coverage — Better Than Expected

Test files exist di semua major service packages:
- `internal/service/cot/` ✅
- `internal/service/fred/` ✅
- `internal/service/price/` ✅
- `internal/service/backtest/` ✅
- `internal/service/news/` ✅
- `internal/service/ta/` ✅
- `internal/adapter/telegram/format/` ❌ MISSING — formatter logic tidak ada test

---

## Analisis Execution Order

Rekomendasi urutan berdasarkan:
1. **Phase 1 dulu** (non-breaking) → bisa dikerjakan Dev-B atau Dev-C tanpa conflict
2. **Phase 2 kemudian** (god file split) → butuh koordinasi agar tidak overlap

### Phase 1 Quick Wins (Non-Breaking):
1. `contracts.go` — pindahkan hardcoded codes (TASK-017)
2. `constants.go` — magic numbers dan durations (TASK-018)
3. `fmtutil` expansion — helper methods (TASK-019)

### Phase 2 High Impact (Perlu koordinasi):
1. Split `formatter.go` → dev-b (TASK-015)
2. Split `handler.go` → dev-c (TASK-016)
→ Harus di branch berbeda, tidak boleh concurrent

---

## Tasks Direkomendasikan

| ID | Nama | Priority | Effort |
|----|------|----------|--------|
| TASK-015 | Split formatter.go per domain | HIGH | L |
| TASK-016 | Split handler.go per domain | HIGH | L |
| TASK-017 | CFTC contract code constants | MEDIUM | S |
| TASK-018 | Magic numbers → named constants | MEDIUM | S |
| TASK-019 | Expand pkg/fmtutil helpers | LOW | S |
