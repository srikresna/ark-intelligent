# TASK-168: Capture Python Subprocess Stderr + Fix Double CommandContext

**Priority:** medium
**Type:** fix
**Estimated:** S
**Area:** internal/adapter/telegram/

## Deskripsi

1. Fix double `exec.CommandContext()` allocation di handler_quant.go (first call wasted)
2. Replace `cmd.Stderr = os.Stderr` dengan buffer capture untuk structured logging

## Problem 1: Double Allocation (handler_quant.go:446-452)

```go
cmd := exec.CommandContext(context.Background(), ...) // WASTED
cmd = exec.CommandContext(cmdCtx, ...)                // REPLACES
```

Fix: remove first allocation.

## Problem 2: Stderr Direct Redirect

```go
cmd.Stderr = os.Stderr // Python errors go to process stderr, not structured log
```

Fix:
```go
var stderr bytes.Buffer
cmd.Stderr = &stderr
if err := cmd.Run(); err != nil {
    log.Error().Str("stderr", stderr.String()).Err(err).Msg("Python subprocess failed")
    return nil, fmt.Errorf("subprocess failed: %w", err)
}
```

## File Changes

- `internal/adapter/telegram/handler_quant.go` — Remove double CommandContext, capture stderr
- `internal/adapter/telegram/handler_vp.go` — Capture stderr
- `internal/adapter/telegram/handler_cta.go` — Capture stderr

## Acceptance Criteria

- [ ] Single exec.CommandContext per subprocess call
- [ ] Stderr captured in bytes.Buffer
- [ ] Python error messages logged via structured logger
- [ ] stderr.String() included in error return for debugging
- [ ] No behavior change for successful subprocess calls
