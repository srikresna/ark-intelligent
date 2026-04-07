# TASK-165: Panic Recovery di News Scheduler + Python Subprocess Goroutines

**Priority:** high
**Type:** refactor
**Estimated:** M
**Area:** internal/service/news/, internal/adapter/telegram/

## Deskripsi

Tambah panic recovery (`defer func() { if r := recover() ... }()`) ke semua goroutine yang belum protected. Focus: news scheduler Start() outer goroutines dan Python subprocess handler functions.

## File Changes

- `internal/service/news/scheduler.go` — Wrap 5 goroutines di Start() (line 114-127) dengan panic recovery + structured logging
- `internal/adapter/telegram/handler_vp.go` — Add defer recover() di runVPEngine()
- `internal/adapter/telegram/handler_cta.go` — Add defer recover() di chart generation functions
- `internal/adapter/telegram/handler_quant.go` — Add defer recover() di Python subprocess functions

## Pattern

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Error().Interface("panic", r).
                Str("stack", string(debug.Stack())).
                Msg("recovered from panic in scheduler goroutine")
        }
    }()
    s.runInitialSync(ctx)
}()
```

## Acceptance Criteria

- [ ] All 5 news scheduler goroutines have panic recovery
- [ ] All Python subprocess handler functions have panic recovery
- [ ] Panic recovery logs stack trace via structured logging
- [ ] Panic in one goroutine does not crash entire bot
- [ ] No behavior change — only safety wrapper added
- [ ] `go vet ./...` clean
