# TASK-166: Goroutine Pool for Bot Update Dispatch

**Priority:** high
**Type:** refactor
**Estimated:** M
**Area:** internal/adapter/telegram/

## Deskripsi

Replace unbounded `go b.handleUpdate()` di bot polling loop dengan worker pool pattern. Prevent goroutine explosion saat Telegram kirim burst updates.

## Detail Teknis

Current (bot.go line 205):
```go
for _, update := range updates {
    go b.handleUpdate(ctx, update) // UNBOUNDED
}
```

Proposed:
```go
// Worker pool with semaphore
type Bot struct {
    // ...
    workerSem chan struct{} // buffered channel, size = maxConcurrent
}

// In polling loop:
for _, update := range updates {
    b.workerSem <- struct{}{} // block if pool full
    go func(u Update) {
        defer func() { <-b.workerSem }()
        b.handleUpdate(ctx, u)
    }(update)
}
```

## File Changes

- `internal/adapter/telegram/bot.go` — Add worker semaphore, modify dispatch loop
- `internal/config/constants.go` — Add `MaxConcurrentHandlers = 20` constant

## Acceptance Criteria

- [ ] Maximum 20 concurrent handler goroutines (configurable)
- [ ] Burst updates queued, not dropped
- [ ] No goroutine leak — semaphore always released (defer)
- [ ] Panic in handler releases semaphore slot
- [ ] Log warning when pool is full (backpressure indicator)
- [ ] No behavior change for normal operation (<20 concurrent)
- [ ] `go vet ./...` clean
