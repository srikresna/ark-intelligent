# TASK-044: Modernisasi interface{} → any — DONE

**Completed by:** Dev-B
**Completed at:** 2026-04-01 15:50 WIB
**Branch:** feat/TASK-044-interface-any

## Changes
- internal/adapter/telegram/api.go: map[string]interface{} → map[string]any
- internal/service/marketdata/deribit/client.go: dest interface{} → dest any

## Verification
- go build ./... ✅
- go vet ./... ✅
- Zero behavior change (pure alias rename)
