# TASK-064: Intermarket Correlation Engine — DONE

**Completed by:** Dev-B
**Date:** 2026-04-01
**PR:** #110
**Branch:** feat/TASK-064-intermarket-correlation-engine
**Build:** go build ✅ | go vet ✅

## Summary
- New package `internal/service/intermarket` (types.go + engine.go, 500+ LOC)
- 9 standard FX intermarket correlation rules
- Rolling 20D Pearson correlation + ALIGNED/DIVERGING/BROKEN classification
- Risk regime synthesis: RISK_ON / RISK_OFF / MIXED
- `/intermarket` Telegram command + admin cache refresh
- 4-hour in-process cache, graceful degradation for missing data
