# TASK-098: Fix Impact Recorder Goroutine Context Saat Shutdown

**Priority:** LOW
**Siklus:** 5 (Bug Hunting)
**Estimasi:** 30 menit

## Problem

Di `internal/service/news/scheduler.go` line 676:

```go
go func() {
    defer recover()
    s.impactRecorder.RecordImpact(ctx, ev, ev.SurpriseScore, []string{"15m", "30m", "1h", "4h"})
}()
```

Goroutine ini menggunakan `ctx` dari outer loop. Saat bot shutdown (ctx cancel), goroutine yang baru saja dilaunched akan langsung fail karena semua HTTP calls dalam `RecordImpact` akan return `context.Canceled`. Akibatnya: price impact tidak ter-record untuk event yang baru saja masuk sebelum shutdown.

## Solution

Gunakan detached context dengan timeout:

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            schedLog.Error().Interface("panic", r).Str("event", ev.Event).Msg("PANIC in RecordImpact goroutine")
        }
    }()
    // Detach dari parent ctx agar tidak ikut cancel saat shutdown
    // Tapi tetap ada timeout agar tidak hang selamanya
    recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    s.impactRecorder.RecordImpact(recordCtx, ev, ev.SurpriseScore, []string{"15m", "30m", "1h", "4h"})
}()
```

## Acceptance Criteria
- [ ] Goroutine RecordImpact tidak terpengaruh shutdown ctx
- [ ] Timeout 5 menit mencegah goroutine hang selamanya
- [ ] Existing behavior tidak berubah (tetap non-blocking)
- [ ] `go build ./...` clean

## Files to Modify
- `internal/service/news/scheduler.go`
