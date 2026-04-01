# TASK-129: Error Retry Buttons — Claimed by Dev-C

**Agent:** Dev-C
**Claimed:** 2026-04-02T07:20+07:00
**Branch:** feat/TASK-129-error-retry-buttons
**Status:** in-progress

## Scope

Add error retry inline keyboard buttons to command handlers so users can
retry failed commands with one tap instead of retyping.

### Implementation:
1. ErrorRetryKeyboard helper in keyboard.go
2. retry: callback handler in handler.go
3. Apply to error paths in handler_wyckoff.go, handler_gex.go, handler_cta.go,
   handler_quant.go, handler_vp.go, handler_alpha.go
