# TASK-095: Worker Pool untuk Telegram Polling Loop

**Priority:** HIGH
**Siklus:** 5 (Bug Hunting)
**Estimasi:** 2-3 jam

## Problem

`StartPolling()` di `internal/adapter/telegram/bot.go` spawn goroutine tanpa batas untuk setiap update:

```go
for _, update := range updates {
    go b.handleUpdate(ctx, update) // unbounded
}
```

Telegram returns up to 100 updates/call. Jika bot kena spam atau flood, ratusan goroutine concurrent bisa muncul — masing-masing bisa trigger AI call (30-120s latency). Ini bisa menyebabkan memory exhaustion di VPS kecil.

## Solution

Implementasi worker pool dengan semaphore buffered channel:

```go
// Add to Bot struct
workerSem chan struct{}

// In NewBot
b.workerSem = make(chan struct{}, 20) // max 20 concurrent handlers

// In StartPolling
for _, update := range updates {
    b.offset = update.UpdateID + 1
    b.workerSem <- struct{}{} // acquire slot
    go func(u Update) {
        defer func() { <-b.workerSem }() // release slot
        b.handleUpdate(ctx, u)
    }(update)
}
```

## Acceptance Criteria
- [ ] Max concurrent goroutines dari handleUpdate terbatas (default 20)
- [ ] Configurable via env/config (e.g., `HANDLER_CONCURRENCY=20`)
- [ ] Semaphore acquire tidak blocking context — gunakan select dengan ctx.Done()
- [ ] Unit test: verify concurrent cap dengan goroutine counter
- [ ] `go build ./...` clean

## Files to Modify
- `internal/adapter/telegram/bot.go`
- `cmd/bot/main.go` (jika perlu pass config)
