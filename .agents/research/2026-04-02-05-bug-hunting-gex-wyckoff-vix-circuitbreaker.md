# Research Report: Bug Hunting Siklus 5 Putaran 3 — GEX Zero Spot, Wyckoff Phase -1, VIX Div/0, Circuit Breaker Race

**Tanggal:** 2026-04-02 05:00 WIB
**Fokus:** Bug Hunting & Edge Cases (Siklus 5, Putaran 3)
**Siklus:** 5/5

---

## Ringkasan

Bug hunting di fitur yang baru di-merge: GEX (TASK-012), Wyckoff (TASK-011), VIX term structure (TASK-061), circuit breaker (TASK-066). Ditemukan 5 bugs: GEX silent data corruption, expired options leak, Wyckoff invalid phase boundary, VIX division edge case, circuit breaker race condition.

---

## Bug Details

### 1. HIGH: GEX Calculator — Spot Price Zero = Silent Data Corruption
- **File:** `service/gex/calculator.go:40`
- **Issue:** `multiplier := contractSize * spot * spot` — jika spot=0 (API failure, no price), semua GEX values jadi 0
- **Tidak ada error return** — user lihat GEX profile dengan semua NetGEX=0, menyerupai data valid
- **Root cause:** Fallback chain dari UnderlyingPrice → MarkPrice, tapi jika keduanya missing, spot=0
- **Impact:** Data corruption — user buat trading decision berdasarkan false "flat gamma" signal

### 2. MEDIUM: Deribit Client — Expired Options Not Filtered in BookSummary
- **File:** `service/marketdata/deribit/client.go:93-96`
- **Issue:** `GetInstruments()` filter `expired=false`, tapi `GetBookSummary()` TIDAK filter
- **Impact:** Di expiry day, expired options masuk ke analysis, instrument map mismatch, incomplete gamma calc

### 3. MEDIUM: Wyckoff Phase — Invalid Boundary When arDistIdx = -1
- **File:** `service/wyckoff/phase.go:111-112`
- **Issue:** Jika AR_D event tidak ditemukan, `arDistIdx = -1`, maka `phA.End = -1`
- `eventsInRange(events, 0, -1)` include ALL events karena condition `e.BarIndex > -1` always true
- **Impact:** Phase A incorrectly contains semua events, corrupting phase classification

### 4. LOW: VIX SlopePct Division by Zero Edge Case
- **File:** `service/vix/fetcher.go:269`
- **Issue:** Guard `ts.M1 > 0` ada, tapi jika CSV data malformed dan M1 parsed as 0, SlopePct = NaN/Inf
- **Impact:** Corrupts contango/backwardation classification downstream

### 5. LOW: Circuit Breaker Negative Retry Duration + Race
- **File:** `pkg/circuitbreaker/breaker.go:82-84`
- **Issue:** Error message computes `resetTimeout - time.Since(lastFailure)` — bisa negative jika timeout sudah lewat
- **Impact:** User lihat "retry after -5ms" — confusing, tapi tidak breaking

---

## Task Recommendations

1. **TASK-145**: GEX spot price zero guard — return error instead of silent corruption [HIGH]
2. **TASK-146**: Deribit BookSummary expired filter — match GetInstruments behavior [MEDIUM]
3. **TASK-147**: Wyckoff phase boundary -1 guard [MEDIUM]
4. **TASK-148**: VIX SlopePct zero guard + NaN check [LOW]
5. **TASK-149**: Circuit breaker negative duration + race condition fix [LOW]
