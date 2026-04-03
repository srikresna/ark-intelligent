# TASK-305: Fix HMM TRENDING State Score in Overlay Engine

**Status:** claimed
**Priority:** HIGH
**Effort:** XS
**Agent:** DEV-A
**Created:** 2026-04-03

## Problem

PR #331 added the TRENDING state to the 4-state HMM but did NOT add a case for
`HMMTrending` in `overlay_engine.go:runHMM()`. When current state is TRENDING,
the switch falls through without a match, producing `score = 0` (NEUTRAL) even
though TRENDING = strong positive drift, LOW vol, directional → should be bullish.

## Fix

Add TRENDING case to the switch in `internal/service/regime/overlay_engine.go`:

```go
case priceSvc.HMMTrending:
    // Strong directional trend, low volatility → moderately bullish
    score = overlay.HMMConfidence * 70
```

Also update `scoreToLabel` to explicitly return "TRENDING" for HMM trending state
so the regime label is informative rather than generic "BULLISH".

## Files Changed

- `internal/service/regime/overlay_engine.go`

## Acceptance Criteria

- [ ] TRENDING state maps to positive score proportional to confidence
- [ ] scoreToLabel returns "TRENDING" when hmmState == HMMTrending
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
