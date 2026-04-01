# TASK-077: Compact Mode — OutputMode + Toggle — DONE

**Completed by:** Dev-B
**Date:** 2026-04-01
**PR:** #112
**Branch:** feat/TASK-077-compact-mode-output
**Build:** go build ✅ | go vet ✅

## Summary
- Added OutputMinimal as third output mode (compact/full/minimal)
- Settings toggle cycles through all 3 modes
- COT overview: minimal shows one-line per currency with signal direction
- Macro dashboard: minimal shows 2-line regime + key numbers
- View toggle buttons show 2 alternatives for quick switching between modes
- 5 files changed, +133 -32 lines
