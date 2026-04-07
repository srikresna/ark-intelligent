# TASK-246: BUG-002 — Circuit breaker HalfOpen membolehkan multiple concurrent probes

**Priority:** medium
**Type:** bugfix
**Estimated:** S
**Area:** pkg/circuitbreaker/breaker.go
**Created by:** Research Agent
**Created at:** 2026-04-02 23:00 WIB

## Deskripsi

Circuit breaker di `pkg/circuitbreaker/breaker.go` memiliki komentar:
```
// HalfOpen: one probe request is allowed through. Success → Closed, Failure → Open.
```

Namun implementasinya mengizinkan **multiple concurrent probes** karena:
1. `allowRequest()` mengecek state di bawah lock, melihat `HalfOpen`, lalu melepas lock
2. `fn()` (probe function) berjalan **di luar lock**
3. Goroutine lain yang concurrent bisa masuk `allowRequest()` saat goroutine pertama masih di dalam `fn()`, melihat state masih `HalfOpen`, dan juga lolos

Ini melanggar semantik "satu probe" dan bisa membebani backend yang sedang dalam kondisi fragile dengan banyak request sekaligus.

## File yang Harus Diubah

- `pkg/circuitbreaker/breaker.go`
  - Tambah field `probing bool` di struct `Breaker`
  - Set `probing = true` saat HalfOpen mengizinkan request pertama (dalam lock)
  - Tolak request HalfOpen berikutnya jika `probing == true`
  - Reset `probing = false` saat `recordSuccess()` atau `recordFailure()` dipanggil

## Implementasi

### Tambah field ke struct:
```go
type Breaker struct {
    // ... existing fields ...
    probing bool // true when a probe is in-flight (HalfOpen only)
}
```

### Update `allowRequest()`:
```go
func (b *Breaker) allowRequest() bool {
    b.mu.Lock()
    defer b.mu.Unlock()

    switch b.currentState() {
    case Closed:
        return true
    case HalfOpen:
        if b.probing {
            return false // another probe is already in-flight
        }
        b.probing = true // claim the single probe slot
        return true
    case Open:
        return false
    }
    return true
}
```

### Update `recordSuccess()` dan `recordFailure()`:
```go
func (b *Breaker) recordSuccess() {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.probing = false  // release probe slot
    b.failures = 0
    b.lastSuccess = time.Now()
    if b.state != Closed {
        b.setState(Closed)
    }
}

func (b *Breaker) recordFailure() {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.probing = false  // release probe slot
    b.failures++
    b.lastFailure = time.Now()
    if b.state == HalfOpen {
        b.setState(Open)
        return
    }
    if b.failures >= b.maxFailures {
        b.setState(Open)
    }
}
```

### Update `Reset()`:
```go
func (b *Breaker) Reset() {
    b.mu.Lock()
    defer b.mu.Unlock()
    old := b.state
    b.state = Closed
    b.failures = 0
    b.probing = false  // add this
    if old != Closed && b.OnStateChange != nil {
        b.OnStateChange(b.name, old, Closed)
    }
}
```

## Acceptance Criteria

- [ ] Field `probing bool` ditambahkan ke struct `Breaker`
- [ ] `allowRequest()` menolak probe kedua saat `probing == true` dalam HalfOpen state
- [ ] `probing` di-reset di `recordSuccess()` dan `recordFailure()`
- [ ] `probing` di-reset di `Reset()`
- [ ] `go build ./...` sukses

## Referensi

- `.agents/research/2026-04-02-23-bug-hunt-putaran11.md` — BUG-002
- `pkg/circuitbreaker/breaker.go:130–140` — kode `allowRequest()`
