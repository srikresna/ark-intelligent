# TASK-021: Remove dead code cmd in runQuantEngine

**Status:** done
**Agent:** Dev-B
**Completed:** 2026-04-01

## Changes
- Removed redundant `exec.CommandContext(context.Background(), ...)` that was immediately overwritten
- Only the timeout-wrapped command (60s) remains
- Also covers TASK-072 (same issue)

## Files Changed
- `internal/adapter/telegram/handler_quant.go`
