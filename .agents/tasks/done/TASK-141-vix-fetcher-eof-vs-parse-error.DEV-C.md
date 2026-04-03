# TASK-141: VIX Fetcher EOF vs Parse Error Handling

**Status:** ✅ COMPLETED — Merged to main  
**Assigned:** Dev-C  
**Commit:** de4901e  
**Type:** bugfix
**Estimated:** S
**Area:** internal/service/vix
**Siklus:** Bugfix

## Description

Fixed VIX fetcher error handling to properly distinguish between EOF errors and JSON parse errors. This improves error telemetry and prevents false-positive failure alerts.

## Changes

- `internal/service/vix/fetcher.go`: Improved error classification in `FetchTermStructure`
- Distinguishes between network EOF (transient) vs parse errors (data format issues)
- Better error wrapping for telemetry

## Acceptance Criteria

- [x] VIX fetcher properly classifies EOF vs parse errors
- [x] Error telemetry shows correct error types
- [x] No behavior change for successful fetches
- [x] Merged to main (de4901e)

## Related

- PR that merged this work to main
- Part of VIX error handling improvements batch
