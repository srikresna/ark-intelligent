# TASK-178: Multi-Step Progress Indicator for Python Subprocess Commands

**Priority:** medium
**Type:** ux
**Estimated:** M
**Area:** internal/adapter/telegram/

## Deskripsi

Replace static "Computing..." loading message dengan multi-step progress yang di-edit setiap 3 detik. Menunjukkan ke user bahwa bot sedang aktif processing, bukan frozen.

## Progress Steps

```
Step 1 (0s):  ⏳ Fetching market data for EUR...
Step 2 (3s):  🔄 Running statistical models...
Step 3 (6s):  📊 Generating charts...
Step 4 (9s):  ✨ Finalizing analysis...
```

## Implementation

```go
func (h *Handler) runWithProgress(ctx, chatID, steps []string, work func() error) error {
    msgID, _ := h.bot.SendHTML(ctx, chatID, steps[0])
    step := 0
    ticker := time.NewTicker(3 * time.Second)
    done := make(chan error, 1)

    go func() { done <- work() }()

    for {
        select {
        case err := <-done:
            ticker.Stop()
            h.bot.DeleteMessage(ctx, chatID, msgID)
            return err
        case <-ticker.C:
            step++
            if step < len(steps) {
                h.bot.EditMessage(ctx, chatID, msgID, steps[step])
            }
        }
    }
}
```

## File Changes

- `internal/adapter/telegram/progress.go` — NEW: Progress indicator helper
- `internal/adapter/telegram/handler_quant.go` — Use progress for /quant
- `internal/adapter/telegram/handler_cta.go` — Use progress for /cta chart
- `internal/adapter/telegram/handler_vp.go` — Use progress for /vp

## Acceptance Criteria

- [ ] Loading message updates every 3 seconds with new step
- [ ] Steps context-aware: "Fetching EUR data" not generic "Loading..."
- [ ] Progress deleted and replaced with result on completion
- [ ] If command fails, progress cleaned up properly
- [ ] No Telegram API rate limit issues (max 4 edits per command)
- [ ] Fallback to static loading if EditMessage fails
