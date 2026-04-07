# PHI-SEC-002: Add Goroutine Limiter for Telegram Update Handler

## Problem Statement
`internal/adapter/telegram/bot.go:208` spawns unbounded goroutines for each incoming update:
```go
for _, update := range updates {
    b.offset = update.UpdateID + 1
    go b.handleUpdate(ctx, update)  // ← Unbounded goroutine creation
}
```

Under high load or malicious flooding, this can exhaust memory and file descriptors, causing Denial of Service.

## Expected Behavior
- Limit concurrent update handling with worker pool
- Queue updates when workers are busy
- Drop oldest queued updates when queue is full (circuit breaker)
- Add metrics for queue depth and worker utilization

## Acceptance Criteria
- [ ] Implement worker pool with configurable max workers (default: 100)
- [ ] Add buffered channel for update queue (default: 1000)
- [ ] Implement graceful degradation when queue full (drop + log)
- [ ] Add prometheus-style metrics for:
  - `telegram_updates_queued`
  - `telegram_updates_dropped`
  - `telegram_workers_busy`
  - `telegram_handler_duration_ms`
- [ ] Add configuration env vars:
  - `TELEGRAM_MAX_WORKERS` (default: 100)
  - `TELEGRAM_QUEUE_SIZE` (default: 1000)
- [ ] Unit test pool behavior under load
- [ ] Verify no goroutine leaks via pprof

## Files to Modify
- `internal/adapter/telegram/bot.go` — main loop
- `internal/config/config.go` — new env vars
- Add tests in `internal/adapter/telegram/bot_test.go` (create if not exists)

## Implementation Sketch
```go
type UpdateWorkerPool struct {
    workers   int
    queue     chan Update
    sem       chan struct{}  // Counting semaphore
    wg        sync.WaitGroup
}

func (p *UpdateWorkerPool) Submit(update Update) bool {
    select {
    case p.queue <- update:
        return true
    default:
        metrics.IncTelegramUpdatesDropped()
        return false
    }
}
```

## Risk Assessment
**Impact**: HIGH — DoS vulnerability  
**Effort**: MEDIUM — 4-8 hours  
**Priority**: P0 (Critical)

## Related
- CRITICAL-001 in research audit
