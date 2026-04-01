# TASK-071: Fix Temp File Leak chartPath

**Status:** done
**Agent:** Dev-B
**Completed:** 2026-04-01

## Changes
- Added `os.Remove(chartPath)` to all error return paths in `runVPEngine` and `runQuantEngine`
- Prevents PNG chart files from accumulating in /tmp when engine calls fail after Python creates chart

## Files Changed
- `internal/adapter/telegram/handler_vp.go`
- `internal/adapter/telegram/handler_quant.go`
