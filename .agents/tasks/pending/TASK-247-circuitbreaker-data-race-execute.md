# TASK-247: BUG-003 — Circuit breaker data race: baca `b.failures`/`b.lastFailure` tanpa lock

**Priority:** low
**Type:** bugfix
**Estimated:** XS
**Area:** pkg/circuitbreaker/breaker.go
**Created by:** Research Agent
**Created at:** 2026-04-02 23:00 WIB

## Deskripsi

Di fungsi `Execute()`, setelah `allowRequest()` melepas mutex, ada pembacaan field `b.failures` dan `b.lastFailure` tanpa lock untuk keperluan error message formatting:

```go
func (b *Breaker) Execute(fn func() error) error {
    if !b.allowRequest() {
        return fmt.Errorf("%s: %w (failures=%d, retry after %v)",
            b.name, ErrCircuitOpen, b.failures,        // ← baca tanpa lock!
            b.resetTimeout-time.Since(b.lastFailure))  // ← baca tanpa lock!
    }
    // ...
}
```

Go race detector akan menandai ini sebagai data race karena goroutine lain bisa menulis `b.failures` (via `recordFailure()`) secara bersamaan.

## File yang Harus Diubah

- `pkg/circuitbreaker/breaker.go`
  - Ubah `allowRequest()` untuk mengembalikan snapshot nilai yang dibutuhkan saat circuit open, ATAU
  - Baca `b.failures` dan `b.lastFailure` di bawah lock sebelum return

## Implementasi

### Opsi A — snapshot di dalam lock (recommended):

Ubah `allowRequest()` menjadi internal helper yang return 3 nilai:

```go
// allowRequest returns (allowed, failureCount, lastFailureTime).
// Must be called without b.mu held.
func (b *Breaker) allowRequest() (allowed bool, failures int, lastFail time.Time) {
    b.mu.Lock()
    defer b.mu.Unlock()

    switch b.currentState() {
    case Closed:
        return true, 0, time.Time{}
    case HalfOpen:
        if b.probing {
            return false, b.failures, b.lastFailure
        }
        b.probing = true
        return true, 0, time.Time{}
    case Open:
        return false, b.failures, b.lastFailure
    }
    return true, 0, time.Time{}
}
```

Lalu di `Execute()`:
```go
func (b *Breaker) Execute(fn func() error) error {
    allowed, failures, lastFail := b.allowRequest()
    if !allowed {
        return fmt.Errorf("%s: %w (failures=%d, retry after %v)",
            b.name, ErrCircuitOpen, failures,
            b.resetTimeout-time.Since(lastFail))
    }
    // ...
}
```

**Catatan:** Jika TASK-246 (HalfOpen race fix) dikerjakan terlebih dulu, pastikan signature `allowRequest()` konsisten dengan solusi di TASK-246.

## Acceptance Criteria

- [ ] `b.failures` dan `b.lastFailure` tidak lagi dibaca di luar mutex di `Execute()`
- [ ] Error message tetap informatif (menampilkan failure count dan waktu retry)
- [ ] Go race detector tidak report race pada `Breaker.Execute` saat concurrent access
- [ ] `go build ./...` sukses

## Referensi

- `.agents/research/2026-04-02-23-bug-hunt-putaran11.md` — BUG-003
- `pkg/circuitbreaker/breaker.go:82–85` — lokasi bug
- TASK-246 — fix HalfOpen di file yang sama (bisa dikerjakan bersamaan)
