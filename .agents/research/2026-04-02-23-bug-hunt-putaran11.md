# Research: Bug Hunt — Putaran 11
**Date:** 2026-04-02 23:00 WIB
**Siklus:** 5/5 — Bug Hunt
**Putaran:** 11

---

## Metodologi

Analisis statis codebase Go secara menyeluruh (223 file). Fokus pada:
- Race conditions & goroutine safety
- Nil pointer dereferences
- Data integrity bugs (silent error discard)
- Logic errors dalam state machines

---

## Bug Yang Ditemukan

### BUG-001 — `notifyOwnerDebug` goroutine menangkap request-scoped `ctx`
**File:** `internal/adapter/telegram/handler.go:2585–2593`
**Severity:** Medium
**Type:** Use-after-cancel / goroutine safety

```go
func (h *Handler) notifyOwnerDebug(ctx context.Context, html string) {
    ownerID := h.bot.OwnerID()
    if ownerID <= 0 { return }
    go func() {
        _, _ = h.bot.SendHTML(ctx, ...)  // BUG: ctx dari request Telegram!
    }()
}
```

Goroutine ini menangkap `ctx` dari caller (Telegram request context). Jika koneksi Telegram sudah closed / timeout sebelum goroutine berjalan, `SendHTML` akan langsung return error `context canceled` secara silent. `chat_service.go:notifyOwner` sudah benar menggunakan `context.Background()`. Pattern ini harus disamakan.

---

### BUG-002 — Circuit breaker HalfOpen allows multiple concurrent probes
**File:** `pkg/circuitbreaker/breaker.go:130–140`
**Severity:** Medium
**Type:** Race condition / incorrect state machine semantics

```go
// allowRequest():
case HalfOpen:
    return true // allow one probe ← KOMENTAR BOHONG
```

`allowRequest()` melepas lock sebelum goroutine menjalankan `fn()`. Dua goroutine konkuren yang sama-sama memanggil `Execute()` di state HalfOpen keduanya akan lolos (keduanya lihat HalfOpen, keduanya return true). Ini melanggar semantik "satu probe" dari circuit breaker. Seharusnya ada flag `probing` yang di-set secara atomik di dalam lock sebelum return true.

---

### BUG-003 — Circuit breaker data race: `b.failures` & `b.lastFailure` dibaca tanpa lock
**File:** `pkg/circuitbreaker/breaker.go:82–85`
**Severity:** Low (display only, but triggers race detector)
**Type:** Data race

```go
if !b.allowRequest() {
    return fmt.Errorf("...(failures=%d, retry after %v)",
        b.name, ErrCircuitOpen, b.failures,        // ← race: no lock
        b.resetTimeout-time.Since(b.lastFailure))  // ← race: no lock
}
```

Setelah `allowRequest()` melepas mutex, `b.failures` dan `b.lastFailure` dibaca tanpa lock. Go race detector akan mendeteksi ini karena goroutine lain bisa menulis `b.failures` (via `recordFailure()`) secara bersamaan.

---

### BUG-004 — `socrataToRecord` menyimpan COT record dengan zero-time date
**File:** `internal/service/cot/fetcher.go:344–346`
**Severity:** Medium
**Type:** Silent error discard / data integrity

```go
reportDate, _ := time.Parse("2006-01-02T15:04:05.000", sr.ReportDate)
if reportDate.IsZero() && len(sr.ReportDate) >= 10 {
    reportDate, _ = time.Parse("2006-01-02", sr.ReportDate[:10])
}
// Jika KEDUA parse gagal → reportDate = time.Time{} (0001-01-01)
// Record ini tetap di-return dan di-save ke DB!
```

Jika kedua format parse gagal (misalnya format API berubah), record disimpan dengan `ReportDate = 0001-01-01 00:00:00 UTC`. Ini menyebabkan record tersebut:
- Muncul di query range sebagai record "paling lama" (1 Januari tahun 1)
- Mengacaukan sorting dan Z-score calculations yang bergantung pada urutan tanggal
- Tidak ada log warning sehingga sulit dideteksi

---

### BUG-005 — `generateOutlook` memanggil `h.newsRepo` tanpa nil guard
**File:** `internal/adapter/telegram/handler.go:912`
**Severity:** Low (latent nil-panic)
**Type:** Missing nil guard / defensive programming

```go
func (h *Handler) generateOutlook(...) error {
    // ...
    weekEvts, _ := h.newsRepo.GetByWeek(ctx, now.Format("20060102"))  // BUG: tidak ada nil check!
```

Sedangkan di tempat lain di handler yang sama:
```go
if editMsgID == 0 && h.newsRepo != nil {  // baris 756 — BENAR
    ...
    todayEvts, _ := h.newsRepo.GetByDate(...)
```

`generateOutlook` mengakses `h.newsRepo` tanpa nil guard. Saat ini `newsRepo` selalu di-set di `main.go`, tapi ini adalah latent bug — jika setup berubah atau test di-run dengan handler partial, akan terjadi nil pointer panic. Inkonsisten dengan pattern di handler yang sama.

---

## Summary

| ID     | File                              | Bug                                     | Severity |
|--------|-----------------------------------|-----------------------------------------|----------|
| BUG-001 | handler.go:2590                  | goroutine captures request ctx          | Medium   |
| BUG-002 | pkg/circuitbreaker/breaker.go    | HalfOpen allows concurrent probes       | Medium   |
| BUG-003 | pkg/circuitbreaker/breaker.go    | data race on b.failures/b.lastFailure   | Low      |
| BUG-004 | service/cot/fetcher.go:344       | zero-time date stored silently          | Medium   |
| BUG-005 | handler.go:912                   | h.newsRepo called without nil guard     | Low      |

Tasks dibuat: TASK-245 s/d TASK-249
