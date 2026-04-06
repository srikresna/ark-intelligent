---
id: TASK-CODEQUALITY-005
title: Add ok check for type assertion in sentiment cache
status: in_progress
priority: low
effort: 15m
assigned_to: dev-a
created_by: research
created_at: 2026-04-06T05:13:00Z
claimed_at: 2026-04-06T07:35:00Z
claimed_by: Dev-A
---

## Summary

Add type assertion ok check in `sentiment/cache.go` `GetCachedOrFetch` function for defensive programming.

The `GetCachedOrFetch` function in `sentiment/cache.go` performs an unchecked type assertion at line 116. While the type is guaranteed by singleflight, adding the check improves code robustness.

## Location

- File: `internal/service/sentiment/cache.go`
- Line: 116
- Current code:
```go
return v.(*SentimentData), nil
```

## Expected Behavior

Add an ok check for the type assertion:
```go
data, ok := v.(*SentimentData)
if !ok {
    return nil, fmt.Errorf("unexpected type from singleflight: %T", v)
}
return data, nil
```

## Acceptance Criteria

- [ ] Add type assertion with ok check
- [ ] Return meaningful error if type assertion fails
- [ ] Ensure error is logged by caller

## Context

The singleflight group guarantees the type (it's only set in one place in the Do block), but defensive programming recommends checking. This is a low-priority defensive coding improvement.

## Related

- Uses `golang.org/x/sync/singleflight` for request coalescing
- The type is guaranteed because only `*SentimentData` is returned from the Do function
